package aepi

var VideoModels = []string{
	"adobe-firefly",
	"flux-3",
	"flux-3-max",
	"gemini-omni-flash",
	"gemini-omni-flash-1.1",
	"gemini-omni-flash-1.2",
	"grok",
	"grok-video-1.5",
	"grok-video-1.5-lite",
	"grok-video-edit",
	"heygen-v3",
	"kling-2.6",
	"kling-3.0",
	"kling-4.0",
	"kling-4.0-flash",
	"kling-o3-edit",
	"kling-o3-omni",
	"kling-v3-turbo",
	"ltx-2.5-fast",
	"ltx-2.5-pro",
	"luma-dream-machine",
	"luma-ray-v3.2",
	"minimax-h3",
	"minimax-h3-max",
	"minimax-h3-max-turbo",
	"minimax-hailuo-2.3",
	"pika-2.2",
	"pixverse-v6",
	"runway-aleph-2.0",
	"runway-gen-4.5",
	"seedance-2.0",
	"seedance-2.0-fast",
	"seedance-2.0-mini",
	"seedance-2.5",
	"seedance-2.5-flash",
	"wan-2.6",
	"wan-2.7",
	"wan-2.7-edit",
	"wan-3.0",
	"wan-3.0-prime",
}

var ImageModels = []string{
	"adobe-firefly",
	"dalle",
	"flux-2",
	"flux-2-turbo",
	"flux-3-image",
	"flux-3-klein",
	"flux-pro-kontext",
	"gpt-image-2",
	"gpt-image-2.5-flare",
	"gpt-image-2.5-sunburst",
	"gpt-image-2.6-flare",
	"gpt-image-2.6-sunburst",
	"grok",
	"grok-imagine-2",
	"grok-pro",
	"ideogram-3.0",
	"ideogram-4.0",
	"ideogram-4.5",
	"krea-2",
	"krea-3",
	"meta-muse-image",
	"microsoft-mai-image-2.5",
	"minimax-h3-image",
	"nano-banana",
	"nano-banana-2",
	"nano-banana-2.1",
	"nano-banana-lite",
	"nano-banana-pro",
	"qwen-image",
	"qwen-image-2",
	"qwen-image-2-pro",
	"qwen-image-2.1",
	"qwen-image-3",
	"recraft-v3",
	"recraft-v4.1",
	"recraft-v4.1-pro",
	"seedream-4.0",
	"seedream-4.5",
	"seedream-5-lite",
	"seedream-5.0-flash",
	"seedream-5.0-pro",
	"seedream-5.5",
}

var UtilityModels = []string{
	"music",
	"rescale",
	"upscale",
	"image-upscale",
	"image_upscale",
	"dub",
}

var AllModels = append(append(append([]string{}, VideoModels...), ImageModels...), UtilityModels...)

func IsImageModel(model string) bool {
	for _, m := range ImageModels {
		if m == model {
			return true
		}
	}
	return false
}

func IsVideoModel(model string) bool {
	for _, m := range VideoModels {
		if m == model {
			return true
		}
	}
	return false
}
