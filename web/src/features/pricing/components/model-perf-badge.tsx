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
import { ChevronRight } from 'lucide-react'
import { memo, useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import type { UptimeBucket } from '@/features/performance-metrics/types'
import { getSuccessRateTextClass } from '@/features/performance-metrics/lib/format'
import { cn } from '@/lib/utils'
import type { PricingChannel } from '../types'
import {
  bucketLevel,
  fillBuckets,
  uptimePercent,
  type PerfBar,
  type UptimeLevel,
} from '../lib/uptime'

export type ModelPerfBadgeData = {
  avg_latency_ms: number
  success_rate: number
  avg_tps: number
  recent_success_rates?: number[]
  uptime_buckets?: UptimeBucket[]
  request_count?: number
}

export interface ModelPerfBadgeProps extends React.HTMLAttributes<HTMLDivElement> {
  perf: ModelPerfBadgeData | undefined
  channels?: PricingChannel[]
}

function formatCompactLatency(ms: number): string {
  if (!Number.isFinite(ms) || ms <= 0) return '—'
  if (ms >= 1000) return `${(ms / 1000).toFixed(2)}s`
  return `${Math.round(ms)}ms`
}

function formatCompactThroughput(tps: number): string {
  if (!Number.isFinite(tps) || tps <= 0) return '—'
  return `${tps.toFixed(1)}t/s`
}

/** Number of slots the card draws, one per hour of the day. */
export const PERF_BAR_COUNT = 24
export type { PerfBar }
const PERF_BAR_WIDTH_SECONDS = 3600

const BAR_CLASS: Record<UptimeLevel, string> = {
  ok: 'bg-emerald-500',
  warn: 'bg-amber-500',
  bad: 'bg-destructive',
  empty: 'bg-muted-foreground/25',
}

/**
 * Build the bars of the last day of a model.
 *
 * Every bar is one hour of the real timeline and its colour is the measured
 * success rate of that hour. An hour without requests is drawn as unknown: the
 * panel never puts a green bar on a slot that has no evidence and it never
 * invents red bars to reach an expected failure count.
 */
export function buildPerfBars(
  perf: ModelPerfBadgeData | undefined,
  nowMs: number = Date.now()
): PerfBar[] {
  if (!perf) {
    return fillBuckets(undefined, PERF_BAR_WIDTH_SECONDS, PERF_BAR_COUNT, nowMs).map(
      (bucket) => ({ ts: bucket.ts, level: 'empty' as UptimeLevel, rate: 0, requests: 0 })
    )
  }

  const buckets = fillBuckets(
    perf.uptime_buckets,
    PERF_BAR_WIDTH_SECONDS,
    PERF_BAR_COUNT,
    nowMs
  )
  return buckets.map((bucket) => ({
    ts: bucket.ts,
    level: bucketLevel(bucket),
    rate: bucket.request_count > 0 ? bucket.success_rate : 0,
    requests: bucket.request_count,
  }))
}

export function useModelPerfData(
  perf: ModelPerfBadgeData | undefined,
  channels?: PricingChannel[]
) {
  return useMemo(() => {
    let lat = '—'
    let tps = '—'

    if (perf) {
      lat = formatCompactLatency(perf.avg_latency_ms)
      tps = formatCompactThroughput(perf.avg_tps)
    }

    // The uptime of the model comes from the requests that the gateway served.
    // The status of a channel is a property of the channel and it never proves
    // that the requests of this model succeeded, therefore it is not used here.
    const rate = uptimePercent(perf?.uptime_buckets)

    if (channels && channels.length > 0) {
      const validTtfts = channels
        .map((c) => c.ttft)
        .filter((t): t is number => t != null && t > 0)
      const validRespTimes = channels
        .map((c) => c.response_time)
        .filter((r): r is number => r != null && r > 0)
      const validTps = channels
        .map((c) => c.tps)
        .filter((s): s is number => s != null && s > 0)

      if (lat === '—') {
        if (validTtfts.length > 0) {
          lat = `${Math.min(...validTtfts).toFixed(2)}s`
        } else if (validRespTimes.length > 0) {
          lat = `${Math.min(...validRespTimes)}ms`
        }
      }
      if (tps === '—' && validTps.length > 0) {
        tps = `${Math.max(...validTps).toFixed(1)}t/s`
      }
    }

    return {
      statusRate: rate,
      latencyText: lat,
      tpsText: tps,
      bars: buildPerfBars(perf),
    }
  }, [perf, channels])
}

function statusClass(rate: number | null): string {
  if (rate === null) return 'text-muted-foreground'
  return getSuccessRateTextClass(rate)
}

function statusText(rate: number | null): string {
  return rate === null ? '—' : `${rate.toFixed(2)}%`
}

/**
 * Top row metrics: Status, Latency, TPS
 */
export const ModelPerfStatsRow = memo(function ModelPerfStatsRow(props: {
  perf: ModelPerfBadgeData | undefined
  channels?: PricingChannel[]
  className?: string
}) {
  const { t } = useTranslation()
  const { statusRate, latencyText, tpsText } = useModelPerfData(
    props.perf,
    props.channels
  )

  return (
    <div
      className={cn(
        'flex items-center gap-3 sm:gap-4 min-w-0 text-[11px] leading-tight font-mono',
        props.className
      )}
    >
      <div className='flex items-center gap-1.5 shrink-0'>
        <span className='text-muted-foreground font-sans'>{t('Status')}</span>
        <span className={cn('font-semibold tabular-nums', statusClass(statusRate))}>
          {statusText(statusRate)}
        </span>
      </div>

      <div className='flex items-center gap-1.5 shrink-0'>
        <span className='text-muted-foreground font-sans'>{t('Lat.')}</span>
        <span className='text-foreground font-medium tabular-nums'>
          {latencyText}
        </span>
      </div>

      <div className='flex items-center gap-1.5 shrink-0'>
        <span className='text-muted-foreground font-sans'>{t('TPS')}</span>
        <span className='text-foreground font-medium tabular-nums'>
          {tpsText}
        </span>
      </div>
    </div>
  )
})

/**
 * Full-width 24 sparkline bars row (OpenRouter style)
 */
export const ModelPerfSparkline = memo(function ModelPerfSparkline(props: {
  perf: ModelPerfBadgeData | undefined
  channels?: PricingChannel[]
  className?: string
}) {
  const { t } = useTranslation()
  const { bars } = useModelPerfData(props.perf, props.channels)

  return (
    <div
      className={cn(
        'flex items-center gap-[2.5px] w-full h-2 sm:h-2.5',
        props.className
      )}
    >
      {bars.map((bar) => (
        <div
          key={bar.ts}
          className={cn(
            'flex-1 h-full rounded-[1px] transition-colors',
            BAR_CLASS[bar.level]
          )}
          title={
            bar.level === 'empty'
              ? t('No requests in this hour')
              : `${new Date(bar.ts * 1000).toLocaleString()} · ${bar.rate.toFixed(2)}% (${bar.requests})`
          }
        />
      ))}
    </div>
  )
})

export const ModelPerfBadge = memo(function ModelPerfBadge(
  props: ModelPerfBadgeProps
) {
  return (
    <div className={cn('flex flex-col gap-1.5 w-full', props.className)}>
      <ModelPerfStatsRow perf={props.perf} channels={props.channels} />
      <ModelPerfSparkline perf={props.perf} channels={props.channels} />
    </div>
  )
})

/**
 * OpenRouter-style card telemetry footer matching Image 2:
 * Status % with dense vertical sparkline bars directly below,
 * Latency and TPS side-by-side, Details button on the right.
 */
export const ModelCardTelemetry = memo(function ModelCardTelemetry(props: {
  perf: ModelPerfBadgeData | undefined
  channels?: PricingChannel[]
  onDetailsClick?: () => void
  className?: string
}) {
  const { t } = useTranslation()
  const { statusRate, latencyText, tpsText, bars } = useModelPerfData(
    props.perf,
    props.channels
  )

  return (
    <div
      className={cn(
        'flex items-center justify-between gap-3 text-xs w-full min-w-0',
        props.className
      )}
    >
      <div className='flex items-start gap-3.5 sm:gap-5 min-w-0 font-mono'>
        {/* Status + Vertical Sparkline Bars directly underneath */}
        <div className='flex flex-col gap-1 shrink-0'>
          <div className='flex items-center gap-1.5 text-[11px] leading-none'>
            <span className='text-muted-foreground font-sans'>{t('Status')}</span>
            <span className={cn('font-semibold tabular-nums', statusClass(statusRate))}>
              {statusText(statusRate)}
            </span>
          </div>
          <div
            className='flex items-center gap-[1.5px] h-3.5 pt-0.5'
            title={
              statusRate === null
                ? t('No requests in the last 24 hours')
                : `${statusText(statusRate)} ${t('uptime in the last 24 hours')}`
            }
          >
            {bars.map((bar) => (
              <div
                key={bar.ts}
                className={cn(
                  'w-[2.5px] h-full rounded-[1px] transition-colors shrink-0',
                  BAR_CLASS[bar.level]
                )}
                title={
                  bar.level === 'empty'
                    ? `${new Date(bar.ts * 1000).toLocaleString()} · ${t('No requests in this hour')}`
                    : `${new Date(bar.ts * 1000).toLocaleString()} · ${bar.rate.toFixed(2)}% (${bar.requests})`
                }
              />
            ))}
          </div>
        </div>

        <div className='flex flex-col gap-1 shrink-0'>
          <span className='text-muted-foreground font-sans text-[11px] leading-none'>
            {t('Lat.')}
          </span>
          <span className='text-foreground font-medium tabular-nums text-[11px] leading-none'>
            {latencyText}
          </span>
        </div>

        <div className='flex flex-col gap-1 shrink-0'>
          <span className='text-muted-foreground font-sans text-[11px] leading-none'>
            {t('TPS')}
          </span>
          <span className='text-foreground font-medium tabular-nums text-[11px] leading-none'>
            {tpsText}
          </span>
        </div>
      </div>

      {props.onDetailsClick && (
        <button
          type='button'
          onClick={props.onDetailsClick}
          className='text-muted-foreground hover:text-foreground inline-flex items-center gap-0.5 text-[11px] font-medium transition-colors shrink-0'
        >
          {t('Details')}
          <ChevronRight className='h-3.5 w-3.5' />
        </button>
      )}
    </div>
  )
})
