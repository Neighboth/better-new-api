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
    turnstile?: {
      render: (element: HTMLElement, options: Record<string, unknown>) => void
    }
  }
}

interface TurnstileProps {
  siteKey: string
  onVerify: (token: string) => void
  onExpire?: () => void
  onError?: () => void
  className?: string
}

export function Turnstile({
  siteKey,
  onVerify,
  onExpire,
  onError,
  className,
}: TurnstileProps) {
  const ref = useRef<HTMLDivElement | null>(null)

  useEffect(() => {
    const render = () => {
      if (!ref.current || !window.turnstile) return
      try {
        window.turnstile.render(ref.current, {
          sitekey: siteKey,
          callback: (token: string) => onVerify(token),
          'error-callback': () => (onError ?? onExpire)?.(),
          'expired-callback': () => onExpire?.(),
        })
      } catch {
        onError?.()
      }
    }

    if (window.turnstile) {
      render()
      return
    }
    const scriptId = 'cf-turnstile'

    const existing = document.querySelector(`#${scriptId}`)
    if (existing) {
      // Tag exists from an earlier mount: listen for completion, but never
      // wait forever — on timeout drop the dead tag so a retry re-injects.
      const giveUp = () => {
        window.clearInterval(pollTimer)
        existing.remove()
        onError?.()
      }
      const pollTimer = window.setInterval(() => {
        if (window.turnstile) {
          window.clearInterval(pollTimer)
          render()
        }
      }, 200)
      const timeout = window.setTimeout(giveUp, 10000)
      existing.addEventListener('load', render, { once: true })
      existing.addEventListener('error', giveUp, { once: true })
      return () => {
        window.clearInterval(pollTimer)
        window.clearTimeout(timeout)
      }
    }
    const s = document.createElement('script')
    s.id = scriptId
    s.src =
      'https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit'
    s.async = true
    s.defer = true
    s.addEventListener('load', render)
    s.addEventListener('error', () => {
      s.remove()
      onError?.()
    })
    document.head.appendChild(s)
  }, [siteKey, onVerify, onExpire, onError])

  return <div ref={ref} className={className} />
}
