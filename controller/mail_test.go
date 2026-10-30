package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/service/mail"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestContext(method string, target string, body string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	context, _ := gin.CreateTestContext(recorder)
	context.Request = request
	return context, recorder
}

// The link of the message has to work without a session: the reader opens it
// from the mailbox and the provider posts the one click request of RFC 8058
// without a cookie. A request without a valid token is refused, the settings of
// a user are not changed by a link that somebody guessed.
func TestUnsubscribeRefusesAnInvalidToken(t *testing.T) {
	context, recorder := newTestContext(http.MethodGet, "/api/user/unsubscribe?token=not-a-token", "")
	Unsubscribe(context)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "not valid")
}

// A page of the one click POST must not be answered with a page: the provider
// reads the status of the answer.
func TestUnsubscribeAnswersOneClickPostWithoutAPage(t *testing.T) {
	context, recorder := newTestContext(
		http.MethodPost,
		"/api/user/unsubscribe?token=broken",
		"List-Unsubscribe=One-Click",
	)
	Unsubscribe(context)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Header().Get("Content-Type"), "application/json")
}

func TestUnsubscribeTokenOfAnotherSecretIsRefused(t *testing.T) {
	token := mail.UnsubscribeToken(12)
	require.NotEmpty(t, token)
	context, recorder := newTestContext(http.MethodGet, "/api/user/unsubscribe?token="+token+"-x", "")
	Unsubscribe(context)
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestParseIdsIgnoresGarbage(t *testing.T) {
	assert.Equal(t, []int{3, 8}, parseIds("3, 8"))
	assert.Empty(t, parseIds("a,b"))
	assert.Empty(t, parseIds(""))
	assert.Equal(t, []int{7}, normalizeIds([]int{7, 7, 0, -2}))
}
