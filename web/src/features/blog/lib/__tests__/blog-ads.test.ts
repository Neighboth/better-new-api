import { describe, expect, it } from 'vitest'

import { pickBlogAdSlots } from '../blog-ads'

const ad = (id: string) => ({
  id,
  image: `https://img/${id}.png`,
  url: `https://x/${id}`,
})

describe('pickBlogAdSlots', () => {
  it('returns nothing when nothing is available', () => {
    expect(pickBlogAdSlots([], false, 'both')).toEqual([])
  })
  it('fills all four slots with adsense when only adsense is available', () => {
    const slots = pickBlogAdSlots([], true, 'both')
    expect(slots).toHaveLength(4)
    expect(slots.every((s) => s.kind === 'adsense')).toBe(true)
  })
  it('one custom ad + adsense -> 1 custom, 3 adsense', () => {
    const slots = pickBlogAdSlots([ad('a')], true, 'both')
    expect(slots.filter((s) => s.kind === 'custom')).toHaveLength(1)
    expect(slots.filter((s) => s.kind === 'adsense')).toHaveLength(3)
  })
  it('two custom ads + adsense -> alternate custom/adsense', () => {
    const slots = pickBlogAdSlots([ad('a'), ad('b')], true, 'both')
    expect(slots.map((s) => s.kind)).toEqual([
      'custom',
      'adsense',
      'custom',
      'adsense',
    ])
  })
  it('two custom ads without adsense -> 1,2 then 1,2', () => {
    const slots = pickBlogAdSlots([ad('a'), ad('b')], false, 'both')
    expect(slots.map((s) => (s.kind === 'custom' ? s.ad.id : 'x'))).toEqual([
      'a',
      'b',
      'a',
      'b',
    ])
  })
  it('four custom ads -> each exactly once', () => {
    const slots = pickBlogAdSlots(
      [ad('a'), ad('b'), ad('c'), ad('d')],
      false,
      'custom'
    )
    expect(slots.map((s) => (s.kind === 'custom' ? s.ad.id : 'x'))).toEqual([
      'a',
      'b',
      'c',
      'd',
    ])
  })
  it('four custom ads + adsense -> 2 custom + 2 adsense', () => {
    const slots = pickBlogAdSlots([ad('a'), ad('b'), ad('c'), ad('d')], true, 'both')
    expect(slots.filter((s) => s.kind === 'custom')).toHaveLength(2)
    expect(slots.filter((s) => s.kind === 'adsense')).toHaveLength(2)
  })
  it('adsense-only mode ignores custom ads', () => {
    const slots = pickBlogAdSlots([ad('a'), ad('b')], true, 'adsense')
    expect(slots.every((s) => s.kind === 'adsense')).toBe(true)
  })
  it('custom-only mode ignores adsense even when available', () => {
    const slots = pickBlogAdSlots([ad('a'), ad('b')], true, 'custom')
    expect(slots.every((s) => s.kind === 'custom')).toBe(true)
  })
})
