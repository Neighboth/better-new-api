package discordbot

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
)

// Every bot message is rendered with Components V2: a coloured container that
// behaves like an embed, filled with text displays, separators and action rows.
// Components V2 forbids `content` and `embeds` on the same message, so nothing
// in this package may fall back to those fields.

const (
	// textDisplayLimit is Discord's maximum length for one TextDisplay block.
	textDisplayLimit = 4000
	// panelListLimit keeps a generated list inside the component budget of one
	// message.
	panelListLimit = 25
)

func accentColor() *int {
	color := CurrentSettings.EmbedColor
	if color <= 0 {
		color = 0x5865F2
	}
	return &color
}

func sanitizeMentions(text string) string {
	text = strings.ReplaceAll(text, "@everyone", "@\u200beveryone")
	text = strings.ReplaceAll(text, "@here", "@\u200bhere")
	return text
}

func textDisplay(content string) discordgo.TextDisplay {
	return discordgo.TextDisplay{Content: truncateText(sanitizeMentions(content), textDisplayLimit)}
}

func separator() discordgo.Separator {
	divider := true
	spacing := discordgo.SeparatorSpacingSizeSmall
	return discordgo.Separator{Divider: &divider, Spacing: &spacing}
}

func container(components ...discordgo.MessageComponent) discordgo.Container {
	return discordgo.Container{AccentColor: accentColor(), Components: components}
}

func actionRow(components ...discordgo.MessageComponent) discordgo.ActionsRow {
	return discordgo.ActionsRow{Components: components}
}

func primaryButton(customID, label string) discordgo.Button {
	return discordgo.Button{Style: discordgo.PrimaryButton, Label: label, CustomID: customID}
}

func secondaryButton(customID, label string) discordgo.Button {
	return discordgo.Button{Style: discordgo.SecondaryButton, Label: label, CustomID: customID}
}

func successButton(customID, label string) discordgo.Button {
	return discordgo.Button{Style: discordgo.SuccessButton, Label: label, CustomID: customID}
}

func dangerButton(customID, label string) discordgo.Button {
	return discordgo.Button{Style: discordgo.DangerButton, Label: label, CustomID: customID}
}

// sendPanel posts a message built from components with Components V2 priority and universal V1 fallback.
func sendPanel(s *discordgo.Session, channelID string, components ...discordgo.MessageComponent) (*discordgo.Message, error) {
	return sendPanelReply(s, channelID, nil, components...)
}

// sendPanelReply posts a message or reply with sanitized mentions and V1 fallback.
func sendPanelReply(s *discordgo.Session, channelID string, ref *discordgo.MessageReference, components ...discordgo.MessageComponent) (*discordgo.Message, error) {
	allowedMentions := &discordgo.MessageAllowedMentions{
		Parse:       []discordgo.AllowedMentionType{}, // Never allow AI or bot to ping @everyone, @here, or @roles
		RepliedUser: true,
	}
	msg, err := s.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Components:      []discordgo.MessageComponent{container(components...)},
		Flags:           discordgo.MessageFlagsIsComponentsV2,
		Reference:       ref,
		AllowedMentions: allowedMentions,
	})
	if err == nil {
		return msg, nil
	}
	// Fallback to standard Embed + ActionRows:
	embed, rows := extractV1(components)
	return s.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Embeds:          []*discordgo.MessageEmbed{embed},
		Components:      rows,
		Reference:       ref,
		AllowedMentions: allowedMentions,
	})
}

// editPanel replaces the panel a component interaction was attached to.
func editPanel(s *discordgo.Session, i *discordgo.InteractionCreate, components ...discordgo.MessageComponent) error {
	_, err := s.ChannelMessageEditComplex(&discordgo.MessageEdit{
		ID:         i.Message.ID,
		Channel:    i.ChannelID,
		Components: &[]discordgo.MessageComponent{container(components...)},
		Flags:      discordgo.MessageFlagsIsComponentsV2,
	})
	if err == nil {
		return nil
	}
	embed, rows := extractV1(components)
	_, err = s.ChannelMessageEditComplex(&discordgo.MessageEdit{
		ID:         i.Message.ID,
		Channel:    i.ChannelID,
		Embeds:     &[]*discordgo.MessageEmbed{embed},
		Components: &rows,
	})
	return err
}

// replyPanel answers a component interaction by editing the message it belongs
// to, which keeps one panel instead of stacking a new message per click.
func replyPanel(s *discordgo.Session, i *discordgo.InteractionCreate, components ...discordgo.MessageComponent) error {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Components: []discordgo.MessageComponent{container(components...)},
			Flags:      discordgo.MessageFlagsIsComponentsV2,
		},
	})
	if err == nil {
		return nil
	}
	// Fallback to standard V1 update:
	embed, rows := extractV1(components)
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Embeds:     []*discordgo.MessageEmbed{embed},
			Components: rows,
		},
	})
	if err == nil {
		return nil
	}
	// If update message fails (e.g. from modal submit), respond with channel message:
	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds:     []*discordgo.MessageEmbed{embed},
			Components: rows,
			Flags:      discordgo.MessageFlagsEphemeral,
		},
	})
}

// respondPanel sends a new message in reply to an interaction.
func respondPanel(s *discordgo.Session, i *discordgo.InteractionCreate, components ...discordgo.MessageComponent) error {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Components: []discordgo.MessageComponent{container(components...)},
			Flags:      discordgo.MessageFlagsIsComponentsV2 | discordgo.MessageFlagsEphemeral,
		},
	})
	if err == nil {
		return nil
	}
	// Fallback to standard V1 message:
	embed, rows := extractV1(components)
	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds:     []*discordgo.MessageEmbed{embed},
			Components: rows,
			Flags:      discordgo.MessageFlagsEphemeral,
		},
	})
}

func extractV1(components []discordgo.MessageComponent) (*discordgo.MessageEmbed, []discordgo.MessageComponent) {
	var sb strings.Builder
	var rows []discordgo.MessageComponent
	color := 0x5865F2
	if CurrentSettings.EmbedColor > 0 {
		color = CurrentSettings.EmbedColor
	}

	var walk func(c discordgo.MessageComponent)
	walk = func(c discordgo.MessageComponent) {
		if c == nil {
			return
		}
		switch val := c.(type) {
		case discordgo.Container:
			if val.AccentColor != nil && *val.AccentColor > 0 {
				color = *val.AccentColor
			}
			for _, inner := range val.Components {
				walk(inner)
			}
		case *discordgo.Container:
			if val != nil {
				if val.AccentColor != nil && *val.AccentColor > 0 {
					color = *val.AccentColor
				}
				for _, inner := range val.Components {
					walk(inner)
				}
			}
		case discordgo.TextDisplay:
			if sb.Len() > 0 {
				sb.WriteString("\n\n")
			}
			sb.WriteString(val.Content)
		case *discordgo.TextDisplay:
			if val != nil {
				if sb.Len() > 0 {
					sb.WriteString("\n\n")
				}
				sb.WriteString(val.Content)
			}
		case discordgo.Separator:
			if sb.Len() > 0 {
				sb.WriteString("\n\n───\n")
			}
		case *discordgo.Separator:
			if sb.Len() > 0 {
				sb.WriteString("\n\n───\n")
			}
		case discordgo.ActionsRow:
			rows = append(rows, val)
		case *discordgo.ActionsRow:
			if val != nil {
				rows = append(rows, *val)
			}
		}
	}

	for _, c := range components {
		walk(c)
	}

	desc := sb.String()
	if len(desc) > 4096 {
		desc = desc[:4093] + "…"
	}

	embed := &discordgo.MessageEmbed{
		Description: desc,
		Color:       color,
	}
	return embed, rows
}

// openModal asks the user for input. Modals are the only Components V2
// compatible way to collect free text.
func openModal(s *discordgo.Session, i *discordgo.InteractionCreate, customID, title string, inputs ...discordgo.TextInput) error {
	rows := make([]discordgo.MessageComponent, 0, len(inputs))
	for _, input := range inputs {
		rows = append(rows, actionRow(input))
	}
	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID:   customID,
			Title:      title,
			Components: rows,
		},
	})
}

// safeOpenModal wraps openModal to ensure an error response is always returned to Discord if the modal cannot be opened.
func safeOpenModal(s *discordgo.Session, i *discordgo.InteractionCreate, customID, title string, inputs ...discordgo.TextInput) {
	if err := openModal(s, i, customID, title, inputs...); err != nil {
		_ = respondPanel(s, i, textDisplay("⚠️ Form penceresi açılamadı: "+err.Error()))
	}
}

func textInput(customID, label, placeholder string, required bool, style discordgo.TextInputStyle) discordgo.TextInput {
	return discordgo.TextInput{
		CustomID:    customID,
		Label:       label,
		Placeholder: placeholder,
		Style:       style,
		Required:    required,
		MaxLength:   1024,
	}
}

// field renders a "**Label**\nvalue" block for panels.
func field(label, value string) string {
	if value == "" {
		value = "—"
	}
	return fmt.Sprintf("**%s**\n%s", label, value)
}

// bulletList renders list entries as bullets, or a fallback line when empty.
func bulletList(entries []string, empty string) string {
	if len(entries) == 0 {
		return empty
	}
	if len(entries) > panelListLimit {
		entries = append(entries[:panelListLimit], fmt.Sprintf("_… ve %d kayıt daha_", len(entries)-panelListLimit))
	}
	return "• " + strings.Join(entries, "\n• ")
}

func truncateText(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit-1]) + "…"
}

// inlineCode wraps a value in a code span, escaping any backtick it contains.
func inlineCode(value string) string {
	return "`" + strings.ReplaceAll(value, "`", "'") + "`"
}

// maskSecret keeps only the first and last characters of a sensitive value so a
// panel never prints a token or password in full.
func maskSecret(value string) string {
	if value == "" {
		return "—"
	}
	runes := []rune(value)
	if len(runes) <= 8 {
		return strings.Repeat("•", len(runes))
	}
	return string(runes[:4]) + strings.Repeat("•", 6) + string(runes[len(runes)-2:])
}
