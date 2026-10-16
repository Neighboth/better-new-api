package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

// TicketSetting 工单与在线客服配置
type TicketSetting struct {
	Enabled                bool   `json:"enabled"`                   // 是否启用工单系统
	LiveSupportEnabled     bool   `json:"live_support_enabled"`      // 是否启用在线客服
	NotifyAdminOnNewTicket bool   `json:"notify_admin_on_new_ticket"` // 新工单时邮件通知管理员
	NotifyUserOnReply      bool   `json:"notify_user_on_reply"`       // 工单回复时邮件通知用户
	AiAssistantEnabled     bool   `json:"ai_assistant_enabled"`
	AiAssistantModel       string `json:"ai_assistant_model"`
	AiAssistantSystemPrompt string `json:"ai_assistant_system_prompt"`
}

var ticketSetting = TicketSetting{
	Enabled:                true,
	LiveSupportEnabled:     true,
	NotifyAdminOnNewTicket: true,
	NotifyUserOnReply:      true,
	AiAssistantEnabled:     true,
	AiAssistantModel:       "gemini-1.5-flash",
	AiAssistantSystemPrompt: "You are a helpful support assistant for our API platform.",
}

func init() {
	config.GlobalConfig.Register("ticket_setting", &ticketSetting)
}

func GetTicketSetting() *TicketSetting {
	return &ticketSetting
}

func IsTicketEnabled() bool {
	return ticketSetting.Enabled
}

func IsLiveSupportEnabled() bool {
	return ticketSetting.LiveSupportEnabled
}
