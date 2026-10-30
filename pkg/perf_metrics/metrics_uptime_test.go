package perfmetrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The panel of the models draws the failures of a model on a timeline. The
// answer of the summary therefore has to carry every slot of the window, in the
// order of the time, with the rate of the slot itself and not only the rate of
// the whole window.
func TestAlignedUptimeBucketsCoversEverySlot(t *testing.T) {
	const hour = int64(3600)
	start := int64(1_700_000_000) - int64(1_700_000_000)%hour
	end := start + 5*hour + 1200 // 20 minutes into the last slot

	buckets := map[int64]counters{
		// Slot 1: every request failed.
		start + hour: {requestCount: 4, successCount: 0},
		// Slot 3: one of five requests failed.
		start + 3*hour: {requestCount: 5, successCount: 4},
		// Slot 5: the running slot, all requests are good.
		start + 5*hour: {requestCount: 2, successCount: 2},
	}

	got := alignedUptimeBuckets(buckets, start, end, 0)
	require.Len(t, got, 6)

	for index, bucket := range got {
		assert.Equal(t, start+int64(index)*hour, bucket.Ts)
		assert.Equal(t, hour, bucket.DurationSecond)
	}

	// The failing slot reports the failure, not the rate of the window.
	assert.Equal(t, 0.0, got[1].SuccessRate)
	assert.Equal(t, int64(4), got[1].FailedCount)
	assert.Equal(t, 80.0, got[3].SuccessRate)
	assert.Equal(t, int64(1), got[3].FailedCount)
	assert.Equal(t, 100.0, got[5].SuccessRate)

	// A slot without requests is present, and it is marked as empty instead of
	// claiming that the model was perfect.
	assert.Equal(t, int64(0), got[0].RequestCount)
	assert.Equal(t, 0.0, got[0].SuccessRate)
	assert.Equal(t, int64(0), got[2].RequestCount)
}

func TestAlignedUptimeBucketsBoundsTheWindow(t *testing.T) {
	const hour = int64(3600)
	start := int64(1_700_000_000)
	got := alignedUptimeBuckets(nil, start, start+720*hour, 0)

	// A month of hours is the widest window the panel draws, the last slot is
	// the slot that is running now.
	require.Len(t, got, 721)
	assert.Equal(t, start+720*hour-start%hour, got[len(got)-1].Ts)
	assert.Equal(t, int64(hour), got[1].Ts-got[0].Ts)

	// A wider window is cut to the last month instead of returning a payload
	// that no panel can draw.
	require.Len(t, alignedUptimeBuckets(nil, start-365*24*hour, start, 0), maxUptimeBuckets)
}

// The setting of the metrics can store a minute of counters while the panel
// draws an hour of them. A slot of the answer must be the sum of every stored
// bucket of the slot: the bucket whose timestamp equals the start of the slot is
// one minute of the hour and it is not the hour.
func TestAlignedUptimeBucketsSumsTheFinerBucketsOfASlot(t *testing.T) {
	const hour = int64(3600)
	const minute = int64(60)
	start := int64(1_700_000_000) - int64(1_700_000_000)%hour

	buckets := map[int64]counters{}
	// One hour of minute buckets: 60 requests, 3 of them failed, and the only
	// bucket that starts at the top of the hour is a good minute.
	for index := int64(0); index < 60; index++ {
		buckets[start+index*minute] = counters{requestCount: 1, successCount: 1}
	}
	buckets[start] = counters{requestCount: 1, successCount: 1}
	buckets[start+7*minute] = counters{requestCount: 1, successCount: 0}
	buckets[start+31*minute] = counters{requestCount: 1, successCount: 0}
	buckets[start+44*minute] = counters{requestCount: 1, successCount: 0}

	got := alignedUptimeBuckets(buckets, start, start+hour, 0)

	require.Len(t, got, 2)
	// The hour carries the whole traffic of the hour, not one minute of it.
	assert.Equal(t, int64(60), got[0].RequestCount)
	assert.Equal(t, int64(3), got[0].FailedCount)
	assert.Equal(t, 95.0, got[0].SuccessRate)
	// The hour is not perfect just because the minute at its start was.
	assert.NotEqual(t, 100.0, got[0].SuccessRate)
}
