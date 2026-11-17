package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	operation_setting "github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/QuantumNous/new-api/setting/system_setting"
)

const (
	videoUploadsDir   = "uploads/videos"
	videoAssetDirPerm = 0o755
)

// ErrVideoStorageQuotaExceeded is returned by StoreVideoFromURL when the user
// is over their per-user video storage cap. The caller records the upstream
// URL instead — the frontend imports the video into the browser's IndexedDB,
// and generation is never blocked by a full storage bucket.
var ErrVideoStorageQuotaExceeded = errors.New("video storage quota exceeded")

// VideoMaxQuotaBytes exposes the per-user cap for callers that render quota UI.
func VideoMaxQuotaBytes() int64 { return operation_setting.VideoStorageQuotaBytes() }

// VideoStorageStats reports the user's usage for the quota bar.
func VideoStorageStats(userID int) (used int64, limit int64, count int64, err error) {
	if userID <= 0 {
		return 0, 0, 0, errors.New("invalid user")
	}
	limit = operation_setting.VideoStorageQuotaBytes()
	used, err = model.ActiveVideoStorageBytes(userID)
	if err != nil {
		return 0, limit, 0, err
	}
	cnt, err := model.ActiveVideoStorageCount(userID)
	if err != nil {
		return used, limit, 0, err
	}
	return used, limit, cnt, nil
}

// CheckVideoStorageQuota reports whether the user already sits on their storage
// limit. Admins are exempt. Kept for callers that want the current state;
// generation is never blocked by this now — the store path simply skips
// server-side storage and the frontend falls back to IndexedDB.
func CheckVideoStorageQuota(userID int) *types.NewAPIError {
	if model.IsAdmin(userID) {
		return nil
	}
	limit := operation_setting.VideoStorageQuotaBytes()
	used, err := model.ActiveVideoStorageBytes(userID)
	if err != nil {
		common.SysError("video storage usage query failed: " + err.Error())
		return nil // do not break generation on a metering failure
	}
	if used < limit {
		return nil
	}
	return types.NewErrorWithStatusCode(
		fmt.Errorf("Video storage quota exhausted (%.1f GB of %.1f GB used). New videos will be kept in this browser instead of the server.", float64(used)/float64(1<<30), float64(limit)/float64(1<<30)),
		types.ErrorCodeInsufficientUserQuota,
		http.StatusForbidden,
	)
}

// IsVideoStorageQuotaExhausted reports whether the user's server-side video
// storage is at or over the cap (admins are exempt). Called by the store path.
func IsVideoStorageQuotaExhausted(userID int) bool {
	if model.IsAdmin(userID) {
		return false
	}
	limit := operation_setting.VideoStorageQuotaBytes()
	used, err := model.ActiveVideoStorageBytes(userID)
	if err != nil {
		common.SysError("video storage usage query failed: " + err.Error())
		return false // do not break generation on a metering failure
	}
	return used >= limit
}

// StoreVideoFromURL downloads an upstream media URL to local disk and records it
// against the user's quota. Returns the local public URL. When the user is on or
// over their quota the store is skipped and ErrVideoStorageQuotaExceeded is
// returned so the caller keeps the upstream URL instead — generation itself is
// never blocked.
func StoreVideoFromURL(ctx context.Context, userID int, taskID string, upstreamURL string, kind string) (string, error) {
	if userID <= 0 || upstreamURL == "" {
		return "", errors.New("invalid storage request")
	}
	if IsVideoStorageQuotaExhausted(userID) {
		return "", ErrVideoStorageQuotaExceeded
	}
	ext := videoAssetExtFromURL(upstreamURL)

	if !IsHTTPOrHTTPS(upstreamURL) {
		return "", fmt.Errorf("unsupported video URL scheme")
	}
	if err := ValidateSSRFProtectedFetchURL(upstreamURL); err != nil {
		return "", err
	}
	head, err := headContentLength(ctx, upstreamURL)
	maxBytes := operation_setting.VideoAssetMaxBytes()
	if err == nil && head > maxBytes {
		return "", fmt.Errorf("video exceeds the maximum asset size of %d bytes", maxBytes)
	}
	client := GetSSRFProtectedHTTPClient()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstreamURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "curl/8.4.0")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("upstream returned HTTP %d", resp.StatusCode)
	}

	if kind == "" {
		kind = "video"
	}
	fileName := fmt.Sprintf("%d_%d_%s.%s", userID, time.Now().UnixNano(), common.GetRandomString(8), ext)
	dir := filepath.Join("uploads", "videos", fmt.Sprint(userID))
	if err := os.MkdirAll(dir, videoAssetDirPerm); err != nil {
		return "", err
	}
	dstPath := filepath.Join(dir, fileName)
	out, err := os.Create(dstPath)
	if err != nil {
		return "", err
	}
	limited := io.LimitReader(resp.Body, maxBytes+1)
	written, copyErr := io.Copy(out, limited)
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil || written == 0 || written > maxBytes {
		_ = os.Remove(dstPath)
		if copyErr != nil {
			return "", copyErr
		}
		if written > maxBytes {
			return "", fmt.Errorf("video exceeds the maximum asset size of %d bytes", maxBytes)
		}
		return "", fmt.Errorf("empty video download")
	}
	now := time.Now().Unix()
	ret := operation_setting.VideoRetentionDays()
	expiresAt := now + int64(ret)*86400

	asset := &model.VideoAsset{
		UserId:    userID,
		TaskId:    taskID,
		FileName:  fileName,
		Size:      written,
		Kind:      kind,
		CreatedAt: now,
		ExpiresAt: expiresAt,
	}
	if err := model.DB.Create(asset).Error; err != nil {
		_ = os.Remove(dstPath)
		return "", err
	}
	return "/" + filepath.ToSlash(dstPath), nil
}

// DeleteOldestVideoAssets deletes the user's oldest stored videos until at least
// minBytes are freed (or none remain). It returns freed bytes, delete count and
// the public URLs of every removed file so callers can purge matching studio
// items from their lists. The stored video_assets rows and the studio_items rows
// referencing the same file are removed together — storage, history and the
// popup can never disagree again.
func DeleteOldestVideoAssets(userID int, minBytes int64) (freed int64, deleted int, deletedURLs []string, err error) {
	for {
		var asset model.VideoAsset
		err = model.DB.Where("user_id = ? AND deleted = ?", userID, false).
			Order("expires_at ASC, id ASC").
			First(&asset).Error
		if err != nil {
			// Nothing left to delete — report what was already removed.
			return freed, deleted, deletedURLs, nil
		}
		// storedAssetLocation resolves a stored asset to disk path and public URL.
		path, url := storedAssetLocation(userID, asset)
		fileName := asset.FileName
		removeErr := os.Remove(path)
		if removeErr != nil && !os.IsNotExist(removeErr) {
			// Do not mark the row deleted or bill the user for space that is
			// still occupied — surface the failure instead.
			return freed, deleted, deletedURLs, fmt.Errorf("failed to delete the stored video %s: %w", path, removeErr)
		}
		if selectorErr := removeStudioItemsForFile(userID, fileName); selectorErr != nil {
			common.SysError(fmt.Sprintf("video storage: studio items for %s were not purged: %s", fileName, selectorErr.Error()))
		}
		if markerErr := model.DB.Model(&asset).Updates(map[string]interface{}{
			"deleted":        true,
			"deleted_reason": "user_delete",
		}).Error; markerErr != nil {
			return freed, deleted, deletedURLs, fmt.Errorf("failed to mark the stored video as deleted: %w", markerErr)
		}
		freed += asset.Size
		deleted++
		deletedURLs = append(deletedURLs, url)
		if freed >= minBytes {
			return freed, deleted, deletedURLs, nil
		}
	}
}

// storedAssetLocation resolves a stored asset to (diskPath, publicURL).
func storedAssetLocation(userID int, asset model.VideoAsset) (diskPath string, publicURL string) {
	switch asset.Kind {
	case "merged":
		return filepath.Join("uploads", "merged", asset.FileName), "/uploads/merged/" + asset.FileName
	default:
		directory := fmt.Sprint(userID)
		return filepath.Join(videoUploadsDir, directory, asset.FileName), "/uploads/videos/" + directory + "/" + asset.FileName
	}
}

// removeStudioItemsForFile removes studio_items rows of the user whose payload
// references the given stored file name. Payloads embed /uploads/... URLs, so a
// LIKE on the file name (unique fingerprint suffix) finds them reliably.
func removeStudioItemsForFile(userID int, fileName string) error {
	needle := "%" + fileName + "%"
	return model.DB.Where("user_id = ? AND payload LIKE ?", userID, needle).
		Delete(&model.StudioItem{}).Error
}

func headContentLength(ctx context.Context, url string) (int64, error) {
	client := GetSSRFProtectedHTTPClient()
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "curl/8.4.0")
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("upstream returned HTTP %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Length")
	if ct == "" {
		return 0, nil
	}
	var n int64
	_, err = fmt.Sscan(ct, &n)
	return n, err
}

func videoAssetExtFromURL(url string) string {
	lower := strings.ToLower(url)
	switch {
	case strings.Contains(lower, ".webm"):
		return "webm"
	case strings.Contains(lower, ".mov"):
		return "mov"
	case strings.Contains(lower, ".mkv"):
		return "mkv"
	default:
		return "mp4"
	}
}

// IsHTTPOrHTTPS reports whether the URL uses http or https.
func IsHTTPOrHTTPS(url string) bool {
	return strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")
}

func StartVideoExpirySweeper() {
	go func() {
		for {
			sweepVideoAssets()
			time.Sleep(1 * time.Hour)
		}
	}()
}

func sweepVideoAssets() {
	now := time.Now().Unix()
	theWarnBefore := operation_setting.VideoExpiryWarnDaysBefore()
	warnHorizon := now + int64(theWarnBefore)*86400

	var expiring []model.VideoAsset
	model.DB.Where("warn_sent = ? AND deleted = ? AND expires_at <= ? AND expires_at > ?", false, false, warnHorizon, now).
		Find(&expiring)
	for _, asset := range expiring {
		sendVideoExpiryWarning(&asset)
		model.DB.Model(&asset).Update("warn_sent", true)
	}

	var expired []model.VideoAsset
	model.DB.Where("deleted = ? AND expires_at <= ?", false, now).
		Find(&expired)
	for _, asset := range expired {
		var path string
		switch asset.Kind {
		case "merged":
			path = filepath.Join("uploads", "merged", asset.FileName)
		default:
			path = filepath.Join(videoUploadsDir, fmt.Sprint(asset.UserId), asset.FileName)
		}
		_ = os.Remove(path)
		model.DB.Model(&asset).Updates(map[string]interface{}{
			"deleted":        true,
			"deleted_reason": "expired",
		})
	}
}

func sendVideoExpiryWarning(asset *model.VideoAsset) {
	cache, err := model.GetUserCache(asset.UserId)
	if err != nil || cache.Email == "" {
		common.SysLog(fmt.Sprintf("video expiry warning for asset %d: user %d has no email or cannot be read", asset.ID, asset.UserId))
		return
	}
	var userSetting dto.UserSetting
	lang := "en"
	if raw := strings.TrimSpace(cache.Setting); raw != "" {
		if err := common.Unmarshal([]byte(raw), &userSetting); err == nil && strings.TrimSpace(userSetting.Language) != "" {
			lang = NormalizeEmailLang(userSetting.Language)
		}
	}
	expiresAt := time.Unix(asset.ExpiresAt, 0).UTC().Format("2006-01-02")
	base := strings.TrimRight(system_setting.ServerAddress, "/")
	videoURL := base + "/uploads/videos/" + fmt.Sprint(asset.UserId) + "/" + asset.FileName
	subject, content := RenderEmail("video_expiry", lang, map[string]string{
		"username":   cache.Username,
		"video_url":  videoURL,
		"expires_at": expiresAt,
	})
	if err := common.SendEmail(subject, cache.Email, content); err != nil {
		common.SysError(fmt.Sprintf("video expiry warning mail failed for user %d: %s", asset.UserId, err.Error()))
	}
}
