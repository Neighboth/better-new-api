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
	FallbackModels       string `json:"fallback_models"`
	FallbackChatModels   string `json:"fallback_chat_models"`
	FallbackImageModels  string `json:"fallback_image_models"`
	FallbackTTSModels    string `json:"fallback_tts_models"`
	FallbackSTTModels    string `json:"fallback_stt_models"`
	// System prompt text prepended to every attempt (original + fallback).
	FallbackSystemPrompt string `json:"fallback_system_prompt"`
	// EnableFallback toggles whether the relay fallback chain is active at all.
	EnableFallback bool `json:"enable_fallback"`
}

// 默认配置
var relayFallbackSetting = RelayFallbackSetting{
	EnableFallback:       false,
	FallbackModels:       "",
	FallbackChatModels:   "",
	FallbackImageModels:  "",
	FallbackTTSModels:    "",
	FallbackSTTModels:    "",
	FallbackSystemPrompt: "",
}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("relay_fallback_setting", &relayFallbackSetting)
}

func GetRelayFallbackSetting() *RelayFallbackSetting {
	return &relayFallbackSetting
}

func parseModelList(raw string) []string {
	var models []string
	for _, m := range strings.Split(raw, ",") {
		m = strings.TrimSpace(m)
		if m != "" {
			models = append(models, m)
		}
	}
	return models
}

// FallbackModelList returns the trimmed non-empty ordered model list for general/chat models.
func (s *RelayFallbackSetting) FallbackModelList() []string {
	if s == nil {
		return nil
	}
	if s.FallbackChatModels != "" {
		return parseModelList(s.FallbackChatModels)
	}
	return parseModelList(s.FallbackModels)
}

func (s *RelayFallbackSetting) FallbackChatModelList() []string {
	return s.FallbackModelList()
}

func (s *RelayFallbackSetting) FallbackImageModelList() []string {
	if s == nil {
		return nil
	}
	return parseModelList(s.FallbackImageModels)
}

func (s *RelayFallbackSetting) FallbackTTSModelList() []string {
	if s == nil {
		return nil
	}
	return parseModelList(s.FallbackTTSModels)
}

func (s *RelayFallbackSetting) FallbackSTTModelList() []string {
	if s == nil {
		return nil
	}
	return parseModelList(s.FallbackSTTModels)
}