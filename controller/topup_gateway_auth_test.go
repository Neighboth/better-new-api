package controller

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestShopierRejectsInvalidSignature(t *testing.T) {
	old := setting.ShopierApiSecret
	setting.ShopierApiSecret = "test-secret"
	t.Cleanup(func() { setting.ShopierApiSecret = old })
	form := url.Values{"status": {"success"}, "platform_order_id": {"order"}, "random_nr": {"123"}, "signature": {"invalid"}}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ShopierNotify(c)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestShopifyRejectsMissingOrInvalidSignature(t *testing.T) {
	old := setting.ShopifyWebhookSecret
	setting.ShopifyWebhookSecret = "test-secret"
	t.Cleanup(func() { setting.ShopifyWebhookSecret = old })
	for _, signature := range []string{"", "invalid"} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"note_attributes":[]}`))
		c.Request.Header.Set("X-Shopify-Hmac-Sha256", signature)
		ShopifyWebhook(c)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	}
}
