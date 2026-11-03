/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import {
  Activity,
  BarChart3,
  Clock,
  Coins,
  Cpu,
  Filter,
  Layers,
  Monitor,
  RefreshCw,
  Search,
  Smartphone,
  Tablet,
  TrendingUp,
  Trophy,
  Users,
  Zap,
} from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  Area,
  AreaChart,
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  Legend,
  Line,
  LineChart,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import { toast } from 'sonner'

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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

import { getByogAdminStats } from '../../byog/api'
import type { ByogAdminStatsResponse } from '../../byog/types'

const PIE_COLORS = ['#3b82f6', '#10b981', '#f59e0b', '#8b5cf6', '#ec4899']

export function ByogStatsTab() {
  const { t } = useTranslation()
  const [loading, setLoading] = useState(true)
  const [stats, setStats] = useState<ByogAdminStatsResponse | null>(null)

  // Filters
  const [range, setRange] = useState<'7d' | '14d' | '30d' | '90d' | 'all'>('30d')
  const [deviceType, setDeviceType] = useState<string>('all')
  const [userIdFilter, setUserIdFilter] = useState<string>('')

  const fetchStats = useCallback(async () => {
    try {
      setLoading(true)
      const uId = userIdFilter.trim() ? parseInt(userIdFilter.trim(), 10) : undefined
      const data = await getByogAdminStats({
        range,
        device_type: deviceType !== 'all' ? deviceType : undefined,
        user_id: uId && !isNaN(uId) ? uId : undefined,
      })
      setStats(data)
    } catch (err: any) {
      toast.error(err.message || t('byog.admin.stats_fetch_error', 'İstatistikler yüklenemedi'))
    } finally {
      setLoading(false)
    }
  }, [range, deviceType, userIdFilter, t])

  useEffect(() => {
    fetchStats()
  }, [fetchStats])

  const formatNumber = (num: number) => {
    if (num >= 1_000_000) return `${(num / 1_000_000).toFixed(2)}M`
    if (num >= 1_000) return `${(num / 1_000).toFixed(1)}K`
    return num.toLocaleString()
  }

  // Hourly chart data transform
  const hourlyData = (stats?.hourly_distribution || []).map((count, hour) => ({
    hour: `${hour.toString().padStart(2, '0')}:00`,
    requests: count,
  }))

  return (
    <div className='space-y-6'>
      {/* Filter Toolbar */}
      <Card className='border-border/60 bg-card/60 backdrop-blur-sm'>
        <CardContent className='p-4'>
          <div className='flex flex-wrap items-center justify-between gap-4'>
            <div className='flex flex-wrap items-center gap-2'>
              <span className='text-xs font-semibold text-muted-foreground flex items-center gap-1.5 mr-2'>
                <Filter className='h-3.5 w-3.5' />
                {t('byog.admin.stats_filters', 'Filtreler')}:
              </span>

              {/* Date Range Tabs */}
              <div className='inline-flex rounded-lg border border-border/60 bg-muted/30 p-1 text-xs'>
                {(['7d', '14d', '30d', '90d', 'all'] as const).map((r) => (
                  <button
                    key={r}
                    onClick={() => setRange(r)}
                    className={`rounded-md px-3 py-1 font-medium transition-all ${
                      range === r
                        ? 'bg-background text-foreground shadow-sm'
                        : 'text-muted-foreground hover:text-foreground'
                    }`}
                  >
                    {r === '7d' && t('byog.admin.stats_range_7d', '7 Gün')}
                    {r === '14d' && t('byog.admin.stats_range_14d', '14 Gün')}
                    {r === '30d' && t('byog.admin.stats_range_30d', '30 Gün')}
                    {r === '90d' && t('byog.admin.stats_range_90d', '90 Gün')}
                    {r === 'all' && t('byog.admin.stats_range_all', 'Tümü')}
                  </button>
                ))}
              </div>

              {/* Device Type Select */}
              <Select value={deviceType} onValueChange={setDeviceType}>
                <SelectTrigger className='h-8 w-36 text-xs bg-background'>
                  <SelectValue placeholder={t('byog.admin.stats_all_devices', 'Tüm Cihazlar')} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='all'>{t('byog.admin.stats_all_devices', 'Tüm Cihazlar')}</SelectItem>
                  <SelectItem value='desktop'>{t('byog.admin.stats_desktop', 'Masaüstü')}</SelectItem>
                  <SelectItem value='mobile'>{t('byog.admin.stats_mobile', 'Mobil')}</SelectItem>
                  <SelectItem value='tablet'>{t('byog.admin.stats_tablet', 'Tablet')}</SelectItem>
                </SelectContent>
              </Select>

              {/* User ID search */}
              <div className='relative'>
                <Input
                  type='number'
                  placeholder={t('byog.admin.stats_user_id_placeholder', 'Kullanıcı ID')}
                  value={userIdFilter}
                  onChange={(e) => setUserIdFilter(e.target.value)}
                  onKeyDown={(e) => e.key === 'Enter' && fetchStats()}
                  className='h-8 w-32 text-xs pl-7'
                  autoComplete='off'
                />
                <Search className='h-3 w-3 absolute left-2 top-2.5 text-muted-foreground' />
              </div>
            </div>

            <Button
              variant='outline'
              size='sm'
              onClick={fetchStats}
              disabled={loading}
              className='h-8 text-xs gap-1.5'
            >
              <RefreshCw className={`h-3.5 w-3.5 ${loading ? 'animate-spin' : ''}`} />
              {t('common.refresh', 'Yenile')}
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* Summary KPI Cards */}
      <div className='grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-6'>
        {/* Total Requests */}
        <Card className='border-border/60 bg-gradient-to-br from-blue-500/5 to-transparent'>
          <CardHeader className='p-4 pb-2'>
            <div className='flex items-center justify-between'>
              <span className='text-xs font-medium text-muted-foreground'>
                {t('byog.admin.stats_total_requests', 'Toplam İstek')}
              </span>
              <Activity className='h-4 w-4 text-blue-500' />
            </div>
          </CardHeader>
          <CardContent className='p-4 pt-0'>
            <div className='text-2xl font-black tracking-tight text-foreground'>
              {stats ? formatNumber(stats.total_requests) : '—'}
            </div>
            <p className='text-[10px] text-muted-foreground mt-0.5'>
              {t('byog.admin.stats_period_label', 'Seçili dönemde')}
            </p>
          </CardContent>
        </Card>

        {/* Total Tokens */}
        <Card className='border-border/60 bg-gradient-to-br from-emerald-500/5 to-transparent'>
          <CardHeader className='p-4 pb-2'>
            <div className='flex items-center justify-between'>
              <span className='text-xs font-medium text-muted-foreground'>
                {t('byog.admin.stats_total_tokens', 'İşlenen Token')}
              </span>
              <Zap className='h-4 w-4 text-emerald-500' />
            </div>
          </CardHeader>
          <CardContent className='p-4 pt-0'>
            <div className='text-2xl font-black tracking-tight text-foreground'>
              {stats ? formatNumber(stats.total_tokens) : '—'}
            </div>
            <p className='text-[10px] text-muted-foreground mt-0.5'>
              {t('byog.admin.stats_compute_volume', 'Hesaplama hacmi')}
            </p>
          </CardContent>
        </Card>

        {/* Unique Users */}
        <Card className='border-border/60 bg-gradient-to-br from-purple-500/5 to-transparent'>
          <CardHeader className='p-4 pb-2'>
            <div className='flex items-center justify-between'>
              <span className='text-xs font-medium text-muted-foreground'>
                {t('byog.admin.stats_active_users', 'Aktif Kullanıcı')}
              </span>
              <Users className='h-4 w-4 text-purple-500' />
            </div>
          </CardHeader>
          <CardContent className='p-4 pt-0'>
            <div className='text-2xl font-black tracking-tight text-foreground'>
              {stats ? stats.total_users.toLocaleString() : '—'}
            </div>
            <p className='text-[10px] text-muted-foreground mt-0.5'>
              {t('byog.admin.stats_unique_consumers', 'Farklı hesap')}
            </p>
          </CardContent>
        </Card>

        {/* Online / Total Nodes */}
        <Card className='border-border/60 bg-gradient-to-br from-amber-500/5 to-transparent'>
          <CardHeader className='p-4 pb-2'>
            <div className='flex items-center justify-between'>
              <span className='text-xs font-medium text-muted-foreground'>
                {t('byog.admin.stats_nodes_online', 'Çevrimiçi Cihaz')}
              </span>
              <Cpu className='h-4 w-4 text-amber-500' />
            </div>
          </CardHeader>
          <CardContent className='p-4 pt-0'>
            <div className='flex items-baseline gap-1.5'>
              <span className='text-2xl font-black tracking-tight text-foreground'>
                {stats ? stats.online_nodes : '—'}
              </span>
              <span className='text-xs font-medium text-muted-foreground'>
                / {stats ? stats.total_nodes : '—'}
              </span>
            </div>
            <div className='flex items-center gap-1 mt-0.5'>
              <Badge variant='outline' className='text-[10px] h-4 px-1.5 border-emerald-500/30 text-emerald-500'>
                {stats && stats.total_nodes > 0
                  ? `%${Math.round((stats.online_nodes / stats.total_nodes) * 100)} hazır`
                  : '—'}
              </Badge>
            </div>
          </CardContent>
        </Card>

        {/* Avg Response Time */}
        <Card className='border-border/60 bg-gradient-to-br from-cyan-500/5 to-transparent'>
          <CardHeader className='p-4 pb-2'>
            <div className='flex items-center justify-between'>
              <span className='text-xs font-medium text-muted-foreground'>
                {t('byog.admin.stats_avg_duration', 'Ort. Süre')}
              </span>
              <Clock className='h-4 w-4 text-cyan-500' />
            </div>
          </CardHeader>
          <CardContent className='p-4 pt-0'>
            <div className='text-2xl font-black tracking-tight text-foreground'>
              {stats ? `${stats.avg_use_time_seconds.toFixed(1)}s` : '—'}
            </div>
            <p className='text-[10px] text-muted-foreground mt-0.5'>
              {t('byog.admin.stats_request_duration', 'İstek başına')}
            </p>
          </CardContent>
        </Card>

        {/* Distributed Credits */}
        <Card className='border-border/60 bg-gradient-to-br from-rose-500/5 to-transparent'>
          <CardHeader className='p-4 pb-2'>
            <div className='flex items-center justify-between'>
              <span className='text-xs font-medium text-muted-foreground'>
                {t('byog.admin.stats_credits_earned', 'Kazanılan Kredi')}
              </span>
              <Coins className='h-4 w-4 text-rose-500' />
            </div>
          </CardHeader>
          <CardContent className='p-4 pt-0'>
            <div className='text-2xl font-black tracking-tight text-foreground'>
              {stats ? `$${stats.total_credits_earned.toFixed(2)}` : '—'}
            </div>
            <p className='text-[10px] text-muted-foreground mt-0.5'>
              {t('byog.admin.stats_miners_reward', 'Cihaz sahiplerine')}
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Main Charts: Requests & Tokens Area Chart */}
      <Card className='border-border/60'>
        <CardHeader>
          <div className='flex items-center justify-between'>
            <div>
              <CardTitle className='text-base font-bold flex items-center gap-2'>
                <TrendingUp className='h-4 w-4 text-primary' />
                {t('byog.admin.stats_daily_trend_title', 'Günlük İstek ve Token Hacmi')}
              </CardTitle>
              <CardDescription className='text-xs'>
                {t('byog.admin.stats_daily_trend_desc', 'BYOG ağı üzerinden işlenen günlük çıkarım istekleri ve token üretim eğilimi')}
              </CardDescription>
            </div>
          </div>
        </CardHeader>
        <CardContent>
          <div className='h-[300px] w-full'>
            {stats && stats.time_series && stats.time_series.length > 0 ? (
              <ResponsiveContainer width='100%' height='100%'>
                <AreaChart data={stats.time_series} margin={{ top: 10, right: 20, left: -10, bottom: 0 }}>
                  <defs>
                    <linearGradient id='colorRequests' x1='0' y1='0' x2='0' y2='1'>
                      <stop offset='5%' stopColor='#3b82f6' stopOpacity={0.4} />
                      <stop offset='95%' stopColor='#3b82f6' stopOpacity={0.0} />
                    </linearGradient>
                    <linearGradient id='colorTokens' x1='0' y1='0' x2='0' y2='1'>
                      <stop offset='5%' stopColor='#10b981' stopOpacity={0.4} />
                      <stop offset='95%' stopColor='#10b981' stopOpacity={0.0} />
                    </linearGradient>
                  </defs>
                  <CartesianGrid strokeDasharray='3 3' className='stroke-border/50' />
                  <XAxis
                    dataKey='date'
                    tick={{ fontSize: 11 }}
                    tickFormatter={(v) => v.slice(5)}
                    className='text-muted-foreground'
                  />
                  <YAxis
                    yAxisId='left'
                    tick={{ fontSize: 11 }}
                    tickFormatter={(v) => formatNumber(v)}
                    className='text-muted-foreground'
                  />
                  <YAxis
                    yAxisId='right'
                    orientation='right'
                    tick={{ fontSize: 11 }}
                    tickFormatter={(v) => formatNumber(v)}
                    className='text-muted-foreground'
                  />
                  <Tooltip
                    contentStyle={{
                      backgroundColor: 'hsl(var(--card))',
                      borderColor: 'hsl(var(--border))',
                      borderRadius: '8px',
                      fontSize: '12px',
                    }}
                    formatter={(value: any, name: any) => [
                      formatNumber(Number(value)),
                      name === 'requests'
                        ? t('byog.admin.stats_requests', 'İstek')
                        : name === 'tokens'
                        ? t('byog.admin.stats_tokens', 'Token')
                        : name,
                    ]}
                  />
                  <Legend
                    formatter={(v) =>
                      v === 'requests'
                        ? t('byog.admin.stats_requests', 'İstek Sayısı')
                        : t('byog.admin.stats_tokens', 'Token Hacmi')
                    }
                  />
                  <Area
                    yAxisId='left'
                    type='monotone'
                    dataKey='requests'
                    stroke='#3b82f6'
                    strokeWidth={2}
                    fillOpacity={1}
                    fill='url(#colorRequests)'
                  />
                  <Area
                    yAxisId='right'
                    type='monotone'
                    dataKey='tokens'
                    stroke='#10b981'
                    strokeWidth={2}
                    fillOpacity={1}
                    fill='url(#colorTokens)'
                  />
                </AreaChart>
              </ResponsiveContainer>
            ) : (
              <div className='flex h-full items-center justify-center text-sm text-muted-foreground'>
                {t('byog.admin.stats_no_data', 'Seçili filtre aralığında veri bulunamadı')}
              </div>
            )}
          </div>
        </CardContent>
      </Card>

      {/* Grid: Active Users & Registrations + 24H Distribution */}
      <div className='grid grid-cols-1 gap-6 lg:grid-cols-2'>
        {/* Active Users vs New Registrations */}
        <Card className='border-border/60'>
          <CardHeader>
            <CardTitle className='text-base font-bold flex items-center gap-2'>
              <Users className='h-4 w-4 text-purple-500' />
              {t('byog.admin.stats_user_node_growth_title', 'Kullanıcı ve Cihaz Katılımı')}
            </CardTitle>
            <CardDescription className='text-xs'>
              {t('byog.admin.stats_user_node_growth_desc', 'Günlük aktif kullanıcı sayısı ile yeni kaydedilen BYOG worker cihazları')}
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className='h-[250px] w-full'>
              {stats && stats.time_series && stats.time_series.length > 0 ? (
                <ResponsiveContainer width='100%' height='100%'>
                  <LineChart data={stats.time_series} margin={{ top: 10, right: 20, left: -10, bottom: 0 }}>
                    <CartesianGrid strokeDasharray='3 3' className='stroke-border/50' />
                    <XAxis
                      dataKey='date'
                      tick={{ fontSize: 11 }}
                      tickFormatter={(v) => v.slice(5)}
                    />
                    <YAxis tick={{ fontSize: 11 }} />
                    <Tooltip
                      contentStyle={{
                        backgroundColor: 'hsl(var(--card))',
                        borderColor: 'hsl(var(--border))',
                        borderRadius: '8px',
                        fontSize: '12px',
                      }}
                      formatter={(value: any, name: any) => [
                        value,
                        name === 'unique_users'
                          ? t('byog.admin.stats_unique_users', 'Aktif Kullanıcı')
                          : name === 'new_nodes'
                          ? t('byog.admin.stats_new_nodes', 'Yeni Cihaz Kaydı')
                          : name === 'active_nodes'
                          ? t('byog.admin.stats_active_nodes', 'Aktif Cihaz')
                          : name,
                      ]}
                    />
                    <Legend
                      formatter={(v) =>
                        v === 'unique_users'
                          ? t('byog.admin.stats_unique_users', 'Aktif Kullanıcı')
                          : v === 'new_nodes'
                          ? t('byog.admin.stats_new_nodes', 'Yeni Cihaz')
                          : t('byog.admin.stats_active_nodes', 'Aktif Cihaz')
                      }
                    />
                    <Line
                      type='monotone'
                      dataKey='unique_users'
                      stroke='#8b5cf6'
                      strokeWidth={2}
                      dot={{ r: 3 }}
                    />
                    <Line
                      type='monotone'
                      dataKey='new_nodes'
                      stroke='#f59e0b'
                      strokeWidth={2}
                      dot={{ r: 3 }}
                    />
                    <Line
                      type='monotone'
                      dataKey='active_nodes'
                      stroke='#10b981'
                      strokeWidth={2}
                      strokeDasharray='4 4'
                    />
                  </LineChart>
                </ResponsiveContainer>
              ) : (
                <div className='flex h-full items-center justify-center text-sm text-muted-foreground'>
                  {t('byog.admin.stats_no_data', 'Veri bulunamadı')}
                </div>
              )}
            </div>
          </CardContent>
        </Card>

        {/* 24-Hour Peak Activity Heatmap */}
        <Card className='border-border/60'>
          <CardHeader>
            <CardTitle className='text-base font-bold flex items-center gap-2'>
              <Clock className='h-4 w-4 text-cyan-500' />
              {t('byog.admin.stats_hourly_distribution_title', '24 Saatlik İstek Dağılımı')}
            </CardTitle>
            <CardDescription className='text-xs'>
              {t('byog.admin.stats_hourly_distribution_desc', 'Günün saatlerine göre ağdaki yoğunluk ve yük zirveleri')}
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className='h-[250px] w-full'>
              {hourlyData.length > 0 ? (
                <ResponsiveContainer width='100%' height='100%'>
                  <BarChart data={hourlyData} margin={{ top: 10, right: 10, left: -15, bottom: 0 }}>
                    <CartesianGrid strokeDasharray='3 3' className='stroke-border/50' />
                    <XAxis dataKey='hour' tick={{ fontSize: 10 }} />
                    <YAxis tick={{ fontSize: 10 }} tickFormatter={(v) => formatNumber(v)} />
                    <Tooltip
                      contentStyle={{
                        backgroundColor: 'hsl(var(--card))',
                        borderColor: 'hsl(var(--border))',
                        borderRadius: '8px',
                        fontSize: '12px',
                      }}
                      formatter={(val: any) => [Number(val).toLocaleString(), t('byog.admin.stats_requests', 'İstek')]}
                    />
                    <Bar dataKey='requests' fill='#06b6d4' radius={[4, 4, 0, 0]} />
                  </BarChart>
                </ResponsiveContainer>
              ) : (
                <div className='flex h-full items-center justify-center text-sm text-muted-foreground'>
                  {t('byog.admin.stats_no_data', 'Veri bulunamadı')}
                </div>
              )}
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Grid: Device Breakdown Donut + Top Models Share */}
      <div className='grid grid-cols-1 gap-6 lg:grid-cols-3'>
        {/* Device Type Breakdown */}
        <Card className='border-border/60 lg:col-span-1'>
          <CardHeader>
            <CardTitle className='text-base font-bold flex items-center gap-2'>
              <Monitor className='h-4 w-4 text-primary' />
              {t('byog.admin.stats_device_breakdown_title', 'Cihaz Türü Dağılımı')}
            </CardTitle>
            <CardDescription className='text-xs'>
              {t('byog.admin.stats_device_breakdown_desc', 'Masaüstü, mobil ve tablet düğüm oranları')}
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className='h-[240px] w-full'>
              {stats && stats.device_breakdown && stats.device_breakdown.length > 0 ? (
                <ResponsiveContainer width='100%' height='100%'>
                  <PieChart>
                    <Pie
                      data={stats.device_breakdown}
                      dataKey='node_count'
                      nameKey='device_type'
                      cx='50%'
                      cy='50%'
                      innerRadius={55}
                      outerRadius={80}
                      paddingAngle={4}
                    >
                      {stats.device_breakdown.map((entry, index) => (
                        <Cell key={`cell-${index}`} fill={PIE_COLORS[index % PIE_COLORS.length]} />
                      ))}
                    </Pie>
                    <Tooltip
                      contentStyle={{
                        backgroundColor: 'hsl(var(--card))',
                        borderColor: 'hsl(var(--border))',
                        borderRadius: '8px',
                        fontSize: '12px',
                      }}
                      formatter={(val: any, name: any) => [
                        `${val} cihaz`,
                        name === 'desktop'
                          ? t('byog.admin.stats_desktop', 'Masaüstü')
                          : name === 'mobile'
                          ? t('byog.admin.stats_mobile', 'Mobil')
                          : name === 'tablet'
                          ? t('byog.admin.stats_tablet', 'Tablet')
                          : name,
                      ]}
                    />
                    <Legend
                      formatter={(v) =>
                        v === 'desktop'
                          ? t('byog.admin.stats_desktop', 'Masaüstü')
                          : v === 'mobile'
                          ? t('byog.admin.stats_mobile', 'Mobil')
                          : t('byog.admin.stats_tablet', 'Tablet')
                      }
                    />
                  </PieChart>
                </ResponsiveContainer>
              ) : (
                <div className='flex h-full items-center justify-center text-sm text-muted-foreground'>
                  {t('byog.admin.stats_no_data', 'Veri bulunamadı')}
                </div>
              )}
            </div>
          </CardContent>
        </Card>

        {/* Top Models Share */}
        <Card className='border-border/60 lg:col-span-2'>
          <CardHeader>
            <CardTitle className='text-base font-bold flex items-center gap-2'>
              <Layers className='h-4 w-4 text-emerald-500' />
              {t('byog.admin.stats_top_models_title', 'En Çok Kullanılan Modeller')}
            </CardTitle>
            <CardDescription className='text-xs'>
              {t('byog.admin.stats_top_models_desc', 'BYOG ağı üzerinde en çok çıkarım yapılan yapay zeka modelleri ve istek payları')}
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className='h-[240px] w-full'>
              {stats && stats.top_models && stats.top_models.length > 0 ? (
                <ResponsiveContainer width='100%' height='100%'>
                  <BarChart
                    data={stats.top_models}
                    layout='vertical'
                    margin={{ top: 5, right: 30, left: 40, bottom: 5 }}
                  >
                    <CartesianGrid strokeDasharray='3 3' className='stroke-border/50' />
                    <XAxis type='number' tick={{ fontSize: 10 }} tickFormatter={(v) => formatNumber(v)} />
                    <YAxis
                      dataKey='model_id'
                      type='category'
                      tick={{ fontSize: 11 }}
                      width={120}
                    />
                    <Tooltip
                      contentStyle={{
                        backgroundColor: 'hsl(var(--card))',
                        borderColor: 'hsl(var(--border))',
                        borderRadius: '8px',
                        fontSize: '12px',
                      }}
                      formatter={(val: any, name: any) => [
                        formatNumber(Number(val)),
                        name === 'requests' ? 'İstek' : 'Token',
                      ]}
                    />
                    <Legend />
                    <Bar dataKey='requests' name='İstek' fill='#3b82f6' radius={[0, 4, 4, 0]} />
                    <Bar dataKey='tokens' name='Token' fill='#10b981' radius={[0, 4, 4, 0]} />
                  </BarChart>
                </ResponsiveContainer>
              ) : (
                <div className='flex h-full items-center justify-center text-sm text-muted-foreground'>
                  {t('byog.admin.stats_no_data', 'Veri bulunamadı')}
                </div>
              )}
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Top Compute Nodes Leaderboard Table */}
      <Card className='border-border/60'>
        <CardHeader>
          <div className='flex items-center justify-between'>
            <div>
              <CardTitle className='text-base font-bold flex items-center gap-2'>
                <Trophy className='h-4 w-4 text-amber-500' />
                {t('byog.admin.stats_top_nodes_title', 'En Çok İstek Alan BYOG Düğümleri Lider Tablosu')}
              </CardTitle>
              <CardDescription className='text-xs'>
                {t('byog.admin.stats_top_nodes_desc', 'Ağda en yüksek çıkarım yükünü karşılayan ve en çok token işleyen istemciler')}
              </CardDescription>
            </div>
          </div>
        </CardHeader>
        <CardContent>
          <div className='overflow-x-auto'>
            <table className='w-full text-left text-xs'>
              <thead className='border-b border-border/80 bg-muted/30 text-muted-foreground font-semibold'>
                <tr>
                  <th className='py-2.5 px-3 w-14'>#</th>
                  <th className='py-2.5 px-3'>{t('byog.admin.stats_node_name', 'Düğüm / Cihaz')}</th>
                  <th className='py-2.5 px-3'>{t('byog.admin.stats_type', 'Tür')}</th>
                  <th className='py-2.5 px-3'>{t('byog.admin.stats_owner', 'Sahip (Kullanıcı)')}</th>
                  <th className='py-2.5 px-3 font-mono text-right'>{t('byog.admin.stats_requests_col', 'İstek Sayısı')}</th>
                  <th className='py-2.5 px-3 font-mono text-right'>{t('byog.admin.stats_tokens_col', 'İşlenen Token')}</th>
                </tr>
              </thead>
              <tbody className='divide-y divide-border/40'>
                {stats && stats.top_nodes && stats.top_nodes.length > 0 ? (
                  stats.top_nodes.map((node, index) => {
                    const isTop1 = index === 0
                    const isTop2 = index === 1
                    const isTop3 = index === 2
                    return (
                      <tr key={node.node_id} className='hover:bg-muted/20 transition-colors'>
                        <td className='py-3 px-3'>
                          <span
                            className={`inline-flex items-center justify-center size-6 rounded-full font-bold text-[11px] ${
                              isTop1
                                ? 'bg-amber-500 text-black shadow-sm'
                                : isTop2
                                ? 'bg-slate-300 text-black'
                                : isTop3
                                ? 'bg-amber-700 text-white'
                                : 'bg-muted text-muted-foreground'
                            }`}
                          >
                            {index + 1}
                          </span>
                        </td>
                        <td className='py-3 px-3'>
                          <div className='font-semibold text-foreground'>{node.device_name || node.node_id}</div>
                          <div className='text-[10px] text-muted-foreground font-mono'>{node.node_id}</div>
                        </td>
                        <td className='py-3 px-3'>
                          <div className='flex items-center gap-1.5'>
                            {node.device_type === 'mobile' ? (
                              <Smartphone className='h-3.5 w-3.5 text-emerald-500' />
                            ) : node.device_type === 'tablet' ? (
                              <Tablet className='h-3.5 w-3.5 text-purple-500' />
                            ) : (
                              <Monitor className='h-3.5 w-3.5 text-blue-500' />
                            )}
                            <span className='capitalize text-muted-foreground'>{node.device_type || 'desktop'}</span>
                          </div>
                        </td>
                        <td className='py-3 px-3'>
                          <span className='font-medium text-foreground'>{node.username || `User #${node.user_id}`}</span>
                          <span className='text-[10px] text-muted-foreground ml-1.5 font-mono'>#{node.user_id}</span>
                        </td>
                        <td className='py-3 px-3 font-mono font-bold text-right text-blue-500'>
                          {node.requests.toLocaleString()}
                        </td>
                        <td className='py-3 px-3 font-mono font-bold text-right text-emerald-500'>
                          {formatNumber(node.tokens)}
                        </td>
                      </tr>
                    )
                  })
                ) : (
                  <tr>
                    <td colSpan={6} className='py-8 text-center text-muted-foreground'>
                      {t('byog.admin.stats_no_top_nodes', 'Henüz aktif çıkarım yapmış düğüm kaydı yok')}
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </CardContent>
      </Card>

      {/* Top Consumers Table */}
      <Card className='border-border/60'>
        <CardHeader>
          <div className='flex items-center justify-between'>
            <div>
              <CardTitle className='text-base font-bold flex items-center gap-2'>
                <Users className='h-4 w-4 text-purple-500' />
                {t('byog.admin.stats_top_users_title', 'En Çok İstek Atan Kullanıcılar')}
              </CardTitle>
              <CardDescription className='text-xs'>
                {t('byog.admin.stats_top_users_desc', 'BYOG yerel modellerine en yoğun istek gönderen ve token tüketen hesaplar')}
              </CardDescription>
            </div>
          </div>
        </CardHeader>
        <CardContent>
          <div className='overflow-x-auto'>
            <table className='w-full text-left text-xs'>
              <thead className='border-b border-border/80 bg-muted/30 text-muted-foreground font-semibold'>
                <tr>
                  <th className='py-2.5 px-3 w-14'>#</th>
                  <th className='py-2.5 px-3'>{t('byog.admin.stats_user_name', 'Kullanıcı Adı')}</th>
                  <th className='py-2.5 px-3 font-mono text-right'>{t('byog.admin.stats_requests_col', 'İstek Sayısı')}</th>
                  <th className='py-2.5 px-3 font-mono text-right'>{t('byog.admin.stats_tokens_col', 'İşlenen Token')}</th>
                </tr>
              </thead>
              <tbody className='divide-y divide-border/40'>
                {stats && stats.top_users && stats.top_users.length > 0 ? (
                  stats.top_users.map((user, index) => (
                    <tr key={user.user_id} className='hover:bg-muted/20 transition-colors'>
                      <td className='py-3 px-3 font-mono text-muted-foreground'>{index + 1}</td>
                      <td className='py-3 px-3'>
                        <span className='font-semibold text-foreground'>{user.username || `User #${user.user_id}`}</span>
                        <span className='text-[10px] text-muted-foreground ml-1.5 font-mono'>#{user.user_id}</span>
                      </td>
                      <td className='py-3 px-3 font-mono font-bold text-right text-blue-500'>
                        {user.requests.toLocaleString()}
                      </td>
                      <td className='py-3 px-3 font-mono font-bold text-right text-purple-500'>
                        {formatNumber(user.tokens)}
                      </td>
                    </tr>
                  ))
                ) : (
                  <tr>
                    <td colSpan={4} className='py-8 text-center text-muted-foreground'>
                      {t('byog.admin.stats_no_top_users', 'Henüz BYOG model isteği atan kullanıcı yok')}
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
