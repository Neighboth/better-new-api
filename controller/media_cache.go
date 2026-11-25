package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const (
	legacyMediaCacheDir = "uploads/media_cache"
)

func getMediaCacheKey(rawURL string) (string, string) {
	parsed, err := url.Parse(rawURL)
	ext := ".mp4"
	if err == nil {
		pathExt := filepath.Ext(parsed.Path)
		if pathExt != "" && len(pathExt) <= 6 {
			ext = strings.ToLower(pathExt)
		}
	}
	hash := sha256.Sum256([]byte(rawURL))
	return hex.EncodeToString(hash[:]), ext
}

func setMediaCacheHeaders(c *gin.Context) {
	c.Header("Cache-Control", "public, max-age=31536000, immutable, no-transform")
	c.Header("Accept-Ranges", "bytes")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Expose-Headers", "Content-Range, Content-Length, X-Transcode-Status, X-Transcode-Error")
	c.Header("X-Accel-Buffering", "no")
	c.Header("Content-Encoding", "identity")
}

// resolveLocalUploadPath returns the cleaned path under uploads/ if rawPath
// points to an existing non-empty file strictly within uploads/.
func resolveLocalUploadPath(rawPath string) (string, bool) {
	cleaned := filepath.Clean(strings.TrimPrefix(rawPath, "/"))
	absUploads, err1 := filepath.Abs("uploads")
	absTarget, err2 := filepath.Abs(cleaned)
	if err1 != nil || err2 != nil {
		return "", false
	}
	if !strings.HasPrefix(absTarget, absUploads+string(filepath.Separator)) {
		return "", false
	}
	if info, statErr := os.Stat(cleaned); statErr == nil && !info.IsDir() && info.Size() > 0 {
		return cleaned, true
	}
	return "", false
}

// resolveMediaInput resolves a relative URL, video proxy route, local upload or
// validated remote URL to an input source for serving or live ffmpeg streaming
// WITHOUT writing any extra cached copies to disk.
func resolveMediaInput(rawURL string) (source string, isLocalFile bool, err error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", false, fmt.Errorf("url is empty")
	}

	// 1. Direct local upload (e.g. /uploads/videos/...)
	if strings.HasPrefix(rawURL, "/uploads/") || strings.HasPrefix(rawURL, "uploads/") {
		if localFile, ok := resolveLocalUploadPath(rawURL); ok {
			return localFile, true, nil
		}
	}

	// 2. Full URL pointing to local /uploads/ or /v1/videos/:id/content
	if parsed, parseErr := url.Parse(rawURL); parseErr == nil {
		if strings.HasPrefix(parsed.Path, "/uploads/") {
			if localFile, ok := resolveLocalUploadPath(parsed.Path); ok {
				return localFile, true, nil
			}
		}

		if strings.Contains(parsed.Path, "/v1/videos/") && strings.Contains(parsed.Path, "/content") {
			parts := strings.Split(parsed.Path, "/")
			for i, p := range parts {
				if p == "videos" && i+1 < len(parts) {
					taskID := parts[i+1]
					var task model.Task
					if dbErr := model.DB.Where("task_id = ?", taskID).First(&task).Error; dbErr == nil {
						resURL := task.GetResultURL()
						if resURL != "" && resURL != rawURL {
							return resolveMediaInput(resURL)
						}
					}
					break
				}
			}
		}
	}

	// 3. Remote HTTP/HTTPS URL — validated against SSRF, never cached to disk
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return "", false, fmt.Errorf("invalid URL format: %s", rawURL)
	}
	if ssrfErr := service.ValidateSSRFProtectedFetchURL(rawURL); ssrfErr != nil {
		return "", false, fmt.Errorf("access to URL blocked: %w", ssrfErr)
	}
	return rawURL, false, nil
}

// GetCachedMedia handles GET /api/media/cache?url=...&height=...&fps=...
// Only the original video file is stored on disk; any requested quality or FPS
// variant is transcoded live on the fly and streamed directly to the client
// without saving copies in media_cache.
func GetCachedMedia(c *gin.Context) {
	rawURL := strings.TrimSpace(c.Query("url"))
	if rawURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url parameter is required"})
		return
	}

	heightParam := c.Query("height")
	if heightParam == "" {
		heightParam = c.Query("h")
	}
	fpsParam := c.Query("fps")
	targetHeight, _ := strconv.Atoi(heightParam)
	targetFps, _ := strconv.Atoi(fpsParam)
	if targetHeight > 0 {
		if targetHeight < 144 {
			targetHeight = 144
		} else if targetHeight > 4320 {
			targetHeight = 4320
		}
	}
	if targetFps > 0 {
		if targetFps < 1 {
			targetFps = 1
		} else if targetFps > 120 {
			targetFps = 120
		}
	}

	inputSource, isLocalFile, err := resolveMediaInput(rawURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Live on-the-fly transcoding when quality (height) or FPS is requested:
	// zero disk writes, zero media_cache growth.
	if targetHeight > 0 || targetFps > 0 {
		if err := streamTranscodedMedia(c, inputSource, targetHeight, targetFps); err != nil {
			common.SysError(fmt.Sprintf("live media transcode failed (%v), falling back to original: %s", err, inputSource))
			if !c.Writer.Written() {
				c.Header("X-Transcode-Status", "failed")
				c.Header("X-Transcode-Error", err.Error())
				c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
				if isLocalFile {
					c.File(inputSource)
				} else {
					streamMediaUpstream(c, inputSource, true, c.GetHeader("Range") != "")
				}
			}
		}
		return
	}

	// Original media (no transcoding): serve local original file or stream remote upstream directly.
	if isLocalFile {
		setMediaCacheHeaders(c)
		c.File(inputSource)
		return
	}
	streamMediaUpstream(c, inputSource, true, c.GetHeader("Range") != "")
}

// streamTranscodedMedia runs ffmpeg and pipes fragmented MP4 directly to the
// HTTP response without creating any temporary or cached files on disk.
func streamTranscodedMedia(c *gin.Context, inputSource string, height int, fps int) error {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 180*time.Second)
	defer cancel()

	ffmpegBin, err := service.EnsureFFmpeg(ctx)
	if err != nil || ffmpegBin == "" {
		return fmt.Errorf("ffmpeg not available: %w", err)
	}

	var vfParts []string
	if height > 0 {
		vfParts = append(vfParts, fmt.Sprintf("scale=trunc(oh*a/2)*2:%d", height))
	}
	if fps > 0 {
		vfParts = append(vfParts, fmt.Sprintf("fps=%d", fps))
	}

	args := []string{"-loglevel", "error"}
	if strings.HasPrefix(inputSource, "http://") || strings.HasPrefix(inputSource, "https://") {
		args = append(args, "-user-agent", "curl/8.4.0")
	}
	args = append(args, "-i", inputSource)
	if len(vfParts) > 0 {
		args = append(args, "-vf", strings.Join(vfParts, ","))
	}
	args = append(args,
		"-pix_fmt", "yuv420p",
		"-c:v", "libx264",
		"-preset", "ultrafast",
		"-tune", "zerolatency",
		"-crf", "26",
		"-c:a", "aac",
		"-strict", "experimental",
		"-movflags", "frag_keyframe+empty_moov+default_base_moof",
		"-f", "mp4",
		"pipe:1",
	)

	cmd := exec.CommandContext(ctx, ffmpegBin, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("ffmpeg stdout pipe: %w", err)
	}
	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ffmpeg start failed: %w", err)
	}

	// Read first chunk before sending HTTP 200 so startup failures can fall back cleanly.
	firstBuf := make([]byte, 32*1024)
	n, readErr := stdout.Read(firstBuf)
	if n <= 0 && readErr != nil {
		_ = cmd.Wait()
		return fmt.Errorf("ffmpeg produced no output: %v (stderr: %s)", readErr, strings.TrimSpace(stderrBuf.String()))
	}

	c.Header("Content-Type", "video/mp4")
	c.Header("Cache-Control", "no-store")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Expose-Headers", "X-Transcode-Status, X-Transcode-Error")
	c.Header("X-Accel-Buffering", "no")
	c.Header("X-Transcode-Status", "live-stream")
	c.Writer.WriteHeader(http.StatusOK)

	if _, writeErr := c.Writer.Write(firstBuf[:n]); writeErr != nil {
		cancel()
		_ = cmd.Wait()
		return nil
	}
	c.Writer.Flush()

	buf := make([]byte, 32*1024)
	for {
		rn, rErr := stdout.Read(buf)
		if rn > 0 {
			if _, wErr := c.Writer.Write(buf[:rn]); wErr != nil {
				cancel()
				break
			}
			c.Writer.Flush()
		}
		if rErr == io.EOF {
			break
		}
		if rErr != nil {
			break
		}
	}
	_ = cmd.Wait()
	return nil
}

// streamMediaUpstream proxies the upstream verbatim and never touches disk.
func streamMediaUpstream(c *gin.Context, rawURL string, _ bool, forwardRange bool) {
	client := service.GetSSRFProtectedHTTPClient()
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, rawURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request: " + err.Error()})
		return
	}
	req.Header.Set("User-Agent", "curl/8.4.0")
	if forwardRange {
		if clientRange := c.GetHeader("Range"); clientRange != "" {
			req.Header.Set("Range", clientRange)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to fetch media: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("Upstream returned HTTP %d", resp.StatusCode)})
		return
	}

	copyUpstreamHeaders(c, resp)
	setMediaCacheHeaders(c)
	c.Writer.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(c.Writer, resp.Body)
}

func copyUpstreamHeaders(c *gin.Context, resp *http.Response) {
	for _, header := range []string{
		"Content-Type",
		"Content-Length",
		"Content-Range",
		"Accept-Ranges",
		"Cache-Control",
		"ETag",
		"Last-Modified",
	} {
		if value := resp.Header.Get(header); value != "" {
			c.Header(header, value)
		}
	}
}

// PreCacheMedia handles POST /api/media/cache {"url": "https://..."}
// No-op disk write: returns the live proxy URL directly without creating media_cache files.
func PreCacheMedia(c *gin.Context) {
	var body struct {
		URL string `json:"url"`
	}
	if err := common.DecodeJson(c.Request.Body, &body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid request body"})
		return
	}

	rawURL := strings.TrimSpace(body.URL)
	if rawURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "url is required"})
		return
	}
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Only HTTP/HTTPS URLs are allowed"})
		return
	}
	if err := service.ValidateSSRFProtectedFetchURL(rawURL); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	cachedURL := fmt.Sprintf("/api/media/cache?url=%s", url.QueryEscape(rawURL))
	c.JSON(http.StatusOK, gin.H{"success": true, "cached_url": cachedURL, "cached": false})
}

// DeleteCachedMedia handles DELETE /api/media/cache?url=...
// Cleans any legacy media_cache files if present from earlier server versions.
func DeleteCachedMedia(c *gin.Context) {
	rawURL := strings.TrimSpace(c.Query("url"))
	if rawURL == "" {
		var body struct {
			URL string `json:"url"`
		}
		if err := common.DecodeJson(c.Request.Body, &body); err == nil && body.URL != "" {
			rawURL = strings.TrimSpace(body.URL)
		}
	}
	if rawURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "url is required"})
		return
	}

	hash, ext := getMediaCacheKey(rawURL)
	fileName := hash + ext
	if entries, err := os.ReadDir(legacyMediaCacheDir); err == nil {
		for _, entry := range entries {
			name := entry.Name()
			if strings.HasPrefix(name, hash+"_") || strings.HasPrefix(name, fileName+".") || name == fileName {
				_ = os.Remove(filepath.Join(legacyMediaCacheDir, name))
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "media cache deleted"})
}

type MediaProbeResult struct {
	Width    int     `json:"width"`
	Height   int     `json:"height"`
	FPS      float64 `json:"fps"`
	Duration float64 `json:"duration"`
}

var mediaProbeCache sync.Map

// ProbeMediaInfo returns video resolution, fps, and duration using ffprobe
// without storing any extra copies on disk.
func ProbeMediaInfo(c *gin.Context) {
	rawURL := strings.TrimSpace(c.Query("url"))
	if rawURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url parameter is required"})
		return
	}

	if val, ok := mediaProbeCache.Load(rawURL); ok {
		c.JSON(http.StatusOK, val)
		return
	}

	inputSource, _, err := resolveMediaInput(rawURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	args := []string{
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height,r_frame_rate,duration",
		"-of", "json",
	}
	if strings.HasPrefix(inputSource, "http://") || strings.HasPrefix(inputSource, "https://") {
		args = append(args, "-user-agent", "curl/8.4.0")
	}
	args = append(args, inputSource)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	ffprobeBin := service.FindFFprobe()
	if ffprobeBin == "" {
		c.JSON(http.StatusOK, MediaProbeResult{
			Width:    1280,
			Height:   720,
			FPS:      30,
			Duration: 0,
		})
		return
	}

	cmd := exec.CommandContext(ctx, ffprobeBin, args...)
	out, err := cmd.Output()
	if err != nil {
		c.JSON(http.StatusOK, MediaProbeResult{
			Width:    1280,
			Height:   720,
			FPS:      30,
			Duration: 0,
		})
		return
	}

	var parsed struct {
		Streams []struct {
			Width      int    `json:"width"`
			Height     int    `json:"height"`
			RFrameRate string `json:"r_frame_rate"`
			Duration   string `json:"duration"`
		} `json:"streams"`
	}
	if err := common.Unmarshal(out, &parsed); err != nil || len(parsed.Streams) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse probe data"})
		return
	}

	stream := parsed.Streams[0]
	fps := parseFrameRate(stream.RFrameRate)
	dur, _ := strconv.ParseFloat(stream.Duration, 64)

	res := MediaProbeResult{
		Width:    stream.Width,
		Height:   stream.Height,
		FPS:      fps,
		Duration: dur,
	}
	mediaProbeCache.Store(rawURL, res)
	c.JSON(http.StatusOK, res)
}

func parseFrameRate(r string) float64 {
	parts := strings.Split(r, "/")
	if len(parts) == 2 {
		num, err1 := strconv.ParseFloat(parts[0], 64)
		den, err2 := strconv.ParseFloat(parts[1], 64)
		if err1 == nil && err2 == nil && den != 0 {
			return math.Round((num/den)*100) / 100
		}
	} else if len(parts) == 1 {
		num, err := strconv.ParseFloat(parts[0], 64)
		if err == nil {
			return num
		}
	}
	return 30
}
