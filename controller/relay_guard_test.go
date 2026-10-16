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
	badRequest := types.NewErrorWithStatusCode(errors.New("bad"), types.ErrorCodeBadResponse, http.StatusBadRequest)
	assert.True(t, isUserFacingNoRetryError(badRequest))
	quota := types.NewError(errors.New("quota"), types.ErrorCodeInsufficientUserQuota)
	assert.True(t, isUserFacingNoRetryError(quota))
	upstream := types.NewErrorWithStatusCode(errors.New("boom"), types.ErrorCodeBadResponse, http.StatusBadGateway)
	assert.False(t, isUserFacingNoRetryError(upstream))
}
