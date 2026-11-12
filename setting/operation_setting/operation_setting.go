package operation_setting

import "strings"

var DemoSiteEnabled = false
var SelfUseModeEnabled = false

// ModelsAPIRequireKey decides whether GET /v1/models needs an API key.
//
// It is on by default, which is how the endpoint behaved before the switch
// existed: a listing that anybody may read tells the world which models and
// which prices an installation runs. An administrator that publishes the
// catalogue (a landing page, a price comparison, a client that only lists
// models before it asks for a key) turns it off and the endpoint answers an
// anonymous request with the default group of the installation.
var ModelsAPIRequireKey = true

var AutomaticDisableKeywords = []string{
	"Your credit balance is too low",
	"This organization has been disabled.",
	"You exceeded your current quota",
	"Permission denied",
	"The security token included in the request is invalid",
	"Operation not allowed",
	"Your account is not authorized",
}

func AutomaticDisableKeywordsToString() string {
	return strings.Join(AutomaticDisableKeywords, "\n")
}

func AutomaticDisableKeywordsFromString(s string) {
	AutomaticDisableKeywords = []string{}
	ak := strings.Split(s, "\n")
	for _, k := range ak {
		k = strings.TrimSpace(k)
		k = strings.ToLower(k)
		if k != "" {
			AutomaticDisableKeywords = append(AutomaticDisableKeywords, k)
		}
	}
}
