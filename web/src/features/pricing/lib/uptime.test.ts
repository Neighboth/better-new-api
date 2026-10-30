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
  barsFromHistory,
  bucketLevel,
  buildDailyUptime,
  failedRequests,
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

  it('draws the slot of a model green or red and never a third colour', () => {
    // A slot without requests proves no failure: it is green, the tooltip of
    // the bar tells that the hour carried no request.
    expect(bucketLevel(bucket(NOW, 0, 0))).toBe('ok')
    expect(bucketLevel(undefined)).toBe('ok')
    expect(bucketLevel(bucket(NOW, 10, 0))).toBe('ok')
    // A slot that lost a request of a hundred is red, the strip has no yellow
    // state between a good hour and a failed one.
    expect(bucketLevel(bucket(NOW, 100, 1))).toBe('bad')
    expect(bucketLevel(bucket(NOW, 100, 2))).toBe('bad')
    expect(bucketLevel(bucket(NOW, 10, 5))).toBe('bad')
    // The rounding of the rate of the server does not turn a perfect hour red.
    expect(
      bucketLevel({
        ts: NOW,
        duration_seconds: HOUR,
        success_rate: 99.95,
        request_count: 10,
        failed_count: 0,
      })
    ).toBe('ok')
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

describe('a payload without the counter of the failures', () => {
  it('reads the failures from the measured rate of the slot', () => {
    // The server measured the slot and reported the rate, the counter of the
    // failures did not arrive. Reading the slot as "no failure" would paint the
    // whole square green.
    const measured: UptimeBucket = {
      ts: NOW,
      duration_seconds: HOUR,
      success_rate: 97.36,
      request_count: 100,
    } as UptimeBucket
    expect(failedRequests(measured)).toBe(3)
    expect(uptimePercent([measured])).toBeCloseTo(97, 5)
    expect(bucketLevel(fillBuckets([measured], HOUR, 24, NOW * 1000)[23])).not.toBe(
      'ok'
    )
  })

  it('sums the finer buckets of a slot instead of keeping one of them', () => {
    // A minute of counters inside an hour of bars: the hour of the card is the
    // traffic of the hour. Reading only the bucket at the top of the hour would
    // report a perfect hour out of one good minute.
    const top = NOW - (NOW % HOUR)
    const minutes: UptimeBucket[] = []
    for (let index = 0; index < 60; index++) {
      minutes.push({
        ts: top + index * 60,
        duration_seconds: 60,
        success_rate: index % 20 === 0 ? 0 : 100,
        request_count: 1,
        failed_count: index % 20 === 0 ? 1 : 0,
      })
    }
    const filled = fillBuckets(minutes, HOUR, 24, NOW * 1000)
    const hour = filled[filled.length - 1]
    expect(hour.request_count).toBe(60)
    expect(hour.failed_count).toBe(3)
    expect(hour.success_rate).toBeCloseTo(95, 5)
    expect(bucketLevel(hour)).toBe('bad')
  })
})

describe('barsFromHistory', () => {
  it('draws an hour without requests green and gives it no rate', () => {
    // The history of the backend holds one entry per hour of the window and a
    // null for the hours without requests. Those hours are green, the tooltip of
    // their bar says that the hour carried no request.
    const bars = barsFromHistory([null, null, 100], NOW * 1000)
    expect(bars).toHaveLength(24)
    expect(bars.slice(0, 21).every((bar) => bar.level === 'ok')).toBe(true)
    expect(bars.slice(0, 21).every((bar) => bar.requests === 0)).toBe(true)
    expect(bars[23].level).toBe('ok')
    expect(bars[23].requests).toBeUndefined()
  })

  it('keeps the traffic of an hour on the hour that measured it', () => {
    // The only busy hour of the day is the first one of the list. Dropping the
    // silent hours would have slid that traffic to the newest slot of the strip
    // and reported an incident of the morning as an incident of now.
    const history: (number | null)[] = [0, null, null, null]
    const bars = barsFromHistory(history, NOW * 1000)
    expect(bars[20].level).toBe('bad')
    expect(bars[20].requests).toBeUndefined()
    expect(bars[21].level).toBe('ok')
    expect(bars[22].level).toBe('ok')
    expect(bars[23].level).toBe('ok')
  })

  it('never reads a low rate as a fraction of a perfect hour', () => {
    // A hour that lost ninety-nine requests of a hundred reports one percent,
    // not a fraction of one, and the strip must keep it red.
    const bars = barsFromHistory([1], NOW * 1000)
    expect(bars[23].level).toBe('bad')
    expect(bars[23].rate).toBe(1)
  })

  it('draws an empty history as a green day without a measurement', () => {
    const bars = barsFromHistory(undefined, NOW * 1000)
    expect(bars).toHaveLength(24)
    expect(bars.every((bar) => bar.level === 'ok')).toBe(true)
    expect(bars.every((bar) => bar.requests === 0)).toBe(true)
  })
})
