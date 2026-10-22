package byog

import (
	"encoding/json"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/model"
)

type Scheduler struct{}

var GlobalScheduler = &Scheduler{}

// Evaluate and assign models to a node based on rules and strict hardware limits
func (s *Scheduler) EvaluateNodeModelAssignment(node *model.ByogNode) ([]string, error) {
	rules, err := model.GetAllAssignmentRules()
	if err != nil {
		return nil, err
	}

	assignedModelSet := make(map[string]bool)
	var assignedList []string

	for _, rule := range rules {
		if !rule.IsEnabled {
			continue
		}

		// 1. Device Class Check
		if rule.DeviceClass != "all" {
			rc := strings.ToLower(strings.TrimSpace(rule.DeviceClass))
			dt := strings.ToLower(strings.TrimSpace(node.DeviceType))
			matched := (rc == dt) ||
				(rc == "mobile_tablet" && (dt == "mobile" || dt == "tablet")) ||
				(rc == "desktop_server" && (dt == "desktop" || dt == "laptop" || dt == "server"))
			if !matched {
				continue
			}
		}

		// 2. Physical RAM Check (Strictly Physical - Ignoring Swap/ZRAM!)
		//
		// The user gives a part of the memory of the device to the network, the
		// models are assigned with the memory that is given and not with the
		// whole memory of the device.
		if availableRamMB(node) < rule.MinPhysicalRamMB {
			continue
		}

		// 3. VRAM Check
		if rule.MinVramMB > 0 {
			effectiveVram := node.VramMB + node.SharedVramMB
			if effectiveVram < rule.MinVramMB {
				continue
			}
		}

		// 4. Cores Check
		if rule.MinCores > 0 && node.AllocatedCores < rule.MinCores {
			continue
		}

		// Parse target models
		var targetModels []string
		if err := json.Unmarshal([]byte(rule.TargetModels), &targetModels); err == nil {
			for _, mId := range targetModels {
				// 5. Section 6.1 Strict Mobile Tiering & Safety Filter
				if node.DeviceType == "mobile" || node.DeviceType == "tablet" {
					if !s.IsModelSafeForMobile(node, mId) {
						continue
					}
				}

				if !assignedModelSet[mId] {
					assignedModelSet[mId] = true
					assignedList = append(assignedList, mId)
				}
			}
		}
	}

	return assignedList, nil
}

// availableRamMB returns the memory that the node can use for the models.
//
// The user gives a part of the memory of the device to the network, the models
// are assigned with the memory that is given and not with the whole memory of
// the device. The swap and the zram of the device are never counted.
func availableRamMB(node *model.ByogNode) int {
	if node == nil {
		return 0
	}
	available := node.PhysicalRamMB
	if node.AllocatedRamGB > 0 {
		allocated := node.AllocatedRamGB * 1024
		if allocated < available {
			available = allocated
		}
	}
	return available
}

// Section 6.1 Strict Mobile Safety & Tiering Engine
func (s *Scheduler) IsModelSafeForMobile(node *model.ByogNode, modelId string) bool {
	modelDef, err := model.GetByogModelByModelId(modelId)
	if err != nil || modelDef == nil {
		return false
	}

	// Always allow embedding models on mobile
	if modelDef.ModelType == "embedding" || modelDef.ModelType == "reranker" {
		return true
	}

	physicalRamMB := availableRamMB(node) // Safely ignores ZRAM / Swap
	paramSize := strings.ToUpper(strings.TrimSpace(modelDef.ParamSize))

	// Entry level: 3GB - 4GB Physical RAM -> max 0.5B
	if physicalRamMB <= 4096 {
		return paramSize == "0.5B" || strings.Contains(paramSize, "0.5")
	}

	// Mid range: 6GB Physical RAM (e.g. Realme C55 with 6GB real + 6GB virtual) -> max 1.5B or 3B, NEVER 4B or 7B!
	if physicalRamMB <= 6144 {
		return paramSize == "0.5B" || paramSize == "1B" || paramSize == "1.5B" || paramSize == "3B"
	}

	// Mid-high range: 8GB - 12GB Physical RAM -> max 3B / 4B
	if physicalRamMB <= 12288 {
		return paramSize == "0.5B" || paramSize == "1B" || paramSize == "1.5B" || paramSize == "3B" || paramSize == "4B"
	}

	// Ultra flagship: 16GB+ Physical LPDDR5X (Snapdragon 8 Elite / Gen 4 / Apple A18 Pro / M-series iPad) -> 7B allowed
	if physicalRamMB >= 16384 {
		return true
	}

	return false
}

// Safe Allocatable Mobile Memory Budget Formula (Section 6.1)
// Max Model Budget = min(MemAvailable * 0.55, Physical MemTotal * 0.40)
func (s *Scheduler) CalculateSafeMobileBudgetMB(physicalTotalMB int, memAvailableMB int) int {
	budgetA := float64(memAvailableMB) * 0.55
	budgetB := float64(physicalTotalMB) * 0.40
	return int(math.Min(budgetA, budgetB))
}

// LRU Cache Purge Decision (Section 2.3)
func (s *Scheduler) DetermineLruModelToPurge(node *model.ByogNode, newModelSizeMB int64) (string, error) {
	var installed []string
	if err := json.Unmarshal([]byte(node.InstalledModels), &installed); err != nil || len(installed) == 0 {
		return "", nil
	}

	// Find the least recently used model or model not used in 7 days
	// Query logs or pick the first candidate that frees up enough disk
	for _, mId := range installed {
		mDef, err := model.GetByogModelByModelId(mId)
		if err == nil && mDef != nil {
			// Found candidate to purge
			return mId, nil
		}
	}

	return installed[0], nil
}
