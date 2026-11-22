package colab

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	DefaultBaseURL       = "https://colaboratory.googleapis.com/v1beta"
	DefaultTunnelBaseURL = "https://colab.research.google.com/tun/m"
	DefaultTimeout       = 30 * time.Second
)

// AcceleratorSpec describes a Google Colab runtime accelerator option.
type AcceleratorSpec struct {
	ID                string `json:"id"`
	Label             string `json:"label"`
	Kind              string `json:"kind"` // cpu, gpu, tpu
	Variant           string `json:"variant"`
	Shape             string `json:"shape"`
	TunnelVariant     string `json:"-"`
	TunnelAccelerator string `json:"-"`
	HighMem           bool   `json:"-"`
}

// SupportedAccelerators lists the CPU, GPU, and TPU options for Google Colab.
var SupportedAccelerators = []AcceleratorSpec{
	{ID: "CPU", Label: "CPU (Standard)", Kind: "cpu", Variant: "VARIANT_UNSPECIFIED", Shape: "SHAPE_UNSPECIFIED", TunnelVariant: "", TunnelAccelerator: ""},
	{ID: "NVIDIA_TESLA_T4", Label: "GPU: NVIDIA Tesla T4 (16GB)", Kind: "gpu", Variant: "VARIANT_GPU", Shape: "SHAPE_STANDARD", TunnelVariant: "GPU", TunnelAccelerator: "T4"},
	{ID: "NVIDIA_L4", Label: "GPU: NVIDIA L4 (24GB)", Kind: "gpu", Variant: "VARIANT_GPU", Shape: "SHAPE_HIGHMEM", TunnelVariant: "GPU", TunnelAccelerator: "L4"},
	{ID: "NVIDIA_TESLA_A100", Label: "GPU: NVIDIA A100 (40GB)", Kind: "gpu", Variant: "VARIANT_GPU", Shape: "SHAPE_HIGHMEM", TunnelVariant: "GPU", TunnelAccelerator: "A100", HighMem: true},
	{ID: "NVIDIA_TESLA_A100_80GB", Label: "GPU: NVIDIA A100 (80GB)", Kind: "gpu", Variant: "VARIANT_GPU", Shape: "SHAPE_HIGHMEM", TunnelVariant: "GPU", TunnelAccelerator: "A100", HighMem: true},
	{ID: "NVIDIA_RTX_PRO_6000", Label: "GPU: NVIDIA RTX PRO 6000", Kind: "gpu", Variant: "VARIANT_GPU", Shape: "SHAPE_HIGHMEM", TunnelVariant: "GPU", TunnelAccelerator: "G4", HighMem: true},
	{ID: "NVIDIA_H100_80GB", Label: "GPU: NVIDIA H100 (80GB)", Kind: "gpu", Variant: "VARIANT_GPU", Shape: "SHAPE_HIGHMEM", TunnelVariant: "GPU", TunnelAccelerator: "H100", HighMem: true},
	{ID: "TPU_V2_8", Label: "TPU: v2-8", Kind: "tpu", Variant: "VARIANT_TPU", Shape: "SHAPE_UNSPECIFIED", TunnelVariant: "TPU", TunnelAccelerator: ""},
	{ID: "TPU_V5E_1", Label: "TPU: v5e-1", Kind: "tpu", Variant: "VARIANT_TPU", Shape: "SHAPE_UNSPECIFIED", TunnelVariant: "TPU", TunnelAccelerator: "V5E1"},
	{ID: "TPU_V6E_1", Label: "TPU: v6e-1", Kind: "tpu", Variant: "VARIANT_TPU", Shape: "SHAPE_UNSPECIFIED", TunnelVariant: "TPU", TunnelAccelerator: "V6E1"},
}

// LookupAccelerator resolves an accelerator ID to its spec, defaulting to T4.
func LookupAccelerator(id string) AcceleratorSpec {
	trimmed := strings.TrimSpace(id)
	for _, spec := range SupportedAccelerators {
		if strings.EqualFold(spec.ID, trimmed) {
			return spec
		}
	}
	return SupportedAccelerators[1]
}

type Client struct {
	AccessToken   string
	BaseURL       string
	TunnelBaseURL string
	HTTPClient    *http.Client
}

func NewClient(accessToken string) *Client {
	return &Client{
		AccessToken:   strings.TrimSpace(accessToken),
		BaseURL:       DefaultBaseURL,
		TunnelBaseURL: DefaultTunnelBaseURL,
		HTTPClient:    &http.Client{Timeout: DefaultTimeout},
	}
}

type APIError struct {
	Code    int
	Message string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("colab (HTTP %d): %s", e.Code, e.Message)
	}
	return fmt.Sprintf("colab: request failed with HTTP %d", e.Code)
}

func (c *Client) do(method, path string, body any, out any) error {
	var bodyReader io.Reader
	if body != nil {
		raw, err := common.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, strings.TrimRight(c.BaseURL, "/")+path, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.AccessToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Colab-Client-Agent", "colab-cli")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(data))
		if len(msg) > 400 {
			msg = msg[:400] + "..."
		}
		return &APIError{Code: resp.StatusCode, Message: msg}
	}
	if out != nil && len(bytes.TrimSpace(data)) > 0 {
		if err := common.Unmarshal(data, out); err != nil {
			return fmt.Errorf("colab: decode response: %w", err)
		}
	}
	return nil
}

type RuntimeSpecsResponse struct {
	RuntimeSpecs []map[string]any `json:"runtimeSpecs"`
}

// ListRuntimeSpecs queries eligible runtime specs to verify OAuth token and entitlement.
func (c *Client) ListRuntimeSpecs() (*RuntimeSpecsResponse, error) {
	var out RuntimeSpecsResponse
	if err := c.do(http.MethodGet, "/runtimespecs", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func stripXSSIPrefix(raw []byte) []byte {
	trimmed := bytes.TrimSpace(raw)
	if bytes.HasPrefix(trimmed, []byte(")]}'")) {
		if idx := bytes.IndexByte(trimmed, '\n'); idx >= 0 {
			return bytes.TrimSpace(trimmed[idx+1:])
		}
		return bytes.TrimSpace(trimmed[4:])
	}
	return trimmed
}

func uuidToWebSafeBase64(u string) string {
	s := strings.ReplaceAll(strings.TrimSpace(u), "-", "_")
	for len(s) < 44 {
		s += "."
	}
	return s
}

func isTransientNetErr(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code == http.StatusBadGateway ||
			apiErr.Code == http.StatusServiceUnavailable ||
			apiErr.Code == http.StatusGatewayTimeout
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	lower := strings.ToLower(err.Error())
	for _, needle := range []string{
		"connection reset",
		"broken pipe",
		"forcibly closed",
		"tls handshake timeout",
		"eof",
		"connection refused",
		"use of closed network connection",
	} {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

func doWithRetry(fn func() error) error {
	var err error
	backoff := time.Second
	for attempt := 0; attempt < 4; attempt++ {
		err = fn()
		if err == nil || !isTransientNetErr(err) {
			return err
		}
		if attempt < 3 {
			time.Sleep(backoff)
			backoff *= 2
		}
	}
	return err
}

func (c *Client) doTunnel(method, fullURL, xsrfToken string, out any) error {
	return doWithRetry(func() error {
		req, err := http.NewRequest(method, fullURL, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.AccessToken)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("X-Colab-Client-Agent", "colab-cli")
		if xsrfToken != "" {
			req.Header.Set("X-Goog-Colab-Token", xsrfToken)
		}

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		data, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		if err != nil {
			return err
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			msg := strings.TrimSpace(string(data))
			if len(msg) > 400 {
				msg = msg[:400] + "..."
			}
			return &APIError{Code: resp.StatusCode, Message: msg}
		}
		if out != nil {
			cleaned := stripXSSIPrefix(data)
			if len(cleaned) > 0 {
				if err := common.Unmarshal(cleaned, out); err != nil {
					return fmt.Errorf("colab: decode tunnel response: %w", err)
				}
			}
		}
		return nil
	})
}

// AccountInfo describes eligible accelerators and balance from /tun/m/ccu-info.
type AccountInfo struct {
	EligibleGPUs     []string `json:"eligibleGpus"`
	EligibleTPUs     []string `json:"eligibleTpus"`
	SubscriptionTier string   `json:"subscriptionTier"`
	CurrentBalance   float64  `json:"currentBalance"`
}

// GetAccountInfo fetches eligible GPUs/TPUs and tier info from Colab's ccu-info endpoint.
func (c *Client) GetAccountInfo() (*AccountInfo, error) {
	tunnelBase := strings.TrimRight(c.TunnelBaseURL, "/")
	if tunnelBase == "" {
		tunnelBase = DefaultTunnelBaseURL
	}
	var info AccountInfo
	if err := c.doTunnel(http.MethodGet, tunnelBase+"/ccu-info?authuser=0", "", &info); err != nil {
		return nil, err
	}
	return &info, nil
}

// TestConnection verifies Colab OAuth credentials against the consumer Colab tunnel API
// (/tun/m/ccu-info) and falls back to the Colab Public API (/runtimes).
func (c *Client) TestConnection() (int, error) {
	tunnelBase := strings.TrimRight(c.TunnelBaseURL, "/")
	if tunnelBase == "" {
		tunnelBase = DefaultTunnelBaseURL
	}
	var ccuOut map[string]any
	if err := c.doTunnel(http.MethodGet, tunnelBase+"/ccu-info?authuser=0", "", &ccuOut); err == nil {
		return 1, nil
	} else {
		var listOut struct {
			Runtimes []Runtime `json:"runtimes"`
		}
		if pubErr := c.do(http.MethodGet, "/runtimes", nil, &listOut); pubErr == nil {
			return len(listOut.Runtimes), nil
		}
		return 0, err
	}
}

type ConnectionInfo struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}

type Runtime struct {
	Name           string         `json:"name"`
	State          string         `json:"state"`
	ConnectionInfo ConnectionInfo `json:"connectionInfo"`
	ProxyURL       string         `json:"proxyUrl"`
}

type tunnelAssignResponse struct {
	Endpoint         string `json:"endpoint"`
	Token            string `json:"token"`
	RuntimeProxyInfo *struct {
		URL   string `json:"url"`
		Token string `json:"token"`
	} `json:"runtimeProxyInfo"`
}

// CreateRuntime provisions a Colab runtime via the consumer Colab Tunnel API (/tun/m/assign).
func (c *Client) CreateRuntime(accelerator string) (*Runtime, error) {
	spec := LookupAccelerator(accelerator)
	tunnelBase := strings.TrimRight(c.TunnelBaseURL, "/")
	if tunnelBase == "" {
		tunnelBase = DefaultTunnelBaseURL
	}

	q := url.Values{}
	q.Set("authuser", "0")
	q.Set("nbh", uuidToWebSafeBase64(uuid.New().String()))
	if spec.TunnelVariant != "" {
		q.Set("variant", spec.TunnelVariant)
	}
	if spec.TunnelAccelerator != "" {
		q.Set("accelerator", spec.TunnelAccelerator)
	}
	if spec.HighMem {
		q.Set("shape", "hm")
	}

	assignURL := tunnelBase + "/assign?" + q.Encode()
	var assignRes tunnelAssignResponse
	if err := c.doTunnel(http.MethodGet, assignURL, "", &assignRes); err != nil {
		return nil, err
	}

	if assignRes.Endpoint == "" || assignRes.RuntimeProxyInfo == nil || assignRes.RuntimeProxyInfo.URL == "" {
		if assignRes.Token == "" {
			return nil, fmt.Errorf("colab: assign did not return an XSRF token")
		}
		var postRes tunnelAssignResponse
		if err := c.doTunnel(http.MethodPost, assignURL, assignRes.Token, &postRes); err != nil {
			return nil, err
		}
		assignRes = postRes
	}

	if assignRes.Endpoint == "" || assignRes.RuntimeProxyInfo == nil || assignRes.RuntimeProxyInfo.URL == "" {
		return nil, fmt.Errorf("colab: assign succeeded without endpoint/runtimeProxyInfo")
	}

	return &Runtime{
		Name:  assignRes.Endpoint,
		State: "RUNNING",
		ConnectionInfo: ConnectionInfo{
			URL:   assignRes.RuntimeProxyInfo.URL,
			Token: assignRes.RuntimeProxyInfo.Token,
		},
		ProxyURL: assignRes.RuntimeProxyInfo.URL,
	}, nil
}

// GetRuntime retrieves runtime state and connectionInfo (proxy URL & token).
func (c *Client) GetRuntime(name string) (*Runtime, error) {
	path := "/" + strings.TrimLeft(name, "/")
	var out Runtime
	if err := c.do(http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteRuntime terminates a Colab runtime via tunnel unassign or Public API DELETE.
func (c *Client) DeleteRuntime(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return nil
	}
	if !strings.Contains(trimmed, "/") {
		tunnelBase := strings.TrimRight(c.TunnelBaseURL, "/")
		if tunnelBase == "" {
			tunnelBase = DefaultTunnelBaseURL
		}
		unassignURL := fmt.Sprintf("%s/unassign/%s?authuser=0", tunnelBase, url.PathEscape(trimmed))
		var getRes struct {
			Token string `json:"token"`
		}
		if err := c.doTunnel(http.MethodGet, unassignURL, "", &getRes); err != nil {
			return err
		}
		if getRes.Token == "" {
			return nil
		}
		return c.doTunnel(http.MethodPost, unassignURL, getRes.Token, nil)
	}
	path := "/" + strings.TrimLeft(trimmed, "/")
	return c.do(http.MethodDelete, path, nil, nil)
}

// UploadFile uploads a text file (such as serve.py) to the runtime's Jupyter server.
func (c *Client) UploadFile(proxyURL, proxyToken, filePath, content string) error {
	target := strings.TrimRight(proxyURL, "/") + "/api/contents/" + strings.TrimLeft(filePath, "/")
	payload, err := common.Marshal(map[string]any{
		"type":    "file",
		"format":  "text",
		"content": content,
	})
	if err != nil {
		return err
	}
	return doWithRetry(func() error {
		req, err := http.NewRequest(http.MethodPut, target, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Colab-Client-Agent", "colab-cli")
		if proxyToken != "" {
			req.Header.Set("X-Colab-Runtime-Proxy-Token", proxyToken)
		}

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
			return &APIError{Code: resp.StatusCode, Message: string(raw)}
		}
		return nil
	})
}

// StartKernel starts a Python Jupyter kernel on the Colab runtime.
func (c *Client) StartKernel(proxyURL, proxyToken string) (string, error) {
	target := strings.TrimRight(proxyURL, "/") + "/api/kernels"
	payload, err := common.Marshal(map[string]any{"name": "python3"})
	if err != nil {
		return "", err
	}
	var kernelID string
	err = doWithRetry(func() error {
		req, err := http.NewRequest(http.MethodPost, target, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Colab-Client-Agent", "colab-cli")
		if proxyToken != "" {
			req.Header.Set("X-Colab-Runtime-Proxy-Token", proxyToken)
		}

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return &APIError{Code: resp.StatusCode, Message: string(raw)}
		}
		var kernel struct {
			ID string `json:"id"`
		}
		_ = common.Unmarshal(raw, &kernel)
		kernelID = kernel.ID
		return nil
	})
	return kernelID, err
}

// ExecuteCode runs Python code in an active Jupyter kernel over WebSocket channels.
func (c *Client) ExecuteCode(proxyURL, proxyToken, kernelID, code string) error {
	if strings.TrimSpace(proxyURL) == "" || strings.TrimSpace(kernelID) == "" {
		return fmt.Errorf("colab: proxyURL and kernelID are required")
	}
	return doWithRetry(func() error {
		return c.executeCodeOnce(proxyURL, proxyToken, kernelID, code)
	})
}

func (c *Client) executeCodeOnce(proxyURL, proxyToken, kernelID, code string) error {
	wsBase := strings.TrimRight(proxyURL, "/")
	if strings.HasPrefix(wsBase, "https://") {
		wsBase = "wss://" + strings.TrimPrefix(wsBase, "https://")
	} else if strings.HasPrefix(wsBase, "http://") {
		wsBase = "ws://" + strings.TrimPrefix(wsBase, "http://")
	}
	sessionID := uuid.New().String()
	msgID := uuid.New().String()
	wsURL := fmt.Sprintf("%s/api/kernels/%s/channels?session_id=%s", wsBase, url.PathEscape(kernelID), url.QueryEscape(sessionID))

	headers := http.Header{}
	headers.Set("X-Colab-Client-Agent", "colab-cli")
	if proxyToken != "" {
		headers.Set("X-Colab-Runtime-Proxy-Token", proxyToken)
	}
	dialer := websocket.Dialer{HandshakeTimeout: 15 * time.Second}
	conn, _, err := dialer.Dial(wsURL, headers)
	if err != nil {
		return fmt.Errorf("colab: websocket dial failed: %w", err)
	}
	defer conn.Close()

	reqMsg := map[string]any{
		"header": map[string]any{
			"msg_id":   msgID,
			"username": "colab",
			"session":  sessionID,
			"date":     time.Now().UTC().Format(time.RFC3339),
			"msg_type": "execute_request",
			"version":  "5.3",
		},
		"parent_header": map[string]any{},
		"metadata":      map[string]any{},
		"content": map[string]any{
			"code":             code,
			"silent":           false,
			"store_history":    false,
			"user_expressions": map[string]any{},
			"allow_stdin":      false,
		},
		"buffers": []any{},
		"channel": "shell",
	}
	payload, err := common.Marshal(reqMsg)
	if err != nil {
		return err
	}
	if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
		return fmt.Errorf("colab: write execute_request: %w", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(15 * time.Second))
	var gotReply, gotIdle bool
	var execErrText string
	var readErr error
	for {
		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			readErr = err
			break
		}
		var env struct {
			MsgType string `json:"msg_type"`
			Parent  struct {
				MsgID string `json:"msg_id"`
			} `json:"parent_header"`
			Content struct {
				Status         string `json:"status"`
				ExecutionState string `json:"execution_state"`
				EName          string `json:"ename"`
				EValue         string `json:"evalue"`
			} `json:"content"`
		}
		if common.Unmarshal(msgBytes, &env) == nil && env.Parent.MsgID == msgID {
			switch env.MsgType {
			case "error":
				execErrText = strings.TrimSpace(env.Content.EName + ": " + env.Content.EValue)
			case "execute_reply":
				gotReply = true
				if env.Content.Status == "error" && execErrText == "" {
					execErrText = strings.TrimSpace(env.Content.EName + ": " + env.Content.EValue)
				}
			case "status":
				if env.Content.ExecutionState == "idle" {
					gotIdle = true
				}
			}
			if gotReply || gotIdle {
				break
			}
		}
	}
	if execErrText != "" {
		return fmt.Errorf("colab: kernel execution error: %s", execErrText)
	}
	if !gotReply && !gotIdle {
		if readErr != nil {
			return fmt.Errorf("colab: kernel execution did not confirm completion: %w", readErr)
		}
		return fmt.Errorf("colab: kernel execution did not confirm completion")
	}
	return nil
}

// ReadFile reads a text file from the runtime's Jupyter contents API.
func (c *Client) ReadFile(proxyURL, proxyToken, filePath string) (string, error) {
	target := strings.TrimRight(proxyURL, "/") + "/api/contents/" + strings.TrimLeft(filePath, "/") + "?content=1"
	var content string
	err := doWithRetry(func() error {
		req, err := http.NewRequest(http.MethodGet, target, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("X-Colab-Client-Agent", "colab-cli")
		if proxyToken != "" {
			req.Header.Set("X-Colab-Runtime-Proxy-Token", proxyToken)
		}
		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if err != nil {
			return err
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return &APIError{Code: resp.StatusCode, Message: strings.TrimSpace(string(raw))}
		}
		var out struct {
			Content string `json:"content"`
		}
		if err := common.Unmarshal(raw, &out); err != nil {
			return err
		}
		content = out.Content
		return nil
	})
	return content, err
}
