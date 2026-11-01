package controller

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type HfRepoItem struct {
	Id        string `json:"id"`
	Author    string `json:"author"`
	Downloads int    `json:"downloads"`
	Likes     int    `json:"likes"`
	Gated     bool   `json:"gated"`
	Private   bool   `json:"private"`
}

type HfFileDetail struct {
	Filename       string  `json:"filename"`
	Size           int64   `json:"size"`
	SizeMB         int     `json:"size_mb"`
	SizeGB         float64 `json:"size_gb"`
	DownloadUrl    string  `json:"download_url"`
	Quantization   string  `json:"quantization"`
	RecommendedRam float64 `json:"recommended_ram_gb"`
	RecommendedVram int    `json:"recommended_vram_gb"`
	RecommendedCores int   `json:"recommended_cores"`
}

// SearchHuggingFaceModels proxies model search requests to Hugging Face API
func SearchHuggingFaceModels(c *gin.Context) {
	query := strings.TrimSpace(c.Query("q"))
	if query == "" {
		query = "gguf"
	}

	hfApiUrl := fmt.Sprintf("https://huggingface.co/api/models?search=%s&limit=30&full=false", url.QueryEscape(query))
	client := &http.Client{Timeout: 12 * time.Second}

	req, err := http.NewRequest("GET", hfApiUrl, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	req.Header.Set("User-Agent", "BYOG-HF-Explorer/1.0")

	token := strings.TrimSpace(c.Query("token"))
	if token != "" && strings.HasPrefix(token, "hf_") {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": "Hugging Face bağlantı hatası: " + err.Error()})
		return
	}

	// Hugging Face search is public: if a custom token resulted in 401/403, retry anonymously
	if (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden) && token != "" {
		resp.Body.Close()
		reqNoAuth, errNoAuth := http.NewRequest("GET", hfApiUrl, nil)
		if errNoAuth == nil {
			reqNoAuth.Header.Set("User-Agent", "BYOG-HF-Explorer/1.0")
			if retryResp, retryErr := client.Do(reqNoAuth); retryErr == nil {
				resp = retryResp
			}
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"success": false, "message": fmt.Sprintf("Hugging Face API yanıtı: %d", resp.StatusCode)})
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	var rawRepos []map[string]interface{}
	if err := json.Unmarshal(body, &rawRepos); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "JSON ayrıştırma hatası"})
		return
	}

	var results []HfRepoItem
	for _, r := range rawRepos {
		repoId, _ := r["id"].(string)
		if repoId == "" {
			continue
		}
		downloads := 0
		if dl, ok := r["downloads"].(float64); ok {
			downloads = int(dl)
		}
		likes := 0
		if lk, ok := r["likes"].(float64); ok {
			likes = int(lk)
		}
		isGated := false
		if gVal, ok := r["gated"]; ok && gVal != nil {
			if gb, ok := gVal.(bool); ok {
				isGated = gb
			} else if gs, ok := gVal.(string); ok && gs != "" && gs != "false" {
				isGated = true
			}
		}
		isPrivate := false
		if pVal, ok := r["private"].(bool); ok {
			isPrivate = pVal
		}

		parts := strings.Split(repoId, "/")
		author := ""
		if len(parts) > 1 {
			author = parts[0]
		}

		results = append(results, HfRepoItem{
			Id:        repoId,
			Author:    author,
			Downloads: downloads,
			Likes:     likes,
			Gated:     isGated,
			Private:   isPrivate,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    results,
	})
}

// GetHuggingFaceModelFiles retrieves available quantizations/GGUF files and recommended hardware
func GetHuggingFaceModelFiles(c *gin.Context) {
	repoId := strings.TrimSpace(c.Query("repo"))
	if repoId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "repo parametresi gereklidir"})
		return
	}

	token := strings.TrimSpace(c.Query("token"))
	hfApiUrl := fmt.Sprintf("https://huggingface.co/api/models/%s", repoId)

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", hfApiUrl, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	req.Header.Set("User-Agent", "BYOG-HF-Explorer/1.0")
	if token != "" && strings.HasPrefix(token, "hf_") {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": "Hugging Face bağlantı hatası: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"gated":   true,
			"message": "Bu model Hugging Face üzerinde gated (erişim şartı onaylanması gereken) bir modeldir. Lütfen erişim izni olan Hugging Face User Access Token girin.",
		})
		return
	}

	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"success": false, "message": fmt.Sprintf("Hugging Face API yanıtı: %d", resp.StatusCode)})
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	var repoDetail map[string]interface{}
	if err := json.Unmarshal(body, &repoDetail); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "JSON ayrıştırma hatası"})
		return
	}

	isGated := false
	if gVal, ok := repoDetail["gated"]; ok && gVal != nil {
		if gb, ok := gVal.(bool); ok {
			isGated = gb
		} else if gs, ok := gVal.(string); ok && gs != "" && gs != "false" {
			isGated = true
		}
	}

	siblings, _ := repoDetail["siblings"].([]interface{})
	var files []HfFileDetail

	quantRe := regexp.MustCompile(`(?i)(q[0-9]_[k|m|s|0-9|_]+|bf16|fp16|fp32|f16|f32)`)

	for _, s := range siblings {
		sibMap, ok := s.(map[string]interface{})
		if !ok {
			continue
		}
		filename, _ := sibMap["rfilename"].(string)
		if filename == "" {
			continue
		}

		lower := strings.ToLower(filename)
		if !strings.HasSuffix(lower, ".gguf") && !strings.HasSuffix(lower, ".bin") && !strings.HasSuffix(lower, ".safetensors") {
			continue
		}

		// Estimate or detect size from siblings or HEAD request
		var fileSize int64 = 0
		if sz, ok := sibMap["size"].(float64); ok && sz > 0 {
			fileSize = int64(sz)
		}

		downloadUrl := fmt.Sprintf("https://huggingface.co/%s/resolve/main/%s", repoId, filename)

		// Quantization detection
		quant := "Standard"
		if match := quantRe.FindString(filename); match != "" {
			quant = strings.ToUpper(match)
		}

		sizeMB := int(fileSize / (1024 * 1024))
		sizeGB := math.Round((float64(fileSize)/(1024.0*1024.0*1024.0))*10.0) / 10.0
		if sizeGB <= 0 {
			// Estimate based on common quant size if not specified in siblings
			if strings.Contains(lower, "0.5b") {
				sizeGB = 0.5
			} else if strings.Contains(lower, "1.5b") {
				sizeGB = 1.2
			} else if strings.Contains(lower, "3b") || strings.Contains(lower, "4b") {
				sizeGB = 2.8
			} else if strings.Contains(lower, "7b") || strings.Contains(lower, "8b") {
				sizeGB = 4.8
			} else if strings.Contains(lower, "14b") {
				sizeGB = 9.2
			} else {
				sizeGB = 4.0
			}
			sizeMB = int(sizeGB * 1024)
			fileSize = int64(sizeMB) * 1024 * 1024
		}

		// Recommended specs
		recRam := math.Max(math.Ceil((sizeGB*1.25+1.0)*2.0)/2.0, 4.0) // 0.5 steps
		recVram := int(math.Max(math.Ceil(sizeGB+1.0), 2.0))
		recCores := 4
		if sizeGB > 6.0 {
			recCores = 6
		}
		if sizeGB > 12.0 {
			recCores = 8
		}

		files = append(files, HfFileDetail{
			Filename:         filename,
			Size:             fileSize,
			SizeMB:           sizeMB,
			SizeGB:           sizeGB,
			DownloadUrl:      downloadUrl,
			Quantization:     quant,
			RecommendedRam:   recRam,
			RecommendedVram:  recVram,
			RecommendedCores: recCores,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"repo":     repoId,
			"is_gated": isGated,
			"files":    files,
		},
	})
}
