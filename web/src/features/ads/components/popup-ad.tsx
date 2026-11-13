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

import { Button } from '@/components/ui/button'

import { fetchPopupAds, trackConsoleAdImpression } from '../api'
import { resolveAdImage } from '../lib/ad-image'
import {
  pickPopupAd,
  popupAdAllowed,
  writePopupAdDismissed,
} from '../lib/popup-ad'

/**
 * The large banner of the popup surface.
 *
 * It opens over the page the reader entered the site on, one ad per visit, and
 * stays closed for five hours after the skip button is used. The root of the
 * site draws it once, so a navigation inside the site never opens it again.
 */
export function PopupAd() {
  const { t, i18n } = useTranslation()
  const { data } = useQuery({
    queryKey: ['popup-ads'],
    queryFn: fetchPopupAds,
    staleTime: 60_000,
  })

  // One decision per page load: the reader that closes the banner keeps it
  // closed while the component re-renders on a theme change or a refetch.
  const decided = useRef(false)
  const [ad, setAd] = useState<ReturnType<typeof pickPopupAd>>(null)

  const close = () => {
    writePopupAdDismissed()
    setAd(null)
  }

  const ads = data?.custom_ads ?? []
  const adsKey = ads.map((item) => item.id).join('|')

  useEffect(() => {
    if (decided.current) return
    if (!data) return
    decided.current = true
    if (!data.enabled || ads.length === 0) return
    if (!popupAdAllowed()) return
    setAd(pickPopupAd(ads))
    // The pick follows the campaign that arrived with the first response; a
    // refetch does not swap the banner of a reader that already saw it.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [data, adsKey])

  const skipSeconds = Math.max(0, Math.floor(ad?.popup_skip_seconds ?? 0))
  const [remaining, setRemaining] = useState(skipSeconds)
  const reported = useRef(false)

  useEffect(() => {
    if (!ad) return
    setRemaining(skipSeconds)
  }, [ad, skipSeconds])

  useEffect(() => {
    if (!ad || remaining <= 0) return
    const timer = window.setTimeout(() => setRemaining((value) => value - 1), 1000)
    return () => window.clearTimeout(timer)
  }, [ad, remaining])

  useEffect(() => {
    if (!ad || reported.current) return
    reported.current = true
    void trackConsoleAdImpression(ad.id)
  }, [ad])

  const image = useMemo(
    () => (ad ? resolveAdImage(ad, i18n.language) : ''),
    [ad, i18n.language]
  )

  useEffect(() => {
    if (!ad) return
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape' && remaining <= 0) close()
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [ad, remaining])

  if (!ad) return null

  return (
    <div
      aria-label={t('Advertisement')}
      aria-modal='true'
      className='fixed inset-0 z-[90] flex items-center justify-center bg-black/75 p-4 backdrop-blur-md sm:p-8'
      role='dialog'
    >
      <div className='relative w-full max-w-[500px] sm:max-w-xl'>
        <div className='overflow-hidden rounded-2xl border border-white/10 bg-zinc-950 text-zinc-100 shadow-[0_24px_80px_-12px_rgba(0,0,0,0.8)] ring-1 ring-white/5'>
          <a
            href={ad.url}
            target='_blank'
            rel='noopener noreferrer sponsored'
            className='group block'
            onClick={close}
          >
            <div className='relative flex h-52 items-center justify-center overflow-hidden bg-zinc-900 sm:h-64'>
              <img
                src={image}
                alt={ad.id}
                className='h-full w-full object-contain transition-transform duration-700 ease-out group-hover:scale-[1.02]'
              />
              <span className='absolute left-3 top-3 rounded-full bg-black/65 px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.18em] text-zinc-300 backdrop-blur'>
                {t('Sponsored')}
              </span>
            </div>
          </a>
          <div className='flex items-center justify-between gap-3 border-t border-white/5 px-4 py-2.5 sm:py-3'>
            <span className='max-w-[45%] truncate text-[11px] text-zinc-500'>
              {t('This ad supports the site')}
            </span>
            <Button
              disabled={remaining > 0}
              onClick={close}
              size='sm'
              type='button'
              variant={remaining > 0 ? 'secondary' : 'default'}
              className={
                remaining > 0
                  ? 'h-9 rounded-full border border-white/10 bg-zinc-800/80 px-4 text-zinc-300 hover:bg-zinc-800'
                  : 'h-9 rounded-full bg-zinc-100 px-4 text-zinc-900 hover:bg-white'
              }
            >
              {remaining > 0
                ? t('Skip ad in {{seconds}}s', { seconds: remaining })
                : t('Skip ad')}
              {remaining > 0 && (
                <span
                  aria-hidden
                  className='ml-2 inline-block h-1.5 w-1.5 animate-pulse rounded-full bg-amber-400'
                />
              )}
            </Button>
          </div>
        </div>
      </div>
    </div>
  )
}
