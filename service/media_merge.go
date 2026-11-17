// Server-side video merge with ffmpeg.
//
// The studio records short parts and, before handing them to the user, asks
// this service to stitch them into a single file. Doing it server-side keeps
// Aepi media URLs on the server (parts may expire upstream), produces one
// fast-start H.264 file that plays progressively everywhere, and lets us
// register the result against the user's storage quota like any other video.
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
	"os/exec"
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
	hasAudio  bool
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
// H.264+AAC MP4 stored under uploads/merged/. Returns the public URL.
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
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return "", errors.New("ffmpeg is not available on this server; install ffmpeg to enable merged videos")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		return "", errors.New("ffprobe is not available on this server; install ffmpeg to enable merged videos")
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

	for index, raw := range urls {
		path, isTemp, err := resolveMergePart(ctx, raw, userID, index)
		if err != nil {
			return "", err
		}
		hasAudio, err := ffprobeHasAudio(ctx, path)
		if err != nil {
			return "", fmt.Errorf("failed to inspect part %d: %w", index, err)
		}
		parts = append(parts, mergePart{localPath: path, temp: isTemp, hasAudio: hasAudio})
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

	// Pass 1: normalize every part to h264/yuv420p/aac/44.1kHz with a silent
	// audio track when the part has none, and force square pixels.
	normalized := make([]string, 0, len(parts))
	for i, part := range parts {
		normPath := filepath.Join(videoMergeTempDir, fmt.Sprintf("%s_norm_%d.mp4", nameSuffix, i))
		args := []string{}
		if part.hasAudio {
			args = append(args, "-i", part.localPath)
		} else {
			args = append(args, "-i", part.localPath, "-f", "lavfi", "-i", "anullsrc=channel_layout=stereo:sample_rate=44100")
		}
		vmap := []string{"-map", "0:v"}
		if !part.hasAudio {
			vmap = append(vmap, "-map", "1:a")
		} else {
			vmap = append(vmap, "-map", "0:a")
		}
		args = append(args, vmap...)
		args = append(args, "-c:v", "libx264", "-preset", "veryfast", "-pix_fmt", "yuv420p", "-vf", "setsar=1")
		args = append(args, "-c:a", "aac", "-af", "aresample=44100", "-shortest", "-y", normPath)
		if err := runFFmpegCtx(ctx, args...); err != nil {
			return "", fmt.Errorf("failed to normalize part %d: %w", i, err)
		}
		normalized = append(normalized, normPath)
	}

	// Pass 2: lossless concat of identical-parameter parts.
	concatPath := filepath.Join(videoMergeTempDir, nameSuffix+"_concat.txt")
	concatBody := strings.Builder{}
	for _, p := range normalized {
		concatBody.WriteString("file '")
		concatBody.WriteString(strings.ReplaceAll(absPath(p), "'", "'\\''"))
		concatBody.WriteString("'\n")
	}
	if err := os.WriteFile(concatPath, []byte(concatBody.String()), 0o600); err != nil {
		return "", err
	}
	concatArgs := []string{"-f", "concat", "-safe", "0", "-i", concatPath, "-c", "copy", "-movflags", "+faststart", "-y", outPath}
	if err := runFFmpegCtx(ctx, concatArgs...); err != nil {
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

func absPath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return abs
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
			cfgOwner := int(0)
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

func ffprobeHasAudio(ctx context.Context, path string) (bool, error) {
	cmd := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-select_streams", "a:0", "-show_entries", "stream=codec_type", "-of", "csv=p=0", path)
	out, err := cmd.Output()
	if err != nil {
		// No audio stream is the common case for silent generations.
		return false, nil
	}
	return strings.Contains(strings.TrimSpace(string(out)), "audio"), nil
}

func runFFmpegCtx(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		s := stderr.String()
		if len(s) > 800 {
			s = s[len(s)-800:]
		}
		return fmt.Errorf("ffmpeg: %s", s)
	}
	return nil
}
