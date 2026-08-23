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
  Message,
  MessageAttachment,
  PlanStep,
  ToolEvent,
} from '../../types'

/**
 * Record a tool call that just started executing on the assistant message and
 * flag the tool as actively used so the UI can show a "using tool" indicator.
 */
export function applyToolEventStart(
  message: Message,
  event: ToolEvent
): Message {
  return {
    ...message,
    toolEvents: [...(message.toolEvents ?? []), event],
    activeTool: event.name,
  }
}

export type ToolEventFinish = {
  status: 'done' | 'error'
  summary?: string
  error?: string
  plan?: PlanStep[]
  attachments?: MessageAttachment[]
  sources?: { href: string; title: string }[]
}

/**
 * Resolve a running tool event with its outcome. Generated images, plan
 * snapshots and search sources are folded into the message here.
 */
export function applyToolEventFinish(
  message: Message,
  toolCallId: string,
  finish: ToolEventFinish
): Message {
  const toolEvents = (message.toolEvents ?? []).map((event) =>
    event.id === toolCallId
      ? {
          ...event,
          status: finish.status,
          summary: finish.summary ?? event.summary,
          error: finish.error,
          completedAt: Date.now(),
        }
      : event
  )

  const updated: Message = { ...message, toolEvents }

  if (finish.plan) {
    updated.plan = finish.plan
  }
  if (finish.attachments?.length) {
    updated.attachments = [
      ...(message.attachments ?? []),
      ...finish.attachments,
    ]
  }
  if (finish.sources?.length) {
    const existingHrefs = new Set(
      (message.sources ?? []).map((source) => source.href)
    )
    updated.sources = [
      ...(message.sources ?? []),
      ...finish.sources.filter((source) => !existingHrefs.has(source.href)),
    ]
  }

  return updated
}

/**
 * Short human-readable summary of a tool call shown next to its badge while
 * the call is still running (before the outcome summary is known).
 */
export function summarizeToolCallArguments(
  args: Record<string, unknown> | null
): string | undefined {
  if (!args) {
    return undefined
  }

  const value = args.prompt ?? args.query ?? args.url
  if (typeof value === 'string' && value.trim()) {
    const trimmed = value.trim()
    return trimmed.length > 80 ? `${trimmed.slice(0, 80)}…` : trimmed
  }

  return undefined
}
