package claudesub

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	newapiCommon "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/claude"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/relayconvert"
	"github.com/QuantumNous/new-api/relaykit/types"

	"bytes"
	"strings"

	"github.com/gin-gonic/gin"
)

// claudeCodeIdentity is the first block Claude Code puts in the system
// prompt. Claude Code OAuth-scoped credentials are tied to the official CLI,
// so the first system block must carry this identity or the upstream
// rejects the request as non-CLI traffic.
const claudeCodeIdentity = "You are Claude Code, Anthropic's official CLI for Claude."

type Adaptor struct {
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	baseUrl := info.ChannelBaseUrl
	if baseUrl == "" {
		baseUrl = "https://api.anthropic.com"
	}
	return fmt.Sprintf("%s/v1/messages", baseUrl), nil
}

func containsIdentity(text string) bool {
	return strings.Contains(strings.ToLower(text), strings.ToLower(claudeCodeIdentity))
}

// ensureClaudeCodeSystem prepends the Claude Code identity block to the
// system prompt, converting whatever shape the caller used (nil, string, or a
// block array) into the block array Claude Code itself uses.
func ensureClaudeCodeSystem(req *dto.ClaudeRequest) {
	if req == nil {
		return
	}
	identityBlock := map[string]any{"type": "text", "text": claudeCodeIdentity}

	switch s := req.System.(type) {
	case nil:
		req.System = []map[string]any{identityBlock}
		return
	case string:
		if containsIdentity(s) {
			req.System = []map[string]any{identityBlock}
			if strings.TrimSpace(s) != claudeCodeIdentity {
				req.System = append(req.System.([]map[string]any), map[string]any{"type": "text", "text": s})
			}
			return
		}
		blocks := []map[string]any{identityBlock}
		if strings.TrimSpace(s) != "" {
			blocks = append(blocks, map[string]any{"type": "text", "text": s})
		}
		req.System = blocks
		return
	}

	// Block array (or anything JSON-marshalable): round-trip and normalise.
	raw, err := newapiCommon.Marshal(req.System)
	if err != nil {
		req.System = []map[string]any{identityBlock}
		return
	}
	var blocks []any
	if err := newapiCommon.Unmarshal(raw, &blocks); err != nil {
		req.System = []map[string]any{identityBlock}
		return
	}
	for _, block := range blocks {
		if m, ok := block.(map[string]any); ok {
			if text, _ := m["text"].(string); text != "" && containsIdentity(text) {
				return
			}
		}
	}
	req.System = append([]any{identityBlock}, blocks...)
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)
	token, newRaw, err := EnsureValidToken(info.ChannelId, info.ApiKey)
	if err != nil {
		return err
	}
	if token == "" {
		return errors.New("claudesub: a Claude subscription credential is required")
	}
	if newRaw != info.ApiKey {
		info.ApiKey = newRaw
	}
	// Claude Code OAuth credentials are scoped to the official CLI. Mimic the
	// CLI's wire shape so the upstream accepts the request.
	req.Set("Authorization", "Bearer "+token)
	req.Set("anthropic-version", "2023-06-01")
	req.Set("anthropic-beta", "claude-code-20250219,oauth-2025-04-20,interleaved-thinking-2025-05-14,fine-grained-tool-streaming-2025-05-14")
	req.Set("User-Agent", "claude-cli/2.1.321 (external, cli)")
	req.Set("X-App", "cli")
	req.Set("anthropic-dangerous-direct-browser-access", "true")
	if info.IsStream {
		req.Set("Accept", "text/event-stream")
	} else if req.Get("Accept") == "" {
		req.Set("Accept", "application/json")
	}
	return nil
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	result, err := relayconvert.ConvertRequest(c, info, types.RelayFormatClaude, request)
	if err != nil {
		return nil, err
	}
	claudeReq, ok := result.Value.(*dto.ClaudeRequest)
	if !ok {
		return nil, fmt.Errorf("expected Claude request, got %T", result.Value)
	}
	ensureClaudeCodeSystem(claudeReq)
	return claudeReq, nil
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	ensureClaudeCodeSystem(request)
	return request, nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	result, err := relayconvert.ConvertRequest(c, info, types.RelayFormatClaude, request)
	if err != nil {
		return nil, err
	}
	claudeReq, ok := result.Value.(*dto.ClaudeRequest)
	if !ok {
		return nil, fmt.Errorf("expected Claude request, got %T", result.Value)
	}
	ensureClaudeCodeSystem(claudeReq)
	return claudeReq, nil
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	result, err := relayconvert.ConvertRequest(c, info, types.RelayFormatClaude, &request)
	if err != nil {
		return nil, err
	}
	claudeReq, ok := result.Value.(*dto.ClaudeRequest)
	if !ok {
		return nil, fmt.Errorf("expected Claude request, got %T", result.Value)
	}
	ensureClaudeCodeSystem(claudeReq)
	return claudeReq, nil
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	return nil, errors.New("claudesub channel: audio endpoint not supported")
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	return nil, errors.New("claudesub channel: image endpoint not supported")
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, errors.New("claudesub channel: /v1/rerank endpoint not supported")
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return nil, errors.New("claudesub channel: /v1/embeddings endpoint not supported")
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	var bodyBytes []byte
	if requestBody != nil {
		bodyBytes, _ = io.ReadAll(requestBody)
	}
	resp, err := channel.DoApiRequest(a, c, info, bytes.NewReader(bodyBytes))
	// A 401 means the access token was rejected: if the credential carries a
	// refresh token, force a refresh and retry exactly once.
	if resp != nil && resp.StatusCode == http.StatusUnauthorized && info != nil && info.ChannelId > 0 {
		_ = resp.Body.Close()
		if token, newRaw, refreshErr := RefreshNow(info.ChannelId, info.ApiKey); refreshErr == nil && token != "" {
			info.ApiKey = newRaw
			var retryBody io.Reader
			if len(bodyBytes) > 0 {
				retryBody = bytes.NewReader(bodyBytes)
			}
			return channel.DoApiRequest(a, c, info, retryBody)
		}
	}
	return resp, err
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	info.FinalRequestRelayFormat = types.RelayFormatClaude
	if info.IsStream {
		return claude.ClaudeStreamHandler(c, resp, info)
	}
	return claude.ClaudeHandler(c, resp, info)
}
