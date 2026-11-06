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
			require.NoError(t, db.AutoMigrate(&User{}, &Redemption{}, &UserBalancePackage{}, &ByogPairingSession{}))
			owner := createReserveTestUser(t, 100)
			t.Cleanup(func() {
				db.Unscoped().Where("user_id = ?", owner.Id).Delete(&Redemption{})
				db.Unscoped().Where("id = ?", owner.Id).Delete(&User{})
			})
			var issued atomic.Int32
			var wg sync.WaitGroup
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
			require.Equal(t, 40, getUserQuotaFromDB(t, owner.Id)+getUserQuotaFromDB(t, recipient.Id))

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
		})
	}
}
