package controller

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	byogservice "github.com/QuantumNous/new-api/service/byog"
	"github.com/gin-gonic/gin"
)

// Generate pure random pairing code without prefix (Section 4.5 e.g. 9K4L-28XM-73PQ-51ZR)
func generateRandomPairingCode() string {
	const charset = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return fmt.Sprintf("%s-%s-%s-%s", string(b[0:4]), string(b[4:8]), string(b[8:12]), string(b[12:16]))
}

// resolveNodeGeoIP accurately resolves the country, city, and coordinates of an IP.
// Defaults safely to Turkey (Istanbul) instead of (0,0) Mali / Null Island.
func resolveNodeGeoIP(ip string) (country, city string, lat, lon float64) {
	country = "Turkey"
	city = "Istanbul"
	lat = 41.0082
	lon = 28.9784

	client := &http.Client{Timeout: 3 * time.Second}
	isLocal := ip == "" || strings.HasPrefix(ip, "127.") || strings.HasPrefix(ip, "192.168.") || strings.HasPrefix(ip, "10.") || strings.HasPrefix(ip, "::1") || strings.HasPrefix(ip, "172.16.") || strings.HasPrefix(ip, "172.17.") || strings.HasPrefix(ip, "172.18.") || strings.HasPrefix(ip, "172.19.") || strings.HasPrefix(ip, "172.20.") || strings.HasPrefix(ip, "172.21.") || strings.HasPrefix(ip, "172.22.") || strings.HasPrefix(ip, "172.23.") || strings.HasPrefix(ip, "172.24.") || strings.HasPrefix(ip, "172.25.") || strings.HasPrefix(ip, "172.26.") || strings.HasPrefix(ip, "172.27.") || strings.HasPrefix(ip, "172.28.") || strings.HasPrefix(ip, "172.29.") || strings.HasPrefix(ip, "172.30.") || strings.HasPrefix(ip, "172.31.")

	queryUrl := "http://ip-api.com/json/" + ip + "?fields=status,country,city,lat,lon"
	if isLocal {
		queryUrl = "http://ip-api.com/json/?fields=status,country,city,lat,lon"
	}

	resp, err := client.Get(queryUrl)
	if err == nil {
		defer resp.Body.Close()
		var res struct {
			Status  string  `json:"status"`
			Country string  `json:"country"`
			City    string  `json:"city"`
			Lat     float64 `json:"lat"`
			Lon     float64 `json:"lon"`
		}
		if json.NewDecoder(resp.Body).Decode(&res) == nil && res.Status == "success" && res.Country != "" {
			return res.Country, res.City, res.Lat, res.Lon
		}
	}
	return country, city, lat, lon
}

// injectApkConfig embeds server_url, system_name, and favicon into the APK Signing Block (ID 0x71777777).
// Does not break v2/v3 signatures and passes apksigner verification 100%.
func injectApkConfig(apkData []byte, configJson []byte) ([]byte, error) {
	eocdSig := []byte{0x50, 0x4b, 0x05, 0x06}
	eocdPos := bytes.LastIndex(apkData, eocdSig)
	if eocdPos == -1 {
		return nil, fmt.Errorf("EOCD not found")
	}

	cdOffset := binary.LittleEndian.Uint32(apkData[eocdPos+16 : eocdPos+20])
	if int(cdOffset) > len(apkData) || cdOffset < 24 {
		return nil, fmt.Errorf("invalid CD offset")
	}
	magic := apkData[cdOffset-16 : cdOffset]
	if string(magic) != "APK Sig Block 42" {
		return nil, fmt.Errorf("APK Sig Block 42 not found")
	}

	blockSize := binary.LittleEndian.Uint64(apkData[cdOffset-24 : cdOffset-16])
	blockStart := int64(cdOffset) - 8 - int64(blockSize)
	if blockStart < 0 {
		return nil, fmt.Errorf("invalid block start")
	}

	pairId := uint32(0x71777777)
	pairLen := uint64(4 + len(configJson))
	pairBytes := make([]byte, 8+4+len(configJson))
	binary.LittleEndian.PutUint64(pairBytes[0:8], pairLen)
	binary.LittleEndian.PutUint32(pairBytes[8:12], pairId)
	copy(pairBytes[12:], configJson)

	newBlockSize := blockSize + uint64(len(pairBytes))
	sizeBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(sizeBytes, newBlockSize)

	part1 := apkData[:blockStart]
	existingPairs := apkData[blockStart+8 : cdOffset-24]
	tail := append(sizeBytes, magic...)
	cdAndEocd := append([]byte{}, apkData[cdOffset:]...)

	newCdOffset := cdOffset + uint32(len(pairBytes))
	eocdInSlice := eocdPos - int(cdOffset)
	binary.LittleEndian.PutUint32(cdAndEocd[eocdInSlice+16:eocdInSlice+20], newCdOffset)

	var result bytes.Buffer
	result.Write(part1)
	result.Write(sizeBytes)
	result.Write(pairBytes)
	result.Write(existingPairs)
	result.Write(tail)
	result.Write(cdAndEocd)

	return result.Bytes(), nil
}

func getFaviconBase64(faviconUrl string) string {
	if faviconUrl == "" {
		return ""
	}
	if strings.HasPrefix(faviconUrl, "data:") {
		return faviconUrl
	}

	// 1. If it's a remote URL, fetch the image bytes directly
	if strings.HasPrefix(faviconUrl, "http://") || strings.HasPrefix(faviconUrl, "https://") {
		client := http.Client{Timeout: 4 * time.Second}
		if resp, err := client.Get(faviconUrl); err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			if data, err := io.ReadAll(resp.Body); err == nil && len(data) > 0 && len(data) <= 1024*1024 {
				mime := http.DetectContentType(data)
				return fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(data))
			}
		}
	}

	// 2. If it's a local path or uploaded file
	u := faviconUrl
	if idx := strings.Index(u, "://"); idx != -1 {
		if pathIdx := strings.Index(u[idx+3:], "/"); pathIdx != -1 {
			u = u[idx+3+pathIdx:]
		}
	}
	cleanPath := strings.TrimPrefix(u, "/")
	if cleanPath != "" && cleanPath != "favicon.ico" && cleanPath != "logo.png" {
		var searchPaths []string
		searchPaths = append(searchPaths,
			cleanPath,
			filepath.Join("uploads", cleanPath),
			filepath.Join("..", "uploads", cleanPath),
			filepath.Join("uploads", "ads", cleanPath),
			filepath.Join("web", "public", cleanPath),
			filepath.Join("..", "web", "public", cleanPath),
			filepath.Join("data", cleanPath),
			filepath.Join("..", "data", cleanPath),
		)
		for _, p := range searchPaths {
			if b, err := os.ReadFile(p); err == nil && len(b) > 0 && len(b) <= 1024*1024 {
				mime := "image/png"
				lower := strings.ToLower(p)
				if strings.HasSuffix(lower, ".ico") {
					mime = "image/x-icon"
				} else if strings.HasSuffix(lower, ".svg") {
					mime = "image/svg+xml"
				} else if strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") {
					mime = "image/jpeg"
				} else if strings.HasSuffix(lower, ".webp") {
					mime = "image/webp"
				}
				return fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(b))
			}
		}

		// Also check uploaded files stored directly in managed_files database table
		if model.DB != nil {
			var mf model.ManagedFile
			if err := model.DB.Where("path = ? OR name = ?", cleanPath, filepath.Base(cleanPath)).First(&mf).Error; err == nil && len(mf.Content) > 0 {
				mime := http.DetectContentType(mf.Content)
				return fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(mf.Content))
			}
		}
	}

	// 3. Only if it's default favicon or no custom image was found, fall back to web/public assets
	if cleanPath == "favicon.ico" || cleanPath == "logo.png" || cleanPath == "" {
		fallbackPaths := []string{
			filepath.Join("web", "public", "favicon.ico"),
			filepath.Join("..", "web", "public", "favicon.ico"),
			filepath.Join("web", "public", "logo.png"),
			filepath.Join("..", "web", "public", "logo.png"),
		}
		for _, p := range fallbackPaths {
			if b, err := os.ReadFile(p); err == nil && len(b) > 0 {
				mime := "image/png"
				if strings.HasSuffix(strings.ToLower(p), ".ico") {
					mime = "image/x-icon"
				}
				return fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(b))
			}
		}
	}

	return ""
}

func getBranding(serverUrl string) (systemName string, appName string, faviconUrl string, faviconBase64 string) {
	// 1. Try in-memory OptionMap
	common.OptionMapRWMutex.RLock()
	for _, k := range []string{"SystemName_tr", "SystemName", "SystemName_en", "general_setting.system_name"} {
		if v := strings.TrimSpace(common.OptionMap[k]); v != "" && v != "New API" {
			systemName = v
			break
		}
	}
	if systemName == "" {
		for k, v := range common.OptionMap {
			if strings.HasPrefix(k, "SystemName_") && strings.TrimSpace(v) != "" && strings.TrimSpace(v) != "New API" {
				systemName = strings.TrimSpace(v)
				break
			}
		}
	}
	common.OptionMapRWMutex.RUnlock()

	// 2. Try Database directly using AllOption() across all database engines
	if systemName == "" && model.DB != nil {
		if opts, err := model.AllOption(); err == nil {
			for _, o := range opts {
				if o.Key == "SystemName_tr" && strings.TrimSpace(o.Value) != "" && strings.TrimSpace(o.Value) != "New API" {
					systemName = strings.TrimSpace(o.Value)
					break
				}
			}
			if systemName == "" {
				for _, o := range opts {
					if (o.Key == "SystemName" || o.Key == "SystemName_en" || o.Key == "general_setting.system_name") && strings.TrimSpace(o.Value) != "" && strings.TrimSpace(o.Value) != "New API" {
						systemName = strings.TrimSpace(o.Value)
						break
					}
				}
			}
			if systemName == "" {
				for _, o := range opts {
					if strings.HasPrefix(o.Key, "SystemName_") && strings.TrimSpace(o.Value) != "" && strings.TrimSpace(o.Value) != "New API" {
						systemName = strings.TrimSpace(o.Value)
						break
					}
				}
			}
		}
	}

	// 3. Try common.SystemName if explicitly modified
	if systemName == "" && common.SystemName != "" && common.SystemName != "New API" {
		systemName = strings.TrimSpace(common.SystemName)
	}

	// 4. Fallback if untouched
	if systemName == "" || systemName == "New API" {
		systemName = "BYOG Worker"
		appName = "BYOG Worker"
	} else {
		appName = fmt.Sprintf("%s BYOG", systemName)
	}

	// Favicon and Logo resolution:
	common.OptionMapRWMutex.RLock()
	for _, k := range []string{"Favicon", "Logo", "LogoDark", "general_setting.favicon", "general_setting.logo"} {
		if v := strings.TrimSpace(common.OptionMap[k]); v != "" {
			faviconUrl = v
			break
		}
	}
	common.OptionMapRWMutex.RUnlock()

	if faviconUrl == "" {
		if v := strings.TrimSpace(common.Favicon); v != "" {
			faviconUrl = v
		} else if v := strings.TrimSpace(common.Logo); v != "" {
			faviconUrl = v
		}
	}

	if faviconUrl == "" && model.DB != nil {
		if opts, err := model.AllOption(); err == nil {
			for _, o := range opts {
				if (o.Key == "Favicon" || o.Key == "Logo" || o.Key == "LogoDark" || o.Key == "general_setting.favicon" || o.Key == "general_setting.logo") && strings.TrimSpace(o.Value) != "" {
					faviconUrl = strings.TrimSpace(o.Value)
					break
				}
			}
		}
	}

	if faviconUrl == "" {
		faviconUrl = "/favicon.ico"
	}
	if strings.HasPrefix(faviconUrl, "/") && serverUrl != "" {
		faviconUrl = fmt.Sprintf("%s%s", strings.TrimRight(serverUrl, "/"), faviconUrl)
	}
	faviconBase64 = getFaviconBase64(faviconUrl)
	return
}



type userPairingSession struct {
	UserId    int
	CreatedAt int64
}

var (
	userPairingSessionsMu sync.RWMutex
	userPairingSessions   = make(map[string]userPairingSession)
)

// GetUserPairingCode generates a temporary 15-minute pairing code for the authenticated user
func GetUserPairingCode(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Unauthorized"})
		return
	}

	code := generateRandomPairingCode()
	cleanCode := strings.ReplaceAll(strings.ToUpper(code), "-", "")
	now := time.Now().Unix()

	userPairingSessionsMu.Lock()
	for k, v := range userPairingSessions {
		if now-v.CreatedAt > 1800 {
			delete(userPairingSessions, k)
		}
	}
	userPairingSessions[code] = userPairingSession{
		UserId:    userId,
		CreatedAt: now,
	}
	userPairingSessions[cleanCode] = userPairingSession{
		UserId:    userId,
		CreatedAt: now,
	}
	userPairingSessionsMu.Unlock()

	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	serverUrl := fmt.Sprintf("%s://%s", scheme, c.Request.Host)

	systemName, appName, faviconUrl, faviconBase64 := getBranding(serverUrl)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"pairing_code":   code,
			"server_url":     serverUrl,
			"system_name":    systemName,
			"app_name":       appName,
			"favicon":        faviconUrl,
			"favicon_base64": faviconBase64,
			"expires_in":     900,
		},
	})
}

// ----------------- USER CONTROLLERS -----------------

// GetByogStatus returns whether BYOG is enabled and user's ban status
func GetByogStatus(c *gin.Context) {
	setting, err := model.GetByogSetting()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}

	userId := c.GetInt("id")
	isBanned := false
	var banInfo *model.ByogNodeBan
	var availableTokens int64 = 0
	if userId > 0 {
		isBanned, banInfo = model.IsUserBannedFromByog(userId)
		availableTokens = model.GetUserAvailableTokens(userId)
	}

	hasWin := hasLocalWorkerBinary("windows", "amd64")
	hasLin := hasLocalWorkerBinary("linux", "amd64")
	hasDar := hasLocalWorkerBinary("darwin", "arm64")
	hasAnd := hasLocalWorkerBinary("android", "apk")

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"enabled":          setting.Enabled,
			"is_banned":        isBanned,
			"ban_info":         banInfo,
			"available_tokens": availableTokens,
			"client_downloads": gin.H{
				"windows": gin.H{"available": hasWin},
				"linux":   gin.H{"available": hasLin},
				"darwin":  gin.H{"available": hasDar},
				"android": gin.H{"available": hasAnd},
			},
		},
	})
}

// GetUserByogNodes lists all devices registered to the user
func GetUserByogNodes(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Unauthorized"})
		return
	}

	nodes, err := model.GetUserByogNodes(userId)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}

	// Calculate live multipliers for each node
	for _, n := range nodes {
		mult, bonuses := model.CalculateNodeMultipliers(n)
		bonusJson, _ := json.Marshal(bonuses)
		n.ActiveMultipliers = string(bonusJson)
		_ = mult
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    nodes,
	})
}

// PairByogNode pairs a new device using the random pairing code
func PairByogNode(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Unauthorized"})
		return
	}

	var req struct {
		PairingCode string `json:"pairing_code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Pairing code required"})
		return
	}

	node, err := model.GetByogNodeByPairingCode(req.PairingCode)
	if err != nil || node == nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Invalid or expired pairing code"})
		return
	}

	// Bind to user
	node.UserId = userId
	node.PairingCode = "" // Consume pairing code
	if node.Status != "banned" && node.Status != "gamer_mode" && node.Status != "paused" {
		node.Status = "online"
	}
	node.LastHeartbeatAt = time.Now().Unix()
	node.UpdatedAt = time.Now().Unix()
	if err := model.SaveByogNode(node); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    node,
		"message": "Device paired successfully",
	})
}

// UpdateUserByogNode updates device name, operating mode, sliders, schedules
func UpdateUserByogNode(c *gin.Context) {
	userId := c.GetInt("id")
	nodeId := c.Param("id")

	node, err := model.GetByogNodeByNodeId(nodeId)
	if err != nil || node == nil || node.UserId != userId {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Device not found"})
		return
	}

	var req model.ByogNode
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	if req.DeviceName != "" {
		node.DeviceName = req.DeviceName
	}
	if req.OperatingMode != "" {
		node.OperatingMode = req.OperatingMode
	}
	if req.PowerProfile != "" {
		node.PowerProfile = req.PowerProfile
	}
	if req.ScheduleConfig != "" {
		node.ScheduleConfig = req.ScheduleConfig
	}
	if req.AllocatedDiskGB > 0 {
		node.AllocatedDiskGB = req.AllocatedDiskGB
	}
	if req.AllocatedVramGB >= 0 {
		node.AllocatedVramGB = req.AllocatedVramGB
	}
	if req.AllocatedRamGB > 0 {
		node.AllocatedRamGB = req.AllocatedRamGB
	}
	if req.AllocatedCores > 0 {
		node.AllocatedCores = req.AllocatedCores
	}
	node.HotStartEnabled = req.HotStartEnabled

	// Mobile controls
	node.WifiOnlyDownload = req.WifiOnlyDownload
	node.MonthlyDataLimitMB = req.MonthlyDataLimitMB
	node.RunOnlyWhileCharging = req.RunOnlyWhileCharging
	node.BatteryCutoffEnabled = req.BatteryCutoffEnabled
	node.BatteryCutoffPercent = req.BatteryCutoffPercent
	if req.MaxBatteryTemp > 0 {
		node.MaxBatteryTemp = req.MaxBatteryTemp
	}

	node.UpdatedAt = time.Now().Unix()
	if err := model.SaveByogNode(node); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": node})
}

// DeleteUserByogNode unlinks and removes device
func DeleteUserByogNode(c *gin.Context) {
	userId := c.GetInt("id")
	nodeId := c.Param("id")

	node, err := model.GetByogNodeByNodeId(nodeId)
	if err != nil || node == nil {
		if numId, errConv := strconv.Atoi(nodeId); errConv == nil && numId > 0 {
			var n model.ByogNode
			if model.DB != nil && model.DB.Where("id = ?", numId).First(&n).Error == nil {
				node = &n
			}
		}
	}

	if node == nil || (node.UserId != userId && c.GetInt("role") < common.RoleAdminUser) {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Device not found"})
		return
	}

	targetNodeId := node.NodeId
	targetId := node.Id
	hwid := node.HWID
	byogservice.GlobalHub.Unregister(targetNodeId)

	if model.DB != nil {
		if targetId > 0 {
			if err := model.DB.Unscoped().Where("id = ?", targetId).Delete(&model.ByogNode{}).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
				return
			}
		}
		if targetNodeId != "" {
			_ = model.DB.Unscoped().Where("node_id = ?", targetNodeId).Delete(&model.ByogNode{}).Error
		}
		if hwid != "" {
			_ = model.DB.Unscoped().Where("hw_id = ?", hwid).Delete(&model.ByogNode{}).Error
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Device deleted"})
}

// GetUserRewardCatalog returns all active rewards in the store
func GetUserRewardCatalog(c *gin.Context) {
	rewards, err := model.GetAllRewardItems(true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": rewards})
}

// ClaimUserReward creates claim, burns token quota, and opens support ticket
func ClaimUserReward(c *gin.Context) {
	userId := c.GetInt("id")
	rewardId, _ := strconv.Atoi(c.Param("id"))

	reward, err := model.GetRewardItemById(rewardId)
	if err != nil || reward == nil || !reward.IsActive || reward.Stock <= 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Reward unavailable or out of stock"})
		return
	}

	// Verify required tokens against user's BYOG processed tokens net of previous claims
	var sum struct {
		Total int64
	}
	_ = model.DB.Model(&model.ByogNode{}).Where("user_id = ?", userId).Select("COALESCE(SUM(total_tokens_processed), 0) as total").Scan(&sum)
	var totalBurned int64 = 0
	_ = model.DB.Model(&model.ByogRewardClaim{}).Where("user_id = ? AND status != 'rejected'", userId).Select("COALESCE(SUM(tokens_burned), 0)").Scan(&totalBurned)
	availableTokens := sum.Total - totalBurned
	if availableTokens < reward.RequiredTokens {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": fmt.Sprintf("Yetersiz token bakiyesi. Gerekli: %d token, Kullanılabilir: %d token", reward.RequiredTokens, availableTokens),
		})
		return
	}

	// Decrement stock
	reward.Stock--
	_ = model.SaveRewardItem(reward)

	// Automatically create a support ticket (Section 1.1)
	ticket := &model.Ticket{
		UserId:      userId,
		Title:       fmt.Sprintf("[BYOG Reward Claim] %s", reward.Title),
		Category:    "billing",
		Priority:    "high",
		Status:      "open",
		CreatedAt:   time.Now().Unix(),
		UpdatedAt:   time.Now().Unix(),
		LastReplyAt: time.Now().Unix(),
	}
	_ = model.DB.Create(ticket)

	ticketMessage := &model.TicketMessage{
		TicketId:  ticket.Id,
		UserId:    userId,
		IsAdmin:   false,
		Content:   fmt.Sprintf("User requested reward claim for: %s. %d tokens were deducted from wallet. Verification: Genuine token production logged.", reward.Title, reward.RequiredTokens),
		CreatedAt: time.Now().Unix(),
	}
	_ = model.DB.Create(ticketMessage)

	// Create claim record
	claim := &model.ByogRewardClaim{
		UserId:          userId,
		RewardItemId:    reward.Id,
		RewardTitle:     reward.Title,
		TokensBurned:    reward.RequiredTokens,
		Status:          "pending",
		TicketId:        ticket.Id,
		VerificationLog: fmt.Sprintf("Timestamp: %s, User ID: %d, Burned: %d tokens, Stock left: %d", time.Now().Format(time.RFC3339), userId, reward.RequiredTokens, reward.Stock),
		CreatedAt:       time.Now().Unix(),
		UpdatedAt:       time.Now().Unix(),
	}
	_ = model.DB.Create(claim)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    claim,
		"message": "Reward claim submitted and support ticket opened.",
	})
}

// SubmitBanAppeal allows user to appeal a ban and auto-creates an appeal ticket
func SubmitBanAppeal(c *gin.Context) {
	userId := c.GetInt("id")
	var req struct {
		NodeId     string `json:"node_id" binding:"required"`
		AppealText string `json:"appeal_text" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	var ban model.ByogNodeBan
	err := model.DB.Where("node_id = ? AND is_active = ?", req.NodeId, true).First(&ban).Error
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "No active ban found for this device"})
		return
	}

	// Create appeal support ticket
	ticket := &model.Ticket{
		UserId:      userId,
		Title:       fmt.Sprintf("[BYOG Ban Appeal] Device %s - %s", req.NodeId, ban.ViolationCode),
		Category:    "technical",
		Priority:    "urgent",
		Status:      "open",
		CreatedAt:   time.Now().Unix(),
		UpdatedAt:   time.Now().Unix(),
		LastReplyAt: time.Now().Unix(),
	}
	_ = model.DB.Create(ticket)

	msg := &model.TicketMessage{
		TicketId:  ticket.Id,
		UserId:    userId,
		IsAdmin:   false,
		Content:   fmt.Sprintf("Appeal: %s\n\nSecurity Details:\nViolation Code: %s\nReason: %s\nExpected Logprob: %f\nActual Logprob: %f\nTimestamp: %s", req.AppealText, ban.ViolationCode, ban.BanReason, ban.ExpectedLogprob, ban.ActualLogprob, time.Unix(ban.BannedAt, 0).Format(time.RFC3339)),
		CreatedAt: time.Now().Unix(),
	}
	_ = model.DB.Create(msg)

	ban.AppealedAt = time.Now().Unix()
	ban.AppealTicketId = ticket.Id
	_ = model.DB.Save(&ban)

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"ticket_id": ticket.Id,
		"message":   "Appeal submitted. A support ticket has been created.",
	})
}

// ----------------- ADMIN CONTROLLERS -----------------

// Admin Settings
func GetByogAdminSettings(c *gin.Context) {
	setting, err := model.GetByogSetting()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": setting})
}

func UpdateByogAdminSettings(c *gin.Context) {
	var req model.ByogSetting
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	setting, err := model.GetByogSetting()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}

	setting.Enabled = req.Enabled
	setting.Tier1ProviderPrice = req.Tier1ProviderPrice
	setting.Tier1CustomerPrice = req.Tier1CustomerPrice
	setting.Tier2ProviderPrice = req.Tier2ProviderPrice
	setting.Tier2CustomerPrice = req.Tier2CustomerPrice
	setting.Tier3ProviderPrice = req.Tier3ProviderPrice
	setting.Tier3CustomerPrice = req.Tier3CustomerPrice
	setting.MaxColdStartTimeout = req.MaxColdStartTimeout
	setting.NightOwlSurgeEnabled = req.NightOwlSurgeEnabled
	setting.WeekendCrunchEnabled = req.WeekendCrunchEnabled
	if req.HardwareBackends != "" {
		setting.HardwareBackends = req.HardwareBackends
	}

	if err := model.UpdateByogSetting(setting); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": setting})
}

// Admin Reward Store
func GetByogAdminRewards(c *gin.Context) {
	rewards, err := model.GetAllRewardItems(false)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": rewards})
}

func CreateByogAdminReward(c *gin.Context) {
	var item model.ByogRewardItem
	if err := c.ShouldBindJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	if err := model.SaveRewardItem(&item); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": item})
}

func UpdateByogAdminReward(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	item, err := model.GetRewardItemById(id)
	if err != nil || item == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Reward not found"})
		return
	}

	if err := c.ShouldBindJSON(item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	item.Id = id
	if err := model.SaveRewardItem(item); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": item})
}

func DeleteByogAdminReward(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	_ = model.DeleteRewardItem(id)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Reward deleted"})
}

// Admin Claims
func GetByogAdminClaims(c *gin.Context) {
	status := c.Query("status")
	claims, err := model.GetAllRewardClaims(status)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": claims})
}

func ApproveByogAdminClaim(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var claim model.ByogRewardClaim
	if err := model.DB.First(&claim, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Claim not found"})
		return
	}

	claim.Status = "approved"
	claim.UpdatedAt = time.Now().Unix()
	_ = model.DB.Save(&claim)

	c.JSON(http.StatusOK, gin.H{"success": true, "data": claim})
}

func RejectByogAdminClaim(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var claim model.ByogRewardClaim
	if err := model.DB.First(&claim, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Claim not found"})
		return
	}

	claim.Status = "rejected"
	claim.UpdatedAt = time.Now().Unix()
	_ = model.DB.Save(&claim)

	// Refund tokens to user
	_ = model.IncreaseUserQuota(claim.UserId, int(claim.TokensBurned), true)

	c.JSON(http.StatusOK, gin.H{"success": true, "data": claim, "message": "Claim rejected and tokens refunded"})
}

func ExportByogAdminClaims(c *gin.Context) {
	claims, err := model.GetAllRewardClaims("")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.Header("Content-Disposition", "attachment; filename=byog_reward_claims.json")
	c.Header("Content-Type", "application/json")
	c.JSON(http.StatusOK, claims)
}

// Admin Assignment Rules
func GetByogAdminRules(c *gin.Context) {
	rules, err := model.GetAllAssignmentRules()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": rules})
}

func CreateByogAdminRule(c *gin.Context) {
	var rule model.ByogAssignmentRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	if err := model.SaveAssignmentRule(&rule); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": rule})
}

func UpdateByogAdminRule(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var rule model.ByogAssignmentRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	rule.Id = id
	if err := model.SaveAssignmentRule(&rule); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": rule})
}

func DeleteByogAdminRule(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	_ = model.DeleteAssignmentRule(id)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Rule deleted"})
}

// Admin Models & LoRAs
func GetByogAdminModels(c *gin.Context) {
	modelsList, err := model.GetAllByogModels(false)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	loras, _ := model.GetAllByogLoras()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"models": modelsList,
			"loras":  loras,
		},
	})
}

func CreateByogAdminModel(c *gin.Context) {
	var m model.ByogModel
	if err := c.ShouldBindJSON(&m); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	if err := model.SaveByogModel(&m); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": m})
}

func UpdateByogAdminModel(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var m model.ByogModel
	if err := c.ShouldBindJSON(&m); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	m.Id = id
	if err := model.SaveByogModel(&m); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": m})
}

func DeleteByogAdminModel(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	_ = model.DeleteByogModel(id)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Model deleted"})
}

func CreateByogAdminLora(c *gin.Context) {
	var lora model.ByogModelLora
	if err := c.ShouldBindJSON(&lora); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	if err := model.SaveByogLora(&lora); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": lora})
}

func DeleteByogAdminLora(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	_ = model.DeleteByogLora(id)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "LoRA deleted"})
}

// Admin Nodes & Live Telemetry
func GetByogAdminNodes(c *gin.Context) {
	nodes, err := model.GetAllByogNodes()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}

	// Attach live session indicators
	for _, n := range nodes {
		session, online := byogservice.GlobalHub.GetSession(n.NodeId)
		if online && session != nil {
			n.Status = "online"
			if session.IsGamerMode {
				n.Status = "gamer_mode"
			}
		} else {
			n.Status = "offline"
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": nodes})
}

// Deep Inspect Modal (Section 1.5)
func InspectByogAdminNode(c *gin.Context) {
	nodeId := c.Param("id")
	node, err := model.GetByogNodeByNodeId(nodeId)
	if err != nil || node == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Node not found"})
		return
	}

	mult, bonuses := model.CalculateNodeMultipliers(node)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"node":              node,
			"multiplier":        mult,
			"bonuses":           bonuses,
			"tee_supported":     node.TeeSupported,
			"tee_type":          node.TeeType,
			"temperatures": gin.H{
				"cpu":     node.CpuTemp,
				"gpu":     node.GpuTemp,
				"battery": node.BatteryTemp,
				"fan":     node.FanSpeed,
			},
			"bandwidth": gin.H{
				"down_mbps": node.BandwidthDownMbps,
				"up_mbps":   node.BandwidthUpMbps,
				"ping_ms":   node.PingMs,
				"net_type":  node.NetworkType,
				"ip_type":   node.IpType,
			},
			"memory": gin.H{
				"physical_ram_mb": node.PhysicalRamMB,
				"swap_ram_mb":     node.SwapRamMB,
				"vram_mb":         node.VramMB,
				"shared_vram_mb":  node.SharedVramMB,
			},
			"performance": gin.H{
				"tps":           node.CurrentTps,
				"ttft_ms":       node.CurrentTtftMs,
				"tokens":        node.TotalTokensProcessed,
				"uptime_secs":   node.UptimeSeconds,
				"reputation":    node.ReputationScore,
				"power_profile": node.PowerProfile,
			},
		},
	})
}

// Banned Nodes Tab
func GetByogAdminBannedNodes(c *gin.Context) {
	bans, err := model.GetAllBannedNodes()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": bans})
}

func UnbanByogAdminNode(c *gin.Context) {
	banId, _ := strconv.Atoi(c.Param("id"))
	if err := model.UnbanNode(banId); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Ban lifted successfully"})
}

// DeleteAdminByogNode permanently deletes a node from the network and database
func DeleteAdminByogNode(c *gin.Context) {
	nodeId := c.Param("id")
	if nodeId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Node ID required"})
		return
	}

	byogservice.GlobalHub.Unregister(nodeId)
	err := model.DeleteByogNode(nodeId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Node permanently deleted from network"})
}

// ----------------- WORKER / REVERSE TUNNEL CONTROLLERS -----------------

// RegisterByogWorker is called when worker binary starts up
func RegisterByogWorker(c *gin.Context) {
	var req struct {
		NodeId          string  `json:"node_id"`
		HWID            string  `json:"hwid" binding:"required"`
		PairingCode     string  `json:"pairing_code"`
		OS              string  `json:"os"`
		Arch            string  `json:"arch"`
		DeviceName      string  `json:"device_name"`
		DeviceType      string  `json:"device_type"`
		CpuModel        string  `json:"cpu_model"`
		GpuModel        string  `json:"gpu_model"`
		NpuModel        string  `json:"npu_model"`
		PhysicalRam     int     `json:"physical_ram_mb"`
		SwapRam         int     `json:"swap_ram_mb"`
		VramMB          int     `json:"vram_mb"`
		SharedVramMB    int     `json:"shared_vram_mb"`
		TeeSupported    bool    `json:"tee_supported"`
		TeeType         string  `json:"tee_type"`
		DownMbps        float64 `json:"bandwidth_down_mbps"`
		UpMbps          float64 `json:"bandwidth_up_mbps"`
		PingMs          int     `json:"ping_ms"`
		NetworkType     string  `json:"network_type"`
		ClientIp        string  `json:"client_ip"`
		BatteryPercent  int     `json:"battery_percent"`
		BatteryTemp     float64 `json:"battery_temp"`
		IsCharging      *bool   `json:"is_charging"`
		InstalledModels string  `json:"installed_models"`
		DownloadQueue   string  `json:"download_queue"`
		ActiveVramModel string  `json:"active_vram_model"`
		ModelStatus     string  `json:"model_status"`
		DownloadedMB    int     `json:"downloaded_mb"`
		TotalMB          int     `json:"total_mb"`
		DownloadProgress int     `json:"download_progress"`
		TotalDiskGB      int     `json:"total_disk_gb"`
		FreeDiskGB       int     `json:"free_disk_gb"`
		CurrentTps       float64 `json:"current_tps"`
		CurrentTtftMs    int     `json:"current_ttft_ms"`
		TotalTokens      int64   `json:"total_tokens"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	// Zero-Trust Anti-Cheat HWID check (Section 3.1)
	valid, reason := byogservice.GlobalAntiCheat.ValidateHWID(req.HWID, req.OS, req.DeviceType, map[string]interface{}{
		"cpu_model": req.CpuModel,
	})
	if !valid {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"code":    "#SEC-HWID-BAN",
			"message": reason,
		})
		return
	}

	// Generate salt and node ID
	hash := sha256.Sum256([]byte(req.HWID + common.SessionSecret))
	nodeId := hex.EncodeToString(hash[:16])

	var matchedUserId int
	if req.PairingCode != "" {
		codeUpper := strings.ToUpper(strings.TrimSpace(req.PairingCode))
		codeClean := strings.ReplaceAll(strings.ReplaceAll(codeUpper, "-", ""), " ", "")
		userPairingSessionsMu.Lock()
		if sess, ok := userPairingSessions[codeUpper]; ok {
			matchedUserId = sess.UserId
			delete(userPairingSessions, codeUpper)
			delete(userPairingSessions, codeClean)
		} else if sess, ok := userPairingSessions[codeClean]; ok {
			matchedUserId = sess.UserId
			delete(userPairingSessions, codeUpper)
			delete(userPairingSessions, codeClean)
		}
		userPairingSessionsMu.Unlock()

		if matchedUserId == 0 {
			if existingNode, _ := model.GetByogNodeByPairingCode(codeUpper); existingNode != nil && existingNode.UserId > 0 {
				matchedUserId = existingNode.UserId
			} else if existingNode, _ := model.GetByogNodeByPairingCode(codeClean); existingNode != nil && existingNode.UserId > 0 {
				matchedUserId = existingNode.UserId
			}
		}

		if matchedUserId == 0 {
			if existingNode, _ := model.GetByogNodeByHWID(req.HWID); existingNode != nil && existingNode.UserId > 0 {
				matchedUserId = existingNode.UserId
			} else {
				c.JSON(http.StatusBadRequest, gin.H{
					"success": false,
					"message": "Geçersiz veya süresi dolmuş eşleştirme kodu. Lütfen siteden yeni kod alıp tekrar deneyiniz.",
				})
				return
			}
		}
	}

	var node *model.ByogNode
	var err error
	if req.NodeId != "" {
		node, err = model.GetByogNodeByNodeId(req.NodeId)
	}
	if node == nil {
		node, err = model.GetByogNodeByHWID(req.HWID)
	}
	// If unpaired and not providing valid pairing code, reject to prevent ghost node creation
	if matchedUserId == 0 {
		if node == nil || node.UserId == 0 {
			if node != nil && node.UserId == 0 {
				_ = model.DeleteByogNode(node.NodeId)
			}
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Cihaz henüz bir kullanıcı hesabıyla eşleştirilmemiş. Lütfen eşleştirme kodunu girin.",
			})
			return
		}
		matchedUserId = node.UserId
	}

	country, city, lat, lon := resolveNodeGeoIP(c.ClientIP())
	isMobile := req.DeviceType == "mobile" || req.DeviceType == "tablet"
	if isMobile {
		req.VramMB = 0
		req.SharedVramMB = 0
	}

	if err == nil && node != nil {
		// Existing node: update hardware specs & live telemetry
		node.DeviceName = req.DeviceName
		node.CpuModel = req.CpuModel
		node.GpuModel = req.GpuModel
		node.NpuModel = req.NpuModel
		node.PhysicalRamMB = req.PhysicalRam
		node.SwapRamMB = req.SwapRam
		node.VramMB = req.VramMB
		node.SharedVramMB = req.SharedVramMB
		if isMobile {
			node.AllocatedVramGB = 0
		}
		node.TeeSupported = req.TeeSupported
		node.TeeType = req.TeeType
		node.BandwidthDownMbps = req.DownMbps
		node.BandwidthUpMbps = req.UpMbps
		node.PingMs = req.PingMs
		node.NetworkType = req.NetworkType
		node.Country = country
		node.City = city
		node.Latitude = lat
		node.Longitude = lon
		if node.Status != "banned" && node.Status != "gamer_mode" && node.Status != "paused" {
			node.Status = "online"
		}
		node.UpdatedAt = time.Now().Unix()
		node.LastHeartbeatAt = time.Now().Unix()
		if req.BatteryPercent >= 0 {
			node.BatteryPercent = req.BatteryPercent
		}
		if req.BatteryTemp > 0 {
			node.BatteryTemp = req.BatteryTemp
		}
		if req.IsCharging != nil {
			node.IsCharging = *req.IsCharging
		}
		if req.InstalledModels != "" {
			node.InstalledModels = req.InstalledModels
			if node.DownloadQueue != "" && node.DownloadQueue != "[]" {
				var inst []string
				if json.Unmarshal([]byte(req.InstalledModels), &inst) == nil && len(inst) > 0 {
					var q []string
					if json.Unmarshal([]byte(node.DownloadQueue), &q) == nil {
						newQ := []string{}
						for _, qItem := range q {
							found := false
							for _, instItem := range inst {
								if instItem == qItem {
									found = true
									break
								}
							}
							if !found {
								newQ = append(newQ, qItem)
							}
						}
						qBytes, _ := json.Marshal(newQ)
						node.DownloadQueue = string(qBytes)
					}
				}
			}
		}
		if req.DownloadQueue != "" {
			node.DownloadQueue = req.DownloadQueue
		}
		if req.ModelStatus != "" {
			node.ModelStatus = req.ModelStatus
		}
		if req.TotalMB > 0 || req.DownloadedMB > 0 || req.ModelStatus == "downloading" {
			if req.TotalMB > 0 {
				node.TotalMB = req.TotalMB
			}
			node.DownloadedMB = req.DownloadedMB
			node.DownloadProgress = req.DownloadProgress
			if node.DownloadProgress <= 0 && node.TotalMB > 0 {
				node.DownloadProgress = int((float64(node.DownloadedMB) / float64(node.TotalMB)) * 100)
			}
		}
		if req.ActiveVramModel != "" {
			node.ActiveVramModel = req.ActiveVramModel
		}
		if req.TotalDiskGB > 0 {
			node.TotalDiskGB = req.TotalDiskGB
		}
		if req.FreeDiskGB > 0 {
			node.FreeDiskGB = req.FreeDiskGB
			if node.AllocatedDiskGB > node.FreeDiskGB {
				node.AllocatedDiskGB = node.FreeDiskGB
			}
		}
		node.CurrentTps = req.CurrentTps
		if node.ModelStatus == "ready" || node.ModelStatus == "idle" || node.ModelStatus == "downloading" {
			node.CurrentTps = 0.0
		}
		if req.CurrentTtftMs > 0 {
			node.CurrentTtftMs = req.CurrentTtftMs
		}
		if req.TotalTokens > 0 && req.TotalTokens > node.TotalTokensProcessed {
			node.TotalTokensProcessed = req.TotalTokens
		}
		node.UserId = matchedUserId
		node.PairingCode = ""
		_ = model.SaveByogNode(node)
	} else {
		// New node
		initialBatteryPercent := 100
		if req.BatteryPercent >= 0 {
			initialBatteryPercent = req.BatteryPercent
		}
		initialBatteryTemp := 32.0
		if req.BatteryTemp > 0 {
			initialBatteryTemp = req.BatteryTemp
		}
		initialIsCharging := false
		if req.IsCharging != nil {
			initialIsCharging = *req.IsCharging
		}

		allocRam := 8
		if req.PhysicalRam > 0 {
			allocRam = int(math.Max(2, float64(req.PhysicalRam)/2048))
		}
		allocVram := 0
		if !isMobile && req.VramMB > 0 {
			allocVram = req.VramMB / 1024
		}
		allocDisk := 40
		if req.FreeDiskGB > 0 {
			allocDisk = int(math.Min(20, math.Max(2, float64(req.FreeDiskGB)/2)))
		}

		node = &model.ByogNode{
			NodeId:               nodeId,
			UserId:               matchedUserId,
			PairingCode:          "",
			DeviceName:           req.DeviceName,
			DeviceType:           req.DeviceType,
			OS:                   req.OS,
			Arch:                 req.Arch,
			HWID:                 req.HWID,
			CpuModel:             req.CpuModel,
			GpuModel:             req.GpuModel,
			NpuModel:             req.NpuModel,
			PhysicalRamMB:        req.PhysicalRam,
			SwapRamMB:            req.SwapRam,
			VramMB:               req.VramMB,
			SharedVramMB:         req.SharedVramMB,
			TeeSupported:         req.TeeSupported,
			TeeType:              req.TeeType,
			BandwidthDownMbps:    req.DownMbps,
			BandwidthUpMbps:      req.UpMbps,
			PingMs:               req.PingMs,
			NetworkType:          req.NetworkType,
			IpAddress:            c.ClientIP(),
			Country:              country,
			City:                 city,
			Latitude:             lat,
			Longitude:            lon,
			Status:               "online",
			OperatingMode:        "mode_b",
			PowerProfile:         "full_power",
			AllocatedDiskGB:      allocDisk,
			TotalDiskGB:          req.TotalDiskGB,
			FreeDiskGB:           req.FreeDiskGB,
			AllocatedVramGB:      allocVram,
			AllocatedRamGB:       allocRam,
			AllocatedCores:       4,
			WifiOnlyDownload:     true,
			MonthlyDataLimitMB:   0,
			RunOnlyWhileCharging: false,
			BatteryCutoffEnabled: false,
			BatteryCutoffPercent: 20,
			MaxBatteryTemp:       50.0,
			BatteryTempSupported: true,
			BatteryPercent:       initialBatteryPercent,
			BatteryTemp:          initialBatteryTemp,
			IsCharging:           initialIsCharging,
			ReputationScore:      50,
			InstalledModels:      req.InstalledModels,
			DownloadQueue:        req.DownloadQueue,
			ModelStatus:          req.ModelStatus,
			DownloadedMB:         req.DownloadedMB,
			TotalMB:              req.TotalMB,
			DownloadProgress:     req.DownloadProgress,
			ActiveVramModel:      req.ActiveVramModel,
			CurrentTps:           0.0,
			CurrentTtftMs:        req.CurrentTtftMs,
			TotalTokensProcessed: req.TotalTokens,
			LastHeartbeatAt:      time.Now().Unix(),
			CreatedAt:            time.Now().Unix(),
			UpdatedAt:            time.Now().Unix(),
		}
		_ = model.SaveByogNode(node)
	}

	assignedModels, _ := byogservice.GlobalScheduler.EvaluateNodeModelAssignment(node)
	if len(assignedModels) > 0 {
		var installed []string
		_ = json.Unmarshal([]byte(node.InstalledModels), &installed)
		installedMap := make(map[string]bool)
		totalUsedMB := 0
		for _, m := range installed {
			installedMap[m] = true
			totalUsedMB += getByogModelEstimatedSizeMB(m)
		}

		var queue []string
		_ = json.Unmarshal([]byte(node.DownloadQueue), &queue)
		queueMap := make(map[string]bool)
		for _, qItem := range queue {
			queueMap[qItem] = true
			totalUsedMB += getByogModelEstimatedSizeMB(qItem)
		}

		maxDiskMB := node.AllocatedDiskGB * 1024
		if maxDiskMB <= 0 {
			maxDiskMB = 40 * 1024
		}

		changed := false
		for _, am := range assignedModels {
			if !installedMap[am] && !queueMap[am] {
				mSize := getByogModelEstimatedSizeMB(am)
				if totalUsedMB+mSize <= maxDiskMB {
					queue = append(queue, am)
					queueMap[am] = true
					totalUsedMB += mSize
					changed = true
				}
			}
		}
		if changed || ((node.DownloadQueue == "" || node.DownloadQueue == "[]") && len(queue) > 0) {
			qBytes, _ := json.Marshal(queue)
			node.DownloadQueue = string(qBytes)
			_ = model.SaveByogNode(node)
		}
	}

	if node.TotalMB <= 0 {
		var q []string
		_ = json.Unmarshal([]byte(node.DownloadQueue), &q)
		if len(q) > 0 {
			node.TotalMB = getByogModelEstimatedSizeMB(q[0])
		} else if len(assignedModels) > 0 {
			node.TotalMB = getByogModelEstimatedSizeMB(assignedModels[0])
		} else {
			node.TotalMB = 380
		}
		if node.DownloadedMB > 0 && node.TotalMB > 0 {
			node.DownloadProgress = int((float64(node.DownloadedMB) / float64(node.TotalMB)) * 100)
		}
		_ = model.SaveByogNode(node)
	}

	type AssignedModelDetail struct {
		ModelId     string `json:"model_id"`
		DownloadUrl string `json:"download_url"`
		FileSize    int64  `json:"file_size"`
		TotalMB     int    `json:"total_mb"`
	}
	var assignedModelDetails []AssignedModelDetail
	for _, am := range assignedModels {
		dUrl := ""
		var fSize int64 = 0
		if mObj, _ := model.GetByogModelByModelId(am); mObj != nil {
			dUrl = mObj.DownloadUrl
			fSize = mObj.FileSize
		}
		if dUrl == "" {
			amLower := strings.ToLower(am)
			if strings.Contains(amLower, "0.5b") {
				dUrl = "https://huggingface.co/Qwen/Qwen2.5-0.5B-Instruct-GGUF/resolve/main/qwen2.5-0.5b-instruct-q4_k_m.gguf"
			} else if strings.Contains(amLower, "1.5b") {
				dUrl = "https://huggingface.co/Qwen/Qwen2.5-1.5B-Instruct-GGUF/resolve/main/qwen2.5-1.5b-instruct-q4_k_m.gguf"
			} else {
				dUrl = fmt.Sprintf("https://huggingface.co/Qwen/Qwen2.5-0.5B-Instruct-GGUF/resolve/main/%s.gguf", am)
			}
		}
		mSize := getByogModelEstimatedSizeMB(am)
		if fSize <= 0 {
			fSize = int64(mSize) * 1024 * 1024
		}
		assignedModelDetails = append(assignedModelDetails, AssignedModelDetail{
			ModelId:     am,
			DownloadUrl: dUrl,
			FileSize:    fSize,
			TotalMB:     mSize,
		})
	}

	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	serverUrl := fmt.Sprintf("%s://%s", scheme, c.Request.Host)
	systemName, appName, faviconUrl, faviconBase64 := getBranding(serverUrl)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"node_id":                node.NodeId,
			"pairing_code":           node.PairingCode,
			"user_id":                node.UserId,
			"is_paired":              node.UserId > 0,
			"system_name":            systemName,
			"app_name":               appName,
			"favicon":                faviconUrl,
			"favicon_base64":         faviconBase64,
			"assigned_models":        assignedModels,
			"assigned_model_details": assignedModelDetails,
			"hot_start_enabled":      node.HotStartEnabled,
			"allocated_disk_gb":      node.AllocatedDiskGB,
		},
	})
}

// ByogWorkerTunnel upgrades WebSocket connection for reverse tunneling
func ByogWorkerTunnel(c *gin.Context) {
	nodeId := c.Query("node_id")
	if nodeId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node_id required"})
		return
	}

	node, err := model.GetByogNodeByNodeId(nodeId)
	if err != nil || node == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
		return
	}

	conn, err := byogservice.Upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		common.SysError(fmt.Sprintf("[BYOG TUNNEL] Upgrade error: %v", err))
		return
	}

	session := &byogservice.NodeSession{
		NodeId:          nodeId,
		UserId:          node.UserId,
		Conn:            conn,
		SendChan:        make(chan byogservice.TunnelFrame, 100),
		LastHeartbeat:   time.Now(),
		UptimeStart:     time.Now(),
		MaxConcurrency:  4,
		DeviceType:      node.DeviceType,
		InstalledModels: make(map[string]bool),
	}

	if node.InstalledModels != "" {
		var list []string
		if json.Unmarshal([]byte(node.InstalledModels), &list) == nil {
			for _, m := range list {
				session.InstalledModels[m] = true
			}
		}
	}

	byogservice.GlobalHub.Register(session)
	defer byogservice.GlobalHub.Unregister(nodeId)

	// Keepalive ping loop
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			if err := session.WriteFrame(byogservice.TunnelFrame{Type: "ping"}); err != nil {
				return
			}
		}
	}()

	// Read frame loop
	for {
		var frame byogservice.TunnelFrame
		if err := conn.ReadJSON(&frame); err != nil {
			break
		}

		switch frame.Type {
		case byogservice.MsgTypeHeartbeat:
			session.HandleHeartbeat(&frame)

		case byogservice.MsgTypeInferenceChunk, byogservice.MsgTypeInferenceDone, byogservice.MsgTypeInferenceError,
			byogservice.MsgTypeCanaryResp, byogservice.MsgTypeByteProbeResp:
			if frame.Id != "" {
				if chVal, ok := session.PendingRequests.Load(frame.Id); ok {
					ch := chVal.(chan *byogservice.TunnelFrame)
					select {
					case ch <- &frame:
					default:
					}
				}
			}
		}
	}
}

// ReportByogWorkerCrash receives crash reports and logs them for deep inspection
func ReportByogWorkerCrash(c *gin.Context) {
	var report model.ByogCrashReport
	if err := c.ShouldBindJSON(&report); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	// Automatic Recommendation based on error code (Section 5.6)
	if strings.Contains(strings.ToLower(report.ErrorCode), "oom") || strings.Contains(strings.ToLower(report.ErrorMessage), "out of memory") {
		report.Recommendation = "Bellek yetersizliği (OOM) sebebiyle işlem sonlandırıldı. Önerilen: Daha küçük bir model seçin veya RAM bütçesini artırın."
	} else if strings.Contains(strings.ToLower(report.ErrorMessage), "timeout") {
		report.Recommendation = "Zaman aşımı oluştu. Önerilen: Ağ bağlantınızı ve gecikme sürenizi kontrol edin."
	} else {
		report.Recommendation = "Sistem çökmesi kaydedildi. Sürücülerinizi ve motor bileşenlerinizi güncelleyin."
	}

	_ = model.SaveCrashReport(&report)

	// Update node status
	_ = model.DB.Model(&model.ByogNode{}).
		Where("node_id = ?", report.NodeId).
		Update("status", "paused")

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Crash report recorded"})
}

func hasLocalWorkerBinary(platform, arch string) bool {
	fileName := fmt.Sprintf("byog-worker-%s-%s", platform, arch)
	if platform == "windows" {
		fileName += ".exe"
	} else if platform == "android" && arch == "apk" {
		fileName = "byog-worker.apk"
	}

	searchPaths := []string{
		filepath.Join("worker", "binaries", fileName),
		filepath.Join("/worker", "binaries", fileName),
		filepath.Join("/data", "worker", "binaries", fileName),
	}

	for _, p := range searchPaths {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() && fi.Size() > 0 {
			return true
		}
	}
	return false
}

// GetByogWorkerVersion returns latest version for the self-updater engine
func GetByogWorkerVersion(c *gin.Context) {
	winUrl := "/api/v1/worker/download/windows/amd64"
	linUrl := "/api/v1/worker/download/linux/amd64"
	darUrl := "/api/v1/worker/download/darwin/arm64"
	apkUrl := "/api/v1/worker/download/android/apk"

	c.JSON(http.StatusOK, gin.H{
		"version":      "1.8.0",
		"min_version":  "1.0.0",
		"release_date": time.Now().Format("2006-01-02"),
		"downloads": gin.H{
			"windows_amd64": winUrl,
			"linux_amd64":   linUrl,
			"darwin_arm64":  darUrl,
			"android_apk":   apkUrl,
		},
	})
}

// DownloadByogWorker serves worker release binary or APK if present, or redirects to configured external download URL
func DownloadByogWorker(c *gin.Context) {
	platform := c.Param("platform")
	arch := c.Param("arch")
	fileName := fmt.Sprintf("byog-worker-%s-%s", platform, arch)
	if platform == "windows" {
		fileName += ".exe"
	} else if platform == "android" {
		if arch == "apk" {
			fileName = "byog-worker.apk"
		} else {
			fileName = "byog-worker-android-arm64"
		}
	}

	searchPaths := []string{
		filepath.Join("worker", "binaries", fileName),
		filepath.Join("..", "worker", "binaries", fileName),
		filepath.Join("/worker", "binaries", fileName),
		filepath.Join("/data", "worker", "binaries", fileName),
		filepath.Join(".", "worker", "binaries", fileName),
	}
	if execPath, err := os.Executable(); err == nil {
		searchPaths = append(searchPaths, filepath.Join(filepath.Dir(execPath), "worker", "binaries", fileName))
		searchPaths = append(searchPaths, filepath.Join(filepath.Dir(execPath), "..", "worker", "binaries", fileName))
	}

	for _, p := range searchPaths {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() && fi.Size() > 0 {
			if strings.HasSuffix(fileName, ".apk") {
				c.Header("Content-Type", "application/vnd.android.package-archive")
				c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileName))
				if raw, err := os.ReadFile(p); err == nil {
					scheme := "http"
					if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
						scheme = "https"
					}
					serverUrl := fmt.Sprintf("%s://%s", scheme, c.Request.Host)

					systemName, appName, faviconUrl, faviconBase64 := getBranding(serverUrl)

					cfgPayload, _ := json.Marshal(map[string]string{
						"server_url":     serverUrl,
						"system_name":    systemName,
						"app_name":       appName,
						"favicon":        faviconUrl,
						"favicon_base64": faviconBase64,
					})

					if customizedApk, err := injectApkConfig(raw, cfgPayload); err == nil {
						c.Data(http.StatusOK, "application/vnd.android.package-archive", customizedApk)
						return
					}
				}
				c.File(p)
				return
			} else {
				c.Header("Content-Type", "application/octet-stream")
				c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileName))
				c.File(p)
				return
			}
		}
	}

	c.JSON(http.StatusNotFound, gin.H{
		"success": false,
		"message": "Worker client binary is not present on this server.",
	})
}

type ByogWorkerInferenceTestReq struct {
	NodeId string `json:"node_id"`
	HWID   string `json:"hwid"`
	Prompt string `json:"prompt"`
	Model  string `json:"model"`
}

func TestByogNodeInference(c *gin.Context) {
	var req ByogWorkerInferenceTestReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Prompt == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid test request"})
		return
	}

	var node *model.ByogNode
	if req.NodeId != "" {
		node, _ = model.GetByogNodeByNodeId(req.NodeId)
	}
	if node == nil && req.HWID != "" {
		node, _ = model.GetByogNodeByHWID(req.HWID)
	}
	if node == nil {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "Unregistered device"})
		return
	}

	start := time.Now()
	testModel := strings.TrimSpace(req.Model)
	if testModel == "" && node.InstalledModels != "" {
		var models []string
		if err := json.Unmarshal([]byte(node.InstalledModels), &models); err == nil && len(models) > 0 {
			testModel = models[0]
		}
	}
	if testModel == "" {
		testModel = "qwen-2.5-0.5b-instruct"
	}

	testPrompt := strings.TrimSpace(req.Prompt)

	// Check if the node is currently connected to the WebSocket tunnel
	session, ok := byogservice.GlobalHub.GetSession(node.NodeId)
	if !ok || session == nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "BYOG cihazı çevrimdışı. Lütfen cihazınızda uygulamayı veya worker'ı çalıştırarak bağlantıyı sağlayın.",
		})
		return
	}

	reqId := fmt.Sprintf("test_%d", time.Now().UnixNano())
	payload := byogservice.InferencePayload{
		RequestId:   reqId,
		ModelId:     testModel,
		Prompt:      testPrompt,
		Stream:      false,
		Temperature: 0.7,
		MaxTokens:   512,
	}
	rawPayload, _ := json.Marshal(payload)
	respChan := make(chan *byogservice.TunnelFrame, 100)
	session.PendingRequests.Store(reqId, respChan)
	defer session.PendingRequests.Delete(reqId)

	err := session.WriteFrame(byogservice.TunnelFrame{
		Type:    byogservice.MsgTypeInferenceReq,
		Id:      reqId,
		Payload: rawPayload,
	})
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": fmt.Sprintf("Cihaza test isteği iletilemedi: %v", err),
		})
		return
	}

	var replyBuilder strings.Builder
	var reportedTps float64 = 32.0
	timeout := time.After(30 * time.Second)
	done := false

	for !done {
		select {
		case frame, ok := <-respChan:
			if !ok {
				done = true
				break
			}
			switch frame.Type {
			case byogservice.MsgTypeInferenceChunk:
				var chunk byogservice.ChunkPayload
				if err := json.Unmarshal(frame.Payload, &chunk); err == nil {
					replyBuilder.WriteString(chunk.Content)
				}
			case byogservice.MsgTypeInferenceDone:
				var donePayload byogservice.DonePayload
				if err := json.Unmarshal(frame.Payload, &donePayload); err == nil && donePayload.Tps > 0 {
					reportedTps = donePayload.Tps
				}
				done = true
			}
		case <-timeout:
			done = true
		}
	}

	reply := strings.TrimSpace(replyBuilder.String())
	if reply == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Cihazdan test yanıtı alınamadı (zaman aşımı veya cihaz meşgul).",
		})
		return
	}

	elapsed := time.Since(start).Milliseconds()
	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"reply":      reply,
		"latency_ms": elapsed,
		"tps":        reportedTps,
	})
}

func getByogModelEstimatedSizeMB(modelId string) int {
	lower := strings.ToLower(modelId)
	if strings.Contains(lower, "0.5b") {
		return 380
	}
	if strings.Contains(lower, "1.5b") || strings.Contains(lower, "2b") || strings.Contains(lower, "3b") {
		return 1150
	}
	if strings.Contains(lower, "7b") || strings.Contains(lower, "8b") {
		return 4600
	}
	if strings.Contains(lower, "embedding") || strings.Contains(lower, "bge") {
		return 450
	}
	return 500
}



