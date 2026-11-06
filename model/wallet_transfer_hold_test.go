package model

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestWalletHoldBlocksTransfersAndSettlementIsIdempotent(t *testing.T) {
	truncateTables(t)
	resetBatchUpdateTestState(t)
	require.NoError(t, DB.AutoMigrate(&Redemption{}, &UserBalancePackage{}))
	user := createReserveTestUser(t, 100)
	other := createReserveTestUser(t, 100)
	t.Cleanup(func() {
		DB.Where("user_id IN ?", []int{user.Id, other.Id}).Delete(&WalletTransferHold{})
		DB.Unscoped().Where("user_id = ?", user.Id).Delete(&Redemption{})
	})
	id, err := BeginWalletTransferHold(user.Id)
	require.NoError(t, err)
	_, err = CreateResellerBalanceCodes(user.Id, 30, 1, "blocked")
	require.Error(t, err)
	require.Equal(t, 100, getUserQuotaFromDB(t, user.Id))
	require.Error(t, ResolveWalletTransferHold(other.Id, id, -30), "one user cannot settle another user's hold")

	var success atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ResolveWalletTransferHold(user.Id, id, -30) == nil {
				success.Add(1)
			}
		}()
	}
	wg.Wait()
	require.Equal(t, int32(10), success.Load(), "identical retries are safe")
	require.Equal(t, 70, getUserQuotaFromDB(t, user.Id), "charge is applied exactly once")
	require.ErrorIs(t, ResolveWalletTransferHold(user.Id, id, -29), ErrWalletHoldConflict)
	_, err = CreateResellerBalanceCodes(user.Id, 70, 1, "after-settlement")
	require.NoError(t, err)
	require.Equal(t, 0, getUserQuotaFromDB(t, user.Id))
}

func TestWalletHoldSettlementOutOfRangeStaysOpen(t *testing.T) {
	truncateTables(t)
	resetBatchUpdateTestState(t)
	require.NoError(t, DB.AutoMigrate(&Redemption{}, &UserBalancePackage{}))
	user := createReserveTestUser(t, common.MaxQuota)
	t.Cleanup(func() { DB.Where("user_id = ?", user.Id).Delete(&WalletTransferHold{}) })
	id, err := BeginWalletTransferHold(user.Id)
	require.NoError(t, err)
	require.Error(t, ResolveWalletTransferHold(user.Id, id, 1))
	require.Equal(t, common.MaxQuota, getUserQuotaFromDB(t, user.Id))
	var hold WalletTransferHold
	require.NoError(t, DB.First(&hold, "id = ?", id).Error)
	require.Equal(t, "open", hold.State)
	require.NoError(t, ResolveWalletTransferHold(user.Id, id, 0))
}

func TestWalletHoldRetryInvalidatesRehydratedCache(t *testing.T) {
	truncateTables(t)
	resetBatchUpdateTestState(t)
	useUserCacheMiniRedis(t)
	user := createReserveTestUser(t, 100)
	t.Cleanup(func() { DB.Where("user_id = ?", user.Id).Delete(&WalletTransferHold{}) })
	require.NoError(t, populateUserCache(user))
	id, err := BeginWalletTransferHold(user.Id)
	require.NoError(t, err)
	require.NoError(t, ResolveWalletTransferHold(user.Id, id, -10))
	// Simulate a stale rehydration after the SQL commit but before an
	// ambiguous settlement acknowledgment was retried.
	require.NoError(t, populateUserCache(user))
	require.NoError(t, ResolveWalletTransferHold(user.Id, id, -10))
	cached, err := GetUserCache(user.Id)
	require.NoError(t, err)
	require.Equal(t, 90, cached.Quota)
	require.Equal(t, 90, getUserQuotaFromDB(t, user.Id))
}
