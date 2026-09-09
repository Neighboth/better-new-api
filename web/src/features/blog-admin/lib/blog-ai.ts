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

export function buildBlogAiSystemPrompt(
  step: 'titles' | 'summaries' | 'tags' | 'seo' | 'content_en' | 'content_translate'
): string {
  const base = [
    'You are an expert multilingual content writer for a tech product company (an AI API gateway/platform).',
    'Respond ONLY with a single valid JSON object. Do not include markdown fences, do not output any reasoning/thinking logic, do not include any commentary.',
    'Your output MUST start exactly with { and end with }.',
  ]

  const langs = BLOG_LOCALE_CODES

  if (step === 'titles') {
    return [
      ...base,
      'Generate ONLY titles for all languages based on the prompt.',
      'The JSON must have this structure:',
      JSON.stringify({ title: langs.reduce((acc, code) => ({ ...acc, [code]: '...' }), {}) }, null, 2),
      'Requirements: max ~70 chars, catchy.',
      BLOG_AI_LANGUAGE_PROMPT,
    ].join('\n')
  }

  if (step === 'summaries') {
    return [
      ...base,
      'Generate ONLY summaries for all languages based on the title and prompt.',
      'The JSON must have this structure:',
      JSON.stringify({ summary: langs.reduce((acc, code) => ({ ...acc, [code]: '...' }), {}) }, null, 2),
      'Requirements: 1-2 sentences teaser shown in the blog list.',
      BLOG_AI_LANGUAGE_PROMPT,
    ].join('\n')
  }

  if (step === 'tags') {
    return [
      ...base,
      'Generate ONLY tags for all languages based on the title, summary, and prompt.',
      'The JSON must have this structure:',
      JSON.stringify({ tags: langs.reduce((acc, code) => ({ ...acc, [code]: '...' }), {}) }, null, 2),
      'Requirements: comma-separated, 3-5 tags.',
      BLOG_AI_LANGUAGE_PROMPT,
    ].join('\n')
  }

  if (step === 'seo') {
    return [
      ...base,
      'Generate ONLY SEO descriptions for all languages.',
      'The JSON must have this structure:',
      JSON.stringify({ seo_description: langs.reduce((acc, code) => ({ ...acc, [code]: '...' }), {}) }, null, 2),
      'Requirements: meta description for search engines, 1-2 sentences, include relevant keywords, no quotes.',
      BLOG_AI_LANGUAGE_PROMPT,
    ].join('\n')
  }

  if (step === 'content_en') {
    return [
      ...base,
      'Generate ONLY the English content.',
      'The JSON must have this structure:',
      JSON.stringify({ content: { en: '...' } }, null, 2),
      'Requirements: full markdown article (headings, lists, bold, links as appropriate): Aim for 600-1200 words.',
    ].join('\n')
  }

  if (step === 'content_translate') {
    return [
      ...base,
      'Translate the given English content into the specified target language.',
      'Return ONLY the translated content in the specified language key.',
      'The JSON must have this structure (where <lang> is the target language code):',
      JSON.stringify({ content: { '<lang>': '...' } }, null, 2),
      'Requirements: fluid translation, maintain all markdown formatting.',
    ].join('\n')
  }

  return ''
}

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

    text = text.replace(/<think>[\s\S]*?<\/think>/g, '').trim()

    const firstBrace = text.indexOf('{')
    const lastBrace = text.lastIndexOf('}')
    if (firstBrace !== -1 && lastBrace !== -1 && lastBrace > firstBrace) {
      text = text.slice(firstBrace, lastBrace + 1)
    }

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
  } catch (err) {
    console.error("AI Blog parsing error:", err, rawText)
    return null
  }
}
