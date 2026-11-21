package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	mediaCacheDir = "uploads/media_cache"
	maxMediaSize  = 150 * 1024 * 1024 // 150MB max cached per media file
)

func init() {
	_ = os.MkdirAll(mediaCacheDir, 0755)
}

// cacheFillLocks serializes concurrent fills of the same cache key.
var cacheFillLocks sync.Map

func lockCacheFill(key string) func() {
	value, _ := cacheFillLocks.LoadOrStore(key, &sync.Mutex{})
	mutex := value.(*sync.Mutex)
	mutex.Lock()
	return mutex.Unlock
}

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

// resolveMediaSource resolves a relative URL, video proxy route, local upload or remote URL
// to an accessible file path on disk (downloading/caching remote files if needed).
func resolveMediaSource(ctx context.Context, rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", fmt.Errorf("url is empty")
	}

	// 1. Check if direct local upload (e.g. /uploads/...)
	if strings.HasPrefix(rawURL, "/uploads/") || strings.HasPrefix(rawURL, "uploads/") {
		localFile := filepath.Clean(strings.TrimPrefix(rawURL, "/"))
		if info, err := os.Stat(localFile); err == nil && info.Size() > 0 {
			return localFile, nil
		}
	}

	// 2. Check if URL contains /uploads/ or /v1/videos/
	if parsed, err := url.Parse(rawURL); err == nil {
		if strings.HasPrefix(parsed.Path, "/uploads/") {
			localFile := filepath.Clean(strings.TrimPrefix(parsed.Path, "/"))
			if info, err := os.Stat(localFile); err == nil && info.Size() > 0 {
				return localFile, nil
			}
		}

		if strings.Contains(parsed.Path, "/v1/videos/") && strings.Contains(parsed.Path, "/content") {
			parts := strings.Split(parsed.Path, "/")
			for i, p := range parts {
				if p == "videos" && i+1 < len(parts) {
					taskID := parts[i+1]
					var task model.Task
					if err := model.DB.Where("task_id = ?", taskID).First(&task).Error; err == nil {
						resURL := task.GetResultURL()
						if resURL != "" && resURL != rawURL {
							return resolveMediaSource(ctx, resURL)
						}
					}
					break
				}
			}
		}
	}

	// 3. For remote or cached HTTP/HTTPS URLs
	hash, ext := getMediaCacheKey(rawURL)
	fileName := hash + ext
	filePath := filepath.Join(mediaCacheDir, fileName)

	// If already cached locally, return existing cache file
	if info, err := os.Stat(filePath); err == nil && info.Size() > 0 {
		return filePath, nil
	}

	// Validate remote URL
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return "", fmt.Errorf("invalid URL format: %s", rawURL)
	}
	if err := service.ValidateSSRFProtectedFetchURL(rawURL); err != nil {
		return "", fmt.Errorf("access to URL blocked: %w", err)
	}

	// Fetch and cache the original file to disk
	unlock := lockCacheFill(fileName)
	defer unlock()

	if info, err := os.Stat(filePath); err == nil && info.Size() > 0 {
		return filePath, nil
	}

	client := service.GetSSRFProtectedHTTPClient()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "curl/8.4.0")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch remote media: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return "", fmt.Errorf("upstream returned status %d", resp.StatusCode)
	}

	tempPath := fmt.Sprintf("%s.%s.tmp", filePath, common.GetRandomString(6))
	out, err := os.Create(tempPath)
	if err != nil {
		return "", fmt.Errorf("failed to create temp cache file: %w", err)
	}

	n, copyErr := copyToTee(out, io.Discard, resp.Body, maxMediaSize)
	_ = out.Close()

	if copyErr != nil || n <= 0 || n > maxMediaSize {
		_ = os.Remove(tempPath)
		return "", fmt.Errorf("failed to stream media or file exceeds size limit")
	}

	if err := os.Rename(tempPath, filePath); err != nil {
		_ = os.Remove(tempPath)
		return "", fmt.Errorf("failed to save cached media: %w", err)
	}

	return filePath, nil
}

// GetCachedMedia handles GET /api/media/cache?url=...
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

	// If quality or FPS transcoding is requested:
	if targetHeight > 0 || targetFps > 0 {
		hash, _ := getMediaCacheKey(rawURL)
		transcodeName := fmt.Sprintf("%s_h%d_fps%d.mp4", hash, targetHeight, targetFps)
		transcodePath := filepath.Join(mediaCacheDir, transcodeName)

		// Fast path: cached transcoded file exists
		if info, err := os.Stat(transcodePath); err == nil && info.Size() > 0 {
			setMediaCacheHeaders(c)
			c.File(transcodePath)
			return
		}

		// Transcode and serve
		unlock := lockCacheFill(transcodeName)
		defer unlock()

		if info, err := os.Stat(transcodePath); err == nil && info.Size() > 0 {
			setMediaCacheHeaders(c)
			c.File(transcodePath)
			return
		}

		inputPath, err := resolveMediaSource(c.Request.Context(), rawURL)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		transcodeCtx, transcodeCancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer transcodeCancel()

		if err := transcodeMediaToFile(transcodeCtx, inputPath, transcodePath, targetHeight, targetFps); err != nil {
			common.SysError(fmt.Sprintf("media transcode failed (%v), falling back to original media: %s", err, inputPath))
			c.Header("X-Transcode-Status", "failed")
			c.Header("X-Transcode-Error", err.Error())
			c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
			// Fallback gracefully to original media so playback never fails
			c.File(inputPath)
			return
		}

		c.Header("X-Transcode-Status", "success")
		setMediaCacheHeaders(c)
		c.File(transcodePath)
		return
	}

	// Normal path (no transcoding):
	inputPath, err := resolveMediaSource(c.Request.Context(), rawURL)
	if err == nil && inputPath != "" {
		if info, err := os.Stat(inputPath); err == nil && info.Size() > 0 {
			setMediaCacheHeaders(c)
			c.File(inputPath)
			return
		}
	}

	// Fallback to streaming proxy
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only HTTP/HTTPS URLs are allowed"})
		return
	}
	nostore := c.Query("nostore") == "1" || strings.EqualFold(c.Query("nostore"), "true")
	clientRange := c.GetHeader("Range")
	streamMediaUpstream(c, rawURL, nostore, clientRange != "")
}

// streamMediaUpstream proxies the upstream verbatim and never touches disk.
// Range requests land here so partial content is streamed as-is; the browser
// handles both 206 and 200 responses correctly.
func streamMediaUpstream(c *gin.Context, rawURL string, nostore bool, forwardRange bool) {
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

// streamMediaUpstreamNoLock tees the upstream body to the browser and to the
// cache file. Headers go out first so the response starts immediately instead
// of waiting for the whole download.
func streamMediaUpstreamNoLock(c *gin.Context, rawURL, filePath, fileName string) {
	client := service.GetSSRFProtectedHTTPClient()
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, rawURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request: " + err.Error()})
		return
	}
	req.Header.Set("User-Agent", "curl/8.4.0")

	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to fetch media: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("Upstream returned HTTP %d", resp.StatusCode)})
		return
	}

	tempPath := fmt.Sprintf("%s.%s.tmp", filePath, common.GetRandomString(6))
	out, err := os.Create(tempPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create cache file"})
		return
	}

	copyUpstreamHeaders(c, resp)
	setMediaCacheHeaders(c)
	c.Writer.WriteHeader(http.StatusOK)

	tmpBytes, copyErr := copyToTee(out, c.Writer, resp.Body, maxMediaSize)
	_ = out.Close()

	if copyErr != nil || tmpBytes <= 0 || int64(tmpBytes) > int64(maxMediaSize) {
		_ = os.Remove(tempPath)
		return
	}
	if err := os.Rename(tempPath, filePath); err != nil {
		common.SysError("media cache: rename failed for " + fileName + ": " + err.Error())
		_ = os.Remove(tempPath)
	}
}

// copyToTee streams from src to the client while writing at most budget bytes
// to tmp. The client always receives the full upstream body.
func copyToTee(tmp io.Writer, client io.Writer, src io.Reader, budget int64) (int64, error) {
	buf := make([]byte, 32*1024)
	var written int64
	for {
		n, readErr := src.Read(buf)
		if n > 0 {
			if _, err := client.Write(buf[:n]); err != nil {
				return written, err
			}
			if written < budget {
				remaining := budget - written
				chunk := buf[:n]
				if int64(len(chunk)) > remaining {
					chunk = chunk[:remaining]
				}
				if _, err := tmp.Write(chunk); err != nil {
					// The client keeps streaming even if the cache write fails.
					_ = err
				}
			}
			written += int64(n)
		}
		if readErr == io.EOF {
			return written, nil
		}
		if readErr != nil {
			return written, readErr
		}
	}
}

// copyUpstreamHeaders relays the whitelisted upstream headers so the browser
// sees the same media as if it had hit the CDN directly.
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

	hash, ext := getMediaCacheKey(rawURL)
	fileName := hash + ext
	filePath := filepath.Join(mediaCacheDir, fileName)

	cachedURL := fmt.Sprintf("/api/media/cache?url=%s", url.QueryEscape(rawURL))

	// If already exists, return immediately
	if info, err := os.Stat(filePath); err == nil && info.Size() > 0 {
		c.JSON(http.StatusOK, gin.H{"success": true, "cached_url": cachedURL, "cached": true})
		return
	}

	// Trigger async background caching so client is not blocked
	go func(targetURL, targetPath, key string) {
		if err := service.ValidateSSRFProtectedFetchURL(targetURL); err != nil {
			return
		}
		unlock := lockCacheFill(key)
		defer unlock()

		client := service.GetSSRFProtectedHTTPClient()
		req, err := http.NewRequest(http.MethodGet, targetURL, nil)
		if err != nil {
			return
		}
		req.Header.Set("User-Agent", "curl/8.4.0")
		resp, err := client.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			if resp != nil && resp.Body != nil {
				_ = resp.Body.Close()
			}
			return
		}
		defer resp.Body.Close()

		tempPath := fmt.Sprintf("%s.%s.tmp", targetPath, common.GetRandomString(6))
		out, err := os.Create(tempPath)
		if err != nil {
			return
		}
		n, copyErr := copyToTee(out, io.Discard, resp.Body, maxMediaSize)
		_ = out.Close()
		if copyErr != nil || n <= 0 || int64(n) > int64(maxMediaSize) {
			_ = os.Remove(tempPath)
			return
		}
		_ = os.Rename(tempPath, targetPath)
	}(rawURL, filePath, fileName)

	c.JSON(http.StatusOK, gin.H{"success": true, "cached_url": cachedURL, "cached": false})
}

// DeleteCachedMedia handles DELETE /api/media/cache?url=...
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
	filePath := filepath.Join(mediaCacheDir, fileName)

	_ = os.Remove(filePath)

	// Clean all transcoded variants (<hash>_h..._fps...) and temp files belonging to this key.
	if entries, err := os.ReadDir(mediaCacheDir); err == nil {
		for _, entry := range entries {
			name := entry.Name()
			if strings.HasPrefix(name, hash+"_") || strings.HasPrefix(name, fileName+".") || name == fileName {
				_ = os.Remove(filepath.Join(mediaCacheDir, name))
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "media cache deleted"})
}

// transcodeMediaToFile transcodes an input media file to a faststart MP4 file.
// Yields fully seekable video with HTTP 206 Range support and moov atom at the front.
func transcodeMediaToFile(ctx context.Context, inputSource, outputPath string, height int, fps int) error {
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

	tempPath := fmt.Sprintf("%s.tmp.%s.mp4", outputPath, common.GetRandomString(6))

	args := []string{
		"-loglevel", "error",
	}

	if strings.HasPrefix(inputSource, "http://") || strings.HasPrefix(inputSource, "https://") {
		args = append(args, "-user-agent", "curl/8.4.0")
	}

	args = append(args,
		"-i", inputSource,
		"-vf", strings.Join(vfParts, ","),
		"-pix_fmt", "yuv420p",
		"-c:v", "libx264",
		"-preset", "ultrafast",
		"-tune", "zerolatency",
		"-crf", "26",
		"-c:a", "aac",
		"-strict", "experimental",
		"-f", "mp4",
		"-movflags", "+faststart",
		"-y",
		tempPath,
	)

	cmd := exec.CommandContext(ctx, ffmpegBin, args...)
	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf
	if err := cmd.Run(); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("ffmpeg execution failed: %w (stderr: %s)", err, stderrBuf.String())
	}

	if err := os.Rename(tempPath, outputPath); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	return nil
}

type MediaProbeResult struct {
	Width    int     `json:"width"`
	Height   int     `json:"height"`
	FPS      float64 `json:"fps"`
	Duration float64 `json:"duration"`
}

var mediaProbeCache sync.Map

// ProbeMediaInfo returns video resolution, fps, and duration using ffprobe.
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

	inputSource, err := resolveMediaSource(c.Request.Context(), rawURL)
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
	if err := json.Unmarshal(out, &parsed); err != nil || len(parsed.Streams) == 0 {
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

