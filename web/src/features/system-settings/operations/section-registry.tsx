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
import { ByogSettingsSection } from './byog-settings-section'
import { OnlineSearchSettingsSection } from './online-search-settings-section'
import { PlaygroundSettingsSection } from './playground-settings-section'
import { DiscordBotSettingsSection } from './discord-bot-settings-section'
import { EmailSettingsSection } from '../integrations/email-settings-section'
import { MonitoringSettingsSection } from '../integrations/monitoring-settings-section'
import { WorkerSettingsSection } from '../integrations/worker-settings-section'
import { LogSettingsSection } from '../maintenance/log-settings-section'
import { PerformanceSection } from '../maintenance/performance-section'
import { AutoModelSettingsSection } from './auto-model-settings-section'
import { SystemFilesSection } from '../maintenance/system-files-section'
import { UpdateCheckerSection } from '../maintenance/update-checker-section'
import type { OperationsSettings } from '../types'
import { createSectionRegistry } from '../utils/section-registry'

/**
 * Reads the rankings of the Auto Model.
 *
 * The option holds a JSON object of ordered model lists; a value that is not
 * that object (an installation that never saved the card) is an empty map, so
 * the router uses the general ranking instead of failing on a parse error.
 */
function parseAutoModelRankings(raw: unknown): Record<string, string[]> {
  if (typeof raw !== 'string' || raw.trim() === '') return {}
  try {
    const parsed = JSON.parse(raw) as unknown
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return {}
    const result: Record<string, string[]> = {}
    for (const [key, value] of Object.entries(parsed as Record<string, unknown>)) {
      if (!Array.isArray(value)) continue
      result[key] = value
        .map((entry) => (typeof entry === 'string' ? entry.trim() : ''))
        .filter(Boolean)
    }
    return result
  } catch {
    return {}
  }
}

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
          ModelsAPIRequireKey: settings.ModelsAPIRequireKey ?? true,
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
          fallback_video_models:
            settings['relay_fallback_setting.fallback_video_models'] ?? '',
          fallback_system_prompt:
            settings['relay_fallback_setting.fallback_system_prompt'] ?? '',
        }}
      />
    ),
  },
  {
    id: 'auto-model',
    titleKey: 'Auto Model',
    build: (settings: OperationsSettings) => (
      <AutoModelSettingsSection
        defaultValues={{
          enabled: settings['auto_model_setting.enabled'] ?? false,
          routingModel: settings['auto_model_setting.routing_model'] ?? '',
          rankings: parseAutoModelRankings(
            settings['auto_model_setting.rankings']
          ),
          systemPrompt: settings['auto_model_setting.system_prompt'] ?? '',
          defaultBudgetTokens:
            settings['auto_model_setting.default_budget_tokens'] ?? 0,
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
    id: 'byog',
    titleKey: 'BYOG Management',
    build: (settings: OperationsSettings) => (
      <ByogSettingsSection
        defaultValues={{
          enabled: settings['byog_setting.enabled'] ?? true,
        }}
      />
    ),
  },
  {
    id: 'online-search',
    titleKey: 'Web Search',
    build: (settings: OperationsSettings) => (
      <OnlineSearchSettingsSection
        defaultValues={{
          searxngUrl: settings['online_search_setting.searxng_url'] ?? '',
          onlineSearchPrice: Number(
            settings['online_search_setting.price_per_search'] ?? 0
          ),
          onlineSearchMax: Number(
            settings['online_search_setting.max_searches'] ?? 5
          ),
          playgroundEnabled:
            settings['playground_setting.web_search_playground_enabled'] ??
            false,
          discordEnabled:
            settings['playground_setting.web_search_discord_enabled'] ?? false,
          liveEnabled:
            settings['playground_setting.web_search_live_enabled'] ?? false,
          blogEnabled:
            settings['playground_setting.web_search_blog_enabled'] ?? false,
        }}
      />
    ),
  },
  {
    id: 'playground',
    titleKey: 'Playground & Studio',
    build: (settings: OperationsSettings) => (
      <PlaygroundSettingsSection
        defaultValues={{
          ...playgroundDefaults(settings),
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
          enabled: settings['discord_bot.enabled'] ?? false,
          botToken:
            settings['discord_bot.token'] ??
            settings['discord_bot.bot_token'] ??
            '',
          botName: settings['discord_bot.bot_name'] ?? 'MyAIBot',
          prefix: settings['discord_bot.prefix'] ?? '!',
          status: settings['discord_bot.status'] ?? 'Ready to help',
          language: settings['discord_bot.language'] ?? 'en',
          embedColor: settings['discord_bot.embed_color'] ?? '#00ff00',
          aiSystemPrompt:
            settings['discord_bot.ai_system_prompt'] ??
            'You are a helpful AI assistant.',
          rpgEnabled: settings['discord_bot.rpc_enabled'] ?? true,
          rpcAppId: settings['discord_bot.rpc_app_id'] ?? '',
          rpcDetails: settings['discord_bot.rpc_details'] ?? '',
          rpcState: settings['discord_bot.rpc_state'] ?? '',
          rpcLargeImage: settings['discord_bot.rpc_large_image'] ?? '',
          rpcLargeText: settings['discord_bot.rpc_large_text'] ?? '',
          rpcSmallImage: settings['discord_bot.rpc_small_image'] ?? '',
          rpcSmallText: settings['discord_bot.rpc_small_text'] ?? '',
          autoReplyChannelId:
            settings['discord_bot.auto_reply_channel_id'] ?? '',
          autoReplyModel: settings['discord_bot.auto_reply_model'] ?? '',
          imageModel: settings['discord_bot.image_model'] ?? '',
          onlyLinkedAccounts:
            settings['discord_bot.only_linked_accounts'] ?? false,
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
      <EmailSettingsSection defaultValues={settings as any} />
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

/**
 * Builds the defaults of the playground / studio card.
 *
 * The media models are stored as a JSON list per mode (`image_models`,
 * `video_models`). Installations configured before that list existed only have
 * the single-model values; those become the first entry so the card opens with
 * the configuration the operator knows.
 */
function playgroundDefaults(settings: OperationsSettings) {
  const parseModels = (raw: unknown) => {
    if (typeof raw !== 'string' || raw.trim() === '') return undefined
    try {
      const parsed = JSON.parse(raw)
      return Array.isArray(parsed) ? parsed : undefined
    } catch {
      return undefined
    }
  }

  return {
    imageEnabled: settings['playground_setting.image_enabled'] ?? true,
    imageModels: parseModels(settings['playground_setting.image_models']),
    imageModel: settings['playground_setting.image_model'] ?? '',
    imageSizes:
      settings['playground_setting.image_sizes'] ??
      '1024x1024,1024x1792,1792x1024',
    imageQualities:
      settings['playground_setting.image_qualities'] ?? 'standard,hd',
    imageDefaultSize:
      settings['playground_setting.image_default_size'] ?? '1024x1024',
    imageDefaultQuality:
      settings['playground_setting.image_default_quality'] ?? 'standard',
    imageDefaultCount: Number(
      settings['playground_setting.image_default_count'] ?? 1
    ),
    videoEnabled: settings['playground_setting.video_enabled'] ?? false,
    videoModels: parseModels(settings['playground_setting.video_models']),
    videoModel: settings['playground_setting.video_model'] ?? '',
    videoDurations: settings['playground_setting.video_durations'] ?? '5,10',
    videoRatios: settings['playground_setting.video_ratios'] ?? '16:9,9:16,1:1',
    videoQualities:
      settings['playground_setting.video_qualities'] ?? 'standard,high',
    videoDefaultRatio:
      settings['playground_setting.video_default_ratio'] ?? '16:9',
    videoDefaultQuality:
      settings['playground_setting.video_default_quality'] ?? 'standard',
    videoReferenceImage:
      settings['playground_setting.video_reference_image'] ?? true,
    studioEnabled: settings['playground_setting.studio_enabled'] ?? true,
  }
}
