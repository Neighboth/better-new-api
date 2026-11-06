package controller

import (
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// The observability of the BYOG network for the administration.
//
// Three endpoints answer the operational questions of the panel: the health
// summary of the node network with the crash counters, the pages of the
// crash reports of the workers with the resolved names of the devices and of
// the users and the list of the nodes that are stuck with a failed model or
// a failed transfer.

// byogObservabilitySummary is the health part of the answer.
type byogObservabilitySummary struct {
	NodeHealth        *model.ByogNodeHealth      `json:"node_health"`
	CrashReports24h   int64                      `json:"crash_reports_24h"`
	CrashReports7d    int64                      `json:"crash_reports_7d"`
	CrashReportsTotal int64                      `json:"crash_reports_total"`
	TopErrorCodes     []model.SiteStatsNameCount `json:"top_error_codes"`
}

// GetByogObservabilitySummary answers with the operational state of the
// node network.
func GetByogObservabilitySummary(c *gin.Context) {
	now := time.Now().Unix()
	health, err := model.GetByogNodeHealth(now)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	summary := byogObservabilitySummary{NodeHealth: health}
	if summary.CrashReports24h, err = model.CountByogCrashReportsSince(now - 86400); err != nil {
		common.ApiError(c, err)
		return
	}
	if summary.CrashReports7d, err = model.CountByogCrashReportsSince(now - 7*86400); err != nil {
		common.ApiError(c, err)
		return
	}
	if summary.CrashReportsTotal, err = model.CountByogCrashReports(); err != nil {
		common.ApiError(c, err)
		return
	}
	if summary.TopErrorCodes, err = model.TopByogCrashErrorCodes(8); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, summary)
}

// byogCrashReportItem is one line of the crash report page, the names of the
// device and of the user are resolved so the panel shows them without a
// second round trip.
type byogCrashReportItem struct {
	*model.ByogCrashReport
	Username   string `json:"username"`
	DeviceName string `json:"device_name"`
}

// GetByogCrashReports answers with one page of the crash reports of the
// workers ordered by the arrival.
func GetByogCrashReports(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	nodeId := c.Query("node_id")
	errorCode := c.Query("error_code")

	rows, total, err := model.ListByogCrashReportsPage(nodeId, errorCode, page, pageSize)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	userIds := make([]int, 0, len(rows))
	nodeIds := make([]string, 0, len(rows))
	seenUsers := make(map[int]bool, len(rows))
	seenNodes := make(map[string]bool, len(rows))
	for _, row := range rows {
		if row.UserId > 0 && !seenUsers[row.UserId] {
			seenUsers[row.UserId] = true
			userIds = append(userIds, row.UserId)
		}
		if row.NodeId != "" && !seenNodes[row.NodeId] {
			seenNodes[row.NodeId] = true
			nodeIds = append(nodeIds, row.NodeId)
		}
	}
	usernames, nameErr := model.GetSiteStatsUsernames(userIds)
	if nameErr != nil {
		usernames = map[int]string{}
	}
	devices, deviceErr := model.GetByogNodeDeviceNames(nodeIds)
	if deviceErr != nil {
		devices = map[string]string{}
	}

	items := make([]byogCrashReportItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, byogCrashReportItem{
			ByogCrashReport: row,
			Username:        usernames[row.UserId],
			DeviceName:      devices[row.NodeId],
		})
	}
	common.ApiSuccess(c, gin.H{
		"items":     items,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// byogDownloadErrorItem is one line of the download problems of the network.
type byogDownloadErrorItem struct {
	NodeId            string `json:"node_id"`
	DeviceName        string `json:"device_name"`
	DeviceType        string `json:"device_type"`
	UserId            int    `json:"user_id"`
	Username          string `json:"username"`
	Status            string `json:"status"`
	ModelStatus       string `json:"model_status"`
	DownloadStatus    string `json:"download_status"`
	DownloadModelId   string `json:"download_model_id"`
	DownloadProgress  int    `json:"download_progress"`
	DownloadedMB      int    `json:"downloaded_mb"`
	TotalMB           int    `json:"total_mb"`
	DownloadStartedAt int64  `json:"download_started_at"`
	LastHeartbeatAt   int64  `json:"last_heartbeat_at"`
}

// GetByogDownloadErrors answers with the nodes that are stuck with a failed
// model or a failed transfer.
func GetByogDownloadErrors(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit < 1 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	nodes, err := model.ListByogNodesWithErrors(limit)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	userIds := make([]int, 0, len(nodes))
	seenUsers := make(map[int]bool, len(nodes))
	for _, node := range nodes {
		if node.UserId > 0 && !seenUsers[node.UserId] {
			seenUsers[node.UserId] = true
			userIds = append(userIds, node.UserId)
		}
	}
	usernames, nameErr := model.GetSiteStatsUsernames(userIds)
	if nameErr != nil {
		usernames = map[int]string{}
	}

	items := make([]byogDownloadErrorItem, 0, len(nodes))
	for _, node := range nodes {
		items = append(items, byogDownloadErrorItem{
			NodeId:            node.NodeId,
			DeviceName:        node.DeviceName,
			DeviceType:        node.DeviceType,
			UserId:            node.UserId,
			Username:          usernames[node.UserId],
			Status:            node.Status,
			ModelStatus:       node.ModelStatus,
			DownloadStatus:    node.DownloadStatus,
			DownloadModelId:   node.DownloadModelId,
			DownloadProgress:  node.DownloadProgress,
			DownloadedMB:      node.DownloadedMB,
			TotalMB:           node.TotalMB,
			DownloadStartedAt: node.DownloadStartedAt,
			LastHeartbeatAt:   node.LastHeartbeatAt,
		})
	}
	common.ApiSuccess(c, gin.H{"items": items})
}

// ClearByogCrashReports archives the reports to the configured log directory
// before deleting them. Live node health and billing logs are not modified.
func ClearByogCrashReports(c *gin.Context) {
	count, err := model.ArchiveAndDeleteByogCrashReports(*common.LogDir)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"deleted": count})
}
