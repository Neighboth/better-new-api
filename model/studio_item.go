package model

import (
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/QuantumNous/new-api/common"
)

// StudioItem is a per-user row for one Studio generation item. The Studio
// keeps its working set in the browser; this table mirrors the item payload
// so the "Previous Generations" view is account-based: deleting a browser or
// switching devices never loses the history, and the payload carries the
// generation details (plan, settings) so they render on any device.
//
// The payload is the StudioItem JSON the client sends. It is capped server-side
// and must never embed media bytes — blob references point at /uploads/... URLs
// or at browser-local IndexedDB entries instead.
type StudioItem struct {
	ID        int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId    int    `json:"-" gorm:"index;not null;uniqueIndex:uk_studio_item_user"`
	ItemId    string `json:"item_id" gorm:"size:64;not null;uniqueIndex:uk_studio_item_user"`
	Payload   string `json:"payload" gorm:"type:text;not null"`
	CreatedAt int64  `json:"created_at" gorm:"not null;index"`
	UpdatedAt int64  `json:"updated_at" gorm:"not null;index"`
}

func (StudioItem) TableName() string { return "studio_items" }

// ListStudioItems returns the caller's stored items, newest first.
func ListStudioItems(userId int, limit int) ([]StudioItem, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	var items []StudioItem
	err := DB.Where("user_id = ?", userId).
		Order("updated_at DESC").
		Limit(limit).
		Find(&items).Error
	return items, err
}

// UpsertStudioItem stores or refreshes one item payload for the user.
func UpsertStudioItem(userId int, itemId, payload string, now int64) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var item StudioItem
		err := tx.Where("user_id = ? AND item_id = ?", userId, itemId).First(&item).Error
		if err == nil {
			item.Payload = payload
			item.UpdatedAt = now
			return tx.Save(&item).Error
		}
		created := StudioItem{
			UserId:    userId,
			ItemId:    itemId,
			Payload:   payload,
			CreatedAt: now,
			UpdatedAt: now,
		}
		return tx.Create(&created).Error
	})
}

// OverlayStudioItem merges the server-side facts of a generation into the
// studio_items row for the user — used when the prompt optimizer and the task
// submission run entirely server-side. It preserves the client's own fields
// (prompt, settings, localOnly, createdAt, urls) and fills in what the server
// knows: the task id, the optimized prompt, and the merged reference images.
func OverlayStudioItem(userId int, itemId string, overlay StudioItemOverlay, now int64) error {
	if DB == nil {
		return nil
	}
	if strings.TrimSpace(itemId) == "" {
		return fmt.Errorf("studio item id is required")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var row StudioItem
		fetchErr := tx.Where("user_id = ? AND item_id = ?", userId, itemId).First(&row).Error
		payload := map[string]interface{}{}
		if fetchErr == nil {
			if strings.TrimSpace(row.Payload) != "" {
				_ = common.UnmarshalJsonStr(row.Payload, &payload)
			}
		}
		// Base fields the client owns — never invent them on the server side
		// when they are missing (the client supplies its own on first sync).
		if _, ok := payload["id"]; !ok {
			payload["id"] = itemId
		}
		if _, ok := payload["mode"]; !ok {
			payload["mode"] = "video"
		}
		if _, ok := payload["prompt"]; !ok && overlay.Prompt != "" {
			payload["prompt"] = overlay.Prompt
		}
		if _, ok := payload["createdAt"]; !ok {
			payload["createdAt"] = now * 1000
		}
		if _, ok := payload["urls"]; !ok {
			payload["urls"] = []string{}
		}
		status, _ := payload["status"].(string)
		if status != "ready" && status != "error" {
			payload["status"] = "pending"
		}
		if overlay.TaskID != "" {
			payload["taskId"] = overlay.TaskID
		}
		if len(overlay.Settings) > 0 {
			settings, _ := payload["settings"].(map[string]interface{})
			if settings == nil {
				settings = map[string]interface{}{}
			}
			for key, value := range overlay.Settings {
				if strings.TrimSpace(value) == "" {
					continue
				}
				settings[key] = value
			}
			payload["settings"] = settings
		}
		details, _ := payload["details"].(map[string]interface{})
		if details == nil {
			details = map[string]interface{}{}
		}
		if overlay.OptimizedPrompt != "" {
			details["optimizedPrompt"] = overlay.OptimizedPrompt
		}
		if overlay.NegativePrompt != "" {
			details["negativePrompt"] = overlay.NegativePrompt
		}
		if len(overlay.References) > 0 {
			kept := make([]string, 0, len(overlay.References))
			for _, ref := range overlay.References {
				// data: URLs are enormous — never persist media bytes in the
				// payload (same rule as the client's own sync).
				if strings.HasPrefix(ref, "data:") {
					continue
				}
				kept = append(kept, ref)
			}
			details["referenceImages"] = kept
		}
		payload["details"] = details
		payload["updatedAt"] = now * 1000
		serialized, err := common.Marshal(payload)
		if err != nil {
			return err
		}
		encoded := string(serialized)
		if fetchErr == nil {
			row.Payload = encoded
			row.UpdatedAt = now
			return tx.Save(&row).Error
		}
		created := StudioItem{
			UserId:    userId,
			ItemId:    itemId,
			Payload:   encoded,
			CreatedAt: now,
			UpdatedAt: now,
		}
		return tx.Create(&created).Error
	})
}

// StudioItemOverlay carries the server-side facts of one generation.
type StudioItemOverlay struct {
	TaskID          string
	Prompt          string
	NegativePrompt  string
	OptimizedPrompt string
	References      []string
	Settings        map[string]string
}

// DeleteStudioItem removes one stored item payload for the user.
func DeleteStudioItem(userId int, itemId string) error {
	return DB.Where("user_id = ? AND item_id = ?", userId, itemId).Delete(&StudioItem{}).Error
}
