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

func RunOnlineSearch(query string, maxResults int) (string, int, error) {
	setting := operation_setting.GetOnlineSearchSetting()
	base := strings.TrimSpace(setting.SearXNGURL)
	if base == "" {
		return "", 0, fmt.Errorf("searxng url is not configured")
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
		return "", 0, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("searxng returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", 0, err
	}
	var parsed struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	if err := common.Unmarshal(body, &parsed); err != nil {
		return "", 0, err
	}
	if len(parsed.Results) == 0 {
		return "", 0, nil
	}
	if len(parsed.Results) > maxResults {
		parsed.Results = parsed.Results[:maxResults]
	}
	var sb strings.Builder
	for i, r := range parsed.Results {
		fmt.Fprintf(&sb, "[%d] %s\n%s\n%s\n\n", i+1, r.Title, r.URL, r.Content)
	}
	return sb.String(), len(parsed.Results), nil
}
