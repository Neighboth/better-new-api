package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

type OnlineSearchSetting struct {
	SearXNGURL      string  `json:"searxng_url"`
	PricePerSearch  float64 `json:"price_per_search"`
	MaxSearches     int     `json:"max_searches"`
}

var onlineSearchSetting = OnlineSearchSetting{
	SearXNGURL:     "",
	PricePerSearch: 0,
	MaxSearches:    10,
}

func init() {
	config.GlobalConfig.Register("online_search_setting", &onlineSearchSetting)
}

func GetOnlineSearchSetting() *OnlineSearchSetting {
	return &onlineSearchSetting
}
