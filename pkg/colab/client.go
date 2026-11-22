package colab

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	DefaultBaseURL = "https://colaboratory.googleapis.com/v1beta"
	DefaultTimeout = 30 * time.Second
)

// AcceleratorSpec describes a Google Colab runtime accelerator option.
type AcceleratorSpec struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Kind    string `json:"kind"` // cpu, gpu, tpu
	Variant string `json:"variant"`
	Shape   string `json:"shape"`
}

// SupportedAccelerators lists the CPU, GPU, and TPU options for Google Colab.
var SupportedAccelerators = []AcceleratorSpec{
	{ID: "CPU", Label: "CPU (Standard)", Kind: "cpu", Variant: "VARIANT_UNSPECIFIED", Shape: "SHAPE_UNSPECIFIED"},
	{ID: "NVIDIA_TESLA_T4", Label: "GPU: NVIDIA Tesla T4", Kind: "gpu", Variant: "VARIANT_GPU", Shape: "SHAPE_UNSPECIFIED"},
	{ID: "NVIDIA_L4", Label: "GPU: NVIDIA L4", Kind: "gpu", Variant: "VARIANT_GPU", Shape: "SHAPE_HIGH_MEM"},
	{ID: "NVIDIA_TESLA_A100", Label: "GPU: NVIDIA A100", Kind: "gpu", Variant: "VARIANT_GPU", Shape: "SHAPE_HIGH_MEM"},
	{ID: "NVIDIA_H100_80GB", Label: "GPU: NVIDIA H100 80GB", Kind: "gpu", Variant: "VARIANT_GPU", Shape: "SHAPE_HIGH_MEM"},
	{ID: "TPU_V2_8", Label: "TPU: v2-8", Kind: "tpu", Variant: "VARIANT_TPU", Shape: "SHAPE_UNSPECIFIED"},
	{ID: "TPU_V5E_1", Label: "TPU: v5e-1", Kind: "tpu", Variant: "VARIANT_TPU", Shape: "SHAPE_UNSPECIFIED"},
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
	AccessToken string
	BaseURL     string
	HTTPClient  *http.Client
}

func NewClient(accessToken string) *Client {
	return &Client{
		AccessToken: strings.TrimSpace(accessToken),
		BaseURL:     DefaultBaseURL,
		HTTPClient:  &http.Client{Timeout: DefaultTimeout},
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

// CreateRuntime provisions a new Colab runtime for the requested accelerator.
func (c *Client) CreateRuntime(accelerator string) (*Runtime, error) {
	spec := LookupAccelerator(accelerator)
	runtimeSpec := map[string]any{
		"variant": spec.Variant,
		"shape":   spec.Shape,
	}
	if spec.Kind != "cpu" {
		runtimeSpec["accelerator"] = spec.ID
	}

	payload := map[string]any{
		"runtime": map[string]any{
			"runtimeSpec": runtimeSpec,
		},
	}
	reqID := url.QueryEscape(common.GetUUID())
	var out Runtime
	if err := c.do(http.MethodPost, "/runtimes?requestId="+reqID, payload, &out); err != nil {
		return nil, err
	}
	return &out, nil
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

// DeleteRuntime terminates a Colab runtime.
func (c *Client) DeleteRuntime(name string) error {
	path := "/" + strings.TrimLeft(name, "/")
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
	req, err := http.NewRequest(http.MethodPut, target, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
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
}

// StartKernel starts a Python Jupyter kernel on the Colab runtime.
func (c *Client) StartKernel(proxyURL, proxyToken string) (string, error) {
	target := strings.TrimRight(proxyURL, "/") + "/api/kernels"
	payload, err := common.Marshal(map[string]any{"name": "python3"})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, target, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if proxyToken != "" {
		req.Header.Set("X-Colab-Runtime-Proxy-Token", proxyToken)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", &APIError{Code: resp.StatusCode, Message: string(raw)}
	}
	var kernel struct {
		ID string `json:"id"`
	}
	_ = common.Unmarshal(raw, &kernel)
	return kernel.ID, nil
}
