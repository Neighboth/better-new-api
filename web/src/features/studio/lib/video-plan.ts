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
 * The Studio prompt optimizer for single-shot video generation.
 *
 * The prompt optimizer (a vision-capable chat model, mandatory) rewrites the
 * user's brief into an exhaustive, physics-consistent English scene
 * description before the job is submitted. When Studio web search is
 * enabled, the optimizer may look up unknown subjects on the web and in the
 * image index and attach the found images as reference images automatically.
 */

import { api } from '@/lib/api'
import { getPlaygroundRuntimeConfig } from '@/features/playground/api'

// Appended to every optimizer call, after the configured system prompt, so the
// rewrite can never silently drop the speech the user asked for.
const NON_NEGOTIABLE_RULES = `
NON-NEGOTIABLE RULES (these take precedence over anything above):
- If the input text contains spoken dialogue, quotations or speech lines, you MUST keep them verbatim: never translate, paraphrase, summarize, shorten or omit them. Preserve the exact original wording, language and punctuation of every quoted line.
- Output ONLY the rewritten text, with no commentary, labels, quotation marks around the result or markdown formatting.`

const DEFAULT_VIDEO_OPTIMIZER_SYSTEM_PROMPT =
  'You are a professional video generation prompt enhancer. Rewrite the given prompt into an EXHAUSTIVELY detailed, cinematic English scene description that video generation models render reliably. Rules, in order of importance:\n' +
  '1. NEVER reinterpret, rename, translate away or drop a user-supplied brand, product, company or proper noun. "Tofaş" is a Turkish automobile brand, never a junction/roundabout; "Audi", "Mercedes", "Verity" and similar are real products — keep them verbatim and design the scene around the object the user meant. If a term is genuinely ambiguous, keep the user\'s exact term and the surrounding context unambiguous.\n' +
  '2. Be EXTREMELY specific and exhaustive: name the exact subject identities, colors, materials, shapes and textures; the exact environment (road surface, sky, foliage, architecture); lighting direction, hardness and color; time of day and weather; camera position, lens, height, angle and movement pattern; subject pose, micro-expressions, micro-movements, and how the shot resolves. Every element the user implied must become an explicit concrete visual fact. Ambiguity is forbidden.\n' +
  '3. Single continuous shot language: no jump cuts, no timestamp/HUD/counting digits, no overlays, no on-screen clock or text, no "prophecy" frames. Describe only what a physical camera would see in one unbroken take.\n' +
  '4. Physics consistency: real momentum, real masses, real gravity. Nothing appears from nowhere, nothing dematerializes; collisions have cause and effect (crumple, skid, debris, secondary motion) and other characters react within the same frame.\n' +
  '5. Composition discipline: one coherent camera position/grammar per shot, subjects fully explained in frame, background elements reacting coherently, no teleporting props or sudden unexplained changes.\n' +
  '6. Negative prompt discipline: the negative_prompt must never contradict the main prompt (e.g. main prompt says fixed/static camera — never put "camera shake/movement/handheld" in negative; main says rain — never "no rain"). Merge user negatives carefully and drop anything that fights the positive description.\n' +
  '7. If the prompt contains any dialogue, spoken quotes, or speech lines, DO NOT translate the spoken dialogue — keep all spoken words/quotes exactly as written, in their original language.\n' +
  '8. Output ONLY the optimized prompt text without any introductory text, explanation, or quotes around the prompt.'

export type VideoPlanSource = {
  prompt: string
  userReferenceImages?: string[]
  /** Optional first/last frame anchors the scene must open/close on. */
  startFrame?: string
  endFrame?: string
}

async function resolveOptimizerConfig(signal?: AbortSignal): Promise<{ model: string; systemPrompt: string }> {
  const runtimeConfig = await getPlaygroundRuntimeConfig(signal)
  const model = runtimeConfig?.studio?.prompt_optimizer_model?.trim()
  if (!model) {
    throw new Error(
      'Prompt optimizer is not configured. Open System Settings → Playground & Studio → Studio Prompt Optimizer and set a vision-capable model.'
    )
  }
  const systemPrompt =
    runtimeConfig?.studio?.prompt_optimizer_system_prompt?.trim() || DEFAULT_VIDEO_OPTIMIZER_SYSTEM_PROMPT
  return { model, systemPrompt }
}

type OptimizerChatMessage = {
  role: 'system' | 'user' | 'assistant'
  content: string | Array<Record<string, unknown>>
}

async function callOptimizerOnce(
  model: string,
  systemPrompt: string,
  messages: OptimizerChatMessage[],
  signal?: AbortSignal
): Promise<string> {
  const response = await api.post(
    '/pg/chat/completions',
    {
      model,
      temperature: 0.4,
      messages: [{ role: 'system', content: `${systemPrompt}\n\n${NON_NEGOTIABLE_RULES}` }, ...messages],
    },
    { signal, skipErrorHandler: true, skipAuthRefresh: true } as Record<string, unknown>
  )
  const content = response?.data?.choices?.[0]?.message?.content
  if (typeof content !== 'string' || !content.trim()) {
    throw new Error('The optimizer returned an empty response')
  }
  return content
}

async function callOptimizer(
  model: string,
  systemPrompt: string,
  messages: OptimizerChatMessage[],
  signal?: AbortSignal
): Promise<string> {
  let lastError: unknown = null
  for (let attempt = 0; attempt < 2; attempt++) {
    try {
      return await callOptimizerOnce(model, systemPrompt, messages, signal)
    } catch (err) {
      if (signal?.aborted) throw err
      lastError = err
    }
  }
  throw lastError instanceof Error ? lastError : new Error('The optimizer request failed')
}

export type LabeledImage = { url: string; label: string }

const STUDIO_SEARCH_TOOLS = `You may call two tools that are available ONLY because the administrator enabled Studio web/image search for this installation:
- web_search(query: string): searches the public web. Use it when you do not know a brand, character, product, place or object mentioned in the brief (e.g. "Verity", a regional car model).
- image_search(query: string): searches the public image index. The returned image URLs are attached to the video job as reference images automatically.
How to call a tool: reply with ONLY the JSON object, nothing else: {"tool":"web_search","arguments":{"query":"your query"}} or {"tool":"image_search","arguments":{"query":"your query"}}.
When you have enough information, reply with the final answer TEXT only — no JSON, no explanation, no preamble.
Describe briefly inside the final text which references were added and what they show, e.g. "Reference 1: the exact yellow Verity ball design" — so the video model knows which reference is which.`

export type SearchOptimizedPrompt = { text: string; references: string[] }

async function areSearchToolsEnabled(signal?: AbortSignal): Promise<boolean> {
  const runtime = await getPlaygroundRuntimeConfig(signal)
  const flag = (runtime?.web_search as Record<string, unknown> | undefined)?.studio
  return Boolean(flag && runtime?.web_search?.configured)
}

function parseToolCall(raw: string): { tool: string; arguments: { query?: string } } | null {
  const trimmed = (raw ?? '').trim()
  if (!trimmed) return null
  let candidate = trimmed
  const fence = trimmed.match(/```(?:json)?\s*([\s\S]*?)```/i)
  if (fence?.[1]) candidate = fence[1].trim()
  try {
    const parsed = JSON.parse(candidate) as Record<string, unknown>
    if (typeof parsed?.tool !== 'string') return null
    const args = (parsed.arguments ?? {}) as { query?: unknown }
    return {
      tool: parsed.tool.toLowerCase(),
      arguments: { query: typeof args?.query === 'string' ? args.query : undefined },
    }
  } catch {
    return null
  }
}

// Runs one web/image search query through the same session-authenticated
// endpoint the playground uses — billing handled by the server toggle.
async function runSearch(query: string, category: 'general' | 'images', signal?: AbortSignal) {
  const res = await api.post(
    '/pg/web-search',
    { query, max_results: 5, category },
    { signal, skipErrorHandler: true, skipAuthRefresh: true } as Record<string, unknown>
  )
  const data = res?.data as Record<string, unknown> | undefined
  const payload = (data?.data ?? data) as Record<string, unknown> | undefined
  const results = Array.isArray(payload?.results) ? (payload.results as Array<Record<string, unknown>>) : []
  return results.slice(0, 5).map((hit) => ({
    title: String(hit?.title ?? ''),
    url: String(hit?.url ?? ''),
    content: String(hit?.content ?? ''),
  }))
}

/**
 * Shared optimizer entry. When the Studio search toggle is on, the assistant
 * may call web_search / image_search (which the installation bills as searches)
 * and image results flow back as reference images for the video job.
 */
async function optimizerWithTools(
  model: string,
  systemPrompt: string,
  text: string,
  images: LabeledImage[] | undefined,
  signal?: AbortSignal
): Promise<{ answer: string; references: string[] }> {
  const toolsEnabled = await areSearchToolsEnabled(signal)
  if (!toolsEnabled) {
    return {
      answer: await callOptimizerSmart(model, systemPrompt, text, images, signal),
      references: [],
    }
  }

  const messages: OptimizerChatMessage[] = []
  if (images && images.length > 0) {
    messages.push({
      role: 'user',
      content: [{ type: 'text', text }, ...labeledImageParts(images)],
    })
  } else {
    messages.push({ role: 'user', content: text })
  }

  const references: string[] = []
  const systemWithTools = `${systemPrompt}\n\n${STUDIO_SEARCH_TOOLS}`
  let answer = ''
  let withImages = Boolean(images && images.length > 0)
  for (let round = 0; round < 4; round += 1) {
    let raw: string
    try {
      raw = await callOptimizer(model, systemWithTools, messages, signal)
    } catch (err) {
      if (signal?.aborted) throw err
      // The configured optimizer may not accept image parts. The frames still
      // reach the video model as reference images, so degrade to a text-only
      // conversation instead of failing the whole generation.
      if (withImages && round === 0) {
        withImages = false
        messages.length = 0
        messages.push({ role: 'user', content: text })
        continue
      }
      throw err
    }
    const toolCall = parseToolCall(raw)
    if (!toolCall) {
      answer = raw
      break
    }
    if (toolCall.tool === 'web_search') {
      const query = toolCall.arguments.query?.trim() || text.slice(0, 120)
      const hits = await runSearch(query, 'general', signal).catch(() => [])
      messages.push(
        { role: 'assistant', content: raw },
        { role: 'user', content: `web_search results for "${query}":\n${JSON.stringify(hits)}` }
      )
      continue
    }
    if (toolCall.tool === 'image_search') {
      const query = toolCall.arguments.query?.trim() || text.slice(0, 120)
      const hits = await runSearch(query, 'images', signal).catch(() => [])
      for (const hit of hits) {
        if (hit.url && !references.includes(hit.url) && references.length < 4) {
          references.push(hit.url)
        }
      }
      messages.push(
        { role: 'assistant', content: raw },
        { role: 'user', content: `image_search results for "${query}" (these images become reference images):\n${JSON.stringify(hits)}` }
      )
      continue
    }
    answer = raw
    break
  }
  if (!answer.trim()) {
    return {
      answer: await callOptimizerSmart(model, systemPrompt, text, images, signal),
      references,
    }
  }
  return { answer, references }
}

/**
 * Interleaves text labels before every image part so the optimizer knows
 * exactly which attachment plays which role (reference subject, first frame,
 * finish frame). A vision model maps images to labels in order; raw
 * unlabeled image_url parts force the model to guess — which is exactly what
 * produced off-topic optimized prompts.
 */
function labeledImageParts(items: LabeledImage[]): Array<Record<string, unknown>> {
  const parts: Array<Record<string, unknown>> = []
  for (const item of items) {
    parts.push({ type: 'text', text: item.label })
    parts.push({ type: 'image_url', image_url: { url: item.url, detail: 'high' } })
  }
  return parts
}

function buildReferenceList(
  refs: string[],
  startFrame?: string,
  endFrame?: string
): LabeledImage[] {
  // The optimizer's vision model accepts at most five images per request —
  // pick the highest-value ones deterministically: user refs first, then the
  // first/finish frame anchors.
  const refBudget = Math.max(0, 5 - (startFrame?.trim() ? 1 : 0) - (endFrame?.trim() ? 1 : 0))
  const pickedRefs = refs.slice(0, refBudget)
  const out: LabeledImage[] = []
  pickedRefs.forEach((url, index) => {
    out.push({
      url,
      label: `USER REFERENCE IMAGE ${index + 1}/${pickedRefs.length} — attached by the user; match this subject's appearance, colors, wardrobe, brand/livery and overall look exactly.`,
    })
  })
  if (startFrame?.trim()) {
    out.push({
      url: startFrame,
      label: 'FIRST FRAME — the video must open exactly from this image: composition, subject pose, environment, lighting and framing must continue from it.',
    })
  }
  if (endFrame?.trim()) {
    out.push({
      url: endFrame,
      label: 'FINISH FRAME — the video must end exactly on this image: composition, subject pose, environment, lighting and framing must converge onto it.',
    })
  }
  return out
}

export function gatherVisionImages(source: VideoPlanSource): LabeledImage[] {
  const refs = source.userReferenceImages?.filter((url) => typeof url === 'string' && url.trim()) ?? []
  return buildReferenceList(refs, source.startFrame, source.endFrame)
}

async function callOptimizerSmart(
  model: string,
  systemPrompt: string,
  text: string,
  images: LabeledImage[] | undefined,
  signal?: AbortSignal
): Promise<string> {
  const withImages = (images?.length ?? 0) > 0
  const buildMessages = (includeImages: boolean): OptimizerChatMessage[] => {
    if (includeImages && withImages) {
      return [{ role: 'user', content: [{ type: 'text', text }, ...labeledImageParts(images ?? [])] }]
    }
    return [{ role: 'user', content: text }]
  }
  if (!withImages) {
    return callOptimizer(model, systemPrompt, buildMessages(false), signal)
  }
  try {
    return await callOptimizer(model, systemPrompt, buildMessages(true), signal)
  } catch (err) {
    if (signal?.aborted) throw err
    // The configured optimizer model may not accept image parts. Frames still
    // reach the video model as reference images, so degrade to a text-only
    // optimizer call rather than failing the entire generation.
    return callOptimizer(model, systemPrompt, buildMessages(false), signal)
  }
}

export async function optimizeVideoPrompt(
  prompt: string,
  signal?: AbortSignal,
  images?: LabeledImage[]
): Promise<SearchOptimizedPrompt> {
  const trimmed = prompt.trim()
  if (!trimmed) return { text: prompt, references: [] }
  const { model, systemPrompt } = await resolveOptimizerConfig(signal)
  const { answer, references } = await optimizerWithTools(model, systemPrompt, trimmed, images, signal)
  const cleaned = answer.replaceAll(/^["'`]+|["'`]+$/g, '').trim()
  if (!cleaned) throw new Error('The optimizer returned an empty prompt')
  return { text: cleaned, references }
}

export async function optimizeImagePrompt(
  prompt: string,
  signal?: AbortSignal,
  images?: LabeledImage[]
): Promise<string> {
  const trimmed = prompt.trim()
  if (!trimmed) return prompt
  const runtimeConfig = await getPlaygroundRuntimeConfig(signal)
  const model = runtimeConfig?.studio?.prompt_optimizer_model?.trim()
  if (!model) {
    throw new Error(
      'Prompt optimizer is not configured. Open System Settings → Playground & Studio → Studio Prompt Optimizer and set a vision-capable model.'
    )
  }
  const systemPrompt =
    runtimeConfig?.studio?.prompt_optimizer_system_prompt?.trim() ||
    'You are a professional image prompt optimizer. Rewrite the prompt into a descriptive, high-quality English prompt with lighting, composition, visual details, and style that generative AI models accept without safety violations. CRITICAL REQUIREMENT: If the prompt contains any text or quotes intended for rendering, DO NOT translate them — keep all quotes exactly as written, in their original language. Output ONLY the optimized prompt text without any introductory text, explanation, or quotes.'
  const raw = await callOptimizerSmart(
    model,
    systemPrompt,
    trimmed,
    images,
    signal
  )
  const cleaned = raw.replaceAll(/^["'`]+|["'`]+$/g, '').trim()
  if (!cleaned) throw new Error('The optimizer returned an empty prompt')
  return cleaned
}
