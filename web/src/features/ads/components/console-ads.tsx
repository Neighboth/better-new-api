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
import { useEffect, useMemo, useRef } from 'react'

import { cn } from '@/lib/utils'

import {
  fetchConsoleAds,
  trackConsoleAdImpression,
  type ConsoleAd,
} from '../api'
import { CONSOLE_AD_COUNT, pickConsoleAds } from '../lib/console-ads'

// The row of the console: two banners side by side, centred, the same height
// as the banners of the blog so that an image of the campaign fits both
// surfaces without a second version.
const CONSOLE_AD_CLASS =
  'flex h-[60px] sm:h-[75px] md:h-[90px] w-full max-w-[26rem] items-center justify-center overflow-hidden rounded-lg border'

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

  // The row is picked once per set of ads, so a re-render of the dashboard does
  // not swap the banners under the reader while the images load.
  const picked = useMemo(
    () => pickConsoleAds(ads, CONSOLE_AD_COUNT),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [adsKey]
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
    >
      {picked.map((ad) => (
        <ConsoleAdBanner key={ad.id} ad={ad} />
      ))}
    </div>
  )
}

function ConsoleAdBanner(props: { ad: ConsoleAd }) {
  const reported = useRef(false)
  useEffect(() => {
    if (reported.current) return
    reported.current = true
    void trackConsoleAdImpression(props.ad.id)
  }, [props.ad.id])

  return (
    <a
      href={props.ad.url}
      target='_blank'
      rel='noopener noreferrer sponsored'
      className={CONSOLE_AD_CLASS}
      title={props.ad.id}
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
