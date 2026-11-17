package model

// VideoAsset tracks one server-stored media file bound to a task.
// Rows are kept even after deletion (Deleted=true) so the audit trail and
// the quota accounting of what happened in the past stay consistent;
// deleted files of deleted tasks are garbage-collected by the expiry sweeper.
type VideoAsset struct {
	ID            int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId        int    `json:"user_id" gorm:"index;not null"`
	TaskId        string `json:"task_id" gorm:"size:64;index;not null"`
	FileName      string `json:"-" gorm:"size:255;not null"`
	Size          int64  `json:"size" gorm:"not null"`
	Kind          string `json:"kind" gorm:"size:16;not null;default:'video';index"`
	CreatedAt     int64  `json:"created_at" gorm:"index;not null"`
	ExpiresAt     int64  `json:"expires_at" gorm:"index;not null"`
	WarnSent      bool   `json:"warn_sent" gorm:"not null;default:false"`
	Deleted       bool   `json:"deleted" gorm:"not null;default:false;index"`
	DeletedReason string `json:"deleted_reason,omitempty" gorm:"size:32"`
}

func (VideoAsset) TableName() string { return "video_assets" }

func ActiveVideoStorageBytes(userID int) (int64, error) {
	var total int64
	err := DB.Model(&VideoAsset{}).
		Where("user_id = ? AND deleted = ?", userID, false).
		Select("COALESCE(SUM(size), 0)").
		Scan(&total).Error
	if err != nil {
		return 0, err
	}
	return total, nil
}

func ActiveVideoStorageCount(userID int) (int64, error) {
	var count int64
	err := DB.Model(&VideoAsset{}).
		Where("user_id = ? AND deleted = ?", userID, false).
		Count(&count).Error
	return count, err
}
