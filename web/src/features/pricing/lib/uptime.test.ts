/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { describe, expect, it } from 'vitest'

import type { UptimeBucket } from '@/features/performance-metrics/types'

import {
  aggregateDailyUptime,
  bucketLevel,
  buildDailyUptime,
  fillBuckets,
  uptimePercent,
} from './uptime'

const HOUR = 3600
const DAY = 86_400

/** A fixed point of the time, the tests do not depend on the clock. */
const NOW = Date.UTC(2026, 9, 30, 12, 30, 0) / 1000

function bucket(
  ts: number,
  requests: number,
  failed: number
): UptimeBucket {
  return {
    ts,
    duration_seconds: HOUR,
    success_rate: requests > 0 ? ((requests - failed) / requests) * 100 : 0,
    request_count: requests,
    failed_count: failed,
  }
}

describe('uptime of a model', () => {
  it('weights the window with the requests and not with the slots', () => {
    const buckets = [bucket(NOW - 2 * HOUR, 100, 0), bucket(NOW - HOUR, 1, 1)]
    // One failed request of a single request is half of the requests of the
    // window, the hundred good requests do not hide it with an average of the
    // two rates (100 and 0) that would say fifty percent.
    expect(uptimePercent(buckets)).toBeCloseTo((100 / 101) * 100, 5)
  })

  it('has no rate when the model served nothing', () => {
    expect(uptimePercent([])).toBeNull()
    expect(uptimePercent(undefined)).toBeNull()
    expect(uptimePercent([bucket(NOW - HOUR, 0, 0)])).toBeNull()
  })

  it('never reports a rate below zero or above a hundred', () => {
    expect(uptimePercent([bucket(NOW - HOUR, 2, 5)])).toBe(0)
    expect(uptimePercent([bucket(NOW - HOUR, 2, -5)])).toBe(100)
  })

  it('draws the slot of a model by the requests of the slot', () => {
    expect(bucketLevel(bucket(NOW, 0, 0))).toBe('empty')
    expect(bucketLevel(bucket(NOW, 10, 0))).toBe('ok')
    // A slot that lost one request of a hundred is degraded, a slot that lost
    // half of its requests is a failure of the model.
    expect(bucketLevel(bucket(NOW, 100, 2))).toBe('warn')
    expect(bucketLevel(bucket(NOW, 10, 5))).toBe('bad')
    expect(bucketLevel(undefined)).toBe('empty')
  })
})

describe('fillBuckets', () => {
  it('puts the last slot of the answer at the current hour', () => {
    const filled = fillBuckets([], HOUR, 24, NOW * 1000)
    expect(filled).toHaveLength(24)
    expect(filled[filled.length - 1].ts).toBe(NOW - (NOW % HOUR))
    expect(filled[0].ts).toBe(NOW - (NOW % HOUR) - 23 * HOUR)
  })

  it('keeps the slots of the answer on their own hour', () => {
    // The answer carries 25 slots for a day of hours, the panel draws the last
    // 24 of them and the failure of an hour stays on that hour.
    const failing = bucket(NOW - (NOW % HOUR) - 5 * HOUR, 4, 4)
    const filled = fillBuckets([bucket(NOW, 10, 0), failing], HOUR, 24, NOW * 1000)
    const index = filled.findIndex((entry) => entry.ts === failing.ts)
    expect(index).toBe(24 - 6)
    expect(filled[index].failed_count).toBe(4)
    expect(filled[0].request_count).toBe(0)
  })
})

describe('buildDailyUptime', () => {
  it('groups the hours by the day of the timestamp', () => {
    const today = NOW - (NOW % DAY)
    const series = buildDailyUptime(
      [
        bucket(today + 10 * HOUR, 10, 1),
        bucket(today + 11 * HOUR, 10, 1),
        bucket(today - DAY + 10 * HOUR, 4, 0),
      ],
      30,
      NOW * 1000
    )

    expect(series).toHaveLength(30)
    const last = series[series.length - 1]
    expect(last.requests).toBe(20)
    expect(last.uptime_pct).toBeCloseTo(90, 5)
    expect(last.incidents).toBe(2)

    const yesterday = series[series.length - 2]
    expect(yesterday.requests).toBe(4)
    expect(yesterday.uptime_pct).toBe(100)
    expect(yesterday.incidents).toBe(0)
  })

  it('marks a day without requests as unknown instead of perfect', () => {
    const series = buildDailyUptime([], 30, NOW * 1000)
    expect(series).toHaveLength(30)
    for (const day of series) {
      expect(day.requests).toBe(0)
      expect(day.uptime_pct).toBe(0)
    }
    const summary = aggregateDailyUptime(series)
    expect(summary.days).toBe(0)
    expect(summary.uptime_pct).toBe(0)
  })

  it('aggregates the days with the requests of the days', () => {
    const series = buildDailyUptime(
      [bucket(NOW - 2 * DAY, 100, 0), bucket(NOW - DAY, 10, 10)],
      30,
      NOW * 1000
    )
    const summary = aggregateDailyUptime(series)
    expect(summary.days).toBe(2)
    expect(summary.uptime_pct).toBeCloseTo((100 / 110) * 100, 5)
    expect(summary.incidents).toBe(1)
  })
})
