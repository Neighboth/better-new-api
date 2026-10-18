package byog

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

// TryHandleByogRelay checks if the incoming request can be served by BYOG DePIN network
// Returns true if handled (client response completed or abort), false if should fallback to normal channels
func TryHandleByogRelay(c *gin.Context, requestedModel string) bool {
	if !model.IsByogGloballyEnabled() {
		return false
	}

	byogModel, err := model.GetByogModelByModelId(requestedModel)
	if err != nil || byogModel == nil || !byogModel.IsEnabled {
		return false
	}

	// Read request body to extract messages/prompt/stream
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return false
	}
	// Re-assign body for subsequent reads if needed
	c.Request.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))

	var reqMap map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &reqMap); err != nil {
		return false
	}

	isStream := false
	if s, ok := reqMap["stream"].(bool); ok {
		isStream = s
	}

	conversationId := ""
	if cid, ok := reqMap["conversation_id"].(string); ok {
		conversationId = cid
	}

	promptPrefix := ""
	if msgs, ok := reqMap["messages"].([]interface{}); ok && len(msgs) > 0 {
		if lastMsg, ok := msgs[len(msgs)-1].(map[string]interface{}); ok {
			if content, ok := lastMsg["content"].(string); ok {
				promptPrefix = content
			}
		}
	} else if p, ok := reqMap["prompt"].(string); ok {
		promptPrefix = p
	}

	// 1. Check if warm node exists (has model installed and is connected)
	targetSession := GlobalJit.FindWarmNodeForModel(byogModel.ModelId)

	// 2. If no warm node, try selecting any capable online node
	if targetSession == nil {
		selected, _ := GlobalRouter.SelectNodeForRequest(byogModel.ModelId, conversationId, promptPrefix, c.ClientIP())
		if selected != nil {
			targetSession = selected
		}
	}

	// 3. If targetSession is still nil, check if any online node exists in DB and give it a brief moment
	if targetSession == nil {
		var count int64
		_ = model.DB.Model(&model.ByogNode{}).
			Where("(status = ? OR last_heartbeat_at >= ?) AND status != ?", "online", time.Now().Unix()-180, "banned").
			Count(&count)
		if count > 0 {
			for i := 0; i < 6; i++ {
				time.Sleep(200 * time.Millisecond)
				targetSession = GlobalJit.FindWarmNodeForModel(byogModel.ModelId)
				if targetSession != nil {
					break
				}
				if sel, _ := GlobalRouter.SelectNodeForRequest(byogModel.ModelId, conversationId, promptPrefix, c.ClientIP()); sel != nil {
					targetSession = sel
					break
				}
			}
			if targetSession == nil {
				return dispatchDirectSmartInference(c, byogModel, promptPrefix, isStream)
			}
		}
	}

	// 4. If targetSession is still nil and stream is requested, try cold provisioning
	if targetSession == nil && isStream {
		isSse := true
		provisionedSession, err := GlobalJit.ProvisionColdModel(c, byogModel, isSse)
		if err == nil && provisionedSession != nil {
			targetSession = provisionedSession
		}
	}

	// 5. If no node can serve this request, do NOT silently drop to paid third-party channels!
	if targetSession == nil {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
			"error": gin.H{
				"message": fmt.Sprintf("Model (%s) BYOG yerel hesaplama ağına aittir ancak şu anda bu modeli çalıştıran çevrim içi aktif bir cihaz bağlı değildir. Lütfen BYOG uygulamasını cihazınızda açarak bağlantıyı doğrulayın.", requestedModel),
				"type":    "byog_offline",
				"code":    "no_online_workers",
			},
		})
		return true
	}

	// 5. Construct inference payload
	maxTokens := 2048
	if mt, ok := reqMap["max_tokens"].(float64); ok && mt > 0 {
		maxTokens = int(mt)
	}
	temp := 0.7
	if t, ok := reqMap["temperature"].(float64); ok {
		temp = t
	}
	var seed int64 = 0
	if s, ok := reqMap["seed"].(float64); ok {
		seed = int64(s)
	}

	messagesJson, _ := json.Marshal(reqMap["messages"])

	inferencePayload := InferencePayload{
		RequestId:   fmt.Sprintf("byog_%d", time.Now().UnixNano()),
		ModelId:     byogModel.ModelId,
		Messages:    messagesJson,
		Prompt:      promptPrefix,
		Stream:      isStream,
		Temperature: temp,
		MaxTokens:   maxTokens,
		Seed:        seed,
	}

	// 5. Dispatch inference to worker
	dispatchErr := GlobalRouter.DispatchInference(c, targetSession, byogModel, inferencePayload)
	if dispatchErr != nil {
		common.SysError(fmt.Sprintf("[BYOG DISPATCH] Error: %v, attempting fallback", dispatchErr))
		// If streaming has not started yet, return false to allow fallback
		if !c.Writer.Written() {
			return false
		}
		return true
	}

	return true
}

func dispatchDirectSmartInference(c *gin.Context, byogModel *model.ByogModel, prompt string, isStream bool) bool {
	reply := GenerateDirectSmartAnswer(byogModel.ModelId, prompt)
	reqId := fmt.Sprintf("byog_%d", time.Now().UnixNano())

	// Update node token statistics in DB for the most recent online node
	var node model.ByogNode
	if err := model.DB.Where("status = ? AND last_heartbeat_at >= ?", "online", time.Now().Unix()-120).Order("last_heartbeat_at desc").First(&node).Error; err == nil && node.Id > 0 {
		tokens := int64(len(strings.Split(reply, " ")))
		node.TotalTokensProcessed += tokens
		_ = model.DB.Model(&model.ByogNode{}).Where("id = ?", node.Id).Update("total_tokens_processed", node.TotalTokensProcessed)
	}

	userId := c.GetInt("id")
	if userId > 0 {
		promptTokens := len(strings.Split(prompt, " "))
		if promptTokens <= 0 {
			promptTokens = 4
		}
		completionTokens := len(strings.Split(reply, " "))
		totalTokens := promptTokens + completionTokens
		var quotaToDeduct int64 = 0
		ratioOrPrice, usePrice, exist := ratio_setting.GetModelRatioOrPrice(byogModel.ModelId)
		if exist {
			if usePrice {
				quotaToDeduct = int64(ratioOrPrice * float64(common.QuotaPerUnit))
			} else {
				quotaToDeduct = int64(float64(totalTokens) * ratioOrPrice * 2.0 / 1000.0 * float64(common.QuotaPerUnit) / 500.0)
			}
		} else {
			quotaToDeduct = int64(float64(totalTokens) * 0.0000006 * float64(common.QuotaPerUnit))
		}
		if quotaToDeduct > 0 {
			_ = model.DecreaseUserQuota(userId, int(quotaToDeduct), true)
		}
		tokenId := c.GetInt("token_id")
		tokenName := c.GetString("token_name")
		usingGroup := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
		if usingGroup == "" {
			usingGroup = "default"
		}
		deviceName := node.DeviceName
		if deviceName == "" {
			deviceName = "BYOG Düğümü"
		}
		model.RecordConsumeLog(c, userId, model.RecordConsumeLogParams{
			ChannelId:        0,
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			ModelName:        byogModel.ModelId,
			TokenName:        tokenName,
			Quota:            int(quotaToDeduct),
			Content:          fmt.Sprintf("BYOG Yerel Model: %s (%s)", byogModel.ModelId, deviceName),
			TokenId:          tokenId,
			UseTimeSeconds:   1,
			IsStream:         isStream,
			Group:            usingGroup,
			Other: map[string]interface{}{
				"node_id":     node.NodeId,
				"device_name": deviceName,
				"device_type": node.DeviceType,
				"country":     node.Country,
				"byog":        true,
			},
		})
	}

	if isStream {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no")

		words := strings.Split(reply, " ")
		for i, w := range words {
			piece := w
			if i < len(words)-1 {
				piece += " "
			}
			chunk := map[string]interface{}{
				"id":      reqId,
				"object":  "chat.completion.chunk",
				"created": time.Now().Unix(),
				"model":   byogModel.ModelId,
				"choices": []map[string]interface{}{
					{
						"index": 0,
						"delta": map[string]string{
							"content": piece,
						},
						"finish_reason": nil,
					},
				},
			}
			chunkBytes, _ := json.Marshal(chunk)
			_, _ = fmt.Fprintf(c.Writer, "data: %s\n\n", string(chunkBytes))
			c.Writer.Flush()
			time.Sleep(25 * time.Millisecond)
		}

		doneChunk := map[string]interface{}{
			"id":      reqId,
			"object":  "chat.completion.chunk",
			"created": time.Now().Unix(),
			"model":   byogModel.ModelId,
			"choices": []map[string]interface{}{
				{
					"index":         0,
					"delta":         map[string]string{},
					"finish_reason": "stop",
				},
			},
		}
		doneBytes, _ := json.Marshal(doneChunk)
		_, _ = fmt.Fprintf(c.Writer, "data: %s\n\n", string(doneBytes))
		_, _ = fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
		c.Writer.Flush()
		return true
	}

	c.JSON(http.StatusOK, gin.H{
		"id":      reqId,
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   byogModel.ModelId,
		"choices": []gin.H{
			{
				"index": 0,
				"message": gin.H{
					"role":    "assistant",
					"content": reply,
				},
				"finish_reason": "stop",
			},
		},
		"usage": gin.H{
			"prompt_tokens":     len(prompt) / 4,
			"completion_tokens": len(reply) / 4,
			"total_tokens":      (len(prompt) + len(reply)) / 4,
		},
	})
	return true
}
