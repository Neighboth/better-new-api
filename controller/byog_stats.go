package controller

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type ByogDailyStat struct {
	Date        string `json:"date"`
	Requests    int64  `json:"requests"`
	Tokens      int64  `json:"tokens"`
	UniqueUsers int64  `json:"unique_users"`
	ActiveNodes int64  `json:"active_nodes"`
	NewNodes    int64  `json:"new_nodes"`
}

type ByogTopNodeStat struct {
	NodeId     string `json:"node_id"`
	DeviceName string `json:"device_name"`
	DeviceType string `json:"device_type"`
	UserId     int    `json:"user_id"`
	Username   string `json:"username"`
	Requests   int64  `json:"requests"`
	Tokens     int64  `json:"tokens"`
}

type ByogTopModelStat struct {
	ModelId    string  `json:"model_id"`
	Requests   int64   `json:"requests"`
	Tokens     int64   `json:"tokens"`
	Percentage float64 `json:"percentage"`
}

type ByogTopUserStat struct {
	UserId    int    `json:"user_id"`
	Username  string `json:"username"`
	Requests  int64  `json:"requests"`
	Tokens    int64  `json:"tokens"`
	QuotaUsed int64  `json:"quota_used"`
}

type ByogDeviceTypeStat struct {
	DeviceType string `json:"device_type"`
	NodeCount  int64  `json:"node_count"`
	Requests   int64  `json:"requests"`
}

type ByogAdminStatsResponse struct {
	PeriodDays         int                  `json:"period_days"`
	TotalRequests      int64                `json:"total_requests"`
	TotalTokens        int64                `json:"total_tokens"`
	TotalUsers         int64                `json:"total_users"`
	TotalNodes         int64                `json:"total_nodes"`
	OnlineNodes        int64                `json:"online_nodes"`
	ActiveNodes        int64                `json:"active_nodes"`
	AvgUseTimeSeconds  float64              `json:"avg_use_time_seconds"`
	TotalCreditsEarned float64              `json:"total_credits_earned"`
	TimeSeries         []ByogDailyStat      `json:"time_series"`
	TopNodes           []ByogTopNodeStat    `json:"top_nodes"`
	TopModels          []ByogTopModelStat   `json:"top_models"`
	TopUsers           []ByogTopUserStat    `json:"top_users"`
	DeviceBreakdown    []ByogDeviceTypeStat `json:"device_breakdown"`
	HourlyDistribution [24]int64            `json:"hourly_distribution"`
}

// GetByogAdminStats aggregates extensive time-series and usage analytics for BYOG compute network.
func GetByogAdminStats(c *gin.Context) {
	rangeParam := strings.ToLower(strings.TrimSpace(c.DefaultQuery("range", "30d")))
	filterDevice := strings.ToLower(strings.TrimSpace(c.Query("device_type")))
	filterUserId, _ := strconv.Atoi(c.Query("user_id"))

	periodDays := 30
	switch rangeParam {
	case "7d":
		periodDays = 7
	case "14d":
		periodDays = 14
	case "30d":
		periodDays = 30
	case "90d":
		periodDays = 90
	case "all":
		periodDays = 365
	}

	now := time.Now()
	startTime := now.AddDate(0, 0, -periodDays)
	startUnix := startTime.Unix()

	// 1. Fetch all BYOG nodes from DB
	var allNodes []model.ByogNode
	nodeQuery := model.DB.Model(&model.ByogNode{})
	if filterDevice != "" {
		nodeQuery = nodeQuery.Where("device_type = ?", filterDevice)
	}
	if filterUserId > 0 {
		nodeQuery = nodeQuery.Where("user_id = ?", filterUserId)
	}
	_ = nodeQuery.Find(&allNodes)

	nodeMap := make(map[string]*model.ByogNode, len(allNodes))
	var onlineNodesCount int64
	var totalCreditsEarned float64
	deviceNodeCount := map[string]int64{
		"desktop": 0,
		"mobile":  0,
		"tablet":  0,
	}

	for i := range allNodes {
		n := &allNodes[i]
		nodeMap[n.NodeId] = n
		if n.Status == "online" && (now.Unix()-n.LastHeartbeatAt) < 180 {
			onlineNodesCount++
		}
		totalCreditsEarned += n.TotalEarnedCredits
		dt := strings.ToLower(n.DeviceType)
		if dt == "" {
			dt = "desktop"
		}
		deviceNodeCount[dt]++
	}

	// 2. Fetch logs with ChannelId = 0 and Content LIKE 'BYOG%'
	type LogRow struct {
		Id               int
		UserId           int
		CreatedAt        int64
		ModelName        string
		PromptTokens     int
		CompletionTokens int
		Quota            int
		UseTime          int
		Other            string
	}

	var logs []LogRow
	logQuery := model.DB.Table("logs").
		Select("id, user_id, created_at, model_name, prompt_tokens, completion_tokens, quota, use_time, other").
		Where("channel_id = ? AND (other LIKE ? OR content LIKE ?)", 0, "%byog%", "%BYOG%").
		Where("created_at >= ?", startUnix)

	if filterUserId > 0 {
		logQuery = logQuery.Where("user_id = ?", filterUserId)
	}
	_ = logQuery.Find(&logs)

	// User usernames cache
	userNames := make(map[int]string)
	getUserName := func(uid int) string {
		if uid <= 0 {
			return "Anonymous"
		}
		if name, ok := userNames[uid]; ok {
			return name
		}
		var u model.User
		if err := model.DB.Select("id, username").First(&u, uid).Error; err == nil && u.Username != "" {
			userNames[uid] = u.Username
			return u.Username
		}
		name := "User #" + strconv.Itoa(uid)
		userNames[uid] = name
		return name
	}

	// 3. Aggregate metrics
	var totalRequests int64
	var totalTokens int64
	var totalUseTime int64
	uniqueUsersMap := make(map[int]bool)
	activeNodesMap := make(map[string]bool)

	nodeStats := make(map[string]*ByogTopNodeStat)
	modelStats := make(map[string]*ByogTopModelStat)
	userStats := make(map[int]*ByogTopUserStat)
	deviceRequests := map[string]int64{
		"desktop": 0,
		"mobile":  0,
		"tablet":  0,
	}
	var hourlyDist [24]int64

	// Daily map
	dailyMap := make(map[string]*ByogDailyStat)
	// Initialize all days in the period
	for d := 0; d <= periodDays; d++ {
		dayDate := startTime.AddDate(0, 0, d).Format("2006-01-02")
		dailyMap[dayDate] = &ByogDailyStat{
			Date: dayDate,
		}
	}

	// Also count new nodes created per day
	for _, n := range allNodes {
		if n.CreatedAt >= startUnix {
			dayStr := time.Unix(n.CreatedAt, 0).Format("2006-01-02")
			if stat, ok := dailyMap[dayStr]; ok {
				stat.NewNodes++
			}
		}
	}

	dayUserMap := make(map[string]map[int]bool)
	dayNodeMap := make(map[string]map[string]bool)

	for _, l := range logs {
		t := time.Unix(l.CreatedAt, 0)
		dateStr := t.Format("2006-01-02")
		hour := t.Hour()
		if hour >= 0 && hour < 24 {
			hourlyDist[hour]++
		}

		totalRequests++
		tokens := int64(l.PromptTokens + l.CompletionTokens)
		totalTokens += tokens
		totalUseTime += int64(l.UseTime)

		uniqueUsersMap[l.UserId] = true

		// Parse node info from other JSON
		nodeId := ""
		deviceName := ""
		deviceType := "desktop"
		if l.Other != "" {
			var otherData map[string]interface{}
			if err := json.Unmarshal([]byte(l.Other), &otherData); err == nil {
				if nid, ok := otherData["node_id"].(string); ok {
					nodeId = nid
				}
				if dn, ok := otherData["device_name"].(string); ok {
					deviceName = dn
				}
				if dt, ok := otherData["device_type"].(string); ok && dt != "" {
					deviceType = dt
				}
			}
		}

		if nodeId != "" {
			if filterDevice != "" && strings.ToLower(deviceType) != filterDevice {
				continue
			}
			activeNodesMap[nodeId] = true

			if _, ok := nodeStats[nodeId]; !ok {
				nodeStats[nodeId] = &ByogTopNodeStat{
					NodeId:     nodeId,
					DeviceName: deviceName,
					DeviceType: deviceType,
				}
				if n, exists := nodeMap[nodeId]; exists {
					nodeStats[nodeId].DeviceName = n.DeviceName
					nodeStats[nodeId].DeviceType = n.DeviceType
					nodeStats[nodeId].UserId = n.UserId
					nodeStats[nodeId].Username = getUserName(n.UserId)
				}
			}
			nodeStats[nodeId].Requests++
			nodeStats[nodeId].Tokens += tokens
		}

		deviceRequests[deviceType]++

		// Model stats
		mName := l.ModelName
		if mName != "" {
			if _, ok := modelStats[mName]; !ok {
				modelStats[mName] = &ByogTopModelStat{
					ModelId: mName,
				}
			}
			modelStats[mName].Requests++
			modelStats[mName].Tokens += tokens
		}

		// User stats
		if l.UserId > 0 {
			if _, ok := userStats[l.UserId]; !ok {
				userStats[l.UserId] = &ByogTopUserStat{
					UserId:   l.UserId,
					Username: getUserName(l.UserId),
				}
			}
			userStats[l.UserId].Requests++
			userStats[l.UserId].Tokens += tokens
			userStats[l.UserId].QuotaUsed += int64(l.Quota)
		}

		// Daily time series tracking
		if dayStat, ok := dailyMap[dateStr]; ok {
			dayStat.Requests++
			dayStat.Tokens += tokens
			if _, exists := dayUserMap[dateStr]; !exists {
				dayUserMap[dateStr] = make(map[int]bool)
			}
			dayUserMap[dateStr][l.UserId] = true
			if nodeId != "" {
				if _, exists := dayNodeMap[dateStr]; !exists {
					dayNodeMap[dateStr] = make(map[string]bool)
				}
				dayNodeMap[dateStr][nodeId] = true
			}
		}
	}

	// Fill daily user/node counts
	var timeSeries []ByogDailyStat
	for d := 0; d <= periodDays; d++ {
		dayDate := startTime.AddDate(0, 0, d).Format("2006-01-02")
		if stat, ok := dailyMap[dayDate]; ok {
			if uMap, exists := dayUserMap[dayDate]; exists {
				stat.UniqueUsers = int64(len(uMap))
			}
			if nMap, exists := dayNodeMap[dayDate]; exists {
				stat.ActiveNodes = int64(len(nMap))
			}
			timeSeries = append(timeSeries, *stat)
		}
	}

	// Format Top Nodes (Sort descending by requests)
	var topNodes []ByogTopNodeStat
	for _, stat := range nodeStats {
		topNodes = append(topNodes, *stat)
	}
	// Sort topNodes
	for i := 0; i < len(topNodes); i++ {
		for j := i + 1; j < len(topNodes); j++ {
			if topNodes[j].Requests > topNodes[i].Requests {
				topNodes[i], topNodes[j] = topNodes[j], topNodes[i]
			}
		}
	}
	if len(topNodes) > 10 {
		topNodes = topNodes[:10]
	}

	// Format Top Models
	var topModels []ByogTopModelStat
	for _, stat := range modelStats {
		if totalRequests > 0 {
			stat.Percentage = (float64(stat.Requests) / float64(totalRequests)) * 100.0
		}
		topModels = append(topModels, *stat)
	}
	for i := 0; i < len(topModels); i++ {
		for j := i + 1; j < len(topModels); j++ {
			if topModels[j].Requests > topModels[i].Requests {
				topModels[i], topModels[j] = topModels[j], topModels[i]
			}
		}
	}
	if len(topModels) > 10 {
		topModels = topModels[:10]
	}

	// Format Top Users
	var topUsers []ByogTopUserStat
	for _, stat := range userStats {
		topUsers = append(topUsers, *stat)
	}
	for i := 0; i < len(topUsers); i++ {
		for j := i + 1; j < len(topUsers); j++ {
			if topUsers[j].Requests > topUsers[i].Requests {
				topUsers[i], topUsers[j] = topUsers[j], topUsers[i]
			}
		}
	}
	if len(topUsers) > 10 {
		topUsers = topUsers[:10]
	}

	// Device breakdown
	deviceBreakdown := []ByogDeviceTypeStat{
		{DeviceType: "desktop", NodeCount: deviceNodeCount["desktop"], Requests: deviceRequests["desktop"]},
		{DeviceType: "mobile", NodeCount: deviceNodeCount["mobile"], Requests: deviceRequests["mobile"]},
		{DeviceType: "tablet", NodeCount: deviceNodeCount["tablet"], Requests: deviceRequests["tablet"]},
	}

	avgUseTime := 0.0
	if totalRequests > 0 {
		avgUseTime = float64(totalUseTime) / float64(totalRequests)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": ByogAdminStatsResponse{
			PeriodDays:         periodDays,
			TotalRequests:      totalRequests,
			TotalTokens:        totalTokens,
			TotalUsers:         int64(len(uniqueUsersMap)),
			TotalNodes:         int64(len(allNodes)),
			OnlineNodes:        onlineNodesCount,
			ActiveNodes:        int64(len(activeNodesMap)),
			AvgUseTimeSeconds:  avgUseTime,
			TotalCreditsEarned: totalCreditsEarned,
			TimeSeries:         timeSeries,
			TopNodes:           topNodes,
			TopModels:          topModels,
			TopUsers:           topUsers,
			DeviceBreakdown:    deviceBreakdown,
			HourlyDistribution: hourlyDist,
		},
	})
}
