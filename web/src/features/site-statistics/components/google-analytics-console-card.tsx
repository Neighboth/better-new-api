import { useQuery } from '@tanstack/react-query'
import {
  Activity,
  ChevronLeft,
  ChevronRight,
  Clock,
  Eye,
  MousePointerClick,
  Search,
  TrendingUp,
  Users,
} from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
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

const CHANNEL_COLORS = ['#3b82f6', '#10b981', '#f59e0b', '#8b5cf6', '#ec4899']

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
      <div className='grid grid-cols-2 gap-3 lg:grid-cols-5'>
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
          <CardDescription>
            {t(
              'Configure Google Analytics GA4 and Search Console credentials in System Settings > SEO to enable search terms, rankings, and traffic analytics.'
            )}
          </CardDescription>
        </CardHeader>
      </Card>
    )
  }

  const ga = gaQuery.data
  const gsc = gscQuery.data

  return (
    <div className='space-y-4'>
      {/* Google Analytics 4 */}
      {ga?.configured && ga.summary && (
        <div className='space-y-4'>
          <div className='grid grid-cols-2 gap-3 md:grid-cols-3 lg:grid-cols-5'>
            <Card>
              <CardHeader className='pb-2'>
                <CardTitle className='text-muted-foreground flex items-center gap-2 text-sm font-medium'>
                  <Users className='size-4 shrink-0 text-blue-500' />
                  <span className='truncate'>{t('Active Users')}</span>
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className='truncate text-2xl font-bold tabular-nums'>
                  {formatNumber(ga.summary.active_users)}
                </div>
                <p className='text-muted-foreground mt-1 truncate text-xs'>
                  +{formatNumber(ga.summary.new_users)} {t('New')}
                </p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className='pb-2'>
                <CardTitle className='text-muted-foreground flex items-center gap-2 text-sm font-medium'>
                  <Activity className='size-4 shrink-0 text-indigo-500' />
                  <span className='truncate'>{t('Total Sessions')}</span>
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className='truncate text-2xl font-bold tabular-nums'>
                  {formatNumber(ga.summary.sessions)}
                </div>
                <p className='text-muted-foreground mt-1 truncate text-xs'>
                  GA4 {ga.property_id ? `#${ga.property_id}` : ga.ga_id}
                </p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className='pb-2'>
                <CardTitle className='text-muted-foreground flex items-center gap-2 text-sm font-medium'>
                  <Eye className='size-4 shrink-0 text-emerald-500' />
                  <span className='truncate'>{t('Page Views')}</span>
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className='truncate text-2xl font-bold tabular-nums'>
                  {formatNumber(ga.summary.page_views)}
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className='pb-2'>
                <CardTitle className='text-muted-foreground flex items-center gap-2 text-sm font-medium'>
                  <Clock className='size-4 shrink-0 text-amber-500' />
                  <span className='truncate'>{t('Avg. Session Duration')}</span>
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className='truncate text-2xl font-bold tabular-nums'>
                  {Math.floor(ga.summary.average_session_duration_s / 60)}m{' '}
                  {ga.summary.average_session_duration_s % 60}s
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className='pb-2'>
                <CardTitle className='text-muted-foreground flex items-center gap-2 text-sm font-medium'>
                  <TrendingUp className='size-4 shrink-0 text-violet-500' />
                  <span className='truncate'>{t('Bounce Rate')}</span>
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className='truncate text-2xl font-bold tabular-nums'>
                  {ga.summary.bounce_rate_percentage}%
                </div>
              </CardContent>
            </Card>
          </div>

          <div className='grid grid-cols-1 gap-4 xl:grid-cols-2'>
            <Card>
              <CardHeader>
                <CardTitle className='text-base'>
                  {t('Traffic Acquisition Channels')}
                </CardTitle>
                <CardDescription>
                  {t(
                    'Active users, session duration, bounce rate, and acquisition traffic channels.'
                  )}
                </CardDescription>
              </CardHeader>
              <CardContent className='overflow-x-auto'>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('Channel')}</TableHead>
                      <TableHead className='text-right'>{t('Users')}</TableHead>
                      <TableHead className='text-right'>{t('Share')}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {(ga.acquisition_channels || []).map((ch, idx) => (
                      <TableRow key={idx}>
                        <TableCell className='font-medium'>
                          <span className='inline-flex items-center gap-2'>
                            <span
                              className='size-2 rounded-full'
                              style={{
                                backgroundColor:
                                  CHANNEL_COLORS[idx % CHANNEL_COLORS.length],
                              }}
                            />
                            {ch.channel}
                          </span>
                        </TableCell>
                        <TableCell className='text-right tabular-nums'>
                          {formatNumber(ch.users)}
                        </TableCell>
                        <TableCell className='text-right tabular-nums'>
                          <Badge variant='outline'>{ch.percentage}%</Badge>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle className='text-base'>
                  {t('Top Landing Pages')}
                </CardTitle>
              </CardHeader>
              <CardContent className='overflow-x-auto'>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('Page')}</TableHead>
                      <TableHead className='text-right'>{t('Views')}</TableHead>
                      <TableHead className='text-right'>
                        {t('Avg. Duration')}
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {(ga.top_landing_pages || []).map((p, idx) => (
                      <TableRow key={idx}>
                        <TableCell className='max-w-56 truncate font-medium'>
                          {p.page}
                        </TableCell>
                        <TableCell className='text-right tabular-nums'>
                          {formatNumber(p.views)}
                        </TableCell>
                        <TableCell className='text-muted-foreground text-right tabular-nums'>
                          {p.avg_duration_s}s
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          </div>
        </div>
      )}

      {/* Google Search Console */}
      {gsc?.configured && gsc.summary && (
        <div className='space-y-4'>
          <div className='grid grid-cols-2 gap-3 lg:grid-cols-4'>
            <Card>
              <CardHeader className='pb-2'>
                <CardTitle className='text-muted-foreground flex items-center gap-2 text-sm font-medium'>
                  <MousePointerClick className='size-4 shrink-0 text-blue-500' />
                  <span className='truncate'>{t('Total Clicks')}</span>
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className='truncate text-2xl font-bold tabular-nums'>
                  {formatNumber(gsc.summary.total_clicks)}
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className='pb-2'>
                <CardTitle className='text-muted-foreground flex items-center gap-2 text-sm font-medium'>
                  <Eye className='size-4 shrink-0 text-emerald-500' />
                  <span className='truncate'>{t('Total Impressions')}</span>
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className='truncate text-2xl font-bold tabular-nums'>
                  {formatNumber(gsc.summary.total_impressions)}
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className='pb-2'>
                <CardTitle className='text-muted-foreground flex items-center gap-2 text-sm font-medium'>
                  <TrendingUp className='size-4 shrink-0 text-amber-500' />
                  <span className='truncate'>{t('Average CTR')}</span>
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className='truncate text-2xl font-bold tabular-nums'>
                  {gsc.summary.average_ctr}%
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className='pb-2'>
                <CardTitle className='text-muted-foreground flex items-center gap-2 text-sm font-medium'>
                  <Search className='size-4 shrink-0 text-violet-500' />
                  <span className='truncate'>{t('Average Position')}</span>
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className='truncate text-2xl font-bold tabular-nums'>
                  #{gsc.summary.average_position}
                </div>
              </CardContent>
            </Card>
          </div>

          <div className='grid grid-cols-1 gap-4 xl:grid-cols-2'>
            <Card>
              <CardHeader>
                <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
                  <div>
                    <CardTitle className='text-base'>
                      {t('Google Search Queries')}
                    </CardTitle>
                    <CardDescription>
                      {t(
                        'Exact search terms used by visitors finding your site via Google Search.'
                      )}
                    </CardDescription>
                  </div>
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
                        <TableCell className='font-medium'>{q.query}</TableCell>
                        <TableCell className='text-right tabular-nums'>
                          {formatNumber(q.clicks)}
                        </TableCell>
                        <TableCell className='text-muted-foreground text-right tabular-nums'>
                          {formatNumber(q.impressions)}
                        </TableCell>
                        <TableCell className='text-right tabular-nums'>
                          {q.ctr}%
                        </TableCell>
                        <TableCell className='text-right font-medium tabular-nums'>
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
                  {gsc.site_url && (
                    <CardDescription>{gsc.site_url}</CardDescription>
                  )}
                </CardHeader>
                <CardContent className='overflow-x-auto'>
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>{t('Landing Page URL')}</TableHead>
                        <TableHead className='text-right'>{t('Clicks')}</TableHead>
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
                          <TableCell className='max-w-56 truncate font-medium'>
                            {p.page}
                          </TableCell>
                          <TableCell className='text-right tabular-nums'>
                            {formatNumber(p.clicks)}
                          </TableCell>
                          <TableCell className='text-muted-foreground text-right tabular-nums'>
                            {formatNumber(p.impressions)}
                          </TableCell>
                          <TableCell className='text-right tabular-nums'>
                            {p.ctr}%
                          </TableCell>
                          <TableCell className='text-right font-medium tabular-nums'>
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
