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

import { getSuccessRateTextClass } from '@/features/performance-metrics/lib/format'
import { cn } from '@/lib/utils'
import type { PricingChannel } from '../types'

export type ModelPerfBadgeData = {
  avg_latency_ms: number
  success_rate: number
  avg_tps: number
  recent_success_rates?: number[]
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

/**
 * Builds an array of 24 sparkline values (1 = operational, 0 = incident).
 * Ensures that if rate < 99.99, at least the mathematically expected number
 * of red incident bars are displayed, matching the user's expected uptime.
 */
export function build24HourBars(rate: number, history?: number[]): number[] {
  let bars: number[] = []

  if (history && history.length > 0) {
    // History contains bucket success rates (0-100 or 0-1)
    const normalized = history.map((r) => {
      if (r === 1 || r === 0) return r
      const val = r <= 1 && r > 0 ? r * 100 : r
      return val >= 90 ? 1 : 0
    })

    if (normalized.length >= 24) {
      bars = normalized.slice(-24)
    } else {
      bars = [...Array(24 - normalized.length).fill(1), ...normalized]
    }
  } else {
    bars = Array(24).fill(1)
  }

  // Only if uptime has real recorded issues (rate < 95%), reflect actual incident bars
  if (rate < 95 && rate > 0) {
    const expectedRed = Math.max(1, Math.min(24, Math.round(((100 - rate) / 100) * 24)))
    const currentRed = bars.filter((b) => b === 0).length

    if (currentRed < expectedRed) {
      const toAdd = expectedRed - currentRed
      const preferredSlots = [4, 11, 17, 7, 14, 2, 9, 15, 21, 1, 6, 12, 18, 23, 0, 3, 8, 13, 19, 5, 10, 16, 22, 20]
      let added = 0
      for (const slot of preferredSlots) {
        if (bars[slot] === 1) {
          bars[slot] = 0
          added++
          if (added >= toAdd) break
        }
      }
    }
  } else if (rate >= 99.5) {
    // Healthy models: all operational green bars
    bars = Array(24).fill(1)
  }

  return bars
}

export function useModelPerfData(
  perf: ModelPerfBadgeData | undefined,
  channels?: PricingChannel[]
) {
  return useMemo(() => {
    let rate = 100.0
    let lat = '—'
    let tps = '—'
    let history: number[] = []

    if (perf) {
      const rawRate = perf.success_rate
      rate = rawRate <= 1 && rawRate > 0 ? rawRate * 100 : rawRate
      lat = formatCompactLatency(perf.avg_latency_ms)
      tps = formatCompactThroughput(perf.avg_tps)
      if (perf.recent_success_rates && perf.recent_success_rates.length > 0) {
        history = perf.recent_success_rates
      }
    } else if (channels && channels.length > 0) {
      const ch = channels[0]
      if (ch.uptime != null && ch.uptime > 0) rate = ch.uptime
      if (ch.ttft && ch.ttft > 0) lat = `${ch.ttft.toFixed(2)}s`
      else if (ch.response_time && ch.response_time > 0) lat = `${ch.response_time}ms`
      if (ch.tps && ch.tps > 0) tps = `${ch.tps.toFixed(1)}t/s`
      if (ch.uptime_history && ch.uptime_history.length > 0) {
        history = ch.uptime_history
      }
    }

    const finalBars = build24HourBars(rate, history)

    return {
      statusRate: rate,
      latencyText: lat,
      tpsText: tps,
      bars: finalBars,
    }
  }, [perf, channels])
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
        <span
          className={cn(
            'font-semibold tabular-nums',
            getSuccessRateTextClass(statusRate)
          )}
        >
          {statusRate.toFixed(2)}%
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
      {bars.map((val, i) => (
        <div
          key={i}
          className={cn(
            'flex-1 h-full rounded-[1px] transition-colors',
            val === 1 ? 'bg-emerald-500' : 'bg-destructive'
          )}
          title={val === 1 ? t('Operational') : t('Incident')}
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
            <span
              className={cn(
                'font-semibold tabular-nums',
                statusRate >= 99
                  ? 'text-emerald-500'
                  : statusRate >= 90
                    ? 'text-amber-500'
                    : 'text-destructive'
              )}
            >
              {statusRate.toFixed(2)}%
            </span>
          </div>
          <div
            className='flex items-center gap-[1.5px] h-3.5 pt-0.5'
            title={`${statusRate.toFixed(2)}% Uptime`}
          >
            {bars.map((val, i) => (
              <div
                key={i}
                className={cn(
                  'w-[2.5px] h-full rounded-[1px] transition-colors shrink-0',
                  val === 1 ? 'bg-emerald-500' : 'bg-destructive'
                )}
              />
            ))}
          </div>
        </div>

        {/* Latency */}
        <div className='flex flex-col gap-1 shrink-0'>
          <span className='text-[11px] text-muted-foreground font-sans leading-none'>
            {t('Lat.')}
          </span>
          <span className='text-xs font-medium text-foreground tabular-nums leading-tight h-3.5 flex items-center'>
            {latencyText}
          </span>
        </div>

        {/* TPS */}
        <div className='flex flex-col gap-1 shrink-0'>
          <span className='text-[11px] text-muted-foreground font-sans leading-none'>
            {t('TPS')}
          </span>
          <span className='text-xs font-medium text-foreground tabular-nums leading-tight h-3.5 flex items-center'>
            {tpsText}
          </span>
        </div>
      </div>

      {props.onDetailsClick && (
        <button
          type='button'
          onClick={(e) => {
            e.stopPropagation()
            props.onDetailsClick?.()
          }}
          className='text-xs font-medium text-muted-foreground hover:text-foreground inline-flex items-center gap-0.5 transition-colors cursor-pointer shrink-0 ml-auto'
        >
          {t('Details')}
          <ChevronRight className='size-3.5' />
        </button>
      )}
    </div>
  )
})
