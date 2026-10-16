package service

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/ionet"
)

// ServerlessCache holds active serverless deployments and priority settings
type ServerlessCache struct {
	sync.RWMutex
	// maps modelName -> io.net API base URL (dummy URL for now until Modal/io.net APIs finalize endpoints)
	activeDeployments map[string]string
	// maps modelName -> ServerlessPriority (0=Channels First, 1=Serverless First)
	modelPriority     map[string]int
}

var serverlessCache *ServerlessCache

func init() {
	serverlessCache = &ServerlessCache{
		activeDeployments: make(map[string]string),
		modelPriority:     make(map[string]int),
	}
}

// SyncServerlessCache syncs Modal and io.net deployments and model priorities periodically
func SyncServerlessCache(frequency int) {
	if frequency == 0 {
		return
	}
	for {
		time.Sleep(time.Duration(frequency) * time.Second)
		common.SysLog("syncing serverless deployments from io.net / Modal")

		// 1. Sync Model Priorities from DB
		limit := 1000
		offset := 0
		var allModels []*model.Model
		newPriorityMap := make(map[string]int)

		for {
			models, _, err := model.SearchModels("", "", "", "", offset, limit)
			if err != nil {
				common.SysLog(fmt.Sprintf("error fetching models for serverless cache: %v", err))
				break
			}
			allModels = append(allModels, models...)
			if len(models) < limit {
				break
			}
			offset += limit
		}

		for _, m := range allModels {
			newPriorityMap[m.ModelName] = m.ServerlessPriority
		}

		// 2. Fetch Active io.net Deployments
		newActiveDeployments := make(map[string]string)

		common.OptionMapRWMutex.RLock()
		apiKey := common.OptionMap["model_deployment.ionet.api_key"]
		enabled := common.OptionMap["model_deployment.ionet.enabled"] == "true"
		common.OptionMapRWMutex.RUnlock()

		if enabled && strings.TrimSpace(apiKey) != "" {
			client := ionet.NewEnterpriseClient(apiKey)
			opts := &ionet.ListDeploymentsOptions{
				Status:    "running",
				PageSize:  1000,
			}
			dl, err := client.ListDeployments(opts)
			if err != nil {
				common.SysLog(fmt.Sprintf("failed to fetch io.net deployments: %v", err))
			} else {
				for _, d := range dl.Deployments {
					// We currently lack a direct "modelName" field on ionet.Deployment.
					// In a real system, you might encode the modelName in the DeploymentName or Description.
					// We'll simulate finding it if it matches the format "new-api-model-{modelName}"
					if strings.HasPrefix(d.Name, "new-api-model-") {
						mName := strings.TrimPrefix(d.Name, "new-api-model-")
						// Since we don't have the exact API endpoint format yet, we simulate one:
						endpoint := fmt.Sprintf("https://io.net/api/v1/%s", d.ID)
						newActiveDeployments[mName] = endpoint
					}
				}
			}
		}

		// (Optional) Fetch Active Modal Deployments
		// ... when Modal package gets a "List Deployments" returning names ...

		// 3. Update the global cache
		serverlessCache.Lock()
		serverlessCache.activeDeployments = newActiveDeployments
		serverlessCache.modelPriority = newPriorityMap
		serverlessCache.Unlock()
	}
}

// GetServerlessPriority gets the priority (0 or 1) for a model name
func GetServerlessPriority(modelName string) int {
	serverlessCache.RLock()
	defer serverlessCache.RUnlock()

	if p, ok := serverlessCache.modelPriority[modelName]; ok {
		return p
	}
	return 0
}

// GetActiveServerlessChannel constructs a dummy Channel if an active serverless endpoint exists
func GetActiveServerlessChannel(modelName string) *model.Channel {
	serverlessCache.RLock()
	defer serverlessCache.RUnlock()

	// Direct lookup for model
	if endpoint, ok := serverlessCache.activeDeployments[modelName]; ok {
		return constructServerlessChannel(endpoint)
	}

	// Case insensitive/normalized lookup
	safeName := strings.ToLower(modelName)
	if endpoint, ok := serverlessCache.activeDeployments[safeName]; ok {
		return constructServerlessChannel(endpoint)
	}

	return nil
}

func constructServerlessChannel(endpoint string) *model.Channel {
	if strings.HasPrefix(endpoint, "https://io.net/api/v1/") || strings.TrimSpace(endpoint) == "" {
		return nil
	}
	common.OptionMapRWMutex.RLock()
	apiKey := strings.TrimSpace(common.OptionMap["model_deployment.ionet.api_key"])
	common.OptionMapRWMutex.RUnlock()
	if apiKey == "" {
		return nil
	}
	return &model.Channel{
		Id:      999999, // A high ID so it doesn't conflict with real channels
		Type:    constant.ChannelTypeOpenAI,
		Name:    "Serverless Model (io.net/Modal)",
		Key:     apiKey,
		BaseURL: &endpoint,
	}
}
