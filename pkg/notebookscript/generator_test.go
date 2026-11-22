package notebookscript

import (
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
}
