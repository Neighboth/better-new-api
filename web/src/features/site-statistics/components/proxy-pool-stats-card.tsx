import { useQuery } from '@tanstack/react-query'
import { Activity, KeyRound, Network, Radio, Server } from 'lucide-react'
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
import { SITE_STATS_KPI_GRID_CLASS, SiteStatCard } from './stat-card'

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

  const proxies = data.proxies || []
  const dailyHistory = data.daily_history || []
  const totalMB = dailyHistory.reduce((acc, cur) => acc + (cur.mb || 0), 0)
  const totalRequests = dailyHistory.reduce(
    (acc, cur) => acc + (cur.requests || 0),
    0
  )
  const activeProxies = proxies.filter((p) => p.enabled)
  const totalAssignedKeys = proxies.reduce(
    (acc, cur) => acc + (cur.assigned_keys || 0),
    0
  )

  return (
    <div className='space-y-4'>
      <div className={SITE_STATS_KPI_GRID_CLASS}>
        <SiteStatCard
          icon={<Activity className='size-4 text-blue-500' />}
          label={t('Total MB Transferred')}
          value={`${totalMB.toFixed(2)} MB`}
        />
        <SiteStatCard
          icon={<Server className='size-4 text-emerald-500' />}
          label={t('Active Proxies')}
          value={`${formatNumber(activeProxies.length)} / ${formatNumber(proxies.length)}`}
        />
        <SiteStatCard
          icon={<KeyRound className='size-4 text-indigo-500' />}
          label={t('Assigned Keys')}
          value={formatNumber(totalAssignedKeys)}
        />
        <SiteStatCard
          icon={<Network className='size-4 text-cyan-500' />}
          label={t('Daily Average')}
          value={`${(totalMB / (dailyHistory.length || 1)).toFixed(2)} MB`}
        />
        <SiteStatCard
          icon={<Radio className='size-4 text-teal-500' />}
          label={t('Proxy Requests')}
          value={formatNumber(totalRequests)}
        />
      </div>

      <div className='grid grid-cols-1 gap-4 xl:grid-cols-2'>
        <Card>
          <CardHeader className='pb-2'>
            <CardTitle className='text-base'>
              {t('Proxy Pool Usage & Traffic')}
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
                      tickLine={false}
                      axisLine={false}
                      tick={{ fontSize: 11 }}
                      width={56}
                      tickFormatter={(v: number) => `${formatCompactNumber(v)}M`}
                    />
                    <Tooltip
                      formatter={(val) => [
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
          </CardHeader>
          <CardContent className='overflow-x-auto'>
            {proxies.length > 0 ? (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className='w-12'>#</TableHead>
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
                  {proxies.map((p, index) => (
                    <TableRow key={p.id}>
                      <TableCell className='text-muted-foreground tabular-nums'>
                        {index + 1}
                      </TableCell>
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
                      <TableCell className='text-right tabular-nums'>
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
