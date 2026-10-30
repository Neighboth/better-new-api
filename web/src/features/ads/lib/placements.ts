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

/** The surfaces of the site where an ad may appear. */
export type AdSurface = 'blog' | 'console'

/** Every surface, in the order that the panel draws them. */
export const AD_SURFACES: AdSurface[] = ['blog', 'console']

/**
 * Reads the surfaces of one ad.
 *
 * The server writes the list of the surfaces into every ad. An ad of an older
 * panel carries no list and belongs to the blog, the surface that existed when
 * the ad was written: an upgrade never moves the ad of an administrator to a
 * page where nobody placed it.
 */
export function parsePlacements(value: unknown): AdSurface[] {
  if (!Array.isArray(value)) return ['blog']
  const surfaces = AD_SURFACES.filter((surface) => value.includes(surface))
  return surfaces.length > 0 ? surfaces : ['blog']
}

/**
 * Reads the surfaces of the installation from its comma separated option.
 *
 * An installation that never saved the option keeps the blog only, which is
 * what the ads of the installation did before the console had a row.
 */
export function parseSurfaces(raw: string | undefined): AdSurface[] {
  const listed = new Set(
    (raw ?? '').split(',').map((part) => part.trim().toLowerCase())
  )
  const surfaces = AD_SURFACES.filter((surface) => listed.has(surface))
  return surfaces.length > 0 ? surfaces : ['blog']
}

/** Writes the surfaces of the installation into the option of the settings. */
export function serializeSurfaces(surfaces: AdSurface[]): string {
  const ordered = AD_SURFACES.filter((surface) => surfaces.includes(surface))
  return (ordered.length > 0 ? ordered : ['blog']).join(',')
}
