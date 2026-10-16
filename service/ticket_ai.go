package service

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

type aiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type aiRequest struct {
	Model    string      `json:"model"`
	Messages []aiMessage `json:"messages"`
	Stream   bool        `json:"stream"`
}

func InvokeAiAssistant(ticketId int) {
	setting := operation_setting.GetTicketSetting()
	if !setting.AiAssistantEnabled || setting.AiAssistantModel == "" {
		return
	}

	messages, err := model.GetTicketMessages(ticketId)
	if err != nil || len(messages) == 0 {
		return
	}

	// Build chat history
	var chatMessages []aiMessage
	chatMessages = append(chatMessages, aiMessage{
		Role:    "system",
		Content: setting.AiAssistantSystemPrompt,
	})

	for _, m := range messages {
		role := "user"
		if m.IsAdmin {
			role = "assistant"
		}
		chatMessages = append(chatMessages, aiMessage{
			Role:    role,
			Content: m.Content,
		})
	}

	reqBody := aiRequest{
		Model:    setting.AiAssistantModel,
		Messages: chatMessages,
		Stream:   false,
	}

	jsonBody, err := common.Marshal(reqBody)
	if err != nil {
		common.SysError("failed to marshal AI request: " + err.Error())
		return
	}

	port := os.Getenv("PORT")
	if port == "" && common.Port != nil {
		port = strconv.Itoa(*common.Port)
	}
	if port == "" {
		port = "3000"
	}
	url := fmt.Sprintf("http://127.0.0.1:%s/v1/chat/completions", port)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		common.SysError("failed to create AI request: " + err.Error())
		return
	}

	// Find a usable token to make the API call. Prefer an enabled,
	// unexpired root token with quota instead of the first row.
	var rootToken string
	var token model.Token
	now := time.Now().Unix()
	if err := model.DB.Where("user_id = 1 AND status = 1").Order("id ASC").First(&token).Error; err == nil {
		if token.ExpiredTime == -1 || token.ExpiredTime == 0 || token.ExpiredTime > now || token.UnlimitedQuota {
			rootToken = token.Key
		}
	}
	if rootToken == "" {
		var fallback model.Token
		if err := model.DB.Where("status = 1").Order("id ASC").First(&fallback).Error; err == nil {
			if fallback.ExpiredTime == -1 || fallback.ExpiredTime == 0 || fallback.ExpiredTime > now || fallback.UnlimitedQuota {
				rootToken = fallback.Key
			} else {
				rootToken = fallback.Key
			}
		}
	}
	if rootToken == "" {
		common.SysError(fmt.Sprintf("ticket #%d: AI assistant skipped, no usable API token found", ticketId))
		return
	}

	req.Header.Set("Content-Type", "application/json")
	if rootToken != "" {
		req.Header.Set("Authorization", "Bearer "+rootToken)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		common.SysError(fmt.Sprintf("ticket #%d: failed to call AI: %s", ticketId, err.Error()))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		common.SysError(fmt.Sprintf("ticket #%d: AI returned status %d for model %s", ticketId, resp.StatusCode, setting.AiAssistantModel))
		return
	}

	var resData struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := common.DecodeJson(resp.Body, &resData); err != nil {
		common.SysError("failed to decode AI response: " + err.Error())
		return
	}

	if len(resData.Choices) == 0 {
		common.SysError(fmt.Sprintf("ticket #%d: AI returned no choices for model %s", ticketId, setting.AiAssistantModel))
		return
	}

	replyContent := resData.Choices[0].Message.Content

	// Add AI reply to ticket (acting as admin)
	msg, err := model.AddTicketMessage(ticketId, 0, true, replyContent, "")
	if err != nil {
		common.SysError("failed to add AI message: " + err.Error())
		return
	}

	GlobalTicketHub.BroadcastToTicket(ticketId, "new_message", msg)
}
