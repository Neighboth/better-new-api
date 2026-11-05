package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cleanSiteStatsTables empties the tables that the site statistics read so
// that the assertions of this file never see the rows of another test.
func cleanSiteStatsTables(t *testing.T) {
	t.Helper()
	clean := func() {
		DB.Exec("DELETE FROM logs")
		DB.Exec("DELETE FROM users")
		DB.Exec("DELETE FROM channels")
		DB.Exec("DELETE FROM byog_nodes")
	}
	clean()
	t.Cleanup(clean)
}

func TestGetSiteStatsSeriesCarriesTokenSplitsAndByog(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Log{}, &User{}, &ByogNode{}))
	cleanSiteStatsTables(t)

	now := time.Now().Unix()
	users := []User{{Username: "alice", Status: 1}, {Username: "bob", Status: 1}}
	require.NoError(t, DB.Create(&users).Error)

	logs := []Log{
		{
			UserId: users[0].Id, Username: "alice", CreatedAt: now - 3600,
			Type: LogTypeConsume, ModelName: "gpt-4o", TokenName: "alice-key",
			ChannelId: 5, Group: "default", Ip: "10.0.0.1",
			Quota: 700, PromptTokens: 100, CompletionTokens: 40, UseTime: 1200,
		},
		{
			UserId: users[1].Id, Username: "bob", CreatedAt: now - 1800,
			Type: LogTypeConsume, ModelName: "byog-qwen", TokenName: "bob-key",
			ChannelId: 0, Group: "default", Ip: "10.0.0.2",
			Quota: 300, PromptTokens: 50, CompletionTokens: 10, UseTime: 800,
			Other: `{"byog":true,"admin_info":{}}`,
		},
		{UserId: users[0].Id, Username: "alice", CreatedAt: now - 900, Type: LogTypeError, Content: "upstream timeout"},
	}
	require.NoError(t, DB.Create(&logs).Error)

	f := SiteStatsFilter{StartTs: now - 86400, EndTs: now + 60, TzOffsetSec: 0}

	// The day buckets can split around the UTC midnight, the totals of every
	// bucket together are the stable assertion.
	series, err := GetSiteStatsSeries(f, 86400)
	require.NoError(t, err)
	var requests, tokens, prompt, completion, quota, byog int64
	for _, row := range series {
		requests += row.Requests
		tokens += row.Tokens
		prompt += row.PromptTokens
		completion += row.CompletionTokens
		quota += row.Quota
		byog += row.ByogRequests
	}
	assert.Equal(t, int64(2), requests)
	assert.Equal(t, int64(200), tokens)
	assert.Equal(t, int64(150), prompt)
	assert.Equal(t, int64(50), completion)
	assert.Equal(t, int64(1000), quota)
	assert.Equal(t, int64(1), byog)

	// The day and the user pairs of the week and the month merge carry the
	// same columns.
	pairs, err := GetSiteStatsDayUserPairs(f)
	require.NoError(t, err)
	var pairRequests, pairPrompt, pairCompletion, pairByog int64
	for _, pair := range pairs {
		pairRequests += pair.Requests
		pairPrompt += pair.PromptTokens
		pairCompletion += pair.CompletionTokens
		pairByog += pair.ByogRequests
	}
	assert.Equal(t, int64(2), pairRequests)
	assert.Equal(t, int64(150), pairPrompt)
	assert.Equal(t, int64(50), pairCompletion)
	assert.Equal(t, int64(1), pairByog)

	summary, err := GetSiteStatsSummary(f)
	require.NoError(t, err)
	assert.Equal(t, int64(2), summary.Requests)
	assert.Equal(t, int64(150), summary.PromptTokens)
	assert.Equal(t, int64(50), summary.CompletionTokens)
	assert.Equal(t, int64(200), summary.Tokens)
	assert.Equal(t, int64(1000), summary.Quota)
	assert.Equal(t, int64(2), summary.UniqueUsers)
	assert.Equal(t, int64(1), summary.ByogRequests)
	assert.InDelta(t, 1000.0, summary.AvgUseTimeSecond, 0.01)
}

func TestGetSiteStatsTopGroupsUsesTheGroupColumn(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Log{}, &User{}))
	cleanSiteStatsTables(t)

	now := time.Now().Unix()
	user := User{Username: "carol", Status: 1}
	require.NoError(t, DB.Create(&user).Error)
	logs := []Log{
		{UserId: user.Id, Username: "carol", CreatedAt: now - 3600, Type: LogTypeConsume, ModelName: "gpt-4o", Group: "default", Quota: 100, PromptTokens: 10, CompletionTokens: 5},
		{UserId: user.Id, Username: "carol", CreatedAt: now - 1800, Type: LogTypeConsume, ModelName: "gpt-4o", Group: "vip", Quota: 200, PromptTokens: 20, CompletionTokens: 5},
		{UserId: user.Id, Username: "carol", CreatedAt: now - 900, Type: LogTypeConsume, ModelName: "gpt-4o", Group: "vip", Quota: 300, PromptTokens: 30, CompletionTokens: 5},
	}
	require.NoError(t, DB.Create(&logs).Error)

	f := SiteStatsFilter{StartTs: now - 86400, EndTs: now + 60, TzOffsetSec: 0}

	// TestMain runs initCol so the quoted group column of the dialect is in
	// place, the ranking must answer without an error.
	groups, err := GetSiteStatsTopGroups(f, "requests", "desc", 10)
	require.NoError(t, err)
	require.Len(t, groups, 2)
	assert.Equal(t, "vip", groups[0].GroupName)
	assert.Equal(t, int64(2), groups[0].Requests)
	assert.Equal(t, "default", groups[1].GroupName)
	assert.Equal(t, int64(1), groups[1].Requests)

	// The group filter of the request applies on the same column.
	filtered := f
	filtered.Group = "vip"
	summary, err := GetSiteStatsSummary(filtered)
	require.NoError(t, err)
	assert.Equal(t, int64(2), summary.Requests)
	assert.Equal(t, int64(500), summary.Quota)
}

func TestGetSiteStatsByogDistributions(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&ByogNode{}, &User{}))
	cleanSiteStatsTables(t)

	now := time.Now().Unix()
	user := User{Username: "dave", Status: 1}
	require.NoError(t, DB.Create(&user).Error)
	nodes := []ByogNode{
		{NodeId: "dist-n1", UserId: user.Id, HWID: "hw-dist-1", DeviceName: "dave-desktop", DeviceType: "desktop", Country: "Turkey", Status: "online", LastHeartbeatAt: now},
		{NodeId: "dist-n2", UserId: user.Id, HWID: "hw-dist-2", DeviceName: "dave-phone", DeviceType: "mobile", Country: "Germany", Status: "offline"},
		{NodeId: "dist-n3", UserId: user.Id, HWID: "hw-dist-3", DeviceName: "dave-tablet", DeviceType: "mobile", Country: "Germany", Status: "paused"},
		{NodeId: "dist-n4", UserId: user.Id, HWID: "hw-dist-4", DeviceName: "dave-empty", DeviceType: "", Country: "", Status: "offline"},
	}
	require.NoError(t, DB.Create(&nodes).Error)

	deviceTypes, err := GetSiteStatsByogDeviceTypes()
	require.NoError(t, err)
	require.Len(t, deviceTypes, 2)
	assert.Equal(t, "mobile", deviceTypes[0].Name)
	assert.Equal(t, int64(2), deviceTypes[0].Count)
	assert.Equal(t, "desktop", deviceTypes[1].Name)
	assert.Equal(t, int64(1), deviceTypes[1].Count)

	countries, err := GetSiteStatsByogCountries(10)
	require.NoError(t, err)
	require.Len(t, countries, 2)
	assert.Equal(t, "Germany", countries[0].Name)
	assert.Equal(t, int64(2), countries[0].Count)
	assert.Equal(t, "Turkey", countries[1].Name)
}
