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
import { useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { cn } from '@/lib/utils'

import {
  fetchConsoleAds,
  trackConsoleAdImpression,
  type ConsoleAd,
} from '../api'
import { resolveAdImage } from '../lib/ad-image'
import {
  CONSOLE_AD_COUNT,
  consoleAdCountForWidth,
  pickConsoleAds,
} from '../lib/console-ads'

// The row of the console: two banners side by side, centred, the same height
// as the banners of the blog so that an image of the campaign fits both
// surfaces without a second version.
const CONSOLE_AD_CLASS =
  'flex h-[60px] sm:h-[75px] md:h-[90px] w-full min-w-0 flex-1 basis-[18rem] max-w-[26rem] items-center justify-center overflow-hidden rounded-lg border'

/**
 * The row of ads of the console.
 *
 * It is drawn between the cards of the dashboard and only when the
 * administrator turned the console on and wrote at least one ad for it.
 */
export function ConsoleAds(props: { className?: string }) {
  const { data } = useQuery({
    queryKey: ['console-ads'],
    queryFn: fetchConsoleAds,
    staleTime: 60_000,
  })

  const ads = data?.custom_ads ?? []
  const adsKey = ads.map((ad) => ad.id).join('|')

  // The row holds two banners only when the dashboard is wide enough to show
  // both at a readable width: a narrow window shows the same banner squeezed
  // into half a phone, so it gets one ad instead of two unreadable ones.
  const rowRef = useRef<HTMLDivElement | null>(null)
  const [rowWidth, setRowWidth] = useState<number | null>(null)

  useEffect(() => {
    const element = rowRef.current
    if (!element) return
    const measure = () => setRowWidth(element.clientWidth)
    measure()
    if (typeof ResizeObserver === 'undefined') {
      window.addEventListener('resize', measure)
      return () => window.removeEventListener('resize', measure)
    }
    const observer = new ResizeObserver(measure)
    observer.observe(element)
    return () => observer.disconnect()
  }, [data?.enabled])

  const count = consoleAdCountForWidth(rowWidth ?? 0, CONSOLE_AD_COUNT)

  // The row is picked once per set of ads and per number of slots, so a
  // re-render of the dashboard does not swap the banners under the reader while
  // the images load.
  const picked = useMemo(
    () => pickConsoleAds(ads, count),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [adsKey, count]
  )

  // The console draws image ads only: AdSense belongs to the blog, where its
  // script is loaded. An installation that runs AdSense alone shows nothing
  // here instead of an empty slot.
  if (!data?.enabled || data.mode === 'adsense' || picked.length === 0) {
    return null
  }

  return (
    <div
      className={cn(
        'flex w-full flex-wrap items-center justify-center gap-3 sm:gap-4',
        props.className
      )}
      ref={rowRef}
    >
      {rowWidth !== null &&
        picked.map((ad) => <ConsoleAdBanner key={ad.id} ad={ad} />)}
    </div>
  )
}

function ConsoleAdBanner(props: { ad: ConsoleAd }) {
  const { i18n } = useTranslation()
  const reported = useRef(false)
  useEffect(() => {
    if (reported.current) return
    reported.current = true
    void trackConsoleAdImpression(props.ad.id)
  }, [props.ad.id])

  const displayImage = resolveAdImage(props.ad, i18n.language)

  return (
    <a
      href={props.ad.url}
      target='_blank'
      rel='noopener noreferrer sponsored'
      className={CONSOLE_AD_CLASS}
      title={props.ad.id}
    >
      <img
        src={displayImage}
        alt={props.ad.id}
        className='h-full w-full object-cover'
        loading='lazy'
      />
    </a>
  )
}
