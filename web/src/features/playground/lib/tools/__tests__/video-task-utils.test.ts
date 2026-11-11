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
import { describe, expect, it } from 'vitest'

import { buildToolAvailabilityPrompt } from '@/features/playground/lib/tools/tool-definitions'
import { resolveSubmittedTaskId } from '@/features/playground/lib/tools/video-task-utils'

describe('resolveSubmittedTaskId', () => {
  it('reads the id the relay adds to the body', () => {
    expect(resolveSubmittedTaskId({ task_id: 'task_public_1' })).toBe(
      'task_public_1'
    )
  })

  it('reads the id of providers that answer with a plain id', () => {
    expect(resolveSubmittedTaskId({ id: 'vid_123', status: 'queued' })).toBe(
      'vid_123'
    )
  })

  it('reads nested answers', () => {
    expect(resolveSubmittedTaskId({ data: { task_id: 'nested_1' } })).toBe(
      'nested_1'
    )
    expect(resolveSubmittedTaskId({ output: { id: 'nested_2' } })).toBe(
      'nested_2'
    )
  })

  it('accepts numeric ids', () => {
    expect(resolveSubmittedTaskId({ id: 42 })).toBe('42')
  })

  it('returns an empty string when there is no id', () => {
    expect(resolveSubmittedTaskId({ success: true, data: [] })).toBe('')
    expect(resolveSubmittedTaskId(null)).toBe('')
    expect(resolveSubmittedTaskId(undefined)).toBe('')
    expect(resolveSubmittedTaskId('plain text')).toBe('')
  })

  it('does not loop on cyclic objects', () => {
    const cyclic: Record<string, unknown> = { status: 'queued' }
    cyclic.self = cyclic
    expect(resolveSubmittedTaskId(cyclic)).toBe('')
  })
})

describe('buildToolAvailabilityPrompt', () => {
  it('describes the enabled tools only', () => {
    const prompt = buildToolAvailabilityPrompt({
      generate_image: false,
      generate_video: false,
      web_search: true,
    })

    expect(prompt).toContain('web_search')
    expect(prompt).not.toContain('generate_image')
    expect(prompt).not.toContain('generate_video')
  })

  it('is empty when no tool is enabled', () => {
    expect(
      buildToolAvailabilityPrompt({
        generate_image: false,
        generate_video: false,
        web_search: false,
      })
    ).toBe('')
  })
})
