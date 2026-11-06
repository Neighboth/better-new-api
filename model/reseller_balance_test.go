package model

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestResellerBalanceCreationAndSingleRefund(t *testing.T) {
	truncateTables(t)
	resetBatchUpdateTestState(t)
	require.NoError(t, DB.AutoMigrate(&Redemption{}, &UserBalancePackage{}))
	user := createReserveTestUser(t, 100)
	t.Cleanup(func() { DB.Unscoped().Where("user_id = ?", user.Id).Delete(&Redemption{}) })
	keys, err := CreateResellerBalanceCodes(user.Id, 30, 2, "test")
	require.NoError(t, err)
	require.Len(t, keys, 2)
	require.Equal(t, 40, getUserQuotaFromDB(t, user.Id))
	_, err = CreateResellerBalanceCodes(user.Id, 21, 2, "test")
	require.ErrorIs(t, err, ErrResellerInsufficientBalance)
	var row Redemption
	require.NoError(t, DB.Where("user_id = ?", user.Id).First(&row).Error)
	require.NoError(t, RefundResellerCode(user.Id, row.Id))
	require.Equal(t, 70, getUserQuotaFromDB(t, user.Id))
	require.Error(t, RefundResellerCode(user.Id, row.Id))
	require.Equal(t, 70, getUserQuotaFromDB(t, user.Id))
}

func TestResellerCannotRefundUsedCode(t *testing.T) {
	truncateTables(t)
	resetBatchUpdateTestState(t)
	require.NoError(t, DB.AutoMigrate(&Redemption{}, &UserBalancePackage{}))
	user := createReserveTestUser(t, 100)
	t.Cleanup(func() { DB.Unscoped().Where("user_id = ?", user.Id).Delete(&Redemption{}) })
	code := Redemption{UserId: user.Id, Name: "used", Key: "used-" + common.GetRandomString(8), Quota: 20, Status: common.RedemptionCodeStatusUsed, UsedUserId: 1000}
	require.NoError(t, DB.Create(&code).Error)
	require.Error(t, RefundResellerCode(user.Id, code.Id))
	require.Equal(t, 100, getUserQuotaFromDB(t, user.Id))
}

func TestResellerRejectsBatchModeEvenWithRedis(t *testing.T) {
	truncateTables(t)
	resetBatchUpdateTestState(t)
	require.NoError(t, DB.AutoMigrate(&Redemption{}, &UserBalancePackage{}))
	useUserCacheMiniRedis(t)
	common.BatchUpdateEnabled = true
	user := createReserveTestUser(t, 100)
	t.Cleanup(func() { DB.Unscoped().Where("user_id = ?", user.Id).Delete(&Redemption{}) })
	_, err := CreateResellerBalanceCodes(user.Id, 1, 1, "test")
	require.Error(t, err)
	require.Equal(t, 100, getUserQuotaFromDB(t, user.Id))
}

func TestResellerConcurrentCreationDoesNotOverspend(t *testing.T) {
	truncateTables(t)
	resetBatchUpdateTestState(t)
	require.NoError(t, DB.AutoMigrate(&Redemption{}, &UserBalancePackage{}))
	user := createReserveTestUser(t, 100)
	t.Cleanup(func() { DB.Unscoped().Where("user_id = ?", user.Id).Delete(&Redemption{}) })
	var success atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := CreateResellerBalanceCodes(user.Id, 30, 1, "concurrent"); err == nil {
				success.Add(1)
			}
		}()
	}
	wg.Wait()
	require.Equal(t, int32(3), success.Load())
	require.Equal(t, 10, getUserQuotaFromDB(t, user.Id))
	var count int64
	require.NoError(t, DB.Model(&Redemption{}).Where("user_id = ?", user.Id).Count(&count).Error)
	require.Equal(t, int64(3), count)
}

func TestResellerRefundAndRedeemCannotBothSucceed(t *testing.T) {
	truncateTables(t)
	resetBatchUpdateTestState(t)
	require.NoError(t, DB.AutoMigrate(&Redemption{}, &UserBalancePackage{}))
	owner := createReserveTestUser(t, 80)
	recipient := createReserveTestUser(t, 0)
	t.Cleanup(func() { DB.Unscoped().Where("user_id = ?", owner.Id).Delete(&Redemption{}) })
	code := Redemption{UserId: owner.Id, Name: "race", Key: "race-" + common.GetRandomString(12), Quota: 20, Status: common.RedemptionCodeStatusEnabled}
	require.NoError(t, DB.Create(&code).Error)
	start := make(chan struct{})
	var refundErr, redeemErr error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); <-start; refundErr = RefundResellerCode(owner.Id, code.Id) }()
	go func() { defer wg.Done(); <-start; _, redeemErr = Redeem(code.Key, recipient.Id) }()
	close(start)
	wg.Wait()
	require.NotEqual(t, refundErr == nil, redeemErr == nil)
	require.Equal(t, 100, getUserQuotaFromDB(t, owner.Id)+getUserQuotaFromDB(t, recipient.Id))
}

func TestResellerCanRefundLegacyRequestCode(t *testing.T) {
	truncateTables(t)
	resetBatchUpdateTestState(t)
	require.NoError(t, DB.AutoMigrate(&Redemption{}, &UserBalancePackage{}))
	user := createReserveTestUser(t, 100)
	t.Cleanup(func() { DB.Unscoped().Where("user_id = ?", user.Id).Delete(&Redemption{}) })
	code := Redemption{UserId: user.Id, Name: "legacy", Key: "legacy-" + common.GetRandomString(8), Quota: 20, Type: 1, Status: common.RedemptionCodeStatusEnabled}
	require.NoError(t, DB.Create(&code).Error)
	require.NoError(t, RefundResellerCode(user.Id, code.Id))
	var reloaded User
	require.NoError(t, DB.First(&reloaded, user.Id).Error)
	require.Equal(t, 20, reloaded.RequestsBalance)
	require.Equal(t, 100, reloaded.Quota)
}

func TestResellerCannotLaunderModelRestrictedBalance(t *testing.T) {
	truncateTables(t)
	resetBatchUpdateTestState(t)
	require.NoError(t, DB.AutoMigrate(&Redemption{}, &UserBalancePackage{}))
	user := createReserveTestUser(t, 100)
	t.Cleanup(func() { DB.Where("user_id = ?", user.Id).Delete(&UserBalancePackage{}) })
	pkg := UserBalancePackage{UserId: user.Id, Type: 0, RemainingAmount: 50, ModelFilterMode: "whitelist", Models: "only-this-model"}
	require.NoError(t, DB.Create(&pkg).Error)
	_, err := CreateResellerBalanceCodes(user.Id, 25, 1, "test")
	require.Error(t, err)
	require.Equal(t, 100, getUserQuotaFromDB(t, user.Id))
	var count int64
	require.NoError(t, DB.Model(&Redemption{}).Where("user_id = ?", user.Id).Count(&count).Error)
	require.Zero(t, count)
}

func TestResellerFailedIssuanceInvalidatesCache(t *testing.T) {
	truncateTables(t)
	resetBatchUpdateTestState(t)
	require.NoError(t, DB.AutoMigrate(&Redemption{}, &UserBalancePackage{}))
	useUserCacheMiniRedis(t)
	user := createReserveTestUser(t, 100)
	t.Cleanup(func() { DB.Where("user_id = ?", user.Id).Delete(&UserBalancePackage{}) })
	require.NoError(t, populateUserCache(user))
	require.NoError(t, DB.Create(&UserBalancePackage{UserId: user.Id, Type: 0, RemainingAmount: 50, ModelFilterMode: "whitelist"}).Error)
	_, err := CreateResellerBalanceCodes(user.Id, 25, 1, "test")
	require.Error(t, err)
	cached, err := GetUserCache(user.Id)
	require.NoError(t, err)
	require.Equal(t, 100, cached.Quota)
	require.Equal(t, 100, getUserQuotaFromDB(t, user.Id))
}
