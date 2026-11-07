package byog

import (
	"errors"
	"fmt"
	"math"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

// Paid BYOG inference must be bounded before the worker sees the request.
// The worker-provided usage is not trusted beyond these server-enforced limits.
const byogDefaultOutputLimit = 4096
const byogMaxOutputLimit = 32768
const byogMaxRequestBytes = 8 << 20

type byogPrice struct {
	fixed bool
	value float64
}
type byogAuthorization struct {
	price                             byogPrice
	inputLimit, outputLimit, reserved int
}

func snapshotByogPrice(modelDef *model.ByogModel) (byogPrice, error) {
	value, fixed, exists := ratio_setting.GetModelRatioOrPrice(modelDef.ModelId)
	if !exists {
		settings, err := model.GetByogSetting()
		if err != nil {
			return byogPrice{}, err
		}
		provider := settings.Tier2ProviderPrice
		if modelDef.PricingTier == 1 {
			provider = settings.Tier1ProviderPrice
		}
		if modelDef.PricingTier == 3 {
			provider = settings.Tier3ProviderPrice
		}
		value = 3 * provider * common.QuotaPerUnit / 1000000
	} else if fixed {
		value *= common.QuotaPerUnit
	} else {
		value *= 2.0 / 1000.0 * common.QuotaPerUnit / 500.0
	}
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return byogPrice{}, errors.New("invalid BYOG price")
	}
	return byogPrice{fixed: fixed, value: value}, nil
}

func (p byogPrice) quota(input, output int, reserve bool) (int, error) {
	if input < 0 || output < 0 {
		return 0, errors.New("invalid BYOG token usage")
	}
	result := p.value
	if !p.fixed {
		result *= float64(input) + float64(output)
	}
	if reserve {
		result = math.Ceil(result)
	} else {
		result = math.Floor(result)
	}
	if math.IsNaN(result) || math.IsInf(result, 0) || result < 0 || result > float64(common.MaxQuota) {
		return 0, errors.New("BYOG cost exceeds supported range")
	}
	return int(result), nil
}

func newByogAuthorization(modelDef *model.ByogModel, payload *InferencePayload) (*byogAuthorization, error) {
	if modelDef == nil || payload == nil {
		return nil, errors.New("BYOG request missing")
	}
	if payload.MaxTokens <= 0 {
		payload.MaxTokens = byogDefaultOutputLimit
	}
	if payload.MaxTokens > byogMaxOutputLimit {
		return nil, fmt.Errorf("max_tokens exceeds BYOG limit of %d", byogMaxOutputLimit)
	}
	if len(payload.Prompt) > byogMaxRequestBytes || len(payload.Messages) > byogMaxRequestBytes || len(payload.Prompt)+len(payload.Messages) > byogMaxRequestBytes {
		return nil, errors.New("BYOG prompt exceeds size limit")
	}
	// Limit what a worker may report, not a claim that all model tokenizers
	// have this ratio. An implausible report is rejected and reconciled.
	inputLimit := 2*(len(payload.Prompt)+len(payload.Messages)) + 4096
	price, err := snapshotByogPrice(modelDef)
	if err != nil {
		return nil, err
	}
	reserve, err := price.quota(inputLimit, payload.MaxTokens, true)
	if err != nil {
		return nil, err
	}
	return &byogAuthorization{price: price, inputLimit: inputLimit, outputLimit: payload.MaxTokens, reserved: reserve}, nil
}
