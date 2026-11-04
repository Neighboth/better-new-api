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
  AlertTriangle,
  Coins,
  Filter,
  Globe,
  KeyRound,
  LogIn,
  Network,
  Radio,
  RefreshCw,
  Server,
  Ticket,
  UserPlus,
  Users,
  Zap,
} from 'lucide-react'
import {
  useCallback,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react'
import { useTranslation } from 'react-i18next'
import {
  Area,
  AreaChart,
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  Legend,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
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
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import dayjs from '@/lib/dayjs'
import {
  formatCompactNumber,
  formatNumber,
  formatQuota,
  formatTokens,
  formatUseTime,
} from '@/lib/format'

import {
  getSiteStatistics,
  type SiteStatisticsResponse,
  type SiteStatsGranularity,
  type SiteStatsRangePreset,
} from './api'

const PIE_COLORS = ['#3b82f6', '#10b981', '#f59e0b', '#8b5cf6', '#ec4899']

const RANGE_PRESETS: SiteStatsRangePreset[] = [
  'today',
  '24h',
  '7d',
  '30d',
  '90d',
  '180d',
  '365d',
  'all',
]

const RANGE_LABEL_KEYS: Record<SiteStatsRangePreset, string> = {
  today: 'Today',
  '24h': 'Last 24 Hours',
  '7d': 'Last 7 Days',
  '30d': 'Last 30 Days',
  '90d': 'Last 90 Days',
  '180d': 'Last 180 Days',
  '365d': 'Last 365 Days',
  all: 'All Time',
}

const WEEKDAY_KEYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']

const LOG_TYPE_KEYS: Record<number, string> = {
  0: 'Unknown',
  1: 'Top-up',
  2: 'Consume',
  3: 'Manage',
  4: 'System',
  5: 'Error',
  6: 'Refund',
  7: 'Login',
}

type SeriesMetric =
  | 'requests'
  | 'tokens'
  | 'quota'
  | 'unique_users'
  | 'errors'
  | 'registrations'

const SERIES_METRICS: { value: SeriesMetric; labelKey: string }[] = [
  { value: 'requests', labelKey: 'Requests' },
  { value: 'tokens', labelKey: 'Tokens' },
  { value: 'quota', labelKey: 'Quota Used' },
  { value: 'unique_users', labelKey: 'Unique Users' },
  { value: 'errors', labelKey: 'Errors' },
  { value: 'registrations', labelKey: 'Registrations' },
]

interface TextFilters {
  includeUsers: string
  excludeUsers: string
  model: string
  channelId: string
  group: string
}

const EMPTY_TEXT_FILTERS: TextFilters = {
  includeUsers: '',
  excludeUsers: '',
  model: '',
  channelId: '',
  group: '',
}

// formatUptime renders the lifetime seconds of a node in a compact form.
function formatUptime(seconds: number): string {
  if (seconds < 60) return `${Math.max(0, Math.floor(seconds))}s`
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  if (days > 0) return `${days}d ${hours}h`
  if (hours > 0) return `${hours}h ${minutes}m`
  return `${minutes}m`
}

interface StatCardProps {
  icon: ReactNode
  label: string
  value: string
  description?: string
}

function StatCard({ icon, label, value, description }: StatCardProps) {
  return (
    <Card>
      <CardHeader className='pb-2'>
        <CardTitle className='flex items-center gap-2 text-sm font-medium text-muted-foreground'>
          <span className='flex size-4 shrink-0 items-center justify-center'>
            {icon}
          </span>
          <span className='truncate'>{label}</span>
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div className='truncate text-2xl font-bold tabular-nums'>{value}</div>
        {description ? (
          <p className='mt-1 truncate text-xs text-muted-foreground'>
            {description}
          </p>
        ) : null}
      </CardContent>
    </Card>
  )
}

interface RankRow {
  key: string
  name: string
  detail?: string
  requests: number
  tokens: number
  quota?: number
}

interface RankTableProps {
  rows: RankRow[]
  nameHeader: string
  showQuota?: boolean
}

function RankTable({ rows, nameHeader, showQuota = true }: RankTableProps) {
  const { t } = useTranslation()
  const max = Math.max(1, ...rows.map((row) => row.requests))
  if (rows.length === 0) {
    return (
      <div className='py-8 text-center text-sm text-muted-foreground'>
        {t('No Data')}
      </div>
    )
  }
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead className='w-12'>#</TableHead>
          <TableHead>{nameHeader}</TableHead>
          <TableHead className='w-44'>{t('Requests')}</TableHead>
          <TableHead className='w-28 text-right'>{t('Tokens')}</TableHead>
          {showQuota ? (
            <TableHead className='w-28 text-right'>{t('Quota Used')}</TableHead>
          ) : null}
        </TableRow>
      </TableHeader>
      <TableBody>
        {rows.map((row, index) => (
          <TableRow key={row.key}>
            <TableCell className='tabular-nums text-muted-foreground'>
              {index + 1}
            </TableCell>
            <TableCell>
              <div className='flex flex-col'>
                <span className='max-w-56 truncate font-medium'>{row.name}</span>
                {row.detail ? (
                  <span className='text-xs text-muted-foreground'>
                    {row.detail}
                  </span>
                ) : null}
              </div>
            </TableCell>
            <TableCell>
              <div className='flex items-center gap-2'>
                <div className='h-1.5 w-20 shrink-0 overflow-hidden rounded-full bg-muted'>
                  <div
                    className='h-full rounded-full bg-blue-500'
                    style={{ width: `${(row.requests / max) * 100}%` }}
                  />
                </div>
                <span className='tabular-nums'>
                  {formatNumber(row.requests)}
                </span>
              </div>
            </TableCell>
            <TableCell className='text-right tabular-nums'>
              {formatTokens(row.tokens)}
            </TableCell>
            {showQuota ? (
              <TableCell className='text-right tabular-nums'>
                {row.quota !== undefined ? formatQuota(row.quota) : '-'}
              </TableCell>
            ) : null}
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}

export function SiteStatistics() {
  const { t } = useTranslation()
  const [loading, setLoading] = useState(true)
  const [data, setData] = useState<SiteStatisticsResponse | null>(null)

  // The selects reload on change, the text inputs reload only through the
  // apply button so that typing does not fire a request per keystroke.
  const [rangePreset, setRangePreset] =
    useState<SiteStatsRangePreset>('30d')
  const [granularity, setGranularity] = useState<'auto' | SiteStatsGranularity>(
    'auto'
  )
  const [sort, setSort] = useState<'requests' | 'tokens' | 'quota'>('requests')
  const [order, setOrder] = useState<'asc' | 'desc'>('desc')
  const [limit, setLimit] = useState(20)
  const [textDrafts, setTextDrafts] = useState<TextFilters>(EMPTY_TEXT_FILTERS)
  const [textFilters, setTextFilters] =
    useState<TextFilters>(EMPTY_TEXT_FILTERS)

  const [metric, setMetric] = useState<SeriesMetric>('requests')

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const channelId = Number(textFilters.channelId.trim())
      const res = await getSiteStatistics({
        range: rangePreset,
        granularity: granularity === 'auto' ? undefined : granularity,
        tz_offset: new Date().getTimezoneOffset(),
        user_ids: textFilters.includeUsers.trim() || undefined,
        exclude_user_ids: textFilters.excludeUsers.trim() || undefined,
        model: textFilters.model.trim() || undefined,
        channel_id:
          Number.isFinite(channelId) && channelId > 0 ? channelId : undefined,
        group: textFilters.group.trim() || undefined,
        sort,
        order,
        limit,
      })
      setData(res)
    } catch {
      toast.error(t('Failed to load statistics'))
    } finally {
      setLoading(false)
    }
  }, [rangePreset, granularity, sort, order, limit, textFilters, t])

  useEffect(() => {
    load()
  }, [load])

  const applyFilters = () => {
    setTextFilters({ ...textDrafts })
  }

  const seriesData = useMemo(() => {
    if (!data) return []
    const fmt =
      data.range.granularity === 'hour'
        ? 'HH:mm'
        : data.range.granularity === 'month'
          ? 'YYYY-MM'
          : 'MM-DD'
    return data.series.map((point) => ({
      label: dayjs.unix(point.bucket).format(fmt),
      requests: point.requests,
      tokens: point.tokens,
      quota: point.quota,
      unique_users: point.unique_users,
      errors: point.errors,
      registrations: point.registrations,
    }))
  }, [data])

  const hourOfDayData = useMemo(() => {
    if (!data) return []
    return data.distributions.hour_of_day.map((count, hour) => ({
      label: String(hour).padStart(2, '0'),
      count,
    }))
  }, [data])

  const weekdayData = useMemo(() => {
    if (!data) return []
    return data.distributions.weekday.map((count, index) => ({
      label: t(WEEKDAY_KEYS[index] ?? ''),
      count,
    }))
  }, [data, t])

  const logTypeData = useMemo(() => {
    if (!data) return []
    return data.rankings.log_types
      .filter((row) => row.count > 0)
      .map((row) => ({
        name: t(LOG_TYPE_KEYS[row.type] ?? 'Unknown'),
        value: row.count,
      }))
  }, [data, t])

  const tooltipValue = useCallback(
    (value: number) => {
      switch (metric) {
        case 'tokens':
          return formatTokens(value)
        case 'quota':
          return formatQuota(value)
        default:
          return formatNumber(value)
      }
    },
    [metric]
  )

  const yAxisTick = useCallback(
    (value: number) =>
      metric === 'quota' ? formatQuota(value) : formatCompactNumber(value),
    [metric]
  )

  const summary = data?.summary
  const counters = data?.counters

  const userRows: RankRow[] = (data?.rankings.users ?? []).map((row) => ({
    key: `user-${row.user_id}`,
    name: row.username || `#${row.user_id}`,
    detail: `#${row.user_id}`,
    requests: row.requests,
    tokens: row.tokens,
    quota: row.quota,
  }))
  const modelRows: RankRow[] = (data?.rankings.models ?? []).map((row) => ({
    key: `model-${row.model_name}`,
    name: row.model_name,
    requests: row.requests,
    tokens: row.tokens,
    quota: row.quota,
  }))
  const channelRows: RankRow[] = (data?.rankings.channels ?? []).map((row) => ({
    key: `channel-${row.channel_id}`,
    name: row.channel_name || `#${row.channel_id}`,
    requests: row.requests,
    tokens: row.tokens,
    quota: row.quota,
  }))
  const clientRows: RankRow[] = (data?.rankings.clients ?? []).map((row) => ({
    key: `client-${row.token_name}`,
    name: row.token_name || t('Unknown'),
    requests: row.requests,
    tokens: row.tokens,
    quota: row.quota,
  }))
  const groupRows: RankRow[] = (data?.rankings.groups ?? []).map((row) => ({
    key: `group-${row.group_name}`,
    name: row.group_name || t('Unknown'),
    requests: row.requests,
    tokens: row.tokens,
    quota: row.quota,
  }))
  const ipRows: RankRow[] = (data?.rankings.ips ?? []).map((row) => ({
    key: `ip-${row.ip}`,
    name: row.ip,
    requests: row.requests,
    tokens: row.tokens,
  }))

  const byogNodes = data?.rankings.byog_nodes ?? []

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Statistics')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button variant='outline' size='sm' onClick={load} disabled={loading}>
          <RefreshCw className={loading ? 'size-4 animate-spin' : 'size-4'} />
          {t('Refresh')}
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='space-y-4'>
          <p className='text-sm text-muted-foreground'>
            {t('Site-wide statistics of the whole installation.')}
          </p>

          {/* Filters */}
          <Card>
            <CardHeader className='pb-3'>
              <CardTitle className='flex items-center gap-2 text-base'>
                <Filter className='size-4' />
                {t('Filters')}
              </CardTitle>
            </CardHeader>
            <CardContent className='space-y-3'>
              <div className='grid grid-cols-2 gap-3 md:grid-cols-5'>
                <div className='space-y-1'>
                  <label className='text-xs text-muted-foreground'>
                    {t('Time Range')}
                  </label>
                  <Select
                    value={rangePreset}
                    onValueChange={(value) =>
                      setRangePreset(value as SiteStatsRangePreset)
                    }
                  >
                    <SelectTrigger className='w-full'>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {RANGE_PRESETS.map((preset) => (
                        <SelectItem key={preset} value={preset}>
                          {t(RANGE_LABEL_KEYS[preset])}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className='space-y-1'>
                  <label className='text-xs text-muted-foreground'>
                    {t('Granularity')}
                  </label>
                  <Select
                    value={granularity}
                    onValueChange={(value) =>
                      setGranularity(value as 'auto' | SiteStatsGranularity)
                    }
                  >
                    <SelectTrigger className='w-full'>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value='auto'>{t('Auto')}</SelectItem>
                      <SelectItem value='hour'>{t('Hour')}</SelectItem>
                      <SelectItem value='day'>{t('Day')}</SelectItem>
                      <SelectItem value='week'>{t('Week')}</SelectItem>
                      <SelectItem value='month'>{t('Month')}</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div className='space-y-1'>
                  <label className='text-xs text-muted-foreground'>
                    {t('Sort')}
                  </label>
                  <Select
                    value={sort}
                    onValueChange={(value) =>
                      setSort(value as 'requests' | 'tokens' | 'quota')
                    }
                  >
                    <SelectTrigger className='w-full'>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value='requests'>{t('Requests')}</SelectItem>
                      <SelectItem value='tokens'>{t('Tokens')}</SelectItem>
                      <SelectItem value='quota'>{t('Quota Used')}</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div className='space-y-1'>
                  <label className='text-xs text-muted-foreground'>
                    {t('Sort Order')}
                  </label>
                  <Select
                    value={order}
                    onValueChange={(value) => setOrder(value as 'asc' | 'desc')}
                  >
                    <SelectTrigger className='w-full'>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value='desc'>{t('Descending')}</SelectItem>
                      <SelectItem value='asc'>{t('Ascending')}</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div className='space-y-1'>
                  <label className='text-xs text-muted-foreground'>
                    {t('Top Count')}
                  </label>
                  <Select
                    value={String(limit)}
                    onValueChange={(value) => setLimit(Number(value))}
                  >
                    <SelectTrigger className='w-full'>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {[10, 20, 50, 100].map((value) => (
                        <SelectItem key={value} value={String(value)}>
                          {value}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              </div>
              <div className='grid grid-cols-1 gap-3 md:grid-cols-5'>
                <Input
                  placeholder={t('Include Users')}
                  title={t('User IDs, comma separated')}
                  value={textDrafts.includeUsers}
                  onChange={(event) =>
                    setTextDrafts((drafts) => ({
                      ...drafts,
                      includeUsers: event.target.value,
                    }))
                  }
                />
                <Input
                  placeholder={t('Exclude Users')}
                  title={t('User IDs, comma separated')}
                  value={textDrafts.excludeUsers}
                  onChange={(event) =>
                    setTextDrafts((drafts) => ({
                      ...drafts,
                      excludeUsers: event.target.value,
                    }))
                  }
                />
                <Input
                  placeholder={t('Model')}
                  value={textDrafts.model}
                  onChange={(event) =>
                    setTextDrafts((drafts) => ({
                      ...drafts,
                      model: event.target.value,
                    }))
                  }
                />
                <Input
                  placeholder={t('Channel ID')}
                  value={textDrafts.channelId}
                  onChange={(event) =>
                    setTextDrafts((drafts) => ({
                      ...drafts,
                      channelId: event.target.value,
                    }))
                  }
                />
                <Input
                  placeholder={t('Group')}
                  value={textDrafts.group}
                  onChange={(event) =>
                    setTextDrafts((drafts) => ({
                      ...drafts,
                      group: event.target.value,
                    }))
                  }
                />
              </div>
              <div className='flex justify-end'>
                <Button size='sm' onClick={applyFilters}>
                  {t('Apply')}
                </Button>
              </div>
            </CardContent>
          </Card>

          {/* Summary of the range */}
          {summary ? (
            <div className='grid grid-cols-2 gap-3 md:grid-cols-3 lg:grid-cols-5'>
              <StatCard
                icon={<Activity className='size-4 text-blue-500' />}
                label={t('Requests')}
                value={formatNumber(summary.requests)}
                description={`${t('Avg. Response')}: ${formatUseTime(summary.avg_use_time_seconds)}`}
              />
              <StatCard
                icon={<Users className='size-4 text-indigo-500' />}
                label={t('Unique Users')}
                value={formatNumber(summary.unique_users)}
              />
              <StatCard
                icon={<Zap className='size-4 text-amber-500' />}
                label={t('Tokens')}
                value={formatTokens(summary.tokens)}
              />
              <StatCard
                icon={<Coins className='size-4 text-yellow-500' />}
                label={t('Quota Used')}
                value={formatQuota(summary.quota)}
              />
              <StatCard
                icon={<Radio className='size-4 text-emerald-500' />}
                label={t('BYOG Requests')}
                value={formatNumber(summary.byog_requests)}
              />
              <StatCard
                icon={<Server className='size-4 text-cyan-500' />}
                label={t('Upstream Requests')}
                value={formatNumber(summary.upstream_requests)}
              />
              <StatCard
                icon={<AlertTriangle className='size-4 text-red-500' />}
                label={t('Errors')}
                value={formatNumber(summary.errors)}
              />
              <StatCard
                icon={<LogIn className='size-4 text-sky-500' />}
                label={t('Logins')}
                value={formatNumber(summary.logins)}
              />
              <StatCard
                icon={<UserPlus className='size-4 text-violet-500' />}
                label={t('Registrations')}
                value={formatNumber(summary.registrations)}
              />
              <StatCard
                icon={<Globe className='size-4 text-teal-500' />}
                label={t('BYOG Nodes Online')}
                value={
                  counters
                    ? `${formatNumber(counters.byog_nodes_online)} / ${formatNumber(counters.byog_nodes_total)}`
                    : '-'
                }
              />
            </div>
          ) : null}

          {/* Global counters */}
          {counters ? (
            <div className='grid grid-cols-2 gap-3 md:grid-cols-3 lg:grid-cols-5'>
              <StatCard
                icon={<Users className='size-4 text-muted-foreground' />}
                label={t('Total Users')}
                value={formatNumber(counters.total_users)}
                description={`${t('Active Users')}: ${formatNumber(counters.active_users)}`}
              />
              <StatCard
                icon={<Network className='size-4 text-muted-foreground' />}
                label={t('Total Channels')}
                value={formatNumber(counters.total_channels)}
                description={`${t('Active Channels')}: ${formatNumber(counters.active_channels)}`}
              />
              <StatCard
                icon={<KeyRound className='size-4 text-muted-foreground' />}
                label={t('API Keys')}
                value={formatNumber(counters.total_api_tokens)}
                description={`${t('Active API Keys')}: ${formatNumber(counters.active_api_tokens)}`}
              />
              <StatCard
                icon={<Ticket className='size-4 text-muted-foreground' />}
                label={t('Redemption Codes')}
                value={formatNumber(counters.total_redemptions)}
                description={`${t('Unused Codes')}: ${formatNumber(counters.unused_redemptions)}`}
              />
              <StatCard
                icon={<Globe className='size-4 text-muted-foreground' />}
                label={t('BYOG Nodes')}
                value={formatNumber(counters.byog_nodes_total)}
                description={`${t('BYOG Nodes Online')}: ${formatNumber(counters.byog_nodes_online)}`}
              />
            </div>
          ) : null}

          {/* Series chart */}
          <Card>
            <CardHeader className='pb-2'>
              <div className='flex flex-wrap items-center justify-between gap-2'>
                <div>
                  <CardTitle className='text-base'>
                    {t('Traffic Over Time')}
                  </CardTitle>
                  <CardDescription>
                    {data
                      ? `${dayjs.unix(data.range.start).format('YYYY-MM-DD HH:mm')} — ${dayjs.unix(data.range.end).format('YYYY-MM-DD HH:mm')} · ${t(data.range.granularity === 'hour' ? 'Hour' : data.range.granularity === 'day' ? 'Day' : data.range.granularity === 'week' ? 'Week' : 'Month')}`
                      : ''}
                  </CardDescription>
                </div>
                <Select
                  value={metric}
                  onValueChange={(value) => setMetric(value as SeriesMetric)}
                >
                  <SelectTrigger className='w-44'>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {SERIES_METRICS.map((entry) => (
                      <SelectItem key={entry.value} value={entry.value}>
                        {t(entry.labelKey)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </CardHeader>
            <CardContent>
              {seriesData.length > 0 ? (
                <div className='h-80'>
                  <ResponsiveContainer width='100%' height='100%'>
                    <AreaChart data={seriesData}>
                      <defs>
                        <linearGradient
                          id='statsSeriesGradient'
                          x1='0'
                          y1='0'
                          x2='0'
                          y2='1'
                        >
                          <stop offset='5%' stopColor='#3b82f6' stopOpacity={0.4} />
                          <stop offset='95%' stopColor='#3b82f6' stopOpacity={0.05} />
                        </linearGradient>
                      </defs>
                      <CartesianGrid strokeDasharray='3 3' className='stroke-muted' />
                      <XAxis
                        dataKey='label'
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
                        tickFormatter={yAxisTick}
                      />
                      <Tooltip
                        formatter={(value) => [
                          tooltipValue(Number(value)),
                          t(
                            SERIES_METRICS.find((entry) => entry.value === metric)
                              ?.labelKey ?? ''
                          ),
                        ]}
                      />
                      <Area
                        type='monotone'
                        dataKey={metric}
                        stroke='#3b82f6'
                        strokeWidth={2}
                        fill='url(#statsSeriesGradient)'
                      />
                    </AreaChart>
                  </ResponsiveContainer>
                </div>
              ) : (
                <div className='flex h-40 items-center justify-center text-sm text-muted-foreground'>
                  {t('No Data')}
                </div>
              )}
            </CardContent>
          </Card>

          {/* Distributions */}
          <div className='grid grid-cols-1 gap-4 lg:grid-cols-3'>
            <Card>
              <CardHeader className='pb-2'>
                <CardTitle className='text-base'>{t('Hour of Day')}</CardTitle>
                <CardDescription>{t('Requests')}</CardDescription>
              </CardHeader>
              <CardContent>
                <div className='h-56'>
                  <ResponsiveContainer width='100%' height='100%'>
                    <BarChart data={hourOfDayData}>
                      <CartesianGrid strokeDasharray='3 3' className='stroke-muted' />
                      <XAxis
                        dataKey='label'
                        tickLine={false}
                        axisLine={false}
                        tick={{ fontSize: 10 }}
                        interval={1}
                      />
                      <YAxis
                        tickLine={false}
                        axisLine={false}
                        tick={{ fontSize: 11 }}
                        width={44}
                        tickFormatter={(value: number) =>
                          formatCompactNumber(value)
                        }
                      />
                      <Tooltip
                        formatter={(value) => [
                          formatNumber(Number(value)),
                          t('Requests'),
                        ]}
                      />
                      <Bar dataKey='count' fill='#3b82f6' radius={[3, 3, 0, 0]} />
                    </BarChart>
                  </ResponsiveContainer>
                </div>
              </CardContent>
            </Card>
            <Card>
              <CardHeader className='pb-2'>
                <CardTitle className='text-base'>{t('Weekday')}</CardTitle>
                <CardDescription>{t('Requests')}</CardDescription>
              </CardHeader>
              <CardContent>
                <div className='h-56'>
                  <ResponsiveContainer width='100%' height='100%'>
                    <BarChart data={weekdayData}>
                      <CartesianGrid strokeDasharray='3 3' className='stroke-muted' />
                      <XAxis
                        dataKey='label'
                        tickLine={false}
                        axisLine={false}
                        tick={{ fontSize: 11 }}
                      />
                      <YAxis
                        tickLine={false}
                        axisLine={false}
                        tick={{ fontSize: 11 }}
                        width={44}
                        tickFormatter={(value: number) =>
                          formatCompactNumber(value)
                        }
                      />
                      <Tooltip
                        formatter={(value) => [
                          formatNumber(Number(value)),
                          t('Requests'),
                        ]}
                      />
                      <Bar dataKey='count' fill='#10b981' radius={[3, 3, 0, 0]} />
                    </BarChart>
                  </ResponsiveContainer>
                </div>
              </CardContent>
            </Card>
            <Card>
              <CardHeader className='pb-2'>
                <CardTitle className='text-base'>{t('Log Types')}</CardTitle>
                <CardDescription>{t('Count')}</CardDescription>
              </CardHeader>
              <CardContent>
                {logTypeData.length > 0 ? (
                  <div className='h-56'>
                    <ResponsiveContainer width='100%' height='100%'>
                      <PieChart>
                        <Pie
                          data={logTypeData}
                          dataKey='value'
                          nameKey='name'
                          cx='50%'
                          cy='50%'
                          innerRadius={45}
                          outerRadius={75}
                          paddingAngle={2}
                        >
                          {logTypeData.map((entry, index) => (
                            <Cell
                              key={entry.name}
                              fill={PIE_COLORS[index % PIE_COLORS.length]}
                            />
                          ))}
                        </Pie>
                        <Tooltip
                          formatter={(value) => [
                            formatNumber(Number(value)),
                            t('Count'),
                          ]}
                        />
                        <Legend
                          verticalAlign='bottom'
                          height={32}
                          iconSize={8}
                          formatter={(value) => (
                            <span className='text-xs'>{value}</span>
                          )}
                        />
                      </PieChart>
                    </ResponsiveContainer>
                  </div>
                ) : (
                  <div className='flex h-56 items-center justify-center text-sm text-muted-foreground'>
                    {t('No Data')}
                  </div>
                )}
              </CardContent>
            </Card>
          </div>

          {/* Rankings */}
          <Card>
            <CardHeader className='pb-2'>
              <CardTitle className='text-base'>{t('Rankings')}</CardTitle>
            </CardHeader>
            <CardContent>
              <Tabs defaultValue='users'>
                <TabsList className='flex flex-wrap'>
                  <TabsTrigger value='users'>{t('Users')}</TabsTrigger>
                  <TabsTrigger value='models'>{t('Models')}</TabsTrigger>
                  <TabsTrigger value='channels'>{t('Channels')}</TabsTrigger>
                  <TabsTrigger value='clients'>{t('Clients')}</TabsTrigger>
                  <TabsTrigger value='groups'>{t('Groups')}</TabsTrigger>
                  <TabsTrigger value='ips'>{t('Addresses')}</TabsTrigger>
                  <TabsTrigger value='byog'>{t('BYOG Nodes')}</TabsTrigger>
                </TabsList>
                <TabsContent value='users'>
                  <RankTable rows={userRows} nameHeader={t('User')} />
                </TabsContent>
                <TabsContent value='models'>
                  <RankTable rows={modelRows} nameHeader={t('Model')} />
                </TabsContent>
                <TabsContent value='channels'>
                  <RankTable rows={channelRows} nameHeader={t('Channel')} />
                </TabsContent>
                <TabsContent value='clients'>
                  <RankTable rows={clientRows} nameHeader={t('Client')} />
                </TabsContent>
                <TabsContent value='groups'>
                  <RankTable rows={groupRows} nameHeader={t('Group')} />
                </TabsContent>
                <TabsContent value='ips'>
                  <RankTable
                    rows={ipRows}
                    nameHeader={t('Address')}
                    showQuota={false}
                  />
                </TabsContent>
                <TabsContent value='byog'>
                  {byogNodes.length > 0 ? (
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>{t('Device')}</TableHead>
                          <TableHead>{t('Owner')}</TableHead>
                          <TableHead>{t('Country')}</TableHead>
                          <TableHead>{t('Status')}</TableHead>
                          <TableHead className='text-right'>
                            {t('Tokens')}
                          </TableHead>
                          <TableHead className='text-right'>
                            {t('Uptime')}
                          </TableHead>
                          <TableHead className='text-right'>
                            {t('Reputation')}
                          </TableHead>
                          <TableHead className='text-right'>
                            {t('Credits')}
                          </TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {byogNodes.map((node) => (
                          <TableRow key={node.node_id}>
                            <TableCell>
                              <div className='flex flex-col'>
                                <span className='max-w-48 truncate font-medium'>
                                  {node.device_name || node.node_id}
                                </span>
                                <span className='text-xs text-muted-foreground'>
                                  {node.device_type}
                                </span>
                              </div>
                            </TableCell>
                            <TableCell>
                              {node.username || `#${node.user_id}`}
                            </TableCell>
                            <TableCell>
                              {[node.city, node.country]
                                .filter(Boolean)
                                .join(', ') || '-'}
                            </TableCell>
                            <TableCell>
                              <Badge
                                variant='outline'
                                className={
                                  node.status === 'online'
                                    ? 'border-emerald-500/30 text-emerald-600 dark:text-emerald-400'
                                    : 'text-muted-foreground'
                                }
                              >
                                {node.status}
                              </Badge>
                            </TableCell>
                            <TableCell className='text-right tabular-nums'>
                              {formatCompactNumber(node.total_tokens_processed)}
                            </TableCell>
                            <TableCell className='text-right tabular-nums'>
                              {formatUptime(node.uptime_seconds)}
                            </TableCell>
                            <TableCell className='text-right tabular-nums'>
                              {node.reputation_score}
                            </TableCell>
                            <TableCell className='text-right tabular-nums'>
                              {formatNumber(
                                Math.round(node.total_earned_credits * 100) / 100
                              )}
                            </TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  ) : (
                    <div className='py-8 text-center text-sm text-muted-foreground'>
                      {t('No Data')}
                    </div>
                  )}
                </TabsContent>
              </Tabs>
            </CardContent>
          </Card>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
