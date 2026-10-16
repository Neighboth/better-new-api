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
import type { Message, PlanStep, ThoughtBlock, ToolEvent } from '../../types'

export type MessageRenderItem =
  | { kind: 'text'; key: string; text: string }
  | { kind: 'thought'; key: string; thought: ThoughtBlock }
  | { kind: 'tool-running'; key: string; event: ToolEvent }
  | { kind: 'plan'; key: string; steps: PlanStep[] }

/**
 * Merge thoughts that are only separated by tool calls or whitespace: two
 * thinking passes with no user-visible text between them read as one.
 */
export function mergeAdjacentThoughts(
  content: string,
  thoughts: ThoughtBlock[]
): ThoughtBlock[] {
  const sorted = [...thoughts].sort((a, b) => a.anchor - b.anchor)
  const merged: ThoughtBlock[] = []

  for (const thought of sorted) {
    const previous = merged.at(-1)
    if (!previous) {
      merged.push({ ...thought })
      continue
    }

    const between = content.slice(previous.anchor, thought.anchor).trim()
    if (between === '') {
      previous.content = `${previous.content}\n\n${thought.content}`
      previous.completedAt = thought.completedAt ?? previous.completedAt
    } else {
      merged.push({ ...thought })
    }
  }

  return merged
}

function clampAnchor(
  anchor: number | undefined,
  contentLength: number
): number {
  if (anchor === undefined || Number.isNaN(anchor)) {
    return contentLength
  }
  return Math.max(0, Math.min(anchor, contentLength))
}

/**
 * Split the assistant content at tool/thought anchors into an ordered list of
 * render items, so blocks appear exactly where the model produced them.
 */
export function buildMessageRenderItems(message: Message): MessageRenderItem[] {
  const content = message.versions[0]?.content ?? ''
  const thoughts = mergeAdjacentThoughts(content, message.thoughts ?? [])
  const runningEvents = (message.toolEvents ?? []).filter(
    (event) => event.status === 'running'
  )

  // Every update_plan call renders its own snapshot table at the position it
  // was made, so the history of plan changes stays visible.
  const planEvents = (message.toolEvents ?? []).filter(
    (event) => event.name === 'update_plan' && event.status !== 'running'
  )
  const planItems = planEvents.flatMap((event, order) => {
    const steps =
      event.plan ?? (order === planEvents.length - 1 ? message.plan : undefined)
    if (!steps?.length) {
      return []
    }
    return [
      {
        anchor: clampAnchor(event.anchor, content.length),
        order: thoughts.length + runningEvents.length + order,
        item: { kind: 'plan' as const, key: `plan-${event.id}`, steps },
      },
    ]
  })
  // Legacy messages only keep the latest plan on the message itself.
  if (planItems.length === 0 && message.plan?.length) {
    planItems.push({
      anchor: content.length,
      order: thoughts.length + runningEvents.length,
      item: { kind: 'plan' as const, key: 'plan-legacy', steps: message.plan },
    })
  }

  type Positioned = {
    anchor: number
    order: number
    item: MessageRenderItem
  }
  const positioned: Positioned[] = [
    ...thoughts.map((thought, order) => ({
      anchor: clampAnchor(thought.anchor, content.length),
      order,
      item: { kind: 'thought' as const, key: thought.id, thought },
    })),
    ...runningEvents.map((event, order) => ({
      anchor: clampAnchor(event.anchor, content.length),
      order: thoughts.length + order,
      item: { kind: 'tool-running' as const, key: event.id, event },
    })),
    ...planItems,
  ].sort((a, b) => a.anchor - b.anchor || a.order - b.order)

  const items: MessageRenderItem[] = []
  let cursor = 0
  for (const entry of positioned) {
    const text = content.slice(cursor, entry.anchor)
    if (text) {
      items.push({ kind: 'text', key: `text-${cursor}`, text })
    }
    items.push(entry.item)
    cursor = entry.anchor
  }

  const rest = content.slice(cursor)
  if (rest) {
    items.push({ kind: 'text', key: `text-${cursor}`, text: rest })
  }

  return items
}
