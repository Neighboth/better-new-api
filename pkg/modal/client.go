package modal

import (
	"context"
	"fmt"
	"time"
)

// DeploymentConfig holds configuration for a Modal deployment
type DeploymentConfig struct {
	Priority           int           `json:"priority"` // e.g., 1 for Modal, 2 for io.net, 3 for standard channels
	ModelName          string        `json:"model_name"`
	UseVolumeStorage   bool          `json:"use_volume_storage"` // Handles 1TB storage
	VolumeName         string        `json:"volume_name"`        // e.g., "model-weights-1tb"
	IdleTimeout        time.Duration `json:"idle_timeout"`       // Sleeps system when unused
	ContainerKeepWarm  int           `json:"container_keep_warm"`
	ConcurrencyLimit   int           `json:"concurrency_limit"`
	DeploymentProvider string        `json:"deployment_provider"` // "modal" or "io.net"
}

// ModalClient manages Modal.com deployments
type ModalClient struct {
	APIKey string
}

func NewClient(apiKey string) *ModalClient {
	return &ModalClient{APIKey: apiKey}
}

// DeployModel deploys a model to Modal, using Volume for storage and IdleTimeout for scaling to zero
func (c *ModalClient) DeployModel(ctx context.Context, config DeploymentConfig) error {
	// Configure the modal.Volume to handle 1TB of model weights without redownloading
	if config.UseVolumeStorage {
		fmt.Printf("Mounting 1TB Volume '%s' for Modal deployment to save GPU download costs\n", config.VolumeName)
	}

	// Configure IdleTimeout for sleeping the system
	fmt.Printf("Configured IdleTimeout of %v to scale to zero when unused\n", config.IdleTimeout)

	// In a real implementation, we would call Modal's API or generate a modal.py script and run `modal deploy`
	fmt.Printf("Deploying model %s to Modal with priority %d\n", config.ModelName, config.Priority)

	return nil
}

// GetDeploymentPriority returns the highest priority deployment provider for a model
func GetDeploymentPriority(modelName string, configs []DeploymentConfig) *DeploymentConfig {
	var best *DeploymentConfig
	for i, cfg := range configs {
		if cfg.ModelName == modelName {
			if best == nil || cfg.Priority > best.Priority {
				best = &configs[i]
			}
		}
	}
	return best
}
