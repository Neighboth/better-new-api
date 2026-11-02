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
	token := strings.TrimSpace(c.Query("token"))
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
	if token != "" && strings.HasPrefix(token, "hf_") {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": "Hugging Face bağlantı hatası: " + err.Error()})
		return
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

func computeRecommendedSpecs(sizeGB float64) (recRam float64, recVram int, recCores int) {
	if sizeGB <= 0 {
		return 4.0, 2, 4
	}
	// RAM: Model file size in RAM + 25% KV cache overhead + 2 GB cushion, rounded up to 0.5 GB step (min 4.0 GB)
	recRam = math.Max(math.Ceil((sizeGB*1.25+2.0)*2.0)/2.0, 4.0)
	// VRAM: Model in VRAM + 2 GB buffer (min 2 GB)
	recVram = int(math.Max(math.Ceil(sizeGB+2.0), 2.0))
	// Cores: Scaled to model size for sufficient compute throughput
	if sizeGB > 30.0 {
		recCores = 16
	} else if sizeGB > 14.0 {
		recCores = 12
	} else if sizeGB > 6.0 {
		recCores = 8
	} else if sizeGB > 2.5 {
		recCores = 6
	} else {
		recCores = 4
	}
	return recRam, recVram, recCores
}

// GetHuggingFaceModelFiles retrieves available quantizations/GGUF files and recommended hardware
func GetHuggingFaceModelFiles(c *gin.Context) {
	repoId := strings.TrimSpace(c.Query("repo"))
	if repoId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "repo parametresi gereklidir"})
		return
	}

	token := strings.TrimSpace(c.Query("token"))
	client := &http.Client{Timeout: 15 * time.Second}

	// 1. Fetch file tree from Hugging Face API to get exact file sizes
	treeUrls := []string{
		fmt.Sprintf("https://huggingface.co/api/models/%s/tree/main?recursive=true", repoId),
		fmt.Sprintf("https://huggingface.co/api/models/%s/tree/master?recursive=true", repoId),
	}

	var treeItems []map[string]interface{}
	treeSuccess := false
	isGated := false

	for _, tUrl := range treeUrls {
		req, err := http.NewRequest("GET", tUrl, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "BYOG-HF-Explorer/1.0")
		if token != "" && strings.HasPrefix(token, "hf_") {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			isGated = true
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"gated":   true,
				"message": "Bu model Hugging Face üzerinde gated (erişim şartı onaylanması gereken) bir modeldir. Lütfen erişim izni olan Hugging Face User Access Token girin.",
			})
			return
		}

		if resp.StatusCode == http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			if err == nil {
				if err := json.Unmarshal(body, &treeItems); err == nil && len(treeItems) > 0 {
					treeSuccess = true
					break
				}
			}
		}
	}

	quantRe := regexp.MustCompile(`(?i)(q[0-9]_[k|m|s|0-9|_]+|bf16|fp16|fp32|f16|f32)`)
	var files []HfFileDetail

	if treeSuccess {
		for _, item := range treeItems {
			fType, _ := item["type"].(string)
			if fType != "" && fType != "file" {
				continue
			}
			fPath, _ := item["path"].(string)
			if fPath == "" {
				fPath, _ = item["rfilename"].(string)
			}
			if fPath == "" {
				continue
			}

			lower := strings.ToLower(fPath)
			if !strings.HasSuffix(lower, ".gguf") && !strings.HasSuffix(lower, ".bin") && !strings.HasSuffix(lower, ".safetensors") {
				continue
			}

			var fileSize int64 = 0
			if lfs, ok := item["lfs"].(map[string]interface{}); ok {
				if lfsSz, ok := lfs["size"].(float64); ok && lfsSz > 0 {
					fileSize = int64(lfsSz)
				}
			}
			if fileSize <= 0 {
				if sz, ok := item["size"].(float64); ok && sz > 0 {
					fileSize = int64(sz)
				}
			}

			downloadUrl := fmt.Sprintf("https://huggingface.co/%s/resolve/main/%s", repoId, fPath)

			quant := "Standard"
			if match := quantRe.FindString(fPath); match != "" {
				quant = strings.ToUpper(match)
			}

			sizeMB := int(fileSize / (1024 * 1024))
			sizeGB := math.Round((float64(fileSize)/(1024.0*1024.0*1024.0))*10.0) / 10.0

			recRam, recVram, recCores := computeRecommendedSpecs(sizeGB)

			files = append(files, HfFileDetail{
				Filename:         fPath,
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
	}

	// 2. Fallback to /api/models/{repoId} if tree endpoint did not return files
	if len(files) == 0 {
		hfApiUrl := fmt.Sprintf("https://huggingface.co/api/models/%s", repoId)
		req, err := http.NewRequest("GET", hfApiUrl, nil)
		if err == nil {
			req.Header.Set("User-Agent", "BYOG-HF-Explorer/1.0")
			if token != "" && strings.HasPrefix(token, "hf_") {
				req.Header.Set("Authorization", "Bearer "+token)
			}

			resp, err := client.Do(req)
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
					c.JSON(http.StatusOK, gin.H{
						"success": false,
						"gated":   true,
						"message": "Bu model Hugging Face üzerinde gated (erişim şartı onaylanması gereken) bir modeldir. Lütfen erişim izni olan Hugging Face User Access Token girin.",
					})
					return
				}

				if resp.StatusCode == http.StatusOK {
					body, _ := io.ReadAll(resp.Body)
					var repoDetail map[string]interface{}
					if json.Unmarshal(body, &repoDetail) == nil {
						if gVal, ok := repoDetail["gated"]; ok && gVal != nil {
							if gb, ok := gVal.(bool); ok {
								isGated = gb
							} else if gs, ok := gVal.(string); ok && gs == "manual" {
								isGated = true
							}
						}
						siblings, _ := repoDetail["siblings"].([]interface{})
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

							downloadUrl := fmt.Sprintf("https://huggingface.co/%s/resolve/main/%s", repoId, filename)
							quant := "Standard"
							if match := quantRe.FindString(filename); match != "" {
								quant = strings.ToUpper(match)
							}

							// Estimate size from model name if tree wasn't available
							sizeGB := 4.8
							if strings.Contains(lower, "671b") || strings.Contains(lower, "deepseek-v3") || strings.Contains(lower, "deepseek-r1") && !strings.Contains(lower, "distill") {
								sizeGB = 220.0
							} else if strings.Contains(lower, "236b") {
								sizeGB = 140.0
							} else if strings.Contains(lower, "110b") || strings.Contains(lower, "120b") {
								sizeGB = 65.0
							} else if strings.Contains(lower, "70b") || strings.Contains(lower, "72b") {
								sizeGB = 40.0
							} else if strings.Contains(lower, "8x22b") {
								sizeGB = 80.0
							} else if strings.Contains(lower, "8x7b") {
								sizeGB = 28.0
							} else if strings.Contains(lower, "32b") || strings.Contains(lower, "33b") || strings.Contains(lower, "34b") {
								sizeGB = 20.0
							} else if strings.Contains(lower, "27b") {
								sizeGB = 16.0
							} else if strings.Contains(lower, "14b") {
								sizeGB = 9.2
							} else if strings.Contains(lower, "7b") || strings.Contains(lower, "8b") || strings.Contains(lower, "9b") {
								sizeGB = 4.8
							} else if strings.Contains(lower, "3b") || strings.Contains(lower, "4b") {
								sizeGB = 2.6
							} else if strings.Contains(lower, "1.5b") || strings.Contains(lower, "2b") {
								sizeGB = 1.2
							} else if strings.Contains(lower, "0.5b") {
								sizeGB = 0.5
							}
							sizeMB := int(sizeGB * 1024)
							fileSize := int64(sizeMB) * 1024 * 1024

							recRam, recVram, recCores := computeRecommendedSpecs(sizeGB)

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
					}
				}
			}
		}
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
