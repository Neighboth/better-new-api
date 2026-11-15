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
import { api } from '@/lib/api'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type SiteStatsRangePreset =
  | 'today'
  | '24h'
  | '7d'
  | '30d'
  | '90d'
  | '180d'
  | '365d'
  | 'all'

export type SiteStatsGranularity = 'hour' | 'day' | 'week' | 'month'

export interface SiteStatsQuery {
  /** Range preset. Ignored when both start and end are set. */
  range?: SiteStatsRangePreset
  /** Explicit range start, unix seconds. */
  start?: number
  /** Explicit range end, unix seconds. */
  end?: number
  /** Series granularity. The server picks one when omitted. */
  granularity?: SiteStatsGranularity
  /**
   * Timezone offset of the browser in minutes — the value that
   * `new Date().getTimezoneOffset()` returns.
   */
  tz_offset?: number
  /** Comma separated list of user ids to include. */
  user_ids?: string
  /** Comma separated list of user ids to exclude. */
  exclude_user_ids?: string
  /** Model name filter, matched with LIKE. */
  model?: string
  /** Channel id filter. */
  channel_id?: number
  /** Group filter. */
  group?: string
  /** Ranking sort key. */
  sort?: 'requests' | 'tokens' | 'quota'
  /** Ranking order. */
  order?: 'asc' | 'desc'
  /** Ranking length, capped to 100 by the server. */
  limit?: number
}

export interface SiteStatsRangeInfo {
  start: number
  end: number
  granularity: SiteStatsGranularity
  bucket_size: number
  tz_offset: number
  preset: string
}

export interface SiteStatsSummary {
  requests: number
  tokens: number
  prompt_tokens: number
  completion_tokens: number
  quota: number
  unique_users: number
  avg_use_time_seconds: number
  byog_requests: number
  upstream_requests: number
  errors: number
  logins: number
  registrations: number
}

export interface SiteStatsCounters {
  total_users: number
  active_users: number
  total_channels: number
  active_channels: number
  total_api_tokens: number
  active_api_tokens: number
  total_redemptions: number
  unused_redemptions: number
  byog_nodes_total: number
  byog_nodes_online: number
}

export interface SiteStatsSeriesPoint {
  /** Bucket start, unix seconds. */
  bucket: number
  requests: number
  tokens: number
  prompt_tokens: number
  completion_tokens: number
  quota: number
  unique_users: number
  byog_requests: number
  errors: number
  registrations: number
}

/** A name with a count, used by the distributions of the BYOG network. */
export interface SiteStatsNameCount {
  name: string
  count: number
}

export interface SiteStatsDistributions {
  /** 24 request counts indexed by the local hour of the day. */
  hour_of_day: number[]
  /** 7 request counts indexed by the weekday, 0 is Sunday. */
  weekday: number[]
  /** Node counts per device type of the whole BYOG network. */
  byog_device_types: SiteStatsNameCount[]
  /** Node counts per country of the whole BYOG network. */
  byog_countries: SiteStatsNameCount[]
}

export interface SiteStatsTopUser {
  user_id: number
  username: string
  requests: number
  tokens: number
  quota: number
}

export interface SiteStatsTopModel {
  model_name: string
  requests: number
  tokens: number
  quota: number
}

export interface SiteStatsTopChannel {
  channel_id: number
  channel_name: string
  requests: number
  tokens: number
  quota: number
}

export interface SiteStatsTopClient {
  token_name: string
  requests: number
  tokens: number
  quota: number
}

export interface SiteStatsTopGroup {
  group_name: string
  requests: number
  tokens: number
  quota: number
}

export interface SiteStatsTopIp {
  ip: string
  requests: number
  tokens: number
}

export interface SiteStatsByogNodeRank {
  node_id: string
  device_name: string
  device_type: string
  user_id: number
  username?: string
  country: string
  city: string
  status: string
  total_tokens_processed: number
  uptime_seconds: number
  reputation_score: number
  total_earned_credits: number
  last_heartbeat_at: number
}

export interface SiteStatsLogTypeCount {
  type: number
  count: number
}

export interface SiteStatsApplication {
  client_name: string
  requests: number
  tokens: number
  quota: number
}

export interface SiteStatsOption {
  id: number
  name: string
}

export async function searchSiteStatsOptions(
  kind: 'user' | 'channel',
  keyword: string
): Promise<SiteStatsOption[]> {
  const res = await api.get(
    `/api/statistics/site/${kind === 'user' ? 'users' : 'channels'}`,
    { params: { keyword } }
  )
  if (!res.data.success || !Array.isArray(res.data.data)) {
    throw new Error(res.data.message || 'Failed to load')
  }
  return res.data.data
}

export interface SiteStatsRankings {
  users: SiteStatsTopUser[]
  models: SiteStatsTopModel[]
  channels: SiteStatsTopChannel[]
  api_keys: SiteStatsTopClient[]
  clients: SiteStatsApplication[]
  groups: SiteStatsTopGroup[]
  ips: SiteStatsTopIp[]
  byog_nodes: SiteStatsByogNodeRank[]
  log_types: SiteStatsLogTypeCount[]
}

export interface SiteStatisticsResponse {
  range: SiteStatsRangeInfo
  summary: SiteStatsSummary
  counters: SiteStatsCounters
  series: SiteStatsSeriesPoint[]
  distributions: SiteStatsDistributions
  rankings: SiteStatsRankings
}

// ---------------------------------------------------------------------------
// API
// ---------------------------------------------------------------------------

export async function getSiteStatistics(
  params: SiteStatsQuery
): Promise<SiteStatisticsResponse> {
  const res = await api.get('/api/statistics/site', { params })
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.message || 'Failed to load statistics')
  }
  return res.data.data as SiteStatisticsResponse
}

/**
 * The group names of the installation for the filter picker.
 */
export async function getSiteStatsGroupOptions(): Promise<string[]> {
  const res = await api.get('/api/statistics/site/groups')
  if (!res.data.success || !Array.isArray(res.data.data)) {
    throw new Error(res.data.message || 'Failed to load groups')
  }
  return res.data.data as string[]
}

// ---------------------------------------------------------------------------
// External & Integration Analytics APIs
// ---------------------------------------------------------------------------

export interface ClarityInsightsResponse {
  configured: boolean
  message?: string
  source?: string
  project_id?: string
  cached_at?: number
  summary?: {
    total_sessions: number
    distinct_users: number
    pages_per_session: number
    scroll_depth: number
    active_time_seconds: number
  }
  friction?: {
    rage_clicks: number
    rage_click_rate: number
    dead_clicks: number
    dead_click_rate: number
    excessive_scrolling: number
    excessive_scroll_rate: number
    quick_backs: number
    quick_back_rate: number
    javascript_errors: number
  }
  devices?: Array<{ device: string; sessions: number; percentage: number }>
  browsers?: Array<{ browser: string; sessions: number; percentage: number }>
  operating_systems?: Array<{ os: string; percentage: number }>
  top_pages?: Array<{ url: string; sessions: number; scroll_depth: number }>
  countries?: Array<{ country: string; code: string; sessions: number; percentage: number }>
  insights?: any
}

export interface GoogleAnalyticsReportResponse {
  configured: boolean
  message?: string
  ga_id?: string
  property_id?: string
  cached_at?: number
  summary?: {
    active_users: number
    new_users: number
    sessions: number
    page_views: number
    average_session_duration_s: number
    bounce_rate_percentage: number
  }
  acquisition_channels?: Array<{
    channel: string
    users: number
    sessions: number
    percentage: number
  }>
  top_landing_pages?: Array<{
    page: string
    views: number
    users: number
    avg_duration_s: number
  }>
  device_categories?: Array<{
    category: string
    users: number
    percentage: number
  }>
  user_countries?: Array<{
    country: string
    users: number
    sessions: number
  }>
}

export interface SearchConsoleQueryItem {
  query: string
  clicks: number
  impressions: number
  ctr: number
  position: number
}

export interface SearchConsoleReportResponse {
  configured: boolean
  message?: string
  site_url?: string
  cached_at?: number
  summary?: {
    total_clicks: number
    total_impressions: number
    average_ctr: number
    average_position: number
  }
  queries?: {
    items: SearchConsoleQueryItem[]
    total_count: number
    page: number
    page_size: number
    total_pages: number
  }
  top_search_pages?: Array<{
    page: string
    clicks: number
    impressions: number
    ctr: number
    position: number
  }>
  device_distribution?: Array<{
    device: string
    clicks: number
    impressions: number
    ctr: number
  }>
}

export interface ProxyDailyPoint {
  date: string
  mb: number
  requests: number
}

export interface ProxyPoolStatsResponse {
  daily_history: ProxyDailyPoint[]
  proxies: Array<{
    id: number
    name: string
    url: string
    enabled: boolean
    assigned_keys?: number
    latency_ms?: number
    egress_ip?: string
    last_test_status?: string
    last_tested_at?: number
  }>
}

export async function getClarityInsights(): Promise<ClarityInsightsResponse> {
  const res = await api.get('/api/statistics/clarity')
  return res.data.data
}

export async function getGoogleAnalyticsReport(): Promise<GoogleAnalyticsReportResponse> {
  const res = await api.get('/api/statistics/google-analytics')
  return res.data.data
}

export async function getGoogleSearchConsoleReport(params?: {
  page?: number
  page_size?: number
  query?: string
}): Promise<SearchConsoleReportResponse> {
  const res = await api.get('/api/statistics/google-search-console', { params })
  return res.data.data
}

export async function getProxyPoolStats(): Promise<ProxyPoolStatsResponse> {
  const res = await api.get('/api/statistics/proxy-pool')
  return res.data.data
}
