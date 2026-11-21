package service

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

var (
	cachedFFmpegPath  string
	cachedFFprobePath string
	ffmpegLookupMu    sync.Mutex
	ffmpegDownloadMu  sync.Mutex
)

// FindFFmpeg searches for the ffmpeg executable in PATH, standard system paths,
// and local bin directories.
func FindFFmpeg() string {
	ffmpegLookupMu.Lock()
	defer ffmpegLookupMu.Unlock()

	if cachedFFmpegPath != "" {
		if _, err := os.Stat(cachedFFmpegPath); err == nil {
			return cachedFFmpegPath
		}
	}

	exeName := "ffmpeg"
	if runtime.GOOS == "windows" {
		exeName = "ffmpeg.exe"
	}

	// 1. Explicit environment variable override
	if env := os.Getenv("FFMPEG_PATH"); env != "" {
		if _, err := os.Stat(env); err == nil {
			cachedFFmpegPath = env
			return cachedFFmpegPath
		}
	}

	// 2. System PATH
	if p, err := exec.LookPath(exeName); err == nil {
		cachedFFmpegPath = p
		return cachedFFmpegPath
	}

	// 3. Local app directory candidates only (self-contained within app)
	candidates := []string{
		exeName,
		filepath.Join(".", exeName),
		filepath.Join("bin", exeName),
		filepath.Join("data", "bin", exeName),
		filepath.Join("uploads", "bin", exeName),
	}

	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			cachedFFmpegPath = c
			return cachedFFmpegPath
		}
	}

	return ""
}

// FindFFprobe searches for the ffprobe executable.
func FindFFprobe() string {
	ffmpegLookupMu.Lock()
	defer ffmpegLookupMu.Unlock()

	if cachedFFprobePath != "" {
		if _, err := os.Stat(cachedFFprobePath); err == nil {
			return cachedFFprobePath
		}
	}

	exeName := "ffprobe"
	if runtime.GOOS == "windows" {
		exeName = "ffprobe.exe"
	}

	if env := os.Getenv("FFPROBE_PATH"); env != "" {
		if _, err := os.Stat(env); err == nil {
			cachedFFprobePath = env
			return cachedFFprobePath
		}
	}

	if p, err := exec.LookPath(exeName); err == nil {
		cachedFFprobePath = p
		return cachedFFprobePath
	}

	// 3. Local app directory candidates only (self-contained within app)
	candidates := []string{
		exeName,
		filepath.Join(".", exeName),
		filepath.Join("bin", exeName),
		filepath.Join("data", "bin", exeName),
		filepath.Join("uploads", "bin", exeName),
	}

	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			cachedFFprobePath = c
			return cachedFFprobePath
		}
	}

	return ""
}

// EnsureFFmpeg ensures an ffmpeg binary is available, attempting to auto-download
// a minimal standalone static binary to uploads/bin if absent from the host.
func EnsureFFmpeg(ctx context.Context) (string, error) {
	if p := FindFFmpeg(); p != "" {
		return p, nil
	}

	ffmpegDownloadMu.Lock()
	defer ffmpegDownloadMu.Unlock()

	// Double-check after acquiring lock
	if p := FindFFmpeg(); p != "" {
		return p, nil
	}

	// Prepare target dir
	targetDir := filepath.Join("uploads", "bin")
	_ = os.MkdirAll(targetDir, 0755)

	exeName := "ffmpeg"
	if runtime.GOOS == "windows" {
		exeName = "ffmpeg.exe"
	}
	targetPath := filepath.Join(targetDir, exeName)

	if info, err := os.Stat(targetPath); err == nil && info.Size() > 0 {
		cachedFFmpegPath = targetPath
		return targetPath, nil
	}

	// Resolve download URL based on OS and architecture
	var downloadURL string
	switch runtime.GOOS {
	case "linux":
		if runtime.GOARCH == "arm64" {
			downloadURL = "https://github.com/eugeneware/ffmpeg-static/releases/download/b6.1.1/ffmpeg-linux-arm64.gz"
		} else {
			downloadURL = "https://github.com/eugeneware/ffmpeg-static/releases/download/b6.1.1/ffmpeg-linux-x64.gz"
		}
	case "windows":
		downloadURL = "https://github.com/eugeneware/ffmpeg-static/releases/download/b6.1.1/ffmpeg-win32-x64.gz"
	case "darwin":
		if runtime.GOARCH == "arm64" {
			downloadURL = "https://github.com/eugeneware/ffmpeg-static/releases/download/b6.1.1/ffmpeg-darwin-arm64.gz"
		} else {
			downloadURL = "https://github.com/eugeneware/ffmpeg-static/releases/download/b6.1.1/ffmpeg-darwin-x64.gz"
		}
	default:
		return "", fmt.Errorf("no automated ffmpeg download available for OS %s", runtime.GOOS)
	}

	common.SysLog(fmt.Sprintf("ffmpeg not found on host, downloading static binary from %s to %s...", downloadURL, targetPath))

	reqCtx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create download request: %w", err)
	}
	req.Header.Set("User-Agent", "curl/8.4.0")

	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download ffmpeg binary: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ffmpeg download server returned status %d", resp.StatusCode)
	}

	gzReader, err := gzip.NewReader(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	tmpPath := fmt.Sprintf("%s.%s.tmp", targetPath, common.GetRandomString(6))
	outFile, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create temp ffmpeg file: %w", err)
	}

	if _, err := io.Copy(outFile, gzReader); err != nil {
		_ = outFile.Close()
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("failed to decompress ffmpeg binary: %w", err)
	}
	_ = outFile.Close()
	_ = os.Chmod(tmpPath, 0755)

	if err := os.Rename(tmpPath, targetPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("failed to install ffmpeg binary: %w", err)
	}
	_ = os.Chmod(targetPath, 0755)

	common.SysLog(fmt.Sprintf("ffmpeg successfully installed to %s", targetPath))
	cachedFFmpegPath = targetPath
	return targetPath, nil
}
