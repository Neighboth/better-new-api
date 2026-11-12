// videomodel_constraints.go is generated from the live catalog of the
// Aepi Media API (GET /v1/models at https://videoapi.aerobox.eu.cc, fetched
// 2026-11-12). Keep it in sync with that endpoint when the catalog changes.
// The constraints are used to answer a misconfigured request with a clear
// 400 instead of an opaque 422 from the provider.
package aepi

type videoModelConstraint struct {
	Aspects   []string
	RenderRes []string
}

var videoModelConstraints = map[string]videoModelConstraint{
	"seedance-2.0-fast": {Aspects: []string{"auto","16:9","9:16","21:9","4:3","3:4","1:1"}, RenderRes: []string{"480p","720p"}},
	"seedance-2.5": {Aspects: []string{"auto","1:1","3:4","4:3","21:9","9:16","16:9"}, RenderRes: []string{"1080p","480p","720p"}},
	"seedance-2.0": {Aspects: []string{"auto","16:9","9:16","21:9","4:3","3:4","1:1"}, RenderRes: []string{"1080p","480p","4k","720p"}},
	"minimax-h3": {Aspects: []string{"21:9","16:9","4:3","1:1","3:4","9:16"}, RenderRes: []string{"2k","768p"}},
	"wan-3.0": {Aspects: []string{"16:9","1:1","9:16","3:4","4:3"}, RenderRes: []string{"480p","720p"}},
	"flux-3": {Aspects: []string{"auto","21:9","2:1","16:9","4:3","1:1","3:4","9:16"}, RenderRes: []string{"1080p","720p"}},
	"flux-3-max": {Aspects: []string{"9:16","3:4","1:1","4:3","16:9","2:1","21:9","auto"}, RenderRes: []string{"720p","1080p"}},
	"veo-3.1-fast": {Aspects: []string{"9:16","16:9"}, RenderRes: []string{"720p","1080p"}},
	"kling-v3-turbo": {Aspects: []string{"16:9","1:1","9:16"}, RenderRes: []string{"1080p","720p"}},
	"kling-4.0": {Aspects: []string{"16:9","9:16","1:1","21:9"}, RenderRes: []string{"720p","1080p"}},
	"grok-video-1.5": {Aspects: []string{"16:9","1:1","9:16","3:4","4:3","3:2","2:3"}, RenderRes: []string{"480p","720p"}},
	"grok-video-1.5-lite": {Aspects: []string{"16:9","1:1","9:16","3:4","4:3","3:2","2:3"}, RenderRes: []string{"480p","720p","1080p"}},
	"ltx-2.5-pro": {Aspects: []string{"16:9","9:16"}, RenderRes: []string{"720p","1080p"}},
	"kling-3.0": {Aspects: []string{"1:1","16:9","9:16"}, RenderRes: []string{"720p","1080p","4k"}},
	"ltx-2.5-fast": {Aspects: []string{"16:9","9:16"}, RenderRes: []string{"720p","1080p"}},
	"veo-3.1": {Aspects: []string{"9:16","16:9"}, RenderRes: []string{"720p","1080p","4k"}},
	"kling-o3-omni": {Aspects: []string{"1:1","16:9","9:16"}, RenderRes: []string{"720p","1080p","4k"}},
	"runway-gen-4.5": {Aspects: []string{"9:16","16:9"}, RenderRes: []string{"720p","1080p","4k"}},
	"wan-2.7": {Aspects: []string{"1:1","9:16","16:9","3:4","4:3"}, RenderRes: []string{"720p","1080p"}},
	"gemini-omni-flash": {Aspects: []string{"16:9","9:16"}, RenderRes: []string{"720p","1080p","4k"}},
	"gemini-omni-flash-1.1": {Aspects: []string{"16:9","9:16"}, RenderRes: []string{"360p","720p","1080p","4k"}},
	"grok": {Aspects: []string{"1:1","16:9","9:16"}, RenderRes: []string{"480p","720p"}},
	"adobe-firefly": {Aspects: []string{"1:1","16:9","9:16","3:4","4:3"}, RenderRes: []string{"480p","720p"}},
	"pixverse-v6": {Aspects: []string{"1:1","3:4","9:16","4:3","16:9","21:9"}, RenderRes: []string{"1080p","720p"}},
	"luma-ray-v3.2": {Aspects: []string{"1:1","3:4","9:16","4:3","16:9","21:9"}, RenderRes: []string{"720p","1080p"}},
	"luma-dream-machine": {Aspects: []string{"1:1","3:4","9:16","4:3","16:9","21:9"}, RenderRes: []string{"720p","1080p"}},
	"heygen-v3": {Aspects: []string{"16:9","9:16"}, RenderRes: []string{"360p","480p","540p","720p","1080p"}},
	"wan-2.6": {Aspects: []string{"1:1","9:16","16:9","3:4","4:3"}, RenderRes: []string{"720p","1080p"}},
	"kling-2.6": {Aspects: []string{"1:1","16:9","9:16"}, RenderRes: []string{"720p","1080p"}},
	"seedance-1.5-pro": {Aspects: nil, RenderRes: []string{"720p","1080p"}},
	"kling-o3-edit": {Aspects: nil, RenderRes: []string{"720p","1080p","4k"}},
	"wan-2.7-edit": {Aspects: []string{"1:1","9:16","16:9","3:4","4:3"}, RenderRes: []string{"720p","1080p"}},
	"grok-video-edit": {Aspects: nil, RenderRes: []string{"480p","720p"}},
	"runway-aleph-2.0": {Aspects: nil, RenderRes: []string{"720p","1080p","4k"}},
	"pika-2.2": {Aspects: nil, RenderRes: []string{"720p","1080p"}},
	"seedance-2.0-mini": {Aspects: []string{"1:1","3:4","4:3","21:9","9:16","16:9","auto"}, RenderRes: []string{"480p","720p"}},
	"minimax-h3-max-turbo": {Aspects: []string{"9:16","3:4","1:1","4:3","16:9","21:9"}, RenderRes: []string{"720p"}},
	"minimax-h3-max": {Aspects: []string{"21:9","16:9","4:3","1:1","3:4","9:16"}, RenderRes: []string{"480p","768p"}},
	"gemini-omni-flash-1.2": {Aspects: []string{"9:16","16:9"}, RenderRes: []string{"360p","720p","1080p","4k"}},
	"wan-3.0-prime": {Aspects: []string{"4:3","3:4","9:16","1:1","16:9"}, RenderRes: []string{"720p","480p"}},
}
