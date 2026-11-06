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
  useRef,
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
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
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
  getSiteStatsGroupOptions,
  searchSiteStatsOptions,
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
        <CardTitle className='text-muted-foreground flex items-center gap-2 text-sm font-medium'>
          <span className='flex size-4 shrink-0 items-center justify-center'>
            {icon}
          </span>
          <span className='truncate'>{label}</span>
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div className='truncate text-2xl font-bold tabular-nums'>{value}</div>
        {description ? (
          <p className='text-muted-foreground mt-1 truncate text-xs'>
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
      <div className='text-muted-foreground py-8 text-center text-sm'>
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
            <TableCell className='text-muted-foreground tabular-nums'>
              {index + 1}
            </TableCell>
            <TableCell>
              <div className='flex flex-col'>
                <span className='max-w-56 truncate font-medium'>
                  {row.name}
                </span>
                {row.detail ? (
                  <span className='text-muted-foreground text-xs'>
                    {row.detail}
                  </span>
                ) : null}
              </div>
            </TableCell>
            <TableCell>
              <div className='flex items-center gap-2'>
                <div className='bg-muted h-1.5 w-20 shrink-0 overflow-hidden rounded-full'>
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

type NamedOption = { id: string; name: string }

function NamePicker({
  label,
  kind,
  selected,
  onChange,
  multiple = false,
}: {
  label: string
  kind: 'user' | 'channel'
  selected: NamedOption[]
  onChange: (items: NamedOption[]) => void
  multiple?: boolean
}) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [options, setOptions] = useState<NamedOption[]>([])
  const [pending, setPending] = useState(false)
  const [failed, setFailed] = useState(false)
  useEffect(() => {
    if (!open) return
    let active = true
    setPending(true)
    const timer = setTimeout(async () => {
      try {
        const items = await searchSiteStatsOptions(kind, query)
        if (!active) return
        setOptions(
          items.map((item) => ({ id: String(item.id), name: item.name }))
        )
        setFailed(false)
      } catch {
        if (active) {
          setOptions([])
          setFailed(true)
        }
      } finally {
        if (active) setPending(false)
      }
    }, 300)
    return () => {
      active = false
      clearTimeout(timer)
    }
  }, [open, query, kind])
  return (
    <div className='space-y-1'>
      <span className='text-muted-foreground text-xs'>{label}</span>
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger
          render={
            <Button
              variant='outline'
              className='w-full justify-start truncate'
            />
          }
        >
          <span className='truncate'>
            {selected.length
              ? selected.map((x) => x.name || `#${x.id}`).join(', ')
              : label}
          </span>
        </PopoverTrigger>
        <PopoverContent align='start' className='w-80 max-w-[90vw]'>
          <Input
            aria-label={label}
            placeholder={t('Search...')}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
          {selected.length > 0 && (
            <div className='flex flex-wrap gap-1'>
              {selected.map((item) => (
                <Button
                  key={item.id}
                  size='sm'
                  variant='secondary'
                  onClick={() =>
                    onChange(selected.filter((x) => x.id !== item.id))
                  }
                >
                  {item.name || `#${item.id}`} ×
                </Button>
              ))}
            </div>
          )}
          <div className='max-h-52 overflow-y-auto'>
            {pending ? (
              <p className='p-2 text-sm'>{t('Loading...')}</p>
            ) : failed ? (
              <p className='text-destructive p-2 text-sm'>
                {t('Failed to load')}
              </p>
            ) : options.length === 0 ? (
              <p className='text-muted-foreground p-2 text-sm'>
                {t('No Data')}
              </p>
            ) : (
              options.map((item) => (
                <button
                  type='button'
                  key={item.id}
                  className='hover:bg-muted w-full rounded px-2 py-2 text-left text-sm'
                  onClick={() => {
                    onChange(
                      multiple
                        ? selected.some((x) => x.id === item.id)
                          ? selected.filter((x) => x.id !== item.id)
                          : [...selected, item]
                        : [item]
                    )
                    if (!multiple) setOpen(false)
                  }}
                >
                  {selected.some((x) => x.id === item.id) ? '✓ ' : ''}
                  {item.name || t('Unknown')}{' '}
                  <span className='text-muted-foreground'>#{item.id}</span>
                </button>
              ))
            )}
          </div>
        </PopoverContent>
      </Popover>
    </div>
  )
}

export function SiteStatistics() {
  const { t } = useTranslation()
  const [loading, setLoading] = useState(true)
  const [data, setData] = useState<SiteStatisticsResponse | null>(null)

  // The selects reload on change, the text inputs reload only through the
  // apply button so that typing does not fire a request per keystroke.
  const [rangePreset, setRangePreset] = useState<SiteStatsRangePreset>('30d')
  const [granularity, setGranularity] = useState<'auto' | SiteStatsGranularity>(
    'auto'
  )
  const [sort, setSort] = useState<'requests' | 'tokens' | 'quota'>('requests')
  const [order, setOrder] = useState<'asc' | 'desc'>('desc')
  const [limit, setLimit] = useState(20)
  const [textDrafts, setTextDrafts] = useState<TextFilters>(EMPTY_TEXT_FILTERS)
  const [textFilters, setTextFilters] =
    useState<TextFilters>(EMPTY_TEXT_FILTERS)

  const [included, setIncluded] = useState<NamedOption[]>([])
  const [excluded, setExcluded] = useState<NamedOption[]>([])
  const [channel, setChannel] = useState<NamedOption[]>([])
  const [groups, setGroups] = useState<string[]>([])
  const [error, setError] = useState(false)
  const requestId = useRef(0)
  useEffect(() => {
    getSiteStatsGroupOptions()
      .then(setGroups)
      .catch(() => toast.error(t('Failed to load')))
  }, [t])

  const load = useCallback(async () => {
    const current = ++requestId.current
    setLoading(true)
    setError(false)
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
      if (current === requestId.current) setData(res)
    } catch {
      if (current !== requestId.current) return
      setError(true)
      toast.error(t('Failed to load statistics'))
    } finally {
      if (current === requestId.current) setLoading(false)
    }
  }, [rangePreset, granularity, sort, order, limit, textFilters, t])

  useEffect(() => {
    load()
  }, [load])

  const applyFilters = () => {
    setTextFilters({
      ...textDrafts,
      includeUsers: included.map((x) => x.id).join(','),
      excludeUsers: excluded.map((x) => x.id).join(','),
      channelId: channel[0]?.id ?? '',
    })
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
      prompt_tokens: point.prompt_tokens,
      completion_tokens: point.completion_tokens,
      byog_requests: point.byog_requests,
      upstream_requests: Math.max(0, point.requests - point.byog_requests),
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
  const clientRows: RankRow[] = (data?.rankings.api_keys ?? []).map((row) => ({
    key: `client-${row.token_name}`,
    name: row.token_name || t('Unknown'),
    requests: row.requests,
    tokens: row.tokens,
    quota: row.quota,
  }))
  const applicationRows: RankRow[] = (data?.rankings.clients ?? []).map(
    (row) => ({
      key: `application-${row.client_name}`,
      name: row.client_name || t('Unknown'),
      requests: row.requests,
      tokens: row.tokens,
      quota: row.quota,
    })
  )
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
          <p className='text-muted-foreground text-sm'>
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
                  <label className='text-muted-foreground text-xs'>
                    {t('Time Range')}
                  </label>
                  <Select
                    value={rangePreset}
                    onValueChange={(value) =>
                      setRangePreset(value as SiteStatsRangePreset)
                    }
                  >
                    <SelectTrigger className='w-full'>
                      <SelectValue>
                        {t(RANGE_LABEL_KEYS[rangePreset])}
                      </SelectValue>
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
                  <label className='text-muted-foreground text-xs'>
                    {t('Granularity')}
                  </label>
                  <Select
                    value={granularity}
                    onValueChange={(value) =>
                      setGranularity(value as 'auto' | SiteStatsGranularity)
                    }
                  >
                    <SelectTrigger className='w-full'>
                      <SelectValue>
                        {t(
                          (
                            {
                              auto: 'Auto',
                              hour: 'Hour',
                              day: 'Day',
                              week: 'Week',
                              month: 'Month',
                            } as const
                          )[granularity]
                        )}
                      </SelectValue>
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
                  <label className='text-muted-foreground text-xs'>
                    {t('Sort')}
                  </label>
                  <Select
                    value={sort}
                    onValueChange={(value) =>
                      setSort(value as 'requests' | 'tokens' | 'quota')
                    }
                  >
                    <SelectTrigger className='w-full'>
                      <SelectValue>
                        {t(
                          (
                            {
                              requests: 'Requests',
                              tokens: 'Tokens',
                              quota: 'Quota Used',
                            } as const
                          )[sort]
                        )}
                      </SelectValue>
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value='requests'>{t('Requests')}</SelectItem>
                      <SelectItem value='tokens'>{t('Tokens')}</SelectItem>
                      <SelectItem value='quota'>{t('Quota Used')}</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div className='space-y-1'>
                  <label className='text-muted-foreground text-xs'>
                    {t('Sort Order')}
                  </label>
                  <Select
                    value={order}
                    onValueChange={(value) => setOrder(value as 'asc' | 'desc')}
                  >
                    <SelectTrigger className='w-full'>
                      <SelectValue>
                        {t(order === 'desc' ? 'Descending' : 'Ascending')}
                      </SelectValue>
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value='desc'>{t('Descending')}</SelectItem>
                      <SelectItem value='asc'>{t('Ascending')}</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div className='space-y-1'>
                  <label className='text-muted-foreground text-xs'>
                    {t('Top Count')}
                  </label>
                  <Select
                    value={String(limit)}
                    onValueChange={(value) => setLimit(Number(value))}
                  >
                    <SelectTrigger className='w-full'>
                      <SelectValue>{String(limit)}</SelectValue>
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
                <NamePicker
                  label={t('Include Users')}
                  kind='user'
                  selected={included}
                  onChange={setIncluded}
                  multiple
                />
                <NamePicker
                  label={t('Exclude Users')}
                  kind='user'
                  selected={excluded}
                  onChange={setExcluded}
                  multiple
                />
                <div className='space-y-1'>
                  <label
                    htmlFor='stats-model'
                    className='text-muted-foreground text-xs'
                  >
                    {t('Model')}
                  </label>
                  <Input
                    id='stats-model'
                    placeholder={t('Model')}
                    value={textDrafts.model}
                    onChange={(e) =>
                      setTextDrafts((d) => ({ ...d, model: e.target.value }))
                    }
                  />
                </div>
                <NamePicker
                  label={t('Channel')}
                  kind='channel'
                  selected={channel}
                  onChange={setChannel}
                />
                <div className='space-y-1'>
                  <label className='text-muted-foreground text-xs'>
                    {t('Group')}
                  </label>
                  <Select
                    value={textDrafts.group || '__all__'}
                    onValueChange={(value) =>
                      setTextDrafts((d) => ({
                        ...d,
                        group: value === '__all__' ? '' : (value ?? ''),
                      }))
                    }
                  >
                    <SelectTrigger className='w-full'>
                      <SelectValue>
                        {textDrafts.group || t('All Groups')}
                      </SelectValue>
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value='__all__'>{t('All Groups')}</SelectItem>
                      {groups.map((group) => (
                        <SelectItem key={group} value={group}>
                          {group}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              </div>
              <div className='flex justify-end gap-2'>
                <Button
                  size='sm'
                  variant='outline'
                  onClick={() => {
                    setIncluded([])
                    setExcluded([])
                    setChannel([])
                    setTextDrafts(EMPTY_TEXT_FILTERS)
                    setTextFilters(EMPTY_TEXT_FILTERS)
                  }}
                >
                  {t('Reset')}
                </Button>
                <Button size='sm' onClick={applyFilters}>
                  {t('Apply')}
                </Button>
              </div>
            </CardContent>
          </Card>

          {loading && (
            <div
              role='status'
              className='text-muted-foreground flex items-center gap-2 text-sm'
            >
              <RefreshCw className='size-4 animate-spin' />
              {t('Loading...')}
            </div>
          )}
          {error && (
            <div
              role='alert'
              className='text-destructive flex items-center gap-3'
            >
              {t('Failed to load statistics')}{' '}
              <Button variant='outline' size='sm' onClick={load}>
                {t('Retry')}
              </Button>
            </div>
          )}
          {loading && !data && (
            <div className='grid grid-cols-2 gap-3 md:grid-cols-5'>
              {Array.from({ length: 10 }, (_, i) => (
                <Skeleton key={i} className='h-28' />
              ))}
            </div>
          )}
          <div
            className={
              loading ? 'pointer-events-none space-y-4 opacity-60' : 'space-y-4'
            }
            aria-busy={loading}
          >
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
                  icon={<Zap className='size-4 text-amber-500' />}
                  label={t('Prompt Tokens')}
                  value={formatTokens(summary.prompt_tokens)}
                />
                <StatCard
                  icon={<Zap className='size-4 text-violet-500' />}
                  label={t('Completion Tokens')}
                  value={formatTokens(summary.completion_tokens)}
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
                  icon={<Users className='text-muted-foreground size-4' />}
                  label={t('Total Users')}
                  value={formatNumber(counters.total_users)}
                  description={`${t('Active Users')}: ${formatNumber(counters.active_users)}`}
                />
                <StatCard
                  icon={<Network className='text-muted-foreground size-4' />}
                  label={t('Total Channels')}
                  value={formatNumber(counters.total_channels)}
                  description={`${t('Active Channels')}: ${formatNumber(counters.active_channels)}`}
                />
                <StatCard
                  icon={<KeyRound className='text-muted-foreground size-4' />}
                  label={t('API Keys')}
                  value={formatNumber(counters.total_api_tokens)}
                  description={`${t('Active API Keys')}: ${formatNumber(counters.active_api_tokens)}`}
                />
                <StatCard
                  icon={<Ticket className='text-muted-foreground size-4' />}
                  label={t('Redemption Codes')}
                  value={formatNumber(counters.total_redemptions)}
                  description={`${t('Unused Codes')}: ${formatNumber(counters.unused_redemptions)}`}
                />
                <StatCard
                  icon={<Globe className='text-muted-foreground size-4' />}
                  label={t('BYOG Nodes')}
                  value={formatNumber(counters.byog_nodes_total)}
                  description={`${t('BYOG Nodes Online')}: ${formatNumber(counters.byog_nodes_online)}`}
                />
              </div>
            ) : null}

            {/* All time series are visible together. */}
            <div className='grid grid-cols-1 gap-4 xl:grid-cols-2'>
              {[
                {
                  title: t('Requests'),
                  keys: [
                    { key: 'requests', label: t('Requests'), color: '#3b82f6' },
                  ],
                },
                {
                  title: t('Tokens'),
                  keys: [
                    {
                      key: 'prompt_tokens',
                      label: t('Prompt Tokens'),
                      color: '#8b5cf6',
                    },
                    {
                      key: 'completion_tokens',
                      label: t('Completion Tokens'),
                      color: '#10b981',
                    },
                  ],
                },
                {
                  title: t('Quota Used'),
                  keys: [
                    { key: 'quota', label: t('Quota Used'), color: '#f59e0b' },
                  ],
                },
                {
                  title: t('Unique Users'),
                  keys: [
                    {
                      key: 'unique_users',
                      label: t('Unique Users'),
                      color: '#6366f1',
                    },
                  ],
                },
                {
                  title: t('Errors'),
                  keys: [
                    { key: 'errors', label: t('Errors'), color: '#ef4444' },
                  ],
                },
                {
                  title: t('Registrations'),
                  keys: [
                    {
                      key: 'registrations',
                      label: t('Registrations'),
                      color: '#14b8a6',
                    },
                  ],
                },
                {
                  title: `${t('BYOG Requests')} / ${t('Upstream Requests')}`,
                  keys: [
                    {
                      key: 'byog_requests',
                      label: t('BYOG Requests'),
                      color: '#10b981',
                    },
                    {
                      key: 'upstream_requests',
                      label: t('Upstream Requests'),
                      color: '#3b82f6',
                    },
                  ],
                },
              ].map((chart) => (
                <Card key={chart.title}>
                  <CardHeader className='pb-2'>
                    <CardTitle className='text-base'>{chart.title}</CardTitle>
                    <CardDescription>{t('Traffic Over Time')}</CardDescription>
                  </CardHeader>
                  <CardContent>
                    {seriesData.length ? (
                      <div className='h-64'>
                        <ResponsiveContainer width='100%' height='100%'>
                          <AreaChart data={seriesData}>
                            <CartesianGrid
                              strokeDasharray='3 3'
                              className='stroke-muted'
                            />
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
                              tickFormatter={(value: number) =>
                                chart.title === t('Quota Used')
                                  ? formatQuota(value)
                                  : formatCompactNumber(value)
                              }
                            />
                            <Tooltip
                              formatter={(value) =>
                                chart.title === t('Quota Used')
                                  ? formatQuota(Number(value))
                                  : formatNumber(Number(value))
                              }
                            />
                            {chart.keys.length > 1 && <Legend />}
                            {chart.keys.map((entry) => (
                              <Area
                                key={entry.key}
                                type='monotone'
                                dataKey={entry.key}
                                name={entry.label}
                                stroke={entry.color}
                                fill={entry.color}
                                fillOpacity={0.25}
                                stackId={
                                  chart.keys.length > 1 ? 'stack' : undefined
                                }
                              />
                            ))}
                          </AreaChart>
                        </ResponsiveContainer>
                      </div>
                    ) : (
                      <div className='text-muted-foreground flex h-40 items-center justify-center text-sm'>
                        {t('No Data')}
                      </div>
                    )}
                  </CardContent>
                </Card>
              ))}
            </div>

            {/* Distributions */}
            <div className='grid grid-cols-1 gap-4 lg:grid-cols-3'>
              <Card>
                <CardHeader className='pb-2'>
                  <CardTitle className='text-base'>
                    {t('Hour of Day')}
                  </CardTitle>
                  <CardDescription>{t('Requests')}</CardDescription>
                </CardHeader>
                <CardContent>
                  <div className='h-56'>
                    <ResponsiveContainer width='100%' height='100%'>
                      <BarChart data={hourOfDayData}>
                        <CartesianGrid
                          strokeDasharray='3 3'
                          className='stroke-muted'
                        />
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
                        <Bar
                          dataKey='count'
                          fill='#3b82f6'
                          radius={[3, 3, 0, 0]}
                        />
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
                        <CartesianGrid
                          strokeDasharray='3 3'
                          className='stroke-muted'
                        />
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
                        <Bar
                          dataKey='count'
                          fill='#10b981'
                          radius={[3, 3, 0, 0]}
                        />
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
                    <div className='text-muted-foreground flex h-56 items-center justify-center text-sm'>
                      {t('No Data')}
                    </div>
                  )}
                </CardContent>
              </Card>
            </div>

            <div className='grid grid-cols-1 gap-4 lg:grid-cols-2'>
              {[
                {
                  title: t('BYOG Device Types'),
                  rows: data?.distributions.byog_device_types ?? [],
                },
                {
                  title: t('BYOG Countries'),
                  rows: data?.distributions.byog_countries ?? [],
                },
              ].map((chart) => (
                <Card key={chart.title}>
                  <CardHeader>
                    <CardTitle className='text-base'>{chart.title}</CardTitle>
                    <CardDescription>
                      {t('Global BYOG network (not affected by filters)')}
                    </CardDescription>
                  </CardHeader>
                  <CardContent>
                    {chart.rows.length ? (
                      <div className='h-56'>
                        <ResponsiveContainer width='100%' height='100%'>
                          <BarChart data={chart.rows}>
                            <CartesianGrid
                              strokeDasharray='3 3'
                              className='stroke-muted'
                            />
                            <XAxis dataKey='name' tick={{ fontSize: 10 }} />
                            <YAxis
                              tickFormatter={(value: number) =>
                                formatCompactNumber(value)
                              }
                            />
                            <Tooltip />
                            <Bar
                              dataKey='count'
                              name={t('Count')}
                              fill='#10b981'
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
              ))}
            </div>

            {/* Rankings are simultaneously visible. */}
            <div className='grid grid-cols-1 gap-4 xl:grid-cols-2'>
              {[
                { title: t('Users'), rows: userRows, header: t('User') },
                { title: t('Models'), rows: modelRows, header: t('Model') },
                {
                  title: t('Channels'),
                  rows: channelRows,
                  header: t('Channel'),
                },
                {
                  title: t('API Keys'),
                  rows: clientRows,
                  header: t('API Key'),
                },
                {
                  title: t('Clients'),
                  rows: applicationRows,
                  header: t('Client'),
                  description:
                    t(
                      'Client names come from request headers; older requests may be unknown.'
                    ) +
                    ' ' +
                    t(
                      'Client headers can be spoofed and are not proof of identity.'
                    ),
                },
                { title: t('Groups'), rows: groupRows, header: t('Group') },
                {
                  title: t('Addresses'),
                  rows: ipRows,
                  header: t('Address'),
                  showQuota: false,
                },
              ].map((rank) => (
                <Card key={rank.title}>
                  <CardHeader>
                    <CardTitle className='text-base'>{rank.title}</CardTitle>
                    {rank.description && (
                      <CardDescription>{rank.description}</CardDescription>
                    )}
                  </CardHeader>
                  <CardContent className='overflow-x-auto'>
                    <RankTable
                      rows={rank.rows}
                      nameHeader={rank.header}
                      showQuota={rank.showQuota}
                    />
                  </CardContent>
                </Card>
              ))}
            </div>
            <Card>
              <CardHeader>
                <CardTitle className='text-base'>{t('BYOG Nodes')}</CardTitle>
              </CardHeader>
              <CardContent className='overflow-x-auto'>
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
                              <span className='text-muted-foreground text-xs'>
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
                  <div className='text-muted-foreground py-8 text-center text-sm'>
                    {t('No Data')}
                  </div>
                )}
              </CardContent>
            </Card>
          </div>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
