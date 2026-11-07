package model

import (
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Run only against disposable databases supplied by TEST_*_DSN.
func TestResellerAndPairingOnConfiguredDatabases(t *testing.T) {
	cases := []struct {
		name, env string
		kind      common.DatabaseType
		open      func(string) gorm.Dialector
	}{
		{"mysql", "TEST_MYSQL_DSN", common.DatabaseTypeMySQL, func(s string) gorm.Dialector { return mysql.Open(s) }},
		{"postgres", "TEST_POSTGRES_DSN", common.DatabaseTypePostgreSQL, func(s string) gorm.Dialector { return postgres.Open(s) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dsn := strings.TrimSpace(os.Getenv(tc.env))
			if dsn == "" {
				t.Skip(tc.env + " not configured")
			}
			db, err := gorm.Open(tc.open(dsn), &gorm.Config{})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(20)
			t.Cleanup(func() { _ = sqlDB.Close() })
			oldDB, oldType := DB, common.MainDatabaseType()
			DB = db
			common.SetMainDatabaseType(tc.kind)
			initCol()
			t.Cleanup(func() { DB = oldDB; common.SetMainDatabaseType(oldType); initCol() })
			resetBatchUpdateTestState(t)
			require.NoError(t, db.AutoMigrate(&User{}, &Redemption{}, &UserBalancePackage{}, &ByogPairingSession{}, &ByogNodeBan{}, &WalletTransferHold{}, &WalletHoldEvent{}, &Task{}, &Midjourney{}, &Token{}, &Channel{}))
			owner := createReserveTestUser(t, 100)
			t.Cleanup(func() {
				db.Unscoped().Where("user_id = ?", owner.Id).Delete(&Redemption{})
				db.Unscoped().Where("id = ?", owner.Id).Delete(&User{})
			})
			var wg sync.WaitGroup
			token := Token{UserId: owner.Id, Key: "db-reserve-" + common.GetRandomString(16), Name: "database-race", Status: common.TokenStatusEnabled, ExpiredTime: -1, RemainQuota: 10}
			require.NoError(t, db.Create(&token).Error)
			t.Cleanup(func() { db.Unscoped().Where("id = ?", token.Id).Delete(&Token{}) })
			var tokenReservations atomic.Int32
			for i := 0; i < 10; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					if reserved, err := TryReserveTokenQuota(token.Id, token.Key, 3, false); err == nil && reserved {
						tokenReservations.Add(1)
					}
				}()
			}
			wg.Wait()
			require.Equal(t, int32(3), tokenReservations.Load())
			require.Equal(t, 1, getTokenFromDB(t, token.Id).RemainQuota)

			byogHold, err := BeginWalletTransferHold(owner.Id)
			require.NoError(t, err)
			t.Cleanup(func() {
				db.Where("hold_id = ?", byogHold).Delete(&WalletHoldEvent{})
				db.Where("id = ?", byogHold).Delete(&WalletTransferHold{})
			})
			ok, err := ReserveByogWalletAndToken(owner.Id, byogHold, token.Id, 4)
			require.NoError(t, err)
			require.False(t, ok)
			require.Equal(t, 100, getUserQuotaFromDB(t, owner.Id))
			require.NoError(t, db.Model(&Token{}).Where("id = ?", token.Id).Update("remain_quota", 5).Error)
			var byogSuccess atomic.Int32
			for i := 0; i < 10; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					if ok, err := ReserveByogWalletAndToken(owner.Id, byogHold, token.Id, 4); err == nil && ok {
						byogSuccess.Add(1)
					}
				}()
			}
			wg.Wait()
			require.Equal(t, int32(10), byogSuccess.Load())
			require.Equal(t, 96, getUserQuotaFromDB(t, owner.Id))
			require.Equal(t, 1, getTokenFromDB(t, token.Id).RemainQuota)
			byogSuccess.Store(0)
			for i := 0; i < 10; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					if FinalizeByogWalletAndToken(owner.Id, byogHold, token.Id, 4, 2) == nil {
						byogSuccess.Add(1)
					}
				}()
			}
			wg.Wait()
			require.Equal(t, int32(10), byogSuccess.Load())
			require.Equal(t, 98, getUserQuotaFromDB(t, owner.Id))
			require.Equal(t, 3, getTokenFromDB(t, token.Id).RemainQuota)
			require.ErrorIs(t, FinalizeByogWalletAndToken(owner.Id, byogHold, token.Id, 4, 3), ErrWalletHoldConflict)
			require.NoError(t, ResolveWalletTransferHold(owner.Id, byogHold, 0))
			require.NoError(t, db.Model(&User{}).Where("id = ?", owner.Id).Update("quota", 100).Error)
			require.NoError(t, db.Model(&Token{}).Where("id = ?", token.Id).Updates(map[string]interface{}{"remain_quota": 1, "used_quota": 9}).Error)

			mjHold, err := BeginWalletTransferHold(owner.Id)
			require.NoError(t, err)
			t.Cleanup(func() {
				db.Where("hold_id = ?", mjHold).Delete(&WalletHoldEvent{})
				db.Where("id = ?", mjHold).Delete(&WalletTransferHold{})
				db.Where("wallet_hold_id = ?", mjHold).Delete(&Midjourney{})
			})
			require.NoError(t, db.Model(&Token{}).Where("id = ?", token.Id).Updates(map[string]interface{}{"remain_quota": 100, "used_quota": 0}).Error)
			ok, err = ReserveMidjourneyWalletAndToken(owner.Id, mjHold, token.Id, 30)
			require.NoError(t, err)
			require.True(t, ok)
			mjTask := Midjourney{UserId: owner.Id, WalletHoldID: mjHold, Quota: 30, BillingPending: true}
			require.NoError(t, db.Create(&mjTask).Error)
			var mjDecisions, mjDecisionErrors atomic.Int32
			for i := 0; i < 10; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					fresh, err := FinalizeMidjourneyReservedTask(owner.Id, mjHold, mjTask.Id, token.Id, 30)
					if err != nil {
						mjDecisionErrors.Add(1)
					}
					if fresh {
						mjDecisions.Add(1)
					}
				}()
			}
			wg.Wait()
			require.Zero(t, mjDecisionErrors.Load())
			require.Equal(t, int32(1), mjDecisions.Load())
			require.Equal(t, 70, getUserQuotaFromDB(t, owner.Id))
			require.Equal(t, 70, getTokenFromDB(t, token.Id).RemainQuota)
			var chargedUser User
			require.NoError(t, db.First(&chargedUser, owner.Id).Error)
			require.Equal(t, 30, chargedUser.UsedQuota)
			require.Equal(t, 1, chargedUser.RequestCount)
			require.ErrorIs(t, ReleaseMidjourneyWalletAndToken(owner.Id, mjHold, token.Id, 30), ErrWalletHoldConflict)
			require.NoError(t, ResolveWalletTransferHold(owner.Id, mjHold, 0))
			require.NoError(t, db.Model(&User{}).Where("id = ?", owner.Id).Updates(map[string]interface{}{"quota": 100, "used_quota": 0, "request_count": 0}).Error)
			require.NoError(t, db.Model(&Token{}).Where("id = ?", token.Id).Updates(map[string]interface{}{"remain_quota": 1, "used_quota": 9}).Error)

			var issued atomic.Int32
			for i := 0; i < 10; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					if _, err := CreateResellerBalanceCodes(owner.Id, 30, 1, "race"); err == nil {
						issued.Add(1)
					}
				}()
			}
			wg.Wait()
			require.Equal(t, int32(3), issued.Load())
			require.Equal(t, 10, getUserQuotaFromDB(t, owner.Id))
			var code Redemption
			require.NoError(t, db.Where("user_id = ?", owner.Id).First(&code).Error)
			var refunded atomic.Int32
			for i := 0; i < 10; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					if RefundResellerCode(owner.Id, code.Id) == nil {
						refunded.Add(1)
					}
				}()
			}
			wg.Wait()
			require.Equal(t, int32(1), refunded.Load())
			require.Equal(t, 40, getUserQuotaFromDB(t, owner.Id))

			holdID, err := BeginWalletTransferHold(owner.Id)
			require.NoError(t, err)
			t.Cleanup(func() { db.Where("user_id = ?", owner.Id).Delete(&WalletTransferHold{}) })
			_, err = CreateResellerBalanceCodes(owner.Id, 10, 1, "blocked-by-hold")
			require.Error(t, err)
			var settled atomic.Int32
			for i := 0; i < 10; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					if ResolveWalletTransferHold(owner.Id, holdID, -5) == nil {
						settled.Add(1)
					}
				}()
			}
			wg.Wait()
			require.Equal(t, int32(10), settled.Load())
			require.Equal(t, 35, getUserQuotaFromDB(t, owner.Id))
			require.ErrorIs(t, ResolveWalletTransferHold(owner.Id, holdID, -6), ErrWalletHoldConflict)

			journalHold, err := BeginWalletTransferHold(owner.Id)
			require.NoError(t, err)
			var eventSuccess atomic.Int32
			for i := 0; i < 10; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					if ok, err := ReserveWalletHoldQuota(owner.Id, journalHold, "precharge", 5); err == nil && ok {
						eventSuccess.Add(1)
					}
				}()
			}
			wg.Wait()
			require.Equal(t, int32(10), eventSuccess.Load())
			require.Equal(t, 30, getUserQuotaFromDB(t, owner.Id))
			require.NoError(t, ApplyWalletHoldDelta(owner.Id, journalHold, "refund", 5))
			require.NoError(t, ApplyWalletHoldDelta(owner.Id, journalHold, "refund", 5))
			require.Equal(t, 35, getUserQuotaFromDB(t, owner.Id))
			require.NoError(t, ResolveWalletTransferHold(owner.Id, journalHold, 0))
			t.Cleanup(func() { db.Where("user_id = ?", owner.Id).Delete(&WalletHoldEvent{}) })

			mj := Midjourney{UserId: owner.Id, MjId: "database-refund-test", Quota: 10, Status: "FAILURE"}
			require.NoError(t, db.Create(&mj).Error)
			t.Cleanup(func() { db.Where("id = ?", mj.Id).Delete(&Midjourney{}) })
			var mjRefund atomic.Int32
			var mjErrors atomic.Int32
			for i := 0; i < 10; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					amount, err := RefundMidjourneyWalletOnce(owner.Id, mj.Id)
					if err != nil {
						mjErrors.Add(1)
					} else {
						mjRefund.Add(int32(amount))
					}
				}()
			}
			wg.Wait()
			require.Zero(t, mjErrors.Load())
			require.Equal(t, int32(10), mjRefund.Load())
			require.Equal(t, 45, getUserQuotaFromDB(t, owner.Id))
			require.NoError(t, db.Model(&User{}).Where("id = ?", owner.Id).Updates(map[string]interface{}{"quota": 35, "used_quota": 10}).Error)
			require.NoError(t, db.Model(&Token{}).Where("id = ?", token.Id).Update("used_quota", 10).Error)
			channel := Channel{Name: "midjourney-refund-race", UsedQuota: 10}
			require.NoError(t, db.Create(&channel).Error)
			t.Cleanup(func() { db.Unscoped().Where("id = ?", channel.Id).Delete(&Channel{}) })
			mjFull := Midjourney{UserId: owner.Id, MjId: "database-full-refund-test", Quota: 10, TokenId: token.Id, ChannelId: channel.Id, Status: "FAILURE", Progress: "100%", RefundPending: true}
			require.NoError(t, db.Create(&mjFull).Error)
			t.Cleanup(func() { db.Where("id = ?", mjFull.Id).Delete(&Midjourney{}) })
			var fullRefund, fullErrors atomic.Int32
			for i := 0; i < 10; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					amount, err := RefundMidjourneyAccountingOnce(owner.Id, mjFull.Id)
					if err != nil {
						fullErrors.Add(1)
					} else {
						fullRefund.Add(int32(amount))
					}
				}()
			}
			wg.Wait()
			require.Zero(t, fullErrors.Load())
			require.Equal(t, int32(10), fullRefund.Load())
			require.Equal(t, 45, getUserQuotaFromDB(t, owner.Id))
			var refundedUser User
			require.NoError(t, db.First(&refundedUser, owner.Id).Error)
			require.Zero(t, refundedUser.UsedQuota)
			refundedToken := getTokenFromDB(t, token.Id)
			require.Equal(t, 11, refundedToken.RemainQuota)
			require.Zero(t, refundedToken.UsedQuota)
			var refundedChannel Channel
			require.NoError(t, db.First(&refundedChannel, channel.Id).Error)
			require.Zero(t, refundedChannel.UsedQuota)
			var refundedTask Midjourney
			require.NoError(t, db.First(&refundedTask, mjFull.Id).Error)
			require.Zero(t, refundedTask.Quota)
			require.False(t, refundedTask.RefundPending)
			require.NoError(t, db.Model(&User{}).Where("id = ?", owner.Id).Update("quota", 35).Error)

			// Hold insertion and issuance must serialize across DB connections,
			// including the interval before the hold transaction commits.
			tx := db.Begin()
			require.NoError(t, tx.Error)
			var locked User
			require.NoError(t, lockForUpdate(tx).Select("id").Where("id = ?", owner.Id).First(&locked).Error)
			manual := WalletTransferHold{ID: "manual" + common.GetRandomString(26), UserID: owner.Id, State: "open", CreatedAt: time.Now().Unix()}
			require.NoError(t, tx.Create(&manual).Error)
			startIssue := make(chan struct{})
			issueResult := make(chan error, 1)
			go func() {
				close(startIssue)
				_, issueErr := CreateResellerBalanceCodes(owner.Id, 10, 1, "race-with-uncommitted-hold")
				issueResult <- issueErr
			}()
			<-startIssue
			early := false
			var issueErr error
			select {
			case issueErr = <-issueResult:
				early = true
			case <-time.After(100 * time.Millisecond):
			}
			commitErr := tx.Commit().Error
			if !early {
				issueErr = <-issueResult
			}
			require.NoError(t, commitErr)
			require.False(t, early, "issuance must wait for the in-progress user-row transaction")
			require.Error(t, issueErr, "the committed open hold must prevent issuance")
			require.Equal(t, 35, getUserQuotaFromDB(t, owner.Id))
			require.NoError(t, ResolveWalletTransferHold(owner.Id, manual.ID, 0))

			// Redeeming and refunding the same code must never both credit a wallet.
			recipient := createReserveTestUser(t, 0)
			t.Cleanup(func() { db.Unscoped().Where("id = ?", recipient.Id).Delete(&User{}) })
			keys, err := CreateResellerBalanceCodes(owner.Id, 20, 1, "refund-redeem-race")
			require.NoError(t, err)
			var found []Redemption
			require.NoError(t, QueryResellerRedemptions(owner.Id, keys[0]).Find(&found).Error)
			require.Len(t, found, 1)
			require.NoError(t, QueryResellerRedemptions(recipient.Id, keys[0]).Find(&found).Error)
			require.Empty(t, found, "a reseller must not search another reseller's codes")
			var raceCode Redemption
			require.NoError(t, db.Where("user_id = ? AND name = ?", owner.Id, "refund-redeem-race").First(&raceCode).Error)
			start := make(chan struct{})
			var refundErr, redeemErr error
			wg.Add(2)
			go func() { defer wg.Done(); <-start; refundErr = RefundResellerCode(owner.Id, raceCode.Id) }()
			go func() { defer wg.Done(); <-start; _, redeemErr = Redeem(keys[0], recipient.Id) }()
			close(start)
			wg.Wait()
			require.NotEqual(t, refundErr == nil, redeemErr == nil)
			require.Equal(t, 35, getUserQuotaFromDB(t, owner.Id)+getUserQuotaFromDB(t, recipient.Id))

			// Failed credits cannot burn issued codes, including on SQL dialects
			// that enforce 32-bit integer overflow differently.
			overflowCode := Redemption{UserId: owner.Id, Name: "overflow", Key: "overflow-" + common.GetRandomString(16), Quota: 20, Status: common.RedemptionCodeStatusEnabled, ModelFilterMode: "whitelist", Models: "model-a"}
			require.NoError(t, db.Create(&overflowCode).Error)
			t.Cleanup(func() { db.Where("redemption_id = ?", overflowCode.Id).Delete(&UserBalancePackage{}) })
			require.NoError(t, db.Model(&User{}).Where("id = ?", recipient.Id).Update("quota", common.MaxQuota-10).Error)
			_, err = Redeem(overflowCode.Key, recipient.Id)
			require.Error(t, err)
			var unchanged Redemption
			require.NoError(t, db.First(&unchanged, overflowCode.Id).Error)
			require.Equal(t, common.RedemptionCodeStatusEnabled, unchanged.Status)
			var packages int64
			require.NoError(t, db.Model(&UserBalancePackage{}).Where("redemption_id = ?", overflowCode.Id).Count(&packages).Error)
			require.Zero(t, packages)
			require.NoError(t, db.Model(&User{}).Where("id = ?", recipient.Id).Update("quota", common.MaxQuota-20).Error)
			_, err = Redeem(overflowCode.Key, recipient.Id)
			require.NoError(t, err)
			require.Equal(t, common.MaxQuota, getUserQuotaFromDB(t, recipient.Id))

			// Account and hardware bans must both match on every SQL dialect.
			ban := ByogNodeBan{NodeId: "integration-banned-node", UserId: owner.Id, HWID: "integration-banned-hardware", BanReason: "test", ViolationCode: "test", IsActive: true}
			require.NoError(t, db.Create(&ban).Error)
			t.Cleanup(func() { db.Where("id = ?", ban.Id).Delete(&ByogNodeBan{}) })
			blocked, err := CheckByogBan(owner.Id, "different-hardware")
			require.NoError(t, err)
			require.True(t, blocked)
			blocked, err = CheckByogBan(recipient.Id, ban.HWID)
			require.NoError(t, err)
			require.True(t, blocked)
			blocked, err = CheckByogBan(recipient.Id, "different-hardware")
			require.NoError(t, err)
			require.False(t, blocked)
			require.NoError(t, db.Model(&ban).Update("is_active", false).Error)
			blocked, err = CheckByogBan(owner.Id, ban.HWID)
			require.NoError(t, err)
			require.False(t, blocked)

			pairing := "test-" + common.GetRandomString(16)
			require.NoError(t, CreateByogPairingSession(pairing, owner.Id, time.Now().Unix()+900))
			var claimed atomic.Int32
			for i := 0; i < 10; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					if id, err := ConsumeByogPairingSession(pairing); err == nil && id == owner.Id {
						claimed.Add(1)
					}
				}()
			}
			wg.Wait()
			require.Equal(t, int32(1), claimed.Load())

			// Concurrent recovery of the same held task must refund wallet,
			// token and counters once and close its hold atomically.
			refundUser := createReserveTestUser(t, 70)
			refundToken := Token{UserId: refundUser.Id, Key: "task-refund-" + common.GetRandomString(12), Status: common.TokenStatusEnabled, ExpiredTime: -1, RemainQuota: 70, UsedQuota: 30}
			require.NoError(t, db.Create(&refundToken).Error)
			refundChannel := Channel{Name: "task-refund-channel", Key: "test", UsedQuota: 30}
			require.NoError(t, db.Create(&refundChannel).Error)
			require.NoError(t, db.Model(&User{}).Where("id = ?", refundUser.Id).Update("used_quota", 30).Error)
			refundHold, err := BeginWalletTransferHold(refundUser.Id)
			require.NoError(t, err)
			refundTask := Task{TaskID: "held-refund-" + common.GetRandomString(12), UserId: refundUser.Id, ChannelId: refundChannel.Id, Quota: 30, Status: TaskStatusFailure, RefundPending: true, PrivateData: TaskPrivateData{WalletHoldID: refundHold, BillingSource: "wallet", TokenId: refundToken.Id}}
			require.NoError(t, db.Create(&refundTask).Error)
			t.Cleanup(func() {
				db.Where("hold_id = ?", refundHold).Delete(&WalletHoldEvent{})
				db.Where("id = ?", refundHold).Delete(&WalletTransferHold{})
				db.Where("id = ?", refundTask.ID).Delete(&Task{})
				db.Unscoped().Where("id = ?", refundToken.Id).Delete(&Token{})
				db.Unscoped().Where("id = ?", refundChannel.Id).Delete(&Channel{})
				db.Unscoped().Where("id = ?", refundUser.Id).Delete(&User{})
			})
			require.Error(t, ResolveWalletTransferHold(refundUser.Id, refundHold, 0))
			var taskRefunded, taskRefundErrors atomic.Int32
			for i := 0; i < 10; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					amount, err := RefundHeldTaskWalletOnce(refundUser.Id, refundTask.ID, refundHold)
					if err != nil {
						taskRefundErrors.Add(1)
					}
					if amount == 30 {
						taskRefunded.Add(1)
					}
				}()
			}
			wg.Wait()
			require.Zero(t, taskRefundErrors.Load())
			require.Equal(t, int32(1), taskRefunded.Load())
			require.Equal(t, 100, getUserQuotaFromDB(t, refundUser.Id))
			updatedToken := getTokenFromDB(t, refundToken.Id)
			require.Equal(t, 100, updatedToken.RemainQuota)
			require.Zero(t, updatedToken.UsedQuota)
			var updatedUser User
			require.NoError(t, db.First(&updatedUser, refundUser.Id).Error)
			require.Zero(t, updatedUser.UsedQuota)
			var updatedChannel Channel
			require.NoError(t, db.First(&updatedChannel, refundChannel.Id).Error)
			require.Zero(t, updatedChannel.UsedQuota)
			var refundTaskAfter Task
			require.NoError(t, db.First(&refundTaskAfter, refundTask.ID).Error)
			require.False(t, refundTaskAfter.RefundPending)
			require.Zero(t, refundTaskAfter.Quota)

			// Closing the hold and recording a new refund intent serialize even
			// when separate coordinators race on the same user.
			raceHold, err := BeginWalletTransferHold(refundUser.Id)
			require.NoError(t, err)
			raceTask := Task{TaskID: "intent-race-" + common.GetRandomString(12), UserId: refundUser.Id, Quota: 30, Status: TaskStatusInProgress, PrivateData: TaskPrivateData{WalletHoldID: raceHold, BillingSource: "wallet"}}
			require.NoError(t, db.Create(&raceTask).Error)
			t.Cleanup(func() {
				db.Where("id = ?", raceTask.ID).Delete(&Task{})
				db.Where("id = ?", raceHold).Delete(&WalletTransferHold{})
			})
			raceTask.Status = TaskStatusFailure
			raceTask.RefundPending = true
			raceStart := make(chan struct{})
			var intentWon bool
			var intentErr, resolveErr error
			wg.Add(2)
			go func() {
				defer wg.Done()
				<-raceStart
				intentWon, intentErr = raceTask.UpdateWithStatus(TaskStatusInProgress)
			}()
			go func() {
				defer wg.Done()
				<-raceStart
				resolveErr = ResolveWalletTransferHold(refundUser.Id, raceHold, 0)
			}()
			close(raceStart)
			wg.Wait()
			require.NotEqual(t, intentErr == nil && intentWon, resolveErr == nil)
			var intentAfter Task
			var holdAfter WalletTransferHold
			require.NoError(t, db.First(&intentAfter, raceTask.ID).Error)
			require.NoError(t, db.First(&holdAfter, "id = ?", raceHold).Error)
			require.False(t, intentAfter.RefundPending && holdAfter.State == "resolved")
			var resolved WalletTransferHold
			require.NoError(t, db.Where("id = ?", refundHold).First(&resolved).Error)
			require.Equal(t, "resolved", resolved.State)
		})
	}
}
