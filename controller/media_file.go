// Per-item media file deletion for the Studio's generation history.
//
// The trash button in the Studio's Previous Generations dialog removes one
// generation at a time. A per-user video stored at /uploads/videos/<uid>/<file>
// (or a merged MP4 at /uploads/merged/<file>) must release both the file and
// its video_assets quota row; deleting only the browser entry leaks the server
// quota invisibly.
package controller

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// DeleteMediaFile handles DELETE /api/media/file?url=<path> (or JSON {"url": ...}).
// It removes a media file owned by the caller and marks the matching
// video_assets row(s) deleted, so the quota is released immediately.
func DeleteMediaFile(c *gin.Context) {
	userId := c.GetInt("id")
	if userId <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "authentication required"})
		return
	}
	raw := strings.TrimSpace(c.Query("url"))
	if raw == "" {
		var body struct {
			URL string `json:"url"`
		}
		if err := common.DecodeJson(c.Request.Body, &body); err == nil {
			raw = strings.TrimSpace(body.URL)
		}
	}
	if raw == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "url is required"})
		return
	}

	var diskPath string
	var ownerOk bool

	switch {
	case strings.HasPrefix(raw, "/uploads/videos/"):
		parts := strings.Split(strings.TrimPrefix(raw, "/uploads/videos/"), "/")
		if len(parts) != 2 {
			break
		}
		owner, name := 0, parts[1]
		_, _ = fmt.Sscan(parts[0], &owner)
		if owner != userId || strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
			break
		}
		diskPath = filepath.Join("uploads", "videos", parts[0], name)
		ownerOk = true
	case strings.HasPrefix(raw, "/uploads/merged/"):
		name := strings.TrimPrefix(raw, "/uploads/merged/")
		owner, rest := 0, name
		_, _ = fmt.Sscan(strings.SplitN(rest, "_", 2)[0], &owner)
		if owner == userId && !strings.ContainsAny(name, `/\`) && !strings.Contains(name, "..") {
			diskPath = filepath.Join("uploads", "merged", name)
			ownerOk = true
		}
	}

	if !ownerOk {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "file does not belong to you"})
		return
	}

	fileName := filepath.Base(diskPath)
	_ = os.Remove(diskPath)
	// Mark quota rows for this file deleted (keeps accounting honest).
	_ = model.DB.Model(&model.VideoAsset{}).
		Where("user_id = ? AND file_name = ? AND deleted = ?", userId, fileName, false).
		Updates(map[string]interface{}{"deleted": true, "deleted_reason": "user_delete"}).Error

	c.JSON(http.StatusOK, gin.H{"success": true})
}
