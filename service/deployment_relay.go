package service

import (
	"bytes"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// ShouldPreferDeployments reports whether Model Deployments take priority over upstream channels.
func ShouldPreferDeployments() bool {
	common.OptionMapRWMutex.RLock()
	p := strings.TrimSpace(common.OptionMap["model_deployment.priority"])
	common.OptionMapRWMutex.RUnlock()
	return p != "channels_first"
}

func deploymentMatchesGroup(groupCSV, targetGroup string) bool {
	target := strings.TrimSpace(targetGroup)
	if target == "" {
		target = "default"
	}
	if strings.TrimSpace(groupCSV) == "" {
		return target == "default" || target == "auto"
	}
	for _, g := range strings.Split(groupCSV, ",") {
		g = strings.TrimSpace(g)
		if g == "" {
			continue
		}
		if g == "all" || target == "auto" || strings.EqualFold(g, target) {
			return true
		}
	}
	return false
}

// FindMatchingDeployments returns enabled, running deployments serving modelName for usingGroup.
func FindMatchingDeployments(modelName, usingGroup string) []model.NotebookDeployment {
	items, err := model.ListNotebookDeployments("")
	if err != nil || len(items) == 0 {
		return nil
	}
	targetModel := strings.TrimSpace(modelName)
	var out []model.NotebookDeployment
	for _, d := range items {
		if d.Enabled == 2 || !strings.EqualFold(d.Status, "running") || strings.TrimSpace(d.EndpointURL) == "" {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(d.ModelRepo), targetModel) &&
			!strings.EqualFold(strings.TrimSpace(d.Name), targetModel) {
			continue
		}
		if !deploymentMatchesGroup(d.Group, usingGroup) {
			continue
		}
		out = append(out, d)
	}
	return out
}

// PickWeightedDeployment selects a deployment by highest priority, then weighted random.
func PickWeightedDeployment(candidates []model.NotebookDeployment) model.NotebookDeployment {
	if len(candidates) == 1 {
		return candidates[0]
	}
	maxPrio := candidates[0].Priority
	for _, c := range candidates[1:] {
		if c.Priority > maxPrio {
			maxPrio = c.Priority
		}
	}
	var top []model.NotebookDeployment
	var totalWeight int
	for _, c := range candidates {
		if c.Priority == maxPrio {
			w := int(c.Weight)
			if w <= 0 {
				w = 1
			}
			totalWeight += w
			top = append(top, c)
		}
	}
	if len(top) == 1 || totalWeight <= 0 {
		return top[0]
	}
	r := rand.Intn(totalWeight)
	for _, c := range top {
		w := int(c.Weight)
		if w <= 0 {
			w = 1
		}
		if r < w {
			return c
		}
		r -= w
	}
	return top[0]
}

// TryHandleDeploymentRelay proxies the incoming OpenAI-compatible request to a matching
// enabled & running Model Deployment if one exists.
func TryHandleDeploymentRelay(c *gin.Context, modelName, usingGroup string) bool {
	candidates := FindMatchingDeployments(modelName, usingGroup)
	if len(candidates) == 0 {
		return false
	}
	picked := PickWeightedDeployment(candidates)

	var bodyBytes []byte
	if c.Request.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(c.Request.Body)
		if err == nil {
			c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}
	}

	baseEndpoint := strings.TrimRight(strings.TrimSpace(picked.EndpointURL), "/")
	reqPath := c.Request.URL.Path
	isJobEligible := (picked.Provider == "colab" || picked.Provider == "kaggle" || picked.Provider == "modal") &&
		c.Request.Method == http.MethodPost &&
		(reqPath == "/v1/images/generations" || reqPath == "/v1/videos/generations" || reqPath == "/v1/audio/speech" || reqPath == "/v1/audio/generations" || reqPath == "/v1/chat/completions") &&
		!bytes.Contains(bodyBytes, []byte(`"stream":true`)) && !bytes.Contains(bodyBytes, []byte(`"stream": true`))

	if isJobEligible {
		shortClient := &http.Client{Timeout: 15 * time.Second}
		if jReq, jErr := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, baseEndpoint+"/_jobs", bytes.NewReader(bodyBytes)); jErr == nil {
			jReq.Header.Set("Content-Type", "application/json")
			if jResp, doErr := shortClient.Do(jReq); doErr == nil {
				jBytes, _ := io.ReadAll(io.LimitReader(jResp.Body, 1<<20))
				_ = jResp.Body.Close()
				if jResp.StatusCode >= 200 && jResp.StatusCode < 300 {
					var started struct {
						JobID string `json:"job_id"`
					}
					if common.Unmarshal(jBytes, &started) == nil && started.JobID != "" {
						pollURL := baseEndpoint + "/_jobs/" + url.PathEscape(started.JobID)
						deadline := time.Now().Add(15 * time.Minute)
						consecutiveErrs := 0
						for time.Now().Before(deadline) {
							select {
							case <-c.Request.Context().Done():
								return true
							case <-time.After(1500 * time.Millisecond):
							}
							pReq, pErr := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, pollURL, nil)
							if pErr != nil {
								return false
							}
							pResp, pDoErr := shortClient.Do(pReq)
							if pDoErr != nil {
								consecutiveErrs++
								if consecutiveErrs >= 8 {
									return false
								}
								continue
							}
							pBytes, _ := io.ReadAll(io.LimitReader(pResp.Body, 8<<20))
							_ = pResp.Body.Close()
							if pResp.StatusCode == http.StatusBadGateway || pResp.StatusCode == http.StatusServiceUnavailable || pResp.StatusCode == http.StatusGatewayTimeout || pResp.StatusCode == 524 {
								consecutiveErrs++
								if consecutiveErrs >= 8 {
									return false
								}
								continue
							}
							consecutiveErrs = 0
							var st struct {
								Status string         `json:"status"`
								Result map[string]any `json:"result"`
								Error  string         `json:"error"`
							}
							if common.Unmarshal(pBytes, &st) != nil {
								continue
							}
							if st.Status == "completed" {
								c.JSON(http.StatusOK, st.Result)
								return true
							}
							if st.Status == "failed" {
								c.JSON(http.StatusInternalServerError, gin.H{"error": st.Error})
								return true
							}
						}
						return false
					}
				}
			}
		}
	}

	targetURL := baseEndpoint + c.Request.URL.RequestURI()
	req, err := http.NewRequestWithContext(c.Request.Context(), c.Request.Method, targetURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return false
	}
	if ct := c.GetHeader("Content-Type"); ct != "" {
		req.Header.Set("Content-Type", ct)
	} else {
		req.Header.Set("Content-Type", "application/json")
	}
	if acc := c.GetHeader("Accept"); acc != "" {
		req.Header.Set("Accept", acc)
	}

	client := &http.Client{Timeout: 600 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	for k, vals := range resp.Header {
		if strings.EqualFold(k, "Content-Length") || strings.EqualFold(k, "Transfer-Encoding") {
			continue
		}
		for _, v := range vals {
			c.Writer.Header().Add(k, v)
		}
	}
	c.Status(resp.StatusCode)
	_, _ = io.Copy(c.Writer, resp.Body)
	return true
}
