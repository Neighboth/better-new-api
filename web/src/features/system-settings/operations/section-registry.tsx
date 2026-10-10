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
import { SystemBehaviorSection } from '../general/system-behavior-section'
import { RelayFallbackSection } from '../general/relay-fallback-section'
import { TicketSettingsSection } from './ticket-settings-section'
import { DiscordBotSettingsSection } from './discord-bot-settings-section'
import { EmailSettingsSection } from '../integrations/email-settings-section'
import { MonitoringSettingsSection } from '../integrations/monitoring-settings-section'
import { WorkerSettingsSection } from '../integrations/worker-settings-section'
import { LogSettingsSection } from '../maintenance/log-settings-section'
import { PerformanceSection } from '../maintenance/performance-section'
import { SystemFilesSection } from '../maintenance/system-files-section'
import { UpdateCheckerSection } from '../maintenance/update-checker-section'
import type { OperationsSettings } from '../types'
import { createSectionRegistry } from '../utils/section-registry'

const OPERATIONS_SECTIONS = [
  {
    id: 'behavior',
    titleKey: 'System Behavior',
    build: (settings: OperationsSettings) => (
      <SystemBehaviorSection
        defaultValues={{
          DefaultCollapseSidebar: settings.DefaultCollapseSidebar,
          DemoSiteEnabled: settings.DemoSiteEnabled,
          SelfUseModeEnabled: settings.SelfUseModeEnabled,
        }}
      />
      ),
    },
    {
      id: 'relay-fallback',
    titleKey: 'Relay Fallback',
    build: (settings: OperationsSettings) => (
      <RelayFallbackSection
        defaultValues={{
          enable_fallback:
            settings['relay_fallback_setting.enable_fallback'] ?? false,
          fallback_models:
            settings['relay_fallback_setting.fallback_models'] ?? '',
          fallback_chat_models:
            settings['relay_fallback_setting.fallback_chat_models'] ?? '',
          fallback_image_models:
            settings['relay_fallback_setting.fallback_image_models'] ?? '',
          fallback_tts_models:
            settings['relay_fallback_setting.fallback_tts_models'] ?? '',
          fallback_stt_models:
            settings['relay_fallback_setting.fallback_stt_models'] ?? '',
          fallback_system_prompt:
            settings['relay_fallback_setting.fallback_system_prompt'] ?? '',
        }}
      />
    ),
  },
  {
    id: 'ticket',
    titleKey: 'Support & Tickets',
    build: (settings: OperationsSettings) => (
      <TicketSettingsSection
        defaultValues={{
          enabled: settings['ticket_setting.enabled'] ?? true,
          liveSupportEnabled:
            settings['ticket_setting.live_support_enabled'] ?? true,
          notifyAdminOnNewTicket:
            settings['ticket_setting.notify_admin_on_new_ticket'] ?? true,
          notifyUserOnReply:
            settings['ticket_setting.notify_user_on_reply'] ?? true,
          aiAssistantEnabled:
            settings['ticket_setting.ai_assistant_enabled'] ?? true,
          aiAssistantModel:
            settings['ticket_setting.ai_assistant_model'] ?? 'gemini-1.5-flash',
          aiAssistantSystemPrompt:
            settings['ticket_setting.ai_assistant_system_prompt'] ??
            'You are a helpful customer support agent for our API platform. Use markdown and BUTTON[Text](url) when directing users to external links.',
        }}
      />
    ),
  },
  {
    id: 'discord-bot',
    titleKey: 'Discord Bot',
    build: (settings: OperationsSettings) => (
      <DiscordBotSettingsSection
        defaultValues={{
          enabled: settings['discord.enabled'] ?? false,
          botToken: settings['discord.bot_token'] ?? '',
          botName: settings['discord.bot_name'] ?? 'MyAIBot',
          prefix: settings['discord.prefix'] ?? '!',
          status: settings['discord.status'] ?? 'Ready to help',
          language: settings['discord.language'] ?? 'en',
          embedColor: settings['discord.embed_color'] ?? '#00ff00',
          aiSystemPrompt: settings['discord.ai_system_prompt'] ?? 'You are a helpful AI assistant.',
          rpgEnabled: settings['discord.rpg_enabled'] ?? true,
          autoReplyChannelId: settings['discord.auto_reply_channel_id'] ?? '',
          autoReplyModel: settings['discord.auto_reply_model'] ?? '',
          onlyLinkedAccounts: settings['discord.only_linked_accounts'] ?? false,
        }}
      />
    ),
  },
  {
    id: 'files',
    titleKey: 'Files Manager',
    build: () => <SystemFilesSection />,
  },
  {
    id: 'alerts',
    titleKey: 'Monitoring & Alerts',
    build: (settings: OperationsSettings) => (
      <MonitoringSettingsSection
        defaultValues={{
          QuotaRemindThreshold: settings.QuotaRemindThreshold,
          'perf_metrics_setting.enabled':
            settings['perf_metrics_setting.enabled'] ?? true,
          'perf_metrics_setting.flush_interval':
            settings['perf_metrics_setting.flush_interval'] ?? 5,
          'perf_metrics_setting.bucket_time':
            settings['perf_metrics_setting.bucket_time'] ?? 'hour',
          'perf_metrics_setting.retention_days':
            settings['perf_metrics_setting.retention_days'] ?? 0,
        }}
      />
    ),
  },
  {
    id: 'email',
    titleKey: 'SMTP Email',
    build: (settings: OperationsSettings) => (
      <EmailSettingsSection
        defaultValues={{
          SMTPServer: settings.SMTPServer,
          SMTPPort: settings.SMTPPort,
          SMTPAccount: settings.SMTPAccount,
          SMTPFrom: settings.SMTPFrom,
          SMTPToken: settings.SMTPToken,
          SMTPSSLEnabled: settings.SMTPSSLEnabled,
          SMTPStartTLSEnabled: settings.SMTPStartTLSEnabled,
          SMTPInsecureSkipVerify: settings.SMTPInsecureSkipVerify,
          SMTPForceAuthLogin: settings.SMTPForceAuthLogin,
          EmailSubject_verification_tr: settings.EmailSubject_verification_tr ?? '',
          EmailBody_verification_tr: settings.EmailBody_verification_tr ?? '',
          EmailSubject_verification_en: settings.EmailSubject_verification_en ?? '',
          EmailBody_verification_en: settings.EmailBody_verification_en ?? '',
          EmailSubject_verification_zh_CN: settings.EmailSubject_verification_zh_CN ?? '',
          EmailBody_verification_zh_CN: settings.EmailBody_verification_zh_CN ?? '',
          EmailSubject_password_reset_tr: settings.EmailSubject_password_reset_tr ?? '',
          EmailBody_password_reset_tr: settings.EmailBody_password_reset_tr ?? '',
          EmailSubject_password_reset_en: settings.EmailSubject_password_reset_en ?? '',
          EmailBody_password_reset_en: settings.EmailBody_password_reset_en ?? '',
          EmailSubject_password_reset_zh_CN: settings.EmailSubject_password_reset_zh_CN ?? '',
          EmailBody_password_reset_zh_CN: settings.EmailBody_password_reset_zh_CN ?? '',
        }}
      />
    ),
  },
  {
    id: 'worker',
    titleKey: 'Worker Proxy',
    build: (settings: OperationsSettings) => (
      <WorkerSettingsSection
        defaultValues={{
          WorkerUrl: settings.WorkerUrl,
          WorkerValidKey: settings.WorkerValidKey,
          WorkerAllowHttpImageRequestEnabled:
            settings.WorkerAllowHttpImageRequestEnabled,
        }}
      />
    ),
  },
  {
    id: 'logs',
    titleKey: 'Log Maintenance',
    build: (settings: OperationsSettings) => (
      <LogSettingsSection
        defaultEnabled={Boolean(settings.LogConsumeEnabled)}
      />
    ),
  },
  {
    id: 'performance',
    titleKey: 'Performance',
    build: (settings: OperationsSettings) => (
      <PerformanceSection
        defaultValues={{
          'performance_setting.disk_cache_enabled':
            settings['performance_setting.disk_cache_enabled'] ?? false,
          'performance_setting.disk_cache_threshold_mb':
            settings['performance_setting.disk_cache_threshold_mb'] ?? 10,
          'performance_setting.disk_cache_max_size_mb':
            settings['performance_setting.disk_cache_max_size_mb'] ?? 1024,
          'performance_setting.disk_cache_path':
            settings['performance_setting.disk_cache_path'] ?? '',
          'performance_setting.monitor_enabled':
            settings['performance_setting.monitor_enabled'] ?? false,
          'performance_setting.monitor_cpu_threshold':
            settings['performance_setting.monitor_cpu_threshold'] ?? 90,
          'performance_setting.monitor_memory_threshold':
            settings['performance_setting.monitor_memory_threshold'] ?? 90,
          'performance_setting.monitor_disk_threshold':
            settings['performance_setting.monitor_disk_threshold'] ?? 95,
        }}
      />
    ),
  },
  {
    id: 'update-checker',
    titleKey: 'System maintenance',
    build: (
      _settings: OperationsSettings,
      currentVersion?: string | null,
      startTime?: number | null
    ) => (
      <UpdateCheckerSection
        currentVersion={currentVersion}
        startTime={startTime}
      />
    ),
  },
] as const

export type OperationsSectionId = (typeof OPERATIONS_SECTIONS)[number]['id']

const operationsRegistry = createSectionRegistry<
  OperationsSectionId,
  OperationsSettings,
  [string | null | undefined, number | null | undefined]
>({
  sections: OPERATIONS_SECTIONS,
  defaultSection: 'behavior',
  basePath: '/system-settings/operations',
  urlStyle: 'path',
})

export const OPERATIONS_SECTION_IDS = operationsRegistry.sectionIds
export const OPERATIONS_DEFAULT_SECTION = operationsRegistry.defaultSection
export const getOperationsSectionNavItems =
  operationsRegistry.getSectionNavItems
export const getOperationsSectionContent = operationsRegistry.getSectionContent
export const getOperationsSectionMeta = operationsRegistry.getSectionMeta
