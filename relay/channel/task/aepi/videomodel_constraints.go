// videomodel_constraints.go is generated from the live catalog of the
// Aepi Media API (GET /v1/models at https://videoapi.aerobox.eu.cc, fetched
// 2026-11-17). Keep it in sync with that endpoint when the catalog changes.
// The constraints are used to answer a misconfigured request with values the
// model supports instead of an opaque 422 from the provider.
package aepi

type videoModelConstraint struct {
	Aspects   []string
	RenderRes []string
	MinDur    int
	MaxDur    int
}

var videoModelConstraints = map[string]videoModelConstraint{
	"adobe-firefly": {Aspects: []string{"1:1", "16:9", "9:16", "3:4", "4:3"}, RenderRes: []string{"480p", "720p"}, MinDur: 2, MaxDur: 15},
	"flux-3": {Aspects: []string{"auto", "21:9", "2:1", "16:9", "4:3", "1:1", "3:4", "9:16"}, RenderRes: []string{"720p", "1080p"}, MinDur: 5, MaxDur: 20},
	"flux-3-max": {Aspects: []string{"9:16", "3:4", "1:1", "4:3", "16:9", "2:1", "21:9", "auto"}, RenderRes: []string{"720p", "1080p"}, MinDur: 5, MaxDur: 20},
	"gemini-omni-flash": {Aspects: []string{"16:9", "9:16"}, RenderRes: []string{"720p", "1080p", "4k"}, MinDur: 3, MaxDur: 10},
	"gemini-omni-flash-1.1": {Aspects: []string{"16:9", "9:16"}, RenderRes: []string{"360p", "720p", "1080p", "4k"}, MinDur: 3, MaxDur: 10},
	"gemini-omni-flash-1.2": {Aspects: []string{"9:16", "16:9"}, RenderRes: []string{"360p", "720p", "1080p", "4k"}, MinDur: 3, MaxDur: 15},
	"grok": {Aspects: []string{"1:1", "16:9", "9:16"}, RenderRes: []string{"480p", "720p"}, MinDur: 3, MaxDur: 15},
	"grok-video-1.5": {Aspects: []string{"16:9", "1:1", "9:16", "3:4", "4:3", "3:2", "2:3"}, RenderRes: []string{"480p", "720p"}, MinDur: 5, MaxDur: 15},
	"grok-video-1.5-lite": {Aspects: []string{"16:9", "1:1", "9:16", "3:4", "4:3", "3:2", "2:3"}, RenderRes: []string{"480p", "720p", "1080p"}, MinDur: 5, MaxDur: 15},
	"grok-video-edit": {Aspects: []string{}, RenderRes: []string{"480p", "720p"}, MinDur: 0, MaxDur: 0},
	"heygen-v3": {Aspects: []string{"16:9", "9:16"}, RenderRes: []string{"360p", "480p", "540p", "720p", "1080p"}, MinDur: 0, MaxDur: 0},
	"kling-2.6": {Aspects: []string{"1:1", "16:9", "9:16"}, RenderRes: []string{"720p", "1080p"}, MinDur: 5, MaxDur: 10},
	"kling-3.0": {Aspects: []string{"1:1", "16:9", "9:16"}, RenderRes: []string{"720p", "1080p", "4k"}, MinDur: 3, MaxDur: 15},
	"kling-4.0": {Aspects: []string{"16:9", "9:16", "1:1", "21:9"}, RenderRes: []string{"720p", "1080p"}, MinDur: 3, MaxDur: 30},
	"kling-4.0-flash": {Aspects: []string{"16:9", "9:16", "1:1", "21:9"}, RenderRes: []string{"720p"}, MinDur: 3, MaxDur: 20},
	"kling-o3-edit": {Aspects: []string{}, RenderRes: []string{"720p", "1080p", "4k"}, MinDur: 0, MaxDur: 0},
	"kling-o3-omni": {Aspects: []string{"1:1", "16:9", "9:16"}, RenderRes: []string{"720p", "1080p", "4k"}, MinDur: 3, MaxDur: 15},
	"kling-v3-turbo": {Aspects: []string{"16:9", "1:1", "9:16"}, RenderRes: []string{"720p", "1080p"}, MinDur: 3, MaxDur: 15},
	"ltx-2.5-fast": {Aspects: []string{"16:9", "9:16"}, RenderRes: []string{"720p", "1080p"}, MinDur: 6, MaxDur: 20},
	"ltx-2.5-pro": {Aspects: []string{"16:9", "9:16"}, RenderRes: []string{"720p", "1080p"}, MinDur: 6, MaxDur: 10},
	"luma-dream-machine": {Aspects: []string{"1:1", "3:4", "9:16", "4:3", "16:9", "21:9"}, RenderRes: []string{"720p", "1080p"}, MinDur: 0, MaxDur: 0},
	"luma-ray-v3.2": {Aspects: []string{"1:1", "3:4", "9:16", "4:3", "16:9", "21:9"}, RenderRes: []string{"720p", "1080p"}, MinDur: 0, MaxDur: 0},
	"minimax-h3": {Aspects: []string{"21:9", "16:9", "4:3", "1:1", "3:4", "9:16"}, RenderRes: []string{"768p", "2k"}, MinDur: 5, MaxDur: 15},
	"minimax-h3-max": {Aspects: []string{"21:9", "16:9", "4:3", "1:1", "3:4", "9:16"}, RenderRes: []string{"480p", "768p", "1080p"}, MinDur: 5, MaxDur: 15},
	"minimax-h3-max-turbo": {Aspects: []string{"9:16", "3:4", "1:1", "4:3", "16:9", "21:9"}, RenderRes: []string{"480p", "768p"}, MinDur: 5, MaxDur: 15},
	"minimax-hailuo-2.3": {Aspects: []string{}, RenderRes: []string{"720p", "1080p"}, MinDur: 0, MaxDur: 0},
	"pika-2.2": {Aspects: []string{}, RenderRes: []string{"720p", "1080p"}, MinDur: 5, MaxDur: 10},
	"pixverse-v6": {Aspects: []string{"1:1", "3:4", "9:16", "4:3", "16:9", "21:9"}, RenderRes: []string{"720p", "1080p"}, MinDur: 1, MaxDur: 15},
	"runway-aleph-2.0": {Aspects: []string{}, RenderRes: []string{"720p", "1080p", "4k"}, MinDur: 0, MaxDur: 0},
	"runway-gen-4.5": {Aspects: []string{"9:16", "16:9"}, RenderRes: []string{"720p", "1080p", "4k"}, MinDur: 2, MaxDur: 10},
	"seedance-2.0": {Aspects: []string{"auto", "16:9", "9:16", "21:9", "4:3", "3:4", "1:1"}, RenderRes: []string{"480p", "720p", "1080p", "4k"}, MinDur: 4, MaxDur: 15},
	"seedance-2.0-fast": {Aspects: []string{"auto", "16:9", "9:16", "21:9", "4:3", "3:4", "1:1"}, RenderRes: []string{"480p", "720p"}, MinDur: 4, MaxDur: 15},
	"seedance-2.0-mini": {Aspects: []string{"1:1", "3:4", "4:3", "21:9", "9:16", "16:9", "auto"}, RenderRes: []string{"480p", "720p"}, MinDur: 4, MaxDur: 15},
	"seedance-2.5": {Aspects: []string{"auto", "1:1", "3:4", "4:3", "21:9", "9:16", "16:9"}, RenderRes: []string{"480p", "720p", "1080p", "4k"}, MinDur: 4, MaxDur: 30},
	"seedance-2.5-flash": {Aspects: []string{"16:9", "9:16", "21:9", "4:3", "3:4", "1:1", "auto"}, RenderRes: []string{"480p", "720p"}, MinDur: 4, MaxDur: 30},
	"wan-2.6": {Aspects: []string{"1:1", "9:16", "16:9", "3:4", "4:3"}, RenderRes: []string{"720p", "1080p"}, MinDur: 5, MaxDur: 15},
	"wan-2.7": {Aspects: []string{"1:1", "9:16", "16:9", "3:4", "4:3"}, RenderRes: []string{"720p", "1080p"}, MinDur: 2, MaxDur: 15},
	"wan-2.7-edit": {Aspects: []string{"1:1", "9:16", "16:9", "3:4", "4:3"}, RenderRes: []string{"720p", "1080p"}, MinDur: 2, MaxDur: 10},
	"wan-3.0": {Aspects: []string{"16:9", "1:1", "9:16", "3:4", "4:3"}, RenderRes: []string{"480p", "720p"}, MinDur: 2, MaxDur: 30},
	"wan-3.0-prime": {Aspects: []string{"4:3", "3:4", "9:16", "1:1", "16:9"}, RenderRes: []string{"480p", "720p"}, MinDur: 2, MaxDur: 30},
}

func constraintContains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
