package claudesub

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/model"
)

// Claude OAuth client id used by Claude Code (same public client id used by
// sub2api-compatible Claude subscription bridges).
const claudeOAuthClientID = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
const claudeOAuthTokenURL = "https://platform.claude.com/v1/oauth/token"

// ClaudeSubKey is the credential of a Claude subscription channel. It is
// either a raw Claude Code OAuth access token (manual token mode) or a JSON
// object with access_token plus optional refresh_token / expiry.
//
// Manual token mode:
//
//	{"access_token": "sk-ant-oat01-..."}
//
// Full OAuth refresh mode:
//
//	{"access_token": "sk-ant-oat01-...", "refresh_token": "sk-ant-ort01-...", "expired": "2026-11-16T10:00:00Z"}
//
// A plain, non-JSON string is treated as a manual access token.
type ClaudeSubKey struct {
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Expired      string `json:"expired,omitempty"`
	// ExpiresAt is the unix-seconds variant some tools emit; when both are
	// set, the later deadline wins.
	ExpiresAt int64 `json:"expires_at,omitempty"`
}

func ParseClaudeSubKey(raw string) *ClaudeSubKey {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "{") {
		return &ClaudeSubKey{AccessToken: trimmed}
	}
	var key ClaudeSubKey
	if err := json.Unmarshal([]byte(trimmed), &key); err != nil {
		return &ClaudeSubKey{}
	}
	return &key
}

func (k *ClaudeSubKey) expiryTime() time.Time {
	expiry := time.Time{}
	if strings.TrimSpace(k.Expired) != "" {
		if parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(k.Expired)); err == nil {
			expiry = parsed
		}
	}
	if k.ExpiresAt > 0 {
		if unixExpiry := time.Unix(k.ExpiresAt, 0); unixExpiry.After(expiry) {
			expiry = unixExpiry
		}
	}
	return expiry
}

// IsRefreshable reports whether the credential carries a refresh token.
func (k *ClaudeSubKey) IsRefreshable() bool {
	return k != nil && strings.TrimSpace(k.RefreshToken) != ""
}

// NeedsRefresh reports whether the access token is missing or expires within
// the safety window (5 minutes) used before every request.
func (k *ClaudeSubKey) NeedsRefresh() bool {
	if k == nil {
		return true
	}
	if strings.TrimSpace(k.AccessToken) == "" {
		return true
	}
	expiry := k.expiryTime()
	if expiry.IsZero() {
		// No expiry information at all: treat it as valid and let a real
		// 401 trigger the reactive refresh path.
		return false
	}
	return time.Now().After(expiry.Add(-5 * time.Minute))
}

func (k *ClaudeSubKey) raw() string {
	encoded, err := json.Marshal(k)
	if err != nil {
		return ""
	}
	return string(encoded)
}

type claudeOAuthTokenResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	ExpiresIn        int64  `json:"expires_in"`
	ExpiresAt        int64  `json:"expires_at"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// refreshExchangeLock prevents a stampede of concurrent refreshes from one
// process for the same credential shape.
var refreshExchangeLock sync.Mutex

func refreshClaudeSubToken(k *ClaudeSubKey) error {
	if k == nil || strings.TrimSpace(k.RefreshToken) == "" {
		return errors.New("claudesub: no refresh token available")
	}
	refreshExchangeLock.Lock()
	defer refreshExchangeLock.Unlock()

	payloadBytes, err := json.Marshal(map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": strings.TrimSpace(k.RefreshToken),
		"client_id":     claudeOAuthClientID,
	})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, claudeOAuthTokenURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("User-Agent", "axios/1.13.6")
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	var res claudeOAuthTokenResponse
	if err := json.Unmarshal(respBody, &res); err != nil {
		return fmt.Errorf("claudesub: token refresh returned a non-JSON response (HTTP %d)", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK || strings.TrimSpace(res.AccessToken) == "" {
		msg := strings.TrimSpace(res.ErrorDescription)
		if msg == "" {
			msg = strings.TrimSpace(res.Error)
		}
		if msg == "" {
			msg = strings.TrimSpace(string(respBody))
		}
		return fmt.Errorf("claudesub: token refresh failed (HTTP %d): %s", resp.StatusCode, msg)
	}
	k.AccessToken = strings.TrimSpace(res.AccessToken)
	if rotated := strings.TrimSpace(res.RefreshToken); rotated != "" {
		k.RefreshToken = rotated
	}
	switch {
	case res.ExpiresIn > 0:
		k.Expired = time.Now().Add(time.Duration(res.ExpiresIn) * time.Second).Format(time.RFC3339)
		k.ExpiresAt = 0
	case res.ExpiresAt > 0:
		k.ExpiresAt = res.ExpiresAt
		k.Expired = ""
	}
	return nil
}

// persistClaudeSubKey writes the credential back to the channel row so the
// refreshed token survives restarts. Best effort: a failure to persist must
// not break the inflight request.
func persistClaudeSubKey(channelID int, k *ClaudeSubKey) {
	if channelID <= 0 || k == nil {
		return
	}
	encoded, err := json.Marshal(k)
	if err != nil {
		return
	}
	_ = model.DB.Model(&model.Channel{}).Where("id = ?", channelID).Update("key", string(encoded)).Error
}

// EnsureValidToken returns a valid access token for the credential, refreshing
// it first when refresh mode is configured and the token is missing/expired.
// When a refresh succeeds, the updated credential is persisted back to the
// channel row and the fresh credential JSON is returned so the caller can
// update its context. It never fails a manual token just because the token
// expired — a 401 will tell the admin to rotate it.
func EnsureValidToken(channelID int, rawKey string) (token string, newRaw string, err error) {
	key := ParseClaudeSubKey(rawKey)
	if strings.TrimSpace(key.AccessToken) == "" && !key.IsRefreshable() {
		return "", "", errors.New("claudesub: access_token is required (or a refresh_token for the OAuth refresh flow)")
	}
	if !key.NeedsRefresh() {
		return strings.TrimSpace(key.AccessToken), rawKey, nil
	}
	if !key.IsRefreshable() {
		return strings.TrimSpace(key.AccessToken), rawKey, nil
	}
	if err := refreshClaudeSubToken(key); err != nil {
		return "", rawKey, err
	}
	persistClaudeSubKey(channelID, key)
	return strings.TrimSpace(key.AccessToken), key.raw(), nil
}

// RefreshNow forces a refresh regardless of the expiry window. Used by the
// reactive 401 path: if the upstream rejected the token once, do not give it
// another chance with the same credential.
func RefreshNow(channelID int, rawKey string) (token string, newRaw string, err error) {
	key := ParseClaudeSubKey(rawKey)
	if !key.IsRefreshable() {
		return "", rawKey, errors.New("claudesub: no refresh token available")
	}
	if err := refreshClaudeSubToken(key); err != nil {
		return "", rawKey, err
	}
	persistClaudeSubKey(channelID, key)
	return strings.TrimSpace(key.AccessToken), key.raw(), nil
}
