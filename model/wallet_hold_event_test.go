package model

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWalletHoldEventsAreAtomicIdempotentAndBoundToUser(t *testing.T) {
	truncateTables(t)
	resetBatchUpdateTestState(t)
	user := createReserveTestUser(t, 100)
	other := createReserveTestUser(t, 100)
	t.Cleanup(func() {
		DB.Where("user_id IN ?", []int{user.Id, other.Id}).Delete(&WalletHoldEvent{})
		DB.Where("user_id IN ?", []int{user.Id, other.Id}).Delete(&WalletTransferHold{})
	})
	id, err := BeginWalletTransferHold(user.Id)
	require.NoError(t, err)
	var successes atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := ReserveWalletHoldQuota(user.Id, id, "precharge", 60)
			if err == nil && ok {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()
	require.Equal(t, int32(10), successes.Load())
	require.Equal(t, 40, getUserQuotaFromDB(t, user.Id))
	ok, err := ReserveWalletHoldQuota(user.Id, id, "other", 41)
	require.NoError(t, err)
	require.False(t, ok)
	require.Error(t, ApplyWalletHoldDelta(other.Id, id, "final", -10))
	require.NoError(t, ApplyWalletHoldDelta(user.Id, id, "final", -10))
	require.NoError(t, ApplyWalletHoldDelta(user.Id, id, "final", -10))
	require.ErrorIs(t, ApplyWalletHoldDelta(user.Id, id, "final", -11), ErrWalletHoldConflict)
	require.Equal(t, 30, getUserQuotaFromDB(t, user.Id))
	require.NoError(t, ResolveWalletTransferHold(user.Id, id, 0))
	require.NoError(t, ApplyWalletHoldDelta(user.Id, id, "final", -10), "a committed event retry is safe after closure")
	require.Error(t, ApplyWalletHoldDelta(user.Id, id, "late", -1))
}
