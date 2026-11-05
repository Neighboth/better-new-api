package controller

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// observabilityEnvelope mirrors the answer of the observability summary.
type observabilityEnvelope struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		NodeHealth struct {
			Total       int64 `json:"total"`
			Online      int64 `json:"online"`
			Stale       int64 `json:"stale"`
			Offline     int64 `json:"offline"`
			Paused      int64 `json:"paused"`
			Banned      int64 `json:"banned"`
			Downloading int64 `json:"downloading"`
			Errors      int64 `json:"errors"`
		} `json:"node_health"`
		CrashReports24h   int64 `json:"crash_reports_24h"`
		CrashReports7d    int64 `json:"crash_reports_7d"`
		CrashReportsTotal int64 `json:"crash_reports_total"`
		TopErrorCodes     []struct {
			Name  string `json:"name"`
			Count int64  `json:"count"`
		} `json:"top_error_codes"`
	} `json:"data"`
}

// crashReportsEnvelope mirrors the answer of the crash report page.
type crashReportsEnvelope struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		Items []struct {
			Id             int    `json:"id"`
			NodeId         string `json:"node_id"`
			UserId         int    `json:"user_id"`
			ErrorCode      string `json:"error_code"`
			ErrorMessage   string `json:"error_message"`
			StackTrace     string `json:"stack_trace"`
			MemoryUsageMB  int    `json:"memory_usage_mb"`
			ModelId        string `json:"model_id"`
			Recommendation string `json:"recommendation"`
			CreatedAt      int64  `json:"created_at"`
			Username       string `json:"username"`
			DeviceName     string `json:"device_name"`
		} `json:"items"`
		Total    int64 `json:"total"`
		Page     int   `json:"page"`
		PageSize int   `json:"page_size"`
	} `json:"data"`
}

// downloadErrorsEnvelope mirrors the answer of the download problem list.
type downloadErrorsEnvelope struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		Items []struct {
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
			DownloadStartedAt int64  `json:"download_started_at"`
			LastHeartbeatAt   int64  `json:"last_heartbeat_at"`
		} `json:"items"`
	} `json:"data"`
}

// setupByogObservabilityTestDB reuses the statistics world and adds the
// crash reports and the broken transfer of a node.
func setupByogObservabilityTestDB(t *testing.T) *siteStatsFixture {
	t.Helper()
	fixture := setupSiteStatsTestDB(t)
	now := time.Now().Unix()

	// The fixture carries an online and an offline node, the observability
	// world adds a stale, a paused, a downloading and a broken node.
	require.NoError(t, model.DB.Create(&[]model.ByogNode{
		{NodeId: "o-n3", UserId: fixture.AliceId, HWID: "hw-o3", DeviceName: "stale-rig", DeviceType: "desktop", Status: "online", LastHeartbeatAt: now - 3600},
		{NodeId: "o-n4", UserId: fixture.BobId, HWID: "hw-o4", DeviceName: "crashed-phone", DeviceType: "mobile", Status: "paused", LastHeartbeatAt: now - 500},
		{NodeId: "o-n5", UserId: fixture.AliceId, HWID: "hw-o5", DeviceName: "downloading-rig", DeviceType: "laptop", Status: "online", ModelStatus: "downloading", DownloadModelId: "qwen-7b", DownloadProgress: 55, LastHeartbeatAt: now},
		{NodeId: "o-n6", UserId: fixture.BobId, HWID: "hw-o6", DeviceName: "broken-phone", DeviceType: "mobile", Status: "online", DownloadStatus: "error", DownloadModelId: "qwen-7b", DownloadProgress: 42, LastHeartbeatAt: now},
	}).Error)

	// The crash reports of the paused node and of the broken phone.
	reports := []*model.ByogCrashReport{
		{NodeId: "s-n1", UserId: fixture.AliceId, ErrorCode: "oom", ErrorMessage: "out of memory", StackTrace: "goroutine 1 ...", MemoryUsageMB: 15800, ModelId: "qwen-7b", Recommendation: "Model daha kucuk bir olcek secin.", CreatedAt: now - 10},
		{NodeId: "s-n1", UserId: fixture.AliceId, ErrorCode: "oom", ErrorMessage: "out of memory again", CreatedAt: now - 20},
		{NodeId: "s-n1", UserId: fixture.AliceId, ErrorCode: "timeout", ErrorMessage: "worker timeout", CreatedAt: now - 30},
		{NodeId: "o-n4", UserId: fixture.BobId, ErrorCode: "crash", ErrorMessage: "signal 11", CreatedAt: now - 40},
	}
	for _, report := range reports {
		require.NoError(t, model.DB.Create(report).Error)
	}
	return fixture
}

func TestGetByogObservabilitySummary(t *testing.T) {
	setupByogObservabilityTestDB(t)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("GET", "/api/byog/admin/observability/summary", nil)
	GetByogObservabilitySummary(c)
	require.Equal(t, 200, recorder.Code)

	env := &observabilityEnvelope{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), env))
	require.True(t, env.Success, env.Message)

	// Two nodes of the fixture plus four of this world.
	assert.Equal(t, int64(6), env.Data.NodeHealth.Total)
	// Fresh heartbeats: s-n1 of the fixture, o-n5 and o-n6.
	assert.Equal(t, int64(3), env.Data.NodeHealth.Online)
	// s-n2 of the fixture is offline, o-n4 is paused.
	assert.Equal(t, int64(1), env.Data.NodeHealth.Offline)
	assert.Equal(t, int64(1), env.Data.NodeHealth.Paused)
	// o-n3 claims to be online with a dead heartbeat.
	assert.Equal(t, int64(1), env.Data.NodeHealth.Stale)
	assert.Equal(t, int64(1), env.Data.NodeHealth.Downloading)
	assert.Equal(t, int64(1), env.Data.NodeHealth.Errors)

	assert.Equal(t, int64(4), env.Data.CrashReportsTotal)
	assert.Equal(t, int64(4), env.Data.CrashReports24h)
	assert.Equal(t, int64(4), env.Data.CrashReports7d)
	require.Len(t, env.Data.TopErrorCodes, 3)
	assert.Equal(t, "oom", env.Data.TopErrorCodes[0].Name)
	assert.Equal(t, int64(2), env.Data.TopErrorCodes[0].Count)
}

func TestGetByogCrashReportsResolvesNamesAndPages(t *testing.T) {
	fixture := setupByogObservabilityTestDB(t)

	// The first page of everything, newest first, with the resolved names.
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("GET", "/api/byog/admin/observability/crash-reports?page=1&page_size=2", nil)
	GetByogCrashReports(c)
	require.Equal(t, 200, recorder.Code)

	env := &crashReportsEnvelope{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), env))
	require.True(t, env.Success, env.Message)
	assert.Equal(t, int64(4), env.Data.Total)
	assert.Equal(t, 1, env.Data.Page)
	assert.Equal(t, 2, env.Data.PageSize)
	require.Len(t, env.Data.Items, 2)
	assert.Equal(t, "oom", env.Data.Items[0].ErrorCode)
	assert.Equal(t, "out of memory", env.Data.Items[0].ErrorMessage)
	assert.Equal(t, "alice", env.Data.Items[0].Username)
	assert.Equal(t, "alice-desktop", env.Data.Items[0].DeviceName)
	assert.Equal(t, "s-n1", env.Data.Items[0].NodeId)
	assert.Equal(t, fixture.AliceId, env.Data.Items[0].UserId)
	assert.Greater(t, env.Data.Items[0].Id, 0)

	// The second page continues the order.
	recorder = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("GET", "/api/byog/admin/observability/crash-reports?page=2&page_size=2", nil)
	GetByogCrashReports(c)
	env = &crashReportsEnvelope{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), env))
	require.True(t, env.Success, env.Message)
	require.Len(t, env.Data.Items, 2)
	assert.Equal(t, "crash", env.Data.Items[1].ErrorCode)
	assert.Equal(t, "bob", env.Data.Items[1].Username)
	assert.Equal(t, "crashed-phone", env.Data.Items[1].DeviceName)

	// The node filter.
	recorder = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("GET", "/api/byog/admin/observability/crash-reports?node_id=o-n4", nil)
	GetByogCrashReports(c)
	env = &crashReportsEnvelope{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), env))
	require.True(t, env.Success, env.Message)
	assert.Equal(t, int64(1), env.Data.Total)
	assert.Equal(t, 20, env.Data.PageSize)

	// The error code filter and the page size cap.
	recorder = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("GET", "/api/byog/admin/observability/crash-reports?error_code=oom&page_size=500", nil)
	GetByogCrashReports(c)
	env = &crashReportsEnvelope{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), env))
	require.True(t, env.Success, env.Message)
	assert.Equal(t, int64(2), env.Data.Total)
	assert.Equal(t, 100, env.Data.PageSize)
}

func TestGetByogDownloadErrors(t *testing.T) {
	setupByogObservabilityTestDB(t)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("GET", "/api/byog/admin/observability/download-errors", nil)
	GetByogDownloadErrors(c)
	require.Equal(t, 200, recorder.Code)

	env := &downloadErrorsEnvelope{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), env))
	require.True(t, env.Success, env.Message)
	require.Len(t, env.Data.Items, 1)
	assert.Equal(t, "o-n6", env.Data.Items[0].NodeId)
	assert.Equal(t, "broken-phone", env.Data.Items[0].DeviceName)
	assert.Equal(t, "mobile", env.Data.Items[0].DeviceType)
	assert.Equal(t, "bob", env.Data.Items[0].Username)
	assert.Equal(t, "error", env.Data.Items[0].DownloadStatus)
	assert.Equal(t, "qwen-7b", env.Data.Items[0].DownloadModelId)
	assert.Equal(t, 42, env.Data.Items[0].DownloadProgress)
}
