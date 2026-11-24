package model

import (
	"errors"
	"strings"
	"time"
)

// UserModelAlias maps a private user-defined alias (e.g. "auto-myapp") to a
// target model ID (e.g. "opus-5.5"). Aliases are strictly scoped to UserId.
type UserModelAlias struct {
	Id          int    `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId      int    `json:"user_id" gorm:"index;not null"`
	Alias       string `json:"alias" gorm:"type:varchar(128);index;not null"`
	TargetModel string `json:"target_model" gorm:"type:varchar(256);not null"`
	Description string `json:"description" gorm:"type:varchar(256)"`
	CreatedAt   int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt   int64  `json:"updated_at" gorm:"bigint"`
}

func ListUserModelAliases(userId int) ([]UserModelAlias, error) {
	if DB == nil || userId <= 0 {
		return nil, nil
	}
	var items []UserModelAlias
	err := DB.Where("user_id = ?", userId).Order("id desc").Find(&items).Error
	return items, err
}

func CreateUserModelAlias(userId int, alias, targetModel, description string) (*UserModelAlias, error) {
	alias = strings.TrimSpace(alias)
	targetModel = strings.TrimSpace(targetModel)
	description = strings.TrimSpace(description)
	if alias == "" || targetModel == "" {
		return nil, errors.New("alias and target_model are required")
	}
	if strings.EqualFold(alias, targetModel) {
		return nil, errors.New("alias cannot equal target_model")
	}
	var existing UserModelAlias
	if err := DB.Where("user_id = ? AND alias = ?", userId, alias).First(&existing).Error; err == nil && existing.Id > 0 {
		return nil, errors.New("alias already exists")
	}
	now := time.Now().Unix()
	item := &UserModelAlias{
		UserId:      userId,
		Alias:       alias,
		TargetModel: targetModel,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := DB.Create(item).Error; err != nil {
		return nil, err
	}
	return item, nil
}

func UpdateUserModelAlias(userId, id int, alias, targetModel, description string) (*UserModelAlias, error) {
	var item UserModelAlias
	if err := DB.Where("id = ? AND user_id = ?", id, userId).First(&item).Error; err != nil {
		return nil, err
	}
	if a := strings.TrimSpace(alias); a != "" {
		item.Alias = a
	}
	if tm := strings.TrimSpace(targetModel); tm != "" {
		item.TargetModel = tm
	}
	if strings.EqualFold(item.Alias, item.TargetModel) {
		return nil, errors.New("alias cannot equal target_model")
	}
	var conflict UserModelAlias
	if err := DB.Where("user_id = ? AND alias = ? AND id <> ?", userId, item.Alias, id).First(&conflict).Error; err == nil && conflict.Id > 0 {
		return nil, errors.New("alias already exists")
	}
	item.Description = strings.TrimSpace(description)
	item.UpdatedAt = time.Now().Unix()
	if err := DB.Save(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func DeleteUserModelAlias(userId, id int) error {
	return DB.Where("id = ? AND user_id = ?", id, userId).Delete(&UserModelAlias{}).Error
}

func ResolveUserModelAlias(userId int, alias string) (string, bool) {
	if DB == nil || userId <= 0 {
		return "", false
	}
	trimmed := strings.TrimSpace(alias)
	if trimmed == "" {
		return "", false
	}
	var item UserModelAlias
	if err := DB.Where("user_id = ? AND alias = ?", userId, trimmed).First(&item).Error; err != nil {
		return "", false
	}
	target := strings.TrimSpace(item.TargetModel)
	if target == "" {
		return "", false
	}
	return target, true
}
