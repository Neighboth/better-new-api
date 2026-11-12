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
package operation_setting

import (
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
)

// AutoModelDimensions is the ordered list of ranking dimensions the Auto Model
// understands. Every entry is a key of AutoModelSetting.Rankings: an ordered
// list of model ids, best first.
//
// The list is exported so the panel, the routing prompt and the settings card
// describe the same dimensions: a dimension that the panel shows but the
// router ignores would be a field an administrator fills without effect.
var AutoModelDimensions = []string{
	"smartest_first",
	"dumbest_first",
	"coding",
	"code_review",
	"reasoning",
	"math",
	"science",
	"creative_writing",
	"translation",
	"summarization",
	"vision",
	"tool_calling",
	"agentic",
	"long_context",
	"instruction_following",
	"json_output",
	"sql",
	"debugging",
	"data_analysis",
	"multilingual",
	"education",
	"legal",
	"medical",
	"finance",
	"roleplay",
	"reliability",
	"cheapest",
	"fastest",
	"stt",
	"tts",
	"image_generation",
	"video_generation",
}

// AutoModelDefaultPrompt is the routing prompt of the Auto Model.
//
// It is a template: the router replaces the placeholders with the candidates
// of the request (with their live prices), the detected task and the ranking
// lists of the administrator. The text is kept readable on purpose, an
// administrator that edits it sees the same shape that the router renders.
const AutoModelDefaultPrompt = `You route requests of an API gateway to the model that serves them best.

Candidates (the only models you may answer with), with their price in USD per
1M tokens (input price / output price; "$X/req" is a fixed price per request):
{{candidates}}

Rankings of this installation, best first. An empty ranking is unknown, not
"no model":
{{rankings}}

What the request needs, detected from its text and shape:
{{request_profile}}

Caller preference: {{preference}}

{{budget}}

Rules:
1. Answer with one of the candidates, spelled exactly as in the list.
2. Prefer the model the administrator ranked highest for the task of the
   request. When a ranking says nothing about the task, follow the general
   ranking.
3. Use the prices as a tie-breaker: when two models are comparable in quality
   and fit the task, pick the cheaper one. A cheap model that the
   administration ranked for this task beats an expensive model that nobody
   ranked.
4. A request that asks for code, a diff or a fix belongs to the coding and
   debugging rankings; a request that asks for reasoning, a proof or a plan
   belongs to the reasoning ranking; a request with images belongs to the
   vision ranking; audio transcription belongs to stt and speech synthesis to
   tts.
5. Never pick a model that cannot handle the shape of the request (an STT model
   for a chat request, an image model for a text request).
6. Respect the caller preference: "fast" favors low latency models, "smart"
   favors the smartest models, "cheap" favors the cheapest model that fits,
   "balanced" weighs quality and price. A per-request budget of the caller
   or an explicit caller instruction always wins.
Answer with JSON only: {"model": "<candidate>", "reason": "<one sentence>"}

The request that has to be routed:
{{request}}`

// AutoModelSetting configures the virtual "auto" model.
//
// The model is virtual: it exists in the catalogue and in the panel, it has no
// price of its own and no channel. The router reads the request, decides which
// real model serves it and rewrites the request to that model, so the price of
// the answer is the price of the model that answered.
type AutoModelSetting struct {
	Enabled bool `json:"enabled"`

	// RoutingModel decides the route. When it is empty (or not usable) the
	// router falls back to the ranking of the detected task, which keeps the
	// feature working without a second model call.
	RoutingModel string `json:"routing_model"`

	// Rankings holds one ordered model list per AutoModelDimensions entry,
	// best first. The order is the opinion of the administrator and the router
	// follows it.
	Rankings map[string][]string `json:"rankings"`

	// SystemPrompt overrides AutoModelDefaultPrompt. Empty means "use the
	// default", so a reset of the prompt in the panel never loses the
	// documented behaviour.
	SystemPrompt string `json:"system_prompt"`

	// DefaultBudgetTokens is the optional token budget of one routed request.
	// Zero means "no budget": the router picks the best model of the task.
	DefaultBudgetTokens int `json:"default_budget_tokens"`
}

var autoModelSetting = AutoModelSetting{
	Enabled:             false,
	RoutingModel:        "",
	Rankings:            map[string][]string{},
	SystemPrompt:        "",
	DefaultBudgetTokens: 0,
}

func init() {
	config.GlobalConfig.Register("auto_model_setting", &autoModelSetting)
}

// GetAutoModelSetting returns the live configuration of the Auto Model.
func GetAutoModelSetting() *AutoModelSetting {
	return &autoModelSetting
}

// AutoModelName is the id of the virtual model.
const AutoModelName = "auto"

// AutoModelPrompt returns the prompt that the router renders. An administrator
// that saved a prompt sees it; an installation that never touched it gets the
// documented default.
func (s *AutoModelSetting) AutoModelPrompt() string {
	if strings.TrimSpace(s.SystemPrompt) != "" {
		return s.SystemPrompt
	}
	return AutoModelDefaultPrompt
}

// Ranking returns the ordered list of one dimension, trimmed and without
// duplicates.
func (s *AutoModelSetting) Ranking(dimension string) []string {
	raw := s.Rankings[dimension]
	if len(raw) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(raw))
	list := make([]string, 0, len(raw))
	for _, model := range raw {
		model = strings.TrimSpace(model)
		if model == "" || model == AutoModelName || seen[model] {
			continue
		}
		seen[model] = true
		list = append(list, model)
	}
	return list
}

// RankedModels returns every model that appears in any ranking, ordered by the
// dimension that matters for the given task first.
//
// The router uses it to build the candidate list it shows to the routing model:
// a model that the administrator ranked is a model the administrator trusts,
// and the ones ranked for the task come first.
func (s *AutoModelSetting) RankedModels(dimension string) []string {
	seen := make(map[string]bool)
	list := make([]string, 0)
	appendList := func(models []string) {
		for _, model := range models {
			if seen[model] {
				continue
			}
			seen[model] = true
			list = append(list, model)
		}
	}
	if dimension != "" {
		appendList(s.Ranking(dimension))
	}
	appendList(s.Ranking("smartest_first"))
	dimensions := make([]string, 0, len(AutoModelDimensions))
	dimensions = append(dimensions, AutoModelDimensions...)
	sort.Strings(dimensions)
	for _, name := range dimensions {
		appendList(s.Ranking(name))
	}
	return list
}
