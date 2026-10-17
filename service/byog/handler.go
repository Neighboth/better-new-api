package byog

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
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
		if firstMsg, ok := msgs[0].(map[string]interface{}); ok {
			if content, ok := firstMsg["content"].(string); ok {
				promptPrefix = content
			}
		}
	} else if p, ok := reqMap["prompt"].(string); ok {
		promptPrefix = p
	}

	// 1. Check if warm node exists
	targetSession := GlobalJit.FindWarmNodeForModel(byogModel.ModelId)

	// 2. If no warm node, check if cold provisioning is needed
	if targetSession == nil {
		if c.GetHeader("X-BYOG-No-Wait") == "true" {
			// Fallback to standard provider
			return false
		}

		// Perform JIT Cold Provisioning (Section 2.4)
		isSse := isStream || strings.Contains(c.GetHeader("Accept"), "text/event-stream")
		provisionedSession, err := GlobalJit.ProvisionColdModel(c, byogModel, isSse)
		if err != nil || provisionedSession == nil {
			common.SysLog(fmt.Sprintf("[BYOG JIT] Cold provisioning skipped or failed: %v", err))
			// Fallback to standard channel
			return false
		}
		targetSession = provisionedSession
	}

	// 3. If targetSession still not selected, use Smart Geo-Routing & Sticky Session
	if targetSession == nil {
		selected, err := GlobalRouter.SelectNodeForRequest(byogModel.ModelId, conversationId, promptPrefix, c.ClientIP())
		if err != nil || selected == nil {
			return false
		}
		targetSession = selected
	}

	// 4. Construct inference payload
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
