package service

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

const exchangeRateCacheTTL = 1 * time.Minute

type exchangeRateCache struct {
	mu        sync.RWMutex
	rates     map[string]float64
	fetchedAt time.Time
}

var globalExchangeRates = &exchangeRateCache{
	rates: map[string]float64{"USD": 1},
}

// GetExchangeRates returns cached USD-based exchange rates (1 USD = rate[CODE]).
// Falls back to stale cache if live API calls fail.
func GetExchangeRates(forceRefresh bool) (map[string]float64, error) {
	globalExchangeRates.mu.RLock()
	if !forceRefresh && len(globalExchangeRates.rates) > 1 && time.Since(globalExchangeRates.fetchedAt) < exchangeRateCacheTTL {
		copied := make(map[string]float64, len(globalExchangeRates.rates))
		for k, v := range globalExchangeRates.rates {
			copied[k] = v
		}
		globalExchangeRates.mu.RUnlock()
		return copied, nil
	}
	globalExchangeRates.mu.RUnlock()

	rates, err := fetchLiveUSDExchangeRates()
	if err != nil {
		globalExchangeRates.mu.RLock()
		copied := make(map[string]float64, len(globalExchangeRates.rates))
		for k, v := range globalExchangeRates.rates {
			copied[k] = v
		}
		globalExchangeRates.mu.RUnlock()
		return copied, err
	}

	globalExchangeRates.mu.Lock()
	globalExchangeRates.rates = rates
	globalExchangeRates.fetchedAt = time.Now()
	globalExchangeRates.mu.Unlock()

	copied := make(map[string]float64, len(rates))
	for k, v := range rates {
		copied[k] = v
	}
	return copied, nil
}

func fetchLiveUSDExchangeRates() (map[string]float64, error) {
	client := &http.Client{Timeout: 8 * time.Second}

	// Primary: jsDelivr fawazahmed0 currency-api (zero rate-limit CDN)
	if rates, err := fetchFawazCurrencyAPI(client); err == nil && len(rates) > 1 {
		return rates, nil
	}
	// Fallback: open.er-api.com (no API key required)
	if rates, err := fetchOpenErAPI(client); err == nil && len(rates) > 1 {
		return rates, nil
	}
	return nil, fmt.Errorf("failed to fetch USD exchange rates from live providers")
}

func fetchFawazCurrencyAPI(client *http.Client) (map[string]float64, error) {
	resp, err := client.Get("https://cdn.jsdelivr.net/npm/@fawazahmed0/currency-api@latest/v1/currencies/usd.json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fawaz currency API status %d", resp.StatusCode)
	}
	var payload struct {
		USD map[string]float64 `json:"usd"`
	}
	if err := common.DecodeJson(resp.Body, &payload); err != nil {
		return nil, err
	}
	rates := map[string]float64{"USD": 1}
	for k, v := range payload.USD {
		if v > 0 {
			rates[strings.ToUpper(strings.TrimSpace(k))] = v
		}
	}
	return rates, nil
}

func fetchOpenErAPI(client *http.Client) (map[string]float64, error) {
	resp, err := client.Get("https://open.er-api.com/v6/latest/USD")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("open.er-api status %d", resp.StatusCode)
	}
	var payload struct {
		Result string             `json:"result"`
		Rates  map[string]float64 `json:"rates"`
	}
	if err := common.DecodeJson(resp.Body, &payload); err != nil {
		return nil, err
	}
	rates := map[string]float64{"USD": 1}
	for k, v := range payload.Rates {
		if v > 0 {
			rates[strings.ToUpper(strings.TrimSpace(k))] = v
		}
	}
	return rates, nil
}

// ResolveDisplayCurrencies returns the configured display currencies with live
// exchange rates populated for any currency configured with AutoRate or rate <= 0.
func ResolveDisplayCurrencies() []operation_setting.DisplayCurrencyOption {
	gs := operation_setting.GetGeneralSetting()
	configured := gs.DisplayCurrencies
	if len(configured) == 0 {
		return nil
	}

	var liveRates map[string]float64
	needsLive := gs.AutoExchangeRates
	if !needsLive {
		for _, c := range configured {
			if c.AutoRate || (c.ExchangeRate <= 0 && !strings.EqualFold(c.Code, "TOKENS")) {
				needsLive = true
				break
			}
		}
	}
	if needsLive {
		liveRates, _ = GetExchangeRates(false)
	}

	out := make([]operation_setting.DisplayCurrencyOption, 0, len(configured))
	for _, c := range configured {
		code := strings.ToUpper(strings.TrimSpace(c.Code))
		if code == "" {
			continue
		}
		item := c
		item.Code = code
		if code == "USD" {
			item.ExchangeRate = 1
			if item.Symbol == "" {
				item.Symbol = "$"
			}
		} else if code == "TOKENS" {
			item.ExchangeRate = 1
		} else if (c.AutoRate || gs.AutoExchangeRates || c.ExchangeRate <= 0) && liveRates != nil {
			if r, ok := liveRates[code]; ok && r > 0 {
				item.ExchangeRate = r
			}
		}
		out = append(out, item)
	}
	return out
}
