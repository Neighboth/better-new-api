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
//   - a slot without requests proves nothing and is drawn as unknown instead of
//     being counted as a failure or as a success,
//   - the position of a failing slot on the timeline follows from its timestamp,
//     so an incident from yesterday never appears as an incident of today.

export type UptimeLevel = 'ok' | 'warn' | 'bad' | 'empty'

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

/** Level of a single slot of the monitoring window. */
export function bucketLevel(bucket: UptimeBucket | undefined): UptimeLevel {
  if (!bucket || bucket.request_count <= 0) return 'empty'
  if (bucket.success_rate >= 99.9) return 'ok'
  if (bucket.success_rate >= 95) return 'warn'
  return 'bad'
}

/**
 * Uptime of the window, weighted with the requests of every slot.
 *
 * Returns null when the model did not serve a single request in the window:
 * a model without traffic has no measured uptime and the panel shows a dash
 * instead of a made up 100%.
 */
export function uptimePercent(buckets: UptimeBucket[] | undefined): number | null {
  if (!buckets || buckets.length === 0) return null
  let requests = 0
  let succeeded = 0
  for (const bucket of buckets) {
    if (!Number.isFinite(bucket.request_count) || bucket.request_count <= 0) continue
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

  const byDate = new Map<string, { requests: number; failed: number; slots: number; failingSlots: number }>()
  for (const bucket of buckets ?? []) {
    if (!Number.isFinite(bucket?.ts)) continue
    const key = localDateOf(bucket.ts)
    const current = byDate.get(key) ?? { requests: 0, failed: 0, slots: 0, failingSlots: 0 }
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
      series.push({ date: key, uptime_pct: 0, incidents: 0, outage_minutes: 0, requests: 0 })
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
 * The history carries the last hours of the channel, oldest first, without a
 * timestamp per entry. The entries fill the most recent slots and the hours
 * that the history does not cover stay unknown.
 */
export function barsFromHistory(
  history: number[] | undefined,
  nowMs: number = Date.now()
): PerfBar[] {
  const slots = fillBuckets(undefined, 3600, 24, nowMs)
  const values = (history ?? []).filter((value) => Number.isFinite(value))
  const start = Math.max(0, slots.length - values.length)
  return slots.map((slot, index) => {
    const value = index >= start ? values[index - start] : undefined
    if (value === undefined) {
      return { ts: slot.ts, level: 'empty' as UptimeLevel, rate: 0, requests: 0 }
    }
    const rate = value <= 1 ? value * 100 : value
    const level: UptimeLevel = rate >= 99.9 ? 'ok' : rate >= 95 ? 'warn' : 'bad'
    return { ts: slot.ts, level, rate, requests: 1 }
  })
}

export type PerfBar = {
  ts: number
  level: UptimeLevel
  rate: number
  requests: number
}
