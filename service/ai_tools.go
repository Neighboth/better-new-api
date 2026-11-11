package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

type ToolDefinition struct {
	Type     string       `json:"type"`
	Function FunctionSpec `json:"function"`
}

type FunctionSpec struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

func GetLiveSupportTools() []ToolDefinition {
	tools := []ToolDefinition{
		{
			Type: "function",
			Function: FunctionSpec{
				Name:        "get_current_time",
				Description: "Get the current live date, time, year, day of week, and timezone",
			},
		},
		{
			Type: "function",
			Function: FunctionSpec{
				Name:        "get_all_models",
				Description: "Get a list of all enabled AI models and their pricing/token ratios",
			},
		},
		{
			Type: "function",
			Function: FunctionSpec{
				Name:        "get_free_models",
				Description: "Get a list of all completely free models (price and ratio are 0)",
			},
		},
	}
	// The web search tool exists only when the administrator enabled it for
	// the live support and the installation has a search engine configured.
	if LiveSupportWebSearchEnabled() {
		tools = append(tools, ToolDefinition{
			Type: "function",
			Function: FunctionSpec{
				Name:        "web_search",
				Description: "Search the public web. Use it for fresh facts, news, prices or documentation links instead of guessing.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query":       map[string]any{"type": "string", "description": "The search query"},
						"max_results": map[string]any{"type": "integer", "description": "How many results to read, 1-10"},
					},
					"required": []string{"query"},
				},
			},
		})
	}
	for i := range tools {
		if tools[i].Function.Parameters == nil {
			tools[i].Function.Parameters = map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}
		}
	}
	return tools
}

// LiveSupportWebSearchEnabled reports whether the live support may search.
func LiveSupportWebSearchEnabled() bool {
	playground := operation_setting.GetPlaygroundSetting()
	if playground == nil || !playground.WebSearchLiveEnabled {
		return false
	}
	search := operation_setting.GetOnlineSearchSetting()
	return search != nil && strings.TrimSpace(search.SearXNGURL) != ""
}

// LiveSupportToolPrompt tells the assistant which tools it really has, so it
// stops answering that it cannot look anything up.
func LiveSupportToolPrompt() string {
	if !LiveSupportWebSearchEnabled() {
		return ""
	}
	return "Tools you can call right now: `web_search` searches the public web and returns titles, links and snippets. Use it instead of answering that you cannot browse the web."
}

// ExecuteLiveSupportTool runs one tool call of the live support assistant.
func ExecuteLiveSupportTool(name string, arguments string) string {
	switch name {
	case "web_search":
		if !LiveSupportWebSearchEnabled() {
			return "Web search is disabled."
		}
		var args struct {
			Query      string `json:"query"`
			MaxResults int    `json:"max_results"`
		}
		if err := json.Unmarshal([]byte(arguments), &args); err != nil {
			return "Invalid arguments for web_search."
		}
		query := strings.TrimSpace(args.Query)
		if query == "" {
			return "Please provide a search query."
		}
		maxResults := args.MaxResults
		if maxResults <= 0 {
			maxResults = 5
		}
		if maxResults > 10 {
			maxResults = 10
		}
		hits, err := SearchOnline(query, maxResults)
		if err != nil {
			return fmt.Sprintf("The search failed: %v", err)
		}
		if len(hits) == 0 {
			return "No results found."
		}
		var sb strings.Builder
		sb.WriteString("Search results:\n")
		for index, hit := range hits {
			sb.WriteString(fmt.Sprintf("%d. %s\n%s\n%s\n", index+1, hit.Title, hit.URL, hit.Content))
		}
		return sb.String()
	case "get_current_time":
		now := time.Now()
		return fmt.Sprintf("Live Time: %s, Year: %d, Date: %s, Day: %s, Timezone: %s",
			now.Format("15:04:05"), now.Year(), now.Format("2006-01-02"), now.Weekday().String(), now.Location().String())
	case "get_all_models":
		pricingList := model.GetPricing()
		if len(pricingList) == 0 {
			return "No models currently available."
		}
		var sb strings.Builder
		sb.WriteString("Available Models & Pricing:\n")
		for _, p := range pricingList {
			if p.QuotaType == 1 {
				sb.WriteString(fmt.Sprintf("- %s: $%.4f / call (Owner: %s)\n", p.ModelName, p.ModelPrice, p.OwnerBy))
			} else {
				sb.WriteString(fmt.Sprintf("- %s: Ratio: %.2f, Completion Ratio: %.2f (Owner: %s)\n", p.ModelName, p.ModelRatio, p.CompletionRatio, p.OwnerBy))
			}
		}
		return sb.String()
	case "get_free_models":
		pricingList := model.GetPricing()
		var freeModels []string
		for _, p := range pricingList {
			if p.ModelPrice == 0 && p.ModelRatio == 0 {
				freeModels = append(freeModels, p.ModelName)
			}
		}
		if len(freeModels) == 0 {
			return "No free models found."
		}
		return "Free Models (Price 0):\n- " + strings.Join(freeModels, "\n- ")
	default:
		return "Unknown tool: " + name
	}
}
