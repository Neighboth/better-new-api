package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
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

func ExecuteLiveSupportTool(name string) string {
	switch name {
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
