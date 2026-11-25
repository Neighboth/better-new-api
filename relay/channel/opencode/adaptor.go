// Portions adapted from github.com/jasonxu114514/opencode2api (GPL-3.0) by
// jasonxu114514. See THIRD-PARTY-LICENSES.md.
package opencode

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
)

const (
	opencodeAnonymousBaseURL = "https://opencode.ai/inference/openai/v1/chat/completions"
	opencodeCLIUserAgent     = "opencode/2.0.0"
	opencodeBase62Alphabet   = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
)

// Adaptor serves the OpenCode (Anon) channel. The upstream free tier only
// accepts requests that match the official OpenCode CLI: valid /v1 endpoint,
// authentic ses_/msg_ identifier format, exact CLI headers, official system
// prompt, renamed client tools, and official agent toolset.
type Adaptor struct {
	openai.Adaptor
}

const renamesContextKey = "opencode_tool_renames"

var (
	sessionMu    sync.Mutex
	channelSesID = map[int]string{}
	idCounter    uint64
)

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
	a.Adaptor.Init(info)
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	// Honor custom base_url when configured, while normalizing legacy URLs
	// such as "https://opencode.ai/inference/openai" (missing /v1) that
	// otherwise produce upstream 404 errors.
	base := strings.TrimRight(strings.TrimSpace(info.ChannelBaseUrl), "/")
	if base == "" {
		return opencodeAnonymousBaseURL, nil
	}
	if strings.HasSuffix(base, "/chat/completions") {
		if strings.HasSuffix(base, "/inference/openai/chat/completions") {
			return strings.TrimSuffix(base, "/chat/completions") + "/v1/chat/completions", nil
		}
		if strings.HasSuffix(base, "/zen/chat/completions") {
			return strings.TrimSuffix(base, "/chat/completions") + "/v1/chat/completions", nil
		}
		return base, nil
	}
	if strings.HasSuffix(base, "/v1") {
		return base + "/chat/completions", nil
	}
	return base + "/v1/chat/completions", nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, header *http.Header, info *relaycommon.RelayInfo) error {
	header.Set("Content-Type", "application/json")
	if info.IsStream {
		header.Set("Accept", "text/event-stream")
	} else {
		header.Set("Accept", "application/json")
	}
	key := strings.TrimSpace(info.ApiKey)
	if key == "" || key == "default" || key == "local" {
		key = "public"
	}
	header.Set("Authorization", "Bearer "+key)
	// Match official OpenCode CLI headers: versioned User-Agent, cli client
	// tag, descending ses_ session ID, and per-request ascending msg_ ID.
	header.Set("User-Agent", opencodeCLIUserAgent)
	header.Set("x-opencode-client", "cli")
	sessionID := getOrCreateOpenCodeSessionID(info.ChannelId)
	requestID := generateOpenCodeID("msg", false)
	header.Set("x-opencode-session", sessionID)
	header.Set("x-opencode-request", requestID)
	header.Set("x-session-affinity", sessionID)
	return nil
}

// RefreshRequestHeaders rotates the session & request IDs on an outgoing
// OpenCode request after an IP/proxy switch (e.g. Psiphon IP rotation on 429).
func RefreshRequestHeaders(req *http.Request, channelID int) {
	if req == nil {
		return
	}
	sessionID := RotateOpenCodeSessionID(channelID)
	requestID := generateOpenCodeID("msg", false)
	req.Header.Set("x-opencode-session", sessionID)
	req.Header.Set("x-opencode-request", requestID)
	req.Header.Set("x-session-affinity", sessionID)
}

// RotateOpenCodeSessionID mints a fresh descending ses_ identifier for the channel.
func RotateOpenCodeSessionID(channelID int) string {
	id := generateOpenCodeID("ses", true)
	sessionMu.Lock()
	channelSesID[channelID] = id
	sessionMu.Unlock()
	return id
}

func getOrCreateOpenCodeSessionID(channelID int) string {
	sessionMu.Lock()
	defer sessionMu.Unlock()
	if existing, ok := channelSesID[channelID]; ok && existing != "" {
		return existing
	}
	id := generateOpenCodeID("ses", true)
	channelSesID[channelID] = id
	return id
}

// generateOpenCodeID produces the exact identifier format minted by OpenCode's
// Identifier module: "<prefix>_<12-hex-timestamp><14-base62-random>" (30 chars).
// Session IDs use descending timestamps; message/request IDs use ascending.
func generateOpenCodeID(prefix string, descending bool) string {
	seq := atomic.AddUint64(&idCounter, 1) & 0xfff

	ts := (uint64(time.Now().UnixMilli()) << 12) | seq
	ts &= 0xffffffffffff
	if descending {
		ts = (^ts) & 0xffffffffffff
	}
	randBytes := make([]byte, 14)
	_, _ = rand.Read(randBytes)
	var sb strings.Builder
	sb.Grow(len(prefix) + 1 + 12 + 14)
	sb.WriteString(prefix)
	sb.WriteByte('_')
	sb.WriteString(fmt.Sprintf("%012x", ts))
	for _, b := range randBytes {
		sb.WriteByte(opencodeBase62Alphabet[int(b)%len(opencodeBase62Alphabet)])
	}
	return sb.String()
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	converted, err := a.Adaptor.ConvertOpenAIRequest(c, info, request)
	if err != nil {
		return nil, err
	}
	// The shared helper makes sure request rewriting happens exactly once
	// and the upstream body stays self-consistent.
	raw, err := common.Marshal(converted)
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := common.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	renames := ShapeChatRequest(payload)
	if renames != nil && !renames.empty() {
		c.Set(renamesContextKey, renames)
	}
	return payload, nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

// DoResponse wraps the OpenAI response handlers, restoring wire tool names.
func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	renames := renamesFromContext(c)
	if info.IsStream {
		resp = wrapStreamWithRestore(resp, renames)
	} else {
		data, errRead := io.ReadAll(resp.Body)
		if errRead == nil {
			_ = resp.Body.Close()
			restored := RestoreChatResponse(data, renames)
			resp.Body = io.NopCloser(bytes.NewReader(restored))
			resp.ContentLength = int64(len(restored))
			resp.Header.Set("Content-Length", fmt.Sprint(len(restored)))
		}
	}
	return a.Adaptor.DoResponse(c, resp, info)
}

func renamesFromContext(c *gin.Context) *toolRenames {
	if c == nil {
		return nil
	}
	value, exists := c.Get(renamesContextKey)
	if !exists {
		return nil
	}
	renames, ok := value.(*toolRenames)
	if !ok {
		return nil
	}
	return renames
}

// wrapStreamWithRestore splices a restore-only goroutine into the SSE pipe so
// the client sees its own tool names while everything else passes through.
func wrapStreamWithRestore(resp *http.Response, renames *toolRenames) *http.Response {
	if resp == nil || resp.Body == nil || renames == nil || renames.empty() {
		return resp
	}
	reader, writer := io.Pipe()
	go func() {
		defer resp.Body.Close()
		defer writer.Close()
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		for scanner.Scan() {
			line := scanner.Bytes()
			out := RestoreChatStreamLine(line, renames)
			if _, err := writer.Write(out); err != nil {
				return
			}
			if len(out) == 0 || out[len(out)-1] != '\n' {
				if _, err := writer.Write([]byte("\n")); err != nil {
					return
				}
			}
		}
	}()
	wrapped := *resp
	wrapped.Body = reader
	wrapped.ContentLength = -1
	wrapped.Header.Del("Content-Length")
	return &wrapped
}
