// Portions adapted from github.com/jasonxu114514/opencode2api (GPL-3.0) by
// jasonxu114514. See THIRD-PARTY-LICENSES.md.
package opencode

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
)

const (
	opencodeAnonymousBaseURL = "https://opencode.ai/inference/openai/v1/chat/completions"
	opencodeCLIUserAgent     = "opencode/latest/2.1.1/cli"
	opencodeBase62Alphabet   = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
)

// Adaptor serves the OpenCode (Anon) channel. The upstream free tier only
// accepts requests that match the official OpenCode v2 CLI: valid /v1 endpoint,
// Bun BoringSSL TLS fingerprint + HTTP/1.1 header order, authentic ses_ session
// headers + fresh 40-hex x-opencode-project, official system prompt, renamed
// client tools, official agent toolset, and streaming upstream transport.
type Adaptor struct {
	openai.Adaptor
}

const (
	renamesContextKey      = "opencode_tool_renames"
	forcedStreamContextKey = "opencode_forced_stream"
)

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

func isAnonymousKey(rawKey string) bool {
	k := strings.TrimSpace(rawKey)
	return k == "" || k == "default" || k == "local" || k == "public"
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, header *http.Header, info *relaycommon.RelayInfo) error {
	header.Set("Content-Type", "application/json")
	if info.IsStream || isAnonymousKey(info.ApiKey) {
		header.Set("Accept", "text/event-stream")
	} else {
		header.Set("Accept", "application/json")
	}
	key := strings.TrimSpace(info.ApiKey)
	if isAnonymousKey(key) {
		key = "public"
	}
	header.Set("Authorization", "Bearer "+key)
	// Match official OpenCode v2 CLI headers exactly:
	// User-Agent: opencode/latest/2.1.1/cli, x-opencode-client: cli,
	// session headers (ses_...), fresh 40-char lowercase hex x-opencode-project,
	// and NO x-opencode-request (rejected by v2 /inference gate).
	header.Set("User-Agent", opencodeCLIUserAgent)
	header.Set("x-opencode-client", "cli")
	sessionID := getOrCreateOpenCodeSessionID(info.ChannelId)
	projectID := generateOpenCodeProjectID()
	header.Set("x-opencode-session", sessionID)
	header.Set("x-opencode-session-id", sessionID)
	header.Set("x-session-affinity", sessionID)
	header.Set("X-Session-Id", sessionID)
	header.Set("x-opencode-project", projectID)
	header.Del("x-opencode-request")
	return nil
}

// RefreshRequestHeaders rotates the session & project IDs on an outgoing
// OpenCode request after an IP/proxy switch (e.g. Psiphon IP rotation on 429).
func RefreshRequestHeaders(req *http.Request, channelID int) {
	if req == nil {
		return
	}
	sessionID := RotateOpenCodeSessionID(channelID)
	projectID := generateOpenCodeProjectID()
	req.Header.Set("x-opencode-session", sessionID)
	req.Header.Set("x-opencode-session-id", sessionID)
	req.Header.Set("x-session-affinity", sessionID)
	req.Header.Set("X-Session-Id", sessionID)
	req.Header.Set("x-opencode-project", projectID)
	req.Header.Del("x-opencode-request")
}

// generateOpenCodeProjectID mints a fresh 40-character lowercase hex project ID
// (git-SHA-shaped), matching OpenCode CLI's x-opencode-project header.
func generateOpenCodeProjectID() string {
	b := make([]byte, 20)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
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
	if renames != nil && !renames.empty() && c != nil {
		c.Set(renamesContextKey, renames)
	}
	// OpenCode's anonymous free tier enforces streaming requests; when a
	// downstream caller (including Channel Test) sends stream=false, force
	// stream=true upstream and collapse the SSE stream back to JSON in DoResponse.
	if isAnonymousKey(info.ApiKey) {
		streamRequested := info.IsStream
		if s, ok := payload["stream"].(bool); ok {
			streamRequested = s
		}
		if !streamRequested {
			payload["stream"] = true
			payload["stream_options"] = map[string]any{"include_usage": true}
			if c != nil {
				c.Set(forcedStreamContextKey, true)
			}
		}
	}
	return payload, nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	fullRequestURL, err := a.GetRequestURL(info)
	if err != nil {
		return nil, fmt.Errorf("get request url failed: %w", err)
	}
	var bodyBytes []byte
	if requestBody != nil {
		bodyBytes, err = io.ReadAll(requestBody)
		if err != nil {
			return nil, fmt.Errorf("read request body failed: %w", err)
		}
	}

	rawProxyURL := info.ChannelSetting.Proxy
	keyIndex := -1
	if info.ChannelIsMultiKey {
		keyIndex = info.ChannelMultiKeyIndex
	}
	rawProxyURL = service.EffectiveRelayProxyURL(info.ChannelId, keyIndex, info.ApiKey, rawProxyURL)
	isLocalhost := strings.Contains(fullRequestURL, "localhost") || strings.Contains(fullRequestURL, "127.0.0.1")
	usingPsiphon := false
	if !isLocalhost {
		if psiphonURL, ok := service.GetPsiphonProxyURL(info.ChannelId); ok && psiphonURL != "" {
			rawProxyURL = psiphonURL
			usingPsiphon = true
		}
	}
	if !usingPsiphon && !isLocalhost && common.OptionMap["ProxyPoolEnabled"] == "true" {
		rawProxyURL = service.GetNextProxyForAnonChannel(info.ChannelId, rawProxyURL)
	}
	if service.IsCloudflareWorkerProxy(rawProxyURL) {
		return channel.DoApiRequest(a, c, info, bytes.NewReader(bodyBytes))
	}

	method := http.MethodPost
	if c != nil && c.Request != nil && c.Request.Method != "" {
		method = c.Request.Method
	}
	req, err := http.NewRequest(method, fullRequestURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("new request failed: %w", err)
	}
	headers := req.Header
	if err := a.SetupRequestHeader(c, &headers, info); err != nil {
		return nil, fmt.Errorf("setup request header failed: %w", err)
	}
	headerOverride, err := channel.ResolveHeaderOverride(info, c)
	if err != nil {
		return nil, err
	}
	for k, v := range headerOverride {
		req.Header.Set(k, v)
		if strings.EqualFold(k, "Host") {
			req.Host = v
		}
	}

	var reqCtx = req.Context()
	if c != nil && c.Request != nil {
		reqCtx = c.Request.Context()
	}

	resp, err := doBunHTTPRequest(reqCtx, req, bodyBytes, rawProxyURL)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeDoRequestFailed, types.ErrOptionWithHideErrMsg("upstream error: do request failed"))
	}

	// Automatic Psiphon IP rotation on 429 (up to 3 fresh egress IPs)
	if usingPsiphon && resp.StatusCode == http.StatusTooManyRequests {
		for retry := 0; retry < 3; retry++ {
			nextProxy, rotErr := service.RotatePsiphonForChannel(info.ChannelId, rawProxyURL)
			if rotErr != nil || nextProxy == "" {
				errDetail := "rotation unavailable"
				if rotErr != nil {
					errDetail = rotErr.Error()
				}
				model.RecordProxyPoolEvent(info.ChannelId, -1, 0, rawProxyURL, "psiphon_error", fmt.Sprintf("Psiphon IP rotation failed after 429 (attempt %d/3): %s", retry+1, errDetail))
				continue
			}
			rawProxyURL = nextProxy
			st := service.GetPsiphonStatus(info.ChannelId)
			model.RecordProxyPoolEvent(info.ChannelId, -1, 0, rawProxyURL, "psiphon_rotate_429", fmt.Sprintf("Rotated Psiphon IP after upstream 429 (attempt %d/3, region=%s, ip=%s)", retry+1, st.Region, st.EgressIP))
			_ = resp.Body.Close()

			RefreshRequestHeaders(req, info.ChannelId)
			newResp, doErr := doBunHTTPRequest(reqCtx, req, bodyBytes, rawProxyURL)
			if doErr != nil {
				model.RecordProxyPoolEvent(info.ChannelId, -1, 0, rawProxyURL, "psiphon_error", fmt.Sprintf("Retry request failed over rotated Psiphon IP (attempt %d/3): %s", retry+1, doErr.Error()))
				continue
			}
			resp = newResp
			if resp.StatusCode != http.StatusTooManyRequests {
				break
			}
		}
	}

	// Smart retry on 429 across proxy pool (30m cooldown, max 6 retries)
	if !usingPsiphon && !isLocalhost && resp.StatusCode == http.StatusTooManyRequests && common.OptionMap["ProxyPoolEnabled"] == "true" {
		for retry := 0; retry < 6; retry++ {
			service.RecordProxyRateLimitedWithChannel(rawProxyURL, info.ChannelId, 30*time.Minute)
			nextProxy := service.GetNextProxyForAnonChannel(info.ChannelId, rawProxyURL)
			if nextProxy == "" || nextProxy == rawProxyURL {
				service.RecordPoolExhausted(info.ChannelId)
				break
			}
			rawProxyURL = nextProxy
			model.RecordProxyPoolEvent(info.ChannelId, -1, 0, rawProxyURL, "switch_429", fmt.Sprintf("Switched proxy after an upstream 429 (attempt %d/6)", retry+1))
			_ = resp.Body.Close()

			RefreshRequestHeaders(req, info.ChannelId)
			if service.IsCloudflareWorkerProxy(rawProxyURL) {
				if parsedW, pErr := url.Parse(rawProxyURL); pErr == nil {
					req.Header.Set("X-Target-URL", fullRequestURL)
					req.URL = parsedW
					req.Host = parsedW.Host
				}
			}
			newResp, doErr := doBunHTTPRequest(reqCtx, req, bodyBytes, rawProxyURL)
			if doErr != nil {
				break
			}
			resp = newResp
			if resp.StatusCode != http.StatusTooManyRequests {
				break
			}
		}
	}

	if c != nil {
		if upID := resp.Header.Get(common.RequestIdKey); upID != "" {
			c.Set(common.UpstreamRequestIdKey, upID)
		}
		if c.Request != nil && c.Request.Body != nil {
			_ = c.Request.Body.Close()
		}
	}
	return resp, nil
}

// DoResponse wraps the OpenAI response handlers, collapsing forced upstream
// SSE streams when downstream requested non-streaming JSON, and restoring wire
// tool names.
func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	renames := renamesFromContext(c)
	forcedStream := c != nil && c.GetBool(forcedStreamContextKey)
	if info.IsStream && !forcedStream {
		resp = wrapStreamWithRestore(resp, renames)
	} else {
		data, errRead := io.ReadAll(resp.Body)
		if errRead == nil {
			_ = resp.Body.Close()
			if forcedStream && resp.StatusCode >= 200 && resp.StatusCode < 300 {
				if collapsed, ok := collapseSSEStreamToJSON(data); ok {
					data = collapsed
					resp.Header.Set("Content-Type", "application/json")
				}
			}
			restored := RestoreChatResponse(data, renames)
			resp.Body = io.NopCloser(bytes.NewReader(restored))
			resp.ContentLength = int64(len(restored))
			resp.Header.Set("Content-Length", fmt.Sprint(len(restored)))
		}
	}
	return a.Adaptor.DoResponse(c, resp, info)
}

type collapsedToolCall struct {
	ID       string
	Type     string
	Name     string
	Args     strings.Builder
	HasIndex bool
}

type collapsedChoice struct {
	Role             string
	Content          strings.Builder
	ReasoningContent strings.Builder
	FinishReason     any
	ToolCalls        map[int]*collapsedToolCall
	ToolCallOrder    []int
}

func collapseSSEStreamToJSON(raw []byte) ([]byte, bool) {
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	var (
		id       string
		modelStr string
		created  any
		usageObj any
		choices  = map[int]*collapsedChoice{}
		order    []int
		sawChunk bool
	)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		var chunk map[string]any
		if err := common.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		sawChunk = true
		if id == "" {
			if v, ok := chunk["id"].(string); ok && v != "" {
				id = v
			}
		}
		if modelStr == "" {
			if v, ok := chunk["model"].(string); ok && v != "" {
				modelStr = v
			}
		}
		if created == nil {
			if v, ok := chunk["created"]; ok && v != nil {
				created = v
			}
		}
		if u, ok := chunk["usage"]; ok && u != nil {
			usageObj = u
		}

		rawChoices, _ := chunk["choices"].([]any)
		for _, rc := range rawChoices {
			chMap, ok := rc.(map[string]any)
			if !ok {
				continue
			}
			idx := 0
			if f, ok := chMap["index"].(float64); ok {
				idx = int(f)
			}
			cc, exists := choices[idx]
			if !exists {
				cc = &collapsedChoice{Role: "assistant", ToolCalls: map[int]*collapsedToolCall{}}
				choices[idx] = cc
				order = append(order, idx)
			}
			if fr, ok := chMap["finish_reason"]; ok && fr != nil {
				cc.FinishReason = fr
			}
			delta, ok := chMap["delta"].(map[string]any)
			if !ok {
				continue
			}
			if role, ok := delta["role"].(string); ok && role != "" {
				cc.Role = role
			}
			if content, ok := delta["content"].(string); ok && content != "" {
				cc.Content.WriteString(content)
			}
			if reasoning, ok := delta["reasoning_content"].(string); ok && reasoning != "" {
				cc.ReasoningContent.WriteString(reasoning)
			}
			if tcs, ok := delta["tool_calls"].([]any); ok {
				for _, rtc := range tcs {
					tcMap, ok := rtc.(map[string]any)
					if !ok {
						continue
					}
					tIdx := 0
					if f, ok := tcMap["index"].(float64); ok {
						tIdx = int(f)
					}
					tc, exists := cc.ToolCalls[tIdx]
					if !exists {
						tc = &collapsedToolCall{Type: "function", HasIndex: true}
						cc.ToolCalls[tIdx] = tc
						cc.ToolCallOrder = append(cc.ToolCallOrder, tIdx)
					}
					if tid, ok := tcMap["id"].(string); ok && tid != "" {
						tc.ID = tid
					}
					if ttype, ok := tcMap["type"].(string); ok && ttype != "" {
						tc.Type = ttype
					}
					if fn, ok := tcMap["function"].(map[string]any); ok {
						if name, ok := fn["name"].(string); ok && name != "" {
							tc.Name += name
						}
						if args, ok := fn["arguments"].(string); ok && args != "" {
							tc.Args.WriteString(args)
						}
					}
				}
			}
		}
	}

	if !sawChunk {
		return nil, false
	}

	if id == "" {
		id = "chatcmpl-" + generateOpenCodeID("msg", false)
	}
	if created == nil {
		created = time.Now().Unix()
	}

	outChoices := make([]map[string]any, 0, len(order))
	for _, idx := range order {
		cc := choices[idx]
		msg := map[string]any{
			"role":    cc.Role,
			"content": cc.Content.String(),
		}
		if cc.ReasoningContent.Len() > 0 {
			msg["reasoning_content"] = cc.ReasoningContent.String()
		}
		if len(cc.ToolCallOrder) > 0 {
			tcs := make([]map[string]any, 0, len(cc.ToolCallOrder))
			for _, tIdx := range cc.ToolCallOrder {
				tc := cc.ToolCalls[tIdx]
				tcs = append(tcs, map[string]any{
					"id":   tc.ID,
					"type": tc.Type,
					"function": map[string]any{
						"name":      tc.Name,
						"arguments": tc.Args.String(),
					},
				})
			}
			msg["tool_calls"] = tcs
		}
		fr := cc.FinishReason
		if fr == nil {
			fr = "stop"
		}
		outChoices = append(outChoices, map[string]any{
			"index":         idx,
			"message":       msg,
			"finish_reason": fr,
		})
	}

	result := map[string]any{
		"id":      id,
		"object":  "chat.completion",
		"created": created,
		"model":   modelStr,
		"choices": outChoices,
	}
	if usageObj != nil {
		result["usage"] = usageObj
	}
	encoded, err := common.Marshal(result)
	if err != nil {
		return nil, false
	}
	return encoded, true
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
