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

/**
 * Client-side helpers for the studio's media lifecycle.
 *
 * The merge itself runs on the server (see controller/media_merge.go):
 * the parts are stitched into a single fast-start MP4 with a pure-Go remuxer,
 * no external tools required. The upload endpoint accepts a JSON { urls: [...] }
 * body — local same-origin /uploads/... paths and SSRF-protected remote URLs
 * are supported.
 */

import { api } from '@/lib/api'

export type MergeProgress = (progress: number) => void

export function getMergedUploadUrl(raw: { success?: boolean; url?: string } | null | undefined): string | null {
  const url = raw?.url
  return typeof url === 'string' && url ? url : null
}

/**
 * Server-side merge of every part of a chained generation into one
 * fast-start MP4. Reports indeterminate progress until the server answers;
 * resolves with the public URL of the merged file.
 */
export async function mergeVideoParts(
  urls: string[],
  options: { onProgress?: MergeProgress; signal?: AbortSignal } = {}
): Promise<string> {
  const parts = urls.filter((u) => typeof u === 'string' && u.trim() !== '')
  if (parts.length === 0) {
    throw new Error('No parts to merge')
  }
  const { onProgress, signal } = options

  // The merge is a single request; the client cannot see inside it. A crawl
  // keeps the bar alive without lying about the real state.
  let progress = 8
  onProgress?.(progress)
  const ticker = window.setInterval(() => {
    progress = Math.min(85, progress + 4)
    onProgress?.(progress)
  }, 700)

  try {
    const res = await api.post('/api/media/merged', { urls: parts }, {
      signal,
      skipErrorHandler: true,
      skipAuthRefresh: true,
    } as Record<string, unknown>)
    const url = getMergedUploadUrl(res?.data?.data ?? res?.data)
    if (!url) {
      throw new Error('The video parts could not be merged into one file')
    }
    onProgress?.(100)
    return url
  } finally {
    window.clearInterval(ticker)
  }
}

/** Measures a media URL's duration in seconds via a metadata-only probe. */
export function measureVideoDuration(url: string): Promise<number | null> {
  return new Promise((resolve) => {
    const video = document.createElement('video')
    video.preload = 'metadata'
    video.muted = true
    video.playsInline = true
    const resolved = url.startsWith('http')
      ? `/api/media/cache?url=${encodeURIComponent(url)}`
      : url
    const timer = window.setTimeout(() => {
      cleanup()
      resolve(null)
    }, 20000)
    const cleanup = () => {
      window.clearTimeout(timer)
      video.removeAttribute('src')
      video.load()
    }
    video.onerror = () => {
      cleanup()
      resolve(null)
    }
    video.onloadedmetadata = () => {
      const duration = Number.isFinite(video.duration) && video.duration > 0 ? video.duration : null
      cleanup()
      resolve(duration)
    }
    video.src = resolved
  })
}

export async function deleteMergedMedia(name: string): Promise<void> {
  try {
    await api.delete('/api/media/merged', { params: { name } })
  } catch {
    // best effort cleanup
  }
}
