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
import { useQuery } from '@tanstack/react-query'
import { Clock, Gauge } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { IconBadge } from '@/components/ui/icon-badge'
import { Progress } from '@/components/ui/progress'
import { api } from '@/lib/api'

interface RateLimitStatus {
  enabled: boolean
  group: string
  limit: number
  used: number
  remaining: number
  window_minutes: number
  reset_at: number
  reset_in_seconds?: number
}

function formatRemainingDuration(
  seconds: number,
  t: (key: string, opts?: Record<string, unknown>) => string
): string {
  const clamped = Math.max(0, Math.floor(seconds))
  const hours = Math.floor(clamped / 3600)
  const minutes = Math.floor((clamped % 3600) / 60)
  const secs = clamped % 60
  if (hours > 0) {
    return `${hours}${t('h')} ${minutes}${t('m')}`
  }
  if (minutes > 0) {
    return `${minutes}${t('m')} ${secs}${t('s')}`
  }
  return `${secs}${t('s')}`
}

export function GroupRateLimitCard() {
  const { t } = useTranslation()
  const [nowMs, setNowMs] = useState(() => Date.now())

  const { data, dataUpdatedAt, refetch } = useQuery({
    queryKey: ['user-rate-limit'],
    queryFn: async () => {
      const res = await api.get<{ success: boolean; data?: RateLimitStatus }>(
        '/api/user/rate_limit',
        { skipErrorHandler: true }
      )
      return res.data?.data ?? null
    },
    refetchInterval: 30 * 1000,
  })

  // Anchor countdown to server-computed reset_in_seconds to eliminate client-server clock skew
  const targetMs =
    data && typeof data.reset_in_seconds === 'number' && data.reset_in_seconds > 0
      ? dataUpdatedAt + data.reset_in_seconds * 1000
      : data?.reset_at
        ? data.reset_at * 1000
        : 0

  const secondsUntilReset =
    targetMs > 0 ? Math.max(0, Math.ceil((targetMs - nowMs) / 1000)) : 0

  useEffect(() => {
    if (targetMs <= 0) return
    setNowMs(Date.now())
    const id = window.setInterval(() => {
      const current = Date.now()
      setNowMs(current)
      if (current >= targetMs) {
        window.clearInterval(id)
        void refetch()
      }
    }, 1000)
    return () => window.clearInterval(id)
  }, [targetMs, refetch])

  if (!data || !data.enabled || data.limit <= 0) {
    return null
  }

  const percent = Math.min(100, Math.round((data.used / data.limit) * 100))

  return (
    <section className='bg-card overflow-hidden rounded-2xl border shadow-xs'>
      <div className='flex items-center gap-2 border-b px-4 py-3 sm:px-5'>
        <IconBadge tone='info' size='sm'>
          <Gauge />
        </IconBadge>
        <h3 className='text-sm font-semibold'>
          {t('Group Request Rate Limit')}
        </h3>
        <Badge variant='secondary' className='ml-auto text-xs font-medium'>
          {data.group || 'default'}
        </Badge>
      </div>
      <div className='space-y-3 p-4 sm:p-5'>
        <div className='flex flex-wrap items-baseline justify-between gap-2'>
          <div className='text-sm font-semibold tabular-nums'>
            {data.remaining.toLocaleString()} / {data.limit.toLocaleString()}{' '}
            <span className='text-muted-foreground font-normal'>
              {t('remaining')}
            </span>
          </div>
          <div className='text-muted-foreground flex items-center gap-1.5 text-xs tabular-nums'>
            <Clock className='h-3.5 w-3.5 shrink-0' />
            {secondsUntilReset > 0 ? (
              <span>
                {t('Resets in {{time}}', {
                  time: formatRemainingDuration(secondsUntilReset, t),
                })}
              </span>
            ) : (
              <span>
                {t('Window')}: {data.window_minutes} {t('min')}
              </span>
            )}
          </div>
        </div>
        <Progress value={percent} className='h-2' />
        <p className='text-muted-foreground text-xs'>
          {t('Used {{used}} of {{limit}} requests in the current window.', {
            used: data.used.toLocaleString(),
            limit: data.limit.toLocaleString(),
          })}
        </p>
      </div>
    </section>
  )
}
