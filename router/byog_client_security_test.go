package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestWorkerClientPageDisablesChildFrames(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/client", func(c *gin.Context) { secureWorkerClientPage(c); c.Status(http.StatusOK) })
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/client?lang=tr", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, strings.Contains(rec.Header().Get("Content-Security-Policy"), "frame-src 'none'"))
}
