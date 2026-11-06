package model

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"math"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

var ErrResellerInsufficientBalance = errors.New("insufficient reseller balance")
var ErrResellerCodeUnavailable = errors.New("redemption code is unavailable")

// CreateResellerBalanceCodes reserves the live Redis balance before the SQL
// transaction when Redis is enabled. The SQL debit and creation of every code
// are atomic; failure invalidates the cache reservation. Fail closed on cache
// failures instead of spending a potentially stale database balance.
func CreateResellerBalanceCodes(userID, amount, count int, name string) ([]string, error) {
	if userID <= 0 || amount <= 0 || count <= 0 || count > 100 || amount > common.MaxQuota/count {
		return nil, errors.New("invalid reseller amount")
	}
	total := amount * count
	// A Redis cache miss can rehydrate from a database that still has queued
	// balance changes. Do not issue transferable value from that stale base.
	if common.BatchUpdateEnabled {
		return nil, errors.New("reseller balance codes are unavailable while batch quota updates are enabled")
	}
	cacheReserved := false
	if common.RedisEnabled {
		result, err := cacheTryReserveUserQuota(userID, int64(total))
		if err == nil && result == cacheQuotaMiss {
			if _, err = GetUserCache(userID); err == nil {
				result, err = cacheTryReserveUserQuota(userID, int64(total))
			}
		}
		if err != nil {
			return nil, fmt.Errorf("reseller balance cache unavailable: %w", err)
		}
		if result == cacheQuotaInsufficient {
			return nil, ErrResellerInsufficientBalance
		}
		if result != cacheQuotaOK {
			return nil, errors.New("reseller balance cache unavailable")
		}
		cacheReserved = true
	}
	keys := make([]string, 0, count)
	err := DB.Transaction(func(tx *gorm.DB) error {
		debit := tx.Model(&User{}).Where("id = ? AND quota >= ?", userID, total).Update("quota", gorm.Expr("quota - ?", total))
		if debit.Error != nil {
			return debit.Error
		}
		if debit.RowsAffected != 1 {
			return ErrResellerInsufficientBalance
		}
		// Model-restricted credit must not be turned into an unrestricted code.
		// Until packages can be split/propagated, fail closed for such accounts.
		var restricted int64
		if err := tx.Model(&UserBalancePackage{}).
			Where("user_id = ? AND type = ? AND remaining_amount > 0 AND model_filter_mode IN ? AND (expired_at = 0 OR expired_at > ?)", userID, 0, []string{"whitelist", "blacklist"}, common.GetTimestamp()).
			Count(&restricted).Error; err != nil {
			return err
		}
		if restricted > 0 {
			return errors.New("restricted model balances cannot be used for reseller codes")
		}
		for i := 0; i < count; i++ {
			random := make([]byte, 21)
			if _, err := rand.Read(random); err != nil {
				return err
			}
			key := "cdk_" + base64.RawURLEncoding.EncodeToString(random)
			row := Redemption{UserId: userID, Name: name, Key: key, Quota: amount, Type: 0, CreatedTime: common.GetTimestamp(), Status: common.RedemptionCodeStatusEnabled}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
			keys = append(keys, key)
		}
		return nil
	})
	if err != nil {
		if cacheReserved {
			// The reserved entry could have been evicted and rehydrated while SQL
			// was pending; adding a delta could create phantom balance.
			if cacheErr := invalidateUserCache(userID); cacheErr != nil {
				common.SysError(fmt.Sprintf("reseller quota cache invalidation failed for user %d: %v", userID, cacheErr))
			}
		}
		return nil, err
	}
	return keys, nil
}

// RefundResellerCode claims a never-redeemed code and credits its original
// balance in one transaction. Historical request/token codes retain refunds.
func RefundResellerCode(userID, codeID int) error {
	var code Redemption
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := lockForUpdate(tx).Where("id = ? AND user_id = ?", codeID, userID).First(&code).Error; err != nil {
			return ErrResellerCodeUnavailable
		}
		if code.Quota <= 0 || code.Status == common.RedemptionCodeStatusUsed || code.UsedUserId != 0 {
			return ErrResellerCodeUnavailable
		}
		claimed := tx.Where("id = ? AND user_id = ? AND status = ? AND COALESCE(used_user_id, 0) = 0", codeID, userID, code.Status).Delete(&Redemption{})
		if claimed.Error != nil {
			return claimed.Error
		}
		if claimed.RowsAffected != 1 {
			return ErrResellerCodeUnavailable
		}
		column := "quota"
		max := int64(common.MaxQuota)
		switch code.Type {
		case 0:
		case 1:
			column = "requests_balance"
		case 2:
			column = "tokens_balance"
			max = math.MaxInt64
		default:
			return errors.New("invalid historical code type")
		}
		credit := tx.Model(&User{}).Where("id = ? AND "+column+" <= ?", userID, max-int64(code.Quota)).Update(column, gorm.Expr(column+" + ?", code.Quota))
		if credit.Error != nil {
			return credit.Error
		}
		if credit.RowsAffected != 1 {
			return errors.New("refund exceeds balance limit or user missing")
		}
		return nil
	})
	if err == nil && code.Type == 0 {
		syncCreditUserQuotaCache(userID, code.Quota, "reseller refund")
	}
	return err
}
