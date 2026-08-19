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

	router.Use(static.Serve("/", frontendFS))
	router.NoRoute(func(c *gin.Context) {
		c.Set(middleware.RouteTagKey, "web")
		if strings.HasPrefix(c.Request.RequestURI, "/v1") || strings.HasPrefix(c.Request.RequestURI, "/api") || strings.HasPrefix(c.Request.RequestURI, "/assets") {
			controller.RelayNotFound(c)
			return
		}
		controller.ServeIndex(c, assets.IndexPage)
	})
}
