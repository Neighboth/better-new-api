package notebookscript

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeTaskType(t *testing.T) {
	assert.Equal(t, "video", NormalizeTaskType("text-to-video"))
	assert.Equal(t, "image", NormalizeTaskType("text-to-image"))
	assert.Equal(t, "audio", NormalizeTaskType("text-to-speech"))
	assert.Equal(t, "embeddings", NormalizeTaskType("feature-extraction"))
	assert.Equal(t, "text", NormalizeTaskType("text-generation"))
}

func TestGenerateScriptsForTasks(t *testing.T) {
	for _, task := range []string{"text", "image", "video", "audio", "embeddings"} {
		script := Generate(Config{
			Provider:           "kaggle",
			DeploymentName:     "demo",
			ModelRepo:          "org/model",
			TaskType:           task,
			Accelerator:        "NvidiaTeslaT4",
			IdleTimeoutSeconds: 300,
		})
		assert.Contains(t, script, `MODEL_REPO = "org/model"`)
		assert.Contains(t, script, `IDLE_TIMEOUT_SECONDS = 300`)
		assert.Contains(t, script, `uvicorn.run`)
	}

	videoScript := Generate(Config{ModelRepo: "Wan-AI/Wan2.1", TaskType: "video"})
	assert.Contains(t, videoScript, "/v1/videos/generations")
	assert.Contains(t, videoScript, "export_to_video")

	ggufScript := Generate(Config{
		Provider:    "kaggle",
		ModelRepo:   "Qwen/Qwen2.5-0.5B-Instruct-GGUF",
		TaskType:    "text",
		Accelerator: "CPU",
		CallbackURL: "https://example.com/api/deployments/notebooks/1/callback?token=abc",
	})
	assert.Contains(t, ggufScript, `llama_cpp`)
	assert.Contains(t, ggufScript, `--prefer-binary`)
	assert.NotContains(t, ggufScript, `import torch`)
	assert.Contains(t, ggufScript, `endpoint_url.txt`)
	assert.Contains(t, ggufScript, `trycloudflare\.com`)
	assert.Contains(t, ggufScript, `CALLBACK_URL = "https://example.com/api/deployments/notebooks/1/callback?token=abc"`)
	assert.Less(t, strings.Index(ggufScript, "threading.Thread(target=_start_cloudflared, daemon=True).start()"), strings.Index(ggufScript, "_ensure_packages("))

	stdTextScript := Generate(Config{
		Provider:  "colab",
		ModelRepo: "Qwen/Qwen2.5-0.5B-Instruct",
		TaskType:  "text",
	})
	assert.NotContains(t, stdTextScript, `llama-cpp-python`)
	assert.Contains(t, stdTextScript, `import torch`)

	tpuGGUFScript := Generate(Config{
		Provider:    "kaggle",
		ModelRepo:   "Qwen/Qwen2.5-0.5B-Instruct-GGUF",
		TaskType:    "text",
		Accelerator: "TpuV5E8",
	})
	assert.Contains(t, tpuGGUFScript, `https://abetlen.github.io/llama-cpp-python/whl/cpu`)
	assert.Contains(t, tpuGGUFScript, `os.path.exists("/dev/nvidia0")`)
}
