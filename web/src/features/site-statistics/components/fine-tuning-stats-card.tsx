import { useQuery } from '@tanstack/react-query'
import { CheckCircle2, Coins, Cpu, HardDrive, Zap } from 'lucide-react'
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

import { SITE_STATS_KPI_GRID_CLASS, SiteStatCard } from './stat-card'

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
        <div className={SITE_STATS_KPI_GRID_CLASS}>
          {Array.from({ length: 5 }, (_, i) => (
            <Skeleton key={i} className='h-28' />
          ))}
        </div>
        <div className='grid grid-cols-1 gap-4 xl:grid-cols-2'>
          <Skeleton className='h-80' />
          <Skeleton className='h-80' />
        </div>
      </div>
    )
  }

  if (isError || !data) {
    return null
  }

  const dailyHistory = data.daily_history ?? []
  const byAccelerator = data.by_accelerator ?? []
  const maxAccelJobs = Math.max(1, ...byAccelerator.map((r) => r.jobs))
  const successRate =
    data.total_jobs > 0
      ? `${Math.round((data.succeeded_jobs / data.total_jobs) * 100)}%`
      : '0%'

  return (
    <div className='space-y-4'>
      <div className={SITE_STATS_KPI_GRID_CLASS}>
        <SiteStatCard
          icon={<Cpu className='size-4 text-violet-500' />}
          label={t('Fine-Tuning Jobs')}
          value={formatNumber(data.total_jobs)}
          description={`${t('Succeeded')}: ${formatNumber(data.succeeded_jobs)} · ${t('Running')}: ${formatNumber(data.running_jobs)}`}
        />
        <SiteStatCard
          icon={<Coins className='size-4 text-yellow-500' />}
          label={t('Fine-Tuning Revenue')}
          value={formatCurrencyFromUSD(data.total_revenue_usd)}
        />
        <SiteStatCard
          icon={<Zap className='size-4 text-amber-500' />}
          label={t('Trained Tokens')}
          value={formatTokens(data.total_trained_tokens)}
        />
        <SiteStatCard
          icon={<CheckCircle2 className='size-4 text-emerald-500' />}
          label={t('Success Rate')}
          value={successRate}
          description={`${t('Failed')}: ${formatNumber(data.failed_jobs)}`}
        />
        <SiteStatCard
          icon={<HardDrive className='size-4 text-blue-500' />}
          label={t('Dataset Volume')}
          value={`${(data.total_dataset_mb || 0).toFixed(1)} MB`}
        />
      </div>

      <div className='grid grid-cols-1 gap-4 xl:grid-cols-2'>
        <Card>
          <CardHeader className='pb-2'>
            <CardTitle className='text-base'>
              {t('Last 30 Days Fine-Tuning Jobs')}
            </CardTitle>
            <CardDescription>{t('Traffic Over Time')}</CardDescription>
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
                      width={56}
                      tickFormatter={(v: number) => formatCompactNumber(v)}
                    />
                    <Tooltip
                      formatter={(val) => [
                        formatNumber(Number(val)),
                        t('Jobs'),
                      ]}
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
            <CardTitle className='text-base'>{t('Accelerator')}</CardTitle>
          </CardHeader>
          <CardContent className='overflow-x-auto'>
            {byAccelerator.length > 0 ? (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className='w-12'>#</TableHead>
                    <TableHead>{t('Accelerator')}</TableHead>
                    <TableHead className='w-44'>{t('Jobs')}</TableHead>
                    <TableHead className='w-28 text-right'>
                      {t('Tokens')}
                    </TableHead>
                    <TableHead className='w-28 text-right'>
                      {t('Revenue')}
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {byAccelerator.map((row, index) => (
                    <TableRow key={row.gpu}>
                      <TableCell className='text-muted-foreground tabular-nums'>
                        {index + 1}
                      </TableCell>
                      <TableCell>
                        <span className='max-w-56 truncate font-medium'>
                          {row.gpu}
                        </span>
                      </TableCell>
                      <TableCell>
                        <div className='flex items-center gap-2'>
                          <div className='bg-muted h-1.5 w-20 shrink-0 overflow-hidden rounded-full'>
                            <div
                              className='h-full rounded-full bg-blue-500'
                              style={{
                                width: `${(row.jobs / maxAccelJobs) * 100}%`,
                              }}
                            />
                          </div>
                          <span className='tabular-nums'>
                            {formatNumber(row.jobs)}
                          </span>
                        </div>
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
