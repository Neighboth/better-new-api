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

import type {
  ByogAssignmentRule,
  ByogModel,
  ByogModelLora,
  ByogNode,
  ByogNodeBan,
  ByogRewardClaim,
  ByogRewardItem,
  ByogSetting,
  ByogStatusData,
} from './types'

// User Endpoints
export async function getByogStatus(): Promise<ByogStatusData> {
  const res = await api.get('/api/byog/status')
  return res.data.data
}

export async function getUserNodes(): Promise<ByogNode[]> {
  const res = await api.get('/api/byog/user/nodes')
  return res.data.data
}

export async function pairNode(pairingCode: string): Promise<ByogNode> {
  const res = await api.post('/api/byog/user/nodes/pair', {
    pairing_code: pairingCode,
  })
  if (!res.data.success) {
    throw new Error(res.data.message || 'Pairing failed')
  }
  return res.data.data
}

export async function getUserPairingCode(): Promise<{
  pairing_code: string
  server_url: string
  system_name: string
  app_name: string
  favicon: string
  expires_in: number
}> {
  const res = await api.get('/api/byog/user/pair/code')
  if (!res.data.success) {
    throw new Error(res.data.message || 'Failed to get pairing code')
  }
  return res.data.data
}

export async function updateNode(
  nodeId: string,
  data: Partial<ByogNode>
): Promise<ByogNode> {
  const res = await api.put(`/api/byog/user/nodes/${nodeId}`, data)
  return res.data.data
}

export async function deleteNode(nodeId: string): Promise<void> {
  const res = await api.delete(`/api/byog/user/nodes/${nodeId}`)
  if (res.data && res.data.success === false) {
    throw new Error(res.data.message || 'Cihaz silinemedi')
  }
}

export async function getUserRewards(): Promise<ByogRewardItem[]> {
  const res = await api.get('/api/byog/user/rewards')
  return res.data.data
}

export async function claimReward(rewardId: number): Promise<ByogRewardClaim> {
  const res = await api.post(`/api/byog/user/rewards/${rewardId}/claim`)
  if (!res.data.success) {
    throw new Error(res.data.message || 'Claim failed')
  }
  return res.data.data
}

export async function submitBanAppeal(
  nodeId: string,
  appealText: string
): Promise<{ ticket_id: number }> {
  const res = await api.post('/api/byog/user/appeal', {
    node_id: nodeId,
    appeal_text: appealText,
  })
  if (!res.data.success) {
    throw new Error(res.data.message || 'Appeal failed')
  }
  return res.data
}

// Admin Endpoints
export async function getAdminSettings(): Promise<ByogSetting> {
  const res = await api.get('/api/byog/admin/settings')
  return res.data.data
}

export async function updateAdminSettings(
  settings: Partial<ByogSetting>
): Promise<ByogSetting> {
  const res = await api.post('/api/byog/admin/settings', settings)
  return res.data.data
}

export async function getAdminRewards(): Promise<ByogRewardItem[]> {
  const res = await api.get('/api/byog/admin/rewards')
  return res.data.data
}

export async function createAdminReward(
  reward: Partial<ByogRewardItem>
): Promise<ByogRewardItem> {
  const res = await api.post('/api/byog/admin/rewards', reward)
  return res.data.data
}

export async function updateAdminReward(
  id: number,
  reward: Partial<ByogRewardItem>
): Promise<ByogRewardItem> {
  const res = await api.put(`/api/byog/admin/rewards/${id}`, reward)
  return res.data.data
}

export async function deleteAdminReward(id: number): Promise<void> {
  await api.delete(`/api/byog/admin/rewards/${id}`)
}

export async function getAdminClaims(
  status?: string
): Promise<ByogRewardClaim[]> {
  const res = await api.get('/api/byog/admin/claims', { params: { status } })
  return res.data.data
}

export async function approveAdminClaim(id: number): Promise<ByogRewardClaim> {
  const res = await api.post(`/api/byog/admin/claims/${id}/approve`)
  return res.data.data
}

export async function rejectAdminClaim(id: number): Promise<ByogRewardClaim> {
  const res = await api.post(`/api/byog/admin/claims/${id}/reject`)
  return res.data.data
}

export async function getAdminRules(): Promise<ByogAssignmentRule[]> {
  const res = await api.get('/api/byog/admin/rules')
  return res.data.data
}

export async function createAdminRule(
  rule: Partial<ByogAssignmentRule>
): Promise<ByogAssignmentRule> {
  const res = await api.post('/api/byog/admin/rules', rule)
  return res.data.data
}

export async function updateAdminRule(
  id: number,
  rule: Partial<ByogAssignmentRule>
): Promise<ByogAssignmentRule> {
  const res = await api.put(`/api/byog/admin/rules/${id}`, rule)
  return res.data.data
}

export async function deleteAdminRule(id: number): Promise<void> {
  await api.delete(`/api/byog/admin/rules/${id}`)
}

export async function getAdminModels(): Promise<{
  models: ByogModel[]
  loras: ByogModelLora[]
}> {
  const res = await api.get('/api/byog/admin/models')
  return res.data.data
}

export async function createAdminModel(
  model: Partial<ByogModel>
): Promise<ByogModel> {
  const res = await api.post('/api/byog/admin/models', model)
  return res.data.data
}

export async function updateAdminModel(
  id: number,
  model: Partial<ByogModel>
): Promise<ByogModel> {
  const res = await api.put(`/api/byog/admin/models/${id}`, model)
  return res.data.data
}

export async function deleteAdminModel(id: number): Promise<void> {
  await api.delete(`/api/byog/admin/models/${id}`)
}

export async function createAdminLora(
  modelId: number,
  lora: Partial<ByogModelLora>
): Promise<ByogModelLora> {
  const res = await api.post(`/api/byog/admin/models/${modelId}/loras`, lora)
  return res.data.data
}

export async function deleteAdminLora(id: number): Promise<void> {
  await api.delete(`/api/byog/admin/loras/${id}`)
}

export async function getAdminNodes(): Promise<ByogNode[]> {
  const res = await api.get('/api/byog/admin/nodes')
  return res.data.data
}

export async function inspectAdminNode(nodeId: string): Promise<any> {
  const res = await api.get(`/api/byog/admin/nodes/${nodeId}/inspect`)
  return res.data.data
}

export async function getAdminBannedNodes(): Promise<ByogNodeBan[]> {
  const res = await api.get('/api/byog/admin/banned')
  return res.data.data
}

export async function unbanAdminNode(id: number): Promise<void> {
  await api.post(`/api/byog/admin/banned/${id}/unban`)
}

export async function deleteAdminNode(nodeId: string): Promise<void> {
  await api.delete(`/api/byog/admin/nodes/${nodeId}`)
}
