package byog

import (
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
)

// testContext builds a gin context together with the recorder of its response.
func testContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/worker/models/test-model/file", nil)
	return context, recorder
}
