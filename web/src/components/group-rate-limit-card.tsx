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
import { Gauge } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
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
}

export function GroupRateLimitCard() {
  const { t } = useTranslation()

  const { data } = useQuery({
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

  if (!data || !data.enabled || data.limit <= 0) {
    return null
  }

  const percent = Math.min(100, Math.round((data.used / data.limit) * 100))

  return (
    <Card>
      <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
        <CardTitle className='flex items-center gap-2 text-sm font-medium'>
          <Gauge className='text-muted-foreground h-4 w-4' />
          {t('Group Request Rate Limit')}
        </CardTitle>
        <Badge variant='outline' className='font-mono text-xs'>
          {data.group || 'default'}
        </Badge>
      </CardHeader>
      <CardContent className='space-y-2'>
        <div className='flex items-baseline justify-between text-sm'>
          <span className='font-mono font-semibold'>
            {data.remaining} / {data.limit} {t('remaining')}
          </span>
          <span className='text-muted-foreground text-xs'>
            {t('Window')}: {data.window_minutes} {t('min')}
          </span>
        </div>
        <Progress value={percent} className='h-2' />
        <p className='text-muted-foreground text-xs'>
          {t('Used {{used}} of {{limit}} requests in the current window.', {
            used: data.used,
            limit: data.limit,
          })}
        </p>
      </CardContent>
    </Card>
  )
}
