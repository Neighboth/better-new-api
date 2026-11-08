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
import { useCallback, useEffect, useMemo, useRef } from 'react'
import { useTranslation } from 'react-i18next'

import { PublicLayout } from '@/components/layout'
import { Footer } from '@/components/layout/components/footer'
import { RichContent } from '@/components/rich-content'
import { useTheme } from '@/context/theme-provider'
import { isLikelyHtml } from '@/lib/content-format'
import { useAuthStore } from '@/stores/auth-store'

import { CTA, Features, Hero, HowItWorks, Stats } from './components'
import { useHomePageContent } from './hooks'
import { withThemeParam } from './lib/theme-param'

function prepareThemedHtml(rawHtml: string, theme: 'dark' | 'light'): string {
  if (!rawHtml) return ''

  const helperScript = `
<script>
  (function() {
    function applyTheme(t) {
      if (!t) return;
      document.documentElement.classList.remove('light', 'dark');
      document.documentElement.classList.add(t);
      document.documentElement.setAttribute('data-theme', t);
      document.documentElement.style.colorScheme = t;
      if (document.body) {
        document.body.classList.remove('light', 'dark');
        document.body.classList.add(t);
        document.body.setAttribute('data-theme', t);
        document.body.style.colorScheme = t;
      }
    }
    window.addEventListener('message', function(e) {
      if (e.data && e.data.themeMode) {
        applyTheme(e.data.themeMode);
      }
    });
    document.addEventListener('click', function(e) {
      var a = e.target && e.target.closest ? e.target.closest('a') : null;
      if (a && a.href && !a.getAttribute('target')) {
        a.target = '_top';
      }
    }, true);
  })();
</script>`

  const helperHead = `
  <base target="_top">
  <style>
    :root { color-scheme: ${theme}; }
    html.dark, body.dark, [data-theme='dark'] { color-scheme: dark; }
    html.light, body.light, [data-theme='light'] { color-scheme: light; }
  </style>`

  let html = rawHtml
  const hasHtmlTag = /<html[^>]*>/i.test(html)
  const hasHeadTag = /<head[^>]*>/i.test(html)
  const hasBodyTag = /<body[^>]*>/i.test(html)

  if (hasHtmlTag || hasHeadTag || hasBodyTag) {
    if (hasHtmlTag) {
      html = html.replace(/<html([^>]*)>/i, (_match, attrs) => {
        let clean = attrs.replace(/\bclass=["'][^"']*["']/gi, (c: string) => {
          return c.replace(/\b(light|dark)\b/g, '').trim()
        })
        let existingClass = ''
        const classMatch = attrs.match(/\bclass=["']([^"']*)["']/i)
        if (classMatch) {
          existingClass = classMatch[1].replace(/\b(light|dark)\b/g, '').trim()
        }
        clean = clean.replace(/\bclass=["'][^"']*["']/gi, '')
        clean = clean.replace(/\bdata-theme=["'][^"']*["']/gi, '')
        clean = clean.replace(/\bstyle=["']([^"']*?)color-scheme:[^;]+;?([^"']*?)["']/gi, 'style="$1$2"')
        return `<html${clean} class="${existingClass ? existingClass + ' ' : ''}${theme}" data-theme="${theme}" style="color-scheme: ${theme};">`
      })
    }

    if (hasBodyTag) {
      html = html.replace(/<body([^>]*)>/i, (_match, attrs) => {
        let clean = attrs.replace(/\bclass=["'][^"']*["']/gi, (c: string) => {
          return c.replace(/\b(light|dark)\b/g, '').trim()
        })
        let existingClass = ''
        const classMatch = attrs.match(/\bclass=["']([^"']*)["']/i)
        if (classMatch) {
          existingClass = classMatch[1].replace(/\b(light|dark)\b/g, '').trim()
        }
        clean = clean.replace(/\bclass=["'][^"']*["']/gi, '')
        clean = clean.replace(/\bdata-theme=["'][^"']*["']/gi, '')
        return `<body${clean} class="${existingClass ? existingClass + ' ' : ''}${theme}" data-theme="${theme}">`
      })
    }

    if (hasHeadTag) {
      html = html.replace(/<head([^>]*)>/i, `<head$1>${helperHead}`)
    } else if (hasHtmlTag) {
      html = html.replace(/<html([^>]*)>/i, `<html$1><head>${helperHead}</head>`)
    }

    if (/<\/body>/i.test(html)) {
      html = html.replace(/<\/body>/i, `${helperScript}</body>`)
    } else {
      html += helperScript
    }

    return html
  }

  return `<!DOCTYPE html>
<html class="${theme}" data-theme="${theme}" style="color-scheme: ${theme};">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  ${helperHead}
</head>
<body class="${theme}" data-theme="${theme}">
  ${html}
  ${helperScript}
</body>
</html>`
}

export function Home() {
  const { i18n, t } = useTranslation()
  const iframeRef = useRef<HTMLIFrameElement>(null)
  const { resolvedTheme } = useTheme()
  const { auth } = useAuthStore()
  const isAuthenticated = !!auth.user
  const { content, isLoaded, isUrl } = useHomePageContent()

  const syncIframePreferences = useCallback(() => {
    try {
      iframeRef.current?.contentWindow?.postMessage(
        { themeMode: resolvedTheme },
        '*'
      )
      iframeRef.current?.contentWindow?.postMessage(
        { lang: i18n.language },
        '*'
      )
    } catch {
      // Cross-origin frames may reject access while navigating.
    }
  }, [i18n.language, resolvedTheme])

  const themedContent = useMemo(() => {
    if (!isUrl || !content) return content
    return withThemeParam(content, resolvedTheme === 'dark' ? 'dark' : 'light')
  }, [content, isUrl, resolvedTheme])

  const themedHtml = useMemo(() => {
    if (!content || isUrl || !isLikelyHtml(content)) return ''
    return prepareThemedHtml(
      content,
      resolvedTheme === 'dark' ? 'dark' : 'light'
    )
  }, [content, isUrl, resolvedTheme])

  useEffect(() => {
    if (isUrl || isLikelyHtml(content)) {
      syncIframePreferences()
    }
  }, [content, isUrl, syncIframePreferences, themedContent, themedHtml, resolvedTheme])

  if (!isLoaded) {
    return (
      <PublicLayout showMainContainer={false}>
        <main className='flex min-h-screen items-center justify-center'>
          <div className='text-muted-foreground'>{t('Loading...')}</div>
        </main>
      </PublicLayout>
    )
  }

  if (content) {
    if (isUrl) {
      return (
        <PublicLayout showMainContainer={false}>
          <iframe
            ref={iframeRef}
            src={themedContent}
            className='h-screen w-full border-none'
            title={t('Custom Home Page')}
            sandbox='allow-forms allow-popups allow-popups-to-escape-sandbox allow-scripts allow-top-navigation allow-top-navigation-by-user-activation'
            allow='fullscreen'
            referrerPolicy='no-referrer'
            onLoad={syncIframePreferences}
          />
        </PublicLayout>
      )
    }

    const contentIsHtml = isLikelyHtml(content)

    if (contentIsHtml) {
      return (
        <PublicLayout showMainContainer={false}>
          <iframe
            ref={iframeRef}
            srcDoc={themedHtml}
            className='min-h-screen w-full border-none'
            title={t('Custom Home Page')}
            sandbox='allow-scripts allow-forms allow-popups allow-popups-to-escape-sandbox allow-same-origin allow-modals allow-top-navigation allow-top-navigation-by-user-activation'
            allow='fullscreen; clipboard-read; clipboard-write'
            onLoad={syncIframePreferences}
          />
        </PublicLayout>
      )
    }

    return (
      <PublicLayout>
        <div className='mx-auto max-w-6xl px-4 py-8'>
          <RichContent
            mode='markdown'
            content={content}
            className='custom-home-content'
          />
        </div>
      </PublicLayout>
    )
  }

  return (
    <PublicLayout showMainContainer={false}>
      <Hero isAuthenticated={isAuthenticated} />
      <Stats />
      <Features />
      <HowItWorks />
      <CTA isAuthenticated={isAuthenticated} />
      <Footer />
    </PublicLayout>
  )
}
