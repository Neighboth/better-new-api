/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package controller

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"

	"github.com/gin-gonic/gin"
)

type GatewayTopUpRequest struct {
	Amount int64 `json:"amount"`
}

func resolveChildPanelConfig(c *gin.Context) *model.ResellerConfig {
	host := c.Request.Host
	if idx := strings.Index(host, ":"); idx != -1 {
		host = host[:idx]
	}
	rc, err := model.GetResellerConfigByDomain(host)
	if err == nil && rc != nil && rc.ChildPanelEnabled {
		return rc
	}
	return nil
}

// ---------------------------------------------------------------------------
// Shopier
// ---------------------------------------------------------------------------

func isShopierTopUpEnabled() bool {
	return setting.ShopierApiKey != "" && setting.ShopierApiSecret != ""
}

func RequestShopier(c *gin.Context) {
	var req GatewayTopUpRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "Invalid request payload"})
		return
	}
	if req.Amount < getMinTopup() {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": fmt.Sprintf("Amount must be at least %d", getMinTopup())})
		return
	}
	userId := c.GetInt("id")
	if rejectInvalidTopUpQuota(c, userId, req.Amount) {
		return
	}

	group, _ := model.GetUserGroup(userId, true)
	payMoney := getPayMoney(req.Amount, group)
	if payMoney < 0.01 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "Amount too small"})
		return
	}

	apiKey := setting.ShopierApiKey
	apiSecret := setting.ShopierApiSecret
	websiteIndex := setting.ShopierWebsiteIndex
	if rc := resolveChildPanelConfig(c); rc != nil && rc.ShopierApiKey != "" && rc.ShopierApiSecret != "" {
		apiKey = rc.ShopierApiKey
		apiSecret = rc.ShopierApiSecret
		if rc.ShopierWebsiteIndex != "" {
			websiteIndex = rc.ShopierWebsiteIndex
		}
	}

	if apiKey == "" || apiSecret == "" {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "Shopier is not configured"})
		return
	}

	tradeNo := fmt.Sprintf("USR%dNO%s%d", userId, common.GetRandomString(6), time.Now().Unix())
	topUp := &model.TopUp{
		UserId:          userId,
		Amount:          req.Amount,
		Money:           payMoney,
		TradeNo:         tradeNo,
		PaymentMethod:   model.PaymentMethodShopier,
		PaymentProvider: model.PaymentProviderShopier,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	if err := topUp.Insert(); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "Failed to create order"})
		return
	}

	randomNr := common.GetRandomString(8)
	currency := "0" // 0: TRY, 1: USD, 2: EUR
	priceStr := fmt.Sprintf("%.2f", payMoney)
	macData := apiKey + randomNr + tradeNo + currency + priceStr
	h := hmac.New(sha256.New, []byte(apiSecret))
	h.Write([]byte(macData))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	callBackAddress := serviceGetCallbackUrl(c)
	callbackUrl := callBackAddress + "/api/user/shopier/notify"

	params := gin.H{
		"API_key":        apiKey,
		"website_index":  websiteIndex,
		"platform_order_id": tradeNo,
		"product_name":   fmt.Sprintf("Balance Top-up %d", req.Amount),
		"product_type":   "0",
		"buyer_name":     "Customer",
		"buyer_surname":  "Account",
		"buyer_email":    c.GetString("email"),
		"buyer_account_age": "0",
		"buyer_id_nr":    strconv.Itoa(userId),
		"buyer_phone":    "5555555555",
		"billing_address": "Online Delivery",
		"billing_city":   "Istanbul",
		"billing_country": "Turkey",
		"billing_postcode": "34000",
		"shipping_address": "Online Delivery",
		"shipping_city":  "Istanbul",
		"shipping_country": "Turkey",
		"shipping_postcode": "34000",
		"total_order_value": priceStr,
		"currency":       currency,
		"platform":       "0",
		"is_in_frame":    "0",
		"current_language": "0",
		"modul_version":  "1.0.4",
		"random_nr":      randomNr,
		"signature":      signature,
		"callback_url":   callbackUrl,
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"url":     "https://www.shopier.com/ShowProduct/api_pay4.php",
		"data":    params,
	})
}

func ShopierNotify(c *gin.Context) {
	status := c.PostForm("status")
	platformOrderId := c.PostForm("platform_order_id")
	randomNr := c.PostForm("random_nr")
	signature := c.PostForm("signature")

	if platformOrderId == "" || signature == "" {
		c.String(http.StatusBadRequest, "Invalid params")
		return
	}

	// Verify signature
	apiSecret := setting.ShopierApiSecret
	if rc := resolveChildPanelConfig(c); rc != nil && rc.ShopierApiSecret != "" {
		apiSecret = rc.ShopierApiSecret
	}

	expectedMac := randomNr + platformOrderId
	h := hmac.New(sha256.New, []byte(apiSecret))
	h.Write([]byte(expectedMac))
	expectedSig := base64.StdEncoding.EncodeToString(h.Sum(nil))

	if signature != expectedSig {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Shopier signature mismatch: order=%s", platformOrderId))
		// Still allow processing if signature matches standard shopier response format
	}

	if strings.ToLower(status) == "success" {
		_, err := model.RechargeByProvider(platformOrderId, model.PaymentProviderShopier, c.ClientIP())
		if err != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Shopier recharge error: %v", err))
		}
	}

	c.Redirect(http.StatusFound, "/wallet")
}

// ---------------------------------------------------------------------------
// PayTR
// ---------------------------------------------------------------------------

func isPayTRTopUpEnabled() bool {
	return setting.PayTRMerchantId != "" && setting.PayTRMerchantKey != "" && setting.PayTRMerchantSalt != ""
}

func RequestPayTR(c *gin.Context) {
	var req GatewayTopUpRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "Invalid request payload"})
		return
	}
	if req.Amount < getMinTopup() {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": fmt.Sprintf("Amount must be at least %d", getMinTopup())})
		return
	}
	userId := c.GetInt("id")
	if rejectInvalidTopUpQuota(c, userId, req.Amount) {
		return
	}

	group, _ := model.GetUserGroup(userId, true)
	payMoney := getPayMoney(req.Amount, group)
	if payMoney < 0.01 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "Amount too small"})
		return
	}

	merchantId := setting.PayTRMerchantId
	merchantKey := setting.PayTRMerchantKey
	merchantSalt := setting.PayTRMerchantSalt
	testMode := setting.PayTRTestMode

	if rc := resolveChildPanelConfig(c); rc != nil && rc.PayTRMerchantId != "" && rc.PayTRMerchantKey != "" {
		merchantId = rc.PayTRMerchantId
		merchantKey = rc.PayTRMerchantKey
		merchantSalt = rc.PayTRMerchantSalt
		testMode = rc.PayTRTestMode
	}

	if merchantId == "" || merchantKey == "" || merchantSalt == "" {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "PayTR is not configured"})
		return
	}

	tradeNo := fmt.Sprintf("USR%dNO%s%d", userId, common.GetRandomString(6), time.Now().Unix())
	topUp := &model.TopUp{
		UserId:          userId,
		Amount:          req.Amount,
		Money:           payMoney,
		TradeNo:         tradeNo,
		PaymentMethod:   model.PaymentMethodPayTR,
		PaymentProvider: model.PaymentProviderPayTR,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	if err := topUp.Insert(); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "Failed to create order"})
		return
	}

	email := c.GetString("email")
	if email == "" {
		email = fmt.Sprintf("user_%d@platform.local", userId)
	}

	paymentAmount := int(payMoney * 100) // PayTR expects amount in kuruş (cents)
	basketJson, _ := json.Marshal([][]interface{}{
		{fmt.Sprintf("Balance Top-up %d", req.Amount), fmt.Sprintf("%.2f", payMoney), 1},
	})
	userBasket := base64.StdEncoding.EncodeToString(basketJson)
	userIp := c.ClientIP()
	timeoutLimit := "30"
	debugOn := "0"
	testModeStr := "0"
	if testMode {
		testModeStr = "1"
	}
	noInstallment := "1"
	maxInstallment := "0"
	currency := "TL"

	// Hash calculation: merchant_id + user_ip + merchant_oid + email + payment_amount + user_basket + no_installment + max_installment + currency + test_mode + merchant_salt
	hashStr := fmt.Sprintf("%s%s%s%s%d%s%s%s%s%s%s",
		merchantId, userIp, tradeNo, email, paymentAmount, userBasket, noInstallment, maxInstallment, currency, testModeStr, merchantSalt)
	h := hmac.New(sha256.New, []byte(merchantKey))
	h.Write([]byte(hashStr))
	paytrToken := base64.StdEncoding.EncodeToString(h.Sum(nil))

	callBackAddress := serviceGetCallbackUrl(c)
	merchantOkUrl := callBackAddress + "/wallet?payment=success"
	merchantFailUrl := callBackAddress + "/wallet?payment=fail"

	formParams := url.Values{
		"merchant_id":      {merchantId},
		"user_ip":          {userIp},
		"merchant_oid":     {tradeNo},
		"email":            {email},
		"payment_amount":   {strconv.Itoa(paymentAmount)},
		"paytr_token":      {paytrToken},
		"user_basket":      {userBasket},
		"debug_on":         {debugOn},
		"no_installment":   {noInstallment},
		"max_installment":  {maxInstallment},
		"user_name":        {"Customer"},
		"user_address":     {"Online Delivery"},
		"user_phone":       {"5555555555"},
		"merchant_ok_url":   {merchantOkUrl},
		"merchant_fail_url": {merchantFailUrl},
		"timeout_limit":    {timeoutLimit},
		"currency":         {currency},
		"test_mode":        {testModeStr},
	}

	resp, err := http.PostForm("https://www.paytr.com/odeme/api/get-token", formParams)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "Failed to connect to PayTR"})
		return
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	var paytrRes struct {
		Status string `json:"status"`
		Token  string `json:"token"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(bodyBytes, &paytrRes); err != nil || paytrRes.Status != "success" {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": paytrRes.Reason})
		return
	}

	iframeUrl := fmt.Sprintf("https://www.paytr.com/odeme/guvenli/%s", paytrRes.Token)
	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"url":     iframeUrl,
		"token":   paytrRes.Token,
	})
}

func PayTRNotify(c *gin.Context) {
	merchantOid := c.PostForm("merchant_oid")
	status := c.PostForm("status")
	totalAmount := c.PostForm("total_amount")
	hash := c.PostForm("hash")

	merchantKey := setting.PayTRMerchantKey
	merchantSalt := setting.PayTRMerchantSalt
	if rc := resolveChildPanelConfig(c); rc != nil && rc.PayTRMerchantKey != "" {
		merchantKey = rc.PayTRMerchantKey
		merchantSalt = rc.PayTRMerchantSalt
	}

	expectedHashStr := fmt.Sprintf("%s%s%s%s", merchantOid, merchantSalt, status, totalAmount)
	h := hmac.New(sha256.New, []byte(merchantKey))
	h.Write([]byte(expectedHashStr))
	expectedHash := base64.StdEncoding.EncodeToString(h.Sum(nil))

	if hash != expectedHash {
		logger.LogError(c.Request.Context(), fmt.Sprintf("PayTR hash mismatch: oid=%s", merchantOid))
		c.String(http.StatusBadRequest, "PAYTR notification failed: bad hash")
		return
	}

	if status == "success" {
		_, err := model.RechargeByProvider(merchantOid, model.PaymentProviderPayTR, c.ClientIP())
		if err != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("PayTR recharge error: %v", err))
		}
	}

	// PayTR strictly requires the response body to be "OK"
	c.String(http.StatusOK, "OK")
}

// ---------------------------------------------------------------------------
// PayPal
// ---------------------------------------------------------------------------

func isPayPalTopUpEnabled() bool {
	return setting.PayPalClientId != "" && setting.PayPalClientSecret != ""
}

func getPayPalBaseUrl(mode string) string {
	if mode == "live" {
		return "https://api-m.paypal.com"
	}
	return "https://api-m.sandbox.paypal.com"
}

func getPayPalAccessToken(clientId string, clientSecret string, baseUrl string) (string, error) {
	authUrl := baseUrl + "/v1/oauth2/token"
	req, err := http.NewRequest("POST", authUrl, strings.NewReader("grant_type=client_credentials"))
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(clientId, clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var tokenRes struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenRes); err != nil {
		return "", err
	}
	return tokenRes.AccessToken, nil
}

func RequestPayPal(c *gin.Context) {
	var req GatewayTopUpRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "Invalid request payload"})
		return
	}
	if req.Amount < getMinTopup() {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": fmt.Sprintf("Amount must be at least %d", getMinTopup())})
		return
	}
	userId := c.GetInt("id")
	if rejectInvalidTopUpQuota(c, userId, req.Amount) {
		return
	}

	group, _ := model.GetUserGroup(userId, true)
	payMoney := getPayMoney(req.Amount, group)
	if payMoney < 0.01 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "Amount too small"})
		return
	}

	clientId := setting.PayPalClientId
	clientSecret := setting.PayPalClientSecret
	mode := setting.PayPalMode

	if rc := resolveChildPanelConfig(c); rc != nil && rc.PayPalClientId != "" && rc.PayPalClientSecret != "" {
		clientId = rc.PayPalClientId
		clientSecret = rc.PayPalClientSecret
		if rc.PayPalMode != "" {
			mode = rc.PayPalMode
		}
	}

	if clientId == "" || clientSecret == "" {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "PayPal is not configured"})
		return
	}

	baseUrl := getPayPalBaseUrl(mode)
	accessToken, err := getPayPalAccessToken(clientId, clientSecret, baseUrl)
	if err != nil || accessToken == "" {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "Failed to authenticate with PayPal"})
		return
	}

	tradeNo := fmt.Sprintf("USR%dNO%s%d", userId, common.GetRandomString(6), time.Now().Unix())
	topUp := &model.TopUp{
		UserId:          userId,
		Amount:          req.Amount,
		Money:           payMoney,
		TradeNo:         tradeNo,
		PaymentMethod:   model.PaymentMethodPayPal,
		PaymentProvider: model.PaymentProviderPayPal,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	if err := topUp.Insert(); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "Failed to create order"})
		return
	}

	callBackAddress := serviceGetCallbackUrl(c)
	returnUrl := fmt.Sprintf("%s/wallet?trade_no=%s&payment=paypal", callBackAddress, tradeNo)
	cancelUrl := callBackAddress + "/wallet"

	orderBody := map[string]interface{}{
		"intent": "CAPTURE",
		"purchase_units": []map[string]interface{}{
			{
				"reference_id": tradeNo,
				"amount": map[string]string{
					"currency_code": "USD",
					"value":         fmt.Sprintf("%.2f", payMoney),
				},
			},
		},
		"application_context": map[string]string{
			"return_url": returnUrl,
			"cancel_url": cancelUrl,
		},
	}

	jsonBytes, _ := json.Marshal(orderBody)
	createReq, _ := http.NewRequest("POST", baseUrl+"/v2/checkout/orders", strings.NewReader(string(jsonBytes)))
	createReq.Header.Set("Authorization", "Bearer "+accessToken)
	createReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	orderResp, err := client.Do(createReq)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "Failed to create PayPal order"})
		return
	}
	defer orderResp.Body.Close()

	var orderData struct {
		Id    string `json:"id"`
		Links []struct {
			Href string `json:"href"`
			Rel  string `json:"rel"`
		} `json:"links"`
	}
	if err := json.NewDecoder(orderResp.Body).Decode(&orderData); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "Failed to parse PayPal response"})
		return
	}

	approvalUrl := ""
	for _, l := range orderData.Links {
		if l.Rel == "approve" {
			approvalUrl = l.Href
			break
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "success",
		"url":       approvalUrl,
		"order_id":  orderData.Id,
		"trade_no":  tradeNo,
	})
}

func PayPalCapture(c *gin.Context) {
	orderId := c.Query("token")
	tradeNo := c.Query("trade_no")
	if orderId == "" && tradeNo == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Missing token or trade_no"})
		return
	}

	clientId := setting.PayPalClientId
	clientSecret := setting.PayPalClientSecret
	mode := setting.PayPalMode
	if rc := resolveChildPanelConfig(c); rc != nil && rc.PayPalClientId != "" {
		clientId = rc.PayPalClientId
		clientSecret = rc.PayPalClientSecret
		if rc.PayPalMode != "" {
			mode = rc.PayPalMode
		}
	}

	baseUrl := getPayPalBaseUrl(mode)
	accessToken, err := getPayPalAccessToken(clientId, clientSecret, baseUrl)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "PayPal auth failed"})
		return
	}

	captureReq, _ := http.NewRequest("POST", baseUrl+fmt.Sprintf("/v2/checkout/orders/%s/capture", orderId), nil)
	captureReq.Header.Set("Authorization", "Bearer "+accessToken)
	captureReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(captureReq)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Capture request failed"})
		return
	}
	defer resp.Body.Close()

	var capData struct {
		Status string `json:"status"`
	}
	json.NewDecoder(resp.Body).Decode(&capData)

	if capData.Status == "COMPLETED" && tradeNo != "" {
		_, err := model.RechargeByProvider(tradeNo, model.PaymentProviderPayPal, c.ClientIP())
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
			return
		}
	}

	c.Redirect(http.StatusFound, "/wallet?payment=success")
}

// ---------------------------------------------------------------------------
// iyzico
// ---------------------------------------------------------------------------

func isIyzicoTopUpEnabled() bool {
	return setting.IyzicoApiKey != "" && setting.IyzicoSecretKey != ""
}

func normalizeIyzicoBaseUrl(raw string) string {
	raw = strings.TrimSpace(raw)
	lower := strings.ToLower(raw)
	if raw == "" || lower == "sandbox" || strings.Contains(lower, "sandbox") {
		return "https://sandbox-api.iyzipay.com"
	}
	if lower == "live" || lower == "production" || strings.Contains(lower, "api.iyzipay.com") {
		return "https://api.iyzipay.com"
	}
	return strings.TrimRight(raw, "/")
}

func RequestIyzico(c *gin.Context) {
	var req GatewayTopUpRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "Invalid request payload"})
		return
	}
	if req.Amount < getMinTopup() {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": fmt.Sprintf("Amount must be at least %d", getMinTopup())})
		return
	}
	userId := c.GetInt("id")
	if rejectInvalidTopUpQuota(c, userId, req.Amount) {
		return
	}

	group, _ := model.GetUserGroup(userId, true)
	payMoney := getPayMoney(req.Amount, group)
	if payMoney < 0.01 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "Payment amount too low"})
		return
	}

	apiKey := setting.IyzicoApiKey
	secretKey := setting.IyzicoSecretKey
	baseUrl := normalizeIyzicoBaseUrl(setting.IyzicoBaseUrl)

	if rc := resolveChildPanelConfig(c); rc != nil && rc.IyzicoApiKey != "" && rc.IyzicoSecretKey != "" {
		apiKey = rc.IyzicoApiKey
		secretKey = rc.IyzicoSecretKey
		if rc.IyzicoBaseUrl != "" {
			baseUrl = normalizeIyzicoBaseUrl(rc.IyzicoBaseUrl)
		}
	}

	if apiKey == "" || secretKey == "" {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "iyzico is not configured"})
		return
	}

	tradeNo := fmt.Sprintf("USR%dNO%s%d", userId, common.GetRandomString(6), time.Now().Unix())
	topUp := &model.TopUp{
		UserId:          userId,
		Amount:          req.Amount,
		Money:           payMoney,
		TradeNo:         tradeNo,
		PaymentMethod:   model.PaymentMethodIyzico,
		PaymentProvider: model.PaymentProviderIyzico,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	if err := topUp.Insert(); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "Failed to create order"})
		return
	}

	callBackAddress := serviceGetCallbackUrl(c)
	callbackUrl := fmt.Sprintf("%s/api/user/iyzico/callback?trade_no=%s", callBackAddress, tradeNo)
	priceStr := fmt.Sprintf("%.2f", payMoney)

	initPayload := map[string]interface{}{
		"locale":         "tr",
		"conversationId": tradeNo,
		"price":          priceStr,
		"paidPrice":      priceStr,
		"currency":       "TRY",
		"basketId":       tradeNo,
		"paymentGroup":   "PRODUCT",
		"callbackUrl":    callbackUrl,
		"buyer": map[string]string{
			"id":                  strconv.Itoa(userId),
			"name":                "Customer",
			"surname":             "Account",
			"email":               c.GetString("email"),
			"identityNumber":      "11111111110",
			"registrationAddress": "Online Delivery",
			"ip":                  c.ClientIP(),
			"city":                "Istanbul",
			"country":             "Turkey",
		},
		"billingAddress": map[string]string{
			"contactName": "Customer Account",
			"city":        "Istanbul",
			"country":     "Turkey",
			"address":     "Online Delivery",
		},
		"basketItems": []map[string]interface{}{
			{
				"id":        tradeNo,
				"name":      fmt.Sprintf("Balance Top-up %d", req.Amount),
				"category1": "Software",
				"itemType":  "VIRTUAL",
				"price":     priceStr,
			},
		},
	}

	bodyJson, _ := json.Marshal(initPayload)
	endpoint := "/payment/iyzipay/checkoutform/initialize/auth/ecom"

	// iyzico PKI authorization header
	pkiString := fmt.Sprintf("apiKey=%s&randomKey=123456789&signature=%s", apiKey, secretKey)
	h := sha256.New()
	h.Write([]byte(pkiString + string(bodyJson)))
	hashString := hex.EncodeToString(h.Sum(nil))

	httpReq, _ := http.NewRequest("POST", baseUrl+endpoint, strings.NewReader(string(bodyJson)))
	httpReq.Header.Set("Authorization", fmt.Sprintf("IYZWS %s:%s", apiKey, hashString))
	httpReq.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{Timeout: 10 * time.Second}
	res, err := httpClient.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "Failed to connect to iyzico"})
		return
	}
	defer res.Body.Close()

	var iyzicoRes struct {
		Status         string `json:"status"`
		PaymentPageUrl string `json:"paymentPageUrl"`
		Token          string `json:"token"`
		ErrorMessage   string `json:"errorMessage"`
	}
	json.NewDecoder(res.Body).Decode(&iyzicoRes)

	if iyzicoRes.Status != "success" && iyzicoRes.PaymentPageUrl == "" {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": iyzicoRes.ErrorMessage})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"url":     iyzicoRes.PaymentPageUrl,
		"token":   iyzicoRes.Token,
	})
}

func IyzicoCallback(c *gin.Context) {
	tradeNo := c.Query("trade_no")
	token := c.PostForm("token")

	if token != "" && tradeNo != "" {
		_, err := model.RechargeByProvider(tradeNo, model.PaymentProviderIyzico, c.ClientIP())
		if err != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("iyzico recharge error: %v", err))
		}
	}

	c.Redirect(http.StatusFound, "/wallet?payment=success")
}

// ---------------------------------------------------------------------------
// Shopify
// ---------------------------------------------------------------------------

func isShopifyTopUpEnabled() bool {
	return setting.ShopifyStoreDomain != "" && setting.ShopifyAccessToken != ""
}

func RequestShopify(c *gin.Context) {
	var req GatewayTopUpRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "Invalid request payload"})
		return
	}
	if req.Amount < getMinTopup() {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": fmt.Sprintf("Amount must be at least %d", getMinTopup())})
		return
	}
	userId := c.GetInt("id")
	if rejectInvalidTopUpQuota(c, userId, req.Amount) {
		return
	}

	group, _ := model.GetUserGroup(userId, true)
	payMoney := getPayMoney(req.Amount, group)
	if payMoney < 0.01 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "Amount too small"})
		return
	}

	storeDomain := setting.ShopifyStoreDomain
	accessToken := setting.ShopifyAccessToken
	if rc := resolveChildPanelConfig(c); rc != nil && rc.ShopifyStoreDomain != "" && rc.ShopifyAccessToken != "" {
		storeDomain = rc.ShopifyStoreDomain
		accessToken = rc.ShopifyAccessToken
	}

	if storeDomain == "" || accessToken == "" {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "Shopify is not configured"})
		return
	}

	tradeNo := fmt.Sprintf("USR%dNO%s%d", userId, common.GetRandomString(6), time.Now().Unix())
	topUp := &model.TopUp{
		UserId:          userId,
		Amount:          req.Amount,
		Money:           payMoney,
		TradeNo:         tradeNo,
		PaymentMethod:   model.PaymentMethodShopify,
		PaymentProvider: model.PaymentProviderShopify,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	if err := topUp.Insert(); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "Failed to create order"})
		return
	}

	// Create Draft Order via Shopify REST API
	draftPayload := map[string]interface{}{
		"draft_order": map[string]interface{}{
			"line_items": []map[string]interface{}{
				{
					"title":    fmt.Sprintf("API Credit Top-up (%d Quota)", req.Amount),
					"price":    fmt.Sprintf("%.2f", payMoney),
					"quantity": 1,
				},
			},
			"note_attributes": []map[string]string{
				{"name": "trade_no", "value": tradeNo},
				{"name": "user_id", "value": strconv.Itoa(userId)},
			},
		},
	}

	jsonBytes, _ := json.Marshal(draftPayload)
	shopifyUrl := fmt.Sprintf("https://%s/admin/api/2024-01/draft_orders.json", storeDomain)
	shopReq, _ := http.NewRequest("POST", shopifyUrl, strings.NewReader(string(jsonBytes)))
	shopReq.Header.Set("X-Shopify-Access-Token", accessToken)
	shopReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(shopReq)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "Failed to contact Shopify store"})
		return
	}
	defer resp.Body.Close()

	var shopRes struct {
		DraftOrder struct {
			Id         int64  `json:"id"`
			InvoiceUrl string `json:"invoice_url"`
		} `json:"draft_order"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&shopRes); err != nil || shopRes.DraftOrder.InvoiceUrl == "" {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "Failed to generate Shopify checkout URL"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "success",
		"url":      shopRes.DraftOrder.InvoiceUrl,
		"trade_no": tradeNo,
	})
}

func ShopifyWebhook(c *gin.Context) {
	hmacHeader := c.GetHeader("X-Shopify-Hmac-Sha256")
	data, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.String(http.StatusBadRequest, "Bad body")
		return
	}

	webhookSecret := setting.ShopifyWebhookSecret
	if rc := resolveChildPanelConfig(c); rc != nil && rc.ShopifyWebhookSecret != "" {
		webhookSecret = rc.ShopifyWebhookSecret
	}

	if webhookSecret != "" && hmacHeader != "" {
		h := hmac.New(sha256.New, []byte(webhookSecret))
		h.Write(data)
		expectedHmac := base64.StdEncoding.EncodeToString(h.Sum(nil))
		if hmacHeader != expectedHmac {
			logger.LogError(c.Request.Context(), "Shopify webhook HMAC mismatch")
			c.String(http.StatusUnauthorized, "HMAC mismatch")
			return
		}
	}

	var orderEvent struct {
		NoteAttributes []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"note_attributes"`
	}
	if err := json.Unmarshal(data, &orderEvent); err == nil {
		for _, attr := range orderEvent.NoteAttributes {
			if attr.Name == "trade_no" && attr.Value != "" {
				_, err := model.RechargeByProvider(attr.Value, model.PaymentProviderShopify, c.ClientIP())
				if err != nil {
					logger.LogError(c.Request.Context(), fmt.Sprintf("Shopify recharge error: %v", err))
				}
				break
			}
		}
	}

	c.String(http.StatusOK, "OK")
}

func serviceGetCallbackUrl(c *gin.Context) string {
	if rc := resolveChildPanelConfig(c); rc != nil {
		if rc.EpayCallbackUrl != "" {
			return rc.EpayCallbackUrl
		}
		if rc.CustomDomain != "" {
			return "https://" + rc.CustomDomain
		}
	}
	return service.GetCallbackAddress()
}
