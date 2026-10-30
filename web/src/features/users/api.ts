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
import type { PermissionCatalog } from '@/lib/admin-permissions'
import { api } from '@/lib/api'
import type { CustomOAuthBinding } from '@/lib/oauth'

import type {
  User,
  GetUsersParams,
  GetUsersResponse,
  SearchUsersParams,
  UserFormData,
  ManageUserAction,
  ManageUserQuotaPayload,
  ApiResponse,
} from './types'

// ============================================================================
// User Management APIs
// ============================================================================

/**
 * Get paginated users list
 */
export async function getUsers(
  params: GetUsersParams = {}
): Promise<GetUsersResponse> {
  const { p = 1, page_size = 10, sort_by, sort_order } = params
  const res = await api.get('/api/user/', {
    params: {
      p,
      page_size,
      sort_by,
      sort_order,
    },
  })
  return res.data
}

/**
 * Search users by keyword or group
 */
export async function searchUsers(
  params: SearchUsersParams
): Promise<GetUsersResponse> {
  const {
    keyword = '',
    group = '',
    role = '',
    status = '',
    p = 1,
    page_size = 10,
    sort_by,
    sort_order,
  } = params
  const queryParams = new URLSearchParams()
  queryParams.set('keyword', keyword)
  queryParams.set('group', group)
  if (role) queryParams.set('role', role)
  if (status) queryParams.set('status', status)
  queryParams.set('p', String(p))
  queryParams.set('page_size', String(page_size))
  if (sort_by) queryParams.set('sort_by', sort_by)
  if (sort_order) queryParams.set('sort_order', sort_order)
  const res = await api.get(`/api/user/search?${queryParams.toString()}`)
  return res.data
}

/**
 * Get single user by ID
 */
export async function getUser(id: number): Promise<ApiResponse<User>> {
  const res = await api.get(`/api/user/${id}`)
  return res.data
}

/**
 * Create a new user
 */
export async function createUser(
  data: UserFormData
): Promise<ApiResponse<User>> {
  const res = await api.post('/api/user/', data)
  return res.data
}

/**
 * Update an existing user
 */
export async function updateUser(
  data: UserFormData & { id: number }
): Promise<ApiResponse<Partial<User>>> {
  const res = await api.put('/api/user/', data)
  return res.data
}

/**
 * Delete a single user (hard delete)
 */
export async function deleteUser(id: number): Promise<ApiResponse> {
  const res = await api.delete(`/api/user/${id}/`)
  return res.data
}

/**
 * Manage user (promote, demote, enable, disable, delete)
 */
export async function manageUser(
  id: number,
  action: ManageUserAction
): Promise<ApiResponse<Partial<User>>> {
  const res = await api.post('/api/user/manage', { id, action })
  return res.data
}

/**
 * Adjust user quota atomically (add/subtract/override)
 */
export async function adjustUserQuota(
  payload: ManageUserQuotaPayload
): Promise<ApiResponse<Partial<User>>> {
  const res = await api.post('/api/user/manage', payload)
  return res.data
}

/**
 * Reset user's Passkey registration
 */
export async function resetUserPasskey(id: number): Promise<ApiResponse> {
  const res = await api.delete(`/api/user/${id}/reset_passkey`)
  return res.data
}

/**
 * Reset user's Two-Factor Authentication setup
 */
export async function resetUserTwoFA(id: number): Promise<ApiResponse> {
  const res = await api.delete(`/api/user/${id}/2fa`)
  return res.data
}

/**
 * Get all available groups
 */
export async function getGroups(): Promise<ApiResponse<string[]>> {
  const res = await api.get('/api/group/')
  return res.data
}

/**
 * Get the permission catalog (resources, actions, and role baselines).
 * Source of truth lives in the backend authz package.
 */
export async function getPermissionCatalog(): Promise<PermissionCatalog> {
  const res = await api.get('/api/authz/catalog')
  return {
    resources: res.data?.data?.resources ?? [],
    roles: res.data?.data?.roles ?? [],
  }
}

// ============================================================================
// Admin Binding Management APIs
// ============================================================================

/**
 * Get user's custom OAuth bindings (admin)
 */
export async function getUserOAuthBindings(
  userId: number
): Promise<ApiResponse<CustomOAuthBinding[]>> {
  const res = await api.get(`/api/user/${userId}/oauth/bindings`)
  return res.data
}

/**
 * Clear a user's built-in binding (admin)
 */
export async function adminClearUserBinding(
  userId: number,
  bindingType: string
): Promise<ApiResponse> {
  const res = await api.delete(`/api/user/${userId}/bindings/${bindingType}`)
  return res.data
}

/**
 * Unbind custom OAuth for a user (admin)
 */
export async function adminUnbindCustomOAuth(
  userId: number,
  providerId: number
): Promise<ApiResponse> {
  const res = await api.delete(
    `/api/user/${userId}/oauth/bindings/${providerId}`
  )
  return res.data
}

// ============================================================================
// Mail of the administrator
// ============================================================================

/** A user that the selector of the mail page offers. */
export interface MailRecipientOption {
  id: number
  username: string
  display_name: string
  email: string
  marketing: boolean
}

/** The number of the readers of a message. */
export interface MailAudience {
  addresses: number
  marketing: number
  selected?: number
  selected_marketing?: number
}

/** The state of the mail server of the installation. */
export interface MailStatus {
  configured: boolean
  server: string
  from: string
  encrypted: boolean
  sending: boolean
}

/** One advice of the deliverability report. */
export interface MailHint {
  level: 'ok' | 'warn' | 'bad'
  message: string
}

/** The DNS report of the domain of the sender. */
export interface MailDeliverability {
  domain: string
  encrypted: boolean
  spf: string
  spf_ok: boolean
  dmarc: string
  dmarc_ok: boolean
  dkim: string
  dkim_ok: boolean
  mx: string
  mx_ok: boolean
  hints: MailHint[]
  readiness: number
}

/** The progress of a campaign. */
export interface MailCampaign {
  id: number
  subject: string
  is_html: boolean
  is_promotional: boolean
  audience: string
  status: 'queued' | 'running' | 'done' | 'failed'
  total: number
  sent: number
  failed: number
  skipped: number
  created_at: number
  started_at: number
  ended_at: number
  last_error: string
  failures: string
}

/** The message of the page of the users. */
export interface SendMailPayload {
  subject: string
  content: string
  is_html: boolean
  promotional: boolean
  all_users: boolean
  user_ids: number[]
  test?: boolean
  test_email?: string
}

export async function getMailAudience(ids: number[] = []) {
  const params = ids.length > 0 ? { ids: ids.join(',') } : undefined
  const res = await api.get<ApiResponse<MailAudience>>('/api/user/mail/audience', {
    params,
  })
  return res.data
}

export async function searchMailRecipients(keyword: string, limit = 50) {
  const res = await api.get<
    ApiResponse<{ items: MailRecipientOption[]; count: number }>
  >('/api/user/mail/recipients', { params: { keyword, limit } })
  return res.data
}

/**
 * The users of the given ids.
 *
 * The dialog of the mail of the table starts with the rows that the
 * administrator selected and it asks the installation for those users only.
 */
export async function getMailRecipientsByIds(ids: number[]) {
  const res = await api.get<
    ApiResponse<{ items: MailRecipientOption[]; count: number }>
  >('/api/user/mail/recipients', { params: { ids: ids.join(',') } })
  return res.data
}

export async function getMailStatus() {
  const res = await api.get<ApiResponse<MailStatus>>('/api/user/mail/status')
  return res.data
}

export async function getMailDeliverability() {
  const res = await api.get<ApiResponse<MailDeliverability>>(
    '/api/user/mail/deliverability'
  )
  return res.data
}

export async function sendMail(payload: SendMailPayload) {
  const res = await api.post<
    ApiResponse<{ campaign_id?: number; total?: number; test?: boolean }>
  >('/api/user/mail/send', payload)
  return res.data
}

export async function getMailCampaign(id: number) {
  const res = await api.get<ApiResponse<MailCampaign>>(
    `/api/user/mail/campaigns/${id}`
  )
  return res.data
}
