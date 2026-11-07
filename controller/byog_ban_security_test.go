package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestByogAccountBanBlocksPairingRegistrationAndWorkerCredential(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "ban.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	old := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = old; _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&model.ByogNode{}, &model.ByogNodeBan{}, &model.ByogPairingSession{}))
	secret, hash, err := newByogWorkerCredential()
	require.NoError(t, err)
	node := model.ByogNode{NodeId: "another-device", UserId: 42, HWID: "different-hardware", Status: "online", WorkerSecretHash: hash}
	require.NoError(t, db.Create(&node).Error)
	ban := model.ByogNodeBan{NodeId: "banned-device", UserId: 42, HWID: "old-hardware", BanReason: "sensitive", ViolationCode: "private", IsActive: true}
	require.NoError(t, db.Create(&ban).Error)
	require.NoError(t, model.CreateByogPairingSession("ABCD-EFGH-IJKL-MNOP", 42, time.Now().Unix()+900))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("id", 42)
	c.Request = httptest.NewRequest(http.MethodGet, "/pair", nil)
	GetUserPairingCode(c)
	require.Equal(t, http.StatusForbidden, w.Code)

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/register", bytes.NewBufferString(`{"hwid":"different-hardware","os":"windows","device_type":"desktop","pairing_code":"ABCD-EFGH-IJKL-MNOP"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	RegisterByogWorker(c)
	require.Equal(t, http.StatusForbidden, w.Code)
	require.NotContains(t, w.Body.String(), "sensitive")
	var nodeCount int64
	require.NoError(t, db.Model(&model.ByogNode{}).Count(&nodeCount).Error)
	require.EqualValues(t, 1, nodeCount)

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/tunnel", nil)
	c.Request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", secret))
	_, err = authenticatedByogWorker(c, node.NodeId)
	require.Error(t, err)
	require.NoError(t, db.Model(&ban).Update("is_active", false).Error)
	got, err := authenticatedByogWorker(c, node.NodeId)
	require.NoError(t, err)
	require.Equal(t, node.NodeId, got.NodeId)
	// A database failure must never silently authorize the worker.
	require.NoError(t, db.Migrator().DropTable(&model.ByogNodeBan{}))
	_, err = authenticatedByogWorker(c, node.NodeId)
	require.Error(t, err)
}
