package modaldeploy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildAppScriptModal(t *testing.T) {
	script := BuildAppScript(DeployConfig{
		AppName:              "qwen-gguf",
		ModelRepo:            "Qwen/Qwen2.5-0.5B-Instruct-GGUF",
		TaskType:             "text",
		Accelerator:          "CPU",
		FallbackAccelerators: []string{"T4", "L4"},
		IdleTimeoutSeconds:   300,
	})
	assert.Contains(t, script, `@modal.asgi_app()`)
	assert.Contains(t, script, `gpu=["T4", "L4"]`)
	assert.Contains(t, script, `scaledown_window=300`)
	assert.Contains(t, script, `llama-cpp-python`)

	cpuOnly := BuildAppScript(DeployConfig{
		AppName:            "cpu-only",
		ModelRepo:          "Qwen/Qwen2.5-0.5B-Instruct",
		TaskType:           "text",
		Accelerator:        "CPU",
		IdleTimeoutSeconds: 0,
	})
	assert.Contains(t, cpuOnly, `gpu=None`)
	assert.NotContains(t, cpuOnly, `scaledown_window`)
}
