package controller

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/byog"
	"github.com/gin-gonic/gin"
)

// ServeByogWorkerModel streams a model file to a worker.
//
// The worker downloads the model from the coordinator, the coordinator caches the file
// so that every device of the installation fetches it over a fast and stable path.
func ServeByogWorkerModel(c *gin.Context) {
	modelId := c.Param("model_id")
	if modelId == "" {
		c.JSON(400, gin.H{"success": false, "message": "model_id required"})
		return
	}
	modelDef, _ := model.GetByogModelByModelId(modelId)
	if modelDef != nil && modelDef.DownloadUrl != "" && (strings.HasPrefix(modelDef.DownloadUrl, "http://") || strings.HasPrefix(modelDef.DownloadUrl, "https://")) {
		// Client directly downloads from origin url without burdening server disk or network
		c.Redirect(http.StatusFound, modelDef.DownloadUrl)
		return
	}
	byog.GlobalModelStore.ServeModelFile(c, modelId)
}
