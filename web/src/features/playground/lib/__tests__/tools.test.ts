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
import { fetchPageWithFallback, isFetchablePageUrl } from '../tools/page-fetch'
import {
  getToolProviderKeys,
  setToolProviderKeys,
} from '../tools/provider-keys'
import {
  appendPlanStep,
  normalizePlanSteps,
  togglePlanStep,
} from '../tools/plan-utils'
import {
  mergeToolCallDeltas,
  parseToolArguments,
  truncateToolResult,
} from '../tools/tool-call-utils'
import { buildPlaygroundToolDefinitions } from '../tools/tool-definitions'
import { searchWebWithFallback } from '../tools/web-search'

function mockFetchSequence(
  handlers: Array<(input: string) => Response | Promise<Response>>
) {
  const calls: string[] = []
  vi.stubGlobal('fetch', async (input: RequestInfo | URL) => {
    const url = String(input)
    calls.push(url)
    const handler = handlers[calls.length - 1]
    if (!handler) {
      throw new Error(`unexpected fetch call: ${url}`)
    }
    return handler(url)
  })
  return calls
}

afterEach(() => {
  vi.unstubAllGlobals()
  window.localStorage.clear()
})

describe('tool provider keys', () => {
  it('round-trips keys through localStorage and strips empties', () => {
    setToolProviderKeys({ tavily: ' tvly-abc ', firecrawl: '' })
    expect(getToolProviderKeys()).toEqual({ tavily: 'tvly-abc' })
  })

  it('removes the stored entry when every key is cleared', () => {
    setToolProviderKeys({ tavily: 'tvly-abc', firecrawl: 'fc-1' })
    setToolProviderKeys({ tavily: '', firecrawl: '' })
    expect(getToolProviderKeys()).toEqual({})
    expect(window.localStorage.getItem('playground_tool_api_keys')).toBeNull()
  })

  it('sends the Tavily key in the request body when configured', async () => {
    setToolProviderKeys({ tavily: 'tvly-secret', firecrawl: '' })
    let capturedBody = ''
    vi.stubGlobal('fetch', async (_input: RequestInfo | URL, init?: RequestInit) => {
      capturedBody = String(init?.body ?? '')
      return new Response(JSON.stringify({ results: [{ url: 'https://a.com' }] }), {
        status: 200,
      })
    })

    await searchWebWithFallback('q', 1)
    expect(capturedBody).toContain('tvly-secret')
  })
})

describe('searchWebWithFallback', () => {
  it('returns Tavily results when the first provider succeeds', async () => {
    mockFetchSequence([
      () =>
        new Response(
          JSON.stringify({
            results: [
              { title: 'Docs', url: 'https://example.com', content: 'info' },
            ],
          }),
          { status: 200 }
        ),
    ])

    const outcome = await searchWebWithFallback('new-api', 3)

    expect(outcome.provider).toBe('tavily')
    expect(outcome.results).toEqual([
      { title: 'Docs', url: 'https://example.com', snippet: 'info' },
    ])
  })

  it('falls back to Firecrawl when Tavily rejects the request', async () => {
    const calls = mockFetchSequence([
      () => new Response('forbidden', { status: 403 }),
      () =>
        new Response(
          JSON.stringify({
            data: [
              {
                url: 'https://firecrawl.dev',
                title: 'Firecrawl result',
                description: 'scraping API',
              },
            ],
          }),
          { status: 200 }
        ),
    ])

    const outcome = await searchWebWithFallback('firecrawl', 3)

    expect(outcome.provider).toBe('firecrawl')
    expect(outcome.results).toEqual([
      {
        title: 'Firecrawl result',
        url: 'https://firecrawl.dev',
        snippet: 'scraping API',
      },
    ])
    expect(calls[0]).toContain('tavily.com')
    expect(calls[1]).toContain('firecrawl.dev')
  })

  it('reports both provider errors when each one fails', async () => {
    mockFetchSequence([
      () => {
        throw new Error('network down')
      },
      () => new Response('bad gateway', { status: 502 }),
    ])

    await expect(searchWebWithFallback('anything', 2)).rejects.toThrow(
      /tavily: network down; firecrawl: HTTP 502/
    )
  })

  it('treats an empty result set as a failure and falls back', async () => {
    const calls = mockFetchSequence([
      () => new Response(JSON.stringify({ results: [] }), { status: 200 }),
      () =>
        new Response(
          JSON.stringify({
            data: [{ url: 'https://example.com', title: 'Hit', content: 'x' }],
          }),
          { status: 200 }
        ),
    ])

    const outcome = await searchWebWithFallback('query', 2)

    expect(outcome.provider).toBe('firecrawl')
    expect(calls).toHaveLength(2)
  })
})

describe('fetchPageWithFallback', () => {
  it('returns Tavily extracted content on success', async () => {
    mockFetchSequence([
      () =>
        new Response(
          JSON.stringify({ results: [{ raw_content: '# Hello' }] }),
          { status: 200 }
        ),
    ])

    const page = await fetchPageWithFallback('https://example.com')

    expect(page.provider).toBe('tavily')
    expect(page.content).toBe('# Hello')
  })

  it('falls back to r.jina.ai when Tavily finds nothing', async () => {
    const calls = mockFetchSequence([
      () => new Response(JSON.stringify({ results: [] }), { status: 200 }),
      () => new Response('page body', { status: 200 }),
    ])

    const page = await fetchPageWithFallback('https://example.com/docs')

    expect(page.provider).toBe('jina')
    expect(page.content).toBe('page body')
    expect(calls[1]).toBe('https://r.jina.ai/https://example.com/docs')
  })

  it('rejects when every provider fails', async () => {
    mockFetchSequence([
      () => new Response('nope', { status: 500 }),
      () => new Response('nope', { status: 404 }),
    ])

    await expect(fetchPageWithFallback('https://example.com')).rejects.toThrow(
      /tavily: HTTP 500; jina: HTTP 404/
    )
  })
})

describe('isFetchablePageUrl', () => {
  it('accepts http(s) URLs only', () => {
    expect(isFetchablePageUrl('https://example.com')).toBe(true)
    expect(isFetchablePageUrl('http://example.com')).toBe(true)
    expect(isFetchablePageUrl('ftp://example.com')).toBe(false)
    expect(isFetchablePageUrl('not a url')).toBe(false)
  })
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

  it('lets the user toggle a step between done and pending', () => {
    const plan = appendPlanStep([], 'draft')
    const completed = togglePlanStep(plan, plan[0].id)
    expect(completed[0].status).toBe('completed')
    const reopened = togglePlanStep(completed, plan[0].id)
    expect(reopened[0].status).toBe('pending')
  })
})

describe('buildPlaygroundToolDefinitions', () => {
  it('includes only enabled tools', () => {
    const tools = buildPlaygroundToolDefinitions({
      generate_image: true,
      web_search: true,
      fetch_page: false,
      update_plan: false,
    })

    expect(tools.map((tool) => tool.function.name)).toEqual([
      'generate_image',
      'web_search',
    ])
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
      generate_image: false,
      web_search: true,
      fetch_page: false,
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

describe('buildApiTranscript tool annotation', () => {
  it('folds recorded tool events into assistant content for later turns', () => {
    const user = createUserMessage('find news', 1)
    const assistant = createLoadingAssistantMessage(1)
    assistant.versions[0].content = 'Here is what I found.'
    assistant.status = 'complete'
    const toolEvent: ToolEvent = {
      id: 'call_1',
      name: 'web_search',
      status: 'done',
      summary: 'latest new-api release',
      startedAt: 1,
    }
    assistant.toolEvents = [toolEvent]

    const transcript = buildApiTranscript([user, assistant])

    expect(transcript).toHaveLength(2)
    const assistantContent = transcript[1].content as string
    expect(assistantContent).toContain('Here is what I found.')
    expect(assistantContent).toContain('web_search (done)')
    expect(assistantContent).toContain('latest new-api release')
  })
})
