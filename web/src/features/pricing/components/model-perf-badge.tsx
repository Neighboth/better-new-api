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
import { memo, useMemo } from 'react'
import { useTranslation } from 'react-i18next'

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

export const ModelPerfBadge = memo(function ModelPerfBadge(
  props: ModelPerfBadgeProps
) {
  const { t } = useTranslation()

  const { statusRate, latencyText, tpsText, bars } = useMemo(() => {
    let rate = 99.9
    let lat = '—'
    let tps = '—'
    let history: number[] = []

    if (props.perf) {
      const rawRate = props.perf.success_rate
      rate = rawRate <= 1 && rawRate > 0 ? rawRate * 100 : rawRate
      lat = formatCompactLatency(props.perf.avg_latency_ms)
      tps = formatCompactThroughput(props.perf.avg_tps)

      if (props.perf.recent_success_rates && props.perf.recent_success_rates.length > 0) {
        history = props.perf.recent_success_rates.map((r) =>
          r >= 80 || (r > 0 && r <= 1 && r >= 0.8) ? 1 : 0
        )
      }
    } else if (props.channels && props.channels.length > 0) {
      const ch = props.channels[0]
      if (ch.uptime != null && ch.uptime > 0) rate = ch.uptime
      if (ch.ttft && ch.ttft > 0) lat = `${ch.ttft.toFixed(2)}s`
      else if (ch.response_time && ch.response_time > 0) lat = `${ch.response_time}ms`
      if (ch.tps && ch.tps > 0) tps = `${ch.tps.toFixed(1)}t/s`
      if (ch.uptime_history && ch.uptime_history.length > 0) {
        history = ch.uptime_history
      }
    }

    // Build 24 bars
    let finalBars: number[] = []
    if (history.length > 0) {
      if (history.length >= 24) {
        finalBars = history.slice(-24)
      } else {
        finalBars = [...Array(24 - history.length).fill(1), ...history]
      }
    } else {
      // Derive 24 bars from rate
      const greenCount = Math.min(24, Math.max(0, Math.round((rate / 100) * 24)))
      const redCount = 24 - greenCount
      finalBars = Array(24).fill(1)
      if (redCount > 0) {
        const step = Math.max(1, Math.floor(24 / redCount))
        for (let i = 0; i < redCount; i++) {
          const idx = Math.min(23, (i * step) % 24)
          finalBars[idx] = 0
        }
      }
    }

    return {
      statusRate: rate,
      latencyText: lat,
      tpsText: tps,
      bars: finalBars,
    }
  }, [props.perf, props.channels])

  return (
    <div
      className={cn(
        'flex items-center gap-3 sm:gap-5 min-w-0 font-mono',
        props.className
      )}
    >
      {/* Column 1: Status & 24 Sparkline Bars */}
      <div className='flex flex-col gap-1 w-[142px] shrink-0'>
        <div className='flex items-center justify-between text-[11px] leading-tight'>
          <span className='text-muted-foreground font-sans'>{t('Status')}</span>
          <span
            className={cn(
              'font-semibold tabular-nums',
              statusRate >= 90 ? 'text-emerald-500' : 'text-amber-500'
            )}
          >
            {statusRate.toFixed(2)}%
          </span>
        </div>
        <div className='flex items-center gap-[2px]'>
          {bars.map((val, i) => (
            <div
              key={i}
              className={cn(
                'w-1 h-3 rounded-[1px] transition-colors',
                val === 1 ? 'bg-emerald-500' : 'bg-destructive'
              )}
              title={val === 1 ? t('Operational') : t('Incident')}
            />
          ))}
        </div>
      </div>

      {/* Column 2: Latency */}
      <div className='flex flex-col text-[11px] leading-tight shrink-0'>
        <span className='text-muted-foreground font-sans'>{t('Lat.')}</span>
        <span className='text-foreground font-medium mt-1 tabular-nums'>
          {latencyText}
        </span>
      </div>

      {/* Column 3: TPS */}
      <div className='flex flex-col text-[11px] leading-tight shrink-0'>
        <span className='text-muted-foreground font-sans'>{t('TPS')}</span>
        <span className='text-foreground font-medium mt-1 tabular-nums'>
          {tpsText}
        </span>
      </div>
    </div>
  )
})
