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
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
)

const (
	mergedMediaDir     = "uploads/merged"
	maxMergedMediaSize = int64(400) << 20 // 400 MB
)

var mergedMediaRandomSegment = regexp.MustCompile(`^[A-Za-z0-9]{6,}$`)

func validMergedMediaName(name string) bool {
	if name == "" {
		return false
	}
	if strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		// path traversal
		return false
	}
	base := filepath.Base(name)
	if base != name {
		return false
	}
	ext := strings.ToLower(filepath.Ext(name))
	if ext != ".webm" && ext != ".mp4" {
		return false
	}
	parts := strings.SplitN(name, "_", 3)
	if len(parts) != 3 {
		return false
	}
	ownerIdText, tsText, randomWithExt := parts[0], parts[1], parts[2]
	if _, err := strconv.Atoi(ownerIdText); err != nil {
		return false
	}
	if _, err := strconv.ParseInt(tsText, 10, 64); err != nil {
		return false
	}
	stem := strings.TrimSuffix(randomWithExt, ext)
	return mergedMediaRandomSegment.MatchString(stem)
}

// UploadMergedMedia stores a merged video produced in the browser (the
// studio chains every generated part into one file). The file is served
// afterwards through the static /uploads mount.
func UploadMergedMedia(c *gin.Context) {
	userId := c.GetInt("id")
	if userId <= 0 {
		common.ApiError(c, fmt.Errorf("authentication required"))
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		common.ApiErrorMsg(c, "no file uploaded")
		return
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext != ".webm" && ext != ".mp4" {
		common.ApiErrorMsg(c, "unsupported file format (webm or mp4 only)")
		return
	}
	if file.Size > maxMergedMediaSize {
		common.ApiErrorMsg(c, "file is too large (400 MB max)")
		return
	}

	if err := os.MkdirAll(mergedMediaDir, 0o755); err != nil {
		common.ApiError(c, err)
		return
	}

	src, err := file.Open()
	if err != nil {
		common.ApiError(c, err)
		return
	}

	name := fmt.Sprintf("%d_%d_%s%s", userId, time.Now().UnixNano(), common.GetRandomString(8), ext)
	dstPath := filepath.Join(mergedMediaDir, name)
	dst, err := os.Create(dstPath)
	if err != nil {
		_ = src.Close()
		common.ApiError(c, err)
		return
	}

	written, copyErr := io.Copy(dst, src)
	syncErr := dst.Sync()
	closeErr := dst.Close()
	_ = src.Close()
	switch {
	case copyErr != nil:
		_ = os.Remove(dstPath)
		common.ApiError(c, copyErr)
		return
	case syncErr != nil:
		_ = os.Remove(dstPath)
		common.ApiError(c, syncErr)
		return
	case closeErr != nil:
		_ = os.Remove(dstPath)
		common.ApiError(c, closeErr)
		return
	}
	if written == 0 {
		_ = os.Remove(dstPath)
		common.ApiErrorMsg(c, "empty file")
		return
	}

	common.ApiSuccess(c, gin.H{
		"url": "/uploads/merged/" + name,
	})
}

// DeleteMergedMedia removes a merged file owned by the caller.
func DeleteMergedMedia(c *gin.Context) {
	userId := c.GetInt("id")
	if userId <= 0 {
		common.ApiError(c, fmt.Errorf("authentication required"))
		return
	}
	name := strings.TrimSpace(c.Query("name"))
	if name == "" {
		var body struct {
			Name string `json:"name"`
		}
		if err := common.DecodeJson(c.Request.Body, &body); err != nil {
			common.ApiErrorMsg(c, "invalid request body")
			return
		}
		name = strings.TrimSpace(body.Name)
	}
	if !validMergedMediaName(name) {
		common.ApiErrorMsg(c, "invalid file name")
		return
	}
	ownerPart := strings.SplitN(name, "_", 2)[0]
	ownerId, err := strconv.Atoi(ownerPart)
	if err != nil || ownerId != userId {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "not your file"})
		return
	}
	if err := os.Remove(filepath.Join(mergedMediaDir, name)); err != nil && !os.IsNotExist(err) {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{})
}


