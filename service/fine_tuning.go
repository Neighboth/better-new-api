package service

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

func IsFineTuningEnabled() bool {
	common.OptionMapRWMutex.RLock()
	enabled := common.OptionMap["model_deployment.fine_tuning.enabled"] == "true"
	common.OptionMapRWMutex.RUnlock()
	return enabled
}

func IsFineTuningServingEnabled() bool {
	common.OptionMapRWMutex.RLock()
	ftEnabled := common.OptionMap["model_deployment.fine_tuning.enabled"] == "true"
	servingEnabled := common.OptionMap["model_deployment.fine_tuning.serving_enabled"] == "true"
	common.OptionMapRWMutex.RUnlock()
	return ftEnabled && servingEnabled
}

type FineTuningGPUOption struct {
	ID               string   `json:"id"`
	Label            string   `json:"label"`
	Kind             string   `json:"kind,omitempty"` // cpu | gpu | tpu
	Providers        []string `json:"providers,omitempty"`
	SupportsColabRAM bool     `json:"supports_colab_ram,omitempty"`
	PricePerSecond   float64  `json:"price_per_second"`
	Enabled          bool     `json:"enabled"`
}

type hwCatalogItem struct {
	ID             string
	Label          string
	Kind           string
	DefaultPricePS float64
}

var providerHardwareCatalog = map[string][]hwCatalogItem{
	"modal": {
		{ID: "B300", Label: "GPU: NVIDIA B300 (262GB)", Kind: "gpu", DefaultPricePS: 0.018},
		{ID: "B200", Label: "GPU: NVIDIA B200 (180GB)", Kind: "gpu", DefaultPricePS: 0.015},
		{ID: "H200", Label: "GPU: NVIDIA H200 SXM (141GB)", Kind: "gpu", DefaultPricePS: 0.012},
		{ID: "H100", Label: "GPU: NVIDIA H100 (80GB)", Kind: "gpu", DefaultPricePS: 0.010},
		{ID: "RTX-PRO-6000", Label: "GPU: NVIDIA RTX PRO 6000 (96GB)", Kind: "gpu", DefaultPricePS: 0.008},
		{ID: "A100-80GB", Label: "GPU: NVIDIA A100 (80GB)", Kind: "gpu", DefaultPricePS: 0.006},
		{ID: "A100-40GB", Label: "GPU: NVIDIA A100 (40GB)", Kind: "gpu", DefaultPricePS: 0.0045},
		{ID: "L40S", Label: "GPU: NVIDIA L40S (48GB)", Kind: "gpu", DefaultPricePS: 0.004},
		{ID: "A10G", Label: "GPU: NVIDIA A10G (24GB)", Kind: "gpu", DefaultPricePS: 0.003},
		{ID: "L4", Label: "GPU: NVIDIA L4 (24GB)", Kind: "gpu", DefaultPricePS: 0.0025},
		{ID: "T4", Label: "GPU: NVIDIA T4 (16GB)", Kind: "gpu", DefaultPricePS: 0.0015},
		{ID: "CPU", Label: "CPU (Standard)", Kind: "cpu", DefaultPricePS: 0.0005},
	},
	"colab": {
		{ID: "NVIDIA_TESLA_T4", Label: "GPU: NVIDIA Tesla T4 (16GB)", Kind: "gpu", DefaultPricePS: 0.0015},
		{ID: "NVIDIA_L4", Label: "GPU: NVIDIA L4 (24GB)", Kind: "gpu", DefaultPricePS: 0.0025},
		{ID: "NVIDIA_TESLA_A100", Label: "GPU: NVIDIA A100 (40GB)", Kind: "gpu", DefaultPricePS: 0.005},
		{ID: "NVIDIA_TESLA_A100_80GB", Label: "GPU: NVIDIA A100 (80GB)", Kind: "gpu", DefaultPricePS: 0.006},
		{ID: "NVIDIA_RTX_PRO_6000", Label: "GPU: NVIDIA RTX PRO 6000", Kind: "gpu", DefaultPricePS: 0.008},
		{ID: "NVIDIA_H100_80GB", Label: "GPU: NVIDIA H100 (80GB)", Kind: "gpu", DefaultPricePS: 0.010},
		{ID: "TPU_V2_8", Label: "TPU: v2-8", Kind: "tpu", DefaultPricePS: 0.003},
		{ID: "TPU_V5E_1", Label: "TPU: v5e-1", Kind: "tpu", DefaultPricePS: 0.0025},
		{ID: "TPU_V6E_1", Label: "TPU: v6e-1", Kind: "tpu", DefaultPricePS: 0.004},
		{ID: "CPU", Label: "CPU (Standard)", Kind: "cpu", DefaultPricePS: 0.0005},
	},
	"kaggle": {
		{ID: "NvidiaTeslaT4", Label: "GPU: NVIDIA Tesla T4 x2", Kind: "gpu", DefaultPricePS: 0.002},
		{ID: "NvidiaL4", Label: "GPU: NVIDIA L4", Kind: "gpu", DefaultPricePS: 0.0025},
		{ID: "TpuV5E8", Label: "TPU: v5e-8", Kind: "tpu", DefaultPricePS: 0.0035},
		{ID: "TpuV6E8", Label: "TPU: v6e-8", Kind: "tpu", DefaultPricePS: 0.005},
		{ID: "CPU", Label: "CPU (4 cores, 30 GB RAM)", Kind: "cpu", DefaultPricePS: 0.0005},
	},
	"huggingface": {
		{ID: "t4-small", Label: "GPU: NVIDIA T4 Small (16GB)", Kind: "gpu", DefaultPricePS: 0.0015},
		{ID: "t4-medium", Label: "GPU: NVIDIA T4 Medium", Kind: "gpu", DefaultPricePS: 0.002},
		{ID: "l4x1", Label: "GPU: NVIDIA L4 x1 (24GB)", Kind: "gpu", DefaultPricePS: 0.0025},
		{ID: "l4x4", Label: "GPU: NVIDIA L4 x4 (96GB)", Kind: "gpu", DefaultPricePS: 0.008},
		{ID: "a10g-small", Label: "GPU: NVIDIA A10G Small (24GB)", Kind: "gpu", DefaultPricePS: 0.003},
		{ID: "a10g-large", Label: "GPU: NVIDIA A10G Large", Kind: "gpu", DefaultPricePS: 0.004},
		{ID: "l40sx1", Label: "GPU: NVIDIA L40S x1 (48GB)", Kind: "gpu", DefaultPricePS: 0.0045},
		{ID: "l40sx4", Label: "GPU: NVIDIA L40S x4 (192GB)", Kind: "gpu", DefaultPricePS: 0.015},
		{ID: "a100-large", Label: "GPU: NVIDIA A100 x1 (80GB)", Kind: "gpu", DefaultPricePS: 0.006},
		{ID: "a100x2", Label: "GPU: NVIDIA A100 x2 (160GB)", Kind: "gpu", DefaultPricePS: 0.012},
		{ID: "a100x4", Label: "GPU: NVIDIA A100 x4 (320GB)", Kind: "gpu", DefaultPricePS: 0.024},
		{ID: "a100x8", Label: "GPU: NVIDIA A100 x8 (640GB)", Kind: "gpu", DefaultPricePS: 0.048},
		{ID: "h100x1", Label: "GPU: NVIDIA H100 x1 (80GB)", Kind: "gpu", DefaultPricePS: 0.010},
		{ID: "h100x4", Label: "GPU: NVIDIA H100 x4 (320GB)", Kind: "gpu", DefaultPricePS: 0.040},
		{ID: "h100x8", Label: "GPU: NVIDIA H100 x8 (640GB)", Kind: "gpu", DefaultPricePS: 0.080},
		{ID: "h200x1", Label: "GPU: NVIDIA H200 x1 (141GB)", Kind: "gpu", DefaultPricePS: 0.012},
		{ID: "h200x8", Label: "GPU: NVIDIA H200 x8", Kind: "gpu", DefaultPricePS: 0.096},
		{ID: "cpu-basic", Label: "CPU: Basic (x2)", Kind: "cpu", DefaultPricePS: 0.0004},
		{ID: "cpu-upgrade", Label: "CPU: Upgrade (x4)", Kind: "cpu", DefaultPricePS: 0.0008},
	},
	"runpod": {
		{ID: "B200", Label: "GPU: NVIDIA B200 (180GB)", Kind: "gpu", DefaultPricePS: 0.015},
		{ID: "H200", Label: "GPU: NVIDIA H200 (141GB)", Kind: "gpu", DefaultPricePS: 0.012},
		{ID: "H100-SXM", Label: "GPU: NVIDIA H100 SXM (80GB)", Kind: "gpu", DefaultPricePS: 0.010},
		{ID: "A100-80GB", Label: "GPU: NVIDIA A100 (80GB)", Kind: "gpu", DefaultPricePS: 0.006},
		{ID: "L40S", Label: "GPU: NVIDIA L40S (48GB)", Kind: "gpu", DefaultPricePS: 0.004},
		{ID: "RTX-4090", Label: "GPU: NVIDIA RTX 4090 (24GB)", Kind: "gpu", DefaultPricePS: 0.0025},
		{ID: "RTX-3090", Label: "GPU: NVIDIA RTX 3090 (24GB)", Kind: "gpu", DefaultPricePS: 0.0018},
		{ID: "CPU", Label: "CPU (Standard)", Kind: "cpu", DefaultPricePS: 0.0005},
	},
	"hostinger": {
		{ID: "H100", Label: "GPU: NVIDIA H100 (80GB)", Kind: "gpu", DefaultPricePS: 0.010},
		{ID: "A100", Label: "GPU: NVIDIA A100 (80GB)", Kind: "gpu", DefaultPricePS: 0.006},
		{ID: "L40S", Label: "GPU: NVIDIA L40S (48GB)", Kind: "gpu", DefaultPricePS: 0.004},
		{ID: "RTX-4090", Label: "GPU: NVIDIA RTX 4090 (24GB)", Kind: "gpu", DefaultPricePS: 0.0025},
		{ID: "CPU", Label: "CPU (KVM VPS)", Kind: "cpu", DefaultPricePS: 0.0005},
	},
	"ionet": {
		{ID: "H100-80GB", Label: "GPU: NVIDIA H100 (80GB)", Kind: "gpu", DefaultPricePS: 0.010},
		{ID: "A100-80GB", Label: "GPU: NVIDIA A100 (80GB)", Kind: "gpu", DefaultPricePS: 0.006},
		{ID: "L40S", Label: "GPU: NVIDIA L40S (48GB)", Kind: "gpu", DefaultPricePS: 0.004},
		{ID: "RTX-4090", Label: "GPU: NVIDIA RTX 4090 (24GB)", Kind: "gpu", DefaultPricePS: 0.0025},
		{ID: "RTX-3090", Label: "GPU: NVIDIA RTX 3090 (24GB)", Kind: "gpu", DefaultPricePS: 0.0018},
		{ID: "CPU", Label: "CPU (Cluster)", Kind: "cpu", DefaultPricePS: 0.0005},
	},
}

var providerTierAllowedHardware = map[string]map[string][]string{
	"modal": {
		"free_no_card": {"CPU"},
		"starter_paid": nil, // nil = all
	},
	"colab": {
		"free":            {"CPU", "NVIDIA_TESLA_T4", "TPU_V5E_1"},
		"google_ai_pro":   {"CPU", "NVIDIA_TESLA_T4", "TPU_V2_8", "TPU_V5E_1"},
		"colab_pro":       {"CPU", "NVIDIA_TESLA_T4", "NVIDIA_L4", "NVIDIA_TESLA_A100", "TPU_V2_8", "TPU_V5E_1"},
		"google_ai_ultra": nil,
	},
	"kaggle": {
		"verified_free": {"CPU", "NvidiaTeslaT4", "TpuV5E8"},
		"unverified":    {"CPU"},
		"cloud_linked":  nil,
	},
}

var fineTuningProviderOrder = []string{"colab", "kaggle", "modal", "huggingface", "runpod", "hostinger", "ionet"}

var defaultFineTuningGPUs = []FineTuningGPUOption{
	{ID: "CPU", Label: "CPU (Standard)", Kind: "cpu", PricePerSecond: 0.0005, Enabled: true},
	{ID: "NVIDIA_TESLA_T4", Label: "GPU: NVIDIA Tesla T4 (16GB)", Kind: "gpu", PricePerSecond: 0.0015, Enabled: true},
	{ID: "NVIDIA_L4", Label: "GPU: NVIDIA L4 (24GB)", Kind: "gpu", PricePerSecond: 0.0025, Enabled: true},
	{ID: "NVIDIA_A10G", Label: "GPU: NVIDIA A10G (24GB)", Kind: "gpu", PricePerSecond: 0.0030, Enabled: true},
	{ID: "NVIDIA_TESLA_A100", Label: "GPU: NVIDIA A100 (40GB)", Kind: "gpu", PricePerSecond: 0.0060, Enabled: true},
	{ID: "NVIDIA_H100_80GB", Label: "GPU: NVIDIA H100 (80GB)", Kind: "gpu", PricePerSecond: 0.0100, Enabled: true},
}

type deploymentAccountLite struct {
	Tier    string `json:"tier"`
	Enabled bool   `json:"enabled"`
}

func defaultTierForProviderLite(provider string) string {
	switch provider {
	case "modal":
		return "free_no_card"
	case "colab":
		return "free"
	case "kaggle":
		return "verified_free"
	default:
		return "standard"
	}
}

func hardwareAllowedForTier(provider, tier string) []hwCatalogItem {
	all := providerHardwareCatalog[provider]
	if len(all) == 0 {
		return nil
	}
	if tier == "" {
		tier = defaultTierForProviderLite(provider)
	}
	tierMap, hasTiers := providerTierAllowedHardware[provider]
	if !hasTiers {
		return all
	}
	allowed, ok := tierMap[tier]
	if !ok {
		allowed = tierMap[defaultTierForProviderLite(provider)]
	}
	if allowed == nil {
		return all
	}
	allowedSet := make(map[string]bool, len(allowed))
	for _, id := range allowed {
		allowedSet[id] = true
	}
	filtered := make([]hwCatalogItem, 0, len(allowed))
	for _, item := range all {
		if allowedSet[item.ID] {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// collectAccountAvailableHardware returns the union of CPU/GPU/TPU hardware
// available across all currently enabled deployment providers and their
// active accounts' tiers. Returns (items, hasAnyEnabledProvider).
func collectAccountAvailableHardware() ([]FineTuningGPUOption, bool) {
	common.OptionMapRWMutex.RLock()
	enabledProviders := make([]string, 0, len(fineTuningProviderOrder))
	accountsRaw := make(map[string]string, len(fineTuningProviderOrder))
	for _, p := range fineTuningProviderOrder {
		if common.OptionMap["model_deployment."+p+".enabled"] == "true" {
			enabledProviders = append(enabledProviders, p)
			accountsRaw[p] = strings.TrimSpace(common.OptionMap["model_deployment."+p+".accounts"])
		}
	}
	common.OptionMapRWMutex.RUnlock()

	if len(enabledProviders) == 0 {
		return nil, false
	}

	byID := make(map[string]*FineTuningGPUOption)
	order := make([]string, 0, 16)

	addItem := func(provider string, item hwCatalogItem) {
		if existing, ok := byID[item.ID]; ok {
			hasProv := false
			for _, ep := range existing.Providers {
				if ep == provider {
					hasProv = true
					break
				}
			}
			if !hasProv {
				existing.Providers = append(existing.Providers, provider)
			}
			if provider == "colab" {
				existing.SupportsColabRAM = true
			}
			return
		}
		opt := &FineTuningGPUOption{
			ID:               item.ID,
			Label:            item.Label,
			Kind:             item.Kind,
			Providers:        []string{provider},
			SupportsColabRAM: provider == "colab",
			PricePerSecond:   item.DefaultPricePS,
			Enabled:          true,
		}
		byID[item.ID] = opt
		order = append(order, item.ID)
	}

	for _, p := range enabledProviders {
		var accounts []deploymentAccountLite
		if raw := accountsRaw[p]; raw != "" {
			_ = common.UnmarshalJsonStr(raw, &accounts)
		}
		activeTiers := make([]string, 0, len(accounts))
		for _, acc := range accounts {
			if acc.Enabled {
				t := strings.TrimSpace(acc.Tier)
				if t == "" {
					t = defaultTierForProviderLite(p)
				}
				activeTiers = append(activeTiers, t)
			}
		}
		// If provider is enabled with legacy single-account config (no accounts JSON yet), use default tier
		if len(accounts) == 0 {
			activeTiers = append(activeTiers, defaultTierForProviderLite(p))
		}
		for _, tier := range activeTiers {
			for _, item := range hardwareAllowedForTier(p, tier) {
				addItem(p, item)
			}
		}
	}

	result := make([]FineTuningGPUOption, 0, len(order))
	for _, id := range order {
		result = append(result, *byID[id])
	}
	return result, true
}

func IsColabDeploymentEnabled() bool {
	common.OptionMapRWMutex.RLock()
	enabled := common.OptionMap["model_deployment.colab.enabled"] == "true"
	common.OptionMapRWMutex.RUnlock()
	return enabled
}

func GetColabRAMMultipliers() (normalMult float64, highMult float64) {
	normalMult = 1.0
	highMult = 1.5
	common.OptionMapRWMutex.RLock()
	rawNormal := strings.TrimSpace(common.OptionMap["model_deployment.fine_tuning.colab_normal_ram_multiplier"])
	rawHigh := strings.TrimSpace(common.OptionMap["model_deployment.fine_tuning.colab_high_ram_multiplier"])
	common.OptionMapRWMutex.RUnlock()
	if rawNormal != "" {
		if v, err := strconv.ParseFloat(rawNormal, 64); err == nil && v > 0 {
			normalMult = v
		}
	}
	if rawHigh != "" {
		if v, err := strconv.ParseFloat(rawHigh, 64); err == nil && v > 0 {
			highMult = v
		}
	}
	return normalMult, highMult
}

func GetFineTuningRetentionDays() int {
	days := 30
	common.OptionMapRWMutex.RLock()
	raw := strings.TrimSpace(common.OptionMap["model_deployment.fine_tuning.retention_days"])
	common.OptionMapRWMutex.RUnlock()
	if raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v >= 1 && v <= 365 {
			days = v
		}
	}
	return days
}

func GetFineTuningExpiryWarningDays() int {
	days := 3
	common.OptionMapRWMutex.RLock()
	raw := strings.TrimSpace(common.OptionMap["model_deployment.fine_tuning.expiry_warning_days"])
	common.OptionMapRWMutex.RUnlock()
	if raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v >= 0 && v <= 60 {
			days = v
		}
	}
	return days
}

// GetFineTuningMaxStorageGB returns the global storage limit in gigabytes across
// all fine-tuning artifacts, datasets, staged uploads, and checkpoints (0 = unlimited).
func GetFineTuningMaxStorageGB() float64 {
	common.OptionMapRWMutex.RLock()
	raw := strings.TrimSpace(common.OptionMap["model_deployment.fine_tuning.max_storage_gb"])
	common.OptionMapRWMutex.RUnlock()
	if raw == "" {
		return 0
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 {
		return 0
	}
	return v
}

// GetFineTuningMaxStorageBytes returns the global fine-tuning storage cap in bytes (0 = unlimited).
func GetFineTuningMaxStorageBytes() int64 {
	gb := GetFineTuningMaxStorageGB()
	if gb <= 0 {
		return 0
	}
	const bytesPerGB = 1024.0 * 1024.0 * 1024.0
	maxF := gb * bytesPerGB
	if maxF >= float64(math.MaxInt64/2) {
		return math.MaxInt64 / 2
	}
	return int64(math.Round(maxF))
}

var fineTuningRootStorageDirOverride string

// FineTuningRootStorageDir returns the root directory containing all fine-tuning files.
func FineTuningRootStorageDir() string {
	if strings.TrimSpace(fineTuningRootStorageDirOverride) != "" {
		return fineTuningRootStorageDirOverride
	}
	return filepath.Join("uploads", "fine_tuning")
}

// CalculateFineTuningStorageUsedBytes sums the disk size of every file under uploads/fine_tuning.
func CalculateFineTuningStorageUsedBytes() int64 {
	root := FineTuningRootStorageDir()
	var total int64
	_ = filepath.WalkDir(root, func(_ string, d os.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return nil
		}
		if info, iErr := d.Info(); iErr == nil && info.Size() > 0 {
			total += info.Size()
		}
		return nil
	})
	return total
}

// CheckFineTuningStorageQuota verifies whether adding additionalBytes stays within
// the global fine-tuning storage limit configured in System Settings.
func CheckFineTuningStorageQuota(additionalBytes int64) (allowed bool, usedBytes int64, maxBytes int64) {
	maxBytes = GetFineTuningMaxStorageBytes()
	usedBytes = CalculateFineTuningStorageUsedBytes()
	if maxBytes <= 0 {
		return true, usedBytes, 0
	}
	if additionalBytes < 0 {
		additionalBytes = 0
	}
	return usedBytes+additionalBytes <= maxBytes, usedBytes, maxBytes
}

var customFineTunedModelIDRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:-]{0,118}[a-zA-Z0-9]$`)

// ValidateCustomFineTunedModelID validates a direct user-chosen fine-tuned model ID
// (e.g. "benim-modelim", "acme-support-v2").
func ValidateCustomFineTunedModelID(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("Model ID is required")
	}
	if len(trimmed) < 2 || len(trimmed) > 120 {
		return "", fmt.Errorf("Model ID must be between 2 and 120 characters")
	}
	if !customFineTunedModelIDRe.MatchString(trimmed) {
		return "", fmt.Errorf("Model ID may only contain letters, numbers, hyphens (-), underscores (_), dots (.), and colons (:), and must start and end with a letter or number")
	}
	return trimmed, nil
}

// QLoRAQuantizationFactor returns the VRAM multiplier relative to FP16 weight size
// for a given QLoRA quantization profile.
func QLoRAQuantizationFactor(quant string) float64 {
	q := strings.ToUpper(strings.TrimSpace(quant))
	switch {
	case q == "" || q == "NF4" || q == "FP4" || strings.HasPrefix(q, "Q4_") || strings.HasPrefix(q, "IQ4_"):
		return 0.40
	case q == "INT8" || strings.HasPrefix(q, "Q8_"):
		return 0.78
	case strings.HasPrefix(q, "Q6_"):
		return 0.62
	case strings.HasPrefix(q, "Q5_"):
		return 0.54
	case strings.HasPrefix(q, "Q3_") || strings.HasPrefix(q, "IQ3_"):
		return 0.36
	case strings.HasPrefix(q, "Q2_") || strings.HasPrefix(q, "IQ2_"):
		return 0.32
	case q == "FP16" || q == "BF16":
		return 1.20
	default:
		return 0.40
	}
}

var modelSizeBillionsRe = regexp.MustCompile(`(?i)(?:^|[^0-9a-z])(\d+(?:\.\d+)?)\s*([bm])(?:[^a-z]|$)`)

// ParseModelBillions extracts an approximate parameter count (in billions) from
// explicit HF metadata, a model repository/filename string, and/or weight file
// size in bytes (~2 bytes/param for FP16/BF16). When modelFileBytes is provided,
// the file-size floor is respected so huge safetensors weights (e.g. 94 GB) are
// never underestimated as a small model.
func ParseModelBillions(modelName string, paramSizeHint string, modelFileBytes int64) float64 {
	parseHint := func(raw string) float64 {
		s := strings.TrimSpace(raw)
		if s == "" {
			return 0
		}
		if matches := modelSizeBillionsRe.FindAllStringSubmatch(s, -1); len(matches) > 0 {
			best := 0.0
			for _, m := range matches {
				v, err := strconv.ParseFloat(m[1], 64)
				if err != nil || v <= 0 {
					continue
				}
				if strings.EqualFold(m[2], "m") {
					v = v / 1000.0
				}
				if v > best && v <= 2000 {
					best = v
				}
			}
			if best > 0 {
				return best
			}
		}
		return 0
	}

	fileEst := 0.0
	if modelFileBytes > 0 {
		// FP16/BF16 weights are ~2 bytes per parameter (FP8 ~1 byte/param)
		fileEst = float64(modelFileBytes) / 2_000_000_000.0
		if fileEst < 0.1 {
			fileEst = 0.1
		}
		if fileEst > 1000 {
			fileEst = 1000
		}
	}

	if v := parseHint(paramSizeHint); v > 0 {
		if fileEst > v {
			return fileEst
		}
		return v
	}
	if v := parseHint(modelName); v > 0 {
		if fileEst > v {
			return fileEst
		}
		return v
	}
	if fileEst > 0 {
		return fileEst
	}
	lower := strings.ToLower(modelName)
	switch {
	case strings.Contains(lower, "deepseek-r1"), strings.Contains(lower, "deepseek-v3"):
		if !strings.Contains(lower, "distill") {
			return 671.0
		}
	case strings.Contains(lower, "deepseek-v2.5"), strings.Contains(lower, "deepseek-v2"):
		if !strings.Contains(lower, "lite") {
			return 236.0
		}
		return 16.0
	case strings.Contains(lower, "flux"):
		return 12.0
	case strings.Contains(lower, "sdxl"), strings.Contains(lower, "xl"):
		return 6.5
	case strings.Contains(lower, "sd3"):
		return 8.0
	case strings.Contains(lower, "stable-diffusion"), strings.Contains(lower, "cyberrealistic"), strings.Contains(lower, "sd1.5"):
		return 1.2
	case strings.Contains(lower, "whisper-large"):
		return 1.55
	}
	return 7.0
}

// FineTuningHardwareSpec describes memory capacity for an accelerator ID.
type FineTuningHardwareSpec struct {
	Kind   string  `json:"kind"` // cpu | gpu | tpu
	VRAMGB float64 `json:"vram_gb"`
	RAMGB  float64 `json:"ram_gb"`
}

// GetAcceleratorSpecs returns the usable VRAM/HBM and system RAM (GB) for a given
// accelerator ID and optional Colab RAM tier ("normal" | "high").
func GetAcceleratorSpecs(gpuID string, ramTier string) FineTuningHardwareSpec {
	highRAM := strings.EqualFold(strings.TrimSpace(ramTier), "high")
	trimmed := strings.TrimSpace(gpuID)
	switch trimmed {
	case "CPU", "cpu-basic":
		ram := 16.0
		if highRAM {
			ram = 51.0
		}
		return FineTuningHardwareSpec{Kind: "cpu", VRAMGB: 0, RAMGB: ram}
	case "cpu-upgrade":
		return FineTuningHardwareSpec{Kind: "cpu", VRAMGB: 0, RAMGB: 32.0}
	case "T4", "NVIDIA_TESLA_T4", "t4-small":
		ram := 12.7
		if highRAM {
			ram = 51.0
		}
		return FineTuningHardwareSpec{Kind: "gpu", VRAMGB: 16.0, RAMGB: ram}
	case "t4-medium":
		return FineTuningHardwareSpec{Kind: "gpu", VRAMGB: 16.0, RAMGB: 30.0}
	case "NvidiaTeslaT4": // Kaggle T4 x2
		return FineTuningHardwareSpec{Kind: "gpu", VRAMGB: 32.0, RAMGB: 30.0}
	case "L4", "NVIDIA_L4", "NvidiaL4", "l4x1":
		ram := 53.0
		if highRAM {
			ram = 62.0
		}
		return FineTuningHardwareSpec{Kind: "gpu", VRAMGB: 24.0, RAMGB: ram}
	case "l4x4":
		return FineTuningHardwareSpec{Kind: "gpu", VRAMGB: 96.0, RAMGB: 128.0}
	case "A10G", "NVIDIA_A10G", "a10g-small", "a10g-large":
		return FineTuningHardwareSpec{Kind: "gpu", VRAMGB: 24.0, RAMGB: 46.0}
	case "RTX-3090", "RTX-4090":
		return FineTuningHardwareSpec{Kind: "gpu", VRAMGB: 24.0, RAMGB: 48.0}
	case "L40S", "l40sx1":
		return FineTuningHardwareSpec{Kind: "gpu", VRAMGB: 48.0, RAMGB: 64.0}
	case "l40sx4":
		return FineTuningHardwareSpec{Kind: "gpu", VRAMGB: 192.0, RAMGB: 256.0}
	case "A100-40GB", "NVIDIA_TESLA_A100":
		return FineTuningHardwareSpec{Kind: "gpu", VRAMGB: 40.0, RAMGB: 83.0}
	case "A100-80GB", "NVIDIA_TESLA_A100_80GB", "A100", "a100-large":
		return FineTuningHardwareSpec{Kind: "gpu", VRAMGB: 80.0, RAMGB: 167.0}
	case "a100x2":
		return FineTuningHardwareSpec{Kind: "gpu", VRAMGB: 160.0, RAMGB: 256.0}
	case "a100x4":
		return FineTuningHardwareSpec{Kind: "gpu", VRAMGB: 320.0, RAMGB: 512.0}
	case "a100x8":
		return FineTuningHardwareSpec{Kind: "gpu", VRAMGB: 640.0, RAMGB: 1024.0}
	case "RTX-PRO-6000", "NVIDIA_RTX_PRO_6000":
		return FineTuningHardwareSpec{Kind: "gpu", VRAMGB: 96.0, RAMGB: 128.0}
	case "H100", "H100-SXM", "H100-80GB", "NVIDIA_H100_80GB", "h100x1":
		return FineTuningHardwareSpec{Kind: "gpu", VRAMGB: 80.0, RAMGB: 167.0}
	case "h100x4":
		return FineTuningHardwareSpec{Kind: "gpu", VRAMGB: 320.0, RAMGB: 512.0}
	case "h100x8":
		return FineTuningHardwareSpec{Kind: "gpu", VRAMGB: 640.0, RAMGB: 1024.0}
	case "H200", "h200x1":
		return FineTuningHardwareSpec{Kind: "gpu", VRAMGB: 141.0, RAMGB: 256.0}
	case "h200x8":
		return FineTuningHardwareSpec{Kind: "gpu", VRAMGB: 1128.0, RAMGB: 1024.0}
	case "B200":
		return FineTuningHardwareSpec{Kind: "gpu", VRAMGB: 180.0, RAMGB: 256.0}
	case "B300":
		return FineTuningHardwareSpec{Kind: "gpu", VRAMGB: 262.0, RAMGB: 384.0}
	case "TPU_V2_8":
		return FineTuningHardwareSpec{Kind: "tpu", VRAMGB: 64.0, RAMGB: 334.0}
	case "TPU_V5E_1":
		return FineTuningHardwareSpec{Kind: "tpu", VRAMGB: 16.0, RAMGB: 47.0}
	case "TPU_V6E_1":
		return FineTuningHardwareSpec{Kind: "tpu", VRAMGB: 32.0, RAMGB: 172.0}
	case "TpuV5E8":
		return FineTuningHardwareSpec{Kind: "tpu", VRAMGB: 128.0, RAMGB: 330.0}
	case "TpuV6E8":
		return FineTuningHardwareSpec{Kind: "tpu", VRAMGB: 256.0, RAMGB: 330.0}
	}

	upper := strings.ToUpper(trimmed)
	switch {
	case strings.Contains(upper, "B300"):
		return FineTuningHardwareSpec{Kind: "gpu", VRAMGB: 262.0, RAMGB: 384.0}
	case strings.Contains(upper, "B200"):
		return FineTuningHardwareSpec{Kind: "gpu", VRAMGB: 180.0, RAMGB: 256.0}
	case strings.Contains(upper, "H200"):
		return FineTuningHardwareSpec{Kind: "gpu", VRAMGB: 141.0, RAMGB: 256.0}
	case strings.Contains(upper, "H100"):
		return FineTuningHardwareSpec{Kind: "gpu", VRAMGB: 80.0, RAMGB: 167.0}
	case strings.Contains(upper, "A100"):
		return FineTuningHardwareSpec{Kind: "gpu", VRAMGB: 40.0, RAMGB: 83.0}
	case strings.Contains(upper, "L40S"):
		return FineTuningHardwareSpec{Kind: "gpu", VRAMGB: 48.0, RAMGB: 64.0}
	case strings.Contains(upper, "L4"), strings.Contains(upper, "A10G"), strings.Contains(upper, "4090"), strings.Contains(upper, "3090"):
		return FineTuningHardwareSpec{Kind: "gpu", VRAMGB: 24.0, RAMGB: 48.0}
	case strings.Contains(upper, "T4"):
		return FineTuningHardwareSpec{Kind: "gpu", VRAMGB: 16.0, RAMGB: 16.0}
	case strings.Contains(upper, "TPU"):
		return FineTuningHardwareSpec{Kind: "tpu", VRAMGB: 32.0, RAMGB: 96.0}
	case strings.Contains(upper, "CPU"):
		return FineTuningHardwareSpec{Kind: "cpu", VRAMGB: 0, RAMGB: 16.0}
	default:
		return FineTuningHardwareSpec{Kind: "gpu", VRAMGB: 16.0, RAMGB: 32.0}
	}
}

// FineTuningCompatCheck reports whether the selected accelerator has enough
// VRAM/RAM for the selected model, method, and dataset size.
type FineTuningCompatCheck struct {
	Compatible      bool    `json:"compatible"`
	Code            string  `json:"code,omitempty"` // "" | "vram_exceeded" | "ram_exceeded" | "cpu_too_large"
	RequiredVRAMGB  float64 `json:"required_vram_gb"`
	AvailableVRAMGB float64 `json:"available_vram_gb"`
	RequiredRAMGB   float64 `json:"required_ram_gb"`
	AvailableRAMGB  float64 `json:"available_ram_gb"`
	ModelSizeGB     float64 `json:"model_size_gb"`
	Reason          string  `json:"reason,omitempty"`
}

func estimateModelSizeGB(in FineTuningEstimateInput) (modelB float64, modelGB float64) {
	modelB = ParseModelBillions(in.ModelName, in.ModelParamHint, in.ModelFileBytes)
	modelGB = modelB * 2.0
	if in.ModelFileBytes > 0 {
		fileGB := float64(in.ModelFileBytes) / 1_000_000_000.0
		if fileGB > modelGB {
			modelGB = fileGB
		}
	}
	return modelB, modelGB
}

// CheckFineTuningHardwareCompat checks whether the chosen model + method + dataset
// can realistically fit in the selected CPU/GPU/TPU memory.
func CheckFineTuningHardwareCompat(in FineTuningEstimateInput) FineTuningCompatCheck {
	spec := GetAcceleratorSpecs(in.GPU, in.RamTier)
	_, modelGB := estimateModelSizeGB(in)
	datasetGB := float64(in.DatasetBytes) / 1_000_000_000.0
	method := strings.ToLower(strings.TrimSpace(in.Method))

	var reqVRAM float64
	switch method {
	case "full":
		// Weights + gradients + AdamW FP32 states + activations (~4x FP16 weight size)
		reqVRAM = modelGB*4.0 + 3.0
	case "qlora":
		// Quantized base weights (scaled by selected QLoRA bit-width) + LoRA adapters + activations
		reqVRAM = modelGB*QLoRAQuantizationFactor(in.Quantization) + 2.2
	default:
		// LoRA / SFT / DPO / DreamBooth: FP16/BF16 weights + adapter states + activations
		reqVRAM = modelGB*1.25 + 2.0
	}
	reqRAM := modelGB*1.15 + datasetGB*2.0 + 2.0

	res := FineTuningCompatCheck{
		Compatible:      true,
		RequiredVRAMGB:  math.Round(reqVRAM*10) / 10,
		AvailableVRAMGB: spec.VRAMGB,
		RequiredRAMGB:   math.Round(reqRAM*10) / 10,
		AvailableRAMGB:  spec.RAMGB,
		ModelSizeGB:     math.Round(modelGB*10) / 10,
	}

	if spec.Kind == "cpu" {
		if reqVRAM+datasetGB*1.5 > spec.RAMGB*0.85 || (method == "full" && modelGB > 2.5) || modelGB > 6.0 {
			res.Compatible = false
			res.Code = "cpu_too_large"
			res.Reason = fmt.Sprintf("Model (~%.1f GB) with %s requires ~%.1f GB memory, exceeding CPU runtime capacity (~%.0f GB RAM).", modelGB, strings.ToUpper(nonEmptyOr(method, "lora")), reqVRAM, spec.RAMGB)
		}
		return res
	}

	if reqVRAM > spec.VRAMGB*0.92 {
		res.Compatible = false
		res.Code = "vram_exceeded"
		res.Reason = fmt.Sprintf("Estimated memory required (~%.1f GB VRAM for %s on ~%.1f GB weights) exceeds selected accelerator capacity (%.0f GB VRAM).", reqVRAM, strings.ToUpper(nonEmptyOr(method, "lora")), modelGB, spec.VRAMGB)
		return res
	}
	if reqRAM > spec.RAMGB*0.92 {
		res.Compatible = false
		res.Code = "ram_exceeded"
		res.Reason = fmt.Sprintf("Model (~%.1f GB) and dataset (~%.2f GB) require ~%.1f GB system RAM, exceeding selected runtime RAM (%.0f GB).", modelGB, datasetGB, reqRAM, spec.RAMGB)
		return res
	}
	return res
}

// ValidateFineTuningDatasetFile strictly validates a dataset file's extension
// and leading magic bytes against the selected fine-tuning task type so invalid
// archives (e.g. .zip uploaded for Text/LLM) or disguised binaries are rejected.
func ValidateFineTuningDatasetFile(taskType string, filename string, headBytes []byte) error {
	task := strings.ToLower(strings.TrimSpace(taskType))
	if task == "" {
		task = "text"
	}
	ext := strings.ToLower(strings.TrimSpace(filepath.Ext(filename)))
	allowedByTask := map[string]map[string]bool{
		"text": {
			".jsonl": true, ".json": true, ".parquet": true, ".csv": true, ".txt": true,
		},
		"image": {
			".zip": true, ".tar": true, ".gz": true, ".parquet": true,
			".png": true, ".jpg": true, ".jpeg": true, ".webp": true,
		},
		"audio": {
			".zip": true, ".tar": true, ".gz": true, ".jsonl": true, ".parquet": true, ".csv": true,
			".wav": true, ".mp3": true, ".flac": true, ".ogg": true, ".m4a": true,
		},
		"vision": {
			".jsonl": true, ".json": true, ".zip": true, ".parquet": true,
		},
	}
	allowed, ok := allowedByTask[task]
	if !ok {
		allowed = allowedByTask["text"]
	}
	if ext == "" || !allowed[ext] {
		switch task {
		case "text":
			return fmt.Errorf("invalid dataset format %q for Text / LLM fine-tuning: only .jsonl, .json, .parquet, .csv, or .txt files are allowed (ZIP/archives are not supported for text tasks)", ext)
		case "image":
			return fmt.Errorf("invalid dataset format %q for Image / Diffusion fine-tuning: only .zip, .tar, .gz, .parquet, or image files (.png/.jpg/.webp) are allowed", ext)
		case "audio":
			return fmt.Errorf("invalid dataset format %q for Audio / Speech fine-tuning: only .zip, .tar, .gz, .jsonl, .parquet, .csv, or audio files are allowed", ext)
		case "vision":
			return fmt.Errorf("invalid dataset format %q for Vision / Multimodal fine-tuning: only .jsonl, .json, .zip, or .parquet files are allowed", ext)
		}
	}

	if len(headBytes) >= 4 {
		isZipMagic := bytes.HasPrefix(headBytes, []byte{0x50, 0x4B, 0x03, 0x04}) || bytes.HasPrefix(headBytes, []byte{0x50, 0x4B, 0x05, 0x06})
		isGzipMagic := bytes.HasPrefix(headBytes, []byte{0x1F, 0x8B})
		isPNGMagic := bytes.HasPrefix(headBytes, []byte{0x89, 0x50, 0x4E, 0x47})
		isJPGMagic := bytes.HasPrefix(headBytes, []byte{0xFF, 0xD8, 0xFF})
		isELFMagic := bytes.HasPrefix(headBytes, []byte{0x7F, 'E', 'L', 'F'})
		isPEMagic := bytes.HasPrefix(headBytes, []byte{'M', 'Z'})
		isPDFMagic := bytes.HasPrefix(headBytes, []byte{'%', 'P', 'D', 'F'})
		isParquetMagic := bytes.HasPrefix(headBytes, []byte{'P', 'A', 'R', '1'})

		if isELFMagic || isPEMagic || isPDFMagic {
			return fmt.Errorf("dataset file %q contains an unsupported binary header", filepath.Base(filename))
		}
		if ext == ".jsonl" || ext == ".json" || ext == ".csv" || ext == ".txt" {
			if isZipMagic || isGzipMagic || isPNGMagic || isJPGMagic || bytes.IndexByte(headBytes, 0x00) >= 0 {
				return fmt.Errorf("dataset file %q appears to be a binary or compressed archive, not plain %s text data", filepath.Base(filename), strings.ToUpper(strings.TrimPrefix(ext, ".")))
			}
		}
		if ext == ".zip" && !isZipMagic {
			return fmt.Errorf("dataset file %q does not have a valid ZIP archive header", filepath.Base(filename))
		}
		if ext == ".parquet" && !isParquetMagic {
			return fmt.Errorf("dataset file %q does not have a valid Parquet header", filepath.Base(filename))
		}
	}
	return nil
}

// GetAcceleratorSpeedFactor returns relative training throughput compared to
// 1x NVIDIA Tesla T4 (1.0x), accurately reflecting GPU tier and multi-GPU counts
// (e.g. Kaggle NvidiaTeslaT4 = 2x T4 = 1.85x vs 1x T4 = 1.0x).
func GetAcceleratorSpeedFactor(gpuID string) float64 {
	trimmed := strings.TrimSpace(gpuID)
	switch trimmed {
	case "NvidiaTeslaT4": // Kaggle T4 x2
		return 1.85
	case "t4-medium":
		return 1.35
	case "T4", "NVIDIA_TESLA_T4", "t4-small":
		return 1.0
	case "l4x4":
		return 7.8
	case "L4", "NVIDIA_L4", "NvidiaL4", "l4x1":
		return 2.1
	case "a10g-large":
		return 3.4
	case "A10G", "NVIDIA_A10G", "a10g-small":
		return 2.4
	case "RTX-3090":
		return 2.3
	case "RTX-4090":
		return 4.0
	case "l40sx4":
		return 15.5
	case "L40S", "l40sx1":
		return 4.2
	case "A100-40GB", "NVIDIA_TESLA_A100":
		return 5.4
	case "A100-80GB", "NVIDIA_TESLA_A100_80GB", "A100", "a100-large":
		return 6.2
	case "a100x2":
		return 12.0
	case "a100x4":
		return 23.0
	case "a100x8":
		return 44.0
	case "RTX-PRO-6000", "NVIDIA_RTX_PRO_6000":
		return 7.0
	case "H100", "H100-SXM", "H100-80GB", "NVIDIA_H100_80GB", "h100x1":
		return 12.0
	case "h100x4":
		return 45.0
	case "h100x8":
		return 85.0
	case "H200", "h200x1":
		return 14.5
	case "h200x8":
		return 105.0
	case "B200":
		return 18.0
	case "B300":
		return 24.0
	case "TPU_V2_8":
		return 3.6
	case "TPU_V5E_1":
		return 3.0
	case "TPU_V6E_1":
		return 4.2
	case "TpuV5E8":
		return 10.5
	case "TpuV6E8":
		return 16.0
	case "cpu-upgrade":
		return 0.12
	case "CPU", "cpu-basic":
		return 0.07
	}

	upper := strings.ToUpper(trimmed)
	switch {
	case strings.Contains(upper, "B300"):
		return 24.0
	case strings.Contains(upper, "B200"):
		return 18.0
	case strings.Contains(upper, "H200"):
		return 14.5
	case strings.Contains(upper, "H100"):
		return 12.0
	case strings.Contains(upper, "A100"):
		return 5.8
	case strings.Contains(upper, "RTX-PRO-6000"), strings.Contains(upper, "RTX_PRO_6000"):
		return 7.0
	case strings.Contains(upper, "L40S"):
		return 4.2
	case strings.Contains(upper, "4090"):
		return 4.0
	case strings.Contains(upper, "TPU"):
		return 3.5
	case strings.Contains(upper, "A10G"), strings.Contains(upper, "3090"):
		return 2.4
	case strings.Contains(upper, "L4"):
		return 2.1
	case strings.Contains(upper, "T4"):
		return 1.0
	case strings.Contains(upper, "CPU"):
		return 0.07
	default:
		return 1.0
	}
}

// FineTuningEstimateInput holds all real workload factors affecting training time.
type FineTuningEstimateInput struct {
	GPU            string
	RamTier        string
	TaskType       string
	Method         string
	Quantization   string
	Epochs         int
	DatasetBytes   int64
	DatasetSamples int64 // exact non-empty lines/records when known
	ModelName      string
	ModelParamHint string
	ModelFileBytes int64
}

// EstimateFineTuningDurationRealistic computes realistic training duration in
// seconds from model parameter count & file size (GB), dataset sample count &
// bytes, optimizer gradient steps, Full-FT checkpoint/export I/O, VRAM spill
// pressure, method overhead, task modality, and hardware throughput.
func EstimateFineTuningDurationRealistic(in FineTuningEstimateInput) int64 {
	epochs := in.Epochs
	if epochs < 1 {
		epochs = 1
	}
	datasetBytes := in.DatasetBytes
	if datasetBytes < 0 {
		datasetBytes = 0
	}

	modelB, modelGB := estimateModelSizeGB(in)
	speed := GetAcceleratorSpeedFactor(in.GPU)
	if speed <= 0 {
		speed = 1.0
	}

	taskFactor := 1.0
	switch strings.ToLower(strings.TrimSpace(in.TaskType)) {
	case "image":
		taskFactor = 1.85
	case "vision":
		taskFactor = 1.60
	case "audio":
		taskFactor = 1.35
	default:
		taskFactor = 1.0
	}

	method := strings.ToLower(strings.TrimSpace(in.Method))
	methodFactor := 1.0
	switch method {
	case "full":
		methodFactor = 3.2
	case "dpo":
		methodFactor = 1.65
	case "dreambooth":
		methodFactor = 1.45
	case "sft":
		methodFactor = 1.15
	case "qlora":
		methodFactor = 1.15
	default:
		methodFactor = 1.0
	}

	samples := in.DatasetSamples
	if samples <= 0 {
		if datasetBytes > 0 {
			samples = datasetBytes / 180
			if samples < 16 {
				samples = 16
			}
		} else {
			samples = 100
		}
	}

	// Effective tokens per epoch: account for tokenization, chat templates, and padding
	tokensFromBytes := float64(datasetBytes) / 4.0
	tokensFromSamples := float64(samples) * 32.0
	if strings.EqualFold(in.TaskType, "image") {
		tokensFromSamples = float64(samples) * 320.0
	}
	estTokensPerEpoch := math.Max(tokensFromBytes, tokensFromSamples)

	// Setup/download/model load time scales linearly with actual weight size (GB):
	// downloading & deserializing a 94 GB safetensors model takes ~22+ min, not 3 min.
	setupSec := 55.0 + modelGB*14.0
	if setupSec > 21600.0 {
		setupSec = 21600.0
	}

	// Memory spill penalty when required VRAM exceeds selected accelerator capacity
	compat := CheckFineTuningHardwareCompat(in)
	spillPenalty := 1.0
	if !compat.Compatible && compat.AvailableVRAMGB > 0 && compat.RequiredVRAMGB > compat.AvailableVRAMGB {
		ratio := compat.RequiredVRAMGB / compat.AvailableVRAMGB
		if ratio > 1.0 {
			spillPenalty = math.Min(12.0, math.Pow(ratio, 0.85))
		}
	}

	// Token FLOPs compute time: ~650 tok/s for 7B LoRA on 1x T4
	tokenComputePerEpoch := (estTokensPerEpoch / 650.0) * (modelB / 7.0) * methodFactor * taskFactor * spillPenalty / speed

	// Per-optimizer-step kernel, gradient, and AdamW parameter-update overhead
	const effectiveBatchSize = 16.0
	stepsPerEpoch := math.Ceil(float64(samples) / effectiveBatchSize)
	perStepSec := (0.28 + 0.09*modelB) * taskFactor / math.Sqrt(speed)
	if method == "full" {
		// Full fine-tuning updates all model parameters + FP32 optimizer states every step
		perStepSec += (modelGB * 0.45 * spillPenalty) / math.Max(0.5, speed)
	}
	stepOverheadPerEpoch := stepsPerEpoch * perStepSec

	// Continuous checkpointing + epoch checkpoint serialization I/O
	checkpointPerEpochSec := 10.0 + modelB*0.4
	exportSec := 25.0 + modelB*2.5
	if method == "full" {
		// Full fine-tuning writes & packages full model weights every epoch + final .zip archive
		checkpointPerEpochSec = 20.0 + modelGB*8.0
		exportSec = 40.0 + modelGB*16.0
	}
	if exportSec > 18000.0 {
		exportSec = 18000.0
	}

	total := int64(math.Round(setupSec + float64(epochs)*(tokenComputePerEpoch+stepOverheadPerEpoch+checkpointPerEpochSec) + exportSec))
	if total < 45 {
		total = 45
	}
	if total > 14*86400 {
		total = 14 * 86400
	}
	return total
}

// EstimateFineTuningDurationSeconds keeps backward compatibility for callers that
// only pass GPU, task type, epochs, and dataset byte size.
func EstimateFineTuningDurationSeconds(gpuID string, taskType string, epochs int, datasetBytes int64) int64 {
	return EstimateFineTuningDurationRealistic(FineTuningEstimateInput{
		GPU:          gpuID,
		TaskType:     taskType,
		Method:       "lora",
		Epochs:       epochs,
		DatasetBytes: datasetBytes,
	})
}

func parseSavedPricingConfig(rawConfig string, basePrice float64) map[string]FineTuningGPUOption {
	saved := make(map[string]FineTuningGPUOption)
	if strings.HasPrefix(rawConfig, "[") {
		var rows []FineTuningGPUOption
		if common.UnmarshalJsonStr(rawConfig, &rows) == nil {
			for _, r := range rows {
				if r.ID == "" {
					continue
				}
				if r.PricePerSecond < 0 {
					r.PricePerSecond = 0
				}
				saved[r.ID] = r
			}
		}
		return saved
	}
	if strings.HasPrefix(rawConfig, "{") {
		var legacy map[string]float64
		if common.UnmarshalJsonStr(rawConfig, &legacy) == nil {
			for id, mult := range legacy {
				saved[id] = FineTuningGPUOption{
					ID:             id,
					Label:          id,
					PricePerSecond: basePrice * mult,
					Enabled:        mult >= 0,
				}
			}
		}
	}
	return saved
}

func parseSavedFineTuningPricing() map[string]FineTuningGPUOption {
	common.OptionMapRWMutex.RLock()
	rawPrice := strings.TrimSpace(common.OptionMap["model_deployment.fine_tuning.price_per_second"])
	rawConfig := strings.TrimSpace(common.OptionMap["model_deployment.fine_tuning.gpu_multipliers"])
	common.OptionMapRWMutex.RUnlock()

	basePrice := 0.002
	if rawPrice != "" {
		if parsed, err := strconv.ParseFloat(rawPrice, 64); err == nil && parsed >= 0 {
			basePrice = parsed
		}
	}
	return parseSavedPricingConfig(rawConfig, basePrice)
}

func GetFineTuningServingColabRAMMultipliers() (normalMult float64, highMult float64) {
	normalMult = 1.0
	highMult = 1.5
	common.OptionMapRWMutex.RLock()
	rawNormal := strings.TrimSpace(common.OptionMap["model_deployment.fine_tuning.serving_colab_normal_ram_multiplier"])
	rawHigh := strings.TrimSpace(common.OptionMap["model_deployment.fine_tuning.serving_colab_high_ram_multiplier"])
	common.OptionMapRWMutex.RUnlock()
	if rawNormal != "" {
		if v, err := strconv.ParseFloat(rawNormal, 64); err == nil && v > 0 {
			normalMult = v
		}
	}
	if rawHigh != "" {
		if v, err := strconv.ParseFloat(rawHigh, 64); err == nil && v > 0 {
			highMult = v
		}
	}
	return normalMult, highMult
}

func GetFineTuningServingGPUOptions() []FineTuningGPUOption {
	common.OptionMapRWMutex.RLock()
	rawConfig := strings.TrimSpace(common.OptionMap["model_deployment.fine_tuning.serving_gpu_multipliers"])
	common.OptionMapRWMutex.RUnlock()

	saved := parseSavedPricingConfig(rawConfig, 0.0015)
	available, hasEnabledProviders := collectAccountAvailableHardware()

	if hasEnabledProviders {
		for i := range available {
			if cfg, ok := saved[available[i].ID]; ok {
				available[i].PricePerSecond = cfg.PricePerSecond
				available[i].Enabled = cfg.Enabled
			}
		}
		return available
	}

	out := make([]FineTuningGPUOption, len(defaultFineTuningGPUs))
	copy(out, defaultFineTuningGPUs)
	for i := range out {
		if cfg, ok := saved[out[i].ID]; ok {
			out[i].PricePerSecond = cfg.PricePerSecond
			out[i].Enabled = cfg.Enabled
		}
	}
	return out
}

func FindEnabledFineTuningServingGPU(gpuID string) (*FineTuningGPUOption, bool) {
	trimmed := strings.TrimSpace(gpuID)
	for _, opt := range GetFineTuningServingGPUOptions() {
		if opt.ID == trimmed && opt.Enabled {
			copied := opt
			return &copied, true
		}
	}
	return nil, false
}

func ResolveFineTuningServingPricePerSecond(gpuID string, ramTier string) float64 {
	trimmed := strings.TrimSpace(gpuID)
	normalMult, highMult := GetFineTuningServingColabRAMMultipliers()
	ramMult := 1.0
	for _, opt := range GetFineTuningServingGPUOptions() {
		if opt.ID == trimmed {
			if opt.SupportsColabRAM {
				if strings.EqualFold(strings.TrimSpace(ramTier), "high") {
					ramMult = highMult
				} else {
					ramMult = normalMult
				}
			}
			if opt.PricePerSecond < 0 {
				return 0
			}
			return opt.PricePerSecond * ramMult
		}
	}
	return 0.0015
}

func GetFineTuningGPUOptions() []FineTuningGPUOption {
	saved := parseSavedFineTuningPricing()
	available, hasEnabledProviders := collectAccountAvailableHardware()

	if hasEnabledProviders {
		for i := range available {
			if cfg, ok := saved[available[i].ID]; ok {
				available[i].PricePerSecond = cfg.PricePerSecond
				available[i].Enabled = cfg.Enabled
			}
		}
		return available
	}

	// Fallback when no deployment provider is configured yet (e.g. unit tests / fresh install)
	out := make([]FineTuningGPUOption, len(defaultFineTuningGPUs))
	copy(out, defaultFineTuningGPUs)
	for i := range out {
		if cfg, ok := saved[out[i].ID]; ok {
			out[i].PricePerSecond = cfg.PricePerSecond
			out[i].Enabled = cfg.Enabled
		}
	}
	return out
}

func GetFineTuningPricing() (float64, map[string]float64) {
	opts := GetFineTuningGPUOptions()
	mults := make(map[string]float64, len(opts))
	for _, o := range opts {
		if o.Enabled {
			mults[o.ID] = o.PricePerSecond
		}
	}
	return 1.0, mults
}

func FindEnabledFineTuningGPU(gpuID string) (*FineTuningGPUOption, bool) {
	trimmed := strings.TrimSpace(gpuID)
	for _, opt := range GetFineTuningGPUOptions() {
		if opt.ID == trimmed && opt.Enabled {
			copied := opt
			return &copied, true
		}
	}
	return nil, false
}

func EstimateFineTuningCostUSD(gpu string, durationSeconds int64) float64 {
	return EstimateFineTuningCostUSDWithRAM(gpu, "normal", durationSeconds)
}

func EstimateFineTuningCostUSDWithRAM(gpu string, ramTier string, durationSeconds int64) float64 {
	if durationSeconds <= 0 {
		return 0
	}
	trimmed := strings.TrimSpace(gpu)
	normalMult, highMult := GetColabRAMMultipliers()
	ramMult := 1.0
	for _, opt := range GetFineTuningGPUOptions() {
		if opt.ID == trimmed {
			if opt.SupportsColabRAM {
				if strings.EqualFold(strings.TrimSpace(ramTier), "high") {
					ramMult = highMult
				} else {
					ramMult = normalMult
				}
			}
			return opt.PricePerSecond * ramMult * float64(durationSeconds)
		}
	}
	return 0.002 * float64(durationSeconds)
}

func sanitizeFineTuningSegment(s string) string {
	clean := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, strings.TrimSpace(s))
	if clean == "" {
		return "unknown"
	}
	return clean
}

func FineTuningStorageDirForUser(userID int, jobID string) string {
	return filepath.Join(FineTuningRootStorageDir(), strconv.Itoa(userID), sanitizeFineTuningSegment(jobID))
}

func FineTuningStagedDirForUser(userID int, uploadID string) string {
	return filepath.Join(FineTuningRootStorageDir(), strconv.Itoa(userID), "_staged", sanitizeFineTuningSegment(uploadID))
}

func FineTuningStorageDir(jobID string) string {
	return filepath.Join(FineTuningRootStorageDir(), "0", sanitizeFineTuningSegment(jobID))
}

func CleanupFineTuningStorageForUser(userID int, jobID string) {
	CancelFineTuningJobRunner(jobID)
	_ = os.RemoveAll(FineTuningStorageDirForUser(userID, jobID))
	// Clean legacy data/fine_tuning path if present from earlier versions
	_ = os.RemoveAll(filepath.Join("data", "fine_tuning", sanitizeFineTuningSegment(jobID)))
}

func CleanupFineTuningStorage(jobID string) {
	CleanupFineTuningStorageForUser(0, jobID)
}

func GenerateFineTuningJobID() string {
	buf := make([]byte, 12)
	_, _ = rand.Read(buf)
	return "ftjob-" + hex.EncodeToString(buf)
}

func FormatOpenAIFineTuningJob(job *model.FineTuningJob) map[string]any {
	var hyper map[string]any
	if strings.TrimSpace(job.Hyperparameters) != "" {
		_ = common.UnmarshalJsonStr(job.Hyperparameters, &hyper)
	}
	if hyper == nil {
		hyper = map[string]any{}
	}
	var fineTunedModel any
	if strings.TrimSpace(job.FineTunedModel) != "" {
		fineTunedModel = job.FineTunedModel
	}
	var validationFile any
	if strings.TrimSpace(job.ValidationFile) != "" {
		validationFile = job.ValidationFile
	}
	var finishedAt any
	if job.FinishedAt > 0 {
		finishedAt = job.FinishedAt
	}
	var errObj any
	if strings.TrimSpace(job.Error) != "" {
		errObj = map[string]any{
			"code":    "fine_tuning_error",
			"message": job.Error,
		}
	}
	return map[string]any{
		"object":           "fine_tuning.job",
		"id":               job.JobID,
		"model":            job.Model,
		"created_at":       job.CreatedAt,
		"finished_at":      finishedAt,
		"fine_tuned_model": fineTunedModel,
		"organization_id":  "org-newapi",
		"result_files":     []string{},
		"status":           job.Status,
		"validation_file":  validationFile,
		"training_file":    job.TrainingFile,
		"hyperparameters":  hyper,
		"trained_tokens":   job.TrainedTokens,
		"error":            errObj,
	}
}

func ApplyFineTuningWorkerCallback(job *model.FineTuningJob, status, fineTunedModel, errMsg string, trainedTokens int64) error {
	now := time.Now().Unix()
	updates := map[string]any{
		"status":     status,
		"updated_at": now,
	}
	if strings.TrimSpace(fineTunedModel) != "" {
		updates["fine_tuned_model"] = strings.TrimSpace(fineTunedModel)
	}
	if strings.TrimSpace(errMsg) != "" {
		updates["error"] = strings.TrimSpace(errMsg)
	}
	if trainedTokens > 0 {
		updates["trained_tokens"] = trainedTokens
	}
	if status == "succeeded" || status == "failed" || status == "cancelled" {
		updates["finished_at"] = now
	}
	return model.UpdateFineTuningJobStatus(job.JobID, updates)
}
