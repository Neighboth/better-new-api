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
  'today' | '24h' | '7d' | '30d' | '90d' | '180d' | '365d' | 'all'

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

export interface SiteStatsRankings {
  users: SiteStatsTopUser[]
  models: SiteStatsTopModel[]
  channels: SiteStatsTopChannel[]
  clients: SiteStatsTopClient[]
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
  params: SiteStatsQuery,
): Promise<SiteStatisticsResponse> {
  const res = await api.get('/api/statistics/site', { params })
  if (!res.data.success || !res.data.data)
    throw new Error(res.data.message || 'Failed to load statistics')
  return res.data.data as SiteStatisticsResponse
}

/**
 * The group names of the installation for the filter picker.
 */
export async function getSiteStatsGroupOptions(): Promise<string[]> {
  const res = await api.get('/api/group/')
  if (!res.data.success || !Array.isArray(res.data.data))
    throw new Error(res.data.message || 'Failed to load groups')
  return res.data.data as string[]
}
