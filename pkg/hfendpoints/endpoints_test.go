package hfendpoints

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLookupHardware(t *testing.T) {
	assert.Equal(t, "nvidia-t4", LookupHardware("t4-small").InstanceType)
	assert.Equal(t, "cpu", LookupHardware("cpu-basic").Accelerator)
	assert.Equal(t, "nvidia-a100", LookupHardware("hf-a100-large").InstanceType)
	assert.Equal(t, "nvidia-t4", LookupHardware("unknown-key").InstanceType)
}

func TestBuildCreatePayload(t *testing.T) {
	payload := BuildCreatePayload(CreateEndpointRequest{
		Name:       "my-llm",
		Repository: "meta-llama/Llama-3-8B",
		Task:       "text-generation",
		Hardware:   "t4-small",
		Region:     "us-east-1",
		Protected:  true,
	})
	require.Equal(t, "my-llm", payload["name"])
	require.Equal(t, "protected", payload["type"])
	model, ok := payload["model"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "meta-llama/Llama-3-8B", model["repository"])
	assert.Equal(t, "main", model["revision"])
	compute, ok := payload["compute"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "nvidia-t4", compute["instanceType"])
	provider, ok := payload["provider"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "us-east-1", provider["region"])
}
