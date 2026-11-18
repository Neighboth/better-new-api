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

export interface StudioSkill {
  id: string
  title: string
  description: string
  image: string
  content: string
  sort_order: number
  enabled: boolean
  created_at: number
  updated_at: number
}

export interface CreateSkillPayload {
  title: string
  description: string
  image: string
  content: string
  sort_order: number
}

type Envelope<T> = {
  success: boolean
  message?: string
  data?: T
}

// Public list — what the Studio's skill picker renders.
export async function listEnabledSkills(): Promise<StudioSkill[]> {
  const res = await api.get<Envelope<StudioSkill[]>>('/api/playground/skills')
  if (!res.data?.success) {
    throw new Error(res.data?.message || 'Failed to load skills')
  }
  return res.data.data ?? []
}

// Admin list — everything, including disabled skills.
export async function listSkills(): Promise<StudioSkill[]> {
  const res = await api.get<Envelope<StudioSkill[]>>('/api/playground/admin/skills/')
  if (!res.data?.success) {
    throw new Error(res.data?.message || 'Failed to load skills')
  }
  return res.data.data ?? []
}

export async function createSkill(
  payload: CreateSkillPayload
): Promise<StudioSkill> {
  const res = await api.post<Envelope<StudioSkill>>(
    '/api/playground/admin/skills/',
    payload
  )
  if (!res.data?.success || !res.data.data) {
    throw new Error(res.data?.message || 'Failed to create the skill')
  }
  return res.data.data
}

export async function updateSkill(
  id: string,
  payload: Partial<CreateSkillPayload> & { enabled?: boolean }
): Promise<StudioSkill> {
  const res = await api.put<Envelope<StudioSkill>>(
    `/api/playground/admin/skills/${encodeURIComponent(id)}`,
    payload
  )
  if (!res.data?.success) {
    throw new Error(res.data?.message || 'Failed to update the skill')
  }
  return res.data?.data ?? ({} as StudioSkill)
}

export async function deleteSkill(id: string): Promise<void> {
  const res = await api.delete<Envelope<object>>(
    `/api/playground/admin/skills/${encodeURIComponent(id)}`
  )
  if (!res.data?.success) {
    throw new Error(res.data?.message || 'Failed to delete the skill')
  }
}

export async function uploadSkillImage(file: File): Promise<string> {
  const formData = new FormData()
  formData.append('file', file)
  const res = await api.post<Envelope<{ url: string }>>(
    '/api/playground/admin/skills/upload',
    formData,
    { headers: { 'Content-Type': 'multipart/form-data' } }
  )
  if (!res.data?.success || !res.data.data?.url) {
    throw new Error(res.data?.message || 'Failed to upload the image')
  }
  return res.data.data.url
}
