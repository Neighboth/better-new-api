package service

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
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

// acceleratorSpeedFactor returns relative training throughput used to estimate
// realistic training duration alongside dataset size and epochs.
func acceleratorSpeedFactor(gpuID string) float64 {
	upper := strings.ToUpper(strings.TrimSpace(gpuID))
	switch {
	case strings.Contains(upper, "B300"), strings.Contains(upper, "B200"), strings.Contains(upper, "H200"), strings.Contains(upper, "H100"):
		return 5.5
	case strings.Contains(upper, "A100"), strings.Contains(upper, "RTX-PRO-6000"), strings.Contains(upper, "RTX_PRO_6000"):
		return 3.5
	case strings.Contains(upper, "L40S"), strings.Contains(upper, "4090"):
		return 2.6
	case strings.Contains(upper, "TPU"):
		return 2.4
	case strings.Contains(upper, "A10G"), strings.Contains(upper, "3090"):
		return 2.0
	case strings.Contains(upper, "L4"):
		return 1.6
	case strings.Contains(upper, "T4"):
		return 1.0
	case strings.Contains(upper, "CPU"):
		return 0.35
	default:
		return 1.0
	}
}

// EstimateFineTuningDurationSeconds computes estimated training duration in
// seconds accounting for dataset file size, epochs, task modality, and
// accelerator throughput.
func EstimateFineTuningDurationSeconds(gpuID string, taskType string, epochs int, datasetBytes int64) int64 {
	if epochs < 1 {
		epochs = 1
	}
	if datasetBytes < 0 {
		datasetBytes = 0
	}
	datasetMB := float64(datasetBytes) / (1024.0 * 1024.0)
	taskFactor := 1.0
	switch strings.ToLower(strings.TrimSpace(taskType)) {
	case "image":
		taskFactor = 1.8
	case "vision":
		taskFactor = 1.6
	case "audio":
		taskFactor = 1.3
	default:
		taskFactor = 1.0
	}
	speed := acceleratorSpeedFactor(gpuID)
	basePerEpochSec := 45.0 + datasetMB*90.0
	est := int64((basePerEpochSec * taskFactor / speed) * float64(epochs))
	if est < 30 {
		est = 30
	}
	if est > 7*86400 {
		est = 7 * 86400
	}
	return est
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
	return filepath.Join("uploads", "fine_tuning", strconv.Itoa(userID), sanitizeFineTuningSegment(jobID))
}

func FineTuningStagedDirForUser(userID int, uploadID string) string {
	return filepath.Join("uploads", "fine_tuning", strconv.Itoa(userID), "_staged", sanitizeFineTuningSegment(uploadID))
}

func FineTuningStorageDir(jobID string) string {
	return filepath.Join("uploads", "fine_tuning", "0", sanitizeFineTuningSegment(jobID))
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
