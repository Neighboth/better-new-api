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
import { RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
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
      <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-3'>
        {[1, 2, 3, 4, 5].map((i) => (
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
            {t('Overview of your distributed codes and remaining quotas.')}
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

      <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-3'>
        <Card>
          <CardHeader className='pb-2'>
            <CardTitle className='text-sm font-medium text-muted-foreground'>
              {t('Remaining Balance / Quota')}
            </CardTitle>
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

        <Card>
          <CardHeader className='pb-2'>
            <CardTitle className='text-sm font-medium text-muted-foreground'>
              {t('Total Codes Generated')}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className='text-2xl font-bold tracking-tight'>{formatNumber(totalCodes)}</div>
            <p className='text-xs text-muted-foreground mt-1'>
              {t('All generated redemption codes')}
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className='pb-2'>
            <CardTitle className='text-sm font-medium text-muted-foreground'>
              {t('Redeemed / Sold Codes')}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className='text-2xl font-bold tracking-tight'>
              {formatNumber(usedCodes)}
            </div>
            <div className='flex items-center gap-1.5 text-xs text-muted-foreground mt-1'>
              <span className='font-medium text-foreground'>{usageRate}%</span>
              <span>{t('redemption rate')}</span>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className='pb-2'>
            <CardTitle className='text-sm font-medium text-muted-foreground'>
              {t('Pending / Unused Codes')}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className='text-2xl font-bold tracking-tight'>
              {formatNumber(unusedCodes)}
            </div>
            <p className='text-xs text-muted-foreground mt-1'>
              {t('Ready for sale or redemption')}
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className='pb-2'>
            <CardTitle className='text-sm font-medium text-muted-foreground'>
              {t('Total Quota Distributed')}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className='text-2xl font-bold tracking-tight'>
              {formatQuota(summary?.total_quota_distributed ?? 0)}
            </div>
            <p className='text-xs text-muted-foreground mt-1'>
              {t('Distributed via quota-type codes')}
            </p>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
