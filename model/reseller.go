package model

import (
	"strings"

	"gorm.io/gorm"
)

type ResellerConfig struct {
	Id                int            `json:"id" gorm:"primaryKey"`
	UserId            int            `json:"user_id" gorm:"uniqueIndex;not null"`
	ChildPanelEnabled bool           `json:"child_panel_enabled" gorm:"default:false"`
	CustomDomain      string         `json:"custom_domain" gorm:"type:varchar(255);index"`
	SiteName          string         `json:"site_name" gorm:"type:varchar(255)"`
	Logo              string         `json:"logo" gorm:"type:text"`
	Favicon           string         `json:"favicon" gorm:"type:text"`
	SeoTitle          string         `json:"seo_title" gorm:"type:varchar(255)"`
	SeoDescription    string         `json:"seo_description" gorm:"type:text"`
	SeoKeywords       string         `json:"seo_keywords" gorm:"type:text"`
	HomepageContent      string         `json:"homepage_content" gorm:"type:text"`
	EpayPartnerId        string         `json:"epay_partner_id" gorm:"type:varchar(255)"`
	EpayKey              string         `json:"epay_key" gorm:"type:varchar(255)"`
	EpayUrl              string         `json:"epay_url" gorm:"type:varchar(255)"`
	EpayCallbackUrl      string         `json:"epay_callback_url" gorm:"type:varchar(255)"`
	StripeApiSecret      string         `json:"stripe_api_secret" gorm:"type:varchar(255)"`
	StripeWebhookSecret  string         `json:"stripe_webhook_secret" gorm:"type:varchar(255)"`
	StripePriceId        string         `json:"stripe_price_id" gorm:"type:varchar(255)"`
	CreemApiKey          string         `json:"creem_api_key" gorm:"type:varchar(255)"`
	CreemWebhookSecret   string         `json:"creem_webhook_secret" gorm:"type:varchar(255)"`
	CreemTestMode        bool           `json:"creem_test_mode" gorm:"default:false"`
	WaffoMerchantId      string         `json:"waffo_merchant_id" gorm:"type:varchar(255)"`
	WaffoApiKey          string         `json:"waffo_api_key" gorm:"type:varchar(255)"`
	WaffoPrivateKey      string         `json:"waffo_private_key" gorm:"type:text"`
	ShopierApiKey        string         `json:"shopier_api_key" gorm:"type:varchar(255)"`
	ShopierApiSecret     string         `json:"shopier_api_secret" gorm:"type:varchar(255)"`
	ShopierWebsiteIndex  string         `json:"shopier_website_index" gorm:"type:varchar(255)"`
	PayTRMerchantId      string         `json:"paytr_merchant_id" gorm:"type:varchar(255)"`
	PayTRMerchantKey     string         `json:"paytr_merchant_key" gorm:"type:varchar(255)"`
	PayTRMerchantSalt    string         `json:"paytr_merchant_salt" gorm:"type:varchar(255)"`
	PayTRTestMode        bool           `json:"paytr_test_mode" gorm:"default:false"`
	PayPalClientId       string         `json:"paypal_client_id" gorm:"type:varchar(255)"`
	PayPalClientSecret   string         `json:"paypal_client_secret" gorm:"type:varchar(255)"`
	PayPalMode           string         `json:"paypal_mode" gorm:"type:varchar(255);default:'sandbox'"`
	IyzicoApiKey         string         `json:"iyzico_api_key" gorm:"type:varchar(255)"`
	IyzicoSecretKey      string         `json:"iyzico_secret_key" gorm:"type:varchar(255)"`
	IyzicoBaseUrl        string         `json:"iyzico_base_url" gorm:"type:varchar(255)"`
	ShopifyStoreDomain   string         `json:"shopify_store_domain" gorm:"type:varchar(255)"`
	ShopifyAccessToken   string         `json:"shopify_access_token" gorm:"type:varchar(255)"`
	ShopifyWebhookSecret string         `json:"shopify_webhook_secret" gorm:"type:varchar(255)"`
	CreatedAt            int64          `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt            int64          `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt            gorm.DeletedAt `json:"-" gorm:"index"`
}

func GetResellerConfigByUserId(userId int) (*ResellerConfig, error) {
	var config ResellerConfig
	err := DB.Where("user_id = ?", userId).First(&config).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func GetOrCreateResellerConfig(userId int) (*ResellerConfig, error) {
	var config ResellerConfig
	err := DB.Where("user_id = ?", userId).First(&config).Error
	if err == nil {
		return &config, nil
	}
	if err == gorm.ErrRecordNotFound {
		config = ResellerConfig{
			UserId:            userId,
			ChildPanelEnabled: false,
		}
		if createErr := DB.Create(&config).Error; createErr != nil {
			return nil, createErr
		}
		return &config, nil
	}
	return nil, err
}

func GetResellerConfigByDomain(domain string) (*ResellerConfig, error) {
	domain = strings.TrimSpace(strings.ToLower(domain))
	if domain == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var config ResellerConfig
	err := DB.Where("LOWER(custom_domain) = ? AND child_panel_enabled = ?", domain, true).First(&config).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func UpdateResellerConfig(config *ResellerConfig) error {
	return DB.Save(config).Error
}
