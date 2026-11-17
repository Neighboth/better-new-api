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
 * Frame extraction for the multi-part video chain.
 *
 * Key frames of a finished clip are fed (a) as reference images to the next
 * generation job and (b) as vision input to the prompt optimizer when it
 * plans the next step — that keeps faces, wardrobe, vehicles, liveries,
 * environment, weather and lighting identical across every part, however
 * long the chain gets.
 */

export type CapturedFrames = {
  /** JPEG data URLs at ascending timestamps. */
  frames: string[]
  /** The frame closest to the very end — used as the next clip's start frame. */
  lastFrame: string
  /** Duration of the source video in seconds. */
  duration: number
}

export function frameCacheUrl(url: string): string {
  if (url.startsWith('data:') || url.startsWith('blob:') || url.startsWith('/api/')) {
    return url
  }
  return `/api/media/cache?url=${encodeURIComponent(url)}`
}

function loadVideo(cacheUrl: string): Promise<HTMLVideoElement> {
  return new Promise((resolve, reject) => {
    const video = document.createElement('video')
    // Only cross-origin sources need (and tolerate) an explicit CORS mode;
    // same-origin /api/ URLs are CORS-clean for canvas as-is and would fail
    // to load entirely when asked for CORS headers the proxy never sent.
    if (!cacheUrl.startsWith('/')) {
      video.crossOrigin = 'anonymous'
    }
    video.preload = 'auto'
    video.muted = true
    video.playsInline = true
    const timer = window.setTimeout(() => {
      cleanup()
      reject(new Error('Timeout loading video for frame capture'))
    }, 45000)
    const cleanup = () => {
      window.clearTimeout(timer)
      video.removeAttribute('src')
      video.load()
    }
    video.onerror = () => {
      cleanup()
      reject(new Error('Video loading error during frame extraction'))
    }
    video.onloadedmetadata = () => {
      window.clearTimeout(timer)
      resolve(video)
    }
    video.src = cacheUrl
  })
}

function seekTo(video: HTMLVideoElement, time: number): Promise<void> {
  return new Promise((resolve, reject) => {
    const timer = window.setTimeout(() => {
      video.removeEventListener('seeked', onSeeked)
      video.removeEventListener('error', onError)
      reject(new Error('Timeout seeking video for frame capture'))
    }, 15000)
    const onSeeked = () => {
      window.clearTimeout(timer)
      video.removeEventListener('error', onError)
      resolve()
    }
    const onError = () => {
      window.clearTimeout(timer)
      video.removeEventListener('seeked', onSeeked)
      reject(new Error('Error seeking video for frame capture'))
    }
    video.addEventListener('seeked', onSeeked, { once: true })
    video.addEventListener('error', onError, { once: true })
    video.currentTime = Math.min(Math.max(time, 0), Math.max(0, video.duration - 0.05))
  })
}

function drawFrame(video: HTMLVideoElement, quality = 0.88): string {
  const canvas = document.createElement('canvas')
  canvas.width = video.videoWidth || 1280
  canvas.height = video.videoHeight || 720
  const ctx = canvas.getContext('2d')
  if (!ctx) throw new Error('Canvas 2D context not available')
  ctx.drawImage(video, 0, 0, canvas.width, canvas.height)
  return canvas.toDataURL('image/jpeg', quality)
}

/**
 * Captures frames at the given fractions of the video duration. Always
 * includes a near-final frame (at 97% of the duration) for start-frame
 * chaining; the returned `lastFrame` is that near-final frame.
 */
export async function captureVideoFrames(
  videoUrl: string,
  fractions: number[] = [0.2, 0.5, 0.8],
  quality = 0.88
): Promise<CapturedFrames> {
  const cacheUrl = frameCacheUrl(videoUrl)
  const video = await loadVideo(cacheUrl)
  try {
    const duration = Number.isFinite(video.duration) && video.duration > 0 ? video.duration : 0
    const points = [...new Set([...fractions.map((f) => Math.min(Math.max(f, 0), 1)), 0.97])].sort(
      (a, b) => a - b
    )
    const frames: string[] = []
    for (const fraction of points) {
      try {
        await seekTo(video, fraction * duration)
        frames.push(drawFrame(video, quality))
      } catch {
        // Skip a frame that refuses to seek; the others still serve as reference.
      }
    }
    const lastFrame = frames.at(-1)
    if (!lastFrame) {
      throw new Error('Could not capture any frame from the video')
    }
    return { frames, lastFrame, duration }
  } finally {
    video.removeAttribute('src')
    video.load()
  }
}

/** Single near-final frame, for callers that only need the next start frame. */
export async function captureLastVideoFrame(videoUrl: string): Promise<string> {
  const captured = await captureVideoFrames(videoUrl, [0.97], 0.92)
  return captured.lastFrame
}
