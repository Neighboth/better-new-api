package system_setting

import "github.com/QuantumNous/new-api/setting/config"

type LegalSettings struct {
	UserAgreement string `json:"user_agreement"`
	PrivacyPolicy string `json:"privacy_policy"`
	// Optional per-language variants keyed by content language code
	// ("tr", "fr", ...). The base fields above stay English; a missing
	// language is machine-translated on the fly and falls back to English.
	UserAgreementI18n map[string]string `json:"user_agreement_i18n"`
	PrivacyPolicyI18n map[string]string `json:"privacy_policy_i18n"`
}

var defaultLegalSettings = LegalSettings{
	UserAgreement: "",
	PrivacyPolicy: "",
}

func init() {
	config.GlobalConfig.Register("legal", &defaultLegalSettings)
}

func GetLegalSettings() *LegalSettings {
	return &defaultLegalSettings
}
