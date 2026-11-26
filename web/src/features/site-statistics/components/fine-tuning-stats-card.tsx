import { useQuery } from '@tanstack/react-query'
import { Activity, CheckCircle2, Coins, Cpu, Zap } from 'lucide-react'
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

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
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
import { formatCompactNumber, formatNumber, formatTokens } from '@/lib/format'
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
    staleTime: 30_000,
  })

  if (isLoading) {
    return (
      <div className='space-y-4'>
        <div className='grid grid-cols-2 gap-3 lg:grid-cols-4'>
          {Array.from({ length: 4 }, (_, i) => (
            <Skeleton key={i} className='h-28' />
          ))}
        </div>
        <Skeleton className='h-72' />
      </div>
    )
  }

  if (isError || !data) {
    return null
  }

  const dailyHistory = data.daily_history ?? []
  const byAccelerator = data.by_accelerator ?? []
  const successRate =
    data.total_jobs > 0
      ? `${Math.round((data.succeeded_jobs / data.total_jobs) * 100)}%`
      : '0%'

  return (
    <div className='space-y-4'>
      {/* KPI Stat Cards — matching upper Site Statistics StatCard layout */}
      <div className='grid grid-cols-2 gap-3 lg:grid-cols-4'>
        <Card>
          <CardHeader className='pb-2'>
            <CardTitle className='text-muted-foreground flex items-center gap-2 text-sm font-medium'>
              <Cpu className='size-4 shrink-0 text-violet-500' />
              <span className='truncate'>{t('Total Jobs')}</span>
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className='truncate text-2xl font-bold tabular-nums'>
              {formatNumber(data.total_jobs)}
            </div>
            <p className='text-muted-foreground mt-1 truncate text-xs'>
              {formatNumber(data.succeeded_jobs)} {t('Succeeded')} ·{' '}
              {formatNumber(data.running_jobs)} {t('Running')}
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className='pb-2'>
            <CardTitle className='text-muted-foreground flex items-center gap-2 text-sm font-medium'>
              <Coins className='size-4 shrink-0 text-yellow-500' />
              <span className='truncate'>{t('Fine-Tuning Revenue')}</span>
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className='truncate text-2xl font-bold tabular-nums'>
              {formatCurrencyFromUSD(data.total_revenue_usd)}
            </div>
            <p className='text-muted-foreground mt-1 truncate text-xs'>
              {t('Fine-Tuning Statistics')}
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className='pb-2'>
            <CardTitle className='text-muted-foreground flex items-center gap-2 text-sm font-medium'>
              <Zap className='size-4 shrink-0 text-amber-500' />
              <span className='truncate'>{t('Trained Tokens')}</span>
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className='truncate text-2xl font-bold tabular-nums'>
              {formatTokens(data.total_trained_tokens)}
            </div>
            <p className='text-muted-foreground mt-1 truncate text-xs'>
              {t('Fine-Tuning Statistics')}
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className='pb-2'>
            <CardTitle className='text-muted-foreground flex items-center gap-2 text-sm font-medium'>
              <CheckCircle2 className='size-4 shrink-0 text-emerald-500' />
              <span className='truncate'>{t('Success Rate')}</span>
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className='truncate text-2xl font-bold tabular-nums'>
              {successRate}
            </div>
            <p className='text-muted-foreground mt-1 truncate text-xs'>
              <Activity className='mr-1 inline size-3' />
              {formatNumber(data.failed_jobs)} {t('Failed')}
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Chart + Accelerator Table — matching upper 2-column grid & card styling */}
      <div className='grid grid-cols-1 gap-4 xl:grid-cols-2'>
        <Card>
          <CardHeader className='pb-2'>
            <CardTitle className='text-base'>
              {t('Last 30 Days Fine-Tuning Jobs')}
            </CardTitle>
            <CardDescription>
              {t(
                'Last 30 days fine-tuning job volume, trained tokens, and accelerator revenue.'
              )}
            </CardDescription>
          </CardHeader>
          <CardContent>
            {dailyHistory.length > 0 ? (
              <div className='h-64'>
                <ResponsiveContainer width='100%' height='100%'>
                  <AreaChart data={dailyHistory}>
                    <CartesianGrid
                      strokeDasharray='3 3'
                      className='stroke-muted'
                    />
                    <XAxis
                      dataKey='date'
                      tickLine={false}
                      axisLine={false}
                      tick={{ fontSize: 11 }}
                      minTickGap={24}
                    />
                    <YAxis
                      allowDecimals={false}
                      tickLine={false}
                      axisLine={false}
                      tick={{ fontSize: 11 }}
                      width={44}
                      tickFormatter={(v: number) => formatCompactNumber(v)}
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
                      name={t('Jobs')}
                      stroke='#8b5cf6'
                      fill='#8b5cf6'
                      fillOpacity={0.25}
                    />
                  </AreaChart>
                </ResponsiveContainer>
              </div>
            ) : (
              <div className='text-muted-foreground flex h-40 items-center justify-center text-sm'>
                {t('No Data')}
              </div>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className='text-base'>
              {t('Fine-Tuning Statistics')} · {t('Accelerator')}
            </CardTitle>
            <CardDescription>
              {t(
                'Last 30 days fine-tuning job volume, trained tokens, and accelerator revenue.'
              )}
            </CardDescription>
          </CardHeader>
          <CardContent className='overflow-x-auto'>
            {byAccelerator.length > 0 ? (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('Accelerator')}</TableHead>
                    <TableHead className='text-right'>{t('Jobs')}</TableHead>
                    <TableHead className='text-right'>
                      {t('Trained Tokens')}
                    </TableHead>
                    <TableHead className='text-right'>{t('Revenue')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {byAccelerator.map((row) => (
                    <TableRow key={row.gpu}>
                      <TableCell className='font-medium'>{row.gpu}</TableCell>
                      <TableCell className='text-right tabular-nums'>
                        {formatNumber(row.jobs)}
                      </TableCell>
                      <TableCell className='text-right tabular-nums'>
                        {formatTokens(row.trained_tokens)}
                      </TableCell>
                      <TableCell className='text-right tabular-nums'>
                        {formatCurrencyFromUSD(row.revenue_usd)}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            ) : (
              <div className='text-muted-foreground py-8 text-center text-sm'>
                {t('No Data')}
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
