package controller

import (
	"fmt"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

const maskedAccountSecret = "••••••••"

// DeploymentAccount represents a single configured account/credential set for a deployment provider.
type DeploymentAccount struct {
	ID           string   `json:"id"`
	Label        string   `json:"label"`
	Tier         string   `json:"tier"`
	Enabled      bool     `json:"enabled"`
	TokenID      string   `json:"token_id,omitempty"`
	TokenSecret  string   `json:"token_secret,omitempty"`
	Username     string   `json:"username,omitempty"`
	APIKey       string   `json:"api_key,omitempty"`
	Token        string   `json:"token,omitempty"`
	Org          string   `json:"org,omitempty"`
	AccessToken  string   `json:"access_token,omitempty"`
	RefreshToken string   `json:"refresh_token,omitempty"`
	ExpiresAt    string   `json:"expires_at,omitempty"`
	AccountEmail string   `json:"account_email,omitempty"`
	ProjectID    string   `json:"project_id,omitempty"`
	Region       string   `json:"region,omitempty"`
	EligibleGPUs []string `json:"eligible_gpus,omitempty"`
}

// DeploymentAccountSummary is the secret-free view sent to the frontend.
type DeploymentAccountSummary struct {
	ID             string   `json:"id"`
	Index          int      `json:"index"`
	Label          string   `json:"label"`
	Tier           string   `json:"tier"`
	Enabled        bool     `json:"enabled"`
	Configured     bool     `json:"configured"`
	Email          string   `json:"email,omitempty"`
	Username       string   `json:"username,omitempty"`
	Org            string   `json:"org,omitempty"`
	ProjectID      string   `json:"project_id,omitempty"`
	Region         string   `json:"region,omitempty"`
	EligibleGPUs   []string `json:"eligible_gpus,omitempty"`
	HasTokenID     bool     `json:"has_token_id,omitempty"`
	HasTokenSecret bool     `json:"has_token_secret,omitempty"`
	HasAPIKey      bool     `json:"has_api_key,omitempty"`
	HasToken       bool     `json:"has_token,omitempty"`
}

var deploymentAccountsMu sync.Mutex

func defaultTierForProvider(provider string) string {
	switch provider {
	case "modal":
		return "free_no_card"
	case "colab":
		return "free"
	case "kaggle":
		return "verified_free"
	default:
		return "standard"
	}
}

func isAccountConfigured(provider string, acc DeploymentAccount) bool {
	switch provider {
	case "modal":
		return strings.TrimSpace(acc.TokenID) != "" && strings.TrimSpace(acc.TokenSecret) != ""
	case "kaggle":
		return strings.TrimSpace(acc.Username) != "" && strings.TrimSpace(acc.APIKey) != ""
	case "colab":
		return strings.TrimSpace(acc.AccessToken) != "" || strings.TrimSpace(acc.RefreshToken) != ""
	case "huggingface":
		return strings.TrimSpace(acc.Token) != "" || strings.TrimSpace(acc.APIKey) != ""
	default:
		return strings.TrimSpace(acc.APIKey) != "" || strings.TrimSpace(acc.Token) != ""
	}
}

func persistDeploymentOption(key, value string) {
	common.OptionMapRWMutex.Lock()
	common.OptionMap[key] = value
	common.OptionMapRWMutex.Unlock()
	_ = model.UpdateOption(key, value)
}

func getLegacyProviderAccount(provider string) *DeploymentAccount {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()

	acc := DeploymentAccount{
		ID:      "acc-1",
		Label:   "#1",
		Tier:    defaultTierForProvider(provider),
		Enabled: true,
	}
	switch provider {
	case "modal":
		acc.TokenID = strings.TrimSpace(common.OptionMap["model_deployment.modal.token_id"])
		acc.TokenSecret = strings.TrimSpace(common.OptionMap["model_deployment.modal.token_secret"])
	case "kaggle":
		acc.Username = strings.TrimSpace(common.OptionMap["model_deployment.kaggle.username"])
		acc.APIKey = strings.TrimSpace(common.OptionMap["model_deployment.kaggle.api_key"])
	case "colab":
		acc.AccessToken = strings.TrimSpace(common.OptionMap["model_deployment.colab.access_token"])
		acc.RefreshToken = strings.TrimSpace(common.OptionMap["model_deployment.colab.refresh_token"])
		acc.ExpiresAt = strings.TrimSpace(common.OptionMap["model_deployment.colab.token_expires_at"])
		acc.AccountEmail = strings.TrimSpace(common.OptionMap["model_deployment.colab.account_email"])
		acc.ProjectID = strings.TrimSpace(common.OptionMap["model_deployment.colab.project_id"])
		acc.Region = strings.TrimSpace(common.OptionMap["model_deployment.colab.region"])
	case "huggingface":
		tok := strings.TrimSpace(common.OptionMap["model_deployment.huggingface.token"])
		if tok == "" {
			tok = strings.TrimSpace(common.OptionMap["model_deployment.huggingface.api_key"])
		}
		acc.Token = tok
		acc.Org = strings.TrimSpace(common.OptionMap["model_deployment.huggingface.org"])
	case "ionet":
		acc.APIKey = strings.TrimSpace(common.OptionMap["model_deployment.ionet.api_key"])
	case "runpod":
		acc.APIKey = strings.TrimSpace(common.OptionMap["model_deployment.runpod.api_key"])
	case "hostinger":
		acc.APIKey = strings.TrimSpace(common.OptionMap["model_deployment.hostinger.api_key"])
	}
	if !isAccountConfigured(provider, acc) {
		return nil
	}
	return &acc
}

// GetProviderAccounts returns configured accounts for a provider, falling back to legacy single-account options.
func GetProviderAccounts(provider string) []DeploymentAccount {
	p := strings.TrimSpace(strings.ToLower(provider))
	common.OptionMapRWMutex.RLock()
	raw := strings.TrimSpace(common.OptionMap["model_deployment."+p+".accounts"])
	common.OptionMapRWMutex.RUnlock()

	var accounts []DeploymentAccount
	if raw != "" {
		_ = common.Unmarshal([]byte(raw), &accounts)
	}
	if len(accounts) == 0 {
		if legacy := getLegacyProviderAccount(p); legacy != nil {
			accounts = []DeploymentAccount{*legacy}
		}
	}
	for i := range accounts {
		if strings.TrimSpace(accounts[i].ID) == "" {
			accounts[i].ID = fmt.Sprintf("acc-%d", i+1)
		}
		if strings.TrimSpace(accounts[i].Label) == "" {
			accounts[i].Label = fmt.Sprintf("#%d", i+1)
		}
		if strings.TrimSpace(accounts[i].Tier) == "" {
			accounts[i].Tier = defaultTierForProvider(p)
		}
	}
	return accounts
}

// SaveProviderAccounts saves the provider's account list and mirrors Account #1 to legacy option keys.
func SaveProviderAccounts(provider string, incoming []DeploymentAccount) error {
	p := strings.TrimSpace(strings.ToLower(provider))
	if !isValidDeploymentProvider(p) {
		return fmt.Errorf("unsupported deployment provider")
	}

	deploymentAccountsMu.Lock()
	defer deploymentAccountsMu.Unlock()

	existing := GetProviderAccounts(p)
	existingByID := make(map[string]DeploymentAccount, len(existing))
	for _, ex := range existing {
		existingByID[ex.ID] = ex
	}

	cleaned := make([]DeploymentAccount, 0, len(incoming))
	for i, acc := range incoming {
		acc.ID = strings.TrimSpace(acc.ID)
		if acc.ID == "" {
			acc.ID = fmt.Sprintf("acc-%s", common.GetUUID()[:8])
		}
		acc.Label = strings.TrimSpace(acc.Label)
		if acc.Label == "" {
			acc.Label = fmt.Sprintf("#%d", i+1)
		}
		acc.Tier = strings.TrimSpace(acc.Tier)
		if acc.Tier == "" {
			acc.Tier = defaultTierForProvider(p)
		}
		if prev, ok := existingByID[acc.ID]; ok {
			if acc.TokenID == "" || acc.TokenID == maskedAccountSecret {
				acc.TokenID = prev.TokenID
			}
			if acc.TokenSecret == "" || acc.TokenSecret == maskedAccountSecret {
				acc.TokenSecret = prev.TokenSecret
			}
			if acc.APIKey == "" || acc.APIKey == maskedAccountSecret {
				acc.APIKey = prev.APIKey
			}
			if acc.Token == "" || acc.Token == maskedAccountSecret {
				acc.Token = prev.Token
			}
			if acc.AccessToken == "" {
				acc.AccessToken = prev.AccessToken
			}
			if acc.RefreshToken == "" {
				acc.RefreshToken = prev.RefreshToken
			}
			if acc.ExpiresAt == "" {
				acc.ExpiresAt = prev.ExpiresAt
			}
			if acc.AccountEmail == "" {
				acc.AccountEmail = prev.AccountEmail
			}
			if len(acc.EligibleGPUs) == 0 {
				acc.EligibleGPUs = prev.EligibleGPUs
			}
		}
		cleaned = append(cleaned, acc)
	}

	encoded, err := common.Marshal(cleaned)
	if err != nil {
		return err
	}
	persistDeploymentOption("model_deployment."+p+".accounts", string(encoded))

	// Mirror first account back to legacy single-account option keys.
	if len(cleaned) > 0 {
		first := cleaned[0]
		switch p {
		case "modal":
			persistDeploymentOption("model_deployment.modal.token_id", first.TokenID)
			persistDeploymentOption("model_deployment.modal.token_secret", first.TokenSecret)
		case "kaggle":
			persistDeploymentOption("model_deployment.kaggle.username", first.Username)
			persistDeploymentOption("model_deployment.kaggle.api_key", first.APIKey)
		case "colab":
			persistDeploymentOption("model_deployment.colab.access_token", first.AccessToken)
			persistDeploymentOption("model_deployment.colab.refresh_token", first.RefreshToken)
			persistDeploymentOption("model_deployment.colab.token_expires_at", first.ExpiresAt)
			persistDeploymentOption("model_deployment.colab.account_email", first.AccountEmail)
			persistDeploymentOption("model_deployment.colab.project_id", first.ProjectID)
			persistDeploymentOption("model_deployment.colab.region", first.Region)
		case "huggingface":
			persistDeploymentOption("model_deployment.huggingface.token", first.Token)
			persistDeploymentOption("model_deployment.huggingface.api_key", first.Token)
			persistDeploymentOption("model_deployment.huggingface.org", first.Org)
		case "ionet":
			persistDeploymentOption("model_deployment.ionet.api_key", first.APIKey)
		case "runpod":
			persistDeploymentOption("model_deployment.runpod.api_key", first.APIKey)
		case "hostinger":
			persistDeploymentOption("model_deployment.hostinger.api_key", first.APIKey)
		}
	}
	return nil
}

// ResolveProviderAccount returns the account matching accountID (or first enabled configured account).
func ResolveProviderAccount(provider, accountID string) (*DeploymentAccount, error) {
	accounts := GetProviderAccounts(provider)
	target := strings.TrimSpace(accountID)
	if target != "" {
		for i := range accounts {
			if accounts[i].ID == target {
				return &accounts[i], nil
			}
		}
	}
	for i := range accounts {
		if accounts[i].Enabled && isAccountConfigured(provider, accounts[i]) {
			return &accounts[i], nil
		}
	}
	for i := range accounts {
		if isAccountConfigured(provider, accounts[i]) {
			return &accounts[i], nil
		}
	}
	return nil, fmt.Errorf("no configured %s account found", provider)
}

// SummarizeProviderAccounts returns secret-free account summaries for the UI.
func SummarizeProviderAccounts(provider string) []DeploymentAccountSummary {
	accounts := GetProviderAccounts(provider)
	out := make([]DeploymentAccountSummary, 0, len(accounts))
	for i, acc := range accounts {
		out = append(out, DeploymentAccountSummary{
			ID:             acc.ID,
			Index:          i + 1,
			Label:          acc.Label,
			Tier:           acc.Tier,
			Enabled:        acc.Enabled,
			Configured:     isAccountConfigured(provider, acc),
			Email:          acc.AccountEmail,
			Username:       acc.Username,
			Org:            acc.Org,
			ProjectID:      acc.ProjectID,
			Region:         acc.Region,
			EligibleGPUs:   acc.EligibleGPUs,
			HasTokenID:     strings.TrimSpace(acc.TokenID) != "",
			HasTokenSecret: strings.TrimSpace(acc.TokenSecret) != "",
			HasAPIKey:      strings.TrimSpace(acc.APIKey) != "",
			HasToken:       strings.TrimSpace(acc.Token) != "",
		})
	}
	return out
}

func GetAllDeploymentAccounts(c *gin.Context) {
	providers := []string{"modal", "colab", "kaggle", "huggingface", "runpod", "hostinger", "ionet"}
	result := make(map[string][]DeploymentAccountSummary, len(providers))
	for _, p := range providers {
		result[p] = SummarizeProviderAccounts(p)
	}
	common.ApiSuccess(c, result)
}

func UpdateProviderAccounts(c *gin.Context) {
	provider := strings.TrimSpace(strings.ToLower(c.Param("provider")))
	if !isValidDeploymentProvider(provider) {
		common.ApiErrorMsg(c, "unsupported deployment provider")
		return
	}
	var req struct {
		Accounts []DeploymentAccount `json:"accounts"`
	}
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "invalid request payload")
		return
	}
	if err := SaveProviderAccounts(provider, req.Accounts); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.ApiSuccess(c, SummarizeProviderAccounts(provider))
}
