package model

import (
	"errors"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// FinalizeMidjourneyReservedTask atomically records the billable decision and
// clears the persisted pending marker. Wallet and token were precharged before
// dispatch. An identical retry is a no-op; an opposite release conflicts.
func FinalizeMidjourneyReservedTask(userID int, holdID string, taskID, tokenID, quota int) (bool, error) {
	if userID <= 0 || holdID == "" || taskID <= 0 || tokenID < 0 || quota <= 0 {
		return false, gorm.ErrInvalidData
	}
	fresh := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).Select("id", "used_quota", "request_count").Where("id = ?", userID).First(&user).Error; err != nil {
			return err
		}
		var hold WalletTransferHold
		if err := lockForUpdate(tx).Where("id = ? AND user_id = ?", holdID, userID).First(&hold).Error; err != nil {
			return err
		}
		var task Midjourney
		if err := lockForUpdate(tx).Where("id = ? AND user_id = ?", taskID, userID).First(&task).Error; err != nil {
			return err
		}
		var pre WalletHoldEvent
		if err := tx.Where("hold_id = ? AND event_key = ?", holdID, "midjourney-precharge").First(&pre).Error; err != nil {
			return err
		}
		tokenPre := 0
		if tokenID > 0 {
			tokenPre = -quota
		}
		if pre.UserID != userID || pre.Delta != -quota || pre.TokenID != tokenID || pre.TokenDelta != tokenPre || task.Quota != quota || task.WalletHoldID != holdID {
			return ErrWalletHoldConflict
		}
		var linked int64
		if err := tx.Model(&Midjourney{}).Where("user_id = ? AND wallet_hold_id = ?", userID, holdID).Count(&linked).Error; err != nil {
			return err
		}
		if linked != 1 {
			return errors.New("Midjourney reservation has an ambiguous task link")
		}
		var decision WalletHoldEvent
		existing := tx.Where("hold_id = ? AND event_key = ?", holdID, "midjourney-decision").Limit(1).Find(&decision)
		if existing.Error != nil {
			return existing.Error
		}
		if existing.RowsAffected > 0 {
			if decision.UserID != userID || decision.Delta != 0 || decision.TokenID != tokenID || decision.TokenDelta != 0 {
				return ErrWalletHoldConflict
			}
			// A marker still pending with an existing decision could be a
			// pre-migration partial commit; never guess whether token charged.
			if task.BillingPending || task.TokenId != tokenID {
				return errors.New("Midjourney decision requires reconciliation")
			}
			return nil
		}
		if hold.State != "open" || !task.BillingPending {
			return errors.New("Midjourney reservation is not pending")
		}
		if int64(user.UsedQuota)+int64(quota) > int64(common.MaxQuota) || int64(user.RequestCount)+1 > int64(common.MaxQuota) {
			return errors.New("Midjourney usage counters exceed supported range")
		}
		event := WalletHoldEvent{HoldID: holdID, EventKey: "midjourney-decision", UserID: userID, Delta: 0, TokenID: tokenID, TokenDelta: 0, CreatedAt: time.Now().Unix()}
		if err := tx.Create(&event).Error; err != nil {
			return err
		}
		result := tx.Model(&Midjourney{}).Where("id = ? AND user_id = ? AND billing_pending = ? AND quota = ?", taskID, userID, true, quota).Updates(map[string]interface{}{"billing_pending": false, "token_id": tokenID})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("Midjourney billing marker changed")
		}
		result = tx.Model(&User{}).Where("id = ?", userID).Updates(map[string]interface{}{"used_quota": gorm.Expr("used_quota + ?", quota), "request_count": gorm.Expr("request_count + 1")})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		if channelID := task.GetBillingChannelId(); channelID > 0 {
			if err := tx.Model(&Channel{}).Where("id = ?", channelID).Update("used_quota", gorm.Expr("used_quota + ?", quota)).Error; err != nil {
				return err
			}
		}
		fresh = true
		return nil
	})
	return fresh, err
}
