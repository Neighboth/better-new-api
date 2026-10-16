export function withThemeParam(rawUrl: string, theme: string): string {
  try {
    const url = new URL(rawUrl, window.location.origin)
    url.searchParams.set('theme', theme)
    // Preserve relative URLs as relative.
    if (/^https?:\/\//i.test(rawUrl)) {
      return url.toString()
    }
    return `${url.pathname}${url.search}${url.hash}`
  } catch {
    const sep = rawUrl.includes('?') ? '&' : '?'
    const hashIndex = rawUrl.indexOf('#')
    if (hashIndex >= 0) {
      return `${rawUrl.slice(0, hashIndex)}${sep}theme=${theme}${rawUrl.slice(hashIndex)}`
    }
    return `${rawUrl}${sep}theme=${theme}`
  }
}
