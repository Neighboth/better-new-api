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
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'

declare global {
  interface Window {
    grecaptcha?: {
      render: (
        element: HTMLElement,
        options: Record<string, unknown>
      ) => number
      ready?: (callback: () => void) => void
    }
    hcaptcha?: {
      render: (
        element: HTMLElement,
        options: Record<string, unknown>
      ) => number
    }
  }
}

interface ThirdPartyCaptchaProps {
  siteKey: string
  onVerify: (token: string) => void
  onExpire?: () => void
  onError?: () => void
}

function useCaptchaScript(
  scriptId: string,
  src: string,
  isReady: () => boolean,
  render: () => void,
  onLoadError: (() => void) | undefined,
  deps: unknown[]
) {
  useEffect(() => {
    if (isReady()) {
      render()
      return
    }
    const existing = document.querySelector(`#${scriptId}`)
    if (!existing) {
      const script = document.createElement('script')
      script.id = scriptId
      script.src = src
      script.async = true
      script.defer = true
      script.addEventListener('load', () => render())
      script.addEventListener('error', () => {
        script.remove()
        onLoadError?.()
      })
      document.head.appendChild(script)
      return
    }
    // Script tag exists but not loaded yet — poll briefly, then give up and
    // remove the dead tag so the next attempt re-injects a fresh one.
    const startedAt = Date.now()
    const timer = window.setInterval(() => {
      if (isReady()) {
        window.clearInterval(timer)
        render()
      } else if (Date.now() - startedAt > 10000) {
        window.clearInterval(timer)
        existing.remove()
        onLoadError?.()
      }
    }, 200)
    return () => window.clearInterval(timer)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps)
}

export function RecaptchaWidget({
  siteKey,
  onVerify,
  onExpire,
  onError,
}: ThirdPartyCaptchaProps) {
  const ref = useRef<HTMLDivElement | null>(null)
  const widgetIdRef = useRef<number | null>(null)

  useCaptchaScript(
    'google-recaptcha',
    'https://www.google.com/recaptcha/api.js?render=explicit',
    () => Boolean(window.grecaptcha?.render),
    () => {
      // With render=explicit the API can be present before it is ready;
      // grecaptcha.ready() is the only safe point to call render().
      const doRender = () => {
        if (!ref.current || !window.grecaptcha) return
        // Do not re-render if already rendered in this container
        if (widgetIdRef.current !== null || ref.current.hasChildNodes()) return
        try {
          widgetIdRef.current = window.grecaptcha.render(ref.current, {
            sitekey: siteKey,
            callback: (token: string) => onVerify(token),
            'expired-callback': () => onExpire?.(),
            'error-callback': () => onError?.(),
          })
        } catch (e) {
          // If already rendered, do not trigger error
          if (!ref.current?.hasChildNodes()) {
            onError?.()
          }
        }
      }
      if (window.grecaptcha?.ready) {
        window.grecaptcha.ready(doRender)
      } else {
        doRender()
      }
    },
    onError,
    [siteKey, onVerify, onExpire, onError]
  )

  useEffect(() => {
    return () => {
      widgetIdRef.current = null
    }
  }, [])

  return <div ref={ref} />
}

export function HCaptchaWidget({
  siteKey,
  onVerify,
  onExpire,
  onError,
}: ThirdPartyCaptchaProps) {
  const ref = useRef<HTMLDivElement | null>(null)
  const widgetIdRef = useRef<number | null>(null)

  useCaptchaScript(
    'hcaptcha-script',
    'https://js.hcaptcha.com/1/api.js?render=explicit',
    () => Boolean(window.hcaptcha?.render),
    () => {
      if (!ref.current || !window.hcaptcha) return
      if (widgetIdRef.current !== null || ref.current.hasChildNodes()) return
      try {
        widgetIdRef.current = window.hcaptcha.render(ref.current, {
          sitekey: siteKey,
          callback: (token: string) => onVerify(token),
          'expired-callback': () => onExpire?.(),
          'error-callback': () => onError?.(),
        })
      } catch {
        if (!ref.current?.hasChildNodes()) {
          onError?.()
        }
      }
    },
    onError,
    [siteKey, onVerify, onExpire, onError]
  )

  useEffect(() => {
    return () => {
      widgetIdRef.current = null
    }
  }, [])

  return <div ref={ref} />
}

/**
 * ALTCHA is an open source, self-hosted proof-of-work captcha. The widget
 * solves the challenge issued by this installation and hands back a signed
 * payload which the server verifies with its own HMAC key.
 */
export function AltchaWidget({
  onVerify,
  onExpire,
  onError,
}: Omit<ThirdPartyCaptchaProps, 'siteKey'>) {
  const { t, i18n } = useTranslation()
  const containerRef = useRef<HTMLDivElement | null>(null)
  const [attempt, setAttempt] = useState(0)
  const [failed, setFailed] = useState(false)

  useEffect(() => {
    const container = containerRef.current
    if (!container) return

    container.innerHTML = ''
    setFailed(false)

    const widget = document.createElement('altcha-widget')
    widget.setAttribute('challengeurl', '/api/captcha/altcha')
    widget.setAttribute('auto', 'onload')
    widget.setAttribute('hidelogo', '')
    widget.setAttribute('hidefooter', '')
    const language = (i18n.language || 'en').split('-')[0]
    if (language) {
      widget.setAttribute('language', language)
    }

    const handleStateChange = (event: Event) => {
      const detail = (
        event as CustomEvent<{ state?: string; payload?: string }>
      ).detail
      if (detail?.state === 'verified') {
        // The payload is normally in the event detail; older builds only put
        // it into the hidden input the widget renders.
        const fromInput = widget.querySelector('input')?.value ?? ''
        const payload = detail.payload || fromInput
        if (payload) {
          onVerify(payload)
        } else {
          setFailed(true)
          onError?.()
        }
        return
      }
      if (detail?.state === 'expired') {
        onExpire?.()
        return
      }
      if (detail?.state === 'error') {
        // The widget could not fetch or solve the challenge (offline, blocked
        // CDN, server error). Show a retry instead of an empty box.
        setFailed(true)
        onError?.()
      }
    }

    widget.addEventListener('statechange', handleStateChange)
    container.appendChild(widget)

    return () => {
      widget.removeEventListener('statechange', handleStateChange)
      widget.remove()
    }
  }, [attempt, i18n.language, onVerify, onExpire, onError])

  return (
    <div className='space-y-2'>
      <div ref={containerRef} className='flex justify-center' />
      {failed && (
        <div className='flex flex-col items-center gap-1'>
          <p className='text-muted-foreground text-xs'>
            {t('The captcha could not be loaded')}
          </p>
          <Button
            onClick={() => setAttempt((value) => value + 1)}
            size='sm'
            type='button'
            variant='outline'
          >
            {t('Retry')}
          </Button>
        </div>
      )}
      <AltchaScript />
    </div>
  )
}

/** Loads the ALTCHA widget script from the official CDN exactly once. */
function AltchaScript() {
  useEffect(() => {
    const scriptId = 'altcha-widget'
    if (document.querySelector(`#${scriptId}`)) return

    const script = document.createElement('script')
    script.id = scriptId
    script.type = 'module'
    script.async = true
    script.src = 'https://cdn.jsdelivr.net/npm/altcha/dist/altcha.min.js'
    document.head.appendChild(script)
  }, [])

  return null
}

/**
 * MTCaptcha renders its own challenge widget and reports the verified token
 * through the global callbacks configured before the script loads.
 */
export function MTCaptchaWidget({
  siteKey,
  onVerify,
  onExpire,
  onError,
}: ThirdPartyCaptchaProps) {
  const ref = useRef<HTMLDivElement | null>(null)

  useEffect(() => {
    const container = ref.current
    if (!container) return

    container.innerHTML = ''
    const widgetId = `mtcaptcha-${Math.random().toString(36).slice(2, 10)}`
    const target = document.createElement('div')
    target.id = widgetId
    target.className = 'mtcaptcha'
    container.appendChild(target)

    if (!siteKey) {
      // Without a site key the provider cannot render anything: report it so
      // the dialog can fall back instead of showing an empty box.
      onError?.()
      return
    }

    const globalWindow = window as unknown as {
      mtcaptchaConfig?: Record<string, unknown>
    }

    // The render queue has to exist before the client script is evaluated,
    // otherwise the widget id is never rendered.
    globalWindow.mtcaptchaConfig = {
      sitekey: siteKey,
      render: 'explicit',
      renderQueue: [widgetId],
      'verified-callback': (state: { verifiedToken?: string }) => {
        const token = state?.verifiedToken
        if (typeof token === 'string' && token) {
          onVerify(token)
        } else {
          onError?.()
        }
      },
      'verifyexpired-callback': () => onExpire?.(),
      'error-callback': () => onError?.(),
    }

    if (!document.querySelector('#mtcaptcha-client')) {
      for (const src of [
        'https://service.mtcaptcha.com/mtcv1/client/mtcaptcha.min.js',
        'https://service2.mtcaptcha.com/mtcv1/client/mtcaptcha2.min.js',
      ]) {
        const script = document.createElement('script')
        script.async = true
        script.src = src
        if (src.includes('mtcaptcha.min.js')) {
          script.id = 'mtcaptcha-client'
        }
        script.addEventListener('error', () => onError?.())
        document.head.appendChild(script)
      }
    } else {
      // Script already loaded (the dialog was opened before): render the new
      // widget explicitly.
      const render = (
        globalWindow as unknown as {
          mtcaptcha?: { render?: (options: unknown) => void }
        }
      ).mtcaptcha?.render
      try {
        render?.({ target: widgetId, sitekey: siteKey })
      } catch {
        /* the queue below still covers the retry */
      }
    }

    return () => {
      container.innerHTML = ''
    }
  }, [siteKey, onVerify, onExpire, onError])

  return <div ref={ref} className='flex justify-center' />
}
