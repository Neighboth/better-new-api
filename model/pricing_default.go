package model

import (
	"strings"
)

// 简化的供应商映射规则
var defaultVendorRules = map[string]string{
	"gpt":       "OpenAI",
	"dall-e":    "OpenAI",
	"whisper":   "OpenAI",
	"o1":        "OpenAI",
	"o3":        "OpenAI",
	"claude":    "Anthropic",
	"gemini":    "Google",
	"moonshot":  "Moonshot",
	"kimi":      "Moonshot",
	"chatglm":   "Z.AI",
	"glm-":      "Z.AI",
	"qwen":      "Qwen",
	"deepseek":  "DeepSeek",
	"abab":      "MiniMax",
	"minimax":   "MiniMax",
	"ernie":     "Baidu",
	"spark-":    "iFlytek",
	"sparkdesk": "iFlytek",
	"iflytek":   "iFlytek",
	"hunyuan":   "Tencent",
	"command":   "Cohere",
	"@cf/":      "Cloudflare",
	"360":       "360",
	"yi-":       "01.AI",
	"jina":      "Jina",
	"mistral":   "Mistral",
	"grok":      "xAI",
	"llama":     "Meta",
	"doubao":    "ByteDance",
	"kling":     "Kuaishou",
	"jimeng":    "Jimeng",
	"vidu":      "Vidu",
	"google":    "Google",
}

// 供应商默认图标映射
var defaultVendorIcons = map[string]string{
	"OpenAI":     "OpenAI",
	"Anthropic":  "Claude.Color",
	"Google":     "Gemini.Color",
	"Moonshot":   "Moonshot",
	"Z.AI":       "Zhipu.Color",
	"Qwen":       "Qwen.Color",
	"DeepSeek":   "DeepSeek.Color",
	"MiniMax":    "Minimax.Color",
	"Baidu":      "Wenxin.Color",
	"iFlytek":    "Spark.Color",
	"Tencent":    "Hunyuan.Color",
	"Cohere":     "Cohere.Color",
	"Cloudflare": "Cloudflare.Color",
	"360":        "Ai360.Color",
	"01.AI":      "Yi.Color",
	"Jina":       "Jina",
	"Mistral":    "Mistral.Color",
	"xAI":        "XAI",
	"Meta":       "Meta.Color",
	"ByteDance":  "Doubao.Color",
	"Kuaishou":   "Kling.Color",
	"Jimeng":     "Jimeng.Color",
	"Vidu":       "Vidu",
	"Microsoft":  "AzureAI",
	"Azure":      "AzureAI",
	"百度":         "Wenxin.Color",
	"讯飞":         "Spark.Color",
	"腾讯":         "Hunyuan.Color",
	"零一万物":       "Yi.Color",
	"字节跳动":       "Doubao.Color",
	"快手":         "Kling.Color",
	"即梦":         "Jimeng.Color",
	"智谱":         "Zhipu.Color",
	"阿里巴巴":       "Qwen.Color",
}

// 明确的前缀优先匹配规则（最高优先级，防止被包含关键字误伤，例如 meta/muse-spark-open 不会被 spark 抢走）
var vendorPrefixRules = []struct {
	Prefix     string
	VendorName string
}{
	{"meta/", "Meta"},
	{"meta-", "Meta"},
	{"google/", "Google"},
	{"openai/", "OpenAI"},
	{"anthropic/", "Anthropic"},
	{"mistralai/", "Mistral"},
	{"deepseek-ai/", "DeepSeek"},
	{"qwen/", "Qwen"},
	{"baichuan-inc/", "Baichuan"},
	{"cohere/", "Cohere"},
	{"iflytek/", "iFlytek"},
	{"bytedance/", "ByteDance"},
	{"kuaishou/", "Kuaishou"},
}

// initDefaultVendorMapping 简化的默认供应商映射
func initDefaultVendorMapping(metaMap map[string]*Model, vendorMap map[int]*Vendor, enableAbilities []AbilityWithChannel) {
	for _, ability := range enableAbilities {
		modelName := ability.Model

		modelLower := strings.ToLower(modelName)
		var vendorID int

		// 1. 优先检查明确的前缀规则
		for _, rule := range vendorPrefixRules {
			if strings.HasPrefix(modelLower, rule.Prefix) {
				vendorID = getOrCreateVendor(rule.VendorName, vendorMap)
				break
			}
		}

		// 2. 检查现有供应商关键字
		if vendorID == 0 {
			vendorID = matchVendorByKeywords(modelLower, vendorMap)
		}

		// 3. 检查默认规则关键字
		if vendorID == 0 {
			for pattern, vendorName := range defaultVendorRules {
				if strings.Contains(modelLower, pattern) {
					vendorID = getOrCreateVendor(vendorName, vendorMap)
					break
				}
			}
		}

		if meta, exists := metaMap[modelName]; exists {
			if meta.VendorID == 0 && vendorID != 0 {
				meta.VendorID = vendorID
				// Also update the database to persist this assignment
				DB.Model(&Model{}).Where("model_name = ?", modelName).Update("vendor_id", vendorID)
			}
			continue
		}

		// 创建模型元数据
		metaMap[modelName] = &Model{
			ModelName: modelName,
			VendorID:  vendorID,
			Status:    1,
			NameRule:  NameRuleExact,
		}
	}
}

func matchVendorByKeywords(modelLower string, vendorMap map[int]*Vendor) int {
	for id, vendor := range vendorMap {
		if vendor.Keywords == "" {
			continue
		}
		keywords := strings.Split(vendor.Keywords, ",")
		for _, kw := range keywords {
			kw = strings.TrimSpace(strings.ToLower(kw))
			if kw != "" && strings.Contains(modelLower, kw) {
				return id
			}
		}
	}
	return 0
}

// 查找或创建供应商
func getOrCreateVendor(vendorName string, vendorMap map[int]*Vendor) int {
	// 查找现有供应商
	for id, vendor := range vendorMap {
		if vendor.Name == vendorName {
			return id
		}
	}

	// 创建新供应商
	newVendor := &Vendor{
		Name:   vendorName,
		Status: 1,
		Icon:   getDefaultVendorIcon(vendorName),
	}

	if err := newVendor.Insert(); err != nil {
		return 0
	}

	vendorMap[newVendor.Id] = newVendor
	return newVendor.Id
}

// 获取供应商默认图标
func getDefaultVendorIcon(vendorName string) string {
	if icon, exists := defaultVendorIcons[vendorName]; exists {
		return icon
	}
	return ""
}
