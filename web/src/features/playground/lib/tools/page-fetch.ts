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
import { ERROR_MESSAGES, MAX_TOOL_RESULT_CHARS } from '../../constants'
import { truncateToolResult } from './tool-call-utils'

const TAVILY_EXTRACT_URL = 'https://api.tavily.com/extract'
const JINA_READER_BASE_URL = 'https://r.jina.ai/'
const ALLORIGINS_RAW_URL = 'https://api.allorigins.win/raw?url='
const FETCH_TIMEOUT_MS = 25_000

export interface PageFetchOutcome {
  provider: 'jina' | 'tavily' | 'allorigins' | 'direct'
  url: string
  content: string
}

/**
 * Reduce raw HTML to readable plain text in the browser. Used by the last
 * fallback providers that return the page as-is.
 */
export function htmlToPlainText(html: string): string {
  const doc = new DOMParser().parseFromString(html, 'text/html')
  doc
    .querySelectorAll(
      'script, style, noscript, svg, iframe, canvas, form, nav, footer, header'
    )
    .forEach((element) => element.remove())

  const text = doc.body?.textContent ?? doc.documentElement.textContent ?? ''
  return text
    .replaceAll(/[ \t]+\n/g, '\n')
    .replaceAll(/\n{3,}/g, '\n\n')
    .replaceAll(/[ \t]{2,}/g, ' ')
    .trim()
}

function looksLikeHtml(content: string): boolean {
  return /^\s*<!doctype html|^\s*<html[\s>]/i.test(content)
}

function withTimeout(signal: AbortSignal | undefined): AbortSignal {
  const timeout = AbortSignal.timeout(FETCH_TIMEOUT_MS)
  return signal ? AbortSignal.any([signal, timeout]) : timeout
}

export function isFetchablePageUrl(url: string): boolean {
  try {
    const parsed = new URL(url)
    return parsed.protocol === 'http:' || parsed.protocol === 'https:'
  } catch {
    return false
  }
}

async function fetchWithTavily(
  url: string,
  signal: AbortSignal | undefined
): Promise<string> {
  const response = await fetch(TAVILY_EXTRACT_URL, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ urls: [url] }),
    signal: withTimeout(signal),
  })

  if (!response.ok) {
    throw new Error(`HTTP ${response.status}`)
  }

  const data = (await response.json()) as { results?: unknown }
  const first = Array.isArray(data?.results) ? data.results[0] : undefined
  const content =
    first && typeof first === 'object'
      ? (first as Record<string, unknown>).raw_content
      : undefined

  if (typeof content !== 'string' || !content.trim()) {
    throw new Error('empty page content')
  }

  return content
}

async function fetchWithJina(
  url: string,
  signal: AbortSignal | undefined
): Promise<string> {
  const response = await fetch(`${JINA_READER_BASE_URL}${url}`, {
    headers: { Accept: 'text/plain' },
    signal: withTimeout(signal),
  })

  if (!response.ok) {
    throw new Error(`HTTP ${response.status}`)
  }

  const content = await response.text()
  if (!content.trim()) {
    throw new Error('empty page content')
  }

  return content
}

async function fetchText(
  url: string,
  signal: AbortSignal | undefined
): Promise<string> {
  const response = await fetch(url, { signal: withTimeout(signal) })
  if (!response.ok) {
    throw new Error(`HTTP ${response.status}`)
  }

  const raw = await response.text()
  const content = looksLikeHtml(raw) ? htmlToPlainText(raw) : raw.trim()
  if (!content.trim()) {
    throw new Error('empty page content')
  }

  return content
}

// CORS proxy that mirrors the raw page; the HTML is converted locally.
async function fetchWithAllOrigins(
  url: string,
  signal: AbortSignal | undefined
): Promise<string> {
  return fetchText(`${ALLORIGINS_RAW_URL}${encodeURIComponent(url)}`, signal)
}

// Last resort: fetch the page directly (only works for CORS-open sites) and
// convert the markup to text locally.
async function fetchDirect(
  url: string,
  signal: AbortSignal | undefined
): Promise<string> {
  return fetchText(url, signal)
}

/**
 * Fetch readable page content from the browser, falling back to the next
 * provider when one fails. The Jina reader goes first because it works
 * keyless most reliably; the chain ends with a direct fetch so every quota
 * exhaustion still has a way out.
 */
export async function fetchPageWithFallback(
  url: string,
  signal?: AbortSignal
): Promise<PageFetchOutcome> {
  const providers = [
    { name: 'jina' as const, run: fetchWithJina },
    { name: 'tavily' as const, run: fetchWithTavily },
    { name: 'allorigins' as const, run: fetchWithAllOrigins },
    { name: 'direct' as const, run: fetchDirect },
  ]

  const errors: string[] = []
  for (const provider of providers) {
    try {
      const content = await provider.run(url, signal)
      return {
        provider: provider.name,
        url,
        content: truncateToolResult(content.trim(), MAX_TOOL_RESULT_CHARS),
      }
    } catch (error) {
      if (signal?.aborted) {
        throw error
      }
      errors.push(
        `${provider.name}: ${error instanceof Error ? error.message : String(error)}`
      )
    }
  }

  throw new Error(`${ERROR_MESSAGES.PAGE_FETCH_FAILED} (${errors.join('; ')})`)
}
