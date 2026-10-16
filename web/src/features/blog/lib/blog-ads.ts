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

export type CustomAdSlot = { id: string; image: string; url: string }

export type AdSlot =
  | { kind: 'adsense' }
  | { kind: 'custom'; ad: CustomAdSlot }

const ADSENSE: AdSlot = { kind: 'adsense' }

function custom(ad: CustomAdSlot): AdSlot {
  return { kind: 'custom', ad }
}

/**
 * Picks the ad slots for a blog post page. Four slots exist (2 above the
 * content, 2 below). Rules, per the product spec:
 * - 1 custom ad + AdSense   -> 3 AdSense, 1 custom
 * - 2 custom ads + AdSense  -> 1 custom + 1 AdSense per row
 * - 2 custom ads, no AdSense -> top: ad1, ad2; bottom: ad1, ad2
 * - 4 custom ads            -> each shown exactly once
 * - 4 custom ads + AdSense  -> 2 custom + 2 AdSense
 * Generally: repeat the same source as little as possible while keeping
 * every slot filled. More than four custom ads rotate round-robin in pairs.
 */
export function pickBlogAdSlots(
  customAds: CustomAdSlot[],
  adsenseAvailable: boolean,
  mode: string
): AdSlot[] {
  const ads = mode === 'adsense' ? [] : customAds.filter((a) => a.image && a.url)
  const useAdsense = mode !== 'custom' && adsenseAvailable

  if (ads.length === 0) {
    return useAdsense ? [ADSENSE, ADSENSE, ADSENSE, ADSENSE] : []
  }
  if (ads.length === 1) {
    return useAdsense
      ? [custom(ads[0]), ADSENSE, ADSENSE, ADSENSE]
      : [custom(ads[0]), custom(ads[0]), custom(ads[0]), custom(ads[0])]
  }
  if (ads.length === 2) {
    return useAdsense
      ? [custom(ads[0]), ADSENSE, custom(ads[1]), ADSENSE]
      : [custom(ads[0]), custom(ads[1]), custom(ads[0]), custom(ads[1])]
  }
  if (ads.length === 3) {
    return useAdsense
      ? [custom(ads[0]), ADSENSE, custom(ads[1]), custom(ads[2])]
      : [custom(ads[0]), custom(ads[1]), custom(ads[2]), custom(ads[0])]
  }
  // 4+ custom ads: each of the first four appears exactly once; with AdSense
  // enabled two custom ads share the page with two AdSense slots.
  if (useAdsense) {
    return [custom(ads[0]), ADSENSE, custom(ads[1]), ADSENSE]
  }
  return [custom(ads[0]), custom(ads[1]), custom(ads[2]), custom(ads[3])]
}
