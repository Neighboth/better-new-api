package discordbot

import (
	"strconv"
	"strings"
	"sync"

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
	ImageModel         string
	OnlyLinkedAccounts bool
}

// defaultSettings is the baseline used when an option key was never saved.
var defaultSettings = Settings{
	EmbedColor:         0x00ff00,
	Language:           "en",
	BotName:            "MyAIBot",
	AISystemPrompt:     "You are a helpful AI assistant.",
	Prefix:             "!",
	Status:             "Ready to help",
	RPCEnabled:         true,
	RPCDetails:         "AI Platform Assistant",
	RPCState:           "Online",
	RPCLargeImageKey:   "bot_logo",
	RPCLargeImageText:  "AI Service",
	AutoReplyModel:     "gpt-4o-mini",
	OnlyLinkedAccounts: false,
}

var (
	settingsMu      sync.RWMutex
	CurrentSettings = defaultSettings
)

// currentSettings returns a snapshot of the live configuration. Handlers must
// work on the snapshot: the bot is restarted on every option save, so reading
// the shared struct repeatedly can observe a torn value.
func currentSettings() Settings {
	settingsMu.RLock()
	defer settingsMu.RUnlock()
	return CurrentSettings
}

func setCurrentSettings(settings Settings) {
	settingsMu.Lock()
	CurrentSettings = settings
	settingsMu.Unlock()
}

func getOptionValue(keys ...string) (string, bool) {
	for _, key := range keys {
		if val, ok := common.OptionMap[key]; ok {
			return strings.TrimSpace(val), true
		}
	}
	return "", false
}

// SyncSettingsFromOptions rebuilds the configuration from the option table and
// publishes it atomically, returning the new snapshot.
func SyncSettingsFromOptions() Settings {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()

	settings := defaultSettings

	if val, ok := getOptionValue("discord_bot.enabled"); ok {
		settings.Enabled = val == "true"
	}
	if val, ok := getOptionValue("discord_bot.token", "discord_bot.bot_token"); ok && val != "" {
		settings.BotToken = val
	}
	if val, ok := getOptionValue("discord_bot.bot_name"); ok && val != "" {
		settings.BotName = val
	}
	if val, ok := getOptionValue("discord_bot.prefix"); ok && val != "" {
		settings.Prefix = val
	}
	if val, ok := getOptionValue("discord_bot.status"); ok && val != "" {
		settings.Status = val
	}
	if val, ok := getOptionValue("discord_bot.language"); ok && val != "" {
		settings.Language = val
	}
	if val, ok := getOptionValue("discord_bot.ai_system_prompt"); ok && val != "" {
		settings.AISystemPrompt = val
	}
	if val, ok := getOptionValue("discord_bot.auto_reply_channel_id"); ok {
		settings.AutoReplyChannelID = val
	}
	if val, ok := getOptionValue("discord_bot.auto_reply_model"); ok && val != "" {
		settings.AutoReplyModel = val
	}
	if val, ok := getOptionValue("discord_bot.image_model"); ok && val != "" {
		settings.ImageModel = val
	}
	if val, ok := getOptionValue("discord_bot.rpc_enabled"); ok {
		settings.RPCEnabled = val == "true"
	}
	if val, ok := getOptionValue("discord_bot.rpc_app_id"); ok {
		settings.RPCApplicationID = val
	}
	if val, ok := getOptionValue("discord_bot.rpc_details"); ok {
		settings.RPCDetails = val
	}
	if val, ok := getOptionValue("discord_bot.rpc_state"); ok {
		settings.RPCState = val
	}
	if val, ok := getOptionValue("discord_bot.rpc_large_image"); ok {
		settings.RPCLargeImageKey = val
	}
	if val, ok := getOptionValue("discord_bot.rpc_large_text"); ok {
		settings.RPCLargeImageText = val
	}
	if val, ok := getOptionValue("discord_bot.rpc_small_image"); ok {
		settings.RPCSmallImageKey = val
	}
	if val, ok := getOptionValue("discord_bot.rpc_small_text"); ok {
		settings.RPCSmallImageText = val
	}
	if val, ok := getOptionValue("discord_bot.only_linked_accounts"); ok {
		settings.OnlyLinkedAccounts = val == "true"
	}
	if val, ok := getOptionValue("discord_bot.embed_color"); ok && val != "" {
		hexStr := strings.TrimPrefix(val, "#")
		if colorVal, err := strconv.ParseInt(hexStr, 16, 64); err == nil {
			settings.EmbedColor = int(colorVal)
		}
	}

	setCurrentSettings(settings)
	return settings
}
