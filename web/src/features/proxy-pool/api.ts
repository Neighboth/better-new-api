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

export interface ProxyPoolProxy {
  id: number
  name: string
  url: string
  proxy_type?: string
  enabled: boolean
  last_test_status: string
  last_test_error: string
  last_tested_at: number
  latency_ms?: number
  egress_ip?: string
  country_code?: string
  country_name?: string
  city?: string
  isp?: string
  assigned_keys?: number
  created_at: number
  updated_at: number
}

export interface ProxyPoolChannel {
  id: number
  name: string
  type: number
  status: number
  group: string
  multi_key_size: number
  assigned_keys: number
}

export interface ProxyPoolAssignment {
  id: number
  channel_id: number
  key_index: number
  proxy_id: number
  created_at: number
  updated_at: number
}

export interface ProxyPoolAssignResult {
  channel_id: number
  key_count: number
  proxy_count: number
  assigned: number
  per_proxy: number
}

export interface ProxyPoolTestResult {
  id: number
  ok: boolean
  egress_ip?: string
  latency_ms?: number
  error?: string
}

export interface ProxyPoolListResult {
  items: ProxyPoolProxy[]
  total: number
  page: number
  page_size: number
  total_pages: number
  enabled_count: number
}

interface ApiEnvelope<T> {
  success: boolean
  message?: string
  data: T
}

export async function listProxies(params?: {
  page?: number
  page_size?: number
  keyword?: string
  status?: string
}): Promise<ApiEnvelope<ProxyPoolListResult | ProxyPoolProxy[]>> {
  const res = await api.get<ApiEnvelope<ProxyPoolListResult | ProxyPoolProxy[]>>(
    '/api/proxy-pool/',
    { params }
  )
  return res.data
}

export async function batchCreateProxies(payload: {
  base_name?: string
  enabled?: boolean
  proxies: Array<{ name?: string; url: string; enabled?: boolean }>
}): Promise<ApiEnvelope<{ count: number }>> {
  const res = await api.post<ApiEnvelope<{ count: number }>>(
    '/api/proxy-pool/batch',
    payload
  )
  return res.data
}

export async function createProxy(payload: {
  name: string
  url: string
  enabled: boolean
}): Promise<ApiEnvelope<ProxyPoolProxy>> {
  const res = await api.post<ApiEnvelope<ProxyPoolProxy>>(
    '/api/proxy-pool/',
    payload
  )
  return res.data
}

export async function updateProxy(
  id: number,
  payload: { name?: string; url?: string; enabled?: boolean }
): Promise<ApiEnvelope<ProxyPoolProxy>> {
  const res = await api.put<ApiEnvelope<ProxyPoolProxy>>(
    `/api/proxy-pool/${id}`,
    payload
  )
  return res.data
}

export async function deleteProxy(id: number): Promise<ApiEnvelope<object>> {
  const res = await api.delete<ApiEnvelope<object>>(`/api/proxy-pool/${id}`)
  return res.data
}

export async function testProxy(
  id: number
): Promise<ApiEnvelope<ProxyPoolTestResult>> {
  const res = await api.post<ApiEnvelope<ProxyPoolTestResult>>(
    `/api/proxy-pool/${id}/test`,
    {}
  )
  return res.data
}

export async function listChannels(): Promise<ApiEnvelope<ProxyPoolChannel[]>> {
  const res = await api.get<ApiEnvelope<ProxyPoolChannel[]>>(
    '/api/proxy-pool/channels'
  )
  return res.data
}

export interface ProxyChannelAssignmentDetail {
  key_index: number
  proxy_id: number
  proxy_name: string
  proxy_url: string
  proxy_enabled: boolean
  egress_ip: string
  latency_ms: number
  last_test_status: string
  last_test_error: string
  last_tested_at: number
  country_code: string
  country_name: string
}

export interface ProxyPoolEvent {
  id: number
  channel_id: number
  key_index: number
  proxy_id: number
  proxy_url: string
  kind: string
  message: string
  created_at: number
}

export interface ProxyChannelDetailData {
  channel: {
    id: number
    name: string
    type: number
    status: number
    group: string
    multi_key_size: number
  }
  assignments: ProxyChannelAssignmentDetail[]
  /** Total number of assigned proxies across all pages (server-side). */
  assignments_total: number
  /** 1-based page the `assignments` list was sliced from. */
  assignments_page: number
  assignments_page_size: number
  cooldowns: Record<string, number>
  events: ProxyPoolEvent[]
  global_events: ProxyPoolEvent[]
}

export async function getChannelDetail(
  channelId: number,
  assignmentsPage = 1,
  assignmentsPageSize = 20
): Promise<ApiEnvelope<ProxyChannelDetailData>> {
  const res = await api.get<ApiEnvelope<ProxyChannelDetailData>>(
    `/api/proxy-pool/channels/${channelId}/detail?assignments_page=${assignmentsPage}&assignments_page_size=${assignmentsPageSize}`
  )
  return res.data
}

export async function listAssignments(
  channelId?: number
): Promise<ApiEnvelope<ProxyPoolAssignment[]>> {
  const query = channelId ? `?channel_id=${channelId}` : ''
  const res = await api.get<ApiEnvelope<ProxyPoolAssignment[]>>(
    `/api/proxy-pool/assignments${query}`
  )
  return res.data
}

export async function assignProxiesEvenly(
  channelId: number
): Promise<ApiEnvelope<ProxyPoolAssignResult>> {
  const res = await api.post<ApiEnvelope<ProxyPoolAssignResult>>(
    '/api/proxy-pool/assign',
    { channel_id: channelId }
  )
  return res.data
}

export async function clearAssignments(
  channelId: number
): Promise<ApiEnvelope<object>> {
  const res = await api.delete<ApiEnvelope<object>>(
    `/api/proxy-pool/assignments?channel_id=${channelId}`
  )
  return res.data
}

export interface ProxyAssignmentDetail {
  channel_id: number
  channel_name: string
  channel_type: number
  key_index: number
}

export interface ProxyDailyMBPoint {
  date: string
  mb: number
  requests: number
}

export interface ProxyDetailData {
  proxy: ProxyPoolProxy
  assigned_keys: number
  assignments: ProxyAssignmentDetail[]
  usage_history: ProxyDailyMBPoint[]
}

export async function getProxyDetail(
  id: number
): Promise<ApiEnvelope<ProxyDetailData>> {
  const res = await api.get<ApiEnvelope<ProxyDetailData>>(
    `/api/proxy-pool/${id}/detail`
  )
  return res.data
}

export async function unassignProxy(
  id: number
): Promise<ApiEnvelope<{ proxy_id: number }>> {
  const res = await api.post<ApiEnvelope<{ proxy_id: number }>>(
    `/api/proxy-pool/${id}/unassign`,
    {}
  )
  return res.data
}

export async function rebalanceAllChannels(): Promise<
  ApiEnvelope<{
    rebalanced_channels: number
    total_keys_assigned: number
    proxy_count: number
  }>
> {
  const res = await api.post<
    ApiEnvelope<{
      rebalanced_channels: number
      total_keys_assigned: number
      proxy_count: number
    }>
  >('/api/proxy-pool/rebalance', {})
  return res.data
}

export async function getProxyPoolUsageStats(): Promise<
  ApiEnvelope<{
    daily_history: ProxyDailyMBPoint[]
    proxies: ProxyPoolProxy[]
  }>
> {
  const res = await api.get<
    ApiEnvelope<{
      daily_history: ProxyDailyMBPoint[]
      proxies: ProxyPoolProxy[]
    }>
  >('/api/proxy-pool/usage-stats')
  return res.data
}
