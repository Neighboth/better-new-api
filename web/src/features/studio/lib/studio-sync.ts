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

import type { StudioItem } from '../types'

/** Window event used to tell the studio history hook which local item URLs
 * were removed on the server (oldest-busting quota sweep); the hook drops the
 * matching cards from its local state. */
export const STUDIO_ITEMS_PURGED_EVENT = 'studio-items-purged'

/**
 * Pure helpers for the server-side studio item sync. Kept free of React and
 * network code so they can be unit tested directly.
 */

/** Strips the live-progress fields (progress, queuePosition) as well: the
 * server history does not need per-second updates and one PUT per poll tick
 * would hammer the sync endpoint while a generation runs. */
export function stripVolatileMediaFields(item: StudioItem): StudioItem {
  const clone: StudioItem = { ...item }
  delete clone.progress
  delete clone.queuePosition
  if (clone.details) {
    const details = { ...clone.details }
    if (typeof details.startFrame === 'string' && details.startFrame.startsWith('data:')) {
      details.startFrame = undefined
    }
    if (typeof details.endFrame === 'string' && details.endFrame.startsWith('data:')) {
      details.endFrame = undefined
    }
    if (Array.isArray(details.referenceImages)) {
      details.referenceImages = details.referenceImages.filter((url) => !url.startsWith('data:'))
      if (details.referenceImages.length === 0) {
        details.referenceImages = undefined
      }
    }
    clone.details = details
    if (Object.values(details).every((value) => value === undefined)) {
      clone.details = undefined
    }
  }
  return clone
}

/** Shape of one row returned by GET /api/studio/items. */
export type StudioItemRow = {
  item_id: string
  payload: unknown
  created_at?: number
  updated_at?: number
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

export function isStudioItem(value: unknown): value is StudioItem {
  if (!isRecord(value)) return false
  if (typeof value.id !== 'string' || value.id === '') return false
  if (typeof value.prompt !== 'string') return false
  if (typeof value.createdAt !== 'number') return false
  if (value.mode !== 'video' && value.mode !== 'image') return false
  if (value.status !== 'pending' && value.status !== 'ready' && value.status !== 'error') return false
  if (!Array.isArray(value.urls)) return false
  if (!isRecord(value.settings)) return false
  return true
}

/**
 * Merges the local working set with the server's rows. Union by id; the
 * server copy wins for the same id (it is the account-wide truth after the
 * first sync), so switching devices never shows an item the other device
 * already deleted.
 */
export function mergeStudioItems(local: StudioItem[], remoteRows: StudioItemRow[]): StudioItem[] {
  const byId = new Map<string, StudioItem>()
  for (const item of local) {
    if (item?.id) byId.set(item.id, item)
  }
  for (const row of remoteRows) {
    if (!isStudioItem(row.payload)) continue
    const remoteItem: StudioItem = row.payload
    // The server stores the payload with the client's own createdAt — the
    // server's row timestamps shadow it as the freshness signal.
    byId.set(remoteItem.id, remoteItem)
  }
  return [...byId.values()].sort((a, b) => (b.createdAt ?? 0) - (a.createdAt ?? 0))
}

/** Compares two items by stable content; used to skip no-op sync uploads. */
export function studioItemsEqual(a: StudioItem | undefined, b: StudioItem | undefined): boolean {
  if (!a || !b) return a === b
  return JSON.stringify(a) === JSON.stringify(b)
}
