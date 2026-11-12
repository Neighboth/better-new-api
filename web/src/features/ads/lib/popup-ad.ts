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
import type { ConsoleAd } from '../api'

/** The storage key of the moment the skip button was used. */
export const POPUP_AD_DISMISS_KEY = 'ad_popup_dismissed_until'

/** How long a skipped popup stays away, in milliseconds. */
export const POPUP_AD_COOLDOWN_MS = 5 * 60 * 60 * 1000

/**
 * Picks the ad of one visit.
 *
 * The administrator may place several campaigns on the popup surface, the
 * reader sees one of them: a popup that drew every ad of the campaign would
 * open a stack of banners over the page. An ad without an image (or without a
 * language that has one) is skipped, it would open an empty frame.
 */
export function pickPopupAd(
  ads: ConsoleAd[],
  random: () => number = Math.random
): ConsoleAd | null {
  const visible = ads.filter((ad) => ad.url && hasImage(ad))
  if (visible.length === 0) return null
  const index = Math.min(
    visible.length - 1,
    Math.max(0, Math.floor(random() * visible.length))
  )
  return visible[index]
}

function hasImage(ad: ConsoleAd): boolean {
  if (ad.image && ad.image.trim() !== '') return true
  if (!ad.images) return false
  return Object.values(ad.images).some((image) => image && image.trim() !== '')
}

/**
 * Reads the moment until which the popup stays closed.
 *
 * A browser that refuses localStorage (private windows, a blocked site data
 * setting) simply has no cooldown: the ad opens again on the next visit, which
 * is what a reader of such a browser expects from a site without storage.
 */
export function readPopupAdDismissedUntil(
  storage: Pick<Storage, 'getItem'> | null = safeStorage()
): number {
  if (!storage) return 0
  try {
    const raw = storage.getItem(POPUP_AD_DISMISS_KEY)
    const value = Number(raw)
    return Number.isFinite(value) && value > 0 ? value : 0
  } catch {
    return 0
  }
}

/** Remembers that the reader closed the popup, for the cooldown. */
export function writePopupAdDismissed(
  now: number = Date.now(),
  storage: Pick<Storage, 'setItem'> | null = safeStorage()
): number {
  const until = now + POPUP_AD_COOLDOWN_MS
  if (!storage) return until
  try {
    storage.setItem(POPUP_AD_DISMISS_KEY, String(until))
  } catch {
    /* a browser without storage keeps the banner closed for this visit only */
  }
  return until
}

/** Whether the popup may open now. */
export function popupAdAllowed(
  now: number = Date.now(),
  storage: Pick<Storage, 'getItem'> | null = safeStorage()
): boolean {
  return now >= readPopupAdDismissedUntil(storage)
}

function safeStorage(): Storage | null {
  try {
    return typeof window === 'undefined' ? null : window.localStorage
  } catch {
    return null
  }
}
