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
*/
import { describe, expect, it } from 'vitest'

import type { Message, ThoughtBlock, ToolEvent } from '../../types'
import {
  buildMessageRenderItems,
  mergeAdjacentThoughts,
} from '../message/message-render-items'
import { createLoadingAssistantMessage } from '../message/message-utils'

function assistantMessage(content: string): Message {
  const message = createLoadingAssistantMessage(1)
  message.versions[0].content = content
  message.status = 'complete'
  return message
}

function thought(content: string, anchor: number): ThoughtBlock {
  return { id: `t-${anchor}-${content.length}`, content, anchor }
}

describe('mergeAdjacentThoughts', () => {
  it('merges thoughts separated only by whitespace (tool-only gap)', () => {
    const merged = mergeAdjacentThoughts('hello world', [
      thought('first pass', 5),
      thought('second pass', 5),
    ])

    expect(merged).toHaveLength(1)
    expect(merged[0].content).toBe('first pass\n\nsecond pass')
  })

  it('keeps thoughts apart when visible text sits between them', () => {
    const merged = mergeAdjacentThoughts('hello world', [
      thought('first pass', 0),
      thought('second pass', 8),
    ])

    expect(merged).toHaveLength(2)
  })
})

function itemShape(item: { kind: string; text?: string }) {
  return { kind: item.kind, text: item.text }
}

describe('buildMessageRenderItems', () => {
  it('interleaves thought blocks at their content position', () => {
    const message = assistantMessage('Let me check. Done!')
    message.thoughts = [thought('reasoning', 14)]

    const items = buildMessageRenderItems(message)

    expect(items.map(itemShape)).toEqual([
      { kind: 'text', text: 'Let me check. ' },
      { kind: 'thought', text: undefined },
      { kind: 'text', text: 'Done!' },
    ])
    expect(items[1]).toMatchObject({
      kind: 'thought',
      thought: { content: 'reasoning' },
    })
  })

  it('renders a running tool indicator at the call position', () => {
    const message = assistantMessage('searching now...')
    const event: ToolEvent = {
      id: 'call_1',
      name: 'web_search',
      status: 'running',
      anchor: 10,
    }
    message.toolEvents = [event]

    const items = buildMessageRenderItems(message)

    expect(items.map(itemShape)).toEqual([
      { kind: 'text', text: 'searching ' },
      { kind: 'tool-running', text: undefined },
      { kind: 'text', text: 'now...' },
    ])
    expect(items[1]).toMatchObject({
      kind: 'tool-running',
      event: { id: 'call_1' },
    })
  })

  it('does not render finished tool events inline', () => {
    const message = assistantMessage('all done')
    message.toolEvents = [
      { id: 'call_1', name: 'web_search', status: 'done', anchor: 4 },
    ]

    const items = buildMessageRenderItems(message)

    expect(items.map(itemShape)).toEqual([{ kind: 'text', text: 'all done' }])
  })

  it('anchors the plan table to the last update_plan call', () => {
    const message = assistantMessage('planning... executing')
    message.plan = [{ id: 's1', title: 'step', status: 'pending' }]
    message.toolEvents = [
      { id: 'c1', name: 'update_plan', status: 'done', anchor: 3 },
      { id: 'c2', name: 'update_plan', status: 'done', anchor: 11 },
    ]

    const items = buildMessageRenderItems(message)

    expect(items.map(itemShape)).toEqual([
      { kind: 'text', text: 'planning...' },
      { kind: 'plan', text: undefined },
      { kind: 'text', text: ' executing' },
    ])
  })

  it('clamps anchors that exceed the content length', () => {
    const message = assistantMessage('short')
    message.thoughts = [thought('late thought', 999)]

    const items = buildMessageRenderItems(message)

    expect(items.map(itemShape)).toEqual([
      { kind: 'text', text: 'short' },
      { kind: 'thought', text: undefined },
    ])
  })
})
