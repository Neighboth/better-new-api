package model

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpsertStudioItemRoundTrip(t *testing.T) {
	truncateTables(t)

	require.NoError(t, UpsertStudioItem(42, "item-a", `{"id":"item-a","prompt":"first","urls":["/uploads/videos/42/f.mp4"]}`, 1000))
	items, err := ListStudioItems(42, 10)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "item-a", items[0].ItemId)
	require.Equal(t, int64(1000), items[0].CreatedAt)
	require.True(t, strings.Contains(items[0].Payload, "first"))

	// The second upsert refreshes payload and UpdatedAt (GORM owns the update
	// timestamp) but keeps the original CreatedAt.
	require.NoError(t, UpsertStudioItem(42, "item-a", `{"id":"item-a","prompt":"second","urls":["/uploads/videos/42/g.mp4"]}`, 2000))
	items, err = ListStudioItems(42, 10)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, int64(1000), items[0].CreatedAt)
	require.Greater(t, items[0].UpdatedAt, int64(1000))
	require.True(t, strings.Contains(items[0].Payload, "second"))

	// Other users see nothing, and cannot delete my row either.
	items, err = ListStudioItems(43, 10)
	require.NoError(t, err)
	require.Empty(t, items)
	require.NoError(t, DeleteStudioItem(43, "item-a"))
	items, err = ListStudioItems(42, 10)
	require.NoError(t, err)
	require.Len(t, items, 1)

	require.NoError(t, DeleteStudioItem(42, "item-a"))
	items, err = ListStudioItems(42, 10)
	require.NoError(t, err)
	require.Empty(t, items)
}

func TestListStudioItemsNewestFirst(t *testing.T) {
	truncateTables(t)

	require.NoError(t, UpsertStudioItem(7, "old", `{"id":"old"}`, 100))
	require.NoError(t, UpsertStudioItem(7, "mid", `{"id":"mid"}`, 200))
	require.NoError(t, UpsertStudioItem(7, "new", `{"id":"new"}`, 300))

	items, err := ListStudioItems(7, 10)
	require.NoError(t, err)
	require.Len(t, items, 3)
	require.Equal(t, []string{"new", "mid", "old"}, []string{items[0].ItemId, items[1].ItemId, items[2].ItemId})
}
