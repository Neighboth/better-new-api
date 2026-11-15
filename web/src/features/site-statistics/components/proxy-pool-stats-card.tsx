import { useQuery } from '@tanstack/react-query'
import { CheckCircle2, CircleX, Gauge, Loader2, Network, Waypoints } from 'lucide-react'
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
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { formatTimestampRelative } from '@/lib/format'

import { getProxyPoolStats } from '../api'

export function ProxyPoolStatsCard() {
  const { t } = useTranslation()

  const { data, isLoading, isError } = useQuery({
    queryKey: ['proxy-pool-stats'],
    queryFn: getProxyPoolStats,
  })

  if (isLoading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className='flex items-center gap-2 text-base'>
            <Network className='size-4 text-primary' />
            {t('Proxy Havuzu Kullanımı')}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className='flex h-36 items-center justify-center text-muted-foreground text-sm'>
            <Loader2 className='mr-2 h-4 w-4 animate-spin' />
            {t('Proxy kullanım verileri yükleniyor...')}
          </div>
        </CardContent>
      </Card>
    )
  }

  if (isError || !data) {
    return null
  }

  const proxies = data.proxies || []
  const dailyHistory = data.daily_history || []
  const totalMB = dailyHistory.reduce((acc, cur) => acc + (cur.mb || 0), 0)
  const activeProxies = proxies.filter((p) => p.enabled)
  const totalAssignedKeys = proxies.reduce((acc, cur) => acc + (cur.assigned_keys || 0), 0)

  return (
    <Card>
      <CardHeader>
        <div className='flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2'>
          <div>
            <CardTitle className='flex items-center gap-2 text-base'>
              <Network className='size-4 text-primary' />
              {t('Proxy Havuzu Kullanımı & Trafik')}
            </CardTitle>
            <CardDescription className='mt-1 text-xs'>
              {t('Son 30 günlük proxy veri aktarımı (MB) ve havuzdaki proxylerin eşleme durumları.')}
            </CardDescription>
          </div>
          <Badge variant='outline' className='w-fit'>
            {proxies.length} {t('Proxy')} · {totalAssignedKeys} {t('Eşli Anahtar')}
          </Badge>
        </div>
      </CardHeader>
      <CardContent className='space-y-5'>
        {/* Metric Highlights */}
        <div className='grid grid-cols-2 gap-3 sm:grid-cols-4'>
          <div className='rounded-lg border bg-muted/10 p-3'>
            <div className='text-muted-foreground text-xs font-medium'>{t('Toplam MB Aktarımı')}</div>
            <div className='mt-1 text-xl font-bold font-mono text-primary'>
              {totalMB.toFixed(2)} MB
            </div>
          </div>
          <div className='rounded-lg border bg-muted/10 p-3'>
            <div className='text-muted-foreground text-xs font-medium'>{t('Aktif Proxyler')}</div>
            <div className='mt-1 text-xl font-bold font-mono text-emerald-600 dark:text-emerald-400'>
              {activeProxies.length} / {proxies.length}
            </div>
          </div>
          <div className='rounded-lg border bg-muted/10 p-3'>
            <div className='text-muted-foreground text-xs font-medium'>{t('Eşli Anahtarlar')}</div>
            <div className='mt-1 text-xl font-bold font-mono text-amber-600 dark:text-amber-400'>
              {totalAssignedKeys}
            </div>
          </div>
          <div className='rounded-lg border bg-muted/10 p-3'>
            <div className='text-muted-foreground text-xs font-medium'>{t('Günlük Ortalama')}</div>
            <div className='mt-1 text-xl font-bold font-mono'>
              {(totalMB / (dailyHistory.length || 1)).toFixed(2)} MB
            </div>
          </div>
        </div>

        {/* 30-Day MB Chart */}
        <div className='rounded-lg border p-4'>
          <div className='text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-3'>
            {t('Son 30 Günlük Veri Akışı (MB)')}
          </div>
          <div className='h-52 w-full'>
            <ResponsiveContainer width='100%' height='100%'>
              <AreaChart
                data={dailyHistory}
                margin={{ top: 5, right: 10, left: -20, bottom: 0 }}
              >
                <CartesianGrid strokeDasharray='3 3' className='stroke-muted' />
                <XAxis dataKey='date' tick={{ fontSize: 10 }} tickLine={false} />
                <YAxis tick={{ fontSize: 10 }} unit='M' tickLine={false} />
                <Tooltip
                  formatter={(val: any) => [`${Number(val).toFixed(2)} MB`, t('Kullanım')]}
                />
                <Area
                  type='monotone'
                  dataKey='mb'
                  stroke='#2563eb'
                  fill='#2563eb'
                  fillOpacity={0.15}
                  strokeWidth={2}
                />
              </AreaChart>
            </ResponsiveContainer>
          </div>
        </div>

        {/* Proxies Table */}
        {proxies.length > 0 && (
          <div className='rounded-lg border overflow-x-auto'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className='text-xs'>{t('Proxy')}</TableHead>
                  <TableHead className='text-xs'>{t('URL')}</TableHead>
                  <TableHead className='text-xs'>{t('Durum')}</TableHead>
                  <TableHead className='text-xs'>{t('Gecikme / Ping')}</TableHead>
                  <TableHead className='text-xs'>{t('Egress IP')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {proxies.map((p) => (
                  <TableRow key={p.id}>
                    <TableCell className='text-xs font-medium'>
                      <div className='flex items-center gap-2'>
                        <span>{p.name || t('Unnamed')}</span>
                        {p.assigned_keys !== undefined && p.assigned_keys > 0 && (
                          <Badge
                            variant='outline'
                            className='border-amber-300 bg-amber-100 text-amber-800 dark:border-amber-500/40 dark:bg-amber-500/10 dark:text-amber-300 font-medium text-[10px]'
                          >
                            {p.assigned_keys} / Eşli
                          </Badge>
                        )}
                      </div>
                    </TableCell>
                    <TableCell className='max-w-[200px] truncate font-mono text-[11px] text-muted-foreground' title={p.url}>
                      {p.url}
                    </TableCell>
                    <TableCell>
                      {p.enabled ? (
                        <Badge variant='outline' className='text-emerald-600 text-[10px]'>
                          {t('Aktif')}
                        </Badge>
                      ) : (
                        <Badge variant='secondary' className='text-[10px]'>
                          {t('Pasif')}
                        </Badge>
                      )}
                    </TableCell>
                    <TableCell className='text-xs font-mono'>
                      {p.latency_ms ? `${p.latency_ms} ms` : '-'}
                    </TableCell>
                    <TableCell className='text-xs font-mono text-muted-foreground'>
                      {p.egress_ip || '-'}
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
