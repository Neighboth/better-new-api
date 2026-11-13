/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

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
 * The large banner of the popup surface. Opens one ad per page load, at
 * most once per five hours per browser. The image uses the site theme
 * (rounded card, contrasty CTA button) so it looks native on both light
 * and dark surfaces.
 */
export function PopupAd() {
  const { t, i18n } = useTranslation()
  const { data } = useQuery({
    queryKey: ['popup-ads'],
    queryFn: fetchPopupAds,
    staleTime: 60_000,
  })

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
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [data, adsKey])

  const skipSeconds = Math.max(0, Math.floor(ad?.popup_skip_seconds ?? 0))
  const [remaining, setRemaining] = useState(skipSeconds)

  useEffect(() => {
    if (!ad) return
    setRemaining(skipSeconds)
  }, [ad, skipSeconds])

  useEffect(() => {
    if (!ad || remaining <= 0) return
    const timer = window.setTimeout(
      () => setRemaining((value) => value - 1),
      1000
    )
    return () => window.clearTimeout(timer)
  }, [ad, remaining])

  const reported = useRef(false)
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
      className='bg-background/85 fixed inset-0 z-[90] flex items-center justify-center p-4 backdrop-blur-md sm:p-6'
      role='dialog'
    >
      <div className='relative w-full max-w-[480px] sm:max-w-2xl'>
        <div className='bg-card text-card-foreground overflow-hidden rounded-2xl border shadow-2xl ring-1 ring-black/5 dark:ring-white/10'>
          <a
            href={ad.url}
            target='_blank'
            rel='noopener noreferrer sponsored'
            className='group block'
            onClick={close}
          >
            <div className='flex aspect-[16/9] max-h-[60vh] items-center justify-center overflow-hidden bg-muted'>
              <img
                src={image}
                alt={ad.id}
                className='max-h-full max-w-full object-contain transition-transform duration-700 ease-out group-hover:scale-[1.015]'
              />
            </div>
          </a>
          <div className='flex items-center justify-between gap-3 border-t px-4 py-2.5 sm:py-3'>
            <a
              href={ad.url}
              target='_blank'
              rel='noopener noreferrer sponsored'
              onClick={close}
              className={cn(
                'group/cta inline-flex items-center gap-1.5 rounded-md px-3 py-1.5 text-xs font-semibold leading-none',
                'bg-primary/15 text-primary hover:bg-primary/25 transition-colors',
              )}
            >
              {t('Visit site')}
              <svg
                width='13'
                height='13'
                viewBox='0 0 24 24'
                fill='none'
                stroke='currentColor'
                strokeWidth='2'
                strokeLinecap='round'
                strokeLinejoin='round'
                aria-hidden='true'
                className='transition-transform duration-200 group-hover/cta:translate-x-0.5'
              >
                <path d='M7 17L17 7M17 7H9M17 7v8' />
              </svg>
            </a>
            <Button
              disabled={remaining > 0}
              onClick={close}
              size='sm'
              type='button'
              variant={remaining > 0 ? 'secondary' : 'default'}
              className={cn(
                'shrink-0 rounded-full px-4 text-xs font-medium',
                remaining > 0
                  ? 'bg-muted text-muted-foreground'
                  : 'bg-primary text-primary-foreground hover:bg-primary/90',
              )}
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
