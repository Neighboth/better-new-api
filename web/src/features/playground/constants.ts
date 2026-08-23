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
import type {
  PlaygroundConfig,
  ParameterEnabled,
  PlaygroundToolsEnabled,
  ThinkingLevel,
} from './types'

// Message constants
export const MESSAGE_ROLES = {
  USER: 'user',
  ASSISTANT: 'assistant',
  SYSTEM: 'system',
} as const

export const MESSAGE_STATUS = {
  LOADING: 'loading',
  STREAMING: 'streaming',
  COMPLETE: 'complete',
  ERROR: 'error',
} as const

// API endpoints
export const API_ENDPOINTS = {
  CHAT_COMPLETIONS: '/pg/chat/completions',
  IMAGE_GENERATIONS: '/pg/images/generations',
  USER_MODELS: '/api/user/models',
  USER_GROUPS: '/api/user/self/groups',
} as const

// Default group — uses 'default' as the safe fallback; auto-group is
// only selected when the backend confirms it is available for the user.
export const DEFAULT_GROUP = 'default' as const

// Playground tool loop limits
export const MAX_TOOL_ROUNDS = 5
export const MAX_TOOL_RESULT_CHARS = 12_000

// Default tool availability — these tools are always offered to the model,
// which decides on its own whether to call them. The `think` tool is offered
// separately when the user forces thinking on a non-reasoning model.
export const DEFAULT_TOOLS_ENABLED: PlaygroundToolsEnabled = {
  generate_image: true,
  web_search: true,
  fetch_page: true,
  update_plan: true,
}

// Forced-thinking defaults. The level text is injected into the system prompt
// so the model knows how deeply to reason inside the `think` tool.
export const DEFAULT_THINKING_ENABLED = false
export const DEFAULT_THINKING_LEVEL: ThinkingLevel = 'medium'

export const THINKING_LEVELS: ThinkingLevel[] = [
  'lite',
  'low',
  'medium',
  'high',
  'ultra',
]

export const THINKING_LEVEL_PROMPTS: Record<ThinkingLevel, string> = {
  lite: 'Keep each thinking pass extremely brief: one or two sentences only.',
  low: 'Keep each thinking pass short: a few sentences covering only the key considerations.',
  medium: 'Think step by step with moderate detail before acting or answering.',
  high: 'Think deeply and thoroughly: explore alternatives, edge cases, and verify your approach before acting or answering.',
  ultra:
    'Think exhaustively: decompose the problem, weigh multiple approaches, verify assumptions, and double-check conclusions before acting or answering.',
}

// Silent fallback: how many alternative models to try before surfacing an
// error. The same model gets one immediate retry first (the gateway then
// routes to a different channel hosting it).
export const MAX_MODEL_FALLBACKS = 4

// Display labels (i18n keys) for tool names.
export const TOOL_LABEL_KEYS: Record<string, string> = {
  generate_image: 'Image generation',
  web_search: 'Web search',
  fetch_page: 'Page fetch',
  update_plan: 'Plan update',
  think: 'Thinking',
}

export function getToolLabelKey(name: string): string {
  return TOOL_LABEL_KEYS[name] ?? name
}

// Attachment limits for playground image inputs
export const ATTACHMENT_ACCEPT = 'image/*' as const
export const MAX_ATTACHMENTS = 5
export const MAX_ATTACHMENT_SIZE_BYTES = 10 * 1024 * 1024

// Default configuration
export const DEFAULT_CONFIG: PlaygroundConfig = {
  model: 'gpt-4o',
  group: DEFAULT_GROUP,
  temperature: 0.7,
  top_p: 1,
  max_tokens: 4096,
  frequency_penalty: 0,
  presence_penalty: 0,
  seed: null,
  stream: true,
}

export const DEFAULT_PARAMETER_ENABLED: ParameterEnabled = {
  temperature: true,
  top_p: true,
  max_tokens: false,
  frequency_penalty: true,
  presence_penalty: true,
  seed: false,
}

// Storage keys
export const STORAGE_KEYS = {
  CONFIG: 'playground_config',
  IMAGES: 'playground_images',
  MESSAGES: 'playground_messages',
  PARAMETER_ENABLED: 'playground_parameter_enabled',
  TOOLS_ENABLED: 'playground_tools_enabled',
  THINKING: 'playground_thinking',
} as const

// Error messages
export const ERROR_MESSAGES = {
  API_REQUEST_ERROR: 'Request error occurred',
  NETWORK_ERROR: 'Network connection failed or server not responding',
  PARSE_ERROR: 'Error parsing response data',
  STREAM_START_ERROR: 'Error establishing connection',
  CONNECTION_CLOSED: 'Connection closed',
  INTERRUPTED: 'Generation was interrupted',
  IMAGE_EMPTY_RESULT: 'The model did not return an image',
  IMAGE_MODEL_UNAVAILABLE: 'No image generation model is available',
  TOOL_CALL_FAILED: 'Tool call failed',
  TOOL_ARGS_INVALID: 'Invalid tool arguments',
  IMAGE_PROMPT_REQUIRED: 'Missing image prompt',
  SEARCH_QUERY_REQUIRED: 'Missing search query',
  PAGE_URL_INVALID: 'Invalid page URL',
  PLAN_STEPS_REQUIRED: 'Plan must contain at least one step',
  WEB_SEARCH_FAILED: 'Web search failed',
  PAGE_FETCH_FAILED: 'Failed to fetch the page',
} as const

// Message action button styles
export const MESSAGE_ACTION_BUTTON_STYLES = {
  BASE: 'size-7 text-muted-foreground hover:text-foreground',
  DELETE: 'size-7 text-muted-foreground hover:text-destructive',
  ICON: 'size-4',
} as const

// Message action labels
export const MESSAGE_ACTION_LABELS = {
  COPY: 'Copy',
  COPIED: 'Copied!',
  REGENERATE: 'Regenerate',
  SHOW_PREVIEW: 'Show preview',
  SHOW_SOURCE: 'Show source',
  EDIT: 'Edit',
  DELETE: 'Delete',
  NO_CONTENT: 'No content to copy',
  WAIT_GENERATION: 'Please wait for the current generation to complete',
} as const
