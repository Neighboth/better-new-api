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

package controller

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// ListStudioSkills returns the enabled skills for the studio picker.
func ListStudioSkills(c *gin.Context) {
	skills, err := model.ListStudioSkills()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, skills)
}

// ListStudioSkillsAdmin returns every skill for the admin console.
func ListStudioSkillsAdmin(c *gin.Context) {
	skills, err := model.ListStudioSkillsAdmin()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, skills)
}

// CreateStudioSkill creates a skill (admin only).
func CreateStudioSkill(c *gin.Context) {
	var body struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Image       string `json:"image"`
		Content     string `json:"content"`
		SortOrder   int    `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		common.ApiErrorMsg(c, "Invalid request body")
		return
	}
	skill, err := model.CreateStudioSkill(body.Title, body.Description, body.Image, body.Content, body.SortOrder, common.GetTimestamp())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	common.ApiSuccess(c, skill)
}

// UpdateStudioSkill applies a partial update to a skill (admin only).
func UpdateStudioSkill(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		common.ApiErrorMsg(c, "Missing skill id")
		return
	}
	var patch map[string]interface{}
	if err := c.ShouldBindJSON(&patch); err != nil {
		common.ApiErrorMsg(c, "Invalid request body")
		return
	}
	if err := model.UpdateStudioSkill(id, patch, common.GetTimestamp()); err != nil {
		common.ApiError(c, err)
		return
	}
	skill, err := model.GetStudioSkill(id)
	if err != nil {
		common.ApiSuccess(c, gin.H{})
		return
	}
	common.ApiSuccess(c, skill)
}

// DeleteStudioSkill removes a skill (admin only).
func DeleteStudioSkill(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		common.ApiErrorMsg(c, "Missing skill id")
		return
	}
	if err := model.DeleteStudioSkill(id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{})
}

// UploadStudioSkillImage stores one skill image under uploads/skills/ and
// returns its public URL (admin only).
func UploadStudioSkillImage(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		common.ApiErrorMsg(c, "No file uploaded: "+err.Error())
		return
	}
	if file.Size > 2<<20 {
		common.ApiErrorMsg(c, "The image is too large (2 MB max)")
		return
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" && ext != ".svg" {
		common.ApiErrorMsg(c, "Unsupported file format (jpg, jpeg, png, webp, svg)")
		return
	}
	if err := os.MkdirAll(filepath.Join("uploads", "skills"), 0o755); err != nil {
		common.ApiError(c, err)
		return
	}
	fileName := fmt.Sprintf("%d_%s%s", time.Now().UnixNano(), common.GetRandomString(6), ext)
	if err := c.SaveUploadedFile(file, filepath.Join("uploads", "skills", fileName)); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"url": "/uploads/skills/" + fileName,
	})
}
