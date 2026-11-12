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

/**
 * The image the reader of one language sees.
 *
 * The administrator uploads one image per language (eight of them) and the ad
 * falls back to English, then to any image of the campaign: a banner of a
 * language that was never uploaded shows the campaign rather than an empty
 * frame.
 */
export function resolveAdImage(
  ad: Pick<ConsoleAd, 'image' | 'images'>,
  language: string
): string {
  const current = (language || 'en').toLowerCase().replace('_', '-')
  const langKey = current.startsWith('tr')
    ? 'tr'
    : current.startsWith('zh-tw')
      ? 'zh_TW'
      : current.startsWith('zh')
        ? 'zh_CN'
        : current.slice(0, 2)
  const images = ad.images ?? {}
  const anyImage = Object.values(images).find(
    (value) => value && value.trim() !== ''
  )
  return (
    images[langKey] ||
    images[current] ||
    images['en'] ||
    ad.image ||
    anyImage ||
    ''
  )
}
