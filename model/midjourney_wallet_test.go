package model

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMidjourneyWalletRefundAtomicAndStaleStatusUpdate(t *testing.T) {
	truncateTables(t)
	resetBatchUpdateTestState(t)
	require.NoError(t, DB.AutoMigrate(&Midjourney{}))
	user := createReserveTestUser(t, 100)
	task := Midjourney{UserId: user.Id, MjId: "atomic-refund", Quota: 20, Status: "FAILURE", Progress: "100%"}
	require.NoError(t, DB.Create(&task).Error)
	t.Cleanup(func() { DB.Where("id = ?", task.Id).Delete(&Midjourney{}) })
	stale := task
	var refunds atomic.Int32
	var errorsSeen atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			amount, err := RefundMidjourneyWalletOnce(user.Id, task.Id)
			if err != nil {
				errorsSeen.Add(1)
			} else {
				refunds.Add(int32(amount))
			}
		}()
	}
	wg.Wait()
	require.Equal(t, int32(0), errorsSeen.Load())
	require.Equal(t, int32(20), refunds.Load())
	require.Equal(t, 120, getUserQuotaFromDB(t, user.Id))
	stale.FailReason = "updated by a stale poller"
	won, err := stale.UpdateWithStatus("FAILURE")
	require.NoError(t, err)
	require.True(t, won)
	var persisted Midjourney
	require.NoError(t, DB.First(&persisted, task.Id).Error)
	require.Zero(t, persisted.Quota)
	amount, err := RefundMidjourneyWalletOnce(user.Id, task.Id)
	require.NoError(t, err)
	require.Zero(t, amount)
	require.Equal(t, 120, getUserQuotaFromDB(t, user.Id))
}

func TestMidjourneyPendingChargeCannotBeRefunded(t *testing.T) {
	truncateTables(t)
	resetBatchUpdateTestState(t)
	require.NoError(t, DB.AutoMigrate(&Midjourney{}))
	user := createReserveTestUser(t, 100)
	task := Midjourney{UserId: user.Id, MjId: "pending", Quota: 20, BillingPending: true}
	require.NoError(t, DB.Create(&task).Error)
	t.Cleanup(func() { DB.Where("id = ?", task.Id).Delete(&Midjourney{}) })
	_, err := RefundMidjourneyWalletOnce(user.Id, task.Id)
	require.Error(t, err)
	require.Equal(t, 100, getUserQuotaFromDB(t, user.Id))
	var persisted Midjourney
	require.NoError(t, DB.First(&persisted, task.Id).Error)
	require.Equal(t, 20, persisted.Quota)
	require.True(t, persisted.BillingPending)
}
