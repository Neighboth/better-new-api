package helper

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
)

func strPtr(s string) *string { return &s }

func TestGetChannelModelDiscount(t *testing.T) {
	assert.Equal(t, 1.0, GetChannelModelDiscount(nil, "gpt-4o"))
	assert.Equal(t, 1.0, GetChannelModelDiscount(&model.Channel{}, "gpt-4o"))
	assert.Equal(t, 1.0, GetChannelModelDiscount(&model.Channel{ModelRatioOverride: strPtr("")}, "gpt-4o"))
	assert.Equal(t, 1.0, GetChannelModelDiscount(&model.Channel{ModelRatioOverride: strPtr("not-json")}, "gpt-4o"))
	assert.Equal(t, 0.5, GetChannelModelDiscount(&model.Channel{ModelRatioOverride: strPtr(`{"gpt-4o":0.5}`)}, "gpt-4o"))
	assert.Equal(t, 1.0, GetChannelModelDiscount(&model.Channel{ModelRatioOverride: strPtr(`{"gpt-4o":0.5}`)}, "other-model"))
	assert.Equal(t, 1.0, GetChannelModelDiscount(&model.Channel{ModelRatioOverride: strPtr(`{"gpt-4o":0}`)}, "gpt-4o"))
	assert.Equal(t, 1.0, GetChannelModelDiscount(&model.Channel{ModelRatioOverride: strPtr(`{"gpt-4o":-1}`)}, "gpt-4o"))
}
