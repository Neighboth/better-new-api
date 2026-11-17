// Portions adapted from github.com/jasonxu114514/opencode2api (GPL-3.0) by
// jasonxu114514. See THIRD-PARTY-LICENSES.md.
package opencode

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
)

const opencodeAnonymousBaseURL = "https://opencode.ai/inference/openai/v1/chat/completions"

// Adaptor serves the OpenCode (Anon) channel. The upstream free tier only
// accepts requests that look like the official OpenCode CLI, so every request
// is re-shaped: the official system prompt is injected, client tool names are
// renamed to opaque digests, and the official agent tool set is appended. The
// channel works with no key at all — the anonymous `public` credential is
// used; with an OpenCode Zen/Go API key set on the channel, requests are
// authenticated against the same endpoint instead.
type Adaptor struct {
	openai.Adaptor
}

const renamesContextKey = "opencode_tool_renames"

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
	a.Adaptor.Init(info)
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	// An explicit base_url (for example the OpenCode Go lane,
	// https://opencode.ai/inference/go/openai/v1) is honored when the operator
	// sets one on the channel; the embedded anonymous lane is the default.
	base := strings.TrimRight(strings.TrimSpace(info.ChannelBaseUrl), "/")
	if base == "" {
		return opencodeAnonymousBaseURL, nil
	}
	if !strings.HasSuffix(base, "/chat/completions") {
		base += "/chat/completions"
	}
	return base, nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, header *http.Header, info *relaycommon.RelayInfo) error {
	header.Set("Content-Type", "application/json")
	header.Set("Accept", "application/json")
	key := info.ApiKey
	if key == "" || key == "default" {
		key = "public"
	}
	header.Set("Authorization", "Bearer "+key)
	// Mirror the official CLI as closely as possible: its exact user agent,
	// and a session id that persists per channel instead of changing on
	// every request (the anonymous lane fingerprints per-session traffic).
	header.Set("User-Agent", "opencode/1.19.6")
	header.Set("x-opencode-client", "cli")
	stable := opencodeSessionID(info.ChannelId)
	header.Set("x-opencode-session-id", stable)
	header.Set("x-opencode-session", stable)
	header.Set("x-opencode-request", stable)
	header.Set("x-session-affinity", stable)
	return nil
}

// opencodeSessionID derives a deterministic ses_<64hex> id from the channel id,
// matching the format the official CLI mints for its sessions.
func opencodeSessionID(channelID int) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("opencode-anon-session:%d", channelID)))
	return "ses_" + hex.EncodeToString(sum[:])
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
