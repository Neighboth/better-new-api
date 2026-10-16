package deepseekweb

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
)

const (
	defaultBaseURL = "https://chat.deepseek.com"

	completionPath = "/api/v0/chat/completion"
	sessionPath    = "/api/v0/chat_session/create"
	challengePath  = "/api/v0/chat/create_pow_challenge"
	uploadPath     = "/api/v0/file/upload_file"
	fetchFilesPath = "/api/v0/file/fetch_files"

	// The web UI fetches attachments back after upload because the backend
	// parses them asynchronously: a freshly uploaded file is PENDING and only
	// becomes usable once it reports SUCCESS.
	fileReadyStatus = "SUCCESS"

	fileReadyPollInterval = 400 * time.Millisecond
	fileReadyTimeout      = 45 * time.Second

	browserUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36"
	clientAppVersion = "20241129.1"
	clientVersion    = "1.0.0-always"

	// maxAttachmentBytes bounds a single attachment pulled from the caller or
	// from a remote URL so a relayed request cannot buffer unbounded data.
	maxAttachmentBytes = 50 << 20

	// remoteAttachmentTimeout bounds a remote download separately, because the
	// caller controls the URL and a slow body must not stall a relay slot.
	remoteAttachmentTimeout = 30 * time.Second
)

// isFailedFileStatus reports whether an attachment reached a terminal failure.
// The concrete values (FAILED, PARSE_FAILED, AUDIT_FAILED, ERROR…) are not
// documented, so the check matches on the stable substrings instead of pinning
// an exhaustive list that upstream could extend.
func isFailedFileStatus(status string) bool {
	upper := strings.ToUpper(status)
	return strings.Contains(upper, "FAIL") || strings.Contains(upper, "ERROR") || strings.Contains(upper, "REJECT")
}

type webClient struct {
	baseURL string
	token   string
	http    *http.Client
}

func newWebClient(ctx context.Context, apiKey, baseURL string) (*webClient, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	token, err := resolveToken(ctx, apiKey, baseURL)
	if err != nil {
		return nil, err
	}
	return &webClient{
		baseURL: baseURL,
		token:   token,
		http:    &http.Client{Timeout: 180 * time.Second},
	}, nil
}

func (c *webClient) applyHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("User-Agent", browserUserAgent)
	req.Header.Set("Origin", "https://chat.deepseek.com")
	req.Header.Set("Referer", "https://chat.deepseek.com/")
	req.Header.Set("x-client-platform", "web")
	req.Header.Set("x-app-version", clientAppVersion)
	req.Header.Set("x-client-version", clientVersion)
}

// doJSON posts body to path and decodes the standard DeepSeek envelope,
// surfacing the upstream business error code instead of a bare HTTP status.
func (c *webClient) doJSON(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		payload, err := common.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c.applyHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("deepseek web: %s returned HTTP %d: %s", path, resp.StatusCode, truncateForError(raw))
	}

	var envelope struct {
		Code    int             `json:"code"`
		Msg     string          `json:"msg"`
		BizCode int             `json:"biz_code"`
		BizMsg  string          `json:"biz_msg"`
		Data    json.RawMessage `json:"data"`
	}
	if err := common.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("deepseek web: cannot decode %s response: %w", path, err)
	}
	if envelope.Code != 0 {
		return fmt.Errorf("deepseek web: %s rejected the request (code %d): %s", path, envelope.Code, envelope.Msg)
	}

	if out == nil {
		return nil
	}
	// Business payloads nest one level deeper: {"data":{"biz_code":0,"biz_data":{...}}}
	var inner struct {
		BizCode int             `json:"biz_code"`
		BizMsg  string          `json:"biz_msg"`
		BizData json.RawMessage `json:"biz_data"`
	}
	if err := common.Unmarshal(envelope.Data, &inner); err != nil {
		return fmt.Errorf("deepseek web: cannot decode %s payload: %w", path, err)
	}
	if inner.BizCode != 0 {
		return fmt.Errorf("deepseek web: %s failed (biz_code %d): %s", path, inner.BizCode, inner.BizMsg)
	}
	if err := common.Unmarshal(inner.BizData, out); err != nil {
		return fmt.Errorf("deepseek web: cannot decode %s biz_data: %w", path, err)
	}
	return nil
}

func (c *webClient) createSession(ctx context.Context) (string, error) {
	var out struct {
		ID string `json:"id"`
	}
	if err := c.doJSON(ctx, http.MethodPost, sessionPath, map[string]any{}, &out); err != nil {
		return "", err
	}
	if out.ID == "" {
		return "", errors.New("deepseek web: session id not returned by server")
	}
	return out.ID, nil
}

// powHeader fetches and solves a challenge bound to targetPath. DeepSeek issues
// a separate proof of work per endpoint, so the upload and completion paths each
// need their own header.
func (c *webClient) powHeader(ctx context.Context, targetPath string) (string, error) {
	var out struct {
		Challenge PowChallenge `json:"challenge"`
	}
	if err := c.doJSON(ctx, http.MethodPost, challengePath, map[string]string{"target_path": targetPath}, &out); err != nil {
		return "", err
	}
	if out.Challenge.Challenge == "" {
		return "", errors.New("deepseek web: empty pow challenge returned")
	}
	return SolvePoWChallenge(ctx, &out.Challenge)
}

type uploadedFile struct {
	ID         string `json:"id"`
	Status     string `json:"status"`
	FileName   string `json:"file_name"`
	ErrorCode  string `json:"error_code"`
	TokenUsage *int   `json:"token_usage"`
}

// uploadAttachment uploads one file and waits until DeepSeek finishes parsing it,
// because a freshly uploaded file is not usable by the completion endpoint yet.
func (c *webClient) uploadAttachment(ctx context.Context, name, mimeType string, data []byte) (string, error) {
	pow, err := c.powHeader(ctx, uploadPath)
	if err != nil {
		return "", err
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, escapeFilename(name)))
	if mimeType != "" {
		header.Set("Content-Type", mimeType)
	}
	part, err := writer.CreatePart(header)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(data); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+uploadPath, bytes.NewReader(body.Bytes()))
	if err != nil {
		return "", err
	}
	c.applyHeaders(req)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("x-ds-pow-response", pow)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	var envelope struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			BizCode int          `json:"biz_code"`
			BizMsg  string       `json:"biz_msg"`
			BizData uploadedFile `json:"biz_data"`
		} `json:"data"`
	}
	if err := common.Unmarshal(raw, &envelope); err != nil {
		return "", fmt.Errorf("deepseek web: cannot decode upload response: %w", err)
	}
	if envelope.Code != 0 {
		return "", fmt.Errorf("deepseek web: upload rejected (code %d): %s", envelope.Code, envelope.Msg)
	}
	if envelope.Data.BizCode != 0 {
		return "", fmt.Errorf("deepseek web: upload failed (biz_code %d): %s", envelope.Data.BizCode, envelope.Data.BizMsg)
	}
	if envelope.Data.BizData.ID == "" {
		return "", errors.New("deepseek web: upload returned no file id")
	}

	if err := c.waitForFile(ctx, envelope.Data.BizData.ID); err != nil {
		return "", err
	}
	return envelope.Data.BizData.ID, nil
}

func (c *webClient) waitForFile(ctx context.Context, fileID string) error {
	deadline := time.Now().Add(fileReadyTimeout)
	for {
		file, err := c.fetchFile(ctx, fileID)
		if err != nil {
			return err
		}
		switch {
		case file.Status == fileReadyStatus:
			return nil
		case isFailedFileStatus(file.Status):
			return fmt.Errorf("deepseek web: attachment %q could not be parsed (status %s, error %s)", file.FileName, file.Status, file.ErrorCode)
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("deepseek web: attachment %q is still %s after %s", file.FileName, file.Status, fileReadyTimeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(fileReadyPollInterval):
		}
	}
}

func (c *webClient) fetchFile(ctx context.Context, fileID string) (*uploadedFile, error) {
	query := url.Values{"file_ids": []string{fileID}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+fetchFilesPath+"?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	c.applyHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Data struct {
			BizData struct {
				Files []uploadedFile `json:"files"`
			} `json:"biz_data"`
		} `json:"data"`
	}
	if err := common.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("deepseek web: cannot decode file status response: %w", err)
	}
	if len(envelope.Data.BizData.Files) == 0 {
		return nil, fmt.Errorf("deepseek web: uploaded attachment %s is no longer available", fileID)
	}
	return &envelope.Data.BizData.Files[0], nil
}

type completionRequest struct {
	ChatSessionID   string   `json:"chat_session_id"`
	ParentMessageID *string  `json:"parent_message_id"`
	Prompt          string   `json:"prompt"`
	RefFileIDs      []string `json:"ref_file_ids"`
	ThinkingEnabled bool     `json:"thinking_enabled"`
}

// completion opens the streaming completion endpoint. The caller owns the body.
func (c *webClient) completion(ctx context.Context, sessionID, prompt string, fileIDs []string, thinkingEnabled bool) (*http.Response, error) {
	pow, err := c.powHeader(ctx, completionPath)
	if err != nil {
		return nil, err
	}
	if fileIDs == nil {
		fileIDs = []string{}
	}
	payload, err := common.Marshal(completionRequest{
		ChatSessionID:   sessionID,
		ParentMessageID: nil,
		Prompt:          prompt,
		RefFileIDs:      fileIDs,
		ThinkingEnabled: thinkingEnabled,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+completionPath, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	c.applyHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("x-ds-pow-response", pow)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return nil, fmt.Errorf("deepseek web: completion returned HTTP %d: %s", resp.StatusCode, truncateForError(raw))
	}
	return resp, nil
}

// resolveToken turns a channel key into a DeepSeek user token. The key is either
// the raw userToken, its JSON wrapper, or "email:password" for an interactive
// login.
func resolveToken(ctx context.Context, apiKey, baseURL string) (string, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return "", errors.New("deepseek web credentials missing: enter username and password or user token")
	}
	if strings.HasPrefix(apiKey, "{") {
		var tokenObj map[string]any
		if err := common.Unmarshal([]byte(apiKey), &tokenObj); err == nil {
			for _, key := range []string{"value", "token", "userToken"} {
				if val, ok := tokenObj[key].(string); ok && val != "" {
					return val, nil
				}
			}
		}
	}
	apiKey = strings.Trim(apiKey, "\"")
	if !strings.Contains(apiKey, ":") {
		return apiKey, nil
	}

	parts := strings.SplitN(apiKey, ":", 2)
	email, password := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])

	tokenCacheMu.RLock()
	cached, ok := tokenCache[email]
	tokenCacheMu.RUnlock()
	if ok && time.Now().Before(cached.expiresAt) {
		return cached.token, nil
	}

	payload, err := common.Marshal(map[string]string{"email": email, "password": password, "mobile": email})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/v0/users/login", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", browserUserAgent)
	req.Header.Set("Origin", "https://chat.deepseek.com")
	req.Header.Set("Referer", "https://chat.deepseek.com/")
	req.Header.Set("x-client-platform", "web")
	req.Header.Set("x-app-version", clientAppVersion)
	req.Header.Set("x-client-locale", "zh_CN")
	req.Header.Set("x-client-version", clientVersion)

	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return "", fmt.Errorf("deepseek login failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusAccepted {
		return "", errors.New("deepseek web: upstream asked for a CAPTCHA/security verification (HTTP 202); log in to chat.deepseek.com in a browser and paste the userToken from Local Storage into the channel key")
	}
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return "", fmt.Errorf("deepseek login failed (HTTP %d): %s", resp.StatusCode, truncateForError(raw))
	}

	var loginResp struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
		Msg string `json:"msg"`
	}
	if err := common.DecodeJson(resp.Body, &loginResp); err != nil {
		return "", fmt.Errorf("failed to decode login response: %w", err)
	}
	if loginResp.Data.Token == "" {
		if loginResp.Msg != "" {
			return "", fmt.Errorf("deepseek login failed: %s", loginResp.Msg)
		}
		return "", errors.New("deepseek login returned empty token")
	}

	tokenCacheMu.Lock()
	tokenCache[email] = cachedToken{token: loginResp.Data.Token, expiresAt: time.Now().Add(12 * time.Hour)}
	tokenCacheMu.Unlock()

	return loginResp.Data.Token, nil
}

// readAttachment materialises a caller supplied attachment into bytes.
func readAttachment(ctx context.Context, source types.FileSource) ([]byte, string, error) {
	if source == nil {
		return nil, "", errors.New("empty attachment")
	}
	if source.IsURL() {
		data, mimeType, err := downloadAttachment(ctx, source.GetRawData())
		return data, mimeType, err
	}

	raw := strings.TrimSpace(source.GetRawData())
	mimeType := ""
	if base64Source, ok := source.(*types.Base64Source); ok {
		mimeType = base64Source.MimeType
	}
	if strings.HasPrefix(raw, "data:") {
		comma := strings.Index(raw, ",")
		if comma < 0 {
			return nil, "", errors.New("malformed data URI attachment")
		}
		meta := raw[len("data:"):comma]
		raw = raw[comma+1:]
		if idx := strings.Index(meta, ";"); idx >= 0 {
			meta = meta[:idx]
		}
		if meta != "" {
			mimeType = meta
		}
	}
	if raw == "" {
		return nil, "", errors.New("empty attachment data")
	}
	if len(raw) > maxAttachmentBytes*4/3 {
		return nil, "", fmt.Errorf("attachment exceeds the %d byte limit", maxAttachmentBytes)
	}
	data, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		// Some clients omit the base64 padding.
		data, err = base64.RawStdEncoding.DecodeString(raw)
	}
	if err != nil {
		return nil, "", fmt.Errorf("cannot decode base64 attachment: %w", err)
	}
	if len(data) > maxAttachmentBytes {
		return nil, "", fmt.Errorf("attachment exceeds the %d byte limit", maxAttachmentBytes)
	}
	return data, mimeType, nil
}

func downloadAttachment(ctx context.Context, rawURL string) ([]byte, string, error) {
	ctx, cancel := context.WithTimeout(ctx, remoteAttachmentTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := (&http.Client{Timeout: remoteAttachmentTimeout}).Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("cannot download attachment: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("cannot download attachment: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxAttachmentBytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("cannot download attachment: %w", err)
	}
	if len(data) > maxAttachmentBytes {
		return nil, "", fmt.Errorf("attachment exceeds the %d byte limit", maxAttachmentBytes)
	}

	mimeType := strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0])
	return data, mimeType, nil
}

// attachmentName derives a stable file name for an attachment, because DeepSeek
// keeps the extension to decide how to parse the upload. A document attachment
// carries no image metadata, so media may be nil.
func attachmentName(index int, media *dto.MessageImageUrl) string {
	extension := ".png"
	mimeType := ""
	if media != nil {
		mimeType = media.MimeType
		if mimeType == "" {
			mimeType = mimeFromURL(media.Url)
		}
	}
	switch mimeType {
	case "image/jpeg", "image/jpg":
		extension = ".jpg"
	case "image/gif":
		extension = ".gif"
	case "image/webp":
		extension = ".webp"
	case "application/pdf":
		extension = ".pdf"
	case "text/plain":
		extension = ".txt"
	}
	return fmt.Sprintf("attachment-%d%s", index+1, extension)
}

func mimeFromURL(rawURL string) string {
	path := rawURL
	if idx := strings.Index(path, "?"); idx >= 0 {
		path = path[:idx]
	}
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".jpg"), strings.HasSuffix(lower, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(lower, ".gif"):
		return "image/gif"
	case strings.HasSuffix(lower, ".webp"):
		return "image/webp"
	case strings.HasSuffix(lower, ".pdf"):
		return "application/pdf"
	case strings.HasSuffix(lower, ".txt"):
		return "text/plain"
	case strings.HasSuffix(lower, ".png"):
		return "image/png"
	}
	return ""
}

func escapeFilename(name string) string {
	return strings.NewReplacer(`"`, "", "\r", "", "\n", "").Replace(name)
}

func truncateForError(raw []byte) string {
	const limit = 400
	text := strings.TrimSpace(string(raw))
	if len(text) > limit {
		return text[:limit] + "..."
	}
	return text
}
