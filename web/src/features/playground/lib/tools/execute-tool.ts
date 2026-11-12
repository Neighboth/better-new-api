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
import { generateImages, generateVideo, searchWeb } from '../../api'
import { ERROR_MESSAGES } from '../../constants'
import type {
  MessageAttachment,
  ModelOption,
  PlaygroundConfig,
  PlaygroundRuntimeConfig,
  PlaygroundToolId,
} from '../../types'
import { toGeneratedImageAttachments } from '../message/image-message-utils'
import { isImageCapableModel, pickImageModel } from '../model-capabilities'
import { parseRequestErrorDetails } from '../streaming/request-error-utils'
import {
  getNumberArg,
  getStringArg,
  parseToolArguments,
} from './tool-call-utils'
import { pollVideoTask, resolveSubmittedTaskIdWithLookup } from './video-task-utils'

export interface ToolExecutionOutcome {
  /** JSON payload sent back to the model as the tool message content. */
  content: string
  /** Short human-readable summary shown on the tool event badge. */
  summary?: string
  /** Generated images attached to the assistant message. */
  attachments?: MessageAttachment[]
  /** Search hit links surfaced in the message sources block. */
  sources?: { href: string; title: string }[]
  /** Reasoning text recorded by the think tool, shown as a thought block. */
  thought?: string
}

export interface ToolExecutionContext {
  config: PlaygroundConfig
  /** Available chat models, used to fall back to an image-capable one. */
  models?: ModelOption[]
  /** Administrator configuration of the media models and the tools. */
  runtime?: PlaygroundRuntimeConfig
  signal?: AbortSignal
}

function requireArgs(argumentsJson: string): Record<string, unknown> {
  const args = parseToolArguments(argumentsJson)
  if (!args) {
    throw new Error(ERROR_MESSAGES.TOOL_ARGS_INVALID)
  }
  return args
}

async function executeGenerateImage(
  args: Record<string, unknown>,
  ctx: ToolExecutionContext
): Promise<ToolExecutionOutcome> {
  const prompt = getStringArg(args, 'prompt')
  if (!prompt) {
    throw new Error(ERROR_MESSAGES.IMAGE_PROMPT_REQUIRED)
  }

  // The selected chat model often cannot generate images (400/503 from the
  // relay). Fall back to an image-capable model from the user's list.
  const candidates = [ctx.config.model]
  const fallbackModel = pickImageModel(ctx.models ?? [], ctx.config.model)
  if (fallbackModel && fallbackModel !== ctx.config.model) {
    candidates.push(fallbackModel)
  }

  let lastError: unknown = null
  for (const model of candidates) {
    try {
      const response = await generateImages(
        { model, group: ctx.config.group, prompt, n: 1 },
        ctx.signal
      )
      const attachments = toGeneratedImageAttachments(response, prompt)
      if (attachments.length === 0) {
        throw new Error(ERROR_MESSAGES.IMAGE_EMPTY_RESULT)
      }

      return {
        // Base64 payloads stay out of the transcript: the image is already
        // rendered in the chat, the model only needs to know it succeeded.
        content: JSON.stringify({
          note: 'The generated image is displayed to the user in the chat.',
          model,
          images: attachments.map((attachment) =>
            attachment.url.startsWith('data:')
              ? '[inline image]'
              : attachment.url
          ),
        }),
        summary: prompt,
        attachments,
      }
    } catch (error) {
      if (ctx.signal?.aborted) {
        throw error
      }
      lastError = error
    }
  }

  if (!fallbackModel && !isImageCapableModel(ctx.config.model)) {
    throw new Error(ERROR_MESSAGES.IMAGE_MODEL_UNAVAILABLE)
  }

  const { errorMessage } = parseRequestErrorDetails(lastError)
  throw new Error(errorMessage)
}

function executeThink(args: Record<string, unknown>): ToolExecutionOutcome {
  const thought = getStringArg(args, 'thought')
  if (!thought) {
    throw new Error(ERROR_MESSAGES.TOOL_ARGS_INVALID)
  }

  return {
    content: JSON.stringify({ recorded: true }),
    thought,
  }
}

async function executeGenerateVideo(
  args: Record<string, unknown>,
  ctx: ToolExecutionContext
): Promise<ToolExecutionOutcome> {
  const prompt = getStringArg(args, 'prompt')
  if (!prompt) {
    throw new Error(ERROR_MESSAGES.VIDEO_PROMPT_REQUIRED)
  }

  const submittedAt = Date.now()
  const videoConfig = ctx.runtime?.video
  const model = videoConfig?.model?.trim() || ctx.config.model
  const minDuration = videoConfig?.min_duration || 1
  const maxDuration = videoConfig?.max_duration || 60
  const duration = getNumberArg(
    args,
    'duration',
    videoConfig?.default_duration || minDuration,
    minDuration,
    maxDuration
  )
  const ratioArg = getStringArg(args, 'ratio')
  const ratio =
    ratioArg && (!videoConfig?.ratios?.length || videoConfig.ratios.includes(ratioArg))
      ? ratioArg
      : videoConfig?.default_ratio || ''

  const submitted = await generateVideo(
    {
      model,
      group: ctx.config.group,
      prompt,
      duration,
      ratio: ratio || undefined,
      quality: videoConfig?.default_quality || undefined,
    },
    ctx.signal
  )

  const taskId = await resolveSubmittedTaskIdWithLookup(submitted, {
    model,
    submittedAt,
    signal: ctx.signal,
  })
  if (!taskId) {
    throw new Error(ERROR_MESSAGES.VIDEO_TASK_FAILED)
  }

  const finished = await pollVideoTask(taskId, ctx.signal, () =>
    resolveSubmittedTaskIdWithLookup(submitted, {
      model,
      submittedAt,
      signal: ctx.signal,
    })
  )
  if (!finished.result_url) {
    throw new Error(finished.fail_reason || ERROR_MESSAGES.VIDEO_TASK_FAILED)
  }

  const attachments: MessageAttachment[] = [
    {
      url: finished.result_url,
      mediaType: 'video/mp4',
      filename: `${prompt.slice(0, 60) || 'video'}.mp4`,
    },
  ]

  return {
    content: JSON.stringify({
      note: 'The generated video is displayed to the user in the chat.',
      model,
      duration,
      ratio: ratio || null,
      url: finished.result_url,
    }),
    summary: prompt,
    attachments,
  }
}

async function executeWebSearch(
  args: Record<string, unknown>,
  ctx: ToolExecutionContext
): Promise<ToolExecutionOutcome> {
  const query = getStringArg(args, 'query')
  if (!query) {
    throw new Error(ERROR_MESSAGES.SEARCH_QUERY_REQUIRED)
  }
  const maxResults = getNumberArg(args, 'max_results', 5, 1, 10)

  let payload
  try {
    payload = await searchWeb(query, maxResults, ctx.signal)
  } catch (error) {
    if (ctx.signal?.aborted) throw error
    const { errorMessage } = parseRequestErrorDetails(error)
    throw new Error(errorMessage || ERROR_MESSAGES.WEB_SEARCH_FAILED)
  }

  const results = Array.isArray(payload?.results) ? payload.results : []
  if (results.length === 0) {
    return {
      content: JSON.stringify({ query, results: [], note: 'No result found.' }),
      summary: query,
    }
  }

  const lines = results.map(
    (hit, index) =>
      `[${index + 1}] ${hit.title}\n${hit.url}\n${hit.content || ''}`.trim()
  )

  return {
    // The model reads the compact list of hits, the user gets the sources
    // block of the message and can open every link directly.
    content: `Web search results for "${query}":\n\n${lines.join('\n\n')}`,
    summary: query,
    sources: results.map((hit) => ({ href: hit.url, title: hit.title })),
  }
}

/**
 * Execute a playground tool call on the client. Throws an Error whose message
 * is forwarded to the model as the tool error payload.
 */
export async function executePlaygroundTool(
  name: PlaygroundToolId,
  argumentsJson: string,
  ctx: ToolExecutionContext
): Promise<ToolExecutionOutcome> {
  const args = requireArgs(argumentsJson)

  switch (name) {
    case 'generate_image':
      return executeGenerateImage(args, ctx)
    case 'generate_video':
      return executeGenerateVideo(args, ctx)
    case 'web_search':
      return executeWebSearch(args, ctx)
    case 'think':
      return executeThink(args)
  }
}
