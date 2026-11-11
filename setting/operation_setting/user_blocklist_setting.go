package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

// UserBlocklistSetting controls whether a user may exclude channels and models
// from their own requests.
//
// The blocklists themselves live in the profile settings of every user
// (`setting.blocked_channels` / `setting.blocked_models`). The relay reads them
// while it selects a channel and while it walks the fallback chain, so a
// blocked channel is never tried — not even as a fallback — for that user.
type UserBlocklistSetting struct {
	// Enabled turns the whole feature on or off for the installation.
	Enabled bool `json:"enabled"`
	// AllowChannels lets users block channel ids.
	AllowChannels bool `json:"allow_channels"`
	// AllowModels lets users block model names.
	AllowModels bool `json:"allow_models"`
}

var userBlocklistSetting = UserBlocklistSetting{
	Enabled:       true,
	AllowChannels: true,
	AllowModels:   true,
}

func init() {
	config.GlobalConfig.Register("user_blocklist_setting", &userBlocklistSetting)
}

func GetUserBlocklistSetting() *UserBlocklistSetting {
	return &userBlocklistSetting
}
