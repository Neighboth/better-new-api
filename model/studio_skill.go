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
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
)

// StudioSkill is an installation-level creative direction ("make the video in
// the style of GTA 6", "draw it like A24 trailers", …). The administrator
// manages skills in System Settings; the studio shows them to every user.
// When the user picks one, its content is appended to the prompt optimizer's
// system prompt for that generation.
type StudioSkill struct {
	ID          string `json:"id" gorm:"primaryKey;size:64"`
	Title       string `json:"title" gorm:"size:200;not null"`
	Description string `json:"description" gorm:"size:1000"`
	Image       string `json:"image" gorm:"type:text"`
	Content     string `json:"content" gorm:"type:text;not null"`
	SortOrder   int    `json:"sort_order" gorm:"index"`
	Enabled     bool   `json:"enabled" gorm:"index"`
	CreatedAt   int64  `json:"created_at" gorm:"not null;index"`
	UpdatedAt   int64  `json:"updated_at" gorm:"not null;index"`
}

func (StudioSkill) TableName() string { return "studio_skills" }

// ListStudioSkills returns the enabled skills for the picker, ordered for
// display.
func ListStudioSkills() ([]StudioSkill, error) {
	if DB == nil {
		return []StudioSkill{}, nil
	}
	var skills []StudioSkill
	err := DB.Where("enabled = ?", true).Order("sort_order ASC, id ASC").Find(&skills).Error
	return skills, err
}

// ListStudioSkillsAdmin returns every skill for the admin console, ordered
// for display.
func ListStudioSkillsAdmin() ([]StudioSkill, error) {
	if DB == nil {
		return []StudioSkill{}, nil
	}
	var skills []StudioSkill
	err := DB.Order("sort_order ASC, id ASC").Find(&skills).Error
	return skills, err
}

// GetStudioSkill returns one skill by id, or nil when not found.
func GetStudioSkill(id string) (*StudioSkill, error) {
	if DB == nil {
		return nil, nil
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, nil
	}
	var skill StudioSkill
	if err := DB.Where("id = ?", id).First(&skill).Error; err != nil {
		return nil, err
	}
	return &skill, nil
}

func newStudioSkillID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "skl_" + hex.EncodeToString(buf), nil
}

// CreateStudioSkill validates and inserts a skill.
func CreateStudioSkill(title, description, image, content string, sortOrder int, now int64) (*StudioSkill, error) {
	title = strings.TrimSpace(title)
	content = strings.TrimSpace(content)
	if title == "" {
		return nil, fmt.Errorf("skill title is required")
	}
	if content == "" {
		return nil, fmt.Errorf("skill content is required")
	}
	id, err := newStudioSkillID()
	if err != nil {
		return nil, err
	}
	skill := &StudioSkill{
		ID:          id,
		Title:       title,
		Description: strings.TrimSpace(description),
		Image:       strings.TrimSpace(image),
		Content:     content,
		SortOrder:   sortOrder,
		Enabled:     true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := DB.Create(skill).Error; err != nil {
		return nil, err
	}
	return skill, nil
}

// UpdateStudioSkill applies a partial update to a skill.
func UpdateStudioSkill(id string, patch map[string]interface{}, now int64) error {
	if DB == nil {
		return nil
	}
	updates := map[string]interface{}{
		"updated_at": now,
	}
	for key, value := range patch {
		switch key {
		case "title":
			s, ok := value.(string)
			if !ok {
				continue
			}
			s = strings.TrimSpace(s)
			if s != "" {
				updates["title"] = s
			}
		case "description":
			s, ok := value.(string)
			if ok {
				updates["description"] = strings.TrimSpace(s)
			}
		case "image":
			s, ok := value.(string)
			if ok {
				updates["image"] = strings.TrimSpace(s)
			}
		case "content":
			s, ok := value.(string)
			if ok {
				s = strings.TrimSpace(s)
				if s != "" {
					updates["content"] = s
				}
			}
		case "sort_order":
			n, ok := value.(float64)
			if ok {
				updates["sort_order"] = int(n)
			}
		case "enabled":
			b, ok := value.(bool)
			if ok {
				updates["enabled"] = b
			}
		}
	}
	return DB.Model(&StudioSkill{}).Where("id = ?", id).Updates(updates).Error
}

// DeleteStudioSkill removes a skill.
func DeleteStudioSkill(id string) error {
	if DB == nil {
		return nil
	}
	return DB.Where("id = ?", id).Delete(&StudioSkill{}).Error
}
