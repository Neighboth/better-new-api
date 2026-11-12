/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

// The referral balance of a user. It is money that the site granted to the
// user as an invitation reward: it is tracked separately from the plain wallet
// quota, it is always spent before the plain wallet, and the user sees it at
// the display amount the administrator configured.
//
// Two numbers are stored: ReferralQuota (real quota units, what a settlement
// actually spends) and ReferralDisplay (display quota units, what the wallet
// shows). The two shrink together at the ratio that holds when the money was
// spent, which keeps the displayed balance and the real balance consistent no
// matter how grants of different sides were mixed.

var ErrReferralQuotaNotFound = errors.New("referral balance of the user was not found")

// referralDisplayDelta returns how much the display balance must shrink when
// `take` real quota units are deducted from `real` real units with `display`
// display units. The split is proportional, rounded down, so the display never
// promises more than the ratio allows.
func referralDisplayDelta(real, display, take int64) int64 {
	if real <= 0 {
		return 0
	}
	delta := display * take / real
	if delta > display {
		delta = display
	}
	return delta
}

// referralDisplayAdd returns how much the display balance must grow when
// `amount` real quota units are added to a user holding `real` real units and
// `display` display units.
func referralDisplayAdd(real, display, amount int64) int64 {
	if real <= 0 {
		// A user whose real balance is 0 has no ratio to restore: credit the
		// display at the ratio of the last grant, which is 1:1 by definition of
		// an empty baseline.
		if display > 0 {
			// Ratio unknown but display exists: keep it simple and restore at
			// the same proportion as other users usually see (1:1).
			return amount
		}
		return amount
	}
	add := display * amount / real
	if add <= 0 && amount > 0 {
		add = 1
	}
	return add
}

// ReserveReferralQuota atomically moves up to `amount` real quota units from
// the referral balance of the user. The display balance shrinks by the
// proportional amount. It returns how much was taken; when the referral
// balance is empty it takes 0, which is not an error.
func ReserveReferralQuota(userID int, amount int64) (taken int64, displayTaken int64, err error) {
	if userID <= 0 {
		return 0, 0, gorm.ErrInvalidData
	}
	if amount <= 0 {
		return 0, 0, nil
	}
	err = DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if txErr := lockForUpdate(tx).Select("id", "referral_quota", "referral_display").Where("id = ?", userID).First(&user).Error; txErr != nil {
			return txErr
		}
		if user.ReferralQuota <= 0 {
			taken, displayTaken = 0, 0
			return nil
		}
		take := user.ReferralQuota
		if take > amount {
			take = amount
		}
		deltaDisplay := referralDisplayDelta(user.ReferralQuota, user.ReferralDisplay, take)
		result := tx.Model(&User{}).
			Where("id = ? AND referral_quota >= ?", userID, take).
			Updates(map[string]interface{}{
				"referral_quota":   gorm.Expr("referral_quota - ?", take),
				"referral_display": gorm.Expr("referral_display - ?", deltaDisplay),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("referral balance changed during reservation")
		}
		taken, displayTaken = take, deltaDisplay
		return nil
	})
	return taken, displayTaken, err
}

// RefundReferralQuota credits `amount` real quota units back to the referral
// balance of the user. The display balance grows by the proportional amount.
// It is used when a pre-reserved amount is returned.
func RefundReferralQuota(userID int, amount int64) error {
	if userID <= 0 || amount <= 0 {
		return nil
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).Select("id", "referral_quota", "referral_display").Where("id = ?", userID).First(&user).Error; err != nil {
			return err
		}
		addDisplay := referralDisplayAdd(user.ReferralQuota, user.ReferralDisplay, amount)
		result := tx.Model(&User{}).Where("id = ?", userID).Updates(map[string]interface{}{
			"referral_quota":   gorm.Expr("referral_quota + ?", amount),
			"referral_display": gorm.Expr("referral_display + ?", addDisplay),
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

// CreditReferralBalance grants a fixed amount of referral reward to the user:
// `realAmount` real quota units and `displayAmount` display quota units. It is
// what an invitation reward calls.
func CreditReferralBalance(userID int, realAmount, displayAmount int) error {
	if userID <= 0 {
		return gorm.ErrInvalidData
	}
	if realAmount < 0 || displayAmount < 0 {
		return gorm.ErrInvalidData
	}
	if realAmount == 0 && displayAmount == 0 {
		return nil
	}
	result := DB.Model(&User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"referral_quota":   gorm.Expr("referral_quota + ?", realAmount),
		"referral_display": gorm.Expr("referral_display + ?", displayAmount),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	syncReferralBalanceCache(userID)
	return nil
}

// SetReferralBalance writes the referral balance of the user to absolute
// values. It is the administrator's edit of the real and of the displayed
// balance.
func SetReferralBalance(userID int, realBalance, displayBalance int64) error {
	if userID <= 0 || realBalance < 0 || displayBalance < 0 {
		return gorm.ErrInvalidData
	}
	result := DB.Model(&User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"referral_quota":   realBalance,
		"referral_display": displayBalance,
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	syncReferralBalanceCache(userID)
	return nil
}

// GetReferralQuota returns the real referral balance of the user.
func GetReferralQuota(userID int) (int64, error) {
	var user User
	if err := DB.Model(&User{}).Where("id = ?", userID).Select("referral_quota").First(&user).Error; err != nil {
		return 0, err
	}
	return user.ReferralQuota, nil
}

// GetReferralDisplay returns the display referral balance of the user.
func GetReferralDisplay(userID int) (int64, error) {
	var user User
	if err := DB.Model(&User{}).Where("id = ?", userID).Select("referral_display").First(&user).Error; err != nil {
		return 0, err
	}
	return user.ReferralDisplay, nil
}

// syncReferralBalanceCache keeps the Redis user cache consistent with the SQL
// values of the referral columns. The referral columns are not part of the
// cached quota, so this is a best-effort invalidation of the cached row.
func syncReferralBalanceCache(userID int) {
	if !common.RedisEnabled {
		return
	}
	_ = invalidateUserCache(userID)
}
