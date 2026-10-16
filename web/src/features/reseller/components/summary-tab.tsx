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
import {
  Activity,
  CheckCircle2,
  Clock,
  Coins,
  MessageSquareCode,
  RefreshCw,
  Ticket,
  TrendingUp,
  Users,
  WalletCards,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { IconBadge } from '@/components/ui/icon-badge'
import { Skeleton } from '@/components/ui/skeleton'
import { formatQuota } from '@/lib/format'

import { fetchResellerSummary } from '../api'

export function SummaryTab() {
  const { t } = useTranslation()

  const {
    data: summary,
    isLoading,
    isRefetching,
    refetch,
  } = useQuery({
    queryKey: ['reseller-summary'],
    queryFn: fetchResellerSummary,
  })

  if (isLoading) {
    return (
      <div className='space-y-6'>
        <div className='grid gap-4 md:grid-cols-3'>
          {[1, 2, 3].map((i) => (
            <Card key={i}>
              <CardHeader className='pb-2'>
                <Skeleton className='h-4 w-32' />
              </CardHeader>
              <CardContent>
                <Skeleton className='h-8 w-24 mb-2' />
                <Skeleton className='h-3 w-40' />
              </CardContent>
            </Card>
          ))}
        </div>
        <div className='grid gap-4 md:grid-cols-4'>
          {[1, 2, 3, 4].map((i) => (
            <Card key={i}>
              <CardHeader className='pb-2'>
                <Skeleton className='h-4 w-28' />
              </CardHeader>
              <CardContent>
                <Skeleton className='h-7 w-20' />
              </CardContent>
            </Card>
          ))}
        </div>
      </div>
    )
  }

  const formatNumber = (num?: number) => {
    return (num ?? 0).toLocaleString()
  }

  const totalCodes = summary?.total_codes ?? 0
  const usedCodes = summary?.used_codes ?? 0
  const unusedCodes = summary?.unused_codes ?? 0
  const usageRate = totalCodes > 0 ? Math.round((usedCodes / totalCodes) * 100) : 0

  return (
    <div className='space-y-6'>
      <div className='flex items-center justify-between'>
        <div>
          <h2 className='text-lg font-semibold tracking-tight'>{t('Reseller Dashboard Overview')}</h2>
          <p className='text-sm text-muted-foreground'>
            {t('Overview of your distributed codes, remaining quotas, and active sub-users.')}
          </p>
        </div>
        <Button
          variant='outline'
          size='sm'
          onClick={() => refetch()}
          disabled={isRefetching}
          className='gap-2'
        >
          <RefreshCw className={`h-4 w-4 ${isRefetching ? 'animate-spin' : ''}`} />
          <span>{t('Refresh')}</span>
        </Button>
      </div>

      {/* Section 1: Reseller Remaining Balances */}
      <div>
        <h3 className='text-sm font-medium text-muted-foreground uppercase tracking-wider mb-3'>
          {t('Your Current Reseller Quotas')}
        </h3>
        <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-3'>
          <Card className='relative overflow-hidden border-primary/20 bg-gradient-to-br from-primary/5 via-background to-background'>
            <CardHeader className='flex flex-row items-center justify-between pb-2 space-y-0'>
              <CardTitle className='text-sm font-medium'>{t('Remaining Balance / Quota')}</CardTitle>
              <IconBadge tone='primary' size='sm'>
                <WalletCards className='h-4 w-4' />
              </IconBadge>
            </CardHeader>
            <CardContent>
              <div className='text-2xl font-bold tracking-tight'>
                {formatQuota(summary?.reseller_quota ?? 0)}
              </div>
              <p className='text-xs text-muted-foreground mt-1'>
                {t('Available balance you can distribute as codes')}
              </p>
            </CardContent>
          </Card>

          <Card className='relative overflow-hidden border-blue-500/20 bg-gradient-to-br from-blue-500/5 via-background to-background'>
            <CardHeader className='flex flex-row items-center justify-between pb-2 space-y-0'>
              <CardTitle className='text-sm font-medium'>{t('Remaining Request Rights')}</CardTitle>
              <IconBadge tone='info' size='sm'>
                <Activity className='h-4 w-4' />
              </IconBadge>
            </CardHeader>
            <CardContent>
              <div className='text-2xl font-bold tracking-tight'>
                {formatNumber(summary?.reseller_requests)} {t('requests')}
              </div>
              <p className='text-xs text-muted-foreground mt-1'>
                {t('Available pool for direct request count')}
              </p>
            </CardContent>
          </Card>

          <Card className='relative overflow-hidden border-amber-500/20 bg-gradient-to-br from-amber-500/5 via-background to-background'>
            <CardHeader className='flex flex-row items-center justify-between pb-2 space-y-0'>
              <CardTitle className='text-sm font-medium'>{t('Remaining Token Rights')}</CardTitle>
              <IconBadge tone='warning' size='sm'>
                <Coins className='h-4 w-4' />
              </IconBadge>
            </CardHeader>
            <CardContent>
              <div className='text-2xl font-bold tracking-tight'>
                {formatNumber(summary?.reseller_tokens)} {t('tokens')}
              </div>
              <p className='text-xs text-muted-foreground mt-1'>
                {t('Available pool for token-based usage')}
              </p>
            </CardContent>
          </Card>
        </div>
      </div>

      {/* Section 2: Codes and Sub-users */}
      <div>
        <h3 className='text-sm font-medium text-muted-foreground uppercase tracking-wider mb-3'>
          {t('Redemption Codes & Sub-users')}
        </h3>
        <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-4'>
          <Card>
            <CardHeader className='flex flex-row items-center justify-between pb-2 space-y-0'>
              <CardTitle className='text-sm font-medium'>{t('Total Codes Generated')}</CardTitle>
              <IconBadge tone='primary' size='sm'>
                <Ticket className='h-4 w-4' />
              </IconBadge>
            </CardHeader>
            <CardContent>
              <div className='text-2xl font-bold tracking-tight'>{formatNumber(totalCodes)}</div>
              <p className='text-xs text-muted-foreground mt-1'>
                {t('All generated redemption codes')}
              </p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className='flex flex-row items-center justify-between pb-2 space-y-0'>
              <CardTitle className='text-sm font-medium'>{t('Redeemed / Sold Codes')}</CardTitle>
              <IconBadge tone='success' size='sm'>
                <CheckCircle2 className='h-4 w-4' />
              </IconBadge>
            </CardHeader>
            <CardContent>
              <div className='text-2xl font-bold tracking-tight text-emerald-600 dark:text-emerald-400'>
                {formatNumber(usedCodes)}
              </div>
              <div className='flex items-center gap-1.5 text-xs text-muted-foreground mt-1'>
                <span className='font-medium text-foreground'>{usageRate}%</span>
                <span>{t('redemption rate')}</span>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className='flex flex-row items-center justify-between pb-2 space-y-0'>
              <CardTitle className='text-sm font-medium'>{t('Pending / Unused Codes')}</CardTitle>
              <IconBadge tone='info' size='sm'>
                <Clock className='h-4 w-4' />
              </IconBadge>
            </CardHeader>
            <CardContent>
              <div className='text-2xl font-bold tracking-tight text-sky-600 dark:text-sky-400'>
                {formatNumber(unusedCodes)}
              </div>
              <p className='text-xs text-muted-foreground mt-1'>
                {t('Ready for sale or redemption')}
              </p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className='flex flex-row items-center justify-between pb-2 space-y-0'>
              <CardTitle className='text-sm font-medium'>{t('Active Sub-Users')}</CardTitle>
              <IconBadge tone='chart-5' size='sm'>
                <Users className='h-4 w-4' />
              </IconBadge>
            </CardHeader>
            <CardContent>
              <div className='text-2xl font-bold tracking-tight text-violet-600 dark:text-violet-400'>
                {formatNumber(summary?.active_sub_users)}
              </div>
              <p className='text-xs text-muted-foreground mt-1'>
                {t('Users registered via your codes / panel')}
              </p>
            </CardContent>
          </Card>
        </div>
      </div>

      {/* Section 3: Distribution Stats */}
      <div>
        <h3 className='text-sm font-medium text-muted-foreground uppercase tracking-wider mb-3'>
          {t('Total Distributed Volume')}
        </h3>
        <div className='grid gap-4 sm:grid-cols-3'>
          <Card>
            <CardHeader className='flex flex-row items-center justify-between pb-2 space-y-0'>
              <CardTitle className='text-sm font-medium'>{t('Total Quota Distributed')}</CardTitle>
              <IconBadge tone='success' size='sm'>
                <TrendingUp className='h-4 w-4' />
              </IconBadge>
            </CardHeader>
            <CardContent>
              <div className='text-xl font-bold tracking-tight text-emerald-600 dark:text-emerald-400'>
                {formatQuota(summary?.total_quota_distributed ?? 0)}
              </div>
              <p className='text-xs text-muted-foreground mt-1'>
                {t('Distributed via quota-type codes')}
              </p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className='flex flex-row items-center justify-between pb-2 space-y-0'>
              <CardTitle className='text-sm font-medium'>{t('Total Requests Distributed')}</CardTitle>
              <IconBadge tone='info' size='sm'>
                <MessageSquareCode className='h-4 w-4' />
              </IconBadge>
            </CardHeader>
            <CardContent>
              <div className='text-xl font-bold tracking-tight text-blue-600 dark:text-blue-400'>
                {formatNumber(summary?.total_requests_distributed)}
              </div>
              <p className='text-xs text-muted-foreground mt-1'>
                {t('Distributed via request-type codes')}
              </p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className='flex flex-row items-center justify-between pb-2 space-y-0'>
              <CardTitle className='text-sm font-medium'>{t('Total Tokens Distributed')}</CardTitle>
              <IconBadge tone='warning' size='sm'>
                <Coins className='h-4 w-4' />
              </IconBadge>
            </CardHeader>
            <CardContent>
              <div className='text-xl font-bold tracking-tight text-amber-600 dark:text-amber-400'>
                {formatNumber(summary?.total_tokens_distributed)}
              </div>
              <p className='text-xs text-muted-foreground mt-1'>
                {t('Distributed via token-type codes')}
              </p>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  )
}
