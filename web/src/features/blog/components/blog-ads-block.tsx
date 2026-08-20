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
import { useQuery } from '@tanstack/react-query'
import { useEffect, useRef } from 'react'

import { fetchBlogAds, trackAdImpression, type BlogAdsConfig } from '../api'
import { pickBlogAdSlots, type AdSlot } from '../lib/blog-ads'

declare global {
  interface Window {
    adsbygoogle?: unknown[]
  }
}

// Ad slots render as low-height, wide banners, two side by side per row.
const BLOG_AD_BANNER_CLASS =
  'flex h-[90px] w-full items-center justify-center overflow-hidden rounded-md border md:h-[110px]'

/** Four blog ad slots: two above the post, two below. */
export function BlogAds() {
  const { data } = useQuery({
    queryKey: ['blog-ads'],
    queryFn: fetchBlogAds,
    staleTime: 60_000,
  })

  if (!data || !data.enabled) return null

  const adsenseAvailable = Boolean(data.adsense_client_id && data.adsense_slot_id)
  const slots = pickBlogAdSlots(data.custom_ads ?? [], adsenseAvailable, data.mode)
  if (slots.length === 0) return null

  return (
    <>
      <BlogAdRow slots={slots.slice(0, 2)} config={data} />
      <BlogAdRow slots={slots.slice(2, 4)} config={data} isBottom />
    </>
  )
}

function BlogAdRow(props: {
  slots: AdSlot[]
  config: BlogAdsConfig
  isBottom?: boolean
}) {
  if (props.slots.length === 0) return null

  return (
    <div className={`grid grid-cols-2 gap-2 ${props.isBottom ? 'mt-6' : ''}`}>
      {props.slots.map((slot, slotIndex) => {
        // Slots are fixed (2 per row) and positions are stable, so index keys
        // are safe here — but use a data-derived key anyway to satisfy lint.
        const rowPrefix = props.isBottom ? 'bottom' : 'top'
        const key =
          slot.kind === 'adsense'
            ? `adsense-${rowPrefix}-${slotIndex}`
            : `${slot.ad.id}-${rowPrefix}-${slotIndex}`
        return slot.kind === 'adsense' ? (
          <AdSenseBanner
            key={key}
            clientId={props.config.adsense_client_id}
            slotId={props.config.adsense_slot_id}
            onImpression={() => void trackAdImpression('adsense')}
          />
        ) : (
          <CustomAdBanner key={key} ad={slot.ad} />
        )
      })}
    </div>
  )
}

function CustomAdBanner(props: { ad: { id: string; image: string; url: string } }) {
  const reported = useRef(false)
  useEffect(() => {
    if (reported.current) return
    reported.current = true
    void trackAdImpression(props.ad.id)
  }, [props.ad.id])

  return (
    <a
      href={props.ad.url}
      target='_blank'
      rel='noopener noreferrer sponsored'
      className={BLOG_AD_BANNER_CLASS}
    >
      <img
        src={props.ad.image}
        alt={props.ad.id}
        className='h-full w-full object-cover'
        loading='lazy'
      />
    </a>
  )
}

function AdSenseBanner(props: {
  clientId: string
  slotId: string
  onImpression: () => void
}) {
  const pushed = useRef(false)

  useEffect(() => {
    if (pushed.current) return
    pushed.current = true

    const scriptId = 'adsense-script'
    if (!document.querySelector(`#${CSS.escape(scriptId)}`)) {
      const script = document.createElement('script')
      script.id = scriptId
      script.async = true
      script.crossOrigin = 'anonymous'
      script.src = `https://pagead2.googlesyndication.com/pagead/js/adsbygoogle.js?client=${encodeURIComponent(props.clientId)}`
      document.head.appendChild(script)
    }

    try {
      window.adsbygoogle = window.adsbygoogle || []
      window.adsbygoogle.push({})
      props.onImpression()
    } catch {
      /* adsense not ready yet */
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [props.clientId])

  return (
    <ins
      className={`adsbygoogle ${BLOG_AD_BANNER_CLASS} block`}
      data-ad-client={props.clientId}
      data-ad-slot={props.slotId}
      data-ad-format='auto'
      data-full-width-responsive='true'
    />
  )
}
