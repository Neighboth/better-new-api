package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	proxyPoolExpiresAt time.Time
)

// InvalidateProxyPoolCache drops the cached mapping. Call it after every
// proxy or assignment write so the next request sees the new state.
func InvalidateProxyPoolCache() {
	proxyPoolMu.Lock()
	proxyPoolMap = nil
	proxyPoolExpiresAt = time.Time{}
	proxyPoolMu.Unlock()
}

func loadProxyPoolMap() map[int]map[int]string {
	proxyPoolMu.RLock()
	if proxyPoolMap != nil && time.Now().Before(proxyPoolExpiresAt) {
		defer proxyPoolMu.RUnlock()
		return proxyPoolMap
	}
	proxyPoolMu.RUnlock()

	proxyPoolMu.Lock()
	defer proxyPoolMu.Unlock()
	if proxyPoolMap != nil && time.Now().Before(proxyPoolExpiresAt) {
		return proxyPoolMap
	}
	m, err := model.ProxyPoolAssignmentURLMap()
	if err != nil {
		// Fail open to the channel's own path rather than routing through an
		// unknown exit point. The stale cache (if any) is kept.
		if proxyPoolMap == nil {
			proxyPoolMap = map[int]map[int]string{}
			proxyPoolExpiresAt = time.Now().Add(5 * time.Second)
		}
		return proxyPoolMap
	}
	proxyPoolMap = m
	proxyPoolExpiresAt = time.Now().Add(proxyPoolCacheTTL)
	return proxyPoolMap
}

// EffectiveRelayProxyURL resolves which proxy URL the relay should use for a
// specific (channel, key index) pair. When a proxy pool assignment exists for
// that key it wins over the channel's own proxy setting; otherwise the
// channel's own setting (possibly empty) is used.
func EffectiveRelayProxyURL(channelId, keyIndex int, channelProxy string) string {
	if channelId <= 0 || keyIndex < 0 {
		return channelProxy
	}
	m := loadProxyPoolMap()
	channelKeys, ok := m[channelId]
	if !ok {
		return channelProxy
	}
	url, ok := channelKeys[keyIndex]
	if !ok || url == "" {
		return channelProxy
	}
	return url
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
