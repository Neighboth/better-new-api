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

/** How many ads the console draws in its row. */
export const CONSOLE_AD_COUNT = 2

/**
 * Picks the ads of one visit of the console.
 *
 * The pick is random on purpose: a reader that opens the console twice does not
 * see the same two ads of a campaign of ten. The row never holds the same ad
 * twice, an ad without an image or a target is skipped (it would draw an empty
 * frame) and a row of a single ad is one ad rather than the same banner twice.
 */
export function pickConsoleAds(
  ads: ConsoleAd[],
  count: number = CONSOLE_AD_COUNT,
  random: () => number = Math.random
): ConsoleAd[] {
  const visible = ads.filter((ad) => ad.image && ad.url)
  if (count <= 0) return []
  if (visible.length <= count) return visible

  // A Fisher-Yates shuffle of the pool: every ad has the same chance of being
  // drawn on a visit and the order of the row varies as well.
  const pool = [...visible]
  for (let index = pool.length - 1; index > 0; index--) {
    const swap = Math.floor(random() * (index + 1))
    ;[pool[index], pool[swap]] = [pool[swap], pool[index]]
  }
  return pool.slice(0, count)
}
