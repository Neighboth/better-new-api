package service

import (
	"encoding/binary"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildValidSafetensorsSpecCompliance(t *testing.T) {
	job := &model.FineTuningJob{
		JobID:          "ftjob-test123",
		Model:          "Qwen/Qwen2.5-0.5B",
		FineTunedModel: "ft:Qwen/Qwen2.5-0.5B:test123",
	}
	raw, err := buildValidSafetensors(job, "text", "lora", "abc123", 0.42)
	require.NoError(t, err)
	require.Greater(t, len(raw), 16)

	headerLen := binary.LittleEndian.Uint64(raw[:8])
	require.Less(t, uint64(8)+headerLen, uint64(len(raw)))

	var header map[string]any
	require.NoError(t, common.Unmarshal(raw[8:8+headerLen], &header))
	meta, ok := header["__metadata__"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "pt", meta["format"])
	assert.Equal(t, "Qwen/Qwen2.5-0.5B", meta["base_model"])

	// Verify tensor offsets match exact binary payload length
	payloadLen := len(raw) - int(8+headerLen)
	maxEnd := 0
	tensorCount := 0
	for k, v := range header {
		if k == "__metadata__" {
			continue
		}
		tensorCount++
		entry := v.(map[string]any)
		offsets := entry["data_offsets"].([]any)
		end := int(offsets[1].(float64))
		if end > maxEnd {
			maxEnd = end
		}
	}
	assert.Equal(t, 4, tensorCount)
	assert.Equal(t, payloadLen, maxEnd)
}

func TestEstimateFineTuningDurationScalesWithDatasetSize(t *testing.T) {
	small := EstimateFineTuningDurationSeconds("NVIDIA_TESLA_T4", "text", 3, 30*1024)
	large := EstimateFineTuningDurationSeconds("NVIDIA_TESLA_T4", "text", 3, 50*1024*1024)
	assert.Greater(t, large, small*10, "50MB dataset must estimate substantially longer duration than 30KB dataset")

	fastGPU := EstimateFineTuningDurationSeconds("NVIDIA_H100_80GB", "text", 3, 50*1024*1024)
	assert.Less(t, fastGPU, large, "H100 must estimate shorter duration than T4 for same dataset")
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
