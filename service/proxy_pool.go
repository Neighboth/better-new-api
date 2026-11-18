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
	proxyPoolMu        sync.RWMutex
	proxyPoolMap       map[int]map[int]string // channel_id -> key_index -> proxy url
	proxyPoolEnabled   []model.ProxyPoolProxy // sorted by ID for stable hashing
	proxyPoolExpiresAt time.Time
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

var (
	proxy429Cooldown   sync.Map // string -> time.Time
	anonChannelCounter sync.Map // int -> uint64
	poolExhaustedMarks sync.Map // int -> time.Time
)

// RecordPoolExhausted logs (at most once per minute per channel) that every
// assigned proxy for an anonymous channel is on 429 cooldown, or none are
// assigned — i.e. there is no exit left to ride. This is the signal an
// administrator needs to see when an anonymous channel degrades to dead.
func RecordPoolExhausted(channelID int) {
	if channelID <= 0 {
		return
	}
	now := time.Now()
	if last, ok := poolExhaustedMarks.Load(channelID); ok {
		if t, ok := last.(time.Time); ok && now.Sub(t) < time.Minute {
			return
		}
	}
	poolExhaustedMarks.Store(channelID, now)
	model.RecordProxyPoolEvent(channelID, -1, 0, "", "pool_exhausted", "No proxy left to serve this anonymous channel — every assigned proxy is on 429 cooldown (or nothing is assigned)")
}

// RecordProxyRateLimited places a proxy on cooldown following a 429 response.
// Accepts an optional duration (e.g. 30 minutes for OpenCode / Kilo Code); defaults to 60s.
func RecordProxyRateLimited(proxyURL string, duration ...time.Duration) {
	RecordProxyRateLimitedWithChannel(proxyURL, 0, duration...)
}

// RecordProxyRateLimitedWithChannel places a proxy on cooldown following an
// upstream 429 attributed to the given channel, and records the audit event.
// Channel 0 means "pool-level" (no channel context available).
func RecordProxyRateLimitedWithChannel(proxyURL string, channelID int, duration ...time.Duration) {
	trimmed := strings.TrimSpace(proxyURL)
	if trimmed == "" {
		return
	}
	cd := 60 * time.Second
	if len(duration) > 0 && duration[0] > 0 {
		cd = duration[0]
	}
	proxy429Cooldown.Store(trimmed, time.Now().Add(cd))

	// Cooldowns never last longer than the relay state we hold; recording is
	// best-effort and never blocks a request. The proxy row id is resolved
	// from the cached pool data where available instead of querying the DB
	// on every cooldown (the url column is not indexed).
	message := fmt.Sprintf("Proxy placed on 429 cooldown for %s", cd)
	model.RecordProxyPoolEvent(channelID, -1, 0, trimmed, "cooldown_429", message)
}

// IsProxyRateLimited checks whether a proxy is currently within its 429 cooldown period.
func IsProxyRateLimited(proxyURL string) bool {
	trimmed := strings.TrimSpace(proxyURL)
	if trimmed == "" {
		return false
	}
	val, ok := proxy429Cooldown.Load(trimmed)
	if !ok {
		return false
	}
	until, ok := val.(time.Time)
	if !ok {
		return false
	}
	if time.Now().Before(until) {
		return true
	}
	// Lazily evict expired entries.
	proxy429Cooldown.Delete(trimmed)
	return false
}

func proxy429CooldownRemaining(proxyURL string) int64 {
	trimmed := strings.TrimSpace(proxyURL)
	if trimmed == "" {
		return 0
	}
	if val, ok := proxy429Cooldown.Load(trimmed); ok {
		if until, ok := val.(time.Time); ok {
			seconds := time.Until(until).Seconds()
			if seconds > 0 {
				return int64(seconds)
			}
		}
	}
	return 0
}

// ProxyCooldownSnapshot returns the remaining cooldown seconds for every URL
// currently on 429 cooldown, keyed by URL.
func ProxyCooldownSnapshot(urls []string) map[string]int64 {
	out := map[string]int64{}
	for _, url := range urls {
		if seconds := proxy429CooldownRemaining(url); seconds > 0 {
			out[url] = seconds
		}
	}
	return out
}

// GetProxiesForChannel returns the proxies explicitly assigned to a channel.
//
// Anonymous channels (OpenCode, Kilo Code) must not be sprayed across every
// enabled proxy in the pool: the assignment list is the only opt-in signal the
// administrator gives, and the free tiers of those providers quota per exit IP,
// so a blanket fallback burns the server's own allowance through shared
// proxies. When nothing is assigned the channel goes out directly.
func GetProxiesForChannel(channelId int) []string {
	urlMap, _ := loadProxyPoolData()
	var assignedURLs []string
	if channelKeys, ok := urlMap[channelId]; ok && len(channelKeys) > 0 {
		seen := make(map[string]bool, len(channelKeys))
		for _, u := range channelKeys {
			u = strings.TrimSpace(u)
			if u != "" && !seen[u] {
				seen[u] = true
				assignedURLs = append(assignedURLs, u)
			}
		}
	}
	return assignedURLs
}

// GetNextProxyForAnonChannel selects the next healthy proxy for an anonymous channel
// (such as OpenCode or Kilo Code). It filters out currentProxy and proxies in 429 cooldown.
func GetNextProxyForAnonChannel(channelId int, currentProxy string) string {
	candidates := GetProxiesForChannel(channelId)
	if len(candidates) == 0 {
		return currentProxy
	}

	trimmedCurrent := strings.TrimSpace(currentProxy)

	// Filter out current proxy and proxies on cooldown
	var healthy []string
	for _, p := range candidates {
		if p == trimmedCurrent {
			continue
		}
		if !IsProxyRateLimited(p) {
			healthy = append(healthy, p)
		}
	}

	if len(healthy) > 0 {
		val, _ := anonChannelCounter.LoadOrStore(channelId, uint64(0))
		idx := val.(uint64)
		anonChannelCounter.Store(channelId, idx+1)
		return healthy[idx%uint64(len(healthy))]
	}

	for _, p := range candidates {
		if p != trimmedCurrent {
			return p
		}
	}

	return candidates[0]
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

type ProxyTestResult struct {
	EgressIP    string
	LatencyMs   int64
	ProxyType   string
	CountryCode string
	CountryName string
	City        string
	ISP         string
}

func lookupGeoIP(ip string) (countryCode, countryName, city, isp string) {
	if ip == "" || ip == "cloudflare-worker" || strings.Contains(ip, "127.0.0.1") || strings.Contains(ip, "localhost") {
		return "", "", "", ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://ip-api.com/json/"+ip+"?fields=status,country,countryCode,city,isp", nil)
	if err != nil {
		return "", "", "", ""
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", "", ""
	}
	defer resp.Body.Close()
	var res struct {
		Status      string `json:"status"`
		Country     string `json:"country"`
		CountryCode string `json:"countryCode"`
		City        string `json:"city"`
		ISP         string `json:"isp"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err == nil && res.Status == "success" {
		return res.CountryCode, res.Country, res.City, res.ISP
	}
	return "", "", "", ""
}

func testCloudflareWorkerEgress(workerURL string) (*ProxyTestResult, error) {
	if !strings.HasPrefix(workerURL, "http://") && !strings.HasPrefix(workerURL, "https://") {
		workerURL = "https://" + workerURL
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, workerURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Target-URL", "https://api.ipify.org?format=json")
	req.Header.Set("User-Agent", "curl/8.4.0")
	started := time.Now()
	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
	latency := time.Since(started).Milliseconds()
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return nil, err
	}
	bodyStr := string(body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("worker status %d: %s", resp.StatusCode, bodyStr)
	}
	var parsed struct {
		IP string `json:"ip"`
	}
	ip := "cloudflare-worker"
	if err := json.Unmarshal(body, &parsed); err == nil && parsed.IP != "" {
		ip = parsed.IP
	}
	cc, cn, city, isp := lookupGeoIP(ip)
	return &ProxyTestResult{
		EgressIP:    ip,
		LatencyMs:   latency,
		ProxyType:   "cloudflare_worker",
		CountryCode: cc,
		CountryName: cn,
		City:        city,
		ISP:         isp,
	}, nil
}

// TestProxyEgressInfo checks that a proxy resolves and can reach the public
// internet, returning the egress IP, latency, Geo-IP data and resolved proxy type.
func TestProxyEgressInfo(rawProxyURL string, proxyType ...string) (*ProxyTestResult, error) {
	trimmed := strings.TrimSpace(rawProxyURL)
	if trimmed == "" {
		return nil, fmt.Errorf("empty proxy URL")
	}

	// 1. Dedicated handling for Cloudflare Worker proxies
	if (len(proxyType) > 0 && proxyType[0] == "cloudflare_worker") || IsCloudflareWorkerProxy(trimmed) {
		return testCloudflareWorkerEgress(trimmed)
	}

	// 2. Standard HTTP/HTTPS/SOCKS5 Proxies
	if normURL, err := common.NormalizeProxyLine(rawProxyURL); err == nil && normURL != "" {
		rawProxyURL = normURL
	}
	if _, err := common.ParseProxyURLStrict(rawProxyURL); err != nil {
		return nil, err
	}
	client, err := GetHttpClientWithProxy(rawProxyURL)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.ipify.org?format=json", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "curl/8.4.0")
	started := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(started).Milliseconds()
	if err != nil {
		if strings.Contains(err.Error(), "first record does not look like a TLS handshake") || strings.Contains(err.Error(), "proxyconnect") {
			if strings.HasPrefix(trimmed, "https://") || strings.Contains(trimmed, "worker") {
				return testCloudflareWorkerEgress(trimmed)
			}
		}
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d from egress endpoint", resp.StatusCode)
	}
	var parsed struct {
		IP string `json:"ip"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil || parsed.IP == "" {
		return nil, fmt.Errorf("unexpected egress response")
	}

	cc, cn, city, isp := lookupGeoIP(parsed.IP)
	pt := "standard"
	if len(proxyType) > 0 && proxyType[0] != "" {
		pt = proxyType[0]
	}
	return &ProxyTestResult{
		EgressIP:    parsed.IP,
		LatencyMs:   latency,
		ProxyType:   pt,
		CountryCode: cc,
		CountryName: cn,
		City:        city,
		ISP:         isp,
	}, nil
}

// TestProxyEgress checks that a proxy resolves and can reach the public
// internet, returning the egress IP as seen by a remote service and the
// round-trip duration in milliseconds. The call uses a short context timeout
// so a dead proxy never blocks the admin UI.
func TestProxyEgress(rawProxyURL string, proxyType ...string) (string, int64, error) {
	res, err := TestProxyEgressInfo(rawProxyURL, proxyType...)
	if err != nil {
		return "", 0, err
	}
	return res.EgressIP, res.LatencyMs, nil
}

// StartProxyPoolAutoTester periodically runs health/latency checks on registered
// proxies in the pool when enabled via System Settings. A bounded worker pool
// keeps the sweep fast without overwhelming the server; each sweep covers only
// the stalest slice of the pool so a 65k-entry pool never hammers the network
// or the database in one pass.
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

			proxies, err := model.ListProxyPoolProxiesForAutoTest(1000)
			if err != nil || len(proxies) == 0 {
				continue
			}

			autoDisable := common.OptionMap["ProxyAutoDisableUnhealthy"] == "true"
			type result struct {
				proxy    *model.ProxyPoolProxy
				egressIP string
				latency  int64
				testErr  error
			}

			jobs := make(chan model.ProxyPoolProxy)
			results := make(chan result, len(proxies))
			var workers sync.WaitGroup
			const workerCount = 8
			for i := 0; i < workerCount; i++ {
				workers.Add(1)
				go func() {
					defer workers.Done()
					for p := range jobs {
						egressIP, latency, testErr := TestProxyEgress(p.URL)
						results <- result{proxy: &p, egressIP: egressIP, latency: latency, testErr: testErr}
					}
				}()
			}
			for i := range proxies {
				jobs <- proxies[i]
			}
			close(jobs)
			workers.Wait()
			close(results)

			for r := range results {
				p := *r.proxy
				prevStatus := p.LastTestStatus
				prevEnabled := p.Enabled
				p.LastTestedAt = time.Now().Unix()
				p.LatencyMs = r.latency
				if r.testErr != nil {
					p.LastTestStatus = "fail"
					p.LastTestError = r.testErr.Error()
					p.EgressIP = ""
					if autoDisable {
						p.Enabled = false
					}
					if prevStatus != "fail" {
						model.RecordProxyPoolEvent(0, -1, p.Id, p.URL, "test_fail", fmt.Sprintf("Proxy failed its health check: %s", r.testErr.Error()))
					}
					if autoDisable && prevEnabled {
						model.RecordProxyPoolEvent(0, -1, p.Id, p.URL, "auto_disabled", "Proxy auto-disabled after a failed health check")
					}
				} else {
					p.LastTestStatus = "ok"
					p.LastTestError = ""
					p.EgressIP = r.egressIP
					if autoDisable && !p.Enabled {
						p.Enabled = true
					}
					if prevStatus == "fail" {
						model.RecordProxyPoolEvent(0, -1, p.Id, p.URL, "test_ok", "Proxy recovered and passes its health check")
					}
					if autoDisable && !prevEnabled {
						model.RecordProxyPoolEvent(0, -1, p.Id, p.URL, "auto_enabled", "Proxy auto-enabled after a passing health check")
					}
				}
				_ = model.UpdateProxyPoolProxy(&p)
			}
			InvalidateProxyPoolCache()
		}
	}()
}
