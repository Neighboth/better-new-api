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
  enabled: boolean
  last_test_status: string
  last_test_error: string
  last_tested_at: number
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

interface ApiEnvelope<T> {
  success: boolean
  message?: string
  data: T
}

export async function listProxies(): Promise<ApiEnvelope<ProxyPoolProxy[]>> {
  const res = await api.get<ApiEnvelope<ProxyPoolProxy[]>>('/api/proxy-pool/')
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
