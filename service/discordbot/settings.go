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
	RPCEnabled         bool
	RPCApplicationID   string
	RPCDetails         string
	RPCState           string
	RPCLargeImageKey   string
	RPCLargeImageText  string
	RPCSmallImageKey   string
	RPCSmallImageText  string
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
	RPCEnabled:         true,
	RPCApplicationID:   "",
	RPCDetails:         "AI Platform Assistant",
	RPCState:           "Online",
	RPCLargeImageKey:   "bot_logo",
	RPCLargeImageText:  "AI Service",
	AutoReplyChannelID: "",
	AutoReplyModel:     "gpt-4o-mini",
	OnlyLinkedAccounts: false,
}

func getOptionValue(keys ...string) (string, bool) {
	for _, k := range keys {
		if val, ok := common.OptionMap[k]; ok {
			return strings.TrimSpace(val), true
		}
	}
	return "", false
}

func SyncSettingsFromOptions() {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()

	if val, ok := getOptionValue("discord_bot.enabled", "discord.enabled"); ok {
		CurrentSettings.Enabled = val == "true"
	}
	if val, ok := getOptionValue("discord_bot.token", "discord_bot.bot_token", "discord.bot_token"); ok && val != "" {
		CurrentSettings.BotToken = val
	}
	if val, ok := getOptionValue("discord_bot.bot_name", "discord.bot_name"); ok && val != "" {
		CurrentSettings.BotName = val
	}
	if val, ok := getOptionValue("discord_bot.prefix", "discord.prefix"); ok && val != "" {
		CurrentSettings.Prefix = val
	}
	if val, ok := getOptionValue("discord_bot.status", "discord.status"); ok && val != "" {
		CurrentSettings.Status = val
	}
	if val, ok := getOptionValue("discord_bot.language", "discord.language"); ok && val != "" {
		CurrentSettings.Language = val
	}
	if val, ok := getOptionValue("discord_bot.ai_system_prompt", "discord.ai_system_prompt"); ok && val != "" {
		CurrentSettings.AISystemPrompt = val
	}
	if val, ok := getOptionValue("discord_bot.auto_reply_channel_id", "discord.auto_reply_channel_id"); ok {
		CurrentSettings.AutoReplyChannelID = val
	}
	if val, ok := getOptionValue("discord_bot.auto_reply_model", "discord.auto_reply_model"); ok && val != "" {
		CurrentSettings.AutoReplyModel = val
	}
	if val, ok := getOptionValue("discord_bot.rpc_enabled", "discord.rpc_enabled", "discord.rpg_enabled"); ok {
		CurrentSettings.RPCEnabled = val == "true"
	}
	if val, ok := getOptionValue("discord_bot.rpc_app_id"); ok {
		CurrentSettings.RPCApplicationID = val
	}
	if val, ok := getOptionValue("discord_bot.rpc_details"); ok {
		CurrentSettings.RPCDetails = val
	}
	if val, ok := getOptionValue("discord_bot.rpc_state"); ok {
		CurrentSettings.RPCState = val
	}
	if val, ok := getOptionValue("discord_bot.rpc_large_image"); ok {
		CurrentSettings.RPCLargeImageKey = val
	}
	if val, ok := getOptionValue("discord_bot.rpc_large_text"); ok {
		CurrentSettings.RPCLargeImageText = val
	}
	if val, ok := getOptionValue("discord_bot.rpc_small_image"); ok {
		CurrentSettings.RPCSmallImageKey = val
	}
	if val, ok := getOptionValue("discord_bot.rpc_small_text"); ok {
		CurrentSettings.RPCSmallImageText = val
	}
	if val, ok := getOptionValue("discord_bot.only_linked_accounts", "discord.only_linked_accounts"); ok {
		CurrentSettings.OnlyLinkedAccounts = val == "true"
	}
	if val, ok := getOptionValue("discord_bot.embed_color", "discord.embed_color"); ok && val != "" {
		hexStr := strings.TrimPrefix(val, "#")
		if colorVal, err := strconv.ParseInt(hexStr, 16, 64); err == nil {
			CurrentSettings.EmbedColor = int(colorVal)
		}
	}
}
