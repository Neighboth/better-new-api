package discordbot

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

type Settings struct {
	Enabled            bool
	BotToken           string
	EmbedColor         int
	Language           string
	BotName            string
	AISystemPrompt     string
	Prefix             string
	Status             string
	RPGEnabled         bool
	AutoReplyChannelID string
	AutoReplyModel     string
	OnlyLinkedAccounts bool
}

var CurrentSettings = Settings{
	Enabled:            false,
	BotToken:           "",
	EmbedColor:         0x00ff00,
	Language:           "en",
	BotName:            "MyAIBot",
	AISystemPrompt:     "You are a helpful AI assistant.",
	Prefix:             "!",
	Status:             "Ready to help",
	RPGEnabled:         true,
	AutoReplyChannelID: "",
	AutoReplyModel:     "gpt-4o-mini",
	OnlyLinkedAccounts: false,
}

func SyncSettingsFromOptions() {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()

	if val, ok := common.OptionMap["discord.enabled"]; ok {
		CurrentSettings.Enabled = val == "true"
	}
	if val, ok := common.OptionMap["discord.bot_token"]; ok && val != "" {
		CurrentSettings.BotToken = val
	}
	if val, ok := common.OptionMap["discord.bot_name"]; ok && val != "" {
		CurrentSettings.BotName = val
	}
	if val, ok := common.OptionMap["discord.prefix"]; ok && val != "" {
		CurrentSettings.Prefix = val
	}
	if val, ok := common.OptionMap["discord.status"]; ok && val != "" {
		CurrentSettings.Status = val
	}
	if val, ok := common.OptionMap["discord.language"]; ok && val != "" {
		CurrentSettings.Language = val
	}
	if val, ok := common.OptionMap["discord.ai_system_prompt"]; ok && val != "" {
		CurrentSettings.AISystemPrompt = val
	}
	if val, ok := common.OptionMap["discord.auto_reply_channel_id"]; ok {
		CurrentSettings.AutoReplyChannelID = val
	}
	if val, ok := common.OptionMap["discord.auto_reply_model"]; ok && val != "" {
		CurrentSettings.AutoReplyModel = val
	}
	if val, ok := common.OptionMap["discord.rpg_enabled"]; ok {
		CurrentSettings.RPGEnabled = val == "true"
	}
	if val, ok := common.OptionMap["discord.only_linked_accounts"]; ok {
		CurrentSettings.OnlyLinkedAccounts = val == "true"
	}
	if val, ok := common.OptionMap["discord.embed_color"]; ok && val != "" {
		hexStr := strings.TrimPrefix(val, "#")
		if colorVal, err := strconv.ParseInt(hexStr, 16, 64); err == nil {
			CurrentSettings.EmbedColor = int(colorVal)
		}
	}
}
