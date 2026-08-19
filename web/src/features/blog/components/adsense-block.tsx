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
import { useEffect } from 'react'

import { useStatus } from '@/hooks/use-status'

declare global {
  interface Window {
    adsbygoogle?: unknown[]
  }
}

/**
 * Renders a Google AdSense slot. Only rendered when the admin configured an
 * AdSense publisher ID and slot ID; the hook callers guard on that, but this
 * component also self-guards.
 */
export function AdSenseBlock() {
  const { status } = useStatus()
  const clientId = status?.adsense_client_id || ''
  const slotId = status?.adsense_slot_id || ''

  useEffect(() => {
    if (!clientId || !slotId) return

    const scriptId = 'adsense-script'
    if (!document.querySelector(`#${scriptId}`)) {
      const script = document.createElement('script')
      script.id = scriptId
      script.async = true
      script.crossOrigin = 'anonymous'
      script.src = `https://pagead2.googlesyndication.com/pagead/js/adsbygoogle.js?client=${encodeURIComponent(clientId)}`
      document.head.appendChild(script)
    }

    try {
      window.adsbygoogle = window.adsbygoogle || []
      window.adsbygoogle.push({})
    } catch {
      /* adsense not ready yet */
    }
  }, [clientId, slotId])

  if (!clientId || !slotId) return null

  return (
    <ins
      className='adsbygoogle my-6 block'
      data-ad-client={clientId}
      data-ad-slot={slotId}
      data-ad-format='auto'
      data-full-width-responsive='true'
    />
  )
}
