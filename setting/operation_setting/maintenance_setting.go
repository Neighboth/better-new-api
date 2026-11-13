package operation_setting

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// MaintenanceModeSetting is the plain view of the three options that toggle
// the whole installation into maintenance mode. The keys match the option
// memory map keys used by the settings UI.
type MaintenanceModeSetting struct {
	Enabled bool
	ETA     int64 // unix seconds, zero means no ETA advertised
	Message string
}

const (
	MaintenanceModeEnabledKey = "MaintenanceModeEnabled"
	MaintenanceModeETAKey     = "MaintenanceModeETA"
	MaintenanceModeMessageKey = "MaintenanceModeMessage"
)

// GetMaintenanceModeSetting reads the options straight from the map with the
// shared read-lock; the options of the installation are refreshed
// asynchronously by the option loader, so the snapshot may trail the write
// by one tick.
func GetMaintenanceModeSetting() MaintenanceModeSetting {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	rawETA := strings.TrimSpace(common.OptionMap[MaintenanceModeETAKey])
	eta, err := strconv.ParseInt(rawETA, 10, 64)
	if err != nil {
		eta = 0
	}
	return MaintenanceModeSetting{
		Enabled: common.OptionMap[MaintenanceModeEnabledKey] == "true",
		ETA:     eta,
		Message: strings.TrimSpace(common.OptionMap[MaintenanceModeMessageKey]),
	}
}

// IsMaintenanceMode reports whether the installation is in maintenance.
func IsMaintenanceMode() bool {
	return GetMaintenanceModeSetting().Enabled
}
