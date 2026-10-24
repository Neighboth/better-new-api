//go:build !windows

package main

// The interface of the clients that are not built for Windows.
//
// The client serves a small page on the computer, the page shows the state of
// the worker and it pairs the client with the installation. The page is opened
// with the browser of the system.

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/worker/desktop/installer"
)

// runGui starts the interface of the client.
func runGui(serverFlag string, codeFlag string, deviceFlag string, trayFlag bool) {
	_ = trayFlag
	if err := os.MkdirAll(storageRoot(), 0755); err != nil {
		_ = err
	}

	config := loadConfig()
	if serverFlag != "" {
		config.ServerUrl = serverFlag
	}
	if codeFlag != "" {
		config.PairingCode = codeFlag
	}
	if deviceFlag != "" {
		config.DeviceName = deviceFlag
	}
	if config.ServerUrl == "" {
		config.ServerUrl = DefaultServerUrl
	}
	if config.DeviceName == "" {
		config.DeviceName = "Worker-Node-" + generateHWID()[:6]
	}
	if config.StorageDir == "" {
		config.StorageDir = modelDirectory()
	}
	if config.HWID == "" {
		config.HWID = generateHWID()
	}
	applyConfig(config)
	saveConfig(config)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Println("The interface of the client could not be started:", err)
		return
	}
	address := "http://" + listener.Addr().String()
	fmt.Println("The interface of the client:", address)

	// The page of the client of the installation lives on the site, it uses the
	// theme of the site and it follows the changes of it. The server of the
	// state of the client gives the state of the client to the page.
	bridge, err := startLocalBridge()
	if err != nil {
		fmt.Println("The server of the state of the client could not be started:", err)
		return
	}

	go func() {
		if config.PairingCode != "" {
			if _, err := registerWorker(); err != nil {
				updateStatus(func(status *AppStatus) { status.LastError = err.Error() })
			}
			startTunnel()
		}
		startBackgroundTasks()
	}()

	go openInterface(bridge.address + "/")

	server := &http.Server{Handler: interfaceHandler(bridge)}
	_ = server.Serve(listener)
}

// interfaceHandler sends the page of the client to the server of the state.
//
// The page of the first start and the page of the installation are served by
// the server of the state. The server of the page stays for the addresses that
// are already known to the user.
func interfaceHandler(bridge *localBridge) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, bridge.address+"/", http.StatusSeeOther)
	})
}

// openInterface opens the page of the client.
func openInterface(address string) {
	time.Sleep(500 * time.Millisecond)
	var candidates [][]string
	switch runtime.GOOS {
	case "darwin":
		candidates = [][]string{{"open", address}}
	case "android":
		candidates = [][]string{{"termux-gui", address}, {"termux-open-url", address}}
	default:
		candidates = [][]string{{"xdg-open", address}}
	}
	for _, candidate := range candidates {
		command := exec.Command(candidate[0], candidate[1:]...)
		if err := command.Start(); err == nil {
			return
		}
	}
	fmt.Println("The page of the client can be opened with", address)
}

// updateInstallationBranding writes the name of the installation into the
// entries of the system.
//
// The icon of the installation belongs to the system of Windows, the other
// systems do not have it.
func updateInstallationBranding(appName string, systemName string, faviconBase64 string) {
	_ = installer.UpdateBranding(installationDisplayName(appName, systemName), WorkerVersion)
}

// portOf returns the port of a server.
func portOf(address string) int {
	_, port, err := net.SplitHostPort(address)
	if err != nil {
		return 0
	}
	number, err := strconv.Atoi(port)
	if err != nil {
		return 0
	}
	return number
}
