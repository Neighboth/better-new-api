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
import type { ModelOption } from '../types'

// Name-based heuristics: the user-models endpoint only returns model names,
// so capabilities are inferred from well-known naming patterns.
const NATIVE_THINKING_PATTERN =
  /(^|[-_/.])(o1|o3|o4)([-_.]|$)|gpt-5|deepseek-r1|-r1\b|qwq|qwen3|thinking|reason(er|ing)?|gemini-2\.5|glm-4\.[56]|kimi-k2|hunyuan-t1|ernie-x1|seed-1\.6|doubao-1\.[56]-thinking/i

const IMAGE_MODEL_PATTERN =
  /dall-e|gpt-image|flux|sdxl|stable-diffusion|\bsd[0-9]|imagen|seedream|kolors|hunyuan-image|cogview|wanx|qwen-image|midjourney|\bmj\b|doubao.*image|image/i

export function supportsNativeThinking(modelName: string): boolean {
  return NATIVE_THINKING_PATTERN.test(modelName)
}

export function isImageCapableModel(modelName: string): boolean {
  return IMAGE_MODEL_PATTERN.test(modelName)
}

/**
 * Pick the model used for image generation: the current model when it looks
 * image-capable, otherwise the first image-capable model available.
 */
export function pickImageModel(
  models: ModelOption[],
  currentModel: string
): string | null {
  if (isImageCapableModel(currentModel)) {
    return currentModel
  }

  const candidate = models.find((model) => isImageCapableModel(model.value))
  return candidate?.value ?? null
}
