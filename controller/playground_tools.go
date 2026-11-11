package controller

import (
	"math"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

// GetPlaygroundConfig publishes the parts of the playground configuration the
// dashboard needs: the media models of the tools and of the studio, and the
// surfaces where the web search is available.
func GetPlaygroundConfig(c *gin.Context) {
	setting := operation_setting.GetPlaygroundSetting()
	search := operation_setting.GetOnlineSearchSetting()

	imageModels := setting.ImageModelOptions()
	videoModels := setting.VideoModelOptions()

	// The tools and older dashboards read the first model of the list; the
	// studio reads the whole list with the options of every model.
	imageModel := ""
	if len(imageModels) > 0 {
		imageModel = imageModels[0].Model
	}
	videoModel := ""
	if len(videoModels) > 0 {
		videoModel = videoModels[0].Model
	}

	sizes := []string{}
	qualities := []string{}
	defaultSize := ""
	defaultQuality := ""
	for _, model := range imageModels {
		sizes = append(sizes, model.Sizes...)
		qualities = append(qualities, model.Qualities...)
		if defaultSize == "" {
			defaultSize = model.DefaultSize
		}
		if defaultQuality == "" {
			defaultQuality = model.DefaultQuality
		}
	}
	sizes = uniqueStrings(sizes)
	qualities = uniqueStrings(qualities)
	if len(sizes) == 0 {
		sizes = []string{"1024x1024"}
	}
	if len(qualities) == 0 {
		qualities = []string{"standard"}
	}
	if defaultSize == "" {
		defaultSize = sizes[0]
	}
	if defaultQuality == "" {
		defaultQuality = qualities[0]
	}

	durations := []int{}
	ratios := []string{}
	videoQualities := []string{}
	defaultRatio := ""
	videoDefaultQuality := ""
	defaultDuration := 0
	for _, model := range videoModels {
		durations = append(durations, model.Durations...)
		ratios = append(ratios, model.Ratios...)
		videoQualities = append(videoQualities, model.Qualities...)
		if defaultRatio == "" {
			defaultRatio = model.DefaultRatio
		}
		if videoDefaultQuality == "" {
			videoDefaultQuality = model.DefaultQuality
		}
		if defaultDuration == 0 && len(model.Durations) > 0 {
			defaultDuration = model.Durations[0]
		}
	}
	ratios = uniqueStrings(ratios)
	videoQualities = uniqueStrings(videoQualities)
	if len(ratios) == 0 {
		ratios = []string{"16:9"}
	}
	if len(videoQualities) == 0 {
		videoQualities = []string{"standard"}
	}
	if defaultRatio == "" {
		defaultRatio = ratios[0]
	}
	if videoDefaultQuality == "" {
		videoDefaultQuality = videoQualities[0]
	}
	minDuration, maxDuration := setting.VideoDurationRange()
	if defaultDuration == 0 {
		defaultDuration = minDuration
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
			"model":           imageModel,
			"models":          imageModels,
			"sizes":           sizes,
			"default_size":    defaultSize,
			"qualities":       qualities,
			"default_quality": defaultQuality,
			"default_count":   setting.ImageDefaultCount,
		},
		"video": gin.H{
			"enabled":          setting.VideoEnabled,
			"model":            videoModel,
			"models":           videoModels,
			"durations":        durations,
			"default_duration": defaultDuration,
			"min_duration":     minDuration,
			"max_duration":     maxDuration,
			"ratios":           ratios,
			"default_ratio":    defaultRatio,
			"qualities":        videoQualities,
			"default_quality":  videoDefaultQuality,
			"reference_image":  setting.VideoReferenceImage,
		},
		"studio": gin.H{
			"enabled": setting.StudioEnabled,
		},
	})
}

// uniqueStrings removes duplicates while keeping the configuration order.
func uniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
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
