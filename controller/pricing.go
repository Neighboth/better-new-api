package controller

import (
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

func filterPricingByUsableGroups(pricing []model.Pricing, usableGroup map[string]string) []model.Pricing {
	if len(pricing) == 0 {
		return pricing
	}
	if len(usableGroup) == 0 {
		return []model.Pricing{}
	}

	filtered := make([]model.Pricing, 0, len(pricing))
	for _, item := range pricing {
		if common.StringsContains(item.EnableGroup, "all") {
			filtered = append(filtered, item)
			continue
		}
		for _, group := range item.EnableGroup {
			if _, ok := usableGroup[group]; ok {
				filtered = append(filtered, item)
				break
			}
		}
	}
	return filtered
}

// pricingUptime is the measured uptime of every model of the square.
//
// The uptime of a card is the uptime of the requests that the gateway served,
// it is not the uptime of a channel probe and it is never a value of the
// panel: a model that served no request in the window carries no entry and the
// card shows a dash. The answer is cached for the length of the pricing cache,
// the square asks for it on every page load.
type pricingUptimeEntry struct {
	Rate    float64
	History []float64
}

var pricingUptimeCache = struct {
	sync.Mutex
	fetchedAt time.Time
	models    map[string]pricingUptimeEntry
}{}

const pricingUptimeTTL = 30 * time.Second

func loadPricingUptime() map[string]pricingUptimeEntry {
	pricingUptimeCache.Lock()
	defer pricingUptimeCache.Unlock()
	if len(pricingUptimeCache.models) > 0 && time.Since(pricingUptimeCache.fetchedAt) < pricingUptimeTTL {
		return pricingUptimeCache.models
	}

	models := map[string]pricingUptimeEntry{}
	// The window of the card is the last day, the same window the card draws.
	summary, err := perfmetrics.QuerySummaryAll(24, nil)
	if err == nil {
		for _, item := range summary.Models {
			history := make([]float64, 0, len(item.UptimeBuckets))
			for _, bucket := range item.UptimeBuckets {
				if bucket.RequestCount == 0 {
					// An hour without requests is not an hour of perfect
					// service: it stays out of the history of the card.
					continue
				}
				history = append(history, bucket.SuccessRate)
			}
			models[item.ModelName] = pricingUptimeEntry{Rate: item.SuccessRate, History: history}
		}
	}

	pricingUptimeCache.models = models
	pricingUptimeCache.fetchedAt = time.Now()
	return models
}

// applyPricingUptime writes the measured uptime into the pricing payload.
//
// Every channel of a model serves the requests of the model, the measured
// uptime is therefore the uptime of the model and it is written on every
// channel of it. The number of the channel table and the number of the card are
// the same number.
func applyPricingUptime(pricing []model.Pricing) []model.Pricing {
	uptimes := loadPricingUptime()
	if len(uptimes) == 0 {
		return pricing
	}
	// The channels of the cached pricing are shared by every request of the
	// square: the payload of a request is written on a copy of them.
	enriched := make([]model.Pricing, len(pricing))
	for index, item := range pricing {
		if len(item.Channels) > 0 {
			channels := make([]model.PricingChannel, len(item.Channels))
			copy(channels, item.Channels)
			item.Channels = channels
			if entry, ok := uptimes[item.ModelName]; ok && entry.Rate > 0 {
				for channelIndex := range channels {
					channels[channelIndex].Uptime = entry.Rate
					channels[channelIndex].UptimeHistory = entry.History
				}
			}
		}
		enriched[index] = item
	}
	return enriched
}

func GetPricing(c *gin.Context) {
	pricing := model.GetPricing()
	userId, exists := c.Get("id")
	usableGroup := map[string]string{}
	groupRatio := map[string]float64{}
	for s, f := range ratio_setting.GetGroupRatioCopy() {
		groupRatio[s] = f
	}
	var group string
	if exists {
		user, err := model.GetUserCache(userId.(int))
		if err == nil {
			group = user.Group
			for g := range groupRatio {
				ratio, ok := ratio_setting.GetGroupGroupRatio(group, g)
				if ok {
					groupRatio[g] = ratio
				}
			}
		}
	}

	usableGroup = service.GetUserUsableGroups(group)
	pricing = filterPricingByUsableGroups(pricing, usableGroup)
	// Append enabled BYOG Community Edge Compute models so they are visible on Model Square
	byogModels, err := model.GetAllByogModels(true)
	if err == nil && len(byogModels) > 0 {
		byogSetting, _ := model.GetByogSetting()
		for _, bm := range byogModels {
			var pricePerMillion float64 = 0.40
			if byogSetting != nil {
				switch bm.PricingTier {
				case 1:
					pricePerMillion = byogSetting.Tier1CustomerPrice
				case 2:
					pricePerMillion = byogSetting.Tier2CustomerPrice
				case 3:
					pricePerMillion = byogSetting.Tier3CustomerPrice
				default:
					pricePerMillion = 0.20
				}
			}
			ratio := pricePerMillion / 2.0
			if ratio <= 0 {
				ratio = 0.1
			}

			epTypes := []constant.EndpointType{constant.EndpointTypeOpenAI}
			if bm.ModelType == "embedding" {
				epTypes = []constant.EndpointType{constant.EndpointTypeEmbeddings}
			}

			pricing = append(pricing, model.Pricing{
				ModelName:              bm.ModelId,
				Description:            fmt.Sprintf("%s (%s BYOG Edge Compute)", bm.ModelId, bm.ParamSize),
				Icon:                   "byog",
				Tags:                   "byog,community,edge",
				QuotaType:              0,
				ModelRatio:             ratio,
				ModelPrice:             pricePerMillion,
				OwnerBy:                "BYOG Network",
				CompletionRatio:        1.0,
				EnableGroup:            []string{"default", "user"},
				SupportedEndpointTypes: epTypes,
			})
		}
	}

	pricing = applyPricingUptime(pricing)

	c.JSON(200, gin.H{
		"success":            true,
		"data":               pricing,
		"vendors":            model.GetVendors(),
		"group_ratio":        groupRatio,
		"usable_group":       usableGroup,
		"supported_endpoint": model.GetSupportedEndpointMap(),
		"auto_groups":        service.GetUserAutoGroup(group),
		"always_cache_hit":   operation_setting.GetQuotaSetting().AlwaysCacheHit,
		"pricing_version":    "a42d372ccf0b5dd13ecf71203521f9d2",
	})
}

func ResetModelRatio(c *gin.Context) {
	defaultStr := ratio_setting.DefaultModelRatio2JSONString()
	err := model.UpdateOption("ModelRatio", defaultStr)
	if err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	err = ratio_setting.UpdateModelRatioByJSONString(defaultStr)
	if err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(200, gin.H{
		"success": true,
		"message": "重置模型倍率成功",
	})
}
