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
import { Activity, BarChart3, Coins, MessageSquareCode, WalletCards } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { IconBadge, type IconBadgeTone } from '@/components/ui/icon-badge'
import { Skeleton } from '@/components/ui/skeleton'
import { formatQuota } from '@/lib/format'
import { cn } from '@/lib/utils'

import type { UserWalletData } from '../types'

interface WalletStatsCardProps {
  user: UserWalletData | null
  loading?: boolean
  enabledBillingTypes?: {
    requests?: boolean
    tokens?: boolean
    subscription?: boolean
    wallet?: boolean
  }
}

export function WalletStatsCard(props: WalletStatsCardProps) {
  const { t } = useTranslation()
  if (props.loading) {
    return (
      <div className='grid grid-cols-2 sm:grid-cols-3 md:grid-cols-5 gap-px bg-border rounded-lg border overflow-hidden'>
        {['balance', 'requests_pool', 'tokens_pool', 'usage', 'requests'].map((key, idx) => (
          <div
            key={key}
            className={cn(
              'bg-card min-w-0 p-3 sm:p-4',
              idx === 4 ? 'col-span-2 sm:col-span-1' : ''
            )}
          >
            <Skeleton className='h-3.5 w-full' />
            <Skeleton className='mt-2 h-6 w-full sm:h-7' />
            <Skeleton className='mt-1.5 hidden h-3.5 w-24 md:block' />
          </div>
        ))}
      </div>
    )
  }

  const stats: {
    label: string
    value: string
    description: string
    icon: typeof WalletCards
    tone: IconBadgeTone
  }[] = [
    ...(props.enabledBillingTypes?.wallet !== false
      ? [
          {
            label: t('Current Balance'),
            value: formatQuota(
              (props.user?.quota ?? 0) + (props.user?.referral_display ?? 0)
            ),
            description:
              (props.user?.referral_display ?? 0) > 0 &&
              (props.user?.quota ?? 0) === 0
                ? t('Includes promotional referral credits')
                : t('Standard wallet quota'),
            icon: WalletCards,
            tone: 'success' as IconBadgeTone,
          },
        ]
      : []),
    ...(props.enabledBillingTypes?.requests !== false
      ? [
          {
            label: t('Requests Pool'),
            value: (props.user?.requests_balance ?? 0).toLocaleString(),
            description: t('Remaining API requests'),
            icon: MessageSquareCode,
            tone: 'chart-1' as IconBadgeTone,
          },
        ]
      : []),
    ...(props.enabledBillingTypes?.tokens !== false
      ? [
          {
            label: t('Tokens Pool'),
            value: (props.user?.tokens_balance ?? 0).toLocaleString(),
            description: t('Remaining token balance'),
            icon: Coins,
            tone: 'warning' as IconBadgeTone,
          },
        ]
      : []),
    {
      label: t('Total Usage'),
      value: formatQuota(props.user?.used_quota ?? 0),
      description: t('Total consumed quota'),
      icon: BarChart3,
      tone: 'info',
    },
    {
      label: t('API Requests'),
      value: (props.user?.request_count ?? 0).toLocaleString(),
      description: t('Total requests made'),
      icon: Activity,
      tone: 'chart-4',
    },
  ]

  return (
    <div className='grid grid-cols-2 sm:grid-cols-3 md:grid-cols-5 gap-px bg-border rounded-lg border overflow-hidden'>
      {stats.map((item, idx) => (
        <div
          key={item.label}
          className={cn(
            'bg-card min-w-0 p-3 sm:p-4.5 flex flex-col justify-between',
            idx === stats.length - 1 && stats.length % 2 === 1 ? 'col-span-2 sm:col-span-1' : ''
          )}
        >
          <div className='flex items-center gap-1.5 sm:gap-2.5'>
            <IconBadge tone={item.tone} size='stat'>
              <item.icon />
            </IconBadge>
            <div className='text-muted-foreground truncate text-[11px] font-medium tracking-wider uppercase sm:text-xs'>
              {item.label}
            </div>
          </div>

          <div className='text-foreground mt-1.5 font-mono text-sm sm:text-xl xl:text-2xl font-bold tracking-tight break-all tabular-nums sm:mt-2.5'>
            {item.value}
          </div>
          <div className='text-muted-foreground/60 mt-1 hidden text-xs md:block truncate'>
            {item.description}
          </div>
        </div>
      ))}
    </div>
  )
}
