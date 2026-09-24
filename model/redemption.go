package model

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"

	"gorm.io/gorm"
)

type Redemption struct {
	Id              int            `json:"id"`
	UserId          int            `json:"user_id"`
	Key             string         `json:"key" gorm:"type:char(32);uniqueIndex"`
	Status          int            `json:"status" gorm:"default:1"`
	Name            string         `json:"name" gorm:"index"`
	Quota           int            `json:"quota" gorm:"default:100"`
	Type            int            `json:"type" gorm:"type:int;default:0"` // 0: Quota, 1: Requests, 2: Tokens
	ModelFilterMode string         `json:"model_filter_mode" gorm:"type:varchar(16);default:'none'"` // none, whitelist, blacklist
	Models          string         `json:"models" gorm:"type:text"` // comma separated models
	CreatedTime     int64          `json:"created_time" gorm:"bigint"`
	RedeemedTime int64          `json:"redeemed_time" gorm:"bigint"`
	Count        int            `json:"count" gorm:"-:all"` // only for api request
	UsedUserId   int            `json:"used_user_id"`
	UsedUsername string         `json:"used_username" gorm:"-:all"`
	IsReseller   bool           `json:"is_reseller" gorm:"-:all"`
	DeletedAt    gorm.DeletedAt `gorm:"index"`
	ExpiredTime  int64          `json:"expired_time" gorm:"bigint"` // 过期时间，0 表示不过期
}

func GetAllRedemptions(startIdx int, num int) (redemptions []*Redemption, total int64, err error) {
	// 开始事务
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 获取总数
	err = tx.Model(&Redemption{}).Count(&total).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// 获取分页数据
	err = tx.Order("id desc").Limit(num).Offset(startIdx).Find(&redemptions).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// 提交事务
	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}

	return redemptions, total, nil
}

func SearchRedemptions(keyword string, status string, startIdx int, num int) (redemptions []*Redemption, total int64, err error) {
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	query := tx.Model(&Redemption{})

	if keyword != "" {
		if id, err := strconv.Atoi(keyword); err == nil {
			query = query.Where("id = ? OR name LIKE ?", id, keyword+"%")
		} else {
			query = query.Where("name LIKE ?", keyword+"%")
		}
	}

	if status != "" {
		now := common.GetTimestamp()
		switch status {
		case "expired":
			query = query.Where(
				"status = ? AND expired_time != 0 AND expired_time < ?",
				common.RedemptionCodeStatusEnabled,
				now,
			)
		case strconv.Itoa(common.RedemptionCodeStatusEnabled):
			query = query.Where(
				"status = ? AND (expired_time = 0 OR expired_time >= ?)",
				common.RedemptionCodeStatusEnabled,
				now,
			)
		case strconv.Itoa(common.RedemptionCodeStatusDisabled):
			query = query.Where("status = ?", common.RedemptionCodeStatusDisabled)
		case strconv.Itoa(common.RedemptionCodeStatusUsed):
			query = query.Where("status = ?", common.RedemptionCodeStatusUsed)
		}
	}

	// Get total count
	err = query.Count(&total).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// Get paginated data
	err = query.Order("id desc").Limit(num).Offset(startIdx).Find(&redemptions).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}

	return redemptions, total, nil
}

func GetRedemptionById(id int) (*Redemption, error) {
	if id == 0 {
		return nil, errors.New("id 为空！")
	}
	redemption := Redemption{Id: id}
	var err error = nil
	err = DB.First(&redemption, "id = ?", id).Error
	return &redemption, err
}

func Redeem(key string, userId int) (quota int, err error) {
	if key == "" {
		return 0, errors.New("未提供兑换码")
	}
	if userId == 0 {
		return 0, errors.New("无效的 user id")
	}
	redemption := &Redemption{}

	keyCol := "`key`"
	if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		keyCol = `"key"`
	}
	common.RandomSleep()
	err = DB.Transaction(func(tx *gorm.DB) error {
		err := lockForUpdate(tx).Where(keyCol+" = ?", key).First(redemption).Error
		if err != nil {
			return errors.New("无效的兑换码")
		}
		if redemption.Status != common.RedemptionCodeStatusEnabled {
			return errors.New("该兑换码已被使用")
		}
		if redemption.ExpiredTime != 0 && redemption.ExpiredTime < common.GetTimestamp() {
			return errors.New("该兑换码已过期")
		}
		// Compare-and-swap on status: only the transaction that flips
		// enabled -> used may credit quota, so a concurrent redeem of the
		// same code loses here even without a row lock (e.g. on SQLite).
		result := tx.Model(&Redemption{}).
			Where("id = ? AND status = ?", redemption.Id, common.RedemptionCodeStatusEnabled).
			Updates(map[string]interface{}{
				"redeemed_time": common.GetTimestamp(),
				"status":        common.RedemptionCodeStatusUsed,
				"used_user_id":  userId,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return errors.New("该兑换码已被使用")
		}
		if redemption.ModelFilterMode != "" && redemption.ModelFilterMode != "none" {
			pkg := &UserBalancePackage{
				UserId:          userId,
				RedemptionId:    redemption.Id,
				Name:            redemption.Name,
				Type:            redemption.Type,
				InitialAmount:   int64(redemption.Quota),
				RemainingAmount: int64(redemption.Quota),
				ModelFilterMode: redemption.ModelFilterMode,
				Models:          redemption.Models,
				ExpiredAt:       redemption.ExpiredTime,
				CreatedAt:       common.GetTimestamp(),
				UpdatedAt:       common.GetTimestamp(),
			}
			if err := tx.Create(pkg).Error; err != nil {
				return err
			}
		}

		switch redemption.Type {
		case 1:
			return tx.Model(&User{}).Where("id = ?", userId).Update("requests_balance", gorm.Expr("requests_balance + ?", redemption.Quota)).Error
		case 2:
			return tx.Model(&User{}).Where("id = ?", userId).Update("tokens_balance", gorm.Expr("tokens_balance + ?", redemption.Quota)).Error
		default:
			return tx.Model(&User{}).Where("id = ?", userId).Update("quota", gorm.Expr("quota + ?", redemption.Quota)).Error
		}
	})
	if err != nil {
		common.SysError("redemption failed: " + err.Error())
		return 0, ErrRedeemFailed
	}
	switch redemption.Type {
	case 1:
		RecordLog(userId, LogTypeTopup, fmt.Sprintf("通过兑换码充值 %d 次请求，兑换码ID %d", redemption.Quota, redemption.Id))
	case 2:
		RecordLog(userId, LogTypeTopup, fmt.Sprintf("通过兑换码充值 %d Tokens，兑换码ID %d", redemption.Quota, redemption.Id))
	default:
		syncCreditUserQuotaCache(userId, redemption.Quota, "redemption")
		RecordLog(userId, LogTypeTopup, fmt.Sprintf("通过兑换码充值 %s，兑换码ID %d", logger.LogQuota(redemption.Quota), redemption.Id))
	}
	return redemption.Quota, nil
}

type UserBalancePackage struct {
	Id              int    `json:"id" gorm:"primaryKey"`
	UserId          int    `json:"user_id" gorm:"index;not null"`
	RedemptionId    int    `json:"redemption_id" gorm:"index"`
	Name            string `json:"name" gorm:"type:varchar(128)"`
	Type            int    `json:"type" gorm:"default:0"` // 0: Quota, 1: Requests, 2: Tokens
	InitialAmount   int64  `json:"initial_amount" gorm:"bigint"`
	RemainingAmount int64  `json:"remaining_amount" gorm:"bigint"`
	ModelFilterMode string `json:"model_filter_mode" gorm:"type:varchar(16);default:'none'"` // whitelist, blacklist
	Models          string `json:"models" gorm:"type:text"`
	ExpiredAt       int64  `json:"expired_at" gorm:"bigint"` // 0 = never
	CreatedAt       int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt       int64  `json:"updated_at" gorm:"bigint"`
}

func GetUserBalancePackages(userId int) ([]*UserBalancePackage, error) {
	var packages []*UserBalancePackage
	now := common.GetTimestamp()
	err := DB.Where("user_id = ? AND remaining_amount > 0 AND (expired_at = 0 OR expired_at > ?)", userId, now).Order("id asc").Find(&packages).Error
	return packages, err
}

func DeductUserBalancePackage(userId int, modelName string, poolType int, amount int64) (deducted int64, err error) {
	packages, err := GetUserBalancePackages(userId)
	if err != nil || len(packages) == 0 {
		return 0, nil
	}
	for _, pkg := range packages {
		if pkg.Type != poolType {
			continue
		}
		allowed := false
		models := strings.Split(pkg.Models, ",")
		found := false
		for _, m := range models {
			if strings.TrimSpace(m) == modelName {
				found = true
				break
			}
		}
		if pkg.ModelFilterMode == "whitelist" {
			allowed = found
		} else if pkg.ModelFilterMode == "blacklist" {
			allowed = !found
		} else {
			allowed = true
		}
		if !allowed {
			continue
		}
		toDeduct := amount
		if toDeduct > pkg.RemainingAmount {
			toDeduct = pkg.RemainingAmount
		}
		now := common.GetTimestamp()
		updateErr := DB.Model(&UserBalancePackage{}).Where("id = ? AND remaining_amount >= ?", pkg.Id, toDeduct).
			Updates(map[string]interface{}{
				"remaining_amount": gorm.Expr("remaining_amount - ?", toDeduct),
				"updated_at":       now,
			}).Error
		if updateErr == nil {
			return toDeduct, nil
		}
	}
	return 0, nil
}

func (redemption *Redemption) Insert() error {
	var err error
	err = DB.Create(redemption).Error
	return err
}

func (redemption *Redemption) SelectUpdate() error {
	// This can update zero values
	return DB.Model(redemption).Select("redeemed_time", "status").Updates(redemption).Error
}

// Update Make sure your token's fields is completed, because this will update non-zero values
func (redemption *Redemption) Update() error {
	var err error
	err = DB.Model(redemption).Select("name", "status", "quota", "type", "redeemed_time", "expired_time", "model_filter_mode", "models").Updates(redemption).Error
	return err
}

func (redemption *Redemption) Delete() error {
	var err error
	err = DB.Delete(redemption).Error
	return err
}

func DeleteRedemptionById(id int) (err error) {
	if id == 0 {
		return errors.New("id 为空！")
	}
	redemption := Redemption{Id: id}
	err = DB.Where(redemption).First(&redemption).Error
	if err != nil {
		return err
	}
	return redemption.Delete()
}

func DeleteInvalidRedemptions() (int64, error) {
	now := common.GetTimestamp()
	result := DB.Where("status IN ? OR (status = ? AND expired_time != 0 AND expired_time < ?)", []int{common.RedemptionCodeStatusUsed, common.RedemptionCodeStatusDisabled}, common.RedemptionCodeStatusEnabled, now).Delete(&Redemption{})
	return result.RowsAffected, result.Error
}
