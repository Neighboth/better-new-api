package model

import (
	"database/sql"
	"sort"
	"strings"
)

// SiteStatsSinglePassResult holds every log-derived metric computed from a
// single sequential scan of the logs table (eliminating 11 redundant full-table
// scans and SQL temp-table GROUP BY sorts on SQLite, MySQL, and PostgreSQL).
type SiteStatsSinglePassResult struct {
	Series       []SiteStatsBucket
	DayUserPairs []SiteStatsDayUser
	ErrorRows    []SiteStatsCount
	HourRows     []SiteStatsCount
	DayRows      []SiteStatsCount
	LogTypes     []SiteStatsLogTypeCount
	Summary      SiteStatsSummary
	TopUsers     []SiteStatsTopUser
	TopModels    []SiteStatsTopModel
	TopChannels  []SiteStatsTopChannel
	TopTokens    []SiteStatsTopToken
	TopClients   []SiteStatsTopClient
	TopGroups    []SiteStatsTopGroup
	TopIps       []SiteStatsTopIp
}

func alignLocalBucket(ts, tzOffsetSec, divSeconds int64) int64 {
	if divSeconds <= 0 {
		divSeconds = 86400
	}
	shifted := ts + tzOffsetSec
	rem := shifted % divSeconds
	if rem < 0 {
		rem += divSeconds
	}
	return ts - rem
}

func alignLocalHourOfDay(ts, tzOffsetSec int64) int64 {
	shifted := ts + tzOffsetSec
	dayRem := shifted % 86400
	if dayRem < 0 {
		dayRem += 86400
	}
	return dayRem - (dayRem % 3600)
}

// AggregateSiteStatsSinglePass streams matching rows from logs once and
// computes summary, time series, error series, hour/day distributions, log
// type counts, and all rankings in memory.
func AggregateSiteStatsSinglePass(
	f SiteStatsFilter,
	granularity string,
	bucketSize int64,
	errorDiv int64,
	sortKey string,
	order string,
	limit int,
) (*SiteStatsSinglePassResult, error) {
	tx := LOG_DB.Table("logs").
		Select("created_at, type, user_id, COALESCE(username, ''), COALESCE(model_name, ''), "+
			"channel_id, COALESCE(token_name, ''), COALESCE(client_name, ''), "+
			"COALESCE("+logGroupCol+", ''), COALESCE(ip, ''), "+
			"prompt_tokens, completion_tokens, quota, use_time, "+
			"CASE WHEN channel_id = 0 AND other LIKE ? THEN 1 ELSE 0 END",
			byogLogLikePattern)
	tx = f.applyLogsRange(tx)

	rows, err := tx.Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	modelFilterLower := strings.ToLower(strings.TrimSpace(f.ModelName))
	groupFilter := f.Group
	channelFilter := f.ChannelId

	type bucketAgg struct {
		requests         int64
		tokens           int64
		promptTokens     int64
		completionTokens int64
		quota            int64
		byogRequests     int64
		users            map[int]struct{}
	}
	type dayUserKey struct {
		bucket int64
		userID int
	}
	type dayUserAgg struct {
		requests         int64
		tokens           int64
		promptTokens     int64
		completionTokens int64
		quota            int64
		byogRequests     int64
	}
	type rankAgg struct {
		requests int64
		tokens   int64
		quota    int64
		label    string
	}

	isDirectBucket := granularity == "hour" || granularity == "day"
	seriesMap := make(map[int64]*bucketAgg, 64)
	dayUserMap := make(map[dayUserKey]*dayUserAgg)
	errorMap := make(map[int64]int64, 64)
	hourCounts := make([]int64, 24)
	dayMap := make(map[int64]int64, 64)
	logTypeMap := make(map[int]int64, 8)
	summaryUsers := make(map[int]struct{}, 128)

	userMap := make(map[int]*rankAgg, 64)
	modelMap := make(map[string]*rankAgg, 64)
	channelMap := make(map[int]*rankAgg, 32)
	tokenMap := make(map[string]*rankAgg, 64)
	clientMap := make(map[string]*rankAgg, 32)
	groupMap := make(map[string]*rankAgg, 16)
	ipMap := make(map[string]*rankAgg, 64)

	var (
		summary         SiteStatsSummary
		totalUseTimeSum int64
	)

	var (
		createdAt        int64
		logType          int
		userID           int
		username         sql.NullString
		modelName        sql.NullString
		channelID        int
		tokenName        sql.NullString
		clientName       sql.NullString
		groupName        sql.NullString
		ip               sql.NullString
		promptTokens     int64
		completionTokens int64
		quota            int64
		useTime          int64
		isByog           int
	)

	for rows.Next() {
		if scanErr := rows.Scan(
			&createdAt, &logType, &userID, &username, &modelName,
			&channelID, &tokenName, &clientName, &groupName, &ip,
			&promptTokens, &completionTokens, &quota, &useTime, &isByog,
		); scanErr != nil {
			return nil, scanErr
		}

		logTypeMap[logType]++

		if logType == LogTypeError {
			eb := alignLocalBucket(createdAt, f.TzOffsetSec, errorDiv)
			errorMap[eb]++
			continue
		}
		if logType != LogTypeConsume {
			continue
		}

		// Request-level filters apply only to consume logs (exact match with SQL applyLogsRequest).
		mName := modelName.String
		if modelFilterLower != "" && !strings.Contains(strings.ToLower(mName), modelFilterLower) {
			continue
		}
		if channelFilter != 0 && channelID != channelFilter {
			continue
		}
		gName := groupName.String
		if groupFilter != "" && gName != groupFilter {
			continue
		}

		tokens := promptTokens + completionTokens
		byogInc := int64(0)
		if isByog != 0 {
			byogInc = 1
		}

		summary.Requests++
		summary.Tokens += tokens
		summary.PromptTokens += promptTokens
		summary.CompletionTokens += completionTokens
		summary.Quota += quota
		summary.ByogRequests += byogInc
		totalUseTimeSum += useTime
		summaryUsers[userID] = struct{}{}

		// Series buckets
		if isDirectBucket {
			b := alignLocalBucket(createdAt, f.TzOffsetSec, bucketSize)
			agg := seriesMap[b]
			if agg == nil {
				agg = &bucketAgg{users: make(map[int]struct{}, 8)}
				seriesMap[b] = agg
			}
			agg.requests++
			agg.tokens += tokens
			agg.promptTokens += promptTokens
			agg.completionTokens += completionTokens
			agg.quota += quota
			agg.byogRequests += byogInc
			agg.users[userID] = struct{}{}
		} else {
			dayBucket := alignLocalBucket(createdAt, f.TzOffsetSec, 86400)
			k := dayUserKey{bucket: dayBucket, userID: userID}
			agg := dayUserMap[k]
			if agg == nil {
				agg = &dayUserAgg{}
				dayUserMap[k] = agg
			}
			agg.requests++
			agg.tokens += tokens
			agg.promptTokens += promptTokens
			agg.completionTokens += completionTokens
			agg.quota += quota
			agg.byogRequests += byogInc
		}

		// Hour-of-day & weekday distributions
		hourIdx := alignLocalHourOfDay(createdAt, f.TzOffsetSec) / 3600
		if hourIdx >= 0 && hourIdx < 24 {
			hourCounts[hourIdx]++
		}
		if granularity != "day" {
			dayBucket := alignLocalBucket(createdAt, f.TzOffsetSec, 86400)
			dayMap[dayBucket]++
		}

		// Rankings
		uAgg := userMap[userID]
		if uAgg == nil {
			uAgg = &rankAgg{label: username.String}
			userMap[userID] = uAgg
		} else if username.String > uAgg.label {
			uAgg.label = username.String
		}
		uAgg.requests++
		uAgg.tokens += tokens
		uAgg.quota += quota

		if mName != "" {
			mAgg := modelMap[mName]
			if mAgg == nil {
				mAgg = &rankAgg{}
				modelMap[mName] = mAgg
			}
			mAgg.requests++
			mAgg.tokens += tokens
			mAgg.quota += quota
		}

		chAgg := channelMap[channelID]
		if chAgg == nil {
			chAgg = &rankAgg{}
			channelMap[channelID] = chAgg
		}
		chAgg.requests++
		chAgg.tokens += tokens
		chAgg.quota += quota

		tName := tokenName.String
		if tName != "" {
			tAgg := tokenMap[tName]
			if tAgg == nil {
				tAgg = &rankAgg{}
				tokenMap[tName] = tAgg
			}
			tAgg.requests++
			tAgg.tokens += tokens
			tAgg.quota += quota
		}

		cName := clientName.String
		cAgg := clientMap[cName]
		if cAgg == nil {
			cAgg = &rankAgg{}
			clientMap[cName] = cAgg
		}
		cAgg.requests++
		cAgg.tokens += tokens
		cAgg.quota += quota

		if gName != "" {
			gAgg := groupMap[gName]
			if gAgg == nil {
				gAgg = &rankAgg{}
				groupMap[gName] = gAgg
			}
			gAgg.requests++
			gAgg.tokens += tokens
			gAgg.quota += quota
		}

		ipStr := ip.String
		if ipStr != "" {
			ipAgg := ipMap[ipStr]
			if ipAgg == nil {
				ipAgg = &rankAgg{}
				ipMap[ipStr] = ipAgg
			}
			ipAgg.requests++
			ipAgg.tokens += tokens
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	summary.UniqueUsers = int64(len(summaryUsers))
	if summary.Requests > 0 {
		summary.AvgUseTimeSecond = float64(totalUseTimeSum) / float64(summary.Requests)
	}

	res := &SiteStatsSinglePassResult{
		Summary: summary,
	}

	// Build sorted series
	if isDirectBucket {
		res.Series = make([]SiteStatsBucket, 0, len(seriesMap))
		for b, agg := range seriesMap {
			res.Series = append(res.Series, SiteStatsBucket{
				Bucket:           b,
				Requests:         agg.requests,
				Tokens:           agg.tokens,
				PromptTokens:     agg.promptTokens,
				CompletionTokens: agg.completionTokens,
				Quota:            agg.quota,
				UniqueUsers:      int64(len(agg.users)),
				ByogRequests:     agg.byogRequests,
			})
		}
		sort.Slice(res.Series, func(i, j int) bool { return res.Series[i].Bucket < res.Series[j].Bucket })
	} else {
		res.DayUserPairs = make([]SiteStatsDayUser, 0, len(dayUserMap))
		for k, agg := range dayUserMap {
			res.DayUserPairs = append(res.DayUserPairs, SiteStatsDayUser{
				Bucket:           k.bucket,
				UserId:           k.userID,
				Requests:         agg.requests,
				Tokens:           agg.tokens,
				PromptTokens:     agg.promptTokens,
				CompletionTokens: agg.completionTokens,
				Quota:            agg.quota,
				ByogRequests:     agg.byogRequests,
			})
		}
		sort.Slice(res.DayUserPairs, func(i, j int) bool { return res.DayUserPairs[i].Bucket < res.DayUserPairs[j].Bucket })
	}

	res.ErrorRows = make([]SiteStatsCount, 0, len(errorMap))
	for b, c := range errorMap {
		res.ErrorRows = append(res.ErrorRows, SiteStatsCount{Bucket: b, Count: c})
	}
	sort.Slice(res.ErrorRows, func(i, j int) bool { return res.ErrorRows[i].Bucket < res.ErrorRows[j].Bucket })

	res.HourRows = make([]SiteStatsCount, 0, 24)
	for h, c := range hourCounts {
		if c > 0 {
			res.HourRows = append(res.HourRows, SiteStatsCount{Bucket: int64(h) * 3600, Count: c})
		}
	}

	res.DayRows = make([]SiteStatsCount, 0, len(dayMap))
	for b, c := range dayMap {
		res.DayRows = append(res.DayRows, SiteStatsCount{Bucket: b, Count: c})
	}
	sort.Slice(res.DayRows, func(i, j int) bool { return res.DayRows[i].Bucket < res.DayRows[j].Bucket })

	res.LogTypes = make([]SiteStatsLogTypeCount, 0, len(logTypeMap))
	for t, c := range logTypeMap {
		res.LogTypes = append(res.LogTypes, SiteStatsLogTypeCount{Type: t, Count: c})
	}
	sort.Slice(res.LogTypes, func(i, j int) bool { return res.LogTypes[i].Type < res.LogTypes[j].Type })

	capLim := rankingLimit(limit)
	asc := order == "asc"
	metricVal := func(req, tok, q int64) int64 {
		switch sortKey {
		case "tokens":
			return tok
		case "quota":
			return q
		default:
			return req
		}
	}
	cmpRank := func(reqA, tokA, qA, reqB, tokB, qB int64) bool {
		a, b := metricVal(reqA, tokA, qA), metricVal(reqB, tokB, qB)
		if asc {
			return a < b
		}
		return a > b
	}

	// Users
	res.TopUsers = make([]SiteStatsTopUser, 0, len(userMap))
	for uid, agg := range userMap {
		res.TopUsers = append(res.TopUsers, SiteStatsTopUser{
			UserId: uid, Username: agg.label, Requests: agg.requests, Tokens: agg.tokens, Quota: agg.quota,
		})
	}
	sort.Slice(res.TopUsers, func(i, j int) bool {
		return cmpRank(res.TopUsers[i].Requests, res.TopUsers[i].Tokens, res.TopUsers[i].Quota,
			res.TopUsers[j].Requests, res.TopUsers[j].Tokens, res.TopUsers[j].Quota)
	})
	if len(res.TopUsers) > capLim {
		res.TopUsers = res.TopUsers[:capLim]
	}

	// Models
	res.TopModels = make([]SiteStatsTopModel, 0, len(modelMap))
	for name, agg := range modelMap {
		res.TopModels = append(res.TopModels, SiteStatsTopModel{
			ModelName: name, Requests: agg.requests, Tokens: agg.tokens, Quota: agg.quota,
		})
	}
	sort.Slice(res.TopModels, func(i, j int) bool {
		return cmpRank(res.TopModels[i].Requests, res.TopModels[i].Tokens, res.TopModels[i].Quota,
			res.TopModels[j].Requests, res.TopModels[j].Tokens, res.TopModels[j].Quota)
	})
	if len(res.TopModels) > capLim {
		res.TopModels = res.TopModels[:capLim]
	}

	// Channels
	res.TopChannels = make([]SiteStatsTopChannel, 0, len(channelMap))
	for cid, agg := range channelMap {
		res.TopChannels = append(res.TopChannels, SiteStatsTopChannel{
			ChannelId: cid, Requests: agg.requests, Tokens: agg.tokens, Quota: agg.quota,
		})
	}
	sort.Slice(res.TopChannels, func(i, j int) bool {
		return cmpRank(res.TopChannels[i].Requests, res.TopChannels[i].Tokens, res.TopChannels[i].Quota,
			res.TopChannels[j].Requests, res.TopChannels[j].Tokens, res.TopChannels[j].Quota)
	})
	if len(res.TopChannels) > capLim {
		res.TopChannels = res.TopChannels[:capLim]
	}

	// Tokens
	res.TopTokens = make([]SiteStatsTopToken, 0, len(tokenMap))
	for name, agg := range tokenMap {
		res.TopTokens = append(res.TopTokens, SiteStatsTopToken{
			TokenName: name, Requests: agg.requests, Tokens: agg.tokens, Quota: agg.quota,
		})
	}
	sort.Slice(res.TopTokens, func(i, j int) bool {
		return cmpRank(res.TopTokens[i].Requests, res.TopTokens[i].Tokens, res.TopTokens[i].Quota,
			res.TopTokens[j].Requests, res.TopTokens[j].Tokens, res.TopTokens[j].Quota)
	})
	if len(res.TopTokens) > capLim {
		res.TopTokens = res.TopTokens[:capLim]
	}

	// Clients
	res.TopClients = make([]SiteStatsTopClient, 0, len(clientMap))
	for name, agg := range clientMap {
		res.TopClients = append(res.TopClients, SiteStatsTopClient{
			ClientName: name, Requests: agg.requests, Tokens: agg.tokens, Quota: agg.quota,
		})
	}
	sort.Slice(res.TopClients, func(i, j int) bool {
		return cmpRank(res.TopClients[i].Requests, res.TopClients[i].Tokens, res.TopClients[i].Quota,
			res.TopClients[j].Requests, res.TopClients[j].Tokens, res.TopClients[j].Quota)
	})
	if len(res.TopClients) > capLim {
		res.TopClients = res.TopClients[:capLim]
	}

	// Groups
	res.TopGroups = make([]SiteStatsTopGroup, 0, len(groupMap))
	for name, agg := range groupMap {
		res.TopGroups = append(res.TopGroups, SiteStatsTopGroup{
			GroupName: name, Requests: agg.requests, Tokens: agg.tokens, Quota: agg.quota,
		})
	}
	sort.Slice(res.TopGroups, func(i, j int) bool {
		return cmpRank(res.TopGroups[i].Requests, res.TopGroups[i].Tokens, res.TopGroups[i].Quota,
			res.TopGroups[j].Requests, res.TopGroups[j].Tokens, res.TopGroups[j].Quota)
	})
	if len(res.TopGroups) > capLim {
		res.TopGroups = res.TopGroups[:capLim]
	}

	// IPs (always ordered by requests DESC matching GetSiteStatsTopIps)
	res.TopIps = make([]SiteStatsTopIp, 0, len(ipMap))
	for addr, agg := range ipMap {
		res.TopIps = append(res.TopIps, SiteStatsTopIp{
			Ip: addr, Requests: agg.requests, Tokens: agg.tokens,
		})
	}
	sort.Slice(res.TopIps, func(i, j int) bool { return res.TopIps[i].Requests > res.TopIps[j].Requests })
	if len(res.TopIps) > capLim {
		res.TopIps = res.TopIps[:capLim]
	}

	return res, nil
}
