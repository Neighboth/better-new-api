package model

import (
	"errors"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// WalletHoldEvent records an idempotent SQL wallet adjustment against an open
// operation. Rows are retained so an ambiguous-commit retry cannot charge twice.
type WalletHoldEvent struct {
	HoldID    string `gorm:"type:char(32);primaryKey" json:"-"`
	EventKey  string `gorm:"type:varchar(64);primaryKey" json:"-"`
	UserID    int    `gorm:"index;not null" json:"-"`
	Delta     int    `json:"-"`
	CreatedAt int64  `json:"-"`
}

func ApplyWalletHoldDelta(userID int, holdID, eventKey string, delta int) error {
	if userID <= 0 || holdID == "" || len(eventKey) == 0 || len(eventKey) > 64 || int64(delta) > int64(common.MaxQuota) || int64(delta) < -int64(common.MaxQuota) {
		return gorm.ErrInvalidData
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).Select("id", "quota").Where("id = ?", userID).First(&user).Error; err != nil {
			return err
		}
		var hold WalletTransferHold
		if err := lockForUpdate(tx).Where("id = ? AND user_id = ?", holdID, userID).First(&hold).Error; err != nil {
			return err
		}
		var previous WalletHoldEvent
		lookup := tx.Where("hold_id = ? AND event_key = ?", holdID, eventKey).First(&previous)
		if lookup.Error == nil {
			if previous.UserID != userID || previous.Delta != delta {
				return ErrWalletHoldConflict
			}
			return nil
		}
		if !errors.Is(lookup.Error, gorm.ErrRecordNotFound) {
			return lookup.Error
		}
		if hold.State != "open" {
			return errors.New("wallet hold is not open")
		}
		balance := int64(user.Quota) + int64(delta)
		if balance < int64(common.MinQuota) || balance > int64(common.MaxQuota) {
			return errors.New("wallet event exceeds supported balance range")
		}
		event := WalletHoldEvent{HoldID: holdID, EventKey: eventKey, UserID: userID, Delta: delta, CreatedAt: time.Now().Unix()}
		if err := tx.Create(&event).Error; err != nil {
			return err
		}
		if delta != 0 {
			result := tx.Model(&User{}).Where("id = ?", userID).Update("quota", gorm.Expr("quota + ?", delta))
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return gorm.ErrRecordNotFound
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if common.RedisEnabled {
		if err := invalidateUserCache(userID); err != nil {
			common.SysError(fmt.Sprintf("wallet event cache invalidation failed for user %d: %v", userID, err))
		}
	}
	return nil
}

// ReserveWalletHoldQuota combines the SQL balance check, debit and event in one
// transaction. Retrying the same successful reservation does not debit twice.
func ReserveWalletHoldQuota(userID int, holdID, eventKey string, amount int) (bool, error) {
	if userID <= 0 || holdID == "" || len(eventKey) == 0 || len(eventKey) > 64 || amount <= 0 || amount > common.MaxQuota {
		return false, gorm.ErrInvalidData
	}
	reserved := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).Select("id", "quota").Where("id = ?", userID).First(&user).Error; err != nil {
			return err
		}
		var hold WalletTransferHold
		if err := lockForUpdate(tx).Where("id = ? AND user_id = ?", holdID, userID).First(&hold).Error; err != nil {
			return err
		}
		var previous WalletHoldEvent
		lookup := tx.Where("hold_id = ? AND event_key = ?", holdID, eventKey).First(&previous)
		if lookup.Error == nil {
			if previous.UserID != userID || previous.Delta != -amount {
				return ErrWalletHoldConflict
			}
			reserved = true
			return nil
		}
		if !errors.Is(lookup.Error, gorm.ErrRecordNotFound) {
			return lookup.Error
		}
		if hold.State != "open" {
			return errors.New("wallet hold is not open")
		}
		if user.Quota < amount {
			return nil
		}
		event := WalletHoldEvent{HoldID: holdID, EventKey: eventKey, UserID: userID, Delta: -amount, CreatedAt: time.Now().Unix()}
		if err := tx.Create(&event).Error; err != nil {
			return err
		}
		result := tx.Model(&User{}).Where("id = ? AND quota >= ?", userID, amount).Update("quota", gorm.Expr("quota - ?", amount))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("wallet balance changed during reservation")
		}
		reserved = true
		return nil
	})
	if err != nil {
		return false, err
	}
	if reserved && common.RedisEnabled {
		if err := invalidateUserCache(userID); err != nil {
			common.SysError(fmt.Sprintf("wallet reservation cache invalidation failed for user %d: %v", userID, err))
		}
	}
	return reserved, nil
}
