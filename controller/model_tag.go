package controller

import (
	"fmt"
	"io"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

type modelTagRouting struct {
	base   string
	floor  bool
	nitro  bool
	online bool
}

func parseModelTagRouting(raw string) modelTagRouting {
	r := modelTagRouting{base: strings.TrimSpace(raw)}
	for {
		lower := strings.ToLower(r.base)
		if strings.HasSuffix(lower, ":floor") {
			r.floor = true
			r.base = strings.TrimSpace(r.base[:len(r.base)-len(":floor")])
			continue
		}
		if strings.HasSuffix(lower, ":nitro") {
			r.nitro = true
			r.base = strings.TrimSpace(r.base[:len(r.base)-len(":nitro")])
			continue
		}
		if strings.HasSuffix(lower, ":online") {
			r.online = true
			r.base = strings.TrimSpace(r.base[:len(r.base)-len(":online")])
			continue
		}
		break
	}
	if r.base == "" {
		r.base = strings.TrimSpace(raw)
		r.floor, r.nitro, r.online = false, false, false
	}
	return r
}

func applyModelTagRouting(c *gin.Context, info *relaycommon.RelayInfo) modelTagRouting {
	routing := parseModelTagRouting(info.OriginModelName)
	if routing.base == info.OriginModelName {
		return routing
	}
	info.OriginModelName = routing.base
	common.SetContextKey(c, constant.ContextKeyOriginalModel, routing.base)
	c.Set("original_model", routing.base)
	if routing.floor {
		c.Set("model_tag_floor", true)
	}
	if routing.nitro {
		c.Set("model_tag_nitro", true)
	}
	if routing.online {
		c.Set("model_tag_online", true)
	}
	switch req := info.Request.(type) {
	case *dto.GeneralOpenAIRequest:
		req.Model = routing.base
	case *dto.OpenAIResponsesRequest:
		req.Model = routing.base
	case *dto.ClaudeRequest:
		req.Model = routing.base
	case *dto.ImageRequest:
		req.Model = routing.base
	case *dto.AudioRequest:
		req.Model = routing.base
	}
	if storage, err := common.GetBodyStorage(c); err == nil {
		if data, err := storage.Bytes(); err == nil {
			var raw map[string]interface{}
			if err := common.Unmarshal(data, &raw); err == nil {
				if _, ok := raw["model"]; ok {
					raw["model"] = routing.base
					if updated, err := common.Marshal(raw); err == nil {
						if newStorage, err := common.CreateBodyStorage(updated); err == nil {
							_ = storage.Close()
							c.Set(common.KeyBodyStorage, newStorage)
							c.Request.Body = io.NopCloser(newStorage)
						}
					}
				}
			}
		}
	}
	return routing
}

func extractOnlineSearchQuery(c *gin.Context, info *relaycommon.RelayInfo) string {
	switch req := info.Request.(type) {
	case *dto.GeneralOpenAIRequest:
		for i := len(req.Messages) - 1; i >= 0; i-- {
			if req.Messages[i].Role == "user" {
				return truncateQuery(req.Messages[i].StringContent())
			}
		}
	case *dto.ClaudeRequest:
		for i := len(req.Messages) - 1; i >= 0; i-- {
			if req.Messages[i].Role == "user" {
				return truncateQuery(req.Messages[i].GetStringContent())
			}
		}
	case *dto.OpenAIResponsesRequest:
		if req.Input != nil {
			return truncateQuery(fmt.Sprintf("%v", req.Input))
		}
	}
	// Generic fallback for formats without a typed extractor (e.g. Gemini):
	// look for the last user text in the raw body.
	if c != nil {
		if storage, err := common.GetBodyStorage(c); err == nil {
			if data, err := storage.Bytes(); err == nil {
				var raw map[string]interface{}
				if err := common.Unmarshal(data, &raw); err == nil {
					if q := lastUserTextFromRaw(raw); q != "" {
						return truncateQuery(q)
					}
				}
			}
		}
	}
	return ""
}

func truncateQuery(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 500 {
		return s[:500]
	}
	return s
}

func lastUserTextFromRaw(raw map[string]interface{}) string {
	messages, ok := raw["messages"].([]interface{})
	if !ok {
		return ""
	}
	for i := len(messages) - 1; i >= 0; i-- {
		m, ok := messages[i].(map[string]interface{})
		if !ok || m["role"] != "user" {
			continue
		}
		if content, ok := m["content"].(string); ok && strings.TrimSpace(content) != "" {
			return content
		}
	}
	return ""
}

func applyOnlineSearch(c *gin.Context, info *relaycommon.RelayInfo, baseModel string) error {
	query := extractOnlineSearchQuery(c, info)
	if strings.TrimSpace(query) == "" {
		return nil
	}
	setting := operation_setting.GetOnlineSearchSetting()
	if strings.TrimSpace(setting.SearXNGURL) == "" {
		return fmt.Errorf("searxng url is not configured")
	}
	maxResults := setting.MaxSearches
	if maxResults <= 0 {
		maxResults = 5
	}
	if maxResults > 10 {
		maxResults = 10
	}
	results, count, err := service.RunOnlineSearch(query, maxResults)
	if err != nil {
		return err
	}
	if count > 0 {
		c.Set("online_search_count", count)
	}
	if strings.TrimSpace(results) == "" {
		return nil
	}
	injectOnlineSearchContext(c, info, results)
	return nil
}

func injectOnlineSearchContext(c *gin.Context, info *relaycommon.RelayInfo, results string) {
	contextBlock := "Web search results (use them to answer with citations when relevant):\n" + results
	switch req := info.Request.(type) {
	case *dto.GeneralOpenAIRequest:
		sysRole := req.GetSystemRoleName()
		if sysRole == "" {
			sysRole = "system"
		}
		req.Messages = append([]dto.Message{{Role: sysRole, Content: contextBlock}}, req.Messages...)
	case *dto.ClaudeRequest:
		if req.System != nil {
			combined := contextBlock + "\n" + fmt.Sprintf("%v", req.System)
			req.System = combined
		} else {
			req.System = contextBlock
		}
	}
	syncOnlineSearchBodyStorage(c, contextBlock)
}

func syncOnlineSearchBodyStorage(c *gin.Context, contextBlock string) {
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return
	}
	data, err := storage.Bytes()
	if err != nil {
		return
	}
	var raw map[string]interface{}
	if err := common.Unmarshal(data, &raw); err != nil {
		return
	}
	if messages, ok := raw["messages"].([]interface{}); ok {
		raw["messages"] = append([]interface{}{map[string]interface{}{"role": "system", "content": contextBlock}}, messages...)
		if updated, err := common.Marshal(raw); err == nil {
			if newStorage, err := common.CreateBodyStorage(updated); err == nil {
				_ = storage.Close()
				c.Set(common.KeyBodyStorage, newStorage)
				c.Request.Body = io.NopCloser(newStorage)
			}
		}
	}
}
