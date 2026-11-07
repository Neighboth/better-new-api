package service

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestHeldTaskRefundIsAtomicAndRecoversAfterRestart(t *testing.T) {
	truncate(t)
	const userID, tokenID, channelID = 996, 996, 996
	seedUser(t, userID, 70)
	seedToken(t, tokenID, userID, "held-task-token", 70)
	seedChannel(t, channelID)
	seedChargedAccounting(t, userID, channelID, tokenID, 30, 1)
	hold, err := model.BeginWalletTransferHold(userID)
	require.NoError(t, err)
	task := makeTask(userID, channelID, 30, tokenID, BillingSourceWallet, 0)
	task.PrivateData.WalletHoldID = hold
	task.Status = model.TaskStatusInProgress
	require.NoError(t, model.DB.Create(task).Error)
	task.Status = model.TaskStatusFailure
	markHeldTaskRefundPending(task)
	require.True(t, task.RefundPending)
	won, err := task.UpdateWithStatus(model.TaskStatusInProgress)
	require.NoError(t, err)
	require.True(t, won)
	var stale model.Task
	require.NoError(t, model.DB.First(&stale, task.ID).Error)
	require.Error(t, model.ResolveWalletTransferHold(userID, hold, 0))

	// A token error must roll back wallet, marker, counters and journal.
	require.NoError(t, model.DB.Exec(`CREATE TRIGGER fail_held_task_token BEFORE UPDATE ON tokens BEGIN SELECT RAISE(ABORT, 'injected'); END;`).Error)
	t.Cleanup(func() { model.DB.Exec("DROP TRIGGER IF EXISTS fail_held_task_token") })
	require.False(t, RefundTaskQuota(context.Background(), task, "provider failure"))
	require.Equal(t, 70, getUserQuota(t, userID))
	require.Equal(t, 70, getTokenRemainQuota(t, tokenID))
	var persisted model.Task
	require.NoError(t, model.DB.First(&persisted, task.ID).Error)
	require.True(t, persisted.RefundPending)
	require.Equal(t, 30, persisted.Quota)
	var events int64
	require.NoError(t, model.DB.Model(&model.WalletHoldEvent{}).Where("hold_id = ? AND event_key = ?", hold, "task-atomic-refund").Count(&events).Error)
	require.Zero(t, events)
	require.NoError(t, model.DB.Exec("DROP TRIGGER fail_held_task_token").Error)

	// Simulate the next poll after the process that recorded failure died.
	recoverPendingHeldTaskRefunds(context.Background())
	require.Equal(t, 100, getUserQuota(t, userID))
	require.Equal(t, 100, getTokenRemainQuota(t, tokenID))
	require.Zero(t, getTokenUsedQuota(t, tokenID))
	used, requests := getUserUsageAccounting(t, userID)
	require.Zero(t, used)
	require.Equal(t, 1, requests)
	require.Zero(t, getChannelUsedQuota(t, channelID))
	require.NoError(t, model.DB.First(&persisted, task.ID).Error)
	require.False(t, persisted.RefundPending)
	require.Zero(t, persisted.Quota)
	var finalHold model.WalletTransferHold
	require.NoError(t, model.DB.First(&finalHold, "id = ?", hold).Error)
	require.Equal(t, "resolved", finalHold.State)

	// Neither a stale status update nor a replay may recreate the charge.
	stale.Progress = "100%"
	_, err = stale.UpdateWithStatus(model.TaskStatusFailure)
	require.NoError(t, err)
	require.NoError(t, model.DB.First(&persisted, task.ID).Error)
	require.False(t, persisted.RefundPending)
	require.Zero(t, persisted.Quota)
	amount, err := model.RefundHeldTaskWalletOnce(userID, task.ID, hold)
	require.NoError(t, err)
	require.Zero(t, amount)
	require.Equal(t, 100, getUserQuota(t, userID))
}

func TestHeldTaskRefundDoesNotReleaseSharedHold(t *testing.T) {
	truncate(t)
	const userID, tokenID, channelID = 998, 998, 998
	seedUser(t, userID, 70)
	seedToken(t, tokenID, userID, "held-shared-token", 70)
	seedChannel(t, channelID)
	seedChargedAccounting(t, userID, channelID, tokenID, 30, 1)
	hold, err := model.BeginWalletTransferHold(userID)
	require.NoError(t, err)
	task := makeTask(userID, channelID, 30, tokenID, BillingSourceWallet, 0)
	task.Status = model.TaskStatusFailure
	task.RefundPending = true
	task.PrivateData.WalletHoldID = hold
	require.NoError(t, model.DB.Create(task).Error)
	other := makeTask(userID, channelID, 10, tokenID, BillingSourceWallet, 0)
	other.Status = model.TaskStatusInProgress
	other.PrivateData.WalletHoldID = hold
	require.NoError(t, model.DB.Create(other).Error)
	require.False(t, RefundTaskQuota(context.Background(), task, "shared hold"))
	require.Equal(t, 70, getUserQuota(t, userID))
	var current model.WalletTransferHold
	require.NoError(t, model.DB.First(&current, "id = ?", hold).Error)
	require.Equal(t, "open", current.State)
}

func TestHeldTaskRefundRequiresIntentAndRejectsLegacyPartialRefund(t *testing.T) {
	truncate(t)
	const userID, tokenID, channelID = 997, 997, 997
	seedUser(t, userID, 70)
	seedToken(t, tokenID, userID, "held-task-legacy-token", 70)
	seedChannel(t, channelID)
	seedChargedAccounting(t, userID, channelID, tokenID, 30, 1)
	hold, err := model.BeginWalletTransferHold(userID)
	require.NoError(t, err)
	task := makeTask(userID, channelID, 30, tokenID, BillingSourceWallet, 0)
	task.Status = model.TaskStatusFailure
	task.PrivateData.WalletHoldID = hold
	require.NoError(t, model.DB.Create(task).Error)
	require.False(t, RefundTaskQuota(context.Background(), task, "without intent"))
	require.Equal(t, 70, getUserQuota(t, userID))
	require.NoError(t, model.DB.Model(&model.Task{}).Where("id = ?", task.ID).Update("refund_pending", true).Error)
	task.RefundPending = true
	require.NoError(t, model.ApplyWalletHoldDelta(userID, hold, "task-refund", 30))
	require.False(t, RefundTaskQuota(context.Background(), task, "old partial refund"))
	require.Equal(t, 100, getUserQuota(t, userID))
	require.Equal(t, 70, getTokenRemainQuota(t, tokenID))
	require.Error(t, model.ResolveWalletTransferHold(userID, hold, 0))
}
