package service

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// SearchResult is one hit of the SearXNG instance.
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Content string `json:"content"`
}

// SearchOnline queries the SearXNG instance and returns the structured hits.
// It is shared by the relayed :online models and by the web search tool of the
// playground so both surfaces see the same results.
func SearchOnline(query string, maxResults int) ([]SearchResult, error) {
	setting := operation_setting.GetOnlineSearchSetting()
	base := strings.TrimSpace(setting.SearXNGURL)
	if base == "" {
		return nil, fmt.Errorf("searxng url is not configured")
	}
	if maxResults <= 0 {
		maxResults = 5
	}
	if maxResults > 10 {
		maxResults = 10
	}
	if max := setting.MaxSearches; max > 0 && maxResults > max {
		maxResults = max
	}
	endpoint := strings.TrimRight(base, "/") + "/search"
	params := url.Values{}
	params.Set("q", query)
	params.Set("format", "json")
	params.Set("language", "en")
	reqURL := endpoint + "?" + params.Encode()
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("searxng returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Results []SearchResult `json:"results"`
	}
	if err := common.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	if len(parsed.Results) == 0 {
		return nil, nil
	}
	if len(parsed.Results) > maxResults {
		parsed.Results = parsed.Results[:maxResults]
	}
	return parsed.Results, nil
}

// RunOnlineSearch renders the hits as the compact text block injected into the
// conversation of the :online models.
func RunOnlineSearch(query string, maxResults int) (string, int, error) {
	results, err := SearchOnline(query, maxResults)
	if err != nil {
		return "", 0, err
	}
	if len(results) == 0 {
		return "", 0, nil
	}
	var sb strings.Builder
	for i, r := range results {
		fmt.Fprintf(&sb, "[%d] %s\n%s\n%s\n\n", i+1, r.Title, r.URL, r.Content)
	}
	return sb.String(), len(results), nil
}
