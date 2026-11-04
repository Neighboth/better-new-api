package byog

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// messageContentText extracts the plain text of an OpenAI chat message content.
//
// The content is either a string or the array of the content parts of a
// multimodal message, the text parts of the array are joined.
func messageContentText(content interface{}) string {
	switch v := content.(type) {
	case string:
		return v
	case []interface{}:
		var texts []string
		for _, part := range v {
			if partMap, ok := part.(map[string]interface{}); ok {
				if text, ok := partMap["text"].(string); ok && text != "" {
					texts = append(texts, text)
				}
			}
		}
		return strings.Join(texts, "\n")
	}
	return ""
}

// buildConversationTranscript renders the messages of a chat completion as a
// role tagged transcript. The transcript is the prompt of the workers that
// generate from a plain prompt, it keeps the full context of the conversation.
func buildConversationTranscript(messages interface{}) string {
	msgs, ok := messages.([]interface{})
	if !ok || len(msgs) == 0 {
		return ""
	}
	var builder strings.Builder
	for _, item := range msgs {
		msg, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		role, _ := msg["role"].(string)
		if role == "" {
			role = "user"
		}
		text := messageContentText(msg["content"])
		if text == "" {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(role)
		builder.WriteString(": ")
		builder.WriteString(text)
	}
	return builder.String()
}

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

	var reqMap map[string]interface{}
	if err := common.UnmarshalBodyReusable(c, &reqMap); err != nil {
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
			promptPrefix = messageContentText(lastMsg["content"])
		}
	} else if p, ok := reqMap["prompt"].(string); ok {
		promptPrefix = p
	}

	// 1. Prioritize user's own online node if available (Item 18)
	var targetSession *NodeSession
	userId := c.GetInt("id")
	if userId > 0 {
		userNodes, _ := model.GetUserByogNodes(userId)
		for _, un := range userNodes {
			if un.Status != "banned" {
				if s, ok := GlobalHub.GetSession(un.NodeId); ok && s != nil && !s.IsPaused && !s.IsGamerMode {
					if s.InstalledModels != nil && s.InstalledModels[byogModel.ModelId] {
						targetSession = s
						break
					}
					assigned, _ := GlobalScheduler.EvaluateNodeModelAssignment(un)
					for _, am := range assigned {
						if am == byogModel.ModelId {
							targetSession = s
							break
						}
					}
					if targetSession != nil {
						break
					}
				}
			}
		}
	}

	// 2. Check if warm node exists (has model installed and is connected)
	if targetSession == nil {
		targetSession = GlobalJit.FindWarmNodeForModel(byogModel.ModelId)
	}

	// 3. If no warm node, try selecting any capable online node
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
	maxTokens := -1
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

	// The prompt is the transcript of the conversation. A worker that runs on a
	// phone generates from the prompt alone, the transcript gives that worker
	// the full context of the playground conversation instead of the last
	// message only.
	transcript := buildConversationTranscript(reqMap["messages"])
	if transcript == "" {
		transcript = promptPrefix
	}

	topP := 0.0
	if tp, ok := reqMap["top_p"].(float64); ok {
		topP = tp
	}
	var stopWords []string
	switch sv := reqMap["stop"].(type) {
	case string:
		if sv != "" {
			stopWords = []string{sv}
		}
	case []interface{}:
		for _, item := range sv {
			if s, ok := item.(string); ok && s != "" {
				stopWords = append(stopWords, s)
			}
		}
	}

	inferencePayload := InferencePayload{
		RequestId:   fmt.Sprintf("byog_%d", time.Now().UnixNano()),
		ModelId:     byogModel.ModelId,
		Messages:    messagesJson,
		Prompt:      transcript,
		Stream:      isStream,
		Temperature: temp,
		MaxTokens:   maxTokens,
		Seed:        seed,
		Stop:        stopWords,
		TopP:        topP,
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
