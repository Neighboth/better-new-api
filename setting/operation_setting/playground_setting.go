package operation_setting

import (
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
)

// MediaModel is one image or video model the administrator offers in the
// playground tools and in the studio.
//
// Every entry carries its own options: the quality levels of one model are not
// the quality levels of another one, and the durations, sizes and aspect
// ratios differ per model as well. The studio shows the option set of the
// model the user picked.
type MediaModel struct {
	Model          string   `json:"model"`
	Sizes          []string `json:"sizes,omitempty"`
	Qualities      []string `json:"qualities,omitempty"`
	Ratios         []string `json:"ratios,omitempty"`
	Durations      []int    `json:"durations,omitempty"`
	ReferenceImage bool     `json:"reference_image,omitempty"`
	DefaultSize    string   `json:"default_size,omitempty"`
	DefaultQuality string   `json:"default_quality,omitempty"`
	DefaultRatio   string   `json:"default_ratio,omitempty"`
}

// PlaygroundSetting is the administrator configuration of the playground and
// the studio: which media models the tools use, which options the UI offers
// and which web-search surfaces are enabled.
type PlaygroundSetting struct {
	// Web search toggles. The price of one search stays in the online search
	// setting, these switches only decide where the tool is offered.
	WebSearchPlaygroundEnabled bool `json:"web_search_playground_enabled"`
	WebSearchDiscordEnabled    bool `json:"web_search_discord_enabled"`
	WebSearchLiveEnabled       bool `json:"web_search_live_enabled"`
	WebSearchBlogEnabled       bool `json:"web_search_blog_enabled"`

	// Image models of the playground tools and of the studio. Each entry has
	// its own sizes, qualities and defaults.
	ImageEnabled bool         `json:"image_enabled"`
	ImageModels  []MediaModel `json:"image_models"`

	// Video models of the playground tool and of the studio. Each entry has
	// its own durations, aspect ratios, qualities and reference image switch.
	VideoEnabled bool         `json:"video_enabled"`
	VideoModels  []MediaModel `json:"video_models"`

	// Studio page availability.
	StudioEnabled bool `json:"studio_enabled"`

	// ── Legacy single-model fields ──────────────────────────────────────────
	// They are kept so an installation that configured the studio before the
	// per-model options existed keeps working: when the model lists are empty
	// the runtime configuration is derived from these values.
	ImageModel          string `json:"image_model"`
	ImageSizes          string `json:"image_sizes"`
	ImageDefaultSize    string `json:"image_default_size"`
	ImageQualities      string `json:"image_qualities"`
	ImageDefaultQuality string `json:"image_default_quality"`
	ImageDefaultCount   int    `json:"image_default_count"`

	VideoModel           string `json:"video_model"`
	VideoDurations       string `json:"video_durations"`
	VideoDefaultDuration int    `json:"video_default_duration"`
	VideoRatios          string `json:"video_ratios"`
	VideoDefaultRatio    string `json:"video_default_ratio"`
	VideoQualities       string `json:"video_qualities"`
	VideoDefaultQuality  string `json:"video_default_quality"`
	VideoReferenceImage  bool   `json:"video_reference_image"`
}

var playgroundSetting = PlaygroundSetting{
	WebSearchPlaygroundEnabled: false,
	WebSearchDiscordEnabled:    false,
	WebSearchLiveEnabled:       false,
	WebSearchBlogEnabled:       false,

	ImageEnabled:        true,
	ImageModels:         []MediaModel{},
	ImageModel:          "",
	ImageSizes:          "1024x1024,1024x1792,1792x1024",
	ImageDefaultSize:    "1024x1024",
	ImageQualities:      "standard,hd",
	ImageDefaultQuality: "standard",
	ImageDefaultCount:   1,

	VideoEnabled:         false,
	VideoModels:          []MediaModel{},
	VideoModel:           "",
	VideoDurations:       "5,10",
	VideoDefaultDuration: 5,
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

// splitList turns a comma separated setting value into a clean list.
func splitList(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == ';'
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// parseDurations reads the durations of a legacy setting value.
func parseDurations(raw string) []int {
	out := []int{}
	for _, part := range splitList(raw) {
		if value, err := strconv.Atoi(part); err == nil && value > 0 {
			out = append(out, value)
		}
	}
	sort.Ints(out)
	return out
}

// ImageModelOptions returns the image models the studio and the playground
// offer, together with the options of every model.
func (s *PlaygroundSetting) ImageModelOptions() []MediaModel {
	if len(s.ImageModels) > 0 {
		return s.ImageModels
	}
	if strings.TrimSpace(s.ImageModel) == "" {
		return []MediaModel{}
	}
	return []MediaModel{{
		Model:          strings.TrimSpace(s.ImageModel),
		Sizes:          splitList(s.ImageSizes),
		Qualities:      splitList(s.ImageQualities),
		DefaultSize:    s.ImageDefaultSize,
		DefaultQuality: s.ImageDefaultQuality,
	}}
}

// VideoModelOptions returns the video models with their own options.
func (s *PlaygroundSetting) VideoModelOptions() []MediaModel {
	if len(s.VideoModels) > 0 {
		return s.VideoModels
	}
	if strings.TrimSpace(s.VideoModel) == "" {
		return []MediaModel{}
	}
	return []MediaModel{{
		Model:          strings.TrimSpace(s.VideoModel),
		Durations:      parseDurations(s.VideoDurations),
		Ratios:         splitList(s.VideoRatios),
		Qualities:      splitList(s.VideoQualities),
		ReferenceImage: s.VideoReferenceImage,
		DefaultRatio:   s.VideoDefaultRatio,
		DefaultQuality: s.VideoDefaultQuality,
	}}
}

// VideoDurationRange returns the shortest and the longest duration of all
// configured video models. The studio offers exactly the durations of the
// selected model, so the range is only used by the tool schema and by older
// clients.
func (s *PlaygroundSetting) VideoDurationRange() (int, int) {
	minDuration, maxDuration := 0, 0
	for _, model := range s.VideoModelOptions() {
		for _, duration := range model.Durations {
			if duration <= 0 {
				continue
			}
			if minDuration == 0 || duration < minDuration {
				minDuration = duration
			}
			if duration > maxDuration {
				maxDuration = duration
			}
		}
	}
	if minDuration == 0 {
		minDuration = s.VideoDefaultDuration
	}
	if maxDuration == 0 {
		maxDuration = minDuration
	}
	return minDuration, maxDuration
}
