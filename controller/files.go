package controller

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/captcha"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

const managedFilesDir = "managed_files"

func ensureManagedFilesDir() error {
	return os.MkdirAll(managedFilesDir, 0755)
}

func sanitizeRelPath(relPath string) (string, error) {
	relPath = strings.TrimPrefix(relPath, "/")
	clean := filepath.Clean(relPath)
	// Store records with forward slashes regardless of the platform the
	// binary runs on; a browser always uses "/" in URLs.
	clean = filepath.ToSlash(clean)
	if clean == "." || clean == "" {
		return "", nil
	}
	if strings.HasPrefix(clean, "..") || strings.Contains(clean, "/..") {
		return "", fmt.Errorf("invalid path traversal")
	}
	return clean, nil
}

// passwordOfRecord returns the raw stored password for a managed file.
// New files store a bcrypt hash; legacy rows hold plaintext.
func passwordHashOf(password string) string {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return password
	}
	return string(hashed)
}

func verifyFilePassword(submitted, stored string) bool {
	submitted = strings.TrimSpace(submitted)
	stored = strings.TrimSpace(stored)
	if stored == "" || submitted == "" {
		return false
	}
	if strings.HasPrefix(stored, "$2a$") || strings.HasPrefix(stored, "$2b$") || strings.HasPrefix(stored, "$2y$") {
		return bcrypt.CompareHashAndPassword([]byte(stored), []byte(submitted)) == nil
	}
	// Legacy plaintext rows: constant-time compare so the response does not
	// leak prefix matches.
	return subtle.ConstantTimeCompare([]byte(stored), []byte(submitted)) == 1
}

// fileAuthCookieKey is the cookie name that stores the signed proof that the
// visitor authenticated for this exact file path.
func fileAuthCookieKey(path string) string {
	sum := sha256.Sum256([]byte(path))
	return fmt.Sprintf("file_auth_%x", sum[:16])
}

// fileAuthSign signs the proof token with the installation secret, path, password hash,
// and file updated timestamp. When the password changes or the file is updated,
// existing session cookies are instantly invalidated.
func fileAuthSign(path string, passwordHash string, updatedAt int64, expiry int64) string {
	payload := fmt.Sprintf("%s|%s|%d|%d", path, passwordHash, updatedAt, expiry)
	mac := hmac.New(sha256.New, []byte(common.CryptoSecret+":file-auth-v3"))
	_, _ = mac.Write([]byte(payload))
	return fmt.Sprintf("%d.%x", expiry, mac.Sum(nil))
}

func fileAuthVerify(path string, passwordHash string, updatedAt int64, value string) bool {
	parts := strings.SplitN(value, ".", 2)
	if len(parts) != 2 {
		return false
	}
	expiry, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || time.Now().Unix() > expiry {
		return false
	}
	expected := fileAuthSign(path, passwordHash, updatedAt, expiry)
	return subtle.ConstantTimeCompare([]byte(expected), []byte(value)) == 1
}

// findManagedFileWithProtection retrieves a managed file and walks parent directories
// using normalized forward-slash paths so inherited password or captcha settings apply
// on all platforms (including Windows).
func findManagedFileWithProtection(cleanPath string) (model.ManagedFile, bool) {
	var mf model.ManagedFile
	res := model.DB.Where("path = ?", cleanPath).First(&mf)
	exists := res.RowsAffected > 0

	if mf.Password == "" && !mf.EnableCaptcha {
		dir := filepath.ToSlash(filepath.Dir(cleanPath))
		for dir != "." && dir != "/" && dir != "" {
			var parentMf model.ManagedFile
			if model.DB.Where("path = ?", dir).First(&parentMf).RowsAffected > 0 {
				if parentMf.Password != "" || parentMf.EnableCaptcha {
					mf.Password = parentMf.Password
					mf.EnableCaptcha = parentMf.EnableCaptcha
					if mf.UpdatedAt < parentMf.UpdatedAt {
						mf.UpdatedAt = parentMf.UpdatedAt
					}
					break
				}
			}
			nextDir := filepath.ToSlash(filepath.Dir(dir))
			if nextDir == dir {
				break
			}
			dir = nextDir
		}
	}
	return mf, exists
}

// fileAuthLimiter throttles repeated failed attempts against one file path.
var fileAuthLimiter = struct {
	sync.Mutex
	tries map[string][]int64
}{tries: map[string][]int64{}}

func fileAuthBlocked(key string) bool {
	fileAuthLimiter.Lock()
	defer fileAuthLimiter.Unlock()
	now := time.Now().Unix()
	attempts := fileAuthLimiter.tries[key]
	kept := attempts[:0]
	for _, t := range attempts {
		if now-t < 600 {
			kept = append(kept, t)
		}
	}
	fileAuthLimiter.tries[key] = kept
	return len(kept) >= 10
}

func fileAuthFailedAttempt(key string) {
	fileAuthLimiter.Lock()
	defer fileAuthLimiter.Unlock()
	fileAuthLimiter.tries[key] = append(fileAuthLimiter.tries[key], time.Now().Unix())
}

// scanDirectoryToDB recursively scans disk directory and syncs metadata to DB
func syncDiskDirToDB(baseDir string, prefix string) {
	_ = filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || path == baseDir {
			return nil
		}
		rel, err := filepath.Rel(baseDir, path)
		if err != nil {
			return nil
		}
		relPath := filepath.ToSlash(rel)
		if prefix != "" {
			relPath = prefix + "/" + relPath
		}
		var fileRecord model.ManagedFile
		res := model.DB.Where("path = ?", relPath).First(&fileRecord)
		now := common.GetTimestamp()
		if res.RowsAffected == 0 {
			fileRecord = model.ManagedFile{
				Path:      relPath,
				Name:      info.Name(),
				IsDir:     info.IsDir(),
				Size:      info.Size(),
				CreatedAt: now,
				UpdatedAt: now,
			}
			if !info.IsDir() {
				data, err := os.ReadFile(path)
				if err == nil && len(data) < 10*1024*1024 { // max 10MB stored in DB
					fileRecord.Content = data
				}
			}
			model.DB.Create(&fileRecord)
		}
		return nil
	})
}

func ensureUploadFoldersExist() {
	now := common.GetTimestamp()
	dirs := []string{"uploads", "uploads/avatars", "uploads/ads", "uploads/vendors"}
	for _, dir := range dirs {
		var mf model.ManagedFile
		if model.DB.Where("path = ?", dir).First(&mf).RowsAffected == 0 {
			model.DB.Create(&model.ManagedFile{
				Path:      dir,
				Name:      filepath.Base(dir),
				IsDir:     true,
				CreatedAt: now,
				UpdatedAt: now,
			})
		}
	}
}

// ListManagedFiles returns file metadata for admin
func ListManagedFiles(c *gin.Context) {
	ensureUploadFoldersExist()

	// Sync disk files from managed_files and uploads if present
	syncDiskDirToDB(managedFilesDir, "")
	if _, err := os.Stat("uploads"); err == nil {
		syncDiskDirToDB("uploads", "uploads")
	}

	var files []model.ManagedFile
	if err := model.DB.Order("is_dir DESC, name ASC").Find(&files).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	// The admin needs to know whether a file has a password, never the
	// password itself: the list payload exposes a boolean only.
	items := make([]gin.H, 0, len(files))
	for _, file := range files {
		items = append(items, gin.H{
			"id":             file.Id,
			"path":           file.Path,
			"name":           file.Name,
			"is_dir":         file.IsDir,
			"size":           file.Size,
			"created_at":     file.CreatedAt,
			"updated_at":     file.UpdatedAt,
			"has_password":   strings.TrimSpace(file.Password) != "",
			"enable_captcha": file.EnableCaptcha,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    items,
	})
}

// UploadManagedFile handles file upload or directory creation
func isReadOnlyPath(path string) bool {
	clean := strings.TrimPrefix(filepath.ToSlash(path), "/")
	return clean == "uploads" || strings.HasPrefix(clean, "uploads/")
}

func UploadManagedFile(c *gin.Context) {
	isDir := c.PostForm("is_dir") == "true"
	targetPath := c.PostForm("path") // e.g. "deneme.html" or "folder1/test.txt"
	cleanPath, err := sanitizeRelPath(targetPath)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Invalid path"})
		return
	}

	if isReadOnlyPath(cleanPath) {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Uploads directory is read-only"})
		return
	}

	if cleanPath == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Path cannot be empty"})
		return
	}

	if isDir {
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

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "No file provided: " + err.Error()})
		return
	}

	f, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Failed to read file: " + err.Error()})
		return
	}
	defer f.Close()

	fileBytes := make([]byte, fileHeader.Size)
	_, _ = f.Read(fileBytes)

	// File content is stored 100% in database DB. No physical files/folders created on disk.

	var fileRecord model.ManagedFile
	res := model.DB.Where("path = ?", cleanPath).First(&fileRecord)
	now := common.GetTimestamp()
	if res.RowsAffected == 0 {
		fileRecord = model.ManagedFile{
			Path:      cleanPath,
			Name:      filepath.Base(cleanPath),
			IsDir:     false,
			Size:      fileHeader.Size,
			Content:   fileBytes,
			CreatedAt: now,
			UpdatedAt: now,
		}
		model.DB.Create(&fileRecord)
	} else {
		fileRecord.Size = fileHeader.Size
		fileRecord.Content = fileBytes
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

	mf, exists := findManagedFileWithProtection(cleanPath)
	if mf.Password != "" || mf.EnableCaptcha {
		authCookieKey := fileAuthCookieKey(cleanPath)
		cookieVal, _ := c.Cookie(authCookieKey)
		if !fileAuthVerify(cleanPath, mf.Password, mf.UpdatedAt, cookieVal) {
			c.JSON(http.StatusOK, gin.H{
				"success":        false,
				"auth_required":  true,
				"has_password":   mf.Password != "",
				"enable_captcha": mf.EnableCaptcha,
				"message":        "Protection challenge required to view or edit this file",
			})
			return
		}
	}

	if exists && len(mf.Content) > 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    string(mf.Content),
		})
		return
	}

	// Fallback to disk read
	var fullPath string
	if strings.HasPrefix(cleanPath, "uploads/") {
		fullPath = cleanPath
	} else {
		fullPath = filepath.Join(managedFilesDir, cleanPath)
	}
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

	if isReadOnlyPath(cleanPath) {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Uploads directory is read-only"})
		return
	}

	contentBytes := []byte(req.Content)
	size := int64(len(contentBytes))

	now := common.GetTimestamp()
	var record model.ManagedFile
	if model.DB.Where("path = ?", cleanPath).First(&record).RowsAffected == 0 {
		fileRecord := model.ManagedFile{
			Path:      cleanPath,
			Name:      filepath.Base(cleanPath),
			IsDir:     false,
			Size:      size,
			Content:   contentBytes,
			CreatedAt: now,
			UpdatedAt: now,
		}
		model.DB.Create(&fileRecord)
	} else {
		model.DB.Model(&record).Updates(map[string]interface{}{
			"size":       size,
			"content":    contentBytes,
			"updated_at": now,
		})
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// UpdateManagedFileSettings updates password / captcha protection settings
func UpdateManagedFileSettings(c *gin.Context) {
	var req struct {
		Path           string `json:"path"`
		Password       string `json:"password"`
		EnableCaptcha  bool   `json:"enable_captcha"`
		RemovePassword bool   `json:"remove_password"`
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
	passwordToStore := strings.TrimSpace(req.Password)
	if req.RemovePassword {
		// The administrator explicitly asked to drop the stored password.
		passwordToStore = ""
	} else if passwordToStore != "" && (res.RowsAffected == 0 || passwordToStore != fileRecord.Password) {
		passwordToStore = passwordHashOf(passwordToStore)
	} else if res.RowsAffected > 0 && passwordToStore == fileRecord.Password {
		// The admin left the password box untouched (an unchanged hash was
		// round-tripped): keep the stored hash.
	} else {
		// Leave the stored password as is when the box is empty and no
		// explicit removal was requested.
		passwordToStore = fileRecord.Password
	}
	if res.RowsAffected == 0 {
		fileRecord = model.ManagedFile{
			Path:          cleanPath,
			Name:          filepath.Base(cleanPath),
			Password:      passwordToStore,
			EnableCaptcha: req.EnableCaptcha,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		model.DB.Create(&fileRecord)
	} else {
		model.DB.Model(&fileRecord).Updates(map[string]interface{}{
			"password":       passwordToStore,
			"enable_captcha": req.EnableCaptcha,
			"updated_at":     now,
		})
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// DeleteManagedFile removes a file/directory from DB
func DeleteManagedFile(c *gin.Context) {
	relPath := c.Query("path")
	cleanPath, err := sanitizeRelPath(relPath)
	if err != nil || cleanPath == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Invalid path"})
		return
	}

	model.DB.Where("path = ? OR path LIKE ?", cleanPath, cleanPath+"/%").Delete(&model.ManagedFile{})

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// RenameManagedFile renames a file/directory in DB
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

	if isReadOnlyPath(oldClean) || isReadOnlyPath(newClean) {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Uploads directory is read-only"})
		return
	}

	var record model.ManagedFile
	if model.DB.Where("path = ?", oldClean).First(&record).RowsAffected > 0 {
		record.Path = newClean
		record.Name = filepath.Base(newClean)
		record.UpdatedAt = common.GetTimestamp()
		model.DB.Save(&record)
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// serveManagedContent writes the stored bytes of a managed file with a
// content type derived from the file extension, falling back to the
// detector when the extension is unknown.
func serveManagedContent(c *gin.Context, cleanPath string, content []byte) {
	contentType := http.DetectContentType(content)
	switch {
	case strings.HasSuffix(cleanPath, ".css"):
		contentType = "text/css; charset=utf-8"
	case strings.HasSuffix(cleanPath, ".js"):
		contentType = "application/javascript; charset=utf-8"
	case strings.HasSuffix(cleanPath, ".html"):
		contentType = "text/html; charset=utf-8"
	case strings.HasSuffix(cleanPath, ".svg"):
		contentType = "image/svg+xml"
	case strings.HasSuffix(cleanPath, ".png"):
		contentType = "image/png"
	case strings.HasSuffix(cleanPath, ".jpg"), strings.HasSuffix(cleanPath, ".jpeg"):
		contentType = "image/jpeg"
	case strings.HasSuffix(cleanPath, ".webp"):
		contentType = "image/webp"
	case strings.HasSuffix(cleanPath, ".gif"):
		contentType = "image/gif"
	}
	c.Data(http.StatusOK, contentType, content)
	c.Abort()
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
			strings.HasPrefix(reqPath, "/suno") {
			c.Next()
			return
		}

		cleanPath, err := sanitizeRelPath(reqPath)
		if err != nil || cleanPath == "" {
			c.Next()
			return
		}

		mf, exists := findManagedFileWithProtection(cleanPath)
		if !exists {
			// Fallback check physical file on disk
			fullPath := filepath.Join(managedFilesDir, cleanPath)
			info, err := os.Stat(fullPath)
			if err != nil || info.IsDir() {
				if strings.HasPrefix(cleanPath, "uploads/") || cleanPath == "uploads" {
					info, err = os.Stat(cleanPath)
				}
				if err != nil || info.IsDir() {
					c.Next()
					return
				}
			}
		} else if mf.IsDir {
			c.Next()
			return
		}

		needsAuth := mf.Password != "" || mf.EnableCaptcha
		if needsAuth {
			// The cookie carries an HMAC-signed proof of authentication for
			// this exact path, signed with the installation secret. A visitor
			// cannot mint the cookie themselves.
			authCookieKey := fileAuthCookieKey(cleanPath)
			cookieVal, _ := c.Cookie(authCookieKey)

			if c.Request.Method == "POST" && c.PostForm("managed_file_auth") == "1" {
				submittedPass := c.PostForm("password")
				limiterKey := cleanPath + "|" + c.ClientIP()
				if fileAuthBlocked(limiterKey) {
					renderAuthPage(c, cleanPath, mf, "Too many failed attempts. Please try again later.")
					c.Abort()
					return
				}

				if mf.EnableCaptcha {
					if !verifyManagedFileCaptcha(c) {
						fileAuthFailedAttempt(limiterKey)
						renderAuthPage(c, cleanPath, mf, "Captcha Verification Failed")
						c.Abort()
						return
					}
				}

				if mf.Password != "" && !verifyFilePassword(submittedPass, mf.Password) {
					fileAuthFailedAttempt(limiterKey)
					renderAuthPage(c, cleanPath, mf, "Incorrect Password")
					c.Abort()
					return
				}

				// Auth passed! Set a signed cookie (24h expiry) and redirect
				expiry := time.Now().Add(24 * time.Hour).Unix()
				c.SetSameSite(http.SameSiteLaxMode)
				c.SetCookie(authCookieKey, fileAuthSign(cleanPath, mf.Password, mf.UpdatedAt, expiry), 3600*24, "/", "", c.Request.TLS != nil, true)
				c.Redirect(http.StatusSeeOther, c.Request.RequestURI)
				c.Abort()
				return
			} else if !fileAuthVerify(cleanPath, mf.Password, mf.UpdatedAt, cookieVal) {
				renderAuthPage(c, cleanPath, mf, "")
				c.Abort()
				return
			}
		}

		// Serve file content from DB or disk
		if len(mf.Content) > 0 {
			serveManagedContent(c, cleanPath, mf.Content)
			return
		}

		fullPath := filepath.Join(managedFilesDir, cleanPath)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) && (strings.HasPrefix(cleanPath, "uploads/") || cleanPath == "uploads") {
			fullPath = cleanPath
		}
		http.ServeFile(c.Writer, c.Request, fullPath)
		c.Abort()
	}
}

func verifyMTCaptchaForFiles(privateKey, token, remoteIP string) bool {
	privateKey = strings.TrimSpace(privateKey)
	token = strings.TrimSpace(token)
	if privateKey == "" || token == "" {
		return false
	}
	payload, err := json.Marshal(map[string]string{
		"privatekey": privateKey,
		"token":      token,
		"remoteip":   remoteIP,
	})
	if err != nil {
		return false
	}
	req, err := http.NewRequest(http.MethodPost, "https://service.mtcaptcha.com/mtcv1/api/validate", bytes.NewReader(payload))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := clientWithTimeout().Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	var parsed struct {
		Success bool   `json:"success"`
		Status  string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return false
	}
	return parsed.Success || strings.EqualFold(parsed.Status, "OK")
}

func clientWithTimeout() *http.Client {
	return &http.Client{Timeout: 10 * time.Second}
}

func verifyManagedFileCaptchaToken(clientIP, provider, token, captchaId, captchaAns string) bool {
	captchaType := common.GetEffectiveCaptchaType()
	if provider == "" {
		provider = captchaType
	}

	if captchaType != common.CaptchaTypeOff && provider != captchaType {
		if !common.CaptchaFallbackEnabled() && provider != common.CaptchaTypeImage {
			return false
		}
		if !common.IsCaptchaProviderConfigured(provider) {
			return false
		}
	}

	if token == "" {
		if captchaId != "" && captchaAns != "" {
			return captcha.Verify(captchaId, captchaAns)
		}
		return false
	}

	if provider == common.CaptchaTypeAltcha {
		return captcha.VerifyAltcha(token, common.AltchaSigningKey())
	}
	if provider == common.CaptchaTypeMTCaptcha {
		return verifyMTCaptchaForFiles(common.MTCaptchaPrivateKey, token, clientIP)
	}

	if provider == common.CaptchaTypeImage {
		parts := strings.SplitN(token, ":", 2)
		if len(parts) == 2 {
			return captcha.Verify(parts[0], parts[1])
		}
		if captchaId != "" && captchaAns != "" {
			return captcha.Verify(captchaId, captchaAns)
		}
		return false
	}

	var verifyURL, secret string
	switch provider {
	case common.CaptchaTypeTurnstile:
		verifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"
		secret = common.TurnstileSecretKey
	case common.CaptchaTypeRecaptcha:
		verifyURL = "https://www.google.com/recaptcha/api/siteverify"
		secret = common.RecaptchaSecretKey
	case common.CaptchaTypeHCaptcha:
		verifyURL = "https://hcaptcha.com/siteverify"
		secret = common.HCaptchaSecretKey
	default:
		return false
	}

	if secret == "" {
		return false
	}

	resp, err := http.PostForm(verifyURL, url.Values{
		"secret":   {secret},
		"response": {token},
		"remoteip": {clientIP},
	})
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	var result struct {
		Success bool `json:"success"`
	}
	if err := common.DecodeJson(resp.Body, &result); err != nil {
		return false
	}
	return result.Success
}

func verifyManagedFileCaptcha(c *gin.Context) bool {
	token := c.PostForm("captcha_token")
	if token == "" {
		token = c.PostForm("cf-turnstile-response")
	}
	if token == "" {
		token = c.PostForm("g-recaptcha-response")
	}
	if token == "" {
		token = c.PostForm("h-captcha-response")
	}
	if token == "" {
		token = c.PostForm("mtcaptcha-response")
	}

	provider := c.PostForm("captcha_provider")
	captchaId := c.PostForm("captcha_id")
	captchaAns := c.PostForm("captcha_ans")

	return verifyManagedFileCaptchaToken(c.ClientIP(), provider, token, captchaId, captchaAns)
}

type VerifyFileAuthRequest struct {
	Path       string `json:"path"`
	Password   string `json:"password"`
	CaptchaId  string `json:"captcha_id"`
	CaptchaAns string `json:"captcha_ans"`
	Token      string `json:"captcha_token"`
	Provider   string `json:"captcha_provider"`
}

func VerifyManagedFileAuth(c *gin.Context) {
	var req VerifyFileAuthRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Invalid request body"})
		return
	}

	cleanPath, err := sanitizeRelPath(req.Path)
	if err != nil || cleanPath == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Invalid path"})
		return
	}

	mf, _ := findManagedFileWithProtection(cleanPath)
	if mf.Password == "" && !mf.EnableCaptcha {
		c.JSON(http.StatusOK, gin.H{"success": true})
		return
	}

	// Admin users can always unlock/verify files directly from the admin panel
	isAdmin := c.GetInt("role") >= common.RoleAdminUser || c.GetBool("is_admin")

	if !isAdmin {
		limiterKey := cleanPath + "|" + c.ClientIP()
		if fileAuthBlocked(limiterKey) {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "Too many failed attempts. Please try again later."})
			return
		}

		if mf.EnableCaptcha {
			if !verifyManagedFileCaptchaToken(c.ClientIP(), req.Provider, req.Token, req.CaptchaId, req.CaptchaAns) {
				fileAuthFailedAttempt(limiterKey)
				c.JSON(http.StatusOK, gin.H{"success": false, "message": "Captcha verification failed"})
				return
			}
		}

		if mf.Password != "" && !verifyFilePassword(req.Password, mf.Password) {
			fileAuthFailedAttempt(limiterKey)
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "Incorrect password"})
			return
		}
	}

	expiry := time.Now().Add(24 * time.Hour).Unix()
	authCookieKey := fileAuthCookieKey(cleanPath)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(authCookieKey, fileAuthSign(cleanPath, mf.Password, mf.UpdatedAt, expiry), 3600*24, "/", "", c.Request.TLS != nil, true)

	c.JSON(http.StatusOK, gin.H{"success": true})
}

type DownloadFileRequest struct {
	URL  string `json:"url"`
	Path string `json:"path"`
	Name string `json:"name"`
}

func DownloadFileFromURL(c *gin.Context) {
	var req DownloadFileRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Invalid request body"})
		return
	}
	req.URL = strings.TrimSpace(req.URL)
	if req.URL == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "URL is required"})
		return
	}
	if !strings.HasPrefix(req.URL, "http://") && !strings.HasPrefix(req.URL, "https://") {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Only HTTP/HTTPS URLs are allowed"})
		return
	}

	client := service.GetSSRFProtectedHTTPClient()
	httpReq, err := http.NewRequest(http.MethodGet, req.URL, nil)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	httpReq.Header.Set("User-Agent", "curl/8.4.0")

	resp, err := client.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": fmt.Sprintf("Failed to fetch URL: %v", err)})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": fmt.Sprintf("Remote server returned status: %d", resp.StatusCode)})
		return
	}

	// Limit to 50MB
	limitReader := io.LimitReader(resp.Body, 50*1024*1024)
	data, err := io.ReadAll(limitReader)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": fmt.Sprintf("Failed to read file: %v", err)})
		return
	}

	fileName := strings.TrimSpace(req.Name)
	if fileName == "" {
		parsedURL, err := url.Parse(req.URL)
		if err == nil {
			fileName = filepath.Base(parsedURL.Path)
		}
	}
	if fileName == "" || fileName == "." || fileName == "/" {
		fileName = fmt.Sprintf("download_%d.bin", time.Now().Unix())
	}

	targetRel := filepath.Join(req.Path, fileName)
	cleanRelPath, err := sanitizeRelPath(targetRel)
	if err != nil || cleanRelPath == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Invalid target path"})
		return
	}

	if isReadOnlyPath(cleanRelPath) {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Uploads directory is read-only"})
		return
	}

	fullPath := filepath.Join(managedFilesDir, cleanRelPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	if err := os.WriteFile(fullPath, data, 0644); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}

	now := common.GetTimestamp()
	var record model.ManagedFile
	if model.DB.Where("path = ?", cleanRelPath).First(&record).RowsAffected == 0 {
		model.DB.Create(&model.ManagedFile{
			Path:      cleanRelPath,
			Name:      fileName,
			IsDir:     false,
			Size:      int64(len(data)),
			CreatedAt: now,
			UpdatedAt: now,
		})
	} else {
		record.Size = int64(len(data))
		record.UpdatedAt = now
		model.DB.Save(&record)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"path": cleanRelPath,
			"size": len(data),
			"name": fileName,
		},
	})
}


func authCaptchaProviderOrder() []string {
	order := []string{}
	for _, provider := range strings.Split(common.OptionMap["CaptchaProviderOrder"], ",") {
		provider = strings.TrimSpace(strings.ToLower(provider))
		if provider == "" || provider == common.CaptchaTypeOff {
			continue
		}
		order = append(order, provider)
	}
	if len(order) == 0 {
		order = []string{"turnstile", "recaptcha", "hcaptcha", "mtcaptcha", "altcha", "image"}
	}
	return order
}

type authCaptchaSnippet struct {
	script string
	html   string
	key    string
}

func authCaptchaSnippetFor(provider string) (authCaptchaSnippet, bool) {
	switch provider {
	case common.CaptchaTypeTurnstile:
		if strings.TrimSpace(common.TurnstileSiteKey) == "" {
			return authCaptchaSnippet{}, false
		}
		return authCaptchaSnippet{
			script: `<script src="https://challenges.cloudflare.com/turnstile/v0/api.js" async defer></script>`,
			html:   fmt.Sprintf(`<div class="cf-turnstile" data-sitekey="%s"></div>`, common.TurnstileSiteKey),
			key:    common.TurnstileSiteKey,
		}, true
	case common.CaptchaTypeRecaptcha:
		if strings.TrimSpace(common.RecaptchaSiteKey) == "" {
			return authCaptchaSnippet{}, false
		}
		return authCaptchaSnippet{
			script: `<script src="https://www.google.com/recaptcha/api.js" async defer></script>`,
			html:   fmt.Sprintf(`<div class="g-recaptcha" data-sitekey="%s"></div>`, common.RecaptchaSiteKey),
			key:    common.RecaptchaSiteKey,
		}, true
	case common.CaptchaTypeHCaptcha:
		if strings.TrimSpace(common.HCaptchaSiteKey) == "" {
			return authCaptchaSnippet{}, false
		}
		return authCaptchaSnippet{
			script: `<script src="https://js.hcaptcha.com/1/api.js" async defer></script>`,
			html:   fmt.Sprintf(`<div class="h-captcha" data-sitekey="%s"></div>`, common.HCaptchaSiteKey),
			key:    common.HCaptchaSiteKey,
		}, true
	case common.CaptchaTypeMTCaptcha:
		if strings.TrimSpace(common.MTCaptchaSiteKey) == "" {
			return authCaptchaSnippet{}, false
		}
		return authCaptchaSnippet{
			script: fmt.Sprintf(`<script src="https://service.mtcaptcha.com/mtcv1/client/mtcaptcha.min.js" async defer></script>
<script>window.mtcaptchaConfig = { sitekey: "%s", "verify-callback": function(state) { var el = document.getElementById("mtcaptcha-response"); if (el) { el.value = state.verifiedToken || ""; } }, render: "auto" };</script>`, common.MTCaptchaSiteKey),
			html:   `<div class="mtcaptcha"></div><input type="hidden" name="mtcaptcha-response" id="mtcaptcha-response" value="" /><input type="hidden" name="captcha_provider" value="mtcaptcha" id="captcha_provider" />`,
			key:    common.MTCaptchaSiteKey,
		}, true
	case common.CaptchaTypeAltcha:
		// ALTCHA is self-hosted: the challenge is signed by this installation
		// and the proof-of-work happens in the visitor's browser.
		return authCaptchaSnippet{
			script: `<script src="https://cdn.jsdelivr.net/npm/altcha/dist/altcha.min.js" async defer></script>`,
			html:   `<altcha-widget challengeurl="/api/captcha/altcha" auto="onload" name="captcha_token"></altcha-widget><input type="hidden" name="captcha_provider" value="altcha" />`,
			key:    "altcha",
		}, true
	case common.CaptchaTypeImage:
		// Self-hosted image captcha: no keys required, always available.
		return authCaptchaSnippet{
			script: `<script>
			function refreshCaptcha() {
				fetch('/api/captcha/image').then(function (r) { return r.json(); }).then(function (d) {
					if (d && d.success && document.getElementById('captcha-img')) {
						document.getElementById('captcha-img').src = d.data.image;
						document.getElementById('captcha-id').value = d.data.captcha_id;
					}
				});
			}
			window.addEventListener('DOMContentLoaded', refreshCaptcha);
			</script>`,
			html:   `<input type="hidden" id="captcha-id" name="captcha_id" /><div class="flex items-center justify-center gap-2 mb-2"><img id="captcha-img" class="h-10 rounded border cursor-pointer" onclick="refreshCaptcha()" title="Click to refresh" src="" /></div><input type="text" name="captcha_ans" placeholder="Enter captcha code" class="w-full px-3 py-2 border rounded-lg bg-zinc-950/50 border-zinc-800 text-sm focus:outline-none focus:ring-2 focus:ring-primary" required /><input type="hidden" name="captcha_provider" value="image" />`,
			key:    "image",
		}, true
	default:
		return authCaptchaSnippet{}, false
	}
}


// pickRenderableAuthCaptchaSnippet selects the captcha to show on the file
// protection page. It prefers the administrator's effective global type,
// then any configured provider from the configured order, and finally the
// self-hosted altcha.
func pickRenderableAuthCaptchaSnippet() (authCaptchaSnippet, bool) {
	// Prefer configured third-party providers from the admin's order.
	// Altcha's widget requires an "obfuscation" plugin on the client side,
	// so it is excluded here: image always solves.
	for _, p := range authCaptchaProviderOrder() {
		if p == common.CaptchaTypeAltcha {
			continue
		}
		if snippet, ok := authCaptchaSnippetFor(p); ok {
			return snippet, true
		}
	}
	snippet, ok := authCaptchaSnippetFor(common.CaptchaTypeImage)
	if ok {
		return snippet, true
	}
	return authCaptchaSnippet{}, false
}

func renderAuthPage(c *gin.Context, relPath string, mf model.ManagedFile, errorMsg string) {
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")

	var captchaScript, captchaHTML string
	if mf.EnableCaptcha {
		snippet, ok := pickRenderableAuthCaptchaSnippet()
		if !ok {
			snippet, _ = authCaptchaSnippetFor(common.CaptchaTypeImage)
		}
		captchaScript = snippet.script
		captchaHTML = `<div class="captcha-box flex justify-center my-3">` + snippet.html + `</div>`
	}

	systemName := common.SystemName
	if systemName == "" {
		systemName = "New API"
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en" class="dark">
<head>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<title>Protected File - %s</title>
	<script src="https://cdn.tailwindcss.com"></script>
	%s
	<style>
		body { background-color: #09090b; color: #f4f4f5; font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; }
	</style>
</head>
<body class="flex min-h-screen items-center justify-center p-4">
	<div class="w-full max-w-sm rounded-2xl border border-zinc-800 bg-zinc-900/90 p-6 shadow-2xl backdrop-blur-xl">
		<div class="text-center mb-6">
			<div class="mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-xl bg-zinc-800/80 text-zinc-200 border border-zinc-700/50 shadow-inner">
				<svg xmlns="http://www.w3.org/2000/svg" width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="18" height="11" x="3" y="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg>
			</div>
			<h1 class="text-lg font-semibold tracking-tight text-white">%s</h1>
			<p class="text-xs text-zinc-400 mt-1 font-mono truncate">/%s</p>
		</div>

		%s

		<form method="POST" class="space-y-4">
			<input type="hidden" name="managed_file_auth" value="1" />
			%s
			%s
			<button type="submit" class="w-full rounded-lg bg-zinc-100 px-4 py-2.5 text-sm font-medium text-zinc-950 hover:bg-white transition-colors focus:outline-none focus:ring-2 focus:ring-zinc-400 focus:ring-offset-2 focus:ring-offset-zinc-900">
				Access File
			</button>
		</form>
	</div>
</body>
</html>`,
		relPath,
		captchaScript,
		systemName,
		relPath,
		func() string {
			if errorMsg != "" {
				return fmt.Sprintf(`<div class="mb-4 rounded-lg bg-red-500/10 p-3 text-center text-xs font-medium text-red-400 border border-red-500/20">%s</div>`, errorMsg)
			}
			return ""
		}(),
		func() string {
			if mf.Password != "" {
				return `<div class="space-y-1">
					<label class="text-xs font-medium text-zinc-300">Password</label>
					<input type="password" name="password" placeholder="Enter file password" class="w-full rounded-lg border border-zinc-800 bg-zinc-950/60 px-3 py-2 text-sm text-zinc-100 placeholder-zinc-500 focus:border-zinc-600 focus:outline-none focus:ring-1 focus:ring-zinc-600" required />
				</div>`
			}
			return ""
		}(),
		captchaHTML,
	)

	c.String(http.StatusOK, html)
}
