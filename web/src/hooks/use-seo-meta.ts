import { useEffect } from 'react'

import { normalizeInterfaceLanguage } from '@/i18n/languages'

type SeoParams = {
  title: string
  description?: string
  keywords?: string
  /** Per-language localized titles, keyed by locale code. */
  localizedTitles?: Record<string,string>
  /** Per-language localized descriptions, keyed by locale code. */
  localizedDescriptions?: Record<string,string>
}

export function useSeoMeta(params: SeoParams) {
  const { title, description, keywords } = params
  const localizedTitles = params.localizedTitles
  const localizedDescriptions = params.localizedDescriptions

  useEffect(() => {
    setMeta('description', description)
    setMeta('keywords', keywords)

    const notranslate = document.querySelector('meta[name="google"]')
    if (notranslate) notranslate.remove()

    setCanonical(window.location.pathname)

    const pathByLocale: Record<string,string> = {}
    for (const [locale, value] of Object.entries(localizedTitles ?? {})) {
      if (value) pathByLocale[locale] = window.location.pathname
    }
    for (const [locale, value] of Object.entries(localizedDescriptions ?? {})) {
      if (value) pathByLocale[locale] = window.location.pathname
    }
    setHreflang(pathByLocale)

    return () => {
      const googleMeta = document.createElement('meta')
      googleMeta.setAttribute('name', 'google')
      googleMeta.setAttribute('content', 'notranslate')
      document.head.appendChild(googleMeta)

      document.querySelectorAll('meta[name="description"]')?.forEach((el) => el.remove())
      document.querySelectorAll('meta[name="keywords"]')?.forEach((el) => el.remove())
      document.querySelectorAll('meta[property="og:title"]')?.forEach((el) => el.remove())
      document.querySelectorAll('link[rel="canonical"]')?.forEach((el) => el.remove())
      document.querySelectorAll('link[rel="alternate"][hreflang]')?.forEach((el) => el.remove())
    }
    // All values flow from the params object; re-apply when any of them changes。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [title, description, keywords, localizedTitles, localizedDescriptions])
}

function setMeta(name: string, content: string | undefined) {
  if (!content) return
  let meta = document.querySelector<HTMLMetaElement>(`meta[name="${name}"]`)
  if (!meta) {
    meta = document.createElement('meta')
    meta.setAttribute('name', name)
    document.head.appendChild(meta)
  }
  meta.setAttribute('content', content)
}

function setCanonical(pathname: string) {
  let link = document.querySelector<HTMLLinkElement>('link[rel="canonical"]')
  if (!link) {
    link = document.createElement('link')
    link.setAttribute('rel', 'canonical')
    document.head.appendChild(link)
  }
  link.setAttribute('href', `${window.location.origin}${pathname}`)
}

function setHreflang(pathByLocale: Record<string,string>) {

  const seen = new Set<string>()
  for (const [locale, path] of Object.entries(pathByLocale)) {
    const code = normalizeInterfaceLanguage(locale)
    if (!path || seen.has(code)) continue
    seen.add(code)
    let link = document.querySelector<HTMLLinkElement>(
      `link[rel="alternate"][hreflang="${code}"]`
    )
    if (!link) {
      link = document.createElement('link')
      link.setAttribute('rel', 'alternate')
      link.setAttribute('hreflang', code)
      document.head.appendChild(link)
    }
    link.setAttribute('href', `${window.location.origin}${path}`)
  }
}