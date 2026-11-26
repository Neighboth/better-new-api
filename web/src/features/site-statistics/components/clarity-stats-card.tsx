import { useQuery } from '@tanstack/react-query'
import {
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

import { getClarityInsights } from '../api'
import { SITE_STATS_KPI_GRID_CLASS, SiteStatCard } from './stat-card'

export function ClarityStatsCard() {
  const { t } = useTranslation()

  const { data, isLoading } = useQuery({
    queryKey: ['clarity-insights'],
    queryFn: getClarityInsights,
    staleTime: 60_000,
  })

  if (isLoading) {
    return (
      <div className={SITE_STATS_KPI_GRID_CLASS}>
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
        </CardHeader>
        <CardContent>
          <p className='text-muted-foreground text-sm'>
            {t(
              'Add Microsoft Clarity Project ID and API Token in System Settings > SEO to enable user friction, rage clicks, and depth analytics.'
            )}
          </p>
        </CardContent>
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
  const maxPageSessions = Math.max(1, ...topPages.map((p) => p.sessions))
  const maxCountrySessions = Math.max(1, ...countries.map((c) => c.sessions))

  return (
    <div className='space-y-4'>
      <div className={SITE_STATS_KPI_GRID_CLASS}>
        <SiteStatCard
          icon={<Users className='size-4 text-blue-500' />}
          label={t('Total Sessions')}
          value={formatNumber(summary.total_sessions)}
          description={`${t('Distinct Users')}: ${formatNumber(summary.distinct_users)}`}
        />
        <SiteStatCard
          icon={<ScrollText className='size-4 text-emerald-500' />}
          label={t('Average Scroll Depth')}
          value={`${summary.scroll_depth}%`}
        />
        <SiteStatCard
          icon={<Clock className='size-4 text-indigo-500' />}
          label={t('Active Time')}
          value={`${summary.active_time_seconds}s`}
          description={`${summary.pages_per_session} ${t('Pages/session')}`}
        />
        <SiteStatCard
          icon={<MousePointerClick className='size-4 text-amber-500' />}
          label={t('Rage Click Rate')}
          value={`${friction.rage_click_rate}%`}
          description={`${formatNumber(friction.rage_clicks)} ${t('events')}`}
        />
        <SiteStatCard
          icon={<AlertTriangle className='size-4 text-red-500' />}
          label={t('Dead Click Rate')}
          value={`${friction.dead_click_rate}%`}
          description={`${formatNumber(friction.dead_clicks)} ${t('events')}`}
        />
      </div>

      <div className='grid grid-cols-1 gap-4 xl:grid-cols-2'>
        <Card>
          <CardHeader>
            <CardTitle className='text-base'>
              {t('User Friction & Error Indicators')}
            </CardTitle>
          </CardHeader>
          <CardContent className='overflow-x-auto'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('Metric')}</TableHead>
                  <TableHead className='text-right'>{t('Count')}</TableHead>
                  <TableHead className='w-28 text-right'>{t('Share')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                <TableRow>
                  <TableCell className='font-medium'>
                    {t('Excessive Scrolling')}
                  </TableCell>
                  <TableCell className='text-right tabular-nums'>
                    {formatNumber(friction.excessive_scrolling)}
                  </TableCell>
                  <TableCell className='text-right tabular-nums'>
                    {friction.excessive_scroll_rate}%
                  </TableCell>
                </TableRow>
                <TableRow>
                  <TableCell className='font-medium'>
                    {t('Quick Back')}
                  </TableCell>
                  <TableCell className='text-right tabular-nums'>
                    {formatNumber(friction.quick_backs)}
                  </TableCell>
                  <TableCell className='text-right tabular-nums'>
                    {friction.quick_back_rate}%
                  </TableCell>
                </TableRow>
                <TableRow>
                  <TableCell className='font-medium'>
                    {t('JavaScript Client Errors')}
                  </TableCell>
                  <TableCell className='text-right tabular-nums'>
                    {formatNumber(friction.javascript_errors)}
                  </TableCell>
                  <TableCell className='text-muted-foreground text-right tabular-nums'>
                    -
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
              {t('Microsoft Clarity Live Insights')}
            </CardDescription>
          </CardHeader>
          <CardContent>
            {devices.length > 0 ? (
              <div className='h-56'>
                <ResponsiveContainer width='100%' height='100%'>
                  <BarChart data={devices}>
                    <CartesianGrid
                      strokeDasharray='3 3'
                      className='stroke-muted'
                    />
                    <XAxis
                      dataKey='device'
                      tickLine={false}
                      axisLine={false}
                      tick={{ fontSize: 11 }}
                    />
                    <YAxis
                      tickLine={false}
                      axisLine={false}
                      tick={{ fontSize: 11 }}
                      width={44}
                      tickFormatter={(v: number) => `${formatCompactNumber(v)}%`}
                    />
                    <Tooltip
                      formatter={(val) => [`${val}%`, t('Share')]}
                    />
                    <Bar
                      dataKey='percentage'
                      name={t('Share')}
                      fill='#3b82f6'
                      radius={[3, 3, 0, 0]}
                    />
                  </BarChart>
                </ResponsiveContainer>
              </div>
            ) : (
              <div className='text-muted-foreground flex h-56 items-center justify-center text-sm'>
                {t('No Data')}
              </div>
            )}
          </CardContent>
        </Card>
      </div>

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
                    <TableHead className='w-12'>#</TableHead>
                    <TableHead>{t('URL')}</TableHead>
                    <TableHead className='w-44'>{t('Sessions')}</TableHead>
                    <TableHead className='w-28 text-right'>
                      {t('Scroll')}
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {topPages.map((page, idx) => (
                    <TableRow key={idx}>
                      <TableCell className='text-muted-foreground tabular-nums'>
                        {idx + 1}
                      </TableCell>
                      <TableCell className='max-w-56 truncate font-medium'>
                        {page.url}
                      </TableCell>
                      <TableCell>
                        <div className='flex items-center gap-2'>
                          <div className='bg-muted h-1.5 w-20 shrink-0 overflow-hidden rounded-full'>
                            <div
                              className='h-full rounded-full bg-blue-500'
                              style={{
                                width: `${(page.sessions / maxPageSessions) * 100}%`,
                              }}
                            />
                          </div>
                          <span className='tabular-nums'>
                            {formatNumber(page.sessions)}
                          </span>
                        </div>
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
                    <TableHead className='w-12'>#</TableHead>
                    <TableHead>{t('Country')}</TableHead>
                    <TableHead className='w-44'>{t('Sessions')}</TableHead>
                    <TableHead className='w-28 text-right'>
                      {t('Share')}
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {countries.map((c, idx) => (
                    <TableRow key={idx}>
                      <TableCell className='text-muted-foreground tabular-nums'>
                        {idx + 1}
                      </TableCell>
                      <TableCell className='font-medium'>{c.country}</TableCell>
                      <TableCell>
                        <div className='flex items-center gap-2'>
                          <div className='bg-muted h-1.5 w-20 shrink-0 overflow-hidden rounded-full'>
                            <div
                              className='h-full rounded-full bg-blue-500'
                              style={{
                                width: `${(c.sessions / maxCountrySessions) * 100}%`,
                              }}
                            />
                          </div>
                          <span className='tabular-nums'>
                            {formatNumber(c.sessions)}
                          </span>
                        </div>
                      </TableCell>
                      <TableCell className='text-right tabular-nums'>
                        {c.percentage}%
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
