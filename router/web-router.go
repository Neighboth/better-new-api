package router

import (
	"embed"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-contrib/gzip"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
)

// WebAssets holds the embedded dashboard frontend assets.
type WebAssets struct {
	BuildFS   embed.FS
	IndexPage []byte
}

// contentLanguagePrefix detects URLs prefixed with a content language code
// ("/tr/blog/5", "/zh-CN/pricing"). The prefix is stripped so the SPA and its
// assets resolve unchanged — the API surface stays identical — while the
// detected language is stored on the context for localized SEO rendering
// (title, description, hreflang alternates) in ServeIndex/ServeBlogIndex.
// Unprefixed URLs keep working exactly as before.
func contentLanguagePrefix(next gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if len(path) > 2 && path[0] == '/' {
			rest := path[1:]
			seg := rest
			if idx := strings.Index(rest, "/"); idx >= 0 {
				seg = rest[:idx]
				rest = rest[idx:]
			} else {
				rest = "/"
			}
			if lang, ok := common.LookupContentLanguage(seg); ok {
				c.Set("content_lang", lang.Code)
				c.Set("content_path", rest)
				c.Request.URL.Path = rest
			}
		}
		next(c)
	}
}

func SetWebRouter(router *gin.Engine, assets WebAssets) {
	frontendFS := common.EmbedFolder(assets.BuildFS, "web/dist")

	router.Use(gzip.Gzip(gzip.DefaultCompression))
	router.Use(middleware.GlobalWebRateLimit())
	router.Use(middleware.Cache())

	// SEO endpoints must be registered before the static file server so they
	// win even if a same-named file ever lands in the build output.
	router.GET("/robots.txt", controller.BuildRobotsTxt)
	router.GET("/llms.txt", controller.GetLLMSTxt)
	router.GET("/llms-full.txt", controller.GetLLMSFullTxt)
	router.GET("/full-llms.txt", controller.GetLLMSFullTxt)
	router.GET("/sitemap.xml", controller.GetSitemapXML)
	router.GET("/blog/:id", func(c *gin.Context) {
		controller.ServeBlogIndex(c, assets.IndexPage)
	})

	router.Use(contentLanguagePrefix(static.Serve("/", frontendFS)))
	router.NoRoute(func(c *gin.Context) {
		c.Set(middleware.RouteTagKey, "web")
		if strings.HasPrefix(c.Request.RequestURI, "/v1") || strings.HasPrefix(c.Request.RequestURI, "/api") || strings.HasPrefix(c.Request.RequestURI, "/assets") {
			controller.RelayNotFound(c)
			return
		}
		// Language-prefixed blog articles were rewritten by the prefix
		// middleware; render them with post-specific SEO meta here.
		if _, exists := c.Get("content_lang"); exists && strings.HasPrefix(c.Request.URL.Path, "/blog/") {
			controller.ServeBlogIndex(c, assets.IndexPage)
			return
		}
		controller.ServeIndex(c, assets.IndexPage)
	})
}
