package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/QuantumNous/new-api/model"

	"github.com/QuantumNous/new-api/common"
)

func seedUserWithRole(t *testing.T, id int, role int) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.User{
		Id:       id,
		Username: "video_quota_test_user",
		Quota:    1000,
		Status:   common.UserStatusEnabled,
		Role:     role,
	}).Error)
}

func seedBigVideoAsset(t *testing.T, userId int, taskID, fileName string, size int64) {
	t.Helper()
	now := int64(1_700_000_000)
	require.NoError(t, model.DB.Create(&model.VideoAsset{
		UserId:    userId,
		TaskId:    taskID,
		FileName:  fileName,
		Size:      size,
		Kind:      "video",
		CreatedAt: now,
		ExpiresAt: now + 86400,
		WarnSent:  false,
		Deleted:   false,
	}).Error)
}

func TestIsVideoStorageQuotaExhausted(t *testing.T) {
	truncate(t)
	seedUserWithRole(t, 1, common.RoleCommonUser)

	// No assets stored yet: never over quota.
	require.False(t, IsVideoStorageQuotaExhausted(1))

	// A single 2 GiB asset is over the 1 GiB default quota.
	seedBigVideoAsset(t, 1, "task_big", "big.mp4", 2<<30)
	require.True(t, IsVideoStorageQuotaExhausted(1))
}

func TestAdminsExceedQuotaButStoreAnyway(t *testing.T) {
	truncate(t)
	seedUserWithRole(t, 2, common.RoleAdminUser)
	seedBigVideoAsset(t, 2, "task_admin", "admin.mp4", 2<<30)
	require.False(t, IsVideoStorageQuotaExhausted(2))
}

// The quota check runs before any network I/O: a user at the limit gets the
// sentinel error without the upstream URL ever being fetched.
func TestStoreVideoFromURLSkipsStorageWhenQuotaExhausted(t *testing.T) {
	truncate(t)
	seedUserWithRole(t, 3, common.RoleCommonUser)
	seedBigVideoAsset(t, 3, "task_fill", "fill.mp4", 2<<30)

	stored, err := StoreVideoFromURL(context.Background(), 3, "task_new", "https://example.com/video.mp4", "video")
	require.ErrorIs(t, err, ErrVideoStorageQuotaExceeded)
	require.Equal(t, "", stored)
}

// Under quota, an invalid (SSRF-blocked) URL produces a fetch error rather
// than the quota sentinel — proving the quota gate lets valid requests through.
func TestStoreVideoFromURLUnderQuotaAttemptsStore(t *testing.T) {
	truncate(t)
	seedUserWithRole(t, 4, common.RoleCommonUser)

	stored, err := StoreVideoFromURL(context.Background(), 4, "task_small", "not-a-url", "video")
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrVideoStorageQuotaExceeded)
	require.Equal(t, "", stored)
}
