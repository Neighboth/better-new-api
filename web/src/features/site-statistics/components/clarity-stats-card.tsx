import { useQuery } from '@tanstack/react-query'
import {
  Globe,
  Laptop,
  Loader2,
  ShieldAlert,
  Sparkles,
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
          <CardTitle className='flex items-center gap-2 text-base'>
            <Sparkles className='size-4 text-primary' />
            {t('Microsoft Clarity Canlı Analiz')}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className='flex h-36 items-center justify-center text-muted-foreground text-sm'>
            <Loader2 className='mr-2 h-4 w-4 animate-spin' />
            {t('Clarity Live Insights verileri yükleniyor...')}
          </div>
        </CardContent>
      </Card>
    )
  }

  if (!data?.configured) {
    return (
      <Card className='border-dashed'>
        <CardHeader>
          <CardTitle className='flex items-center gap-2 text-base'>
            <Sparkles className='size-4 text-muted-foreground' />
            {t('Microsoft Clarity Analitiği')}
          </CardTitle>
          <CardDescription className='text-xs'>
            {t('System Settings > SEO bölümünden Microsoft Clarity Project ID ve API Token ekleyerek kullanıcı sürtünmeleri, öfkeli tıklamalar ve derinlik analitiğini aktif edebilirsiniz.')}
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
            <CardTitle className='flex items-center gap-2 text-base'>
              <Sparkles className='size-4 text-primary' />
              {t('Microsoft Clarity Kullanıcı Deneyimi & Sürtünme Analizi')}
            </CardTitle>
            <CardDescription className='mt-1 text-xs'>
              {t('Öfkeli tıklamalar (Rage Clicks), ölü tıklamalar (Dead Clicks), kaydırma derinliği ve JS hataları.')}
            </CardDescription>
          </div>
          <Badge variant='outline' className='w-fit text-xs'>
            {t('4 Saatlik Önbellek Korumalı')}
          </Badge>
        </div>
      </CardHeader>
      <CardContent className='space-y-6'>
        {/* KPI Summary Cards */}
        <div className='grid grid-cols-2 gap-3 sm:grid-cols-5'>
          <div className='rounded-lg border bg-muted/10 p-3'>
            <div className='text-muted-foreground text-xs font-medium'>{t('Toplam Oturum')}</div>
            <div className='mt-1 text-xl font-bold font-mono text-primary'>
              {summary.total_sessions.toLocaleString()}
            </div>
            <div className='text-muted-foreground text-[10px] mt-0.5'>
              {summary.distinct_users.toLocaleString()} {t('Tekil Kullanıcı')}
            </div>
          </div>

          <div className='rounded-lg border bg-muted/10 p-3'>
            <div className='text-muted-foreground text-xs font-medium'>{t('Ortalama Kaydırma')}</div>
            <div className='mt-1 text-xl font-bold font-mono text-emerald-600 dark:text-emerald-400'>
              %{summary.scroll_depth}
            </div>
            <div className='text-muted-foreground text-[10px] mt-0.5'>{t('Sayfa derinliği')}</div>
          </div>

          <div className='rounded-lg border bg-muted/10 p-3'>
            <div className='text-muted-foreground text-xs font-medium'>{t('Aktif Süre')}</div>
            <div className='mt-1 text-xl font-bold font-mono'>
              {summary.active_time_seconds}s
            </div>
            <div className='text-muted-foreground text-[10px] mt-0.5'>{summary.pages_per_session} {t('Sayfa/oturum')}</div>
          </div>

          <div className='rounded-lg border bg-muted/10 p-3'>
            <div className='text-muted-foreground text-xs font-medium'>{t('Öfkeli Tıklama (Rage)')}</div>
            <div className='mt-1 text-xl font-bold font-mono text-amber-600 dark:text-amber-400'>
              %{friction.rage_click_rate}
            </div>
            <div className='text-muted-foreground text-[10px] mt-0.5'>{friction.rage_clicks} {t('Olay')}</div>
          </div>

          <div className='rounded-lg border bg-muted/10 p-3'>
            <div className='text-muted-foreground text-xs font-medium'>{t('Ölü Tıklama (Dead)')}</div>
            <div className='mt-1 text-xl font-bold font-mono text-rose-600 dark:text-rose-400'>
              %{friction.dead_click_rate}
            </div>
            <div className='text-muted-foreground text-[10px] mt-0.5'>{friction.dead_clicks} {t('Olay')}</div>
          </div>
        </div>

        {/* Friction Details & Devices */}
        <div className='grid gap-4 md:grid-cols-2'>
          {/* Friction Breakdown */}
          <div className='rounded-lg border p-4 space-y-3'>
            <div className='text-xs font-semibold uppercase tracking-wider text-muted-foreground flex items-center gap-1.5'>
              <ShieldAlert className='size-3.5 text-amber-500' />
              {t('Kullanıcı Sürtünme ve Hata Göstergeleri')}
            </div>
            <div className='grid grid-cols-2 gap-2 text-xs'>
              <div className='p-2.5 rounded border bg-muted/20'>
                <div className='text-muted-foreground'>{t('Aşırı Kaydırma (Excessive)')}</div>
                <div className='text-sm font-bold mt-1 font-mono'>%{friction.excessive_scroll_rate}</div>
                <div className='text-[10px] text-muted-foreground'>{friction.excessive_scrolling} {t('oturum')}</div>
              </div>
              <div className='p-2.5 rounded border bg-muted/20'>
                <div className='text-muted-foreground'>{t('Hızlı Geri Dönüş (Quick Back)')}</div>
                <div className='text-sm font-bold mt-1 font-mono'>%{friction.quick_back_rate}</div>
                <div className='text-[10px] text-muted-foreground'>{friction.quick_backs} {t('oturum')}</div>
              </div>
              <div className='p-2.5 rounded border bg-muted/20 col-span-2 flex items-center justify-between'>
                <div>
                  <div className='text-muted-foreground'>{t('JavaScript İstemci Hataları')}</div>
                  <div className='text-[10px] text-muted-foreground'>{t('Tarayıcı konsoluna düşen istisnalar')}</div>
                </div>
                <Badge variant='outline' className='font-mono text-xs text-rose-600 border-rose-300'>
                  {friction.javascript_errors} {t('Hata')}
                </Badge>
              </div>
            </div>
          </div>

          {/* Device Distribution */}
          <div className='rounded-lg border p-4 space-y-3'>
            <div className='text-xs font-semibold uppercase tracking-wider text-muted-foreground flex items-center gap-1.5'>
              <Laptop className='size-3.5 text-primary' />
              {t('Cihaz Dağılımı')}
            </div>
            <div className='h-40 w-full'>
              <ResponsiveContainer width='100%' height='100%'>
                <BarChart data={devices} layout='vertical' margin={{ left: 10, right: 20 }}>
                  <CartesianGrid strokeDasharray='3 3' className='stroke-muted' />
                  <XAxis type='number' tick={{ fontSize: 10 }} unit='%' />
                  <YAxis type='category' dataKey='device' tick={{ fontSize: 11 }} />
                  <Tooltip formatter={(val: any) => [`%${val}`, t('Oran')]} />
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
              {t('En Çok Ziyaret Edilen Sayfalar')}
            </div>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className='text-xs'>{t('URL')}</TableHead>
                  <TableHead className='text-xs text-right'>{t('Oturum')}</TableHead>
                  <TableHead className='text-xs text-right'>{t('Kaydırma')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {topPages.map((page, idx) => (
                  <TableRow key={idx}>
                    <TableCell className='text-xs font-mono font-medium'>{page.url}</TableCell>
                    <TableCell className='text-xs font-mono text-right'>{page.sessions.toLocaleString()}</TableCell>
                    <TableCell className='text-xs font-mono text-right text-emerald-600'>%{page.scroll_depth}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>

          <div className='rounded-lg border p-4'>
            <div className='text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-3 flex items-center gap-1.5'>
              <Globe className='size-3.5 text-primary' />
              {t('Ülkelere Göre Ziyaretçi Dağılımı')}
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
