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

/**
 * Translates the i18next interface code (`zhCN`, `zhTW`, ...) into the
 * candidate keys that localized content (announcement bars, model texts, ad
 * images) may be stored under.
 *
 * Different admin editors historically used different spellings
 * (`zh`, `zh-CN`, `zh_CN`, `zh-TW`, `zh_TW`, ...). Without this mapping a
 * Chinese visitor only ever saw the English fallback because `zhCN` matched
 * none of the stored keys.
 */
export function contentLanguageCandidates(language?: string | null): string[] {
  const raw = (language || 'en').trim()
  const lower = raw.replaceAll('_', '-').toLowerCase()
  const short = lower.split('-')[0]
  const candidates = new Set<string>()

  const add = (...values: string[]) => values.forEach((v) => v && candidates.add(v))

  add(raw, lower, short)

  if (short === 'zh') {
    if (lower.includes('tw') || lower.includes('hk') || lower.includes('mo') || lower.includes('hant')) {
      add('zhTW', 'zh-TW', 'zh_TW', 'zh-tw')
    } else {
      add('zhCN', 'zh-CN', 'zh_CN', 'zh-cn')
    }
  }

  add('en')
  add('tr')
  return [...candidates]
}

/**
 * Picks the first non-empty value of a localized record for the given
 * interface language, falling back to English and then to the first non-empty
 * entry.
 */
export function pickLocalizedValue(
  record: Record<string, string | undefined> | undefined | null,
  language?: string | null
): string {
  if (!record) return ''
  for (const key of contentLanguageCandidates(language)) {
    const value = record[key]
    if (typeof value === 'string' && value.trim()) return value
  }
  for (const value of Object.values(record)) {
    if (typeof value === 'string' && value.trim()) return value
  }
  return ''
}
