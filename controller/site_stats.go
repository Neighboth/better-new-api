package controller

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// The site wide statistics of the administration.
//
// One endpoint answers with the summary of the whole installation, the time
// series of the requests with the selectable granularity, the distributions
// of the hour of the day and of the weekday and the rankings of the users,
// the models, the channels, the api keys, the groups, the addresses and the
// BYOG nodes. The range, the granularity, the timezone and the filters of the
// users come from the query, the frontend draws the charts and the tables
// from the answer.

// siteStatsSeriesPoint is one bucket of the answer.
type siteStatsSeriesPoint struct {
	Bucket           int64 `json:"bucket"`
	Requests         int64 `json:"requests"`
	Tokens           int64 `json:"tokens"`
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	Quota            int64 `json:"quota"`
	UniqueUsers      int64 `json:"unique_users"`
	ByogRequests     int64 `json:"byog_requests"`
	Errors           int64 `json:"errors"`
	Registrations    int64 `json:"registrations"`
}

// siteStatsRankings carries every ranking of the answer.
type siteStatsRankings struct {
	Users     []model.SiteStatsTopUser         `json:"users"`
	Models    []model.SiteStatsTopModel        `json:"models"`
	Channels  []siteStatsChannelRank           `json:"channels"`
	Tokens    []model.SiteStatsTopToken        `json:"clients"`
	Groups    []model.SiteStatsTopGroup        `json:"groups"`
	Ips       []model.SiteStatsTopIp           `json:"ips"`
	ByogNodes []siteStatsByogNodeRank          `json:"byog_nodes"`
	LogTypes  []model.SiteStatsLogTypeCount    `json:"log_types"`
}

// siteStatsChannelRank is a channel line with the name of the channel.
type siteStatsChannelRank struct {
	ChannelId   int    `json:"channel_id"`
	ChannelName string `json:"channel_name"`
	Requests    int64  `json:"requests"`
	Tokens      int64  `json:"tokens"`
	Quota       int64  `json:"quota"`
}

// siteStatsByogNodeRank is a node line with the name of the owner.
type siteStatsByogNodeRank struct {
	model.SiteStatsByogNodeRank
	Username string `json:"username"`
}

// siteStatsDistributions carries the distributions of the answer.
type siteStatsDistributions struct {
	HourOfDay []int64 `json:"hour_of_day"`
	Weekday   []int64 `json:"weekday"`
	// The distributions of the BYOG network are global, the nodes are long
	// lived and the log range does not describe them.
	ByogDeviceTypes []model.SiteStatsNameCount `json:"byog_device_types"`
	ByogCountries   []model.SiteStatsNameCount `json:"byog_countries"`
}

// siteStatsSummary is the aggregate part of the answer.
type siteStatsSummary struct {
	Requests         int64   `json:"requests"`
	Tokens           int64   `json:"tokens"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	Quota            int64   `json:"quota"`
	UniqueUsers      int64   `json:"unique_users"`
	AvgUseTimeSecond float64 `json:"avg_use_time_seconds"`
	ByogRequests     int64   `json:"byog_requests"`
	UpstreamRequests int64   `json:"upstream_requests"`
	Errors           int64   `json:"errors"`
	Logins           int64   `json:"logins"`
	Registrations    int64   `json:"registrations"`
}

// siteStatsRangeInfo describes the resolved range of the answer.
type siteStatsRangeInfo struct {
	Start       int64  `json:"start"`
	End         int64  `json:"end"`
	Granularity string `json:"granularity"`
	BucketSize  int64  `json:"bucket_size"`
	TzOffsetMin int    `json:"tz_offset"`
	Preset      string `json:"preset"`
}

// parseSiteStatsIdList reads a comma separated list of ids.
func parseSiteStatsIdList(raw string) []int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	ids := make([]int, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.Atoi(part)
		if err != nil || id <= 0 {
			continue
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil
	}
	return ids
}

// resolveSiteStatsRange turns the preset or the explicit timestamps into the
// moments of the range, the timezone offset is the offset of the browser in
// minutes (the value of getTimezoneOffset of JavaScript).
func resolveSiteStatsRange(preset string, explicitStart int64, explicitEnd int64, tzOffsetMin int) (int64, int64) {
	tzSec := int64(-tzOffsetMin) * 60
	now := time.Now().Unix()
	if explicitStart > 0 && explicitEnd > explicitStart {
		return explicitStart, explicitEnd
	}
	switch preset {
	case "today":
		localNow := now + tzSec
		localMidnight := localNow - (localNow % 86400)
		return localMidnight - tzSec, now
	case "24h":
		return now - 86400, now
	case "7d":
		return now - 7*86400, now
	case "90d":
		return now - 90*86400, now
	case "180d":
		return now - 180*86400, now
	case "365d":
		return now - 365*86400, now
	case "all":
		first, err := model.GetSiteStatsFirstLogTimestamp()
		if err != nil || first <= 0 {
			first = now - 30*86400
		}
		return first, now
	default: // 30d
		return now - 30*86400, now
	}
}

// resolveSiteStatsGranularity picks the granularity of the series, an
// explicit choice is kept and only the hour granularity of a long range is
// reduced so that the answer stays small.
func resolveSiteStatsGranularity(explicit string, preset string, start int64, end int64) (string, int64) {
	span := end - start
	granularity := strings.ToLower(strings.TrimSpace(explicit))
	switch granularity {
	case "hour", "day", "week", "month":
	default:
		granularity = ""
	}
	if granularity == "" {
		switch preset {
		case "today", "24h":
			granularity = "hour"
		case "7d", "30d", "90d":
			granularity = "day"
		case "180d", "365d":
			granularity = "week"
		default:
			if span <= 2*86400 {
				granularity = "hour"
			} else if span <= 92*86400 {
				granularity = "day"
			} else if span <= 732*86400 {
				granularity = "week"
			} else {
				granularity = "month"
			}
		}
	}
	if granularity == "hour" && span > 62*86400 {
		granularity = "day"
	}
	switch granularity {
	case "hour":
		return granularity, 3600
	case "week":
		return granularity, 604800
	case "month":
		return granularity, 2592000
	default:
		return "day", 86400
	}
}

// weekStartLocal returns the local moment of the monday of the local day.
func weekStartLocal(localDayStart int64) int64 {
	days := localDayStart / 86400
	dayOfWeek := (days + 4) % 7 // 1970-01-01 was a thursday, 0 is the sunday
	daysSinceMonday := (dayOfWeek + 6) % 7
	return localDayStart - daysSinceMonday*86400
}

// monthStartLocal returns the local moment of the first day of the month.
func monthStartLocal(localDayStart int64) int64 {
	day := time.Unix(localDayStart, 0).UTC()
	first := time.Date(day.Year(), day.Month(), 1, 0, 0, 0, 0, time.UTC)
	return first.Unix()
}

// buildSiteStatsWeekdayIndex returns the index of the weekday of a local day,
// 0 is the sunday.
func buildSiteStatsWeekdayIndex(localDayStart int64) int {
	days := localDayStart / 86400
	return int((days + 4) % 7)
}

// GetSiteStatistics answers with the whole statistics of the site.
func GetSiteStatistics(c *gin.Context) {
	preset := strings.ToLower(strings.TrimSpace(c.DefaultQuery("range", "30d")))
	explicitStart, _ := strconv.ParseInt(c.Query("start"), 10, 64)
	explicitEnd, _ := strconv.ParseInt(c.Query("end"), 10, 64)
	tzOffsetMin, _ := strconv.Atoi(c.Query("tz_offset"))
	if tzOffsetMin < -840 || tzOffsetMin > 840 {
		tzOffsetMin = 0
	}
	tzSec := int64(-tzOffsetMin) * 60

	start, end := resolveSiteStatsRange(preset, explicitStart, explicitEnd, tzOffsetMin)
	if end <= start {
		common.ApiErrorMsg(c, "invalid time range")
		return
	}
	granularity, bucketSize := resolveSiteStatsGranularity(c.Query("granularity"), preset, start, end)

	filter := model.SiteStatsFilter{
		StartTs:        start,
		EndTs:          end,
		TzOffsetSec:    tzSec,
		UserIds:        parseSiteStatsIdList(c.Query("user_ids")),
		ExcludeUserIds: parseSiteStatsIdList(c.Query("exclude_user_ids")),
		ModelName:      strings.TrimSpace(c.Query("model")),
		Group:          strings.TrimSpace(c.Query("group")),
	}
	if channelId, err := strconv.Atoi(c.Query("channel_id")); err == nil && channelId > 0 {
		filter.ChannelId = channelId
	}

	sortKey := strings.ToLower(strings.TrimSpace(c.DefaultQuery("sort", "requests")))
	order := strings.ToLower(strings.TrimSpace(c.DefaultQuery("order", "desc")))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	// The series of the requests. The hour and the day granularity come from
	// one grouped query, the week and the month granularity come from the
	// pairs of the days and the users so that the count of the distinct users
	// of a long bucket is the real count and not the sum of the days.
	seriesMap := make(map[int64]*siteStatsSeriesPoint)
	orderKeys := make([]int64, 0, 64)
	addPoint := func(bucket int64) *siteStatsSeriesPoint {
		point, ok := seriesMap[bucket]
		if !ok {
			point = &siteStatsSeriesPoint{Bucket: bucket}
			seriesMap[bucket] = point
			orderKeys = append(orderKeys, bucket)
		}
		return point
	}

	if granularity == "hour" || granularity == "day" {
		rows, err := model.GetSiteStatsSeries(filter, bucketSize)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		for _, row := range rows {
			point := addPoint(row.Bucket)
			point.Requests = row.Requests
			point.Tokens = row.Tokens
			point.PromptTokens = row.PromptTokens
			point.CompletionTokens = row.CompletionTokens
			point.Quota = row.Quota
			point.UniqueUsers = row.UniqueUsers
			point.ByogRequests = row.ByogRequests
		}
	} else {
		pairs, err := model.GetSiteStatsDayUserPairs(filter)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		userSets := make(map[int64]map[int]bool)
		for _, pair := range pairs {
			localDay := pair.Bucket + tzSec
			var bucket int64
			if granularity == "week" {
				bucket = weekStartLocal(localDay) - tzSec
			} else {
				bucket = monthStartLocal(localDay) - tzSec
			}
			point := addPoint(bucket)
			point.Requests += pair.Requests
			point.Tokens += pair.Tokens
			point.PromptTokens += pair.PromptTokens
			point.CompletionTokens += pair.CompletionTokens
			point.Quota += pair.Quota
			point.ByogRequests += pair.ByogRequests
			set, ok := userSets[bucket]
			if !ok {
				set = make(map[int]bool)
				userSets[bucket] = set
			}
			set[pair.UserId] = true
		}
		for bucket, set := range userSets {
			if point, ok := seriesMap[bucket]; ok {
				point.UniqueUsers = int64(len(set))
			}
		}
	}

	// The errors and the registrations join the buckets of the requests. For
	// the week and the month granularity they arrive as days and are merged
	// into the long buckets.
	mergeCounts := func(rows []model.SiteStatsCount, apply func(point *siteStatsSeriesPoint, count int64)) {
		for _, row := range rows {
			bucket := row.Bucket
			if granularity == "week" || granularity == "month" {
				localDay := row.Bucket + tzSec
				if granularity == "week" {
					bucket = weekStartLocal(localDay) - tzSec
				} else {
					bucket = monthStartLocal(localDay) - tzSec
				}
			}
			apply(addPoint(bucket), row.Count)
		}
	}

	errorDiv := bucketSize
	if granularity == "week" || granularity == "month" {
		errorDiv = 86400
	}
	errorRows, err := model.GetSiteStatsErrorSeries(filter, errorDiv)
	if err == nil {
		mergeCounts(errorRows, func(point *siteStatsSeriesPoint, count int64) { point.Errors += count })
	}
	registrationRows, err := model.GetSiteStatsRegistrationSeries(filter, errorDiv)
	if err == nil {
		mergeCounts(registrationRows, func(point *siteStatsSeriesPoint, count int64) { point.Registrations += count })
	}

	// The axis of the chart is complete, a bucket without traffic is an
	// honest zero and not a hole in the line.
	axis := make([]int64, 0, len(orderKeys))
	if granularity == "hour" || granularity == "day" {
		localStart := start + tzSec
		firstBucket := localStart - (localStart % bucketSize) - tzSec
		for bucket := firstBucket; bucket < end; bucket += bucketSize {
			axis = append(axis, bucket)
			if _, ok := seriesMap[bucket]; !ok {
				seriesMap[bucket] = &siteStatsSeriesPoint{Bucket: bucket}
			}
			if len(axis) > 8808 {
				break
			}
		}
	} else {
		localStart := start + tzSec
		dayStart := localStart - (localStart % 86400)
		for localDay := dayStart; localDay-tzSec < end; localDay += 86400 {
			var bucket int64
			if granularity == "week" {
				bucket = weekStartLocal(localDay) - tzSec
			} else {
				bucket = monthStartLocal(localDay) - tzSec
			}
			if len(axis) == 0 || axis[len(axis)-1] != bucket {
				axis = append(axis, bucket)
				if _, ok := seriesMap[bucket]; !ok {
					seriesMap[bucket] = &siteStatsSeriesPoint{Bucket: bucket}
				}
			}
			if len(axis) > 5000 {
				break
			}
		}
	}
	series := make([]siteStatsSeriesPoint, 0, len(axis))
	for _, bucket := range axis {
		if point, ok := seriesMap[bucket]; ok {
			series = append(series, *point)
		}
	}

	// The distributions of the hour of the day and of the weekday.
	distributions := siteStatsDistributions{
		HourOfDay: make([]int64, 24),
		Weekday:   make([]int64, 7),
	}
	hourRows, err := model.GetSiteStatsHourOfDay(filter)
	if err == nil {
		for _, row := range hourRows {
			index := row.Bucket / 3600
			if index >= 0 && index < 24 {
				distributions.HourOfDay[index] = row.Count
			}
		}
	}
	dayRows, err := model.GetSiteStatsDayCounts(filter)
	if err == nil {
		for _, row := range dayRows {
			index := buildSiteStatsWeekdayIndex(row.Bucket + tzSec)
			if index >= 0 && index < 7 {
				distributions.Weekday[index] += row.Count
			}
		}
	}
	if deviceTypes, err := model.GetSiteStatsByogDeviceTypes(); err == nil {
		distributions.ByogDeviceTypes = deviceTypes
	}
	if countries, err := model.GetSiteStatsByogCountries(12); err == nil {
		distributions.ByogCountries = countries
	}

	// The rankings.
	rankings := siteStatsRankings{}
	if users, err := model.GetSiteStatsTopUsers(filter, sortKey, order, limit); err == nil {
		rankings.Users = users
	}
	if models_, err := model.GetSiteStatsTopModels(filter, sortKey, order, limit); err == nil {
		rankings.Models = models_
	}
	if channels, err := model.GetSiteStatsTopChannels(filter, sortKey, order, limit); err == nil {
		ids := make([]int, 0, len(channels))
		for _, row := range channels {
			if row.ChannelId > 0 {
				ids = append(ids, row.ChannelId)
			}
		}
		names, nameErr := model.GetSiteStatsChannelNames(ids)
		if nameErr != nil {
			names = map[int]string{}
		}
		rankings.Channels = make([]siteStatsChannelRank, 0, len(channels))
		for _, row := range channels {
			name := names[row.ChannelId]
			if row.ChannelId == 0 {
				// The requests of the BYOG nodes and of the local relay do
				// not belong to an upstream channel.
				name = "BYOG"
			} else if name == "" {
				name = "#" + strconv.Itoa(row.ChannelId)
			}
			rankings.Channels = append(rankings.Channels, siteStatsChannelRank{
				ChannelId:   row.ChannelId,
				ChannelName: name,
				Requests:    row.Requests,
				Tokens:      row.Tokens,
				Quota:       row.Quota,
			})
		}
	}
	if tokens, err := model.GetSiteStatsTopTokens(filter, sortKey, order, limit); err == nil {
		rankings.Tokens = tokens
	}
	if groups, err := model.GetSiteStatsTopGroups(filter, sortKey, order, limit); err == nil {
		rankings.Groups = groups
	}
	if ips, err := model.GetSiteStatsTopIps(filter, limit); err == nil {
		rankings.Ips = ips
	}
	if nodes, err := model.GetSiteStatsTopByogNodes(limit); err == nil {
		ids := make([]int, 0, len(nodes))
		for _, node := range nodes {
			if node.UserId > 0 {
				ids = append(ids, node.UserId)
			}
		}
		names, nameErr := model.GetSiteStatsUsernames(ids)
		if nameErr != nil {
			names = map[int]string{}
		}
		rankings.ByogNodes = make([]siteStatsByogNodeRank, 0, len(nodes))
		for _, node := range nodes {
			rankings.ByogNodes = append(rankings.ByogNodes, siteStatsByogNodeRank{
				SiteStatsByogNodeRank: node,
				Username:              names[node.UserId],
			})
		}
	}
	if logTypes, err := model.GetSiteStatsLogTypeCounts(filter); err == nil {
		rankings.LogTypes = logTypes
	}

	// The summary of the range and the counters of the whole installation.
	summary := siteStatsSummary{}
	if row, err := model.GetSiteStatsSummary(filter); err == nil && row != nil {
		summary.Requests = row.Requests
		summary.Tokens = row.Tokens
		summary.PromptTokens = row.PromptTokens
		summary.CompletionTokens = row.CompletionTokens
		summary.Quota = row.Quota
		summary.UniqueUsers = row.UniqueUsers
		summary.ByogRequests = row.ByogRequests
		summary.UpstreamRequests = row.Requests - row.ByogRequests
		// The average of an empty range is not a number on some engines, the
		// answer carries a clean zero instead of a NaN that breaks the JSON.
		if math.IsNaN(row.AvgUseTimeSecond) || math.IsInf(row.AvgUseTimeSecond, 0) {
			row.AvgUseTimeSecond = 0
		}
		summary.AvgUseTimeSecond = math.Round(row.AvgUseTimeSecond*100) / 100
	}
	if errors, err := model.CountSiteStatsLogs(filter, model.LogTypeError); err == nil {
		summary.Errors = errors
	}
	if logins, err := model.CountSiteStatsLogs(filter, model.LogTypeLogin); err == nil {
		summary.Logins = logins
	}
	if registrations, err := model.CountSiteStatsRegistrations(filter); err == nil {
		summary.Registrations = registrations
	}

	counters := model.SiteStatsGlobalCounters{}
	if row, err := model.GetSiteStatsGlobalCounters(); err == nil && row != nil {
		counters = *row
	}
	if online, err := model.CountSiteStatsByogOnlineNodes(time.Now().Unix() - 180); err == nil {
		counters.ByogNodesOnline = online
	}

	common.ApiSuccess(c, gin.H{
		"range": siteStatsRangeInfo{
			Start:       start,
			End:         end,
			Granularity: granularity,
			BucketSize:  bucketSize,
			TzOffsetMin: tzOffsetMin,
			Preset:      preset,
		},
		"summary":       summary,
		"counters":      counters,
		"series":        series,
		"distributions": distributions,
		"rankings":      rankings,
	})
}
