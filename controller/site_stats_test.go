package controller

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// siteStatsEnvelope mirrors the answer of the site statistics endpoint, the
// tests assert on the decoded fields instead of the raw JSON text.
type siteStatsEnvelope struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		Range struct {
			Start       int64  `json:"start"`
			End         int64  `json:"end"`
			Granularity string `json:"granularity"`
			BucketSize  int64  `json:"bucket_size"`
			TzOffset    int    `json:"tz_offset"`
			Preset      string `json:"preset"`
		} `json:"range"`
		Summary struct {
			Requests         int64   `json:"requests"`
			Tokens           int64   `json:"tokens"`
			PromptTokens     int64   `json:"prompt_tokens"`
			CompletionTokens int64   `json:"completion_tokens"`
			Quota            int64   `json:"quota"`
			UniqueUsers      int64   `json:"unique_users"`
			AvgUseTime       float64 `json:"avg_use_time_seconds"`
			ByogRequests     int64   `json:"byog_requests"`
			UpstreamRequests int64   `json:"upstream_requests"`
			Errors           int64   `json:"errors"`
			Logins           int64   `json:"logins"`
			Registrations    int64   `json:"registrations"`
		} `json:"summary"`
		Counters struct {
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
		} `json:"counters"`
		Series []struct {
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
		} `json:"series"`
		Distributions struct {
			HourOfDay       []int64 `json:"hour_of_day"`
			Weekday         []int64 `json:"weekday"`
			ByogDeviceTypes []struct {
				Name  string `json:"name"`
				Count int64  `json:"count"`
			} `json:"byog_device_types"`
			ByogCountries []struct {
				Name  string `json:"name"`
				Count int64  `json:"count"`
			} `json:"byog_countries"`
		} `json:"distributions"`
		Rankings struct {
			Users []struct {
				UserId   int    `json:"user_id"`
				Username string `json:"username"`
				Requests int64  `json:"requests"`
				Tokens   int64  `json:"tokens"`
				Quota    int64  `json:"quota"`
			} `json:"users"`
			Models []struct {
				ModelName string `json:"model_name"`
				Requests  int64  `json:"requests"`
			} `json:"models"`
			Channels []struct {
				ChannelId   int    `json:"channel_id"`
				ChannelName string `json:"channel_name"`
				Requests    int64  `json:"requests"`
			} `json:"channels"`
			Clients []struct {
				TokenName string `json:"token_name"`
				Requests  int64  `json:"requests"`
			} `json:"api_keys"`
			Ips []struct {
				Ip       string `json:"ip"`
				Requests int64  `json:"requests"`
			} `json:"ips"`
			ByogNodes []struct {
				NodeId     string `json:"node_id"`
				DeviceName string `json:"device_name"`
				Username   string `json:"username"`
			} `json:"byog_nodes"`
			LogTypes []struct {
				Type  int   `json:"type"`
				Count int64 `json:"count"`
			} `json:"log_types"`
		} `json:"rankings"`
	} `json:"data"`
}

// siteStatsFixture is the seeded world of the statistics tests.
type siteStatsFixture struct {
	AliceId int
	BobId   int
	Now     int64
}

// setupSiteStatsTestDB builds an isolated SQLite database with a small but
// complete world: two users, one upstream channel, one api token, one
// redemption, two BYOG nodes and the logs of a day.
func setupSiteStatsTestDB(t *testing.T) *siteStatsFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)
	common.RedisEnabled = false

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "site-stats.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	originalDB := model.DB
	model.DB = db
	model.LOG_DB = db
	t.Cleanup(func() {
		model.DB = originalDB
		model.LOG_DB = originalDB
		_ = sqlDB.Close()
	})

	require.NoError(t, db.AutoMigrate(
		&model.Option{},
		&model.User{},
		&model.Channel{},
		&model.Token{},
		&model.Redemption{},
		&model.ByogNode{},
		&model.ByogCrashReport{},
		&model.Log{},
	))

	now := time.Now().Unix()
	users := []model.User{{Username: "alice", AffCode: "aff-alice", Status: 1}, {Username: "bob", AffCode: "aff-bob", Status: 1}}
	require.NoError(t, db.Create(&users).Error)

	require.NoError(t, db.Create(&model.Channel{Id: 5, Name: "upstream-one", Status: 1, Type: 1, Key: "sk-test"}).Error)
	require.NoError(t, db.Create(&model.Token{UserId: users[0].Id, Key: "tk-alice", Name: "alice-key", Status: 1}).Error)
	require.NoError(t, db.Create(&model.Redemption{Key: "0000000000000000000000000000aa01", Name: "welcome", Status: common.RedemptionCodeStatusEnabled}).Error)

	require.NoError(t, db.Create(&[]model.ByogNode{
		{NodeId: "s-n1", UserId: users[0].Id, HWID: "hw-s1", DeviceName: "alice-desktop", DeviceType: "desktop", Country: "Turkey", Status: "online", TotalTokensProcessed: 1000, LastHeartbeatAt: now},
		{NodeId: "s-n2", UserId: users[1].Id, HWID: "hw-s2", DeviceName: "bob-phone", DeviceType: "mobile", Country: "Germany", Status: "offline", LastHeartbeatAt: now - 7200},
	}).Error)

	logs := []model.Log{
		{
			UserId: users[0].Id, Username: "alice", CreatedAt: now - 3600,
			Type: model.LogTypeConsume, ModelName: "gpt-4o", TokenName: "alice-key",
			ChannelId: 5, Group: "default", Ip: "10.0.0.1",
			Quota: 700, PromptTokens: 100, CompletionTokens: 40, UseTime: 1200,
		},
		{
			UserId: users[1].Id, Username: "bob", CreatedAt: now - 1800,
			Type: model.LogTypeConsume, ModelName: "byog-qwen", TokenName: "bob-key",
			ChannelId: 0, Group: "default", Ip: "10.0.0.2",
			Quota: 300, PromptTokens: 50, CompletionTokens: 10, UseTime: 800,
			Other: `{"byog":true,"admin_info":{}}`,
		},
		{UserId: users[0].Id, Username: "alice", CreatedAt: now - 900, Type: model.LogTypeError, Content: "upstream timeout"},
		{UserId: users[1].Id, Username: "bob", CreatedAt: now - 600, Type: model.LogTypeLogin, Content: "login success"},
	}
	require.NoError(t, db.Create(&logs).Error)

	return &siteStatsFixture{AliceId: users[0].Id, BobId: users[1].Id, Now: now}
}

// callSiteStats runs the handler with the query and decodes the answer.
func callSiteStats(t *testing.T, query string) *siteStatsEnvelope {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("GET", "/api/statistics/site?"+query, nil)
	GetSiteStatistics(c)
	require.Equal(t, 200, recorder.Code)

	envelope := &siteStatsEnvelope{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), envelope))
	return envelope
}

func TestGetSiteStatisticsFullAnswer(t *testing.T) {
	setupSiteStatsTestDB(t)

	env := callSiteStats(t, "range=7d&tz_offset=-180")
	require.True(t, env.Success, env.Message)

	// The resolved range echoes the request.
	assert.Equal(t, "7d", env.Data.Range.Preset)
	assert.Equal(t, "day", env.Data.Range.Granularity)
	assert.Equal(t, int64(86400), env.Data.Range.BucketSize)
	assert.Equal(t, -180, env.Data.Range.TzOffset)
	assert.Greater(t, env.Data.Range.End, env.Data.Range.Start)

	// The summary carries the token split and the BYOG share.
	assert.Equal(t, int64(2), env.Data.Summary.Requests)
	assert.Equal(t, int64(200), env.Data.Summary.Tokens)
	assert.Equal(t, int64(150), env.Data.Summary.PromptTokens)
	assert.Equal(t, int64(50), env.Data.Summary.CompletionTokens)
	assert.Equal(t, int64(1000), env.Data.Summary.Quota)
	assert.Equal(t, int64(2), env.Data.Summary.UniqueUsers)
	assert.Equal(t, int64(1), env.Data.Summary.ByogRequests)
	assert.Equal(t, int64(1), env.Data.Summary.UpstreamRequests)
	assert.Equal(t, int64(1), env.Data.Summary.Errors)
	assert.Equal(t, int64(1), env.Data.Summary.Logins)
	assert.InDelta(t, 1000.0, env.Data.Summary.AvgUseTime, 0.01)

	// The counters see the whole installation.
	assert.Equal(t, int64(2), env.Data.Counters.TotalUsers)
	assert.Equal(t, int64(2), env.Data.Counters.ActiveUsers)
	assert.Equal(t, int64(1), env.Data.Counters.TotalChannels)
	assert.Equal(t, int64(1), env.Data.Counters.ActiveChannels)
	assert.Equal(t, int64(1), env.Data.Counters.TotalApiTokens)
	assert.Equal(t, int64(1), env.Data.Counters.TotalRedemptions)
	assert.Equal(t, int64(1), env.Data.Counters.UnusedRedemptions)
	assert.Equal(t, int64(2), env.Data.Counters.ByogNodesTotal)
	assert.Equal(t, int64(1), env.Data.Counters.ByogNodesOnline)

	// The series is complete over the axis, the totals are stable even when
	// the UTC midnight splits the two logs into two buckets.
	assert.GreaterOrEqual(t, len(env.Data.Series), 7)
	var requests, prompt, completion, byog, errors int64
	for _, point := range env.Data.Series {
		requests += point.Requests
		prompt += point.PromptTokens
		completion += point.CompletionTokens
		byog += point.ByogRequests
		errors += point.Errors
	}
	assert.Equal(t, int64(2), requests)
	assert.Equal(t, int64(150), prompt)
	assert.Equal(t, int64(50), completion)
	assert.Equal(t, int64(1), byog)
	assert.Equal(t, int64(1), errors)

	// The distributions are complete arrays plus the BYOG network.
	require.Len(t, env.Data.Distributions.HourOfDay, 24)
	var hourTotal int64
	for _, count := range env.Data.Distributions.HourOfDay {
		hourTotal += count
	}
	assert.Equal(t, int64(2), hourTotal)
	require.Len(t, env.Data.Distributions.Weekday, 7)
	var weekdayTotal int64
	for _, count := range env.Data.Distributions.Weekday {
		weekdayTotal += count
	}
	assert.Equal(t, int64(2), weekdayTotal)
	require.Len(t, env.Data.Distributions.ByogDeviceTypes, 2)
	deviceCounts := map[string]int64{}
	for _, row := range env.Data.Distributions.ByogDeviceTypes {
		deviceCounts[row.Name] = row.Count
	}
	assert.Equal(t, int64(1), deviceCounts["desktop"])
	assert.Equal(t, int64(1), deviceCounts["mobile"])
	require.Len(t, env.Data.Distributions.ByogCountries, 2)

	// The rankings resolve the names.
	require.Len(t, env.Data.Rankings.Users, 2)
	assert.Equal(t, int64(1), env.Data.Rankings.Users[0].Requests)
	require.Len(t, env.Data.Rankings.Models, 2)
	modelNames := map[string]bool{}
	for _, row := range env.Data.Rankings.Models {
		modelNames[row.ModelName] = true
	}
	assert.True(t, modelNames["gpt-4o"])
	assert.True(t, modelNames["byog-qwen"])
	require.Len(t, env.Data.Rankings.Channels, 2)
	byChannel := map[int]string{}
	for _, row := range env.Data.Rankings.Channels {
		byChannel[row.ChannelId] = row.ChannelName
	}
	assert.Equal(t, "upstream-one", byChannel[5])
	assert.Equal(t, "BYOG", byChannel[0])
	require.Len(t, env.Data.Rankings.Clients, 2)
	require.Len(t, env.Data.Rankings.Ips, 2)
	require.Len(t, env.Data.Rankings.ByogNodes, 2)
	assert.Equal(t, "s-n1", env.Data.Rankings.ByogNodes[0].NodeId)
	assert.Equal(t, "alice", env.Data.Rankings.ByogNodes[0].Username)
	require.Len(t, env.Data.Rankings.LogTypes, 3)
	assert.Equal(t, model.LogTypeConsume, env.Data.Rankings.LogTypes[0].Type)
	assert.Equal(t, int64(2), env.Data.Rankings.LogTypes[0].Count)
}

func TestGetSiteStatisticsFilters(t *testing.T) {
	fixture := setupSiteStatsTestDB(t)

	// The include filter of the users.
	env := callSiteStats(t, "range=7d&user_ids="+strconv.Itoa(fixture.AliceId))
	require.True(t, env.Success, env.Message)
	assert.Equal(t, int64(1), env.Data.Summary.Requests)
	assert.Equal(t, int64(0), env.Data.Summary.ByogRequests)
	assert.Equal(t, int64(1), env.Data.Summary.UpstreamRequests)
	assert.Equal(t, int64(100), env.Data.Summary.PromptTokens)

	// The exclude filter of the users.
	env = callSiteStats(t, "range=7d&exclude_user_ids="+strconv.Itoa(fixture.AliceId))
	require.True(t, env.Success, env.Message)
	assert.Equal(t, int64(1), env.Data.Summary.Requests)
	assert.Equal(t, int64(1), env.Data.Summary.ByogRequests)

	// The model filter is a like match.
	env = callSiteStats(t, "range=7d&model=gpt")
	require.True(t, env.Success, env.Message)
	assert.Equal(t, int64(1), env.Data.Summary.Requests)
	assert.Equal(t, int64(40), env.Data.Summary.CompletionTokens)

	// The channel filter is exact.
	env = callSiteStats(t, "range=7d&channel_id=5")
	require.True(t, env.Success, env.Message)
	assert.Equal(t, int64(1), env.Data.Summary.Requests)

	// Both user lists together, the intersection is empty.
	env = callSiteStats(t, "range=7d&user_ids="+strconv.Itoa(fixture.AliceId)+"&exclude_user_ids="+strconv.Itoa(fixture.AliceId))
	require.True(t, env.Success, env.Message)
	assert.Equal(t, int64(0), env.Data.Summary.Requests)
}

func TestGetSiteStatisticsGranularityAndRange(t *testing.T) {
	setupSiteStatsTestDB(t)

	// The 24h preset resolves to the hour granularity.
	env := callSiteStats(t, "range=24h&tz_offset=0")
	require.True(t, env.Success, env.Message)
	assert.Equal(t, "hour", env.Data.Range.Granularity)
	assert.Equal(t, int64(3600), env.Data.Range.BucketSize)
	assert.GreaterOrEqual(t, len(env.Data.Series), 24)

	// The explicit granularity wins over the preset.
	env = callSiteStats(t, "range=7d&granularity=hour&tz_offset=0")
	require.True(t, env.Success, env.Message)
	assert.Equal(t, "hour", env.Data.Range.Granularity)
	assert.Equal(t, int64(3600), env.Data.Range.BucketSize)

	// The week granularity merges the day pairs, the totals survive.
	env = callSiteStats(t, "range=90d&granularity=week&tz_offset=0")
	require.True(t, env.Success, env.Message)
	assert.Equal(t, "week", env.Data.Range.Granularity)
	assert.Equal(t, int64(604800), env.Data.Range.BucketSize)
	var requests, prompt, byog int64
	for _, point := range env.Data.Series {
		requests += point.Requests
		prompt += point.PromptTokens
		byog += point.ByogRequests
	}
	assert.Equal(t, int64(2), requests)
	assert.Equal(t, int64(150), prompt)
	assert.Equal(t, int64(1), byog)

	// The explicit timestamps of the range.
	now := time.Now().Unix()
	env = callSiteStats(t, "start="+strconv.FormatInt(now-7200, 10)+"&end="+strconv.FormatInt(now, 10)+"&tz_offset=0")
	require.True(t, env.Success, env.Message)
	assert.Equal(t, now-7200, env.Data.Range.Start)
	assert.Equal(t, int64(2), env.Data.Summary.Requests)
}

func TestSiteStatsUserOptionsPrioritizeExactNameAndFindID(t *testing.T) {
	fixture := setupSiteStatsTestDB(t)
	// Both searches must find the exact match beyond the first page of similar names.
	for i := 0; i < 60; i++ {
		user := model.User{Username: "alice-" + strconv.Itoa(fixture.AliceId) + "-" + strconv.Itoa(i), AffCode: "aff-extra-" + strconv.Itoa(i), Status: 1}
		require.NoError(t, model.DB.Create(&user).Error)
	}
	for _, tc := range []struct{ query, first string }{{"alice", "alice"}, {strconv.Itoa(fixture.AliceId), "alice"}} {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest("GET", "/api/statistics/site/users?keyword="+tc.query, nil)
		SearchSiteStatsUsers(c)
		var body struct {
			Success bool              `json:"success"`
			Data    []siteStatsOption `json:"data"`
		}
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
		require.True(t, body.Success)
		require.NotEmpty(t, body.Data)
		assert.Equal(t, tc.first, body.Data[0].Name)
	}
}
