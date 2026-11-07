package byog

import (
	"math"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestByogPriceBoundsAndRounding(t *testing.T) {
	for _, price := range []byogPrice{{value: 0.3}, {fixed: true, value: 123.4}, {value: 0}} {
		reserve, err := price.quota(100, 200, true)
		require.NoError(t, err)
		for _, usage := range [][2]int{{0, 0}, {1, 1}, {100, 200}, {50, 20}} {
			actual, err := price.quota(usage[0], usage[1], false)
			require.NoError(t, err)
			require.LessOrEqual(t, actual, reserve)
		}
	}
	_, err := (byogPrice{value: math.Inf(1)}).quota(1, 1, true)
	require.Error(t, err)
	_, err = (byogPrice{value: float64(common.MaxQuota)}).quota(1, 1, true)
	require.Error(t, err)
	_, err = (byogPrice{value: 1}).quota(-1, 1, true)
	require.Error(t, err)
}

func TestByogAuthorizationRejectsUnboundedRequest(t *testing.T) {
	old := model.DB
	model.DB = nil
	t.Cleanup(func() { model.DB = old })
	def := &model.ByogModel{ModelId: "unknown-byog-model", PricingTier: 2}
	payload := &InferencePayload{MaxTokens: -1, Prompt: "hi"}
	authorization, err := newByogAuthorization(def, payload)
	require.NoError(t, err)
	require.Equal(t, byogDefaultOutputLimit, payload.MaxTokens)
	require.Equal(t, byogDefaultOutputLimit, authorization.outputLimit)
	require.Greater(t, authorization.reserved, 0)
	_, err = newByogAuthorization(def, &InferencePayload{MaxTokens: byogMaxOutputLimit + 1})
	require.Error(t, err)
	_, err = newByogAuthorization(def, &InferencePayload{MaxTokens: 1, Prompt: strings.Repeat("x", byogMaxRequestBytes+1)})
	require.Error(t, err)
}
