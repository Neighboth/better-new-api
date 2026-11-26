import { useQuery } from '@tanstack/react-query'
import {
  Activity,
  ChevronLeft,
  ChevronRight,
  Clock,
  Eye,
  FileText,
  MousePointerClick,
  Search,
  TrendingUp,
  Users,
} from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
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

import {
  getGoogleAnalyticsReport,
  getGoogleSearchConsoleReport,
} from '../api'
import { SITE_STATS_KPI_GRID_CLASS, SiteStatCard } from './stat-card'

export function GoogleAnalyticsConsoleCard() {
  const { t } = useTranslation()
  const [queryPage, setQueryPage] = useState(1)
  const [queryFilter, setQueryFilter] = useState('')

  const gaQuery = useQuery({
    queryKey: ['google-analytics-report'],
    queryFn: getGoogleAnalyticsReport,
    staleTime: 60_000,
  })

  const gscQuery = useQuery({
    queryKey: ['google-search-console-report', queryPage, queryFilter],
    queryFn: () =>
      getGoogleSearchConsoleReport({
        page: queryPage,
        page_size: 6,
        query: queryFilter,
      }),
    staleTime: 60_000,
  })

  const isConfigured = gaQuery.data?.configured || gscQuery.data?.configured

  if (gaQuery.isLoading && gscQuery.isLoading) {
    return (
      <div className={SITE_STATS_KPI_GRID_CLASS}>
        {Array.from({ length: 5 }, (_, i) => (
          <Skeleton key={i} className='h-28' />
        ))}
      </div>
    )
  }

  if (!isConfigured) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className='text-base'>
            {t('Google Analytics & Search Console')}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <p className='text-muted-foreground text-sm'>
            {t(
              'Configure Google Analytics GA4 and Search Console credentials in System Settings > SEO to enable search terms, rankings, and traffic analytics.'
            )}
          </p>
        </CardContent>
      </Card>
    )
  }

  const ga = gaQuery.data
  const gsc = gscQuery.data
  const gaChannels = ga?.acquisition_channels || []
  const gaPages = ga?.top_landing_pages || []
  const maxChannelUsers = Math.max(1, ...gaChannels.map((c) => c.users))
  const maxLandingViews = Math.max(1, ...gaPages.map((p) => p.views))

  return (
    <div className='space-y-4'>
      {ga?.configured && ga.summary && (
        <div className='space-y-4'>
          <div className={SITE_STATS_KPI_GRID_CLASS}>
            <SiteStatCard
              icon={<Users className='size-4 text-blue-500' />}
              label={t('Active Users')}
              value={formatNumber(ga.summary.active_users)}
              description={`+${formatNumber(ga.summary.new_users)} ${t('New')}`}
            />
            <SiteStatCard
              icon={<Activity className='size-4 text-indigo-500' />}
              label={t('Total Sessions')}
              value={formatNumber(ga.summary.sessions)}
            />
            <SiteStatCard
              icon={<Eye className='size-4 text-emerald-500' />}
              label={t('Page Views')}
              value={formatNumber(ga.summary.page_views)}
            />
            <SiteStatCard
              icon={<Clock className='size-4 text-amber-500' />}
              label={t('Avg. Session Duration')}
              value={`${Math.floor(ga.summary.average_session_duration_s / 60)}m ${ga.summary.average_session_duration_s % 60}s`}
            />
            <SiteStatCard
              icon={<TrendingUp className='size-4 text-violet-500' />}
              label={t('Bounce Rate')}
              value={`${ga.summary.bounce_rate_percentage}%`}
            />
          </div>

          <div className='grid grid-cols-1 gap-4 xl:grid-cols-2'>
            <Card>
              <CardHeader>
                <CardTitle className='text-base'>
                  {t('Traffic Acquisition Channels')}
                </CardTitle>
              </CardHeader>
              <CardContent className='overflow-x-auto'>
                {gaChannels.length > 0 ? (
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead className='w-12'>#</TableHead>
                        <TableHead>{t('Channel')}</TableHead>
                        <TableHead className='w-44'>{t('Users')}</TableHead>
                        <TableHead className='w-28 text-right'>
                          {t('Share')}
                        </TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {gaChannels.map((ch, idx) => (
                        <TableRow key={idx}>
                          <TableCell className='text-muted-foreground tabular-nums'>
                            {idx + 1}
                          </TableCell>
                          <TableCell className='font-medium'>
                            {ch.channel}
                          </TableCell>
                          <TableCell>
                            <div className='flex items-center gap-2'>
                              <div className='bg-muted h-1.5 w-20 shrink-0 overflow-hidden rounded-full'>
                                <div
                                  className='h-full rounded-full bg-blue-500'
                                  style={{
                                    width: `${(ch.users / maxChannelUsers) * 100}%`,
                                  }}
                                />
                              </div>
                              <span className='tabular-nums'>
                                {formatNumber(ch.users)}
                              </span>
                            </div>
                          </TableCell>
                          <TableCell className='text-right tabular-nums'>
                            {ch.percentage}%
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
                  {t('Top Landing Pages')}
                </CardTitle>
              </CardHeader>
              <CardContent className='overflow-x-auto'>
                {gaPages.length > 0 ? (
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead className='w-12'>#</TableHead>
                        <TableHead>{t('Page')}</TableHead>
                        <TableHead className='w-44'>{t('Views')}</TableHead>
                        <TableHead className='w-28 text-right'>
                          {t('Avg. Duration')}
                        </TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {gaPages.map((p, idx) => (
                        <TableRow key={idx}>
                          <TableCell className='text-muted-foreground tabular-nums'>
                            {idx + 1}
                          </TableCell>
                          <TableCell className='max-w-56 truncate font-medium'>
                            {p.page}
                          </TableCell>
                          <TableCell>
                            <div className='flex items-center gap-2'>
                              <div className='bg-muted h-1.5 w-20 shrink-0 overflow-hidden rounded-full'>
                                <div
                                  className='h-full rounded-full bg-blue-500'
                                  style={{
                                    width: `${(p.views / maxLandingViews) * 100}%`,
                                  }}
                                />
                              </div>
                              <span className='tabular-nums'>
                                {formatNumber(p.views)}
                              </span>
                            </div>
                          </TableCell>
                          <TableCell className='text-right tabular-nums'>
                            {p.avg_duration_s}s
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
      )}

      {gsc?.configured && gsc.summary && (
        <div className='space-y-4'>
          <div className={SITE_STATS_KPI_GRID_CLASS}>
            <SiteStatCard
              icon={<MousePointerClick className='size-4 text-blue-500' />}
              label={t('Total Clicks')}
              value={formatNumber(gsc.summary.total_clicks)}
            />
            <SiteStatCard
              icon={<Eye className='size-4 text-emerald-500' />}
              label={t('Total Impressions')}
              value={formatNumber(gsc.summary.total_impressions)}
            />
            <SiteStatCard
              icon={<TrendingUp className='size-4 text-amber-500' />}
              label={t('Average CTR')}
              value={`${gsc.summary.average_ctr}%`}
            />
            <SiteStatCard
              icon={<Search className='size-4 text-violet-500' />}
              label={t('Average Position')}
              value={`#${gsc.summary.average_position}`}
            />
            <SiteStatCard
              icon={<FileText className='size-4 text-cyan-500' />}
              label={t('Google Search Queries')}
              value={formatNumber(gsc.queries?.total_count ?? 0)}
            />
          </div>

          <div className='grid grid-cols-1 gap-4 xl:grid-cols-2'>
            <Card>
              <CardHeader>
                <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
                  <CardTitle className='text-base'>
                    {t('Google Search Queries')}
                  </CardTitle>
                  <div className='w-full sm:w-56'>
                    <Input
                      placeholder={t('Filter search query...')}
                      value={queryFilter}
                      onChange={(e) => {
                        setQueryFilter(e.target.value)
                        setQueryPage(1)
                      }}
                    />
                  </div>
                </div>
              </CardHeader>
              <CardContent className='space-y-3 overflow-x-auto'>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead className='w-12'>#</TableHead>
                      <TableHead>{t('Search Query')}</TableHead>
                      <TableHead className='text-right'>{t('Clicks')}</TableHead>
                      <TableHead className='text-right'>
                        {t('Impressions')}
                      </TableHead>
                      <TableHead className='text-right'>{t('CTR')}</TableHead>
                      <TableHead className='text-right'>
                        {t('Ranking')}
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {(gsc.queries?.items || []).map((q, idx) => (
                      <TableRow key={idx}>
                        <TableCell className='text-muted-foreground tabular-nums'>
                          {(queryPage - 1) * 6 + idx + 1}
                        </TableCell>
                        <TableCell className='font-medium'>{q.query}</TableCell>
                        <TableCell className='text-right tabular-nums'>
                          {formatNumber(q.clicks)}
                        </TableCell>
                        <TableCell className='text-right tabular-nums'>
                          {formatNumber(q.impressions)}
                        </TableCell>
                        <TableCell className='text-right tabular-nums'>
                          {q.ctr}%
                        </TableCell>
                        <TableCell className='text-right tabular-nums'>
                          #{q.position}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>

                {gsc.queries && gsc.queries.total_pages > 1 && (
                  <div className='flex items-center justify-between border-t pt-3'>
                    <span className='text-muted-foreground text-xs'>
                      {t('Page {{page}} / {{total}} (Total {{count}} queries)', {
                        page: gsc.queries.page,
                        total: gsc.queries.total_pages,
                        count: gsc.queries.total_count,
                      })}
                    </span>
                    <div className='flex items-center gap-1.5'>
                      <Button
                        variant='outline'
                        size='icon'
                        className='h-8 w-8'
                        disabled={queryPage <= 1}
                        onClick={() => setQueryPage((p) => Math.max(1, p - 1))}
                      >
                        <ChevronLeft className='size-4' />
                      </Button>
                      <Button
                        variant='outline'
                        size='icon'
                        className='h-8 w-8'
                        disabled={queryPage >= gsc.queries.total_pages}
                        onClick={() => setQueryPage((p) => p + 1)}
                      >
                        <ChevronRight className='size-4' />
                      </Button>
                    </div>
                  </div>
                )}
              </CardContent>
            </Card>

            {(gsc.top_search_pages || []).length > 0 && (
              <Card>
                <CardHeader>
                  <CardTitle className='text-base'>
                    {t('Top Clicked Pages')}
                  </CardTitle>
                </CardHeader>
                <CardContent className='overflow-x-auto'>
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead className='w-12'>#</TableHead>
                        <TableHead>{t('Landing Page URL')}</TableHead>
                        <TableHead className='text-right'>
                          {t('Clicks')}
                        </TableHead>
                        <TableHead className='text-right'>
                          {t('Impressions')}
                        </TableHead>
                        <TableHead className='text-right'>{t('CTR')}</TableHead>
                        <TableHead className='text-right'>
                          {t('Position')}
                        </TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {(gsc.top_search_pages || []).map((p, idx) => (
                        <TableRow key={idx}>
                          <TableCell className='text-muted-foreground tabular-nums'>
                            {idx + 1}
                          </TableCell>
                          <TableCell className='max-w-56 truncate font-medium'>
                            {p.page}
                          </TableCell>
                          <TableCell className='text-right tabular-nums'>
                            {formatNumber(p.clicks)}
                          </TableCell>
                          <TableCell className='text-right tabular-nums'>
                            {formatNumber(p.impressions)}
                          </TableCell>
                          <TableCell className='text-right tabular-nums'>
                            {p.ctr}%
                          </TableCell>
                          <TableCell className='text-right tabular-nums'>
                            #{p.position}
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </CardContent>
              </Card>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
