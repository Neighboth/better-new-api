// Package uptime turns the counters of the relay into the timeline that the
// panel of the models draws.
//
// The package has no dependency on the store of the metrics: the alignment of a
// window and the rate of a slot are arithmetic, they are used by the summary of
// the metrics and they are tested on their own.
package uptime

import "math"

// Bucket is one slot of the timeline of a model.
//
// Every slot of the window is reported, a slot without requests carries a
// request count of zero. The client draws such a slot as unknown: it is neither
// a success nor a failure of the model.
type Bucket struct {
	Ts         int64
	Requests   int64
	Failures   int64
	SuccessPct float64
}

// SlotStart returns the start of the slot that contains a timestamp.
//
// A width that is not positive is a window of an hour, the setting of the
// metrics is not always available when the counters are read.
func SlotStart(ts int64, width int64) int64 {
	if width <= 0 {
		width = 3600
	}
	if ts >= 0 {
		return ts - ts%width
	}
	return ts - ((ts%width)+width)%width
}

// SlotStarts returns the start of every slot of the window, oldest first.
//
// The slots are aligned to the width of the bucket and the last slot is the slot
// that contains the end of the window, therefore the last slot of the answer is
// the state of the model now. A window that is wider than the limit is cut to
// its most recent slots instead of returning a payload that no panel can draw.
func SlotStarts(startTs int64, endTs int64, width int64, limit int) []int64 {
	if width <= 0 {
		width = 3600
	}
	if endTs < startTs {
		startTs, endTs = endTs, startTs
	}
	first := SlotStart(startTs, width)
	last := SlotStart(endTs, width)
	if last < first {
		return nil
	}
	count := (last-first)/width + 1
	if limit > 0 && count > int64(limit) {
		first = last - int64(limit-1)*width
		count = int64(limit)
	}
	starts := make([]int64, 0, count)
	for ts := first; ts <= last; ts += width {
		starts = append(starts, ts)
	}
	return starts
}

// Percent returns the share of the requests that succeeded, rounded to two
// decimals.
//
// A slot without requests has no rate, the value is zero and the caller shows
// the slot as unknown.
func Percent(requests int64, failures int64) float64 {
	if requests <= 0 {
		return 0
	}
	if failures < 0 {
		failures = 0
	}
	if failures > requests {
		failures = requests
	}
	return math.Round((float64(requests-failures)/float64(requests))*10000) / 100
}

// Merge adds the counters of a slot to the counters of another slot.
//
// The counters of the groups of a model are merged for the timeline of the
// model: a request of a group is a request of the model.
func Merge(requests int64, failures int64, otherRequests int64, otherFailures int64) (int64, int64) {
	return requests + otherRequests, failures + otherFailures
}
