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
  isStudioItem,
  mergeStudioItems,
  stripVolatileMediaFields,
} from '../studio-sync'
import type { StudioItem } from '../../types'

function makeItem(overrides: Partial<StudioItem> = {}): StudioItem {
  return {
    id: 'item-1',
    mode: 'video',
    prompt: 'a prompt',
    createdAt: 1_000,
    status: 'ready',
    urls: ['/uploads/videos/7/file.mp4'],
    settings: { model: 'seedance-2.0-fast' },
    ...overrides,
  }
}

describe('mergeStudioItems', () => {
  it('unions local and remote items and sorts newest first', () => {
    const local = [makeItem({ id: 'local-only', createdAt: 100 })]
    const remote = [{ item_id: 'remote-only', payload: makeItem({ id: 'remote-only', createdAt: 200 }) }]
    const merged = mergeStudioItems(local, remote)
    expect(merged.map((item) => item.id)).toEqual(['remote-only', 'local-only'])
  })

  it('remote payload wins for the same id (account-wide truth)', () => {
    const local = [makeItem({ id: 'shared', prompt: 'local prompt', createdAt: 100 })]
    const remote = [{ item_id: 'shared', payload: makeItem({ id: 'shared', prompt: 'remote prompt', createdAt: 100 }) }]
    const merged = mergeStudioItems(local, remote)
    expect(merged).toHaveLength(1)
    expect(merged[0].prompt).toBe('remote prompt')
  })

  it('skips malformed remote rows instead of breaking the local list', () => {
    const local = [makeItem({ id: 'local', createdAt: 100 })]
    const merged = mergeStudioItems(local, [
      { item_id: 'broken', payload: { id: 'x' } },
    ])
    expect(merged.map((item) => item.id)).toEqual(['local'])
  })
})

describe('isStudioItem', () => {
  it('accepts a well-formed item', () => {
    expect(isStudioItem(makeItem())).toBe(true)
  })

  it('rejects items without id, urls array or known status', () => {
    expect(isStudioItem({})).toBe(false)
    expect(isStudioItem({ ...makeItem(), id: '' })).toBe(false)
    expect(isStudioItem({ ...makeItem(), urls: 'not-an-array' })).toBe(false)
    expect(isStudioItem({ ...makeItem(), status: 'mystery' })).toBe(false)
  })
})

describe('stripVolatileMediaFields', () => {
  it('drops data: URLs from details but keeps real URLs', () => {
    const item = makeItem({
      details: {
        startFrame: 'data:image/png;base64,AAAA',
        endFrame: 'https://cdn.example/end.png',
        referenceImages: ['data:image/png;base64,BBBB', 'https://cdn.example/ref.png'],
        negativePrompt: 'no blur',
      },
    })
    const stripped = stripVolatileMediaFields(item)
    expect(stripped.details?.startFrame).toBeUndefined()
    expect(stripped.details?.endFrame).toBe('https://cdn.example/end.png')
    expect(stripped.details?.referenceImages).toEqual(['https://cdn.example/ref.png'])
    expect(stripped.details?.negativePrompt).toBe('no blur')
  })

  it('drops the details object entirely when nothing remains', () => {
    const item = makeItem({
      details: { startFrame: 'data:image/png;base64,AAAA', referenceImages: ['data:image/png;base64,BBBB'] },
    })
    const stripped = stripVolatileMediaFields(item)
    expect(stripped.details).toBeUndefined()
  })

  it('strips live progress fields so polling does not flood the sync endpoint', () => {
    const item = makeItem({ progress: '42%', queuePosition: 3 })
    const stripped = stripVolatileMediaFields(item)
    expect(stripped.progress).toBeUndefined()
    expect(stripped.queuePosition).toBeUndefined()
    // Everything else survives untouched.
    expect(stripped.urls).toEqual(item.urls)
  })

  it('keeps non-data reference lists as-is', () => {
    const item = makeItem({
      details: { referenceImages: ['https://cdn.example/a.png', 'https://cdn.example/b.png'] },
    })
    expect(stripVolatileMediaFields(item).details?.referenceImages).toHaveLength(2)
  })
})
