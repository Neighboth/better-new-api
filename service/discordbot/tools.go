package discordbot

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/bwmarrin/discordgo"
)

type ToolDefinition struct {
	Type     string       `json:"type"`
	Function FunctionSpec `json:"function"`
}

type FunctionSpec struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// GetDiscordBotTools returns all tools available to the Discord bot assistant.
func GetDiscordBotTools() []ToolDefinition {
	return []ToolDefinition{
		{
			Type: "function",
			Function: FunctionSpec{
				Name:        "get_current_time",
				Description: "Get the current live date, time, year, day of week, and timezone",
			},
		},
		{
			Type: "function",
			Function: FunctionSpec{
				Name:        "get_all_models",
				Description: "Get a list of all enabled AI models and their pricing and token ratios",
			},
		},
		{
			Type: "function",
			Function: FunctionSpec{
				Name:        "get_free_models",
				Description: "Get a list of all completely free models (price 0)",
			},
		},
		{
			Type: "function",
			Function: FunctionSpec{
				Name:        "list_roles",
				Description: "List all roles in the current Discord server with their names, IDs and colors",
			},
		},
		{
			Type: "function",
			Function: FunctionSpec{
				Name:        "list_channels",
				Description: "List all channels and categories in the current Discord server with their names and IDs",
			},
		},
		{
			Type: "function",
			Function: FunctionSpec{
				Name:        "create_roles",
				Description: "Create one or more roles in the Discord server (bulk creation supported). Requires Manage Roles permission.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"roles": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"name":        map[string]any{"type": "string", "description": "The name of the role"},
									"color":       map[string]any{"type": "integer", "description": "Color integer for the role (e.g. 0xFF0000 for red)"},
									"hoist":       map[string]any{"type": "boolean", "description": "Whether to display the role separately in the member list"},
									"mentionable": map[string]any{"type": "boolean", "description": "Whether anyone can mention this role"},
								},
								"required": []string{"name"},
							},
						},
					},
					"required": []string{"roles"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionSpec{
				Name:        "set_role_icon",
				Description: "Set a custom icon for a role (image data or emoji). Requires Manage Roles permission.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"role_id":     map[string]any{"type": "string", "description": "The ID of the role"},
						"icon_base64": map[string]any{"type": "string", "description": "Base64 data URI of the image or icon"},
					},
					"required": []string{"role_id", "icon_base64"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionSpec{
				Name:        "set_role_hierarchy",
				Description: "Adjust the position / hierarchy of a role in the server. Requires Manage Roles permission.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"role_id":  map[string]any{"type": "string", "description": "The ID of the role"},
						"position": map[string]any{"type": "integer", "description": "New position integer for the role"},
					},
					"required": []string{"role_id", "position"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionSpec{
				Name:        "delete_role",
				Description: "Delete a role from the server. Requires Manage Roles permission.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"role_id": map[string]any{"type": "string", "description": "The ID of the role to delete"},
					},
					"required": []string{"role_id"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionSpec{
				Name:        "create_channel",
				Description: "Create a channel in the server (text, voice, or category). Requires Manage Channels permission.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name":      map[string]any{"type": "string", "description": "The name of the channel"},
						"type":      map[string]any{"type": "string", "description": "text, voice, or category"},
						"parent_id": map[string]any{"type": "string", "description": "Optional category ID to place the channel in"},
						"topic":     map[string]any{"type": "string", "description": "Channel topic"},
					},
					"required": []string{"name"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionSpec{
				Name:        "delete_channel",
				Description: "Delete a channel from the server. Requires Manage Channels permission.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"channel_id": map[string]any{"type": "string", "description": "The ID of the channel to delete"},
					},
					"required": []string{"channel_id"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionSpec{
				Name:        "timeout_member",
				Description: "Timeout / mute a member in the server for a specified duration in minutes. Requires Moderate Members permission.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"user_id":          map[string]any{"type": "string", "description": "The Discord ID of the user"},
						"duration_minutes": map[string]any{"type": "integer", "description": "Duration to timeout in minutes"},
						"reason":           map[string]any{"type": "string", "description": "Reason for the timeout"},
					},
					"required": []string{"user_id", "duration_minutes"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionSpec{
				Name:        "kick_member",
				Description: "Kick a member from the server. Requires Kick Members permission.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"user_id": map[string]any{"type": "string", "description": "The Discord ID of the user to kick"},
						"reason":  map[string]any{"type": "string", "description": "Reason for kicking the user"},
					},
					"required": []string{"user_id"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionSpec{
				Name:        "ban_member",
				Description: "Ban a member from the server. Requires Ban Members permission.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"user_id":              map[string]any{"type": "string", "description": "The Discord ID of the user to ban"},
						"delete_message_days": map[string]any{"type": "integer", "description": "Number of days of messages to delete (0-7)"},
						"reason":               map[string]any{"type": "string", "description": "Reason for the ban"},
					},
					"required": []string{"user_id"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionSpec{
				Name:        "unban_member",
				Description: "Unban a member from the server. Requires Ban Members permission.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"user_id": map[string]any{"type": "string", "description": "The Discord ID of the user to unban"},
					},
					"required": []string{"user_id"},
				},
			},
		},
	}
}

// checkUserPermission strictly verifies that the user invoking the AI has native Discord permissions.
// If the user does not have permission, it returns a 401 error.
func checkUserPermission(s *discordgo.Session, guildID, channelID, authorID string, requiredPerm int64) error {
	if guildID == "" {
		return fmt.Errorf("Error 401: Unauthorized. Guild management tools cannot be used in direct messages.")
	}
	guild, err := s.State.Guild(guildID)
	if err != nil {
		guild, err = s.Guild(guildID)
	}
	if err == nil && guild.OwnerID == authorID {
		return nil // Guild owner has all permissions
	}

	perms, err := s.UserChannelPermissions(authorID, channelID)
	if err != nil {
		return fmt.Errorf("Error 401: Unauthorized. Could not verify member permissions: %w", err)
	}
	if (perms & discordgo.PermissionAdministrator) != 0 {
		return nil // Administrator has all permissions
	}
	if (perms & requiredPerm) != requiredPerm {
		return fmt.Errorf("Error 401: Unauthorized. You do not have the required Discord permissions to perform this operation.")
	}
	return nil
}

// ExecuteDiscordTool runs a Discord tool call while strictly enforcing system-level user permissions.
func ExecuteDiscordTool(s *discordgo.Session, m *discordgo.MessageCreate, tc ToolCall) string {
	name := tc.Function.Name
	argsStr := tc.Function.Arguments

	var args map[string]any
	if argsStr != "" {
		_ = common.Unmarshal([]byte(argsStr), &args)
	}

	switch name {
	case "get_current_time":
		now := time.Now()
		return fmt.Sprintf("Live Time: %s, Year: %d, Date: %s, Day: %s, Timezone: %s",
			now.Format("15:04:05"), now.Year(), now.Format("2006-01-02"), now.Weekday().String(), now.Location().String())

	case "get_all_models":
		pricingList := model.GetPricing()
		if len(pricingList) == 0 {
			return "No models currently available."
		}
		var sb strings.Builder
		sb.WriteString("Available Models & Pricing:\n")
		for _, p := range pricingList {
			if p.QuotaType == 1 {
				sb.WriteString(fmt.Sprintf("- %s: $%.4f / call (Owner: %s)\n", p.ModelName, p.ModelPrice, p.OwnerBy))
			} else {
				sb.WriteString(fmt.Sprintf("- %s: Ratio: %.2f, Completion Ratio: %.2f (Owner: %s)\n", p.ModelName, p.ModelRatio, p.CompletionRatio, p.OwnerBy))
			}
		}
		return sb.String()

	case "get_free_models":
		pricingList := model.GetPricing()
		var freeModels []string
		for _, p := range pricingList {
			if p.ModelPrice == 0 && p.ModelRatio == 0 {
				freeModels = append(freeModels, p.ModelName)
			}
		}
		if len(freeModels) == 0 {
			return "No free models found."
		}
		return "Free Models (Price 0):\n- " + strings.Join(freeModels, "\n- ")

	case "list_roles":
		if m.GuildID == "" {
			return "Error 401: Unauthorized. This operation can only be executed in a Discord server."
		}
		roles, err := s.GuildRoles(m.GuildID)
		if err != nil {
			return fmt.Sprintf("Failed to list roles: %v", err)
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Roles (%d total):\n", len(roles)))
		for _, r := range roles {
			sb.WriteString(fmt.Sprintf("- %s (ID: %s, Pos: %d, Color: #%06X)\n", r.Name, r.ID, r.Position, r.Color))
		}
		return sb.String()

	case "list_channels":
		if m.GuildID == "" {
			return "Error 401: Unauthorized. This operation can only be executed in a Discord server."
		}
		channels, err := s.GuildChannels(m.GuildID)
		if err != nil {
			return fmt.Sprintf("Failed to list channels: %v", err)
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Channels (%d total):\n", len(channels)))
		for _, ch := range channels {
			typeStr := "text"
			if ch.Type == discordgo.ChannelTypeGuildVoice {
				typeStr = "voice"
			} else if ch.Type == discordgo.ChannelTypeGuildCategory {
				typeStr = "category"
			}
			sb.WriteString(fmt.Sprintf("- [%s] %s (ID: %s)\n", typeStr, ch.Name, ch.ID))
		}
		return sb.String()

	case "create_roles":
		if err := checkUserPermission(s, m.GuildID, m.ChannelID, m.Author.ID, discordgo.PermissionManageRoles); err != nil {
			return err.Error()
		}
		rolesRaw, ok := args["roles"].([]any)
		if !ok || len(rolesRaw) == 0 {
			return "No roles specified for creation."
		}
		var created []string
		for _, rRaw := range rolesRaw {
			rMap, ok := rRaw.(map[string]any)
			if !ok {
				continue
			}
			roleName, _ := rMap["name"].(string)
			if roleName == "" {
				continue
			}
			color := 0
			if cVal, ok := rMap["color"].(float64); ok {
				color = int(cVal)
			}
			hoist := false
			if hVal, ok := rMap["hoist"].(bool); ok {
				hoist = hVal
			}
			mentionable := false
			if mVal, ok := rMap["mentionable"].(bool); ok {
				mentionable = mVal
			}

			role, err := s.GuildRoleCreate(m.GuildID, &discordgo.RoleParams{
				Name:        roleName,
				Color:       &color,
				Hoist:       &hoist,
				Mentionable: &mentionable,
			})
			if err != nil {
				created = append(created, fmt.Sprintf("Failed to create role '%s': %v", roleName, err))
			} else {
				created = append(created, fmt.Sprintf("Created role '%s' (ID: %s)", role.Name, role.ID))
			}
		}
		return strings.Join(created, "\n")

	case "set_role_icon":
		if err := checkUserPermission(s, m.GuildID, m.ChannelID, m.Author.ID, discordgo.PermissionManageRoles); err != nil {
			return err.Error()
		}
		roleID, _ := args["role_id"].(string)
		iconB64, _ := args["icon_base64"].(string)
		if roleID == "" || iconB64 == "" {
			return "Missing role_id or icon_base64"
		}
		_, err := s.GuildRoleEdit(m.GuildID, roleID, &discordgo.RoleParams{
			Icon: &iconB64,
		})
		if err != nil {
			return fmt.Sprintf("Failed to set role icon: %v", err)
		}
		return fmt.Sprintf("Role icon updated successfully for role %s", roleID)

	case "set_role_hierarchy":
		if err := checkUserPermission(s, m.GuildID, m.ChannelID, m.Author.ID, discordgo.PermissionManageRoles); err != nil {
			return err.Error()
		}
		roleID, _ := args["role_id"].(string)
		posFloat, _ := args["position"].(float64)
		if roleID == "" {
			return "Missing role_id"
		}
		pos := int(posFloat)
		_, err := s.GuildRoleReorder(m.GuildID, []*discordgo.Role{{ID: roleID, Position: pos}})
		if err != nil {
			return fmt.Sprintf("Failed to reorder role: %v", err)
		}
		return fmt.Sprintf("Role %s position moved to %d", roleID, pos)

	case "delete_role":
		if err := checkUserPermission(s, m.GuildID, m.ChannelID, m.Author.ID, discordgo.PermissionManageRoles); err != nil {
			return err.Error()
		}
		roleID, _ := args["role_id"].(string)
		if roleID == "" {
			return "Missing role_id"
		}
		err := s.GuildRoleDelete(m.GuildID, roleID)
		if err != nil {
			return fmt.Sprintf("Failed to delete role: %v", err)
		}
		return fmt.Sprintf("Role %s deleted successfully", roleID)

	case "create_channel":
		if err := checkUserPermission(s, m.GuildID, m.ChannelID, m.Author.ID, discordgo.PermissionManageChannels); err != nil {
			return err.Error()
		}
		name, _ := args["name"].(string)
		chTypeStr, _ := args["type"].(string)
		parentID, _ := args["parent_id"].(string)
		topic, _ := args["topic"].(string)

		chType := discordgo.ChannelTypeGuildText
		if strings.ToLower(chTypeStr) == "voice" {
			chType = discordgo.ChannelTypeGuildVoice
		} else if strings.ToLower(chTypeStr) == "category" {
			chType = discordgo.ChannelTypeGuildCategory
		}

		ch, err := s.GuildChannelCreateComplex(m.GuildID, discordgo.GuildChannelCreateData{
			Name:     name,
			Type:     chType,
			Topic:    topic,
			ParentID: parentID,
		})
		if err != nil {
			return fmt.Sprintf("Failed to create channel: %v", err)
		}
		return fmt.Sprintf("Created channel '%s' (ID: %s)", ch.Name, ch.ID)

	case "delete_channel":
		if err := checkUserPermission(s, m.GuildID, m.ChannelID, m.Author.ID, discordgo.PermissionManageChannels); err != nil {
			return err.Error()
		}
		chID, _ := args["channel_id"].(string)
		if chID == "" {
			return "Missing channel_id"
		}
		_, err := s.ChannelDelete(chID)
		if err != nil {
			return fmt.Sprintf("Failed to delete channel: %v", err)
		}
		return fmt.Sprintf("Channel %s deleted successfully", chID)

	case "timeout_member":
		if err := checkUserPermission(s, m.GuildID, m.ChannelID, m.Author.ID, discordgo.PermissionModerateMembers); err != nil {
			return err.Error()
		}
		userID, _ := args["user_id"].(string)
		durationMinutes, _ := args["duration_minutes"].(float64)
		reason, _ := args["reason"].(string)
		if userID == "" || durationMinutes <= 0 {
			return "Missing user_id or valid duration_minutes"
		}
		until := time.Now().Add(time.Duration(durationMinutes) * time.Minute)
		err := s.GuildMemberTimeout(m.GuildID, userID, &until)
		if err != nil {
			return fmt.Sprintf("Failed to timeout user: %v", err)
		}
		return fmt.Sprintf("User %s timed out for %.0f minutes (Reason: %s)", userID, durationMinutes, reason)

	case "kick_member":
		if err := checkUserPermission(s, m.GuildID, m.ChannelID, m.Author.ID, discordgo.PermissionKickMembers); err != nil {
			return err.Error()
		}
		userID, _ := args["user_id"].(string)
		reason, _ := args["reason"].(string)
		if userID == "" {
			return "Missing user_id"
		}
		err := s.GuildMemberDeleteWithReason(m.GuildID, userID, reason)
		if err != nil {
			return fmt.Sprintf("Failed to kick user: %v", err)
		}
		return fmt.Sprintf("User %s kicked successfully (Reason: %s)", userID, reason)

	case "ban_member":
		if err := checkUserPermission(s, m.GuildID, m.ChannelID, m.Author.ID, discordgo.PermissionBanMembers); err != nil {
			return err.Error()
		}
		userID, _ := args["user_id"].(string)
		daysFloat, _ := args["delete_message_days"].(float64)
		reason, _ := args["reason"].(string)
		if userID == "" {
			return "Missing user_id"
		}
		days := int(daysFloat)
		err := s.GuildBanCreateWithReason(m.GuildID, userID, reason, days)
		if err != nil {
			return fmt.Sprintf("Failed to ban user: %v", err)
		}
		return fmt.Sprintf("User %s banned successfully (Reason: %s)", userID, reason)

	case "unban_member":
		if err := checkUserPermission(s, m.GuildID, m.ChannelID, m.Author.ID, discordgo.PermissionBanMembers); err != nil {
			return err.Error()
		}
		userID, _ := args["user_id"].(string)
		if userID == "" {
			return "Missing user_id"
		}
		err := s.GuildBanDelete(m.GuildID, userID)
		if err != nil {
			return fmt.Sprintf("Failed to unban user: %v", err)
		}
		return fmt.Sprintf("User %s unbanned successfully", userID)

	default:
		return fmt.Sprintf("Unknown Discord tool: %s", name)
	}
}

// ParseUserID extracts pure digits from <@!123456789> or string ID.
func ParseUserID(raw string) string {
	raw = strings.TrimPrefix(raw, "<@!")
	raw = strings.TrimPrefix(raw, "<@")
	raw = strings.TrimSuffix(raw, ">")
	if _, err := strconv.ParseUint(raw, 10, 64); err == nil {
		return raw
	}
	return ""
}
