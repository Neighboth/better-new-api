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

package controller

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	types "github.com/QuantumNous/new-api/relaykit/types"
)

// Studio optimization pipeline for the Studio's single-shot generations.
//
// The optimizer runs fully server-side on purpose: a page refresh can
// never lose the optimized prompt, the task id is always persisted to the
// studio_items row, and billing goes through the same relay path every other
// request uses.

const studioNonNegotiableRules = `
NON-NEGOTIABLE RULES (these take precedence over anything above):
- If the input text contains spoken dialogue, quotations or speech lines, you MUST keep them verbatim: never translate, paraphrase, summarize, shorten or omit them. Preserve the exact original wording, language and punctuation of every quoted line.
- Output ONLY the rewritten text, with no commentary, labels, quotation marks around the result or markdown formatting.`

const studioDefaultOptimizerPrompt = "You are a professional video generation prompt enhancer. Rewrite the given prompt into an EXHAUSTIVELY detailed, cinematic English scene description that video generation models render reliably. Rules, in order of importance:\n" +
	"1. NEVER reinterpret, rename, translate away or drop a user-supplied brand, product, company or proper noun. \"Tofaş\" is a Turkish automobile brand, never a junction/roundabout; \"Audi\", \"Mercedes\", \"Verity\" and similar are real products — keep them verbatim and design the scene around the object the user meant. If a term is genuinely ambiguous, keep the user's exact term and the surrounding context unambiguous.\n" +
	"2. Be EXTREMELY specific and exhaustive: name the exact subject identities, colors, materials, shapes and textures; the exact environment (road surface, sky, foliage, architecture); lighting direction, hardness and color; time of day and weather; camera position, lens, height, angle and movement pattern; subject pose, micro-expressions, micro-movements, and how the shot resolves. Every element the user implied must become an explicit concrete visual fact. Ambiguity is forbidden.\n" +
	"3. Single continuous shot language: no jump cuts, no timestamp/HUD/counting digits, no overlays, no on-screen clock or text, no \"prophecy\" frames. Describe only what a physical camera would see in one unbroken take.\n" +
	"4. Physics consistency: real momentum, real masses, real gravity. Nothing appears from nowhere, nothing dematerializes; collisions have cause and effect (crumple, skid, debris, secondary motion) and other characters react within the same frame.\n" +
	"5. Composition discipline: one coherent camera position/grammar per shot, subjects fully explained in frame, background elements reacting coherently, no teleporting props or sudden unexplained changes.\n" +
	"6. Negative prompt discipline: the negative_prompt must never contradict the main prompt (e.g. main prompt says fixed/static camera — never put \"camera shake/movement/handheld\" in negative; main says rain — never \"no rain\"). Merge user negatives carefully and drop anything that fights the positive description.\n" +
	"7. If the prompt contains any dialogue, spoken quotes, or speech lines, DO NOT translate the spoken dialogue — keep all spoken words/quotes exactly as written, in their original language.\n" +
	"8. Output ONLY the optimized prompt text without any introductory text, explanation, or quotes around the prompt."

const studioSearchToolsInstruction = `You may call two tools that are available ONLY because the administrator enabled Studio web/image search for this installation:
- web_search(query: string): searches the public web. Use it when you do not know a brand, character, product, place or object mentioned in the brief (e.g. "Verity", a regional car model).
- image_search(query: string): searches the public image index. The returned image URLs are attached to the video job as reference images automatically.
How to call a tool: reply with ONLY the JSON object, nothing else: {"tool":"web_search","arguments":{"query":"your query"}} or {"tool":"image_search","arguments":{"query":"your query"}}.
When you have enough information, reply with the final answer TEXT only — no JSON, no explanation, no preamble.
Describe briefly inside the final text which references were added and what they show, e.g. "Reference 1: the exact yellow Verity ball design" — so the video model knows which reference is which.`

const studioToolMaxRounds = 4
const studioSearchMaxReferences = 4

type StudioOptimizeResult struct {
	Prompt         string
	NegativePrompt string
	References     []string
	Optimized      bool
	Fallback       bool
	SkillID        string
}

func metadataStudioFlag(metadata map[string]interface{}, key string) bool {
	if metadata == nil {
		return false
	}
	switch v := metadata[key].(type) {
	case bool:
		return v
	case string:
		trimmed := strings.TrimSpace(v)
		return strings.EqualFold(trimmed, "true") || trimmed == "1"
	default:
		return false
	}
}

// firstOptimizerModel resolves the first configured optimizer model — the
// default selector value when several are configured.
func firstOptimizerModel(raw string) string {
	models := parseOptimizerModels(raw)
	if len(models) == 0 {
		return ""
	}
	return models[0]
}

func parseOptimizerModels(raw string) []string {
	out := []string{}
	for _, item := range strings.Split(raw, ",") {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func metadataStringValue(metadata map[string]interface{}, key string) string {
	if metadata == nil {
		return ""
	}
	s, ok := metadata[key].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

func metadataReferenceImages(metadata map[string]interface{}) []string {
	if metadata == nil {
		return nil
	}
	raw, ok := metadata["reference_images"]
	if !ok {
		return nil
	}
	list, ok := raw.([]interface{})
	if !ok {
		if strList, ok := raw.([]string); ok {
			return strList
		}
		return nil
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		s, ok := item.(string)
		if ok && strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}

type toolCall struct {
	Tool  string
	Query string
}

func parseToolCall(raw string) *toolCall {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	candidate := trimmed
	if match := regexp.MustCompile("(?s)```(?:json)?\\s*([\\s\\S]*?)```").FindStringSubmatch(trimmed); len(match) > 1 {
		candidate = strings.TrimSpace(match[1])
	}
	var parsed map[string]interface{}
	if err := common.UnmarshalJsonStr(candidate, &parsed); err != nil {
		first := strings.Index(candidate, "{")
		last := strings.LastIndex(candidate, "}")
		if first == -1 || last <= first {
			return nil
		}
		if err := common.UnmarshalJsonStr(candidate[first:last+1], &parsed); err != nil {
			return nil
		}
	}
	tool, _ := parsed["tool"].(string)
	tool = strings.ToLower(strings.TrimSpace(tool))
	if tool == "" {
		return nil
	}
	args, _ := parsed["arguments"].(map[string]interface{})
	query := ""
	if args != nil {
		query, _ = args["query"].(string)
	}
	return &toolCall{Tool: tool, Query: strings.TrimSpace(query)}
}

type internalChatMessage struct {
	Role    string
	Content interface{}
}

const studioChatCompletionPath = "/pg/chat/completions"

// internalChatCompletion performs one chat completion through the same
// session-authenticated pipeline the playground uses, writing the answer to a
// recorder we control. The call is detached from the client's context so a
// page refresh can never kill the generation.
func internalChatCompletion(parent *gin.Context, userID int, group string, modelName string, systemPrompt string, messages []internalChatMessage) (string, error) {
	chatMessages := make([]map[string]interface{}, 0, len(messages)+1)
	chatMessages = append(chatMessages, map[string]interface{}{
		"role":    "system",
		"content": systemPrompt + "\n\n" + studioNonNegotiableRules,
	})
	for _, m := range messages {
		chatMessages = append(chatMessages, map[string]interface{}{
			"role":    m.Role,
			"content": m.Content,
		})
	}
	body, err := common.Marshal(map[string]interface{}{
		"model":       modelName,
		"temperature": 0.4,
		"messages":    chatMessages,
	})
	if err != nil {
		return "", err
	}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	detached := context.WithoutCancel(parent.Request.Context())
	detached, cancel := context.WithTimeout(detached, 3*time.Minute)
	defer cancel()
	req := parent.Request.Clone(detached)
	req.Method = http.MethodPost
	req.URL.Path = studioChatCompletionPath
	req.URL.RawQuery = ""
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.ContentLength = int64(len(body))
	req.Header = parent.Request.Header.Clone()
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Set("id", userID)
	c.Set("group", group)

	// Mirror the playground relay wiring: temp token, user context, then the
	// shared Relay pipeline (billing, fallback and channel selection all run).
	userCache, err := model.GetUserCache(userID)
	if err != nil {
		return "", err
	}
	userCache.WriteContext(c)
	targetGroup := strings.TrimSpace(group)
	if targetGroup == "" {
		targetGroup = userCache.Group
	}
	if targetGroup == "" {
		targetGroup = "default"
	}
	tempToken := &model.Token{
		UserId:          userID,
		Name:            fmt.Sprintf("studio-optimizer-%s", targetGroup),
		Group:           targetGroup,
		CrossGroupRetry: false,
	}
	_ = middleware.SetupContextForToken(c, tempToken)

	playgroundRelay(c, types.RelayFormatOpenAI)

	resp := recorder.Body.Bytes()
	if recorder.Code >= 400 {
		return "", fmt.Errorf("optimizer returned HTTP %d: %s", recorder.Code, strings.TrimSpace(common.LocalLogPreview(string(resp))))
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := common.Unmarshal(resp, &parsed); err != nil {
		return "", fmt.Errorf("optimizer returned an unreadable response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("optimizer returned an empty response")
	}
	content := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if content == "" {
		return "", fmt.Errorf("optimizer returned an empty response")
	}
	return content, nil
}

// buildOptimizerMessages produces the vision-ready user message for the first
// optimizer round: brief text plus labeled image parts, in submission order.
func buildOptimizerMessages(brief string, referenceImages []string, startFrame string, endFrame string) []internalChatMessage {
	parts := []map[string]interface{}{}
	parts = append(parts, map[string]interface{}{"type": "text", "text": brief})
	refs := referenceImages
	if len(refs) > 5 {
		refs = refs[:5]
	}
	for i, ref := range refs {
		parts = append(parts,
			map[string]interface{}{"type": "text", "text": fmt.Sprintf("USER REFERENCE IMAGE %d/%d — attached by the user; match this subject's appearance, colors, wardrobe, brand/livery and overall look exactly.", i+1, len(refs))},
			map[string]interface{}{"type": "image_url", "image_url": map[string]interface{}{"url": ref, "detail": "high"}},
		)
	}
	if sf := strings.TrimSpace(startFrame); sf != "" {
		parts = append(parts,
			map[string]interface{}{"type": "text", "text": "FIRST FRAME — the video must open exactly from this image: composition, subject pose, environment, lighting and framing must continue from it."},
			map[string]interface{}{"type": "image_url", "image_url": map[string]interface{}{"url": sf, "detail": "high"}},
		)
	}
	if ef := strings.TrimSpace(endFrame); ef != "" {
		parts = append(parts,
			map[string]interface{}{"type": "text", "text": "FINISH FRAME — the video must end exactly on this image: composition, subject pose, environment, lighting and framing must converge onto it."},
			map[string]interface{}{"type": "image_url", "image_url": map[string]interface{}{"url": ef, "detail": "high"}},
		)
	}
	return []internalChatMessage{
		{Role: "user", Content: parts},
	}
}

func mustMarshal(v interface{}) string {
	data, err := common.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(data)
}

// optimizeStudioPromptWithTools runs the optimizer with the web/image search
// tool loop when the administrator enabled it; otherwise it makes one direct
// call. Tool-loop failures degrade to a direct call so a generation is never
// blocked by the search path.
func optimizeStudioPromptWithTools(parent *gin.Context, userID int, group string, modelName string, systemPrompt string, messages []internalChatMessage) (string, []string, error) {
	playground := operation_setting.GetPlaygroundSetting()
	search := operation_setting.GetOnlineSearchSetting()
	toolsEnabled := playground.StudioPromptSearchEnabled && strings.TrimSpace(search.SearXNGURL) != ""
	if !toolsEnabled {
		answer, err := internalChatCompletion(parent, userID, group, modelName, systemPrompt, messages)
		return answer, nil, err
	}
	systemWithTools := systemPrompt + "\n\n" + studioSearchToolsInstruction
	convo := append([]internalChatMessage{}, messages...)
	references := []string{}
	seenRef := map[string]struct{}{}
	addReference := func(url string) {
		url = strings.TrimSpace(url)
		if url == "" {
			return
		}
		if _, ok := seenRef[url]; ok {
			return
		}
		if len(references) >= studioSearchMaxReferences {
			return
		}
		seenRef[url] = struct{}{}
		references = append(references, url)
	}
	for round := 0; round < studioToolMaxRounds; round += 1 {
		answer, err := internalChatCompletion(parent, userID, group, modelName, systemWithTools, convo)
		if err != nil {
			fallback, fallbackErr := internalChatCompletion(parent, userID, group, modelName, systemPrompt, messages)
			return fallback, nil, fallbackErr
		}
		tool := parseToolCall(answer)
		if tool == nil {
			return answer, references, nil
		}
		query := tool.Query
		if query == "" {
			query = "the subject in the brief"
		}
		switch tool.Tool {
		case "web_search":
			hits, searchErr := service.SearchOnline(query, 5)
			if searchErr == nil {
				_, _ = chargeSearchQuota(parent, userID, group, query)
			} else {
				hits = nil
			}
			convo = append(convo,
				internalChatMessage{Role: "assistant", Content: answer},
				internalChatMessage{Role: "user", Content: fmt.Sprintf("web_search results for %q:\n%s", query, mustMarshal(hits))},
			)
		case "image_search":
			hits, searchErr := service.SearchImages(query, 5)
			if searchErr == nil {
				_, _ = chargeSearchQuota(parent, userID, group, query)
				for _, hit := range hits {
					addReference(hit.URL)
				}
			} else {
				hits = nil
			}
			convo = append(convo,
				internalChatMessage{Role: "assistant", Content: answer},
				internalChatMessage{Role: "user", Content: fmt.Sprintf("image_search results for %q (these images become reference images):\n%s", query, mustMarshal(hits))},
			)
		default:
			return answer, references, nil
		}
	}
	fallback, err := internalChatCompletion(parent, userID, group, modelName, systemPrompt, convo)
	return fallback, references, err
}

var contradictionRules = []struct {
	positive *regexp.Regexp
	drop     []*regexp.Regexp
}{
	{regexp.MustCompile(`(?i)\b(static|fixed|stationary|locked|immobile)\s+(camera|shot|frame|angle)|\b(camera|shot|angle)\s+(static|fixed|stationary|locked)\b`), []*regexp.Regexp{regexp.MustCompile(`(?i)\b(camera shake|camera movement|camera rotation|camera pan|camera drift|camera float|handheld|moving camera|unstable camera|shaky camera|camera motion)\b`)}},
	{regexp.MustCompile(`(?i)\b(rain|raining|downpour|drizzle)\b`), []*regexp.Regexp{regexp.MustCompile(`(?i)\b(no rain|rainless|dry weather|dry|clear weather|no precipitation|sunny)\b`)}},
	{regexp.MustCompile(`(?i)\b(car|vehicle|suv|sedan|truck|bus|motorcycle|scooter|bicycle|automobile|taxi)\b`), []*regexp.Regexp{regexp.MustCompile(`(?i)\b(no (car|vehicle|suv|sedan|truck|bus|traffic)|vehicle-free|no traffic|no cars|empty road|no automobiles)\b`)}},
	{regexp.MustCompile(`(?i)\b(person|people|man|woman|child|pedestrian|driver|crowd|character|figure|silhouette)\b`), []*regexp.Regexp{regexp.MustCompile(`(?i)\b(empty scene|uninhabited|no people|no persons|no humans|nobody|depopulated|no characters)\b`)}},
	{regexp.MustCompile(`(?i)\b(night|dark|dusk|twilight|night-time|nighttime|glow|neon|lamp|moonlight|streetlight|street lamp)\b`), []*regexp.Regexp{regexp.MustCompile(`(?i)\b(daylight|bright (sun|sunlight)|sunny|daytime|high noon|clear blue sky|blinding light)\b`)}},
	{regexp.MustCompile(`(?i)\b(day|daytime|bright|sun|sunlight|sunny|noon|midday)\b`), []*regexp.Regexp{regexp.MustCompile(`(?i)\b(night scene|night mode|darkness|pitch[- ]black|moonlit|candlelight|midnight)\b`)}},
	{regexp.MustCompile(`(?i)\b(wet|rain-soaked|puddle|standing water|wet asphalt|wet ground|wet road)\b`), []*regexp.Regexp{regexp.MustCompile(`(?i)\b(dry (road|ground|asphalt|surface)|bone[- ]dry)\b`)}},
}

var negativeSplitPattern = regexp.MustCompile(`[\n;,•·|]+|\s+-\s+|\s—\s`)

// removeNegativeContradictions drops negative-prompt clauses that contradict
// the positive description. Deterministic; runs regardless of what any model
// or user wrote.
func removeNegativeContradictions(mainPrompt string, negativePrompt string) string {
	clauses := negativeSplitPattern.Split(negativePrompt, -1)
	kept := make([]string, 0, len(clauses))
	for _, clause := range clauses {
		clause = strings.TrimSpace(clause)
		if clause == "" {
			continue
		}
		drop := false
		for _, rule := range contradictionRules {
			if rule.positive.MatchString(mainPrompt) {
				for _, pattern := range rule.drop {
					if pattern.MatchString(clause) {
						drop = true
						break
					}
				}
			}
			if drop {
				break
			}
		}
		if !drop {
			kept = append(kept, clause)
		}
	}
	return strings.Join(kept, ", ")
}

// RunStudioOptimization rewrites the user's brief with the configured vision
// optimizer. It never hard-fails a generation: when the optimizer is
// unconfigured or misbehaving it returns Fallback=true with the raw prompt
// and the caller proceeds.
func RunStudioOptimization(parent *gin.Context, userID int, group string, req *relaycommon.TaskSubmitReq, modelKind string) (*StudioOptimizeResult, error) {
	setting := operation_setting.GetPlaygroundSetting()

	models := parseOptimizerModels(setting.PromptOptimizerModel)
	if len(models) == 0 {
		return &StudioOptimizeResult{Prompt: req.Prompt, Optimized: false, Fallback: true}, fmt.Errorf("prompt_optimizer_not_configured")
	}
	selected := models[0]
	if requested := metadataStringValue(req.Metadata, "optimizer_model"); requested != "" {
		for _, candidate := range models {
			if strings.EqualFold(candidate, requested) {
				selected = candidate
				break
			}
		}
	}

	systemPrompt := strings.TrimSpace(setting.PromptOptimizerSystemPrompt)
	if systemPrompt == "" {
		systemPrompt = studioDefaultOptimizerPrompt
	}

	skillID := metadataStringValue(req.Metadata, "skill_id")
	if skillID != "" {
		if skill, err := model.GetStudioSkill(skillID); err == nil && skill != nil && skill.Enabled {
			systemPrompt += "\n\nSTUDIO SKILL (the user selected this creative direction for this generation; follow it strictly when it does not conflict with safety rules):\n" + skill.Content
		}
	}

	mdl := effectiveModelFor(req.Model, modelKind)
	effective := service.ResolveEffectiveStudioModel(
		mdl, req.Duration, req.Duration > 0, req.Quality, req.Ratio,
	)

	// The reference list is a flat multimodal channel upstream, ordered by the
	// client as image, audio, video — the explicit counters in the metadata
	// split it deterministically. Image references are capped by the model's
	// reference rules; audio and video references have their own caps and are
	// never trimmed to the image cap.
	flat := metadataReferenceImages(req.Metadata)
	imageCount, audioCount, videoCount := referenceCounts(req.Metadata, flat)
	if imageCount > len(flat) {
		imageCount = len(flat)
	}
	if audioCount > len(flat)-imageCount {
		audioCount = len(flat) - imageCount
	}
	if videoCount > len(flat)-imageCount-audioCount {
		videoCount = len(flat) - imageCount - audioCount
	}
	userImageRefs := flat[:imageCount]
	rest := flat[imageCount:]
	userAudioRefs := rest[:audioCount]
	userVideoRefs := rest[audioCount : audioCount+videoCount]

	audioCap := 0
	if mdl.AudioReference {
		audioCap = mdl.MaxAudioReferences
		if audioCap <= 0 {
			audioCap = 3
		}
	}
	videoCap := 0
	if mdl.VideoReference {
		videoCap = mdl.MaxVideoReferences
		if videoCap <= 0 {
			videoCap = 2
		}
	}
	if !effective.ReferencesAllowed {
		userImageRefs = nil
	} else if len(userImageRefs) > effective.MaxReferenceImages {
		userImageRefs = userImageRefs[:effective.MaxReferenceImages]
	}
	if len(userAudioRefs) > audioCap {
		userAudioRefs = userAudioRefs[:audioCap]
	}
	if len(userVideoRefs) > videoCap {
		userVideoRefs = userVideoRefs[:videoCap]
	}

	// Vision only ever sees the image references — audio/video data URLs are
	// never valid image_url parts.
	optimizerMessages := buildOptimizerMessages(req.Prompt, userImageRefs, metadataStringValue(req.Metadata, "start_frame"), metadataStringValue(req.Metadata, "end_frame"))

	answer, searchRefs, err := optimizeStudioPromptWithTools(parent, userID, group, selected, systemPrompt, optimizerMessages)
	if err != nil {
		return &StudioOptimizeResult{Prompt: req.Prompt, Optimized: false, Fallback: true, SkillID: skillID}, err
	}

	negativeRaw := metadataStringValue(req.Metadata, "negative_prompt")
	negativeOut := removeNegativeContradictions(answer, negativeRaw)

	merged := append([]string{}, userImageRefs...)
	if effective.ReferencesAllowed {
		for _, ref := range searchRefs {
			if len(merged) >= effective.MaxReferenceImages {
				break
			}
			dup := false
			for _, existing := range flat {
				if existing == ref {
					dup = true
					break
				}
			}
			if !dup {
				merged = append(merged, ref)
			}
		}
	}
	merged = append(merged, userAudioRefs...)
	merged = append(merged, userVideoRefs...)

	return &StudioOptimizeResult{
		Prompt:         answer,
		NegativePrompt: negativeOut,
		References:     merged,
		Optimized:      true,
		Fallback:       false,
		SkillID:        skillID,
	}, nil
}

func effectiveModelFor(modelName string, kind string) operation_setting.MediaModel {
	m := service.FindMediaModel(kind, modelName)
	if m == nil {
		// Unconfigured model — behave as an all-options-enabled model so no
		// arbitrary rejections occur for custom channels and self-hosted models.
		return operation_setting.MediaModel{Model: modelName, ReferenceImage: true, MaxReferenceImages: 5}
	}
	return *m
}
