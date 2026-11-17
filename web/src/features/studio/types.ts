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
export type StudioMode = 'video' | 'image'

export type StudioItemStatus = 'pending' | 'ready' | 'error'

/** Multi-part plan info for chained (extension) video generations. */
export type StudioItemStep = {
  current: number
  total: number
  title?: string
  /** The live optimizer plan; persisted so a page reload resumes the chain. */
  plan?: import('./lib/video-plan').VideoPlan
}

/** Generation details shown in the Previous Generations details view. */
export type StudioItemDetails = {
  /** Optimized prompt negative prompt handed to the engine, if any. */
  negativePrompt?: string
  /** Start frame used for the first (or only) segment, as URL or data URI. */
  startFrame?: string
  /** End frame for the last segment, when set. */
  endFrame?: string
  /** Character/style reference images passed to the engine. */
  referenceImages?: string[]
  /** Engine/model variant selected for the chain, if set. */
  engine?: string
  /** Sampling seed, when the provider exposes one. */
  seed?: number
  /** Seconds per segment for chained generations. */
  chunkSeconds?: number
}

export type StudioItem = {
  id: string
  mode: StudioMode
  prompt: string
  createdAt: number
  status: StudioItemStatus
  /** Image or video source. Local #/uploads paths play as-is; remote URLs are
   * proxied through /api/media/cache for playback. Empty when the video lives
   * in this browser's IndexedDB (localOnly). */
  urls: string[]
  error?: string
  /** Video task ID for asynchronous polling across sessions and page reloads. */
  taskId?: string
  /** While the task is queued at the upstream, its 1-based FIFO position. */
  queuePosition?: number
  /** Raw progress string of a video task ("0%", "42%", "100%"). */
  progress?: string
  /** Settings used for the generation, shown under the result. */
  settings: Record<string, string | number | undefined>
  /** Generation details for the details view (start/end frames, references,
   * negative prompt, engine, seed). */
  details?: StudioItemDetails
  /** True once the parts were stitched into a single file (single player, single download). */
  merged?: boolean
  /** True when the bytes live in this browser's IndexedDB (server storage full). */
  localOnly?: boolean
  /** Multi-part plan info for chained (extension) video generations. */
  step?: StudioItemStep
}
