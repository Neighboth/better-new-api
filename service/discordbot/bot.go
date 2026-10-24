package discordbot

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

var (
	GlobalBot   *Bot
	globalBotMu sync.Mutex

	restartMu    sync.Mutex
	restartTimer *time.Timer
)

type Bot struct {
	Session *discordgo.Session

	historyMu   sync.RWMutex
	UserHistory map[string][]MessageHistory // "userID@channelID" -> messages

	ownerMu       sync.Mutex
	ownerSessions map[string]*ownerSession     // discord user id -> authenticated session
	ownerAuth     map[string]*ownerAuthRequest // discord user id -> pending password challenge
}

type MessageHistory struct {
	Role      string
	Content   string
	Timestamp time.Time
	AuthorID  string
}

func StartBot() error {
	globalBotMu.Lock()
	defer globalBotMu.Unlock()

	settings := SyncSettingsFromOptions()
	if !settings.Enabled {
		stopBotLocked()
		log.Printf("Discord bot is disabled (discord_bot.enabled is not \"true\"); not starting")
		return nil
	}
	if strings.TrimSpace(settings.BotToken) == "" {
		stopBotLocked()
		log.Printf("Discord bot is enabled but discord_bot.token is empty; not starting")
		return nil
	}
	stopBotLocked()

	bot, err := NewBot()
	if err != nil {
		log.Printf("Failed to initialize Discord bot: %v", err)
		return err
	}
	if err := bot.Start(); err != nil {
		log.Printf("Failed to start Discord bot: %v", err)
		return err
	}

	GlobalBot = bot
	log.Printf("Discord bot started successfully")
	return nil
}

func StopBot() {
	globalBotMu.Lock()
	defer globalBotMu.Unlock()
	stopBotLocked()
}

func stopBotLocked() {
	if GlobalBot == nil {
		return
	}
	GlobalBot.Stop()
	GlobalBot = nil
	log.Printf("Discord bot stopped")
}

func RestartBot() {
	restartMu.Lock()
	defer restartMu.Unlock()
	if restartTimer != nil {
		restartTimer.Stop()
	}
	// Coalesce rapid option saves (the settings form issues ~20 sequential
	// updates) into a single restart so duplicate sessions never stack up
	// and every message gets exactly one reply.
	restartTimer = time.AfterFunc(2*time.Second, func() {
		if err := StartBot(); err != nil {
			log.Printf("Discord bot restart failed: %v", err)
		}
	})
}

func NewBot() (*Bot, error) {
	session, err := discordgo.New("Bot " + currentSettings().BotToken)
	if err != nil {
		return nil, err
	}

	bot := &Bot{
		Session:       session,
		UserHistory:   make(map[string][]MessageHistory),
		ownerSessions: make(map[string]*ownerSession),
		ownerAuth:     make(map[string]*ownerAuthRequest),
	}

	session.AddHandler(bot.messageCreate)
	session.AddHandler(bot.ready)
	session.AddHandler(bot.HandleInteraction)

	// IntentMessageContent is a privileged intent: without it Discord delivers an
	// empty content field for every message, which made the bot ignore all
	// commands while still appearing online. It must also be enabled in the
	// Discord Developer Portal (Bot -> Privileged Gateway Intents).
	session.Identify.Intents = discordgo.IntentsGuilds |
		discordgo.IntentsGuildMessages |
		discordgo.IntentDirectMessages |
		discordgo.IntentMessageContent

	return bot, nil
}

func (b *Bot) Start() error {
	return b.Session.Open()
}

func (b *Bot) Stop() {
	if b.Session != nil {
		_ = b.Session.Close()
	}
}

func (b *Bot) ready(s *discordgo.Session, event *discordgo.Ready) {
	log.Printf("Discord bot logged in as %s (id %s)", s.State.User.Username, s.State.User.ID)

	settings := currentSettings()
	statusText := settings.Status
	if settings.RPCEnabled && settings.RPCDetails != "" {
		statusText = strings.TrimSpace(settings.RPCDetails + " - " + settings.RPCState)
	}
	if statusText == "" {
		statusText = "Ready to help"
	}
	if err := s.UpdateGameStatus(0, statusText); err != nil {
		log.Printf("Discord bot could not update its status: %v", err)
	}
}

func (b *Bot) messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author == nil || m.Author.Bot || m.Author.ID == s.State.User.ID {
		return
	}

	settings := currentSettings()
	content := strings.TrimSpace(m.Content)
	if content == "" {
		return
	}

	// A pending owner-panel challenge consumes the next DM before anything else,
	// so the typed password is never treated as a command or auto-reply prompt.
	if m.GuildID == "" && b.consumeOwnerAuthReply(s, m) {
		return
	}

	if settings.Prefix != "" && strings.HasPrefix(content, settings.Prefix) {
		b.handleCommand(s, m, settings)
		return
	}

	if settings.AutoReplyChannelID != "" && m.ChannelID == settings.AutoReplyChannelID {
		b.handleAutoReply(s, m, settings)
	}
}

func (b *Bot) handleAutoReply(s *discordgo.Session, m *discordgo.MessageCreate, settings Settings) {
	b.historyMu.Lock()
	userKey := m.Author.ID + "@" + m.ChannelID
	cutoff := time.Now().Add(-30 * 24 * time.Hour)

	history := b.UserHistory[userKey][:0]
	for _, entry := range b.UserHistory[userKey] {
		if entry.Timestamp.After(cutoff) {
			history = append(history, entry)
		}
	}
	history = append(history, MessageHistory{Role: "user", Content: m.Content, Timestamp: time.Now(), AuthorID: m.Author.ID})
	b.UserHistory[userKey] = history
	b.historyMu.Unlock()

	systemPrompt := settings.AISystemPrompt
	if systemPrompt == "" {
		systemPrompt = "You are a helpful, friendly AI assistant. Reply concisely and format your text beautifully."
	}
	messages := []chatMessage{{Role: "system", Content: systemPrompt}}
	for _, entry := range history {
		messages = append(messages, chatMessage{Role: entry.Role, Content: entry.Content})
	}

	modelName := settings.AutoReplyModel
	if modelName == "" {
		modelName = "gpt-4o-mini"
	}

	reply, err := callLocalChatCompletionWithTools(s, m, modelName, messages)
	if err != nil {
		log.Printf("Discord auto-reply failed: %v", err)
		_, _ = sendPanel(s, m.ChannelID,
			textDisplay(" **Yanıt üretilemedi**\n"+err.Error()),
		)
		return
	}

	b.historyMu.Lock()
	b.UserHistory[userKey] = append(b.UserHistory[userKey], MessageHistory{
		Role:      "assistant",
		Content:   reply,
		Timestamp: time.Now(),
		AuthorID:  s.State.User.ID,
	})
	b.historyMu.Unlock()

	if _, err := sendPanel(s, m.ChannelID,
		textDisplay(reply),
		separator(),
		textDisplay(fmt.Sprintf("_%s  %s_", settings.BotName, modelName)),
	); err != nil {
		log.Printf("Discord auto-reply could not be delivered: %v", err)
	}
}

type chatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type chatMessageRequest struct {
	Model    string           `json:"model"`
	Messages any              `json:"messages"`
	Stream   bool             `json:"stream"`
	Tools    []ToolDefinition `json:"tools,omitempty"`
}

// callLocalChatCompletionWithTools handles conversational AI with native tool execution and permission checks.
func callLocalChatCompletionWithTools(s *discordgo.Session, m *discordgo.MessageCreate, modelName string, initialMessages []chatMessage) (string, error) {
	tools := GetDiscordBotTools()
	messages := make([]chatMessage, len(initialMessages))
	copy(messages, initialMessages)

	for turn := 0; turn < 4; turn++ {
		request := chatMessageRequest{
			Model:    modelName,
			Messages: messages,
			Stream:   false,
			Tools:    tools,
		}
		var completion struct {
			Choices []struct {
				Message struct {
					Content   string     `json:"content"`
					ToolCalls []ToolCall `json:"tool_calls"`
				} `json:"message"`
			} `json:"choices"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := postToLocalRelay("/v1/chat/completions", request, &completion); err != nil {
			return "", err
		}
		if completion.Error != nil && completion.Error.Message != "" {
			return "", fmt.Errorf("%s", completion.Error.Message)
		}
		if len(completion.Choices) == 0 {
			return "", fmt.Errorf("relay returned no completion choices")
		}

		choiceMsg := completion.Choices[0].Message
		if len(choiceMsg.ToolCalls) == 0 {
			reply := strings.TrimSpace(choiceMsg.Content)
			if reply == "" {
				return "", fmt.Errorf("model returned an empty answer")
			}
			return reply, nil
		}

		messages = append(messages, chatMessage{
			Role:      "assistant",
			Content:   choiceMsg.Content,
			ToolCalls: choiceMsg.ToolCalls,
		})

		for _, tc := range choiceMsg.ToolCalls {
			toolResult := ExecuteDiscordTool(s, m, tc)
			messages = append(messages, chatMessage{
				Role:       "tool",
				Content:    toolResult,
				ToolCallID: tc.ID,
			})
		}
	}
	return "Maximum tool iterations reached.", nil
}
