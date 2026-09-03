package controller

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
)

type fallbackState struct {
	models []string
	idx    int
	orig   string
	on     bool
	prompt string
	done   bool
}

func newFallbackState(info *relaycommon.RelayInfo) *fallbackState {
	s := operation_setting.GetRelayFallbackSetting()
	models := s.FallbackModelList()
	unique := make([]string, 0, len(models))
	seen := map[string]bool{info.GetOriginModelName(): true}
	for _, m := range models {
		if !seen[m] {
			seen[m] = true
			unique = append(unique, m)
		}
	}
	return &fallbackState{
		models: unique,
		orig:   info.GetOriginModelName(),
		on:     s.EnableFallback,
		prompt: strings.TrimSpace(s.FallbackSystemPrompt),
	}
}

func (s *fallbackState) currentModel() (string, bool) {
	if s == nil || !s.on {
		return "", false
	}
	if s.idx < len(s.models) {
		return s.models[s.idx], true
	}
	return "", false
}

func (s *fallbackState) advance() bool {
	if s == nil || !s.on {
		return false
	}
	if s.idx < len(s.models) {
		s.idx++
		return true
	}
	return false
}

func switchRelayModel(c *gin.Context, info *relaycommon.RelayInfo, newModel string) error {
	if newModel == "" {
		return errors.New("fallback model is empty")
	}
	info.OriginModelName = newModel
	common.SetContextKey(c, constant.ContextKeyOriginalModel, newModel)
	c.Set("original_model", newModel)

	switch req := info.Request.(type) {
	case *dto.GeneralOpenAIRequest:
		req.Model = newModel
	case *dto.OpenAIResponsesRequest:
		req.Model = newModel
	case *dto.ClaudeRequest:
		req.Model = newModel
	}
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return err
	}
	data, err := storage.Bytes()
	if err != nil {
		return err
	}
	var raw map[string]interface{}
	if err := common.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("fallback: rewrite request body model: %w", err)
	}
	raw["model"] = newModel
	updated, err := common.Marshal(raw)
	if err != nil {
		return err
	}
	newStorage, err := common.CreateBodyStorage(updated)
	if err != nil {
		return err
	}
	_ = storage.Close()
	c.Set(common.KeyBodyStorage, newStorage)
	c.Request.Body = io.NopCloser(newStorage)
	return nil
}

func applyFallbackSystemPrompt(c *gin.Context, info *relaycommon.RelayInfo, s *fallbackState) error {
	if s == nil || !s.on || s.done {
		return nil
	}
	prompt := strings.TrimSpace(s.prompt)
	if prompt == "" {
		return nil
	}
	prompt = strings.ReplaceAll(prompt, "${modelid}", info.OriginModelName)
	prompt = strings.ReplaceAll(prompt, "${model_id}", info.OriginModelName)
	prompt = strings.ReplaceAll(prompt, "${model}", info.OriginModelName)
	s.done = true

	switch req := info.Request.(type) {
	case *dto.GeneralOpenAIRequest:
		if !hasSystemMessage(req.Messages, req.GetSystemRoleName()) {
			req.Messages = append([]dto.Message{{Role: req.GetSystemRoleName(), Content: prompt}}, req.Messages...)
		}
	}

	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return err
	}
	data, err := storage.Bytes()
	if err != nil {
		return err
	}
	var raw map[string]interface{}
	if err := common.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("fallback: inject system prompt: %w", err)
	}

	if messages, ok := raw["messages"].([]interface{}); ok {
		for _, m := range messages {
			if mm, ok := m.(map[string]interface{}); ok && mm["role"] == "system" {
				return nil
			}
		}
		raw["messages"] = append([]interface{}{map[string]interface{}{"role": "system", "content": prompt}}, messages...)
	} else if sys, ok := raw["system"].(string); ok {
		raw["system"] = prompt + "\n" + sys
	}
	updated, err := common.Marshal(raw)
	if err != nil {
		return err
	}
	newStorage, err := common.CreateBodyStorage(updated)
	if err != nil {
		return err
	}
	_ = storage.Close()
	c.Set(common.KeyBodyStorage, newStorage)
	c.Request.Body = io.NopCloser(newStorage)
	return nil
}

func hasSystemMessage(messages []dto.Message, systemRole string) bool {
	for _, m := range messages {
		if m.Role == systemRole {
			return true
		}
	}
	return false
}

func logRelayFallback(c *gin.Context, from, to string) {
	logger.LogInfo(c, fmt.Sprintf("relay fallback: model %s -> %s", from, to))
}

func isPriceNotConfiguredError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "price not configured") || strings.Contains(msg, "价格未配置") || strings.Contains(msg, "尚未由管理员配置")
}

func advanceFallbackModel(c *gin.Context, relayInfo *relaycommon.RelayInfo, retryParam *service.RetryParam, fb *fallbackState) bool {
	if fb == nil || !fb.on {
		return false
	}

	cand, ok := fb.currentModel()
	if !ok || cand == "" {
		return false
	}
	logRelayFallback(c, relayInfo.OriginModelName, cand)
	if err := switchRelayModel(c, relayInfo, cand); err != nil {
		logger.LogError(c, err.Error())
		return false
	}
	if err := applyFallbackSystemPrompt(c, relayInfo, fb); err != nil {
		logger.LogWarn(c, fmt.Sprintf("failed to apply fallback system prompt: %s", err.Error()))
	}
	retryParam.ModelName = cand
	retryParam.SetRetry(0)
	fb.advance()
	return true
}
