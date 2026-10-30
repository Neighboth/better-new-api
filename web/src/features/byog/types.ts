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
export interface ByogNode {
  id: number
  node_id: string
  user_id: number
  pairing_code: string
  device_name: string
  device_type: 'desktop' | 'laptop' | 'mobile' | 'tablet'
  os: string
  arch: string
  hwid: string
  cpu_model: string
  cpu_cores?: number
  gpu_model: string
  npu_model: string
  physical_ram_mb: number
  swap_ram_mb: number
  vram_mb: number
  shared_vram_mb: number
  tee_supported: boolean
  tee_type: string
  bandwidth_down_mbps: number
  bandwidth_up_mbps: number
  ping_ms: number
  network_type: 'ethernet' | 'wifi' | 'cellular'
  ip_address: string
  ip_type: 'residential' | 'datacenter'
  country: string
  city: string
  latitude: number
  longitude: number
  status:
    | 'online'
    | 'offline'
    | 'paused'
    | 'gamer_mode'
    | 'throttled'
    | 'banned'
    | 'updating'
  operating_mode: 'mode_b' | 'mode_a'
  power_profile: 'full_power' | 'half_power' | 'active_hours' | 'dual_schedule'
  schedule_config: string
  allocated_disk_gb: number
  total_disk_gb?: number
  free_disk_gb?: number
  disks?: Array<{
    drive: string
    label: string
    total_gb: number
    free_gb: number
    allocated_gb?: number
  }>
  allocated_vram_gb: number
  allocated_ram_gb: number
  allocated_cores: number
  hot_start_enabled?: boolean

  // Mobile specific settings
  wifi_only_download: boolean
  monthly_data_limit_mb: number
  monthly_data_used_mb: number
  run_only_while_charging: boolean
  battery_cutoff_enabled: boolean
  battery_cutoff_percent: number
  max_battery_temp: number
  battery_temp_supported: boolean

  // Telemetry
  cpu_temp: number
  gpu_temp: number
  battery_temp: number
  fan_speed: number
  battery_percent: number
  is_charging: boolean
  current_tps: number
  current_ttft_ms: number
  total_tokens_processed: number
  total_earned_credits: number
  uptime_seconds: number
  reputation_score: number
  active_multipliers: string
  installed_models: string
  download_queue: string
  model_status?: string
  downloaded_mb?: number
  total_mb?: number
  download_progress?: number
  active_vram_model?: string
  last_heartbeat_at: number
  created_at: number
  updated_at: number

  // Owner of the device, filled by the admin listing of the nodes. The table of
  // the administrator names the account that paired the device next to the
  // device itself: a node id alone does not tell whose machine is online.
  user_name?: string
  user_email?: string
}

export interface ByogSetting {
  id: number
  enabled: boolean
  tier1_provider_price: number
  tier1_customer_price: number
  tier2_provider_price: number
  tier2_customer_price: number
  tier3_provider_price: number
  tier3_customer_price: number
  max_cold_start_timeout: number
  night_owl_surge_enabled: boolean
  weekend_crunch_enabled: boolean
  hardware_backends: string
  multipliers_config?: string
}

export interface ByogMultipliersConfig {
  high_tps_bonus: number
  sub_second_ttft: number
  high_uptime_bonus: number
  super_uptime_bonus: number
  residential_ip_bonus: number
  gigabit_fiber_bonus: number
  low_latency_bonus: number
  clean_reputation_bonus: number
  low_temp_bonus: number
  battery_eco_bonus: number
  night_owl_surge: number
  weekend_crunch: number
}

export interface ByogRewardItem {
  id: number
  title: string
  description: string
  category: string
  required_tokens: number
  stock: number
  is_active: boolean
  created_at: number
  updated_at: number
}

export interface ByogRewardClaim {
  id: number
  user_id: number
  reward_item_id: number
  reward_title: string
  tokens_burned: number
  status: 'pending' | 'approved' | 'rejected' | 'shipped'
  admin_notes: string
  ticket_id: number
  verification_log: string
  created_at: number
  updated_at: number
  user_name?: string
  user_email?: string
}

export interface ByogAssignmentRule {
  id: number
  name: string
  device_class: string
  min_vram_mb: number
  min_physical_ram_mb: number
  hardware_family: string
  min_cores: number
  vector_instruction_set: string
  target_models: string
  target_loras: string
  priority: number
  is_enabled: boolean
  created_at: number
  updated_at: number
}

export interface ByogModel {
  id: number
  model_id: string
  model_type: string
  vendor: string
  description: string
  param_size: string
  pricing_tier: number
  download_url: string
  file_size: number
  chunk_size: number
  chunk_manifest: string
  modalities_input: string
  modalities_output: string
  vision_mmproj_url: string
  anti_cheat_logic_prompt: string
  anti_cheat_logic_temp: number
  anti_cheat_logic_seed: number
  anti_cheat_logic_expected: string
  anti_cheat_logic_logprob: number
  anti_cheat_logic_tolerance: number
  anti_cheat_sig_prompt: string
  anti_cheat_sig_temp: number
  anti_cheat_sig_seed: number
  anti_cheat_sig_expected: string
  anti_cheat_sig_logprob: number
  anti_cheat_sig_tolerance: number
  is_enabled: boolean
  created_at: number
  updated_at: number
}

export interface ByogModelLora {
  id: number
  base_model_id: string
  lora_id: string
  download_url: string
  file_size: number
  chunk_manifest: string
  modality_tags: string
  anti_cheat_prompt: string
  anti_cheat_seed: number
  anti_cheat_expected: string
  anti_cheat_logprob: number
  anti_cheat_tolerance: number
  is_enabled: boolean
  created_at: number
  updated_at: number
}

export interface ByogNodeBan {
  id: number
  node_id: string
  user_id: number
  hwid: string
  ban_reason: string
  violation_code: string
  test_id: string
  test_prompt: string
  expected_logprob: number
  actual_logprob: number
  tolerance: number
  detailed_logs: string
  is_active: boolean
  banned_at: number
  appealed_at: number
  appeal_ticket_id: number
  user_name?: string
  device_name?: string
}

export interface ByogStatusData {
  enabled: boolean
  is_banned: boolean
  ban_info?: ByogNodeBan
  available_tokens?: number
  client_downloads?: Record<string, { available: boolean }>
}
