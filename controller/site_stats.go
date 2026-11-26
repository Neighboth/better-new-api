package controller

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type siteStatsCacheEntry struct {
	payload   gin.H
	expiresAt time.Time
}

var (
	siteStatsCacheMu sync.RWMutex
	siteStatsCache   = map[string]siteStatsCacheEntry{}
)

const siteStatsCacheTTL = 30 * time.Second

func getCachedSiteStats(key string) (gin.H, bool) {
	siteStatsCacheMu.RLock()
	entry, ok := siteStatsCache[key]
	siteStatsCacheMu.RUnlock()
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.payload, true
}

func setCachedSiteStats(key string, payload gin.H) {
	siteStatsCacheMu.Lock()
	if len(siteStatsCache) >= 64 {
		now := time.Now()
		for k, v := range siteStatsCache {
			if now.After(v.expiresAt) {
				delete(siteStatsCache, k)
			}
		}
		if len(siteStatsCache) >= 64 {
			siteStatsCache = map[string]siteStatsCacheEntry{}
		}
	}
	siteStatsCache[key] = siteStatsCacheEntry{
		payload:   payload,
		expiresAt: time.Now().Add(siteStatsCacheTTL),
	}
	siteStatsCacheMu.Unlock()
}

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
	Tokens    []model.SiteStatsTopToken        `json:"api_keys"`
	Clients   []model.SiteStatsTopClient       `json:"clients"`
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
	bypassCache := strings.EqualFold(strings.TrimSpace(c.Query("refresh")), "true")

	cacheKey := fmt.Sprintf("%s|%d|%d|%s|%d|%s|%s|%s|%d|%s|%s|%s|%d",
		preset, start, end, granularity, tzOffsetMin,
		c.Query("user_ids"), c.Query("exclude_user_ids"),
		filter.ModelName, filter.ChannelId, filter.Group,
		sortKey, order, limit,
	)
	if !bypassCache {
		if cached, ok := getCachedSiteStats(cacheKey); ok {
			common.ApiSuccess(c, cached)
			return
		}
	}

	// Execute all independent database queries concurrently with bounded
	// parallelism so the page loads in a fraction of sequential latency across
	// SQLite, MySQL, and PostgreSQL.
	var (
		seriesRows       []model.SiteStatsBucket
		seriesPairs      []model.SiteStatsDayUser
		seriesErr        error
		errorRows        []model.SiteStatsCount
		registrationRows []model.SiteStatsCount
		hourRows         []model.SiteStatsCount
		dayRows          []model.SiteStatsCount
		deviceTypes      []model.SiteStatsNameCount
		countries        []model.SiteStatsNameCount
		topUsers         []model.SiteStatsTopUser
		topModels        []model.SiteStatsTopModel
		topChannels      []model.SiteStatsTopChannel
		topClients       []model.SiteStatsTopClient
		topTokens        []model.SiteStatsTopToken
		topGroups        []model.SiteStatsTopGroup
		topIps           []model.SiteStatsTopIp
		topByogNodes     []model.SiteStatsByogNodeRank
		logTypes         []model.SiteStatsLogTypeCount
		summaryRow       *model.SiteStatsSummary
		globalCounters   *model.SiteStatsGlobalCounters
		byogOnlineCount  int64
	)

	errorDiv := bucketSize
	if granularity == "week" || granularity == "month" {
		errorDiv = 86400
	}

	sem := make(chan struct{}, 6)
	var wg sync.WaitGroup
	run := func(fn func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			fn()
		}()
	}

	run(func() {
		if granularity == "hour" || granularity == "day" {
			seriesRows, seriesErr = model.GetSiteStatsSeries(filter, bucketSize)
		} else {
			seriesPairs, seriesErr = model.GetSiteStatsDayUserPairs(filter)
		}
	})
	run(func() { errorRows, _ = model.GetSiteStatsErrorSeries(filter, errorDiv) })
	run(func() { registrationRows, _ = model.GetSiteStatsRegistrationSeries(filter, errorDiv) })
	run(func() { hourRows, _ = model.GetSiteStatsHourOfDay(filter) })
	if granularity != "day" {
		// When granularity == "day", weekday distribution is derived directly from seriesRows.
		run(func() { dayRows, _ = model.GetSiteStatsDayCounts(filter) })
	}
	run(func() { deviceTypes, _ = model.GetSiteStatsByogDeviceTypes() })
	run(func() { countries, _ = model.GetSiteStatsByogCountries(12) })
	run(func() { topUsers, _ = model.GetSiteStatsTopUsers(filter, sortKey, order, limit) })
	run(func() { topModels, _ = model.GetSiteStatsTopModels(filter, sortKey, order, limit) })
	run(func() { topChannels, _ = model.GetSiteStatsTopChannels(filter, sortKey, order, limit) })
	run(func() { topClients, _ = model.GetSiteStatsTopClients(filter, sortKey, order, limit) })
	run(func() { topTokens, _ = model.GetSiteStatsTopTokens(filter, sortKey, order, limit) })
	run(func() { topGroups, _ = model.GetSiteStatsTopGroups(filter, sortKey, order, limit) })
	run(func() { topIps, _ = model.GetSiteStatsTopIps(filter, limit) })
	run(func() { topByogNodes, _ = model.GetSiteStatsTopByogNodes(limit) })
	run(func() { logTypes, _ = model.GetSiteStatsLogTypeCounts(filter) })
	run(func() { summaryRow, _ = model.GetSiteStatsSummary(filter) })
	run(func() { globalCounters, _ = model.GetSiteStatsGlobalCounters() })
	run(func() { byogOnlineCount, _ = model.CountSiteStatsByogOnlineNodes(time.Now().Unix() - 180) })

	wg.Wait()

	if seriesErr != nil {
		common.ApiError(c, seriesErr)
		return
	}

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
		for _, row := range seriesRows {
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
		userSets := make(map[int64]map[int]bool)
		for _, pair := range seriesPairs {
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

	mergeCounts(errorRows, func(point *siteStatsSeriesPoint, count int64) { point.Errors += count })
	var totalRegistrations int64
	mergeCounts(registrationRows, func(point *siteStatsSeriesPoint, count int64) {
		point.Registrations += count
		totalRegistrations += count
	})

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

	distributions := siteStatsDistributions{
		HourOfDay:       make([]int64, 24),
		Weekday:         make([]int64, 7),
		ByogDeviceTypes: deviceTypes,
		ByogCountries:   countries,
	}
	for _, row := range hourRows {
		index := row.Bucket / 3600
		if index >= 0 && index < 24 {
			distributions.HourOfDay[index] = row.Count
		}
	}
	if granularity == "day" {
		for _, row := range seriesRows {
			index := buildSiteStatsWeekdayIndex(row.Bucket + tzSec)
			if index >= 0 && index < 7 {
				distributions.Weekday[index] += row.Requests
			}
		}
	} else {
		for _, row := range dayRows {
			index := buildSiteStatsWeekdayIndex(row.Bucket + tzSec)
			if index >= 0 && index < 7 {
				distributions.Weekday[index] += row.Count
			}
		}
	}

	rankings := siteStatsRankings{
		Users:    topUsers,
		Models:   topModels,
		Clients:  topClients,
		Tokens:   topTokens,
		Groups:   topGroups,
		Ips:      topIps,
		LogTypes: logTypes,
	}
	if len(topChannels) > 0 {
		ids := make([]int, 0, len(topChannels))
		for _, row := range topChannels {
			if row.ChannelId > 0 {
				ids = append(ids, row.ChannelId)
			}
		}
		names, nameErr := model.GetSiteStatsChannelNames(ids)
		if nameErr != nil {
			names = map[int]string{}
		}
		rankings.Channels = make([]siteStatsChannelRank, 0, len(topChannels))
		for _, row := range topChannels {
			name := names[row.ChannelId]
			if row.ChannelId == 0 {
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
	if len(topByogNodes) > 0 {
		ids := make([]int, 0, len(topByogNodes))
		for _, node := range topByogNodes {
			if node.UserId > 0 {
				ids = append(ids, node.UserId)
			}
		}
		names, nameErr := model.GetSiteStatsUsernames(ids)
		if nameErr != nil {
			names = map[int]string{}
		}
		rankings.ByogNodes = make([]siteStatsByogNodeRank, 0, len(topByogNodes))
		for _, node := range topByogNodes {
			rankings.ByogNodes = append(rankings.ByogNodes, siteStatsByogNodeRank{
				SiteStatsByogNodeRank: node,
				Username:              names[node.UserId],
			})
		}
	}

	summary := siteStatsSummary{
		Registrations: totalRegistrations,
	}
	if summaryRow != nil {
		summary.Requests = summaryRow.Requests
		summary.Tokens = summaryRow.Tokens
		summary.PromptTokens = summaryRow.PromptTokens
		summary.CompletionTokens = summaryRow.CompletionTokens
		summary.Quota = summaryRow.Quota
		summary.UniqueUsers = summaryRow.UniqueUsers
		summary.ByogRequests = summaryRow.ByogRequests
		summary.UpstreamRequests = summaryRow.Requests - summaryRow.ByogRequests
		if math.IsNaN(summaryRow.AvgUseTimeSecond) || math.IsInf(summaryRow.AvgUseTimeSecond, 0) {
			summaryRow.AvgUseTimeSecond = 0
		}
		summary.AvgUseTimeSecond = math.Round(summaryRow.AvgUseTimeSecond*100) / 100
	}
	// Derive error & login totals directly from logTypes (avoids 2 extra full logs table scans)
	for _, lt := range logTypes {
		switch lt.Type {
		case model.LogTypeError:
			summary.Errors = lt.Count
		case model.LogTypeLogin:
			summary.Logins = lt.Count
		}
	}

	counters := model.SiteStatsGlobalCounters{}
	if globalCounters != nil {
		counters = *globalCounters
	}
	counters.ByogNodesOnline = byogOnlineCount

	payload := gin.H{
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
	}
	setCachedSiteStats(cacheKey, payload)
	common.ApiSuccess(c, payload)
}
