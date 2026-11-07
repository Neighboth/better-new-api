package model

import (
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestStaleByogWorkerCannotUndoBanDeletionOrOwnership(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "nodes.db")), &gorm.Config{})
	require.NoError(t, err)
	old := DB
	DB = db
	t.Cleanup(func() { DB = old })
	require.NoError(t, db.AutoMigrate(&ByogNode{}))
	node := &ByogNode{NodeId: "device-a", UserId: 12, WorkerSecretHash: "original", HWID: "same-hardware", DeviceName: "A", Status: "online"}
	other := &ByogNode{NodeId: "device-b", UserId: 13, WorkerSecretHash: "other", HWID: "same-hardware", DeviceName: "B", Status: "online"}
	require.NoError(t, SaveByogNode(node))
	require.NoError(t, SaveByogNode(other))
	stale := *node
	stale.UserId = 99
	stale.WorkerSecretHash = "forged"
	stale.HWID = "forged"
	stale.DeviceName = "updated"
	require.NoError(t, SaveByogLiveNode(&stale))
	found, err := GetByogNodeByNodeId("device-a")
	require.NoError(t, err)
	require.Equal(t, 12, found.UserId)
	require.Equal(t, "original", found.WorkerSecretHash)
	require.Equal(t, "same-hardware", found.HWID)
	require.Equal(t, "updated", found.DeviceName)
	require.NoError(t, db.Model(&ByogNode{}).Where("id = ?", node.Id).Update("status", "banned").Error)
	stale.Status = "online"
	require.NoError(t, SaveByogLiveNode(&stale))
	found, err = GetByogNodeByNodeId("device-a")
	require.NoError(t, err)
	require.Equal(t, "banned", found.Status)
	require.NoError(t, DeleteByogNode("device-a"))
	require.NoError(t, SaveByogNode(&stale))
	require.ErrorIs(t, db.Where("node_id = ?", "device-a").First(&ByogNode{}).Error, gorm.ErrRecordNotFound)
	require.NoError(t, db.Where("node_id = ?", "device-b").First(&ByogNode{}).Error)
}

func TestPairingSessionIsHashedExpiringAndSingleUse(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "pairing.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	old := DB
	DB = db
	t.Cleanup(func() { DB = old })
	require.NoError(t, db.AutoMigrate(&ByogPairingSession{}))
	require.NoError(t, CreateByogPairingSession("ABCD-EFGH-IJKL-MNOP", 7, time.Now().Unix()+900))
	var session ByogPairingSession
	require.NoError(t, db.First(&session).Error)
	require.NotContains(t, session.CodeHash, "ABCD")
	var successes atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			user, err := ConsumeByogPairingSession("abcd efgh ijkl mnop")
			if err == nil && user == 7 {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()
	require.Equal(t, int32(1), successes.Load())
	require.NoError(t, CreateByogPairingSession("EXPIRED", 7, time.Now().Unix()-1))
	_, err = ConsumeByogPairingSession("EXPIRED")
	require.Error(t, err)
}

func TestUnbanDoesNotReactivateOtherOwnersSharingHardware(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "unban.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	old := DB
	DB = db
	t.Cleanup(func() { DB = old; _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&ByogNode{}, &ByogNodeBan{}))
	one := ByogNode{NodeId: "owner-one", UserId: 1, HWID: "shared", Status: "banned"}
	two := ByogNode{NodeId: "owner-two", UserId: 2, HWID: "shared", Status: "banned"}
	require.NoError(t, db.Create(&one).Error)
	require.NoError(t, db.Create(&two).Error)
	ban := ByogNodeBan{NodeId: one.NodeId, UserId: one.UserId, HWID: one.HWID, BanReason: "reason", ViolationCode: "code", IsActive: true}
	require.NoError(t, db.Create(&ban).Error)
	require.NoError(t, UnbanNode(ban.Id))
	require.NoError(t, db.First(&one, one.Id).Error)
	require.NoError(t, db.First(&two, two.Id).Error)
	require.Equal(t, "offline", one.Status)
	require.Equal(t, "banned", two.Status)
}
