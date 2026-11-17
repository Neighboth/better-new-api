package controller

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
)

// GetVideoStorageStats handles GET /api/media/storage.
func GetVideoStorageStats(c *gin.Context) {
	userId := c.GetInt("id")
	if userId <= 0 {
		common.ApiError(c, fmt.Errorf("authentication required"))
		return
	}
	used, limit, count, err := service.VideoStorageStats(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"used_bytes":  used,
		"limit_bytes": limit,
		"count":       count,
	})
}

// DeleteOldestVideoAssets handles DELETE /api/media/storage?bytes=N — it frees
// at least N bytes by deleting the oldest stored videos of the caller.
func DeleteOldestVideoAssets(c *gin.Context) {
	userId := c.GetInt("id")
	if userId <= 0 {
		common.ApiError(c, fmt.Errorf("authentication required"))
		return
	}
	limit := int64(0)
	if raw := c.Query("bytes"); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err == nil && n > 0 {
			limit = n
		}
	}
	if limit <= 0 {
		// sensible default: free 256 MB so one more short generation fits
		limit = 256 << 20
	}
	freed, deleted, err := service.DeleteOldestVideoAssets(userId, limit)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"freed_bytes": freed,
		"deleted":     deleted,
	})
}
