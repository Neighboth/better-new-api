import { useQuery } from '@tanstack/react-query'
import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import {
  Area,
  AreaChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'

import { Badge } from '@/components/ui/badge'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { api } from '@/lib/api'
import { formatCurrencyFromUSD } from '@/lib/currency'
import { formatNumber, formatTokens } from '@/lib/format'
import { useSystemConfigStore } from '@/stores/system-config-store'

interface FineTuningStatsResponse {
  total_jobs: number
  succeeded_jobs: number
  running_jobs: number
  failed_jobs: number
  total_trained_tokens: number
  total_revenue_usd: number
  total_billed_quota: number
  total_dataset_mb: number
  daily_history: Array<{
    date: string
    jobs: number
    revenue_usd: number
    trained_tokens: number
  }>
  by_accelerator: Array<{
    gpu: string
    jobs: number
    revenue_usd: number
    trained_tokens: number
  }>
  by_task_type: Record<string, number>
}

export function FineTuningStatsCard() {
  const { t } = useTranslation()
  const currencyConfig = useSystemConfigStore((s) => s.config.currency)
  void currencyConfig

  const { data, isLoading, isError } = useQuery({
    queryKey: ['fine-tuning-site-stats'],
    queryFn: async () => {
      const res = await api.get<{
        success: boolean
        data?: FineTuningStatsResponse
      }>('/api/fine_tuning/admin/stats')
      return res.data?.data ?? null
    },
  })

  if (isLoading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className='text-base'>{t('Fine-Tuning Statistics')}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className='text-muted-foreground flex h-36 items-center justify-center text-sm'>
            <Loader2 className='mr-2 h-4 w-4 animate-spin' />
            {t('Loading fine-tuning statistics...')}
          </div>
        </CardContent>
      </Card>
    )
  }

  if (isError || !data) {
    return null
  }

  const dailyHistory = data.daily_history ?? []
  const byAccelerator = data.by_accelerator ?? []

  return (
    <Card>
      <CardHeader>
        <div className='flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
          <div>
            <CardTitle className='text-base'>
              {t('Fine-Tuning Statistics')}
            </CardTitle>
            <CardDescription className='mt-1 text-xs'>
              {t(
                'Last 30 days fine-tuning job volume, trained tokens, and accelerator revenue.'
              )}
            </CardDescription>
          </div>
          <Badge variant='outline' className='w-fit'>
            {formatNumber(data.succeeded_jobs)} {t('Succeeded')} ·{' '}
            {formatNumber(data.running_jobs)} {t('Running')}
          </Badge>
        </div>
      </CardHeader>
      <CardContent className='space-y-5'>
        {/* Metric Highlights */}
        <div className='grid grid-cols-2 gap-3 sm:grid-cols-4'>
          <div className='bg-card rounded-lg border p-3 shadow-xs'>
            <div className='text-muted-foreground text-xs font-medium'>
              {t('Total Jobs')}
            </div>
            <div className='text-foreground mt-1 font-mono text-xl font-bold'>
              {formatNumber(data.total_jobs)}
            </div>
          </div>
          <div className='bg-card rounded-lg border p-3 shadow-xs'>
            <div className='text-muted-foreground text-xs font-medium'>
              {t('Fine-Tuning Revenue')}
            </div>
            <div className='text-foreground mt-1 font-mono text-xl font-bold'>
              {formatCurrencyFromUSD(data.total_revenue_usd)}
            </div>
          </div>
          <div className='bg-card rounded-lg border p-3 shadow-xs'>
            <div className='text-muted-foreground text-xs font-medium'>
              {t('Trained Tokens')}
            </div>
            <div className='text-foreground mt-1 font-mono text-xl font-bold'>
              {formatTokens(data.total_trained_tokens)}
            </div>
          </div>
          <div className='bg-card rounded-lg border p-3 shadow-xs'>
            <div className='text-muted-foreground text-xs font-medium'>
              {t('Success Rate')}
            </div>
            <div className='text-foreground mt-1 font-mono text-xl font-bold'>
              {data.total_jobs > 0
                ? `${Math.round((data.succeeded_jobs / data.total_jobs) * 100)}%`
                : '0%'}
            </div>
          </div>
        </div>

        {/* 30-Day Jobs Chart */}
        <div className='rounded-lg border p-4'>
          <div className='text-muted-foreground mb-3 text-xs font-semibold tracking-wider uppercase'>
            {t('Last 30 Days Fine-Tuning Jobs')}
          </div>
          <div className='h-52 w-full'>
            <ResponsiveContainer width='100%' height='100%'>
              <AreaChart
                data={dailyHistory}
                margin={{ top: 5, right: 10, left: -20, bottom: 0 }}
              >
                <CartesianGrid strokeDasharray='3 3' className='stroke-muted' />
                <XAxis dataKey='date' tick={{ fontSize: 10 }} tickLine={false} />
                <YAxis
                  allowDecimals={false}
                  tick={{ fontSize: 10 }}
                  tickLine={false}
                />
                <Tooltip
                  formatter={(val: any, name: any) =>
                    name === 'revenue_usd'
                      ? [formatCurrencyFromUSD(Number(val)), t('Revenue')]
                      : [formatNumber(Number(val)), t('Jobs')]
                  }
                />
                <Area
                  type='monotone'
                  dataKey='jobs'
                  stroke='#8b5cf6'
                  fill='#8b5cf6'
                  fillOpacity={0.15}
                  strokeWidth={2}
                />
              </AreaChart>
            </ResponsiveContainer>
          </div>
        </div>

        {/* Accelerator Breakdown Table */}
        {byAccelerator.length > 0 && (
          <div className='overflow-x-auto rounded-lg border'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className='text-xs'>{t('Accelerator')}</TableHead>
                  <TableHead className='text-right text-xs'>{t('Jobs')}</TableHead>
                  <TableHead className='text-right text-xs'>
                    {t('Trained Tokens')}
                  </TableHead>
                  <TableHead className='text-right text-xs'>
                    {t('Revenue')}
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {byAccelerator.map((row) => (
                  <TableRow key={row.gpu}>
                    <TableCell className='font-mono text-xs font-medium'>
                      {row.gpu}
                    </TableCell>
                    <TableCell className='text-right font-mono text-xs tabular-nums'>
                      {formatNumber(row.jobs)}
                    </TableCell>
                    <TableCell className='text-right font-mono text-xs tabular-nums'>
                      {formatTokens(row.trained_tokens)}
                    </TableCell>
                    <TableCell className='text-right font-mono text-xs tabular-nums'>
                      {formatCurrencyFromUSD(row.revenue_usd)}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
