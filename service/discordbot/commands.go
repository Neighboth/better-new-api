package discordbot

import (
	"strings"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) handleCommand(s *discordgo.Session, m *discordgo.MessageCreate) {
	args := strings.Split(m.Content[len(CurrentSettings.Prefix):], " ")
	command := strings.ToLower(args[0])

	switch command {
	case "reset":
		b.historyMu.Lock()
		userKey := m.Author.ID + "@" + m.ChannelID
		delete(b.UserHistory, userKey)
		delete(b.History, m.ChannelID)
		b.historyMu.Unlock()
		
		s.ChannelMessageSendReply(m.ChannelID, "Sohbet hafızanız başarıyla sıfırlandı. Yeni bir konuşmaya başlayabiliriz!", &discordgo.MessageReference{
			MessageID: m.ID,
			ChannelID: m.ChannelID,
			GuildID:   m.GuildID,
		})
	
	case "models":
		b.sendModelsMenu(s, m.ChannelID)
	
	case "imagine":
		prompt := strings.Join(args[1:], " ")
		b.generateImage(s, m, prompt)
	}
}

func (b *Bot) sendModelsMenu(s *discordgo.Session, channelID string) {
	embed := &discordgo.MessageEmbed{
		Title:       "Available Models",
		Description: "Select a model category from the dropdown below.",
		Color:       CurrentSettings.EmbedColor,
	}

	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					CustomID:    "model_select",
					Placeholder: "Select a category...",
					Options: []discordgo.SelectMenuOption{
						{Label: "Video", Value: "video", Description: "Video generation models"},
						{Label: "Image", Value: "image", Description: "Image generation models"},
						{Label: "TTS", Value: "tts", Description: "Text-to-Speech models"},
						{Label: "STT", Value: "stt", Description: "Speech-to-Text models"},
						{Label: "Realtime", Value: "realtime", Description: "Realtime AI models"},
						{Label: "Free", Value: "free", Description: "Models with 0 cost"},
					},
				},
			},
		},
	}

	s.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Embeds:     []*discordgo.MessageEmbed{embed},
		Components: components,
	})
}

// Handle Select Menu Interactions
func (b *Bot) HandleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionMessageComponent {
		return
	}

	data := i.MessageComponentData()
	if data.CustomID == "model_select" {
		selected := data.Values[0]
		content := "You selected category: " + selected + ". Available models: ModelA, ModelB" // Mocked models

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: content,
				Flags:   discordgo.MessageFlagsEphemeral, // Private message
			},
		})
	}
}

func (b *Bot) generateImage(s *discordgo.Session, m *discordgo.MessageCreate, prompt string) {
	if CurrentSettings.OnlyLinkedAccounts {
		// Mock account check
		linked := false // Assume false for now
		if !linked {
			s.ChannelMessageSend(m.ChannelID, "You must link your account to generate images.")
			return
		}
	}

	// Mock deduct balance
	balanceDeducted := true
	if !balanceDeducted {
		s.ChannelMessageSend(m.ChannelID, "Insufficient balance.")
		return
	}

	// Generate image
	s.ChannelMessageSend(m.ChannelID, "Generating image for prompt: "+prompt+"... (mocked result)")
}
