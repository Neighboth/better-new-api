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

export interface ResellerConfig {
  id: number
  user_id: number
  child_panel_enabled: boolean
  custom_domain?: string
  site_name?: string
  logo?: string
  logo_url?: string
  favicon?: string
  favicon_url?: string
  seo_title?: string
  seo_description?: string
  seo_keywords?: string
  homepage_content?: string
  home_page_content?: string
  epay_partner_id?: string
  epay_key?: string
  epay_partner_key?: string
  epay_url?: string
  epay_gateway_url?: string
  stripe_api_secret?: string
  stripe_webhook_secret?: string
  stripe_price_id?: string
  creem_api_key?: string
  creem_webhook_secret?: string
  creem_test_mode?: boolean
  waffo_merchant_id?: string
  waffo_api_key?: string
  waffo_private_key?: string
  created_at?: string | number
  updated_at?: string | number
}

export interface ResellerRedemption {
  id: number
  name: string
  key: string
  quota: number
  created_time: number
  redeemed_time: number
  status: number
  used_user_id?: number
  used_username?: string
  is_reseller?: boolean
}
