//go:build !windows

package main

// The interface of the clients that are not built for Windows.
//
// The client serves a small page on the computer, the page shows the state of
// the worker and it pairs the client with the installation. The page is opened
// with the browser of the system.

import (
	"fmt"
	"html/template"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/worker/desktop/installer"
)

// runGui starts the interface of the client.
func runGui(serverFlag string, codeFlag string, deviceFlag string) {
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

	go func() {
		if config.PairingCode != "" {
			if _, err := registerWorker(); err != nil {
				updateStatus(func(status *AppStatus) { status.LastError = err.Error() })
			}
			startTunnel()
		}
		startBackgroundTasks()
	}()

	go openInterface(address)

	server := &http.Server{Handler: interfaceHandler()}
	_ = server.Serve(listener)
}

// interfaceHandler builds the handler of the page of the client.
func interfaceHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		syncStatus()
		status := readStatus()
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = interfaceTemplate.Execute(writer, status)
	})
	mux.HandleFunc("/pair", func(writer http.ResponseWriter, request *http.Request) {
		code := strings.TrimSpace(request.FormValue("code"))
		server := strings.TrimSpace(request.FormValue("server"))
		if server != "" {
			State.Mu.Lock()
			State.Config.ServerUrl = server
			State.Mu.Unlock()
		}
		if code != "" {
			State.Mu.Lock()
			State.Config.PairingCode = strings.ToUpper(code)
			State.Mu.Unlock()
			saveConfig(State.Config)
			go func() {
				if _, err := registerWorker(); err != nil {
					updateStatus(func(status *AppStatus) { status.LastError = err.Error() })
					return
				}
				startTunnel()
			}()
		}
		http.Redirect(writer, request, "/", http.StatusSeeOther)
	})
	return mux
}

// interfaceTemplate is the page of the client.
var interfaceTemplate = template.Must(template.New("interface").Parse(`<!doctype html>
<html lang="tr">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>BYOG Worker</title>
<style>
  body { font-family: system-ui, sans-serif; background: #020817; color: #e6edf7; margin: 0; padding: 32px; }
  main { max-width: 620px; margin: 0 auto; }
  h1 { font-size: 22px; margin: 0 0 24px; }
  .card { background: #0d1730; border: 1px solid #1e2d52; border-radius: 14px; padding: 20px; margin-bottom: 16px; }
  .row { display: flex; justify-content: space-between; padding: 6px 0; border-bottom: 1px solid #16224a; }
  .row:last-child { border-bottom: none; }
  .ok { color: #35d07f; } .bad { color: #ff7b72; } .busy { color: #f2cc60; }
  input { background: #0a1330; border: 1px solid #2a3b66; color: #e6edf7; border-radius: 8px; padding: 10px; width: 100%; box-sizing: border-box; }
  button { background: #2f6df6; border: none; color: white; border-radius: 8px; padding: 11px 18px; cursor: pointer; }
  form { display: flex; gap: 10px; margin-top: 10px; }
</style>
</head>
<body>
<main>
  <h1>{{.AppName}}</h1>
  <div class="card">
    <div class="row"><span>Durum</span><span>{{if .Connected}}<span class="ok">Çevrimiçi</span>{{else}}<span class="bad">Bağlı değil</span>{{end}}</span></div>
    <div class="row"><span>Sunucu</span><span>{{.ServerUrl}}</span></div>
    <div class="row"><span>Cihaz</span><span>{{.DeviceName}}</span></div>
    <div class="row"><span>Model</span><span>{{if .ModelName}}{{.ModelName}}{{else}}{{.ModelStatus}}{{end}}</span></div>
    {{if eq .ModelStatus "downloading"}}<div class="row"><span>İndirme</span><span class="busy">{{.DownloadedMB}} / {{.TotalMB}} MB</span></div>{{end}}
    {{if .LastError}}<div class="row"><span>Hata</span><span class="bad">{{.LastError}}</span></div>{{end}}
  </div>
  <div class="card">
    <strong>Eşleştirme kodu</strong>
    <form method="post" action="/pair">
      <input name="code" placeholder="XXXX-XXXX-XXXX-XXXX" value="{{.PairingCode}}">
      <button type="submit">Bağlan</button>
    </form>
  </div>
</main>
</body>
</html>`))

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
