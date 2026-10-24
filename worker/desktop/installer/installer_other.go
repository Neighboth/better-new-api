//go:build !windows

// Package installer writes the entries of the client into the system.
//
// The systems that have no program directory of the user keep the entries of
// the client in the profile of the user.
package installer

import (
	"os"
	"path/filepath"
)

// Directory returns the directory of the client.
func Directory() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "byog")
	}
	return filepath.Join(home, ".local", "share", "byog-worker")
}

// Executable returns the path of the program of the client.
func Executable() string {
	return filepath.Join(Directory(), "byog-worker")
}

// Install writes the entries of the client into the system.
func Install(displayName string, version string) error {
	writeDesktopEntry(displayName)
	return nil
}

// Uninstall removes the entries of the client from the system.
func Uninstall() error {
	_ = os.RemoveAll(Directory())
	return nil
}

// UpdateBranding writes the name of the installation into the entries of the
// system.
func UpdateBranding(displayName string, version string) error {
	if displayName == "" {
		return nil
	}
	return Install(displayName, version)
}

// UpdateUninstallerLanguage is a no-op on non-Windows platforms.
func UpdateUninstallerLanguage(lang string) {}

// IsInstalled reports whether the client is installed on the computer.
func IsInstalled() bool {
	_, err := os.Stat(Executable())
	return err == nil
}

// writeDesktopEntry writes the entry of the client for the desktop of the
// system.
func writeDesktopEntry(displayName string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	desktop := filepath.Join(home, ".local", "share", "applications")
	_ = os.MkdirAll(desktop, 0755)
	entry := "[Desktop Entry]\n" +
		"Type=Application\n" +
		"Name=" + displayName + "\n" +
		"Exec=" + Executable() + "\n" +
		"Terminal=false\n" +
		"Categories=Utility;\n" +
		"Comment=The client of the local computing network of the installation.\n"
	_ = os.WriteFile(filepath.Join(desktop, "byog-worker.desktop"), []byte(entry), 0644)
}
