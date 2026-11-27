package service

import (
	"bytes"
	"context"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type fineTuningServingStopFunc func(jobID string, reason string)
type fineTuningServingWakeFunc func(ctx context.Context, jobID string) (*model.FineTuningJob, error)

var (
	ftServingStopHookMu sync.RWMutex
	ftServingStopHook   fineTuningServingStopFunc
	ftServingWakeHook   fineTuningServingWakeFunc
)

// RegisterFineTuningServingStopHook registers the controller-level teardown callback
// used when a private fine-tuned model must be immediately stopped (e.g. balance depleted).
func RegisterFineTuningServingStopHook(fn fineTuningServingStopFunc) {
	ftServingStopHookMu.Lock()
	ftServingStopHook = fn
	ftServingStopHookMu.Unlock()
}

// RegisterFineTuningServingWakeHook registers the controller-level auto-wake callback
// used when a request arrives for a sleeping fine-tuned model with auto-wake enabled.
func RegisterFineTuningServingWakeHook(fn fineTuningServingWakeFunc) {
	ftServingStopHookMu.Lock()
	ftServingWakeHook = fn
	ftServingStopHookMu.Unlock()
}

// TriggerFineTuningServingAutoStop invokes the registered remote-runtime stop hook asynchronously.
func TriggerFineTuningServingAutoStop(jobID string, reason string) {
	ftServingStopHookMu.RLock()
	fn := ftServingStopHook
	ftServingStopHookMu.RUnlock()
	if fn != nil {
		go fn(jobID, reason)
	}
}

// WakeFineTuningServingAndWait invokes the registered auto-wake hook to bring a sleeping
// private fine-tuned model back online before forwarding the caller's request.
func WakeFineTuningServingAndWait(ctx context.Context, jobID string) (*model.FineTuningJob, error) {
	ftServingStopHookMu.RLock()
	fn := ftServingWakeHook
	ftServingStopHookMu.RUnlock()
	if fn == nil {
		return nil, nil
	}
	return fn(ctx, jobID)
}

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
		if !strings.EqualFold(strings.TrimSpace(d.ModelID), targetModel) &&
			!strings.EqualFold(strings.TrimSpace(d.ModelRepo), targetModel) &&
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

// TryHandleDeploymentRelay proxies the incoming OpenAI-compatible request to matching
// enabled & running Model Deployments, falling back across candidates on upstream errors.
func TryHandleDeploymentRelay(c *gin.Context, modelName, usingGroup string) bool {
	candidates := FindMatchingDeployments(modelName, usingGroup)
	if len(candidates) == 0 {
		return false
	}

	var bodyBytes []byte
	if c.Request.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(c.Request.Body)
		if err == nil {
			c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}
	}

	remaining := append([]model.NotebookDeployment{}, candidates...)
	for len(remaining) > 0 {
		picked := PickWeightedDeployment(remaining)
		filtered := remaining[:0]
		for _, item := range remaining {
			if item.Id != picked.Id {
				filtered = append(filtered, item)
			}
		}
		remaining = filtered

		if tryProxyDeployment(c, picked, bodyBytes) {
			return true
		}
		if c.Request.Context().Err() != nil {
			return true
		}
	}
	return false
}

// TryHandleUserFineTunedRelay proxies OpenAI/Playground requests directly to a
// user's own actively hosted (or auto-wake sleeping) fine-tuned model. Strictly isolated to userId.
func TryHandleUserFineTunedRelay(c *gin.Context, userId int, modelName string) bool {
	if userId <= 0 || strings.TrimSpace(modelName) == "" || !IsFineTuningServingEnabled() {
		return false
	}
	job, err := model.GetUserActiveServingFineTuningJobByModel(userId, modelName)
	if err != nil || job == nil {
		return false
	}

	// Enforce positive authoritative user balance before forwarding or waking; auto-stop if depleted.
	if quota, qErr := model.GetUserQuota(userId, true); qErr == nil && quota <= 0 {
		TriggerFineTuningServingAutoStop(job.JobID, "insufficient_balance")
		c.JSON(http.StatusPaymentRequired, gin.H{
			"error": gin.H{
				"message": "Your balance has been exhausted; this hosted fine-tuned model has been stopped.",
				"type":    "insufficient_quota",
				"code":    "insufficient_quota",
			},
		})
		return true
	}

	// If the model is currently sleeping (idle auto-sleep) or waking up, automatically wake it and wait.
	if job.ServingStatus == "sleeping" || job.ServingStatus == "starting" || strings.TrimSpace(job.ServingEndpointURL) == "" {
		if !job.ServingAutoWake {
			return false
		}
		wokeJob, wakeErr := WakeFineTuningServingAndWait(c.Request.Context(), job.JobID)
		if wakeErr != nil || wokeJob == nil || strings.TrimSpace(wokeJob.ServingEndpointURL) == "" {
			msg := "Hosted fine-tuned model failed to wake up. Please check its console logs or start it manually."
			if wakeErr != nil {
				msg = wakeErr.Error()
			}
			c.JSON(http.StatusBadGateway, gin.H{
				"error": gin.H{
					"message": msg,
					"type":    "server_error",
					"code":    "fine_tuned_serving_wake_failed",
				},
			})
			return true
		}
		job = wokeJob
	}

	// Touch last-request timestamp so the idle auto-sleep timer resets on every request.
	now := time.Now().Unix()
	if now-job.ServingLastRequestAt >= 5 {
		_ = model.UpdateFineTuningJobStatus(job.JobID, map[string]any{
			"serving_last_request_at": now,
		})
	}

	var bodyBytes []byte
	if c.Request.Body != nil {
		if b, rErr := io.ReadAll(c.Request.Body); rErr == nil {
			bodyBytes = b
			c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}
	}

	picked := model.NotebookDeployment{
		Id:          job.ServingDeploymentID,
		OwnerUserID: userId,
		Provider:    job.ServingProvider,
		ModelID:     job.FineTunedModel,
		ModelRepo:   job.Model,
		EndpointURL: job.ServingEndpointURL,
		Status:      "running",
	}
	if tryProxyDeployment(c, picked, bodyBytes) {
		return true
	}
	if c.Request.Context().Err() != nil {
		return true
	}
	c.JSON(http.StatusBadGateway, gin.H{
		"error": gin.H{
			"message": "Hosted fine-tuned model endpoint did not respond. Please verify its status or restart it.",
			"type":    "server_error",
			"code":    "fine_tuned_serving_unreachable",
		},
	})
	return true
}

func normalizeDeploymentRequestPath(rawPath string) string {
	if strings.HasPrefix(rawPath, "/pg/") {
		return "/v1/" + strings.TrimPrefix(rawPath, "/pg/")
	}
	return rawPath
}

func tryProxyDeployment(c *gin.Context, picked model.NotebookDeployment, bodyBytes []byte) bool {
	baseEndpoint := strings.TrimRight(strings.TrimSpace(picked.EndpointURL), "/")
	reqPath := normalizeDeploymentRequestPath(c.Request.URL.Path)
	// Use async /_jobs polling only for slow media generation endpoints (image/video/audio).
	// /v1/chat/completions proxies directly for minimal TTFT & zero 1.5s poll latency!
	isJobEligible := (picked.Provider == "colab" || picked.Provider == "kaggle" || picked.Provider == "modal") &&
		c.Request.Method == http.MethodPost &&
		(reqPath == "/v1/images/generations" || reqPath == "/v1/videos/generations" || reqPath == "/v1/audio/speech" || reqPath == "/v1/audio/generations") &&
		!bytes.Contains(bodyBytes, []byte(`"stream":true`)) && !bytes.Contains(bodyBytes, []byte(`"stream": true`))

	if isJobEligible {
		jobStartClient := &http.Client{Timeout: 60 * time.Second}
		shortClient := &http.Client{Timeout: 15 * time.Second}
		if jReq, jErr := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, baseEndpoint+"/_jobs", bytes.NewReader(bodyBytes)); jErr == nil {
			jReq.Header.Set("Content-Type", "application/json")
			if jResp, doErr := jobStartClient.Do(jReq); doErr == nil {
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
								return false
							}
						}
						return false
					}
				}
			}
		}
	}

	reqURI := normalizeDeploymentRequestPath(c.Request.URL.RequestURI())
	targetURL := baseEndpoint + reqURI
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

	if resp.StatusCode >= 500 {
		return false
	}

	for k, vals := range resp.Header {
		if strings.EqualFold(k, "Content-Length") || strings.EqualFold(k, "Transfer-Encoding") {
			continue
		}
		for _, v := range vals {
			c.Writer.Header().Add(k, v)
		}
	}

	isSSE := strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream")
	if isSSE {
		c.Writer.Header().Set("X-Accel-Buffering", "no")
		c.Writer.Header().Set("Cache-Control", "no-cache")
	}
	c.Status(resp.StatusCode)
	if isSSE {
		flusher, canFlush := c.Writer.(http.Flusher)
		if canFlush {
			flusher.Flush()
		}
		buf := make([]byte, 4096)
		for {
			n, rErr := resp.Body.Read(buf)
			if n > 0 {
				if _, wErr := c.Writer.Write(buf[:n]); wErr != nil {
					return true
				}
				if canFlush {
					flusher.Flush()
				}
			}
			if rErr != nil {
				break
			}
		}
		return true
	}
	_, _ = io.Copy(c.Writer, resp.Body)
	return true
}
