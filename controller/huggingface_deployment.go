package controller

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/hfendpoints"
	"github.com/gin-gonic/gin"
)

func getHfToken(c *gin.Context) (string, bool) {
	common.OptionMapRWMutex.RLock()
	enabled := common.OptionMap["model_deployment.huggingface.enabled"] == "true"
	token := strings.TrimSpace(common.OptionMap["model_deployment.huggingface.token"])
	if token == "" {
		token = strings.TrimSpace(common.OptionMap["model_deployment.huggingface.api_key"])
	}
	defaultNamespace := strings.TrimSpace(common.OptionMap["model_deployment.huggingface.org"])
	defaultHardware := strings.TrimSpace(common.OptionMap["model_deployment.huggingface.hardware"])
	common.OptionMapRWMutex.RUnlock()
	if !enabled || token == "" {
		common.ApiErrorMsg(c, "Hugging Face deployment is not enabled or token missing")
		return "", false
	}
	c.Set("hf_default_namespace", defaultNamespace)
	c.Set("hf_default_hardware", defaultHardware)
	return token, true
}

func hfNamespace(c *gin.Context, explicit string) string {
	if strings.TrimSpace(explicit) != "" {
		return strings.TrimSpace(explicit)
	}
	if ns, ok := c.Get("hf_default_namespace"); ok {
		if s, ok := ns.(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

// ListHfEndpoints lists the Inference Endpoints of a namespace.
func ListHfEndpoints(c *gin.Context) {
	token, ok := getHfToken(c)
	if !ok {
		return
	}
	namespace := hfNamespace(c, c.Query("namespace"))
	endpoints, err := hfendpoints.NewClient(token).ListEndpoints(namespace)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    endpoints,
	})
}

// CreateHfEndpoint creates an Inference Endpoint.
func CreateHfEndpoint(c *gin.Context) {
	token, ok := getHfToken(c)
	if !ok {
		return
	}
	var req struct {
		Name       string `json:"name"`
		Namespace  string `json:"namespace"`
		Repository string `json:"repository"`
		Revision   string `json:"revision"`
		Task       string `json:"task"`
		Framework  string `json:"framework"`
		Hardware   string `json:"hardware"`
		Region     string `json:"region"`
		Protected  bool   `json:"protected"`
	}
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "invalid request payload")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Repository = strings.TrimSpace(req.Repository)
	if req.Name == "" || req.Repository == "" {
		common.ApiErrorMsg(c, "name and repository are required")
		return
	}
	if req.Task == "" {
		req.Task = "text-generation"
	}
	if req.Region == "" {
		req.Region = "us-east-1"
	}
	if req.Hardware == "" {
		if hw, ok := c.Get("hf_default_hardware"); ok {
			if s, ok := hw.(string); ok {
				req.Hardware = s
			}
		}
	}
	namespace := hfNamespace(c, req.Namespace)
	if namespace == "" {
		common.ApiErrorMsg(c, "namespace is required (set the organization in settings or pass it explicitly)")
		return
	}
	created, err := hfendpoints.NewClient(token).CreateEndpoint(hfendpoints.CreateEndpointRequest{
		Name:       req.Name,
		Namespace:  namespace,
		Repository: req.Repository,
		Revision:   req.Revision,
		Task:       req.Task,
		Framework:  req.Framework,
		Hardware:   req.Hardware,
		Region:     req.Region,
		Protected:  req.Protected,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    created,
	})
}

// GetHfEndpoint fetches one Inference Endpoint.
func GetHfEndpoint(c *gin.Context) {
	token, ok := getHfToken(c)
	if !ok {
		return
	}
	namespace := hfNamespace(c, c.Query("namespace"))
	name := strings.TrimSpace(c.Param("name"))
	if namespace == "" || name == "" {
		common.ApiErrorMsg(c, "namespace and name are required")
		return
	}
	endpoint, err := hfendpoints.NewClient(token).GetEndpoint(namespace, name)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    endpoint,
	})
}

// DeleteHfEndpoint removes an Inference Endpoint.
func DeleteHfEndpoint(c *gin.Context) {
	token, ok := getHfToken(c)
	if !ok {
		return
	}
	namespace := hfNamespace(c, c.Query("namespace"))
	name := strings.TrimSpace(c.Param("name"))
	if namespace == "" || name == "" {
		common.ApiErrorMsg(c, "namespace and name are required")
		return
	}
	if err := hfendpoints.NewClient(token).DeleteEndpoint(namespace, name); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}
