package service

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

// The virtual model of the Auto Model feature. A request that names it is
// routed to a real model before the channel of the request is chosen, so the
// price, the logs and the channel of the answer belong to the model that
// answered and never to a second, price-less model.

// AutoModelDecisionContextKey is where the router stores its decision on the
// request, so the billing code can log it without importing the middleware.
const AutoModelDecisionContextKey = "auto_model_decision"

// AutoModelRequestedContextKey marks a request that asked for the virtual model.
const AutoModelRequestedContextKey = "auto_model_requested"

// AutoModelSelection is the decision of the router.
type AutoModelSelection struct {
	// Model is the real model that serves the request.
	Model string
	// Reason is a short explanation, logged with the request.
	Reason string
	// Dimension is the ranking that drove the decision.
	Dimension string
	// Candidates is how many models the router could choose from.
	Candidates int
}

// The counters of the router. They live in memory and reset with the process:
// they are enough to see the shape of the traffic, and they cost nothing on the
// path of a request.
var (
	autoModelCounterMutex sync.Mutex
	autoModelCounts      = map[string]int64{}
	autoModelTotal       int64
	autoModelFallbacks   int64
)

func recordAutoModelDecision(modelName string, byRouter bool) {
	autoModelCounterMutex.Lock()
	defer autoModelCounterMutex.Unlock()
	autoModelCounts[modelName]++
	autoModelTotal++
	if !byRouter {
		autoModelFallbacks++
	}
}

// AutoModelStats is the counters of the router, sorted by count.
type AutoModelStats struct {
	// Total is how many requests the router answered.
	Total int64 `json:"total"`
	// Fallbacks is how many of them were answered by the rankings because the
	// routing model was unavailable or answered nothing usable.
	Fallbacks int64 `json:"fallbacks"`
	// Models maps a model to how many requests it served.
	Models []AutoModelModelCount `json:"models"`
}

// AutoModelModelCount is the number of requests a model served.
type AutoModelModelCount struct {
	Model string `json:"model"`
	Count int64  `json:"count"`
}

// AutoModelCounters returns a snapshot of the counters of the router.
func AutoModelCounters() AutoModelStats {
	autoModelCounterMutex.Lock()
	defer autoModelCounterMutex.Unlock()
	models := make([]AutoModelModelCount, 0, len(autoModelCounts))
	for name, count := range autoModelCounts {
		models = append(models, AutoModelModelCount{Model: name, Count: count})
	}
	sort.Slice(models, func(i, j int) bool {
		if models[i].Count == models[j].Count {
			return models[i].Model < models[j].Model
		}
		return models[i].Count > models[j].Count
	})
	return AutoModelStats{
		Total:     autoModelTotal,
		Fallbacks: autoModelFallbacks,
		Models:    models,
	}
}

// IsAutoModelRequest reports whether a request asks for the virtual model.
func IsAutoModelRequest(modelName string) bool {
	return strings.EqualFold(strings.TrimSpace(modelName), operation_setting.AutoModelName)
}

// AutoModelEnabled reports whether the administrator turned the feature on.
func AutoModelEnabled() bool {
	setting := operation_setting.GetAutoModelSetting()
	return setting != nil && setting.Enabled
}

// AutoModelBlockedModels is the model blocklist that an administrator assigned
// to the account: the router never chooses a blocked model.
func AutoModelBlockedModels(userId int) []string {
	if userId <= 0 {
		return nil
	}
	settings, err := model.GetUserSetting(userId, false)
	if err != nil {
		return nil
	}
	return settings.BlockedModels
}

// autoModelUsableModels returns the models a request of this user may be routed
// to: the models of the groups the request may use, without the virtual model
// itself, without the models the administrator blocked for the account and,
// outside self-use installations, without models that have no price (the relay
// refuses them anyway).
func autoModelUsableModels(candidates []string, blockedModels []string) []string {
	blocked := make(map[string]bool, len(blockedModels))
	for _, name := range blockedModels {
		blocked[strings.ToLower(strings.TrimSpace(name))] = true
	}
	usable := make([]string, 0, len(candidates))
	for _, name := range candidates {
		if name == "" || IsAutoModelRequest(name) {
			continue
		}
		if blocked[strings.ToLower(name)] {
			continue
		}
		if !operation_setting.SelfUseModeEnabled && !hasModelBillingConfig(name) {
			continue
		}
		usable = append(usable, name)
	}
	return usable
}

// hasModelBillingConfig reports whether the installation knows how to bill a
// model. It mirrors relay/helper.HasModelBillingConfig, which this package
// cannot import (relay/helper imports service).
func hasModelBillingConfig(modelName string) bool {
	if _, ok := ratio_setting.GetModelPrice(modelName, false); ok {
		return true
	}
	if _, ok, _ := ratio_setting.GetModelRatio(modelName); ok {
		return true
	}
	if billing_setting.GetBillingMode(modelName) != billing_setting.BillingModeTieredExpr {
		return false
	}
	expr, ok := billing_setting.GetBillingExpr(modelName)
	return ok && strings.TrimSpace(expr) != ""
}

// autoModelTaskProfile guesses the shape of the request from its text.
//
// The guess is deliberately simple: it orders the candidate list and it is the
// fallback when no routing model is configured. The routing model sees the
// same guess and may disagree with it, which is the point of the prompt.
func autoModelTaskProfile(text string, hasImage bool, hasAudio bool) (string, string) {
	if hasAudio {
		return "stt", "audio transcription"
	}
	lower := strings.ToLower(text)
	type keywordSet struct {
		dimension string
		words     []string
	}
	sets := []keywordSet{
		{"coding", []string{"code", "function", "class ", "bug", "refactor", "compile", "typescript", "python", "golang", "api endpoint", "unit test"}},
		{"debugging", []string{"stack trace", "traceback", "exception", "error message", "debug", "segfault", "null pointer"}},
		{"sql", []string{"select ", "insert into", "postgres", "mysql", "sql query", "join table"}},
		{"math", []string{"integral", "derivative", "equation", "theorem", "probability", "matrix", "solve for"}},
		{"translation", []string{"translate", "translation", "in turkish", "in french", "in german", "in chinese"}},
		{"summarization", []string{"summarize", "summary", "tl;dr", "key points"}},
		{"creative_writing", []string{"write a story", "poem", "screenplay", "novel", "lyrics"}},
		{"reasoning", []string{"reason", "explain why", "step by step", "prove", "plan", "architecture", "trade-off", "compare"}},
		{"legal", []string{"contract", "lawsuit", "gdpr", "legal clause", "attorney"}},
		{"medical", []string{"symptom", "diagnosis", "medication", "dosage", "patient"}},
		{"finance", []string{"investment", "portfolio", "balance sheet", "tax", "invoice", "valuation"}},
		{"education", []string{"teach me", "explain like", "curriculum", "lesson", "student"}},
	}
	if hasImage {
		return "vision", "the request carries an image"
	}
	for _, set := range sets {
		for _, word := range set.words {
			if strings.Contains(lower, word) {
				return set.dimension, "the request mentions " + word
			}
		}
	}
	return "smartest_first", "no specific task detected"
}

// autoModelHeuristic picks the first ranked candidate of a dimension.
func autoModelHeuristic(setting *operation_setting.AutoModelSetting, dimension string, candidates []string) (string, string) {
	allowed := make(map[string]bool, len(candidates))
	for _, name := range candidates {
		allowed[name] = true
	}
	for _, dimensionName := range []string{dimension, "smartest_first"} {
		for _, name := range setting.Ranking(dimensionName) {
			if allowed[name] {
				return name, "first offer of the " + dimensionName + " ranking"
			}
		}
	}
	// Nothing ranked: fall back to the first model of the request's group, so
	// an installation that never filled a ranking still answers.
	if len(candidates) > 0 {
		return candidates[0], "first model of the group, no ranking matched"
	}
	return "", "no candidate"
}

// renderAutoModelPrompt fills the placeholders of the routing prompt.
func renderAutoModelPrompt(
	setting *operation_setting.AutoModelSetting,
	candidates []string,
	dimension string,
	profile string,
	budget int,
	requestText string,
) string {
	rankings := make([]string, 0, len(operation_setting.AutoModelDimensions))
	for _, name := range operation_setting.AutoModelDimensions {
		list := setting.Ranking(name)
		if len(list) == 0 {
			continue
		}
		rankings = append(rankings, fmt.Sprintf("- %s: %s", name, strings.Join(list, " > ")))
	}
	rankingText := strings.Join(rankings, "\n")
	if rankingText == "" {
		rankingText = "(no ranking was filled, choose the strongest candidate)"
	}

	budgetText := "Budget: no limit for this request."
	if budget > 0 {
		budgetText = fmt.Sprintf(
			"Budget: the request may spend %d tokens. Prefer the cheapest candidate that still fits the task when the budget is small.",
			budget,
		)
	}

	if len(requestText) > 4000 {
		requestText = requestText[:4000] + "\n…(truncated)"
	}

	prompt := setting.AutoModelPrompt()
	replacer := strings.NewReplacer(
		"{{candidates}}", strings.Join(candidates, "\n"),
		"{{rankings}}", rankingText,
		"{{request_profile}}", fmt.Sprintf("%s (%s)", dimension, profile),
		"{{budget}}", budgetText,
		"{{request}}", requestText,
	)
	return replacer.Replace(prompt)
}

// localRelayToken returns a token of the installation that the router may use
// for its own call. It is the same rule as the ticket assistant: a usable token
// of the root account first, then the first usable token of the installation.
func localRelayToken() string {
	now := time.Now().Unix()
	usable := func(token *model.Token) bool {
		if token == nil || token.Key == "" {
			return false
		}
		return token.ExpiredTime == -1 || token.ExpiredTime == 0 || token.ExpiredTime > now || token.UnlimitedQuota
	}

	var rootToken model.Token
	if err := model.DB.Where("user_id = 1 AND status = 1").Order("id ASC").First(&rootToken).Error; err == nil && usable(&rootToken) {
		return rootToken.Key
	}
	var fallback model.Token
	if err := model.DB.Where("status = 1").Order("id ASC").First(&fallback).Error; err == nil && usable(&fallback) {
		return fallback.Key
	}
	return ""
}

// callAutoModelRouter asks the routing model which candidate serves the request.
func callAutoModelRouter(routingModel string, prompt string) (string, error) {
	token := localRelayToken()
	if token == "" {
		return "", fmt.Errorf("no usable token for the routing model")
	}
	port := os.Getenv("PORT")
	if port == "" && common.Port != nil {
		port = strconv.Itoa(*common.Port)
	}
	if port == "" {
		port = "3000"
	}
	url := fmt.Sprintf("http://127.0.0.1:%s/v1/chat/completions", port)

	body, err := common.Marshal(map[string]any{
		"model":  routingModel,
		"stream": false,
		"messages": []map[string]string{
			{"role": "system", "content": "You are a router. You answer with JSON only."},
			{"role": "user", "content": prompt},
		},
	})
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: 45 * time.Second}
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("routing model answered %d", response.StatusCode)
	}

	var data struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := common.DecodeJson(response.Body, &data); err != nil {
		return "", err
	}
	if len(data.Choices) == 0 {
		return "", fmt.Errorf("routing model answered nothing")
	}
	content := strings.TrimSpace(data.Choices[0].Message.Content)
	if content == "" {
		return "", fmt.Errorf("routing model answered nothing")
	}
	return content, nil
}

// parseAutoModelAnswer reads the model of the router answer.
//
// The answer should be JSON, but a model that answers with a sentence that
// names the model is still usable: the first candidate that appears in the
// text wins. An answer that names no candidate is an error and the caller uses
// the ranking instead.
func parseAutoModelAnswer(answer string, candidates []string) (string, error) {
	var payload struct {
		Model string `json:"model"`
	}
	if err := common.UnmarshalJsonStr(answer, &payload); err == nil && payload.Model != "" {
		for _, candidate := range candidates {
			if strings.EqualFold(candidate, payload.Model) {
				return candidate, nil
			}
		}
	}
	lower := strings.ToLower(answer)
	for _, candidate := range candidates {
		if strings.Contains(lower, strings.ToLower(candidate)) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("the router named no candidate")
}

// ResolveAutoModel chooses the real model of an "auto" request.
func ResolveAutoModel(
	candidates []string,
	blockedModels []string,
	requestText string,
	hasImage bool,
	hasAudio bool,
	budget int,
) (AutoModelSelection, error) {
	setting := operation_setting.GetAutoModelSetting()
	if setting == nil || !setting.Enabled {
		return AutoModelSelection{}, fmt.Errorf("the auto model is disabled")
	}

	usable := autoModelUsableModels(candidates, blockedModels)
	if len(usable) == 0 {
		return AutoModelSelection{}, fmt.Errorf("no model is available for this request")
	}

	dimension, profile := autoModelTaskProfile(requestText, hasImage, hasAudio)
	if hasAudio {
		// Audio requests can only be served by a model of the matching ranking.
		audioCandidates := make([]string, 0, len(usable))
		allowed := make(map[string]bool, len(usable))
		for _, name := range usable {
			allowed[name] = true
		}
		for _, name := range setting.Ranking(dimension) {
			if allowed[name] {
				audioCandidates = append(audioCandidates, name)
			}
		}
		if len(audioCandidates) > 0 {
			usable = audioCandidates
		}
	}

	// The candidate list the router sees: the models ranked for the task first,
	// then the rest of the group, capped so the prompt stays small.
	ordered := make([]string, 0, len(usable))
	seen := make(map[string]bool, len(usable))
	for _, name := range setting.RankedModels(dimension) {
		if seen[name] {
			continue
		}
		for _, allowed := range usable {
			if allowed == name {
				seen[name] = true
				ordered = append(ordered, name)
				break
			}
		}
	}
	for _, name := range usable {
		if seen[name] {
			continue
		}
		seen[name] = true
		ordered = append(ordered, name)
	}
	if len(ordered) > 80 {
		ordered = ordered[:80]
	}

	if budget <= 0 {
		budget = setting.DefaultBudgetTokens
	}

	// A routing model that the request may not use would loop: it is only
	// called when it is one of the candidates of the group.
	routingModel := strings.TrimSpace(setting.RoutingModel)
	routingUsable := false
	if routingModel != "" && !IsAutoModelRequest(routingModel) {
		for _, name := range usable {
			if name == routingModel {
				routingUsable = true
				break
			}
		}
	}

	if routingUsable {
		prompt := renderAutoModelPrompt(setting, ordered, dimension, profile, budget, requestText)
		if answer, err := callAutoModelRouter(routingModel, prompt); err == nil {
			if chosen, err := parseAutoModelAnswer(answer, ordered); err == nil {
				reason := ""
				var parsed struct {
					Reason string `json:"reason"`
				}
				if err := common.UnmarshalJsonStr(answer, &parsed); err == nil {
					reason = parsed.Reason
				}
				recordAutoModelDecision(chosen, true)
				return AutoModelSelection{
					Model:      chosen,
					Reason:     reason,
					Dimension:  dimension,
					Candidates: len(ordered),
				}, nil
			}
		}
		// The routing model failed: the ranking of the task still answers.
	}

	chosen, reason := autoModelHeuristic(setting, dimension, ordered)
	if chosen == "" {
		return AutoModelSelection{}, fmt.Errorf("no model is available for this request")
	}
	recordAutoModelDecision(chosen, false)
	return AutoModelSelection{
		Model:      chosen,
		Reason:     reason,
		Dimension:  dimension,
		Candidates: len(ordered),
	}, nil
}
