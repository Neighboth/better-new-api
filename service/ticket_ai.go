package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
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

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		common.SysError("failed to marshal AI request: " + err.Error())
		return
	}

	port := *common.Port
	url := fmt.Sprintf("http://127.0.0.1:%d/v1/chat/completions", port)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		common.SysError("failed to create AI request: " + err.Error())
		return
	}

	// Find the root user token to make the API call
	var rootToken string
	// For this system, we can either look up a token or just bypass auth for local requests
	// if we add a backdoor, or we can find a token belonging to user 1.
	var token model.Token
	err = model.DB.Where("user_id = 1").First(&token).Error
	if err == nil {
		rootToken = token.Key
	} else {
		// Just generate a temporary token or find any valid token
		err = model.DB.Where("status = 1").First(&token).Error
		if err == nil {
			rootToken = token.Key
		}
	}

	req.Header.Set("Content-Type", "application/json")
	if rootToken != "" {
		req.Header.Set("Authorization", "Bearer "+rootToken)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		common.SysError("failed to call AI: " + err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		common.SysError(fmt.Sprintf("AI returned status %d", resp.StatusCode))
		return
	}

	var resData struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&resData); err != nil {
		common.SysError("failed to decode AI response: " + err.Error())
		return
	}

	if len(resData.Choices) == 0 {
		common.SysError("AI returned no choices")
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
