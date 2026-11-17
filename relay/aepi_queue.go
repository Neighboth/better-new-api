// Aepi capacity queue.
//
// The upstream Aepi backend accepts only ten concurrent jobs per account.
// Submissions beyond that limit are answered with errors that used to surface
// as mysterious upstream failures in the studio; this file parks them in the
// task table as QUEUED rows and admits them FIFO as soon as a slot frees, so
// the "10 concurrent" ceiling is enforced without failing users.
//
// The same queue absorbs a transient upstream storm: when Aepi answers a
// submit with 429 (or 5xx / no response at all) after every inline retry is
// exhausted, the submission is parked as QUEUED and retried by the dispatcher
// for up to fifteen minutes. If the storm outlasts the window the task is marked
// FAILURE with a clear reason instead of an opaque upstream error — the user
// is never surprised by a task that disappears, and billing is never leaked
// (the pre-consumption is refunded, and the admission replay bills fresh).
//
// Multi-node correctness: the dispatcher claim is a CAS on the row, and the
// in-flight count is read from the shared task table, so several nodes do not
// admit the same task twice. A node that over-admits by a race window only
// wastes one admission attempt, which the upstream then 429s back into the
// queue — the five-minute window absorbs it.
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
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relaydto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
)

// AepiConcurrentSlots is the upstream ceiling for concurrent jobs.
const AepiConcurrentSlots = 10

// AepiUpstreamRetryWindowSeconds bounds how long a submission may bounce
// between the queue and the upstream before it is abandoned.
const AepiUpstreamRetryWindowSeconds = int64(900) // 15 minutes

// aepiStaleClaimSeconds is how long an Aepi row may stay IN_PROGRESS before
// the sweeper reaps it as an orphaned admission claim.
const aepiStaleClaimSeconds = int64(7200) // 2 hours

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

// IsRetryableUpstreamFailureStatus reports whether an upstream answer is a
// transient capacity/health storm rather than a client error: 429, 5xx, and
// 0 (no answer at the transport layer).
func IsRetryableUpstreamFailureStatus(status int) bool {
	if status == 0 {
		return true
	}
	return status == http.StatusTooManyRequests || (status >= 500 && status < 600)
}

// upstreamRetryDelay grows the wait between admission attempts: attempt 1 -> 5s,
// then +5s per attempt, capped at 20s.
func upstreamRetryDelay(attempt int) int64 {
	d := int64(5 + (attempt-1)*5)
	if d > 20 {
		d = 20
	}
	return d
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

func queuedRowBody(c *gin.Context) ([]byte, string, bool) {
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return nil, "", false
	}
	bodyData, err := storage.Bytes()
	if err != nil {
		return nil, "", false
	}
	contentType := strings.TrimSpace(c.GetHeader("Content-Type"))
	if contentType == "" {
		return nil, "", false
	}
	return bodyData, contentType, true
}

func buildQueuedTask(publicTaskID string, relayInfo *relaycommon.RelayInfo, bodyData []byte, contentType string, now int64, attempt int, nextAttemptAt int64) *model.Task {
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
	queued.PrivateData.EnqueuedAt = now
	queued.PrivateData.AttemptCount = attempt
	queued.PrivateData.NextAttemptAt = nextAttemptAt
	return queued
}

func queuedVideoResponse(publicTaskID string, now int64, modelName string) *relaydto.OpenAIVideo {
	ov := relaydto.NewOpenAIVideo()
	ov.ID = publicTaskID
	ov.TaskID = publicTaskID
	ov.Status = relaydto.VideoStatusQueued
	ov.CreatedAt = now
	ov.Model = modelName
	return ov
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
	bodyData, contentType, ok := queuedRowBody(c)
	if !ok {
		return nil, nil
	}

	publicTaskID := model.GenerateTaskID()
	now := time.Now().Unix()
	queued := buildQueuedTask(publicTaskID, relayInfo, bodyData, contentType, now, 0, 0)

	if err := queued.Insert(); err != nil {
		common.SysError(fmt.Sprintf("aepi queue: failed to park task: %s", err.Error()))
		return nil, service.TaskErrorWrapperLocal(err, "aepi_queue_insert_failed", http.StatusInternalServerError)
	}

	ov := queuedVideoResponse(publicTaskID, now, relayInfo.OriginModelName)

	c.Header("X-New-Api-Task-Id", publicTaskID)
	if existing := c.GetHeader("Access-Control-Expose-Headers"); existing == "" {
		c.Header("Access-Control-Expose-Headers", "X-New-Api-Task-Id")
	}
	return ov, nil
}

// TryParkAepiUpstreamFailure absorbs a retryable upstream storm (429/5xx/transport
// error) into the queue after every inline retry is exhausted. It returns the
// parked video object (respond HTTP 202 with it) or nil when the failure is
// not retryable/queueable, in which case the caller proceeds to the normal
// error path.
//
// Fresh inline failure: a new QUEUED row is created and the caller's pre-consume
// hold is refunded (the admission replay bills fresh). Admission replay
// (aepi_queued_task_force_public is set): the claimed placeholder is re-queued
// with an attempts counter and a backoff; when the five-minute window from the
// original park is exhausted the placeholder is marked FAILURE instead.
func TryParkAepiUpstreamFailure(c *gin.Context, relayInfo *relaycommon.RelayInfo, taskErr *dto.TaskError) *relaydto.OpenAIVideo {
	if taskErr == nil || taskErr.LocalError {
		return nil
	}
	if !IsAepiConcurrentLimitedModel(relayInfo.OriginModelName) {
		return nil
	}
	if !IsRetryableUpstreamFailureStatus(taskErr.UpstreamStatusCode) {
		return nil
	}
	if relayInfo.Billing != nil {
		// Refund is idempotent; the deferred refund in RelayTask sees it as done.
		relayInfo.Billing.Refund(c)
	}
	now := time.Now().Unix()

	if forced := c.GetString("aepi_queued_task_force_public"); forced != "" {
		var row model.Task
		if err := model.DB.Where("task_id = ?", forced).First(&row).Error; err != nil {
			common.SysError(fmt.Sprintf("aepi queue: admission replay cannot find task %s to re-queue", forced))
			return nil
		}
		enqueuedAt := row.PrivateData.EnqueuedAt
		if enqueuedAt == 0 {
			enqueuedAt = row.CreatedAt
		}
		attempts := row.PrivateData.AttemptCount + 1
		if now >= enqueuedAt+AepiUpstreamRetryWindowSeconds {
			reason := "Aepi kept returning 429 (upstream capacity) for 15 minutes; the task was cancelled"
			_ = model.DB.Model(&model.Task{}).
				Where("id = ?", row.ID).
				Updates(map[string]interface{}{
					"status":      model.TaskStatusFailure,
					"progress":    taskcommon.ProgressComplete,
					"fail_reason": reason,
					"finish_time": now,
				}).Error
			common.SysError(fmt.Sprintf("aepi queue: task %s abandoned after %d attempts and %d seconds of upstream 429s", forced, attempts, now-enqueuedAt))
			return nil
		}
		privateData := row.PrivateData
		privateData.AttemptCount = attempts
		privateData.NextAttemptAt = now + upstreamRetryDelay(attempts)
		res := model.DB.Model(&model.Task{}).
			Where("id = ? AND status = ?", row.ID, model.TaskStatusInProgress).
			Updates(map[string]interface{}{
				"status":       model.TaskStatusQueued,
				"progress":     taskcommon.ProgressQueued,
				"private_data": privateData,
			})
		if res.RowsAffected != 1 {
			// Someone else moved the row (multi-node); do not touch it. The
			// losing node simply stops here — the shared DB decides.
			common.SysError(fmt.Sprintf("aepi queue: failed to re-queue %s (stale claim)", forced))
			return nil
		}
		common.SysError(fmt.Sprintf("aepi queue: task %s re-queued (attempt %d) after retryable upstream %d", forced, attempts, taskErr.UpstreamStatusCode))
		c.Header("X-New-Api-Task-Id", forced)
		return queuedVideoResponse(forced, now, relayInfo.OriginModelName)
	}

	bodyData, contentType, ok := queuedRowBody(c)
	if !ok {
		// The body is not replayable; nothing we can park. Let the error fly.
		return nil
	}
	publicTaskID := model.GenerateTaskID()
	queued := buildQueuedTask(publicTaskID, relayInfo, bodyData, contentType, now, 1, 0)
	if err := queued.Insert(); err != nil {
		common.SysError(fmt.Sprintf("aepi queue: failed to park retryable failure: %s", err.Error()))
		return nil
	}
	common.SysError(fmt.Sprintf("aepi queue: parked retryable upstream %d for model %s as %s", taskErr.UpstreamStatusCode, relayInfo.OriginModelName, publicTaskID))

	c.Header("X-New-Api-Task-Id", publicTaskID)
	if existing := c.GetHeader("Access-Control-Expose-Headers"); existing == "" {
		c.Header("Access-Control-Expose-Headers", "X-New-Api-Task-Id")
	}
	return queuedVideoResponse(publicTaskID, now, relayInfo.OriginModelName)
}

// failExpiredQueuedAepiTasks marks QUEUED rows whose retry window has closed as
// FAILURE, and Aepi rows stuck IN_PROGRESS for two hours (a node died mid-admission
// — or the dispatcher crashed before recording the outcome) as FAILURE too, so a
// leaked claim can never consume one of the ten upstream slots forever.
func failExpiredQueuedAepiTasks(now int64) int {
	if now <= 0 {
		return 0
	}
	var rows []model.Task
	if err := model.DB.Model(&model.Task{}).
		Where("status = ? AND platform = ?", model.TaskStatusQueued, strconv.Itoa(constant.ChannelTypeAepi)).
		Order("id ASC").
		Limit(25).
		Find(&rows).Error; err != nil {
		return 0
	}
	failed := 0
	for i := range rows {
		row := &rows[i]
		enqueuedAt := row.PrivateData.EnqueuedAt
		if enqueuedAt == 0 {
			enqueuedAt = row.CreatedAt
		}
		if enqueuedAt == 0 || now < enqueuedAt+AepiUpstreamRetryWindowSeconds {
			continue
		}
		reason := "Aepi kept returning 429 (upstream capacity) for 15 minutes; the task was cancelled"
		res := model.DB.Model(&model.Task{}).
			Where("id = ? AND status = ?", row.ID, model.TaskStatusQueued).
			Updates(map[string]interface{}{
				"status":      model.TaskStatusFailure,
				"progress":    taskcommon.ProgressComplete,
				"fail_reason": reason,
				"finish_time": now,
			})
		if res.RowsAffected == 1 {
			common.SysError(fmt.Sprintf("aepi queue: task %s abandoned after 15 minute retry window (upstream 429)", row.TaskID))
			failed++
		}
	}

	var stale []model.Task
	if err := model.DB.Model(&model.Task{}).
		Where("status = ? AND platform = ?", model.TaskStatusInProgress, strconv.Itoa(constant.ChannelTypeAepi)).
		Where("submit_time > 0 AND submit_time < ?", now-aepiStaleClaimSeconds).
		Order("id ASC").
		Limit(10).
		Find(&stale).Error; err != nil {
		return failed
	}
	for i := range stale {
		row := &stale[i]
		// A row with an upstream task id is a real job the poller still owns.
		// Only a placeholder that never reached the upstream after the claim
		// (the admitting node died) is orphaned — fail it so the slot frees.
		if row.PrivateData.UpstreamTaskID != "" {
			continue
		}
		res := model.DB.Model(&model.Task{}).
			Where("id = ? AND status = ?", row.ID, model.TaskStatusInProgress).
			Updates(map[string]interface{}{
				"status":      model.TaskStatusFailure,
				"progress":    taskcommon.ProgressComplete,
				"fail_reason": "Aepi admission did not complete (orphaned claim); the task was marked failed",
				"finish_time": now,
			})
		if res.RowsAffected == 1 {
			common.SysError(fmt.Sprintf("aepi queue: orphaned IN_PROGRESS claim %s reaped", row.TaskID))
			failed++
		}
	}
	return failed
}

// FindClaimableQueuedAepiTask returns the oldest QUEUED Aepi task whose backoff
// has elapsed, or nil. Expired rows are failed as a side effect.
func FindClaimableQueuedAepiTask() *model.Task {
	now := time.Now().Unix()
	failExpiredQueuedAepiTasks(now)

	var rows []model.Task
	if err := model.DB.Model(&model.Task{}).
		Where("status = ? AND platform = ?", model.TaskStatusQueued, strconv.Itoa(constant.ChannelTypeAepi)).
		Order("id ASC").
		Limit(25).
		Find(&rows).Error; err != nil {
		return nil
	}
	for i := range rows {
		row := &rows[i]
		if row.PrivateData.NextAttemptAt > now {
			continue
		}
		return row
	}
	return nil
}
