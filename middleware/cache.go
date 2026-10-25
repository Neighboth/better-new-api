package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

func Cache() func(c *gin.Context) {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		// Only cache hashed static assets, never cache HTML documents or SPA routes.
		isStaticAsset := strings.HasPrefix(path, "/assets/") ||
			strings.HasSuffix(path, ".js") ||
			strings.HasSuffix(path, ".css") ||
			strings.HasSuffix(path, ".woff2") ||
			strings.HasSuffix(path, ".woff") ||
			strings.HasSuffix(path, ".ttf")

		if isStaticAsset {
			c.Header("Cache-Control", "public, max-age=604800, immutable")
		} else {
			// All pages, HTML documents, and dynamic API/SPA routes must revalidate
			c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
			c.Header("Pragma", "no-cache")
			c.Header("Expires", "0")
		}
		c.Header("Cache-Version", "b688f2fb5be447c25e5aa3bd063087a83db32a288bf6a4f35f2d8db310e40b14")
		c.Next()
	}
}
