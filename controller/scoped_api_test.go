package controller

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupScopedAPITestEnv(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	prevDB := model.DB
	prevServerAddr := system_setting.ServerAddress
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ScopedAPIKey{}, &model.BlogPost{}, &model.BlogComment{}, &model.BlogReaction{}, &model.ManagedFile{}))
	model.DB = db
	system_setting.ServerAddress = "https://example.test"

	t.Cleanup(func() {
		model.DB = prevDB
		system_setting.ServerAddress = prevServerAddr
	})

	r := gin.New()
	blogExt := r.Group("/api/v1/blog")
	blogExt.Use(ScopedAPIKeyAuth(model.APIKeyScopeBlog))
	{
		blogExt.GET("/posts", BlogAPIListPosts)
		blogExt.GET("/posts/:id", BlogAPIGetPost)
		blogExt.POST("/posts", BlogAPICreatePost)
		blogExt.PUT("/posts/:id", BlogAPIUpdatePost)
		blogExt.PATCH("/posts/:id", BlogAPIUpdatePost)
		blogExt.DELETE("/posts/:id", BlogAPIDeletePost)
		blogExt.POST("/upload", BlogAPIUploadMedia)
	}

	filesExt := r.Group("/api/v1/files")
	filesExt.Use(ScopedAPIKeyAuth(model.APIKeyScopeFileManager))
	{
		filesExt.GET("", FileManagerAPIList)
		filesExt.GET("/info", FileManagerAPIInfo)
		filesExt.GET("/download", FileManagerAPIDownload)
		filesExt.GET("/content", FileManagerAPIReadContent)
		filesExt.PUT("/content", FileManagerAPIWriteContent)
		filesExt.POST("/upload", FileManagerAPIUpload)
		filesExt.POST("/mkdir", FileManagerAPIMkdir)
		filesExt.POST("/rename", FileManagerAPIRename)
		filesExt.DELETE("", FileManagerAPIDelete)
	}
	return r
}

func TestBlogExternalAPIFullFlowAndAuthIsolation(t *testing.T) {
	r := setupScopedAPITestEnv(t)

	// 1. Without any key -> 401
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/blog/posts", nil))
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// 2. Create a File Manager key -> must NOT grant access to Blog API
	_, fileKey, err := model.CreateScopedAPIKey(model.APIKeyScopeFileManager, "Files only", 1)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/blog/posts", nil)
	req.Header.Set("Authorization", "Bearer "+fileKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// 3. Create valid Blog API key
	blogKeyRec, blogKey, err := model.CreateScopedAPIKey(model.APIKeyScopeBlog, "Blog Publisher", 1)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(blogKey, "sk-blog-"))

	// 4. Create multi-language long post with banner + YouTube + Video.js embeds
	longParagraph := strings.Repeat("Detailed technical benchmark analysis. ", 2500)
	createPayload := map[string]any{
		"banner":    "https://example.test/uploads/blog/hero.png",
		"published": true,
		"translations": map[string]any{
			"en": map[string]any{
				"title":           "Launch Post EN",
				"summary":         "Summary EN",
				"tags":            []string{"release", "video"},
				"seo_description": "SEO EN",
				"content":         "# Launch\n\n::youtube[https://youtu.be/dQw4w9WgXcQ]\n\n::video[/uploads/blog/demo.mp4]\n\n" + longParagraph,
			},
			"tr": map[string]any{
				"title":   "Yayın Yazısı TR",
				"summary": "Özet TR",
				"tags":    "duyuru, rehber",
				"content": "# Yayın\n\nTürkçe içerik",
			},
			"zh-CN": map[string]any{
				"title":   "发布文章",
				"content": "# 发布\n\n中文内容",
			},
		},
	}
	bodyBytes, err := common.Marshal(createPayload)
	require.NoError(t, err)

	req = httptest.NewRequest(http.MethodPost, "/api/v1/blog/posts", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer "+blogKey)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var createdResp struct {
		Success bool `json:"success"`
		Data    struct {
			ID           int64                     `json:"id"`
			URL          string                    `json:"url"`
			Title        string                    `json:"title"`
			CoverImage   string                    `json:"cover_image"`
			Published    bool                      `json:"published"`
			Translations map[string]map[string]any `json:"translations"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &createdResp))
	require.True(t, createdResp.Success)
	assert.Equal(t, fmt.Sprintf("https://example.test/blog/%d", createdResp.Data.ID), createdResp.Data.URL)
	assert.Equal(t, "https://example.test/uploads/blog/hero.png", createdResp.Data.CoverImage)
	assert.True(t, createdResp.Data.Published)
	assert.Equal(t, "Yayın Yazısı TR", createdResp.Data.Translations["tr"]["title"])
	assert.Equal(t, "发布文章", createdResp.Data.Translations["zhCN"]["title"])

	// 5. Partial PATCH adds French translation while preserving English & Turkish
	patchBody := `{"translations":{"fr":{"title":"Article FR","content":"Contenu FR"}}}`
	req = httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/v1/blog/posts/%d", createdResp.Data.ID), strings.NewReader(patchBody))
	req.Header.Set("Authorization", "Bearer "+blogKey)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "Article FR")
	assert.Contains(t, w.Body.String(), "Yayın Yazısı TR")

	// 6. List posts includes full URL
	req = httptest.NewRequest(http.MethodGet, "/api/v1/blog/posts", nil)
	req.Header.Set("Authorization", "Bearer "+blogKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), createdResp.Data.URL)

	// 7. Upload banner image via Blog API
	var mp bytes.Buffer
	mw := multipart.NewWriter(&mp)
	fw, err := mw.CreateFormFile("file", "my banner.png")
	require.NoError(t, err)
	_, err = fw.Write([]byte("\x89PNG\r\n\x1a\nfake-png-bytes"))
	require.NoError(t, err)
	require.NoError(t, mw.WriteField("filename", "custom-hero.png"))
	require.NoError(t, mw.Close())

	req = httptest.NewRequest(http.MethodPost, "/api/v1/blog/upload", &mp)
	req.Header.Set("Authorization", "Bearer "+blogKey)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var uploadResp struct {
		Success bool `json:"success"`
		Data    struct {
			URL  string `json:"url"`
			Path string `json:"path"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &uploadResp))
	assert.True(t, strings.HasPrefix(uploadResp.Data.URL, "https://example.test/uploads/blog/"))
	t.Cleanup(func() {
		_ = os.Remove(filepath.FromSlash(strings.TrimPrefix(uploadResp.Data.Path, "/")))
	})

	// 8. Delete post
	req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/blog/posts/%d", createdResp.Data.ID), nil)
	req.Header.Set("Authorization", "Bearer "+blogKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// 9. Revoking the Blog API key immediately disables external access
	require.NoError(t, model.DeleteScopedAPIKey(model.APIKeyScopeBlog, blogKeyRec.Id))
	req = httptest.NewRequest(http.MethodGet, "/api/v1/blog/posts", nil)
	req.Header.Set("Authorization", "Bearer "+blogKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestFileManagerExternalAPIFullFlow(t *testing.T) {
	r := setupScopedAPITestEnv(t)

	_, fmKey, err := model.CreateScopedAPIKey(model.APIKeyScopeFileManager, "CI Bot", 1)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(fmKey, "sk-files-"))

	// 1. Reject traversal
	req := httptest.NewRequest(http.MethodPost, "/api/v1/files/mkdir", strings.NewReader(`{"path":"../../etc"}`))
	req.Header.Set("Authorization", "Bearer "+fmKey)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// 2. Upload to nested folder with custom filename via multipart
	var mp bytes.Buffer
	mw := multipart.NewWriter(&mp)
	fw, err := mw.CreateFormFile("file", "orig.txt")
	require.NoError(t, err)
	_, err = fw.Write([]byte("hello external file api"))
	require.NoError(t, err)
	require.NoError(t, mw.WriteField("dir", "assets/reports"))
	require.NoError(t, mw.WriteField("filename", "q4-summary.txt"))
	require.NoError(t, mw.Close())

	req = httptest.NewRequest(http.MethodPost, "/api/v1/files/upload", &mp)
	req.Header.Set("Authorization", "Bearer "+fmKey)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "https://example.test/assets/reports/q4-summary.txt")

	// 3. Upload binary via JSON base64
	b64Payload := fmt.Sprintf(`{"path":"assets/reports/badge.bin","content_base64":"%s"}`, base64.StdEncoding.EncodeToString([]byte{1, 2, 3, 4}))
	req = httptest.NewRequest(http.MethodPost, "/api/v1/files/upload", strings.NewReader(b64Payload))
	req.Header.Set("Authorization", "Bearer "+fmKey)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	// 4. Read & Edit text content
	req = httptest.NewRequest(http.MethodGet, "/api/v1/files/content?path=assets/reports/q4-summary.txt", nil)
	req.Header.Set("Authorization", "Bearer "+fmKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "hello external file api")

	req = httptest.NewRequest(http.MethodPut, "/api/v1/files/content", strings.NewReader(`{"path":"assets/reports/q4-summary.txt","content":"updated summary text"}`))
	req.Header.Set("Authorization", "Bearer "+fmKey)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// 5. Download raw bytes reflects updated content
	req = httptest.NewRequest(http.MethodGet, "/api/v1/files/download?path=assets/reports/q4-summary.txt", nil)
	req.Header.Set("Authorization", "Bearer "+fmKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "updated summary text", w.Body.String())

	// 6. Rename entire folder recursively
	req = httptest.NewRequest(http.MethodPost, "/api/v1/files/rename", strings.NewReader(`{"old_path":"assets/reports","new_path":"assets/archive"}`))
	req.Header.Set("Authorization", "Bearer "+fmKey)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// 7. Info on moved child file returns new public URL
	req = httptest.NewRequest(http.MethodGet, "/api/v1/files/info?path=assets/archive/q4-summary.txt", nil)
	req.Header.Set("Authorization", "Bearer "+fmKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "https://example.test/assets/archive/q4-summary.txt")

	// 8. Delete folder recursively
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/files?path=assets", nil)
	req.Header.Set("Authorization", "Bearer "+fmKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	req = httptest.NewRequest(http.MethodGet, "/api/v1/files/info?path=assets/archive/q4-summary.txt", nil)
	req.Header.Set("Authorization", "Bearer "+fmKey)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}
