import { useQuery } from '@tanstack/react-query'
import {
  ChevronLeft,
  ChevronRight,
  Loader2,
} from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

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
  })

  const gscQuery = useQuery({
    queryKey: ['google-search-console-report', queryPage, queryFilter],
    queryFn: () =>
      getGoogleSearchConsoleReport({
        page: queryPage,
        page_size: 6,
        query: queryFilter,
      }),
  })

  const isConfigured = gaQuery.data?.configured || gscQuery.data?.configured

  if (gaQuery.isLoading && gscQuery.isLoading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className='text-base'>
            {t('Google Analytics & Search Console Analitiği')}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className='flex h-36 items-center justify-center text-muted-foreground text-sm'>
            <Loader2 className='mr-2 h-4 w-4 animate-spin' />
            {t('Google Cloud Console analitik verileri yükleniyor...')}
          </div>
        </CardContent>
      </Card>
    )
  }

  if (!isConfigured) {
    return (
      <Card className='border-dashed'>
        <CardHeader>
          <CardTitle className='text-base'>
            {t('Google Analytics & Search Console')}
          </CardTitle>
          <CardDescription className='text-xs'>
            {t('System Settings > SEO sayfasından Google Analytics GA4 ve Search Console Cloud Console kimlik bilgilerini (API anahtarı veya servis hesabı) girerek organik arama terimleri, sıralamalar ve GA4 trafik analitiğini aktif edebilirsiniz.')}
          </CardDescription>
        </CardHeader>
      </Card>
    )
  }

  const ga = gaQuery.data
  const gsc = gscQuery.data

  return (
    <div className='space-y-6'>
      {/* ── Google Analytics 4 Section ── */}
      {ga?.configured && ga.summary && (
        <Card>
          <CardHeader>
            <div className='flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2'>
              <div>
                <CardTitle className='text-base'>
                  {t('Google Analytics 4 (GA4) Trafik ve Kullanıcı Analitiği')}
                </CardTitle>
                <CardDescription className='mt-1 text-xs'>
                  {t('Etkin kullanıcılar, oturum süresi, hemen çıkma oranı ve trafik kaynakları.')}
                </CardDescription>
              </div>
              <Badge variant='outline' className='w-fit text-xs font-mono'>
                GA4 {ga.property_id ? `#${ga.property_id}` : ga.ga_id}
              </Badge>
            </div>
          </CardHeader>
          <CardContent className='space-y-6'>
            {/* GA4 KPI Cards */}
            <div className='grid grid-cols-2 gap-3 sm:grid-cols-5'>
              <div className='rounded-lg border bg-muted/10 p-3'>
                <div className='text-muted-foreground text-xs font-medium'>{t('Aktif Kullanıcı')}</div>
                <div className='mt-1 text-xl font-bold font-mono text-foreground'>
                  {ga.summary.active_users.toLocaleString()}
                </div>
                <div className='text-muted-foreground text-[10px] mt-0.5'>
                  +{ga.summary.new_users.toLocaleString()} {t('Yeni')}
                </div>
              </div>
              <div className='rounded-lg border bg-muted/10 p-3'>
                <div className='text-muted-foreground text-xs font-medium'>{t('Toplam Oturum')}</div>
                <div className='mt-1 text-xl font-bold font-mono text-foreground'>
                  {ga.summary.sessions.toLocaleString()}
                </div>
              </div>
              <div className='rounded-lg border bg-muted/10 p-3'>
                <div className='text-muted-foreground text-xs font-medium'>{t('Sayfa Görüntüleme')}</div>
                <div className='mt-1 text-xl font-bold font-mono text-foreground'>
                  {ga.summary.page_views.toLocaleString()}
                </div>
              </div>
              <div className='rounded-lg border bg-muted/10 p-3'>
                <div className='text-muted-foreground text-xs font-medium'>{t('Ort. Oturum Süresi')}</div>
                <div className='mt-1 text-xl font-bold font-mono text-foreground'>
                  {Math.floor(ga.summary.average_session_duration_s / 60)}d {ga.summary.average_session_duration_s % 60}s
                </div>
              </div>
              <div className='rounded-lg border bg-muted/10 p-3'>
                <div className='text-muted-foreground text-xs font-medium'>{t('Hemen Çıkma Oranı')}</div>
                <div className='mt-1 text-xl font-bold font-mono text-foreground'>
                  %{ga.summary.bounce_rate_percentage}
                </div>
              </div>
            </div>

            {/* Channels & Top Landing Pages */}
            <div className='grid gap-4 md:grid-cols-2'>
              {/* Traffic Channels */}
              <div className='rounded-lg border p-4 space-y-3'>
                <div className='text-xs font-semibold uppercase tracking-wider text-muted-foreground'>
                  {t('Trafik Edinme Kanalları')}
                </div>
                <div className='space-y-2'>
                  {(ga.acquisition_channels || []).map((ch, idx) => (
                    <div key={idx} className='flex items-center justify-between text-xs py-1.5 border-b last:border-0'>
                      <div className='flex items-center gap-2'>
                        <span className='size-2 rounded-full' style={{ backgroundColor: CHANNEL_COLORS[idx % CHANNEL_COLORS.length] }} />
                        <span className='font-medium'>{ch.channel}</span>
                      </div>
                      <div className='flex items-center gap-3'>
                        <span className='font-mono text-muted-foreground'>{ch.users.toLocaleString()} {t('kullanıcı')}</span>
                        <Badge variant='outline' className='font-mono text-[10px]'>%{ch.percentage}</Badge>
                      </div>
                    </div>
                  ))}
                </div>
              </div>

              {/* Top Landing Pages */}
              <div className='rounded-lg border p-4 space-y-3'>
                <div className='text-xs font-semibold uppercase tracking-wider text-muted-foreground'>
                  {t('Popüler Açılış Sayfaları')}
                </div>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead className='text-xs'>{t('Sayfa')}</TableHead>
                      <TableHead className='text-xs text-right'>{t('Görüntüleme')}</TableHead>
                      <TableHead className='text-xs text-right'>{t('Ort. Süre')}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {(ga.top_landing_pages || []).map((p, idx) => (
                      <TableRow key={idx}>
                        <TableCell className='text-xs font-mono font-medium'>{p.page}</TableCell>
                        <TableCell className='text-xs font-mono text-right'>{p.views.toLocaleString()}</TableCell>
                        <TableCell className='text-xs font-mono text-right text-muted-foreground'>
                          {p.avg_duration_s}s
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            </div>
          </CardContent>
        </Card>
      )}

      {/* ── Google Search Console Section ── */}
      {gsc?.configured && gsc.summary && (
        <Card>
          <CardHeader>
            <div className='flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2'>
              <div>
                <CardTitle className='text-base'>
                  {t('Google Search Console Organik Arama Analitiği')}
                </CardTitle>
                <CardDescription className='mt-1 text-xs'>
                  {t('Arama terimleri, tıklama sayıları, gösterimler, tıklama oranı (TO/CTR) ve ortalama sıra.')}
                </CardDescription>
              </div>
              {gsc.site_url && (
                <Badge variant='outline' className='w-fit text-xs font-mono'>
                  {gsc.site_url}
                </Badge>
              )}
            </div>
          </CardHeader>
          <CardContent className='space-y-6'>
            {/* GSC KPI Cards */}
            <div className='grid grid-cols-2 gap-3 sm:grid-cols-4'>
              <div className='rounded-lg border bg-muted/10 p-3'>
                <div className='text-muted-foreground text-xs font-medium'>{t('Toplam Tıklama')}</div>
                <div className='mt-1 text-xl font-bold font-mono text-foreground'>
                  {gsc.summary.total_clicks.toLocaleString()}
                </div>
              </div>
              <div className='rounded-lg border bg-muted/10 p-3'>
                <div className='text-muted-foreground text-xs font-medium'>{t('Toplam Gösterim')}</div>
                <div className='mt-1 text-xl font-bold font-mono text-foreground'>
                  {gsc.summary.total_impressions.toLocaleString()}
                </div>
              </div>
              <div className='rounded-lg border bg-muted/10 p-3'>
                <div className='text-muted-foreground text-xs font-medium'>{t('Ortalama TO (CTR)')}</div>
                <div className='mt-1 text-xl font-bold font-mono text-foreground'>
                  %{gsc.summary.average_ctr}
                </div>
              </div>
              <div className='rounded-lg border bg-muted/10 p-3'>
                <div className='text-muted-foreground text-xs font-medium'>{t('Ortalama Sıra')}</div>
                <div className='mt-1 text-xl font-bold font-mono text-foreground'>
                  #{gsc.summary.average_position}
                </div>
              </div>
            </div>

            {/* Paginated Search Queries Table */}
            <div className='rounded-lg border p-4 space-y-4'>
              <div className='flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3'>
                <div>
                  <div className='text-xs font-semibold uppercase tracking-wider text-muted-foreground'>
                    {t('Google Arama Terimleri & Sorgular')}
                  </div>
                  <div className='text-[11px] text-muted-foreground mt-0.5'>
                    {t('Sitenize Google üzerinden gelen kullanıcıların aradığı tam kelimeler.')}
                  </div>
                </div>
                <div className='w-full sm:w-64'>
                  <Input
                    placeholder={t('Arama terimi filtrele...')}
                    value={queryFilter}
                    onChange={(e) => {
                      setQueryFilter(e.target.value)
                      setQueryPage(1)
                    }}
                    className='h-8 text-xs'
                  />
                </div>
              </div>

              <div className='overflow-x-auto'>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead className='text-xs'>{t('Arama Terimi (Sorgu)')}</TableHead>
                      <TableHead className='text-xs text-right'>{t('Tıklamalar')}</TableHead>
                      <TableHead className='text-xs text-right'>{t('Gösterimler')}</TableHead>
                      <TableHead className='text-xs text-right'>{t('TO (CTR)')}</TableHead>
                      <TableHead className='text-xs text-right'>{t('Sıralama')}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {(gsc.queries?.items || []).map((q, idx) => (
                      <TableRow key={idx}>
                        <TableCell className='text-xs font-medium font-mono'>{q.query}</TableCell>
                        <TableCell className='text-xs font-mono text-right font-semibold'>
                          {q.clicks.toLocaleString()}
                        </TableCell>
                        <TableCell className='text-xs font-mono text-right text-muted-foreground'>
                          {q.impressions.toLocaleString()}
                        </TableCell>
                        <TableCell className='text-xs font-mono text-right'>
                          %{q.ctr}
                        </TableCell>
                        <TableCell className='text-xs font-mono text-right font-bold'>
                          #{q.position}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>

              {/* Pagination Controls */}
              {gsc.queries && gsc.queries.total_pages > 1 && (
                <div className='flex items-center justify-between pt-2 border-t'>
                  <span className='text-xs text-muted-foreground'>
                    {t('Sayfa {{page}} / {{total}} (Toplam {{count}} arama terimi)', {
                      page: gsc.queries.page,
                      total: gsc.queries.total_pages,
                      count: gsc.queries.total_count,
                    })}
                  </span>
                  <div className='flex items-center gap-1.5'>
                    <Button
                      variant='outline'
                      size='sm'
                      className='h-7 w-7 p-0'
                      disabled={queryPage <= 1}
                      onClick={() => setQueryPage((p) => Math.max(1, p - 1))}
                    >
                      <ChevronLeft className='size-3.5' />
                    </Button>
                    <Button
                      variant='outline'
                      size='sm'
                      className='h-7 w-7 p-0'
                      disabled={queryPage >= gsc.queries.total_pages}
                      onClick={() => setQueryPage((p) => p + 1)}
                    >
                      <ChevronRight className='size-3.5' />
                    </Button>
                  </div>
                </div>
              )}
            </div>

            {/* Top Search Pages */}
            {(gsc.top_search_pages || []).length > 0 && (
              <div className='rounded-lg border p-4 space-y-3'>
                <div className='text-xs font-semibold uppercase tracking-wider text-muted-foreground'>
                  {t('En Çok Tıklanan Sayfalar')}
                </div>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead className='text-xs'>{t('Açılış Sayfası URL')}</TableHead>
                      <TableHead className='text-xs text-right'>{t('Tıklamalar')}</TableHead>
                      <TableHead className='text-xs text-right'>{t('Gösterimler')}</TableHead>
                      <TableHead className='text-xs text-right'>{t('TO')}</TableHead>
                      <TableHead className='text-xs text-right'>{t('Sıra')}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {(gsc.top_search_pages || []).map((p, idx) => (
                      <TableRow key={idx}>
                        <TableCell className='text-xs font-mono font-medium'>{p.page}</TableCell>
                        <TableCell className='text-xs font-mono text-right font-semibold'>
                          {p.clicks.toLocaleString()}
                        </TableCell>
                        <TableCell className='text-xs font-mono text-right text-muted-foreground'>
                          {p.impressions.toLocaleString()}
                        </TableCell>
                        <TableCell className='text-xs font-mono text-right'>%{p.ctr}</TableCell>
                        <TableCell className='text-xs font-mono text-right font-bold'>#{p.position}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            )}
          </CardContent>
        </Card>
      )}
    </div>
  )
}
