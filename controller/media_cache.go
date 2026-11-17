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

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const (
	mediaCacheDir = "uploads/media_cache"
	maxMediaSize  = 150 * 1024 * 1024 // 150MB max per media file
)

func init() {
	_ = os.MkdirAll(mediaCacheDir, 0755)
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

// GetCachedMedia handles GET /api/media/cache?url=...
// It serves from local disk cache if already downloaded, or fetches and caches it.
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

	hash, ext := getMediaCacheKey(rawURL)
	fileName := hash + ext
	filePath := filepath.Join(mediaCacheDir, fileName)

	// Check if already cached on disk
	if info, err := os.Stat(filePath); err == nil && info.Size() > 0 {
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
		c.Header("Accept-Ranges", "bytes")
		// Served to the same-origin dashboard and consumed from canvas; an
		// explicit ACAO header lets the browser treat it as CORS-clean.
		c.Header("Access-Control-Allow-Origin", "*")
		c.File(filePath)
		return
	}

	// Not cached, download and cache safely with SSRF protection
	if err := service.ValidateSSRFProtectedFetchURL(rawURL); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access to URL blocked: " + err.Error()})
		return
	}

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

	tempPath := filePath + ".tmp"
	out, err := os.Create(tempPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create cache file"})
		return
	}

	limitReader := io.LimitReader(resp.Body, maxMediaSize)
	_, copyErr := io.Copy(out, limitReader)
	_ = out.Close()

	if copyErr != nil {
		_ = os.Remove(tempPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save media: " + copyErr.Error()})
		return
	}

	_ = os.Rename(tempPath, filePath)

	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.Header("Accept-Ranges", "bytes")
	c.Header("Access-Control-Allow-Origin", "*")
	c.File(filePath)
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
	go func(targetURL, targetPath string) {
		if err := service.ValidateSSRFProtectedFetchURL(targetURL); err != nil {
			return
		}
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

		tempPath := targetPath + ".tmp"
		out, err := os.Create(tempPath)
		if err != nil {
			return
		}
		limitReader := io.LimitReader(resp.Body, maxMediaSize)
		_, copyErr := io.Copy(out, limitReader)
		_ = out.Close()
		if copyErr == nil {
			_ = os.Rename(tempPath, targetPath)
		} else {
			_ = os.Remove(tempPath)
		}
	}(rawURL, filePath)

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
	_ = os.Remove(filePath + ".tmp")

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "media cache deleted"})
}

