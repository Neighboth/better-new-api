package controller

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/colab"
	"github.com/QuantumNous/new-api/pkg/hfendpoints"
	"github.com/QuantumNous/new-api/pkg/kaggle"
	"github.com/QuantumNous/new-api/pkg/modaldeploy"
	"github.com/QuantumNous/new-api/pkg/notebookscript"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
)

var (
	colabBaseURLOverride  string
	kaggleBaseURLOverride string
	colabRefresherOnce    sync.Once
)

func newColabClient(token string) *colab.Client {
	client := colab.NewClient(token)
	if colabBaseURLOverride != "" {
		client.BaseURL = colabBaseURLOverride
	}
	return client
}

func newKaggleClient(username, apiKey string) *kaggle.Client {
	client := kaggle.NewClient(username, apiKey)
	if kaggleBaseURLOverride != "" {
		client.BaseURL = kaggleBaseURLOverride
	}
	return client
}

func persistColabOption(key, value string) {
	common.OptionMapRWMutex.Lock()
	common.OptionMap[key] = value
	common.OptionMapRWMutex.Unlock()
	_ = model.UpdateOption(key, value)
}

// StartColabTokenRefresher starts a background goroutine that periodically
// refreshes all Google Colab OAuth access tokens before they expire.
func StartColabTokenRefresher() {
	colabRefresherOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(25 * time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				common.OptionMapRWMutex.RLock()
				enabled := common.OptionMap["model_deployment.colab.enabled"] == "true"
				common.OptionMapRWMutex.RUnlock()
				if !enabled {
					continue
				}
				accounts := GetProviderAccounts("colab")
				for _, acc := range accounts {
					if strings.TrimSpace(acc.RefreshToken) == "" {
						continue
					}
					ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
					_, _ = getValidColabAccessTokenForAccount(ctx, acc.ID)
					cancel()
				}
			}
		}()
	})
}

func getValidColabAccessToken(ctx context.Context) (string, error) {
	return getValidColabAccessTokenForAccount(ctx, "")
}

func getValidColabAccessTokenForAccount(ctx context.Context, accountID string) (string, error) {
	StartColabTokenRefresher()
	accounts := GetProviderAccounts("colab")
	idx := -1
	targetID := strings.TrimSpace(accountID)
	for i := range accounts {
		if targetID != "" && accounts[i].ID == targetID {
			idx = i
			break
		}
		if targetID == "" && accounts[i].Enabled && (strings.TrimSpace(accounts[i].AccessToken) != "" || strings.TrimSpace(accounts[i].RefreshToken) != "") {
			idx = i
			break
		}
	}
	if idx < 0 && len(accounts) > 0 && targetID == "" {
		idx = 0
	}
	if idx < 0 {
		return "", fmt.Errorf("Google Colab OAuth credentials missing; please sign in with Google")
	}

	acc := &accounts[idx]
	accessTok := strings.TrimSpace(acc.AccessToken)
	refreshTok := strings.TrimSpace(acc.RefreshToken)
	expiresAtStr := strings.TrimSpace(acc.ExpiresAt)

	needsRefresh := accessTok == ""
	if !needsRefresh && expiresAtStr != "" {
		if exp, err := time.Parse(time.RFC3339, expiresAtStr); err == nil {
			if time.Until(exp) < 5*time.Minute {
				needsRefresh = true
			}
		}
	}

	if !needsRefresh {
		return accessTok, nil
	}
	if refreshTok == "" {
		if accessTok != "" {
			return accessTok, nil
		}
		return "", fmt.Errorf("Google Colab OAuth credentials missing for account %s; please sign in with Google", acc.Label)
	}

	newAccess, newExp, err := service.RefreshColabToken(ctx, refreshTok)
	if err != nil {
		if accessTok != "" {
			return accessTok, nil
		}
		return "", err
	}
	acc.AccessToken = newAccess
	acc.ExpiresAt = newExp.Format(time.RFC3339)
	_ = SaveProviderAccounts("colab", accounts)
	return newAccess, nil
}

func StartColabOAuth(c *gin.Context) {
	flow, err := channelOAuthService.Generate(service.ChannelOAuthColab)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.ApiSuccess(c, flow)
}

func inferColabTierFromInfo(info *colab.AccountInfo) string {
	if info == nil {
		return ""
	}
	tierLower := strings.ToLower(info.SubscriptionTier)
	if strings.Contains(tierLower, "ultra") || strings.Contains(tierLower, "pro+") {
		return "google_ai_ultra"
	}
	hasA100OrH100 := false
	hasL4 := false
	hasV5e := false
	for _, g := range info.EligibleGPUs {
		u := strings.ToUpper(strings.TrimSpace(g))
		if strings.Contains(u, "H100") || strings.Contains(u, "A100") {
			hasA100OrH100 = true
		}
		if strings.Contains(u, "L4") {
			hasL4 = true
		}
	}
	for _, t := range info.EligibleTPUs {
		u := strings.ToUpper(strings.TrimSpace(t))
		if strings.Contains(u, "V5E") {
			hasV5e = true
		}
	}
	if hasA100OrH100 {
		return "google_ai_ultra"
	}
	if hasL4 {
		return "colab_pro"
	}
	if hasV5e {
		return "google_ai_pro"
	}
	return "free"
}

func ExchangeColabOAuth(c *gin.Context) {
	var req struct {
		SessionID string `json:"session_id" binding:"required"`
		Callback  string `json:"callback" binding:"required"`
		AccountID string `json:"account_id"`
		Label     string `json:"label"`
		Tier      string `json:"tier"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	credJSON, err := channelOAuthService.Exchange(context.WithoutCancel(c.Request.Context()), service.ChannelOAuthColab, req.SessionID, req.Callback)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	var cred struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		Expired      string `json:"expired"`
		Email        string `json:"email"`
		ProjectID    string `json:"project_id"`
	}
	if err := common.Unmarshal([]byte(credJSON), &cred); err != nil {
		common.ApiErrorMsg(c, "failed to parse OAuth credentials")
		return
	}

	var eligibleGPUs []string
	detectedTier := strings.TrimSpace(req.Tier)
	if info, infoErr := newColabClient(cred.AccessToken).GetAccountInfo(); infoErr == nil && info != nil {
		eligibleGPUs = append(eligibleGPUs, info.EligibleGPUs...)
		eligibleGPUs = append(eligibleGPUs, info.EligibleTPUs...)
		if detectedTier == "" {
			detectedTier = inferColabTierFromInfo(info)
		}
	}

	accounts := GetProviderAccounts("colab")
	targetID := strings.TrimSpace(req.AccountID)
	updated := false
	for i := range accounts {
		if (targetID != "" && accounts[i].ID == targetID) || (targetID == "" && i == 0) {
			accounts[i].AccessToken = cred.AccessToken
			if cred.RefreshToken != "" {
				accounts[i].RefreshToken = cred.RefreshToken
			}
			accounts[i].ExpiresAt = cred.Expired
			accounts[i].AccountEmail = cred.Email
			if cred.ProjectID != "" {
				accounts[i].ProjectID = cred.ProjectID
			}
			if len(eligibleGPUs) > 0 {
				accounts[i].EligibleGPUs = eligibleGPUs
			}
			if detectedTier != "" {
				accounts[i].Tier = detectedTier
			}
			accounts[i].Enabled = true
			targetID = accounts[i].ID
			updated = true
			break
		}
	}
	if !updated {
		if targetID == "" || targetID == "new" {
			targetID = fmt.Sprintf("acc-%d", len(accounts)+1)
		}
		label := strings.TrimSpace(req.Label)
		if label == "" {
			label = fmt.Sprintf("#%d", len(accounts)+1)
		}
		if detectedTier == "" {
			detectedTier = defaultTierForProvider("colab")
		}
		accounts = append(accounts, DeploymentAccount{
			ID:           targetID,
			Label:        label,
			Tier:         detectedTier,
			Enabled:      true,
			AccessToken:  cred.AccessToken,
			RefreshToken: cred.RefreshToken,
			ExpiresAt:    cred.Expired,
			AccountEmail: cred.Email,
			ProjectID:    cred.ProjectID,
			Region:       "us-central1",
			EligibleGPUs: eligibleGPUs,
		})
	}
	_ = SaveProviderAccounts("colab", accounts)
	StartColabTokenRefresher()

	common.ApiSuccess(c, gin.H{
		"account_id":    targetID,
		"email":         cred.Email,
		"expires_at":    cred.Expired,
		"project_id":    cred.ProjectID,
		"eligible_gpus": eligibleGPUs,
		"tier":          detectedTier,
		"accounts":      SummarizeProviderAccounts("colab"),
	})
}

func DisconnectColabOAuth(c *gin.Context) {
	var req struct {
		AccountID string `json:"account_id"`
	}
	rawBody, _ := c.GetRawData()
	if len(bytes.TrimSpace(rawBody)) > 0 {
		_ = common.Unmarshal(rawBody, &req)
	}
	targetID := strings.TrimSpace(req.AccountID)
	accounts := GetProviderAccounts("colab")
	if targetID != "" {
		for i := range accounts {
			if accounts[i].ID == targetID {
				accounts[i].AccessToken = ""
				accounts[i].RefreshToken = ""
				accounts[i].ExpiresAt = ""
				accounts[i].AccountEmail = ""
				accounts[i].EligibleGPUs = nil
				break
			}
		}
		_ = SaveProviderAccounts("colab", accounts)
	} else {
		if len(accounts) > 0 {
			accounts[0].AccessToken = ""
			accounts[0].RefreshToken = ""
			accounts[0].ExpiresAt = ""
			accounts[0].AccountEmail = ""
			accounts[0].EligibleGPUs = nil
			_ = SaveProviderAccounts("colab", accounts)
		}
		persistColabOption("model_deployment.colab.access_token", "")
		persistColabOption("model_deployment.colab.refresh_token", "")
		persistColabOption("model_deployment.colab.token_expires_at", "")
		persistColabOption("model_deployment.colab.account_email", "")
	}
	common.ApiSuccess(c, gin.H{"disconnected": true, "accounts": SummarizeProviderAccounts("colab")})
}

func TestColabConnection(c *gin.Context) {
	var req struct {
		AccessToken string `json:"access_token"`
		AccountID   string `json:"account_id"`
	}
	rawBody, _ := c.GetRawData()
	if len(bytes.TrimSpace(rawBody)) > 0 {
		_ = common.Unmarshal(rawBody, &req)
	}
	token := strings.TrimSpace(req.AccessToken)
	if token == "" {
		var err error
		token, err = getValidColabAccessTokenForAccount(c.Request.Context(), req.AccountID)
		if err != nil {
			common.ApiErrorMsg(c, err.Error())
			return
		}
	}

	client := newColabClient(token)
	count, err := client.TestConnection()
	if err != nil {
		common.ApiErrorMsg(c, fmt.Sprintf("failed to connect to Google Colab: %v", err))
		return
	}
	info, _ := client.GetAccountInfo()
	res := gin.H{
		"provider":       "colab",
		"status":         "connected",
		"runtimes_count": count,
	}
	if info != nil {
		res["eligible_gpus"] = info.EligibleGPUs
		res["eligible_tpus"] = info.EligibleTPUs
		res["subscription_tier"] = info.SubscriptionTier
	}
	common.ApiSuccess(c, res)
}

func TestKaggleConnection(c *gin.Context) {
	var req struct {
		AccountID string `json:"account_id"`
		Username  string `json:"username"`
		ApiKey    string `json:"api_key"`
	}
	rawBody, _ := c.GetRawData()
	if len(bytes.TrimSpace(rawBody)) > 0 {
		_ = common.Unmarshal(rawBody, &req)
	}
	username := strings.TrimSpace(req.Username)
	apiKey := strings.TrimSpace(req.ApiKey)

	if (apiKey == "" || apiKey == maskedAccountSecret) && strings.TrimSpace(req.AccountID) != "" {
		if acc, err := ResolveProviderAccount("kaggle", req.AccountID); err == nil && acc != nil {
			if username == "" {
				username = acc.Username
			}
			apiKey = acc.APIKey
		}
	}
	if apiKey == "" || apiKey == maskedAccountSecret {
		common.OptionMapRWMutex.RLock()
		if username == "" {
			username = strings.TrimSpace(common.OptionMap["model_deployment.kaggle.username"])
		}
		apiKey = strings.TrimSpace(common.OptionMap["model_deployment.kaggle.api_key"])
		common.OptionMapRWMutex.RUnlock()
	}

	if apiKey == "" {
		common.ApiErrorMsg(c, "Kaggle API key is required")
		return
	}

	kernels, err := newKaggleClient(username, apiKey).TestConnection()
	if err != nil {
		common.ApiErrorMsg(c, fmt.Sprintf("failed to connect to Kaggle: %v", err))
		return
	}
	common.ApiSuccess(c, gin.H{
		"provider":      "kaggle",
		"status":        "connected",
		"kernels_count": len(kernels),
	})
}

func ListNotebookDeployments(c *gin.Context) {
	provider := strings.TrimSpace(strings.ToLower(c.Query("provider")))
	items, err := model.ListNotebookDeployments(provider)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, items)
}

func GetNotebookDeployment(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid deployment id")
		return
	}
	item, err := model.GetNotebookDeploymentByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func GetNotebookDeploymentLogs(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid deployment id")
		return
	}
	item, err := model.GetNotebookDeploymentByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	combinedLogs := strings.TrimSpace(item.Logs)

	// Live log fetch from Colab runtime serve.log if proxy URL & token are available
	if item.Provider == "colab" && strings.TrimSpace(item.RemoteURL) != "" && strings.TrimSpace(item.ProxyToken) != "" {
		if tok, tokErr := getValidColabAccessTokenForAccount(c.Request.Context(), item.AccountID); tokErr == nil && tok != "" {
			client := newColabClient(tok)
			if rawLog, readErr := client.ReadFile(item.RemoteURL, item.ProxyToken, "serve.log"); readErr == nil && strings.TrimSpace(rawLog) != "" {
				combinedLogs = strings.TrimSpace(rawLog)
			} else if rawLog, readErr := client.ReadFile(item.RemoteURL, item.ProxyToken, "content/serve.log"); readErr == nil && strings.TrimSpace(rawLog) != "" {
				combinedLogs = strings.TrimSpace(rawLog)
			}
		}
	}

	// Live log fetch from running /_logs endpoint (Kaggle / Colab / Modal)
	if strings.TrimSpace(item.EndpointURL) != "" && strings.TrimSpace(item.CallbackToken) != "" && (item.Provider == "kaggle" || item.Provider == "colab" || item.Provider == "modal") {
		logsURL := strings.TrimRight(strings.TrimSpace(item.EndpointURL), "/") + "/_logs?token=" + url.QueryEscape(item.CallbackToken)
		httpClient := &http.Client{Timeout: 4 * time.Second}
		if resp, reqErr := httpClient.Get(logsURL); reqErr == nil {
			defer resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				var out struct {
					Logs string `json:"logs"`
				}
				if rawBytes, rErr := io.ReadAll(io.LimitReader(resp.Body, 64*1024)); rErr == nil {
					if common.Unmarshal(rawBytes, &out) == nil && strings.TrimSpace(out.Logs) != "" {
						if combinedLogs != "" {
							combinedLogs = combinedLogs + "\n--- Runtime Output ---\n" + strings.TrimSpace(out.Logs)
						} else {
							combinedLogs = strings.TrimSpace(out.Logs)
						}
					}
				}
			}
		}
	}

	// Live log snapshot from Kaggle SSE/output API when endpoint is not ready yet
	if item.Provider == "kaggle" && strings.TrimSpace(item.EndpointURL) == "" && strings.TrimSpace(item.RemoteID) != "" && ((item.Status != "failed" && item.Status != "stopped") || combinedLogs == "") {
		if acc, accErr := ResolveProviderAccount("kaggle", item.AccountID); accErr == nil && acc != nil && strings.TrimSpace(acc.APIKey) != "" {
			ownerSlug := strings.TrimSpace(acc.Username)
			kernelSlug := sanitizeSlug(item.Name)
			cleanedRef := strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(item.RemoteID), "/"), "code/")
			if parts := strings.SplitN(cleanedRef, "/", 2); len(parts) == 2 && parts[0] != "" && parts[1] != "" {
				ownerSlug = parts[0]
				kernelSlug = parts[1]
			}
			snapCtx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
			snapLogs, snapErr := newKaggleClient(acc.Username, acc.APIKey).GetKernelLogsSnapshot(snapCtx, ownerSlug, kernelSlug)
			cancel()
			if snapErr == nil && strings.TrimSpace(snapLogs) != "" {
				if combinedLogs != "" {
					seen := make(map[string]bool)
					for _, prev := range strings.Split(combinedLogs, "\n") {
						norm := strings.TrimSpace(leadingLogTimestampRe.ReplaceAllString(strings.TrimSpace(prev), ""))
						if norm != "" {
							seen[norm] = true
						}
					}
					var extra []string
					for _, ln := range strings.Split(strings.TrimSpace(snapLogs), "\n") {
						trimmed := strings.TrimSpace(ln)
						if trimmed == "" {
							continue
						}
						norm := strings.TrimSpace(leadingLogTimestampRe.ReplaceAllString(trimmed, ""))
						if norm != "" && !seen[norm] {
							seen[norm] = true
							extra = append(extra, trimmed)
						}
					}
					if len(extra) > 0 {
						combinedLogs = combinedLogs + "\n--- Kaggle Live Logs ---\n" + strings.Join(extra, "\n")
					}
				} else {
					combinedLogs = strings.TrimSpace(snapLogs)
				}
			}
		}
	}

	common.ApiSuccess(c, gin.H{
		"status":         item.Status,
		"status_message": item.StatusMessage,
		"error_message":  item.ErrorMessage,
		"logs":           combinedLogs,
	})
}

var slugSanitizerRe = regexp.MustCompile(`[^a-z0-9-]+`)

func sanitizeSlug(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = slugSanitizerRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "model-server"
	}
	return s
}

func isValidDeploymentProvider(p string) bool {
	switch p {
	case "colab", "kaggle", "modal", "huggingface", "ionet", "hostinger", "runpod":
		return true
	default:
		return false
	}
}

func getProviderIdleTimeoutSeconds(provider string) int {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	raw := strings.TrimSpace(common.OptionMap["model_deployment."+provider+".idle_timeout_seconds"])
	if raw == "" {
		return 300
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 0 {
		return 0
	}
	return v
}

func buildAcceleratorCandidates(primary string, fallbacks []string) []string {
	seen := make(map[string]bool)
	var out []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	add(primary)
	for _, fb := range fallbacks {
		add(fb)
	}
	return out
}

var (
	deploymentLogMu       sync.Mutex
	leadingLogTimestampRe = regexp.MustCompile(`^[0-9]{2}:[0-9]{2}:[0-9]{2}\s+`)
)

func appendDeploymentLog(id int, line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	normBody := strings.TrimSpace(leadingLogTimestampRe.ReplaceAllString(line, ""))
	deploymentLogMu.Lock()
	defer deploymentLogMu.Unlock()
	item, err := model.GetNotebookDeploymentByID(id)
	if err != nil {
		return
	}
	existing := strings.TrimSpace(item.Logs)
	if existing != "" && normBody != "" {
		for _, prev := range strings.Split(existing, "\n") {
			prevBody := strings.TrimSpace(leadingLogTimestampRe.ReplaceAllString(strings.TrimSpace(prev), ""))
			if prevBody == normBody {
				return
			}
		}
	}
	next := item.Logs
	if next != "" && !strings.HasSuffix(next, "\n") {
		next += "\n"
	}
	if leadingLogTimestampRe.MatchString(line) {
		next += line + "\n"
	} else {
		next += time.Now().Format("15:04:05") + " " + line + "\n"
	}
	if len(next) > 65536 {
		next = next[len(next)-65536:]
	}
	_ = model.UpdateNotebookDeploymentFields(id, map[string]any{
		"logs": next,
	})
}

func setDeploymentStatus(id int, status, message string) {
	if cur, err := model.GetNotebookDeploymentByID(id); err == nil && cur != nil {
		if cur.Status == status && cur.StatusMessage == message {
			return
		}
	}
	_ = model.UpdateNotebookDeploymentFields(id, map[string]any{
		"status":         status,
		"status_message": message,
		"updated_time":   time.Now().Unix(),
	})
	appendDeploymentLog(id, "["+status+"] "+message)
}

func setDeploymentFailed(id int, errMsg string) {
	_ = model.UpdateNotebookDeploymentFields(id, map[string]any{
		"status":         "failed",
		"status_message": errMsg,
		"error_message":  errMsg,
		"updated_time":   time.Now().Unix(),
	})
	appendDeploymentLog(id, "[failed] "+errMsg)
}

func buildDeploymentCallbackURL(c *gin.Context, id int, token string) string {
	base := strings.TrimSpace(system_setting.ServerAddress)
	if base == "" && c != nil && c.Request != nil && c.Request.Host != "" {
		scheme := "http"
		if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
			scheme = "https"
		}
		base = scheme + "://" + c.Request.Host
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Host == "" {
		return ""
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" || host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "0.0.0.0" {
		return ""
	}
	if ip := net.ParseIP(host); ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()) {
		return ""
	}
	return strings.TrimRight(base, "/") + fmt.Sprintf("/api/deployments/notebooks/%d/callback?token=%s", id, url.QueryEscape(token))
}

func NotebookDeploymentCallback(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid deployment id"})
		return
	}
	var req struct {
		Token       string `json:"token"`
		EndpointURL string `json:"endpoint_url"`
		Status      string `json:"status"`
		Log         string `json:"log"`
	}
	rawBody, _ := c.GetRawData()
	if len(bytes.TrimSpace(rawBody)) > 0 {
		_ = common.Unmarshal(rawBody, &req)
	}
	tok := strings.TrimSpace(c.Query("token"))
	if tok == "" {
		tok = strings.TrimSpace(req.Token)
	}
	item, err := model.GetNotebookDeploymentByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "deployment not found"})
		return
	}
	if tok == "" || item.CallbackToken == "" || subtle.ConstantTimeCompare([]byte(tok), []byte(item.CallbackToken)) != 1 {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "invalid callback token"})
		return
	}
	if logMsg := strings.TrimSpace(req.Log); logMsg != "" {
		appendDeploymentLog(id, logMsg)
		if item.Status != "running" && item.Status != "failed" {
			_ = model.UpdateNotebookDeploymentFields(id, map[string]any{
				"status_message": logMsg,
				"updated_time":   time.Now().Unix(),
			})
		}
		if strings.TrimSpace(req.EndpointURL) == "" {
			c.JSON(http.StatusOK, gin.H{"success": true})
			return
		}
	}
	epURL := strings.TrimSpace(req.EndpointURL)
	if !strings.HasPrefix(epURL, "http://") && !strings.HasPrefix(epURL, "https://") {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "valid endpoint_url is required"})
		return
	}
	nextStatus := "starting"
	nextMsg := "Tunnel connected, verifying model server health..."
	if item.Status == "running" {
		nextStatus = "running"
		nextMsg = "Running"
	}
	_ = model.UpdateNotebookDeploymentFields(id, map[string]any{
		"endpoint_url":   epURL,
		"status":         nextStatus,
		"status_message": nextMsg,
		"updated_time":   time.Now().Unix(),
	})
	appendDeploymentLog(id, "[tunnel] "+epURL)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

type createDeploymentRequest struct {
	Provider             string                         `json:"provider"`
	AccountID            string                         `json:"account_id"`
	Name                 string                         `json:"name"`
	ModelID              string                         `json:"model_id"`
	ModelRepo            string                         `json:"model_repo"`
	ModelFile            string                         `json:"model_file"`
	ModelFiles           []notebookscript.ModelFileSpec `json:"model_files"`
	TaskType             string                         `json:"task_type"`
	Accelerator          string                         `json:"accelerator"`
	FallbackAccelerators []string                       `json:"fallback_accelerators"`
	HighRAM              bool                           `json:"high_ram"`
	CPUCores             float64                        `json:"cpu_cores"`
	MemoryGB             int                            `json:"memory_gb"`
	Group                string                         `json:"group"`
	Priority             int64                          `json:"priority"`
	Weight               uint                           `json:"weight"`
	Enabled              int                            `json:"enabled"`
	HFToken              string                         `json:"hf_token"`
}

func createSingleNotebookDeployment(c *gin.Context, req createDeploymentRequest) (*model.NotebookDeployment, error) {
	provider := strings.TrimSpace(strings.ToLower(req.Provider))
	if !isValidDeploymentProvider(provider) {
		return nil, fmt.Errorf("unsupported deployment provider")
	}
	req.Name = strings.TrimSpace(req.Name)
	req.ModelRepo = strings.TrimSpace(req.ModelRepo)
	if req.Name == "" || req.ModelRepo == "" {
		return nil, fmt.Errorf("name and model_repo are required")
	}

	common.OptionMapRWMutex.RLock()
	modalEnabled := common.OptionMap["model_deployment.modal.enabled"] == "true"
	modalVolPath := strings.TrimSpace(common.OptionMap["model_deployment.modal.shared_volume_path"])
	kaggleEnabled := common.OptionMap["model_deployment.kaggle.enabled"] == "true"
	colabEnabled := common.OptionMap["model_deployment.colab.enabled"] == "true"
	hfEnabled := common.OptionMap["model_deployment.huggingface.enabled"] == "true"
	common.OptionMapRWMutex.RUnlock()

	var modalTokenID, modalTokenSecret string
	var kaggleUser, kaggleKey string
	var hfGlobalToken, hfDefaultOrg string
	var accountID, accountLabel string

	if acc, accErr := ResolveProviderAccount(provider, req.AccountID); accErr == nil && acc != nil {
		accountID = acc.ID
		accountLabel = acc.Label
		switch provider {
		case "modal":
			modalTokenID = strings.TrimSpace(acc.TokenID)
			modalTokenSecret = strings.TrimSpace(acc.TokenSecret)
		case "kaggle":
			kaggleUser = strings.TrimSpace(acc.Username)
			kaggleKey = strings.TrimSpace(acc.APIKey)
		case "huggingface":
			hfGlobalToken = strings.TrimSpace(acc.Token)
			hfDefaultOrg = strings.TrimSpace(acc.Org)
		}
	}

	switch provider {
	case "modal":
		if !modalEnabled || modalTokenID == "" || modalTokenSecret == "" {
			return nil, fmt.Errorf("Modal deployment is not enabled or Token ID / Token Secret is missing in System Settings")
		}
	case "kaggle":
		if !kaggleEnabled || kaggleKey == "" || kaggleUser == "" {
			return nil, fmt.Errorf("Kaggle deployment is not enabled or Username / API Key is missing in System Settings")
		}
	case "colab":
		if !colabEnabled {
			return nil, fmt.Errorf("Google Colab deployment is not enabled in System Settings")
		}
		if _, err := getValidColabAccessTokenForAccount(c.Request.Context(), accountID); err != nil {
			return nil, err
		}
	case "huggingface":
		if !hfEnabled || hfGlobalToken == "" {
			return nil, fmt.Errorf("Hugging Face deployment is not enabled or token is missing in System Settings")
		}
	}

	taskType := notebookscript.NormalizeTaskType(req.TaskType)
	idleTimeout := getProviderIdleTimeoutSeconds(provider)
	accelerator := strings.TrimSpace(req.Accelerator)
	if accelerator == "" {
		accelerator = "CPU"
	}
	candidates := buildAcceleratorCandidates(accelerator, req.FallbackAccelerators)
	fallbacksJSON, _ := common.Marshal(req.FallbackAccelerators)
	modelFilesJSON := ""
	if len(req.ModelFiles) > 0 {
		if b, err := common.Marshal(req.ModelFiles); err == nil {
			modelFilesJSON = string(b)
		}
	}
	groupStr := strings.TrimSpace(req.Group)
	if groupStr == "" {
		groupStr = "default"
	}
	weight := req.Weight
	if weight == 0 {
		weight = 1
	}
	enabled := req.Enabled
	if enabled != 2 {
		enabled = 1
	}
	cpuCores := req.CPUCores
	if cpuCores < 0 {
		cpuCores = 0
	} else if cpuCores > 64 {
		cpuCores = 64
	}
	memoryGB := req.MemoryGB
	if memoryGB < 0 {
		memoryGB = 0
	} else if memoryGB > 512 {
		memoryGB = 512
	}
	callbackToken := common.GetUUID()

	now := time.Now().Unix()
	item := &model.NotebookDeployment{
		Provider:             provider,
		AccountID:            accountID,
		AccountLabel:         accountLabel,
		Name:                 req.Name,
		ModelID:              strings.TrimSpace(req.ModelID),
		ModelRepo:            req.ModelRepo,
		ModelFile:            strings.TrimSpace(req.ModelFile),
		ModelFiles:           modelFilesJSON,
		TaskType:             taskType,
		Accelerator:          accelerator,
		FallbackAccelerators: string(fallbacksJSON),
		HighRAM:              req.HighRAM,
		CPUCores:             cpuCores,
		MemoryGB:             memoryGB,
		Group:                groupStr,
		Priority:             req.Priority,
		Weight:               weight,
		Enabled:              enabled,
		IdleTimeoutSeconds:   idleTimeout,
		Status:               "provisioning",
		StatusMessage:        "Initializing deployment...",
		CallbackToken:        callbackToken,
		CreatedTime:          now,
		UpdatedTime:          now,
	}

	if err := model.CreateNotebookDeployment(item); err != nil {
		return nil, err
	}

	callbackURL := buildDeploymentCallbackURL(c, item.Id, callbackToken)
	script := notebookscript.Generate(notebookscript.Config{
		Provider:             provider,
		DeploymentName:       req.Name,
		ModelRepo:            req.ModelRepo,
		ModelFile:            strings.TrimSpace(req.ModelFile),
		ModelFiles:           req.ModelFiles,
		TaskType:             taskType,
		Accelerator:          accelerator,
		FallbackAccelerators: req.FallbackAccelerators,
		IdleTimeoutSeconds:   idleTimeout,
		HFToken:              strings.TrimSpace(req.HFToken),
		CallbackURL:          callbackURL,
		ShutdownToken:        callbackToken,
	})
	item.Script = script
	_ = model.UpdateNotebookDeploymentFields(item.Id, map[string]any{
		"script": script,
	})

	modalFiles := make([]modaldeploy.ModelFileSpec, 0, len(req.ModelFiles))
	for _, mf := range req.ModelFiles {
		modalFiles = append(modalFiles, modaldeploy.ModelFileSpec{
			Repo: mf.Repo,
			File: mf.File,
			Role: mf.Role,
		})
	}

	go provisionDeploymentAsync(provisionParams{
		ID:                 item.Id,
		Provider:           provider,
		AccountID:          accountID,
		Name:               req.Name,
		ModelRepo:          req.ModelRepo,
		ModelFile:          strings.TrimSpace(req.ModelFile),
		ModelFiles:         modalFiles,
		TaskType:           taskType,
		PrimaryAccelerator: accelerator,
		Fallbacks:          req.FallbackAccelerators,
		Candidates:         candidates,
		HighRAM:            req.HighRAM,
		CPUCores:           cpuCores,
		MemoryGB:           memoryGB,
		IdleTimeoutSeconds: idleTimeout,
		HFToken:            strings.TrimSpace(req.HFToken),
		CallbackURL:        callbackURL,
		CallbackToken:      callbackToken,
		Script:             script,
		ModalTokenID:       modalTokenID,
		ModalTokenSecret:   modalTokenSecret,
		ModalVolPath:       modalVolPath,
		KaggleUser:         kaggleUser,
		KaggleKey:          kaggleKey,
		HFGlobalToken:      hfGlobalToken,
		HFDefaultOrg:       hfDefaultOrg,
	})

	return item, nil
}

func CreateNotebookDeployment(c *gin.Context) {
	var req createDeploymentRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "invalid request payload")
		return
	}
	item, err := createSingleNotebookDeployment(c, req)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.ApiSuccess(c, item)
}

func CreateNotebookDeploymentsBatch(c *gin.Context) {
	var req struct {
		Items []createDeploymentRequest `json:"items"`
	}
	if err := common.DecodeJson(c.Request.Body, &req); err != nil || len(req.Items) == 0 {
		common.ApiErrorMsg(c, "invalid batch deployment payload")
		return
	}
	created := make([]*model.NotebookDeployment, 0, len(req.Items))
	var errs []string
	for _, itemReq := range req.Items {
		item, err := createSingleNotebookDeployment(c, itemReq)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", itemReq.ModelRepo, err))
			continue
		}
		created = append(created, item)
	}
	if len(created) == 0 && len(errs) > 0 {
		common.ApiErrorMsg(c, strings.Join(errs, "; "))
		return
	}
	common.ApiSuccess(c, gin.H{
		"created": created,
		"errors":  errs,
	})
}

type provisionParams struct {
	ID                 int
	Provider           string
	AccountID          string
	Name               string
	ModelRepo          string
	ModelFile          string
	ModelFiles         []modaldeploy.ModelFileSpec
	TaskType           string
	PrimaryAccelerator string
	Fallbacks          []string
	Candidates         []string
	HighRAM            bool
	CPUCores           float64
	MemoryGB           int
	IdleTimeoutSeconds int
	HFToken            string
	CallbackURL        string
	CallbackToken      string
	Script             string
	ModalTokenID           string
	ModalTokenSecret       string
	ModalVolPath           string
	KaggleUser             string
	KaggleKey              string
	HFGlobalToken          string
	HFDefaultOrg           string
	FineTunedModelID       string
	FineTunedArtifactURL   string
	FineTunedArtifactToken string
	FineTuningMethod       string
}

func provisionDeploymentAsync(p provisionParams) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	switch p.Provider {
	case "modal":
		setDeploymentStatus(p.ID, "downloading", "Deploying container & downloading model on Modal...")
		appName := sanitizeSlug(p.Name)
		endpointURL, modalScript, err := modaldeploy.DeployWithLogger(ctx, modaldeploy.DeployConfig{
			AppName:                appName,
			ModelRepo:              p.ModelRepo,
			ModelFile:              p.ModelFile,
			ModelFiles:             p.ModelFiles,
			TaskType:               p.TaskType,
			Accelerator:            p.PrimaryAccelerator,
			FallbackAccelerators:   p.Fallbacks,
			CPUCores:               p.CPUCores,
			MemoryGB:               p.MemoryGB,
			IdleTimeoutSeconds:     p.IdleTimeoutSeconds,
			TokenID:                p.ModalTokenID,
			TokenSecret:            p.ModalTokenSecret,
			HFToken:                p.HFToken,
			SharedVolumePath:       p.ModalVolPath,
			ShutdownToken:          p.CallbackToken,
			FineTunedModelID:       p.FineTunedModelID,
			FineTunedArtifactURL:   p.FineTunedArtifactURL,
			FineTunedArtifactToken: p.FineTunedArtifactToken,
			FineTuningMethod:       p.FineTuningMethod,
		}, func(line string) {
			appendDeploymentLog(p.ID, "[modal] "+line)
			msg := line
			if len(msg) > 140 {
				msg = msg[:140]
			}
			_ = model.UpdateNotebookDeploymentFields(p.ID, map[string]any{
				"status_message": "Modal: " + msg,
				"updated_time":   time.Now().Unix(),
			})
		})
		if err != nil {
			setDeploymentFailed(p.ID, err.Error())
			return
		}
		_ = model.UpdateNotebookDeploymentFields(p.ID, map[string]any{
			"remote_id":      appName,
			"remote_url":     endpointURL,
			"endpoint_url":   endpointURL,
			"script":         modalScript,
			"status":         "starting",
			"status_message": "Warming up Modal endpoint...",
			"updated_time":   time.Now().Unix(),
		})
		if !probeHealth(endpointURL) {
			setDeploymentFailed(p.ID, "Endpoint health check failed (server did not respond with HTTP 200 on /health)")
			return
		}
		setDeploymentStatus(p.ID, "running", "Running")

	case "kaggle":
		setDeploymentStatus(p.ID, "downloading", "Pushing kernel to Kaggle...")
		shortID := sanitizeSlug(p.CallbackToken)
		if len(shortID) > 6 {
			shortID = shortID[:6]
		}
		if shortID == "" {
			shortID = strconv.FormatInt(time.Now().Unix()%100000, 10)
		}
		baseSlug := sanitizeSlug(p.Name)
		kernelSlug := baseSlug + "-" + shortID
		kaggleTitle := strings.ReplaceAll(kernelSlug, "-", " ")
		slug := p.KaggleUser + "/" + kernelSlug
		client := newKaggleClient(p.KaggleUser, p.KaggleKey)
		var pushed *kaggle.PushKernelResponse
		var chosenAcc string
		var lastErr error
		for _, cand := range p.Candidates {
			res, err := client.PushKernel(kaggle.PushKernelRequest{
				Slug:        slug,
				Title:       kaggleTitle,
				ScriptText:  p.Script,
				Accelerator: cand,
				IsPrivate:   true,
			})
			if err == nil {
				pushed = res
				chosenAcc = cand
				break
			}
			lastErr = err
		}
		if pushed == nil {
			setDeploymentFailed(p.ID, fmt.Sprintf("Kaggle push failed (tried %v): %v", p.Candidates, lastErr))
			return
		}
		appendDeploymentLog(p.ID, fmt.Sprintf("[kaggle] Kernel pushed: %s (v%d, accelerator=%s)", pushed.URL, pushed.VersionNumber, chosenAcc))
		remoteID := pushed.Ref
		if remoteID == "" {
			remoteID = slug
		}
		_ = model.UpdateNotebookDeploymentFields(p.ID, map[string]any{
			"accelerator":    chosenAcc,
			"remote_id":      remoteID,
			"remote_url":     pushed.URL,
			"status":         "starting",
			"status_message": "Queued on Kaggle...",
			"updated_time":   time.Now().Unix(),
		})

		ownerSlug := p.KaggleUser
		cleanedRef := strings.TrimPrefix(strings.TrimSpace(remoteID), "/")
		cleanedRef = strings.TrimPrefix(cleanedRef, "code/")
		if parts := strings.SplitN(cleanedRef, "/", 2); len(parts) == 2 && parts[0] != "" && parts[1] != "" {
			ownerSlug = parts[0]
			kernelSlug = parts[1]
		} else if pushed.URL != "" {
			if u, uErr := url.Parse(pushed.URL); uErr == nil {
				uPath := strings.TrimPrefix(strings.TrimPrefix(u.Path, "/"), "code/")
				if parts := strings.SplitN(uPath, "/", 2); len(parts) == 2 && parts[0] != "" && parts[1] != "" {
					ownerSlug = parts[0]
					kernelSlug = parts[1]
				}
			}
		}
		var tunnelURLRe = regexp.MustCompile(`https://[a-zA-Z0-9-]+\.trycloudflare\.com`)
		logCtx, cancelLogs := context.WithCancel(ctx)
		defer cancelLogs()
		go func() {
			seenLines := 0
			var discoveredURL string
			loggedErr := false
			for logCtx.Err() == nil {
				lineIdx := 0
				err := client.StreamKernelLogs(logCtx, ownerSlug, kernelSlug, func(line string) {
					lineIdx++
					if lineIdx <= seenLines {
						return
					}
					seenLines = lineIdx
					appendDeploymentLog(p.ID, line)
					if strings.Contains(line, "[tunnel] Ready:") {
						if m := tunnelURLRe.FindString(line); m != "" && discoveredURL == "" {
							discoveredURL = m
							_ = model.UpdateNotebookDeploymentFields(p.ID, map[string]any{
								"endpoint_url":   discoveredURL,
								"status":         "starting",
								"status_message": "Tunnel URL ready, verifying model server health...",
								"updated_time":   time.Now().Unix(),
							})
						}
					} else {
						msg := strings.TrimSpace(line)
						if len(msg) > 140 {
							msg = msg[len(msg)-140:]
						}
						if msg != "" {
							_ = model.UpdateNotebookDeploymentFields(p.ID, map[string]any{
								"status_message": "Kaggle: " + msg,
								"updated_time":   time.Now().Unix(),
							})
						}
					}
				})
				if err == nil || logCtx.Err() != nil {
					return
				}
				if !loggedErr && (strings.Contains(err.Error(), "HTTP 401") || strings.Contains(err.Error(), "HTTP 403")) {
					loggedErr = true
					appendDeploymentLog(p.ID, fmt.Sprintf("[kaggle] Log stream: %v", err))
				}
				select {
				case <-logCtx.Done():
					return
				case <-time.After(4 * time.Second):
				}
			}
		}()

		deadline := time.Now().Add(15 * time.Minute)
		queuedSince := time.Now()
		consecutiveStatusErrs := 0
		for time.Now().Before(deadline) {
			if cur, dbErr := model.GetNotebookDeploymentByID(p.ID); dbErr == nil && strings.TrimSpace(cur.EndpointURL) != "" {
				if !probeHealth(cur.EndpointURL) {
					setDeploymentFailed(p.ID, "Endpoint health check failed (server did not respond with HTTP 200 on /health)")
					return
				}
				setDeploymentStatus(p.ID, "running", "Running")
				return
			}
			ks, statusErr := client.GetKernelStatus(ownerSlug, kernelSlug)
			if statusErr != nil {
				if strings.Contains(statusErr.Error(), "HTTP 403") {
					msg := "Running on Kaggle, streaming logs & waiting for tunnel URL..."
					setDeploymentStatus(p.ID, "downloading", msg)
				} else {
					consecutiveStatusErrs++
					if consecutiveStatusErrs >= 3 {
						setDeploymentStatus(p.ID, "starting", fmt.Sprintf("Checking Kaggle status: %v", statusErr))
					}
				}
			} else if ks != nil {
				consecutiveStatusErrs = 0
				st := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(ks.Status)), "kernelworkerstatus.")
				switch st {
				case "new_script":
					setDeploymentStatus(p.ID, "starting", "Starting on Kaggle...")
				case "queued":
					if time.Since(queuedSince) > 90*time.Second {
						setDeploymentStatus(p.ID, "starting", "Queued on Kaggle (waiting for free GPU/TPU slot; ensure no other Kaggle session is running or switch to CPU/another account)")
					} else {
						setDeploymentStatus(p.ID, "starting", "Queued on Kaggle...")
					}
				case "running":
					// Only set generic message if no live Kaggle log line has populated status_message yet
					if cur, dbErr := model.GetNotebookDeploymentByID(p.ID); dbErr == nil && !strings.HasPrefix(cur.StatusMessage, "Kaggle: ") {
						setDeploymentStatus(p.ID, "downloading", "Running on Kaggle, installing dependencies & waiting for tunnel URL...")
					}
				case "cancel_requested", "cancel_acknowledged":
					setDeploymentFailed(p.ID, "Kaggle kernel was cancelled")
					return
				case "error":
					time.Sleep(1 * time.Second)
					if snap, sErr := client.GetKernelLogsSnapshot(ctx, ownerSlug, kernelSlug); sErr == nil && strings.TrimSpace(snap) != "" {
						for _, ln := range strings.Split(snap, "\n") {
							if strings.TrimSpace(ln) != "" {
								appendDeploymentLog(p.ID, ln)
							}
						}
					}
					errMsg := strings.TrimSpace(ks.FailureMessage)
					if errMsg == "" {
						errMsg = "Kaggle kernel failed during execution"
					}
					setDeploymentFailed(p.ID, errMsg)
					return
				case "complete":
					time.Sleep(1 * time.Second)
					if cur, dbErr := model.GetNotebookDeploymentByID(p.ID); dbErr == nil && strings.TrimSpace(cur.EndpointURL) != "" {
						if !probeHealth(cur.EndpointURL) {
							setDeploymentFailed(p.ID, "Endpoint health check failed (server did not respond with HTTP 200 on /health)")
							return
						}
						setDeploymentStatus(p.ID, "running", "Running")
						return
					}
					if snap, sErr := client.GetKernelLogsSnapshot(ctx, ownerSlug, kernelSlug); sErr == nil && strings.TrimSpace(snap) != "" {
						for _, ln := range strings.Split(snap, "\n") {
							if strings.TrimSpace(ln) != "" {
								appendDeploymentLog(p.ID, ln)
							}
						}
					}
					setDeploymentFailed(p.ID, "Kaggle kernel completed without reporting an endpoint URL")
					return
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(8 * time.Second):
			}
		}
		setDeploymentFailed(p.ID, "Timed out waiting for Kaggle kernel to start and report tunnel URL")
		return

	case "colab":
		setDeploymentStatus(p.ID, "starting", "Provisioning Google Colab runtime...")
		tok, err := getValidColabAccessTokenForAccount(ctx, p.AccountID)
		if err != nil {
			setDeploymentFailed(p.ID, err.Error())
			return
		}
		client := newColabClient(tok)
		var rt *colab.Runtime
		var chosenAcc string
		var lastErr error
		for _, cand := range p.Candidates {
			r, err := client.CreateRuntimeWithOptions(cand, p.HighRAM)
			if err == nil {
				rt = r
				chosenAcc = cand
				break
			}
			lastErr = err
		}
		if rt == nil {
			setDeploymentFailed(p.ID, fmt.Sprintf("Google Colab provisioning failed (tried %v): %v", p.Candidates, lastErr))
			return
		}
		proxyURL := strings.TrimSpace(rt.ConnectionInfo.URL)
		if proxyURL == "" {
			proxyURL = strings.TrimSpace(rt.ProxyURL)
		}
		_ = model.UpdateNotebookDeploymentFields(p.ID, map[string]any{
			"accelerator":    chosenAcc,
			"remote_id":      rt.Name,
			"remote_url":     proxyURL,
			"proxy_token":    rt.ConnectionInfo.Token,
			"status":         "downloading",
			"status_message": "Uploading model server & starting Colab kernel...",
			"updated_time":   time.Now().Unix(),
		})
		if proxyURL != "" {
			if upErr := client.UploadFile(proxyURL, rt.ConnectionInfo.Token, "serve.py", p.Script); upErr != nil {
				setDeploymentFailed(p.ID, fmt.Sprintf("Colab script upload failed: %v", upErr))
				return
			}
			kernelID, kernErr := client.StartKernel(proxyURL, rt.ConnectionInfo.Token)
			if kernErr != nil {
				setDeploymentFailed(p.ID, fmt.Sprintf("Colab kernel start failed: %v", kernErr))
				return
			}
			execSnippet := `import os, subprocess, sys
script_path = next((p for p in ['serve.py', '/content/serve.py', '/serve.py'] if os.path.exists(p)), None)
if not script_path:
    open('serve_error.txt', 'w').write('serve.py not found in runtime')
else:
    script_dir = os.path.dirname(os.path.abspath(script_path)) or '.'
    log_path = os.path.join(script_dir, 'serve.log')
    with open(log_path, 'w') as lf:
        lf.write('[boot] Starting serve.py...\n')
    runner = (
        f"import os, subprocess, sys\n"
        f"log_path = {log_path!r}\n"
        f"with open(log_path, 'a') as lf:\n"
        f"    rc = subprocess.call([sys.executable, '-u', {os.path.abspath(script_path)!r}], cwd={script_dir!r}, stdout=lf, stderr=subprocess.STDOUT)\n"
        f"if rc != 0:\n"
        f"    tail = open(log_path).read()[-500:] if os.path.exists(log_path) else ''\n"
        f"    open({os.path.join(script_dir, 'serve_error.txt')!r}, 'w').write(f'exit {{rc}}: {{tail}}')\n"
    )
    subprocess.Popen([sys.executable, '-c', runner], cwd=script_dir, start_new_session=True, stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
`
			if execErr := client.ExecuteCode(proxyURL, rt.ConnectionInfo.Token, kernelID, execSnippet); execErr != nil {
				setDeploymentFailed(p.ID, fmt.Sprintf("Colab script execution failed: %v", execErr))
				return
			}
			readColabFile := func(name string) (string, error) {
				if content, err := client.ReadFile(proxyURL, rt.ConnectionInfo.Token, name); err == nil {
					return content, nil
				}
				return client.ReadFile(proxyURL, rt.ConnectionInfo.Token, "content/"+strings.TrimLeft(name, "/"))
			}
			setDeploymentStatus(p.ID, "downloading", "Installing dependencies & starting tunnel on Google Colab...")
			deadline := time.Now().Add(10 * time.Minute)
			for time.Now().Before(deadline) {
				if cur, dbErr := model.GetNotebookDeploymentByID(p.ID); dbErr == nil && strings.TrimSpace(cur.EndpointURL) != "" {
					if !probeHealth(cur.EndpointURL) {
						setDeploymentFailed(p.ID, "Endpoint health check failed (server did not respond with HTTP 200 on /health)")
						return
					}
					setDeploymentStatus(p.ID, "running", "Running")
					return
				}
				if errText, errRead := readColabFile("serve_error.txt"); errRead == nil && strings.TrimSpace(errText) != "" {
					setDeploymentFailed(p.ID, "Colab server error: "+strings.TrimSpace(errText))
					return
				}
				if rawURL, readErr := readColabFile("endpoint_url.txt"); readErr == nil {
					epURL := strings.TrimSpace(rawURL)
					if strings.HasPrefix(epURL, "http://") || strings.HasPrefix(epURL, "https://") {
						_ = model.UpdateNotebookDeploymentFields(p.ID, map[string]any{
							"endpoint_url":   epURL,
							"status":         "starting",
							"status_message": "Warming up Colab endpoint...",
							"updated_time":   time.Now().Unix(),
						})
						if !probeHealth(epURL) {
							setDeploymentFailed(p.ID, "Endpoint health check failed (server did not respond with HTTP 200 on /health)")
							return
						}
						setDeploymentStatus(p.ID, "running", "Running")
						return
					}
				}
				if logText, logErr := readColabFile("serve.log"); logErr == nil && strings.TrimSpace(logText) != "" {
					_ = model.UpdateNotebookDeploymentFields(p.ID, map[string]any{
						"logs": strings.TrimSpace(logText),
					})
					lines := strings.Split(strings.TrimSpace(logText), "\n")
					lastLine := strings.TrimSpace(lines[len(lines)-1])
					if len(lastLine) > 140 {
						lastLine = lastLine[len(lastLine)-140:]
					}
					if lastLine != "" {
						setDeploymentStatus(p.ID, "downloading", "Colab: "+lastLine)
					}
				}
				select {
				case <-ctx.Done():
					return
				case <-time.After(5 * time.Second):
				}
			}
			setDeploymentFailed(p.ID, "Timed out waiting for Google Colab tunnel URL")
			return
		}
		setDeploymentStatus(p.ID, "running", "Running on Google Colab")

	case "huggingface":
		setDeploymentStatus(p.ID, "downloading", "Creating Hugging Face Inference Endpoint...")
		ns := p.HFDefaultOrg
		if ns == "" {
			if resolved, err := hfWhoamiName(p.HFGlobalToken); err == nil {
				ns = resolved
			}
		}
		if ns == "" {
			setDeploymentFailed(p.ID, "Hugging Face organization/username could not be resolved")
			return
		}
		scaleToZeroMin := 0
		if p.IdleTimeoutSeconds > 0 {
			scaleToZeroMin = (p.IdleTimeoutSeconds + 59) / 60
		}
		client := hfendpoints.NewClient(p.HFGlobalToken)
		var ep *hfendpoints.Endpoint
		var chosenAcc string
		var lastErr error
		for _, cand := range p.Candidates {
			created, err := client.CreateEndpoint(hfendpoints.CreateEndpointRequest{
				Name:                      sanitizeSlug(p.Name),
				Namespace:                 ns,
				Repository:                p.ModelRepo,
				Task:                      p.TaskType,
				Hardware:                  cand,
				Region:                    "us-east-1",
				Protected:                 true,
				ScaleToZeroTimeoutMinutes: scaleToZeroMin,
			})
			if err == nil {
				ep = created
				chosenAcc = cand
				break
			}
			lastErr = err
		}
		if ep == nil {
			setDeploymentFailed(p.ID, fmt.Sprintf("Hugging Face endpoint creation failed (tried %v): %v", p.Candidates, lastErr))
			return
		}
		_ = model.UpdateNotebookDeploymentFields(p.ID, map[string]any{
			"accelerator":    chosenAcc,
			"remote_id":      ep.Namespace + "/" + ep.Name,
			"remote_url":     ep.Status.URL,
			"endpoint_url":   ep.Status.URL,
			"status":         "starting",
			"status_message": "Building Hugging Face container...",
			"updated_time":   time.Now().Unix(),
		})
		// Poll briefly for URL if not immediately returned
		for i := 0; i < 20; i++ {
			time.Sleep(6 * time.Second)
			latest, err := client.GetEndpoint(ns, ep.Name)
			if err != nil {
				continue
			}
			st := strings.ToLower(latest.Status.State)
			if latest.Status.URL != "" && strings.Contains(st, "run") {
				_ = model.UpdateNotebookDeploymentFields(p.ID, map[string]any{
					"remote_url":     latest.Status.URL,
					"endpoint_url":   latest.Status.URL,
					"status":         "running",
					"status_message": "Running",
					"updated_time":   time.Now().Unix(),
				})
				return
			}
			if strings.Contains(st, "fail") || strings.Contains(st, "error") {
				setDeploymentFailed(p.ID, latest.Status.Message)
				return
			}
		}
		setDeploymentStatus(p.ID, "running", "Provisioned on Hugging Face")

	default:
		setDeploymentStatus(p.ID, "running", "Configured")
	}
}

func probeHealth(baseURL string) bool {
	if strings.TrimSpace(baseURL) == "" {
		return false
	}
	client := &http.Client{Timeout: 10 * time.Second}
	target := strings.TrimRight(baseURL, "/") + "/health"
	for i := 0; i < 60; i++ {
		resp, err := client.Get(target)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return true
			}
		}
		time.Sleep(5 * time.Second)
	}
	return false
}

func UpdateNotebookDeployment(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid deployment id")
		return
	}
	item, err := model.GetNotebookDeploymentByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var req struct {
		EndpointURL *string `json:"endpoint_url"`
		Status      *string `json:"status"`
		Group       *string `json:"group"`
		Priority    *int64  `json:"priority"`
		Weight      *uint   `json:"weight"`
		Enabled     *int    `json:"enabled"`
	}
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "invalid request payload")
		return
	}
	if req.EndpointURL != nil {
		if u := strings.TrimSpace(*req.EndpointURL); u != "" {
			item.EndpointURL = u
		}
	}
	if req.Status != nil {
		if st := strings.TrimSpace(*req.Status); st != "" {
			item.Status = st
		}
	}
	if req.Group != nil {
		g := strings.TrimSpace(*req.Group)
		if g == "" {
			g = "default"
		}
		item.Group = g
	}
	if req.Priority != nil {
		item.Priority = *req.Priority
	}
	if req.Weight != nil {
		w := *req.Weight
		if w == 0 {
			w = 1
		}
		item.Weight = w
	}
	if req.Enabled != nil {
		if *req.Enabled == 2 {
			item.Enabled = 2
		} else {
			item.Enabled = 1
		}
	}
	item.UpdatedTime = time.Now().Unix()
	if err := model.UpdateNotebookDeployment(item); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func stopNotebookDeploymentRemote(item *model.NotebookDeployment) error {
	if item == nil {
		return nil
	}
	bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var remoteStopErr error
	// Best-effort graceful shutdown of serve.py on Kaggle/Colab so GPU/TPU slot is freed immediately.
	if (item.Provider == "kaggle" || item.Provider == "colab") && strings.TrimSpace(item.EndpointURL) != "" && strings.TrimSpace(item.CallbackToken) != "" {
		shutdownURL := strings.TrimRight(strings.TrimSpace(item.EndpointURL), "/") + "/_shutdown?token=" + url.QueryEscape(item.CallbackToken)
		if req, reqErr := http.NewRequestWithContext(bgCtx, http.MethodPost, shutdownURL, nil); reqErr == nil {
			httpClient := &http.Client{Timeout: 5 * time.Second}
			if resp, doErr := httpClient.Do(req); doErr == nil {
				_ = resp.Body.Close()
			}
		}
	}

	acc, _ := ResolveProviderAccount(item.Provider, item.AccountID)
	switch item.Provider {
	case "modal":
		var modalTokenID, modalTokenSecret string
		if acc != nil {
			modalTokenID = acc.TokenID
			modalTokenSecret = acc.TokenSecret
		}
		appName := strings.TrimSpace(item.RemoteID)
		if appName == "" {
			appName = sanitizeSlug(item.Name)
		}
		if modalTokenID != "" && modalTokenSecret != "" && appName != "" {
			remoteStopErr = modaldeploy.StopApp(bgCtx, appName, modalTokenID, modalTokenSecret)
		}
	case "colab":
		if item.RemoteID != "" {
			if colabToken, tokErr := getValidColabAccessTokenForAccount(bgCtx, item.AccountID); tokErr == nil && colabToken != "" {
				remoteStopErr = newColabClient(colabToken).DeleteRuntime(item.RemoteID)
			} else if tokErr != nil {
				remoteStopErr = tokErr
			}
		}
	case "kaggle":
		if item.RemoteID != "" && acc != nil && acc.APIKey != "" {
			cleaned := strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(item.RemoteID), "/"), "code/")
			parts := strings.SplitN(cleaned, "/", 2)
			if len(parts) == 2 {
				remoteStopErr = newKaggleClient(acc.Username, acc.APIKey).DeleteKernel(parts[0], parts[1])
			}
		}
	case "huggingface":
		if item.RemoteID != "" && acc != nil && acc.Token != "" {
			parts := strings.SplitN(item.RemoteID, "/", 2)
			if len(parts) == 2 {
				remoteStopErr = hfendpoints.NewClient(acc.Token).DeleteEndpoint(parts[0], parts[1])
			}
		}
	}
	return remoteStopErr
}

func DeleteNotebookDeployment(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid deployment id")
		return
	}
	item, err := model.GetNotebookDeploymentByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	remoteStopErr := stopNotebookDeploymentRemote(item)
	if remoteStopErr != nil {
		common.SysError(fmt.Sprintf("DeleteNotebookDeployment remote cleanup failed for #%d (%s): %v", item.Id, item.Provider, remoteStopErr))
	}

	if err := model.DeleteNotebookDeployment(id); err != nil {
		common.ApiError(c, err)
		return
	}
	res := gin.H{"deleted": true}
	if remoteStopErr != nil {
		res["remote_stop_warning"] = remoteStopErr.Error()
	}
	common.ApiSuccess(c, res)
}

func TestNotebookDeployment(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid deployment id")
		return
	}
	item, err := model.GetNotebookDeploymentByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var req struct {
		Prompt string `json:"prompt"`
	}
	rawBody, _ := c.GetRawData()
	if len(bytes.TrimSpace(rawBody)) > 0 {
		_ = common.Unmarshal(rawBody, &req)
	}

	targetURL := strings.TrimSpace(item.EndpointURL)
	if targetURL == "" {
		if item.ErrorMessage != "" {
			common.ApiErrorMsg(c, fmt.Sprintf("deployment failed: %s", item.ErrorMessage))
		} else {
			common.ApiErrorMsg(c, fmt.Sprintf("deployment is currently %s (%s); endpoint is not ready yet", item.Status, item.StatusMessage))
		}
		return
	}

	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		prompt = "Hello! Reply briefly."
	}

	prevStatus := item.Status
	prevMsg := item.StatusMessage
	setDeploymentStatus(item.Id, "testing", "Testing model...")
	result, testErr := executeModelTest(targetURL, item.ModelRepo, item.TaskType, prompt, "")
	if prevStatus == "" || prevStatus == "testing" {
		prevStatus = "running"
	}
	if prevMsg == "" || prevMsg == "Testing model..." {
		prevMsg = "Running"
	}
	setDeploymentStatus(item.Id, prevStatus, prevMsg)

	if testErr != nil {
		common.ApiErrorMsg(c, testErr.Error())
		return
	}
	common.ApiSuccess(c, result)
}

func TestHfEndpoint(c *gin.Context) {
	token, ok := getHfToken(c)
	if !ok {
		return
	}
	namespace, err := hfResolveNamespace(c, token, c.Query("namespace"))
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	name := strings.TrimSpace(c.Param("name"))
	if name == "" {
		common.ApiErrorMsg(c, "endpoint name is required")
		return
	}
	ep, err := hfendpoints.NewClient(token).GetEndpoint(namespace, name)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if strings.TrimSpace(ep.Status.URL) == "" {
		common.ApiErrorMsg(c, "endpoint URL is not ready yet")
		return
	}

	var req struct {
		Prompt string `json:"prompt"`
	}
	rawBody, _ := c.GetRawData()
	if len(bytes.TrimSpace(rawBody)) > 0 {
		_ = common.Unmarshal(rawBody, &req)
	}
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		prompt = "Hello!"
	}

	result, testErr := executeModelTest(ep.Status.URL, ep.Model.Repository, ep.Model.Task, prompt, token)
	if testErr != nil {
		common.ApiErrorMsg(c, testErr.Error())
		return
	}
	common.ApiSuccess(c, result)
}

type modelTestResult struct {
	Type      string `json:"type"` // text | image | video | audio
	Text      string `json:"text,omitempty"`
	MediaURL  string `json:"media_url,omitempty"`
	LatencyMs int64  `json:"latency_ms"`
	Raw       string `json:"raw,omitempty"`
}

type modelTestJobState struct {
	JobID     string           `json:"job_id"`
	Status    string           `json:"status"` // running | completed | failed
	Progress  int              `json:"progress"`
	Stage     string           `json:"stage"`
	Result    *modelTestResult `json:"result,omitempty"`
	Error     string           `json:"error,omitempty"`
	LatencyMs int64            `json:"latency_ms"`
	UpdatedAt time.Time        `json:"-"`
}

var (
	notebookTestJobsMu sync.Mutex
	notebookTestJobs   = make(map[string]*modelTestJobState)
)

func StartNotebookDeploymentTest(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid deployment id")
		return
	}
	item, err := model.GetNotebookDeploymentByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var req struct {
		Prompt string `json:"prompt"`
	}
	rawBody, _ := c.GetRawData()
	if len(bytes.TrimSpace(rawBody)) > 0 {
		_ = common.Unmarshal(rawBody, &req)
	}
	targetURL := strings.TrimSpace(item.EndpointURL)
	if targetURL == "" {
		if item.ErrorMessage != "" {
			common.ApiErrorMsg(c, fmt.Sprintf("deployment failed: %s", item.ErrorMessage))
		} else {
			common.ApiErrorMsg(c, fmt.Sprintf("deployment is currently %s (%s); endpoint is not ready yet", item.Status, item.StatusMessage))
		}
		return
	}
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		prompt = "Hello! Reply briefly."
	}

	jobID := common.GetUUID()
	now := time.Now()
	job := &modelTestJobState{
		JobID:     jobID,
		Status:    "running",
		Progress:  0,
		Stage:     "Starting test...",
		UpdatedAt: now,
	}
	notebookTestJobsMu.Lock()
	for k, v := range notebookTestJobs {
		if now.Sub(v.UpdatedAt) > 30*time.Minute {
			delete(notebookTestJobs, k)
		}
	}
	notebookTestJobs[jobID] = job
	notebookTestJobsMu.Unlock()

	prevStatus := item.Status
	prevMsg := item.StatusMessage
	if prevStatus == "" || prevStatus == "testing" {
		prevStatus = "running"
	}
	if prevMsg == "" || prevMsg == "Testing model..." {
		prevMsg = "Running"
	}
	setDeploymentStatus(item.Id, "testing", "Testing model...")

	go func() {
		defer setDeploymentStatus(item.Id, prevStatus, prevMsg)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer cancel()
		start := time.Now()
		res, testErr := executeModelTestWithProgress(ctx, targetURL, item.ModelRepo, item.TaskType, prompt, "", func(pct int, stage string) {
			notebookTestJobsMu.Lock()
			if j, ok := notebookTestJobs[jobID]; ok {
				j.Progress = pct
				j.Stage = stage
				j.LatencyMs = time.Since(start).Milliseconds()
				j.UpdatedAt = time.Now()
			}
			notebookTestJobsMu.Unlock()
		})
		notebookTestJobsMu.Lock()
		if j, ok := notebookTestJobs[jobID]; ok {
			j.LatencyMs = time.Since(start).Milliseconds()
			j.UpdatedAt = time.Now()
			if testErr != nil {
				j.Status = "failed"
				j.Stage = "Failed"
				j.Error = testErr.Error()
			} else {
				j.Status = "completed"
				j.Progress = 100
				j.Stage = "Completed"
				j.Result = res
			}
		}
		notebookTestJobsMu.Unlock()
	}()

	common.ApiSuccess(c, job)
}

func GetNotebookDeploymentTestStatus(c *gin.Context) {
	jobID := strings.TrimSpace(c.Param("job_id"))
	notebookTestJobsMu.Lock()
	j, ok := notebookTestJobs[jobID]
	var snap modelTestJobState
	if ok && j != nil {
		snap = *j
	}
	notebookTestJobsMu.Unlock()
	if !ok {
		common.ApiErrorMsg(c, "test job not found")
		return
	}
	common.ApiSuccess(c, snap)
}

func formatEndpointHTTPError(statusCode int, body []byte) error {
	msg := strings.TrimSpace(string(body))
	lower := strings.ToLower(msg)
	if statusCode == 530 || strings.Contains(lower, "cloudflare tunnel error") || strings.Contains(lower, "error 1033") {
		return fmt.Errorf("model endpoint tunnel disconnected (HTTP %d): the remote runtime stopped, timed out, or ran out of memory", statusCode)
	}
	if strings.HasPrefix(lower, "<!doctype html") || strings.HasPrefix(lower, "<html") {
		return fmt.Errorf("model endpoint returned HTTP %d", statusCode)
	}
	if len(msg) > 400 {
		msg = msg[:400] + "..."
	}
	return fmt.Errorf("model endpoint returned HTTP %d: %s", statusCode, msg)
}

func executeModelTest(baseURL, modelRepo, rawTask, prompt, bearerToken string) (*modelTestResult, error) {
	return executeModelTestWithProgress(context.Background(), baseURL, modelRepo, rawTask, prompt, bearerToken, nil)
}

func executeModelTestWithProgress(ctx context.Context, baseURL, modelRepo, rawTask, prompt, bearerToken string, onProgress func(int, string)) (*modelTestResult, error) {
	task := notebookscript.NormalizeTaskType(rawTask)
	base := strings.TrimRight(baseURL, "/")
	start := time.Now()

	var endpointPath string
	var payload any

	switch task {
	case "video":
		endpointPath = "/v1/videos/generations"
		payload = map[string]any{"model": modelRepo, "prompt": prompt}
	case "image":
		endpointPath = "/v1/images/generations"
		payload = map[string]any{"model": modelRepo, "prompt": prompt}
	case "audio":
		endpointPath = "/v1/audio/speech"
		payload = map[string]any{"model": modelRepo, "input": prompt, "prompt": prompt}
	case "embeddings":
		endpointPath = "/v1/embeddings"
		payload = map[string]any{"model": modelRepo, "input": prompt}
	default:
		endpointPath = "/v1/chat/completions"
		payload = map[string]any{
			"model": modelRepo,
			"messages": []map[string]string{
				{"role": "user", "content": prompt},
			},
			"max_tokens": 128,
		}
	}

	rawBody, err := common.Marshal(payload)
	if err != nil {
		return nil, err
	}

	// Step A: Try async /_jobs endpoint first (prevents Cloudflare/proxy 524 timeouts and reports % progress).
	jobsUnsupported := false
	if !strings.HasSuffix(base, endpointPath) {
		jobStartClient := &http.Client{Timeout: 60 * time.Second}
		shortClient := &http.Client{Timeout: 15 * time.Second}
		var jobResp *http.Response
		var jobRespBytes []byte
		var startErr error
		for attempt := 0; attempt < 2; attempt++ {
			jobReq, jErr := http.NewRequestWithContext(ctx, http.MethodPost, base+"/_jobs", bytes.NewReader(rawBody))
			if jErr != nil {
				startErr = jErr
				break
			}
			jobReq.Header.Set("Content-Type", "application/json")
			if bearerToken != "" {
				jobReq.Header.Set("Authorization", "Bearer "+bearerToken)
			}
			jobResp, startErr = jobStartClient.Do(jobReq)
			if startErr == nil {
				jobRespBytes, startErr = io.ReadAll(io.LimitReader(jobResp.Body, 1<<20))
				_ = jobResp.Body.Close()
				if startErr == nil && jobResp.StatusCode != http.StatusBadGateway && jobResp.StatusCode != http.StatusServiceUnavailable && jobResp.StatusCode != http.StatusGatewayTimeout && jobResp.StatusCode != 524 {
					break
				}
				if jobResp.StatusCode == http.StatusBadGateway || jobResp.StatusCode == http.StatusServiceUnavailable || jobResp.StatusCode == http.StatusGatewayTimeout || jobResp.StatusCode == 524 {
					startErr = fmt.Errorf("model endpoint returned HTTP %d", jobResp.StatusCode)
				}
			}
			if attempt == 0 {
				time.Sleep(2 * time.Second)
			}
		}

		if startErr != nil {
			return nil, fmt.Errorf("failed to start async model job: %w", startErr)
		}
		if jobResp != nil && (jobResp.StatusCode == http.StatusNotFound || jobResp.StatusCode == http.StatusMethodNotAllowed) {
			jobsUnsupported = true
		} else if jobResp != nil && jobResp.StatusCode >= 200 && jobResp.StatusCode < 300 {
			var started struct {
				JobID    string `json:"job_id"`
				Progress int    `json:"progress"`
				Stage    string `json:"stage"`
			}
			if common.Unmarshal(jobRespBytes, &started) == nil && started.JobID != "" {
				if onProgress != nil {
					onProgress(started.Progress, started.Stage)
				}
				pollURL := base + "/_jobs/" + url.PathEscape(started.JobID)
				deadline := time.Now().Add(20 * time.Minute)
				consecutiveErrs := 0
				for time.Now().Before(deadline) {
					select {
					case <-ctx.Done():
						return nil, ctx.Err()
					case <-time.After(1500 * time.Millisecond):
					}
					pollReq, pErr := http.NewRequestWithContext(ctx, http.MethodGet, pollURL, nil)
					if pErr != nil {
						return nil, pErr
					}
					if bearerToken != "" {
						pollReq.Header.Set("Authorization", "Bearer "+bearerToken)
					}
					pResp, pDoErr := shortClient.Do(pollReq)
					if pDoErr != nil {
						consecutiveErrs++
						if consecutiveErrs >= 8 {
							return nil, fmt.Errorf("model test job polling failed: %w", pDoErr)
						}
						continue
					}
					pBytes, _ := io.ReadAll(io.LimitReader(pResp.Body, 8<<20))
					_ = pResp.Body.Close()
					if pResp.StatusCode == http.StatusBadGateway || pResp.StatusCode == http.StatusServiceUnavailable || pResp.StatusCode == http.StatusGatewayTimeout || pResp.StatusCode == 524 {
						consecutiveErrs++
						if consecutiveErrs >= 8 {
							return nil, fmt.Errorf("model endpoint returned HTTP %d during job poll", pResp.StatusCode)
						}
						continue
					}
					if pResp.StatusCode < 200 || pResp.StatusCode >= 300 {
						return nil, formatEndpointHTTPError(pResp.StatusCode, pBytes)
					}
					consecutiveErrs = 0
					var st struct {
						Status   string          `json:"status"`
						Progress int             `json:"progress"`
						Stage    string          `json:"stage"`
						Result   json.RawMessage `json:"result"`
						Error    string          `json:"error"`
					}
					if uErr := common.Unmarshal(pBytes, &st); uErr != nil {
						continue
					}
					if onProgress != nil {
						onProgress(st.Progress, st.Stage)
					}
					if st.Status == "completed" {
						return parseModelTestJSON(st.Result, task, time.Since(start).Milliseconds())
					}
					if st.Status == "failed" {
						if st.Error != "" {
							return nil, fmt.Errorf("model endpoint error: %s", st.Error)
						}
						return nil, fmt.Errorf("model job failed")
					}
				}
				return nil, fmt.Errorf("model test timed out after 20 minutes")
			}
			return nil, fmt.Errorf("model endpoint returned invalid /_jobs response")
		} else if jobResp != nil {
			return nil, formatEndpointHTTPError(jobResp.StatusCode, jobRespBytes)
		}
	} else {
		jobsUnsupported = true
	}

	if !jobsUnsupported {
		return nil, fmt.Errorf("async model test did not complete")
	}

	// Step B: Direct HTTP fallback (external providers or legacy servers without /_jobs).
	client := &http.Client{Timeout: 600 * time.Second}
	target := base
	if !strings.HasSuffix(base, endpointPath) {
		target = base + endpointPath
	}
	var resp *http.Response
	var respBytes []byte
	var reqErr error
	for attempt := 0; attempt < 3; attempt++ {
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(rawBody))
		if err != nil {
			return nil, err
		}
		httpReq.Header.Set("Content-Type", "application/json")
		if bearerToken != "" {
			httpReq.Header.Set("Authorization", "Bearer "+bearerToken)
		}

		resp, reqErr = client.Do(httpReq)
		if reqErr == nil {
			respBytes, reqErr = io.ReadAll(io.LimitReader(resp.Body, 8<<20))
			_ = resp.Body.Close()
			if reqErr == nil && resp.StatusCode != http.StatusBadGateway && resp.StatusCode != http.StatusServiceUnavailable && resp.StatusCode != http.StatusGatewayTimeout && resp.StatusCode != 524 {
				break
			}
			if resp.StatusCode == http.StatusBadGateway || resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == http.StatusGatewayTimeout || resp.StatusCode == 524 {
				reqErr = fmt.Errorf("model endpoint returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBytes)))
			}
		}
		if attempt < 2 {
			time.Sleep(2 * time.Second)
		}
	}
	if reqErr != nil && resp == nil {
		return nil, fmt.Errorf("model test request failed: %w", reqErr)
	}
	if reqErr != nil && len(respBytes) == 0 {
		return nil, fmt.Errorf("failed to read model test response: %w", reqErr)
	}
	latency := time.Since(start).Milliseconds()
	if resp.StatusCode == 524 {
		return nil, fmt.Errorf("model endpoint timed out (HTTP 524); recreate the deployment to enable async /_jobs progress")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, formatEndpointHTTPError(resp.StatusCode, respBytes)
	}

	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	if task == "audio" && (strings.HasPrefix(ct, "audio/") || strings.Contains(ct, "octet-stream")) {
		mime := strings.Split(ct, ";")[0]
		if mime == "" || mime == "application/octet-stream" {
			mime = "audio/mpeg"
		}
		return &modelTestResult{
			Type:      "audio",
			MediaURL:  "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(respBytes),
			LatencyMs: latency,
		}, nil
	}

	return parseModelTestJSON(respBytes, task, latency)
}

func parseModelTestJSON(respBytes []byte, task string, latency int64) (*modelTestResult, error) {

	var parsed map[string]any
	if err := common.Unmarshal(respBytes, &parsed); err != nil {
		return &modelTestResult{
			Type:      "text",
			Text:      strings.TrimSpace(string(respBytes)),
			LatencyMs: latency,
			Raw:       strings.TrimSpace(string(respBytes)),
		}, nil
	}

	res := &modelTestResult{
		Type:      task,
		LatencyMs: latency,
		Raw:       string(respBytes),
	}

	switch task {
	case "video", "image", "audio":
		res.MediaURL = extractMediaURL(parsed, task)
		if res.MediaURL == "" {
			res.Type = "text"
			res.Text = string(respBytes)
		}
	case "embeddings":
		res.Type = "text"
		if data, ok := parsed["data"].([]any); ok && len(data) > 0 {
			if first, ok := data[0].(map[string]any); ok {
				if emb, ok := first["embedding"].([]any); ok {
					res.Text = fmt.Sprintf("Embedding generated (%d dimensions)", len(emb))
					return res, nil
				}
			}
		}
		res.Text = string(respBytes)
	default:
		res.Type = "text"
		res.Text = extractTextFromResponse(parsed)
		if res.Text == "" {
			res.Text = string(respBytes)
		}
	}

	return res, nil
}

func extractMediaURL(parsed map[string]any, task string) string {
	for _, key := range []string{"url", "video_url", "image_url", "audio_url"} {
		if v, ok := parsed[key].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	if data, ok := parsed["data"].([]any); ok && len(data) > 0 {
		if item, ok := data[0].(map[string]any); ok {
			for _, key := range []string{"url", "video_url", "image_url", "audio_url"} {
				if v, ok := item[key].(string); ok && strings.TrimSpace(v) != "" {
					return strings.TrimSpace(v)
				}
			}
			if b64, ok := item["b64_json"].(string); ok && strings.TrimSpace(b64) != "" {
				mime := "image/png"
				if task == "video" {
					mime = "video/mp4"
				} else if task == "audio" {
					mime = "audio/wav"
				}
				return "data:" + mime + ";base64," + strings.TrimSpace(b64)
			}
		}
	}
	return ""
}

func extractTextFromResponse(parsed map[string]any) string {
	if choices, ok := parsed["choices"].([]any); ok && len(choices) > 0 {
		if first, ok := choices[0].(map[string]any); ok {
			if msg, ok := first["message"].(map[string]any); ok {
				if content, ok := msg["content"].(string); ok {
					return content
				}
			}
			if text, ok := first["text"].(string); ok {
				return text
			}
		}
	}
	if gen, ok := parsed["generated_text"].(string); ok {
		return gen
	}
	return ""
}
