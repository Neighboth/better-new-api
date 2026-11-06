package model

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// WalletTransferHold records an operation with a still-unknown final wallet
// charge. Open holds have no automatic expiry: losing a process must not make
// unresolved value transferable. The row remains after settlement to support
// idempotent retries after an ambiguous commit.
//
// This is the persistence foundation only. It is not yet wired into every
// request, background task, refund or legacy billing path.
type WalletTransferHold struct {
	ID              string `gorm:"type:char(32);primaryKey" json:"-"`
	UserID          int    `gorm:"index:idx_wallet_hold_user_state;not null" json:"-"`
	State           string `gorm:"type:varchar(16);index:idx_wallet_hold_user_state;not null" json:"-"`
	CreatedAt       int64  `gorm:"not null" json:"-"`
	ResolvedAt      int64  `json:"-"`
	SettlementDelta int    `json:"-"`
	ResolvedBy      int    `json:"-"`
	ResolutionNote  string `gorm:"type:varchar(500)" json:"-"`
}

var ErrWalletHoldConflict = errors.New("wallet hold is already resolved with a different settlement")

// BeginWalletTransferHold serializes with issuance through the same user-row
// lock. It must be called before initiating any potentially chargeable work.
func BeginWalletTransferHold(userID int) (string, error) {
	if userID <= 0 {
		return "", gorm.ErrInvalidData
	}
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	hold := WalletTransferHold{ID: hex.EncodeToString(raw[:]), UserID: userID, State: "open", CreatedAt: time.Now().Unix()}
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).Select("id").Where("id = ?", userID).First(&user).Error; err != nil {
			return err
		}
		return tx.Create(&hold).Error
	})
	if err != nil {
		return "", err
	}
	return hold.ID, nil
}

// ResolveWalletTransferHold commits the final SQL wallet delta and releases
// the transfer hold atomically. A repeated identical settlement is a no-op;
// a contradictory retry is rejected. It deliberately allows the existing
// billing model's negative balances: prevention of unbounded debt must also be
// addressed at authorization/usage-limit level before release.
func ResolveWalletTransferHold(userID int, id string, delta int) error {
	return resolveWalletTransferHold(userID, id, delta, 0, "")
}

// ManuallyResolveWalletTransferHold records the root operator and reason in the
// same transaction as the wallet adjustment and hold state change.
func ManuallyResolveWalletTransferHold(userID int, id string, delta, operatorID int, note string) error {
	note = strings.TrimSpace(note)
	if operatorID <= 0 || len(note) < 10 || len(note) > 500 {
		return gorm.ErrInvalidData
	}
	return resolveWalletTransferHold(userID, id, delta, operatorID, note)
}

func resolveWalletTransferHold(userID int, id string, delta, operatorID int, note string) error {
	if userID <= 0 || id == "" || int64(delta) > int64(common.MaxQuota) || int64(delta) < -int64(common.MaxQuota) {
		return gorm.ErrInvalidData
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).Select("id", "quota").Where("id = ?", userID).First(&user).Error; err != nil {
			return err
		}
		var hold WalletTransferHold
		if err := lockForUpdate(tx).Where("id = ? AND user_id = ?", id, userID).First(&hold).Error; err != nil {
			return err
		}
		if hold.State == "resolved" {
			if hold.SettlementDelta != delta {
				return ErrWalletHoldConflict
			}
			return nil
		}
		if hold.State != "open" {
			return errors.New("invalid wallet hold state")
		}
		if delta != 0 {
			balance := int64(user.Quota) + int64(delta)
			if balance < -int64(common.MaxQuota) || balance > int64(common.MaxQuota) {
				return errors.New("wallet settlement exceeds supported balance range")
			}
			result := tx.Model(&User{}).Where("id = ?", userID).Update("quota", gorm.Expr("quota + ?", delta))
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return gorm.ErrRecordNotFound
			}
		}
		result := tx.Model(&WalletTransferHold{}).Where("id = ? AND user_id = ? AND state = ?", id, userID, "open").Updates(map[string]any{"state": "resolved", "resolved_at": time.Now().Unix(), "settlement_delta": delta, "resolved_by": operatorID, "resolution_note": note})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("wallet hold changed during settlement")
		}
		return nil
	})
	if err != nil {
		return err
	}
	// Invalidate even on an idempotent retry: the original commit may have
	// succeeded while its acknowledgment or subsequent cache deletion failed.
	if common.RedisEnabled {
		if cacheErr := invalidateUserCache(userID); cacheErr != nil {
			common.SysError(fmt.Sprintf("wallet hold settlement cache invalidation failed for user %d: %v", userID, cacheErr))
		}
	}
	return nil
}
