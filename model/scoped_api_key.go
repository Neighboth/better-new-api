package model

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

const (
	APIKeyScopeBlog        = "blog"
	APIKeyScopeFileManager = "file_manager"
)

// ScopedAPIKey stores hashed bearer tokens scoped strictly to either the
// external Blog API or the File Manager API. The raw secret is returned once
// upon creation and never stored in plaintext.
type ScopedAPIKey struct {
	Id         int64  `json:"id" gorm:"primaryKey"`
	Scope      string `json:"scope" gorm:"type:varchar(32);not null;index"`
	Name       string `json:"name" gorm:"type:varchar(128);not null"`
	KeyPrefix  string `json:"key_prefix" gorm:"type:varchar(24);not null"`
	KeyHash    string `json:"-" gorm:"type:varchar(64);not null;uniqueIndex"`
	Enabled    bool   `json:"enabled" gorm:"not null"`
	CreatedBy  int    `json:"created_by" gorm:"index"`
	LastUsedAt int64  `json:"last_used_at"`
	CreatedAt  int64  `json:"created_at" gorm:"index"`
}

func HashScopedAPIKey(rawKey string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(rawKey)))
	return hex.EncodeToString(sum[:])
}

func GenerateScopedAPIKey(scope string) (rawKey string, prefix string, hash string, err error) {
	buf := make([]byte, 24)
	if _, err = rand.Read(buf); err != nil {
		return "", "", "", err
	}
	tag := "sk-api-"
	switch scope {
	case APIKeyScopeBlog:
		tag = "sk-blog-"
	case APIKeyScopeFileManager:
		tag = "sk-files-"
	}
	rawKey = tag + hex.EncodeToString(buf)
	prefix = rawKey[:14] + "..."
	hash = HashScopedAPIKey(rawKey)
	return rawKey, prefix, hash, nil
}

func CreateScopedAPIKey(scope, name string, createdBy int) (*ScopedAPIKey, string, error) {
	rawKey, prefix, hash, err := GenerateScopedAPIKey(scope)
	if err != nil {
		return nil, "", err
	}
	rec := &ScopedAPIKey{
		Scope:     scope,
		Name:      strings.TrimSpace(name),
		KeyPrefix: prefix,
		KeyHash:   hash,
		Enabled:   true,
		CreatedBy: createdBy,
		CreatedAt: time.Now().Unix(),
	}
	if rec.Name == "" {
		rec.Name = "API Key"
	}
	if err := DB.Create(rec).Error; err != nil {
		return nil, "", err
	}
	return rec, rawKey, nil
}

func ListScopedAPIKeys(scope string) ([]ScopedAPIKey, error) {
	var keys []ScopedAPIKey
	err := DB.Where("scope = ?", scope).Order("id DESC").Find(&keys).Error
	return keys, err
}

func DeleteScopedAPIKey(scope string, id int64) error {
	return DB.Where("scope = ? AND id = ?", scope, id).Delete(&ScopedAPIKey{}).Error
}

func VerifyScopedAPIKey(scope string, rawKey string) (*ScopedAPIKey, error) {
	trimmed := strings.TrimSpace(rawKey)
	if trimmed == "" {
		return nil, nil
	}
	hash := HashScopedAPIKey(trimmed)
	var rec ScopedAPIKey
	if err := DB.Where("scope = ? AND key_hash = ? AND enabled = ?", scope, hash, true).First(&rec).Error; err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	_ = DB.Model(&ScopedAPIKey{}).Where("id = ?", rec.Id).Update("last_used_at", now).Error
	rec.LastUsedAt = now
	return &rec, nil
}
