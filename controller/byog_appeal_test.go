package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestByogAppealRequiresBanOwnershipAndCommitsAtomically(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "appeal.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	old := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = old; _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&model.ByogNodeBan{}, &model.Ticket{}, &model.TicketMessage{}))
	ban := model.ByogNodeBan{NodeId: "private-device", UserId: 42, HWID: "hw", BanReason: "private reason", ViolationCode: "private-code", IsActive: true}
	require.NoError(t, db.Create(&ban).Error)
	request := func(user int) (int, map[string]interface{}) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("id", user)
		c.Request = httptest.NewRequest(http.MethodPost, "/appeal", bytes.NewBufferString(`{"node_id":"private-device","appeal_text":"please review"}`))
		c.Request.Header.Set("Content-Type", "application/json")
		SubmitBanAppeal(c)
		var body map[string]interface{}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		return w.Code, body
	}
	status, body := request(43)
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, false, body["success"])
	require.NotContains(t, body, "ticket_id")
	var count int64
	require.NoError(t, db.Model(&model.Ticket{}).Count(&count).Error)
	require.Zero(t, count)
	// An injected message error must not leave a ticket or mutate the ban.
	require.NoError(t, db.Exec(`CREATE TRIGGER fail_appeal_message BEFORE INSERT ON ticket_messages BEGIN SELECT RAISE(ABORT, 'injected'); END;`).Error)
	status, body = request(42)
	require.Equal(t, http.StatusInternalServerError, status)
	require.Equal(t, false, body["success"])
	require.NoError(t, db.Model(&model.Ticket{}).Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, db.First(&ban, ban.Id).Error)
	require.Zero(t, ban.AppealTicketId)
	require.NoError(t, db.Exec("DROP TRIGGER fail_appeal_message").Error)
	status, body = request(42)
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, true, body["success"])
	require.NoError(t, db.First(&ban, ban.Id).Error)
	require.True(t, ban.IsActive)
	require.Equal(t, int(body["ticket_id"].(float64)), ban.AppealTicketId)
}
