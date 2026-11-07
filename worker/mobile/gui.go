package main

// The interface of the client of the phone.
//
// The client serves a small page on the phone, the page shows the state of the
// worker and it pairs the client with the installation. The page is opened with
// the graphical interface of Termux.

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

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
		State.Mu.Lock()
		server := strings.TrimRight(State.Config.ServerUrl, "/")
		nodeId := State.Config.NodeId
		State.Mu.Unlock()
		if server == "" {
			server = "https://pixrouter.com"
		}
		if !strings.HasPrefix(server, "http://") && !strings.HasPrefix(server, "https://") {
			server = "https://" + server
		}
		target := server + "/client"
		if nodeId != "" {
			target += "?node=" + url.QueryEscape(nodeId)
		}
		http.Redirect(writer, request, target, http.StatusTemporaryRedirect)
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

// openInterface opens the page of the client directly on the site.
func openInterface(address string) {
	State.Mu.Lock()
	server := strings.TrimRight(State.Config.ServerUrl, "/")
	nodeId := State.Config.NodeId
	State.Mu.Unlock()
	if server == "" {
		server = "https://pixrouter.com"
	}
	if !strings.HasPrefix(server, "http://") && !strings.HasPrefix(server, "https://") {
		server = "https://" + server
	}
	target := server + "/client"
	if nodeId != "" {
		target += "?node=" + url.QueryEscape(nodeId)
	}

	var candidates [][]string
	if runtime.GOOS == "android" {
		candidates = [][]string{
			{"termux-open-url", target},
			{"termux-open-url", address},
		}
	} else {
		candidates = [][]string{{"xdg-open", target}, {"xdg-open", address}}
	}
	for _, candidate := range candidates {
		command := exec.Command(candidate[0], candidate[1:]...)
		if err := command.Start(); err == nil {
			return
		}
	}
	fmt.Println("[BYOG MOBILE] The page of the client can be opened with", target)
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
