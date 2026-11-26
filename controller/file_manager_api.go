package controller

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

const (
	maxFileManagerAPIUploadBytes = 200 << 20 // 200 MB upload cap
	maxFileManagerDBBlobBytes    = 10 << 20  // <= 10 MB stored in DB; larger mirrored on disk in managed_files/
)

func buildFileManagerPublicURL(c *gin.Context, relPath string) string {
	clean := strings.TrimPrefix(filepath.ToSlash(relPath), "/")
	if clean == "" {
		return resolveExternalBaseURL(c)
	}
	segments := strings.Split(clean, "/")
	for i, seg := range segments {
		segments[i] = url.PathEscape(seg)
	}
	encodedPath := "/" + strings.Join(segments, "/")
	if base := resolveExternalBaseURL(c); base != "" {
		return base + encodedPath
	}
	return encodedPath
}

func buildFileManagerDownloadURL(c *gin.Context, relPath string) string {
	clean := strings.TrimPrefix(filepath.ToSlash(relPath), "/")
	q := "/api/v1/files/download?path=" + url.QueryEscape(clean)
	if base := resolveExternalBaseURL(c); base != "" {
		return base + q
	}
	return q
}

func serializeManagedFileAPIItem(c *gin.Context, f model.ManagedFile) gin.H {
	item := gin.H{
		"id":             f.Id,
		"path":           f.Path,
		"name":           f.Name,
		"is_dir":         f.IsDir,
		"size":           f.Size,
		"url":            buildFileManagerPublicURL(c, f.Path),
		"has_password":   strings.TrimSpace(f.Password) != "",
		"enable_captcha": f.EnableCaptcha,
		"created_at":     f.CreatedAt,
		"updated_at":     f.UpdatedAt,
	}
	if !f.IsDir {
		item["download_url"] = buildFileManagerDownloadURL(c, f.Path)
	}
	return item
}

// ensureManagedParentDirs creates missing directory rows for every parent segment
// of relPath (e.g. for "a/b/c.png", ensures "a" and "a/b" exist as IsDir=true rows).
func ensureManagedParentDirs(relPath string) {
	dir := filepath.ToSlash(filepath.Dir(relPath))
	if dir == "." || dir == "/" || dir == "" {
		return
	}
	parts := strings.Split(dir, "/")
	now := common.GetTimestamp()
	acc := ""
	for _, p := range parts {
		if p == "" {
			continue
		}
		if acc == "" {
			acc = p
		} else {
			acc += "/" + p
		}
		var existing model.ManagedFile
		if model.DB.Where("path = ?", acc).First(&existing).RowsAffected == 0 {
			_ = model.DB.Create(&model.ManagedFile{
				Path:      acc,
				Name:      p,
				IsDir:     true,
				CreatedAt: now,
				UpdatedAt: now,
			}).Error
		}
	}
}

func saveManagedFileViaAPI(cleanPath string, data []byte) (*model.ManagedFile, error) {
	ensureManagedParentDirs(cleanPath)

	size := int64(len(data))
	now := common.GetTimestamp()
	diskFullPath := filepath.Join(managedFilesDir, filepath.FromSlash(cleanPath))

	var dbContent []byte
	if size <= maxFileManagerDBBlobBytes {
		dbContent = data
		// Remove any stale disk file so DB remains the single source of truth for <= 10MB files
		_ = os.Remove(diskFullPath)
	} else {
		if err := os.MkdirAll(filepath.Dir(diskFullPath), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(diskFullPath, data, 0o644); err != nil {
			return nil, err
		}
	}

	var rec model.ManagedFile
	res := model.DB.Where("path = ?", cleanPath).First(&rec)
	if res.RowsAffected == 0 {
		rec = model.ManagedFile{
			Path:      cleanPath,
			Name:      filepath.Base(cleanPath),
			IsDir:     false,
			Size:      size,
			Content:   dbContent,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := model.DB.Create(&rec).Error; err != nil {
			return nil, err
		}
	} else {
		rec.Name = filepath.Base(cleanPath)
		rec.IsDir = false
		rec.Size = size
		rec.Content = dbContent
		rec.UpdatedAt = now
		if err := model.DB.Save(&rec).Error; err != nil {
			return nil, err
		}
	}
	return &rec, nil
}

func resolveManagedFileDiskPath(cleanPath string) string {
	if strings.HasPrefix(cleanPath, "uploads/") || cleanPath == "uploads" {
		return filepath.FromSlash(cleanPath)
	}
	return filepath.Join(managedFilesDir, filepath.FromSlash(cleanPath))
}

func readManagedFileBytesViaAPI(cleanPath string) ([]byte, *model.ManagedFile, error) {
	var rec model.ManagedFile
	found := model.DB.Where("path = ? OR path = ?", cleanPath, "/"+cleanPath).First(&rec).RowsAffected > 0
	if found && rec.IsDir {
		return nil, &rec, fmt.Errorf("path is a directory")
	}
	if found && len(rec.Content) > 0 {
		return rec.Content, &rec, nil
	}
	diskPath := resolveManagedFileDiskPath(cleanPath)
	raw, err := os.ReadFile(diskPath)
	if err != nil {
		if found {
			// Empty 0-byte file stored in DB
			return []byte{}, &rec, nil
		}
		return nil, nil, os.ErrNotExist
	}
	if !found {
		rec = model.ManagedFile{
			Path: cleanPath,
			Name: filepath.Base(cleanPath),
			Size: int64(len(raw)),
		}
	}
	return raw, &rec, nil
}

// FileManagerAPIList lists managed files/folders inside an optional folder path.
// GET /api/v1/files?path=folder/sub&recursive=false
func FileManagerAPIList(c *gin.Context) {
	ensureUploadFoldersExist()
	syncDiskDirToDB(managedFilesDir, "")
	if _, err := os.Stat("uploads"); err == nil {
		syncDiskDirToDB("uploads", "uploads")
	}

	rawDir := c.Query("path")
	cleanDir, err := sanitizeRelPath(rawDir)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid path"})
		return
	}
	recursive := strings.EqualFold(strings.TrimSpace(c.Query("recursive")), "true") || c.Query("recursive") == "1"

	var files []model.ManagedFile
	q := model.DB.Order("is_dir DESC, path ASC")
	if cleanDir != "" {
		q = q.Where("path LIKE ?", cleanDir+"/%")
	}
	if err := q.Find(&files).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	items := make([]gin.H, 0, len(files))
	for _, f := range files {
		if !recursive {
			if cleanDir == "" {
				if strings.Contains(f.Path, "/") {
					continue
				}
			} else {
				sub := strings.TrimPrefix(f.Path, cleanDir+"/")
				if strings.Contains(sub, "/") {
					continue
				}
			}
		}
		items = append(items, serializeManagedFileAPIItem(c, f))
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"path":      cleanDir,
			"recursive": recursive,
			"total":     len(items),
			"items":     items,
		},
	})
}

// FileManagerAPIInfo returns metadata and direct public/download URLs for a path.
// GET /api/v1/files/info?path=folder/file.png
func FileManagerAPIInfo(c *gin.Context) {
	cleanPath, err := sanitizeRelPath(c.Query("path"))
	if err != nil || cleanPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Query parameter 'path' is required"})
		return
	}
	var rec model.ManagedFile
	if model.DB.Where("path = ? OR path = ?", cleanPath, "/"+cleanPath).First(&rec).RowsAffected == 0 {
		// Check disk fallback
		diskPath := resolveManagedFileDiskPath(cleanPath)
		info, statErr := os.Stat(diskPath)
		if statErr != nil {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "File or folder not found"})
			return
		}
		rec = model.ManagedFile{
			Path:      cleanPath,
			Name:      info.Name(),
			IsDir:     info.IsDir(),
			Size:      info.Size(),
			UpdatedAt: info.ModTime().Unix(),
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    serializeManagedFileAPIItem(c, rec),
	})
}

// FileManagerAPIDownload streams a managed file as a download attachment.
// GET /api/v1/files/download?path=folder/file.bin
func FileManagerAPIDownload(c *gin.Context) {
	cleanPath, err := sanitizeRelPath(c.Query("path"))
	if err != nil || cleanPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Query parameter 'path' is required"})
		return
	}
	var rec model.ManagedFile
	found := model.DB.Where("path = ? OR path = ?", cleanPath, "/"+cleanPath).First(&rec).RowsAffected > 0
	if found && rec.IsDir {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Cannot download a directory"})
		return
	}
	filename := filepath.Base(cleanPath)
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filename))
	if found && len(rec.Content) > 0 {
		c.Data(http.StatusOK, http.DetectContentType(rec.Content), rec.Content)
		return
	}
	diskPath := resolveManagedFileDiskPath(cleanPath)
	if info, statErr := os.Stat(diskPath); statErr == nil && !info.IsDir() {
		http.ServeFile(c.Writer, c.Request, diskPath)
		return
	}
	if found {
		c.Data(http.StatusOK, "application/octet-stream", []byte{})
		return
	}
	c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "File not found"})
}

// FileManagerAPIReadContent reads a text file's contents.
// GET /api/v1/files/content?path=docs/readme.md
func FileManagerAPIReadContent(c *gin.Context) {
	cleanPath, err := sanitizeRelPath(c.Query("path"))
	if err != nil || cleanPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Query parameter 'path' is required"})
		return
	}
	raw, rec, err := readManagedFileBytesViaAPI(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "File not found"})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"path":         cleanPath,
			"name":         rec.Name,
			"size":         len(raw),
			"content":      string(raw),
			"url":          buildFileManagerPublicURL(c, cleanPath),
			"download_url": buildFileManagerDownloadURL(c, cleanPath),
		},
	})
}

// FileManagerAPIWriteContent creates or updates a text file at any path.
// PUT/POST /api/v1/files/content  {"path": "docs/guide.md", "content": "..."}
func FileManagerAPIWriteContent(c *gin.Context) {
	var req struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := decodeLargeJSON(c.Request.Body, maxBlogAPIJSONBytes, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	cleanPath, err := sanitizeRelPath(req.Path)
	if err != nil || cleanPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Valid 'path' is required"})
		return
	}
	if isReadOnlyPath(cleanPath) {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "The 'uploads' system directory is read-only"})
		return
	}
	rec, err := saveManagedFileViaAPI(cleanPath, []byte(req.Content))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    serializeManagedFileAPIItem(c, *rec),
	})
}

// FileManagerAPIUpload uploads a file to any folder/path with any desired filename.
// Supports both multipart/form-data (file + optional path / dir / folder / filename)
// and application/json ({path, dir, filename, content_base64 | content}).
// POST /api/v1/files/upload
func FileManagerAPIUpload(c *gin.Context) {
	ct := strings.ToLower(c.GetHeader("Content-Type"))
	if strings.HasPrefix(ct, "application/json") {
		var req struct {
			Path          string `json:"path"`
			Dir           string `json:"dir"`
			Folder        string `json:"folder"`
			Filename      string `json:"filename"`
			ContentBase64 string `json:"content_base64"`
			Content       string `json:"content"`
		}
		if err := decodeLargeJSON(c.Request.Body, maxBlogAPIJSONBytes, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
			return
		}
		targetRaw := strings.TrimSpace(req.Path)
		if targetRaw == "" {
			dirPart := strings.TrimSpace(req.Dir)
			if dirPart == "" {
				dirPart = strings.TrimSpace(req.Folder)
			}
			fname := strings.TrimSpace(req.Filename)
			if fname == "" {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Provide 'path' or 'filename'"})
				return
			}
			if dirPart != "" {
				targetRaw = strings.Trim(dirPart, "/") + "/" + strings.TrimPrefix(fname, "/")
			} else {
				targetRaw = fname
			}
		}
		cleanPath, err := sanitizeRelPath(targetRaw)
		if err != nil || cleanPath == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid target path"})
			return
		}
		if isReadOnlyPath(cleanPath) {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "The 'uploads' system directory is read-only"})
			return
		}
		var data []byte
		if req.ContentBase64 != "" {
			decoded, decErr := base64.StdEncoding.DecodeString(req.ContentBase64)
			if decErr != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid base64 content: " + decErr.Error()})
				return
			}
			data = decoded
		} else {
			data = []byte(req.Content)
		}
		rec, err := saveManagedFileViaAPI(cleanPath, data)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{
			"success": true,
			"data":    serializeManagedFileAPIItem(c, *rec),
		})
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxFileManagerAPIUploadBytes)
	fh, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Missing multipart field 'file': " + err.Error()})
		return
	}
	if fh.Size > maxFileManagerAPIUploadBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"success": false, "message": "File exceeds 200 MB limit"})
		return
	}

	targetRaw := strings.TrimSpace(c.PostForm("path"))
	if targetRaw == "" {
		dirPart := strings.TrimSpace(c.PostForm("dir"))
		if dirPart == "" {
			dirPart = strings.TrimSpace(c.PostForm("folder"))
		}
		filename := strings.TrimSpace(c.PostForm("filename"))
		if filename == "" {
			filename = strings.TrimSpace(c.PostForm("name"))
		}
		if filename == "" {
			filename = filepath.Base(strings.ReplaceAll(fh.Filename, "\\", "/"))
		}
		if dirPart != "" {
			targetRaw = strings.Trim(dirPart, "/") + "/" + strings.TrimPrefix(filename, "/")
		} else {
			targetRaw = filename
		}
	}

	cleanPath, err := sanitizeRelPath(targetRaw)
	if err != nil || cleanPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid target path"})
		return
	}
	if isReadOnlyPath(cleanPath) {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "The 'uploads' system directory is read-only"})
		return
	}

	f, err := fh.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, maxFileManagerAPIUploadBytes+1))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	if int64(len(data)) > maxFileManagerAPIUploadBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"success": false, "message": "File exceeds 200 MB limit"})
		return
	}

	rec, err := saveManagedFileViaAPI(cleanPath, data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    serializeManagedFileAPIItem(c, *rec),
	})
}

// FileManagerAPIMkdir creates a directory (including any missing parent folders).
// POST /api/v1/files/mkdir  {"path": "assets/blog/2026"}
func FileManagerAPIMkdir(c *gin.Context) {
	var req struct {
		Path string `json:"path"`
	}
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid JSON body"})
		return
	}
	cleanPath, err := sanitizeRelPath(req.Path)
	if err != nil || cleanPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Valid 'path' is required"})
		return
	}
	if isReadOnlyPath(cleanPath) {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "The 'uploads' system directory is read-only"})
		return
	}
	ensureManagedParentDirs(cleanPath)
	now := common.GetTimestamp()
	var rec model.ManagedFile
	if model.DB.Where("path = ?", cleanPath).First(&rec).RowsAffected == 0 {
		rec = model.ManagedFile{
			Path:      cleanPath,
			Name:      filepath.Base(cleanPath),
			IsDir:     true,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := model.DB.Create(&rec).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
			return
		}
	}
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    serializeManagedFileAPIItem(c, rec),
	})
}

// FileManagerAPIRename renames or moves a file or folder (recursively updating all child paths).
// POST /api/v1/files/rename  {"old_path": "a/old.txt", "new_path": "b/new.txt"}
func FileManagerAPIRename(c *gin.Context) {
	var req struct {
		OldPath string `json:"old_path"`
		NewPath string `json:"new_path"`
		From    string `json:"from"`
		To      string `json:"to"`
	}
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid JSON body"})
		return
	}
	if req.OldPath == "" {
		req.OldPath = req.From
	}
	if req.NewPath == "" {
		req.NewPath = req.To
	}
	oldClean, err1 := sanitizeRelPath(req.OldPath)
	newClean, err2 := sanitizeRelPath(req.NewPath)
	if err1 != nil || err2 != nil || oldClean == "" || newClean == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Valid 'old_path' and 'new_path' are required"})
		return
	}
	if isReadOnlyPath(oldClean) || isReadOnlyPath(newClean) {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "The 'uploads' system directory is read-only"})
		return
	}
	if oldClean == newClean {
		var rec model.ManagedFile
		if model.DB.Where("path = ?", oldClean).First(&rec).RowsAffected == 0 {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Source path not found"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "data": serializeManagedFileAPIItem(c, rec)})
		return
	}
	if strings.HasPrefix(newClean, oldClean+"/") {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Cannot move a directory inside itself"})
		return
	}

	var rec model.ManagedFile
	if model.DB.Where("path = ? OR path = ?", oldClean, "/"+oldClean).First(&rec).RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Source path not found"})
		return
	}

	ensureManagedParentDirs(newClean)
	now := common.GetTimestamp()

	rec.Path = newClean
	rec.Name = filepath.Base(newClean)
	rec.UpdatedAt = now
	if err := model.DB.Save(&rec).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	// If this was a directory, recursively rename all child entries in DB.
	if rec.IsDir {
		var children []model.ManagedFile
		if err := model.DB.Where("path LIKE ?", oldClean+"/%").Find(&children).Error; err == nil {
			prefix := oldClean + "/"
			for i := range children {
				suffix := strings.TrimPrefix(children[i].Path, prefix)
				children[i].Path = newClean + "/" + suffix
				children[i].Name = filepath.Base(children[i].Path)
				children[i].UpdatedAt = now
				_ = model.DB.Save(&children[i]).Error
			}
		}
	}

	oldDisk := filepath.Join(managedFilesDir, filepath.FromSlash(oldClean))
	newDisk := filepath.Join(managedFilesDir, filepath.FromSlash(newClean))
	if _, statErr := os.Stat(oldDisk); statErr == nil {
		_ = os.MkdirAll(filepath.Dir(newDisk), 0o755)
		_ = os.Rename(oldDisk, newDisk)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    serializeManagedFileAPIItem(c, rec),
	})
}

// FileManagerAPIDelete removes a file or folder (recursively) from DB and disk.
// DELETE /api/v1/files?path=folder/file.png
func FileManagerAPIDelete(c *gin.Context) {
	cleanPath, err := sanitizeRelPath(c.Query("path"))
	if err != nil || cleanPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Query parameter 'path' is required"})
		return
	}
	if isReadOnlyPath(cleanPath) {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "The 'uploads' system directory is read-only"})
		return
	}

	res := model.DB.Where("path = ? OR path = ? OR path LIKE ? OR path LIKE ?", cleanPath, "/"+cleanPath, cleanPath+"/%", "/"+cleanPath+"/%").Delete(&model.ManagedFile{})
	diskPath := filepath.Join(managedFilesDir, filepath.FromSlash(cleanPath))
	diskExisted := false
	if _, statErr := os.Stat(diskPath); statErr == nil {
		diskExisted = true
		_ = os.RemoveAll(diskPath)
	}
	if res.RowsAffected == 0 && !diskExisted {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "File or folder not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "deleted",
		"path":    cleanPath,
	})
}
