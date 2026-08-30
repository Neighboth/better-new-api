package model

type ManagedFile struct {
	Id           int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Path         string `json:"path" gorm:"type:varchar(512);not null;uniqueIndex"` // Relative path, e.g. "deneme.html" or "docs/config.json"
	Name         string `json:"name" gorm:"type:varchar(255);not null"`
	IsDir        bool   `json:"is_dir" gorm:"not null;default:false"`
	Size         int64  `json:"size" gorm:"not null;default:0"`
	Password     string `json:"password" gorm:"type:varchar(255);default:''"`
	EnableCaptcha bool  `json:"enable_captcha" gorm:"not null;default:false"`
	CreatedAt    int64  `json:"created_at" gorm:"index"`
	UpdatedAt    int64  `json:"updated_at" gorm:"index"`
}

func initManagedFileDB() {
	_ = DB.AutoMigrate(&ManagedFile{})
}
