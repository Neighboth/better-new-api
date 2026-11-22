package kaggle

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
	DefaultBaseURL = "https://www.kaggle.com/api/v1"
	DefaultTimeout = 30 * time.Second
)

// AcceleratorSpec describes a Kaggle Kernel hardware accelerator option.
type AcceleratorSpec struct {
	ID           string `json:"id"`
	Label        string `json:"label"`
	Kind         string `json:"kind"` // cpu, gpu, tpu
	EnableGPU    bool   `json:"enable_gpu"`
	EnableTPU    bool   `json:"enable_tpu"`
	MachineShape string `json:"machine_shape,omitempty"`
}

// SupportedAccelerators lists every CPU, GPU, and TPU option available on Kaggle.
var SupportedAccelerators = []AcceleratorSpec{
	{ID: "CPU", Label: "CPU (4 cores, 30 GB RAM)", Kind: "cpu", EnableGPU: false, EnableTPU: false, MachineShape: ""},
	{ID: "NvidiaTeslaT4", Label: "GPU: NVIDIA Tesla T4 x2", Kind: "gpu", EnableGPU: true, EnableTPU: false, MachineShape: "NvidiaTeslaT4"},
	{ID: "NvidiaP100", Label: "GPU: NVIDIA Tesla P100", Kind: "gpu", EnableGPU: true, EnableTPU: false, MachineShape: "NvidiaP100"},
	{ID: "NvidiaL4", Label: "GPU: NVIDIA L4", Kind: "gpu", EnableGPU: true, EnableTPU: false, MachineShape: "NvidiaL4"},
	{ID: "TpuV38", Label: "TPU: v3-8", Kind: "tpu", EnableGPU: false, EnableTPU: true, MachineShape: ""},
	{ID: "TpuV5E8", Label: "TPU: v5e-8", Kind: "tpu", EnableGPU: false, EnableTPU: true, MachineShape: "TpuV5E8"},
	{ID: "TpuV6E8", Label: "TPU: v6e-8", Kind: "tpu", EnableGPU: false, EnableTPU: true, MachineShape: "TpuV6E8"},
}

// LookupAccelerator resolves an accelerator ID to its Kaggle kernel flags.
func LookupAccelerator(id string) AcceleratorSpec {
	trimmed := strings.TrimSpace(id)
	for _, spec := range SupportedAccelerators {
		if strings.EqualFold(spec.ID, trimmed) {
			return spec
		}
	}
	return SupportedAccelerators[1] // NvidiaTeslaT4 default
}

type Client struct {
	Username   string
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
}

func NewClient(username, apiKey string) *Client {
	return &Client{
		Username:   strings.TrimSpace(username),
		APIKey:     strings.TrimSpace(apiKey),
		BaseURL:    DefaultBaseURL,
		HTTPClient: &http.Client{Timeout: DefaultTimeout},
	}
}

type APIError struct {
	Code    int
	Message string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("kaggle (HTTP %d): %s", e.Code, e.Message)
	}
	return fmt.Sprintf("kaggle: request failed with HTTP %d", e.Code)
}

func (c *Client) applyAuth(req *http.Request) {
	if strings.HasPrefix(c.APIKey, "KGAT_") || c.Username == "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
		return
	}
	req.SetBasicAuth(c.Username, c.APIKey)
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
	c.applyAuth(req)
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
			return fmt.Errorf("kaggle: decode response: %w", err)
		}
	}
	return nil
}

// TestConnection verifies Kaggle credentials by listing the user's kernels.
func (c *Client) TestConnection() ([]map[string]any, error) {
	var out []map[string]any
	if err := c.do(http.MethodGet, "/kernels/list?group=profile&pageSize=1", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

type PushKernelRequest struct {
	Slug        string
	Title       string
	ScriptText  string
	Accelerator string
	IsPrivate   bool
}

type PushKernelResponse struct {
	Ref           string `json:"ref"`
	URL           string `json:"url"`
	VersionNumber int    `json:"versionNumber"`
	Error         string `json:"error"`
}

// PushKernel pushes a Python script kernel to Kaggle and triggers execution.
func (c *Client) PushKernel(req PushKernelRequest) (*PushKernelResponse, error) {
	spec := LookupAccelerator(req.Accelerator)
	payload := map[string]any{
		"slug":                   req.Slug,
		"newTitle":               req.Title,
		"text":                   req.ScriptText,
		"language":               "python",
		"kernelType":             "script",
		"isPrivate":              req.IsPrivate,
		"enableGpu":              spec.EnableGPU,
		"enableTpu":              spec.EnableTPU,
		"enableInternet":         true,
		"datasetDataSources":     []string{},
		"competitionDataSources": []string{},
		"kernelDataSources":      []string{},
		"modelDataSources":       []string{},
	}
	if spec.MachineShape != "" {
		payload["machineShape"] = spec.MachineShape
	}
	var out PushKernelResponse
	if err := c.do(http.MethodPost, "/kernels/push", payload, &out); err != nil {
		return nil, err
	}
	if strings.TrimSpace(out.Error) != "" {
		return nil, fmt.Errorf("kaggle push error: %s", out.Error)
	}
	return &out, nil
}

type KernelStatus struct {
	Slug           string `json:"slug"`
	Title          string `json:"title"`
	Status         string `json:"status"`
	FailureMessage string `json:"failureMessage"`
	LastRunTime    string `json:"lastRunTime"`
}

// GetKernelStatus checks the latest run status of a Kaggle kernel.
func (c *Client) GetKernelStatus(username, kernelSlug string) (*KernelStatus, error) {
	q := url.Values{
		"userName":   []string{username},
		"kernelSlug": []string{kernelSlug},
	}
	var out KernelStatus
	if err := c.do(http.MethodGet, "/kernels/status?"+q.Encode(), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteKernel deletes a Kaggle kernel.
func (c *Client) DeleteKernel(username, kernelSlug string) error {
	path := fmt.Sprintf("/kernels/%s/%s", url.PathEscape(username), url.PathEscape(kernelSlug))
	return c.do(http.MethodDelete, path, nil, nil)
}
