package model

import (
	"sync"
	"sync/atomic"
	"testing"

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
