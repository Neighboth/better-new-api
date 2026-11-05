//go:build windows

package main

import "testing"

// TestHardwareDetectionOnThisMachine exercises the Windows-only GPU enumeration
// (displayAdapters is implemented in platform_windows.go via DXGI), so the whole
// file is tagged for Windows to keep go vet/test compiling on Linux CI.
func TestHardwareDetectionOnThisMachine(t *testing.T) {
	adapters := displayAdapters()
	t.Logf("Adapters found: %d", len(adapters))
	for i, a := range adapters {
		t.Logf("Adapter %d: %s, VramMB: %d", i, a.Name, a.VramMB)
	}
	vram := dedicatedVRAMMB()
	t.Logf("Dedicated VRAM: %d MB", vram)
	hw := readHardware()
	t.Logf("Hardware: GPU=%s, VramMB=%d, SharedVramMB=%d, RamMB=%d", hw.GpuModel, hw.VramMB, hw.SharedVramMB, hw.PhysicalRamMB)
}
