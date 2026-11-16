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
    'CRITICAL LANGUAGE RULE: Every language key ("en", "tr", "zh", "zh-TW", "fr", "ru", "ja", "vi") MUST contain text written in that specific NATIVE language. For example, "en" MUST be in English, "tr" in Turkish, "zh" in Simplified Chinese, "fr" in French, "ru" in Russian, "ja" in Japanese, and "vi" in Vietnamese. NEVER repeat the prompt language across all fields if they are different languages!',
    'CRITICAL STYLE, PUNCTUATION & CASING RULE: If the user prompt contains ANY casing or style rule (such as ALL UPPERCASE/tamamen büyük harf, all lowercase/küçük harf, no punctuation/noktalama yok, specific tone, or structure), you MUST strictly and aggressively apply that constraint across ALL languages and all keys! For example, if all uppercase is requested, every single character in title, summary, tags, and seo_description must be UPPERCASE in all languages.',
  ]

  const langs = BLOG_LOCALE_CODES

  if (step === 'titles') {
    return [
      ...base,
      'Generate ONLY titles for all languages based on the prompt.',
      'The JSON must have this structure:',
      JSON.stringify({ title: langs.reduce((acc, code) => ({ ...acc, [code]: '...' }), {}) }, null, 2),
      'Requirements: max ~70 chars, catchy. Strictly apply requested casing/punctuation to all languages.',
      BLOG_AI_LANGUAGE_PROMPT,
    ].join('\n')
  }

  if (step === 'summaries') {
    return [
      ...base,
      'Generate ONLY summaries for all languages based on the title and prompt.',
      'The JSON must have this structure:',
      JSON.stringify({ summary: langs.reduce((acc, code) => ({ ...acc, [code]: '...' }), {}) }, null, 2),
      'Requirements: 1-2 sentences teaser shown in the blog list. Strictly apply requested casing/punctuation (e.g. ALL UPPERCASE) to every character in every summary!',
      BLOG_AI_LANGUAGE_PROMPT,
    ].join('\n')
  }

  if (step === 'tags') {
    return [
      ...base,
      'Generate ONLY tags for all languages based on the title, summary, and prompt.',
      'The JSON must have this structure:',
      JSON.stringify({ tags: langs.reduce((acc, code) => ({ ...acc, [code]: '...' }), {}) }, null, 2),
      'Requirements: comma-separated, 3-5 tags in each language. Strictly apply requested casing/punctuation.',
      BLOG_AI_LANGUAGE_PROMPT,
    ].join('\n')
  }

  if (step === 'seo') {
    return [
      ...base,
      'Generate ONLY SEO descriptions for all languages.',
      'The JSON must have this structure:',
      JSON.stringify({ seo_description: langs.reduce((acc, code) => ({ ...acc, [code]: '...' }), {}) }, null, 2),
      'Requirements: meta description for search engines, 1-2 sentences in each language, include relevant keywords, no quotes. Strictly apply requested casing/punctuation.',
      BLOG_AI_LANGUAGE_PROMPT,
    ].join('\n')
  }

  if (step === 'content_en') {
    return [
      'You are an expert content writer for a tech product company (an AI API gateway/platform).',
      'Output ONLY the raw Markdown article directly.',
      'Do NOT output JSON. Do NOT wrap the entire article in code fences (```markdown). Do NOT include conversational commentary, thinking tags, prefaces, or postfaces.',
      'Start immediately with the article content.',
      'CRITICAL LANGUAGE RULE: You MUST write this entire article in ENGLISH! Regardless of what language the user prompt or title is provided in (even if Turkish, German, Chinese, etc.), this article MUST be written in ENGLISH. It acts as the universal master source for translations.',
      'Requirements: full comprehensive markdown article (headings, lists, bold, links as appropriate): Aim for 600-1200 words.',
      'HUMANIZER GUIDELINES (MANDATORY - AVOID AI TELLS):',
      '- Write in a direct, natural human voice like a seasoned practitioner, NOT like a generic chatbot.',
      '- NO "Not X, but Y" or "It\'s not just about X, it\'s about Y" formulas. State facts and observations directly.',
      '- NO dramatic one-line closers (e.g. "That is the real win", "Let that sink in", "The future is now").',
      '- NO staged clichés or empty openers (e.g. "In today\'s fast-paced digital world", "In the rapidly evolving landscape of AI", "Let\'s dive in", "Here\'s the thing").',
      '- NO forced triads or arbitrary rule-of-three groupings.',
      '- NO dash spam (excessive em-dashes —).',
      '- Do NOT bold every item or label mechanically.',
      '- Vary sentence lengths naturally. Use concrete technical specifics instead of vague marketing hype.',
      'CRITICAL CASING & STYLE RULE: If the user prompt specifies casing rules (such as ALL UPPERCASE/tamamen büyük harf, all lowercase) or punctuation rules (no punctuation), you MUST strictly follow them for every character in the generated article from start to finish. Never truncate and never use ellipses (...) to skip content.',
      'Citation policy (this is mandatory): if web research notes are present in the user message, you may cite, and may only cite, the URLs that appear in those notes. If no web research notes are present, do NOT invent, guess or output any source, reference, link, URL, domain, or citation of any kind - the article must read as if it cites nothing.',
    ].join('\n')
  }

  if (step === 'content_translate') {
    return [
      'You are an expert technical translator and writer.',
      'Translate the given English markdown content into the specified target language.',
      'Output ONLY the translated Markdown article directly.',
      'Do NOT output JSON. Do NOT wrap the entire article in code fences (```markdown). Do NOT include conversational commentary, thinking tags, prefaces, or postfaces.',
      'Start immediately with the translated content. Maintain all markdown formatting, headings, code snippets, lists, and links accurately.',
      'HUMANIZER & AUTHENTIC TONE: Ensure the translation reads like a fluent native writer in the target language, avoiding robotic syntactic calques and AI clichés.',
      'CRITICAL RULES:',
      '1. You MUST translate into the exact target language requested. Never leave the output in English, Turkish, or another language unless that is the target language!',
      '2. You MUST output the FULL complete article from start to finish. NEVER truncate, summarize, or stop early, and NEVER use ellipses (...) to skip text!',
      '3. If the user provided casing or formatting instructions (e.g. ALL UPPERCASE, all lowercase, no punctuation), you MUST strictly enforce it on the ENTIRE translated article from the first character to the last.',
    ].join('\n')
  }

  return ''
}

export function applyUserCasingGuarantees(text: string, prompt: string, locale?: string): string {
  if (!text || !prompt) return text
  const p = prompt.toLowerCase()
  const isAllUpper =
    p.includes('büyük harf') ||
    p.includes('büyük yap') ||
    p.includes('uppercase') ||
    p.includes('all caps') ||
    p.includes('tamamen büyük')
  const isAllLower =
    p.includes('küçük harf') ||
    p.includes('küçük yap') ||
    p.includes('lowercase') ||
    p.includes('all lower') ||
    p.includes('tamamen küçük')

  if (isAllUpper) {
    try {
      return text.toLocaleUpperCase(locale || 'en')
    } catch {
      return text.toUpperCase()
    }
  }
  if (isAllLower) {
    try {
      return text.toLocaleLowerCase(locale || 'en')
    } catch {
      return text.toLowerCase()
    }
  }
  return text
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
    const hasAnyField =
      obj.title !== undefined ||
      obj.summary !== undefined ||
      obj.content !== undefined ||
      obj.tags !== undefined ||
      obj.seo_description !== undefined

    if (!hasAnyField) {
      return null
    }

    return {
      titles: obj.title ? sanitizeLocaleValues(obj.title) : {},
      summaries: obj.summary ? sanitizeLocaleValues(obj.summary) : {},
      contents: obj.content ? sanitizeLocaleValues(obj.content) : {},
      tags_list: obj.tags ? sanitizeLocaleValues(obj.tags) : {},
      seo_descriptions: obj.seo_description ? sanitizeLocaleValues(obj.seo_description) : {},
    }
  } catch (err) {
    console.error("AI Blog parsing error:", err, rawText)
    return null
  }
}
