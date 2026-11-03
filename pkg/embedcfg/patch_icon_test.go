package embedcfg

import (
	"os"
	"testing"

	"github.com/QuantumNous/new-api/pkg/icon"
)

func TestPatchInnoUninstaller(t *testing.T) {
	uninsPath := `C:\Users\simse\AppData\Local\Programs\Inno Setup 6\unins000.exe`
	raw, err := os.ReadFile(uninsPath)
	if err != nil {
		t.Skip("Inno Setup unins000.exe not found on test machine")
	}
	icoPath := `Z:\better-new-api\worker\desktop\assets\byog.ico`
	icoData, err := os.ReadFile(icoPath)
	if err != nil {
		t.Fatalf("Failed to read byog.ico: %v", err)
	}
	pngData := icon.PNG(icoData, icon.MaxResourceSize)
	if len(pngData) == 0 {
		t.Fatalf("Failed to get PNG data from ico")
	}
	patched, err := PatchIcon(raw, pngData)
	if err != nil {
		t.Fatalf("PatchIcon failed on Inno Setup unins000.exe: %v", err)
	}
	if len(patched) == 0 {
		t.Fatalf("Patched bytes are empty")
	}
	t.Logf("Successfully patched! len=%d", len(patched))
}
