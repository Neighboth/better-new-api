package discordbot

import (
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service/authz"
	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Permission aliases keep the panel expectations readable.
var (
	authzUserRead          = authz.UserRead
	authzUserWrite         = authz.UserWrite
	authzUserDelete        = authz.UserDelete
	authzBlogRead          = authz.BlogRead
	authzBlogWrite         = authz.BlogWrite
	authzTicketRead        = authz.TicketRead
	authzTicketWrite       = authz.TicketWrite
	authzTicketDelete      = authz.TicketDelete
	authzRedemptionRead    = authz.RedemptionRead
	authzSystemSettingRead = authz.SystemSettingRead
)

func capabilitiesFor(permissions ...authz.Permission) authz.PermissionsMap {
	result := authz.PermissionsMap{}
	for _, permission := range permissions {
		if result[permission.Resource] == nil {
			result[permission.Resource] = map[string]bool{}
		}
		result[permission.Resource][permission.Action] = true
	}
	return result
}

func timeNowPlusMinutes(minutes int) time.Time {
	return time.Now().Add(time.Duration(minutes) * time.Minute)
}

func withOptionMap(t *testing.T, values map[string]string) {
	t.Helper()
	common.OptionMapRWMutex.Lock()
	previous := common.OptionMap
	common.OptionMap = make(map[string]string, len(values))
	for key, value := range values {
		common.OptionMap[key] = value
	}
	common.OptionMapRWMutex.Unlock()

	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previous
		common.OptionMapRWMutex.Unlock()
		setCurrentSettings(defaultSettings)
	})
}

// TestSyncSettingsFromOptionsReadsDiscordBotKeysOnly pins the fix for the toggle
// that kept flipping back on: the bot switch must never inherit the Discord
// OAuth login flag (discord.enabled), which is a different feature.
func TestSyncSettingsFromOptionsReadsDiscordBotKeysOnly(t *testing.T) {
	withOptionMap(t, map[string]string{
		"discord.enabled":       "true",
		"discord.bot_token":     "oauth-namespace-token",
		"discord_bot.bot_name":  "Helper",
		"discord_bot.embed_col": "#ff0000",
	})

	settings := SyncSettingsFromOptions()
	assert.False(t, settings.Enabled, "the bot must not be enabled by the Discord OAuth flag")
	assert.Empty(t, settings.BotToken, "the bot token must not be read from the OAuth namespace")
	assert.Equal(t, "Helper", settings.BotName)
}

func TestSyncSettingsFromOptionsAppliesBooleanAndColorValues(t *testing.T) {
	withOptionMap(t, map[string]string{
		"discord_bot.enabled":               "true",
		"discord_bot.token":                 "bot-token",
		"discord_bot.prefix":                "?",
		"discord_bot.rpc_enabled":           "false",
		"discord_bot.only_linked_accounts":  "true",
		"discord_bot.embed_color":           "#1a2b3c",
		"discord_bot.image_model":           "dall-e-3",
		"discord_bot.auto_reply_model":      "gpt-4o-mini",
		"discord_bot.auto_reply_channel_id": "1234",
	})

	settings := SyncSettingsFromOptions()
	assert.True(t, settings.Enabled)
	assert.Equal(t, "bot-token", settings.BotToken)
	assert.Equal(t, "?", settings.Prefix)
	assert.False(t, settings.RPCEnabled)
	assert.True(t, settings.OnlyLinkedAccounts)
	assert.Equal(t, 0x1a2b3c, settings.EmbedColor)
	assert.Equal(t, "dall-e-3", settings.ImageModel)
	assert.Equal(t, "1234", settings.AutoReplyChannelID)
}

func TestSyncSettingsFromOptionsFallsBackToDefaults(t *testing.T) {
	withOptionMap(t, map[string]string{})

	settings := SyncSettingsFromOptions()
	assert.False(t, settings.Enabled)
	assert.Equal(t, defaultSettings.Prefix, settings.Prefix)
	assert.Equal(t, defaultSettings.EmbedColor, settings.EmbedColor)
	assert.True(t, settings.RPCEnabled)
}

func TestCurrentSettingsIsAPublishedSnapshot(t *testing.T) {
	custom := defaultSettings
	custom.BotName = "Snapshot"
	setCurrentSettings(custom)

	snapshot := currentSettings()
	snapshot.BotName = "Mutated"
	assert.Equal(t, "Snapshot", currentSettings().BotName, "the snapshot must be a copy")

	setCurrentSettings(defaultSettings)
}

func TestOwnerSessionSectionsFollowPermissions(t *testing.T) {
	full := &ownerSession{permissions: capabilitiesFor(
		authzUserRead, authzUserWrite, authzUserDelete,
		authzBlogRead, authzBlogWrite,
		authzTicketRead, authzTicketWrite, authzTicketDelete,
		authzRedemptionRead, authzSystemSettingRead,
	)}
	ids := make([]string, 0, 6)
	for _, section := range full.availableSections() {
		ids = append(ids, section.id)
	}
	assert.Equal(t, []string{
		ownerSectionOverview, ownerSectionUsers, ownerSectionBlog,
		ownerSectionTickets, ownerSectionCoupons, ownerSectionSettings,
	}, ids)

	readOnly := &ownerSession{permissions: capabilitiesFor(authzUserRead)}
	ids = ids[:0]
	for _, section := range readOnly.availableSections() {
		ids = append(ids, section.id)
	}
	assert.Equal(t, []string{ownerSectionOverview, ownerSectionUsers}, ids)
	assert.False(t, readOnly.can(authzUserWrite))
	assert.False(t, readOnly.allowsSection(ownerSectionBlog))
	assert.True(t, readOnly.allowsSection(ownerSectionUsers))
}

func TestOwnerPanelLayoutIsComponentsV2Shaped(t *testing.T) {
	session := &ownerSession{
		username:    "admin",
		userID:      7,
		role:        common.RoleRootUser,
		permissions: capabilitiesFor(authzUserRead, authzUserWrite),
		section:     ownerSectionUsers,
		page:        1,
		expiresAt:   timeNowPlusMinutes(20),
	}

	components := ownerPanelLayout(session, "body", []discordgo.MessageComponent{
		primaryButton(ownerPanelPrefix+"user:create", "Oluştur"),
	}, "notice")

	// notice, separator, header, separator, body, separator, select, actions, footer
	require.Len(t, components, 9)
	assert.IsType(t, discordgo.TextDisplay{}, components[0])
	assert.IsType(t, discordgo.Separator{}, components[1])

	selectRow, ok := components[6].(discordgo.ActionsRow)
	require.True(t, ok)
	require.Len(t, selectRow.Components, 1)
	menu, ok := selectRow.Components[0].(discordgo.SelectMenu)
	require.True(t, ok, "the section selector must be a select menu")
	assert.Equal(t, discordgo.StringSelectMenu, menu.MenuType, "type must be set or Discord rejects the component")
	assert.Equal(t, ownerPanelPrefix+"nav", menu.CustomID)

	footer, ok := components[8].(discordgo.ActionsRow)
	require.True(t, ok)
	assert.Len(t, footer.Components, 2, "refresh and exit live in the footer")
}

func TestOwnerSectionSelectReflectsPermissions(t *testing.T) {
	session := &ownerSession{
		permissions: capabilitiesFor(authzUserRead, authzBlogRead),
		section:     ownerSectionOverview,
	}
	menu := ownerSectionSelect(session)

	require.Len(t, menu.Options, 3)
	assert.Equal(t, ownerSectionOverview, menu.Options[0].Value)
	assert.Equal(t, ownerSectionUsers, menu.Options[1].Value)
	assert.Equal(t, ownerSectionBlog, menu.Options[2].Value)
	assert.True(t, menu.Options[0].Default, "the active section is pre-selected")
}

func TestContainerUsesConfiguredAccentColor(t *testing.T) {
	setCurrentSettings(defaultSettings)
	box := container(textDisplay("hello"))
	require.NotNil(t, box.AccentColor)
	assert.Equal(t, defaultSettings.EmbedColor, *box.AccentColor)

	custom := defaultSettings
	custom.EmbedColor = 0
	setCurrentSettings(custom)
	box = container(textDisplay("hello"))
	require.NotNil(t, box.AccentColor)
	assert.Equal(t, 0x5865F2, *box.AccentColor, "a missing colour falls back to blurple")
	setCurrentSettings(defaultSettings)
}

func TestModalValueReadsTextInputs(t *testing.T) {
	data := discordgo.ModalSubmitInteractionData{
		CustomID: ownerModalPrefix + "user_delete",
		Components: []discordgo.MessageComponent{
			&discordgo.ActionsRow{Components: []discordgo.MessageComponent{
				&discordgo.TextInput{CustomID: "target", Value: " 42 "},
				&discordgo.TextInput{CustomID: "confirm", Value: "SİL"},
			}},
			discordgo.ActionsRow{Components: []discordgo.MessageComponent{
				discordgo.TextInput{CustomID: "extra", Value: "value"},
			}},
		},
	}

	assert.Equal(t, "42", modalValue(data, "target"))
	assert.Equal(t, "SİL", modalValue(data, "confirm"))
	assert.Equal(t, "value", modalValue(data, "extra"), "value-typed rows must also be read")
	assert.Empty(t, modalValue(data, "missing"))
}

func TestParseOwnerBool(t *testing.T) {
	for _, truthy := range []string{"evet", "E", "yes", "TRUE", "1", "açık", "on"} {
		assert.True(t, parseOwnerBool(truthy), truthy)
	}
	for _, falsy := range []string{"hayır", "no", "false", "0", "", "belki"} {
		assert.False(t, parseOwnerBool(falsy), falsy)
	}
}

func TestDescribeHelpers(t *testing.T) {
	assert.Equal(t, "Kurucu", describeRole(common.RoleRootUser))
	assert.Equal(t, "Yönetici", describeRole(common.RoleAdminUser))
	assert.Equal(t, "Kullanıcı", describeRole(common.RoleCommonUser))
	assert.Equal(t, "Misafir", describeRole(common.RoleGuestUser))

	assert.Equal(t, "aktif", describeStatus(common.UserStatusEnabled))
	assert.NotEqual(t, "aktif", describeStatus(common.UserStatusDisabled))

	assert.Equal(t, "açık", describeBool(true))
	assert.Equal(t, "kapalı", describeBool(false))

	assert.Equal(t, "kullanılabilir", describeRedemptionStatus(1))
	assert.Equal(t, "kullanılmış", describeRedemptionStatus(3))
}

func TestMaskSecretNeverLeaksFullValue(t *testing.T) {
	assert.Equal(t, "—", maskSecret(""))
	assert.Equal(t, "••••", maskSecret("abcd"))
	masked := maskSecret("super-secret-bot-token")
	assert.NotContains(t, masked, "secret")
	assert.Contains(t, masked, "•")
}

func TestTruncateAndBulletListRespectDiscordLimits(t *testing.T) {
	long := strings.Repeat("a", textDisplayLimit+100)
	truncated := truncateText(long, textDisplayLimit)
	assert.Equal(t, textDisplayLimit, len([]rune(truncated)))

	entries := make([]string, panelListLimit+5)
	for i := range entries {
		entries[i] = "entry"
	}
	body := bulletList(entries, "none")
	assert.Contains(t, body, "kayıt daha")

	assert.Equal(t, "none", bulletList(nil, "none"))
}

func TestInlineCodeEscapesBackticks(t *testing.T) {
	assert.Equal(t, "`a'b`", inlineCode("a`b"))
}

func TestFormatDuration(t *testing.T) {
	assert.Equal(t, "0 dk", formatDuration(-1))
	assert.Equal(t, "1 dk'dan az", formatDuration(0))
	assert.Equal(t, "5 dk", formatDuration(5*60*1e9))
	assert.Equal(t, "1 sa 1 dk", formatDuration(61*60*1e9))
}
