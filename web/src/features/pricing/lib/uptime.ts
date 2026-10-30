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
import type { UptimeBucket } from '@/features/performance-metrics/types'

// ----------------------------------------------------------------------------
// Uptime of a model
// ----------------------------------------------------------------------------
//
// The uptime of a model is measured by the requests the gateway serves, it is
// not estimated. The backend reports one slot per bucket of the monitoring
// window together with the number of requests of the slot, therefore:
//
//   - the rate of a slot is the rate of the requests of that slot,
//   - a slot without requests is drawn green, because the strip answers whether
//     a request of the model failed and a silent hour lost no request; its
//     tooltip names the hour as empty instead of showing a made up rate,
//   - a slot is never counted as a failure on the strength of the silence of the
//     installation, red comes only from a request that the slot actually lost,
//   - the position of a failing slot on the timeline follows from its timestamp,
//     so an incident from yesterday never appears as an incident of today.

/**
 * Colour of one slot of the uptime timeline.
 *
 * The panel draws two states and not four: a slot where every request of the
 * slot succeeded (or where the model served no request at all) is green and a
 * slot that lost a request is red. A yellow state between the two of them made
 * a reader look for a third meaning that the strip never explained.
 */
export type UptimeLevel = 'ok' | 'bad'

/**
 * The rate from which a slot is drawn green.
 *
 * The rate of a slot is the share of its requests that succeeded, therefore a
 * slot is green when no request of it failed. The tolerance covers the rounding
 * of the rate of the answer: 100 of 100 requests is exactly 100 percent and the
 * value of a slot that served a lot of requests is rounded by the server.
 */
const UPTIME_OK_MIN = 99.9

/**
 * The rate that the square prints for a model without a measurement.
 *
 * A model that served no request has no failed request, therefore the square
 * reads it as a hundred percent. A dash in the column of the status reads as
 * "unknown" and the reader scrolls past the model; the number never claims a
 * measurement that nobody made, the tooltip of the bar still names the hour as
 * empty and the tooltip of the status still says that the model served nothing.
 */
export const NO_REQUEST_RATE = 100

export type UptimeDayPoint = {
  /** Local date of the day, `YYYY-MM-DD`. */
  date: string
  uptime_pct: number
  incidents: number
  outage_minutes: number
  requests: number
}

const DAY_SECONDS = 86_400

/**
 * Failed requests of a slot.
 *
 * The backend reports the number of the requests of a slot and the number of
 * the requests that failed, and the panel never assumes a success: a payload
 * that carries a measured rate but not the counter of the failures is read from
 * the rate. Reading such a payload as "no failure" would paint every model of
 * the square green.
 */
export function failedRequests(bucket: UptimeBucket): number {
  const requests = Math.max(0, bucket.request_count ?? 0)
  if (requests === 0) return 0
  if (
    typeof bucket.failed_count === 'number' &&
    Number.isFinite(bucket.failed_count)
  ) {
    return Math.min(requests, Math.max(0, bucket.failed_count))
  }
  const rate = bucket.success_rate
  if (typeof rate === 'number' && Number.isFinite(rate)) {
    const succeeded = (Math.min(100, Math.max(0, rate)) / 100) * requests
    return Math.min(requests, Math.max(0, Math.round(requests - succeeded)))
  }
  return 0
}

/**
 * Level of a single slot of the monitoring window.
 *
 * A slot without requests is green: the strip of a model tells whether the
 * requests of the model failed, and an hour in which the model was not asked
 * anything is an hour without a failure. The tooltip of the bar still says that
 * the hour carried no request, the reader never mistakes the silence of the
 * installation for a measurement.
 */
export function bucketLevel(bucket: UptimeBucket | undefined): UptimeLevel {
  if (!bucket || bucket.request_count <= 0) return 'ok'
  if (!Number.isFinite(bucket.success_rate)) return 'bad'
  return bucket.success_rate >= UPTIME_OK_MIN ? 'ok' : 'bad'
}

/**
 * Uptime of the window, weighted with the requests of every slot.
 *
 * Returns null when the model did not serve a single request in the window:
 * a model without traffic has no measured uptime and the panel shows a dash
 * instead of a made up 100%.
 */
export function uptimePercent(
  buckets: UptimeBucket[] | undefined
): number | null {
  if (!buckets || buckets.length === 0) return null
  let requests = 0
  let succeeded = 0
  for (const bucket of buckets) {
    if (!Number.isFinite(bucket.request_count) || bucket.request_count <= 0)
      continue
    requests += bucket.request_count
    succeeded += bucket.request_count - failedRequests(bucket)
  }
  if (requests === 0) return null
  return (succeeded / requests) * 100
}

/**
 * Fill the window with empty slots.
 *
 * The last slot of the answer is the slot that is running now, the older slots
 * follow it to the left. The panel draws a bar per slot, a missing slot is a
 * slot without requests and it stays visibly unknown.
 */
export function fillBuckets(
  buckets: UptimeBucket[] | undefined,
  width: number,
  count: number,
  nowMs: number = Date.now()
): UptimeBucket[] {
  const slotWidth = width > 0 ? width : 3600
  const slotCount = count > 0 ? count : 24
  const nowSeconds = Math.floor(nowMs / 1000)
  const last = nowSeconds - (nowSeconds % slotWidth)

  // The stored bucket of the backend can be finer than the slot of the panel
  // (a minute of counters inside an hour of bars). Every bucket is summed into
  // the slot that contains it: a slot is the traffic of the slot, never only
  // the one bucket whose timestamp equals the start of the slot.
  const known = new Map<number, { requests: number; failed: number }>()
  for (const bucket of buckets ?? []) {
    if (!Number.isFinite(bucket?.ts)) continue
    const slot = bucket.ts - (bucket.ts % slotWidth)
    const requests = Math.max(0, bucket.request_count ?? 0)
    if (requests === 0) continue
    const current = known.get(slot) ?? { requests: 0, failed: 0 }
    current.requests += requests
    current.failed += failedRequests(bucket)
    known.set(slot, current)
  }

  const filled: UptimeBucket[] = []
  for (let index = slotCount - 1; index >= 0; index--) {
    const ts = last - index * slotWidth
    const value = known.get(ts)
    if (!value || value.requests === 0) {
      filled.push({
        ts,
        duration_seconds: slotWidth,
        success_rate: 0,
        request_count: 0,
        failed_count: 0,
      })
      continue
    }
    const failed = Math.min(value.requests, value.failed)
    filled.push({
      ts,
      duration_seconds: slotWidth,
      success_rate: ((value.requests - failed) / value.requests) * 100,
      request_count: value.requests,
      failed_count: failed,
    })
  }
  return filled
}

/** Local `YYYY-MM-DD` of a timestamp in seconds. */
export function localDateOf(tsSeconds: number): string {
  const date = new Date(tsSeconds * 1000)
  const month = `${date.getMonth() + 1}`.padStart(2, '0')
  const day = `${date.getDate()}`.padStart(2, '0')
  return `${date.getFullYear()}-${month}-${day}`
}

/**
 * Aggregate the slots of the last `days` days to days.
 *
 * The hours are grouped by the local date of the timestamp, the outage of a day
 * is the part of its requests that failed. A day without requests and without
 * failures is reported with no measurement, `requests` is zero then.
 */
export function buildDailyUptime(
  buckets: UptimeBucket[] | undefined,
  days: number = 30,
  nowMs: number = Date.now()
): UptimeDayPoint[] {
  const dayCount = days > 0 ? days : 30
  const today = new Date(nowMs)
  today.setHours(0, 0, 0, 0)

  const byDate = new Map<
    string,
    { requests: number; failed: number; slots: number; failingSlots: number }
  >()
  for (const bucket of buckets ?? []) {
    if (!Number.isFinite(bucket?.ts)) continue
    const key = localDateOf(bucket.ts)
    const current = byDate.get(key) ?? {
      requests: 0,
      failed: 0,
      slots: 0,
      failingSlots: 0,
    }
    const requests = Math.max(0, bucket.request_count ?? 0)
    const failed = failedRequests(bucket)
    current.requests += requests
    current.failed += failed
    current.slots += 1
    if (failed > 0) current.failingSlots += 1
    byDate.set(key, current)
  }

  const series: UptimeDayPoint[] = []
  for (let index = dayCount - 1; index >= 0; index--) {
    const day = new Date(today.getTime() - index * DAY_SECONDS * 1000)
    const key = localDateOf(Math.floor(day.getTime() / 1000))
    const value = byDate.get(key)
    if (!value || value.requests === 0) {
      series.push({
        date: key,
        uptime_pct: 0,
        incidents: 0,
        outage_minutes: 0,
        requests: 0,
      })
      continue
    }
    const succeeded = value.requests - value.failed
    series.push({
      date: key,
      uptime_pct: (succeeded / value.requests) * 100,
      incidents: value.failingSlots,
      // The duration of the outage follows from the share of the requests that
      // failed, a day of requests is one day of monitoring.
      outage_minutes: Math.round((value.failed / value.requests) * 24 * 60),
      requests: value.requests,
    })
  }
  return series
}

/** Uptime of a daily series over the days that carry requests. */
export function aggregateDailyUptime(points: UptimeDayPoint[]): {
  uptime_pct: number
  incidents: number
  outage_minutes: number
  days: number
} {
  const measured = points.filter((point) => point.requests > 0)
  if (measured.length === 0) {
    return { uptime_pct: 0, incidents: 0, outage_minutes: 0, days: 0 }
  }
  let requests = 0
  let succeeded = 0
  let incidents = 0
  let outageMinutes = 0
  for (const point of measured) {
    requests += point.requests
    succeeded += (point.uptime_pct / 100) * point.requests
    incidents += point.incidents
    outageMinutes += point.outage_minutes
  }
  return {
    uptime_pct: (succeeded / requests) * 100,
    incidents,
    outage_minutes: outageMinutes,
    days: measured.length,
  }
}

/**
 * Bars of a channel from its hourly history.
 *
 * The history carries one entry per hour of the window, oldest first, and the
 * entry of an hour without requests is null. The entries are read from the end
 * of the list, because the last entry is the hour that just ended, and an entry
 * without a rate is an hour that served nothing: it is drawn green and its
 * tooltip says that the hour carried no request.
 */
export function barsFromHistory(
  history: (number | null)[] | undefined,
  nowMs: number = Date.now()
): PerfBar[] {
  const slots = fillBuckets(undefined, 3600, 24, nowMs)
  const values = history ?? []
  const start = Math.max(0, slots.length - values.length)
  return slots.map((slot, index) => {
    const value = index >= start ? values[index - start] : undefined
    if (value === undefined || value === null || !Number.isFinite(value)) {
      return { ts: slot.ts, level: 'ok' as UptimeLevel, rate: 0, requests: 0 }
    }
    // The history is a percentage, the same unit as the rate of a slot of the
    // card. Tricking a rate of one percent into a fraction would draw an hour
    // that lost ninety-nine requests of a hundred as a perfect hour.
    const level: UptimeLevel = value >= UPTIME_OK_MIN ? 'ok' : 'bad'
    return { ts: slot.ts, level, rate: value }
  })
}

export type PerfBar = {
  ts: number
  level: UptimeLevel
  rate: number
  /**
   * Number of requests of the slot, `0` for a slot that served none.
   *
   * Left undefined when the source of the bar carries a rate without a count,
   * which is the case of the hourly history of a channel: the tooltip of such a
   * bar shows the rate alone instead of inventing a request count.
   */
  requests?: number
}
