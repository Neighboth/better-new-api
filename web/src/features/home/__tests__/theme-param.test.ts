import { describe, expect, it } from 'vitest'

import { withThemeParam } from '../lib/theme-param'

describe('withThemeParam', () => {
  it('appends the theme to a bare URL', () => {
    expect(withThemeParam('https://example.com/page', 'dark')).toBe(
      'https://example.com/page?theme=dark'
    )
  })

  it('preserves existing query params and replaces a stale theme', () => {
    expect(
      withThemeParam('https://example.com/page?foo=1&theme=light', 'dark')
    ).toBe('https://example.com/page?foo=1&theme=dark')
  })

  it('keeps hash fragments after the theme param', () => {
    expect(withThemeParam('https://example.com/page#sec', 'light')).toBe(
      'https://example.com/page?theme=light#sec'
    )
  })
})
