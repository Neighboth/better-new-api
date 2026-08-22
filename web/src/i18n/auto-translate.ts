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
// Runtime UI translation for content languages without a bundled locale
// file: the English bundle is machine-translated through /api/translate once
// per language and cached in localStorage. Static locale languages never go
// through this path.
import type { i18n as I18n } from 'i18next'

import { interfaceToContentLanguage } from './content-languages'
import enBundle from './locales/en.json'

const STATIC_LANGUAGES = new Set(['en', 'zhCN', 'zhTW', 'fr', 'ru', 'ja', 'vi'])

// Bump when the translation pipeline changes shape; stale caches drop.
const CACHE_VERSION = 'v1'

const inFlight = new Map<string, Promise<void>>()

function bundleHash(): string {
  const raw = JSON.stringify(enBundle)
  let hash = 5381
  for (let i = 0; i < raw.length; i++) {
    hash = ((hash << 5) + hash + raw.charCodeAt(i)) | 0
  }
  return hash.toString(36)
}

// Mirrors the backend guard: technical tokens (model names, endpoints,
// versions, URLs) are never translated.
function isTranslatable(text: string): boolean {
  const trimmed = text.trim()
  if (!trimmed) return false
  if (!/[A-Za-z\u0080-\uFFFF]/.test(trimmed)) return false
  if (/^https?:\/\//.test(trimmed)) return false
  if (
    !/\s/.test(trimmed) &&
    /^[A-Za-z0-9_./+:-]*[0-9./:-][A-Za-z0-9_./+:-]*$/.test(trimmed)
  ) {
    return false
  }
  return true
}

type TranslateResult = { success: boolean; data?: { texts: string[] } }

async function translateBatch(
  target: string,
  texts: string[]
): Promise<string[]> {
  const res = await fetch('/api/translate', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ source: 'en', target, texts }),
  })
  const json = (await res.json()) as TranslateResult
  if (!json.success || !json.data || json.data.texts.length !== texts.length) {
    throw new Error('translate failed')
  }
  return json.data.texts
}

// Translate every string leaf of the English bundle, keeping the key
// structure identical (keys ARE the English source strings).
async function translateBundle(
  target: string
): Promise<Record<string, unknown>> {
  const leaves: { path: string[]; text: string }[] = []
  const walk = (node: Record<string, unknown>, path: string[]) => {
    for (const [key, value] of Object.entries(node)) {
      if (typeof value === 'string') {
        leaves.push({ path: [...path, key], text: value })
      } else if (value && typeof value === 'object') {
        walk(value as Record<string, unknown>, [...path, key])
      }
    }
  }
  walk(enBundle as Record<string, unknown>, [])

  const translated = new Map<string, string>()
  const todo = leaves.filter((leaf) => isTranslatable(leaf.text))
  for (const leaf of leaves) {
    if (!isTranslatable(leaf.text)) {
      translated.set(leaf.path.join('\u0000'), leaf.text)
    }
  }
  const BATCH_TEXTS = 60
  const BATCH_CHARS = 3000
  let batch: typeof todo = []
  let batchChars = 0
  const flush = async () => {
    if (batch.length === 0) return
    const current = batch
    batch = []
    batchChars = 0
    const out = await translateBatch(
      target,
      current.map((leaf) => leaf.text)
    )
    current.forEach((leaf, i) => {
      translated.set(leaf.path.join('\u0000'), out[i] || leaf.text)
    })
  }
  for (const leaf of todo) {
    if (
      batch.length >= BATCH_TEXTS ||
      batchChars + leaf.text.length > BATCH_CHARS
    ) {
      await flush()
    }
    batch.push(leaf)
    batchChars += leaf.text.length
  }
  await flush()

  const rebuild = (
    node: Record<string, unknown>,
    path: string[]
  ): Record<string, unknown> => {
    const out: Record<string, unknown> = {}
    for (const [key, value] of Object.entries(node)) {
      if (typeof value === 'string') {
        out[key] = translated.get([...path, key].join('\u0000')) ?? value
      } else if (value && typeof value === 'object') {
        out[key] = rebuild(value as Record<string, unknown>, [...path, key])
      } else {
        out[key] = value
      }
    }
    return out
  }
  return rebuild(enBundle as Record<string, unknown>, [])
}

/**
 * Ensure i18next has a resource bundle for the interface language. Languages
 * without a static locale file get a machine-translated bundle, cached in
 * localStorage keyed by the English bundle hash. Failures are silent: the UI
 * stays on English fallback.
 */
export function ensureAutoLanguageBundle(
  i18n: I18n,
  interfaceCode: string
): Promise<void> {
  if (!interfaceCode || STATIC_LANGUAGES.has(interfaceCode)) {
    return Promise.resolve()
  }
  const contentCode = interfaceToContentLanguage(interfaceCode)
  if (contentCode === 'en') return Promise.resolve()

  const cacheKey = `autoI18n.${CACHE_VERSION}.${contentCode}.${bundleHash()}`
  const cached = (() => {
    try {
      return localStorage.getItem(cacheKey)
    } catch {
      return null
    }
  })()
  if (cached) {
    try {
      const bundle = JSON.parse(cached) as Record<string, unknown>
      i18n.addResourceBundle(interfaceCode, 'translation', bundle, true, true)
      return Promise.resolve()
    } catch {
      // corrupted cache; refetch
    }
  }

  const existing = inFlight.get(contentCode)
  if (existing) return existing

  const job = translateBundle(contentCode)
    .then((bundle) => {
      try {
        localStorage.setItem(cacheKey, JSON.stringify(bundle))
      } catch {
        // storage full; the bundle still applies for this session
      }
      i18n.addResourceBundle(interfaceCode, 'translation', bundle, true, true)
    })
    .catch(() => {
      // stay on English fallback
    })
    .finally(() => {
      inFlight.delete(contentCode)
    })
  inFlight.set(contentCode, job)
  return job
}
