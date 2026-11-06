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

export interface ResellerRedemption {
  id: number
  name: string
  key: string
  quota: number
  type?: number // 0: Quota, 1: Requests, 2: Tokens
  created_time: number
  redeemed_time: number
  status: number
  used_user_id?: number
  used_username?: string
  is_reseller?: boolean
}

export interface ResellerSummary {
  total_codes: number
  used_codes: number
  unused_codes: number
  total_quota_distributed: number
  total_requests_distributed: number
  total_tokens_distributed: number
  reseller_quota: number
  reseller_requests: number
  reseller_tokens: number
  active_sub_users: number
}
