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
import { useEffect, useRef } from 'react'

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

  useCaptchaScript(
    'google-recaptcha',
    'https://www.google.com/recaptcha/api.js?render=explicit',
    () => Boolean(window.grecaptcha?.render),
    () => {
      // With render=explicit the API can be present before it is ready;
      // grecaptcha.ready() is the only safe point to call render().
      const doRender = () => {
        if (!ref.current || !window.grecaptcha) return
        try {
          window.grecaptcha.render(ref.current, {
            sitekey: siteKey,
            callback: (token: string) => onVerify(token),
            'expired-callback': () => onExpire?.(),
            'error-callback': () => onError?.(),
          })
        } catch {
          onError?.()
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

  return <div ref={ref} />
}

export function HCaptchaWidget({
  siteKey,
  onVerify,
  onExpire,
  onError,
}: ThirdPartyCaptchaProps) {
  const ref = useRef<HTMLDivElement | null>(null)

  useCaptchaScript(
    'hcaptcha-script',
    'https://js.hcaptcha.com/1/api.js?render=explicit',
    () => Boolean(window.hcaptcha?.render),
    () => {
      if (!ref.current || !window.hcaptcha) return
      try {
        window.hcaptcha.render(ref.current, {
          sitekey: siteKey,
          callback: (token: string) => onVerify(token),
          'expired-callback': () => onExpire?.(),
          'error-callback': () => onError?.(),
        })
      } catch {
        onError?.()
      }
    },
    onError,
    [siteKey, onVerify, onExpire, onError]
  )

  return <div ref={ref} />
}
