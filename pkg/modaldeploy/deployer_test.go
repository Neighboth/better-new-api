package modaldeploy

import (
	"context"
	"strings"
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

func TestModalCmdConstruction(t *testing.T) {
	ctx := context.Background()
	direct := modalCmd{bin: "modal"}
	c1 := direct.command(ctx, "deploy", "app.py")
	assert.Equal(t, []string{"modal", "deploy", "app.py"}, c1.Args)

	pyMod := modalCmd{bin: "py", prefixArgs: []string{"-3", "-m", "modal"}}
	c2 := pyMod.command(ctx, "app", "stop", "demo")
	assert.Equal(t, []string{"py", "-3", "-m", "modal", "app", "stop", "demo"}, c2.Args)

	assert.NotEmpty(t, commonModalCandidates())
}

func TestBuildAppScriptNoTopLevelMLImports(t *testing.T) {
	cases := []DeployConfig{
		{AppName: "txt", ModelRepo: "Qwen/Qwen2.5-0.5B-Instruct", TaskType: "text", Accelerator: "T4"},
		{AppName: "gguf", ModelRepo: "Qwen/Qwen2.5-0.5B-Instruct-GGUF", TaskType: "text", Accelerator: "CPU"},
		{AppName: "img", ModelRepo: "stabilityai/sdxl", TaskType: "image", Accelerator: "L4"},
		{AppName: "vid", ModelRepo: "Wan-AI/Wan2.1", TaskType: "video", Accelerator: "A100"},
		{AppName: "aud", ModelRepo: "suno/bark", TaskType: "audio", Accelerator: "T4"},
		{AppName: "emb", ModelRepo: "BAAI/bge-small-en-v1.5", TaskType: "embeddings", Accelerator: "CPU"},
	}

	mlPrefixes := []string{
		"from huggingface_hub",
		"from diffusers",
		"from transformers",
		"from sentence_transformers",
		"from llama_cpp",
		"import torch",
		"import scipy",
		"@web_app.post",
	}

	for _, cfg := range cases {
		script := BuildAppScript(cfg)
		for _, line := range strings.Split(script, "\n") {
			for _, pfx := range mlPrefixes {
				assert.Falsef(t, strings.HasPrefix(line, pfx), "task=%s repo=%s had unindented line: %s", cfg.TaskType, cfg.ModelRepo, line)
			}
		}
		assert.Contains(t, script, "    @web_app.post(")
	}

	ggufScript := BuildAppScript(cases[1])
	assert.NotContains(t, ggufScript, "import torch")
	assert.Contains(t, ggufScript, "extra_index_url=")

	textScript := BuildAppScript(cases[0])
	assert.NotContains(t, textScript, "llama-cpp-python")
}
