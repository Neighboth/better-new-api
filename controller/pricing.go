package controller

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
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

	c.JSON(200, gin.H{
		"success":            true,
		"data":               pricing,
		"vendors":            model.GetVendors(),
		"group_ratio":        groupRatio,
		"usable_group":       usableGroup,
		"supported_endpoint": model.GetSupportedEndpointMap(),
		"auto_groups":        service.GetUserAutoGroup(group),
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
