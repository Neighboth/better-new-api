package model

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestByogWalletAndTokenReservationAtomicAndReplay(t *testing.T) {
	truncateTables(t)
	resetBatchUpdateTestState(t)
	user := createReserveTestUser(t, 100)
	token := Token{UserId: user.Id, Key: "byog-reserve-" + common.GetRandomString(8), Status: common.TokenStatusEnabled, ExpiredTime: -1, RemainQuota: 19}
	require.NoError(t, DB.Create(&token).Error)
	hold, err := BeginWalletTransferHold(user.Id)
	require.NoError(t, err)
	t.Cleanup(func() {
		DB.Where("hold_id = ?", hold).Delete(&WalletHoldEvent{})
		DB.Where("id = ?", hold).Delete(&WalletTransferHold{})
	})
	ok, err := ReserveByogWalletAndToken(user.Id, hold, token.Id, 20)
	require.NoError(t, err)
	require.False(t, ok)
	require.Equal(t, 100, getUserQuotaFromDB(t, user.Id))
	require.Equal(t, 19, getTokenFromDB(t, token.Id).RemainQuota)
	require.NoError(t, DB.Model(&token).Update("remain_quota", 21).Error)
	var wg sync.WaitGroup
	var successes atomic.Int32
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ok, err := ReserveByogWalletAndToken(user.Id, hold, token.Id, 20); err == nil && ok {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()
	require.Equal(t, int32(10), successes.Load())
	require.Equal(t, 80, getUserQuotaFromDB(t, user.Id))
	require.Equal(t, 1, getTokenFromDB(t, token.Id).RemainQuota)
	successes.Store(0)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if FinalizeByogWalletAndToken(user.Id, hold, token.Id, 20, 7) == nil {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()
	require.Equal(t, int32(10), successes.Load())
	require.Equal(t, 93, getUserQuotaFromDB(t, user.Id))
	finalToken := getTokenFromDB(t, token.Id)
	require.Equal(t, 14, finalToken.RemainQuota)
	require.Equal(t, 7, finalToken.UsedQuota)
	require.ErrorIs(t, FinalizeByogWalletAndToken(user.Id, hold, token.Id, 20, 8), ErrWalletHoldConflict)
	require.ErrorIs(t, ApplyWalletHoldDelta(user.Id, hold, "byog-final", 13), ErrWalletHoldConflict)
	require.NoError(t, ResolveWalletTransferHold(user.Id, hold, 0))
}

func TestByogTokenFailureRollsBackWalletAndJournal(t *testing.T) {
	truncateTables(t)
	resetBatchUpdateTestState(t)
	user := createReserveTestUser(t, 100)
	token := Token{UserId: user.Id, Key: "byog-rollback-" + common.GetRandomString(8), Status: common.TokenStatusEnabled, ExpiredTime: -1, RemainQuota: 50}
	require.NoError(t, DB.Create(&token).Error)
	hold, err := BeginWalletTransferHold(user.Id)
	require.NoError(t, err)
	t.Cleanup(func() {
		DB.Exec("DROP TRIGGER IF EXISTS fail_byog_token")
		DB.Where("hold_id = ?", hold).Delete(&WalletHoldEvent{})
		DB.Where("id = ?", hold).Delete(&WalletTransferHold{})
	})
	require.NoError(t, DB.Exec(`CREATE TRIGGER fail_byog_token BEFORE UPDATE ON tokens BEGIN SELECT RAISE(ABORT, 'token unavailable'); END;`).Error)
	_, err = ReserveByogWalletAndToken(user.Id, hold, token.Id, 20)
	require.Error(t, err)
	require.Equal(t, 100, getUserQuotaFromDB(t, user.Id))
	require.Equal(t, 50, getTokenFromDB(t, token.Id).RemainQuota)
	var events int64
	require.NoError(t, DB.Model(&WalletHoldEvent{}).Where("hold_id = ?", hold).Count(&events).Error)
	require.Zero(t, events)
	require.NoError(t, DB.Exec("DROP TRIGGER fail_byog_token").Error)
	ok, err := ReserveByogWalletAndToken(user.Id, hold, token.Id, 20)
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, DB.Exec(`CREATE TRIGGER fail_byog_token BEFORE UPDATE ON tokens BEGIN SELECT RAISE(ABORT, 'token unavailable'); END;`).Error)
	require.Error(t, FinalizeByogWalletAndToken(user.Id, hold, token.Id, 20, 5))
	require.Equal(t, 80, getUserQuotaFromDB(t, user.Id))
	require.Equal(t, 30, getTokenFromDB(t, token.Id).RemainQuota)
	require.NoError(t, DB.Model(&WalletHoldEvent{}).Where("hold_id = ?", hold).Count(&events).Error)
	require.Equal(t, int64(1), events)
	require.NoError(t, DB.Exec("DROP TRIGGER fail_byog_token").Error)
	require.NoError(t, FinalizeByogWalletAndToken(user.Id, hold, token.Id, 20, 5))
	require.NoError(t, ResolveWalletTransferHold(user.Id, hold, 0))
	require.Equal(t, 95, getUserQuotaFromDB(t, user.Id))
	require.Equal(t, 45, getTokenFromDB(t, token.Id).RemainQuota)
}
