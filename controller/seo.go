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
// instead of the build-time defaults.
func RenderIndexPage(indexPage []byte) []byte {
	siteName := common.SystemName
	if siteName == "" {
		siteName = "New API"
	}
	prefix := seoOption("SEOTitlePrefix")
	title := siteName
	if prefix != "" {
		title = siteName + " - " + prefix
	}
	description := seoOption("SEODescription")
	keywords := seoOption("SEOKeywords")
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

// ServeIndex renders the SPA shell with SEO metadata applied. Rendering is
// cheap (a few string replacements) and always reflects the latest settings,
// so there is no caching here beyond the caller's Cache-Control header.
func ServeIndex(c *gin.Context, indexPage []byte) {
	c.Header("Cache-Control", "no-cache")
	c.Data(http.StatusOK, "text/html; charset=utf-8", RenderIndexPage(indexPage))
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

	page := RenderIndexPage(indexPage)
	siteName := common.SystemName
	title := post.Title + " - " + siteName
	description := post.SeoDescription
	if description == "" {
		description = post.Summary
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

	c.Header("Cache-Control", "no-cache")
	c.Data(http.StatusOK, "text/html; charset=utf-8", bytes.TrimSpace([]byte(pageStr)))
}
