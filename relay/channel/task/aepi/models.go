package aepi

var VideoModels = []string{
	"seedance-2.0-fast",
	"seedance-2.5",
	"seedance-2.0",
	"seedance-1.5-pro",
	"seedance-2.0-mini",
	"minimax-h3",
	"minimax-hailuo-2.3",
	"minimax-h3-max-turbo",
	"minimax-h3-max",
	"wan-3.0",
	"wan-2.7",
	"wan-2.6",
	"wan-3.0-prime",
	"wan-2.7-edit",
	"flux-3",
	"flux-3-max",
	"veo-3.1-fast",
	"veo-3.1",
	"kling-v3-turbo",
	"kling-4.0",
	"kling-3.0",
	"kling-o3-omni",
	"kling-2.6",
	"kling-o3-edit",
	"grok-video-1.5",
	"grok-video-1.5-lite",
	"grok",
	"grok-video-edit",
	"ltx-2.5-pro",
	"ltx-2.5-fast",
	"runway-gen-4.5",
	"runway-aleph-2.0",
	"gemini-omni-flash",
	"gemini-omni-flash-1.1",
	"gemini-omni-flash-1.2",
	"adobe-firefly",
	"pixverse-v6",
	"luma-ray-v3.2",
	"luma-dream-machine",
	"heygen-v3",
	"pika-2.2",
	"davinci-fast-test",
	"davinci-fast",
	"davinci-cinema",
	"davinci-ultra",
}

var ImageModels = []string{
	"nano-banana-pro",
	"nano-banana-2",
	"nano-banana-2.1",
	"nano-banana",
	"nano-banana-lite",
	"gpt-image-2",
	"gpt-image-2.5-flare",
	"gpt-image-2.5-sunburst",
	"gpt-image-2.6-flare",
	"gpt-image-2.6-sunburst",
	"seedream-5.0-pro",
	"seedream-4.5",
	"seedream-4.0",
	"seedream-5-lite",
	"seedream-5.0-flash",
	"grok-pro",
	"grok-imagine-2",
	"flux-2",
	"flux-2-turbo",
	"flux-pro-kontext",
	"flux-3-image",
	"flux-3-klein",
	"ideogram-4.0",
	"ideogram-3.0",
	"ideogram-4.5",
	"recraft-v4.1",
	"recraft-v3",
	"recraft-v4.1-pro",
	"qwen-image",
	"qwen-image-2",
	"qwen-image-2-pro",
	"qwen-image-2.1",
	"qwen-image-3",
	"microsoft-mai-image-2.5",
	"krea-2",
	"krea-3",
	"meta-muse-image",
	"dalle",
}

var UtilityModels = []string{
	"music",
	"rescale",
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
