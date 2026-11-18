package controller

import (
	"errors"
	"fmt"
	"net/http"
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
	allowAll := c.Query("all") == "1" || c.Query("all") == "true"
	freed, deleted, deletedURLs, err := service.DeleteOldestVideoAssets(userId, limit, allowAll)
	if err != nil {
		var wouldDeleteAll *service.WouldDeleteAllError
		if errors.As(err, &wouldDeleteAll) {
			c.JSON(http.StatusConflict, gin.H{
				"success":     false,
				"message":     "Freeing the requested amount would delete the entire stored library",
				"code":        "would_delete_all_stored",
				"total_bytes": wouldDeleteAll.TotalBytes,
				"total_count": wouldDeleteAll.TotalCount,
			})
			return
		}
		common.SysError(fmt.Sprintf("video storage delete-oldest for user %d failed: %s", userId, err.Error()))
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"freed_bytes":  freed,
		"deleted":      deleted,
		"deleted_urls": deletedURLs,
	})
}
