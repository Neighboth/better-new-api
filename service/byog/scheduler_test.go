package byog

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
)

func TestSchedulerDeviceClassMatching(t *testing.T) {
	nodeMobile := &model.ByogNode{
		DeviceType:    "mobile",
		PhysicalRamMB: 8192,
		AllocatedCores: 8,
	}

	nodeTablet := &model.ByogNode{
		DeviceType:    "tablet",
		PhysicalRamMB: 8192,
		AllocatedCores: 8,
	}

	nodeDesktop := &model.ByogNode{
		DeviceType:    "desktop",
		PhysicalRamMB: 16384,
		AllocatedCores: 8,
	}

	// Verify IsModelSafeForMobile
	s := GlobalScheduler
	safe05b := s.IsModelSafeForMobile(nodeMobile, "non-existent")
	assert.False(t, safe05b) // non existent model returns false

	// Test CalculateSafeMobileBudgetMB
	budget := s.CalculateSafeMobileBudgetMB(8192, 4096)
	assert.Greater(t, budget, 1000)
	assert.LessOrEqual(t, budget, 8192)

	_ = nodeTablet
	_ = nodeDesktop
}
