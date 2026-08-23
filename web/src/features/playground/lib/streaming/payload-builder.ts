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

/**
 * Convert display messages into the API transcript used by chat requests and
 * by the client-side tool loop. Tool events recorded on assistant messages are
 * folded into a compact annotation so later turns keep tool context.
 */
export function buildApiTranscript(
  messages: Message[]
): ChatCompletionMessage[] {
  return messages.filter(isValidMessage).map((message) => {
    const apiMessage = formatMessageForAPI(message)
    const toolEvents = message.toolEvents ?? []

    if (
      message.from !== 'assistant' ||
      toolEvents.length === 0 ||
      typeof apiMessage.content !== 'string'
    ) {
      return apiMessage
    }

    const lines = toolEvents.map((event) => {
      const detail = event.error ?? event.summary ?? ''
      return `- ${event.name} (${event.status})${detail ? `: ${detail}` : ''}`
    })
    const annotation = [
      '[Tools used in this turn — call them again if you need fresh data]',
      ...lines,
    ].join('\n')

    return {
      ...apiMessage,
      content: apiMessage.content
        ? `${apiMessage.content}\n\n${annotation}`
        : annotation,
    }
  })
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
