package controller

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/apkbrand"
)

// The launcher of Android shows the application label and the launcher icon of the
// installed package. Both are part of the APK, therefore the branding of the current
// installation (system name and favicon) is written into the archive right before the
// download and the result is signed again.
//
// Rebuilding the archive is expensive, so the result is cached until the base APK, the
// label or the icon change.
var (
	byogApkCacheMu   sync.Mutex
	byogApkCacheKey  string
	byogApkCacheData []byte
)

// byogApkSigner returns the key that signs the worker APK. The key is stored next to
// the database so that it stays stable across restarts, otherwise the installed app
// could not be updated in place.
func byogApkSigner() (*apkbrand.Signer, error) {
	return apkbrand.CachedSigner(byogApkSigningKeyPath())
}

// byogApkSigningKeyPath returns the location of the APK signing key.
//
// The key of the repository is used so that the APK the server hands out carries the
// same signer as the APK that was built from the sources. An application can only be
// updated when the signer stays the same, otherwise the installation fails with
// "App not installed". The location can be overwritten with BYOG_APK_SIGNING_KEY.
func byogApkSigningKeyPath() string {
	if path := strings.TrimSpace(os.Getenv("BYOG_APK_SIGNING_KEY")); path != "" {
		return path
	}

	candidates := []string{
		filepath.Join("worker", "mobile", "apk_signing_key.pem"),
		filepath.Join("..", "worker", "mobile", "apk_signing_key.pem"),
		filepath.Join("/worker", "mobile", "apk_signing_key.pem"),
		filepath.Join("/data", "worker", "mobile", "apk_signing_key.pem"),
	}
	if execPath, err := os.Executable(); err == nil {
		base := filepath.Dir(execPath)
		candidates = append(candidates,
			filepath.Join(base, "worker", "mobile", "apk_signing_key.pem"),
			filepath.Join(base, "..", "worker", "mobile", "apk_signing_key.pem"),
		)
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}

	// Without the shared key a new one is created next to the database. The
	// application then has to be installed again once.
	directory := "."
	sqlitePath := common.SQLitePath
	if index := strings.Index(sqlitePath, "?"); index >= 0 {
		sqlitePath = sqlitePath[:index]
	}
	if sqlitePath != "" && sqlitePath != ":memory:" {
		directory = filepath.Dir(sqlitePath)
	}
	return filepath.Join(directory, "byog_apk_signing_key.pem")
}

// brandByogApk rewrites the launcher label and icon of the APK and signs it again.
func brandByogApk(raw []byte, label string, faviconBase64 string) ([]byte, error) {
	if len(raw) == 0 {
		return nil, errors.New("empty apk")
	}
	if strings.TrimSpace(label) == "" {
		label = "BYOG Worker"
	}

	icon, err := byogApkIcon(faviconBase64)
	if err != nil {
		common.SysError(fmt.Sprintf("[BYOG] launcher icon could not be rendered: %v", err))
	}

	key := fmt.Sprintf("%x|%s|%x", sha256.Sum256(raw), label, sha256.Sum256(icon))
	byogApkCacheMu.Lock()
	if byogApkCacheKey == key && len(byogApkCacheData) > 0 {
		cached := byogApkCacheData
		byogApkCacheMu.Unlock()
		return cached, nil
	}
	byogApkCacheMu.Unlock()

	signer, err := byogApkSigner()
	if err != nil {
		return nil, err
	}
	branded, err := apkbrand.Rebrand(raw, apkbrand.Branding{Label: label, IconPNG: icon}, signer)
	if err != nil {
		return nil, err
	}

	byogApkCacheMu.Lock()
	byogApkCacheKey = key
	byogApkCacheData = branded
	byogApkCacheMu.Unlock()
	return branded, nil
}

// byogApkIcon converts the favicon of the installation into a launcher icon.
func byogApkIcon(faviconBase64 string) ([]byte, error) {
	payload := strings.TrimSpace(faviconBase64)
	if payload == "" {
		return nil, errors.New("no favicon configured")
	}
	if index := strings.Index(payload, ","); index >= 0 && strings.HasPrefix(payload, "data:") {
		payload = payload[index+1:]
	}
	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, err
	}
	return apkbrand.RenderIcon(raw, 192)
}
