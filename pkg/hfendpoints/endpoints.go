package hfendpoints

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	DefaultBaseURL = "https://api.endpoints.huggingface.cloud/v2"
	DefaultTimeout = 30 * time.Second
)

// HardwareOption maps the hardware keys offered in System Settings to the
// compute block of the Inference Endpoints API.
type HardwareOption struct {
	Accelerator  string
	InstanceType string
	InstanceSize string
}

var hardwareOptions = map[string]HardwareOption{
	"cpu-basic":     {Accelerator: "cpu", InstanceSize: "x2"},
	"hf-cpu-basic":  {Accelerator: "cpu", InstanceSize: "x2"},
	"t4-small":      {Accelerator: "gpu", InstanceSize: "x1", InstanceType: "nvidia-t4"},
	"hf-t4-small":   {Accelerator: "gpu", InstanceSize: "x1", InstanceType: "nvidia-t4"},
	"a10g-small":    {Accelerator: "gpu", InstanceSize: "x1", InstanceType: "nvidia-a10g"},
	"hf-a10g-small": {Accelerator: "gpu", InstanceSize: "x1", InstanceType: "nvidia-a10g"},
	"a100-large":    {Accelerator: "gpu", InstanceSize: "x1", InstanceType: "nvidia-a100"},
	"hf-a100-large": {Accelerator: "gpu", InstanceSize: "x1", InstanceType: "nvidia-a100"},
}

// LookupHardware returns the compute block for a settings hardware key.
// Unknown keys fall back to a T4 GPU so requests never fail silently.
func LookupHardware(key string) HardwareOption {
	if opt, ok := hardwareOptions[key]; ok {
		return opt
	}
	return hardwareOptions["t4-small"]
}

// HardwareKeys lists the accepted hardware keys.
func HardwareKeys() []string {
	keys := make([]string, 0, len(hardwareOptions))
	for k := range hardwareOptions {
		keys = append(keys, k)
	}
	return keys
}

type Client struct {
	Token      string
	BaseURL    string
	HTTPClient *http.Client
}

func NewClient(token string) *Client {
	return &Client{
		Token:      token,
		BaseURL:    DefaultBaseURL,
		HTTPClient: &http.Client{Timeout: DefaultTimeout},
	}
}

type APIError struct {
	Code    int
	Message string
	Details string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("huggingface: %s", e.Message)
	}
	return fmt.Sprintf("huggingface: request failed with status %d", e.Code)
}

func (c *Client) do(method, path string, body []byte) ([]byte, error) {
	req, err := http.NewRequest(method, c.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, &APIError{Code: resp.StatusCode, Message: string(data), Details: string(data)}
	}
	return data, nil
}

type EndpointState struct {
	State     string `json:"state"`
	URL       string `json:"url"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
	Message   string `json:"message"`
}

type EndpointModel struct {
	Repository string `json:"repository"`
	Revision   string `json:"revision"`
	Task       string `json:"task"`
	Framework  string `json:"framework"`
}

type EndpointCompute struct {
	Accelerator  string `json:"accelerator"`
	InstanceSize string `json:"instanceSize"`
	InstanceType string `json:"instanceType"`
}

type Endpoint struct {
	Name      string         `json:"name"`
	Namespace string         `json:"namespace"`
	Status    EndpointState  `json:"status"`
	Model     EndpointModel  `json:"model"`
	Compute   EndpointCompute `json:"compute"`
}

type listResponse struct {
	Items []Endpoint `json:"items"`
}

// ListEndpoints returns the endpoints of a namespace (user or org).
func (c *Client) ListEndpoints(namespace string) ([]Endpoint, error) {
	data, err := c.do(http.MethodGet, "/endpoint/"+url.PathEscape(namespace), nil)
	if err != nil {
		return nil, err
	}
	var parsed listResponse
	if err := unmarshal(data, &parsed); err != nil {
		// Some API versions return a bare array.
		var bare []Endpoint
		if err2 := unmarshal(data, &bare); err2 != nil {
			return nil, err
		}
		for i := range bare {
			bare[i].Namespace = namespace
		}
		return bare, nil
	}
	for i := range parsed.Items {
		parsed.Items[i].Namespace = namespace
	}
	return parsed.Items, nil
}

type CreateEndpointRequest struct {
	Name       string
	Namespace  string
	Repository string
	Revision   string
	Task       string
	Framework  string
	Hardware   string
	Region     string
	Protected  bool
}

// BuildCreatePayload renders the Inference Endpoints v2 create body.
func BuildCreatePayload(req CreateEndpointRequest) map[string]any {
	hw := LookupHardware(req.Hardware)
	revision := req.Revision
	if revision == "" {
		revision = "main"
	}
	framework := req.Framework
	if framework == "" {
		framework = "pytorch"
	}
	endpointType := "public"
	if req.Protected {
		endpointType = "private"
	}
	compute := map[string]any{
		"accelerator":  hw.Accelerator,
		"instanceSize": hw.InstanceSize,
		"scaling": map[string]any{
			"minReplica":          0,
			"maxReplica":          1,
			"scaleToZeroTimeout": 15,
		},
	}
	if hw.InstanceType != "" {
		compute["instanceType"] = hw.InstanceType
	}
	return map[string]any{
		"name": req.Name,
		"compute": compute,
		"model": map[string]any{
			"framework":  framework,
			"repository": req.Repository,
			"revision":   revision,
			"task":       req.Task,
			"image": map[string]any{
				"huggingface": map[string]any{
					"hubModelId": req.Repository,
					"revision":   revision,
					"task":       req.Task,
				},
			},
		},
		"provider": map[string]any{
			"region": req.Region,
			"vendor": "aws",
		},
		"type": endpointType,
	}
}

// CreateEndpoint creates an inference endpoint in a namespace.
func (c *Client) CreateEndpoint(req CreateEndpointRequest) (*Endpoint, error) {
	body, err := marshal(BuildCreatePayload(req))
	if err != nil {
		return nil, err
	}
	data, err := c.do(http.MethodPost, "/endpoint/"+url.PathEscape(req.Namespace), body)
	if err != nil {
		return nil, err
	}
	var created Endpoint
	if err := unmarshal(data, &created); err != nil {
		return nil, err
	}
	return &created, nil
}

// GetEndpoint fetches one endpoint by namespace and name.
func (c *Client) GetEndpoint(namespace, name string) (*Endpoint, error) {
	data, err := c.do(http.MethodGet, "/endpoint/"+url.PathEscape(namespace)+"/"+url.PathEscape(name), nil)
	if err != nil {
		return nil, err
	}
	var endpoint Endpoint
	if err := unmarshal(data, &endpoint); err != nil {
		return nil, err
	}
	endpoint.Namespace = namespace
	return &endpoint, nil
}

// DeleteEndpoint removes an endpoint by namespace and name.
func (c *Client) DeleteEndpoint(namespace, name string) error {
	_, err := c.do(http.MethodDelete, "/endpoint/"+url.PathEscape(namespace)+"/"+url.PathEscape(name), nil)
	return err
}
