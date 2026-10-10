package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	ChannelOAuthCodex       = "codex"
	ChannelOAuthAntigravity = "antigravity"

	codexOAuthClientID       = "app_EMoamEEZ73f0CkXaXp7hrann"
	codexOAuthRedirectURI    = "http://localhost:1455/auth/callback"
	antigravityOAuthClientID = "1071006060591-tmhssin2h21lcre235vtolojh4g403ep.apps.googleusercontent.com"
	antigravityRedirectURI   = "http://localhost:8085/callback"
)

var (
	codexOAuthAuthorizeURL       = "https://auth.openai.com/oauth/authorize"
	codexOAuthTokenURL           = "https://auth.openai.com/oauth/token"
	antigravityOAuthAuthorizeURL = "https://accounts.google.com/o/oauth2/v2/auth"
	antigravityOAuthTokenURL     = "https://oauth2.googleapis.com/token"
	antigravityUserInfoURL       = "https://www.googleapis.com/oauth2/v2/userinfo"
	antigravityLoadProjectURL    = "https://cloudcode-pa.googleapis.com/v1internal:loadCodeAssist"
)

type channelOAuthSession struct {
	State        string
	CodeVerifier string
	Provider     string
	RedirectURI  string
	CreatedAt    time.Time
}

type ChannelOAuthFlow struct {
	SessionID string `json:"session_id"`
	AuthURL   string `json:"auth_url"`
	State     string `json:"state"`
}

type ChannelOAuthService struct {
	mu       sync.Mutex
	sessions map[string]channelOAuthSession
	client   *http.Client
	clientFor func() *http.Client
}

func NewChannelOAuthService() *ChannelOAuthService {
	return &ChannelOAuthService{
		sessions: make(map[string]channelOAuthSession),
		client:   &http.Client{Timeout: 20 * time.Second},
	}
}

func (s *ChannelOAuthService) httpClient() *http.Client {
	if s.clientFor != nil {
		if client := s.clientFor(); client != nil {
			return client
		}
	}
	if s.client == nil {
		s.client = &http.Client{Timeout: 20 * time.Second}
	}
	return s.client
}

func (s *ChannelOAuthService) Generate(provider string) (*ChannelOAuthFlow, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	var authEndpoint, clientID, redirectURI, scopes string
	switch provider {
	case ChannelOAuthCodex:
		authEndpoint, clientID = codexOAuthAuthorizeURL, codexOAuthClientID
		redirectURI, scopes = codexOAuthRedirectURI, "openid profile email offline_access"
	case ChannelOAuthAntigravity:
		authEndpoint, clientID = antigravityOAuthAuthorizeURL, antigravityOAuthClientID
		redirectURI = antigravityRedirectURI
		scopes = "https://www.googleapis.com/auth/cloud-platform https://www.googleapis.com/auth/userinfo.email https://www.googleapis.com/auth/userinfo.profile https://www.googleapis.com/auth/cclog https://www.googleapis.com/auth/experimentsandconfigs"
	default:
		return nil, fmt.Errorf("unsupported channel OAuth provider")
	}
	state, err := secureRandomURLString(32)
	if err != nil {
		return nil, err
	}
	verifier, err := secureRandomURLString(48)
	if err != nil {
		return nil, err
	}
	sessionID, err := secureRandomURLString(24)
	if err != nil {
		return nil, err
	}
	challengeHash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(challengeHash[:])
	params := url.Values{
		"client_id":             {clientID},
		"redirect_uri":          {redirectURI},
		"response_type":         {"code"},
		"scope":                 {scopes},
		"state":                 {state},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}
	if provider == ChannelOAuthCodex {
		params.Set("id_token_add_organizations", "true")
		params.Set("codex_cli_simplified_flow", "true")
	} else {
		params.Set("access_type", "offline")
		params.Set("prompt", "consent")
		params.Set("include_granted_scopes", "true")
	}
	s.mu.Lock()
	if s.sessions == nil {
		s.sessions = make(map[string]channelOAuthSession)
	}
	s.pruneLocked(time.Now())
	s.sessions[sessionID] = channelOAuthSession{State: state, CodeVerifier: verifier, Provider: provider, RedirectURI: redirectURI, CreatedAt: time.Now()}
	s.mu.Unlock()
	return &ChannelOAuthFlow{SessionID: sessionID, AuthURL: authEndpoint + "?" + params.Encode(), State: state}, nil
}

// Exchange validates the provider-issued callback URL and returns the JSON
// credential string expected in a Codex or Antigravity channel's key field.
func (s *ChannelOAuthService) Exchange(ctx context.Context, provider, sessionID, callbackURL string) (string, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	u, err := url.Parse(strings.TrimSpace(callbackURL))
	if err != nil || !isLoopbackOAuthCallback(u) {
		return "", fmt.Errorf("paste the full localhost callback URL returned by the provider")
	}
	query := u.Query()
	if providerError := query.Get("error"); providerError != "" {
		return "", fmt.Errorf("OAuth authorization failed: %s", providerError)
	}
	code, state := strings.TrimSpace(query.Get("code")), strings.TrimSpace(query.Get("state"))
	if code == "" || state == "" {
		return "", fmt.Errorf("callback URL must contain code and state")
	}
	s.mu.Lock()
	session, ok := s.sessions[sessionID]
	if ok && time.Since(session.CreatedAt) > 30*time.Minute {
		delete(s.sessions, sessionID)
		ok = false
	}
	if ok && (session.Provider != provider || session.State != state) {
		ok = false
	}
	s.mu.Unlock()
	if !ok {
		return "", fmt.Errorf("OAuth session is missing, expired, or does not match this callback")
	}
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {oauthClientID(provider)},
		"code":          {code},
		"redirect_uri":  {session.RedirectURI},
		"code_verifier": {session.CodeVerifier},
	}
	if provider == ChannelOAuthAntigravity {
		secret := strings.TrimSpace(os.Getenv("ANTIGRAVITY_OAUTH_CLIENT_SECRET"))
		if secret == "" {
			return "", fmt.Errorf("ANTIGRAVITY_OAUTH_CLIENT_SECRET is required for Antigravity OAuth token exchange")
		}
		form.Set("client_secret", secret)
	} else if provider != ChannelOAuthCodex {
		return "", fmt.Errorf("unsupported channel OAuth provider")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, oauthTokenURL(provider), strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	response, err := s.httpClient().Do(request)
	if err != nil {
		return "", fmt.Errorf("OAuth token exchange failed: %w", err)
	}
	defer response.Body.Close()
	var token struct {
		AccessToken  string `json:"access_token"`
		IDToken      string `json:"id_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
		TokenType    string `json:"token_type"`
		Error        string `json:"error"`
		Description  string `json:"error_description"`
	}
	if err := common.DecodeJson(response.Body, &token); err != nil {
		return "", fmt.Errorf("OAuth token response could not be decoded: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message := token.Description
		if message == "" {
			message = token.Error
		}
		if message == "" {
			message = fmt.Sprintf("HTTP %d", response.StatusCode)
		}
		return "", fmt.Errorf("OAuth token exchange rejected: %s", message)
	}
	if strings.TrimSpace(token.AccessToken) == "" || strings.TrimSpace(token.RefreshToken) == "" {
		return "", fmt.Errorf("OAuth token response is missing access_token or refresh_token")
	}
	credential := map[string]string{
		"access_token":  token.AccessToken,
		"refresh_token": token.RefreshToken,
		"type":          provider,
	}
	if token.ExpiresIn > 0 {
		credential["expired"] = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second).Format(time.RFC3339)
	}
	if provider == ChannelOAuthCodex {
		credential["id_token"] = token.IDToken
		accountID, found := ExtractCodexAccountIDFromJWT(token.IDToken)
		if !found {
			accountID, found = ExtractCodexAccountIDFromJWT(token.AccessToken)
		}
		if !found {
			return "", fmt.Errorf("Codex OAuth token did not include a ChatGPT account ID")
		}
		credential["account_id"] = accountID
		if email, found := ExtractEmailFromJWT(token.IDToken); found {
			credential["email"] = email
		}
	} else {
		if tokenInfo, err := s.fetchAntigravityUserInfo(ctx, token.AccessToken); err == nil {
			credential["email"] = tokenInfo
		}
		if projectID, err := s.fetchAntigravityProject(ctx, token.AccessToken); err == nil && projectID != "" {
			credential["project_id"] = projectID
		}
	}
	encoded, err := common.Marshal(credential)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	delete(s.sessions, sessionID)
	s.mu.Unlock()
	return string(encoded), nil
}

func (s *ChannelOAuthService) fetchAntigravityUserInfo(ctx context.Context, accessToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, antigravityUserInfoURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := s.httpClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var user struct { Email string `json:"email"` }
	if err := common.DecodeJson(resp.Body, &user); err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("Google userinfo returned HTTP %d", resp.StatusCode)
	}
	return strings.TrimSpace(user.Email), nil
}

func (s *ChannelOAuthService) fetchAntigravityProject(ctx context.Context, accessToken string) (string, error) {
	body := []byte(`{"metadata":{"ideType":"ANTIGRAVITY","ideVersion":"2.9.1","ideName":"antigravity"}}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, antigravityLoadProjectURL, strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "antigravity/2.9.1 windows/amd64")
	resp, err := s.httpClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var project struct { Project string `json:"cloudaicompanionProject"` }
	if err := common.DecodeJson(resp.Body, &project); err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("Antigravity project lookup returned HTTP %d", resp.StatusCode)
	}
	return strings.TrimSpace(project.Project), nil
}

func (s *ChannelOAuthService) pruneLocked(now time.Time) {
	for id, session := range s.sessions {
		if now.Sub(session.CreatedAt) > 30*time.Minute {
			delete(s.sessions, id)
		}
	}
}

func secureRandomURLString(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func isLoopbackOAuthCallback(u *url.URL) bool {
	if u == nil || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func oauthClientID(provider string) string {
	if provider == ChannelOAuthAntigravity {
		return antigravityOAuthClientID
	}
	return codexOAuthClientID
}

func oauthTokenURL(provider string) string {
	if provider == ChannelOAuthAntigravity {
		return antigravityOAuthTokenURL
	}
	return codexOAuthTokenURL
}
