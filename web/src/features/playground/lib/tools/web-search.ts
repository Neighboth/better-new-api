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
import { ERROR_MESSAGES } from '../../constants'

const TAVILY_SEARCH_URL = 'https://api.tavily.com/search'
const FIRECRAWL_SEARCH_URL = 'https://api.firecrawl.dev/v1/search'
const SEARCH_TIMEOUT_MS = 20_000

export interface WebSearchResultItem {
  title: string
  url: string
  snippet: string
}

export interface WebSearchOutcome {
  provider: 'tavily' | 'firecrawl'
  results: WebSearchResultItem[]
}

function withTimeout(signal: AbortSignal | undefined): AbortSignal {
  const timeout = AbortSignal.timeout(SEARCH_TIMEOUT_MS)
  return signal ? AbortSignal.any([signal, timeout]) : timeout
}

async function postJson(
  url: string,
  body: Record<string, unknown>,
  signal: AbortSignal | undefined
): Promise<unknown> {
  const response = await fetch(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
    signal: withTimeout(signal),
  })

  if (!response.ok) {
    throw new Error(`HTTP ${response.status}`)
  }

  return response.json() as Promise<unknown>
}

function normalizeResults(items: unknown): WebSearchResultItem[] {
  if (!Array.isArray(items)) {
    return []
  }

  return items
    .map((item): WebSearchResultItem | null => {
      if (!item || typeof item !== 'object') {
        return null
      }

      const record = item as Record<string, unknown>
      const url = typeof record.url === 'string' ? record.url : ''
      if (!url) {
        return null
      }

      const snippetSource = [
        record.content,
        record.description,
        record.markdown,
      ].find((value) => typeof value === 'string' && value.trim())

      return {
        title:
          typeof record.title === 'string' && record.title.trim()
            ? record.title
            : url,
        url,
        snippet: typeof snippetSource === 'string' ? snippetSource.trim() : '',
      }
    })
    .filter((item): item is WebSearchResultItem => Boolean(item))
}

// Calls go straight from the browser, keyless; the server never proxies
// third-party traffic.
async function searchWithTavily(
  query: string,
  maxResults: number,
  signal: AbortSignal | undefined
): Promise<WebSearchResultItem[]> {
  const data = (await postJson(
    TAVILY_SEARCH_URL,
    {
      query,
      max_results: maxResults,
      search_depth: 'basic',
      include_answer: false,
    },
    signal
  )) as { results?: unknown }

  return normalizeResults(data?.results)
}

async function searchWithFirecrawl(
  query: string,
  maxResults: number,
  signal: AbortSignal | undefined
): Promise<WebSearchResultItem[]> {
  const data = (await postJson(
    FIRECRAWL_SEARCH_URL,
    { query, limit: maxResults },
    signal
  )) as { data?: unknown }

  return normalizeResults(data?.data)
}

/**
 * Search the web from the browser, falling back to the next provider when one
 * fails (rate limit, outage, keyless quota exhausted, ...). Firecrawl goes
 * first because its keyless tier is the most reliable.
 */
export async function searchWebWithFallback(
  query: string,
  maxResults: number,
  signal?: AbortSignal
): Promise<WebSearchOutcome> {
  const providers = [
    { name: 'firecrawl' as const, run: searchWithFirecrawl },
    { name: 'tavily' as const, run: searchWithTavily },
  ]

  const errors: string[] = []
  for (const provider of providers) {
    try {
      const results = await provider.run(query, maxResults, signal)
      if (results.length === 0) {
        throw new Error('empty result set')
      }
      return { provider: provider.name, results }
    } catch (error) {
      if (signal?.aborted) {
        throw error
      }
      errors.push(
        `${provider.name}: ${error instanceof Error ? error.message : String(error)}`
      )
    }
  }

  throw new Error(`${ERROR_MESSAGES.WEB_SEARCH_FAILED} (${errors.join('; ')})`)
}
