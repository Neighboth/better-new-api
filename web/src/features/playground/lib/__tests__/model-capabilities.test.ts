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
*/
import { describe, expect, it } from 'vitest'

import {
  isImageCapableModel,
  pickImageModel,
  supportsNativeThinking,
} from '../model-capabilities'

describe('supportsNativeThinking', () => {
  it('recognizes common reasoning model families', () => {
    expect(supportsNativeThinking('o3-mini')).toBe(true)
    expect(supportsNativeThinking('gpt-5.1')).toBe(true)
    expect(supportsNativeThinking('deepseek-r1-0528')).toBe(true)
    expect(supportsNativeThinking('qwen3-32b')).toBe(true)
    expect(supportsNativeThinking('gemini-2.5-pro')).toBe(true)
    expect(supportsNativeThinking('glm-4.6')).toBe(true)
  })

  it('does not flag plain chat models', () => {
    expect(supportsNativeThinking('gpt-4o')).toBe(false)
    expect(supportsNativeThinking('claude-sonnet-4-5')).toBe(false)
    expect(supportsNativeThinking('gpt-oss-120b')).toBe(false)
    expect(supportsNativeThinking('llama-3.3-70b')).toBe(false)
  })
})

describe('pickImageModel', () => {
  const models = [
    { label: 'gpt-4o', value: 'gpt-4o' },
    { label: 'dall-e-3', value: 'dall-e-3' },
    { label: 'flux.1-schnell', value: 'flux.1-schnell' },
  ]

  it('keeps the current model when it is image-capable', () => {
    expect(pickImageModel(models, 'dall-e-3')).toBe('dall-e-3')
    expect(isImageCapableModel('gpt-image-1')).toBe(true)
  })

  it('falls back to the first image-capable model for chat models', () => {
    expect(pickImageModel(models, 'gpt-4o')).toBe('dall-e-3')
  })

  it('returns null when no image model is available', () => {
    expect(pickImageModel([{ label: 'x', value: 'gpt-4o' }], 'gpt-4o')).toBe(
      null
    )
  })
})
