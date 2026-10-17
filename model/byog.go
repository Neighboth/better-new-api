package model

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"

	"gorm.io/gorm"
)

// BYOG Settings
type ByogSetting struct {
	Id                   int     `json:"id" gorm:"primaryKey"`
	Enabled              bool    `json:"enabled" gorm:"default:true"`
	Tier1ProviderPrice   float64 `json:"tier1_provider_price" gorm:"default:0.40"` // Turbo GPU 40+ TPS, TTFT < 1.0s (per 1M)
	Tier1CustomerPrice   float64 `json:"tier1_customer_price" gorm:"default:0.80"`
	Tier2ProviderPrice   float64 `json:"tier2_provider_price" gorm:"default:0.20"` // Standard 15-39 TPS, TTFT < 2.5s (per 1M)
	Tier2CustomerPrice   float64 `json:"tier2_customer_price" gorm:"default:0.40"`
	Tier3ProviderPrice   float64 `json:"tier3_provider_price" gorm:"default:0.05"` // Eco 3-14 TPS (per 1M)
	Tier3CustomerPrice   float64 `json:"tier3_customer_price" gorm:"default:0.10"`
	MaxColdStartTimeout  int     `json:"max_cold_start_timeout" gorm:"default:120"` // Max wait seconds for cold provisioning
	NightOwlSurgeEnabled bool    `json:"night_owl_surge_enabled" gorm:"default:true"`
	WeekendCrunchEnabled bool    `json:"weekend_crunch_enabled" gorm:"default:true"`
	HardwareBackends     string  `json:"hardware_backends" gorm:"type:text"` // JSON object of toggled backends
	CreatedAt            int64   `json:"created_at" gorm:"bigint"`
	UpdatedAt            int64   `json:"updated_at" gorm:"bigint"`
}

// Reward Store Item
type ByogRewardItem struct {
	Id             int    `json:"id" gorm:"primaryKey"`
	Title          string `json:"title" gorm:"type:varchar(255);not null"`
	Description    string `json:"description" gorm:"type:text"`
	Category       string `json:"category" gorm:"type:varchar(64);default:'digital'"`
	RequiredTokens int64  `json:"required_tokens" gorm:"not null"` // e.g. 50,000,000 tokens
	Stock          int    `json:"stock" gorm:"default:100"`
	IsActive       bool   `json:"is_active" gorm:"default:true"`
	CreatedAt      int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt      int64  `json:"updated_at" gorm:"bigint"`
}

// User Reward Claim Record
type ByogRewardClaim struct {
	Id              int    `json:"id" gorm:"primaryKey"`
	UserId          int    `json:"user_id" gorm:"index"`
	RewardItemId    int    `json:"reward_item_id" gorm:"index"`
	RewardTitle     string `json:"reward_title" gorm:"type:varchar(255)"`
	TokensBurned    int64  `json:"tokens_burned"`
	Status          string `json:"status" gorm:"type:varchar(32);default:'pending';index"` // pending, approved, rejected, shipped
	AdminNotes      string `json:"admin_notes" gorm:"type:text"`
	TicketId        int    `json:"ticket_id" gorm:"index"` // Link to support ticket
	VerificationLog string `json:"verification_log" gorm:"type:text"`
	CreatedAt       int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt       int64  `json:"updated_at" gorm:"bigint"`

	// Virtual fields
	UserName  string `json:"user_name,omitempty" gorm:"-"`
	UserEmail string `json:"user_email,omitempty" gorm:"-"`
}

// Rule-Based Hardware Assignment Matrix
type ByogAssignmentRule struct {
	Id                    int    `json:"id" gorm:"primaryKey"`
	Name                  string `json:"name" gorm:"type:varchar(255);not null"`
	DeviceClass           string `json:"device_class" gorm:"type:varchar(64);default:'all'"` // desktop_server, laptop, mobile_tablet, all
	MinVramMB             int    `json:"min_vram_mb" gorm:"default:0"`
	MinPhysicalRamMB      int    `json:"min_physical_ram_mb" gorm:"default:0"` // Strictly Physical RAM, excluding swap/zram
	HardwareFamily        string `json:"hardware_family" gorm:"type:varchar(64);default:'all'"` // cuda, rocm, intel_arc, metal, adreno_hexagon, mali, cpu, all
	MinCores              int    `json:"min_cores" gorm:"default:0"`
	VectorInstructionSet  string `json:"vector_instruction_set" gorm:"type:varchar(64);default:'all'"` // avx2, avx512, neon, dotprod, all
	TargetModels          string `json:"target_models" gorm:"type:text"` // JSON array of model IDs
	TargetLoras           string `json:"target_loras" gorm:"type:text"` // JSON array of LoRA IDs
	Priority              int    `json:"priority" gorm:"default:10"`
	IsEnabled             bool   `json:"is_enabled" gorm:"default:true"`
	CreatedAt             int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt             int64  `json:"updated_at" gorm:"bigint"`
}

// Base Model Definition
type ByogModel struct {
	Id                     int     `json:"id" gorm:"primaryKey"`
	ModelId                string  `json:"model_id" gorm:"type:varchar(128);uniqueIndex;not null"`
	ModelType              string  `json:"model_type" gorm:"type:varchar(64);default:'llm'"` // llm, embedding, reranker, vision, image_gen, video, 3d
	Vendor                 string  `json:"vendor" gorm:"type:varchar(64);default:'community'"`
	Description            string  `json:"description" gorm:"type:text"`
	ParamSize              string  `json:"param_size" gorm:"type:varchar(32);default:'7B'"` // 0.5B, 1B, 1.5B, 3B, 7B, 13B, 70B
	PricingTier            int     `json:"pricing_tier" gorm:"default:2"` // 1: Turbo, 2: Standard, 3: Eco
	DownloadUrl            string  `json:"download_url" gorm:"type:varchar(1024);not null"`
	FileSize               int64   `json:"file_size" gorm:"default:0"`
	ChunkSize              int     `json:"chunk_size" gorm:"default:4194304"` // 4MB default chunk size
	ChunkManifest          string  `json:"chunk_manifest" gorm:"type:longtext"` // JSON array of SHA-256 chunk hashes
	ModalitiesInput        string  `json:"modalities_input" gorm:"type:varchar(255);default:'text'"` // JSON array: text, image, video, audio, document, 3d
	ModalitiesOutput       string  `json:"modalities_output" gorm:"type:varchar(255);default:'text'"` // JSON array: text, image, video, audio, 3d
	VisionMmprojUrl        string  `json:"vision_mmproj_url" gorm:"type:varchar(1024)"`
	
	// Anti-Cheat Exam / Canary Settings
	AntiCheatLogicPrompt   string  `json:"anti_cheat_logic_prompt" gorm:"type:text"`
	AntiCheatLogicTemp     float64 `json:"anti_cheat_logic_temp" gorm:"default:0"`
	AntiCheatLogicSeed     int64   `json:"anti_cheat_logic_seed" gorm:"default:42"`
	AntiCheatLogicExpected string  `json:"anti_cheat_logic_expected" gorm:"type:text"`
	AntiCheatLogicLogprob  float64 `json:"anti_cheat_logic_logprob" gorm:"default:0"`
	AntiCheatLogicTolerance float64 `json:"anti_cheat_logic_tolerance" gorm:"default:0.005"`

	AntiCheatSigPrompt     string  `json:"anti_cheat_sig_prompt" gorm:"type:text"`
	AntiCheatSigTemp       float64 `json:"anti_cheat_sig_temp" gorm:"default:0"`
	AntiCheatSigSeed       int64   `json:"anti_cheat_sig_seed" gorm:"default:8472910482910482"`
	AntiCheatSigExpected   string  `json:"anti_cheat_sig_expected" gorm:"type:text"`
	AntiCheatSigLogprob    float64 `json:"anti_cheat_sig_logprob" gorm:"default:-0.0124"`
	AntiCheatSigTolerance  float64 `json:"anti_cheat_sig_tolerance" gorm:"default:0.0020"`

	IsEnabled              bool    `json:"is_enabled" gorm:"default:true"`
	CreatedAt              int64   `json:"created_at" gorm:"bigint"`
	UpdatedAt              int64   `json:"updated_at" gorm:"bigint"`
}

// Multi-LoRA Adapter Definition under Base Model
type ByogModelLora struct {
	Id                 int     `json:"id" gorm:"primaryKey"`
	BaseModelId        string  `json:"base_model_id" gorm:"type:varchar(128);index;not null"`
	LoraId             string  `json:"lora_id" gorm:"type:varchar(128);uniqueIndex;not null"`
	DownloadUrl        string  `json:"download_url" gorm:"type:varchar(1024);not null"`
	FileSize           int64   `json:"file_size" gorm:"default:0"`
	ChunkManifest      string  `json:"chunk_manifest" gorm:"type:text"`
	ModalityTags       string  `json:"modality_tags" gorm:"type:varchar(255)"` // JSON array
	AntiCheatPrompt    string  `json:"anti_cheat_prompt" gorm:"type:text"`
	AntiCheatSeed      int64   `json:"anti_cheat_seed" gorm:"default:42"`
	AntiCheatExpected  string  `json:"anti_cheat_expected" gorm:"type:text"`
	AntiCheatLogprob   float64 `json:"anti_cheat_logprob" gorm:"default:0"`
	AntiCheatTolerance float64 `json:"anti_cheat_tolerance" gorm:"default:0.005"`
	IsEnabled          bool    `json:"is_enabled" gorm:"default:true"`
	CreatedAt          int64   `json:"created_at" gorm:"bigint"`
	UpdatedAt          int64   `json:"updated_at" gorm:"bigint"`
}

// Node Record (Desktop & Mobile)
type ByogNode struct {
	Id                    int     `json:"id" gorm:"primaryKey"`
	NodeId                string  `json:"node_id" gorm:"type:varchar(128);uniqueIndex;not null"`
	UserId                int     `json:"user_id" gorm:"index;not null"`
	PairingCode           string  `json:"pairing_code" gorm:"type:varchar(64);index"` // e.g. 9K4L-28XM-73PQ-51ZR
	DeviceName            string  `json:"device_name" gorm:"type:varchar(128);not null"`
	DeviceType            string  `json:"device_type" gorm:"type:varchar(32);default:'desktop';index"` // desktop, laptop, mobile, tablet
	OS                    string  `json:"os" gorm:"type:varchar(32)"` // windows, linux, darwin, android, ios
	Arch                  string  `json:"arch" gorm:"type:varchar(32)"` // amd64, arm64
	HWID                  string  `json:"hwid" gorm:"type:varchar(255);index;not null"` // Salted cryptographic HWID
	CpuModel              string  `json:"cpu_model" gorm:"type:varchar(255)"`
	GpuModel              string  `json:"gpu_model" gorm:"type:varchar(255)"`
	NpuModel              string  `json:"npu_model" gorm:"type:varchar(255)"`
	PhysicalRamMB         int     `json:"physical_ram_mb" gorm:"default:0"` // Strictly Physical RAM
	SwapRamMB             int     `json:"swap_ram_mb" gorm:"default:0"` // Virtual RAM / ZRAM
	VramMB                int     `json:"vram_mb" gorm:"default:0"`
	SharedVramMB          int     `json:"shared_vram_mb" gorm:"default:0"`
	TeeSupported          bool    `json:"tee_supported" gorm:"default:false"`
	TeeType               string  `json:"tee_type" gorm:"type:varchar(64);default:'none'"` // sgx, sev, trustzone, none
	BandwidthDownMbps     float64 `json:"bandwidth_down_mbps" gorm:"default:0"`
	BandwidthUpMbps       float64 `json:"bandwidth_up_mbps" gorm:"default:0"`
	PingMs                int     `json:"ping_ms" gorm:"default:50"`
	NetworkType           string  `json:"network_type" gorm:"type:varchar(32);default:'ethernet'"` // ethernet, wifi, cellular
	IpAddress             string  `json:"ip_address" gorm:"type:varchar(64)"`
	IpType                string  `json:"ip_type" gorm:"type:varchar(32);default:'residential'"` // residential, datacenter
	Country               string  `json:"country" gorm:"type:varchar(64);default:'Unknown'"`
	City                  string  `json:"city" gorm:"type:varchar(64);default:'Unknown'"`
	Latitude              float64 `json:"latitude" gorm:"default:0"`
	Longitude             float64 `json:"longitude" gorm:"default:0"`
	Status                string  `json:"status" gorm:"type:varchar(32);default:'online';index"` // online, offline, paused, gamer_mode, throttled, banned, updating
	OperatingMode         string  `json:"operating_mode" gorm:"type:varchar(32);default:'mode_b'"` // mode_b (community pool), mode_a (private)
	PowerProfile          string  `json:"power_profile" gorm:"type:varchar(32);default:'full_power'"` // full_power, half_power, active_hours, dual_schedule
	ScheduleConfig        string  `json:"schedule_config" gorm:"type:text"` // JSON object for hourly power distribution
	AllocatedDiskGB       int     `json:"allocated_disk_gb" gorm:"default:40"`
	AllocatedVramGB       int     `json:"allocated_vram_gb" gorm:"default:6"`
	AllocatedRamGB        int     `json:"allocated_ram_gb" gorm:"default:8"`
	AllocatedCores        int     `json:"allocated_cores" gorm:"default:4"`

	// Mobile specific settings & defaults
	WifiOnlyDownload      bool    `json:"wifi_only_download" gorm:"default:true"` // DEFAULT TRUE
	MonthlyDataLimitMB    int64   `json:"monthly_data_limit_mb" gorm:"default:0"` // 0 = UNLIMITED (DEFAULT)
	MonthlyDataUsedMB     int64   `json:"monthly_data_used_mb" gorm:"default:0"`
	RunOnlyWhileCharging  bool    `json:"run_only_while_charging" gorm:"default:false"` // DEFAULT FALSE
	BatteryCutoffEnabled  bool    `json:"battery_cutoff_enabled" gorm:"default:false"` // DEFAULT FALSE
	BatteryCutoffPercent  int     `json:"battery_cutoff_percent" gorm:"default:20"` // 5-50% range
	MaxBatteryTemp        float64 `json:"max_battery_temp" gorm:"default:50.0"` // DEFAULT 50°C (Active if sensor present)
	BatteryTempSupported  bool    `json:"battery_temp_supported" gorm:"default:true"`

	// Live Telemetry
	CpuTemp               float64 `json:"cpu_temp" gorm:"default:45.0"`
	GpuTemp               float64 `json:"gpu_temp" gorm:"default:50.0"`
	BatteryTemp           float64 `json:"battery_temp" gorm:"default:32.0"`
	FanSpeed              int     `json:"fan_speed" gorm:"default:1500"`
	BatteryPercent        int     `json:"battery_percent" gorm:"default:100"`
	IsCharging            bool    `json:"is_charging" gorm:"default:true"`
	CurrentTps            float64 `json:"current_tps" gorm:"default:0"`
	CurrentTtftMs         int     `json:"current_ttft_ms" gorm:"default:0"`
	TotalTokensProcessed  int64   `json:"total_tokens_processed" gorm:"default:0"`
	TotalEarnedCredits    float64 `json:"total_earned_credits" gorm:"default:0"`
	UptimeSeconds         int64   `json:"uptime_seconds" gorm:"default:0"`
	ReputationScore       int     `json:"reputation_score" gorm:"default:50"` // 0 - 100
	ActiveMultipliers     string  `json:"active_multipliers" gorm:"type:text"` // JSON array of active multiplier keys
	InstalledModels       string  `json:"installed_models" gorm:"type:text"` // JSON array of local models/chunks
	DownloadQueue         string  `json:"download_queue" gorm:"type:text"` // JSON array of models to download
	LastHeartbeatAt       int64   `json:"last_heartbeat_at" gorm:"bigint"`
	CreatedAt             int64   `json:"created_at" gorm:"bigint"`
	UpdatedAt             int64   `json:"updated_at" gorm:"bigint"`

	// Virtual fields
	UserName              string  `json:"user_name,omitempty" gorm:"-"`
	UserEmail             string  `json:"user_email,omitempty" gorm:"-"`
}

// Banned Node Record
type ByogNodeBan struct {
	Id               int     `json:"id" gorm:"primaryKey"`
	NodeId           string  `json:"node_id" gorm:"type:varchar(128);index;not null"`
	UserId           int     `json:"user_id" gorm:"index;not null"`
	HWID             string  `json:"hwid" gorm:"type:varchar(255);index;not null"`
	BanReason        string  `json:"ban_reason" gorm:"type:varchar(255);not null"`
	ViolationCode    string  `json:"violation_code" gorm:"type:varchar(64);not null"` // e.g. #SEC-LOGPROB-492
	TestId           string  `json:"test_id" gorm:"type:varchar(64)"`
	TestPrompt       string  `json:"test_prompt" gorm:"type:text"`
	ExpectedLogprob  float64 `json:"expected_logprob"`
	ActualLogprob    float64 `json:"actual_logprob"`
	Tolerance        float64 `json:"tolerance"`
	DetailedLogs     string  `json:"detailed_logs" gorm:"type:text"`
	IsActive         bool    `json:"is_active" gorm:"default:true;index"`
	BannedAt         int64   `json:"banned_at" gorm:"bigint"`
	AppealedAt       int64   `json:"appealed_at" gorm:"bigint;default:0"`
	AppealTicketId   int     `json:"appeal_ticket_id" gorm:"default:0"`

	// Virtual fields
	UserName         string  `json:"user_name,omitempty" gorm:"-"`
	DeviceName       string  `json:"device_name,omitempty" gorm:"-"`
}

// Crash Report & Telemetry
type ByogCrashReport struct {
	Id             int    `json:"id" gorm:"primaryKey"`
	NodeId         string `json:"node_id" gorm:"type:varchar(128);index;not null"`
	UserId         int    `json:"user_id" gorm:"index"`
	ErrorCode      string `json:"error_code" gorm:"type:varchar(64);not null"`
	ErrorMessage   string `json:"error_message" gorm:"type:text;not null"`
	StackTrace     string `json:"stack_trace" gorm:"type:text"`
	MemoryUsageMB  int    `json:"memory_usage_mb"`
	ModelId        string `json:"model_id" gorm:"type:varchar(128)"`
	Recommendation string `json:"recommendation" gorm:"type:text"`
	CreatedAt      int64  `json:"created_at" gorm:"bigint"`
}

// P2P Chunk Tracker Registry
type ByogP2PTracker struct {
	Id             int    `json:"id" gorm:"primaryKey"`
	ModelId        string `json:"model_id" gorm:"type:varchar(128);index:idx_model_chunk"`
	ChunkIndex     int    `json:"chunk_index" gorm:"index:idx_model_chunk"`
	NodeId         string `json:"node_id" gorm:"type:varchar(128);index"`
	LastAnnounceAt int64  `json:"last_announce_at" gorm:"bigint"`
}

// Helpers & CRUD operations for BYOG

func GetByogSetting() (*ByogSetting, error) {
	var setting ByogSetting
	err := DB.First(&setting).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Initialize default
			setting = ByogSetting{
				Enabled:              true,
				Tier1ProviderPrice:   0.40,
				Tier1CustomerPrice:   0.80,
				Tier2ProviderPrice:   0.20,
				Tier2CustomerPrice:   0.40,
				Tier3ProviderPrice:   0.05,
				Tier3CustomerPrice:   0.10,
				MaxColdStartTimeout:  120,
				NightOwlSurgeEnabled: true,
				WeekendCrunchEnabled: true,
				HardwareBackends:     `{"cuda":true,"rocm":true,"intel_arc":true,"metal":true,"adreno_hexagon":true,"mali":true,"cpu_avx":true}`,
				CreatedAt:            time.Now().Unix(),
				UpdatedAt:            time.Now().Unix(),
			}
			err = DB.Create(&setting).Error
			return &setting, err
		}
		return nil, err
	}
	return &setting, nil
}

func UpdateByogSetting(s *ByogSetting) error {
	s.UpdatedAt = time.Now().Unix()
	return DB.Save(s).Error
}

func IsByogGloballyEnabled() bool {
	setting, err := GetByogSetting()
	if err != nil {
		return false
	}
	return setting.Enabled
}

// Reward items
func GetAllRewardItems(activeOnly bool) ([]*ByogRewardItem, error) {
	var items []*ByogRewardItem
	tx := DB.Model(&ByogRewardItem{})
	if activeOnly {
		tx = tx.Where("is_active = ?", true)
	}
	err := tx.Order("required_tokens asc").Find(&items).Error
	return items, err
}

func GetRewardItemById(id int) (*ByogRewardItem, error) {
	var item ByogRewardItem
	err := DB.First(&item, id).Error
	return &item, err
}

func SaveRewardItem(item *ByogRewardItem) error {
	now := time.Now().Unix()
	if item.Id == 0 {
		item.CreatedAt = now
	}
	item.UpdatedAt = now
	return DB.Save(item).Error
}

func DeleteRewardItem(id int) error {
	return DB.Delete(&ByogRewardItem{}, id).Error
}

// Reward Claims
func GetAllRewardClaims(status string) ([]*ByogRewardClaim, error) {
	var claims []*ByogRewardClaim
	tx := DB.Model(&ByogRewardClaim{})
	if status != "" {
		tx = tx.Where("status = ?", status)
	}
	err := tx.Order("created_at desc").Find(&claims).Error
	if err == nil {
		for _, c := range claims {
			var u User
			if DB.Select("username, email").First(&u, c.UserId).Error == nil {
				c.UserName = u.Username
				c.UserEmail = u.Email
			}
		}
	}
	return claims, err
}

func GetUserRewardClaims(userId int) ([]*ByogRewardClaim, error) {
	var claims []*ByogRewardClaim
	err := DB.Where("user_id = ?", userId).Order("created_at desc").Find(&claims).Error
	return claims, err
}

// Assignment Rules
func GetAllAssignmentRules() ([]*ByogAssignmentRule, error) {
	var rules []*ByogAssignmentRule
	err := DB.Order("priority asc, id asc").Find(&rules).Error
	return rules, err
}

func SaveAssignmentRule(r *ByogAssignmentRule) error {
	now := time.Now().Unix()
	if r.Id == 0 {
		r.CreatedAt = now
	}
	r.UpdatedAt = now
	return DB.Save(r).Error
}

func DeleteAssignmentRule(id int) error {
	return DB.Delete(&ByogAssignmentRule{}, id).Error
}

// Models & LoRAs
func GetAllByogModels(enabledOnly bool) ([]*ByogModel, error) {
	var models []*ByogModel
	tx := DB.Model(&ByogModel{})
	if enabledOnly {
		tx = tx.Where("is_enabled = ?", true)
	}
	err := tx.Order("param_size asc, model_id asc").Find(&models).Error
	return models, err
}

func GetByogModelByModelId(modelId string) (*ByogModel, error) {
	var m ByogModel
	err := DB.Where("model_id = ?", modelId).First(&m).Error
	return &m, err
}

func SaveByogModel(m *ByogModel) error {
	now := time.Now().Unix()
	if m.Id == 0 {
		m.CreatedAt = now
	}
	m.UpdatedAt = now
	return DB.Save(m).Error
}

func DeleteByogModel(id int) error {
	var m ByogModel
	if err := DB.First(&m, id).Error; err != nil {
		return err
	}
	// Also delete child LoRAs
	DB.Where("base_model_id = ?", m.ModelId).Delete(&ByogModelLora{})
	return DB.Delete(&m).Error
}

func GetByogLorasForModel(baseModelId string) ([]*ByogModelLora, error) {
	var loras []*ByogModelLora
	err := DB.Where("base_model_id = ? AND is_enabled = ?", baseModelId, true).Find(&loras).Error
	return loras, err
}

func GetAllByogLoras() ([]*ByogModelLora, error) {
	var loras []*ByogModelLora
	err := DB.Find(&loras).Error
	return loras, err
}

func SaveByogLora(l *ByogModelLora) error {
	now := time.Now().Unix()
	if l.Id == 0 {
		l.CreatedAt = now
	}
	l.UpdatedAt = now
	return DB.Save(l).Error
}

func DeleteByogLora(id int) error {
	return DB.Delete(&ByogModelLora{}, id).Error
}

// Nodes CRUD & Queries
func GetAllByogNodes() ([]*ByogNode, error) {
	var nodes []*ByogNode
	err := DB.Order("status asc, last_heartbeat_at desc").Find(&nodes).Error
	if err == nil {
		for _, n := range nodes {
			var u User
			if DB.Select("username, email").First(&u, n.UserId).Error == nil {
				n.UserName = u.Username
				n.UserEmail = u.Email
			}
		}
	}
	return nodes, err
}

func GetUserByogNodes(userId int) ([]*ByogNode, error) {
	var nodes []*ByogNode
	err := DB.Where("user_id = ?", userId).Order("created_at desc").Find(&nodes).Error
	return nodes, err
}

func GetByogNodeByNodeId(nodeId string) (*ByogNode, error) {
	var n ByogNode
	err := DB.Where("node_id = ?", nodeId).First(&n).Error
	return &n, err
}

func GetByogNodeByHWID(hwid string) (*ByogNode, error) {
	var n ByogNode
	err := DB.Where("hwid = ?", hwid).First(&n).Error
	return &n, err
}

func GetByogNodeByPairingCode(code string) (*ByogNode, error) {
	var n ByogNode
	err := DB.Where("pairing_code = ?", strings.ToUpper(strings.TrimSpace(code))).First(&n).Error
	return &n, err
}

func SaveByogNode(n *ByogNode) error {
	now := time.Now().Unix()
	if n.Id == 0 {
		n.CreatedAt = now
	}
	n.UpdatedAt = now
	return DB.Save(n).Error
}

func DeleteByogNode(nodeId string) error {
	return DB.Where("node_id = ?", nodeId).Delete(&ByogNode{}).Error
}

// Banned Nodes
func GetAllBannedNodes() ([]*ByogNodeBan, error) {
	var bans []*ByogNodeBan
	err := DB.Order("banned_at desc").Find(&bans).Error
	if err == nil {
		for _, b := range bans {
			var u User
			if DB.Select("username").First(&u, b.UserId).Error == nil {
				b.UserName = u.Username
			}
			var n ByogNode
			if DB.Select("device_name").Where("node_id = ?", b.NodeId).First(&n).Error == nil {
				b.DeviceName = n.DeviceName
			}
		}
	}
	return bans, err
}

func IsHWIDBanned(hwid string) (bool, *ByogNodeBan) {
	var ban ByogNodeBan
	err := DB.Where("hwid = ? AND is_active = ?", hwid, true).First(&ban).Error
	if err == nil {
		return true, &ban
	}
	return false, nil
}

func IsUserBannedFromByog(userId int) (bool, *ByogNodeBan) {
	var ban ByogNodeBan
	err := DB.Where("user_id = ? AND is_active = ?", userId, true).First(&ban).Error
	if err == nil {
		return true, &ban
	}
	return false, nil
}

func UnbanNode(banId int) error {
	var ban ByogNodeBan
	if err := DB.First(&ban, banId).Error; err != nil {
		return err
	}
	ban.IsActive = false
	if err := DB.Save(&ban).Error; err != nil {
		return err
	}
	// Restore node status
	DB.Model(&ByogNode{}).Where("hwid = ?", ban.HWID).Update("status", "online")
	return nil
}

// Crash reports
func SaveCrashReport(cr *ByogCrashReport) error {
	cr.CreatedAt = time.Now().Unix()
	return DB.Create(cr).Error
}

func GetRecentCrashReports(limit int) ([]*ByogCrashReport, error) {
	var reports []*ByogCrashReport
	err := DB.Order("created_at desc").Limit(limit).Find(&reports).Error
	return reports, err
}

// P2P Tracker Helpers
func AnnounceP2PChunks(nodeId string, modelId string, chunkIndices []int) error {
	now := time.Now().Unix()
	for _, idx := range chunkIndices {
		var entry ByogP2PTracker
		err := DB.Where("node_id = ? AND model_id = ? AND chunk_index = ?", nodeId, modelId, idx).First(&entry).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			DB.Create(&ByogP2PTracker{
				NodeId:         nodeId,
				ModelId:        modelId,
				ChunkIndex:     idx,
				LastAnnounceAt: now,
			})
		} else if err == nil {
			DB.Model(&entry).Update("last_announce_at", now)
		}
	}
	return nil
}

func GetP2PChunkPeers(modelId string, chunkIndex int) ([]string, error) {
	var entries []*ByogP2PTracker
	// Peering valid if announced in last 10 minutes
	tenMinAgo := time.Now().Unix() - 600
	err := DB.Where("model_id = ? AND chunk_index = ? AND last_announce_at > ?", modelId, chunkIndex, tenMinAgo).Find(&entries).Error
	if err != nil {
		return nil, err
	}
	var nodeIds []string
	for _, e := range entries {
		nodeIds = append(nodeIds, e.NodeId)
	}
	return nodeIds, nil
}

// Seed default BYOG configurations and models if database is empty
func InitDefaultByogData() {
	// Check default setting
	_, _ = GetByogSetting()

	// Seed default models if none exist
	var count int64
	DB.Model(&ByogModel{}).Count(&count)
	if count == 0 {
		defaultModels := []*ByogModel{
			{
				ModelId:                "qwen-2.5-0.5b-instruct",
				ModelType:              "llm",
				Vendor:                 "Qwen",
				Description:            "Ultra-lightweight 0.5B model ideal for entry-level mobile devices and low-spec systems.",
				ParamSize:              "0.5B",
				PricingTier:            3,
				DownloadUrl:            "https://huggingface.co/Qwen/Qwen2.5-0.5B-Instruct-GGUF/resolve/main/qwen2.5-0.5b-instruct-q4_k_m.gguf",
				FileSize:               398000000,
				ChunkSize:              4194304,
				ModalitiesInput:        `["text"]`,
				ModalitiesOutput:       `["text"]`,
				AntiCheatLogicPrompt:   "What is 2+2? Answer only with the number.",
				AntiCheatLogicTemp:     0,
				AntiCheatLogicSeed:     42,
				AntiCheatLogicExpected: "4",
				AntiCheatLogicLogprob:  -0.001,
				AntiCheatLogicTolerance: 0.005,
				AntiCheatSigPrompt:     "What is the capital of Turkey?",
				AntiCheatSigTemp:       0,
				AntiCheatSigSeed:       8472910482910482,
				AntiCheatSigExpected:   " Ankara",
				AntiCheatSigLogprob:    -0.015,
				AntiCheatSigTolerance:  0.005,
				IsEnabled:              true,
				CreatedAt:              time.Now().Unix(),
				UpdatedAt:              time.Now().Unix(),
			},
			{
				ModelId:                "qwen-2.5-1.5b-instruct",
				ModelType:              "llm",
				Vendor:                 "Qwen",
				Description:            "Compact 1.5B model for mid-range mobile phones and standard laptops.",
				ParamSize:              "1.5B",
				PricingTier:            2,
				DownloadUrl:            "https://huggingface.co/Qwen/Qwen2.5-1.5B-Instruct-GGUF/resolve/main/qwen2.5-1.5b-instruct-q4_k_m.gguf",
				FileSize:               986000000,
				ChunkSize:              4194304,
				ModalitiesInput:        `["text"]`,
				ModalitiesOutput:       `["text"]`,
				AntiCheatLogicPrompt:   "A barn has 4 chickens and 2 sheep. How many total legs? Number only.",
				AntiCheatLogicTemp:     0,
				AntiCheatLogicSeed:     42,
				AntiCheatLogicExpected: "16",
				AntiCheatLogicLogprob:  -0.008,
				AntiCheatLogicTolerance: 0.004,
				AntiCheatSigPrompt:     "What is the capital of France?",
				AntiCheatSigTemp:       0,
				AntiCheatSigSeed:       8472910482910482,
				AntiCheatSigExpected:   " Paris",
				AntiCheatSigLogprob:    -0.0124,
				AntiCheatSigTolerance:  0.0020,
				IsEnabled:              true,
				CreatedAt:              time.Now().Unix(),
				UpdatedAt:              time.Now().Unix(),
			},
			{
				ModelId:                "qwen-2.5-7b-instruct",
				ModelType:              "llm",
				Vendor:                 "Qwen",
				Description:            "Flagship 7B model for powerful desktop GPUs, Apple Silicon, and workstation hardware.",
				ParamSize:              "7B",
				PricingTier:            1,
				DownloadUrl:            "https://huggingface.co/Qwen/Qwen2.5-7B-Instruct-GGUF/resolve/main/qwen2.5-7b-instruct-q4_k_m.gguf",
				FileSize:               4680000000,
				ChunkSize:              16777216,
				ModalitiesInput:        `["text","image"]`,
				ModalitiesOutput:       `["text"]`,
				AntiCheatLogicPrompt:   "Bir kümeste 4 tavuk ve 2 koyun var. Toplam kaç ayak vardır? Sadece rakam yaz.",
				AntiCheatLogicTemp:     0,
				AntiCheatLogicSeed:     42,
				AntiCheatLogicExpected: "16",
				AntiCheatLogicLogprob:  -0.008,
				AntiCheatLogicTolerance: 0.004,
				AntiCheatSigPrompt:     "Türkiye'nin başkenti neresidir?",
				AntiCheatSigTemp:       0,
				AntiCheatSigSeed:       8472910482910482,
				AntiCheatSigExpected:   " Ankara",
				AntiCheatSigLogprob:    -0.0124,
				AntiCheatSigTolerance:  0.0020,
				IsEnabled:              true,
				CreatedAt:              time.Now().Unix(),
				UpdatedAt:              time.Now().Unix(),
			},
			{
				ModelId:                "bge-large-en-v1.5",
				ModelType:              "embedding",
				Vendor:                 "BAAI",
				Description:            "State of the art text embedding model running efficiently across edge nodes.",
				ParamSize:              "0.3B",
				PricingTier:            2,
				DownloadUrl:            "https://huggingface.co/BAAI/bge-large-en-v1.5/resolve/main/model.safetensors",
				FileSize:               1340000000,
				ChunkSize:              8388608,
				ModalitiesInput:        `["text"]`,
				ModalitiesOutput:       `["text"]`,
				IsEnabled:              true,
				CreatedAt:              time.Now().Unix(),
				UpdatedAt:              time.Now().Unix(),
			},
		}

		for _, m := range defaultModels {
			DB.Create(m)
		}

		// Also seed a sample LoRA
		DB.Create(&ByogModelLora{
			BaseModelId:        "qwen-2.5-7b-instruct",
			LoraId:             "qwen-2.5-7b-coder-pro",
			DownloadUrl:        "https://huggingface.co/Qwen/Qwen2.5-Coder-7B-Instruct-GGUF/resolve/main/adapter.gguf",
			FileSize:           280000000,
			ModalityTags:       `["text","code"]`,
			AntiCheatPrompt:    "Write a python function to add two numbers.",
			AntiCheatSeed:      42,
			AntiCheatExpected:  "def add(a, b): return a + b",
			AntiCheatLogprob:   -0.015,
			AntiCheatTolerance: 0.005,
			IsEnabled:          true,
			CreatedAt:          time.Now().Unix(),
			UpdatedAt:          time.Now().Unix(),
		})
	}

	// Seed default Assignment Rules if none exist
	var ruleCount int64
	DB.Model(&ByogAssignmentRule{}).Count(&ruleCount)
	if ruleCount == 0 {
		defaultRules := []*ByogAssignmentRule{
			{
				Name:                 "Desktop High-Performance CUDA/Metal Rule",
				DeviceClass:          "desktop_server",
				MinVramMB:            6144, // 6GB
				MinPhysicalRamMB:     16384, // 16GB
				HardwareFamily:       "all",
				MinCores:             8,
				VectorInstructionSet: "all",
				TargetModels:         `["qwen-2.5-7b-instruct","bge-large-en-v1.5"]`,
				TargetLoras:          `["qwen-2.5-7b-coder-pro"]`,
				Priority:             1,
				IsEnabled:            true,
				CreatedAt:            time.Now().Unix(),
				UpdatedAt:            time.Now().Unix(),
			},
			{
				Name:                 "Mid-Range Mobile & Laptop Rule (Safe Memory Allocation)",
				DeviceClass:          "mobile_tablet",
				MinVramMB:            0,
				MinPhysicalRamMB:     6144, // 6GB Physical (ZRAM Ignored!)
				HardwareFamily:       "all",
				MinCores:             6,
				VectorInstructionSet: "all",
				TargetModels:         `["qwen-2.5-1.5b-instruct","qwen-2.5-0.5b-instruct"]`,
				TargetLoras:          `[]`,
				Priority:             2,
				IsEnabled:            true,
				CreatedAt:            time.Now().Unix(),
				UpdatedAt:            time.Now().Unix(),
			},
			{
				Name:                 "Entry-Level Mobile Rule (0.5B / Embedding Only)",
				DeviceClass:          "mobile_tablet",
				MinVramMB:            0,
				MinPhysicalRamMB:     3072, // 3GB Physical
				HardwareFamily:       "all",
				MinCores:             4,
				VectorInstructionSet: "all",
				TargetModels:         `["qwen-2.5-0.5b-instruct"]`,
				TargetLoras:          `[]`,
				Priority:             3,
				IsEnabled:            true,
				CreatedAt:            time.Now().Unix(),
				UpdatedAt:            time.Now().Unix(),
			},
		}

		for _, r := range defaultRules {
			DB.Create(r)
		}
	}

	// Seed default Reward Catalog items if none exist
	var rewardCount int64
	DB.Model(&ByogRewardItem{}).Count(&rewardCount)
	if rewardCount == 0 {
		defaultRewards := []*ByogRewardItem{
			{
				Title:          "Minecraft Premium Account",
				Description:    "Full access official Minecraft Java & Bedrock Edition account.",
				Category:       "gaming",
				RequiredTokens: 50000000, // 50M tokens
				Stock:          25,
				IsActive:       true,
				CreatedAt:      time.Now().Unix(),
				UpdatedAt:      time.Now().Unix(),
			},
			{
				Title:          "Windows 11 Pro OEM Key",
				Description:    "Official Microsoft Windows 11 Professional lifetime license key.",
				Category:       "software",
				RequiredTokens: 40000000, // 40M tokens
				Stock:          50,
				IsActive:       true,
				CreatedAt:      time.Now().Unix(),
				UpdatedAt:      time.Now().Unix(),
			},
			{
				Title:          "Steam $20 Gift Card",
				Description:    "Digital wallet code redeemable on Steam Store worldwide.",
				Category:       "giftcard",
				RequiredTokens: 60000000, // 60M tokens
				Stock:          30,
				IsActive:       true,
				CreatedAt:      time.Now().Unix(),
				UpdatedAt:      time.Now().Unix(),
			},
			{
				Title:          "Discord Nitro 3-Month Boost Pack",
				Description:    "3 months of Discord Nitro subscription with 2 server boosts.",
				Category:       "digital",
				RequiredTokens: 35000000, // 35M tokens
				Stock:          40,
				IsActive:       true,
				CreatedAt:      time.Now().Unix(),
				UpdatedAt:      time.Now().Unix(),
			},
		}

		for _, rew := range defaultRewards {
			DB.Create(rew)
		}
	}
}

// Calculate Total Boost Multiplier for a Node
func CalculateNodeMultipliers(node *ByogNode) (float64, []string) {
	var totalBonusPercent float64 = 0.0
	var activeBonuses []string

	// Hardware & Performance based multipliers (Section 7.1)
	// 1. VRAM Pinning: +25%
	if node.AllocatedVramGB >= 4 && node.CurrentTtftMs > 0 && node.CurrentTtftMs < 500 {
		totalBonusPercent += 0.25
		activeBonuses = append(activeBonuses, "VRAM_PINNING_+25%")
	}

	// 2. Ultra Long Context (32K+ KV Cache): +30%
	if node.AllocatedVramGB >= 12 || (node.AllocatedVramGB >= 8 && node.AllocatedRamGB >= 32) {
		totalBonusPercent += 0.30
		activeBonuses = append(activeBonuses, "ULTRA_LONG_CONTEXT_+30%")
	}

	// 3. Continuous Batching: +20%
	if node.AllocatedCores >= 8 && (node.DeviceType == "desktop" || node.DeviceType == "server") {
		totalBonusPercent += 0.20
		activeBonuses = append(activeBonuses, "CONTINUOUS_BATCHING_+20%")
	}

	// 4. High-Bandwidth & Low Ping (<30ms): +10%
	if node.PingMs > 0 && node.PingMs < 30 && node.BandwidthDownMbps >= 50 && node.BandwidthUpMbps >= 50 {
		totalBonusPercent += 0.10
		activeBonuses = append(activeBonuses, "LOW_PING_BANDWIDTH_+10%")
	}

	// 5. Large Model Host (24GB+ VRAM): +40%
	if node.AllocatedVramGB >= 24 {
		totalBonusPercent += 0.40
		activeBonuses = append(activeBonuses, "LARGE_MODEL_HOST_+40%")
	}

	// 6. Confidential AI (TEE/SGX Hardware): +15%
	if node.TeeSupported {
		totalBonusPercent += 0.15
		activeBonuses = append(activeBonuses, "CONFIDENTIAL_TEE_+15%")
	}

	// 7. LoRA Hot-Swap Champion: +20%
	var localModels []string
	_ = json.Unmarshal([]byte(node.InstalledModels), &localModels)
	if len(localModels) >= 5 {
		totalBonusPercent += 0.20
		activeBonuses = append(activeBonuses, "LORA_HOT_SWAP_+20%")
	}

	// 8. Zero-Packet-Drop: +10%
	if node.NetworkType == "ethernet" || (node.BandwidthDownMbps > 80 && node.PingMs < 50) {
		totalBonusPercent += 0.10
		activeBonuses = append(activeBonuses, "ZERO_PACKET_DROP_+10%")
	}

	// 9. Dedicated Tensor Float FP16: +25%
	if strings.Contains(strings.ToLower(node.GpuModel), "rtx 40") ||
		strings.Contains(strings.ToLower(node.GpuModel), "a100") ||
		strings.Contains(strings.ToLower(node.GpuModel), "h100") ||
		strings.Contains(strings.ToLower(node.GpuModel), "m3") ||
		strings.Contains(strings.ToLower(node.GpuModel), "m4") {
		totalBonusPercent += 0.25
		activeBonuses = append(activeBonuses, "DEDICATED_TENSOR_FP16_+25%")
	}

	// 10. Multi-Modal Toolkit: +20%
	if node.AllocatedVramGB >= 8 && node.AllocatedRamGB >= 16 {
		totalBonusPercent += 0.20
		activeBonuses = append(activeBonuses, "MULTI_MODAL_TOOLKIT_+20%")
	}

	// 11. Symmetric Fiber & P2P Seeder: +12%
	if node.BandwidthUpMbps >= 100 && math.Abs(node.BandwidthDownMbps-node.BandwidthUpMbps) < 30 {
		totalBonusPercent += 0.12
		activeBonuses = append(activeBonuses, "SYMMETRIC_FIBER_P2P_+12%")
	}

	// Uptime & Consistency multipliers (Section 7.2)
	// 24h: +10%, 72h: +20%, 7 days: +35%
	if node.UptimeSeconds >= 7*86400 {
		totalBonusPercent += 0.35
		activeBonuses = append(activeBonuses, "UPTIME_7_DAYS_TITAN_+35%")
	} else if node.UptimeSeconds >= 3*86400 {
		totalBonusPercent += 0.20
		activeBonuses = append(activeBonuses, "UPTIME_72_HOURS_+20%")
	} else if node.UptimeSeconds >= 86400 {
		totalBonusPercent += 0.10
		activeBonuses = append(activeBonuses, "UPTIME_24_HOURS_+10%")
	}

	// Reputation Score > 95: +15%
	if node.ReputationScore >= 95 {
		totalBonusPercent += 0.15
		activeBonuses = append(activeBonuses, "REPUTATION_ELITE_+15%")
	}

	// Time-Based Spot Surges (Section 7.3)
	// 1. Night Owl Surge (01:00 - 08:00): +30%
	nowHour := time.Now().Hour()
	if nowHour >= 1 && nowHour < 8 {
		totalBonusPercent += 0.30
		activeBonuses = append(activeBonuses, "NIGHT_OWL_SURGE_+30%")
	}

	// 2. Weekend Crunch (Saturday & Sunday): +20%
	weekday := time.Now().Weekday()
	if weekday == time.Saturday || weekday == time.Sunday {
		totalBonusPercent += 0.20
		activeBonuses = append(activeBonuses, "WEEKEND_CRUNCH_+20%")
	}

	multiplier := 1.0 + totalBonusPercent
	return multiplier, activeBonuses
}
