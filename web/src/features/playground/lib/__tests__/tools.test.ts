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
import { afterEach, describe, expect, it, vi } from 'vitest'

import type { ToolEvent } from '../../types'
import {
  createUserMessage,
  createLoadingAssistantMessage,
} from '../message/message-utils'
import {
  buildApiTranscript,
  buildChatApiPayload,
} from '../streaming/payload-builder'
import { normalizePlanSteps } from '../tools/plan-utils'
import {
  mergeToolCallDeltas,
  parseToolArguments,
  truncateToolResult,
} from '../tools/tool-call-utils'
import {
  buildNativeThinkingSystemPrompt,
  buildPlaygroundToolDefinitions,
  buildThinkToolSystemPrompt,
} from '../tools/tool-definitions'

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('mergeToolCallDeltas', () => {
  it('stitches streamed fragments into a complete tool call', () => {
    let calls = mergeToolCallDeltas(
      [],
      [
        {
          index: 0,
          id: 'call_1',
          function: { name: 'web_search', arguments: '{"que' },
        },
      ]
    )
    calls = mergeToolCallDeltas(calls, [
      { index: 0, function: { arguments: 'ry":"new api"}' } },
    ])

    expect(calls).toHaveLength(1)
    expect(calls[0].id).toBe('call_1')
    expect(calls[0].function.name).toBe('web_search')
    expect(calls[0].function.arguments).toBe('{"query":"new api"}')
  })

  it('keeps parallel calls ordered by index', () => {
    let calls = mergeToolCallDeltas(
      [],
      [{ index: 0, id: 'a', function: { name: 'web_search', arguments: '{}' } }]
    )
    calls = mergeToolCallDeltas(calls, [
      { index: 1, id: 'b', function: { name: 'fetch_page', arguments: '{}' } },
    ])

    expect(calls.map((call) => call.id)).toEqual(['a', 'b'])
  })
})

describe('parseToolArguments', () => {
  it('parses valid JSON objects', () => {
    expect(parseToolArguments('{"query":"x"}')).toEqual({ query: 'x' })
  })

  it('returns an empty object for blank payloads', () => {
    expect(parseToolArguments('  ')).toEqual({})
  })

  it('returns null for invalid JSON or non-object payloads', () => {
    expect(parseToolArguments('{oops')).toBeNull()
    expect(parseToolArguments('42')).toBeNull()
    expect(parseToolArguments('[]')).toBeNull()
  })
})

describe('truncateToolResult', () => {
  it('keeps short text intact and truncates long text with a notice', () => {
    expect(truncateToolResult('short', 100)).toBe('short')
    const truncated = truncateToolResult('x'.repeat(1000), 100)
    expect(truncated.length).toBeLessThan(200)
    expect(truncated).toContain('[truncated]')
  })
})

describe('plan utils', () => {
  it('normalizes model steps, keeping at most one in_progress', () => {
    const steps = normalizePlanSteps([
      { title: 'research', status: 'in_progress' },
      { title: 'write', status: 'in_progress' },
      { title: '', status: 'pending' },
      'garbage',
      { title: 'ship', status: 'done' },
    ])

    expect(steps).toHaveLength(3)
    expect(steps[0]).toMatchObject({ title: 'research', status: 'in_progress' })
    expect(steps[1]).toMatchObject({ title: 'write', status: 'pending' })
    expect(steps[2]).toMatchObject({ title: 'ship', status: 'pending' })
  })
})

describe('buildPlaygroundToolDefinitions', () => {
  it('includes only enabled tools', () => {
    const tools = buildPlaygroundToolDefinitions({
      generate_image: true,
      update_plan: false,
    })

    expect(tools.map((tool) => tool.function.name)).toEqual([
      'generate_image',
    ])
  })

  it('adds the think tool only when forced thinking is on', () => {
    const base = {
      generate_image: false,
      update_plan: false,
    }

    expect(
      buildPlaygroundToolDefinitions(base).map((tool) => tool.function.name)
    ).toEqual([])
    expect(
      buildPlaygroundToolDefinitions(base, true).map(
        (tool) => tool.function.name
      )
    ).toEqual(['think'])
  })
})

describe('thinking system prompts', () => {
  it('embeds the depth instruction of the selected level', () => {
    expect(buildThinkToolSystemPrompt('lite')).toContain('very short')
    expect(buildThinkToolSystemPrompt('ultra')).toContain('exhaustively')
    expect(buildThinkToolSystemPrompt('medium')).toContain('moderate depth')
    expect(buildNativeThinkingSystemPrompt('high')).toContain(
      'several detailed paragraphs'
    )
  })

  it('forces the think tool before the first sentence and mid-message', () => {
    const prompt = buildThinkToolSystemPrompt('medium')
    expect(prompt).toContain('MUST call `think` before writing your first')
    expect(prompt).toContain('mid-message')
  })
})

describe('buildChatApiPayload tools', () => {
  const config = { model: 'gpt-4o', group: '', stream: true } as never
  const parameterEnabled = {
    temperature: false,
    top_p: false,
    max_tokens: false,
    frequency_penalty: false,
    presence_penalty: false,
    seed: false,
  }

  it('attaches tool definitions and tool_choice when tools exist', () => {
    const tools = buildPlaygroundToolDefinitions({
      generate_image: true,
      update_plan: false,
    })
    const payload = buildChatApiPayload(
      [{ role: 'user', content: 'hi' }],
      config,
      parameterEnabled,
      tools
    )

    expect(payload.tool_choice).toBe('auto')
    expect(payload.tools).toHaveLength(1)
  })

  it('omits tools entirely when the list is empty', () => {
    const payload = buildChatApiPayload(
      [{ role: 'user', content: 'hi' }],
      config,
      parameterEnabled,
      []
    )

    expect('tools' in payload).toBe(false)
    expect('tool_choice' in payload).toBe(false)
  })
})

describe('buildApiTranscript tool replay', () => {
  it('replays recorded tool calls in standard tool_calls format', () => {
    const user = createUserMessage('find news', 1)
    const assistant = createLoadingAssistantMessage(1)
    assistant.versions[0].content = 'Let me search. Here is what I found.'
    assistant.status = 'complete'
    const toolEvent: ToolEvent = {
      id: 'call_1',
      name: 'web_search',
      status: 'done',
      anchor: 15,
      arguments: '{"query":"new-api"}',
      result: '{"results":[]}',
    }
    assistant.toolEvents = [toolEvent]

    const transcript = buildApiTranscript([user, assistant])

    expect(transcript).toHaveLength(4)
    expect(transcript[1]).toEqual({
      role: 'assistant',
      content: 'Let me search. ',
      tool_calls: [
        {
          id: 'call_1',
          type: 'function',
          function: { name: 'web_search', arguments: '{"query":"new-api"}' },
        },
      ],
    })
    expect(transcript[2]).toEqual({
      role: 'tool',
      tool_call_id: 'call_1',
      name: 'web_search',
      content: '{"results":[]}',
    })
    expect(transcript[3]).toEqual({
      role: 'assistant',
      content: 'Here is what I found.',
    })
    expect(JSON.stringify(transcript)).not.toContain('Tools used in this turn')
  })

  it('groups parallel calls at the same position into one assistant turn', () => {
    const assistant = createLoadingAssistantMessage(1)
    assistant.versions[0].content = 'done'
    assistant.status = 'complete'
    assistant.toolEvents = [
      {
        id: 'a',
        name: 'web_search',
        status: 'done',
        anchor: 0,
        arguments: '{}',
        result: 'r1',
      },
      {
        id: 'b',
        name: 'fetch_page',
        status: 'done',
        anchor: 0,
        arguments: '{}',
        result: 'r2',
      },
    ]

    const transcript = buildApiTranscript([assistant])

    expect(transcript).toHaveLength(4)
    expect(transcript[0].tool_calls).toHaveLength(2)
    expect(transcript[1].role).toBe('tool')
    expect(transcript[2].role).toBe('tool')
    expect(transcript[3].content).toBe('done')
  })

  it('leaves messages without replay data untouched', () => {
    const user = createUserMessage('hi', 1)
    const assistant = createLoadingAssistantMessage(1)
    assistant.versions[0].content = 'answer'
    assistant.status = 'complete'
    assistant.toolEvents = [
      { id: 'call_1', name: 'web_search', status: 'done', anchor: 0 },
    ]

    const transcript = buildApiTranscript([user, assistant])

    expect(transcript).toHaveLength(2)
    expect(transcript[1]).toEqual({ role: 'assistant', content: 'answer' })
  })
})
