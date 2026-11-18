package controller

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func GetAllTask(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)

	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	// 解析其他查询参数
	queryParams := model.SyncTaskQueryParams{
		Platform:       constant.TaskPlatform(c.Query("platform")),
		TaskID:         c.Query("task_id"),
		Status:         c.Query("status"),
		Action:         c.Query("action"),
		StartTimestamp: startTimestamp,
		EndTimestamp:   endTimestamp,
		ChannelID:      c.Query("channel_id"),
	}

	items := model.TaskGetAllTasks(pageInfo.GetStartIdx(), pageInfo.GetPageSize(), queryParams)
	total := model.TaskCountAllTasks(queryParams)
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(tasksToDto(items, true))
	common.ApiSuccess(c, pageInfo)
}

// CancelTask cancels a queued (never admitted upstream) task owned by the
// user. Only QUEUED tasks can be cancelled: a job already admitted upstream
// cannot be stopped there and its result will still be stored. Cancelled
// placeholders are marked FAILURE so they never reach the dispatcher nor the
// async poller, and any quota that was held is refunded.
func CancelTask(c *gin.Context) {
	userId := c.GetInt("id")
	taskId := strings.TrimSpace(c.Param("task_id"))
	if taskId == "" {
		common.ApiErrorMsg(c, "missing task id")
		return
	}
	task, exist, err := model.GetByTaskId(userId, taskId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !exist {
		common.ApiErrorMsg(c, "task not found")
		return
	}
	if task.Status != model.TaskStatusQueued {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"message": "This generation already started upstream and cannot be cancelled. The result will appear in your history.",
			"code":    "task_already_started",
		})
		return
	}
	now := time.Now().Unix()
	oldStatus := task.Status
	task.Status = model.TaskStatusFailure
	task.Progress = "100%"
	task.FinishTime = now
	task.FailReason = "Cancelled by the user before the job reached the provider"
	won, err := task.UpdateWithStatus(oldStatus)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !won {
		common.ApiErrorMsg(c, "the task already transitioned; refresh the page")
		return
	}
	if task.Quota != 0 {
		_ = service.RefundTaskQuota(context.Background(), task, "cancelled by the user")
	}
	common.ApiSuccess(c, gin.H{
		"task_id": taskId,
		"status":  model.TaskStatusFailure,
	})
}

func GetUserTask(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)

	userId := c.GetInt("id")

	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)

	queryParams := model.SyncTaskQueryParams{
		Platform:       constant.TaskPlatform(c.Query("platform")),
		TaskID:         c.Query("task_id"),
		Status:         c.Query("status"),
		Action:         c.Query("action"),
		StartTimestamp: startTimestamp,
		EndTimestamp:   endTimestamp,
	}

	items := model.TaskGetAllUserTask(userId, pageInfo.GetStartIdx(), pageInfo.GetPageSize(), queryParams)
	total := model.TaskCountAllUserTask(userId, queryParams)
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(tasksToDto(items, false))
	common.ApiSuccess(c, pageInfo)
}

func tasksToDto(tasks []*model.Task, fillUser bool) []*dto.TaskDto {
	var userIdMap map[int]*model.UserBase
	if fillUser {
		userIdMap = make(map[int]*model.UserBase)
		userIds := types.NewSet[int]()
		for _, task := range tasks {
			userIds.Add(task.UserId)
		}
		for _, userId := range userIds.Items() {
			cacheUser, err := model.GetUserCache(userId)
			if err == nil {
				userIdMap[userId] = cacheUser
			}
		}
	}
	result := make([]*dto.TaskDto, len(tasks))
	for i, task := range tasks {
		if fillUser {
			if user, ok := userIdMap[task.UserId]; ok {
				task.Username = user.Username
			}
		}
		result[i] = relay.TaskModel2Dto(task)
		if task.Status == model.TaskStatusQueued {
			// FIFO position in the Aepi capacity queue (1-based).
			result[i].QueuePosition = int(relay.AepiQueuePosition(task))
		}
	}
	return result
}
