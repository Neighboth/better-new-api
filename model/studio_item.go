package model

import (
	"gorm.io/gorm"
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

// DeleteStudioItem removes one stored item payload for the user.
func DeleteStudioItem(userId int, itemId string) error {
	return DB.Where("user_id = ? AND item_id = ?", userId, itemId).Delete(&StudioItem{}).Error
}
