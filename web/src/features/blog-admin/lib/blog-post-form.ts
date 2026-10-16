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
import { INTERFACE_LANGUAGE_OPTIONS } from '@/i18n/languages'

/** Locale codes supported by blog localized fields. */
export const BLOG_LOCALE_CODES = INTERFACE_LANGUAGE_OPTIONS.map((o) => o.code)

/** Per-locale values for one post field. */
export type BlogLocalizedMap = { [locale: string]: string }

/** Per-language blog post form state. */
export type BlogPostForm = {
  title: string
  summary: string
  content: string
  cover_image: string
  tags: string
  seo_description: string
  published: boolean
  /** Per-locale titles, keyed by BLOG_LOCALE_CODES. */
  titles: BlogLocalizedMap
  summaries: BlogLocalizedMap
  contents: BlogLocalizedMap
  tags_list: BlogLocalizedMap
  seo_descriptions: BlogLocalizedMap
}

export const emptyBlogPostForm = (): BlogPostForm => ({
  title: '',
  summary: '',
  content: '',
  cover_image: '',
  tags: '',
  seo_description: '',
  published: true,
  titles: {},
  summaries: {},
  contents: {},
  tags_list: {},
  seo_descriptions: {},
})

export function emptyBlogLocalized(): BlogLocalizedMap {
  const result: BlogLocalizedMap = {}
  for (const code of BLOG_LOCALE_CODES) {
    result[code] = ''
  }
  return result
}

/** Keep only the supported locale codes (drop junk keys before saving). */
export function sanitizeBlogLocalizedMap(
  map: BlogLocalizedMap | undefined
): BlogLocalizedMap {
  const result: BlogLocalizedMap = {}
  for (const code of BLOG_LOCALE_CODES) {
    const value = map?.[code]
    if (typeof value === 'string' && value.trim()) {
      result[code] = value.trim()
    }
  }
  return result
}

/** True when every localized field for every supported locale.is empty. */
export function hasAnyLocalizedContent(form: BlogPostForm): boolean {
  return [
    ...Object.values(form.titles),
    ...Object.values(form.summaries),
    ...Object.values(form.contents),
    ...Object.values(form.tags_list),
    ...Object.values(form.seo_descriptions),
  ].some((value) => value.trim().length > 0)
}

/** Check if at least one language (or scalar) has non-empty title and content. */
export function hasAnyTitleAndContent(form: BlogPostForm): boolean {
  if (form.title.trim() && form.content.trim()) return true
  for (const code of BLOG_LOCALE_CODES) {
    if ((form.titles[code] ?? '').trim() && (form.contents[code] ?? '').trim()) {
      return true
    }
  }
  return false
}

/** Move raw admin-payload item localizations into the form state. */
export function blogLocalizationsFromPayload(
  localizations: {
    titles?: Record<string, string>
    summaries?: Record<string, string>
    contents?: Record<string, string>
    tags_list?: Record<string, string>
    seo_descriptions?: Record<string, string>
  } | undefined
): Pick<BlogPostForm, 'titles' | 'summaries' | 'contents' | 'tags_list' | 'seo_descriptions'> {
  return {
    titles: sanitizeBlogLocalizedMap(localizations?.titles),
    summaries: sanitizeBlogLocalizedMap(localizations?.summaries),
    contents: sanitizeBlogLocalizedMap(localizations?.contents),
    tags_list: sanitizeBlogLocalizedMap(localizations?.tags_list),
    seo_descriptions: sanitizeBlogLocalizedMap(localizations?.seo_descriptions),
  }
}