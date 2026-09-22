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
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'

import type { TicketCategory, TicketPriority, TicketStatus } from '../types'

export function TicketStatusBadge({ status }: { status: TicketStatus | string }) {
  const { t } = useTranslation()

  switch (status) {
    case 'open':
      return (
        <Badge variant='outline' className='border-amber-500/30 bg-amber-500/10 text-amber-600 dark:text-amber-400'>
          {t('Open')}
        </Badge>
      )
    case 'answered':
      return (
        <Badge variant='outline' className='border-emerald-500/30 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'>
          {t('Answered')}
        </Badge>
      )
    case 'waiting_user':
      return (
        <Badge variant='outline' className='border-blue-500/30 bg-blue-500/10 text-blue-600 dark:text-blue-400'>
          {t('Waiting for User')}
        </Badge>
      )
    case 'closed':
      return (
        <Badge variant='secondary' className='text-muted-foreground'>
          {t('Closed')}
        </Badge>
      )
    default:
      return <Badge variant='outline'>{status}</Badge>
  }
}

export function TicketPriorityBadge({ priority }: { priority: TicketPriority | string }) {
  const { t } = useTranslation()

  switch (priority) {
    case 'urgent':
      return (
        <Badge variant='destructive' className='font-semibold'>
          {t('Urgent')}
        </Badge>
      )
    case 'high':
      return (
        <Badge variant='outline' className='border-red-500/30 bg-red-500/10 text-red-600 dark:text-red-400'>
          {t('High')}
        </Badge>
      )
    case 'normal':
      return (
        <Badge variant='outline' className='border-muted text-muted-foreground'>
          {t('Normal')}
        </Badge>
      )
    case 'low':
      return (
        <Badge variant='secondary' className='text-muted-foreground text-xs'>
          {t('Low')}
        </Badge>
      )
    default:
      return <Badge variant='outline'>{priority}</Badge>
  }
}

export function TicketCategoryBadge({ category }: { category: TicketCategory | string }) {
  const { t } = useTranslation()

  const labels: Record<string, string> = {
    billing: t('Billing'),
    technical: t('Technical'),
    account: t('Account'),
    feature_request: t('Feature Request'),
    other: t('Other'),
  }

  return (
    <span className='inline-flex items-center rounded-md bg-muted px-2 py-0.5 text-xs font-medium text-muted-foreground'>
      {labels[category] || category}
    </span>
  )
}
