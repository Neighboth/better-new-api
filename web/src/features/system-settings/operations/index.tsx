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
import { useStatus } from '@/hooks/use-status'

import { SettingsPage } from '../components/settings-page'
import type { OperationsSettings } from '../types'
import {
  OPERATIONS_DEFAULT_SECTION,
  getOperationsSectionContent,
  getOperationsSectionMeta,
} from './section-registry.tsx'

const defaultOperationsSettings: OperationsSettings = {
  DefaultCollapseSidebar: false,
  DemoSiteEnabled: false,
  SelfUseModeEnabled: false,
  ModelsAPIRequireKey: true,
  QuotaRemindThreshold: '',
  SMTPServer: '',
  SMTPPort: '',
  SMTPAccount: '',
  SMTPFrom: '',
  SMTPToken: '',
  SMTPSSLEnabled: false,
  SMTPStartTLSEnabled: false,
  SMTPInsecureSkipVerify: false,
  SMTPForceAuthLogin: false,
  DKIMSelector: '',
  EmailSubject_verification_tr: '',
  EmailBody_verification_tr: '',
  EmailSubject_verification_en: '',
  EmailBody_verification_en: '',
  EmailSubject_verification_zh_CN: '',
  EmailBody_verification_zh_CN: '',
  EmailSubject_password_reset_tr: '',
  EmailBody_password_reset_tr: '',
  EmailSubject_password_reset_en: '',
  EmailBody_password_reset_en: '',
  EmailSubject_password_reset_zh_CN: '',
  EmailBody_password_reset_zh_CN: '',
  WorkerUrl: '',
  WorkerValidKey: '',
  WorkerAllowHttpImageRequestEnabled: false,
  LogConsumeEnabled: false,
  'performance_setting.disk_cache_enabled': false,
  'performance_setting.disk_cache_threshold_mb': 10,
  'performance_setting.disk_cache_max_size_mb': 1024,
  'performance_setting.disk_cache_path': '',
  'performance_setting.monitor_enabled': false,
  'performance_setting.monitor_cpu_threshold': 90,
  'performance_setting.monitor_memory_threshold': 90,
  'performance_setting.monitor_disk_threshold': 95,
  'perf_metrics_setting.enabled': true,
  'perf_metrics_setting.flush_interval': 5,
  'perf_metrics_setting.bucket_time': 'hour',
  'perf_metrics_setting.retention_days': 0,
  'auto_model_setting.enabled': false,
  'auto_model_setting.routing_model': '',
  'auto_model_setting.rankings': '',
  'auto_model_setting.system_prompt': '',
  'auto_model_setting.default_budget_tokens': 0,
  'relay_fallback_setting.enable_fallback': false,
  'relay_fallback_setting.fallback_models': '',
  'relay_fallback_setting.fallback_chat_models': '',
  'relay_fallback_setting.fallback_image_models': '',
  'relay_fallback_setting.fallback_tts_models': '',
  'relay_fallback_setting.fallback_stt_models': '',
  'relay_fallback_setting.fallback_video_models': '',
  'relay_fallback_setting.fallback_system_prompt': '',
  'user_blocklist_setting.enabled': true,
  'user_blocklist_setting.allow_channels': true,
  'user_blocklist_setting.allow_models': true,
  'ticket_setting.enabled': true,
  'ticket_setting.live_support_enabled': true,
  'ticket_setting.notify_admin_on_new_ticket': true,
  'ticket_setting.notify_user_on_reply': true,
  'ticket_setting.ai_assistant_enabled': true,
  'ticket_setting.ai_assistant_model': 'gemini-1.5-flash',
  'ticket_setting.ai_assistant_system_prompt':
    'You are a helpful customer support agent for our API platform. Use markdown and BUTTON[Text](url) when directing users to external links.',
  'byog_setting.enabled': true,
  'online_search_setting.searxng_url': '',
  'online_search_setting.price_per_search': 0,
  'online_search_setting.max_searches': 5,
  // Playground & studio: every key the section reads needs a typed default,
  // otherwise "false" arrives as a truthy string and a saved-off switch looks
  // enabled after a reload.
  'playground_setting.web_search_playground_enabled': false,
  'playground_setting.web_search_discord_enabled': false,
  'playground_setting.web_search_live_enabled': false,
  'playground_setting.image_enabled': true,
  'playground_setting.image_models': '',
  'playground_setting.image_model': '',
  'playground_setting.image_sizes': '',
  'playground_setting.image_qualities': '',
  'playground_setting.image_default_size': '',
  'playground_setting.image_default_quality': '',
  'playground_setting.image_default_count': 1,
  'playground_setting.video_enabled': false,
  'playground_setting.video_models': '',
  'playground_setting.video_model': '',
  'playground_setting.video_durations': '',
  'playground_setting.video_default_duration': 5,
  'playground_setting.video_ratios': '',
  'playground_setting.video_default_ratio': '',
  'playground_setting.video_qualities': '',
  'playground_setting.video_default_quality': '',
  'playground_setting.video_reference_image': true,
  'playground_setting.studio_enabled': true,
  // Discord Bot. The bot reads only the discord_bot.* namespace: discord.*
  // belongs to the Discord OAuth login, and sharing keys between the two made
  // the bot switch follow the login switch.
  'discord_bot.enabled': false,
  'discord_bot.token': '',
  'discord_bot.bot_token': '',
  'discord_bot.bot_name': 'MyAIBot',
  'discord_bot.prefix': '!',
  'discord_bot.status': 'Ready to help',
  'discord_bot.language': 'en',
  'discord_bot.embed_color': '#00ff00',
  'discord_bot.ai_system_prompt': 'You are a helpful AI assistant.',
  'discord_bot.rpc_enabled': true,
  'discord_bot.rpc_app_id': '',
  'discord_bot.rpc_details': 'AI Platform Assistant',
  'discord_bot.rpc_state': 'Online',
  'discord_bot.rpc_large_image': 'bot_logo',
  'discord_bot.rpc_large_text': 'AI Service',
  'discord_bot.rpc_small_image': '',
  'discord_bot.rpc_small_text': '',
  'discord_bot.auto_reply_channel_id': '',
  'discord_bot.auto_reply_model': 'gpt-4o-mini',
  'discord_bot.image_model': '',
  'discord_bot.only_linked_accounts': false,
}

export function OperationsSettings() {
  const { status } = useStatus()

  return (
    <SettingsPage
      routePath='/_authenticated/system-settings/operations/$section'
      defaultSettings={defaultOperationsSettings}
      defaultSection={OPERATIONS_DEFAULT_SECTION}
      getSectionContent={getOperationsSectionContent}
      getSectionMeta={getOperationsSectionMeta}
      extraArgs={[
        status?.version as string | undefined,
        status?.start_time as number | null | undefined,
      ]}
      loadingMessage='Loading maintenance settings...'
    />
  )
}
