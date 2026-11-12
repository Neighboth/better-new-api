package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
)

// GetAutoModelMeta returns what the settings card of the Auto Model needs to
// show the default prompt and the ranking dimensions.
//
// The default prompt lives in the setting package: the panel shows it when the
// administrator never edited it, and the reset button of the card restores the
// empty override that means "use the default".
func GetAutoModelMeta(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"enabled":        operation_setting.GetAutoModelSetting().Enabled,
			"default_prompt": operation_setting.AutoModelDefaultPrompt,
			"dimensions":     operation_setting.AutoModelDimensions,
			"model_name":     operation_setting.AutoModelName,
		},
	})
}

// GetAutoModelStats reports the routing decisions of this process.
//
// It is the only place an administrator sees which model the router picks
// without opening the usage logs: the counters live in memory and reset with
// the process, which is enough to see the shape of the traffic.
func GetAutoModelStats(c *gin.Context) {
	setting := operation_setting.GetAutoModelSetting()
	counters := service.AutoModelCounters()
	common.ApiSuccess(c, gin.H{
		"enabled":       setting.Enabled,
		"dimensions":    len(operation_setting.AutoModelDimensions),
		"routing_model": setting.RoutingModel,
		"total":         counters.Total,
		"fallbacks":     counters.Fallbacks,
		"models":        counters.Models,
	})
}
