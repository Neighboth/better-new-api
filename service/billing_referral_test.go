package service

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBillingSession_TryWalletWithReferralOnly(t *testing.T) {
	require.NotNil(t, model.DB)
	require.NoError(t, model.DB.AutoMigrate(&model.UserBalancePackage{}))

	user := &model.User{
		Username:      "ref-only-user",
		Password:      "hashed-pw",
		DisplayName:   "Referral Only",
		Status:        1,
		Quota:         0,
		ReferralQuota: 500000,
		Group:         "default",
	}
	require.NoError(t, model.DB.Create(user).Error)
	t.Cleanup(func() {
		model.DB.Where("id = ?", user.Id).Delete(&model.User{})
	})

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)

	relayInfo := &relaycommon.RelayInfo{
		UserId:          user.Id,
		OriginModelName: "gpt-4o-mini",
		TokenGroup:      "default",
		IsPlayground:    true,
		UserSetting: dto.UserSetting{
			BillingPriority: []string{BillingSourceWallet},
		},
	}

	session, apiErr := NewBillingSession(c, relayInfo, 1000)
	require.Nil(t, apiErr)
	require.NotNil(t, session)
	assert.Equal(t, BillingSourceWallet, session.funding.Source())

	var updated model.User
	require.NoError(t, model.DB.First(&updated, user.Id).Error)
	assert.Equal(t, int64(499000), updated.ReferralQuota)
	assert.Equal(t, 0, updated.Quota)

	require.NoError(t, session.Settle(250))
	require.NoError(t, model.DB.First(&updated, user.Id).Error)
	assert.Equal(t, int64(499750), updated.ReferralQuota)
	assert.Equal(t, 0, updated.Quota)
}
