package deepseekweb

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const responseStreamFixture = `data: {"v":{"response":{"fragments":[{"id":1,"type":"RESPONSE","content":"E2E"}]}}}` + "\n" +
	`data: {"v":"-OK"}` + "\n" +
	`data: {"p":"response","o":"BATCH","v":[{"p":"accumulated_token_usage","v":42},{"p":"quasi_status","v":"FINISHED"}]}` + "\n" +
	`data: {"p":"response/status","o":"SET","v":"FINISHED"}` + "\n"

func newResponseContext(t *testing.T, clientStream bool) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	common.SetContextKey(c, constant.ContextKeyIsStream, clientStream)
	return c, recorder
}

func deepSeekWebResponse() *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(responseStreamFixture)),
	}
}

// TestDoResponseReturnsJSONForNonStreamingCaller guards the behaviour the relay
// cannot fix on its own: because DeepSeek Web always answers with
// text/event-stream, the framework promotes info.IsStream to true, so the
// adaptor has to consult the caller's own intent to still emit one JSON body.
func TestDoResponseReturnsJSONForNonStreamingCaller(t *testing.T) {
	c, recorder := newResponseContext(t, false)
	info := &relaycommon.RelayInfo{OriginModelName: "deepseek-chat"}

	usage, apiErr := (&Adaptor{}).DoResponse(c, deepSeekWebResponse(), info)
	require.Nil(t, apiErr)

	// The relay asserts the returned usage to *dto.Usage; a value type panics.
	parsed, ok := usage.(*dto.Usage)
	require.True(t, ok, "usage must be a *dto.Usage")
	assert.Positive(t, parsed.TotalTokens)

	body := recorder.Body.String()
	assert.False(t, strings.HasPrefix(strings.TrimSpace(body), "data:"), "a non-streaming caller must not receive SSE")
	assert.Contains(t, body, `"object":"chat.completion"`)
	assert.Contains(t, body, "E2E-OK")
	assert.NotContains(t, body, "chat.completion.chunk")
}

func TestDoResponseStreamsForStreamingCaller(t *testing.T) {
	c, recorder := newResponseContext(t, true)
	info := &relaycommon.RelayInfo{OriginModelName: "deepseek-chat"}

	usage, apiErr := (&Adaptor{}).DoResponse(c, deepSeekWebResponse(), info)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)

	body := recorder.Body.String()
	assert.Contains(t, body, "chat.completion.chunk")
	assert.Contains(t, body, "data: [DONE]")
	assert.Contains(t, body, "E2E")
	assert.Contains(t, body, "-OK")
}

func TestDoResponseReportsEmptyUpstreamAsError(t *testing.T) {
	c, _ := newResponseContext(t, false)
	info := &relaycommon.RelayInfo{OriginModelName: "deepseek-chat"}
	empty := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`data: {"p":"response/status","o":"SET","v":"FINISHED"}` + "\n")),
	}

	_, apiErr := (&Adaptor{}).DoResponse(c, empty, info)
	require.NotNil(t, apiErr, "an empty upstream answer must be reported as an error")
}
