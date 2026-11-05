package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cleanByogObservabilityTables empties the tables that the observability
// queries read so that the assertions never see the rows of another test.
func cleanByogObservabilityTables(t *testing.T) {
	t.Helper()
	clean := func() {
		DB.Exec("DELETE FROM byog_nodes")
		DB.Exec("DELETE FROM byog_crash_reports")
		DB.Exec("DELETE FROM users")
	}
	clean()
	t.Cleanup(clean)
}

func TestGetByogNodeHealthCountsEveryState(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&ByogNode{}))
	cleanByogObservabilityTables(t)

	now := time.Now().Unix()
	nodes := []ByogNode{
		// fresh heartbeat, healthy
		{NodeId: "h-n1", UserId: 1, HWID: "hw-h1", DeviceName: "one", DeviceType: "desktop", Status: "online", LastHeartbeatAt: now},
		// claims online but the heartbeat died
		{NodeId: "h-n2", UserId: 1, HWID: "hw-h2", DeviceName: "two", DeviceType: "desktop", Status: "online", LastHeartbeatAt: now - 3600},
		// offline by the worker
		{NodeId: "h-n3", UserId: 1, HWID: "hw-h3", DeviceName: "three", DeviceType: "laptop", Status: "offline", LastHeartbeatAt: now - 7200},
		// paused after a crash report
		{NodeId: "h-n4", UserId: 1, HWID: "hw-h4", DeviceName: "four", DeviceType: "mobile", Status: "paused", LastHeartbeatAt: now - 500},
		// banned
		{NodeId: "h-n5", UserId: 1, HWID: "hw-h5", DeviceName: "five", DeviceType: "desktop", Status: "banned", LastHeartbeatAt: now - 9000},
		// fresh heartbeat and a running transfer
		{NodeId: "h-n6", UserId: 1, HWID: "hw-h6", DeviceName: "six", DeviceType: "desktop", Status: "online", ModelStatus: "downloading", LastHeartbeatAt: now},
		// fresh heartbeat and a failed transfer
		{NodeId: "h-n7", UserId: 1, HWID: "hw-h7", DeviceName: "seven", DeviceType: "laptop", Status: "online", DownloadStatus: "error", DownloadModelId: "qwen-7b", LastHeartbeatAt: now},
	}
	require.NoError(t, DB.Create(&nodes).Error)

	health, err := GetByogNodeHealth(now)
	require.NoError(t, err)
	assert.Equal(t, int64(7), health.Total)
	assert.Equal(t, int64(3), health.Online)
	assert.Equal(t, int64(1), health.Stale)
	assert.Equal(t, int64(1), health.Offline)
	assert.Equal(t, int64(1), health.Paused)
	assert.Equal(t, int64(1), health.Banned)
	assert.Equal(t, int64(1), health.Downloading)
	assert.Equal(t, int64(1), health.Errors)
}

func TestByogCrashReportPagingFiltersAndCounts(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&ByogCrashReport{}, &ByogNode{}, &User{}))
	cleanByogObservabilityTables(t)

	now := time.Now().Unix()
	users := []User{{Username: "erin", Status: 1}, {Username: "frank", Status: 1}}
	require.NoError(t, DB.Create(&users).Error)
	nodes := []ByogNode{
		{NodeId: "c-n1", UserId: users[0].Id, HWID: "hw-c1", DeviceName: "alpha-desktop", DeviceType: "desktop", Status: "paused"},
		{NodeId: "c-n2", UserId: users[1].Id, HWID: "hw-c2", DeviceName: "beta-phone", DeviceType: "mobile", Status: "online"},
	}
	require.NoError(t, DB.Create(&nodes).Error)

	reports := []*ByogCrashReport{
		{NodeId: "c-n1", UserId: users[0].Id, ErrorCode: "oom", ErrorMessage: "out of memory", CreatedAt: now - 10},
		{NodeId: "c-n1", UserId: users[0].Id, ErrorCode: "oom", ErrorMessage: "out of memory again", CreatedAt: now - 20},
		{NodeId: "c-n1", UserId: users[0].Id, ErrorCode: "timeout", ErrorMessage: "worker timeout", CreatedAt: now - 30},
		{NodeId: "c-n1", UserId: users[0].Id, ErrorCode: "oom", ErrorMessage: "third oom", CreatedAt: now - 40},
		{NodeId: "c-n2", UserId: users[1].Id, ErrorCode: "timeout", ErrorMessage: "phone timeout", CreatedAt: now - 50},
	}
	for _, report := range reports {
		// SaveCrashReport stamps the arrival with the current moment, the
		// test needs the explicit order so the rows go through Create.
		require.NoError(t, DB.Create(report).Error)
	}

	total, err := CountByogCrashReports()
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)

	since, err := CountByogCrashReportsSince(now - 35)
	require.NoError(t, err)
	assert.Equal(t, int64(3), since)

	// The first page of everything, newest first.
	rows, pageTotal, err := ListByogCrashReportsPage("", "", 1, 2)
	require.NoError(t, err)
	assert.Equal(t, int64(5), pageTotal)
	require.Len(t, rows, 2)
	assert.Equal(t, now-10, rows[0].CreatedAt)
	assert.Equal(t, now-20, rows[1].CreatedAt)

	// The second page continues the order.
	rows, _, err = ListByogCrashReportsPage("", "", 2, 2)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, now-30, rows[0].CreatedAt)

	// The node and the error code filters.
	rows, pageTotal, err = ListByogCrashReportsPage("c-n2", "", 1, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(1), pageTotal)
	require.Len(t, rows, 1)
	assert.Equal(t, "phone timeout", rows[0].ErrorMessage)

	rows, pageTotal, err = ListByogCrashReportsPage("", "oom", 1, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(3), pageTotal)
	require.Len(t, rows, 3)

	top, err := TopByogCrashErrorCodes(5)
	require.NoError(t, err)
	require.Len(t, top, 2)
	assert.Equal(t, "oom", top[0].Name)
	assert.Equal(t, int64(3), top[0].Count)
	assert.Equal(t, "timeout", top[1].Name)
	assert.Equal(t, int64(2), top[1].Count)

	devices, err := GetByogNodeDeviceNames([]string{"c-n1", "c-n2", "missing"})
	require.NoError(t, err)
	assert.Equal(t, "alpha-desktop", devices["c-n1"])
	assert.Equal(t, "beta-phone", devices["c-n2"])
	_, found := devices["missing"]
	assert.False(t, found)

	names, err := GetSiteStatsUsernames([]int{users[0].Id, users[1].Id})
	require.NoError(t, err)
	assert.Equal(t, "erin", names[users[0].Id])
	assert.Equal(t, "frank", names[users[1].Id])
}

func TestListByogNodesWithErrors(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&ByogNode{}))
	cleanByogObservabilityTables(t)

	now := time.Now().Unix()
	nodes := []ByogNode{
		{NodeId: "e-n1", UserId: 1, HWID: "hw-e1", DeviceName: "model-error", DeviceType: "desktop", Status: "online", ModelStatus: "error", UpdatedAt: now - 100},
		{NodeId: "e-n2", UserId: 2, HWID: "hw-e2", DeviceName: "transfer-error", DeviceType: "mobile", Status: "online", DownloadStatus: "error", DownloadModelId: "qwen-7b", DownloadProgress: 42, UpdatedAt: now - 50},
		{NodeId: "e-n3", UserId: 1, HWID: "hw-e3", DeviceName: "healthy", DeviceType: "desktop", Status: "online", ModelStatus: "ready", UpdatedAt: now},
	}
	require.NoError(t, DB.Create(&nodes).Error)

	rows, err := ListByogNodesWithErrors(10)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	// The newest update comes first.
	assert.Equal(t, "e-n2", rows[0].NodeId)
	assert.Equal(t, "qwen-7b", rows[0].DownloadModelId)
	assert.Equal(t, 42, rows[0].DownloadProgress)
	assert.Equal(t, "e-n1", rows[1].NodeId)
	assert.Equal(t, "error", rows[1].ModelStatus)
}
