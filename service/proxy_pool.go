package service

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// Proxy pool assignments are read on every relayed request. Instead of
// querying MySQL each time, the mapping is cached in memory and refreshed
// on writes (InvalidateProxyPoolCache) or after a short TTL so key-level
// proxy changes propagate quickly without hammering the database.
const proxyPoolCacheTTL = 30 * time.Second

var (
	proxyPoolMu         sync.RWMutex
	proxyPoolMap        map[int]map[int]string // channel_id -> key_index -> proxy url
	proxyPoolEnabled    []model.ProxyPoolProxy // sorted by ID for stable hashing
	proxyPoolExpiresAt  time.Time
)

// InvalidateProxyPoolCache drops the cached mapping. Call it after every
// proxy or assignment write so the next request sees the new state.
func InvalidateProxyPoolCache() {
	proxyPoolMu.Lock()
	proxyPoolMap = nil
	proxyPoolEnabled = nil
	proxyPoolExpiresAt = time.Time{}
	proxyPoolMu.Unlock()
}

// PickStickyProxy deterministically maps any key string to an enabled proxy
// using FNV-1a hash modulo the number of enabled proxies. As long as that
// proxy is not deleted or disabled, the same key string always maps to the
// exact same proxy.
func PickStickyProxy(key string, enabledProxies []model.ProxyPoolProxy) *model.ProxyPoolProxy {
	if len(enabledProxies) == 0 {
		return nil
	}
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return &enabledProxies[0]
	}
	h := fnv.New32a()
	h.Write([]byte(trimmed))
	idx := int(h.Sum32() % uint32(len(enabledProxies)))
	return &enabledProxies[idx]
}

func loadProxyPoolData() (map[int]map[int]string, []model.ProxyPoolProxy) {
	proxyPoolMu.RLock()
	if proxyPoolMap != nil && time.Now().Before(proxyPoolExpiresAt) {
		defer proxyPoolMu.RUnlock()
		return proxyPoolMap, proxyPoolEnabled
	}
	proxyPoolMu.RUnlock()

	proxyPoolMu.Lock()
	defer proxyPoolMu.Unlock()
	if proxyPoolMap != nil && time.Now().Before(proxyPoolExpiresAt) {
		return proxyPoolMap, proxyPoolEnabled
	}
	m, err := model.ProxyPoolAssignmentURLMap()
	if err != nil {
		if proxyPoolMap == nil {
			proxyPoolMap = map[int]map[int]string{}
			proxyPoolEnabled = nil
			proxyPoolExpiresAt = time.Now().Add(5 * time.Second)
		}
		return proxyPoolMap, proxyPoolEnabled
	}

	var enabled []model.ProxyPoolProxy
	allProxies, pErr := model.ListProxyPoolProxies()
	if pErr == nil {
		for _, p := range allProxies {
			if p.Enabled && strings.TrimSpace(p.URL) != "" {
				enabled = append(enabled, p)
			}
		}
		sort.Slice(enabled, func(i, j int) bool {
			return enabled[i].Id < enabled[j].Id
		})
	}

	proxyPoolMap = m
	proxyPoolEnabled = enabled
	proxyPoolExpiresAt = time.Now().Add(proxyPoolCacheTTL)
	return proxyPoolMap, proxyPoolEnabled
}

// EffectiveRelayProxyURL resolves which proxy URL the relay should use for a
// specific (channel, key index, key string) tuple.
// 1. If global proxy pool is disabled (ProxyPoolEnabled != "true"), always use the channel's proxy setting.
// 2. If an explicit proxy pool assignment exists for (channelId, keyIndex), it wins.
// 3. Otherwise, use the channel's own proxy setting (or direct if empty).
// Unassigned channels are never hijacked by arbitrary proxies.
func EffectiveRelayProxyURL(channelId, keyIndex int, keyString string, channelProxy string) string {
	if common.OptionMap["ProxyPoolEnabled"] != "true" {
		return channelProxy
	}

	urlMap, _ := loadProxyPoolData()

	if channelId > 0 && keyIndex >= 0 {
		if channelKeys, ok := urlMap[channelId]; ok {
			if url, ok := channelKeys[keyIndex]; ok && url != "" {
				return url
			}
		}
	}

	return channelProxy
}

// IsCloudflareWorkerProxy reports whether a proxy URL represents a Cloudflare Worker
// proxy that expects requests via X-Target-URL rather than HTTP CONNECT tunneling.
func IsCloudflareWorkerProxy(proxyURL string) bool {
	trimmed := strings.TrimSpace(proxyURL)
	if trimmed == "" {
		return false
	}
	if strings.Contains(trimmed, ".workers.dev") {
		return true
	}
	_, enabled := loadProxyPoolData()
	for _, p := range enabled {
		if (p.URL == trimmed || strings.TrimRight(p.URL, "/") == strings.TrimRight(trimmed, "/")) && p.ProxyType == "cloudflare_worker" {
			return true
		}
	}
	return false
}

// TestProxyEgress checks that a proxy resolves and can reach the public
// internet, returning the egress IP as seen by a remote service and the
// round-trip duration in milliseconds. The call uses a short context timeout
// so a dead proxy never blocks the admin UI.
func TestProxyEgress(rawProxyURL string) (string, int64, error) {
	trimmed := strings.TrimSpace(rawProxyURL)
	if trimmed == "" {
		return "", 0, fmt.Errorf("empty proxy URL")
	}

	// 1. Dedicated handling for Cloudflare Worker proxies
	if IsCloudflareWorkerProxy(trimmed) {
		workerURL := trimmed
		if !strings.HasPrefix(workerURL, "http://") && !strings.HasPrefix(workerURL, "https://") {
			workerURL = "https://" + workerURL
		}
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, workerURL, nil)
		if err != nil {
			return "", 0, err
		}
		req.Header.Set("X-Target-URL", "https://api.ipify.org?format=json")
		req.Header.Set("User-Agent", "curl/8.4.0")
		started := time.Now()
		client := GetHttpClient()
		if client == nil {
			client = http.DefaultClient
		}
		resp, err := client.Do(req)
		latency := time.Since(started).Milliseconds()
		if err != nil {
			errStr := err.Error()
			if strings.Contains(errStr, "tls:") || strings.Contains(errStr, "handshake") || strings.Contains(workerURL, "workers.dev") {
				return "", latency, fmt.Errorf("workers.dev alan adı servis sağlayıcınız tarafından engelleniyor (Türk Telekom/BTK). Lütfen Cloudflare panelinden Worker'ınıza özel alan adı (Custom Domain, örn: https://proxy.siteniz.com) ekleyerek bu adresi giriniz. Detay: %v", err)
			}
			return "", latency, err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if err != nil {
			return "", latency, err
		}
		bodyStr := string(body)
		if resp.StatusCode == http.StatusTemporaryRedirect || resp.StatusCode == http.StatusMovedPermanently || strings.Contains(bodyStr, "guvenliinternet.turktelekom.com.tr") {
			return "", latency, fmt.Errorf("workers.dev alan adı servis sağlayıcınız tarafından engelleniyor (Türk Telekom/BTK). Lütfen Cloudflare panelinden Worker'ınıza özel alan adı (Custom Domain, örn: https://proxy.siteniz.com) ekleyerek bu adresi giriniz")
		}
		if resp.StatusCode != http.StatusOK {
			return "", latency, fmt.Errorf("worker status %d: %s", resp.StatusCode, bodyStr)
		}
		var parsed struct {
			IP string `json:"ip"`
		}
		if err := json.Unmarshal(body, &parsed); err == nil && parsed.IP != "" {
			return parsed.IP, latency, nil
		}
		return "cloudflare-worker", latency, nil
	}

	// 2. Standard HTTP/HTTPS/SOCKS5 Proxies
	if normURL, err := common.NormalizeProxyLine(rawProxyURL); err == nil && normURL != "" {
		rawProxyURL = normURL
	}
	if _, err := common.ParseProxyURLStrict(rawProxyURL); err != nil {
		return "", 0, err
	}
	client, err := GetHttpClientWithProxy(rawProxyURL)
	if err != nil {
		return "", 0, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.ipify.org?format=json", nil)
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("User-Agent", "curl/8.4.0")
	started := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(started).Milliseconds()
	if err != nil {
		errStr := err.Error()
		if (strings.Contains(errStr, "tls:") || strings.Contains(errStr, "handshake")) && strings.Contains(rawProxyURL, "workers.dev") {
			return "", latency, fmt.Errorf("workers.dev alan adı servis sağlayıcınız tarafından engelleniyor (Türk Telekom/BTK). Lütfen Cloudflare panelinden Worker'ınıza özel alan adı (Custom Domain, örn: https://proxy.siteniz.com) ekleyerek bu adresi giriniz. Detay: %v", err)
		}
		return "", latency, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return "", latency, err
	}
	if resp.StatusCode != http.StatusOK {
		return "", latency, fmt.Errorf("unexpected status %d from egress endpoint", resp.StatusCode)
	}
	var parsed struct {
		IP string `json:"ip"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", latency, fmt.Errorf("unexpected egress response")
	}
	if parsed.IP == "" {
		return "", latency, fmt.Errorf("unexpected egress response")
	}
	return parsed.IP, latency, nil
}

// StartProxyPoolAutoTester periodically runs health/latency checks on registered
// proxies in the pool when enabled via System Settings.
func StartProxyPoolAutoTester() {
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		lastTestTime := time.Now()

		for range ticker.C {
			if common.OptionMap["ProxyPoolEnabled"] != "true" {
				continue
			}

			intervalMin := 15
			if raw := strings.TrimSpace(common.OptionMap["ProxyAutoTestIntervalMinutes"]); raw != "" {
				if v, err := strconv.Atoi(raw); err == nil && v > 0 {
					intervalMin = v
				}
			}

			if time.Since(lastTestTime) < time.Duration(intervalMin)*time.Minute {
				continue
			}
			lastTestTime = time.Now()

			proxies, err := model.ListProxyPoolProxies()
			if err != nil || len(proxies) == 0 {
				continue
			}

			autoDisable := common.OptionMap["ProxyAutoDisableUnhealthy"] == "true"
			for _, p := range proxies {
				egressIP, latency, testErr := TestProxyEgress(p.URL)
				p.LastTestedAt = time.Now().Unix()
				p.LatencyMs = latency
				if testErr != nil {
					p.LastTestStatus = "fail"
					p.LastTestError = testErr.Error()
					p.EgressIP = ""
					if autoDisable {
						p.Enabled = false
					}
				} else {
					p.LastTestStatus = "ok"
					p.LastTestError = ""
					p.EgressIP = egressIP
					if autoDisable && !p.Enabled {
						p.Enabled = true
					}
				}
				_ = model.UpdateProxyPoolProxy(&p)
			}
			InvalidateProxyPoolCache()
		}
	}()
}
