package controller

import (
	"bytes"
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
)

// Well-known AI training crawlers, toggled through the robots policy option.
var aiCrawlerAgents = []string{
	"GPTBot",
	"OAI-SearchBot",
	"ChatGPT-User",
	"ClaudeBot",
	"Claude-User",
	"anthropic-ai",
	"Google-Extended",
	"CCBot",
	"PerplexityBot",
	"Bytespider",
	"Amazonbot",
	"cohere-ai",
	"Meta-ExternalAgent",
	"Applebot-Extended",
}

func seoOption(key string) string {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	return strings.TrimSpace(common.OptionMap[key])
}

// seoLanguagesDefault is used when the SEOLanguages option has not been
// configured yet: the interface languages that already ship translations.
var seoLanguagesDefault = []string{"en", "zh-CN", "zh-TW", "fr", "ru", "ja", "vi"}

// GetSEOLanguages returns the normalized content languages the site advertises
// (hreflang alternates, AI blog translations, language-scoped SEO fields).
// English is always first.
func GetSEOLanguages() []string {
	raw := seoOption("SEOLanguages")
	langs := []string{common.DefaultContentLanguage}
	seen := map[string]struct{}{common.DefaultContentLanguage: {}}
	source := seoLanguagesDefault
	if raw != "" {
		source = strings.Split(raw, ",")
	}
	for _, code := range source {
		normalized := common.NormalizeContentLanguage(code)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		langs = append(langs, normalized)
	}
	return langs
}

// seoLocalizedEntry holds the language-scoped overrides stored in the
// SEOLocalized option: {"tr": {"title_prefix": "...", "description": "...",
// "keywords": "..."}}. English keeps using the base options so existing
// settings keep working unchanged.
type seoLocalizedEntry struct {
	TitlePrefix string `json:"title_prefix"`
	Description string `json:"description"`
	Keywords    string `json:"keywords"`
}

func seoLocalizedFor(lang string) seoLocalizedEntry {
	if lang == "" || lang == common.DefaultContentLanguage {
		return seoLocalizedEntry{}
	}
	raw := seoOption("SEOLocalized")
	if raw == "" {
		return seoLocalizedEntry{}
	}
	var all map[string]seoLocalizedEntry
	if err := common.Unmarshal([]byte(raw), &all); err != nil {
		return seoLocalizedEntry{}
	}
	entry, _ := all[lang]
	return entry
}

// injectHreflang writes <link rel="alternate"> tags for every configured
// language plus x-default (English), matching the language-prefixed URLs the
// site serves. pagePath is the request path without the language prefix.
func injectHreflang(page, base, pagePath string) string {
	if base == "" {
		return page
	}
	if pagePath == "" {
		pagePath = "/"
	}
	var b strings.Builder
	for _, lang := range GetSEOLanguages() {
		b.WriteString(`<link rel="alternate" hreflang="` + html.EscapeString(lang) + `" href="` + html.EscapeString(base+"/"+lang+pagePath) + `" />` + "\n    ")
	}
	b.WriteString(`<link rel="alternate" hreflang="x-default" href="` + html.EscapeString(base+"/"+common.DefaultContentLanguage+pagePath) + `" />` + "\n    ")
	idx := strings.Index(page, "</head>")
	if idx < 0 {
		return page
	}
	return page[:idx] + "    " + b.String() + page[idx:]
}

// setHTMLLang rewrites the <html lang="..."> attribute so crawlers and
// browsers see the served language.
func setHTMLLang(page, lang string) string {
	if lang == "" {
		return page
	}
	idx := strings.Index(page, "<html")
	if idx < 0 {
		return page
	}
	tagEnd := strings.Index(page[idx:], ">")
	if tagEnd < 0 {
		return page
	}
	tag := page[idx : idx+tagEnd]
	if !strings.Contains(tag, `lang="`) {
		return page[:idx+tagEnd] + ` lang="` + html.EscapeString(lang) + `"` + page[idx+tagEnd:]
	}
	start := strings.Index(tag, `lang="`) + len(`lang="`)
	end := strings.Index(tag[start:], `"`)
	if end < 0 {
		return page
	}
	newTag := tag[:start] + html.EscapeString(lang) + tag[start+end:]
	return page[:idx] + newTag + page[idx+tagEnd:]
}

func siteBaseURL(c *gin.Context) string {
	if base := strings.TrimRight(system_setting.ServerAddress, "/"); base != "" {
		return base
	}
	scheme := "https"
	if c.Request.TLS == nil {
		scheme = "http"
	}
	return scheme + "://" + c.Request.Host
}

// BuildRobotsTxt renders robots.txt from the configured policy:
// allow_all / block_all / block_ai / block_ai_training / custom.
func BuildRobotsTxt(c *gin.Context) {
	var b strings.Builder
	policy := seoOption("RobotsPolicy")
	customRules := seoOption("RobotsCustomRules")

	writeGroup := func(agent string, allow bool) {
		b.WriteString("User-agent: " + agent + "\n")
		if allow {
			b.WriteString("Allow: /\n\n")
		} else {
			b.WriteString("Disallow: /\n\n")
		}
	}

	switch policy {
	case "block_all":
		writeGroup("*", false)
	case "block_ai":
		writeGroup("*", true)
		for _, agent := range aiCrawlerAgents {
			writeGroup(agent, false)
		}
	case "custom":
		if customRules != "" {
			b.WriteString(customRules)
			if !strings.HasSuffix(customRules, "\n") {
				b.WriteString("\n")
			}
			b.WriteString("\n")
		} else {
			writeGroup("*", true)
		}
	default: // allow_all
		writeGroup("*", true)
	}

	if policy != "block_all" {
		b.WriteString("Sitemap: " + siteBaseURL(c) + "/sitemap.xml\n")
	}

	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, b.String())
}

func serveLLMSText(c *gin.Context, optionKey string) {
	content := seoOption(optionKey)
	if content == "" {
		c.Status(http.StatusNotFound)
		return
	}
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, content)
}

func GetLLMSTxt(c *gin.Context) {
	serveLLMSText(c, "LLMSTxt")
}

func GetLLMSFullTxt(c *gin.Context) {
	serveLLMSText(c, "LLMSFullTxt")
}

// GetSitemapXML lists the public pages plus every published blog post and any
// custom URLs the administrator added.
func GetSitemapXML(c *gin.Context) {
	base := siteBaseURL(c)
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")

	writeURL := func(loc string, lastmod string, priority string) {
		b.WriteString("  <url>\n")
		b.WriteString("    <loc>" + html.EscapeString(loc) + "</loc>\n")
		if lastmod != "" {
			b.WriteString("    <lastmod>" + lastmod + "</lastmod>\n")
		}
		b.WriteString("    <priority>" + priority + "</priority>\n")
		b.WriteString("  </url>\n")
	}

	writeURL(base+"/", "", "1.0")
	for _, path := range []string{"/pricing", "/about", "/docs"} {
		writeURL(base+path, "", "0.6")
	}

	if seoOption("BlogEnabled") == "true" {
		writeURL(base+"/blog", "", "0.8")
		if posts, err := model.ListPublishedBlogPostsForSitemap(); err == nil {
			for _, post := range posts {
				writeURL(base+fmt.Sprintf("/blog/%d", post.Id), post.UpdatedAt.UTC().Format(time.DateOnly), "0.7")
			}
		}
	}

	for _, raw := range strings.Split(seoOption("SitemapCustomUrls"), "\n") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if strings.HasPrefix(raw, "/") {
			raw = base + raw
		}
		if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
			continue
		}
		writeURL(raw, "", "0.5")
	}

	b.WriteString("</urlset>\n")
	c.Header("Content-Type", "application/xml; charset=utf-8")
	c.String(http.StatusOK, b.String())
}

// RenderIndexPage injects the configured SEO metadata into the SPA shell so
// crawlers see the site name, icon and description in the first HTML response
// instead of the build-time defaults. lang selects the language-scoped
// overrides (SEOLocalized); "" or "en" keeps the base options.
func RenderIndexPage(indexPage []byte, lang string) []byte {
	siteName := common.SystemName
	if siteName == "" {
		siteName = "New API"
	}
	localized := seoLocalizedFor(lang)
	prefix := seoOption("SEOTitlePrefix")
	if localized.TitlePrefix != "" {
		prefix = localized.TitlePrefix
	}
	title := siteName
	if prefix != "" {
		title = siteName + " - " + prefix
	}
	description := seoOption("SEODescription")
	if localized.Description != "" {
		description = localized.Description
	}
	keywords := seoOption("SEOKeywords")
	if localized.Keywords != "" {
		keywords = localized.Keywords
	}
	socialImage := seoOption("SEOSocialImage")
	icon := common.Logo
	if icon == "" {
		icon = "/logo.png"
	}

	page := string(indexPage)

	// Title + meta title/description.
	page = replaceTagContent(page, "<title>", "</title>", html.EscapeString(title))
	page = replaceMeta(page, `name="title"`, html.EscapeString(title))
	if description != "" {
		page = replaceMeta(page, `name="description"`, html.EscapeString(description))
	}

	// Favicon. The built HTML can contain more than one rel=icon link (the
	// bundler injects its own), and browsers honor the LAST one, so every
	// occurrence must be replaced.
	if icon != "" {
		page = replaceAllLinkHrefs(page, `rel="icon"`, html.EscapeString(icon))
	}

	// Extra tags go right before </head>.
	var extra strings.Builder
	if keywords != "" {
		extra.WriteString(`<meta name="keywords" content="` + html.EscapeString(keywords) + `" />` + "\n    ")
	}
	extra.WriteString(`<meta property="og:type" content="website" />` + "\n    ")
	extra.WriteString(`<meta property="og:title" content="` + html.EscapeString(title) + `" />` + "\n    ")
	if description != "" {
		extra.WriteString(`<meta property="og:description" content="` + html.EscapeString(description) + `" />` + "\n    ")
	}
	if socialImage != "" {
		extra.WriteString(`<meta property="og:image" content="` + html.EscapeString(socialImage) + `" />` + "\n    ")
		extra.WriteString(`<meta name="twitter:card" content="summary_large_image" />` + "\n    ")
		extra.WriteString(`<meta name="twitter:image" content="` + html.EscapeString(socialImage) + `" />` + "\n    ")
	} else {
		extra.WriteString(`<meta name="twitter:card" content="summary" />` + "\n    ")
	}
	extra.WriteString(`<meta name="twitter:title" content="` + html.EscapeString(title) + `" />` + "\n    ")
	if description != "" {
		extra.WriteString(`<meta name="twitter:description" content="` + html.EscapeString(description) + `" />` + "\n    ")
	}

	if lang != "" {
		page = setHTMLLang(page, lang)
	}
	idx := strings.Index(page, "</head>")
	if idx >= 0 {
		page = page[:idx] + "    " + extra.String() + page[idx:]
	}
	return []byte(page)
}

func replaceTagContent(page, open, close, content string) string {
	start := strings.Index(page, open)
	if start < 0 {
		return page
	}
	start += len(open)
	end := strings.Index(page[start:], close)
	if end < 0 {
		return page
	}
	return page[:start] + content + page[start+end:]
}

func replaceMeta(page, attr, content string) string {
	needle := "<meta " + attr
	idx := strings.Index(page, needle)
	if idx < 0 {
		return page
	}
	contentIdx := strings.Index(page[idx:], `content="`)
	if contentIdx < 0 {
		return page
	}
	contentStart := idx + contentIdx + len(`content="`)
	contentEnd := strings.Index(page[contentStart:], `"`)
	if contentEnd < 0 {
		return page
	}
	return page[:contentStart] + content + page[contentStart+contentEnd:]
}

func replaceLinkHref(page, attr, href string) string {
	needle := "<link " + attr
	idx := strings.Index(page, needle)
	if idx < 0 {
		return page
	}
	hrefIdx := strings.Index(page[idx:], `href="`)
	if hrefIdx < 0 {
		return page
	}
	hrefStart := idx + hrefIdx + len(`href="`)
	hrefEnd := strings.Index(page[hrefStart:], `"`)
	if hrefEnd < 0 {
		return page
	}
	return page[:hrefStart] + href + page[hrefStart+hrefEnd:]
}

// replaceAllLinkHrefs applies replaceLinkHref to every matching link tag.
func replaceAllLinkHrefs(page, attr, href string) string {
	needle := "<link " + attr
	var out strings.Builder
	rest := page
	for {
		idx := strings.Index(rest, needle)
		if idx < 0 {
			out.WriteString(rest)
			break
		}
		tagEnd := strings.Index(rest[idx:], ">")
		if tagEnd < 0 {
			out.WriteString(rest)
			break
		}
		tagEnd += idx + 1
		out.WriteString(rest[:idx])
		out.WriteString(replaceLinkHref(rest[idx:tagEnd], attr, href))
		rest = rest[tagEnd:]
	}
	return out.String()
}

// servePagePath returns the request path without the language prefix, for
// building hreflang alternates.
func servePagePath(c *gin.Context) string {
	if path, exists := c.Get("content_path"); exists {
		if p, ok := path.(string); ok && p != "" {
			return p
		}
	}
	return c.Request.URL.Path
}

// ServeIndex renders the SPA shell with SEO metadata applied. Rendering is
// cheap (a few string replacements) and always reflects the latest settings,
// so there is no caching here beyond the caller's Cache-Control header.
func ServeIndex(c *gin.Context, indexPage []byte) {
	langValue, _ := c.Get("content_lang")
	langCode, _ := langValue.(string)
	page := string(RenderIndexPage(indexPage, langCode))
	page = injectHreflang(page, siteBaseURL(c), servePagePath(c))
	c.Header("Cache-Control", "no-cache")
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(page))
}

// BlogPostIndex renders the SPA shell with post-specific SEO meta for the
// /blog/:id pages so shared links unfurl with the article title and cover.
func ServeBlogIndex(c *gin.Context, indexPage []byte) {
	idStr := strings.TrimPrefix(c.Request.URL.Path, "/blog/")
	if idStr == "" || idStr == c.Request.URL.Path {
		ServeIndex(c, indexPage)
		return
	}
	var id int64
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil || id <= 0 {
		ServeIndex(c, indexPage)
		return
	}
	post, err := model.GetBlogPostById(id)
	if err != nil || !post.Published {
		ServeIndex(c, indexPage)
		return
	}

	langValue, _ := c.Get("content_lang")
	langCode, _ := langValue.(string)
	// Crawler HTML uses stored translations only (base English as fallback) so
	// bot traffic never fans out into machine translation calls.
	fields := storedBlogFields(post, langCode)
	page := RenderIndexPage(indexPage, langCode)
	siteName := common.SystemName
	title := fields.title + " - " + siteName
	description := fields.seoDesc
	if description == "" {
		description = fields.summary
	}

	pageStr := string(page)
	pageStr = replaceTagContent(pageStr, "<title>", "</title>", html.EscapeString(title))
	pageStr = replaceMeta(pageStr, `name="title"`, html.EscapeString(title))
	if description != "" {
		pageStr = replaceMeta(pageStr, `name="description"`, html.EscapeString(description))
		pageStr = replaceMeta(pageStr, `property="og:description"`, html.EscapeString(description))
		pageStr = replaceMeta(pageStr, `name="twitter:description"`, html.EscapeString(description))
	}
	pageStr = replaceMeta(pageStr, `property="og:title"`, html.EscapeString(title))
	pageStr = replaceMeta(pageStr, `name="twitter:title"`, html.EscapeString(title))
	if post.CoverImage != "" {
		pageStr = replaceMeta(pageStr, `property="og:image"`, html.EscapeString(post.CoverImage))
		pageStr = replaceMeta(pageStr, `name="twitter:image"`, html.EscapeString(post.CoverImage))
	}
	pageStr = replaceMeta(pageStr, `property="og:type"`, "article")
	pageStr = injectHreflang(pageStr, siteBaseURL(c), servePagePath(c))

	c.Header("Cache-Control", "no-cache")
	c.Data(http.StatusOK, "text/html; charset=utf-8", bytes.TrimSpace([]byte(pageStr)))
}
