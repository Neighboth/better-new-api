package model

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"gorm.io/gorm"
)

// ByogPairingSession stores a one-time, time-limited pairing code by hash.
// The shared database makes codes work across coordinator instances.
type ByogPairingSession struct {
	CodeHash  string `json:"-" gorm:"type:char(64);primaryKey"`
	UserId    int    `json:"-" gorm:"index;not null"`
	ExpiresAt int64  `json:"-" gorm:"index;not null"`
}

func byogPairingHash(code string) string {
	code = strings.ReplaceAll(strings.ReplaceAll(strings.ToUpper(strings.TrimSpace(code)), "-", ""), " ", "")
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

func CreateByogPairingSession(code string, userID int, expiresAt int64) error {
	if code == "" || userID <= 0 {
		return gorm.ErrInvalidData
	}
	// Best-effort pruning; correctness does not depend on it.
	_ = DB.Where("expires_at < ?", time.Now().Unix()-900).Delete(&ByogPairingSession{}).Error
	return DB.Create(&ByogPairingSession{CodeHash: byogPairingHash(code), UserId: userID, ExpiresAt: expiresAt}).Error
}

func ConsumeByogPairingSession(code string) (int, error) {
	if strings.TrimSpace(code) == "" {
		return 0, gorm.ErrRecordNotFound
	}
	hash, now := byogPairingHash(code), time.Now().Unix()
	var session ByogPairingSession
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := lockForUpdate(tx).Where("code_hash = ? AND expires_at >= ?", hash, now).First(&session).Error; err != nil {
			return err
		}
		result := tx.Where("code_hash = ? AND expires_at >= ?", hash, now).Delete(&ByogPairingSession{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return session.UserId, nil
}
