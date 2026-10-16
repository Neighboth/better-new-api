package model

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
)

type ManagedFile struct {
	Id            int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Path          string `json:"path" gorm:"type:varchar(512);not null;uniqueIndex"` // Relative path, e.g. "deneme.html" or "docs/config.json"
	Name          string `json:"name" gorm:"type:varchar(255);not null"`
	IsDir         bool   `json:"is_dir" gorm:"not null;default:false"`
	Size          int64  `json:"size" gorm:"not null;default:0"`
	Content       []byte `json:"-" gorm:"type:longblob"`                             // File raw content backup in database
	Password      string `json:"password" gorm:"type:varchar(255);default:''"`
	EnableCaptcha bool   `json:"enable_captcha" gorm:"not null;default:false"`
	CreatedAt     int64  `json:"created_at" gorm:"index"`
	UpdatedAt     int64  `json:"updated_at" gorm:"index"`
}

func initManagedFileDB() {
	_ = DB.AutoMigrate(&ManagedFile{})
}

func CleanupOrphanUploadFiles() {
	var files []ManagedFile
	if err := DB.Where("path LIKE 'uploads/%' AND is_dir = false").Find(&files).Error; err != nil {
		return
	}

	var activeAvatars []string
	DB.Model(&User{}).Where("avatar_url LIKE '/uploads/%' OR avatar_url LIKE 'uploads/%'").Pluck("avatar_url", &activeAvatars)

	var activeVendorLogos []string
	DB.Model(&Vendor{}).Where("icon LIKE '/uploads/%' OR icon LIKE 'uploads/%'").Pluck("icon", &activeVendorLogos)

	activeMap := make(map[string]bool)
	for _, a := range activeAvatars {
		activeMap[strings.TrimPrefix(a, "/")] = true
	}
	for _, v := range activeVendorLogos {
		activeMap[strings.TrimPrefix(v, "/")] = true
	}

	customAdsRaw := common.OptionMap["CustomAds"]
	if customAdsRaw != "" {
		var ads []struct {
			Image string `json:"image"`
		}
		if err := common.UnmarshalJsonStr(customAdsRaw, &ads); err == nil {
			for _, ad := range ads {
				activeMap[strings.TrimPrefix(ad.Image, "/")] = true
			}
		}
	}

	for _, file := range files {
		if !activeMap[file.Path] {
			DB.Delete(&ManagedFile{}, file.Id)
		}
	}
}
