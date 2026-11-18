package controller

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
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
	// no-transform: never let an intermediary re-encode video.
	// X-Accel-Buffering: no: stream through nginx instead of buffering the
	// whole body before flushing to the browser (choppy seeking symptom).
	c.Header("Cache-Control", "public, max-age=31536000, immutable, no-transform")
	c.Header("Accept-Ranges", "bytes")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("X-Accel-Buffering", "no")
}

// GetCachedMedia handles GET /api/media/cache?url=...
// It serves from the local disk cache when possible; on a cold miss it
// streams the upstream response to the browser while (optionally) teeing it
// into the cache. A Range request on a cold miss is proxied to the upstream
// and answered 206 without touching the disk. `nostore=1` streams without
// caching anything (used by the browser to import media into IndexedDB when
// the user's server-side storage quota is exhausted).
func GetCachedMedia(c *gin.Context) {
	rawURL := strings.TrimSpace(c.Query("url"))
	if rawURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url parameter is required"})
		return
	}

	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only HTTP/HTTPS URLs are allowed"})
		return
	}
	nostore := c.Query("nostore") == "1" || strings.EqualFold(c.Query("nostore"), "true")

	hash, ext := getMediaCacheKey(rawURL)
	fileName := hash + ext
	filePath := filepath.Join(mediaCacheDir, fileName)

	// Cache hit: serve from disk (Range supported by http.ServeFile).
	if !nostore {
		if info, err := os.Stat(filePath); err == nil && info.Size() > 0 {
			setMediaCacheHeaders(c)
			c.File(filePath)
			return
		}
	}

	if err := service.ValidateSSRFProtectedFetchURL(rawURL); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access to URL blocked: " + err.Error()})
		return
	}

	clientRange := c.GetHeader("Range")
	if clientRange != "" || nostore {
		streamMediaUpstream(c, rawURL, nostore, clientRange != "")
		return
	}

	unlock := lockCacheFill(fileName)
	defer unlock()
	// Another goroutine may have filled the cache while we were waiting.
	if info, err := os.Stat(filePath); err == nil && info.Size() > 0 {
		setMediaCacheHeaders(c)
		c.File(filePath)
		return
	}

	streamMediaUpstreamNoLock(c, rawURL, filePath, fileName)
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

	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	// Clean any in-flight or stale temp files belonging to this key.
	if entries, err := os.ReadDir(mediaCacheDir); err == nil {
		for _, entry := range entries {
			name := entry.Name()
			if strings.HasPrefix(name, fileName+".") && strings.HasSuffix(name, ".tmp") {
				_ = os.Remove(filepath.Join(mediaCacheDir, name))
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "media cache deleted"})
}
