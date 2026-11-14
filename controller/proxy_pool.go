package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
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
	if testErr != nil {
		proxy.LastTestStatus = "fail"
		proxy.LastTestError = truncateError(testErr.Error())
	} else {
		proxy.LastTestStatus = "ok"
		proxy.LastTestError = ""
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

// AutoAssignProxyPool distributes the enabled proxies across every key of a
// multi-key channel: keys are handed out round-robin so each proxy carries
// as equal a share as possible. With more keys than proxies, proxies split
// (e.g. one proxy serves three keys). Existing assignments for the channel
// are replaced atomically.
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
	var ch *model.Channel
	{
		var err error
		ch, err = model.GetChannelById(req.ChannelId, true)
		if err != nil {
			common.ApiErrorMsg(c, "channel not found")
			return
		}
	}
	keyLines := splitKeyLines(ch.Key)
	if !ch.ChannelInfo.IsMultiKey || len(keyLines) == 0 {
		common.ApiErrorMsg(c, "channel is not a multi-key channel")
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
	mapping := make(map[int]int, len(keyLines))
	for index := 0; index < len(keyLines); index++ {
		mapping[index] = enabled[index%len(enabled)].Id
	}
	if err := model.ReplaceProxyPoolAssignments(req.ChannelId, mapping); err != nil {
		common.ApiError(c, err)
		return
	}
	service.InvalidateProxyPoolCache()
	common.ApiSuccess(c, gin.H{
		"channel_id":   req.ChannelId,
		"key_count":    len(keyLines),
		"proxy_count":  len(enabled),
		"assigned":     len(mapping),
		"per_proxy":    ceilDiv(len(keyLines), len(enabled)),
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
