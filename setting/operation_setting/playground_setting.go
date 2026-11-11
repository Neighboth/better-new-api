package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

// PlaygroundSetting is the administrator configuration of the playground and
// the studio: which media models the tools use, which options the UI offers
// and which web-search surfaces are enabled.
type PlaygroundSetting struct {
	// Web search toggles. The price of one search stays in the online search
	// setting, these switches only decide where the tool is offered.
	WebSearchPlaygroundEnabled bool `json:"web_search_playground_enabled"`
	WebSearchDiscordEnabled    bool `json:"web_search_discord_enabled"`
	WebSearchLiveEnabled       bool `json:"web_search_live_enabled"`

	// Image model of the playground tools and of the studio.
	ImageEnabled        bool   `json:"image_enabled"`
	ImageModel          string `json:"image_model"`
	ImageSizes          string `json:"image_sizes"`
	ImageDefaultSize    string `json:"image_default_size"`
	ImageQualities      string `json:"image_qualities"`
	ImageDefaultQuality string `json:"image_default_quality"`
	ImageDefaultCount   int    `json:"image_default_count"`

	// Video model of the playground tool and of the studio.
	VideoEnabled         bool   `json:"video_enabled"`
	VideoModel           string `json:"video_model"`
	VideoDurations       string `json:"video_durations"`
	VideoDefaultDuration int    `json:"video_default_duration"`
	VideoMinDuration     int    `json:"video_min_duration"`
	VideoMaxDuration     int    `json:"video_max_duration"`
	VideoRatios          string `json:"video_ratios"`
	VideoDefaultRatio    string `json:"video_default_ratio"`
	VideoQualities       string `json:"video_qualities"`
	VideoDefaultQuality  string `json:"video_default_quality"`
	VideoReferenceImage  bool   `json:"video_reference_image"`

	// Studio page availability.
	StudioEnabled bool `json:"studio_enabled"`
}

var playgroundSetting = PlaygroundSetting{
	WebSearchPlaygroundEnabled: false,
	WebSearchDiscordEnabled:    false,
	WebSearchLiveEnabled:       false,

	ImageEnabled:        true,
	ImageModel:         "",
	ImageSizes:         "1024x1024,1024x1792,1792x1024",
	ImageDefaultSize:   "1024x1024",
	ImageQualities:     "standard,hd",
	ImageDefaultQuality: "standard",
	ImageDefaultCount:  1,

	VideoEnabled:         false,
	VideoModel:           "",
	VideoDurations:       "5,10",
	VideoDefaultDuration: 5,
	VideoMinDuration:     5,
	VideoMaxDuration:     10,
	VideoRatios:          "16:9,9:16,1:1",
	VideoDefaultRatio:    "16:9",
	VideoQualities:       "standard,high",
	VideoDefaultQuality:  "standard",
	VideoReferenceImage:  true,

	StudioEnabled: true,
}

func init() {
	config.GlobalConfig.Register("playground_setting", &playgroundSetting)
}

func GetPlaygroundSetting() *PlaygroundSetting {
	return &playgroundSetting
}
