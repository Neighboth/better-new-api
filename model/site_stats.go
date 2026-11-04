package model

import (
	"fmt"

	"gorm.io/gorm"
)

// The site wide statistics of the administration.
//
// Every query of this file runs on the log database and on the main database
// with plain integer arithmetic and plain aggregates, the same code answers on
// SQLite, MySQL, PostgreSQL and ClickHouse. The bucket of a moment is
//
//	created_at - ((created_at + tz_offset) % bucket_size)
//
// which is the start of the bucket in the local timezone of the admin without
// a date function of a dialect.

// SiteStatsFilter carries the filters of the site wide statistics.
type SiteStatsFilter struct {
	StartTs        int64  // inclusive, unix seconds, 0 means no lower bound
	EndTs          int64  // exclusive, unix seconds, 0 means no upper bound
	TzOffsetSec    int64  // offset of the local timezone of the admin in seconds
	UserIds        []int  // the users that are included, empty includes every user
	ExcludeUserIds []int  // the users that are excluded
	ModelName      string // the like filter of the model name
	ChannelId      int    // the exact filter of the channel
	Group          string // the exact filter of the group
}

// SiteStatsBucket is one bucket of the time series of the requests.
type SiteStatsBucket struct {
	Bucket      int64 `gorm:"column:bucket" json:"bucket"`
	Requests    int64 `gorm:"column:requests" json:"requests"`
	Tokens      int64 `gorm:"column:tokens" json:"tokens"`
	Quota       int64 `gorm:"column:quota" json:"quota"`
	UniqueUsers int64 `gorm:"column:unique_users" json:"unique_users"`
}

// SiteStatsCount is a bucket with a single count.
type SiteStatsCount struct {
	Bucket int64 `gorm:"column:bucket" json:"bucket"`
	Count  int64 `gorm:"column:count" json:"count"`
}

// SiteStatsDayUser is the traffic of one user inside one day, the pairs are
// merged in the handler to the week and the month buckets with the real count
// of the distinct users.
type SiteStatsDayUser struct {
	Bucket   int64 `gorm:"column:bucket" json:"bucket"`
	UserId   int   `gorm:"column:user_id" json:"user_id"`
	Requests int64 `gorm:"column:requests" json:"requests"`
	Tokens   int64 `gorm:"column:tokens" json:"tokens"`
	Quota    int64 `gorm:"column:quota" json:"quota"`
}

// SiteStatsTopUser is one line of the ranking of the users.
type SiteStatsTopUser struct {
	UserId   int    `gorm:"column:user_id" json:"user_id"`
	Username string `gorm:"column:username" json:"username"`
	Requests int64  `gorm:"column:requests" json:"requests"`
	Tokens   int64  `gorm:"column:tokens" json:"tokens"`
	Quota    int64  `gorm:"column:quota" json:"quota"`
}

// SiteStatsTopModel is one line of the ranking of the models.
type SiteStatsTopModel struct {
	ModelName string `gorm:"column:model_name" json:"model_name"`
	Requests  int64  `gorm:"column:requests" json:"requests"`
	Tokens    int64  `gorm:"column:tokens" json:"tokens"`
	Quota     int64  `gorm:"column:quota" json:"quota"`
}

// SiteStatsTopChannel is one line of the ranking of the channels.
type SiteStatsTopChannel struct {
	ChannelId int   `gorm:"column:channel_id" json:"channel_id"`
	Requests  int64 `gorm:"column:requests" json:"requests"`
	Tokens    int64 `gorm:"column:tokens" json:"tokens"`
	Quota     int64 `gorm:"column:quota" json:"quota"`
}

// SiteStatsTopToken is one line of the ranking of the api keys, the name of
// the key is the name of the client of the user.
type SiteStatsTopToken struct {
	TokenName string `gorm:"column:token_name" json:"token_name"`
	Requests  int64  `gorm:"column:requests" json:"requests"`
	Tokens    int64  `gorm:"column:tokens" json:"tokens"`
	Quota     int64  `gorm:"column:quota" json:"quota"`
}

// SiteStatsTopGroup is one line of the ranking of the groups.
type SiteStatsTopGroup struct {
	GroupName string `gorm:"column:group_name" json:"group_name"`
	Requests  int64  `gorm:"column:requests" json:"requests"`
	Tokens    int64  `gorm:"column:tokens" json:"tokens"`
	Quota     int64  `gorm:"column:quota" json:"quota"`
}

// SiteStatsTopIp is one line of the ranking of the addresses.
type SiteStatsTopIp struct {
	Ip       string `gorm:"column:ip" json:"ip"`
	Requests int64  `gorm:"column:requests" json:"requests"`
	Tokens   int64  `gorm:"column:tokens" json:"tokens"`
}

// SiteStatsLogTypeCount is the count of one type of the logs.
type SiteStatsLogTypeCount struct {
	Type  int   `gorm:"column:type" json:"type"`
	Count int64 `gorm:"column:count" json:"count"`
}

// SiteStatsSummary is the aggregate of the whole selected range.
type SiteStatsSummary struct {
	Requests         int64   `gorm:"column:requests" json:"requests"`
	Tokens           int64   `gorm:"column:tokens" json:"tokens"`
	Quota            int64   `gorm:"column:quota" json:"quota"`
	UniqueUsers      int64   `gorm:"column:unique_users" json:"unique_users"`
	AvgUseTimeSecond float64 `gorm:"column:avg_use_time" json:"avg_use_time_seconds"`
	ByogRequests     int64   `gorm:"column:byog_requests" json:"byog_requests"`
}

// siteStatsRankingSort maps the key of the sort to the alias of the column,
// a key that is not in the map is rejected by the caller.
var siteStatsRankingSort = map[string]string{
	"requests": "requests",
	"tokens":   "tokens",
	"quota":    "quota",
}

// byogLogLikePattern finds the consume logs of the BYOG relay, the other
// column of the log carries the marker of the local node.
const byogLogLikePattern = `%"byog":true%`

// bucketExpr builds the expression of the bucket start for a column.
func bucketExpr(column string, tzOffsetSec int64, divSeconds int64) string {
	if divSeconds <= 0 {
		divSeconds = 86400
	}
	return fmt.Sprintf("%s - ((%s + (%d)) %% %d)", column, column, tzOffsetSec, divSeconds)
}

// hourOfDayExpr builds the expression of the second of the local day that is
// cut to the start of its hour, the index of the hour is the value divided by
// 3600 in the handler.
func hourOfDayExpr(column string, tzOffsetSec int64) string {
	return fmt.Sprintf("((%s + (%d)) %% 86400) - (((%s + (%d)) %% 86400) %% 3600)",
		column, tzOffsetSec, column, tzOffsetSec)
}

// applyLogsRange adds the range and the user filters to a query of the logs.
func (f SiteStatsFilter) applyLogsRange(tx *gorm.DB) *gorm.DB {
	if f.StartTs > 0 {
		tx = tx.Where("created_at >= ?", f.StartTs)
	}
	if f.EndTs > 0 {
		tx = tx.Where("created_at < ?", f.EndTs)
	}
	if len(f.UserIds) > 0 {
		tx = tx.Where("user_id IN ?", f.UserIds)
	}
	if len(f.ExcludeUserIds) > 0 {
		tx = tx.Where("user_id NOT IN ?", f.ExcludeUserIds)
	}
	return tx
}

// applyLogsRequest adds the filters that belong to the request logs.
func (f SiteStatsFilter) applyLogsRequest(tx *gorm.DB) *gorm.DB {
	tx = f.applyLogsRange(tx)
	if f.ModelName != "" {
		tx = tx.Where("model_name LIKE ?", "%"+f.ModelName+"%")
	}
	if f.ChannelId != 0 {
		tx = tx.Where("channel_id = ?", f.ChannelId)
	}
	if f.Group != "" {
		tx = tx.Where(logGroupCol+" = ?", f.Group)
	}
	return tx
}

// rankingOrder validates the key of the sort and builds the order clause.
func rankingOrder(sort string, order string) string {
	column, ok := siteStatsRankingSort[sort]
	if !ok {
		column = "requests"
	}
	direction := "DESC"
	if order == "asc" {
		direction = "ASC"
	}
	return column + " " + direction
}

// rankingLimit clamps the limit of a ranking.
func rankingLimit(limit int) int {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return limit
}

// GetSiteStatsSeries returns the buckets of the consume logs of the range.
func GetSiteStatsSeries(f SiteStatsFilter, divSeconds int64) ([]SiteStatsBucket, error) {
	expr := bucketExpr("created_at", f.TzOffsetSec, divSeconds)
	var rows []SiteStatsBucket
	tx := LOG_DB.Table("logs").
		Select(expr + " AS bucket, COUNT(*) AS requests, " +
			"COALESCE(SUM(prompt_tokens + completion_tokens), 0) AS tokens, " +
			"COALESCE(SUM(quota), 0) AS quota, " +
			"COUNT(DISTINCT user_id) AS unique_users").
		Where("type = ?", LogTypeConsume)
	tx = f.applyLogsRequest(tx)
	err := tx.Group(expr).Order("bucket ASC").Scan(&rows).Error
	return rows, err
}

// GetSiteStatsDayUserPairs returns the traffic of every user of every day of
// the range, the handler merges the pairs to the week and the month buckets
// with the real count of the distinct users.
func GetSiteStatsDayUserPairs(f SiteStatsFilter) ([]SiteStatsDayUser, error) {
	expr := bucketExpr("created_at", f.TzOffsetSec, 86400)
	var rows []SiteStatsDayUser
	tx := LOG_DB.Table("logs").
		Select(expr + " AS bucket, user_id, COUNT(*) AS requests, " +
			"COALESCE(SUM(prompt_tokens + completion_tokens), 0) AS tokens, " +
			"COALESCE(SUM(quota), 0) AS quota").
		Where("type = ?", LogTypeConsume)
	tx = f.applyLogsRequest(tx)
	err := tx.Group(expr + ", user_id").Order("bucket ASC").Scan(&rows).Error
	return rows, err
}

// GetSiteStatsErrorSeries returns the buckets of the error logs of the range.
func GetSiteStatsErrorSeries(f SiteStatsFilter, divSeconds int64) ([]SiteStatsCount, error) {
	expr := bucketExpr("created_at", f.TzOffsetSec, divSeconds)
	var rows []SiteStatsCount
	tx := LOG_DB.Table("logs").
		Select(expr+" AS bucket, COUNT(*) AS count").
		Where("type = ?", LogTypeError)
	tx = f.applyLogsRange(tx)
	err := tx.Group(expr).Order("bucket ASC").Scan(&rows).Error
	return rows, err
}

// GetSiteStatsRegistrationSeries returns the buckets of the new users of the
// range.
func GetSiteStatsRegistrationSeries(f SiteStatsFilter, divSeconds int64) ([]SiteStatsCount, error) {
	expr := bucketExpr("created_at", f.TzOffsetSec, divSeconds)
	var rows []SiteStatsCount
	tx := DB.Table("users").Select(expr + " AS bucket, COUNT(*) AS count")
	if f.StartTs > 0 {
		tx = tx.Where("created_at >= ?", f.StartTs)
	}
	if f.EndTs > 0 {
		tx = tx.Where("created_at < ?", f.EndTs)
	}
	err := tx.Group(expr).Order("bucket ASC").Scan(&rows).Error
	return rows, err
}

// GetSiteStatsHourOfDay returns the requests of every hour of the local day,
// the key is the second of the day cut to the hour.
func GetSiteStatsHourOfDay(f SiteStatsFilter) ([]SiteStatsCount, error) {
	expr := hourOfDayExpr("created_at", f.TzOffsetSec)
	var rows []SiteStatsCount
	tx := LOG_DB.Table("logs").
		Select(expr+" AS bucket, COUNT(*) AS count").
		Where("type = ?", LogTypeConsume)
	tx = f.applyLogsRequest(tx)
	err := tx.Group(expr).Order("bucket ASC").Scan(&rows).Error
	return rows, err
}

// GetSiteStatsLogTypeCounts returns the count of every type of the logs of
// the range.
func GetSiteStatsLogTypeCounts(f SiteStatsFilter) ([]SiteStatsLogTypeCount, error) {
	var rows []SiteStatsLogTypeCount
	tx := LOG_DB.Table("logs").Select("type, COUNT(*) AS count")
	tx = f.applyLogsRange(tx)
	err := tx.Group("type").Order("type ASC").Scan(&rows).Error
	return rows, err
}

// GetSiteStatsSummary returns the aggregate of the consume logs of the range.
func GetSiteStatsSummary(f SiteStatsFilter) (*SiteStatsSummary, error) {
	var row SiteStatsSummary
	tx := LOG_DB.Table("logs").
		Select("COUNT(*) AS requests, "+
			"COALESCE(SUM(prompt_tokens + completion_tokens), 0) AS tokens, "+
			"COALESCE(SUM(quota), 0) AS quota, "+
			"COUNT(DISTINCT user_id) AS unique_users, "+
			"COALESCE(AVG(use_time), 0) AS avg_use_time, "+
			"COALESCE(SUM(CASE WHEN channel_id = 0 AND other LIKE ? THEN 1 ELSE 0 END), 0) AS byog_requests",
			byogLogLikePattern)
	tx = f.applyLogsRequest(tx.Where("type = ?", LogTypeConsume))
	err := tx.Scan(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// CountSiteStatsLogs counts the logs of one type inside the range, the model
// and the channel filters do not apply because these logs do not carry them.
func CountSiteStatsLogs(f SiteStatsFilter, logType int) (int64, error) {
	var count int64
	tx := LOG_DB.Table("logs").Where("type = ?", logType)
	tx = f.applyLogsRange(tx)
	err := tx.Count(&count).Error
	return count, err
}

// CountSiteStatsRegistrations counts the new users inside the range.
func CountSiteStatsRegistrations(f SiteStatsFilter) (int64, error) {
	var count int64
	tx := DB.Table("users")
	if f.StartTs > 0 {
		tx = tx.Where("created_at >= ?", f.StartTs)
	}
	if f.EndTs > 0 {
		tx = tx.Where("created_at < ?", f.EndTs)
	}
	err := tx.Count(&count).Error
	return count, err
}

// GetSiteStatsTopUsers returns the ranking of the users of the range.
func GetSiteStatsTopUsers(f SiteStatsFilter, sort string, order string, limit int) ([]SiteStatsTopUser, error) {
	var rows []SiteStatsTopUser
	tx := LOG_DB.Table("logs").
		Select("user_id, MAX(username) AS username, COUNT(*) AS requests, " +
			"COALESCE(SUM(prompt_tokens + completion_tokens), 0) AS tokens, " +
			"COALESCE(SUM(quota), 0) AS quota").
		Where("type = ?", LogTypeConsume)
	tx = f.applyLogsRequest(tx)
	err := tx.Group("user_id").
		Order(rankingOrder(sort, order)).
		Limit(rankingLimit(limit)).
		Scan(&rows).Error
	return rows, err
}

// GetSiteStatsTopModels returns the ranking of the models of the range.
func GetSiteStatsTopModels(f SiteStatsFilter, sort string, order string, limit int) ([]SiteStatsTopModel, error) {
	var rows []SiteStatsTopModel
	tx := LOG_DB.Table("logs").
		Select("model_name, COUNT(*) AS requests, " +
			"COALESCE(SUM(prompt_tokens + completion_tokens), 0) AS tokens, " +
			"COALESCE(SUM(quota), 0) AS quota").
		Where("type = ?", LogTypeConsume).
		Where("model_name <> ''")
	tx = f.applyLogsRequest(tx)
	err := tx.Group("model_name").
		Order(rankingOrder(sort, order)).
		Limit(rankingLimit(limit)).
		Scan(&rows).Error
	return rows, err
}

// GetSiteStatsTopChannels returns the ranking of the channels of the range.
func GetSiteStatsTopChannels(f SiteStatsFilter, sort string, order string, limit int) ([]SiteStatsTopChannel, error) {
	var rows []SiteStatsTopChannel
	tx := LOG_DB.Table("logs").
		Select("channel_id, COUNT(*) AS requests, " +
			"COALESCE(SUM(prompt_tokens + completion_tokens), 0) AS tokens, " +
			"COALESCE(SUM(quota), 0) AS quota").
		Where("type = ?", LogTypeConsume)
	tx = f.applyLogsRequest(tx)
	err := tx.Group("channel_id").
		Order(rankingOrder(sort, order)).
		Limit(rankingLimit(limit)).
		Scan(&rows).Error
	return rows, err
}

// GetSiteStatsTopTokens returns the ranking of the api keys of the range, the
// name of a key is the name of the client that the user gave it.
func GetSiteStatsTopTokens(f SiteStatsFilter, sort string, order string, limit int) ([]SiteStatsTopToken, error) {
	var rows []SiteStatsTopToken
	tx := LOG_DB.Table("logs").
		Select("token_name, COUNT(*) AS requests, " +
			"COALESCE(SUM(prompt_tokens + completion_tokens), 0) AS tokens, " +
			"COALESCE(SUM(quota), 0) AS quota").
		Where("type = ?", LogTypeConsume).
		Where("token_name <> ''")
	tx = f.applyLogsRequest(tx)
	err := tx.Group("token_name").
		Order(rankingOrder(sort, order)).
		Limit(rankingLimit(limit)).
		Scan(&rows).Error
	return rows, err
}

// GetSiteStatsTopGroups returns the ranking of the groups of the range.
func GetSiteStatsTopGroups(f SiteStatsFilter, sort string, order string, limit int) ([]SiteStatsTopGroup, error) {
	var rows []SiteStatsTopGroup
	tx := LOG_DB.Table("logs").
		Select(logGroupCol+" AS group_name, COUNT(*) AS requests, " +
			"COALESCE(SUM(prompt_tokens + completion_tokens), 0) AS tokens, " +
			"COALESCE(SUM(quota), 0) AS quota").
		Where("type = ?", LogTypeConsume).
		Where(logGroupCol + " <> ''")
	tx = f.applyLogsRequest(tx)
	err := tx.Group(logGroupCol).
		Order(rankingOrder(sort, order)).
		Limit(rankingLimit(limit)).
		Scan(&rows).Error
	return rows, err
}

// GetSiteStatsTopIps returns the ranking of the addresses of the range.
func GetSiteStatsTopIps(f SiteStatsFilter, limit int) ([]SiteStatsTopIp, error) {
	var rows []SiteStatsTopIp
	tx := LOG_DB.Table("logs").
		Select("ip, COUNT(*) AS requests, " +
			"COALESCE(SUM(prompt_tokens + completion_tokens), 0) AS tokens").
		Where("type = ?", LogTypeConsume).
		Where("ip <> ''")
	tx = f.applyLogsRequest(tx)
	err := tx.Group("ip").
		Order("requests DESC").
		Limit(rankingLimit(limit)).
		Scan(&rows).Error
	return rows, err
}

// GetSiteStatsChannelNames returns the names of the channels of the ids.
func GetSiteStatsChannelNames(ids []int) (map[int]string, error) {
	result := make(map[int]string, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	var rows []struct {
		Id   int    `gorm:"column:id"`
		Name string `gorm:"column:name"`
	}
	err := DB.Table("channels").Select("id, name").Where("id IN ?", ids).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.Id] = row.Name
	}
	return result, nil
}

// SiteStatsGlobalCounters carries the counters of the whole installation, they
// do not depend on the selected range.
type SiteStatsGlobalCounters struct {
	TotalUsers        int64 `json:"total_users"`
	ActiveUsers       int64 `json:"active_users"`
	TotalChannels     int64 `json:"total_channels"`
	ActiveChannels    int64 `json:"active_channels"`
	TotalApiTokens    int64 `json:"total_api_tokens"`
	ActiveApiTokens   int64 `json:"active_api_tokens"`
	TotalRedemptions  int64 `json:"total_redemptions"`
	UnusedRedemptions int64 `json:"unused_redemptions"`
	ByogNodesTotal    int64 `json:"byog_nodes_total"`
	ByogNodesOnline   int64 `json:"byog_nodes_online"`
}

// GetSiteStatsGlobalCounters returns the counters of the tables of the
// installation.
func GetSiteStatsGlobalCounters() (*SiteStatsGlobalCounters, error) {
	counters := &SiteStatsGlobalCounters{}
	if err := DB.Table("users").Count(&counters.TotalUsers).Error; err != nil {
		return nil, err
	}
	if err := DB.Table("users").Where("status = ?", 1).Count(&counters.ActiveUsers).Error; err != nil {
		return nil, err
	}
	if err := DB.Table("channels").Count(&counters.TotalChannels).Error; err != nil {
		return nil, err
	}
	if err := DB.Table("channels").Where("status = ?", 1).Count(&counters.ActiveChannels).Error; err != nil {
		return nil, err
	}
	if err := DB.Table("tokens").Count(&counters.TotalApiTokens).Error; err != nil {
		return nil, err
	}
	if err := DB.Table("tokens").Where("status = ?", 1).Count(&counters.ActiveApiTokens).Error; err != nil {
		return nil, err
	}
	if err := DB.Table("redemptions").Count(&counters.TotalRedemptions).Error; err != nil {
		return nil, err
	}
	if err := DB.Table("redemptions").Where("status = ?", 1).Count(&counters.UnusedRedemptions).Error; err != nil {
		return nil, err
	}
	if err := DB.Table("byog_nodes").Count(&counters.ByogNodesTotal).Error; err != nil {
		// The BYOG module can be switched off, the counters of the nodes stay
		// zero and the rest of the statistics is still answered.
		return counters, nil
	}
	return counters, nil
}

// CountSiteStatsByogOnlineNodes counts the nodes with a fresh heartbeat.
func CountSiteStatsByogOnlineNodes(sinceUnix int64) (int64, error) {
	var count int64
	err := DB.Table("byog_nodes").Where("last_heartbeat_at >= ?", sinceUnix).Count(&count).Error
	if err != nil {
		return 0, err
	}
	return count, nil
}

// GetSiteStatsFirstLogTimestamp returns the moment of the first log, the
// "whole history" range starts there.
func GetSiteStatsFirstLogTimestamp() (int64, error) {
	var row struct {
		First int64 `gorm:"column:first"`
	}
	err := LOG_DB.Table("logs").Select("COALESCE(MIN(created_at), 0) AS first").Scan(&row).Error
	if err != nil {
		return 0, err
	}
	return row.First, nil
}

// GetSiteStatsDayCounts returns the requests of every local day of the range,
// the handler derives the distribution of the weekdays from the days.
func GetSiteStatsDayCounts(f SiteStatsFilter) ([]SiteStatsCount, error) {
	expr := bucketExpr("created_at", f.TzOffsetSec, 86400)
	var rows []SiteStatsCount
	tx := LOG_DB.Table("logs").
		Select(expr+" AS bucket, COUNT(*) AS count").
		Where("type = ?", LogTypeConsume)
	tx = f.applyLogsRequest(tx)
	err := tx.Group(expr).Order("bucket ASC").Scan(&rows).Error
	return rows, err
}

// SiteStatsByogNodeRank is one line of the ranking of the BYOG nodes.
type SiteStatsByogNodeRank struct {
	NodeId               string  `gorm:"column:node_id" json:"node_id"`
	DeviceName           string  `gorm:"column:device_name" json:"device_name"`
	DeviceType           string  `gorm:"column:device_type" json:"device_type"`
	UserId               int     `gorm:"column:user_id" json:"user_id"`
	Country              string  `gorm:"column:country" json:"country"`
	City                 string  `gorm:"column:city" json:"city"`
	Status               string  `gorm:"column:status" json:"status"`
	TotalTokensProcessed int64   `gorm:"column:total_tokens_processed" json:"total_tokens_processed"`
	UptimeSeconds        int64   `gorm:"column:uptime_seconds" json:"uptime_seconds"`
	ReputationScore      int     `gorm:"column:reputation_score" json:"reputation_score"`
	TotalEarnedCredits   float64 `gorm:"column:total_earned_credits" json:"total_earned_credits"`
	LastHeartbeatAt      int64   `gorm:"column:last_heartbeat_at" json:"last_heartbeat_at"`
}

// GetSiteStatsTopByogNodes returns the nodes of the network ordered by the
// tokens that they processed for the whole life of the node.
func GetSiteStatsTopByogNodes(limit int) ([]SiteStatsByogNodeRank, error) {
	var rows []SiteStatsByogNodeRank
	err := DB.Table("byog_nodes").
		Select("node_id, device_name, device_type, user_id, country, city, status, " +
			"total_tokens_processed, uptime_seconds, reputation_score, total_earned_credits, last_heartbeat_at").
		Order("total_tokens_processed DESC").
		Limit(rankingLimit(limit)).
		Scan(&rows).Error
	return rows, err
}

// GetSiteStatsUsernames returns the usernames of the ids, the ranking of the
// nodes shows the name of the owner of the node.
func GetSiteStatsUsernames(ids []int) (map[int]string, error) {
	result := make(map[int]string, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	var rows []struct {
		Id       int    `gorm:"column:id"`
		Username string `gorm:"column:username"`
	}
	err := DB.Table("users").Select("id, username").Where("id IN ?", ids).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.Id] = row.Username
	}
	return result, nil
}
