/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package service

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

// autoModelPrice is the price of one candidate, in USD, as the site bills it.
// Token prices are per 1M tokens; a per-request model has no token price.
type autoModelPrice struct {
	// InputPer1M and OutputPer1M are the token prices per 1M tokens.
	InputPer1M  float64
	OutputPer1M float64
	// PerRequest is set when the model is billed a fixed price per request.
	PerRequest float64
	// HasTokenPrice is false when the model only has a per-request price.
	HasTokenPrice bool
	// Known is false when the installation does not know the price of the
	// model at all (possible in self-use mode).
	Known bool
}

// billingPriceOf resolves the billing configuration of one model with the
// same rules the relay uses.
func billingPriceOf(modelName string) autoModelPrice {
	if price, ok := ratio_setting.GetModelPrice(modelName, false); ok {
		return autoModelPrice{PerRequest: price, Known: true}
	}
	ratio, ok, _ := ratio_setting.GetModelRatio(modelName)
	if !ok {
		// Unknown price. This only reaches the router in self-use mode, where
		// unpriced models are allowed.
		return autoModelPrice{Known: false}
	}
	completion := ratio_setting.GetCompletionRatio(modelName)
	input := ratio * 2 // same conversion the pricing page uses
	return autoModelPrice{
		InputPer1M:    input,
		OutputPer1M:   input * completion,
		HasTokenPrice: true,
		Known:         true,
	}
}

// autoModelCatalogue builds the compact, price-aware catalogue of every
// usable candidate. The lines are deliberately short so a catalogue of a
// thousand models still fits a small prompt.
func autoModelCatalogue(candidates []string) []string {
	lines := make([]string, 0, len(candidates))
	for _, name := range candidates {
		price := billingPriceOf(name)
		var line string
		switch {
		case price.Known && !price.HasTokenPrice:
			line = fmt.Sprintf("- %s ($%g/req)", name, price.PerRequest)
		case price.Known:
			line = fmt.Sprintf("- %s ($%g in / $%g out)", name, price.InputPer1M, price.OutputPer1M)
		default:
			line = fmt.Sprintf("- %s (price unknown)", name)
		}
		lines = append(lines, line)
	}
	return lines
}

// autoModelCatalogueCompact renders the lines the router reads. When the full
// catalogue would take too many tokens for the routing model, the price column
// is dropped first: the names are enough for the router to reason about the
// administration rankings. If even the bare names do not fit, the
// least-ranked models are dropped, which is reported by the returned bool.
func autoModelCatalogueCompact(candidates []string, ranked []string, maxEstTokens int) (string, bool) {
	full := autoModelCatalogue(candidates)
	text := strings.Join(full, "\n")
	if estimateAutoModelTokens(text) <= maxEstTokens {
		return text, false
	}
	// Drop the prices: bare names still tell the router what exists.
	bare := make([]string, 0, len(candidates))
	for _, name := range candidates {
		bare = append(bare, "- "+name)
	}
	text = strings.Join(bare, "\n")
	if estimateAutoModelTokens(text) <= maxEstTokens {
		return text, false
	}
	// Still too big: keep every model the administrator ranked (plus the
	// already-sliced head of the candidate order) and drop the rest.
	rankedSet := make(map[string]bool, len(ranked))
	for _, name := range ranked {
		rankedSet[name] = true
	}
	kept := make([]string, 0, len(candidates))
	keptTokens := 0
	dropped := 0
	budgetChars := maxEstTokens * 4
	for _, name := range candidates {
		line := "- " + name
		if rankedSet[name] || keptTokens+len(line) <= budgetChars {
			kept = append(kept, line)
			keptTokens += len(line) + 1
		} else {
			dropped++
		}
	}
	text = strings.Join(kept, "\n")
	text += fmt.Sprintf("\n(%d models omitted: catalogue too large)", dropped)
	return text, true
}

// estimateAutoModelTokens is a cheap, provider-agnostic token estimate.
func estimateAutoModelTokens(text string) int {
	if text == "" {
		return 0
	}
	// Roughly four bytes per token for English/JSON; whitespace-dense lists
	// compress slightly better, so this errs on the safe side.
	return len(text)/4 + 8
}

// autoModelFilterByPriceRange drops the candidates that do not fit the price
// range the caller asked for, and reports how many were dropped. The range is
// on the input price per 1M tokens; a per-request model has no token price and
// can only be kept when no range is set. In self-use mode models with unknown
// prices are kept.
func autoModelFilterByPriceRange(candidates []string, minPrice, maxPrice float64) ([]string, int) {
	if minPrice <= 0 && maxPrice <= 0 {
		return candidates, 0
	}
	kept := make([]string, 0, len(candidates))
	dropped := 0
	for _, name := range candidates {
		price := billingPriceOf(name)
		switch {
		case !price.Known:
			// Self-use mode with unknown price: keep, the operator asked for it.
			kept = append(kept, name)
		case !price.HasTokenPrice:
			// Per-request model: it cannot be weighed on a token range. Drop.
			dropped++
		case minPrice > 0 && price.InputPer1M < minPrice:
			dropped++
		case maxPrice > 0 && price.InputPer1M > maxPrice:
			dropped++
		default:
			kept = append(kept, name)
		}
	}
	return kept, dropped
}
