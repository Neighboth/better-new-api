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

import {
  pickStillValid,
  resolveEffectiveModelOptions,
  DEFAULT_REFERENCE_IMAGE_CAP,
} from '../model-constraints'

describe('resolveEffectiveModelOptions', () => {
  it('returns the configured lists untouched when no rules match', () => {
    const out = resolveEffectiveModelOptions(
      { qualities: ['720p', '1080p'], durations: [5, 10, 15], ratios: ['16:9'], reference_image: true },
      5,
      '720p',
      '16:9'
    )
    expect(out.durations).toEqual([5, 10, 15])
    expect(out.qualities).toEqual(['720p', '1080p'])
    expect(out.ratios).toEqual(['16:9'])
    expect(out.referencesAllowed).toBe(true)
    expect(out.maxReferenceImages).toBe(DEFAULT_REFERENCE_IMAGE_CAP)
  })

  it('narrows qualities for a matching duration rule (1080p disappears at 15s)', () => {
    const out = resolveEffectiveModelOptions(
      {
        qualities: ['720p', '1080p'],
        durations: [10, 15],
        reference_image: true,
        constraints: [{ durations: [15], qualities_only: ['720p'] }],
      },
      15,
      '720p'
    )
    expect(out.qualities).toEqual(['720p'])
  })

  it('keeps qualities when the duration does not match the rule', () => {
    const out = resolveEffectiveModelOptions(
      {
        qualities: ['720p', '1080p'],
        durations: [10, 15],
        reference_image: true,
        constraints: [{ durations: [15], qualities_only: ['720p'] }],
      },
      10,
      '1080p'
    )
    expect(out.qualities).toEqual(['720p', '1080p'])
  })

  it('disables reference images at a matching duration (-1 sentinel)', () => {
    const out = resolveEffectiveModelOptions(
      {
        reference_image: true,
        max_reference_images: 5,
        constraints: [{ durations: [10], max_reference_images: -1 }],
      },
      10
    )
    expect(out.referencesAllowed).toBe(false)
    expect(out.maxReferenceImages).toBe(0)
  })

  it('overrides the reference cap at a matching quality', () => {
    const out = resolveEffectiveModelOptions(
      {
        reference_image: true,
        max_reference_images: 5,
        constraints: [{ qualities: ['1080p'], max_reference_images: 2 }],
      },
      undefined,
      '1080p'
    )
    expect(out.maxReferenceImages).toBe(2)
  })

  it('applies later rules over earlier ones per field', () => {
    const out = resolveEffectiveModelOptions(
      {
        reference_image: true,
        max_reference_images: 5,
        constraints: [
          { qualities: ['1080p'], max_reference_images: 2 },
          { qualities: ['1080p'], max_reference_images: 1 },
        ],
      },
      undefined,
      '1080p'
    )
    expect(out.maxReferenceImages).toBe(1)
  })

  it('disables references entirely when the master switch is off', () => {
    const out = resolveEffectiveModelOptions(
      { reference_image: false, max_reference_images: 12 },
      undefined,
      undefined
    )
    expect(out.referencesAllowed).toBe(false)
    expect(out.maxReferenceImages).toBe(0)
  })

  it('narrows durations for a matching ratio rule', () => {
    const out = resolveEffectiveModelOptions(
      {
        durations: [5, 10, 15],
        ratios: ['16:9', '9:16'],
        reference_image: true,
        constraints: [{ ratios: ['9:16'], durations_only: [5] }],
      },
      undefined,
      undefined,
      '9:16'
    )
    expect(out.durations).toEqual([5])
  })
})

describe('pickStillValid', () => {
  it('keeps the selection when it is still offered', () => {
    expect(pickStillValid(10, [5, 10, 15])).toBe(10)
  })
  it('falls back to the first option when the selection disappeared', () => {
    expect(pickStillValid(15, [10])).toBe(10)
  })
  it('returns undefined for an empty list', () => {
    expect(pickStillValid(15, [])).toBeUndefined()
  })
})
