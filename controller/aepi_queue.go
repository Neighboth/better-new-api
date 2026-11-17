// Aepi capacity queue admitter.
//
// Lives in the controller package because admission replays the full
// RelayTask pipeline (selection, billing, insertion, settlement) against a
// test context, and RelayTask is controller code.
package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
)

// StartAepiTaskQueueDispatcher admits queued Aepi submissions FIFO as soon as
// a slot frees. Runs until process exit; the claim is a CAS on the row, so a
// crashed or concurrent admitter cannot double-submit a task.
func StartAepiTaskQueueDispatcher() {
	go func() {
		for {
			time.Sleep(2 * time.Second)
			AdmitQueuedAepiTaskOnce()
		}
	}()
}

var aepiAdmitLock = make(chan struct{}, 1)

// AdmitQueuedAepiTaskOnce claims the oldest queued Aepi task and submits it.
// The placeholder row is removed in favour of a freshly inserted row carrying
// the same public task id the user is already polling, re-queued when the
// upstream keeps refusing with a retryable storm, or — when admission fails
// for good — marked as FAILURE so the user sees an end state instead of an error.
func AdmitQueuedAepiTaskOnce() bool {
	select {
	case aepiAdmitLock <- struct{}{}:
		defer func() { <-aepiAdmitLock }()
	default:
		return false
	}
	if relay.AepiInflightCount() >= relay.AepiConcurrentSlots {
		return false
	}
	task := relay.FindClaimableQueuedAepiTask()
	if task == nil {
		return false
	}
	if !claimQueuedAepiTask(task.ID) {
		return false
	}
	if len(task.PrivateData.EnqueuedRequest) == 0 {
		failQueuedAepiTask(task.ID, "queued task payload is missing")
		return true
	}
	defer func() {
		if r := recover(); r != nil {
			logger.LogError(nil, fmt.Sprintf("aepi admission panicked for task %d: %v", task.ID, r))
			failQueuedAepiTask(task.ID, fmt.Sprintf("admission panic: %v", r))
		}
	}()

	w := httptest.NewRecorder()
	testCtx, _ := gin.CreateTestContext(w)
	req, err := http.NewRequest(http.MethodPost, "/pg/videos/generations", bytes.NewReader(task.PrivateData.EnqueuedRequest))
	if err != nil {
		failQueuedAepiTask(task.ID, err.Error())
		return true
	}
	req.Header.Set("Content-Type", task.PrivateData.EnqueuedContentType)
	testCtx.Request = req
	testCtx.Set("id", task.UserId)
	testCtx.Set(string(constant.ContextKeyUserId), task.UserId)
	testCtx.Set(string(constant.ContextKeyUserQuota), 999999999) // the real gate is NewBillingSession → wallet; this feeds RelayInfo only
	testCtx.Set("token_name", fmt.Sprintf("playground-%s", task.Group))
	testCtx.Set("token_id", 0)
	testCtx.Set("token_key", "")
	testCtx.Set("token_unlimited", false)
	testCtx.Set(string(constant.ContextKeyTokenGroup), task.Group)
	testCtx.Set(string(constant.ContextKeyUserGroup), task.Group)
	testCtx.Set(string(constant.ContextKeyUsingGroup), task.Group)
	testCtx.Set(string(constant.ContextKeyUserEmail), "")
	testCtx.Set(string(constant.ContextKeyOriginalModel), task.Properties.OriginModelName)
	testCtx.Set("channel_type", constant.ChannelTypeAepi)
	testCtx.Set("relay_mode", 0)
	testCtx.Set("platform", strconv.Itoa(constant.ChannelTypeAepi))
	testCtx.Set("aepi_queued_task_force_public", task.TaskID)

	RelayTask(testCtx)

	// The replay may have finished a few ways. Re-read the placeholder:
	//   QUEUED  — the park logic put it back to wait for the next attempt
	//   FAILURE — the park logic saw the 5-minute window close and failed it
	// Either way the row is in its final state and there is nothing more to do.
	var placeholder model.Task
	if err := model.DB.Select("id, status").Where("id = ?", task.ID).First(&placeholder).Error; err == nil {
		if placeholder.Status == model.TaskStatusQueued || placeholder.Status == model.TaskStatusFailure {
			return true
		}
	}

	// RelayTask inserted a fresh row carrying the forced public id. If any row
	// with the public id exists (inserted or duplicated failure), drop the
	// placeholder — the insertion replaced it. Only when nothing was inserted at
	// all do we mark the placeholder FAILURE so the user sees an end state.
	var anyRows int64
	model.DB.Model(&model.Task{}).
		Where("task_id = ?", task.TaskID).
		Where("id <> ?", task.ID).
		Count(&anyRows)
	if anyRows > 0 {
		_ = model.DB.Where("id = ?", task.ID).Delete(&model.Task{}).Error
		return true
	}
	failQueuedAepiTask(task.ID, "admission failed: see the relay logs for detail")
	return true
}

func claimQueuedAepiTask(id int64) bool {
	res := model.DB.Model(&model.Task{}).
		Where("id = ? AND status = ?", id, model.TaskStatusQueued).
		Updates(map[string]interface{}{
			"status":   model.TaskStatusInProgress,
			"progress": taskcommon.ProgressQueued,
		})
	return res.RowsAffected == 1
}

func failQueuedAepiTask(id int64, reason string) {
	_ = model.DB.Model(&model.Task{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":      model.TaskStatusFailure,
			"progress":    taskcommon.ProgressComplete,
			"fail_reason": reason,
			"finish_time": time.Now().Unix(),
		}).Error
}
