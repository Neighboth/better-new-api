//go:build windows

// Package installer writes the entries of the client into the system.
//
// The client is shown in the applications of the control panel and in the
// search of the system, it can be removed from there.
//
// An installation of the client has exactly one entry in the applications of
// the control panel. The installer of the system (Inno Setup) writes its entry
// when the client is installed by the download of the installation and the
// client writes an entry of its own when the program is copied to the computer
// without the installer. The two entries of an installation are the duplicate
// that the users of the first versions of the client saw: every function of
// this package removes the entry that it does not own.
package installer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows/registry"
)

func hideProcessWindow(command *exec.Cmd) {
	if command == nil {
		return
	}
	command.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}

const (
	// uninstallKey is the key of the client that installed itself, it is the
	// entry of the applications of the control panel of the program that was
	// copied to the computer without the installer.
	uninstallKey = `Software\Microsoft\Windows\CurrentVersion\Uninstall\BYOGWorker`
	// appId is the identifier of the installation of the installer of the
	// system. The uninstaller writes its entry under the identifier.
	appId = `{5B4C0A1E-6C1B-4C0E-9E4F-7A3D2B5C8E11}`
	// innoUninstallKey is the key of the entry of the installer of the system.
	innoUninstallKey = `Software\Microsoft\Windows\CurrentVersion\Uninstall\` + appId + `_is1`
	// appPathsKey makes the client known to the system, it can be started with
	// its name.
	appPathsKey = `Software\Microsoft\Windows\CurrentVersion\App Paths\BYOG Worker.exe`
	// publisherName is the publisher of the client in the control panel.
	publisherName = "BYOG"
	// startupKey is the key of the programs that start with the system.
	startupKey = `Software\Microsoft\Windows\CurrentVersion\Run`
	// programName is the name of the program of the client.
	programName = "BYOG Worker.exe"
)

// Directory returns the default directory of the client.
func Directory() string {
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		local = os.Getenv("USERPROFILE")
	}
	return filepath.Join(local, "Programs", "BYOG Worker")
}

// InstalledDirectory returns the directory of the running client.
//
// The installer of the system installs the client into a directory that
// carries the name of the installation, the directory of the program is the
// directory of the client.
func InstalledDirectory() string {
	executable, err := os.Executable()
	if err != nil {
		return Directory()
	}
	return filepath.Dir(executable)
}

// Executable returns the path of the program of the client.
func Executable() string {
	return filepath.Join(Directory(), programName)
}

// ShortcutPath returns the path of the shortcut of the client.
func ShortcutPath() string {
	programs := os.Getenv("APPDATA")
	if programs == "" {
		programs = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Roaming")
	}
	return filepath.Join(programs, "Microsoft", "Windows", "Start Menu", "Programs", "BYOG Worker.lnk")
}

// InnoUninstaller returns the uninstaller of the installation of the system.
//
// The uninstaller is written into the directory of the client. The client is
// installed by the installer of the system when the uninstaller is next to the
// program, the entries of that installation belong to the installer.
func InnoUninstaller() (string, bool) {
	directories := []string{InstalledDirectory(), Directory()}
	for _, directory := range directories {
		candidate := filepath.Join(directory, "unins000.exe")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, true
		}
	}
	return "", false
}

// lastError reports whether an installation of the system is registered.
//
// The uninstaller is not always next to the program, an installation that was
// repaired or moved keeps its entry in the registry.
func innoEntryExists() bool {
	for _, root := range []registry.Key{registry.CURRENT_USER, registry.LOCAL_MACHINE} {
		key, err := registry.OpenKey(root, innoUninstallKey, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		key.Close()
		return true
	}
	return false
}

// Install writes the entries of the client into the system.
//
// The program of the client and its icon are already in the program directory.
// An installation of the installer of the system owns the entry of the
// applications of the control panel, the client removes the entry that an
// older version of the client wrote and it does not write a second one.
func Install(displayName string, version string) error {
	executable := filepath.Join(InstalledDirectory(), programName)
	if _, err := os.Stat(executable); err != nil {
		executable = Executable()
	}
	icon := filepath.Join(InstalledDirectory(), "byog.ico")
	if _, err := os.Stat(icon); err != nil {
		icon = filepath.Join(Directory(), "byog.ico")
	}

	if _, managed := InnoUninstaller(); managed || innoEntryExists() {
		removeOwnUninstallEntry()
		writeAppPathsEntry(executable)
		return nil
	}

	writeUninstallEntry(displayName, version, executable, icon)
	writeAppPathsEntry(executable)
	writeStartupEntry(displayName, executable)
	return nil
}

// writeStartupEntry writes the entry that starts the client with the system.
//
// The client works in the background, it starts with the system always.
func writeStartupEntry(displayName string, executable string) {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, startupKey, registry.ALL_ACCESS)
	if err != nil {
		return
	}
	defer key.Close()

	if names, err := key.ReadValueNames(-1); err == nil {
		targetBase := strings.ToLower(filepath.Base(executable))
		for _, name := range names {
			if strings.EqualFold(name, displayName) {
				continue
			}
			val, _, err := key.GetStringValue(name)
			if err == nil {
				lowerVal := strings.ToLower(val)
				if strings.Contains(lowerVal, targetBase) || strings.Contains(strings.ToLower(name), "byog") {
					_ = key.DeleteValue(name)
				}
			}
		}
	}

	_ = key.SetStringValue(displayName, "\""+executable+"\" --tray")
}

// Uninstall removes the entries of the client from the system.
//
// An installation of the installer of the system is removed by the uninstaller
// of the installation: it removes the program, the shortcuts, the entries of
// the registry and the entry of the applications of the control panel. The
// client removes its own entries and the program before the uninstaller runs,
// a second entry of the client in the applications would stay behind.
func Uninstall() error {
	removeOwnUninstallEntry()
	removeStartupEntries()
	removeShortcut()

	for _, root := range []registry.Key{registry.CURRENT_USER, registry.LOCAL_MACHINE} {
		deleteKeyTree(root, innoUninstallKey)
		deleteKeyTree(root, appPathsKey)
	}

	if uninstaller, ok := InnoUninstaller(); ok {
		// The uninstaller of the installation removes the program, the
		// shortcuts and the entries of the system. It cannot remove a program
		// that runs, therefore the uninstaller starts after the client ended.
		if scheduleUninstaller(uninstaller) {
			return nil
		}
	}
	scheduleDirectoryRemoval(InstalledDirectory())
	return nil
}

// scheduleUninstaller starts the uninstaller of the installation after the
// client ended.
//
// A running program holds its file open, the uninstaller of the installation
// waits for the end of the client before the file is removed.
func scheduleUninstaller(uninstaller string) bool {
	script := filepath.Join(os.TempDir(), "byog-uninstall-inno.cmd")
	content := "@echo off\r\n" +
		"ping -n 4 127.0.0.1 > nul\r\n" +
		"start \"\" \"" + uninstaller + "\" /SILENT /NORESTART /SUPPRESSMSGBOXES\r\n" +
		"del /f /q \"%~f0\"\r\n"
	if err := os.WriteFile(script, []byte(content), 0644); err != nil {
		return false
	}
	command := exec.Command("cmd", "/c", "start", "", "/min", script)
	hideProcessWindow(command)
	return command.Start() == nil
}

// scheduleDirectoryRemoval removes the program directory after the client
// ended.
//
// A running program cannot be removed by itself, the system runs a small
// script after the program ended. The script lives in the temporary directory
// of the user: a script inside of the directory that it removes cannot delete
// itself.
func scheduleDirectoryRemoval(directory string) {
	if strings.TrimSpace(directory) == "" {
		return
	}
	script := filepath.Join(os.TempDir(), "byog-uninstall.cmd")
	content := "@echo off\r\n" +
		"ping -n 4 127.0.0.1 > nul\r\n" +
		"taskkill /f /im \"" + programName + "\" > nul 2>&1\r\n" +
		"ping -n 3 127.0.0.1 > nul\r\n" +
		"rmdir /s /q \"" + directory + "\"\r\n" +
		"del /f /q \"%~f0\"\r\n"
	if err := os.WriteFile(script, []byte(content), 0644); err != nil {
		return
	}
	command := exec.Command("cmd", "/c", "start", "", "/min", script)
	hideProcessWindow(command)
	_ = command.Start()
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

// UpdateUninstallerLanguage syncs the language of the Inno Setup uninstaller and registry.
func UpdateUninstallerLanguage(lang string) {
	innoLang := "english"
	switch lang {
	case "tr":
		innoLang = "turkish"
	case "zh_CN":
		innoLang = "zhcn"
	case "zh_TW":
		innoLang = "zhtw"
	case "fr":
		innoLang = "french"
	case "ru":
		innoLang = "russian"
	case "ja":
		innoLang = "japanese"
	case "vi":
		innoLang = "vietnamese"
	}
	for _, root := range []registry.Key{registry.CURRENT_USER, registry.LOCAL_MACHINE} {
		key, err := registry.OpenKey(root, innoUninstallKey, registry.SET_VALUE)
		if err != nil {
			continue
		}
		_ = key.SetStringValue("Inno Setup: Selected Language", innoLang)
		key.Close()
	}
	if key, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\BYOG Worker`, registry.SET_VALUE); err == nil {
		_ = key.SetStringValue("Language", lang)
		key.Close()
	}
}

// updateInstallationName writes the name of the installation into the entry of
// the installation.
//
// The installer of the system writes its own entry, the name of the installation
// is written into that entry. An installation of the system never gets a second
// entry: the client removes the entry of an older version of the client and it
// keeps the entry of the installer.
func updateInstallationName(displayName string, version string) error {
	if _, managed := InnoUninstaller(); managed || innoEntryExists() {
		removeOwnUninstallEntry()
		updateInnoEntry(displayName, version)
		return nil
	}
	return Install(displayName, version)
}

// updateInnoEntry writes the name and the version of the installation into the
// entry of the installer of the system.
//
// The entry of an installation of a user who is not an administrator is in the
// key of the user, the entry of an installation of an administrator is in the
// key of the computer: both keys are written, the key that does not exist is
// skipped.
func updateInnoEntry(displayName string, version string) {
	for _, root := range []registry.Key{registry.CURRENT_USER, registry.LOCAL_MACHINE} {
		key, err := registry.OpenKey(root, innoUninstallKey, registry.SET_VALUE)
		if err != nil {
			continue
		}
		_ = key.SetStringValue("DisplayName", displayName)
		_ = key.SetStringValue("DisplayVersion", version)
		key.Close()
	}
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
		return
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
	hideProcessWindow(command)
	_ = command.Run()
}

// IsInstalled reports whether the client is installed on the computer.
//
// The installer of the system writes the uninstaller into the directory of the
// client, the client that installed itself writes its entry into the registry.
func IsInstalled() bool {
	if _, managed := InnoUninstaller(); managed {
		return true
	}
	if innoEntryExists() {
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

// writeUninstallEntry writes the entry of a client that installed itself in the
// applications of the control panel.
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
	_ = key.SetStringValue("InstallLocation", filepath.Dir(executable))
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

// removeOwnUninstallEntry removes the entry of a client that installed itself.
func removeOwnUninstallEntry() {
	deleteKeyTree(registry.CURRENT_USER, uninstallKey)
	deleteKeyTree(registry.LOCAL_MACHINE, uninstallKey)
}

// removeStartupEntries removes the values that start the client with the system.
func removeStartupEntries() {
	for _, root := range []registry.Key{registry.CURRENT_USER, registry.LOCAL_MACHINE} {
		key, err := registry.OpenKey(root, startupKey, registry.SET_VALUE|registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		if names, err := key.ReadValueNames(-1); err == nil {
			for _, name := range names {
				value, _, err := key.GetStringValue(name)
				if err != nil {
					continue
				}
				if strings.Contains(strings.ToLower(value), strings.ToLower(programName)) ||
					strings.Contains(strings.ToLower(name), "byog") {
					_ = key.DeleteValue(name)
				}
			}
		}
		key.Close()
	}
}

// removeShortcut removes the shortcut of the start menu.
func removeShortcut() {
	directory := filepath.Dir(ShortcutPath())
	entries, err := os.ReadDir(directory)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.ToLower(entry.Name())
		if !strings.HasSuffix(name, ".lnk") {
			continue
		}
		if strings.Contains(name, "byog") {
			_ = os.Remove(filepath.Join(directory, entry.Name()))
		}
	}
}

// deleteKeyTree removes a key of the registry and the keys under it.
//
// The system refuses to remove a key that has keys under it, therefore the keys
// are removed from the deepest one.
func deleteKeyTree(root registry.Key, path string) {
	if strings.TrimSpace(path) == "" {
		return
	}
	key, err := registry.OpenKey(root, path, registry.ENUMERATE_SUB_KEYS|registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return
	}
	children, _ := key.ReadSubKeyNames(-1)
	key.Close()
	for _, child := range children {
		deleteKeyTree(root, path+`\`+child)
	}
	_ = registry.DeleteKey(root, path)
}
