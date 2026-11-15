import { useQuery } from '@tanstack/react-query'
import { Loader2 } from 'lucide-react'
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
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

import { getClarityInsights } from '../api'

export function ClarityStatsCard() {
  const { t } = useTranslation()

  const { data, isLoading } = useQuery({
    queryKey: ['clarity-insights'],
    queryFn: getClarityInsights,
  })

  if (isLoading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className='text-base'>
            {t('Microsoft Clarity Live Insights')}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className='flex h-36 items-center justify-center text-muted-foreground text-sm'>
            <Loader2 className='mr-2 h-4 w-4 animate-spin' />
            {t('Loading Clarity Live Insights data...')}
          </div>
        </CardContent>
      </Card>
    )
  }

  if (!data?.configured) {
    return (
      <Card className='border-dashed'>
        <CardHeader>
          <CardTitle className='text-base'>
            {t('Microsoft Clarity Analytics')}
          </CardTitle>
          <CardDescription className='text-xs'>
            {t('Add Microsoft Clarity Project ID and API Token in System Settings > SEO to enable user friction, rage clicks, and depth analytics.')}
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
    <Card>
      <CardHeader>
        <div className='flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2'>
          <div>
            <CardTitle className='text-base'>
              {t('Microsoft Clarity User Experience & Friction Analytics')}
            </CardTitle>
            <CardDescription className='mt-1 text-xs'>
              {t('Rage clicks, dead clicks, scroll depth, and JavaScript console errors.')}
            </CardDescription>
          </div>
          <Badge variant='outline' className='w-fit text-xs'>
            {t('4-Hour Cache Protected')}
          </Badge>
        </div>
      </CardHeader>
      <CardContent className='space-y-6'>
        {/* KPI Summary Cards */}
        <div className='grid grid-cols-2 gap-3 sm:grid-cols-5'>
          <div className='rounded-lg border bg-muted/10 p-3'>
            <div className='text-muted-foreground text-xs font-medium'>{t('Total Sessions')}</div>
            <div className='mt-1 text-xl font-bold font-mono text-foreground'>
              {summary.total_sessions.toLocaleString()}
            </div>
            <div className='text-muted-foreground text-[10px] mt-0.5'>
              {summary.distinct_users.toLocaleString()} {t('Distinct Users')}
            </div>
          </div>

          <div className='rounded-lg border bg-muted/10 p-3'>
            <div className='text-muted-foreground text-xs font-medium'>{t('Average Scroll Depth')}</div>
            <div className='mt-1 text-xl font-bold font-mono text-foreground'>
              %{summary.scroll_depth}
            </div>
            <div className='text-muted-foreground text-[10px] mt-0.5'>{t('Page depth')}</div>
          </div>

          <div className='rounded-lg border bg-muted/10 p-3'>
            <div className='text-muted-foreground text-xs font-medium'>{t('Active Time')}</div>
            <div className='mt-1 text-xl font-bold font-mono text-foreground'>
              {summary.active_time_seconds}s
            </div>
            <div className='text-muted-foreground text-[10px] mt-0.5'>{summary.pages_per_session} {t('Pages/session')}</div>
          </div>

          <div className='rounded-lg border bg-muted/10 p-3'>
            <div className='text-muted-foreground text-xs font-medium'>{t('Rage Click Rate')}</div>
            <div className='mt-1 text-xl font-bold font-mono text-foreground'>
              %{friction.rage_click_rate}
            </div>
            <div className='text-muted-foreground text-[10px] mt-0.5'>{friction.rage_clicks} {t('events')}</div>
          </div>

          <div className='rounded-lg border bg-muted/10 p-3'>
            <div className='text-muted-foreground text-xs font-medium'>{t('Dead Click Rate')}</div>
            <div className='mt-1 text-xl font-bold font-mono text-foreground'>
              %{friction.dead_click_rate}
            </div>
            <div className='text-muted-foreground text-[10px] mt-0.5'>{friction.dead_clicks} {t('events')}</div>
          </div>
        </div>

        {/* Friction Details & Devices */}
        <div className='grid gap-4 md:grid-cols-2'>
          {/* Friction Breakdown */}
          <div className='rounded-lg border p-4 space-y-3'>
            <div className='text-xs font-semibold uppercase tracking-wider text-muted-foreground'>
              {t('User Friction & Error Indicators')}
            </div>
            <div className='grid grid-cols-2 gap-2 text-xs'>
              <div className='p-2.5 rounded border bg-muted/20'>
                <div className='text-muted-foreground'>{t('Excessive Scrolling')}</div>
                <div className='text-sm font-bold mt-1 font-mono'>%{friction.excessive_scroll_rate}</div>
                <div className='text-[10px] text-muted-foreground'>{friction.excessive_scrolling} {t('sessions')}</div>
              </div>
              <div className='p-2.5 rounded border bg-muted/20'>
                <div className='text-muted-foreground'>{t('Quick Back')}</div>
                <div className='text-sm font-bold mt-1 font-mono'>%{friction.quick_back_rate}</div>
                <div className='text-[10px] text-muted-foreground'>{friction.quick_backs} {t('sessions')}</div>
              </div>
              <div className='p-2.5 rounded border bg-muted/20 col-span-2 flex items-center justify-between'>
                <div>
                  <div className='text-muted-foreground'>{t('JavaScript Client Errors')}</div>
                  <div className='text-[10px] text-muted-foreground'>{t('Exceptions logged to browser console')}</div>
                </div>
                <Badge variant='outline' className='font-mono text-xs'>
                  {friction.javascript_errors} {t('Errors')}
                </Badge>
              </div>
            </div>
          </div>

          {/* Device Distribution */}
          <div className='rounded-lg border p-4 space-y-3'>
            <div className='text-xs font-semibold uppercase tracking-wider text-muted-foreground'>
              {t('Device Distribution')}
            </div>
            <div className='h-40 w-full'>
              <ResponsiveContainer width='100%' height='100%'>
                <BarChart data={devices} layout='vertical' margin={{ left: 10, right: 20 }}>
                  <CartesianGrid strokeDasharray='3 3' className='stroke-muted' />
                  <XAxis type='number' tick={{ fontSize: 10 }} unit='%' />
                  <YAxis type='category' dataKey='device' tick={{ fontSize: 11 }} />
                  <Tooltip formatter={(val: any) => [`%${val}`, t('Share')]} />
                  <Bar dataKey='percentage' fill='#3b82f6' radius={[0, 4, 4, 0]} />
                </BarChart>
              </ResponsiveContainer>
            </div>
          </div>
        </div>

        {/* Top Pages & Browsers */}
        <div className='grid gap-4 md:grid-cols-2'>
          <div className='rounded-lg border p-4'>
            <div className='text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-3'>
              {t('Top Visited Pages')}
            </div>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className='text-xs'>{t('URL')}</TableHead>
                  <TableHead className='text-xs text-right'>{t('Sessions')}</TableHead>
                  <TableHead className='text-xs text-right'>{t('Scroll')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {topPages.map((page, idx) => (
                  <TableRow key={idx}>
                    <TableCell className='text-xs font-mono font-medium'>{page.url}</TableCell>
                    <TableCell className='text-xs font-mono text-right'>{page.sessions.toLocaleString()}</TableCell>
                    <TableCell className='text-xs font-mono text-right'>%{page.scroll_depth}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>

          <div className='rounded-lg border p-4'>
            <div className='text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-3'>
              {t('Visitor Distribution by Country')}
            </div>
            <div className='space-y-2'>
              {countries.map((c, idx) => (
                <div key={idx} className='flex items-center justify-between text-xs py-1 border-b last:border-0'>
                  <span className='font-medium'>{c.country}</span>
                  <div className='flex items-center gap-3'>
                    <span className='font-mono text-muted-foreground'>{c.sessions.toLocaleString()}</span>
                    <Badge variant='outline' className='text-[10px] font-mono'>%{c.percentage}</Badge>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
