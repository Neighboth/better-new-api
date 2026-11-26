package service

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseModelBillions(t *testing.T) {
	tests := []struct {
		name      string
		model     string
		hint      string
		fileBytes int64
		minB      float64
		maxB      float64
	}{
		{name: "0.5B small model", model: "Qwen/Qwen2.5-0.5B-Instruct", minB: 0.49, maxB: 0.51},
		{name: "7B model", model: "Qwen/Qwen2.5-7B-Instruct", minB: 6.9, maxB: 7.1},
		{name: "70B model", model: "meta-llama/Llama-3.1-70B-Instruct", minB: 69, maxB: 71},
		{name: "DeepSeek R1", model: "deepseek-ai/DeepSeek-R1", minB: 600, maxB: 700},
		{name: "DeepSeek R1 Distill 8B", model: "deepseek-ai/DeepSeek-R1-Distill-Llama-8B", minB: 7.9, maxB: 8.1},
		{name: "Param hint takes precedence", model: "org/custom-model", hint: "32B", minB: 31.9, maxB: 32.1},
		{name: "Uploaded safetensors size fallback", model: "custom.safetensors", fileBytes: 4_000_000_000, minB: 1.9, maxB: 2.1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseModelBillions(tc.model, tc.hint, tc.fileBytes)
			assert.GreaterOrEqual(t, got, tc.minB)
			assert.LessOrEqual(t, got, tc.maxB)
		})
	}
}

func TestAcceleratorSpeedFactorMultiGPU(t *testing.T) {
	singleT4 := GetAcceleratorSpeedFactor("NVIDIA_TESLA_T4")
	dualT4 := GetAcceleratorSpeedFactor("NvidiaTeslaT4") // Kaggle T4 x2
	h100x1 := GetAcceleratorSpeedFactor("H100")
	h100x8 := GetAcceleratorSpeedFactor("h100x8")
	cpu := GetAcceleratorSpeedFactor("CPU")

	assert.InDelta(t, 1.0, singleT4, 0.01)
	assert.Greater(t, dualT4, singleT4*1.7, "Kaggle 2x T4 should be meaningfully faster than 1x T4")
	assert.Greater(t, h100x1, dualT4*4)
	assert.Greater(t, h100x8, h100x1*6)
	assert.Less(t, cpu, singleT4*0.2)
}

func TestEstimateFineTuningDurationRealistic(t *testing.T) {
	// 1. Dataset sample count: 65,000 short lines (~1MB) on 7B T4 x1 should take hours, not 1 minute
	manyShortRows := EstimateFineTuningDurationRealistic(FineTuningEstimateInput{
		GPU:          "NVIDIA_TESLA_T4",
		TaskType:       "text",
		Method:         "lora",
		Epochs:         3,
		DatasetBytes:   1_000_000,
		DatasetSamples: 65_000,
		ModelName:      "Qwen/Qwen2.5-7B-Instruct",
	})
	assert.Greater(t, manyShortRows, int64(3600), "65,000 samples x 3 epochs on 7B T4 must exceed 1 hour")

	// 2. Dual T4 (Kaggle) is faster than single T4 (Colab/Modal)
	dualT4Dur := EstimateFineTuningDurationRealistic(FineTuningEstimateInput{
		GPU:          "NvidiaTeslaT4",
		TaskType:       "text",
		Method:         "lora",
		Epochs:         3,
		DatasetBytes:   1_000_000,
		DatasetSamples: 65_000,
		ModelName:      "Qwen/Qwen2.5-7B-Instruct",
	})
	assert.Less(t, dualT4Dur, manyShortRows, "2x T4 must estimate shorter duration than 1x T4")

	// 3. Larger model (70B) takes much longer than 0.5B on identical dataset
	tinyModel := EstimateFineTuningDurationRealistic(FineTuningEstimateInput{
		GPU:          "A100-80GB",
		TaskType:       "text",
		Method:         "lora",
		Epochs:         3,
		DatasetBytes:   5_000_000,
		DatasetSamples: 5_000,
		ModelName:      "Qwen/Qwen2.5-0.5B-Instruct",
	})
	hugeModel := EstimateFineTuningDurationRealistic(FineTuningEstimateInput{
		GPU:          "A100-80GB",
		TaskType:       "text",
		Method:         "lora",
		Epochs:         3,
		DatasetBytes:   5_000_000,
		DatasetSamples: 5_000,
		ModelName:      "meta-llama/Llama-3.1-70B-Instruct",
	})
	assert.Greater(t, hugeModel, tinyModel*10, "70B model must estimate substantially longer duration than 0.5B model")

	// 4. Legacy wrapper still scales cleanly with dataset size & GPU speed
	small := EstimateFineTuningDurationSeconds("NVIDIA_TESLA_T4", "text", 3, 30*1024)
	large := EstimateFineTuningDurationSeconds("NVIDIA_TESLA_T4", "text", 3, 50*1024*1024)
	assert.Greater(t, large, small*5)
}

func TestSanitizeFineTuningLogLineRedactsProvidersAndSecrets(t *testing.T) {
	raw := "Connecting Google Colab worker at https://foo.trycloudflare.com 10.0.0.5 with token=sk-secret123456 on Kaggle"
	clean := SanitizeFineTuningLogLine(raw)
	assert.NotContains(t, clean, "Colab")
	assert.NotContains(t, clean, "Kaggle")
	assert.NotContains(t, clean, "trycloudflare")
	assert.NotContains(t, clean, "10.0.0.5")
	assert.NotContains(t, clean, "sk-secret123456")
}

func TestHugeSafetensorsFullFineTuningRealisticCostAndCompatWarning(t *testing.T) {
	in := FineTuningEstimateInput{
		GPU:            "NVIDIA_TESLA_T4",
		RamTier:        "normal",
		TaskType:       "text",
		Method:         "full",
		Epochs:         3,
		DatasetBytes:   16 * 1024,
		DatasetSamples: 55,
		ModelName:      "org/custom-huge-model",
		ModelFileBytes: 94 * 1_000_000_000, // 94 GB safetensors
	}

	compat := CheckFineTuningHardwareCompat(in)
	assert.False(t, compat.Compatible, "94 GB Full FT must trigger incompatibility warning on 1x T4 (16GB VRAM)")
	assert.Equal(t, "vram_exceeded", compat.Code)
	assert.Greater(t, compat.RequiredVRAMGB, 300.0)
	assert.Equal(t, 16.0, compat.AvailableVRAMGB)

	dur := EstimateFineTuningDurationRealistic(in)
	assert.Greater(t, dur, int64(5400), "94 GB Full FT on 1x T4 must estimate >1.5h, never a few minutes")

	costUSD := EstimateFineTuningCostUSDWithRAM("NVIDIA_TESLA_T4", "normal", dur)
	assert.Greater(t, costUSD, 8.0, "94 GB Full FT on 1x T4 must never estimate ~$0.50")

	// Compatible small model on 1x T4 must NOT emit any warning
	smallCompat := CheckFineTuningHardwareCompat(FineTuningEstimateInput{
		GPU:            "NVIDIA_TESLA_T4",
		RamTier:        "normal",
		TaskType:       "text",
		Method:         "lora",
		Epochs:         3,
		DatasetBytes:   64 * 1024,
		DatasetSamples: 200,
		ModelName:      "HuggingFaceTB/SmolLM2-135M",
		ModelFileBytes: 270 * 1_000_000,
	})
	assert.True(t, smallCompat.Compatible, "SmolLM2-135M LoRA on 1x T4 must be compatible with no warning")
	assert.Empty(t, smallCompat.Reason)
}

func TestValidateFineTuningDatasetFileStrict(t *testing.T) {
	// Reject .zip for text fine-tuning
	err := ValidateFineTuningDatasetFile("text", "dataset.zip", []byte{'P', 'K', 0x03, 0x04})
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "zip")

	// Reject disguised ZIP renamed to .jsonl
	err = ValidateFineTuningDatasetFile("text", "fake.jsonl", []byte{'P', 'K', 0x03, 0x04, 0x14, 0x00})
	require.Error(t, err)

	// Accept genuine JSONL for text
	err = ValidateFineTuningDatasetFile("text", "train.jsonl", []byte(`{"messages":[{"role":"user","content":"hi"}]}`+"\n"))
	require.NoError(t, err)

	// Accept genuine ZIP for image fine-tuning, reject fake non-ZIP
	err = ValidateFineTuningDatasetFile("image", "photos.zip", []byte{'P', 'K', 0x03, 0x04, 0x14, 0x00})
	require.NoError(t, err)
	err = ValidateFineTuningDatasetFile("image", "photos.zip", []byte("plain text not zip"))
	require.Error(t, err)
}

func TestAppendFineTuningLogSingleTimestampAndDedup(t *testing.T) {
	jobID := "ftjob-log-dedup-test"
	defer func() {
		_ = os.RemoveAll(FineTuningStorageDirForUser(999999, jobID))
		ftLogMu.Lock()
		delete(ftRecentLogLines, jobID)
		ftLogMu.Unlock()
	}()

	// Simulate the exact Colab bug: callback + stdout poller both reporting the same timestamped line
	appendFineTuningLog(999999, jobID, "train", "13:10:55 [env] PyTorch 2.10 ready | device=Tesla T4")
	appendFineTuningLog(999999, jobID, "worker", "13:10:55 [env] PyTorch 2.10 ready | device=Tesla T4")

	logs := ReadFineTuningJobLogs(999999, jobID)
	lines := strings.Split(strings.TrimSpace(logs), "\n")
	require.Len(t, lines, 1, "duplicate callback + worker stdout line must appear only once")
	assert.Equal(t, "13:10:55 [env] PyTorch 2.10 ready | device=Tesla T4", lines[0], "must not double-prepend timestamp or [train] tag")
}

func TestValidateOpenAIDatasetContentStrict(t *testing.T) {
	validChat := `{"messages":[{"role":"system","content":"You are helpful."},{"role":"user","content":"Hello"},{"role":"assistant","content":"Hi there!"}]}` + "\n" +
		`{"messages":[{"role":"user","content":"2+2?"},{"role":"assistant","content":"4"}]}` + "\n"
	res, err := ValidateOpenAIDatasetContent("text", "lora", "train.jsonl", strings.NewReader(validChat))
	require.NoError(t, err)
	assert.True(t, res.Valid)
	assert.Equal(t, int64(2), res.SampleCount)
	assert.False(t, res.HasImages)

	// Reject non-.jsonl extension even with valid JSONL body
	_, err = ValidateOpenAIDatasetContent("text", "lora", "train.txt", strings.NewReader(validChat))
	require.Error(t, err)

	// Reject Alpaca format
	alpaca := `{"instruction":"Say hi","input":"","output":"Hi"}` + "\n"
	_, err = ValidateOpenAIDatasetContent("text", "lora", "alpaca.jsonl", strings.NewReader(alpaca))
	require.Error(t, err)

	// Reject raw text format
	rawText := `{"text":"hello world"}` + "\n"
	_, err = ValidateOpenAIDatasetContent("text", "lora", "raw.jsonl", strings.NewReader(rawText))
	require.Error(t, err)

	// Reject invalid role
	badRole := `{"messages":[{"role":"human","content":"hi"},{"role":"bot","content":"hello"}]}` + "\n"
	_, err = ValidateOpenAIDatasetContent("text", "lora", "badrole.jsonl", strings.NewReader(badRole))
	require.Error(t, err)

	// Reject missing assistant reply
	noAssistant := `{"messages":[{"role":"system","content":"sys"},{"role":"user","content":"only user"}]}` + "\n"
	_, err = ValidateOpenAIDatasetContent("text", "lora", "noasst.jsonl", strings.NewReader(noAssistant))
	require.Error(t, err)

	// Accept valid OpenAI DPO JSONL, reject chat format when DPO selected
	validDPO := `{"input":{"messages":[{"role":"user","content":"Explain gravity"}]},"preferred_output":[{"role":"assistant","content":"Gravity attracts masses."}],"non_preferred_output":[{"role":"assistant","content":"Idk"}]}` + "\n"
	resDPO, err := ValidateOpenAIDatasetContent("text", "dpo", "dpo.jsonl", strings.NewReader(validDPO))
	require.NoError(t, err)
	assert.Equal(t, int64(1), resDPO.SampleCount)
	_, err = ValidateOpenAIDatasetContent("text", "dpo", "chat_not_dpo.jsonl", strings.NewReader(validChat))
	require.Error(t, err)

	// Vision multimodal image_url dataset: accepted for vision, rejected for text-only task
	visionChat := `{"messages":[{"role":"user","content":[{"type":"text","text":"What is this?"},{"type":"image_url","image_url":{"url":"https://example.com/a.jpg"}}]},{"role":"assistant","content":"A cat."}]}` + "\n"
	resVis, err := ValidateOpenAIDatasetContent("vision", "lora", "vision.jsonl", strings.NewReader(visionChat))
	require.NoError(t, err)
	assert.True(t, resVis.HasImages)
	_, err = ValidateOpenAIDatasetContent("text", "lora", "vision.jsonl", strings.NewReader(visionChat))
	require.Error(t, err)
}

func TestCheckFineTuningModelCompatibilityLocalArchives(t *testing.T) {
	tmp := t.TempDir()

	// 1. Reject GGUF immediately
	rGGUF := CheckFineTuningModelCompatibility(context.Background(), "org/model-Q4_K_M.gguf", "huggingface", "", "text", "lora", false, "")
	assert.False(t, rGGUF.Compatible)

	// 2. Reject bare .safetensors without config.json & tokenizer for text task
	barePath := filepath.Join(tmp, "weights.safetensors")
	require.NoError(t, os.WriteFile(barePath, []byte("dummy"), 0o644))
	rBare := CheckFineTuningModelCompatibility(context.Background(), "weights.safetensors", "upload", barePath, "text", "lora", false, "")
	assert.False(t, rBare.Compatible)

	makeModelZip := func(name string, configJSON string) string {
		p := filepath.Join(tmp, name)
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		w1, _ := zw.Create("config.json")
		_, _ = w1.Write([]byte(configJSON))
		w2, _ := zw.Create("tokenizer.json")
		_, _ = w2.Write([]byte(`{}`))
		w3, _ := zw.Create("model.safetensors")
		_, _ = w3.Write([]byte("weights"))
		require.NoError(t, zw.Close())
		require.NoError(t, os.WriteFile(p, buf.Bytes(), 0o644))
		return p
	}

	// 3. Compatible LlamaForCausalLM zip for text task
	llamaZip := makeModelZip("llama.zip", `{"architectures":["LlamaForCausalLM"],"model_type":"llama"}`)
	rLlama := CheckFineTuningModelCompatibility(context.Background(), "llama.zip", "upload", llamaZip, "text", "full", false, "")
	assert.True(t, rLlama.Compatible, rLlama.Reason)
	assert.Equal(t, "LlamaForCausalLM", rLlama.Architecture)

	// 4. Reject text-only LlamaForCausalLM when dataset has multimodal images
	rLlamaVis := CheckFineTuningModelCompatibility(context.Background(), "llama.zip", "upload", llamaZip, "text", "lora", true, "")
	assert.False(t, rLlamaVis.Compatible)

	// 5. Reject encoder-only BertModel for OpenAI chat fine-tuning
	bertZip := makeModelZip("bert.zip", `{"architectures":["BertModel"],"model_type":"bert"}`)
	rBert := CheckFineTuningModelCompatibility(context.Background(), "bert.zip", "upload", bertZip, "text", "lora", false, "")
	assert.False(t, rBert.Compatible)
}
