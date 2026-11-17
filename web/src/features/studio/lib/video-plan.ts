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
 * Multi-part video sequencing for the Studio.
 *
 * Long generations are produced as a chain of clips that are stitched into
 * one continuous video in the UI. Every clip is rendered by a separate job,
 * so the prompt optimizer (a vision-capable chat model, mandatory) plans the
 * whole sequence up front and refines each step against frames captured from
 * the ACTUAL footage of the previous step. That keeps subjects, colors,
 * brands, wardrobe, weather, lighting and camera grammar identical across
 * every cut — the result looks like a single one-shot generation.
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
  'You are a professional video generation prompt optimizer and safety enhancer. Rewrite the given prompt into safe, detailed, cinematic English that generative video AI models accept without content violations. Describe camera movement, lighting, visual details, and style cleanly and vividly. CRITICAL REQUIREMENT: If the prompt contains any dialogue, spoken quotes, or speech lines, DO NOT translate the spoken dialogue — keep all spoken words/quotes exactly as written, in their original language. Output ONLY the optimized prompt text without any introductory text, explanation, or quotes around the prompt.'

export type VideoPlanCharacter = {
  name?: string
  appearance?: string
  wardrobe?: string
  distinctive_features?: string
}

export type VideoPlanStep = {
  step: number
  duration_seconds?: number
  action: string
  camera?: string
  start_state?: string
  end_state?: string
  lighting?: string
}

export type VideoPlan = {
  title: string
  global_style: string
  characters: VideoPlanCharacter[]
  location: { description: string; continuity_notes: string }
  consistency_anchors: string[]
  steps: VideoPlanStep[]
  negative_prompt: string
}

export type VideoPlanSource = {
  prompt: string
  totalSeconds: number
  chunkSeconds: number
  ratio?: string
  quality?: string
  negativePrompt?: string
  userReferenceImages?: string[]
  /** Explicit chain anchors supplied by the caller (optional). */
  startFrame?: string
  endFrame?: string
}

export type RefinedStep = {
  stepPrompt: string
  negativePrompt: string
  plan: VideoPlan
}

function extractJsonObject(raw: string): unknown {
  const trimmed = (raw ?? '').trim()
  if (!trimmed) throw new Error('The optimizer returned an empty response')

  const candidates: string[] = [trimmed]
  const fence = trimmed.match(/```(?:json)?\s*([\s\S]*?)```/i)
  if (fence?.[1]) candidates.push(fence[1].trim())
  const firstBrace = trimmed.indexOf('{')
  const lastBrace = trimmed.lastIndexOf('}')
  if (firstBrace !== -1 && lastBrace > firstBrace) {
    candidates.push(trimmed.slice(firstBrace, lastBrace + 1))
  }

  let lastError: unknown = null
  for (const candidate of candidates) {
    try {
      return JSON.parse(candidate)
    } catch (err) {
      lastError = err
    }
  }
  throw new Error(
    `The optimizer did not return valid JSON: ${
      lastError instanceof Error ? lastError.message : 'parse error'
    }`
  )
}

function asString(value: unknown): string {
  return typeof value === 'string' ? value.trim() : ''
}

function coerceAnchorList(value: unknown): string[] {
  if (!Array.isArray(value)) return []
  const out: string[] = []
  for (const item of value) {
    const text = asString(item)
    if (text) out.push(text)
  }
  return out
}

function coerceCharacters(value: unknown): VideoPlanCharacter[] {
  if (!Array.isArray(value)) return []
  const out: VideoPlanCharacter[] = []
  for (const item of value) {
    if (typeof item === 'string') {
      const text = item.trim()
      if (text) out.push({ appearance: text })
      continue
    }
    if (!item || typeof item !== 'object') continue
    const record = item as Record<string, unknown>
    const name = asString(record.name)
    const appearance = asString(record.appearance)
    const wardrobe = asString(record.wardrobe)
    const distinctive = asString(record.distinctive_features ?? record.distinctiveFeatures)
    if (!name && !appearance && !wardrobe && !distinctive) continue
    out.push({
      name: name || undefined,
      appearance: appearance || undefined,
      wardrobe: wardrobe || undefined,
      distinctive_features: distinctive || undefined,
    })
  }
  return out
}

function coerceSteps(value: unknown, stepCount: number, startNumber = 1): VideoPlanStep[] {
  const steps: VideoPlanStep[] = []
  if (Array.isArray(value)) {
    let index = 0
    for (const item of value) {
      if (!item || typeof item !== 'object') continue
      const record = item as Record<string, unknown>
      index += 1
      const action = asString(record.action) || asString(record.prompt) || asString(record.description)
      if (!action) continue
      steps.push({
        step: Number(record.step) > 0 ? Math.floor(Number(record.step)) : index,
        duration_seconds:
          Number(record.duration_seconds ?? record.durationSeconds) > 0
            ? Math.floor(Number(record.duration_seconds ?? record.durationSeconds))
            : undefined,
        action,
        camera: asString(record.camera) || undefined,
        start_state: asString(record.start_state ?? record.startState) || undefined,
        end_state: asString(record.end_state ?? record.endState) || undefined,
        lighting: asString(record.lighting) || undefined,
      })
    }
  }

  if (steps.length === 0) {
    throw new Error('The optimizer returned a plan without usable steps')
  }

  // Keep exactly the number of steps the generator needs: renumber them and
  // pad/truncate deterministically so the loop always finds a chunk plan.
  const normalized: VideoPlanStep[] = []
  for (let i = 0; i < stepCount; i++) {
    const source = steps[Math.min(i, steps.length - 1)]
    normalized.push({ ...source, step: startNumber + i })
  }
  return normalized
}

export function parseVideoPlan(raw: string, stepCount: number): VideoPlan {
  const parsed = extractJsonObject(raw) as Record<string, unknown>
  if (!parsed || typeof parsed !== 'object') {
    throw new Error('The optimizer returned an invalid plan object')
  }
  const locationRaw = parsed.location
  let location = { description: asString(parsed.global_style), continuity_notes: '' }
  if (typeof locationRaw === 'string' && locationRaw.trim()) {
    location = { description: locationRaw.trim(), continuity_notes: '' }
  } else if (locationRaw && typeof locationRaw === 'object') {
    const record = locationRaw as Record<string, unknown>
    location = {
      description:
        asString(record.description) || asString(parsed.global_style) || location.description,
      continuity_notes: asString(record.continuity_notes ?? record.continuityNotes),
    }
  }

  const globalStyle = asString(parsed.global_style) || asString(parsed.globalStyle) || asString(parsed.style) || ''
  const negative = asString(parsed.negative_prompt) || asString(parsed.negativePrompt) || ''
  const anchors = coerceAnchorList(parsed.consistency_anchors ?? parsed.consistencyAnchors ?? parsed.anchors)

  return {
    title: asString(parsed.title) || 'Untitled video',
    global_style: globalStyle || 'Cinematic, coherent live-action footage with consistent subjects, lighting and color grade.',
    characters: coerceCharacters(parsed.characters),
    location,
    consistency_anchors: anchors,
    steps: coerceSteps(parsed.steps, stepCount),
    negative_prompt: negative,
  }
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
  role: 'system' | 'user'
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

/**
 * Interleaves text labels before every image part so the optimizer knows
 * exactly which attachment plays which role (reference subject, footage
 * from the previous clip, first frame, finish frame). A vision model maps
 * images to labels in order; raw unlabeled image_url parts force the model
 * to guess — which is exactly what produced off-topic optimized prompts.
 */
function labeledImageParts(items: LabeledImage[]): Array<Record<string, unknown>> {
  const parts: Array<Record<string, unknown>> = []
  for (const item of items) {
    parts.push({ type: 'text', text: item.label })
    parts.push({ type: 'image_url', image_url: { url: item.url, detail: 'high' } })
  }
  return parts
}

export function gatherVisionImages(source: VideoPlanSource, frames?: string[]): LabeledImage[] {
  const out: LabeledImage[] = []
  const refs = source.userReferenceImages?.filter((url) => typeof url === 'string' && url.trim()) ?? []
  refs.slice(0, 4).forEach((url, index) => {
    out.push({
      url,
      label: `USER REFERENCE IMAGE ${index + 1}/${refs.length} — attached by the user; match this subject's appearance, colors, wardrobe, brand/livery and overall look exactly.`,
    })
  })
  const captured = frames?.filter((url) => typeof url === 'string' && url.trim()) ?? []
  captured.slice(0, 4).forEach((url, index) => {
    out.push({
      url,
      label: `RENDER FRAME ${index + 1}/${captured.length} — captured from the ACTUAL previous clip; ground truth for subject appearance, wardrobe, colors, lighting, weather and environment. Match these exactly.`,
    })
  })
  if (source.startFrame?.trim()) {
    out.push({
      url: source.startFrame,
      label: 'FIRST FRAME — the video must open exactly from this image: composition, subject pose, environment, lighting and framing must continue from it.',
    })
  }
  if (source.endFrame?.trim()) {
    out.push({
      url: source.endFrame,
      label: 'FINISH FRAME — the video must end exactly on this image: composition, subject pose, environment, lighting and framing must converge onto it.',
    })
  }
  return out
}

function buildPlanningText(source: VideoPlanSource, totalSteps: number): string {
  const hasReferences = (source.userReferenceImages?.length ?? 0) > 0
  const text = [
    `Plan a multi-part video. It will be generated as ${totalSteps} sequential AI video clips of ~${source.chunkSeconds}s each (~${source.totalSeconds}s total) that MUST be stitched into one seamless continuous take.`,
    `USER CREATIVE BRIEF (verbatim, may contain Turkish — keep any dialogue/quotes in Turkish exactly as written):`,
    source.prompt.trim(),
    source.ratio ? `Aspect ratio: ${source.ratio}` : '',
    source.quality ? `Quality/resolution: ${source.quality}` : '',
    source.negativePrompt?.trim()
      ? `User-supplied negative guidance (merge into negative_prompt): ${source.negativePrompt.trim()}`
      : '',
    `Requirements for the plan:`,
    `- Invent ONE global visual bible (characters with exact face/hair/wardrobe/colors, vehicles/aircraft with exact livery colors and brand, location, ground and sky appearance, weather, time of day, lighting, color grade, camera style). It must be usable verbatim in every step.`,
    `- Every step is one ~${source.chunkSeconds}s clip. The story must unfold naturally across steps (e.g. first the plane rolls, then it lifts off, then it flies over, then it lands and brakes to a stop).`,
    `- steps[i].end_state must be the natural start of steps[i+1].start_state. The first frame of every clip continues the last frame of the previous clip.`,
    `- Absolutely no contradictory details between the first and last moment of the whole video (colors, brands, weather, lighting, wardrobe, architecture).`,
    `- consistency_anchors: 8-14 ultra-specific invariants (e.g. "pure white fuselage with one thin red cheatline and tail code PX-9", "low golden-hour sun, long shadows", "dark asphalt with dashed yellow centerline").`,
    `- characters: every person who appears, with exact appearance so the same face can be regenerated in every part.`,
    hasReferences
      ? `- The user attached ${source.userReferenceImages?.length ?? 0} reference image(s) — they show the EXACT subjects/look to use. Anchor the bible to them.`
      : '',
    `- negative_prompt: forbid scene resets, jump cuts, subject/wardrobe/color changes, lighting jumps, different aircraft or brands, style drift, watermark.`,
    `Return ONLY valid JSON with this exact shape:`,
    `{"title":string,"global_style":string,"characters":[{"name":string,"appearance":string,"wardrobe":string,"distinctive_features":string}],"location":{"description":string,"continuity_notes":string},"consistency_anchors":[string],"steps":[{"step":number,"duration_seconds":number,"action":string,"camera":string,"start_state":string,"end_state":string,"lighting":string}],"negative_prompt":string}`,
    `steps must contain EXACTLY ${totalSteps} entries.`,
  ]
    .filter(Boolean)
    .join('\n\n')
  return text
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

export async function requireVideoOptimizer(signal?: AbortSignal): Promise<{
  model: string
  systemPrompt: string
}> {
  return resolveOptimizerConfig(signal)
}

export async function planVideoSequence(
  source: VideoPlanSource,
  totalSteps: number,
  signal?: AbortSignal
): Promise<VideoPlan> {
  const { model, systemPrompt } = await resolveOptimizerConfig(signal)
  const planningSpec =
    'You are the continuity supervisor of a multi-part AI video pipeline. You receive a brief and output ONE JSON object only — no markdown fences, no commentary, no extra keys. The whole video must look like a single one-shot generation by one model.'
  const raw = await callOptimizerSmart(
    model,
    `${systemPrompt}\n\n${planningSpec}`,
    buildPlanningText(source, totalSteps),
    gatherVisionImages(source, undefined),
    signal
  )
  return parseVideoPlan(raw, totalSteps)
}

export async function refinePlanForStep(
  source: VideoPlanSource & { plan: VideoPlan; stepIndex: number; frames: string[] },
  signal?: AbortSignal
): Promise<RefinedStep> {
  const stepNumber = source.stepIndex + 1
  const totalSteps = source.plan.steps.length
  const { model, systemPrompt } = await resolveOptimizerConfig(signal)
  const planningSpec =
    'You are the continuity supervisor of a multi-part AI video pipeline. You receive (1) the original brief, (2) the current plan, (3) frames captured from the ACTUAL previous render. Steps 1..{previous} are FINAL and must not be re-planned. Output ONE JSON object only — no markdown fences, no commentary.'
  const negative = source.negativePrompt?.trim()
  const visionImages = gatherVisionImages(source, source.frames)
  const planJson = JSON.stringify(source.plan)
  const text = [
    `Original brief (verbatim; keep dialogue/quotes exactly as written):`,
    source.prompt.trim(),
    `The plan so far (steps 1..${Math.max(0, source.stepIndex)} are rendered and final):`,
    planJson,
    visionImages.length
      ? `Below are ${visionImages.length} labeled image(s): USER REFERENCE IMAGE(s) attached by the user, RENDER FRAME(s) captured from the ACTUAL previous clip (ground truth for subject appearance, colors, livery, wardrobe, weather, lighting and environment), and — when present — FIRST FRAME / FINISH FRAME the scene must open from / end on. Match them exactly.`
      : 'No images were attached; stay as close to the plan as possible.',
    `Produce the FINAL generation prompt for STEP ${stepNumber} of ${totalSteps}.`,
    `Requirements for step_prompt: 4-8 vivid sentences; opens exactly from the previous part's ending; subject/wardrobe/livery/environment/weather/lighting identical to the plan and to the frames; describe the action, camera movement, lighting, and framing; keep dialogue/quotes in the original language; never reset the scene; camera must not contradict the previous part's grammar; do NOT mention "step", "part", JSON, or instructions.`,
    `Also re-check the remaining steps (${stepNumber + 1}..${totalSteps}) against the actual footage — return them as remaining_steps with the same object shape as the plan's steps (renumber from ${stepNumber + 1}); if the original plan still fits, repeat it with adjustments only where the footage diverges.`,
    negative ? `User's negative guidance to honor (merge into negative_prompt): ${negative}` : '',
    `Return ONLY valid JSON with this exact shape:`,
    `{"step_prompt":string,"negative_prompt":string,"remaining_steps":[{"step":number,"duration_seconds":number,"action":string,"camera":string,"start_state":string,"end_state":string,"lighting":string}]}`,
  ]
    .filter(Boolean)
    .join('\n\n')

  const raw = await callOptimizerSmart(
    model,
    `${systemPrompt}\n\n${planningSpec.replace('{previous}', String(Math.max(0, source.stepIndex)))}`,
    text,
    visionImages,
    signal
  )
  const parsed = extractJsonObject(raw) as Record<string, unknown>
  const stepPrompt = asString(parsed.step_prompt) || asString(parsed.stepPrompt) || asString(parsed.prompt)
  if (!stepPrompt) {
    throw new Error('The optimizer did not return a prompt for this step')
  }
  const negativeOut = asString(parsed.negative_prompt) || asString(parsed.negativePrompt) || source.plan.negative_prompt
  const remaining = parsed.remaining_steps ?? parsed.remainingSteps
  const steps = source.plan.steps.slice(0, stepNumber)
  if (Array.isArray(remaining) && remaining.length) {
    const remainingSteps = coerceSteps(remaining, totalSteps - stepNumber, stepNumber + 1)
    steps.push(...remainingSteps)
  } else {
    steps.push(...source.plan.steps.slice(stepNumber))
  }
  const plan: VideoPlan = { ...source.plan, steps, negative_prompt: negativeOut || source.plan.negative_prompt }
  return { stepPrompt, negativePrompt: negativeOut || source.plan.negative_prompt, plan }
}

export async function optimizeVideoPrompt(
  prompt: string,
  signal?: AbortSignal,
  images?: LabeledImage[]
): Promise<string> {
  const trimmed = prompt.trim()
  if (!trimmed) return prompt
  const { model, systemPrompt } = await resolveOptimizerConfig(signal)
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

export async function optimizeImagePrompt(prompt: string, signal?: AbortSignal): Promise<string> {
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
  const raw = await callOptimizer(
    model,
    systemPrompt,
    [{ role: 'user', content: trimmed }],
    signal
  )
  const cleaned = raw.replaceAll(/^["'`]+|["'`]+$/g, '').trim()
  if (!cleaned) throw new Error('The optimizer returned an empty prompt')
  return cleaned
}
