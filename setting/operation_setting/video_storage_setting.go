package operation_setting

import (
	"github.com/QuantumNous/new-api/setting/config"
)

// VideoStorageSetting is the administrator configuration of the server-side
// video storage: how much each user may keep, how long it is retained and the
// expiry warning lead time.
type VideoStorageSetting struct {
	// MaxVideoQuotaBytes caps the total size of stored videos per user.
	// Default: 1 GiB.
	MaxVideoQuotaBytes int64 `json:"max_video_quota_bytes"`
	// RetentionDays is how long one stored video lives after its creation.
	// Default: 30.
	RetentionDays int `json:"retention_days"`
	// ExpiryWarnDaysBefore is how many days before deletion the warning
	// mail is sent. Default: 3.
	ExpiryWarnDaysBefore int `json:"expiry_warn_days_before"`
	// MaxVideoAssetBytes caps the size of a single stored asset.
	// Default: 512 MiB.
	MaxVideoAssetBytes int64 `json:"max_video_asset_bytes"`
}

var videoStorageSetting = VideoStorageSetting{
	MaxVideoQuotaBytes:   1 << 30,
	RetentionDays:        30,
	ExpiryWarnDaysBefore: 3,
	MaxVideoAssetBytes:   512 << 20,
}

func init() {
	config.GlobalConfig.Register("video_storage_setting", &videoStorageSetting)
}

// GetVideoStorageSetting returns the current video storage configuration.
func GetVideoStorageSetting() *VideoStorageSetting {
	return &videoStorageSetting
}

func (s *VideoStorageSetting) quotaBytes() int64 {
	if s.MaxVideoQuotaBytes > 0 {
		return s.MaxVideoQuotaBytes
	}
	return 1 << 30
}

// VideoStorageQuotaBytes returns the per-user video storage cap with a sane default.
func VideoStorageQuotaBytes() int64 {
	return videoStorageSetting.quotaBytes()
}

// VideoRetentionDays returns the retention with a sane default.
func VideoRetentionDays() int {
	if videoStorageSetting.RetentionDays > 0 {
		return videoStorageSetting.RetentionDays
	}
	return 30
}

// VideoExpiryWarnDaysBefore returns the warning lead time with a sane default.
func VideoExpiryWarnDaysBefore() int {
	if videoStorageSetting.ExpiryWarnDaysBefore > 0 {
		return videoStorageSetting.ExpiryWarnDaysBefore
	}
	return 3
}

// VideoAssetMaxBytes returns the single-asset cap with a sane default.
func VideoAssetMaxBytes() int64 {
	if videoStorageSetting.MaxVideoAssetBytes > 0 {
		return videoStorageSetting.MaxVideoAssetBytes
	}
	return 512 << 20
}
