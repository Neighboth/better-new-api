package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
)

func resolveExternalBaseURL(c *gin.Context) string {
	if configured := strings.TrimRight(strings.TrimSpace(system_setting.ServerAddress), "/"); configured != "" {
		return configured
	}
	scheme := "http"
	if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	host := strings.TrimSpace(c.GetHeader("X-Forwarded-Host"))
	if host == "" {
		host = strings.TrimSpace(c.Request.Host)
	}
	if host == "" {
		return ""
	}
	return scheme + "://" + host
}

func extractScopedAPIKey(c *gin.Context) string {
	auth := strings.TrimSpace(c.GetHeader("Authorization"))
	if len(auth) > 7 && strings.EqualFold(auth[:7], "Bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	if xKey := strings.TrimSpace(c.GetHeader("X-API-Key")); xKey != "" {
		return xKey
	}
	return ""
}

// ScopedAPIKeyAuth restricts an external management API (Blog API or File
// Manager API) strictly to valid, active keys created for that exact scope.
// If no keys exist or the provided key is invalid/deleted, all requests fail
// with 401 Unauthorized.
func ScopedAPIKeyAuth(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractScopedAPIKey(c)
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Missing API key. Pass Authorization: Bearer <key> or X-API-Key header.",
			})
			return
		}
		key, err := model.VerifyScopedAPIKey(scope, token)
		if err != nil || key == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Invalid or revoked API key.",
			})
			return
		}
		c.Set("scoped_api_key_id", key.Id)
		c.Set("scoped_api_key_scope", key.Scope)
		c.Next()
	}
}

func listScopedAPIKeysHandler(c *gin.Context, scope string) {
	keys, err := model.ListScopedAPIKeys(scope)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"keys":     keys,
			"base_url": resolveExternalBaseURL(c),
		},
	})
}

func createScopedAPIKeyHandler(c *gin.Context, scope string) {
	var req struct {
		Name string `json:"name"`
	}
	_ = common.DecodeJson(c.Request.Body, &req)
	createdBy := c.GetInt("id")
	rec, rawKey, err := model.CreateScopedAPIKey(scope, req.Name, createdBy)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"key":     rec,
			"raw_key": rawKey,
		},
	})
}

func deleteScopedAPIKeyHandler(c *gin.Context, scope string) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Invalid key ID"})
		return
	}
	if err := model.DeleteScopedAPIKey(scope, id); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// Blog API key management (AdminAuth)
func ListBlogAPIKeys(c *gin.Context) {
	listScopedAPIKeysHandler(c, model.APIKeyScopeBlog)
}

func CreateBlogAPIKey(c *gin.Context) {
	createScopedAPIKeyHandler(c, model.APIKeyScopeBlog)
}

func DeleteBlogAPIKey(c *gin.Context) {
	deleteScopedAPIKeyHandler(c, model.APIKeyScopeBlog)
}

// File Manager API key management (AdminAuth)
func ListFileManagerAPIKeys(c *gin.Context) {
	listScopedAPIKeysHandler(c, model.APIKeyScopeFileManager)
}

func CreateFileManagerAPIKey(c *gin.Context) {
	createScopedAPIKeyHandler(c, model.APIKeyScopeFileManager)
}

func DeleteFileManagerAPIKey(c *gin.Context) {
	deleteScopedAPIKeyHandler(c, model.APIKeyScopeFileManager)
}
