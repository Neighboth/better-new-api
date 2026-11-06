package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestResellerRejectsOverflowAndInvalidBalanceTypes(t *testing.T) {
	for _, body := range []string{
		`{"quota":2147483647,"count":100,"type":0}`,
		`{"quota":1,"count":1,"type":1}`,
		`{"quota":1,"count":1,"type":2}`,
		`{"quota":1,"count":1,"type":3}`,
		`{"quota":1,"count":1,"type":-1}`,
	} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("id", 1)
		c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		ResellerCreateRedemption(c)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	}
}
