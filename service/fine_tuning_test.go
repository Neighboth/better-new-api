package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
