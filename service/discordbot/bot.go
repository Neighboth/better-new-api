package discordbot

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

type Bot struct {
	Session *discordgo.Session
	History map[string][]MessageHistory // channelID -> messages
}

type MessageHistory struct {
	Content   string
	Timestamp time.Time
	AuthorID  string
}

func NewBot() (*Bot, error) {
	s, err := discordgo.New("Bot " + CurrentSettings.BotToken)
	if err != nil {
		return nil, err
	}

	b := &Bot{
		Session: s,
		History: make(map[string][]MessageHistory),
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
	b.Session.Close()
}

func (b *Bot) ready(s *discordgo.Session, event *discordgo.Ready) {
	log.Printf("Logged in as: %v#%v", s.State.User.Username, s.State.User.Discriminator)
	s.UpdateGameStatus(0, CurrentSettings.Status)
}

func (b *Bot) messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	// Save history (30 days logic would be a cleanup cron)
	b.History[m.ChannelID] = append(b.History[m.ChannelID], MessageHistory{
		Content:   m.Content,
		Timestamp: time.Now(),
		AuthorID:  m.Author.ID,
	})

	// Handle commands
	if strings.HasPrefix(m.Content, CurrentSettings.Prefix) {
		b.handleCommand(s, m)
		return
	}

	// Auto-reply feature
	if m.ChannelID == CurrentSettings.AutoReplyChannelID {
		b.handleAutoReply(s, m)
	}
}

func (b *Bot) handleAutoReply(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Call AI model (mocked)
	response := fmt.Sprintf("AI (%s) response to: %s", CurrentSettings.AutoReplyModel, m.Content)
	s.ChannelMessageSend(m.ChannelID, response)
}
