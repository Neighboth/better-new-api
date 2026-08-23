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
// Message types
export type MessageRole = 'user' | 'assistant' | 'system'

export type MessageStatus = 'loading' | 'streaming' | 'complete' | 'error'

export type PlaygroundMessageLayoutMode = 'alternating' | 'left'

export interface MessageVersion {
  id: string
  content: string
}

export interface MessageAttachment {
  url: string
  mediaType?: string
  filename?: string
}

// Playground tool types
export type PlaygroundToolId =
  | 'generate_image'
  | 'web_search'
  | 'fetch_page'
  | 'update_plan'
  | 'think'

// Tools that are always offered to the model. `think` is added dynamically
// when the user forces thinking on a model without native reasoning.
export type AlwaysOnToolId = Exclude<PlaygroundToolId, 'think'>

export type PlaygroundToolsEnabled = Record<AlwaysOnToolId, boolean>

export type ThinkingLevel = 'lite' | 'low' | 'medium' | 'high' | 'ultra'

export type PlanStepStatus = 'pending' | 'in_progress' | 'completed'

export interface PlanStep {
  id: string
  title: string
  status: PlanStepStatus
}

export type ToolEventStatus = 'running' | 'done' | 'error'

export interface ToolEvent {
  id: string
  /** Tool name as called by the model; may be an unknown tool. */
  name: string
  status: ToolEventStatus
  summary?: string
  error?: string
  startedAt?: number
  completedAt?: number
  /**
   * Character offset into the assistant content where the call happened, so
   * transient indicators render at the usage position instead of the top.
   */
  anchor?: number
  /** Raw arguments JSON of the call, kept for API transcript replay. */
  arguments?: string
  /** Tool result payload sent back to the model, kept for transcript replay. */
  result?: string
  /** Plan snapshot produced by this update_plan call. */
  plan?: PlanStep[]
}

/** A think-tool thought rendered exactly like a native reasoning block. */
export interface ThoughtBlock {
  id: string
  content: string
  /** Character offset into the assistant content where the thought occurred. */
  anchor: number
  startedAt?: number
  completedAt?: number
}

export interface Message {
  key: string
  from: MessageRole
  versions: MessageVersion[]
  attachments?: MessageAttachment[]
  createdAt?: number
  startedAt?: number
  completedAt?: number
  durationMs?: number
  sources?: { href: string; title: string }[]
  toolEvents?: ToolEvent[]
  thoughts?: ThoughtBlock[]
  plan?: PlanStep[]
  activeTool?: string | null
  reasoning?: {
    content: string
    duration: number
    startedAt?: number
    completedAt?: number
    durationMs?: number
  }
  isReasoningStreaming?: boolean
  isReasoningComplete?: boolean
  isContentComplete?: boolean
  status?: MessageStatus
  errorCode?: string | null
}

// API payload types
export interface ToolCallFunction {
  name: string
  arguments: string
}

export interface ToolCall {
  id: string
  type: 'function'
  function: ToolCallFunction
}

export interface ChatCompletionMessage {
  role: MessageRole | 'tool'
  content: string | ContentPart[] | null
  tool_calls?: ToolCall[]
  tool_call_id?: string
  name?: string
}

export interface ChatCompletionTool {
  type: 'function'
  function: {
    name: string
    description: string
    parameters: Record<string, unknown>
  }
}

export interface ContentPart {
  type: 'text' | 'image_url'
  text?: string
  image_url?: {
    url: string
  }
}

export interface ChatCompletionRequest {
  model: string
  group?: string
  messages: ChatCompletionMessage[]
  stream: boolean
  temperature?: number
  top_p?: number
  max_tokens?: number
  frequency_penalty?: number
  presence_penalty?: number
  seed?: number
  tools?: ChatCompletionTool[]
  tool_choice?: 'auto'
  reasoning_effort?: 'minimal' | 'low' | 'medium' | 'high'
}

export interface ToolCallDelta {
  index: number
  id?: string
  type?: string
  function?: {
    name?: string
    arguments?: string
  }
}

export interface ChatCompletionChunk {
  id: string
  object: string
  created: number
  model: string
  choices: Array<{
    index: number
    delta: {
      role?: MessageRole
      content?: string
      reasoning_content?: string
      tool_calls?: ToolCallDelta[]
    }
    finish_reason: string | null
  }>
}

export interface ChatCompletionResponse {
  id: string
  object: string
  created: number
  model: string
  choices: Array<{
    index: number
    message: {
      role: MessageRole
      content: string
      reasoning_content?: string
      tool_calls?: ToolCall[]
    }
    finish_reason: string
  }>
  usage?: {
    prompt_tokens: number
    completion_tokens: number
    total_tokens: number
  }
}

export interface ImageGenerationRequest {
  model: string
  group?: string
  prompt: string
  n?: number
}

export interface ImageGenerationResponse {
  created?: number
  data?: Array<{
    url?: string
    b64_json?: string
    revised_prompt?: string
  }>
}

// Configuration types
export interface PlaygroundConfig {
  model: string
  group: string
  temperature: number
  top_p: number
  max_tokens: number
  frequency_penalty: number
  presence_penalty: number
  seed: number | null
  stream: boolean
}

export interface ParameterEnabled {
  temperature: boolean
  top_p: boolean
  max_tokens: boolean
  frequency_penalty: boolean
  presence_penalty: boolean
  seed: boolean
}

// Model and group options
export interface ModelOption {
  label: string
  value: string
}

export interface GroupOption {
  label: string
  value: string
  ratio: number
  desc?: string
}
