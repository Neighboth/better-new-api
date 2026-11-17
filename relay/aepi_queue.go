// Aepi capacity queue.
//
// The upstream Aepi backend accepts only ten concurrent jobs per account.
// Submissions beyond that limit are answered with errors that used to surface
// as mysterious upstream failures in the studio; this file parks them in the
// task table as QUEUED rows and admits them FIFO as soon as a slot frees, so
// the "10 concurrent" ceiling is enforced without failing users.
package relay

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	taskaepi "github.com/QuantumNous/new-api/relay/channel/task/aepi"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaydto "github.com/QuantumNous/new-api/relaykit/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
)

// AepiConcurrentSlots is the upstream ceiling for concurrent jobs.
const AepiConcurrentSlots = 10

// IsAepiConcurrentLimitedModel reports whether a model belongs to the Aepi
// catalog and therefore shares the ten concurrent upstream slots.
func IsAepiConcurrentLimitedModel(modelName string) bool {
	name := strings.ToLower(strings.TrimSpace(modelName))
	if name == "" {
		return false
	}
	for _, candidate := range taskaepi.AllModels {
		if strings.EqualFold(candidate, name) {
			return true
		}
	}
	return false
}

// AepiInflightCount counts the Aepi tasks holding an upstream slot right now.
func AepiInflightCount() int64 {
	var count int64
	model.DB.Model(&model.Task{}).
		Where("platform = ?", strconv.Itoa(constant.ChannelTypeAepi)).
		Where("status NOT IN ?", []model.TaskStatus{
			model.TaskStatusSuccess,
			model.TaskStatusFailure,
			model.TaskStatusQueued,
			model.TaskStatusUnknown,
		}).Count(&count)
	return count
}

// AepiQueuePosition returns the 1-based FIFO position of a queued task or 0.
func AepiQueuePosition(task *model.Task) int64 {
	if task == nil || task.Status != model.TaskStatusQueued {
		return 0
	}
	var count int64
	model.DB.Model(&model.Task{}).
		Where("status = ? AND id <= ?", model.TaskStatusQueued, task.ID).
		Count(&count)
	return count
}

// EnqueueAepiRelay parks the submission as a QUEUED task when the ten
// concurrent upstream slots are exhausted. It returns (video, nil) when the
// submission was enqueued — the caller should respond with HTTP 202 and
// return. It returns (nil, nil) when a slot is free, the model is not
// Aepi-limited, or the body cannot be replayed — the caller proceeds inline as
// usual. On failure it returns a task error the caller should surface.
func EnqueueAepiRelay(c *gin.Context, relayInfo *relaycommon.RelayInfo) (*relaydto.OpenAIVideo, *dto.TaskError) {
	if !IsAepiConcurrentLimitedModel(relayInfo.OriginModelName) {
		return nil, nil
	}
	if AepiInflightCount() < AepiConcurrentSlots {
		return nil, nil
	}
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		// Unreadable body — fall through to the normal path rather than dropping the request.
		return nil, nil
	}
	bodyData, err := storage.Bytes()
	if err != nil {
		return nil, nil
	}
	contentType := strings.TrimSpace(c.GetHeader("Content-Type"))
	if contentType == "" {
		// No body to replay — keep the legacy inline behavior instead of a broken queued row.
		return nil, nil
	}

	publicTaskID := model.GenerateTaskID()
	now := time.Now().Unix()
	queued := &model.Task{
		TaskID:     publicTaskID,
		Platform:   constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeAepi)),
		Action:     constant.TaskActionGenerate,
		UserId:     relayInfo.UserId,
		Group:      relayInfo.TokenGroup,
		Status:     model.TaskStatusQueued,
		Progress:   taskcommon.ProgressQueued,
		Quota:      0,
		CreatedAt:  now,
		SubmitTime: now,
	}
	if queued.Group == "" {
		queued.Group = relayInfo.UserGroup
	}
	queued.Properties.OriginModelName = relayInfo.OriginModelName
	queued.Properties.UpstreamModelName = relayInfo.OriginModelName
	queued.PrivateData.EnqueuedRequest = json.RawMessage(bodyData)
	queued.PrivateData.EnqueuedContentType = contentType

	if err := queued.Insert(); err != nil {
		common.SysError(fmt.Sprintf("aepi queue: failed to park task: %s", err.Error()))
		return nil, service.TaskErrorWrapperLocal(err, "aepi_queue_insert_failed", http.StatusInternalServerError)
	}

	ov := relaydto.NewOpenAIVideo()
	ov.ID = publicTaskID
	ov.TaskID = publicTaskID
	ov.Status = relaydto.VideoStatusQueued
	ov.CreatedAt = now
	ov.Model = relayInfo.OriginModelName

	c.Header("X-New-Api-Task-Id", publicTaskID)
	if existing := c.GetHeader("Access-Control-Expose-Headers"); existing == "" {
		c.Header("Access-Control-Expose-Headers", "X-New-Api-Task-Id")
	}
	return ov, nil
}
