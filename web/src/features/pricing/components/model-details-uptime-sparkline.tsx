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
import { Activity, AlertCircle, CheckCircle2 } from 'lucide-react'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import {
  formatUptimePct,
  getSuccessRateTextClass,
} from '@/features/performance-metrics/lib/format'
import { cn } from '@/lib/utils'

import { aggregateDailyUptime, type UptimeDayPoint } from '../lib/uptime'

// ---------------------------------------------------------------------------
// Uptime sparkline
// ---------------------------------------------------------------------------
//
// Compact 30-day uptime visualisation: a row of small coloured bars where:
//   - A day is green when the requests of the day succeeded and red when one of
//     them failed; the bar of a failed day is shorter
//   - A day without requests is green: the model was not asked anything and
//     nothing failed, the tooltip of the bar says that the day carried no
//     request
//   - Hovering a bar reveals the exact date and uptime
//
// Useful as a header strip ("at-a-glance" status) and as a per-row visual
// inside the per-group performance table.

type SparklineSize = 'sm' | 'md'

type UptimeSparklineProps = {
  series: UptimeDayPoint[]
  size?: SparklineSize
  showOverall?: boolean
  emptyLabel?: string
  className?: string
}

/** The rate from which a day is drawn green, the same rule as the hourly strip. */
const DAY_OK_MIN = 99.9

export function UptimeSparkline(props: UptimeSparklineProps) {
  const { t } = useTranslation()
  const size = props.size ?? 'md'
  const showOverall = props.showOverall ?? true

  if (props.series.length === 0) {
    return (
      <span className={cn('text-muted-foreground text-xs', props.className)}>
        {props.emptyLabel ?? '—'}
      </span>
    )
  }

  // Only the days that really served requests are measured. A day without
  // traffic is drawn green and carries no rate, it is neither a success that a
  // request proved nor a failure that nobody measured.
  const measured = props.series.filter((day) => (day.requests ?? 0) > 0)
  const overall = aggregateDailyUptime(props.series).uptime_pct

  const containerHeight = size === 'sm' ? 'h-3.5' : 'h-5'
  const barWidth = size === 'sm' ? 'w-[3px]' : 'w-1'
  const gap = size === 'sm' ? 'gap-px' : 'gap-[2px]'

  return (
    <div className={cn('flex items-center gap-2', props.className)}>
      <div
        className={cn('flex items-end', containerHeight, gap)}
        role='img'
        aria-label={`30 day uptime ${overall.toFixed(2)}%`}
      >
        {props.series.map((day) => {
          const unknown = (day.requests ?? 0) <= 0
          // Two states: a day is green when nothing failed in it, red when a
          // request of the day failed. A day without requests is green.
          const ok = unknown || day.uptime_pct >= DAY_OK_MIN
          return (
            <Tooltip key={day.date}>
              <TooltipTrigger
                render={
                  <div
                    className={cn(
                      'rounded-sm transition-opacity hover:opacity-80',
                      barWidth,
                      containerHeight,
                      'flex items-end'
                    )}
                  />
                }
              >
                <div
                  className={cn(
                    'w-full rounded-sm',
                    ok ? 'bg-emerald-500 h-full' : 'bg-destructive h-[55%]'
                  )}
                  aria-hidden
                />
              </TooltipTrigger>
              <TooltipContent side='top' className='font-mono text-xs'>
                <div className='font-medium'>{day.date}</div>
                {unknown ? (
                  <div className='text-muted-foreground'>
                    {t('No requests in this day')}
                  </div>
                ) : (
                  <>
                    <div>{day.uptime_pct.toFixed(2)}%</div>
                    <div className='text-muted-foreground'>{day.requests} requests</div>
                    {day.outage_minutes > 0 && (
                      <div className='text-muted-foreground'>
                        {day.outage_minutes} min outage
                      </div>
                    )}
                  </>
                )}
              </TooltipContent>
            </Tooltip>
          )
        })}
      </div>
      {showOverall && (
        <span
          className={cn(
            'font-mono text-sm font-semibold tabular-nums',
            measured.length === 0
              ? 'text-muted-foreground'
              : getSuccessRateTextClass(overall)
          )}
        >
          {measured.length === 0 ? '—' : `${overall.toFixed(1)}%`}
        </span>
      )}
    </div>
  )
}

// ---------------------------------------------------------------------------
// Uptime status row — sparkline + summary text + status icon
// ---------------------------------------------------------------------------

export function UptimeStatusRow(props: {
  series: UptimeDayPoint[]
  className?: string
}) {
  const { t } = useTranslation()
  const summary = useMemo(() => aggregateDailyUptime(props.series), [props.series])
  // Two states, the same rule as the bars: the window is operational when none
  // of its requests failed and it carries an outage when one of them did.
  const status = useMemo(() => {
    if (summary.days === 0) return 'unknown'
    return summary.uptime_pct >= DAY_OK_MIN ? 'operational' : 'outage'
  }, [summary.uptime_pct, summary.days])

  const StatusIcon =
    status === 'unknown'
      ? Activity
      : status === 'operational'
        ? CheckCircle2
        : AlertCircle

  const statusColour =
    status === 'unknown'
      ? 'text-muted-foreground'
      : status === 'operational'
        ? 'text-emerald-600 dark:text-emerald-400'
        : 'text-rose-600 dark:text-rose-400'

  const statusLabel =
    status === 'unknown'
      ? t('No requests were served in the last 30 days')
      : status === 'operational'
        ? t('All systems operational')
        : t('Requests failed in the last 30 days')

  return (
    <div
      className={cn(
        'border-border/60 bg-muted/30 flex flex-wrap items-center gap-3 rounded-lg border px-3 py-2 sm:gap-4 sm:px-4',
        props.className
      )}
    >
      <div className='flex items-center gap-2'>
        <StatusIcon className={cn('size-4 shrink-0', statusColour)} />
        <span className='text-sm font-medium'>{t('Last 30 days uptime')}</span>
      </div>

      <UptimeSparkline series={props.series} className='ml-auto' />

      <div className='flex items-center gap-3 text-xs'>
        <span className={cn('font-medium', statusColour)}>{statusLabel}</span>
        {summary.incidents > 0 && (
          <span className='text-muted-foreground'>
            {summary.incidents}{' '}
            {summary.incidents === 1 ? t('incident') : t('incidents')}
          </span>
        )}
        {summary.outage_minutes > 0 && (
          <span className='text-muted-foreground'>
            {summary.outage_minutes} {t('min downtime')}
          </span>
        )}
        <span className='text-muted-foreground hidden sm:inline'>
          {summary.days === 0
            ? '—'
            : `${formatUptimePct(summary.uptime_pct)} ${t('overall')}`}
        </span>
      </div>
    </div>
  )
}
