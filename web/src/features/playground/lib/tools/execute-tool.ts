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
import { generateImages } from '../../api'
import { ERROR_MESSAGES } from '../../constants'
import type {
  MessageAttachment,
  ModelOption,
  PlanStep,
  PlaygroundConfig,
  PlaygroundToolId,
} from '../../types'
import { toGeneratedImageAttachments } from '../message/image-message-utils'
import { isImageCapableModel, pickImageModel } from '../model-capabilities'
import { parseRequestErrorDetails } from '../streaming/request-error-utils'
import { normalizePlanSteps } from './plan-utils'
import {
  getStringArg,
  parseToolArguments,
} from './tool-call-utils'

export interface ToolExecutionOutcome {
  /** JSON payload sent back to the model as the tool message content. */
  content: string
  /** Short human-readable summary shown on the tool event badge. */
  summary?: string
  /** Latest plan snapshot rendered as a read-only table. */
  plan?: PlanStep[]
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

function executeUpdatePlan(
  args: Record<string, unknown>
): ToolExecutionOutcome {
  const steps = normalizePlanSteps(args.steps)
  if (steps.length === 0) {
    throw new Error(ERROR_MESSAGES.PLAN_STEPS_REQUIRED)
  }

  return {
    content: JSON.stringify({
      steps: steps.map((step) => ({ title: step.title, status: step.status })),
    }),
    summary: `${steps.filter((step) => step.status === 'completed').length}/${steps.length}`,
    plan: steps,
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
    case 'update_plan':
      return executeUpdatePlan(args)
    case 'think':
      return executeThink(args)
  }
}
