package controller

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

func GetAllLogs(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	username := c.Query("username")
	tokenName := c.Query("token_name")
	modelName := c.Query("model_name")
	channel, _ := strconv.Atoi(c.Query("channel"))
	group := c.Query("group")
	requestId := c.Query("request_id")
	upstreamRequestId := c.Query("upstream_request_id")
	logs, total, err := model.GetAllLogs(logType, startTimestamp, endTimestamp, modelName, username, tokenName, pageInfo.GetStartIdx(), pageInfo.GetPageSize(), channel, group, requestId, upstreamRequestId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	common.ApiSuccess(c, pageInfo)
	return
}

func GetUserLogs(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	userId := c.GetInt("id")
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	tokenName := c.Query("token_name")
	modelName := c.Query("model_name")
	group := c.Query("group")
	requestId := c.Query("request_id")
	upstreamRequestId := c.Query("upstream_request_id")
	logs, total, err := model.GetUserLogs(userId, logType, startTimestamp, endTimestamp, modelName, tokenName, pageInfo.GetStartIdx(), pageInfo.GetPageSize(), group, requestId, upstreamRequestId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	common.ApiSuccess(c, pageInfo)
	return
}

// Deprecated: SearchAllLogs 已废弃，前端未使用该接口。
func SearchAllLogs(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": false,
		"message": "该接口已废弃",
	})
}

// Deprecated: SearchUserLogs 已废弃，前端未使用该接口。
func SearchUserLogs(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": false,
		"message": "该接口已废弃",
	})
}

func GetLogByKey(c *gin.Context) {
	tokenId := c.GetInt("token_id")
	if tokenId == 0 {
		c.JSON(200, gin.H{
			"success": false,
			"message": "无效的令牌",
		})
		return
	}
	logs, err := model.GetLogByTokenId(tokenId)
	if err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(200, gin.H{
		"success": true,
		"message": "",
		"data":    logs,
	})
}

// The export of the usage logs.
//
// The panel builds the file in the browser (CSV, TXT, log lines, JSON, a SQL
// dump or Parquet) from the rows of this endpoint, so the server never depends
// on a file format library. What the server owns is the part a browser cannot
// own: the captcha of the download, the ten minute cooldown between two
// downloads and the hard cap of the payload.
const (
	// exportMaxBytes is the size of the file a download may produce.
	exportMaxBytes = 30 << 20
	// exportCooldown is how long an administrator waits between two downloads.
	exportCooldown = 10 * time.Minute
	// exportMaxRows bounds the query: a row is never larger than a few
	// kilobytes, this is the ceiling of the scan before the byte cap applies.
	exportMaxRows = 50000
)

var (
	exportCooldownMutex sync.Mutex
	exportCooldownUntil = map[int]int64{}
)

// exportCooldownRemaining returns the timestamp until which the user may not
// export, or zero when the download is allowed.
func exportCooldownRemaining(userId int, now int64) int64 {
	exportCooldownMutex.Lock()
	defer exportCooldownMutex.Unlock()
	until := exportCooldownUntil[userId]
	if until <= now {
		return 0
	}
	return until
}

func markExportCooldown(userId int, until int64) {
	exportCooldownMutex.Lock()
	defer exportCooldownMutex.Unlock()
	exportCooldownUntil[userId] = until
}

// GetLogsExport returns the rows of the filtered logs for the export of the
// panel.
//
// The rows are trimmed from the oldest end when the payload would pass the cap:
// a download of a year of logs is one year of the newest logs, and the panel
// tells the reader that the file was trimmed.
func GetLogsExport(c *gin.Context) {
	exportLogs(c, false)
}

// GetLogsSelfExport is the export of the logs of the caller.
//
// The username of the filter is replaced by the caller on purpose: a query
// parameter must never widen a download to the logs of somebody else.
func GetLogsSelfExport(c *gin.Context) {
	exportLogs(c, true)
}

func exportLogs(c *gin.Context, selfOnly bool) {
	userId := c.GetInt("id")
	now := time.Now().Unix()
	if until := exportCooldownRemaining(userId, now); until > now {
		c.JSON(http.StatusOK, gin.H{
			"success":         false,
			"code":            "export_cooldown",
			"message":         "The next export is available in a few minutes.",
			"next_allowed_at": until,
		})
		return
	}

	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	username := c.Query("username")
	tokenName := c.Query("token_name")
	modelName := c.Query("model_name")
	channel, _ := strconv.Atoi(c.Query("channel"))
	group := c.Query("group")
	requestId := c.Query("request_id")
	upstreamRequestId := c.Query("upstream_request_id")
	if selfOnly {
		// The download of a user is the download of that user, whatever the
		// query says about a username.
		username = c.GetString("username")
		channel = 0
	}

	logs, total, err := model.GetAllLogs(logType, startTimestamp, endTimestamp, modelName, username, tokenName, 0, exportMaxRows, channel, group, requestId, upstreamRequestId)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// The rows are kept newest first, which is the order of the query. The cap
	// trims the oldest ones, so a reader who exports a huge range still gets the
	// recent history that matters.
	rows := make([]*model.Log, 0, len(logs))
	size := 0
	for _, entry := range logs {
		encoded, err := common.Marshal(entry)
		if err != nil {
			continue
		}
		if size+len(encoded) > exportMaxBytes {
			break
		}
		size += len(encoded)
		rows = append(rows, entry)
	}
	trimmed := int64(len(logs)) - int64(len(rows))
	if total > int64(len(logs)) {
		trimmed = total - int64(len(rows))
	}

	nextAllowed := now + int64(exportCooldown.Seconds())
	markExportCooldown(userId, nextAllowed)

	c.JSON(http.StatusOK, gin.H{
		"success":         true,
		"message":         "",
		"data": gin.H{
			"rows":            rows,
			"total":           total,
			"returned":        len(rows),
			"trimmed":         trimmed,
			"max_bytes":       exportMaxBytes,
			"next_allowed_at": nextAllowed,
			"generated_at":    now,
		},
	})
}

func GetLogsStat(c *gin.Context) {
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	tokenName := c.Query("token_name")
	username := c.Query("username")
	modelName := c.Query("model_name")
	channel, _ := strconv.Atoi(c.Query("channel"))
	group := c.Query("group")
	stat, err := model.SumUsedQuota(logType, startTimestamp, endTimestamp, modelName, username, tokenName, channel, group)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	//tokenNum := model.SumUsedToken(logType, startTimestamp, endTimestamp, modelName, username, "")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"quota": stat.Quota,
			"rpm":   stat.Rpm,
			"tpm":   stat.Tpm,
		},
	})
	return
}

func GetLogsSelfStat(c *gin.Context) {
	username := c.GetString("username")
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	tokenName := c.Query("token_name")
	modelName := c.Query("model_name")
	channel, _ := strconv.Atoi(c.Query("channel"))
	group := c.Query("group")
	quotaNum, err := model.SumUsedQuota(logType, startTimestamp, endTimestamp, modelName, username, tokenName, channel, group)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	//tokenNum := model.SumUsedToken(logType, startTimestamp, endTimestamp, modelName, username, tokenName)
	c.JSON(200, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"quota": quotaNum.Quota,
			"rpm":   quotaNum.Rpm,
			"tpm":   quotaNum.Tpm,
			//"token": tokenNum,
		},
	})
	return
}
