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

const PROVIDER_KEYS_STORAGE_KEY = 'playground_tool_api_keys'

export interface ToolProviderKeys {
  tavily?: string
  firecrawl?: string
}

/**
 * Optional third-party API keys for the client-side search/fetch providers.
 * Both providers work keyless; a key raises quota and reliability. Keys are
 * stored in this browser's localStorage only and never sent to the server.
 */
export function getToolProviderKeys(): ToolProviderKeys {
  if (typeof window === 'undefined') {
    return {}
  }

  try {
    const raw = window.localStorage.getItem(PROVIDER_KEYS_STORAGE_KEY)
    if (!raw) {
      return {}
    }
    const parsed: unknown = JSON.parse(raw)
    if (!parsed || typeof parsed !== 'object') {
      return {}
    }
    const record = parsed as Record<string, unknown>
    return {
      tavily: typeof record.tavily === 'string' ? record.tavily : undefined,
      firecrawl:
        typeof record.firecrawl === 'string' ? record.firecrawl : undefined,
    }
  } catch {
    return {}
  }
}

export function setToolProviderKeys(keys: ToolProviderKeys): void {
  if (typeof window === 'undefined') {
    return
  }

  const cleaned: ToolProviderKeys = {
    tavily: keys.tavily?.trim() || undefined,
    firecrawl: keys.firecrawl?.trim() || undefined,
  }

  try {
    if (!cleaned.tavily && !cleaned.firecrawl) {
      window.localStorage.removeItem(PROVIDER_KEYS_STORAGE_KEY)
    } else {
      window.localStorage.setItem(
        PROVIDER_KEYS_STORAGE_KEY,
        JSON.stringify(cleaned)
      )
    }
  } catch {
    // localStorage may be unavailable (private mode); keys then stay in memory.
  }
}
