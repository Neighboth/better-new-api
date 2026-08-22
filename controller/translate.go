package controller

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

const (
	translateMaxTexts       = 200
	translateMaxTextChars   = 8000
	translateMaxTotalChars  = 200000
	translateMaxSourceChars = translateMaxTextChars * translateMaxTexts
)

type translateRequest struct {
	Source string   `json:"source"`
	Target string   `json:"target"`
	Texts  []string `json:"texts"`
}

// TranslateContent batch-translates public content strings (blog, legal,
// model descriptions, UI fallback bundles) through the machine translation
// service. English is the canonical source; missing translations fall back to
// the original text.
func TranslateContent(c *gin.Context) {
	var req translateRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	source := common.NormalizeContentLanguage(req.Source)
	if source == "" {
		source = common.DefaultContentLanguage
	}
	target := common.NormalizeContentLanguage(req.Target)
	if target == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "unsupported target language"})
		return
	}
	if len(req.Texts) == 0 || len(req.Texts) > translateMaxTexts {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "texts must contain 1-200 entries"})
		return
	}
	total := 0
	texts := make([]string, len(req.Texts))
	for i, text := range req.Texts {
		if len(text) > translateMaxTextChars {
			text = text[:translateMaxTextChars]
		}
		total += len(text)
		if total > translateMaxTotalChars {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "texts are too large"})
			return
		}
		texts[i] = text
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"texts": service.TranslateTexts(source, target, texts),
		},
	})
}

// GetContentLanguages lists every language public content can be served in.
// The frontend mirrors this list for boot-time URL prefix detection.
func GetContentLanguages(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    common.ContentLanguages,
	})
}

// contentLangFromRequest resolves the requested content language from the
// `lang` query parameter, the URL prefix captured by the language-prefix
// middleware, or the Accept-Language header. "" means the default (English).
func contentLangFromRequest(c *gin.Context) string {
	if lang := common.NormalizeContentLanguage(c.Query("lang")); lang != "" {
		return lang
	}
	if lang, exists := c.Get("content_lang"); exists {
		if code, ok := lang.(string); ok && code != "" {
			return code
		}
	}
	header := c.GetHeader("Accept-Language")
	for _, part := range strings.Split(header, ",") {
		tag := strings.TrimSpace(strings.SplitN(part, ";", 2)[0])
		if tag == "" {
			continue
		}
		if lang := common.NormalizeContentLanguage(tag); lang != "" {
			return lang
		}
		// Browsers send regional tags like "tr-TR"; retry with the base tag.
		if idx := strings.Index(tag, "-"); idx > 0 {
			if lang := common.NormalizeContentLanguage(tag[:idx]); lang != "" {
				return lang
			}
		}
	}
	return ""
}
