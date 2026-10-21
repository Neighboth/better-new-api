package main

// The interface of the client of the phone.
//
// The client serves a small page on the phone, the page shows the state of the
// worker and it pairs the client with the installation. The page is opened with
// the graphical interface of Termux.

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
)

// interfaceStatus is the state of the worker that the page shows.
type interfaceStatus struct {
	AppName      string
	ServerUrl    string
	DeviceName   string
	NodeId       string
	PairingCode  string
	Connected    bool
	ModelStatus  string
	DownloadedMB int
	TotalMB      int
	Progress     int
	CurrentTps   float64
	TotalTokens  int64
	MemoryBudget int
	PhysicalRam  int
	BatteryLevel int
	IsCharging   bool
	NetworkType  string
}

// interfaceTemplate is the page of the client.
var interfaceTemplate = template.Must(template.New("interface").Parse(`<!doctype html>
<html lang="tr">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>BYOG Worker</title>
<style>
  body { font-family: system-ui, sans-serif; background: #020817; color: #e6edf7; margin: 0; padding: 20px; }
  h1 { font-size: 20px; margin: 0 0 18px; }
  .card { background: #0d1730; border: 1px solid #1e2d52; border-radius: 14px; padding: 16px; margin-bottom: 14px; }
  .row { display: flex; justify-content: space-between; padding: 6px 0; border-bottom: 1px solid #16224a; }
  .row:last-child { border-bottom: none; }
  .ok { color: #35d07f; } .bad { color: #ff7b72; } .busy { color: #f2cc60; }
  input { background: #0a1330; border: 1px solid #2a3b66; color: #e6edf7; border-radius: 8px; padding: 10px; width: 100%; box-sizing: border-box; }
  button { background: #2f6df6; border: none; color: white; border-radius: 8px; padding: 11px 18px; cursor: pointer; }
  form { display: flex; gap: 10px; margin-top: 10px; }
</style>
</head>
<body>
<h1>BYOG Worker</h1>
<div class="card">
  <div class="row"><span>Durum</span><span>{{if .Connected}}<span class="ok">Çevrimiçi</span>{{else}}<span class="bad">Bağlı değil</span>{{end}}</span></div>
  <div class="row"><span>Sunucu</span><span>{{.ServerUrl}}</span></div>
  <div class="row"><span>Cihaz</span><span>{{.DeviceName}}</span></div>
  <div class="row"><span>Model</span><span>{{.ModelStatus}}</span></div>
  {{if eq .ModelStatus "downloading"}}<div class="row"><span>İndirme</span><span class="busy">{{.DownloadedMB}} / {{.TotalMB}} MB (%{{.Progress}})</span></div>{{end}}
  <div class="row"><span>Bellek</span><span>{{.MemoryBudget}} MB / {{.PhysicalRam}} MB</span></div>
  <div class="row"><span>Pil</span><span>%{{.BatteryLevel}}{{if .IsCharging}} (şarj){{end}}</span></div>
  <div class="row"><span>Ağ</span><span>{{.NetworkType}}</span></div>
  <div class="row"><span>Token</span><span>{{.TotalTokens}}</span></div>
</div>
<div class="card">
  <strong>Eşleştirme kodu</strong>
  <form method="post" action="/pair">
    <input name="code" placeholder="XXXX-XXXX-XXXX-XXXX" value="{{.PairingCode}}">
    <button type="submit">Bağlan</button>
  </form>
</div>
</body>
</html>`))

// startInterface starts the page of the client and opens it.
func startInterface() {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Println("[BYOG MOBILE] The interface could not be started:", err)
		return
	}
	address := "http://" + listener.Addr().String()
	fmt.Println("[BYOG MOBILE] The interface of the client:", address)

	go func() {
		time.Sleep(700 * time.Millisecond)
		openInterface(address)
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = interfaceTemplate.Execute(writer, currentInterfaceStatus())
	})
	mux.HandleFunc("/pair", func(writer http.ResponseWriter, request *http.Request) {
		if err := request.ParseForm(); err == nil {
			code := strings.TrimSpace(request.FormValue("code"))
			if code != "" {
				State.Mu.Lock()
				State.Config.PairingCode = strings.ToUpper(code)
				State.Mu.Unlock()
				registerWithServer()
			}
		}
		http.Redirect(writer, request, "/", http.StatusSeeOther)
	})

	server := &http.Server{Handler: mux}
	go func() {
		_ = server.Serve(listener)
	}()
}

// currentInterfaceStatus reads the state of the worker for the page.
func currentInterfaceStatus() interfaceStatus {
	State.Mu.Lock()
	status := interfaceStatus{
		AppName:      "BYOG Worker",
		ServerUrl:    State.Config.ServerUrl,
		DeviceName:   State.Config.DeviceName,
		NodeId:       State.Config.NodeId,
		PairingCode:  State.Config.PairingCode,
		Connected:    State.IsConnected,
		ModelStatus:  State.ModelStatus,
		DownloadedMB: State.DownloadedMB,
		TotalMB:      State.TotalMB,
		Progress:     State.DownloadProgress,
		CurrentTps:   State.CurrentTps,
		TotalTokens:  State.TotalTokens,
		MemoryBudget: State.Memory.SafeBudgetMB,
		PhysicalRam:  State.Memory.PhysicalTotalMB,
		BatteryLevel: State.BatteryPercent,
		IsCharging:   State.IsCharging,
		NetworkType:  State.NetworkType,
	}
	State.Mu.Unlock()
	return status
}

// openInterface opens the page of the client.
func openInterface(address string) {
	var candidates [][]string
	if runtime.GOOS == "android" {
		candidates = [][]string{
			{"termux-gui", address},
			{"termux-open-url", address},
		}
	} else {
		candidates = [][]string{{"xdg-open", address}}
	}
	for _, candidate := range candidates {
		command := exec.Command(candidate[0], candidate[1:]...)
		if err := command.Start(); err == nil {
			return
		}
	}
	fmt.Println("[BYOG MOBILE] The page of the client can be opened with", address)
}

// portOf returns the port of an address.
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

// interfaceAvailable reports whether the page of the client can be opened.
func interfaceAvailable() bool {
	if runtime.GOOS != "android" {
		return true
	}
	if _, err := os.Stat("/data/data/com.termux/files/usr/bin/termux-gui"); err == nil {
		return true
	}
	return false
}
