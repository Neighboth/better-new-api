package service

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"sort"
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
// 1. If an explicit proxy pool assignment exists for (channelId, keyIndex), it wins.
// 2. If the channel configured its own proxy setting, that is used.
// 3. Otherwise, if enabled proxies exist in the pool, a sticky proxy is deterministically
//    selected based on the key string so key X, Y, Z always consistently route through the
//    same proxy until that proxy is deleted.
func EffectiveRelayProxyURL(channelId, keyIndex int, keyString string, channelProxy string) string {
	urlMap, enabled := loadProxyPoolData()

	if channelId > 0 && keyIndex >= 0 {
		if channelKeys, ok := urlMap[channelId]; ok {
			if url, ok := channelKeys[keyIndex]; ok && url != "" {
				return url
			}
		}
	}

	if channelProxy != "" {
		return channelProxy
	}

	// Sticky fallback from the pool for this key
	if len(enabled) > 0 && strings.TrimSpace(keyString) != "" {
		if sticky := PickStickyProxy(keyString, enabled); sticky != nil {
			return sticky.URL
		}
	}

	return channelProxy
}

// TestProxyEgress checks that a proxy resolves and can reach the public
// internet, returning the egress IP as seen by a remote service and the
// round-trip duration in milliseconds. The call uses a short context timeout
// so a dead proxy never blocks the admin UI.
func TestProxyEgress(rawProxyURL string) (string, int64, error) {
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
	started := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return "", time.Since(started).Milliseconds(), err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return "", time.Since(started).Milliseconds(), err
	}
	if resp.StatusCode != http.StatusOK {
		return "", time.Since(started).Milliseconds(), fmt.Errorf("unexpected status %d from egress endpoint", resp.StatusCode)
	}
	var parsed struct {
		IP string `json:"ip"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", time.Since(started).Milliseconds(), fmt.Errorf("unexpected egress response")
	}
	if parsed.IP == "" {
		return "", time.Since(started).Milliseconds(), fmt.Errorf("unexpected egress response")
	}
	return parsed.IP, time.Since(started).Milliseconds(), nil
}
