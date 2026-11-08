package controller

import (
	"errors"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/assert"
)

func TestIsUserFacingNoRetryError(t *testing.T) {
	assert.False(t, isUserFacingNoRetryError(nil))
	badRequest := types.NewError(errors.New("bad body"), types.ErrorCodeBadRequestBody)
	assert.True(t, isUserFacingNoRetryError(badRequest))
	quota := types.NewError(errors.New("quota"), types.ErrorCodeInsufficientUserQuota)
	assert.True(t, isUserFacingNoRetryError(quota))
	upstream400 := types.NewErrorWithStatusCode(errors.New("prompt too long"), types.ErrorCodeBadResponse, http.StatusBadRequest)
	assert.False(t, isUserFacingNoRetryError(upstream400), "upstream bad response should allow retry/fallback")
	upstream500 := types.NewErrorWithStatusCode(errors.New("boom"), types.ErrorCodeBadResponse, http.StatusBadGateway)
	assert.False(t, isUserFacingNoRetryError(upstream500))
}
