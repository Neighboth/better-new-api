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

import type { ConsoleAd } from '../../api'
import { pickConsoleAds } from '../console-ads'
import {
  parsePlacements,
  parseSurfaces,
  serializeSurfaces,
} from '../placements'

function ad(id: string, overrides: Partial<ConsoleAd> = {}): ConsoleAd {
  return {
    id,
    image: `https://cdn.example.com/${id}.png`,
    url: `https://example.com/${id}`,
    ...overrides,
  }
}

describe('the row of ads of the console', () => {
  it('draws two different ads out of a campaign', () => {
    const ads = [ad('a'), ad('b'), ad('c'), ad('d')]
    const picked = pickConsoleAds(ads, 2, () => 0.5)
    expect(picked).toHaveLength(2)
    expect(new Set(picked.map((item) => item.id)).size).toBe(2)
  })

  it('never draws the same ad twice when the installation holds one', () => {
    // The row of a single ad is one banner in the middle of the page, an empty
    // frame next to it would read as a broken ad of the installation.
    expect(pickConsoleAds([ad('only')], 2, () => 0)).toEqual([ad('only')])
    expect(pickConsoleAds([], 2)).toEqual([])
  })

  it('skips an ad without an image or without a target', () => {
    const ads = [
      ad('good'),
      ad('no-image', { image: '' }),
      ad('no-url', { url: '' }),
    ]
    const picked = pickConsoleAds(ads, 2, () => 0.5)
    expect(picked.map((item) => item.id)).toEqual(['good'])
  })

  it('changes the pair of a campaign of many ads', () => {
    const ads = [ad('a'), ad('b'), ad('c'), ad('d'), ad('e')]
    const first = pickConsoleAds(ads, 2, () => 0).map((item) => item.id)
    const last = pickConsoleAds(ads, 2, () => 0.99).map((item) => item.id)
    expect(first).not.toEqual(last)
    // Every pick still comes from the campaign of the installation.
    for (const id of [...first, ...last]) {
      expect(ads.map((item) => item.id)).toContain(id)
    }
  })
})

describe('the surfaces of an ad', () => {
  it('keeps an ad of an older panel on the blog', () => {
    expect(parsePlacements(undefined)).toEqual(['blog'])
    expect(parsePlacements([])).toEqual(['blog'])
    expect(parsePlacements(['unknown'])).toEqual(['blog'])
  })

  it('reads the two surfaces of an ad and drops the unknown ones', () => {
    expect(parsePlacements(['console'])).toEqual(['console'])
    expect(parsePlacements(['console', 'blog'])).toEqual(['blog', 'console'])
    expect(parsePlacements(['console', 'nope'])).toEqual(['console'])
  })

  it('reads the surfaces of the installation and falls back to the blog', () => {
    expect(parseSurfaces(undefined)).toEqual(['blog'])
    expect(parseSurfaces('')).toEqual(['blog'])
    expect(parseSurfaces('console')).toEqual(['console'])
    expect(parseSurfaces('blog, console')).toEqual(['blog', 'console'])
    expect(parseSurfaces('nonsense')).toEqual(['blog'])
  })

  it('writes the surfaces in a stable order and never writes nothing', () => {
    expect(serializeSurfaces(['console', 'blog'])).toBe('blog,console')
    expect(serializeSurfaces(['console'])).toBe('console')
    expect(serializeSurfaces([])).toBe('blog')
  })
})
