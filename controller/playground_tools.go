package controller

import (
	"math"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

// splitSettingList turns a comma separated setting into a trimmed list.
func splitSettingList(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func splitSettingInts(raw string) []int {
	parts := splitSettingList(raw)
	out := make([]int, 0, len(parts))
	for _, part := range parts {
		if value, err := strconv.Atoi(part); err == nil && value > 0 {
			out = append(out, value)
		}
	}
	return out
}

// GetPlaygroundConfig publishes the parts of the playground configuration the
// dashboard needs: the media models of the tools and of the studio, and the
// surfaces where the web search is available.
func GetPlaygroundConfig(c *gin.Context) {
	setting := operation_setting.GetPlaygroundSetting()
	search := operation_setting.GetOnlineSearchSetting()

	sizes := splitSettingList(setting.ImageSizes)
	if len(sizes) == 0 {
		sizes = []string{"1024x1024"}
	}
	qualities := splitSettingList(setting.ImageQualities)
	if len(qualities) == 0 {
		qualities = []string{"standard"}
	}
	durations := splitSettingInts(setting.VideoDurations)
	if len(durations) == 0 {
		durations = []int{5}
	}
	ratios := splitSettingList(setting.VideoRatios)
	if len(ratios) == 0 {
		ratios = []string{"16:9"}
	}
	videoQualities := splitSettingList(setting.VideoQualities)
	if len(videoQualities) == 0 {
		videoQualities = []string{"standard"}
	}

	common.ApiSuccess(c, gin.H{
		"web_search": gin.H{
			"playground":       setting.WebSearchPlaygroundEnabled,
			"discord":          setting.WebSearchDiscordEnabled,
			"live":             setting.WebSearchLiveEnabled,
			"configured":       strings.TrimSpace(search.SearXNGURL) != "",
			"price_per_search": search.PricePerSearch,
			"max_results":      search.MaxSearches,
		},
		"image": gin.H{
			"enabled":         setting.ImageEnabled,
			"model":           setting.ImageModel,
			"sizes":           sizes,
			"default_size":    setting.ImageDefaultSize,
			"qualities":       qualities,
			"default_quality": setting.ImageDefaultQuality,
			"default_count":   setting.ImageDefaultCount,
		},
		"video": gin.H{
			"enabled":          setting.VideoEnabled,
			"model":            setting.VideoModel,
			"durations":        durations,
			"default_duration": setting.VideoDefaultDuration,
			"min_duration":     setting.VideoMinDuration,
			"max_duration":     setting.VideoMaxDuration,
			"ratios":           ratios,
			"default_ratio":    setting.VideoDefaultRatio,
			"qualities":        videoQualities,
			"default_quality":  setting.VideoDefaultQuality,
			"reference_image":  setting.VideoReferenceImage,
		},
		"studio": gin.H{
			"enabled": setting.StudioEnabled,
		},
	})
}

// PlaygroundWebSearch runs one SearXNG query for the web search tool of the
// playground and bills it like a search that a relayed request would use.
func PlaygroundWebSearch(c *gin.Context) {
	setting := operation_setting.GetPlaygroundSetting()
	search := operation_setting.GetOnlineSearchSetting()

	if !setting.WebSearchPlaygroundEnabled {
		c.JSON(200, gin.H{
			"success": false,
			"message": "Web search is disabled in the playground",
			"code":    "web_search_disabled",
		})
		return
	}
	if strings.TrimSpace(search.SearXNGURL) == "" {
		c.JSON(200, gin.H{
			"success": false,
			"message": "SearXNG is not configured",
			"code":    "web_search_not_configured",
		})
		return
	}

	var request struct {
		Query      string `json:"query"`
		MaxResults int    `json:"max_results"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiErrorMsg(c, "Invalid request body")
		return
	}
	query := strings.TrimSpace(request.Query)
	if query == "" {
		common.ApiErrorMsg(c, "Missing search query")
		return
	}
	if len(query) > 500 {
		query = query[:500]
	}

	maxResults := request.MaxResults
	if maxResults <= 0 {
		maxResults = search.MaxSearches
	}
	if maxResults <= 0 {
		maxResults = 5
	}
	if maxResults > 10 {
		maxResults = 10
	}

	hits, err := service.SearchOnline(query, maxResults)
	if err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": err.Error(),
			"code":    "web_search_failed",
		})
		return
	}

	userId := c.GetInt("id")
	group := c.GetString("group")
	if group == "" {
		group = "default"
	}

	charged := 0
	if search.PricePerSearch > 0 && !math.IsNaN(search.PricePerSearch) && !math.IsInf(search.PricePerSearch, 0) {
		quotaFloat := search.PricePerSearch * common.QuotaPerUnit * ratio_setting.GetGroupRatio(group)
		charged = int(math.Round(quotaFloat))
		if charged > 0 {
			if err := model.DecreaseUserQuota(userId, charged, false); err != nil {
				c.JSON(200, gin.H{
					"success": false,
					"message": "Failed to charge the search",
					"code":    "web_search_billing_failed",
				})
				return
			}
			model.RecordConsumeLog(c, userId, model.RecordConsumeLogParams{
				ModelName: "web_search",
				Quota:     charged,
				Content:   query,
				TokenName: "playground-web-search",
				Group:     group,
			})
		}
	}

	results := make([]gin.H, 0, len(hits))
	for _, hit := range hits {
		results = append(results, gin.H{
			"title":   hit.Title,
			"url":     hit.URL,
			"content": hit.Content,
		})
	}

	common.ApiSuccess(c, gin.H{
		"query":         query,
		"count":         len(results),
		"results":       results,
		"charged_quota": charged,
	})
}
