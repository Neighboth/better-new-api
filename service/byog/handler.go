package byog

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
				c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
					"error": gin.H{
						"message": fmt.Sprintf("Model (%s) BYOG yerel hesaplama ağına aittir ancak şu anda bu modeli çalıştıran çevrim içi aktif bir cihaz bağlı değildir. Lütfen BYOG uygulamasını cihazınızda açarak bağlantıyı doğrulayın.", requestedModel),
						"type":    "byog_offline",
						"code":    "no_online_workers",
					},
				})
				return true
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

	// 6. Construct inference payload
	maxTokens := 64
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

	// 7. Dispatch inference to worker
	dispatchErr := GlobalRouter.DispatchInference(c, targetSession, byogModel, inferencePayload)
	if dispatchErr != nil {
		common.SysError(fmt.Sprintf("[BYOG DISPATCH] Error: %v", dispatchErr))
		if !c.Writer.Written() {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"error": gin.H{
					"message": fmt.Sprintf("BYOG cihazına istek iletilirken hata oluştu: %v", dispatchErr),
					"type":    "byog_dispatch_error",
					"code":    "worker_dispatch_failed",
				},
			})
		}
		return true
	}

	return true
}
