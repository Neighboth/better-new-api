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
  provider: 'firecrawl' | 'tavily' | 'searxng' | 'duckduckgo' | 'wikipedia'
  results: WebSearchResultItem[]
}

function withTimeout(signal: AbortSignal | undefined): AbortSignal {
  const timeout = AbortSignal.timeout(SEARCH_TIMEOUT_MS)
  return signal ? AbortSignal.any([signal, timeout]) : timeout
}

async function postJson(
  url: string,
  body: Record<string, unknown>,
  signal: AbortSignal | undefined,
  headers: Record<string, string> = {}
): Promise<unknown> {
  const response = await fetch(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...headers },
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
  signal: AbortSignal | undefined,
  apiKey: string | undefined
): Promise<WebSearchResultItem[]> {
  const data = (await postJson(
    TAVILY_SEARCH_URL,
    {
      api_key: apiKey || 'tvly-anonymous',
      query,
      max_results: maxResults,
      search_depth: 'basic',
      include_answer: false,
    },
    signal,
    apiKey ? { Authorization: `Bearer ${apiKey}` } : {}
  )) as { results?: unknown }

  return normalizeResults(data?.results)
}

async function searchWithFirecrawl(
  query: string,
  maxResults: number,
  signal: AbortSignal | undefined,
  apiKey: string | undefined
): Promise<WebSearchResultItem[]> {
  const data = (await postJson(
    FIRECRAWL_SEARCH_URL,
    { query, limit: maxResults },
    signal,
    apiKey ? { Authorization: `Bearer ${apiKey}` } : {}
  )) as { data?: unknown }

  return normalizeResults(data?.data)
}

async function searchWithSearXNG(
  query: string,
  maxResults: number,
  signal: AbortSignal | undefined,
  host: string | undefined
): Promise<WebSearchResultItem[]> {
  if (!host) {
    throw new Error('SearXNG host is not configured')
  }

  const url = new URL(`${host.replace(/\/$/, '')}/search`)
  url.searchParams.set('q', query)
  url.searchParams.set('format', 'json')

  const response = await fetch(url.toString(), {
    method: 'GET',
    signal: withTimeout(signal),
  })

  if (!response.ok) {
    throw new Error(`HTTP ${response.status}`)
  }

  const data = (await response.json()) as { results?: unknown[] }
  const results = data.results || []

  return results
    .map((item: any): WebSearchResultItem | null => {
      if (!item || !item.url) return null
      return {
        title: item.title || item.url,
        url: item.url,
        snippet: item.content || '',
      }
    })
    .filter((item): item is WebSearchResultItem => Boolean(item))
    .slice(0, maxResults)
}

// DuckDuckGo instant-answer API: keyless and CORS-friendly, but only returns
// hits for queries with an encyclopedic answer, so it is a late fallback.
async function searchWithDuckDuckGo(
  query: string,
  maxResults: number,
  signal: AbortSignal | undefined
): Promise<WebSearchResultItem[]> {
  const response = await fetch(
    `https://api.duckduckgo.com/?q=${encodeURIComponent(query)}&format=json&no_html=1&no_redirect=1`,
    { signal: withTimeout(signal) }
  )
  if (!response.ok) {
    throw new Error(`HTTP ${response.status}`)
  }

  const data = (await response.json()) as {
    AbstractText?: unknown
    AbstractURL?: unknown
    Heading?: unknown
    RelatedTopics?: unknown
  }

  const results: WebSearchResultItem[] = []
  if (typeof data.AbstractText === 'string' && data.AbstractText.trim()) {
    results.push({
      title:
        typeof data.Heading === 'string' && data.Heading ? data.Heading : query,
      url: typeof data.AbstractURL === 'string' ? data.AbstractURL : '',
      snippet: data.AbstractText.trim(),
    })
  }

  const collectTopics = (topics: unknown): void => {
    if (!Array.isArray(topics) || results.length >= maxResults) {
      return
    }
    for (const topic of topics) {
      if (results.length >= maxResults) {
        return
      }
      if (!topic || typeof topic !== 'object') {
        continue
      }
      const record = topic as Record<string, unknown>
      if (Array.isArray(record.Topics)) {
        collectTopics(record.Topics)
        continue
      }
      if (
        typeof record.Text === 'string' &&
        typeof record.FirstURL === 'string'
      ) {
        const [title, ...rest] = record.Text.split(' - ')
        results.push({
          title: title.trim() || record.FirstURL,
          url: record.FirstURL,
          snippet: rest.join(' - ').trim() || record.Text,
        })
      }
    }
  }
  collectTopics(data.RelatedTopics)

  return results.slice(0, maxResults).filter((item) => item.url)
}

// Wikipedia opensearch: keyless, CORS-enabled via origin=*, decent last
// resort for general knowledge queries.
async function searchWithWikipedia(
  query: string,
  maxResults: number,
  signal: AbortSignal | undefined
): Promise<WebSearchResultItem[]> {
  const response = await fetch(
    `https://en.wikipedia.org/w/api.php?action=opensearch&search=${encodeURIComponent(query)}&limit=${maxResults}&namespace=0&format=json&origin=*`,
    { signal: withTimeout(signal) }
  )
  if (!response.ok) {
    throw new Error(`HTTP ${response.status}`)
  }

  const data = (await response.json()) as unknown
  if (!Array.isArray(data) || data.length < 4) {
    return []
  }

  const [titles, descriptions, urls] = [
    data[1],
    data[2],
    data[3],
  ] as unknown[][]
  if (!Array.isArray(titles) || !Array.isArray(urls)) {
    return []
  }

  return titles
    .map(
      (title, index): WebSearchResultItem => ({
        title: typeof title === 'string' ? title : String(title),
        url: typeof urls[index] === 'string' ? (urls[index] as string) : '',
        snippet:
          Array.isArray(descriptions) && typeof descriptions[index] === 'string'
            ? (descriptions[index] as string)
            : '',
      })
    )
    .filter((item) => item.url)
    .slice(0, maxResults)
}

/**
 * Search the web from the browser, falling back to the next provider when one
 * fails (rate limit, outage, keyless quota exhausted, empty results, ...).
 * Firecrawl goes first because its keyless tier is the most reliable.
 */
export async function searchWebWithFallback(
  query: string,
  maxResults: number,
  settings?: {
    tavilyKey?: string
    tavilyAnonymous?: boolean
    firecrawlKey?: string
    firecrawlAnonymous?: boolean
    searxngHost?: string
    fallbackEnabled?: boolean
  },
  signal?: AbortSignal
): Promise<WebSearchOutcome> {
  const providers: Array<{
    name: WebSearchOutcome['provider']
    run: (
      query: string,
      maxResults: number,
      signal: AbortSignal | undefined,
      ...args: any[]
    ) => Promise<WebSearchResultItem[]>
    args?: any[]
    skip?: boolean
  }> = []

  if (settings?.searxngHost) {
    providers.push({
      name: 'searxng',
      run: searchWithSearXNG,
      args: [settings.searxngHost],
    })
  }

  if (settings?.firecrawlAnonymous) {
    providers.push({
      name: 'firecrawl',
      run: searchWithFirecrawl,
      args: [''],
    })
  }
  if (settings?.firecrawlKey) {
    providers.push({
      name: 'firecrawl',
      run: searchWithFirecrawl,
      args: [settings.firecrawlKey],
    })
  }
  if (!settings?.firecrawlKey && !settings?.firecrawlAnonymous && !settings?.tavilyKey && !settings?.tavilyAnonymous && !settings?.searxngHost) {
    providers.push({ name: 'firecrawl', run: searchWithFirecrawl, args: [''] })
  }

  if (settings?.tavilyAnonymous) {
    providers.push({
      name: 'tavily',
      run: searchWithTavily,
      args: [''],
    })
  }
  if (settings?.tavilyKey) {
    providers.push({
      name: 'tavily',
      run: searchWithTavily,
      args: [settings.tavilyKey],
    })
  }
  if (!settings?.firecrawlKey && !settings?.firecrawlAnonymous && !settings?.tavilyKey && !settings?.tavilyAnonymous && !settings?.searxngHost) {
    providers.push({ name: 'tavily', run: searchWithTavily, args: [''] })
  }

  providers.push({ name: 'duckduckgo', run: searchWithDuckDuckGo })
  providers.push({ name: 'wikipedia', run: searchWithWikipedia })

  const errors: string[] = []

  let providersTried = new Set<string>()

  for (const provider of providers) {
    if (provider.skip) continue

    // If fallback is disabled, allow multiple attempts ONLY for the same provider (e.g. anonymous then key).
    // Break if we are trying to switch to a different provider.
    if (settings && !settings.fallbackEnabled && providersTried.size > 0 && !providersTried.has(provider.name)) {
        break
    }

    try {
      providersTried.add(provider.name)
      const results = await provider.run(query, maxResults, signal, ...(provider.args || []))
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
