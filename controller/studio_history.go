// Studio generation history sync.
//
// The Studio renders the Previous Generations view from a working set kept in
// the browser. This endpoint group mirrors the item payloads server-side so
// the view is account-based: the same items (with their generation details)
// follow the user across browsers and devices. Item media itself lives on
// /uploads/... — the payload is metadata only and is size-capped.
package controller

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const (
	studioPayloadCap = 32 * 1024 // 32 KiB per item payload
	studioBulkCap    = 100
	studioListLimit  = 200
)

// GetStudioItems handles GET /api/studio/items — the caller's stored items,
// newest first.
func GetStudioItems(c *gin.Context) {
	userId := c.GetInt("id")
	if userId <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "authentication required"})
		return
	}
	items, err := model.ListStudioItems(userId, studioListLimit)
	if err != nil {
		common.SysError("list studio items failed: " + err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "failed to list studio items"})
		return
	}
	out := make([]gin.H, 0, len(items))
	for _, item := range items {
		out = append(out, gin.H{
			"item_id":    item.ItemId,
			"payload":    json.RawMessage(item.Payload),
			"created_at": item.CreatedAt,
			"updated_at": item.UpdatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"items": out}})
}

// PutStudioItems handles PUT /api/studio/items — upsert a batch of item
// payloads. The client derives the id from the payload's own `id` field;
// anything else is rejected.
func PutStudioItems(c *gin.Context) {
	userId := c.GetInt("id")
	if userId <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "authentication required"})
		return
	}
	var body struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := common.DecodeJson(c.Request.Body, &body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid request body"})
		return
	}
	if len(body.Items) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "items are required"})
		return
	}
	if len(body.Items) > studioBulkCap {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "too many items in one request (max 100)"})
		return
	}

	now := time.Now().Unix()
	accepted := 0
	for _, raw := range body.Items {
		if len(raw) == 0 || len(raw) > studioPayloadCap {
			continue
		}
		var probe struct {
			ID string `json:"id"`
		}
		if err := common.Unmarshal(raw, &probe); err != nil {
			continue
		}
		id := strings.TrimSpace(probe.ID)
		if id == "" || len(id) > 64 {
			continue
		}
		if err := model.UpsertStudioItem(userId, id, string(raw), now); err != nil {
			common.SysError("upsert studio item failed: " + err.Error())
			continue
		}
		accepted++
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"accepted": accepted}})
}

// DeleteStudioItem handles DELETE /api/studio/items/:id
func DeleteStudioItem(c *gin.Context) {
	userId := c.GetInt("id")
	if userId <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "authentication required"})
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" || len(id) > 64 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid item id"})
		return
	}
	if err := model.DeleteStudioItem(userId, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "failed to delete studio item"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
