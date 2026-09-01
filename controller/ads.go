package controller

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// Custom ad slots are stored in the CustomAds option as a JSON array:
// [{"id":"sponsor-1","image":"https://...","url":"https://..."}]. IDs are
// admin-facing only (labels in the settings UI and impression logs).
type customAd struct {
	ID    string `json:"id"`
	Image string `json:"image"`
	URL   string `json:"url"`
}

func getCustomAds() []customAd {
	raw := common.OptionMap["CustomAds"]
	if raw == "" {
		return nil
	}
	var ads []customAd
	if err := common.UnmarshalJsonStr(raw, &ads); err != nil {
		return nil
	}
	return ads
}

// GetAdsStatus returns the public ad configuration for blog pages.
func GetAdsStatus(c *gin.Context) {
	ads := make([]gin.H, 0)
	for _, ad := range getCustomAds() {
		if ad.Image == "" || ad.URL == "" {
			continue
		}
		ads = append(ads, gin.H{"id": ad.ID, "image": ad.Image, "url": ad.URL})
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"enabled":           common.OptionMap["AdsEnabled"] == "true",
			"mode":              common.OptionMap["AdsMode"],
			"adsense_client_id": common.OptionMap["AdSenseClientId"],
			"adsense_slot_id":   common.OptionMap["AdSenseSlotId"],
			"custom_ads":        ads,
		},
	})
}

// TrackAdImpression records one impression per shown ad into its own table.
func TrackAdImpression(c *gin.Context) {
	var req struct {
		ID       string `json:"id"`
		Referrer string `json:"referrer"`
	}
	if err := common.DecodeJson(c.Request.Body, &req); err != nil || req.ID == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid id"})
		return
	}
	known := false
	adsense := false
	if req.ID == "adsense" {
		known = true
		adsense = true
	} else {
		for _, ad := range getCustomAds() {
			if ad.ID == req.ID {
				known = true
				break
			}
		}
	}
	if !known {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "unknown ad"})
		return
	}

	referrer := req.Referrer
	if referrer == "" {
		referrer = c.Request.Referer()
	}
	userAgent := c.Request.UserAgent()
	userId := c.GetInt("id")
	username := c.GetString("username")
	isMember := userId > 0

	model.RecordAdImpression(req.ID, adsense, c.ClientIP(), referrer, userAgent, isMember, userId, username)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

type adImpressionRow struct {
	AdID        string
	IsAdsense   bool
	Impressions int64
	UniqueIPs   int64
}

func queryAdImpressionStats() ([]adImpressionRow, error) {
	var rows []adImpressionRow
	err := model.DB.Table("ad_impressions").
		Select("ad_id, MAX(is_adsense) as is_adsense, COUNT(*) as impressions, COUNT(DISTINCT ip) as unique_ips").
		Group("ad_id").
		Order("ad_id ASC").
		Scan(&rows).Error
	return rows, err
}

// GetAdImpressionStats returns impression counts per ad for the admin UI.
func GetAdImpressionStats(c *gin.Context) {
	rows, err := queryAdImpressionStats()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	items := make([]gin.H, 0, len(rows))
	var total int64
	for _, row := range rows {
		total += row.Impressions
		items = append(items, gin.H{
			"ad_id":       row.AdID,
			"is_adsense":  row.IsAdsense,
			"impressions": row.Impressions,
			"unique_ips":  row.UniqueIPs,
		})
	}

	// Fetch detailed recent logs
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	keyword := c.Query("keyword")
	adId := c.Query("ad_id")

	tx := model.DB.Model(&model.AdImpression{})
	if keyword != "" {
		like := "%" + keyword + "%"
		tx = tx.Where("ip LIKE ? OR referrer LIKE ? OR user_agent LIKE ? OR username LIKE ?", like, like, like, like)
	}
	if adId != "" {
		tx = tx.Where("ad_id = ?", adId)
	}

	var logsTotal int64
	tx.Count(&logsTotal)

	var recentLogs []model.AdImpression
	tx.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&recentLogs)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"items":      items,
			"total":      total,
			"logs":       recentLogs,
			"logs_total": logsTotal,
			"page":       page,
			"page_size":  pageSize,
		},
	})
}

// ClearAdImpressionStats deletes all recorded ad impressions.
func ClearAdImpressionStats(c *gin.Context) {
	if err := model.DB.Exec("DELETE FROM ad_impressions").Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// DownloadAdImpressionsCSV exports raw impression rows as CSV for the admin.
func DownloadAdImpressionsCSV(c *gin.Context) {
	var logs []model.AdImpression
	if err := model.DB.Order("created_at DESC").Limit(100000).Find(&logs).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	var b strings.Builder
	b.WriteString("id,ad_id,is_adsense,ip,referrer,user_agent,is_member,user_id,username,created_at\n")
	for _, log := range logs {
		b.WriteString(strconv.FormatInt(log.Id, 10))
		b.WriteByte(',')
		b.WriteString(csvEscape(log.AdId))
		b.WriteByte(',')
		b.WriteString(strconv.FormatBool(log.IsAdsense))
		b.WriteByte(',')
		b.WriteString(csvEscape(log.Ip))
		b.WriteByte(',')
		b.WriteString(csvEscape(log.Referrer))
		b.WriteByte(',')
		b.WriteString(csvEscape(log.UserAgent))
		b.WriteByte(',')
		b.WriteString(strconv.FormatBool(log.IsMember))
		b.WriteByte(',')
		b.WriteString(strconv.Itoa(log.UserId))
		b.WriteByte(',')
		b.WriteString(csvEscape(log.Username))
		b.WriteByte(',')
		b.WriteString(log.CreatedAt.Format("2006-01-02 15:04:05"))
		b.WriteByte('\n')
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="ad-impressions.csv"`)
	c.String(http.StatusOK, b.String())
}

func csvEscape(value string) string {
	if !strings.ContainsAny(value, `",`+"\n") {
		return value
	}
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

// UploadAdImage handles uploading an image file for custom ads.
func UploadAdImage(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "No file uploaded: " + err.Error()})
		return
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" && ext != ".gif" && ext != ".svg" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Unsupported file format"})
		return
	}

	uploadDir := filepath.Join("uploads", "ads")
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Failed to create upload directory: " + err.Error()})
		return
	}

	fileName := fmt.Sprintf("%d_%s%s", time.Now().UnixNano(), common.GetRandomString(6), ext)
	dst := filepath.Join(uploadDir, fileName)

	if err := c.SaveUploadedFile(file, dst); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Failed to save file: " + err.Error()})
		return
	}

	url := "/uploads/ads/" + fileName
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    url,
	})
}

// CleanupUnusedAdImages compares stored CustomAds and removes any /uploads/ads/ files no longer referenced.
func CleanupUnusedAdImages(newCustomAdsJson string) {
	var newAds []customAd
	_ = common.UnmarshalJsonStr(newCustomAdsJson, &newAds)

	activeUploads := make(map[string]bool)
	for _, ad := range newAds {
		if strings.HasPrefix(ad.Image, "/uploads/ads/") {
			activeUploads[filepath.Base(ad.Image)] = true
		}
	}

	uploadDir := filepath.Join("uploads", "ads")
	entries, err := os.ReadDir(uploadDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !activeUploads[name] {
			_ = os.Remove(filepath.Join(uploadDir, name))
		}
	}
}
