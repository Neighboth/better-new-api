package discordbot

type Settings struct {
	BotToken       string
	EmbedColor     int
	Language       string
	BotName        string
	AISystemPrompt string
	Prefix         string
	Status         string
	RPGEnabled     bool
	AutoReplyChannelID string
	AutoReplyModel     string
	OnlyLinkedAccounts bool
}

var CurrentSettings = Settings{
	BotToken:       "YOUR_TOKEN_HERE",
	EmbedColor:     0x00ff00,
	Language:       "en",
	BotName:        "MyAIBot",
	AISystemPrompt: "You are a helpful AI assistant.",
	Prefix:         "!",
	Status:         "Ready to help",
	RPGEnabled:     true,
	AutoReplyChannelID: "",
	AutoReplyModel:     "default-text-model",
	OnlyLinkedAccounts: false,
}
