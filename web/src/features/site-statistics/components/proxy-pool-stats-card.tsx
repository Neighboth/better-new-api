import { useQuery } from '@tanstack/react-query'
import { Activity, KeyRound, Network, Server } from 'lucide-react'
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
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { formatCompactNumber, formatNumber } from '@/lib/format'

import { getProxyPoolStats } from '../api'

export function ProxyPoolStatsCard() {
  const { t } = useTranslation()

  const { data, isLoading, isError } = useQuery({
    queryKey: ['proxy-pool-stats'],
    queryFn: getProxyPoolStats,
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

  const proxies = data.proxies || []
  const dailyHistory = data.daily_history || []
  const totalMB = dailyHistory.reduce((acc, cur) => acc + (cur.mb || 0), 0)
  const activeProxies = proxies.filter((p) => p.enabled)
  const totalAssignedKeys = proxies.reduce(
    (acc, cur) => acc + (cur.assigned_keys || 0),
    0
  )

  return (
    <div className='space-y-4'>
      {/* KPI Stat Cards — unified with upper Site Statistics */}
      <div className='grid grid-cols-2 gap-3 lg:grid-cols-4'>
        <Card>
          <CardHeader className='pb-2'>
            <CardTitle className='text-muted-foreground flex items-center gap-2 text-sm font-medium'>
              <Activity className='size-4 shrink-0 text-blue-500' />
              <span className='truncate'>{t('Total MB Transferred')}</span>
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className='truncate text-2xl font-bold tabular-nums'>
              {totalMB.toFixed(2)} MB
            </div>
            <p className='text-muted-foreground mt-1 truncate text-xs'>
              {t('Proxy Pool Usage')}
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className='pb-2'>
            <CardTitle className='text-muted-foreground flex items-center gap-2 text-sm font-medium'>
              <Server className='size-4 shrink-0 text-emerald-500' />
              <span className='truncate'>{t('Active Proxies')}</span>
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className='truncate text-2xl font-bold tabular-nums'>
              {formatNumber(activeProxies.length)} / {formatNumber(proxies.length)}
            </div>
            <p className='text-muted-foreground mt-1 truncate text-xs'>
              {t('Proxies')}
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className='pb-2'>
            <CardTitle className='text-muted-foreground flex items-center gap-2 text-sm font-medium'>
              <KeyRound className='size-4 shrink-0 text-indigo-500' />
              <span className='truncate'>{t('Assigned Keys')}</span>
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className='truncate text-2xl font-bold tabular-nums'>
              {formatNumber(totalAssignedKeys)}
            </div>
            <p className='text-muted-foreground mt-1 truncate text-xs'>
              {t('Proxy Pool Usage')}
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className='pb-2'>
            <CardTitle className='text-muted-foreground flex items-center gap-2 text-sm font-medium'>
              <Network className='size-4 shrink-0 text-cyan-500' />
              <span className='truncate'>{t('Daily Average')}</span>
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className='truncate text-2xl font-bold tabular-nums'>
              {(totalMB / (dailyHistory.length || 1)).toFixed(2)} MB
            </div>
            <p className='text-muted-foreground mt-1 truncate text-xs'>
              {t('Last 30 Days Data Transfer (MB)')}
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Chart + Proxies Table */}
      <div className='grid grid-cols-1 gap-4 xl:grid-cols-2'>
        <Card>
          <CardHeader className='pb-2'>
            <CardTitle className='text-base'>
              {t('Proxy Pool Usage & Traffic')}
            </CardTitle>
            <CardDescription>
              {t(
                'Last 30 days proxy data transfer (MB) and pool key assignment status.'
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
                      tickLine={false}
                      axisLine={false}
                      tick={{ fontSize: 11 }}
                      width={52}
                      tickFormatter={(v: number) => `${formatCompactNumber(v)}M`}
                    />
                    <Tooltip
                      formatter={(val: any) => [
                        `${Number(val).toFixed(2)} MB`,
                        t('Usage'),
                      ]}
                    />
                    <Area
                      type='monotone'
                      dataKey='mb'
                      name={t('Usage')}
                      stroke='#3b82f6'
                      fill='#3b82f6'
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
            <CardTitle className='text-base'>{t('Proxies')}</CardTitle>
            <CardDescription>
              {proxies.length} {t('Proxies')} · {totalAssignedKeys}{' '}
              {t('Assigned Keys')}
            </CardDescription>
          </CardHeader>
          <CardContent className='overflow-x-auto'>
            {proxies.length > 0 ? (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('Proxy')}</TableHead>
                    <TableHead>{t('Status')}</TableHead>
                    <TableHead className='text-right'>
                      {t('Latency / Speed')}
                    </TableHead>
                    <TableHead className='text-right'>
                      {t('Egress IP')}
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {proxies.map((p) => (
                    <TableRow key={p.id}>
                      <TableCell>
                        <div className='flex flex-col'>
                          <span className='max-w-44 truncate font-medium'>
                            {p.name || t('Unnamed')}
                          </span>
                          <span
                            className='text-muted-foreground max-w-44 truncate text-xs'
                            title={p.url}
                          >
                            {p.url}
                          </span>
                        </div>
                      </TableCell>
                      <TableCell>
                        <Badge
                          variant='outline'
                          className={
                            p.enabled
                              ? 'border-emerald-500/30 text-emerald-600 dark:text-emerald-400'
                              : 'text-muted-foreground'
                          }
                        >
                          {p.enabled ? t('Active') : t('Inactive')}
                        </Badge>
                      </TableCell>
                      <TableCell className='text-right tabular-nums'>
                        {p.latency_ms ? `${p.latency_ms} ms` : '-'}
                      </TableCell>
                      <TableCell className='text-muted-foreground text-right tabular-nums'>
                        {p.egress_ip || '-'}
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
