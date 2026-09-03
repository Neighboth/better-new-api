package controller

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/setting/console_setting"

	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"
)

type rssFeed struct {
	XMLName xml.Name `xml:"rss"`
	Channel struct {
		Title string `xml:"title"`
		Items []struct {
			Title       string `xml:"title"`
			Description string `xml:"description"`
			PubDate     string `xml:"pubDate"`
		} `xml:"item"`
	} `xml:"channel"`
}

const (
	requestTimeout   = 30 * time.Second
	httpTimeout      = 10 * time.Second
	uptimeKeySuffix  = "_24"
	apiStatusPath    = "/api/status-page/"
	apiHeartbeatPath = "/api/status-page/heartbeat/"
)

type Monitor struct {
	Name   string  `json:"name"`
	Uptime float64 `json:"uptime"`
	Status int     `json:"status"`
	Group  string  `json:"group,omitempty"`
}

type UptimeGroupResult struct {
	CategoryName string    `json:"categoryName"`
	CustomName   string    `json:"customName,omitempty"`
	Monitors     []Monitor `json:"monitors"`
}

func getAndDecode(ctx context.Context, client *http.Client, url string, dest interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New("non-200 status")
	}

	return json.NewDecoder(resp.Body).Decode(dest)
}

func fetchGroupData(ctx context.Context, client *http.Client, groupConfig map[string]interface{}) UptimeGroupResult {
	url, _ := groupConfig["url"].(string)
	slug, _ := groupConfig["slug"].(string)
	categoryName, _ := groupConfig["categoryName"].(string)
	providerType, _ := groupConfig["type"].(string)
	apiKey, _ := groupConfig["apiKey"].(string)

	result := UptimeGroupResult{
		CategoryName: categoryName,
		CustomName:   getMapString(groupConfig, "customName"),
		Monitors:     []Monitor{},
	}

	// Handle Instatus
	if providerType == "instatus" || strings.Contains(url, "instatus.com") {
		instatusJSONURL := url
		if !strings.HasSuffix(instatusJSONURL, "/summary.json") {
			instatusJSONURL = strings.TrimSuffix(instatusJSONURL, "/") + "/summary.json"
		}
		var instatusData struct {
			Page struct {
				Name   string `json:"name"`
				Status string `json:"status"`
			} `json:"page"`
			Components []struct {
				Name   string `json:"name"`
				Status string `json:"status"`
			} `json:"components"`
		}
		if err := getAndDecode(ctx, client, instatusJSONURL, &instatusData); err == nil && len(instatusData.Components) > 0 {
			for _, comp := range instatusData.Components {
				st := 1
				if strings.ToUpper(comp.Status) != "OPERATIONAL" && strings.ToUpper(comp.Status) != "UP" {
					st = 0
				}
				result.Monitors = append(result.Monitors, Monitor{
					Name:   comp.Name,
					Status: st,
					Uptime: 100,
				})
			}
			return result
		}

		// Fallback to RSS parsing (e.g. https://pixrouter.instatus.com/history.rss)
		rssURL := url
		if !strings.HasSuffix(rssURL, ".rss") {
			rssURL = strings.TrimSuffix(rssURL, "/") + "/history.rss"
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rssURL, nil)
		if err == nil {
			resp, err := client.Do(req)
			if err == nil && resp.StatusCode == http.StatusOK {
				var feed rssFeed
				if xml.NewDecoder(resp.Body).Decode(&feed) == nil {
					resp.Body.Close()
					channelTitle := feed.Channel.Title
					if result.CustomName != "" {
						channelTitle = result.CustomName
					} else if channelTitle == "" {
						channelTitle = categoryName
					}
					st := 1
					if len(feed.Channel.Items) > 0 {
						// Check latest item description
						latest := strings.ToLower(feed.Channel.Items[0].Title + " " + feed.Channel.Items[0].Description)
						if strings.Contains(latest, "outage") || strings.Contains(latest, "degraded") || strings.Contains(latest, "down") {
							st = 0
						}
					}
					result.Monitors = append(result.Monitors, Monitor{
						Name:   channelTitle,
						Status: st,
						Uptime: 100,
					})
					return result
				}
				resp.Body.Close()
			}
		}
	}

	// Handle UptimeRobot API
	if providerType == "uptimerobot" || apiKey != "" || strings.Contains(url, "uptimerobot.com") {
		reqURL := "https://api.uptimerobot.com/v2/getMonitors"
		postBody := strings.NewReader("api_key=" + apiKey + "&format=json&custom_uptime_ratios=30")
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, postBody)
		if err == nil {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			resp, err := client.Do(req)
			if err == nil && resp.StatusCode == http.StatusOK {
				var urData struct {
					Monitors []struct {
						FriendlyName string  `json:"friendly_name"`
						Status       int     `json:"status"`
						UptimeRatio  string  `json:"custom_uptime_ratio"`
					} `json:"monitors"`
				}
				if json.NewDecoder(resp.Body).Decode(&urData) == nil {
					resp.Body.Close()
					for _, m := range urData.Monitors {
						st := 0
						if m.Status == 2 { // 2 = Up in UptimeRobot
							st = 1
						}
						up, _ := strconv.ParseFloat(m.UptimeRatio, 64)
						result.Monitors = append(result.Monitors, Monitor{
							Name:   m.FriendlyName,
							Status: st,
							Uptime: up,
						})
					}
					return result
				}
				resp.Body.Close()
			}
		}
	}

	if url == "" || slug == "" {
		return result
	}

	baseURL := strings.TrimSuffix(url, "/")

	var statusData struct {
		PublicGroupList []struct {
			ID          int    `json:"id"`
			Name        string `json:"name"`
			MonitorList []struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			} `json:"monitorList"`
		} `json:"publicGroupList"`
	}

	var heartbeatData struct {
		HeartbeatList map[string][]struct {
			Status int `json:"status"`
		} `json:"heartbeatList"`
		UptimeList map[string]float64 `json:"uptimeList"`
	}

	g, gCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return getAndDecode(gCtx, client, baseURL+apiStatusPath+slug, &statusData)
	})
	g.Go(func() error {
		return getAndDecode(gCtx, client, baseURL+apiHeartbeatPath+slug, &heartbeatData)
	})

	if g.Wait() != nil {
		return result
	}

	for _, pg := range statusData.PublicGroupList {
		if len(pg.MonitorList) == 0 {
			continue
		}

		for _, m := range pg.MonitorList {
			monitor := Monitor{
				Name:  m.Name,
				Group: pg.Name,
			}

			monitorID := strconv.Itoa(m.ID)

			if uptime, exists := heartbeatData.UptimeList[monitorID+uptimeKeySuffix]; exists {
				monitor.Uptime = uptime
			}

			if heartbeats, exists := heartbeatData.HeartbeatList[monitorID]; exists && len(heartbeats) > 0 {
				monitor.Status = heartbeats[0].Status
			}

			result.Monitors = append(result.Monitors, monitor)
		}
	}

	return result
}

func GetUptimeKumaStatus(c *gin.Context) {
	groups := console_setting.GetUptimeKumaGroups()
	if len(groups) == 0 {
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": []UptimeGroupResult{}})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), requestTimeout)
	defer cancel()

	client := &http.Client{Timeout: httpTimeout}
	results := make([]UptimeGroupResult, len(groups))

	g, gCtx := errgroup.WithContext(ctx)
	for i, group := range groups {
		i, group := i, group
		g.Go(func() error {
			results[i] = fetchGroupData(gCtx, client, group)
			return nil
		})
	}

	g.Wait()
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": results})
}
func getMapString(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}
