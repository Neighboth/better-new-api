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

export interface UserDetailLog {
  id: number
  created_at: number
  type: number
  content: string
  model_name?: string
  token_name?: string
  channel_name?: string
  quota: number
  prompt_tokens?: number
  completion_tokens?: number
  use_time?: number
  is_stream?: boolean
  group?: string
  ip?: string
}

export interface UserDetailSession {
  sid: string
  user_id: number
  status: string
  login_method: string
  ip: string
  user_agent: string
  created_at: number
  last_active_at: number
  expires_at: number
}

export interface UserDetailOAuthBinding {
  id: number
  provider_id: number
  provider_user_id: string
  created_at: string
}

export interface UserDetailSubscription {
  id: number
  plan_id: number
  amount_total: number
  amount_used: number
  start_time: number
  end_time: number
  status: string
  source: string
  upgrade_group: string
  next_reset_time: number
}

export interface UserDetailSubscriptionSummary {
  subscription: UserDetailSubscription
}

export interface UserDetailPayload {
  user: Record<string, unknown> & {
    id: number
    username: string
    display_name: string
    email?: string
    role: number
    status: number
    group?: string
    quota?: number
    used_quota?: number
    request_count?: number
    aff_code?: string
    aff_count?: number
    aff_quota?: number
    aff_history_quota?: number
    inviter_id?: number
    referral_quota?: number
    referral_display?: number
    requests_balance?: number
    tokens_balance?: number
    created_at?: number
    last_login_at?: number
    remark?: string
    stripe_customer?: string
    admin_permissions?: Record<string, Record<string, boolean>>
    github_id?: string
    oidc_id?: string
    wechat_id?: string
    telegram_id?: string
    linux_do_id?: string
    DeletedAt?: { Time?: string } | null
    [key: string]: unknown
  }
  recent_logs: UserDetailLog[]
  recent_logs_total: number
  active_sessions: UserDetailSession[]
  active_sessions_count: number
  two_fa_enabled: boolean
  oauth_bindings: UserDetailOAuthBinding[]
  subscriptions: UserDetailSubscriptionSummary[]
  tickets_count: number
  server_time: number
}

export interface UserDetailResponse {
  success: boolean
  message?: string
  data?: UserDetailPayload
}

export async function getUserDetail(id: number): Promise<UserDetailResponse> {
  const res = await api.get<UserDetailResponse>(`/api/user/${id}/detail`)
  return res.data
}
