package service

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// PsiphonRuntimeStatus captures the live tunnel status for an OpenCode channel.
type PsiphonRuntimeStatus struct {
	Enabled   bool   `json:"enabled"`
	Status    string `json:"status"` // stopped | starting | connected | rotating | error
	ProxyURL  string `json:"proxy_url"`
	EgressIP  string `json:"egress_ip"`
	Region    string `json:"region"`
	LastError string `json:"last_error"`
}

var psiphonRotationRegions = []string{
	"", "US", "GB", "DE", "NL", "CA", "FR", "SG", "JP", "CH", "SE", "AT", "BE", "ES", "IT", "PL",
}

type psiphonTunnelInstance struct {
	channelID   int
	cmd         *exec.Cmd
	cancel      context.CancelFunc
	socksPort   int
	regionIndex int
	region      string
	status      string
	egressIP    string
	lastError   string
	rotatedAt   time.Time
	connectedCh chan struct{}
	onceReady   sync.Once
}

var (
	psiphonMu        sync.Mutex
	psiphonInstances = map[int]*psiphonTunnelInstance{}
	psiphonRotateMu  sync.Map // map[int]*sync.Mutex
)

func lockPsiphonChannelRotate(channelID int) func() {
	val, _ := psiphonRotateMu.LoadOrStore(channelID, &sync.Mutex{})
	mu := val.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

func psiphonChannelDataDir(channelID int) string {
	dir := filepath.Join("psiphon", fmt.Sprintf("channel_%d", channelID))
	if absDir, err := filepath.Abs(dir); err == nil {
		return absDir
	}
	return dir
}

// findPsiphonBinary locates the psiphon-tunnel-core executable on the host.
func findPsiphonBinary() (string, error) {
	if envPath := strings.TrimSpace(os.Getenv("PSIPHON_BINARY_PATH")); envPath != "" {
		if _, err := os.Stat(envPath); err == nil {
			return envPath, nil
		}
	}
	candidates := []string{
		"psiphon-tunnel-core",
		"psiphon-tunnel-core-x86_64",
		"psiphon",
	}
	if runtime.GOOS == "windows" {
		candidates = append([]string{"psiphon-tunnel-core.exe", "psiphon.exe"}, candidates...)
	}
	for _, name := range candidates {
		if p, err := exec.LookPath(name); err == nil {
			return p, nil
		}
	}
	localPaths := []string{
		filepath.Join("data", "bin", "psiphon-tunnel-core"),
		filepath.Join("data", "bin", "psiphon-tunnel-core.exe"),
		"/usr/local/bin/psiphon-tunnel-core",
		"/usr/bin/psiphon-tunnel-core",
	}
	for _, lp := range localPaths {
		if _, err := os.Stat(lp); err == nil {
			return lp, nil
		}
	}
	return "", fmt.Errorf("psiphon-tunnel-core binary not found (install psiphon-tunnel-core in PATH, /usr/local/bin/psiphon-tunnel-core, or set PSIPHON_BINARY_PATH)")
}

func allocateLocalPort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	addr := ln.Addr().(*net.TCPAddr)
	port := addr.Port
	_ = ln.Close()
	return port, nil
}

const (
	defaultPsiphonPropagationChannelID    = "FFFFFFFFFFFFFFFF"
	defaultPsiphonSponsorID               = "FFFFFFFFFFFFFFFF"
	defaultPsiphonRemoteServerListURL     = "https://s3.amazonaws.com//psiphon/web/mjr4-p23r-puwl/server_list_compressed"
	defaultPsiphonRemoteServerListFile    = "remote_server_list"
	defaultPsiphonRemoteServerListPubKey  = "MIICIDANBgkqhkiG9w0BAQEFAAOCAg0AMIICCAKCAgEAt7Ls+/39r+T6zNW7GiVpJfzq/xvL9SBH5rIFnk0RXYEYavax3WS6HOD35eTAqn8AniOwiH+DOkvgSKF2caqk/y1dfq47Pdymtwzp9ikpB1C5OfAysXzBiwVJlCdajBKvBZDerV1cMvRzCKvKwRmvDmHgphQQ7WfXIGbRbmmk6opMBh3roE42KcotLFtqp0RRwLtcBRNtCdsrVsjiI1Lqz/lH+T61sGjSjQ3CHMuZYSQJZo/KrvzgQXpkaCTdbObxHqb6/+i1qaVOfEsvjoiyzTxJADvSytVtcTjijhPEV6XskJVHE1Zgl+7rATr/pDQkw6DPCNBS1+Y6fy7GstZALQXwEDN/qhQI9kWkHijT8ns+i1vGg00Mk/6J75arLhqcodWsdeG/M/moWgqQAnlZAGVtJI1OgeF5fsPpXu4kctOfuZlGjVZXQNW34aOzm8r8S0eVZitPlbhcPiR4gT/aSMz/wd8lZlzZYsje/Jr8u/YtlwjjreZrGRmG8KMOzukV3lLmMppXFMvl4bxv6YFEmIuTsOhbLTwFgh7KYNjodLj/LsqRVfwz31PgWQFTEPICV7GCvgVlPRxnofqKSjgTWI4mxDhBpVcATvaoBl1L/6WLbFvBsoAUBItWwctO2xalKxF5szhGm8lccoc5MZr8kfE0uxMgsxz4er68iCID+rsCAQM="
	defaultPsiphonClientPlatform          = "Android_4.0.4_com.example.exampleClientLibraryApp"
)

func writePsiphonConfig(dir string, socksPort int, region string) (string, error) {
	absDir, err := filepath.Abs(dir)
	if err == nil {
		dir = absDir
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	cfgPath := filepath.Join(dir, "psiphon.config")

	// Allow operators to supply a custom psiphon.config template via env.
	cfgMap := map[string]any{}
	if customPath := strings.TrimSpace(os.Getenv("PSIPHON_CONFIG_PATH")); customPath != "" {
		if raw, err := os.ReadFile(customPath); err == nil {
			_ = common.Unmarshal(raw, &cfgMap)
		}
	}

	if _, ok := cfgMap["PropagationChannelId"]; !ok {
		propID := strings.TrimSpace(os.Getenv("PSIPHON_PROPAGATION_CHANNEL_ID"))
		if propID == "" {
			propID = defaultPsiphonPropagationChannelID
		}
		cfgMap["PropagationChannelId"] = propID
	}
	if _, ok := cfgMap["SponsorId"]; !ok {
		sponsorID := strings.TrimSpace(os.Getenv("PSIPHON_SPONSOR_ID"))
		if sponsorID == "" {
			sponsorID = defaultPsiphonSponsorID
		}
		cfgMap["SponsorId"] = sponsorID
	}
	if _, ok := cfgMap["RemoteServerListUrl"]; !ok {
		cfgMap["RemoteServerListUrl"] = defaultPsiphonRemoteServerListURL
	}
	if _, ok := cfgMap["RemoteServerListDownloadFilename"]; !ok {
		cfgMap["RemoteServerListDownloadFilename"] = defaultPsiphonRemoteServerListFile
	}
	if _, ok := cfgMap["RemoteServerListSignaturePublicKey"]; !ok {
		cfgMap["RemoteServerListSignaturePublicKey"] = defaultPsiphonRemoteServerListPubKey
	}
	if _, ok := cfgMap["ClientPlatform"]; !ok {
		cfgMap["ClientPlatform"] = defaultPsiphonClientPlatform
	}
	if _, ok := cfgMap["ClientVersion"]; !ok {
		cfgMap["ClientVersion"] = "1"
	}
	if _, ok := cfgMap["NetworkID"]; !ok {
		cfgMap["NetworkID"] = "test"
	}

	cfgMap["LocalSocksProxyPort"] = socksPort
	cfgMap["DisableLocalHTTPProxy"] = true
	cfgMap["DataRootDirectory"] = dir
	cfgMap["TunnelPoolSize"] = 1
	cfgMap["EstablishTunnelTimeoutSeconds"] = 60
	cfgMap["UseIndistinguishableTLS"] = true
	if region != "" {
		cfgMap["EgressRegion"] = region
	} else {
		delete(cfgMap, "EgressRegion")
	}

	encoded, err := common.Marshal(cfgMap)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(cfgPath, encoded, 0o600); err != nil {
		return "", err
	}
	return cfgPath, nil
}

func startPsiphonInstanceLocked(channelID int, regionIdx int) (*psiphonTunnelInstance, error) {
	binPath, err := findPsiphonBinary()
	if err != nil {
		_ = model.SetChannelPsiphon(channelID, true, "", "error", "", err.Error())
		return nil, err
	}
	port, err := allocateLocalPort()
	if err != nil {
		_ = model.SetChannelPsiphon(channelID, true, "", "error", "", err.Error())
		return nil, err
	}
	region := psiphonRotationRegions[regionIdx%len(psiphonRotationRegions)]
	dataDir := psiphonChannelDataDir(channelID)
	cfgPath, err := writePsiphonConfig(dataDir, port, region)
	if err != nil {
		_ = model.SetChannelPsiphon(channelID, true, region, "error", "", err.Error())
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, binPath, "-config", cfgPath, "-dataRootDirectory", dataDir)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, err
	}

	inst := &psiphonTunnelInstance{
		channelID:   channelID,
		cmd:         cmd,
		cancel:      cancel,
		socksPort:   port,
		regionIndex: regionIdx,
		region:      region,
		status:      "starting",
		connectedCh: make(chan struct{}),
	}

	if err := cmd.Start(); err != nil {
		cancel()
		_ = model.SetChannelPsiphon(channelID, true, region, "error", "", err.Error())
		return nil, fmt.Errorf("failed to start psiphon-tunnel-core: %w", err)
	}

	_ = model.SetChannelPsiphon(channelID, true, region, "starting", "", "")
	psiphonInstances[channelID] = inst

	readNotices := func(r io.Reader) {
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 0, 16*1024), 512*1024)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var notice struct {
				NoticeType string         `json:"noticeType"`
				Data       map[string]any `json:"data"`
			}
			if common.UnmarshalJsonStr(line, &notice) != nil {
				continue
			}
			switch notice.NoticeType {
			case "ListeningSocksProxyPort":
				if p, ok := notice.Data["port"].(float64); ok && int(p) > 0 {
					psiphonMu.Lock()
					inst.socksPort = int(p)
					psiphonMu.Unlock()
				}
			case "Tunnels":
				if cnt, ok := notice.Data["count"].(float64); ok && int(cnt) >= 1 {
					psiphonMu.Lock()
					inst.status = "connected"
					inst.lastError = ""
					proxyURL := fmt.Sprintf("socks5://127.0.0.1:%d", inst.socksPort)
					curRegion := inst.region
					psiphonMu.Unlock()
					inst.onceReady.Do(func() { close(inst.connectedCh) })
					_ = model.SetChannelPsiphon(channelID, true, curRegion, "connected", "", "")
					go probePsiphonEgressIP(channelID, inst, proxyURL)
				}
			case "ClientRegion":
				if reg, ok := notice.Data["region"].(string); ok && reg != "" {
					psiphonMu.Lock()
					if inst.region == "" {
						inst.region = reg
					}
					psiphonMu.Unlock()
				}
			case "EstablishTunnelTimeout":
				psiphonMu.Lock()
				inst.status = "error"
				if inst.lastError == "" {
					inst.lastError = "psiphon tunnel timed out before connecting"
				}
				curRegion := inst.region
				errStr := inst.lastError
				psiphonMu.Unlock()
				inst.onceReady.Do(func() { close(inst.connectedCh) })
				_ = model.SetChannelPsiphon(channelID, true, curRegion, "error", "", errStr)
			case "Alert":
				if msg, ok := notice.Data["message"].(string); ok && msg != "" {
					psiphonMu.Lock()
					inst.lastError = truncatePsiphonErr(msg)
					psiphonMu.Unlock()
				}
			}
		}
	}

	go readNotices(stderr)
	go readNotices(stdout)
	go func() {
		waitErr := cmd.Wait()
		psiphonMu.Lock()
		current := psiphonInstances[channelID]
		if current == inst {
			if ctx.Err() == nil {
				inst.status = "error"
				if waitErr != nil && inst.lastError == "" {
					inst.lastError = truncatePsiphonErr(waitErr.Error())
				}
				_ = model.SetChannelPsiphon(channelID, true, inst.region, "error", inst.egressIP, inst.lastError)
			}
		}
		psiphonMu.Unlock()
		inst.onceReady.Do(func() { close(inst.connectedCh) })
	}()

	return inst, nil
}

func truncatePsiphonErr(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 300 {
		return s[:300]
	}
	return s
}

func probePsiphonEgressIPSync(channelID int, inst *psiphonTunnelInstance, proxyURL string, timeout time.Duration) string {
	client, err := GetHttpClientWithProxy(proxyURL)
	if err != nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.ipify.org?format=json", nil)
	if err != nil {
		return ""
	}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if err != nil {
		return ""
	}
	var parsed struct {
		IP string `json:"ip"`
	}
	if common.Unmarshal(body, &parsed) == nil && parsed.IP != "" {
		psiphonMu.Lock()
		if psiphonInstances[channelID] == inst {
			inst.egressIP = parsed.IP
			reg := inst.region
			st := inst.status
			psiphonMu.Unlock()
			_ = model.SetChannelPsiphon(channelID, true, reg, st, parsed.IP, "")
			return parsed.IP
		}
		psiphonMu.Unlock()
	}
	return ""
}

func probePsiphonEgressIP(channelID int, inst *psiphonTunnelInstance, proxyURL string) {
	_ = probePsiphonEgressIPSync(channelID, inst, proxyURL, 10*time.Second)
}

func stopPsiphonInstanceLocked(channelID int) {
	inst, ok := psiphonInstances[channelID]
	if !ok || inst == nil {
		return
	}
	delete(psiphonInstances, channelID)
	if inst.cancel != nil {
		inst.cancel()
	}
	if inst.cmd != nil && inst.cmd.Process != nil {
		_ = inst.cmd.Process.Kill()
	}
}

// StartPsiphonForChannel enables and launches a psiphon-tunnel-core proxy for an OpenCode channel.
func StartPsiphonForChannel(channelID int) (PsiphonRuntimeStatus, error) {
	psiphonMu.Lock()
	existing := psiphonInstances[channelID]
	if existing != nil && (existing.status == "connected" || existing.status == "starting") {
		st := PsiphonRuntimeStatus{
			Enabled:   true,
			Status:    existing.status,
			ProxyURL:  fmt.Sprintf("socks5://127.0.0.1:%d", existing.socksPort),
			EgressIP:  existing.egressIP,
			Region:    existing.region,
			LastError: existing.lastError,
		}
		psiphonMu.Unlock()
		return st, nil
	}
	stopPsiphonInstanceLocked(channelID)
	inst, err := startPsiphonInstanceLocked(channelID, 0)
	psiphonMu.Unlock()
	if err != nil {
		return PsiphonRuntimeStatus{
			Enabled:   true,
			Status:    "error",
			LastError: err.Error(),
		}, err
	}

	// Wait briefly for initial tunnel handshake so UI shows connected immediately when fast.
	select {
	case <-inst.connectedCh:
	case <-time.After(8 * time.Second):
	}
	return GetPsiphonStatus(channelID), nil
}

// StopPsiphonForChannel terminates the psiphon-tunnel-core process for a channel and clears DB state.
func StopPsiphonForChannel(channelID int) {
	psiphonMu.Lock()
	stopPsiphonInstanceLocked(channelID)
	psiphonMu.Unlock()
	_ = model.DeleteChannelPsiphon(channelID)
}

// RotatePsiphonForChannel reconnects psiphon-tunnel-core to a fresh egress region/IP.
// A per-channel mutex prevents concurrent 429 retries from tearing down each
// other's tunnel mid-handshake. When failedProxyURL is provided and another
// goroutine already rotated the channel to a different connected proxy within
// the last 10 seconds, that fresh proxy is reused directly instead of churning.
func RotatePsiphonForChannel(channelID int, failedProxyURL ...string) (string, error) {
	unlock := lockPsiphonChannelRotate(channelID)
	defer unlock()

	failed := ""
	if len(failedProxyURL) > 0 {
		failed = strings.TrimSpace(failedProxyURL[0])
	}

	psiphonMu.Lock()
	nextRegionIdx := 1
	if cur, ok := psiphonInstances[channelID]; ok && cur != nil {
		curProxy := fmt.Sprintf("socks5://127.0.0.1:%d", cur.socksPort)
		if failed != "" && curProxy != failed && cur.status == "connected" && !cur.rotatedAt.IsZero() && time.Since(cur.rotatedAt) < 10*time.Second {
			psiphonMu.Unlock()
			return curProxy, nil
		}
		nextRegionIdx = (cur.regionIndex + 1) % len(psiphonRotationRegions)
	}
	stopPsiphonInstanceLocked(channelID)
	// Remove cached datastore to encourage a fresh server & egress IP selection
	dataDir := psiphonChannelDataDir(channelID)
	_ = os.Remove(filepath.Join(dataDir, "psiphon.boltdb"))

	inst, err := startPsiphonInstanceLocked(channelID, nextRegionIdx)
	psiphonMu.Unlock()
	if err != nil {
		return "", err
	}

	select {
	case <-inst.connectedCh:
	case <-time.After(25 * time.Second):
	}

	psiphonMu.Lock()
	if psiphonInstances[channelID] != inst {
		psiphonMu.Unlock()
		return "", fmt.Errorf("psiphon instance replaced during rotation")
	}
	if inst.status != "connected" {
		errMsg := inst.lastError
		if errMsg == "" {
			errMsg = fmt.Sprintf("psiphon tunnel did not connect in region %q (status=%s)", inst.region, inst.status)
		}
		psiphonMu.Unlock()
		return "", fmt.Errorf("%s", errMsg)
	}
	inst.rotatedAt = time.Now()
	proxyURL := fmt.Sprintf("socks5://127.0.0.1:%d", inst.socksPort)
	psiphonMu.Unlock()

	// Synchronously probe new egress IP briefly so logs & UI reflect the rotated IP immediately.
	_ = probePsiphonEgressIPSync(channelID, inst, proxyURL, 4*time.Second)
	return proxyURL, nil
}

// GetPsiphonProxyURL returns the active local SOCKS5 proxy URL if Psiphon is assigned to this channel.
func GetPsiphonProxyURL(channelID int) (string, bool) {
	if channelID <= 0 {
		return "", false
	}
	psiphonMu.Lock()
	inst := psiphonInstances[channelID]
	if inst != nil && inst.socksPort > 0 && inst.status != "error" {
		url := fmt.Sprintf("socks5://127.0.0.1:%d", inst.socksPort)
		psiphonMu.Unlock()
		return url, true
	}
	psiphonMu.Unlock()

	if !model.IsChannelPsiphonEnabled(channelID) {
		return "", false
	}
	st, err := StartPsiphonForChannel(channelID)
	if err != nil || st.ProxyURL == "" {
		return "", false
	}
	return st.ProxyURL, true
}

// GetPsiphonStatus returns live runtime status merged with persisted DB state.
func GetPsiphonStatus(channelID int) PsiphonRuntimeStatus {
	psiphonMu.Lock()
	inst := psiphonInstances[channelID]
	if inst != nil {
		st := PsiphonRuntimeStatus{
			Enabled:   true,
			Status:    inst.status,
			ProxyURL:  fmt.Sprintf("socks5://127.0.0.1:%d", inst.socksPort),
			EgressIP:  inst.egressIP,
			Region:    inst.region,
			LastError: inst.lastError,
		}
		psiphonMu.Unlock()
		return st
	}
	psiphonMu.Unlock()

	rec, err := model.GetChannelPsiphon(channelID)
	if err != nil || rec == nil || !rec.Enabled {
		return PsiphonRuntimeStatus{Enabled: false, Status: "stopped"}
	}
	return PsiphonRuntimeStatus{
		Enabled:   rec.Enabled,
		Status:    rec.Status,
		EgressIP:  rec.EgressIP,
		Region:    rec.Region,
		LastError: rec.LastError,
	}
}
