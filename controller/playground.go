package controller

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
)

func Playground(c *gin.Context) {
	playgroundRelay(c, types.RelayFormatOpenAI)
}

// PlaygroundImages relays playground image generation requests.
func PlaygroundImages(c *gin.Context) {
	if c.GetBool("use_access_token") {
		playgroundRelay(c, types.RelayFormatOpenAIImage)
		return
	}

	// Parse the body once. Studio requests carry the studio_optimize flag —
	// the prompt optimization then runs entirely on the server; the browser
	// just waits for the image.
	bodyBytes, readErr := io.ReadAll(c.Request.Body)
	imageReq := map[string]interface{}{}
	studioOptimize := false
	if readErr == nil && len(bodyBytes) > 0 {
		_ = common.Unmarshal(bodyBytes, &imageReq)
		if md, ok := imageReq["metadata"].(map[string]interface{}); ok {
			studioOptimize = metadataStudioFlag(md, "studio_optimize")
		}
	}

	modelName, _ := imageReq["model"].(string)
	size, _ := imageReq["size"].(string)
	quality, _ := imageReq["quality"].(string)
	md, mdOk := imageReq["metadata"].(map[string]interface{})
	if !mdOk {
		md = nil
	}

	// Rule enforcement runs before anything else.
	if !enforceStudioDiscordGate(c, modelName) {
		return
	}
	if !enforceStudioImageRules(c, modelName, size, quality, md) {
		return
	}

	userId := c.GetInt("id")
	optimized := (*StudioOptimizeResult)(nil)
	originalPrompt, _ := imageReq["prompt"].(string)
	if studioOptimize {
		group := c.GetString("group")
		if group == "" {
			if userCache, err := model.GetUserCache(userId); err == nil && userCache != nil {
				group = userCache.Group
			}
			if group == "" {
				group = "default"
			}
		}
		reqTr := &relaycommon.TaskSubmitReq{
			Prompt:   originalPrompt,
			Model:    modelName,
			Quality:  quality,
			Metadata: md,
		}
		var optimizeErr error
		optimized, optimizeErr = RunStudioOptimization(c, userId, group, reqTr, "image")
		if optimizeErr != nil {
			if optimizeErr.Error() == "prompt_optimizer_not_configured" {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": gin.H{
						"message": "Prompt optimizer is not configured. Open System Settings → Playground & Studio → Studio Assistant Model and set a vision-capable model.",
						"type":    "invalid_request_error",
					},
				})
				return
			}
			optimized = &StudioOptimizeResult{Prompt: originalPrompt, Fallback: true}
		}
		if optimized != nil {
			imageReq["prompt"] = optimized.Prompt
			if md == nil {
				md = map[string]interface{}{}
			}
			if optimized.NegativePrompt != "" {
				md["negative_prompt"] = optimized.NegativePrompt
			}
			if optimized.References != nil {
				refs := make([]interface{}, 0, len(optimized.References))
				for _, ref := range optimized.References {
					refs = append(refs, ref)
				}
				md["reference_images"] = refs
			}
			imageReq["metadata"] = md
		}
		bodyBytes, _ = common.Marshal(imageReq)
	}
	if len(bodyBytes) > 0 {
		c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		c.Request.ContentLength = int64(len(bodyBytes))
	}

	playgroundRelay(c, types.RelayFormatOpenAIImage)

	if studioOptimize && md != nil {
		if itemId := metadataStringValue(md, "studio_item_id"); itemId != "" {
			overlay := model.StudioItemOverlay{
				Prompt:   originalPrompt,
				Settings: map[string]string{"model": modelName},
			}
			if optimized != nil {
				overlay.OptimizedPrompt = optimized.Prompt
				overlay.NegativePrompt = optimized.NegativePrompt
				overlay.References = optimized.References
			}
			if err := model.OverlayStudioItem(userId, itemId, overlay, time.Now().Unix()); err != nil {
				common.SysError(fmt.Sprintf("studio: overlay for image item %s failed: %s", itemId, err.Error()))
			}
		}
	}
}

// PlaygroundVideo submits an asynchronous video generation task with the
// session of the dashboard user. The task is polled afterwards through
// /api/task/self, which already belongs to the user.
func PlaygroundVideo(c *gin.Context) {
	if c.GetBool("use_access_token") {
		c.JSON(http.StatusForbidden, gin.H{
			"error": gin.H{"message": "access token is not supported for this endpoint", "type": "invalid_request_error"},
		})
		return
	}

	// Parse the body once. When the request carries the studio_optimize flag,
	// the prompt optimization and the web/image search run entirely on the
	// server — the browser only polls the task afterwards, so a refresh can
	// never kill an in-flight optimization or orphan the generation.
	bodyBytes, readErr := io.ReadAll(c.Request.Body)
	if readErr != nil {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{
			"error": gin.H{"message": readErr.Error(), "type": "invalid_request_error"},
		})
		return
	}
	req := &relaycommon.TaskSubmitReq{}
	studioOptimize := false
	optimized := (*StudioOptimizeResult)(nil)
	if len(bodyBytes) > 0 {
		if err := common.Unmarshal(bodyBytes, req); err == nil {
			studioOptimize = metadataStudioFlag(req.Metadata, "studio_optimize")
		} else {
			req = nil
		}
	}

	if req != nil {
		// Rule enforcement runs before anything else — an invalid combination
		// never reaches the relay or burns optimizer tokens.
		if !enforceStudioDiscordGate(c, req.Model) {
			return
		}
		if !enforceStudioVideoRules(c, req) {
			return
		}
	}

	if studioOptimize && req != nil {
		userId := c.GetInt("id")
		group := c.GetString("group")
		if group == "" {
			if userCache, err := model.GetUserCache(userId); err == nil {
				group = userCache.Group
			}
			if group == "" {
				group = "default"
			}
		}
		var optimizeErr error
		optimized, optimizeErr = RunStudioOptimization(c, userId, group, req, "video")
		if optimizeErr != nil {
			switch optimizeErr.Error() {
			case "prompt_optimizer_not_configured":
				c.JSON(http.StatusBadRequest, gin.H{
					"error": gin.H{
						"message": "Prompt optimizer is not configured. Open System Settings → Playground & Studio → Studio Assistant Model and set a vision-capable model.",
						"type":    "invalid_request_error",
					},
				})
				return
			default:
				// Transient optimizer failures never block the generation:
				// fall back to the raw brief and let the relay run.
				optimized = &StudioOptimizeResult{Prompt: req.Prompt, Fallback: true}
			}
		}
		if optimized != nil {
			req.Prompt = optimized.Prompt
			if optimized.NegativePrompt != "" {
				if req.Metadata == nil {
					req.Metadata = map[string]interface{}{}
				}
				req.Metadata["negative_prompt"] = optimized.NegativePrompt
			}
			if optimized.References != nil {
				if req.Metadata == nil {
					req.Metadata = map[string]interface{}{}
				}
				refs := make([]interface{}, 0, len(optimized.References))
				for _, ref := range optimized.References {
					refs = append(refs, ref)
				}
				req.Metadata["reference_images"] = refs
			}
		}
		bodyBytes, _ = common.Marshal(req)
	}

	// Hand the (possibly rewritten) body to the relay.
	c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	c.Request.ContentLength = int64(len(bodyBytes))

	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatTask, nil, nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{"message": err.Error(), "type": "invalid_request_error"},
		})
		return
	}

	// Storage is never a gate for generation. When the user's server-side
	// video storage is full the store path skips saving to the server and the
	// studio keeps the new video in the browser's IndexedDB instead; the
	// quota bar in the Studio shows the warning.
	userId := c.GetInt("id")

	userCache, err := model.GetUserCache(userId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{"message": err.Error(), "type": "server_error"},
		})
		return
	}
	userCache.WriteContext(c)

	targetGroup := relayInfo.UsingGroup
	if targetGroup == "" {
		targetGroup = userCache.Group
	}
	if targetGroup == "" {
		targetGroup = "default"
	}

	tempToken := &model.Token{
		UserId:          userId,
		Name:            fmt.Sprintf("playground-%s", targetGroup),
		Group:           targetGroup,
		CrossGroupRetry: false,
	}
	_ = middleware.SetupContextForToken(c, tempToken)

	RelayTask(c)

	// Persist the server-side facts onto the studio_items row so the client
	// picks them up on the next sync (after a refresh, with no client code).
	if studioOptimize && req != nil {
		itemId := metadataStringValue(req.Metadata, "studio_item_id")
		if itemId != "" {
			taskId := c.GetString("studio_public_task_id")
			settings := map[string]string{
				"model":   req.Model,
				"quality": req.Quality,
				"ratio":   req.Ratio,
			}
			if req.Duration > 0 {
				settings["duration"] = fmt.Sprintf("%d", req.Duration)
			}
			overlay := model.StudioItemOverlay{
				TaskID:          taskId,
				Prompt:          req.Prompt,
				OptimizedPrompt: "",
				Settings:        settings,
			}
			if optimized != nil {
				overlay.OptimizedPrompt = optimized.Prompt
				overlay.NegativePrompt = optimized.NegativePrompt
				overlay.References = optimized.References
			}
			if err := model.OverlayStudioItem(userId, itemId, overlay, time.Now().Unix()); err != nil {
				common.SysError(fmt.Sprintf("studio: overlay for item %s failed: %s", itemId, err.Error()))
			}
		}
	}
}

func playgroundRelay(c *gin.Context, relayFormat types.RelayFormat) {
	var newAPIError *types.NewAPIError

	defer func() {
		if newAPIError != nil {
			c.JSON(newAPIError.StatusCode, gin.H{
				"error": newAPIError.ToOpenAIError(),
			})
		}
	}()

	useAccessToken := c.GetBool("use_access_token")
	if useAccessToken {
		newAPIError = types.NewError(errors.New("暂不支持使用 access token"), types.ErrorCodeAccessDenied, types.ErrOptionWithSkipRetry())
		return
	}

	relayInfo, err := relaycommon.GenRelayInfo(c, relayFormat, nil, nil)
	if err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
		return
	}

	userId := c.GetInt("id")

	// Write user context to ensure acceptUnsetRatio is available
	userCache, err := model.GetUserCache(userId)
	if err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeQueryDataError, types.ErrOptionWithSkipRetry())
		return
	}
	userCache.WriteContext(c)

	targetGroup := relayInfo.UsingGroup
	if targetGroup == "" {
		targetGroup = userCache.Group
	}
	if targetGroup == "" {
		targetGroup = "default"
	}

	tempToken := &model.Token{
		UserId:          userId,
		Name:            fmt.Sprintf("playground-%s", targetGroup),
		Group:           targetGroup,
		CrossGroupRetry: false,
	}
	_ = middleware.SetupContextForToken(c, tempToken)

	Relay(c, relayFormat)
}
