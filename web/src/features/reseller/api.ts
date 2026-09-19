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
import type { ResellerConfig, ResellerRedemption, ResellerSummary } from './types'

export async function fetchResellerSummary(): Promise<ResellerSummary> {
  const res = await api.get('/api/reseller/summary')
  if (!res.data?.success) {
    throw new Error(res.data?.message || 'Failed to fetch reseller summary')
  }
  return res.data.data
}

export async function fetchResellerConfig(): Promise<ResellerConfig> {
  const res = await api.get('/api/reseller/self')
  if (!res.data?.success) {
    throw new Error(res.data?.message || 'Failed to fetch reseller config')
  }
  return res.data.data
}

export async function updateResellerConfig(
  data: Partial<ResellerConfig>
): Promise<ResellerConfig> {
  const res = await api.put('/api/reseller/self', data)
  if (!res.data?.success) {
    throw new Error(res.data?.message || 'Failed to update reseller config')
  }
  return res.data.data
}

export interface RedemptionsResponse {
  items: ResellerRedemption[]
  total: number
}

export async function fetchResellerRedemptions(
  page: number = 1,
  pageSize: number = 10,
  keyword: string = ''
): Promise<RedemptionsResponse> {
  const res = await api.get('/api/reseller/redemptions', {
    params: { p: page, page_size: pageSize, keyword },
  })
  if (!res.data?.success) {
    throw new Error(res.data?.message || 'Failed to fetch redemptions')
  }
  return {
    items: res.data.data?.items || [],
    total: res.data.data?.total || 0,
  }
}

export async function createResellerRedemption(data: {
  name: string
  quota: number
  count: number
  type?: number
}): Promise<{ keys: string[] }> {
  const res = await api.post('/api/reseller/redemptions', data)
  if (!res.data?.success) {
    throw new Error(res.data?.message || 'Failed to create redemption code')
  }
  return res.data.data
}

export async function deleteResellerRedemption(id: number): Promise<void> {
  const res = await api.delete(`/api/reseller/redemptions/${id}`)
  if (!res.data?.success) {
    throw new Error(res.data?.message || 'Failed to delete redemption code')
  }
}

export async function fetchRedeemedUserLogs(
  userId: number,
  page: number = 1,
  pageSize: number = 10
): Promise<{ items: any[]; total: number }> {
  const res = await api.get(`/api/reseller/users/${userId}/logs`, {
    params: { p: page, page_size: pageSize },
  })
  if (!res.data?.success) {
    throw new Error(res.data?.message || 'Failed to fetch user logs')
  }
  return {
    items: res.data.data?.items || [],
    total: res.data.data?.total || 0,
  }
}

