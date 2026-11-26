import { useQuery } from '@tanstack/react-query'
import {
  Activity,
  AlertTriangle,
  Clock,
  MousePointerClick,
  ScrollText,
  Users,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import {
  Bar,
  BarChart,
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
import { formatNumber } from '@/lib/format'

import { getClarityInsights } from '../api'

export function ClarityStatsCard() {
  const { t } = useTranslation()

  const { data, isLoading } = useQuery({
    queryKey: ['clarity-insights'],
    queryFn: getClarityInsights,
    staleTime: 60_000,
  })

  if (isLoading) {
    return (
      <div className='grid grid-cols-2 gap-3 lg:grid-cols-5'>
        {Array.from({ length: 5 }, (_, i) => (
          <Skeleton key={i} className='h-28' />
        ))}
      </div>
    )
  }

  if (!data?.configured) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className='text-base'>
            {t('Microsoft Clarity Analytics')}
          </CardTitle>
          <CardDescription>
            {t(
              'Add Microsoft Clarity Project ID and API Token in System Settings > SEO to enable user friction, rage clicks, and depth analytics.'
            )}
          </CardDescription>
        </CardHeader>
      </Card>
    )
  }

  const summary = data.summary || {
    total_sessions: 0,
    distinct_users: 0,
    pages_per_session: 0,
    scroll_depth: 0,
    active_time_seconds: 0,
  }

  const friction = data.friction || {
    rage_clicks: 0,
    rage_click_rate: 0,
    dead_clicks: 0,
    dead_click_rate: 0,
    excessive_scrolling: 0,
    excessive_scroll_rate: 0,
    quick_backs: 0,
    quick_back_rate: 0,
    javascript_errors: 0,
  }

  const devices = data.devices || []
  const topPages = data.top_pages || []
  const countries = data.countries || []

  return (
    <div className='space-y-4'>
      {/* KPI Stat Cards — unified with upper Site Statistics */}
      <div className='grid grid-cols-2 gap-3 md:grid-cols-3 lg:grid-cols-5'>
        <Card>
          <CardHeader className='pb-2'>
            <CardTitle className='text-muted-foreground flex items-center gap-2 text-sm font-medium'>
              <Users className='size-4 shrink-0 text-blue-500' />
              <span className='truncate'>{t('Total Sessions')}</span>
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className='truncate text-2xl font-bold tabular-nums'>
              {formatNumber(summary.total_sessions)}
            </div>
            <p className='text-muted-foreground mt-1 truncate text-xs'>
              {formatNumber(summary.distinct_users)} {t('Distinct Users')}
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className='pb-2'>
            <CardTitle className='text-muted-foreground flex items-center gap-2 text-sm font-medium'>
              <ScrollText className='size-4 shrink-0 text-emerald-500' />
              <span className='truncate'>{t('Average Scroll Depth')}</span>
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className='truncate text-2xl font-bold tabular-nums'>
              {summary.scroll_depth}%
            </div>
            <p className='text-muted-foreground mt-1 truncate text-xs'>
              {t('Page depth')}
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className='pb-2'>
            <CardTitle className='text-muted-foreground flex items-center gap-2 text-sm font-medium'>
              <Clock className='size-4 shrink-0 text-indigo-500' />
              <span className='truncate'>{t('Active Time')}</span>
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className='truncate text-2xl font-bold tabular-nums'>
              {summary.active_time_seconds}s
            </div>
            <p className='text-muted-foreground mt-1 truncate text-xs'>
              {summary.pages_per_session} {t('Pages/session')}
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className='pb-2'>
            <CardTitle className='text-muted-foreground flex items-center gap-2 text-sm font-medium'>
              <MousePointerClick className='size-4 shrink-0 text-amber-500' />
              <span className='truncate'>{t('Rage Click Rate')}</span>
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className='truncate text-2xl font-bold tabular-nums'>
              {friction.rage_click_rate}%
            </div>
            <p className='text-muted-foreground mt-1 truncate text-xs'>
              {formatNumber(friction.rage_clicks)} {t('events')}
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className='pb-2'>
            <CardTitle className='text-muted-foreground flex items-center gap-2 text-sm font-medium'>
              <AlertTriangle className='size-4 shrink-0 text-red-500' />
              <span className='truncate'>{t('Dead Click Rate')}</span>
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className='truncate text-2xl font-bold tabular-nums'>
              {friction.dead_click_rate}%
            </div>
            <p className='text-muted-foreground mt-1 truncate text-xs'>
              {formatNumber(friction.dead_clicks)} {t('events')}
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Friction + Devices */}
      <div className='grid grid-cols-1 gap-4 xl:grid-cols-2'>
        <Card>
          <CardHeader>
            <CardTitle className='text-base'>
              {t('User Friction & Error Indicators')}
            </CardTitle>
            <CardDescription>
              {t(
                'Rage clicks, dead clicks, scroll depth, and JavaScript console errors.'
              )}
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableBody>
                <TableRow>
                  <TableCell className='font-medium'>
                    {t('Excessive Scrolling')}
                  </TableCell>
                  <TableCell className='text-muted-foreground text-right tabular-nums'>
                    {formatNumber(friction.excessive_scrolling)} {t('sessions')}
                  </TableCell>
                  <TableCell className='text-right font-medium tabular-nums'>
                    {friction.excessive_scroll_rate}%
                  </TableCell>
                </TableRow>
                <TableRow>
                  <TableCell className='font-medium'>
                    {t('Quick Back')}
                  </TableCell>
                  <TableCell className='text-muted-foreground text-right tabular-nums'>
                    {formatNumber(friction.quick_backs)} {t('sessions')}
                  </TableCell>
                  <TableCell className='text-right font-medium tabular-nums'>
                    {friction.quick_back_rate}%
                  </TableCell>
                </TableRow>
                <TableRow>
                  <TableCell className='font-medium'>
                    <Activity className='mr-1.5 inline size-4 text-red-500' />
                    {t('JavaScript Client Errors')}
                  </TableCell>
                  <TableCell className='text-muted-foreground text-right text-xs'>
                    {t('Exceptions logged to browser console')}
                  </TableCell>
                  <TableCell className='text-right tabular-nums'>
                    <Badge variant='outline'>
                      {formatNumber(friction.javascript_errors)} {t('Errors')}
                    </Badge>
                  </TableCell>
                </TableRow>
              </TableBody>
            </Table>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className='pb-2'>
            <CardTitle className='text-base'>
              {t('Device Distribution')}
            </CardTitle>
            <CardDescription>
              {t('Microsoft Clarity Live Insights')} ·{' '}
              {t('4-Hour Cache Protected')}
            </CardDescription>
          </CardHeader>
          <CardContent>
            {devices.length > 0 ? (
              <div className='h-56'>
                <ResponsiveContainer width='100%' height='100%'>
                  <BarChart
                    data={devices}
                    layout='vertical'
                    margin={{ left: 10, right: 20 }}
                  >
                    <CartesianGrid
                      strokeDasharray='3 3'
                      className='stroke-muted'
                    />
                    <XAxis
                      type='number'
                      tickLine={false}
                      axisLine={false}
                      tick={{ fontSize: 11 }}
                      unit='%'
                    />
                    <YAxis
                      type='category'
                      dataKey='device'
                      tickLine={false}
                      axisLine={false}
                      tick={{ fontSize: 11 }}
                    />
                    <Tooltip
                      formatter={(val: any) => [`${val}%`, t('Share')]}
                    />
                    <Bar dataKey='percentage' fill='#3b82f6' radius={[0, 3, 3, 0]} />
                  </BarChart>
                </ResponsiveContainer>
              </div>
            ) : (
              <div className='text-muted-foreground flex h-40 items-center justify-center text-sm'>
                {t('No Data')}
              </div>
            )}
          </CardContent>
        </Card>
      </div>

      {/* Top Pages + Countries */}
      <div className='grid grid-cols-1 gap-4 xl:grid-cols-2'>
        <Card>
          <CardHeader>
            <CardTitle className='text-base'>{t('Top Visited Pages')}</CardTitle>
          </CardHeader>
          <CardContent className='overflow-x-auto'>
            {topPages.length > 0 ? (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('URL')}</TableHead>
                    <TableHead className='text-right'>{t('Sessions')}</TableHead>
                    <TableHead className='text-right'>{t('Scroll')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {topPages.map((page, idx) => (
                    <TableRow key={idx}>
                      <TableCell className='max-w-56 truncate font-medium'>
                        {page.url}
                      </TableCell>
                      <TableCell className='text-right tabular-nums'>
                        {formatNumber(page.sessions)}
                      </TableCell>
                      <TableCell className='text-right tabular-nums'>
                        {page.scroll_depth}%
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

        <Card>
          <CardHeader>
            <CardTitle className='text-base'>
              {t('Visitor Distribution by Country')}
            </CardTitle>
          </CardHeader>
          <CardContent className='overflow-x-auto'>
            {countries.length > 0 ? (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('Country')}</TableHead>
                    <TableHead className='text-right'>{t('Sessions')}</TableHead>
                    <TableHead className='text-right'>{t('Share')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {countries.map((c, idx) => (
                    <TableRow key={idx}>
                      <TableCell className='font-medium'>{c.country}</TableCell>
                      <TableCell className='text-right tabular-nums'>
                        {formatNumber(c.sessions)}
                      </TableCell>
                      <TableCell className='text-right tabular-nums'>
                        <Badge variant='outline'>{c.percentage}%</Badge>
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
