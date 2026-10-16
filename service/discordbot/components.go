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

func textDisplay(content string) discordgo.TextDisplay {
	return discordgo.TextDisplay{Content: truncateText(content, textDisplayLimit)}
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

// sendPanel posts a Components V2 message built from a single container.
func sendPanel(s *discordgo.Session, channelID string, components ...discordgo.MessageComponent) (*discordgo.Message, error) {
	return s.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Components: []discordgo.MessageComponent{container(components...)},
		Flags:      discordgo.MessageFlagsIsComponentsV2,
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
	return err
}

// replyPanel answers a component interaction by editing the message it belongs
// to, which keeps one panel instead of stacking a new message per click.
func replyPanel(s *discordgo.Session, i *discordgo.InteractionCreate, components ...discordgo.MessageComponent) error {
	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Components: []discordgo.MessageComponent{container(components...)},
			Flags:      discordgo.MessageFlagsIsComponentsV2,
		},
	})
}

// respondPanel sends a new Components V2 message in reply to an interaction.
func respondPanel(s *discordgo.Session, i *discordgo.InteractionCreate, components ...discordgo.MessageComponent) error {
	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Components: []discordgo.MessageComponent{container(components...)},
			Flags:      discordgo.MessageFlagsIsComponentsV2 | discordgo.MessageFlagsEphemeral,
		},
	})
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
