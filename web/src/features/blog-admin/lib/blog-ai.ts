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

import { BLOG_LOCALE_CODES, type BlogPostForm } from './blog-post-form'

const BLOG_AI_LANGUAGE_PROMPT = INTERFACE_LANGUAGE_OPTIONS.map(
  (lang) => `  "${lang.code}": "${lang.label}" (language: ${lang.code})`
).join('\n')

/**
 * Build the system prompt that instructs the model to produce a complete,
 * per-language blog post matching the site's supported locales.
 */
export function buildBlogAiSystemPrompt(): string {
  return [
    'You are an expert multilingual content writer. Write a complete blog post for a tech product company (an AI API gateway/platform).',
    'Respond ONLY with a single valid JSON object - no markdown fences, no commentary, no trailing commas.',
    'The JSON must have exactly these keys:',
    JSON.stringify(
      {
        title: { en: '...', zhCN: '...', zhTW: '...', fr: '...', ja: '...', ru: '...', vi: '...' },
        summary: { en: '...', zhCN: '...', zhTW: '...', fr: '...', ja: '...', ru: '...', vi: '...' },
        content: { en: '...', zhCN: '...', zhTW: '...', fr: '...', ja: '...', ru: '...', vi: '...' },
        tags: { en: 'news, update', zhCN: '...', zhTW: '...', fr: '...', ja: '...', ru: '...', vi: '...' },
        seo_description: { en: '...', zhCN: '...', zhTW: '...', fr: '...', ja: '...', ru: '...', vi: '...' },
      },
      null,
      2
    ),
    'For each of the following locales, provide the value in that language (the en one is English, all other fields must be fluent translations of the English content, NOT machine-gun transliterations).',
    BLOG_AI_LANGUAGE_PROMPT,
    'Requirements:',
    '- title: max ~70 chars, catchy.',
    '- summary: 1-2 sentences teaser shown in the blog list.',
    '- content: full markdown article (headings, lists, bold, links as appropriate): Aim for 600-1200 words. Use "en" for English.',
    '- tags: comma-separated, 3-5 tags.',
    '- seo_description: meta description for search engines, 1-2 sentences, include relevant keywords, no quotes.',
    '',
  ].join('\n')
}

/**
 * Parse the model's response text (which may be wrapped in code fences)
 * into a per-language blog post draft. Returns null when the output is
 * unusable to avoid clobbering the admin's existing draft.on bad input.
 */
function sanitizeLocaleValues(
  raw: unknown
): Record<string, string> {
  const result: Record<string, string> = {}

  if (typeof raw === 'object' && raw !== null && !Array.isArray(raw)) {
    for (const code of BLOG_LOCALE_CODES) {
      const value = (raw as Record<string, unknown>)[code]
      if (typeof value === 'string') {
        const normalized = value.trim()
        if (normalized) result[code] = normalized
      }
    }
  }

  // Some models flatten the object into a plain string lane (single-language).
  if (typeof raw === 'string' && raw.trim()) {
    result.en = raw.trim()
  }

  return result
}

export function parseBlogAiResponse(
  rawText: string
): Pick<
  BlogPostForm,
  'titles' | 'summaries' | 'contents' | 'tags_list' | 'seo_descriptions'
> | null {
  try {
    let text = rawText.trim()
    const fenceMatch = text.match(/```(?:json)?\s*([\s\S]*?)```/)
    if (fenceMatch) text = fenceMatch[1].trim()
    const parsed: unknown = JSON.parse(text)

    if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
      return null
    }
    const obj = parsed as Record<string, unknown>
    if (typeof obj.title !== 'string' && typeof obj.title !== 'object') {
      return null
    }

    return {
      titles: sanitizeLocaleValues(obj.title),
      summaries: sanitizeLocaleValues(obj.summary),
      contents: sanitizeLocaleValues(obj.content),
      tags_list: sanitizeLocaleValues(obj.tags),
      seo_descriptions: sanitizeLocaleValues(obj.seo_description),
    }
  } catch {
    return null
  }
}