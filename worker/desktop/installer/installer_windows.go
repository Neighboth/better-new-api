//go:build windows

// Package installer writes the entries of the client into the system.
//
// The client is shown in the applications of the control panel and in the
// search of the system, it can be removed from there.
package installer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows/registry"
)

const (
	// uninstallKey is the key of the client in the list of the applications of
	// the control panel.
	uninstallKey = `Software\Microsoft\Windows\CurrentVersion\Uninstall\BYOGWorker`
	// appPathsKey makes the client known to the system, it can be started with
	// its name.
	appPathsKey = `Software\Microsoft\Windows\CurrentVersion\App Paths\BYOG Worker.exe`
	// publisherName is the publisher of the client in the control panel.
	publisherName = "BYOG"
)

// Directory returns the directory of the client.
func Directory() string {
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		local = os.Getenv("USERPROFILE")
	}
	return filepath.Join(local, "Programs", "BYOG Worker")
}

// Executable returns the path of the program of the client.
func Executable() string {
	return filepath.Join(Directory(), "BYOG Worker.exe")
}

// ShortcutPath returns the path of the shortcut of the client.
func ShortcutPath() string {
	programs := os.Getenv("APPDATA")
	if programs == "" {
		programs = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Roaming")
	}
	return filepath.Join(programs, "Microsoft", "Windows", "Start Menu", "Programs", "BYOG Worker.lnk")
}

// Install writes the entries of the client into the system.
//
// The program of the client and its icon are already in the program directory.
func Install(displayName string, version string) error {
	executable := Executable()
	icon := filepath.Join(Directory(), "byog.ico")

	writeUninstallEntry(displayName, version, executable, icon)
	writeAppPathsEntry(executable)
	writeStartMenuShortcut(executable, icon)
	return nil
}

// Uninstall removes the entries of the client from the system.
//
// The program of the client removes itself after it ended.
func Uninstall() error {
	removeRegistryKey(uninstallKey)
	removeRegistryKey(appPathsKey)

	shortcut := ShortcutPath()
	if _, err := os.Stat(shortcut); err == nil {
		_ = os.Remove(shortcut)
	}

	directory := Directory()
	script := filepath.Join(directory, "uninstall.cmd")
	_ = os.WriteFile(script, []byte(
		"@echo off\r\n"+
			"ping -n 3 127.0.0.1 > nul\r\n"+
			fmt.Sprintf("rmdir /s /q %q\r\n", directory)), 0644)
	command := exec.Command("cmd", "/c", "start", "", "/min", script)
	_ = command.Start()
	return nil
}

// UpdateBranding writes the name and the icon of the installation into the
// entries of the system.
//
// The dashboard sends its name and its icon, the client shows them in the
// control panel and in the search of the system.
func UpdateBranding(displayName string, version string) error {
	if displayName == "" {
		return nil
	}
	if err := updateInstallationName(displayName, version); err != nil {
		return err
	}
	renameShortcut(displayName)
	return nil
}

// The key of the installer of the system.
const systemUninstallKey = `Software\Microsoft\Windows\CurrentVersion\Uninstall\{5B4C0A1E-6C1B-4C0E-9E4F-7A3D2B5C8E11}_is1`

// updateInstallationName writes the name of the installation into the entry of
// the installation.
//
// The installer of the system writes its own entry, the name of the installation
// is written into that entry when it is there. The client writes its own entry
// when the installation was made by the client.
func updateInstallationName(displayName string, version string) error {
	if _, err := os.Stat(filepath.Join(Directory(), "unins000.exe")); err == nil {
		key, err := registry.OpenKey(registry.CURRENT_USER, systemUninstallKey, registry.SET_VALUE)
		if err != nil {
			return Install(displayName, version)
		}
		defer key.Close()
		_ = key.SetStringValue("DisplayName", displayName)
		_ = key.SetStringValue("DisplayVersion", version)
		_ = key.SetStringValue("DisplayIcon", filepath.Join(Directory(), "byog.ico"))
		return nil
	}
	return Install(displayName, version)
}

// renameShortcut gives the shortcut of the client the name of the installation.
//
// The shortcut of the start menu is the way the user starts the client, it shows
// the name of the installation.
func renameShortcut(displayName string) {
	current := ShortcutPath()
	if _, err := os.Stat(current); err != nil {
		return
	}
	cleaned := strings.Map(func(character rune) rune {
		switch character {
		case '\\', '/', ':', '*', '?', '"', '<', '>', '|':
			return '-'
		}
		return character
	}, displayName)
	target := filepath.Join(filepath.Dir(current), strings.TrimSpace(cleaned)+".lnk")
	if target == current {
		return
	}
	if _, err := os.Stat(target); err == nil {
		_ = os.Remove(target)
	}
	if err := os.Rename(current, target); err != nil {
		return
	}
	script := fmt.Sprintf(
		"$shell = New-Object -ComObject WScript.Shell; "+
			"$shortcut = $shell.CreateShortcut(%q); "+
			"$shortcut.IconLocation = %q; "+
			"$shortcut.Save()",
		target, filepath.Join(Directory(), "byog.ico"))
	command := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	_ = command.Run()
}

// IsInstalled reports whether the client is installed on the computer.
//
// The installer of the system writes the uninstaller into the directory of the
// client, the entries of the system are written by the client itself.
func IsInstalled() bool {
	if _, err := os.Stat(filepath.Join(Directory(), "unins000.exe")); err == nil {
		return true
	}
	if _, err := os.Stat(Executable()); err == nil {
		key, err := registry.OpenKey(registry.CURRENT_USER, uninstallKey, registry.QUERY_VALUE)
		if err == nil {
			defer key.Close()
			if _, _, err := key.GetIntegerValue("NoModify"); err == nil {
				return true
			}
		}
	}
	return false
}

// writeUninstallEntry writes the entry of the client in the applications of the
// control panel.
func writeUninstallEntry(displayName string, version string, executable string, icon string) {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, uninstallKey, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return
	}
	defer key.Close()

	_ = key.SetStringValue("DisplayName", displayName)
	_ = key.SetStringValue("DisplayVersion", version)
	_ = key.SetStringValue("Publisher", publisherName)
	_ = key.SetStringValue("DisplayIcon", icon)
	_ = key.SetStringValue("InstallLocation", Directory())
	_ = key.SetStringValue("UninstallString", fmt.Sprintf("\"%s\" --uninstall", executable))
	_ = key.SetStringValue("QuietUninstallString", fmt.Sprintf("\"%s\" --uninstall", executable))
	_ = key.SetStringValue("InstallDate", time.Now().Format("2006-01-02"))
	_ = key.SetDWordValue("NoModify", 1)
	_ = key.SetDWordValue("NoRepair", 1)
	_ = key.SetDWordValue("EstimatedSize", 12000)
	_ = key.SetStringValue("Comments", "The client of the local computing network of the installation.")
}

// writeAppPathsEntry writes the path of the program of the client, the system
// starts the client with its name.
func writeAppPathsEntry(executable string) {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, appPathsKey, registry.SET_VALUE)
	if err != nil {
		return
	}
	defer key.Close()
	_ = key.SetStringValue("", executable)
}

// removeRegistryKey removes a key of the registry of the user.
func removeRegistryKey(path string) {
	_ = registry.DeleteKey(registry.CURRENT_USER, path)
}

// writeStartMenuShortcut writes the shortcut of the client in the start menu.
//
// The shortcut shows the client in the search of the system, it is the way the
// user starts the client. The system writes the file, it knows the way to the
// program of the shortcut.
func writeStartMenuShortcut(executable string, icon string) {
	shortcut := ShortcutPath()
	if err := os.MkdirAll(filepath.Dir(shortcut), 0755); err != nil {
		return
	}
	script := fmt.Sprintf(
		"$shell = New-Object -ComObject WScript.Shell; "+
			"$shortcut = $shell.CreateShortcut(%q); "+
			"$shortcut.TargetPath = %q; "+
			"$shortcut.WorkingDirectory = %q; "+
			"$shortcut.IconLocation = %q; "+
			"$shortcut.Description = %q; "+
			"$shortcut.Save()",
		shortcut, executable, Directory(), icon, "BYOG Worker")
	command := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	_ = command.Run()
}
