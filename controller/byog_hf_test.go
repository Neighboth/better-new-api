package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHuggingFaceParameterSize(t *testing.T) {
	assert.Equal(t, "~2B", huggingFaceParameterSize("vngrs/Kumru-2B", nil))
	assert.Equal(t, "1.6T", huggingFaceParameterSize("org/model", map[string]interface{}{"safetensors": map[string]interface{}{"total": 1.6e12}}))
	assert.Equal(t, "~120B", huggingFaceParameterSize("org/model-120B-Instruct", nil))
	assert.Equal(t, "", huggingFaceParameterSize("org/Mixtral-8x7B", nil))
	assert.Equal(t, "", huggingFaceParameterSize("org/unknown", nil))
}

func TestRecommendedSpecsAreConservativeAndUnknownSizeIsUnknown(t *testing.T) {
	ram, vram, cores := computeRecommendedSpecs(2.6)
	assert.GreaterOrEqual(t, ram, 8.0)
	assert.GreaterOrEqual(t, vram, 4)
	assert.GreaterOrEqual(t, cores, 8)
	ram, vram, cores = computeRecommendedSpecs(0)
	assert.Zero(t, ram)
	assert.Zero(t, vram)
	assert.Zero(t, cores)
}

func TestShardedFileDoesNotUnderstateWholeModelRequirements(t *testing.T) {
	ram, vram, cores := recommendedSpecsForFile("model-00001-of-00004.gguf", 4*1024*1024*1024)
	assert.Zero(t, ram)
	assert.Zero(t, vram)
	assert.Zero(t, cores)
	ram, vram, cores = recommendedSpecsForFile("model-Q4_K_M.gguf", 4*1024*1024*1024)
	assert.Greater(t, ram, 0.0)
	assert.Greater(t, vram, 0)
	assert.Greater(t, cores, 0)
}
