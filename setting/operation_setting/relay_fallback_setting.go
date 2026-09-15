package operation_setting

import (
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
)

// RelayFallbackSetting lets an admin define an ordered fallback model list and a
// system prompt that is applied to every relay attempt. When the originally
// requested model fails on every eligible channel, the gateway walks down
// the fallback list (still trying all channels per model) and finally tries
// arbitrary enabled models so the user still gets an answer.
type RelayFallbackSetting struct {
	// Ordered comma-separated fallback model names attempted after the original model.

	FallbackModels string `json:"fallback_models"`
	// System prompt text prepended to every attempt (original + fallback).
	FallbackSystemPrompt string `json:"fallback_system_prompt"`
	// EnableFallback toggles whether the relay fallback chain is active at all.
	EnableFallback bool `json:"enable_fallback"`
	// FallbackTimeout is the number of seconds to wait for the first token before falling back.
	FallbackTimeout int `json:"fallback_timeout"`
}

// 默认配置
var relayFallbackSetting = RelayFallbackSetting{
	EnableFallback:    false,
	FallbackModels:    "",
	FallbackSystemPrompt: "",
	FallbackTimeout:   10,
}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("relay_fallback_setting", &relayFallbackSetting)
}

func GetRelayFallbackSetting() *RelayFallbackSetting {
	return &relayFallbackSetting
}

// FallbackModelList returns the trimmed non-empty ordered model listr.
func (s *RelayFallbackSetting) FallbackModelList() []string {
	if s == nil {
		return nil
	}
	var models []string
	for _, m := range strings.Split(s.FallbackModels, ",") {
		m = strings.TrimSpace(m)
		if m != "" {
			models = append(models, m)
		}
	}
	return models
}