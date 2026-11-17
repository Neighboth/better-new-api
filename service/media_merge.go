// Server-side video merge without external tools.
//
// The studio records short parts and, before handing them to the user, asks
// this service to stitch them into a single file. Doing it server-side keeps
// Aepi media URLs on the server (parts may expire upstream) and produces one
// fast-start H.264 file that plays everywhere; the result is registered
// against the user's storage quota like any other video.
//
// The parts are always produced by the same model on the same engine, so the
// merged file is built by a pure-Go remuxer (sample tables are rebuilt, no
// re-encode) — it needs no ffmpeg on the host. When the parts' codec
// configurations diverge the endpoint fails fast and the client falls back to
// in-browser merging.
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	operation_setting "github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/QuantumNous/new-api/model"
)

const (
	mergedMediaMaxParts = 12
	mergedMediaMaxSize  = int64(600) << 20 // 600 MB result cap
	mediaMergeFilePerm  = 0o644
)

var videoMergeTempDir = "/tmp/better-new-api-merge"

type mergePart struct {
	localPath string
	temp      bool
}

// taskIDStub returns a random hex string for asset file names.
func randHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// MergeVideosLocalOrRemote merges remote/same-origin/local videos into a single
// MP4 stored under uploads/merged/. Returns the public URL.
func MergeVideos(ctx context.Context, userID int, urls []string) (string, error) {
	if userID <= 0 {
		return "", errors.New("authentication required")
	}
	if len(urls) == 0 {
		return "", errors.New("no parts to merge")
	}
	if len(urls) > mergedMediaMaxParts {
		return "", fmt.Errorf("too many parts (max %d)", mergedMediaMaxParts)
	}

	if err := os.MkdirAll(videoMergeTempDir, 0o755); err != nil {
		return "", err
	}

	parts := make([]mergePart, 0, len(urls))
	cleanup := func() {
		for _, part := range parts {
			if part.temp {
				_ = os.Remove(part.localPath)
			}
		}
	}
	defer cleanup()

	localPaths := make([]string, 0, len(urls))
	for index, raw := range urls {
		path, isTemp, err := resolveMergePart(ctx, raw, userID, index)
		if err != nil {
			return "", err
		}
		parts = append(parts, mergePart{localPath: path, temp: isTemp})
		localPaths = append(localPaths, path)
	}

	if err := os.MkdirAll("uploads/merged", 0o755); err != nil {
		return "", err
	}
	nameSuffix, err := randHex(6)
	if err != nil {
		return "", err
	}
	outName := fmt.Sprintf("%d_%d_%s.mp4", userID, time.Now().UnixNano(), nameSuffix)
	outPath := filepath.Join("uploads/merged", outName)

	if err := ConcatMP4Files(localPaths, outPath); err != nil {
		_ = os.Remove(outPath)
		// Incompatible or corrupt parts — the client falls back to
		// in-browser merging on this error.
		return "", fmt.Errorf("failed to stitch parts: %w", err)
	}

	fi, err := os.Stat(outPath)
	if err != nil || fi.Size() == 0 || fi.Size() > mergedMediaMaxSize {
		_ = os.Remove(outPath)
		return "", fmt.Errorf("merged file exceeds the maximum size of %d bytes", mergedMediaMaxSize)
	}
	now := time.Now().Unix()
	ret := operation_setting.VideoRetentionDays()
	asset := &model.VideoAsset{
		UserId:    userID,
		TaskId:    fmt.Sprintf("merged_%s", nameSuffix),
		FileName:  outName,
		Size:      fi.Size(),
		Kind:      "merged",
		CreatedAt: now,
		ExpiresAt: now + int64(ret)*86400,
	}
	if err := model.DB.Create(asset).Error; err != nil {
		_ = os.Remove(outPath)
		return "", err
	}
	_ = os.Chmod(outPath, mediaMergeFilePerm)
	return "/uploads/merged/" + outName, nil
}

func resolveMergePart(ctx context.Context, raw string, userID int, index int) (string, bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false, errors.New("empty part url")
	}

	// Same-origin local references (served by this instance).
	if strings.HasPrefix(raw, "/uploads/merged/") {
		if !validMergedPath(raw, userID) {
			return "", false, errors.New("merged file does not belong to you or is not recognized")
		}
		return strings.TrimPrefix(raw, "/"), false, nil
	}
	if strings.HasPrefix(raw, "/uploads/videos/") {
		parts := strings.Split(strings.TrimPrefix(raw, "/uploads/videos/"), "/")
		if len(parts) == 2 {
			cfgOwner := 0
			_, _ = fmt.Sscan(parts[0], &cfgOwner)
			if cfgOwner != userID {
				return "", false, errors.New("part does not belong to you")
			}
			name := parts[1]
			if strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
				return "", false, errors.New("invalid file name")
			}
			return filepath.Join("uploads", "videos", parts[0], name), false, nil
		}
		return "", false, errors.New("invalid video url")
	}

	// The client-side /api/media/cache proxy: resolve the wrapped URL and fetch it ourselves.
	srcURL := raw
	if strings.HasPrefix(raw, "/api/media/cache?url=") {
		parsed, err := url.Parse(raw)
		if err != nil {
			return "", false, err
		}
		srcURL = parsed.Query().Get("url")
	}
	if !IsHTTPOrHTTPS(srcURL) {
		return "", false, errors.New("unsupported part url")
	}
	if err := ValidateSSRFProtectedFetchURL(srcURL); err != nil {
		return "", false, err
	}
	tmp, err := os.CreateTemp(videoMergeTempDir, fmt.Sprintf("part_%d_*.mp4", index))
	if err != nil {
		return "", false, err
	}
	maxBytes := operation_setting.VideoAssetMaxBytes()
	client := GetSSRFProtectedHTTPClient()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srcURL, nil)
	if err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return "", false, err
	}
	req.Header.Set("User-Agent", "curl/8.4.0")
	resp, err := client.Do(req)
	if err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return "", false, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return "", false, fmt.Errorf("part %d: upstream returned HTTP %d", index, resp.StatusCode)
	}
	limited := io.LimitReader(resp.Body, maxBytes+1)
	written, copyErr := io.Copy(tmp, limited)
	closeErr := tmp.Close()
	resp.Body.Close()
	if copyErr != nil || closeErr != nil || written == 0 || written > maxBytes {
		_ = os.Remove(tmp.Name())
		return "", false, fmt.Errorf("part %d: download failed", index)
	}
	return tmp.Name(), true, nil
}

func validMergedPath(raw string, userID int) bool {
	name := strings.TrimPrefix(raw, "/uploads/merged/")
	if name == "" || strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return false
	}
	var owner int
	if _, err := fmt.Sscan(strings.SplitN(name, "_", 2)[0], &owner); err != nil {
		return false
	}
	return owner == userID
}
