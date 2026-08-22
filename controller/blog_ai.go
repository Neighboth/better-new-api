package controller

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// AI-assisted blog drafting. The admin picks one of the models served by the
// enabled channels and describes the article; the upstream model writes the
// English base plus a translation for every configured content language. The
// result is a draft the admin can still edit before publishing — nothing is
// persisted here.

const blogAICallTimeout = 180 * time.Second
const blogAIMaxChannels = 3

type blogAIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type blogAIChatRequest struct {
	Model       string              `json:"model"`
	Messages    []blogAIChatMessage `json:"messages"`
	Temperature float64             `json:"temperature"`
}

type blogAIChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// enabledChannelsForModel returns enabled channels that list the model,
// capped so a failing provider does not stall generation forever.
func enabledChannelsForModel(modelName string) []*model.Channel {
	channels, err := model.GetAllChannels(0, 0, true, false)
	if err != nil {
		return nil
	}
	var matched []*model.Channel
	for _, channel := range channels {
		if channel.Status != common.ChannelStatusEnabled {
			continue
		}
		if strings.TrimSpace(channel.GetBaseURL()) == "" {
			continue
		}
		if len(channel.GetKeys()) == 0 {
			continue
		}
		for _, offered := range channel.GetModels() {
			if strings.TrimSpace(offered) == modelName {
				matched = append(matched, channel)
				break
			}
		}
		if len(matched) >= blogAIMaxChannels {
			break
		}
	}
	return matched
}

// GetBlogAIModels lists every model currently served by an enabled channel so
// the admin can pick one for AI drafting.
func GetBlogAIModels(c *gin.Context) {
	channels, err := model.GetAllChannels(0, 0, false, false)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	seen := map[string]struct{}{}
	var models []string
	for _, channel := range channels {
		if channel.Status != common.ChannelStatusEnabled {
			continue
		}
		for _, name := range channel.GetModels() {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			models = append(models, name)
		}
	}
	sort.Strings(models)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": models})
}

func buildBlogAISystemPrompt(languages []string) string {
	var names []string
	for _, code := range languages {
		if lang, ok := common.LookupContentLanguage(code); ok {
			names = append(names, fmt.Sprintf("%s (%s)", lang.Native, lang.Code))
		}
	}
	return `You are the content engine of an AI API gateway's public blog.

Write a complete blog post from the admin's brief below and answer with ONE
JSON object and nothing else (no markdown fences, no commentary):

{
  "title": "English title",
  "summary": "1-2 sentence English summary for cards and listings",
  "content": "Full English article in Markdown",
  "seo_description": "English meta description, max 160 characters",
  "tags": "comma, separated, english, keywords",
  "translations": {
    "<lang code>": {
      "title": "...",
      "summary": "...",
      "content": "...",
      "seo_description": "...",
      "tags": "comma, separated, translated, keywords"
    }
  }
}

Rules:
- The base language is English; write the base fields in English.
- Provide a full translation for EVERY one of these languages: ` + strings.Join(names, ", ") + `.
- Translate naturally, do not transliterate. Keep model names, vendor names,
  endpoints, URLs and code blocks untranslated inside every language.
- "content" is GitHub-flavored Markdown and must keep identical structure
  (headings, lists, code blocks, links) across languages.
- Never invent prices, quotas or feature claims the brief does not contain.`
}

func extractBlogAIJSON(raw string) ([]byte, error) {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	trimmed = strings.TrimSpace(trimmed)
	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("model did not return a JSON object")
	}
	return []byte(trimmed[start : end+1]), nil
}

func callBlogAIChannel(channel *model.Channel, modelName string, messages []blogAIChatMessage) (string, error) {
	payload, err := common.Marshal(blogAIChatRequest{
		Model:       modelName,
		Messages:    messages,
		Temperature: 0.7,
	})
	if err != nil {
		return "", err
	}
	baseURL := strings.TrimRight(strings.TrimSpace(channel.GetBaseURL()), "/")
	ctx, cancel := context.WithTimeout(context.Background(), blogAICallTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(channel.GetKeys()[0]))

	resp, err := service.GetHttpClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", err
	}
	var decoded blogAIChatResponse
	if err := common.Unmarshal(body, &decoded); err != nil {
		return "", fmt.Errorf("upstream returned an unreadable response (status %d)", resp.StatusCode)
	}
	if decoded.Error != nil && decoded.Error.Message != "" {
		return "", fmt.Errorf("upstream error: %s", decoded.Error.Message)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("upstream returned status %d", resp.StatusCode)
	}
	if len(decoded.Choices) == 0 || strings.TrimSpace(decoded.Choices[0].Message.Content) == "" {
		return "", fmt.Errorf("upstream returned no content")
	}
	return decoded.Choices[0].Message.Content, nil
}

type blogAIGenerateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type blogAIDraftTranslation struct {
	Title          string `json:"title"`
	Summary        string `json:"summary"`
	Content        string `json:"content"`
	SeoDescription string `json:"seo_description"`
	Tags           string `json:"tags"`
}

type blogAIDraft struct {
	Title          string                            `json:"title"`
	Summary        string                            `json:"summary"`
	Content        string                            `json:"content"`
	SeoDescription string                            `json:"seo_description"`
	Tags           string                            `json:"tags"`
	Translations   map[string]blogAIDraftTranslation `json:"translations"`
}

// GenerateBlogPostWithAI drafts a multilingual post through an enabled
// channel. The draft is returned unsaved so the admin can review, edit and
// then publish from the editor.
func GenerateBlogPostWithAI(c *gin.Context) {
	var req blogAIGenerateRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	req.Model = strings.TrimSpace(req.Model)
	req.Prompt = strings.TrimSpace(req.Prompt)
	if req.Model == "" || req.Prompt == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "model and prompt are required"})
		return
	}
	if len(req.Prompt) > 20000 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "prompt is too long"})
		return
	}

	languages := GetSEOLanguages()
	var targets []string
	for _, lang := range languages {
		if lang != common.DefaultContentLanguage {
			targets = append(targets, lang)
		}
	}

	channels := enabledChannelsForModel(req.Model)
	if len(channels) == 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "no enabled channel serves this model"})
		return
	}

	messages := []blogAIChatMessage{
		{Role: "system", Content: buildBlogAISystemPrompt(targets)},
		{Role: "user", Content: req.Prompt},
	}

	var content string
	var lastErr error
	for _, channel := range channels {
		content, lastErr = callBlogAIChannel(channel, req.Model, messages)
		if lastErr == nil {
			break
		}
		common.SysError(fmt.Sprintf("blog AI generation via channel %d failed: %s", channel.Id, lastErr.Error()))
	}
	if lastErr != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": lastErr.Error()})
		return
	}

	jsonBytes, err := extractBlogAIJSON(content)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	var draft blogAIDraft
	if err := common.Unmarshal(jsonBytes, &draft); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "model returned invalid JSON: " + err.Error()})
		return
	}
	if strings.TrimSpace(draft.Title) == "" || strings.TrimSpace(draft.Content) == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "model returned an incomplete draft"})
		return
	}
	// Drop translations for languages that are not configured, so the draft
	// matches what the editor can persist.
	cleaned := map[string]blogAIDraftTranslation{}
	for lang, tr := range draft.Translations {
		code := common.NormalizeContentLanguage(lang)
		if code == "" || code == common.DefaultContentLanguage {
			continue
		}
		cleaned[code] = tr
	}
	draft.Translations = cleaned

	c.JSON(http.StatusOK, gin.H{"success": true, "data": draft})
}
