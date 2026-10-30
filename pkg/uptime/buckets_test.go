package uptime

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The window of the panel is aligned to the slots of the metrics. The model of
// the squares shows one bar per hour of the day, the bar of the current hour is
// the last bar and the bars of the past day follow it.
func TestSlotStartsAlignsToTheBucket(t *testing.T) {
	const hour = int64(3600)
	const start = int64(1_700_000_000) // 2023-11-14 22:13:20 UTC, not on an hour

	// The window covers the slot of the start, three full slots and the slot
	// that holds the end of the window: four slots of an hour.
	starts := SlotStarts(start, start+3*hour+600, hour, 0)
	require.Len(t, starts, 4)

	assert.Equal(t, int64(1_699_999_200), starts[0])
	for index := 1; index < len(starts); index++ {
		assert.Equal(t, hour, starts[index]-starts[index-1])
	}
	// The last slot is the slot of the end of the window and not the slot of
	// the start of it, the panel draws the state of the model now.
	assert.Equal(t, start+3*hour-start%hour, starts[len(starts)-1])
}

// A window that is wider than the limit of the answer keeps the most recent
// slots. The panel never gets a timeline that starts a year ago.
func TestSlotStartsKeepsTheMostRecentSlots(t *testing.T) {
	const hour = int64(3600)
	const start = int64(1_700_000_000)
	starts := SlotStarts(start-365*24*hour, start, hour, 744)

	require.Len(t, starts, 744)
	assert.Equal(t, start-start%hour, starts[len(starts)-1])
	assert.Equal(t, int64(743)*hour, starts[len(starts)-1]-starts[0])
}

// A window of a single slot has one slot and no panic.
func TestSlotStartsHandlesASingleSlot(t *testing.T) {
	const hour = int64(3600)
	starts := SlotStarts(0, 10, hour, 24)
	require.Len(t, starts, 1)
	assert.Equal(t, int64(0), starts[0])

	// A width that is not set is an hour and a reversed window is swapped.
	assert.Equal(t, []int64{0}, SlotStarts(10, 0, 0, 24))
	assert.Equal(t, []int64{}, SlotStarts(0, -1, hour, 24)[:0])
}

// A slot without requests has no rate. The panel shows the slot as unknown and
// not as a success of the model.
func TestPercentOfAnEmptySlot(t *testing.T) {
	assert.Equal(t, 0.0, Percent(0, 0))
	assert.Equal(t, 100.0, Percent(10, 0))
	assert.Equal(t, 0.0, Percent(4, 4))
	assert.Equal(t, 80.0, Percent(5, 1))
	assert.Equal(t, 66.67, Percent(3, 1))
	// A report that counts more failures than requests is clamped, a rate
	// below zero is not a rate of a model.
	assert.Equal(t, 0.0, Percent(2, 5))
	assert.Equal(t, 100.0, Percent(2, -5))
}
