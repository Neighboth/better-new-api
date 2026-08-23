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
  ChatCompletionMessage,
  ChatCompletionRequest,
  ChatCompletionTool,
  Message,
  PlaygroundConfig,
  ParameterEnabled,
} from '../../types'
import { formatMessageForAPI, isValidMessage } from '../message/message-utils'

// Tool results replayed into the transcript are capped so old turns never
// blow up the context window.
const MAX_REPLAYED_TOOL_RESULT_CHARS = 4_000

/**
 * Convert display messages into the API transcript used by chat requests and
 * by the client-side tool loop. Assistant messages with recorded tool calls
 * are replayed in the standard assistant tool_calls + tool message format,
 * split at the content position where each call happened.
 */
export function buildApiTranscript(
  messages: Message[]
): ChatCompletionMessage[] {
  const transcript: ChatCompletionMessage[] = []

  for (const message of messages.filter(isValidMessage)) {
    const apiMessage = formatMessageForAPI(message)
    const replayable = (message.toolEvents ?? []).filter(
      (event) => event.arguments != null && event.result != null
    )

    if (
      message.from !== 'assistant' ||
      replayable.length === 0 ||
      typeof apiMessage.content !== 'string'
    ) {
      transcript.push(apiMessage)
      continue
    }

    const content = apiMessage.content
    const sorted = [...replayable].sort(
      (a, b) => (a.anchor ?? 0) - (b.anchor ?? 0)
    )

    // Group events that happened at the same content position so they replay
    // as one assistant turn with parallel tool calls.
    const groups = new Map<number, typeof sorted>()
    for (const event of sorted) {
      const anchor = Math.max(0, Math.min(event.anchor ?? 0, content.length))
      const group = groups.get(anchor) ?? []
      group.push(event)
      groups.set(anchor, group)
    }

    let cursor = 0
    for (const [anchor, events] of groups) {
      transcript.push({
        role: 'assistant',
        content: content.slice(cursor, anchor) || null,
        tool_calls: events.map((event) => ({
          id: event.id,
          type: 'function',
          function: { name: event.name, arguments: event.arguments ?? '{}' },
        })),
      })
      for (const event of events) {
        const result = event.result ?? ''
        transcript.push({
          role: 'tool',
          tool_call_id: event.id,
          name: event.name,
          content:
            result.length > MAX_REPLAYED_TOOL_RESULT_CHARS
              ? `${result.slice(0, MAX_REPLAYED_TOOL_RESULT_CHARS)}\n[truncated]`
              : result,
        })
      }
      cursor = anchor
    }

    const rest = content.slice(cursor)
    if (rest) {
      transcript.push({ role: 'assistant', content: rest })
    }
  }

  return transcript
}

/**
 * Build a chat completion payload from an already formatted API transcript.
 */
export function buildChatApiPayload(
  apiMessages: ChatCompletionMessage[],
  config: PlaygroundConfig,
  parameterEnabled: ParameterEnabled,
  tools: ChatCompletionTool[] = []
): ChatCompletionRequest {
  const payload: ChatCompletionRequest = {
    model: config.model,
    group: config.group,
    messages: apiMessages,
    stream: config.stream,
  }

  if (tools.length > 0) {
    payload.tools = tools
    payload.tool_choice = 'auto'
  }

  if (parameterEnabled.temperature) {
    payload.temperature = config.temperature
  }

  if (parameterEnabled.top_p) {
    payload.top_p = config.top_p
  }

  if (parameterEnabled.max_tokens) {
    payload.max_tokens = config.max_tokens
  }

  if (parameterEnabled.frequency_penalty) {
    payload.frequency_penalty = config.frequency_penalty
  }

  if (parameterEnabled.presence_penalty) {
    payload.presence_penalty = config.presence_penalty
  }

  if (parameterEnabled.seed && config.seed !== null) {
    payload.seed = config.seed
  }

  return payload
}
