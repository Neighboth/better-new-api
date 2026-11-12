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
import { cn } from '@/lib/utils'

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

  const skipSeconds = Math.max(0, Math.floor(data?.popup_skip_seconds ?? 0))
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
      className='fixed inset-0 z-[90] flex items-center justify-center bg-black/70 p-3 backdrop-blur-sm sm:p-6'
      role='dialog'
    >
      <div className='relative w-full max-w-4xl'>
        <div className='bg-background overflow-hidden rounded-xl border shadow-2xl'>
          <a
            href={ad.url}
            target='_blank'
            rel='noopener noreferrer sponsored'
            className='block'
            onClick={close}
          >
            <img
              src={image}
              alt={ad.id}
              className={cn(
                'w-full bg-muted object-contain',
                // A tall, very wide rectangle: the popup carries its own
                // tall artwork instead of a crop of the wide strip the
                // blog and the console draw.
                'aspect-[16/10] max-h-[70vh]'
              )}
            />
          </a>
          <div className='flex items-center justify-between gap-3 border-t px-3 py-2'>
            <span className='text-muted-foreground text-xs'>
              {t('Advertisement')}
            </span>
            <Button
              disabled={remaining > 0}
              onClick={close}
              size='sm'
              type='button'
              variant={remaining > 0 ? 'secondary' : 'default'}
            >
              {remaining > 0
                ? t('Skip ad in {{seconds}}s', { seconds: remaining })
                : t('Skip ad')}
            </Button>
          </div>
        </div>
      </div>
    </div>
  )
}
