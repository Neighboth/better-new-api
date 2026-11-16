package controller

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

func ListProxyPoolProxies(c *gin.Context) {
	proxies, err := model.ListProxyPoolProxies()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if proxies == nil {
		proxies = []model.ProxyPoolProxy{}
	}
	common.ApiSuccess(c, proxies)
}

type proxyPoolProxyRequest struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	Enabled *bool  `json:"enabled"`
}

func validateProxyPoolURL(url string) error {
	trimmed := strings.TrimSpace(url)
	if trimmed == "" {
		return fmt.Errorf("proxy URL must not be empty")
	}
	if _, err := common.ParseProxyURLStrict(trimmed); err != nil {
		return err
	}
	return nil
}

func CreateProxyPoolProxy(c *gin.Context) {
	var req proxyPoolProxyRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "invalid request body")
		return
	}
	req.URL = strings.TrimSpace(req.URL)
	if err := validateProxyPoolURL(req.URL); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	proxy := &model.ProxyPoolProxy{
		Name:    strings.TrimSpace(req.Name),
		URL:     req.URL,
		Enabled: enabled,
	}
	if err := model.CreateProxyPoolProxy(proxy); err != nil {
		common.ApiError(c, err)
		return
	}
	service.InvalidateProxyPoolCache()
	common.ApiSuccess(c, proxy)
}

func UpdateProxyPoolProxy(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid proxy id")
		return
	}
	var req proxyPoolProxyRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "invalid request body")
		return
	}
	proxy, err := model.GetProxyPoolProxy(id)
	if err != nil {
		common.ApiErrorMsg(c, "proxy not found")
		return
	}
	if url := strings.TrimSpace(req.URL); url != "" {
		if err := validateProxyPoolURL(url); err != nil {
			common.ApiErrorMsg(c, err.Error())
			return
		}
		proxy.URL = url
	}
	if req.Name != "" {
		proxy.Name = strings.TrimSpace(req.Name)
	}
	if req.Enabled != nil {
		proxy.Enabled = *req.Enabled
	}
	if err := model.UpdateProxyPoolProxy(proxy); err != nil {
		common.ApiError(c, err)
		return
	}
	service.InvalidateProxyPoolCache()
	common.ApiSuccess(c, proxy)
}

func DeleteProxyPoolProxy(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid proxy id")
		return
	}
	if err := model.DeleteProxyPoolProxy(id); err != nil {
		common.ApiError(c, err)
		return
	}
	service.InvalidateProxyPoolCache()
	common.ApiSuccess(c, gin.H{})
}

func TestProxyPoolProxy(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid proxy id")
		return
	}
	proxy, err := model.GetProxyPoolProxy(id)
	if err != nil {
		common.ApiErrorMsg(c, "proxy not found")
		return
	}
	egressIP, latency, testErr := service.TestProxyEgress(proxy.URL)
	proxy.LastTestedAt = time.Now().Unix()
	proxy.LatencyMs = latency
	if testErr != nil {
		proxy.LastTestStatus = "fail"
		proxy.LastTestError = truncateError(testErr.Error())
		proxy.EgressIP = ""
	} else {
		proxy.LastTestStatus = "ok"
		proxy.LastTestError = ""
		proxy.EgressIP = egressIP
	}
	_ = model.UpdateProxyPoolProxy(proxy)
	service.InvalidateProxyPoolCache()
	if testErr != nil {
		c.JSON(http.StatusOK, gin.H{
			"success":   true,
			"message":   "",
			"data": gin.H{
				"id":         id,
				"ok":         false,
				"latency_ms": latency,
				"error":      proxy.LastTestError,
			},
		})
		return
	}
	common.ApiSuccess(c, gin.H{
		"id":         id,
		"ok":         true,
		"egress_ip":  egressIP,
		"latency_ms": latency,
	})
}

func truncateError(text string) string {
	if len(text) <= 480 {
		return text
	}
	return text[:480] + "…"
}

// ListProxyPoolChannels exposes multi-key channels together with their key
// count and current assignment state so the admin UI can bind proxies per key.
func ListProxyPoolChannels(c *gin.Context) {
	channels, err := model.ListMultiKeyChannels()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	assignments, err := model.ListProxyPoolAssignments(0)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	assignedByChannel := make(map[int]int, len(assignments))
	for _, assignment := range assignments {
		assignedByChannel[assignment.ChannelId]++
	}
	type channelSummary struct {
		model.MultiKeyChannelSummary
		AssignedKeys int `json:"assigned_keys"`
	}
	summaries := make([]channelSummary, 0, len(channels))
	for _, channel := range channels {
		summaries = append(summaries, channelSummary{
			MultiKeyChannelSummary: channel,
			AssignedKeys:           assignedByChannel[channel.Id],
		})
	}
	common.ApiSuccess(c, summaries)
}

func ListProxyPoolAssignments(c *gin.Context) {
	channelId := 0
	if raw := strings.TrimSpace(c.Query("channel_id")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			common.ApiErrorMsg(c, "invalid channel id")
			return
		}
		channelId = parsed
	}
	assignments, err := model.ListProxyPoolAssignments(channelId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if assignments == nil {
		assignments = []model.ProxyPoolAssignment{}
	}
	common.ApiSuccess(c, assignments)
}

type proxyPoolAssignRequest struct {
	ChannelId int `json:"channel_id"`
}

func fairAssignKeysToProxies(channelId int, keyCount int, enabled []model.ProxyPoolProxy) map[int]int {
	mapping := make(map[int]int, keyCount)
	if len(enabled) == 0 || keyCount == 0 {
		return mapping
	}

	// Count existing assignments per proxy across all channels excluding this channel
	usage := make(map[int]int, len(enabled))
	allAssignments, _ := model.ListProxyPoolAssignments(0)
	for _, a := range allAssignments {
		if a.ChannelId != channelId {
			usage[a.ProxyId]++
		}
	}

	roundUsage := make(map[int]int, len(enabled))
	for _, p := range enabled {
		roundUsage[p.Id] = usage[p.Id]
	}

	for k := 0; k < keyCount; k++ {
		bestIdx := 0
		bestVal := roundUsage[enabled[0].Id]
		for i := 1; i < len(enabled); i++ {
			val := roundUsage[enabled[i].Id]
			if val < bestVal {
				bestVal = val
				bestIdx = i
			}
		}
		chosenId := enabled[bestIdx].Id
		mapping[k] = chosenId
		roundUsage[chosenId]++
	}

	return mapping
}

// AutoAssignProxyPool distributes the enabled proxies across every key of a
// multi-key channel: keys are handed out with balanced, least-loaded distribution
// so each proxy carries as equal a share as possible across the entire system.
func AutoAssignProxyPool(c *gin.Context) {
	var req proxyPoolAssignRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "invalid request body")
		return
	}
	if req.ChannelId <= 0 {
		common.ApiErrorMsg(c, "invalid channel id")
		return
	}
	ch, err := model.GetChannelById(req.ChannelId, true)
	if err != nil {
		common.ApiErrorMsg(c, "channel not found")
		return
	}
	keyLines := splitKeyLines(ch.Key)
	isAnon := ch.Type == constant.ChannelTypeOpenCode || ch.Type == constant.ChannelTypeKiloCodeAnon
	if (!ch.ChannelInfo.IsMultiKey && !isAnon) || (len(keyLines) == 0 && !isAnon) {
		common.ApiErrorMsg(c, "channel is not a multi-key or pool channel")
		return
	}
	proxies, err := model.ListProxyPoolProxies()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	enabled := make([]model.ProxyPoolProxy, 0, len(proxies))
	for _, proxy := range proxies {
		if proxy.Enabled {
			enabled = append(enabled, proxy)
		}
	}
	if len(enabled) == 0 {
		common.ApiErrorMsg(c, "no enabled proxy in the pool")
		return
	}
	var mapping map[int]int
	if isAnon {
		// Anonymous channels can bind unlimited proxies across the pool
		mapping = make(map[int]int, len(enabled))
		for i, p := range enabled {
			mapping[i] = p.Id
		}
	} else {
		mapping = fairAssignKeysToProxies(req.ChannelId, len(keyLines), enabled)
	}
	if err := model.ReplaceProxyPoolAssignments(req.ChannelId, mapping); err != nil {
		common.ApiError(c, err)
		return
	}
	service.InvalidateProxyPoolCache()
	common.ApiSuccess(c, gin.H{
		"channel_id":  req.ChannelId,
		"key_count":   len(keyLines),
		"proxy_count": len(enabled),
		"assigned":    len(mapping),
		"per_proxy":   ceilDiv(len(keyLines), len(enabled)),
	})
}

type ProxyAssignmentDetail struct {
	ChannelId   int    `json:"channel_id"`
	ChannelName string `json:"channel_name"`
	ChannelType int    `json:"channel_type"`
	KeyIndex    int    `json:"key_index"`
}

func GetProxyPoolProxyDetail(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid proxy id")
		return
	}
	proxy, err := model.GetProxyPoolProxy(id)
	if err != nil {
		common.ApiErrorMsg(c, "proxy not found")
		return
	}

	var assignments []model.ProxyPoolAssignment
	_ = model.DB.Where("proxy_id = ?", id).Order("channel_id ASC, key_index ASC").Find(&assignments).Error

	details := make([]ProxyAssignmentDetail, 0, len(assignments))
	if len(assignments) > 0 {
		channelIds := make([]int, 0, len(assignments))
		for _, a := range assignments {
			channelIds = append(channelIds, a.ChannelId)
		}
		var channels []model.Channel
		_ = model.DB.Where("id IN ?", channelIds).Find(&channels).Error
		chMap := make(map[int]model.Channel, len(channels))
		for _, ch := range channels {
			chMap[ch.Id] = ch
		}
		for _, a := range assignments {
			ch := chMap[a.ChannelId]
			details = append(details, ProxyAssignmentDetail{
				ChannelId:   a.ChannelId,
				ChannelName: ch.Name,
				ChannelType: ch.Type,
				KeyIndex:    a.KeyIndex,
			})
		}
	}

	usageHistory := model.GetProxyPoolDailyUsageHistory(id, 30)

	common.ApiSuccess(c, gin.H{
		"proxy":         proxy,
		"assigned_keys": len(assignments),
		"assignments":   details,
		"usage_history": usageHistory,
	})
}

func UnassignProxyPoolProxy(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid proxy id")
		return
	}
	if err := model.UnassignProxyPoolProxy(id); err != nil {
		common.ApiError(c, err)
		return
	}
	service.InvalidateProxyPoolCache()
	common.ApiSuccess(c, gin.H{
		"proxy_id": id,
	})
}

func RebalanceAllProxyPoolChannels(c *gin.Context) {
	channels, err := model.ListMultiKeyChannels()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	proxies, err := model.ListProxyPoolProxies()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	enabled := make([]model.ProxyPoolProxy, 0, len(proxies))
	for _, proxy := range proxies {
		if proxy.Enabled {
			enabled = append(enabled, proxy)
		}
	}
	if len(enabled) == 0 {
		common.ApiErrorMsg(c, "no enabled proxy in the pool")
		return
	}

	rebalancedChannels := 0
	totalKeysAssigned := 0

	// Clear all existing assignments first so fair allocation runs globally
	_ = model.DeleteProxyPoolAssignments(0)

	roundUsage := make(map[int]int, len(enabled))
	for _, summary := range channels {
		ch, err := model.GetChannelById(summary.Id, true)
		if err != nil {
			continue
		}
		isAnon := ch.Type == constant.ChannelTypeOpenCode || ch.Type == constant.ChannelTypeKiloCodeAnon
		if isAnon {
			mapping := make(map[int]int, len(enabled))
			for i, p := range enabled {
				mapping[i] = p.Id
			}
			if err := model.ReplaceProxyPoolAssignments(summary.Id, mapping); err == nil {
				rebalancedChannels++
				totalKeysAssigned += len(mapping)
			}
			continue
		}
		keyLines := splitKeyLines(ch.Key)
		if len(keyLines) == 0 {
			continue
		}
		mapping := make(map[int]int, len(keyLines))
		for k := 0; k < len(keyLines); k++ {
			bestIdx := 0
			bestVal := roundUsage[enabled[0].Id]
			for i := 1; i < len(enabled); i++ {
				val := roundUsage[enabled[i].Id]
				if val < bestVal {
					bestVal = val
					bestIdx = i
				}
			}
			chosenId := enabled[bestIdx].Id
			mapping[k] = chosenId
			roundUsage[chosenId]++
		}
		if err := model.ReplaceProxyPoolAssignments(summary.Id, mapping); err == nil {
			rebalancedChannels++
			totalKeysAssigned += len(mapping)
		}
	}

	service.InvalidateProxyPoolCache()
	common.ApiSuccess(c, gin.H{
		"rebalanced_channels": rebalancedChannels,
		"total_keys_assigned": totalKeysAssigned,
		"proxy_count":         len(enabled),
	})
}

func ClearChannelProxyPoolAssignments(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid channel id")
		return
	}
	if err := model.DeleteProxyPoolAssignments(id); err != nil {
		common.ApiError(c, err)
		return
	}
	service.InvalidateProxyPoolCache()
	common.ApiSuccess(c, gin.H{
		"channel_id": id,
	})
}

type cloudflareWorkerDeployRequest struct {
	ApiToken   string `json:"api_token"`
	AccountId  string `json:"account_id"`
	WorkerName string `json:"worker_name"`
	WorkerURL  string `json:"worker_url"`
	Name       string `json:"name"`
}

const CloudflareWorkerProxyScript = `addEventListener('fetch', event => {
  event.respondWith(handleRequest(event.request));
});

async function handleRequest(request) {
  const url = new URL(request.url);
  let target = request.headers.get("X-Target-URL") || url.searchParams.get("url");
  if (!target && url.pathname.length > 1) {
    const raw = url.pathname.slice(1) + url.search;
    if (raw.startsWith("http://") || raw.startsWith("https://")) {
      target = raw;
    }
  }

  if (!target) {
    return new Response(JSON.stringify({
      status: "ok",
      service: "Cloudflare Worker Proxy for Better-New-API",
      ip: request.headers.get("cf-connecting-ip")
    }), {
      headers: { "content-type": "application/json" }
    });
  }

  const modifiedHeaders = new Headers(request.headers);
  modifiedHeaders.delete("x-target-url");
  modifiedHeaders.delete("host");

  try {
    const response = await fetch(target, {
      method: request.method,
      headers: modifiedHeaders,
      body: request.method !== "GET" && request.method !== "HEAD" ? request.body : undefined,
      redirect: "follow",
    });
    return response;
  } catch (err) {
    return new Response(JSON.stringify({ error: err.message }), {
      status: 502,
      headers: { "content-type": "application/json" }
    });
  }
}`

func GetCloudflareWorkerScript(c *gin.Context) {
	common.ApiSuccess(c, gin.H{
		"script": CloudflareWorkerProxyScript,
	})
}

func DeployCloudflareWorkerProxy(c *gin.Context) {
	var req cloudflareWorkerDeployRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "invalid request body")
		return
	}

	workerURL := strings.TrimSpace(req.WorkerURL)
	apiToken := strings.TrimSpace(req.ApiToken)
	accountId := strings.TrimSpace(req.AccountId)
	workerName := strings.TrimSpace(req.WorkerName)
	if workerName == "" {
		workerName = "better-new-api-proxy"
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "Cloudflare Worker Proxy"
	}

	// If user provided CF API token & Account ID, deploy worker via Cloudflare API
	if apiToken != "" && accountId != "" {
		deployURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/workers/scripts/%s", accountId, workerName)
		httpReq, err := http.NewRequest(http.MethodPut, deployURL, strings.NewReader(CloudflareWorkerProxyScript))
		if err != nil {
			common.ApiErrorMsg(c, "failed to create deployment request: "+err.Error())
			return
		}
		httpReq.Header.Set("Authorization", "Bearer "+apiToken)
		httpReq.Header.Set("Content-Type", "application/javascript")

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(httpReq)
		if err != nil {
			common.ApiErrorMsg(c, "Cloudflare API request failed: "+err.Error())
			return
		}
		defer resp.Body.Close()
		respBytes, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			common.ApiErrorMsg(c, fmt.Sprintf("Cloudflare deployment failed (status %d): %s", resp.StatusCode, string(respBytes)))
			return
		}

		// Enable workers.dev subdomain for this script
		subdomainURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/workers/scripts/%s/subdomain", accountId, workerName)
		subdomainReq, _ := http.NewRequest(http.MethodPost, subdomainURL, strings.NewReader(`{"enabled":true}`))
		if subdomainReq != nil {
			subdomainReq.Header.Set("Authorization", "Bearer "+apiToken)
			subdomainReq.Header.Set("Content-Type", "application/json")
			subResp, subErr := client.Do(subdomainReq)
			if subErr == nil {
				subResp.Body.Close()
			}
		}

		// Determine subdomain URL if workerURL was not provided
		if workerURL == "" {
			subAccURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/workers/subdomain", accountId)
			subAccReq, _ := http.NewRequest(http.MethodGet, subAccURL, nil)
			if subAccReq != nil {
				subAccReq.Header.Set("Authorization", "Bearer "+apiToken)
				if subResp, err := client.Do(subAccReq); err == nil {
					defer subResp.Body.Close()
					var subData struct {
						Result struct {
							Subdomain string `json:"subdomain"`
						} `json:"result"`
					}
					if b, err := io.ReadAll(subResp.Body); err == nil {
						_ = json.Unmarshal(b, &subData)
						if subData.Result.Subdomain != "" {
							workerURL = fmt.Sprintf("https://%s.%s.workers.dev", workerName, subData.Result.Subdomain)
						}
					}
				}
			}
		}
	}

	if workerURL == "" {
		common.ApiErrorMsg(c, "worker_url is required when API token and Account ID are not provided")
		return
	}

	if !strings.HasPrefix(workerURL, "http://") && !strings.HasPrefix(workerURL, "https://") {
		workerURL = "https://" + workerURL
	}
	workerURL = strings.TrimRight(workerURL, "/")

	// Create the proxy record
	proxy := &model.ProxyPoolProxy{
		Name:      name,
		URL:       workerURL,
		ProxyType: "cloudflare_worker",
		Enabled:   true,
	}
	if err := model.CreateProxyPoolProxy(proxy); err != nil {
		common.ApiError(c, err)
		return
	}
	service.InvalidateProxyPoolCache()
	common.ApiSuccess(c, proxy)
}

func GetProxyPoolUsageStats(c *gin.Context) {
	overallHistory := model.GetProxyPoolDailyUsageHistory(0, 30)
	proxies, _ := model.ListProxyPoolProxies()
	if proxies == nil {
		proxies = []model.ProxyPoolProxy{}
	}
	common.ApiSuccess(c, gin.H{
		"daily_history": overallHistory,
		"proxies":       proxies,
	})
}

func ClearProxyPoolAssignments(c *gin.Context) {
	channelId := 0
	if raw := strings.TrimSpace(c.Query("channel_id")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			common.ApiErrorMsg(c, "invalid channel id")
			return
		}
		channelId = parsed
	}
	if err := model.DeleteProxyPoolAssignments(channelId); err != nil {
		common.ApiError(c, err)
		return
	}
	service.InvalidateProxyPoolCache()
	common.ApiSuccess(c, gin.H{})
}

func splitKeyLines(raw string) []string {
	lines := strings.Split(raw, "\n")
	keys := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		keys = append(keys, trimmed)
	}
	return keys
}

func ceilDiv(a, b int) int {
	if b <= 0 {
		return 0
	}
	if a%b == 0 {
		return a / b
	}
	return a/b + 1
}
