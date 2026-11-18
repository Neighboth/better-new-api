package controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
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
	pageStr := c.Query("page")
	if pageStr == "" {
		pageStr = c.Query("p")
	}
	if pageStr != "" || c.Query("page_size") != "" {
		page, _ := strconv.Atoi(pageStr)
		if page < 1 {
			page = 1
		}
		pageSize, _ := strconv.Atoi(c.Query("page_size"))
		if pageSize <= 0 {
			pageSize = 20
		}
		keyword := strings.TrimSpace(c.Query("keyword"))
		status := strings.TrimSpace(c.Query("status"))
		result, err := model.ListProxyPoolProxiesPaged(page, pageSize, keyword, status)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		common.ApiSuccess(c, result)
		return
	}

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

type batchCreateProxyRequest struct {
	BaseName string                  `json:"base_name"`
	Enabled  *bool                   `json:"enabled"`
	Proxies  []proxyPoolProxyRequest `json:"proxies"`
}

func BatchCreateProxyPoolProxies(c *gin.Context) {
	var req batchCreateProxyRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "invalid request body")
		return
	}
	if len(req.Proxies) == 0 {
		common.ApiErrorMsg(c, "no proxies provided")
		return
	}

	baseEnabled := true
	if req.Enabled != nil {
		baseEnabled = *req.Enabled
	}
	baseName := strings.TrimSpace(req.BaseName)
	if baseName == "" {
		baseName = "Proxy"
	}

	toCreate := make([]model.ProxyPoolProxy, 0, len(req.Proxies))
	for idx, item := range req.Proxies {
		rawURL := strings.TrimSpace(item.URL)
		if rawURL == "" {
			continue
		}
		normalized, err := common.NormalizeProxyLine(rawURL)
		if err != nil {
			if _, parseErr := common.ParseProxyURLStrict(rawURL); parseErr != nil {
				continue
			}
			normalized = rawURL
		}
		enabled := baseEnabled
		if item.Enabled != nil {
			enabled = *item.Enabled
		}
		name := strings.TrimSpace(item.Name)
		if name == "" {
			if len(req.Proxies) == 1 {
				name = baseName
			} else {
				name = fmt.Sprintf("%s-%d", baseName, idx+1)
			}
		}
		toCreate = append(toCreate, model.ProxyPoolProxy{
			Name:      name,
			URL:       normalized,
			ProxyType: "standard",
			Enabled:   enabled,
		})
	}

	if len(toCreate) == 0 {
		common.ApiErrorMsg(c, "no valid proxies to add")
		return
	}

	count, err := model.BatchCreateProxyPoolProxies(toCreate)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	service.InvalidateProxyPoolCache()
	model.RecordProxyPoolEvent(0, -1, 0, "", "import", fmt.Sprintf("Imported %d proxy(ies) into the pool", count))
	common.ApiSuccess(c, gin.H{
		"count": count,
	})
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
	res, testErr := service.TestProxyEgressInfo(proxy.URL, proxy.ProxyType)
	proxy.LastTestedAt = time.Now().Unix()
	if testErr != nil {
		proxy.LastTestStatus = "fail"
		proxy.LastTestError = truncateError(testErr.Error())
		proxy.EgressIP = ""
		proxy.LatencyMs = 0
	} else {
		proxy.LastTestStatus = "ok"
		proxy.LastTestError = ""
		proxy.EgressIP = res.EgressIP
		proxy.LatencyMs = res.LatencyMs
		if res.CountryCode != "" {
			proxy.CountryCode = res.CountryCode
			proxy.CountryName = res.CountryName
			proxy.City = res.City
			proxy.ISP = res.ISP
		}
		if res.ProxyType != "" && proxy.ProxyType != res.ProxyType {
			proxy.ProxyType = res.ProxyType
		}
	}
	_ = model.UpdateProxyPoolProxy(proxy)
	service.InvalidateProxyPoolCache()
	if testErr != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "",
			"data": gin.H{
				"id":         id,
				"ok":         false,
				"latency_ms": proxy.LatencyMs,
				"error":      proxy.LastTestError,
			},
		})
		return
	}
	common.ApiSuccess(c, gin.H{
		"id":           id,
		"ok":           true,
		"egress_ip":    proxy.EgressIP,
		"latency_ms":   proxy.LatencyMs,
		"country_code": proxy.CountryCode,
		"country_name": proxy.CountryName,
		"city":         proxy.City,
		"isp":          proxy.ISP,
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

// proxyPoolManualAssignRequest carries an explicit hand-picked set of proxies
// to bind to the channel.
type proxyPoolManualAssignRequest struct {
	ProxyIds []int `json:"proxy_ids"`
}

// AssignProxyPoolManual replaces ALL assignments of one channel with an
// explicitly chosen set of proxies ("manual assign"). Ordering is honored:
// anonymous channels bind proxy N at key_index N; multi-key channels
// distribute the chosen proxies round-robin across every key of the channel.
func AssignProxyPoolManual(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid channel id")
		return
	}
	var req proxyPoolManualAssignRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "invalid request body")
		return
	}
	seen := map[int]struct{}{}
	proxyIds := make([]int, 0, len(req.ProxyIds))
	for _, proxyId := range req.ProxyIds {
		if proxyId <= 0 {
			continue
		}
		if _, dup := seen[proxyId]; dup {
			continue
		}
		seen[proxyId] = struct{}{}
		proxyIds = append(proxyIds, proxyId)
	}
	if len(proxyIds) == 0 {
		common.ApiErrorMsg(c, "choose at least one proxy")
		return
	}
	ch, err := model.GetChannelById(id, true)
	if err != nil {
		common.ApiErrorMsg(c, "channel not found")
		return
	}
	keyLines := splitKeyLines(ch.Key)
	isAnon := ch.Type == constant.ChannelTypeOpenCode || ch.Type == constant.ChannelTypeKiloCodeAnon || ch.Type == constant.ChannelTypeOHVCloudAnon
	if (!ch.ChannelInfo.IsMultiKey && !isAnon) || (len(keyLines) == 0 && !isAnon) {
		common.ApiErrorMsg(c, "channel is not a multi-key or pool channel")
		return
	}
	proxies, err := model.ListProxyPoolProxiesByIDs(proxyIds)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	found := make(map[int]model.ProxyPoolProxy, len(proxies))
	for _, proxy := range proxies {
		found[proxy.Id] = proxy
	}
	missing := make([]int, 0)
	ordered := make([]model.ProxyPoolProxy, 0, len(proxyIds))
	for _, proxyId := range proxyIds {
		proxy, ok := found[proxyId]
		if !ok {
			missing = append(missing, proxyId)
			continue
		}
		ordered = append(ordered, proxy)
	}
	if len(missing) > 0 {
		common.ApiErrorMsg(c, fmt.Sprintf("proxies not found: %v", missing))
		return
	}
	if len(ordered) == 0 {
		common.ApiErrorMsg(c, "none of the chosen proxies exist")
		return
	}
	mapping := make(map[int]int, len(ordered))
	if isAnon {
		for i, proxy := range ordered {
			mapping[i] = proxy.Id
		}
	} else {
		for keyIdx := 0; keyIdx < len(keyLines); keyIdx++ {
			mapping[keyIdx] = ordered[keyIdx%len(ordered)].Id
		}
	}
	if err := model.ReplaceProxyPoolAssignments(id, mapping); err != nil {
		common.ApiError(c, err)
		return
	}
	service.InvalidateProxyPoolCache()
	model.RecordProxyPoolEvent(id, -1, 0, "", "assign_manual", fmt.Sprintf("Manual assignment: bound %d proxy(ies) across %d key slot(s)", len(ordered), len(mapping)))
	common.ApiSuccess(c, gin.H{
		"channel_id": id,
		"key_count":  len(keyLines),
		"proxy_count": len(ordered),
		"assigned":   len(mapping),
	})
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
	isAnon := ch.Type == constant.ChannelTypeOpenCode || ch.Type == constant.ChannelTypeKiloCodeAnon || ch.Type == constant.ChannelTypeOHVCloudAnon
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
	model.RecordProxyPoolEvent(req.ChannelId, -1, 0, "", "assign", fmt.Sprintf("Assigned %d key index entries to %d enabled proxy(ies)", len(mapping), len(enabled)))
	common.ApiSuccess(c, gin.H{
		"channel_id":  req.ChannelId,
		"key_count":   len(keyLines),
		"proxy_count": len(enabled),
		"assigned":    len(mapping),
		"per_proxy":   ceilDiv(len(keyLines), len(enabled)),
	})
}

// GetProxyChannelDetail returns the proxy-pool view of one channel: its
// current assignments joined with the proxies' health state, live 429 cooldown
// remaining seconds, and the audit event log for the channel plus global pool
// events.
func GetProxyChannelDetail(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid channel id")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("assignments_page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("assignments_page_size", "20"))
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	ch, err := model.GetChannelById(id, true)
	if err != nil {
		common.ApiErrorMsg(c, "channel not found")
		return
	}
	assignments, err := model.ListProxyPoolChannelAssignmentsDetail(id, pageSize, offset)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	totalAssignments, err := model.CountProxyPoolChannelAssignments(id)
	if err != nil {
		totalAssignments = int64(len(assignments))
	}
	urls := make([]string, 0, len(assignments))
	for _, assignment := range assignments {
		urls = append(urls, assignment.ProxyURL)
	}
	cooldowns := service.ProxyCooldownSnapshot(urls)
	events, err := model.ListProxyPoolEvents(id, 200)
	if err != nil {
		events = []model.ProxyPoolEvent{}
	}
	globalEvents, err := model.ListProxyPoolEvents(0, 60)
	if err != nil {
		globalEvents = []model.ProxyPoolEvent{}
	}
	common.ApiSuccess(c, gin.H{
		"channel": gin.H{
			"id":             ch.Id,
			"name":           ch.Name,
			"type":           ch.Type,
			"status":         ch.Status,
			"group":          ch.Group,
			"multi_key_size": ch.ChannelInfo.MultiKeySize,
		},
		"assignments":       assignments,
		"assignments_total": totalAssignments,
		"assignments_page":  page,
		"assignments_page_size": pageSize,
		"cooldowns":         cooldowns,
		"events":            events,
		"global_events":     globalEvents,
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
	model.RecordProxyPoolEvent(0, -1, id, "", "unassign", fmt.Sprintf("Unassigned proxy #%d from all channels", id))
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
	// Clear every assignment before re-balancing: no stale mapping may survive.
	_ = model.DeleteAllProxyPoolAssignments()

	roundUsage := make(map[int]int, len(enabled))
	for _, summary := range channels {
		ch, err := model.GetChannelById(summary.Id, true)
		if err != nil {
			continue
		}
		isAnon := ch.Type == constant.ChannelTypeOpenCode || ch.Type == constant.ChannelTypeKiloCodeAnon || ch.Type == constant.ChannelTypeOHVCloudAnon
		if isAnon {
			mapping := make(map[int]int, len(enabled))
			for i, p := range enabled {
				mapping[i] = p.Id
			}
			if err := model.ReplaceProxyPoolAssignments(summary.Id, mapping); err == nil {
				rebalancedChannels++
				totalKeysAssigned += len(mapping)
				model.RecordProxyPoolEvent(summary.Id, -1, 0, "", "assign", fmt.Sprintf("Rebalanced: bound %d proxy(ies) to this anonymous channel", len(mapping)))
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
			model.RecordProxyPoolEvent(summary.Id, -1, 0, "", "assign", fmt.Sprintf("Rebalanced: bound %d key(s) to proxies", len(mapping)))
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
	model.RecordProxyPoolEvent(id, -1, 0, "", "clear", "Cleared all proxy assignments for this channel")
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
	Enabled    *bool  `json:"enabled"`
}

const CloudflareWorkerProxyScript = `export default {
  async fetch(request, env, ctx) {
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
  }
};`

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
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)

		// 1. metadata part
		metaHeader := make(textproto.MIMEHeader)
		metaHeader.Set("Content-Disposition", `form-data; name="metadata"`)
		metaHeader.Set("Content-Type", "application/json")
		metaPart, err := writer.CreatePart(metaHeader)
		if err != nil {
			common.ApiErrorMsg(c, "failed to create metadata part: "+err.Error())
			return
		}
		_, _ = metaPart.Write([]byte(`{"main_module":"worker.js","compatibility_date":"2024-01-01"}`))

		// 2. script part
		scriptHeader := make(textproto.MIMEHeader)
		scriptHeader.Set("Content-Disposition", `form-data; name="worker.js"; filename="worker.js"`)
		scriptHeader.Set("Content-Type", "application/javascript+module")
		scriptPart, err := writer.CreatePart(scriptHeader)
		if err != nil {
			common.ApiErrorMsg(c, "failed to create script part: "+err.Error())
			return
		}
		_, _ = scriptPart.Write([]byte(CloudflareWorkerProxyScript))

		_ = writer.Close()

		deployURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/workers/scripts/%s", accountId, workerName)
		httpReq, err := http.NewRequest(http.MethodPut, deployURL, &body)
		if err != nil {
			common.ApiErrorMsg(c, "failed to create deployment request: "+err.Error())
			return
		}
		httpReq.Header.Set("Authorization", "Bearer "+apiToken)
		httpReq.Header.Set("Content-Type", writer.FormDataContentType())

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

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	// Create the proxy record
	proxy := &model.ProxyPoolProxy{
		Name:      name,
		URL:       workerURL,
		ProxyType: "cloudflare_worker",
		Enabled:   enabled,
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
	if channelId == 0 {
		model.RecordProxyPoolEvent(0, -1, 0, "", "clear", "Cleared ALL proxy assignments across all channels")
	} else {
		model.RecordProxyPoolEvent(channelId, -1, 0, "", "clear", "Cleared all proxy assignments for this channel")
	}
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

// DeleteAllProxyPoolProxies removes the entire proxy pool in one operation:
// every assignment is cleared and every proxy row is deleted. The caller is
// expected to have confirmed with the user first — there is no undo.
func DeleteAllProxyPoolProxies(c *gin.Context) {
	var count int64
	if err := model.DB.Model(&model.ProxyPoolProxy{}).Count(&count).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.DeleteAllProxyPoolAssignments(); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.DB.Where("id > ?", 0).Delete(&model.ProxyPoolProxy{}).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	service.InvalidateProxyPoolCache()
	model.RecordProxyPoolEvent(0, -1, 0, "", "clear_all", fmt.Sprintf("Deleted all %d proxies and cleared all pool assignments", count))
	common.ApiSuccess(c, gin.H{
		"deleted": count,
	})
}
