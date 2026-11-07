package model

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// RefundHeldTaskWalletOnce atomically refunds a confirmed, durably marked
// failure. It returns the amount only on the first committed application.
// A prior wallet-only task-refund event is deliberately not migrated here:
// its token and counter outcome is unknown and needs manual reconciliation.
func RefundHeldTaskWalletOnce(userID int, taskID int64, holdID string) (int, error) {
	if userID <= 0 || taskID <= 0 || holdID == "" {
		return 0, gorm.ErrInvalidData
	}
	amount := 0
	tokenKey := ""
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).Select("id", "quota", "used_quota").Where("id = ?", userID).First(&user).Error; err != nil {
			return err
		}
		var hold WalletTransferHold
		if err := lockForUpdate(tx).Where("id = ? AND user_id = ?", holdID, userID).First(&hold).Error; err != nil {
			return err
		}
		var task Task
		if err := lockForUpdate(tx).Where("id = ? AND user_id = ?", taskID, userID).First(&task).Error; err != nil {
			return err
		}
		if task.PrivateData.WalletHoldID != holdID || task.PrivateData.BillingSource != "wallet" || task.PrivateData.WalletHoldNeedsReview || task.Status != TaskStatusFailure {
			return errors.New("task wallet refund requires reconciliation")
		}
		// A hold is only safe to close if no other potentially billable task
		// points to it. JSON representation differs across supported SQL
		// dialects, so check decoded private data rather than a JSON operator.
		var related []Task
		if err := tx.Select("id", "private_data").Where("user_id = ? AND id <> ? AND (quota <> ? OR refund_pending = ? OR status NOT IN ?)", userID, taskID, 0, true, []TaskStatus{TaskStatusSuccess, TaskStatusFailure}).Find(&related).Error; err != nil {
			return err
		}
		for _, other := range related {
			if other.PrivateData.WalletHoldID == holdID {
				return errors.New("task hold is shared and requires reconciliation")
			}
		}
		var old WalletHoldEvent
		if err := tx.Where("hold_id = ? AND event_key = ?", holdID, "task-refund").First(&old).Error; err == nil {
			return errors.New("legacy task refund requires reconciliation")
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var previous WalletHoldEvent
		lookup := tx.Where("hold_id = ? AND event_key = ?", holdID, "task-atomic-refund").Limit(1).Find(&previous)
		if lookup.Error != nil {
			return lookup.Error
		}
		if lookup.RowsAffected > 0 {
			if previous.UserID != userID || previous.Delta <= 0 || previous.TokenID != task.PrivateData.TokenId || previous.TokenDelta != tokenRefundDelta(task.PrivateData.TokenId, previous.Delta) || task.RefundPending || task.Quota != 0 {
				return ErrWalletHoldConflict
			}
			if task.PrivateData.TokenId > 0 {
				var token Token
				if err := tx.Unscoped().Select("id", "key").Where("id = ? AND user_id = ?", task.PrivateData.TokenId, userID).First(&token).Error; err != nil {
					return err
				}
				tokenKey = token.Key
			}
			return nil
		}
		if hold.State != "open" || !task.RefundPending || task.Quota <= 0 {
			return errors.New("task refund is not pending")
		}
		var otherReservations int64
		if err := tx.Model(&WalletHoldEvent{}).Where("hold_id = ? AND event_key IN ?", holdID, []string{"byog-precharge", "midjourney-precharge"}).Count(&otherReservations).Error; err != nil {
			return err
		}
		if otherReservations != 0 {
			return errors.New("task hold has another reservation requiring reconciliation")
		}
		quota := task.Quota
		if int64(user.Quota)+int64(quota) > int64(common.MaxQuota) || int64(user.UsedQuota)-int64(quota) < int64(common.MinQuota) {
			return errors.New("task refund exceeds wallet accounting range")
		}
		var token Token
		if task.PrivateData.TokenId > 0 {
			if err := lockForUpdate(tx.Unscoped()).Where("id = ? AND user_id = ?", task.PrivateData.TokenId, userID).First(&token).Error; err != nil {
				return err
			}
			tokenKey = token.Key
			if int64(token.RemainQuota)+int64(quota) > int64(common.MaxQuota) || int64(token.UsedQuota)-int64(quota) < int64(common.MinQuota) {
				return errors.New("task refund exceeds token accounting range")
			}
		}
		var channel Channel
		if task.ChannelId > 0 {
			found := lockForUpdate(tx).Where("id = ?", task.ChannelId).Limit(1).Find(&channel)
			if found.Error != nil {
				return found.Error
			}
			if found.RowsAffected > 0 && channel.UsedQuota < math.MinInt64+int64(quota) {
				return errors.New("task refund exceeds channel accounting range")
			}
		}
		event := WalletHoldEvent{HoldID: holdID, EventKey: "task-atomic-refund", UserID: userID, Delta: quota, TokenID: task.PrivateData.TokenId, TokenDelta: tokenRefundDelta(task.PrivateData.TokenId, quota), CreatedAt: time.Now().Unix()}
		if err := tx.Create(&event).Error; err != nil {
			return err
		}
		result := tx.Model(&Task{}).Where("id = ? AND user_id = ? AND quota = ? AND refund_pending = ? AND status = ?", taskID, userID, quota, true, TaskStatusFailure).Updates(map[string]interface{}{"quota": 0, "refund_pending": false})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("task refund marker changed")
		}
		result = tx.Model(&User{}).Where("id = ?", userID).Updates(map[string]interface{}{"quota": gorm.Expr("quota + ?", quota), "used_quota": gorm.Expr("used_quota - ?", quota)})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		if task.PrivateData.TokenId > 0 {
			result = tx.Unscoped().Model(&Token{}).Where("id = ? AND user_id = ?", task.PrivateData.TokenId, userID).Updates(map[string]interface{}{"remain_quota": gorm.Expr("remain_quota + ?", quota), "used_quota": gorm.Expr("used_quota - ?", quota), "accessed_time": common.GetTimestamp()})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return gorm.ErrRecordNotFound
			}
		}
		if channel.Id > 0 {
			if err := tx.Model(&Channel{}).Where("id = ?", channel.Id).Update("used_quota", gorm.Expr("used_quota - ?", quota)).Error; err != nil {
				return err
			}
		}
		// Close the task hold in the same commit. A crash cannot leave a
		// successfully refunded task stuck behind an orphaned open hold.
		result = tx.Model(&WalletTransferHold{}).Where("id = ? AND user_id = ? AND state = ?", holdID, userID, "open").Updates(map[string]interface{}{"state": "resolved", "resolved_at": time.Now().Unix(), "settlement_delta": 0})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("task refund hold changed")
		}
		amount = quota
		return nil
	})
	if err != nil {
		return 0, err
	}
	if common.RedisEnabled {
		invalidateReservationCaches(userID, tokenKey)
	}
	return amount, nil
}

func tokenRefundDelta(tokenID, amount int) int {
	if tokenID > 0 {
		return amount
	}
	return 0
}

// PendingHeldTaskRefunds is intentionally bounded. Legacy terminal tasks do
// not have a refund intent and must not be guessed or backfilled.
func PendingHeldTaskRefunds(limit int, afterID int64) ([]*Task, error) {
	if limit < 1 || limit > 1000 || afterID < 0 {
		return nil, fmt.Errorf("invalid task refund recovery cursor")
	}
	var tasks []*Task
	err := DB.Where("refund_pending = ? AND status = ? AND id > ?", true, TaskStatusFailure, afterID).Order("id ASC").Limit(limit).Find(&tasks).Error
	return tasks, err
}
