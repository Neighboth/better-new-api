package discordbot

import (
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/model"
	"github.com/bwmarrin/discordgo"
)

// Commands are prefix based. Every reply is rendered with Components V2 so the
// bot never mixes the legacy `content`/`embeds` fields with components.
func (b *Bot) handleCommand(s *discordgo.Session, m *discordgo.MessageCreate, settings Settings) {
	trimmed := strings.TrimSpace(m.Content)
	if !strings.HasPrefix(trimmed, settings.Prefix) {
		return
	}
	args := strings.Fields(strings.TrimPrefix(trimmed, settings.Prefix))
	if len(args) == 0 {
		return
	}

	switch strings.ToLower(args[0]) {
	case "help", "yardim", "yardım":
		b.sendHelpPanel(s, m.ChannelID, settings)
	case "reset":
		b.resetHistory(s, m)
	case "models", "modeller":
		b.sendModelsPanel(s, m.ChannelID)
	case "imagine", "resim":
		b.generateImage(s, m, strings.Join(args[1:], " "), settings)
	case "owner", "panel", "admin", "yonetim", "yönetim":
		b.handleOwnerPanelCommand(s, m)
	}
}

func (b *Bot) resetHistory(s *discordgo.Session, m *discordgo.MessageCreate) {
	b.historyMu.Lock()
	delete(b.UserHistory, m.Author.ID+"@"+m.ChannelID)
	b.historyMu.Unlock()

	if _, err := sendPanel(s, m.ChannelID,
		textDisplay("Sohbet hafizasi sifirlandi.\nYeni bir konusmaya baslayabiliriz."),
	); err != nil {
		log.Printf("Discord reset reply failed: %v", err)
		_, _ = s.ChannelMessageSend(m.ChannelID, "Sohbet hafizasi sifirlandi.")
	}
}

func (b *Bot) sendHelpPanel(s *discordgo.Session, channelID string, settings Settings) {
	prefix := settings.Prefix
	lines := []string{
		fmt.Sprintf("%s %s - komut listesi", inlineCode(prefix+"help"), settings.BotName),
		"",
		fmt.Sprintf("%s - bu yardim panelini gosterir", inlineCode(prefix+"help")),
		fmt.Sprintf("%s - bu kanaldaki sohbet hafizanizi sifirlar", inlineCode(prefix+"reset")),
		fmt.Sprintf("%s - sunucudaki kullanilabilir modelleri listeler", inlineCode(prefix+"models")),
		fmt.Sprintf("%s <prompt> - gorsel uretir", inlineCode(prefix+"imagine")),
		fmt.Sprintf("%s - yonetici paneli (yalnizca DM, hesabi bagli yoneticiler)", inlineCode(prefix+"owner")),
	}
	if _, err := sendPanel(s, channelID, textDisplay(strings.Join(lines, "\n"))); err != nil {
		log.Printf("Discord help reply failed: %v", err)
		_, _ = s.ChannelMessageSend(channelID, strings.Join(lines, "\n"))
	}
}

type modelListResponse struct {
	Data []struct {
		ID      string `json:"id"`
		OwnedBy string `json:"owned_by"`
	} `json:"data"`
}

func (b *Bot) fetchAvailableModels() ([]string, error) {
	var response modelListResponse
	if err := getFromLocalRelay("/v1/models", &response); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(response.Data))
	for _, item := range response.Data {
		if item.ID != "" {
			names = append(names, item.ID)
		}
	}
	sort.Strings(names)
	return names, nil
}

func (b *Bot) sendModelsPanel(s *discordgo.Session, channelID string) {
	names, err := b.fetchAvailableModels()
	if err != nil {
		if _, serr := sendPanel(s, channelID, textDisplay("Modeller alinamadi:\n"+err.Error())); serr != nil {
			_, _ = s.ChannelMessageSend(channelID, "Modeller alinamadi: "+err.Error())
		}
		return
	}
	if _, err := sendPanel(s, channelID,
		textDisplay(fmt.Sprintf("Kullanilabilir modeller (%d)", len(names))),
		separator(),
		textDisplay(bulletList(names, "Bu sunucuda etkin model yok.")),
	); err != nil {
		log.Printf("Discord models reply failed: %v", err)
		_, _ = s.ChannelMessageSend(channelID, "Kullanilabilir modeller: "+strings.Join(names, ", "))
	}
}

type imageGenerationResponse struct {
	Data []struct {
		URL           string `json:"url"`
		RevisedPrompt string `json:"revised_prompt"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (b *Bot) generateImage(s *discordgo.Session, m *discordgo.MessageCreate, prompt string, settings Settings) {
	reply := func(text string) {
		if _, err := sendPanel(s, m.ChannelID, textDisplay(text)); err != nil {
			log.Printf("Discord imagine reply failed: %v", err)
			_, _ = s.ChannelMessageSend(m.ChannelID, text)
		}
	}
	if strings.TrimSpace(prompt) == "" {
		reply("Kullanim: `" + settings.Prefix + "imagine <prompt>`")
		return
	}
	if settings.ImageModel == "" {
		reply("Gorsel modeli yapilandirilmamis.\nYonetici panelinden discord_bot.image_model ayarini doldurun.")
		return
	}
	if !b.userIsLinked(m.Author.ID) && settings.OnlyLinkedAccounts {
		reply("Hesap bagli degil.\nGorsel uretmek icin site hesabinizi Discord ile baglamaniz gerekiyor.")
		return
	}

	// Acknowledge first so the command never looks unanswered while relay runs.
	reply("Gorsel uretiliyor, lutfen bekleyin.")
	var response imageGenerationResponse
	err := postToLocalRelay("/v1/images/generations", map[string]any{
		"model":  settings.ImageModel,
		"prompt": prompt,
		"n":      1,
	}, &response)
	if err != nil {
		reply("Gorsel uretilemedi:\n" + err.Error())
		return
	}
	if response.Error != nil && response.Error.Message != "" {
		reply("Gorsel uretilemedi:\n" + response.Error.Message)
		return
	}
	if len(response.Data) == 0 || response.Data[0].URL == "" {
		reply("Gorsel uretilemedi:\nSaglayici gorsel dondurmedi.")
		return
	}

	_, _ = sendPanel(s, m.ChannelID,
		textDisplay("Gorsel hazir:\n`"+truncateText(prompt, 200)+"`"),
		discordgo.MediaGallery{Items: []discordgo.MediaGalleryItem{{Media: discordgo.UnfurledMediaItem{URL: response.Data[0].URL}}}},
	)
}

// userIsLinked reports whether a Discord account is bound to a site user.
func (b *Bot) userIsLinked(discordUserID string) bool {
	user, err := findUserByDiscordID(discordUserID)
	return err == nil && user != nil
}

// findUserByDiscordID resolves a site user from a Discord snowflake. The model
// helper FillUserByDiscordId swallows its lookup error, so the query is repeated
// here to keep "not found" distinguishable from a database failure.
func findUserByDiscordID(discordUserID string) (*model.User, error) {
	if strings.TrimSpace(discordUserID) == "" {
		return nil, fmt.Errorf("discord hesabı belirtilmedi")
	}
	var user model.User
	if err := model.DB.Where("discord_id = ?", discordUserID).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// HandleInteraction routes slash commands, component and modal interactions. Non owner-panel
// interactions fall back to the legacy model selector.
func (b *Bot) HandleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		b.handleApplicationCommand(s, i)
		return
	case discordgo.InteractionMessageComponent:
		data := i.MessageComponentData()
		if strings.HasPrefix(data.CustomID, ownerPanelPrefix) {
			b.handleOwnerPanelComponent(s, i, data)
			return
		}
		b.handleLegacyComponent(s, i, data)
	case discordgo.InteractionModalSubmit:
		data := i.ModalSubmitData()
		if strings.HasPrefix(data.CustomID, ownerModalPrefix) {
			b.handleOwnerPanelModal(s, i, data)
			return
		}
	}
}

func (b *Bot) handleApplicationCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()
	settings := currentSettings()

	switch strings.ToLower(data.Name) {
	case "help":
		lines := []string{
			fmt.Sprintf("**%s - Komut Listesi**", settings.BotName),
			"",
			"`/help` - Bu yardım panelini gösterir",
			"`/reset` - Bu kanaldaki sohbet hafızasını sıfırlar",
			"`/models` - Kullanılabilir AI modellerini listeler",
			"`/imagine <prompt>` - Yapay zeka ile görsel üretir",
			"`/stats` - Sunucu ve bot istatistiklerini gösterir",
			"`/ping` - Bot yanıt süresini ölçer",
			"`/owner` - Yönetici kontrol panelini açar (DM)",
		}
		_ = respondPanel(s, i, textDisplay(strings.Join(lines, "\n")))

	case "reset":
		userID := interactionUserID(i)
		b.historyMu.Lock()
		delete(b.UserHistory, userID+"@"+i.ChannelID)
		b.historyMu.Unlock()
		_ = respondPanel(s, i, textDisplay("Sohbet hafızası sıfırlandı.\nYeni bir konuşmaya başlayabiliriz."))

	case "models":
		names, err := b.fetchAvailableModels()
		if err != nil {
			_ = respondPanel(s, i, textDisplay("Modeller alınamadı:\n"+err.Error()))
			return
		}
		_ = respondPanel(s, i,
			textDisplay(fmt.Sprintf("Kullanılabilir Modeller (%d)", len(names))),
			separator(),
			textDisplay(bulletList(names, "_Etkin model yok._")),
		)

	case "stats":
		guildName := "DM"
		memberCount := 0
		if i.GuildID != "" {
			if guild, err := s.GuildWithCounts(i.GuildID); err == nil {
				guildName = guild.Name
				memberCount = guild.ApproximateMemberCount
			}
		}
		statsText := fmt.Sprintf("📊 **Bot & Sunucu İstatistikleri**\n• Bot: `%s`\n• Konum: **%s** (Üye: %d)\n• Durum: 🟢 Çalışıyor\n• Modeller: `/models` ile listelenebilir",
			settings.BotName, guildName, memberCount)
		_ = respondPanel(s, i, textDisplay(statsText))

	case "ping":
		heartbeat := s.HeartbeatLatency().Milliseconds()
		_ = respondPanel(s, i, textDisplay(fmt.Sprintf("🏓 **Pong!** Bot gecikmesi: `%dms`", heartbeat)))

	case "imagine":
		prompt := ""
		for _, opt := range data.Options {
			if opt.Name == "prompt" {
				prompt = opt.StringValue()
			}
		}
		if prompt == "" {
			_ = respondPanel(s, i, textDisplay("Lütfen üretmek istediğiniz görsel için bir açıklama (prompt) girin."))
			return
		}
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		})
		go func() {
			var request struct {
				Prompt string `json:"prompt"`
				Model  string `json:"model"`
			}
			request.Prompt = prompt
			request.Model = settings.AutoReplyModel
			if request.Model == "" {
				request.Model = "dall-e-3"
			}
			var response struct {
				Data []struct {
					URL string `json:"url"`
				} `json:"data"`
				Error *struct {
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := postToLocalRelay("/v1/images/generations", request, &response); err != nil {
				_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
					Content: "❌ Görsel üretilemedi: " + err.Error(),
				})
				return
			}
			if response.Error != nil && response.Error.Message != "" {
				_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
					Content: "❌ Hata: " + response.Error.Message,
				})
				return
			}
			if len(response.Data) == 0 || response.Data[0].URL == "" {
				_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
					Content: "❌ Görsel bağlantısı alınamadı.",
				})
				return
			}
			imageURL := response.Data[0].URL
			_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: fmt.Sprintf("🎨 **Prompt:** %s\n%s", prompt, imageURL),
			})
		}()

	case "owner", "panel", "admin":
		_ = respondPanel(s, i, textDisplay("🛡️ **Yönetici Paneli**\nYönetici kontrol panelini kullanmak için lütfen bota doğrudan özel mesaj (DM) üzerinden `!owner` yazın."))

	default:
		_ = respondPanel(s, i, textDisplay("Bilinmeyen komut."))
	}
}

func (b *Bot) handleLegacyComponent(s *discordgo.Session, i *discordgo.InteractionCreate, data discordgo.MessageComponentInteractionData) {
	if data.CustomID != "model_select" || len(data.Values) == 0 {
		return
	}
	names, err := b.fetchAvailableModels()
	if err != nil {
		_ = respondPanel(s, i, textDisplay(" **Modeller alınamadı**\n"+err.Error()))
		return
	}
	_ = respondPanel(s, i,
		textDisplay(fmt.Sprintf(" **%s** kategorisi seçildi", data.Values[0])),
		separator(),
		textDisplay(bulletList(names, "_Etkin model yok._")),
	)
}
