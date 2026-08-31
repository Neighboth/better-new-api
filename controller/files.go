package controller

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/captcha"
	"github.com/gin-gonic/gin"
)

const managedFilesDir = "managed_files"

func ensureManagedFilesDir() error {
	return os.MkdirAll(managedFilesDir, 0755)
}

func sanitizeRelPath(relPath string) (string, error) {
	relPath = strings.TrimPrefix(relPath, "/")
	clean := filepath.Clean(relPath)
	if clean == "." || clean == "" {
		return "", nil
	}
	if strings.HasPrefix(clean, "..") || strings.Contains(clean, "/..") {
		return "", fmt.Errorf("invalid path traversal")
	}
	return clean, nil
}

// ListManagedFiles returns file metadata for admin
func ListManagedFiles(c *gin.Context) {
	if err := ensureManagedFilesDir(); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}

	var files []model.ManagedFile
	if err := model.DB.Order("is_dir DESC, name ASC").Find(&files).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    files,
	})
}

// UploadManagedFile handles file upload or directory creation
func UploadManagedFile(c *gin.Context) {
	if err := ensureManagedFilesDir(); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}

	isDir := c.PostForm("is_dir") == "true"
	targetPath := c.PostForm("path") // e.g. "deneme.html" or "folder1/test.txt"
	cleanPath, err := sanitizeRelPath(targetPath)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Invalid path"})
		return
	}

	if cleanPath == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Path cannot be empty"})
		return
	}

	fullPath := filepath.Join(managedFilesDir, cleanPath)

	if isDir {
		if err := os.MkdirAll(fullPath, 0755); err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
			return
		}

		var fileRecord model.ManagedFile
		model.DB.Where("path = ?", cleanPath).FirstOrCreate(&fileRecord, model.ManagedFile{
			Path:      cleanPath,
			Name:      filepath.Base(cleanPath),
			IsDir:     true,
			CreatedAt: common.GetTimestamp(),
			UpdatedAt: common.GetTimestamp(),
		})

		c.JSON(http.StatusOK, gin.H{"success": true})
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "No file provided: " + err.Error()})
		return
	}

	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}

	if err := c.SaveUploadedFile(file, fullPath); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}

	var fileRecord model.ManagedFile
	res := model.DB.Where("path = ?", cleanPath).First(&fileRecord)
	now := common.GetTimestamp()
	if res.RowsAffected == 0 {
		fileRecord = model.ManagedFile{
			Path:      cleanPath,
			Name:      filepath.Base(cleanPath),
			IsDir:     false,
			Size:      file.Size,
			CreatedAt: now,
			UpdatedAt: now,
		}
		model.DB.Create(&fileRecord)
	} else {
		fileRecord.Size = file.Size
		fileRecord.UpdatedAt = now
		model.DB.Save(&fileRecord)
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": fileRecord})
}

// GetManagedFileContent returns the raw text content of a managed file
func GetManagedFileContent(c *gin.Context) {
	relPath := c.Query("path")
	cleanPath, err := sanitizeRelPath(relPath)
	if err != nil || cleanPath == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Invalid path"})
		return
	}

	fullPath := filepath.Join(managedFilesDir, cleanPath)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    string(content),
	})
}

// SaveManagedFileContent updates text content of a managed file
func SaveManagedFileContent(c *gin.Context) {
	var req struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Invalid params"})
		return
	}

	cleanPath, err := sanitizeRelPath(req.Path)
	if err != nil || cleanPath == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Invalid path"})
		return
	}

	fullPath := filepath.Join(managedFilesDir, cleanPath)
	if err := os.WriteFile(fullPath, []byte(req.Content), 0644); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}

	info, _ := os.Stat(fullPath)
	size := int64(len(req.Content))
	if info != nil {
		size = info.Size()
	}

	model.DB.Model(&model.ManagedFile{}).Where("path = ?", cleanPath).Updates(map[string]interface{}{
		"size":       size,
		"updated_at": common.GetTimestamp(),
	})

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// UpdateManagedFileSettings updates password / captcha protection settings
func UpdateManagedFileSettings(c *gin.Context) {
	var req struct {
		Path          string `json:"path"`
		Password      string `json:"password"`
		EnableCaptcha bool   `json:"enable_captcha"`
	}
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Invalid params"})
		return
	}

	cleanPath, err := sanitizeRelPath(req.Path)
	if err != nil || cleanPath == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Invalid path"})
		return
	}

	now := common.GetTimestamp()
	var fileRecord model.ManagedFile
	res := model.DB.Where("path = ?", cleanPath).First(&fileRecord)
	if res.RowsAffected == 0 {
		fileRecord = model.ManagedFile{
			Path:          cleanPath,
			Name:          filepath.Base(cleanPath),
			Password:      req.Password,
			EnableCaptcha: req.EnableCaptcha,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		model.DB.Create(&fileRecord)
	} else {
		model.DB.Model(&fileRecord).Updates(map[string]interface{}{
			"password":       req.Password,
			"enable_captcha": req.EnableCaptcha,
			"updated_at":     now,
		})
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// DeleteManagedFile removes a file/directory from disk and DB
func DeleteManagedFile(c *gin.Context) {
	relPath := c.Query("path")
	cleanPath, err := sanitizeRelPath(relPath)
	if err != nil || cleanPath == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Invalid path"})
		return
	}

	fullPath := filepath.Join(managedFilesDir, cleanPath)
	_ = os.RemoveAll(fullPath)

	model.DB.Where("path = ? OR path LIKE ?", cleanPath, cleanPath+"/%").Delete(&model.ManagedFile{})

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// RenameManagedFile renames a file/directory
func RenameManagedFile(c *gin.Context) {
	var req struct {
		OldPath string `json:"old_path"`
		NewPath string `json:"new_path"`
	}
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Invalid params"})
		return
	}

	oldClean, err1 := sanitizeRelPath(req.OldPath)
	newClean, err2 := sanitizeRelPath(req.NewPath)
	if err1 != nil || err2 != nil || oldClean == "" || newClean == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Invalid paths"})
		return
	}

	oldFull := filepath.Join(managedFilesDir, oldClean)
	newFull := filepath.Join(managedFilesDir, newClean)

	if err := os.Rename(oldFull, newFull); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}

	// Update DB records
	var record model.ManagedFile
	if model.DB.Where("path = ?", oldClean).First(&record).RowsAffected > 0 {
		record.Path = newClean
		record.Name = filepath.Base(newClean)
		record.UpdatedAt = common.GetTimestamp()
		model.DB.Save(&record)
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ServeManagedFileMiddleware handles serving files under /managed_files/ or root fallback
func ServeManagedFileMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		reqPath := c.Request.URL.Path
		// Skip /api, /v1, /pg, /mj, /suno, etc.
		if strings.HasPrefix(reqPath, "/api") ||
			strings.HasPrefix(reqPath, "/v1") ||
			strings.HasPrefix(reqPath, "/pg") ||
			strings.HasPrefix(reqPath, "/mj") ||
			strings.HasPrefix(reqPath, "/suno") ||
			strings.HasPrefix(reqPath, "/uploads") {
			c.Next()
			return
		}

		cleanPath, err := sanitizeRelPath(reqPath)
		if err != nil || cleanPath == "" {
			c.Next()
			return
		}

		fullPath := filepath.Join(managedFilesDir, cleanPath)
		info, err := os.Stat(fullPath)
		if err != nil || info.IsDir() {
			c.Next()
			return
		}

		// File exists! Check password and captcha requirements.
		var mf model.ManagedFile
		model.DB.Where("path = ?", cleanPath).First(&mf)

		// Also check parent directories for protection
		if mf.Password == "" && !mf.EnableCaptcha {
			dir := filepath.Dir(cleanPath)
			for dir != "." && dir != "/" {
				var parentMf model.ManagedFile
				if model.DB.Where("path = ?", dir).First(&parentMf).RowsAffected > 0 {
					if parentMf.Password != "" || parentMf.EnableCaptcha {
						mf.Password = parentMf.Password
						mf.EnableCaptcha = parentMf.EnableCaptcha
						break
					}
				}
				dir = filepath.Dir(dir)
			}
		}

		needsAuth := mf.Password != "" || mf.EnableCaptcha
		if needsAuth {
			// Check session or POST submission
			authCookieKey := fmt.Sprintf("file_auth_%x", sha256.Sum256([]byte(cleanPath)))
			cookieVal, _ := c.Cookie(authCookieKey)

			if c.Request.Method == "POST" && c.PostForm("managed_file_auth") == "1" {
				submittedPass := c.PostForm("password")

				if mf.EnableCaptcha {
					captchaId := c.PostForm("captcha_id")
					captchaAns := c.PostForm("captcha_ans")
					if captchaId == "" || captchaAns == "" || !captcha.Verify(captchaId, captchaAns) {
						renderAuthPage(c, cleanPath, mf, "Invalid Captcha")
						c.Abort()
						return
					}
				}

				if mf.Password != "" && submittedPass != mf.Password {
					renderAuthPage(c, cleanPath, mf, "Incorrect Password")
					c.Abort()
					return
				}

				// Auth passed! Set cookie and serve file
				c.SetCookie(authCookieKey, "1", 3600*24, "/", "", false, true)
			} else if cookieVal != "1" {
				renderAuthPage(c, cleanPath, mf, "")
				c.Abort()
				return
			}
		}

		// Serve static file
		http.ServeFile(c.Writer, c.Request, fullPath)
		c.Abort()
	}
}

func renderAuthPage(c *gin.Context, relPath string, mf model.ManagedFile, errorMsg string) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
	<meta charset="utf-8">
	<title>Protected File - %s</title>
	<style>
		body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #f4f4f5; display: flex; align-items: center; justify-content: center; height: 100vh; margin: 0; }
		.card { background: white; padding: 2rem; border-radius: 12px; box-shadow: 0 4px 12px rgba(0,0,0,0.1); width: 340px; text-align: center; }
		h2 { margin-top: 0; color: #18181b; font-size: 1.25rem; }
		p { color: #71717a; font-size: 0.875rem; margin-bottom: 1.5rem; }
		input { width: 100%%; padding: 0.625rem; margin-bottom: 1rem; border: 1px solid #e4e4e7; border-radius: 6px; box-sizing: border-box; font-size: 0.875rem; }
		button { width: 100%%; padding: 0.625rem; background: #18181b; color: white; border: none; border-radius: 6px; font-size: 0.875rem; cursor: pointer; font-weight: 500; }
		button:hover { background: #27272a; }
		.error { color: #ef4444; font-size: 0.875rem; margin-bottom: 1rem; }
		.captcha-box { margin-bottom: 1rem; }
		img.captcha-img { height: 50px; border: 1px solid #e4e4e7; border-radius: 6px; cursor: pointer; }
	</style>
</head>
<body>
	<div class="card">
		<h2>Access Protected File</h2>
		<p>%s</p>
		%s
		<form method="POST">
			<input type="hidden" name="managed_file_auth" value="1" />
			%s
			%s
			<button type="submit">Submit</button>
		</form>
	</div>
	<script>
		function refreshCaptcha() {
			fetch('/api/captcha/image').then(r => r.json()).then(d => {
				if(d.success) {
					document.getElementById('captcha-img').src = d.data.image;
					document.getElementById('captcha-id').value = d.data.captcha_id;
				}
			});
		}
	</script>
</body>
</html>`,
		relPath, relPath,
		func() string {
			if errorMsg != "" {
				return fmt.Sprintf(`<div class="error">%s</div>`, errorMsg)
			}
			return ""
		}(),
		func() string {
			if mf.Password != "" {
				return `<input type="password" name="password" placeholder="Enter password" required />`
			}
			return ""
		}(),
		func() string {
			if mf.EnableCaptcha {
				return `<div class="captcha-box">
					<input type="hidden" id="captcha-id" name="captcha_id" />
					<img id="captcha-img" class="captcha-img" onclick="refreshCaptcha()" title="Click to refresh" src="" />
					<input type="text" name="captcha_ans" placeholder="Captcha code" required />
				</div>
				<script>window.addEventListener("DOMContentLoaded", refreshCaptcha);</script>`
			}
			return ""
		}(),
	)

	c.String(http.StatusOK, html)
}
