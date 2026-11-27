package controller

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
)

// StudioDiscordGateConfig represents the admin configuration and prerequisite state
// for conditional Studio models that require linking Discord + joining a Discord server.
type StudioDiscordGateConfig struct {
	BotEnabled         bool     `json:"bot_enabled"`
	OAuthEnabled       bool     `json:"oauth_enabled"`
	PrerequisitesReady bool     `json:"prerequisites_ready"`
	Active             bool     `json:"active"`
	GatedModels        []string `json:"gated_models"`
	InviteURL          string   `json:"invite_url"`
	GuildID            string   `json:"guild_id"`
	DiscordClientID    string   `json:"discord_client_id"`
	BotToken           string   `json:"-"`
}

func parseCommaSeparatedModels(raw string) []string {
	parts := strings.Split(raw, ",")
	seen := make(map[string]bool, len(parts))
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, trimmed)
	}
	return out
}

// GetStudioDiscordGateConfig resolves current Studio Discord gate settings along with
// live prerequisite validation (BOTH Discord Bot enabled+token AND Discord OAuth login enabled).
func GetStudioDiscordGateConfig() StudioDiscordGateConfig {
	common.OptionMapRWMutex.RLock()
	botEnabledFlag := strings.EqualFold(strings.TrimSpace(common.OptionMap["discord_bot.enabled"]), "true")
	botToken := strings.TrimSpace(common.OptionMap["discord_bot.token"])
	if botToken == "" {
		botToken = strings.TrimSpace(common.OptionMap["discord_bot.bot_token"])
	}
	rawModels := strings.TrimSpace(common.OptionMap["playground_setting.studio_discord_gate_models"])
	inviteURL := strings.TrimSpace(common.OptionMap["playground_setting.studio_discord_gate_invite_url"])
	guildID := strings.TrimSpace(common.OptionMap["playground_setting.studio_discord_gate_guild_id"])
	common.OptionMapRWMutex.RUnlock()

	discordOAuth := system_setting.GetDiscordSettings()
	oauthEnabled := discordOAuth.Enabled && strings.TrimSpace(discordOAuth.ClientId) != ""
	botReady := botEnabledFlag && botToken != ""
	prereqReady := botReady && oauthEnabled
	gatedModels := parseCommaSeparatedModels(rawModels)
	active := prereqReady && len(gatedModels) > 0 && guildID != ""

	return StudioDiscordGateConfig{
		BotEnabled:         botReady,
		OAuthEnabled:       oauthEnabled,
		PrerequisitesReady: prereqReady,
		Active:             active,
		GatedModels:        gatedModels,
		InviteURL:          inviteURL,
		GuildID:            guildID,
		DiscordClientID:    strings.TrimSpace(discordOAuth.ClientId),
		BotToken:           botToken,
	}
}

// IsModelInStudioDiscordGate checks if modelName matches any configured gated Studio model.
func (cfg StudioDiscordGateConfig) IsModelGated(modelName string) bool {
	if !cfg.Active {
		return false
	}
	target := strings.ToLower(strings.TrimSpace(modelName))
	if target == "" {
		return false
	}
	for _, m := range cfg.GatedModels {
		if strings.ToLower(strings.TrimSpace(m)) == target {
			return true
		}
	}
	return false
}

type discordMemberCacheEntry struct {
	inServer  bool
	checkedAt time.Time
	backoffTo time.Time
}

var (
	discordGuildCheckMu    sync.Mutex
	discordGuildCheckCache = map[string]discordMemberCacheEntry{}
	discordAPIBaseURL      = "https://discord.com/api/v10"
)

// CheckDiscordGuildMembership queries the Discord Bot REST API to check in real time
// whether discordUserID is currently a member of guildID. Uses an 850ms cache when
// bypassCache is false so 1-second frontend polling stays responsive without hitting 429s.
func CheckDiscordGuildMembership(ctx context.Context, botToken, guildID, discordUserID string, bypassCache bool) (bool, error) {
	botToken = strings.TrimSpace(botToken)
	guildID = strings.TrimSpace(guildID)
	discordUserID = strings.TrimSpace(discordUserID)
	if botToken == "" || guildID == "" || discordUserID == "" {
		return false, nil
	}

	cacheKey := guildID + ":" + discordUserID
	now := time.Now()
	if !bypassCache {
		discordGuildCheckMu.Lock()
		if ent, ok := discordGuildCheckCache[cacheKey]; ok {
			if now.Before(ent.backoffTo) || now.Sub(ent.checkedAt) < 850*time.Millisecond {
				discordGuildCheckMu.Unlock()
				return ent.inServer, nil
			}
		}
		discordGuildCheckMu.Unlock()
	}

	endpoint := fmt.Sprintf("%s/guilds/%s/members/%s",
		strings.TrimRight(discordAPIBaseURL, "/"),
		url.PathEscape(guildID),
		url.PathEscape(discordUserID),
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bot "+botToken)
	req.Header.Set("User-Agent", "BetterNewAPI-StudioGate/1.0")

	client := &http.Client{Timeout: 4 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		discordGuildCheckMu.Lock()
		discordGuildCheckCache[cacheKey] = discordMemberCacheEntry{inServer: true, checkedAt: time.Now()}
		discordGuildCheckMu.Unlock()
		return true, nil
	case http.StatusNotFound:
		discordGuildCheckMu.Lock()
		discordGuildCheckCache[cacheKey] = discordMemberCacheEntry{inServer: false, checkedAt: time.Now()}
		discordGuildCheckMu.Unlock()
		return false, nil
	case http.StatusTooManyRequests:
		// Honor brief backoff while returning last known status cleanly
		discordGuildCheckMu.Lock()
		ent := discordGuildCheckCache[cacheKey]
		ent.backoffTo = time.Now().Add(2 * time.Second)
		discordGuildCheckCache[cacheKey] = ent
		discordGuildCheckMu.Unlock()
		return ent.inServer, nil
	default:
		return false, fmt.Errorf("discord api returned status %d", resp.StatusCode)
	}
}

// GetStudioDiscordGateStatus returns the live Discord gate configuration and the
// authenticated user's real-time Discord link + guild membership status.
// Designed to be polled every 1 second by Studio when a gated model is selected.
func GetStudioDiscordGateStatus(c *gin.Context) {
	cfg := GetStudioDiscordGateConfig()
	userId := c.GetInt("id")

	discordLinked := false
	inServer := false
	checkError := ""

	if cfg.Active && userId > 0 {
		// Read fresh user row from DB (not stale cache) so linking Discord in another tab is detected within 1s
		user, err := model.GetUserById(userId, false)
		if err == nil && user != nil && strings.TrimSpace(user.DiscordId) != "" {
			discordLinked = true
			member, mErr := CheckDiscordGuildMembership(c.Request.Context(), cfg.BotToken, cfg.GuildID, user.DiscordId, false)
			if mErr != nil {
				checkError = mErr.Error()
			} else {
				inServer = member
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"active":              cfg.Active,
			"prerequisites_ready": cfg.PrerequisitesReady,
			"bot_enabled":         cfg.BotEnabled,
			"oauth_enabled":       cfg.OAuthEnabled,
			"gated_models":        cfg.GatedModels,
			"invite_url":          cfg.InviteURL,
			"guild_id":            cfg.GuildID,
			"discord_client_id":   cfg.DiscordClientID,
			"discord_linked":      discordLinked,
			"in_server":           inServer,
			"check_error":         checkError,
		},
	})
}

// enforceStudioDiscordGate blocks Studio image/video generation server-side when
// the requested model is gated and the user has not linked Discord or is not in the required server.
func enforceStudioDiscordGate(c *gin.Context, modelName string) bool {
	cfg := GetStudioDiscordGateConfig()
	if !cfg.IsModelGated(modelName) {
		return true
	}

	userId := c.GetInt("id")
	discordLinked := false
	inServer := false
	if userId > 0 {
		if user, err := model.GetUserById(userId, false); err == nil && user != nil && strings.TrimSpace(user.DiscordId) != "" {
			discordLinked = true
			if member, mErr := CheckDiscordGuildMembership(c.Request.Context(), cfg.BotToken, cfg.GuildID, user.DiscordId, true); mErr == nil {
				inServer = member
			}
		}
	}

	if discordLinked && inServer {
		return true
	}

	msg := "You must link your Discord account and join our Discord server to use this model."
	if discordLinked && !inServer {
		msg = "You must join our Discord server to use this model."
	}
	c.JSON(http.StatusForbidden, gin.H{
		"error": gin.H{
			"message": msg,
			"type":    "studio_discord_gate_required",
			"code":    "STUDIO_DISCORD_GATE_REQUIRED",
		},
		"discord_gate": gin.H{
			"active":            true,
			"gated_models":      cfg.GatedModels,
			"invite_url":        cfg.InviteURL,
			"guild_id":          cfg.GuildID,
			"discord_client_id": cfg.DiscordClientID,
			"discord_linked":    discordLinked,
			"in_server":         inServer,
		},
	})
	return false
}
