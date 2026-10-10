package discordbot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/bwmarrin/discordgo"
)

var (
	GlobalBot   *Bot
	globalBotMu sync.Mutex
)

type Bot struct {
	Session     *discordgo.Session
	historyMu   sync.RWMutex
	History     map[string][]MessageHistory // channelID -> messages
	UserHistory map[string][]MessageHistory // "userID@channelID" -> messages
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

	SyncSettingsFromOptions()
	if !CurrentSettings.Enabled || strings.TrimSpace(CurrentSettings.BotToken) == "" {
		if GlobalBot != nil {
			GlobalBot.Stop()
			GlobalBot = nil
		}
		return nil
	}

	if GlobalBot != nil {
		GlobalBot.Stop()
		GlobalBot = nil
	}

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

	if GlobalBot != nil {
		GlobalBot.Stop()
		GlobalBot = nil
		log.Printf("Discord bot stopped")
	}
}

func RestartBot() {
	go func() {
		_ = StartBot()
	}()
}

func NewBot() (*Bot, error) {
	s, err := discordgo.New("Bot " + CurrentSettings.BotToken)
	if err != nil {
		return nil, err
	}

	b := &Bot{
		Session:     s,
		History:     make(map[string][]MessageHistory),
		UserHistory: make(map[string][]MessageHistory),
	}

	s.AddHandler(b.messageCreate)
	s.AddHandler(b.ready)
	s.AddHandler(b.HandleInteraction)

	s.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentDirectMessages | discordgo.IntentsGuilds

	return b, nil
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
	log.Printf("Logged in as: %v#%v", s.State.User.Username, s.State.User.Discriminator)
	statusText := CurrentSettings.Status
	if CurrentSettings.RPCEnabled && CurrentSettings.RPCDetails != "" {
		statusText = fmt.Sprintf("%s - %s", CurrentSettings.RPCDetails, CurrentSettings.RPCState)
	}
	_ = s.UpdateGameStatus(0, statusText)
}

func (b *Bot) messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author == nil || m.Author.Bot || m.Author.ID == s.State.User.ID {
		return
	}

	content := strings.TrimSpace(m.Content)
	if content == "" {
		return
	}

	// Handle commands
	if strings.HasPrefix(content, CurrentSettings.Prefix) {
		b.handleCommand(s, m)
		return
	}

	// Auto-reply feature in designated channel
	if CurrentSettings.AutoReplyChannelID != "" && m.ChannelID == CurrentSettings.AutoReplyChannelID {
		b.handleAutoReply(s, m)
	}
}

func (b *Bot) handleAutoReply(s *discordgo.Session, m *discordgo.MessageCreate) {
	b.historyMu.Lock()
	userKey := m.Author.ID + "@" + m.ChannelID
	now := time.Now()
	thirtyDaysAgo := now.Add(-30 * 24 * time.Hour)

	// Filter history to last 30 days
	var validUserHistory []MessageHistory
	for _, h := range b.UserHistory[userKey] {
		if h.Timestamp.After(thirtyDaysAgo) {
			validUserHistory = append(validUserHistory, h)
		}
	}
	validUserHistory = append(validUserHistory, MessageHistory{
		Role:      "user",
		Content:   m.Content,
		Timestamp: now,
		AuthorID:  m.Author.ID,
	})
	b.UserHistory[userKey] = validUserHistory
	b.historyMu.Unlock()

	// Build API request messages
	type chatMsg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	var apiMsgs []chatMsg

	systemPrompt := CurrentSettings.AISystemPrompt
	if systemPrompt == "" {
		systemPrompt = "You are a helpful, friendly AI assistant. Reply concisely and format your text beautifully."
	}
	apiMsgs = append(apiMsgs, chatMsg{
		Role:    "system",
		Content: systemPrompt,
	})

	for _, h := range validUserHistory {
		apiMsgs = append(apiMsgs, chatMsg{
			Role:    h.Role,
			Content: h.Content,
		})
	}

	modelName := CurrentSettings.AutoReplyModel
	if modelName == "" {
		modelName = "gpt-4o-mini"
	}

	reqBody := map[string]any{
		"model":    modelName,
		"messages": apiMsgs,
		"stream":   false,
	}

	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		log.Printf("Failed to marshal discord bot AI request: %v", err)
		return
	}

	// Determine port and token
	port := os.Getenv("PORT")
	if port == "" && common.Port != nil {
		port = strconv.Itoa(*common.Port)
	}
	if port == "" {
		port = "3000"
	}

	var rootToken string
	var tok model.Token
	if err := model.DB.Where("user_id = 1").First(&tok).Error; err == nil {
		rootToken = tok.Key
	} else if err := model.DB.Where("status = 1").First(&tok).Error; err == nil {
		rootToken = tok.Key
	}

	httpReq, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://127.0.0.1:%s/v1/chat/completions", port), bytes.NewReader(jsonBytes))
	if err != nil {
		log.Printf("Failed to create AI request: %v", err)
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if rootToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+rootToken)
	}

	// Show typing indicator in discord
	_ = s.ChannelTyping(m.ChannelID)

	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		log.Printf("Failed to execute AI request: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Discord AI completion returned status: %d", resp.StatusCode)
		return
	}

	var completionResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&completionResp); err != nil || len(completionResp.Choices) == 0 {
		log.Printf("Failed to decode AI response: %v", err)
		return
	}

	aiReply := strings.TrimSpace(completionResp.Choices[0].Message.Content)
	if aiReply == "" {
		return
	}

	// Save AI reply to 30-day user history
	b.historyMu.Lock()
	b.UserHistory[userKey] = append(b.UserHistory[userKey], MessageHistory{
		Role:      "assistant",
		Content:   aiReply,
		Timestamp: time.Now(),
		AuthorID:  s.State.User.ID,
	})
	b.historyMu.Unlock()

	// Send formatted Discord reply
	embedColor := CurrentSettings.EmbedColor
	if embedColor <= 0 {
		embedColor = 0x5865F2 // Discord Blurple
	}

	// If message is short enough for embed description (max 4096)
	if len(aiReply) <= 4000 {
		embed := &discordgo.MessageEmbed{
			Description: aiReply,
			Color:       embedColor,
			Footer: &discordgo.MessageEmbedFooter{
				Text: fmt.Sprintf("%s • %s", CurrentSettings.BotName, modelName),
			},
			Timestamp: time.Now().Format(time.RFC3339),
		}
		_, _ = s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
			Embeds: []*discordgo.MessageEmbed{embed},
			Reference: &discordgo.MessageReference{
				MessageID: m.ID,
				ChannelID: m.ChannelID,
				GuildID:   m.GuildID,
			},
		})
	} else {
		// Split message if extremely long
		for len(aiReply) > 0 {
			chunkSize := 1950
			if len(aiReply) < chunkSize {
				chunkSize = len(aiReply)
			}
			chunk := aiReply[:chunkSize]
			aiReply = aiReply[chunkSize:]
			_, _ = s.ChannelMessageSend(m.ChannelID, chunk)
		}
	}
}
