package model

import (
	"errors"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// ReserveByogWalletAndToken reserves both spend limits in one SQL transaction.
// TokenID 0 is used by the authenticated playground (which has no API token).
// The immutable event is also the evidence used by the final settlement.
func ReserveByogWalletAndToken(userID int, holdID string, tokenID, amount int) (bool, error) {
	return reserveWalletAndToken(userID, holdID, tokenID, amount, "byog-precharge")
}

func ReserveMidjourneyWalletAndToken(userID int, holdID string, tokenID, amount int) (bool, error) {
	return reserveWalletAndToken(userID, holdID, tokenID, amount, "midjourney-precharge")
}

func reserveWalletAndToken(userID int, holdID string, tokenID, amount int, eventKey string) (bool, error) {
	if userID <= 0 || holdID == "" || tokenID < 0 || amount <= 0 || amount > common.MaxQuota {
		return false, gorm.ErrInvalidData
	}
	reserved := false
	tokenKey := ""
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).Select("id", "quota").Where("id = ?", userID).First(&user).Error; err != nil {
			return err
		}
		var hold WalletTransferHold
		if err := lockForUpdate(tx).Where("id = ? AND user_id = ?", holdID, userID).First(&hold).Error; err != nil {
			return err
		}
		var token Token
		if tokenID > 0 {
			if err := lockForUpdate(tx).Where("id = ? AND user_id = ?", tokenID, userID).First(&token).Error; err != nil {
				return err
			}
			tokenKey = token.Key
		}
		var prev WalletHoldEvent
		lookup := tx.Where("hold_id = ? AND event_key = ?", holdID, eventKey).Limit(1).Find(&prev)
		if lookup.Error != nil {
			return lookup.Error
		}
		tokenDelta := 0
		if tokenID > 0 {
			tokenDelta = -amount
		}
		if lookup.RowsAffected > 0 {
			if prev.UserID != userID || prev.Delta != -amount || prev.TokenID != tokenID || prev.TokenDelta != tokenDelta {
				return ErrWalletHoldConflict
			}
			reserved = true
			return nil
		}
		if hold.State != "open" {
			return errors.New("wallet hold is not open")
		}
		if user.Quota < amount {
			return nil
		}
		if tokenID > 0 {
			if token.Status != common.TokenStatusEnabled || (token.ExpiredTime != -1 && token.ExpiredTime < common.GetTimestamp()) {
				return errors.New("reservation token is no longer valid")
			}
			if !token.UnlimitedQuota && token.RemainQuota < amount {
				return nil
			}
			if int64(token.RemainQuota)-int64(amount) < int64(common.MinQuota) || int64(token.UsedQuota)+int64(amount) > int64(common.MaxQuota) {
				return errors.New("token reservation exceeds supported range")
			}
		}
		ev := WalletHoldEvent{HoldID: holdID, EventKey: eventKey, UserID: userID, Delta: -amount, TokenID: tokenID, TokenDelta: tokenDelta, CreatedAt: time.Now().Unix()}
		if err := tx.Create(&ev).Error; err != nil {
			return err
		}
		result := tx.Model(&User{}).Where("id = ? AND quota >= ?", userID, amount).Update("quota", gorm.Expr("quota - ?", amount))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("wallet reservation changed")
		}
		if tokenID > 0 {
			result = tx.Model(&Token{}).Where("id = ? AND user_id = ?", tokenID, userID).Updates(map[string]interface{}{"remain_quota": gorm.Expr("remain_quota - ?", amount), "used_quota": gorm.Expr("used_quota + ?", amount), "accessed_time": common.GetTimestamp()})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return errors.New("token reservation changed")
			}
		}
		reserved = true
		return nil
	})
	if err != nil {
		return false, err
	}
	if reserved {
		invalidateReservationCaches(userID, tokenKey)
	}
	return reserved, nil
}

// FinalizeByogWalletAndToken returns the unused authorization atomically to
// the wallet and token. Replays with the same usage are no-ops; conflicting
// usage is rejected. Both events are retained for ambiguous commit recovery.
func FinalizeByogWalletAndToken(userID int, holdID string, tokenID, reserved, actual int) error {
	return finalizeWalletAndToken(userID, holdID, tokenID, reserved, actual, "byog-precharge", "byog-final", nil)
}

// ReleaseMidjourneyWalletAndToken is only for work known not to be billable.
func ReleaseMidjourneyWalletAndToken(userID int, holdID string, tokenID, reserved int) error {
	return finalizeWalletAndToken(userID, holdID, tokenID, reserved, 0, "midjourney-precharge", "midjourney-decision", func(tx *gorm.DB) error {
		var tasks []Midjourney
		if err := lockForUpdate(tx).Where("user_id = ? AND wallet_hold_id = ?", userID, holdID).Limit(2).Find(&tasks).Error; err != nil {
			return err
		}
		if len(tasks) > 1 {
			return errors.New("multiple Midjourney tasks reference one reservation")
		}
		if len(tasks) == 0 {
			return nil
		}
		task := tasks[0]
		if !task.BillingPending && task.Quota == 0 {
			return nil
		}
		if !task.BillingPending || task.Quota != reserved {
			return ErrWalletHoldConflict
		}
		result := tx.Model(&Midjourney{}).Where("id = ? AND billing_pending = ? AND quota = ?", task.Id, true, reserved).Updates(map[string]interface{}{"quota": 0, "billing_pending": false, "token_id": 0})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("Midjourney pending task changed")
		}
		return nil
	})
}

func finalizeWalletAndToken(userID int, holdID string, tokenID, reserved, actual int, preKey, finalKey string, finish func(*gorm.DB) error) error {
	if userID <= 0 || holdID == "" || tokenID < 0 || reserved <= 0 || reserved > common.MaxQuota || actual < 0 || actual > reserved {
		return gorm.ErrInvalidData
	}
	refund := reserved - actual
	tokenKey := ""
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).Select("id", "quota").Where("id = ?", userID).First(&user).Error; err != nil {
			return err
		}
		var hold WalletTransferHold
		if err := lockForUpdate(tx).Where("id = ? AND user_id = ?", holdID, userID).First(&hold).Error; err != nil {
			return err
		}
		var token Token
		if tokenID > 0 {
			if err := lockForUpdate(tx.Unscoped()).Where("id = ? AND user_id = ?", tokenID, userID).First(&token).Error; err != nil {
				return err
			}
			tokenKey = token.Key
		}
		var pre WalletHoldEvent
		if err := tx.Where("hold_id = ? AND event_key = ?", holdID, preKey).First(&pre).Error; err != nil {
			return err
		}
		tokenPre := 0
		tokenRefund := 0
		if tokenID > 0 {
			tokenPre = -reserved
			tokenRefund = refund
		}
		if pre.UserID != userID || pre.Delta != -reserved || pre.TokenID != tokenID || pre.TokenDelta != tokenPre {
			return ErrWalletHoldConflict
		}
		var previous WalletHoldEvent
		lookup := tx.Where("hold_id = ? AND event_key = ?", holdID, finalKey).Limit(1).Find(&previous)
		if lookup.Error != nil {
			return lookup.Error
		}
		if lookup.RowsAffected > 0 {
			if previous.UserID != userID || previous.Delta != refund || previous.TokenID != tokenID || previous.TokenDelta != tokenRefund {
				return ErrWalletHoldConflict
			}
			if finish != nil {
				return finish(tx)
			}
			return nil
		}
		if hold.State != "open" {
			return errors.New("wallet hold is not open")
		}
		if int64(user.Quota)+int64(refund) > int64(common.MaxQuota) {
			return errors.New("wallet reservation refund exceeds supported range")
		}
		if tokenID > 0 && (int64(token.RemainQuota)+int64(refund) > int64(common.MaxQuota) || int64(token.UsedQuota)-int64(refund) < int64(common.MinQuota)) {
			return errors.New("token reservation refund exceeds supported range")
		}
		event := WalletHoldEvent{HoldID: holdID, EventKey: finalKey, UserID: userID, Delta: refund, TokenID: tokenID, TokenDelta: tokenRefund, CreatedAt: time.Now().Unix()}
		if err := tx.Create(&event).Error; err != nil {
			return err
		}
		if refund > 0 {
			result := tx.Model(&User{}).Where("id = ?", userID).Update("quota", gorm.Expr("quota + ?", refund))
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return gorm.ErrRecordNotFound
			}
			if tokenID > 0 {
				result = tx.Unscoped().Model(&Token{}).Where("id = ? AND user_id = ?", tokenID, userID).Updates(map[string]interface{}{"remain_quota": gorm.Expr("remain_quota + ?", refund), "used_quota": gorm.Expr("used_quota - ?", refund), "accessed_time": common.GetTimestamp()})
				if result.Error != nil {
					return result.Error
				}
				if result.RowsAffected != 1 {
					return errors.New("token reservation refund changed")
				}
			}
		}
		if finish != nil {
			return finish(tx)
		}
		return nil
	})
	if err != nil {
		return err
	}
	invalidateReservationCaches(userID, tokenKey)
	return nil
}

func invalidateReservationCaches(userID int, tokenKey string) {
	if !common.RedisEnabled {
		return
	}
	if err := invalidateUserCache(userID); err != nil {
		common.SysError(fmt.Sprintf("reservation user cache invalidation failed for %d: %v", userID, err))
	}
	if err := invalidateTokenCacheForMutation(tokenKey); err != nil {
		common.SysError(fmt.Sprintf("reservation token cache invalidation failed for %d: %v", userID, err))
	}
}
