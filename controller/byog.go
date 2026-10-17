package controller

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	if userId > 0 {
		isBanned, banInfo = model.IsUserBannedFromByog(userId)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"enabled":   setting.Enabled,
			"is_banned": isBanned,
			"ban_info":  banInfo,
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

// DeleteUserByogNode unlinks device
func DeleteUserByogNode(c *gin.Context) {
	userId := c.GetInt("id")
	nodeId := c.Param("id")

	node, err := model.GetByogNodeByNodeId(nodeId)
	if err != nil || node == nil || node.UserId != userId {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Device not found"})
		return
	}

	byogservice.GlobalHub.Unregister(nodeId)
	_ = model.DeleteByogNode(nodeId)

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

	// Fetch user balance
	user, err := model.GetUserById(userId, false)
	if err != nil || user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "User not found"})
		return
	}

	// Verify required tokens (quota)
	if int64(user.Quota) < reward.RequiredTokens {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": fmt.Sprintf("Insufficient token balance. Required: %d tokens, Available: %d tokens", reward.RequiredTokens, user.Quota),
		})
		return
	}

	// Deduct user quota
	if err := model.DecreaseUserQuota(userId, int(reward.RequiredTokens), true); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Failed to deduct tokens"})
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
		VerificationLog: fmt.Sprintf("Timestamp: %s, User: %s, Burned: %d tokens, Stock left: %d", time.Now().Format(time.RFC3339), user.Username, reward.RequiredTokens, reward.Stock),
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

// ----------------- WORKER / REVERSE TUNNEL CONTROLLERS -----------------

// RegisterByogWorker is called when worker binary starts up
func RegisterByogWorker(c *gin.Context) {
	var req struct {
		HWID         string  `json:"hwid" binding:"required"`
		OS           string  `json:"os"`
		Arch         string  `json:"arch"`
		DeviceName   string  `json:"device_name"`
		DeviceType   string  `json:"device_type"`
		CpuModel     string  `json:"cpu_model"`
		GpuModel     string  `json:"gpu_model"`
		NpuModel     string  `json:"npu_model"`
		PhysicalRam  int     `json:"physical_ram_mb"`
		SwapRam      int     `json:"swap_ram_mb"`
		VramMB       int     `json:"vram_mb"`
		SharedVramMB int     `json:"shared_vram_mb"`
		TeeSupported bool    `json:"tee_supported"`
		TeeType      string  `json:"tee_type"`
		DownMbps     float64 `json:"bandwidth_down_mbps"`
		UpMbps       float64 `json:"bandwidth_up_mbps"`
		PingMs       int     `json:"ping_ms"`
		NetworkType  string  `json:"network_type"`
		ClientIp     string  `json:"client_ip"`
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

	pairingCode := generateRandomPairingCode()

	node, err := model.GetByogNodeByHWID(req.HWID)
	if err == nil && node != nil {
		// Existing node: update hardware specs
		node.DeviceName = req.DeviceName
		node.CpuModel = req.CpuModel
		node.GpuModel = req.GpuModel
		node.NpuModel = req.NpuModel
		node.PhysicalRamMB = req.PhysicalRam
		node.SwapRamMB = req.SwapRam
		node.VramMB = req.VramMB
		node.SharedVramMB = req.SharedVramMB
		node.TeeSupported = req.TeeSupported
		node.TeeType = req.TeeType
		node.BandwidthDownMbps = req.DownMbps
		node.BandwidthUpMbps = req.UpMbps
		node.PingMs = req.PingMs
		node.NetworkType = req.NetworkType
		if node.UserId == 0 {
			node.PairingCode = pairingCode
		}
		_ = model.SaveByogNode(node)
	} else {
		// New node
		node = &model.ByogNode{
			NodeId:               nodeId,
			PairingCode:          pairingCode,
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
			Status:               "online",
			OperatingMode:        "mode_b", // DEFAULT & RECOMMENDED (Section 4.3)
			PowerProfile:         "full_power",
			AllocatedDiskGB:      40,
			AllocatedVramGB:      6,
			AllocatedRamGB:       8,
			AllocatedCores:       4,
			WifiOnlyDownload:     true, // DEFAULT TRUE (Section 6.3)
			MonthlyDataLimitMB:   0,    // DEFAULT UNLIMITED (Section 6.3)
			RunOnlyWhileCharging: false, // DEFAULT FALSE (Section 6.3)
			BatteryCutoffEnabled: false, // DEFAULT FALSE (Section 6.3)
			BatteryCutoffPercent: 20,
			MaxBatteryTemp:       50.0, // DEFAULT 50°C (Section 6.3)
			BatteryTempSupported: true,
			ReputationScore:      50,
			CreatedAt:            time.Now().Unix(),
			UpdatedAt:            time.Now().Unix(),
		}
		_ = model.SaveByogNode(node)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"node_id":      node.NodeId,
			"pairing_code": node.PairingCode,
			"user_id":      node.UserId,
			"is_paired":    node.UserId > 0,
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
		NodeId:         nodeId,
		UserId:         node.UserId,
		Conn:           conn,
		SendChan:       make(chan byogservice.TunnelFrame, 100),
		LastHeartbeat:  time.Now(),
		UptimeStart:    time.Now(),
		MaxConcurrency: 4,
		DeviceType:     node.DeviceType,
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

// GetByogWorkerVersion returns latest version for the self-updater engine
func GetByogWorkerVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"version":      "1.3.0",
		"min_version":  "1.0.0",
		"release_date": time.Now().Format("2006-01-02"),
		"downloads": gin.H{
			"windows_amd64": "/api/v1/worker/download/windows/amd64",
			"linux_amd64":   "/api/v1/worker/download/linux/amd64",
			"darwin_arm64":  "/api/v1/worker/download/darwin/arm64",
			"android_apk":   "/api/v1/worker/download/android/apk",
		},
	})
}

// DownloadByogWorker serves worker release binary or APK if present, or returns 404
func DownloadByogWorker(c *gin.Context) {
	platform := c.Param("platform")
	arch := c.Param("arch")
	fileName := fmt.Sprintf("byog-worker-%s-%s", platform, arch)
	if platform == "windows" {
		fileName += ".exe"
	} else if platform == "android" && arch == "apk" {
		fileName = "byog-worker.apk"
	}
	filePath := filepath.Join("worker", "binaries", fileName)
	if _, err := os.Stat(filePath); err == nil {
		c.FileAttachment(filePath, fileName)
		return
	}
	c.JSON(http.StatusNotFound, gin.H{
		"success": false,
		"message": "Worker release package is not available for this platform yet",
	})
}

