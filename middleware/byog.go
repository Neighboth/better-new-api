package middleware

import (
	"net/http"

	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// ByogEnabled rejects the BYOG endpoints of the users and the download of the
// worker application when the feature is switched off in the System Settings.
//
// The dashboard hides the pages of a disabled module, but the address of the
// page is public knowledge: without this guard /dashboard/byog answers the
// status request of the page and the worker binaries stay downloadable, even
// when the administrator turned the module off.
func ByogEnabled() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !model.IsByogGloballyEnabled() {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "BYOG is disabled on this installation",
				"code":    "byog_disabled",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
