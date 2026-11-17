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
 * Client-side merge of every part of a chained (video extension) generation
 * into a single video file.
 *
 * No re-encode is possible without either a server-side ffmpeg (not shipped
 * in this installation) or a heavy WASM dependency, so each part is played
 * once into a canvas while a MediaRecorder captures the result (with audio
 * when the browser can mix it). The merged blob is then uploaded through
 * POST /api/media/merged (see controller/media_merge.go) and persisted like
 * any other media on the item.
 */

import { api } from '@/lib/api'

export type MergeProgress = (progress: number) => void

export function getMergedUploadUrl(raw: { success?: boolean; url?: string } | null | undefined): string | null {
  const url = raw?.url
  return typeof url === 'string' && url ? url : null
}

function playableMediaUrl(url: string): string {
  if (url.startsWith('http://') || url.startsWith('https://') || url.startsWith('/api/') || url.startsWith('/uploads/')) {
    return url.startsWith('http://') || url.startsWith('https://')
      ? `/api/media/cache?url=${encodeURIComponent(url)}`
      : url
  }
  return url
}

function loadVideoElement(src: string, waitForCanPlayThrough = true): Promise<HTMLVideoElement> {
  return new Promise((resolve, reject) => {
    const video = document.createElement('video')
    video.muted = true
    video.playsInline = true
    video.preload = 'auto'
    video.disableRemotePlayback = true
    const resolvedSrc = playableMediaUrl(src)
    // Same-origin /api/ URLs are CORS-clean for canvas without credentials;
    // demanding CORS headers there would break playback entirely.
    if (!resolvedSrc.startsWith('/')) {
      video.crossOrigin = 'anonymous'
    }
    const timer = window.setTimeout(() => {
      cleanup()
      reject(new Error('Timeout loading part for merge'))
    }, 90000)
    // 'canplaythrough' guarantees the whole clip is buffered so no stall
    // happens mid-record (stalls were recorded as frozen frames and made the
    // merged file several seconds longer than the sum of its parts). When the
    // browser never fires it (heuristics), fall back to metadata + a short
    // settle window instead of aborting the merge.
    let settleTimer: number | undefined
    const cleanup = () => {
      window.clearTimeout(timer)
      if (settleTimer !== undefined) window.clearTimeout(settleTimer)
      video.removeAttribute('src')
      video.load()
    }
    const ready = () => {
      window.clearTimeout(timer)
      if (settleTimer !== undefined) window.clearTimeout(settleTimer)
      resolve(video)
    }
    video.onerror = () => {
      cleanup()
      reject(new Error('Video loading error during merge'))
    }
    if (waitForCanPlayThrough) {
      video.addEventListener('canplaythrough', ready, { once: true })
      video.addEventListener('loadedmetadata', () => {
        settleTimer = window.setTimeout(ready, 2500)
      }, { once: true })
    } else {
      video.onloadedmetadata = ready
    }
    video.src = resolvedSrc
  })
}

/**
 * Best-effort read of a clip's exact length before recording, so progress
 * and the final length check can be accurate. Uses preload='metadata' so the
 * audio/video bytes are never downloaded. Resolves with null when the
 * metadata cannot be read.
 */
export function measureVideoDuration(url: string): Promise<number | null> {
  return new Promise((resolve) => {
    const video = document.createElement('video')
    video.preload = 'metadata'
    video.muted = true
    video.playsInline = true
    const resolvedSrc = playableMediaUrl(url)
    if (!resolvedSrc.startsWith('/')) {
      video.crossOrigin = 'anonymous'
    }
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
    video.src = resolvedSrc
  })
}

function pickRecorderMimeType(): string {
  const candidates = [
    'video/mp4;codecs=avc1.64001f',
    'video/mp4;codecs=avc1',
    'video/webm;codecs=vp9',
    'video/webm;codecs=vp8',
    'video/webm',
  ]
  for (const candidate of candidates) {
    if (typeof MediaRecorder !== 'undefined' && MediaRecorder.isTypeSupported(candidate)) {
      return candidate
    }
  }
  return 'video/webm'
}

/**
 * Merges every part into one blob. Plays each part sequentially into a
 * canvas at 1x realtime while the recorder captures; resolves with the
 * merged blob. Pass `signal` to cancel.
 *
 * Every part is fully preloaded (canplaythrough) before the recorder starts
 * and the canvas emits frames only when the source produces one
 * (captureStream(0) + requestFrame), so the output length is the exact sum
 * of the parts — no frozen frames from mid-merge stalls.
 */
export async function mergeVideoParts(
  urls: string[],
  options: { onProgress?: MergeProgress; signal?: AbortSignal } = {}
): Promise<Blob> {
  const parts = urls.filter((u) => typeof u === 'string' && u.trim() !== '')
  if (parts.length === 0) {
    throw new Error('No parts to merge')
  }
  const { onProgress, signal } = options

  const measurable: HTMLVideoElement[] = []
  for (const url of parts) {
    if (signal?.aborted) {
      throw new DOMException('Aborted', 'AbortError')
    }
    measurable.push(await loadVideoElement(url))
  }
  const first = measurable[0]
  const canvas = document.createElement('canvas')
  canvas.width = first.videoWidth || 1280
  canvas.height = first.videoHeight || 720
  const ctx = canvas.getContext('2d')
  if (!ctx) {
    for (const video of measurable) {
      video.removeAttribute('src')
      video.load()
    }
    throw new Error('Canvas 2D context not available')
  }

  // Audio is best effort: without a user gesture the AudioContext may stay
  // suspended, and not every part even carries an audio track.
  let audioDestination: MediaStreamAudioDestinationNode | null = null
  let audioContext: AudioContext | null = null
  try {
    audioContext = new AudioContext()
    if (audioContext.state === 'suspended') {
      await audioContext.resume().catch(() => {})
    }
    if (audioContext.state === 'running') {
      audioDestination = audioContext.createMediaStreamDestination()
    }
  } catch {
    audioContext = null
  }

  const stream = canvas.captureStream(0)
  const videoTrack = stream.getVideoTracks()[0] as
    | (MediaStreamTrack & { requestFrame?: () => void })
    | undefined
  if (audioDestination) {
    for (const track of audioDestination.stream.getAudioTracks()) {
      stream.addTrack(track)
    }
  }

  const mimeType = pickRecorderMimeType()
  const chunks: BlobPart[] = []
  const recorder = new MediaRecorder(stream, { mimeType, videoBitsPerSecond: 8_000_000 })
  recorder.ondataavailable = (event) => {
    if (event.data && event.data.size > 0) {
      chunks.push(event.data)
    }
  }
  recorder.start(250)

  // Durations were read from the fully preloaded elements: they are exact,
  // unless the container reports none (then progress is an estimate).
  const partDurations = measurable.map((video) =>
    Number.isFinite(video.duration) && video.duration > 0 ? video.duration : 10
  )
  const durationSum = partDurations.reduce((sum, d) => sum + d, 0)

  const reportProgressFor = (video: HTMLVideoElement, index: number) => {
    if (!onProgress) return
    const total = durationSum > 0 ? durationSum : partDurations.reduce((sum, d) => sum + d, 0)
    if (!Number.isFinite(total) || total <= 0) return
    let completedBefore = 0
    for (let i = 0; i < index; i++) {
      completedBefore += partDurations[i] ?? 0
    }
    const current = Number.isFinite(video.currentTime) ? video.currentTime : 0
    onProgress(Math.min(99, Math.max(0, Math.round(((completedBefore + current) / total) * 100))))
  }

  const drawFrame = (video: HTMLVideoElement, index: number) => {
    ctx.drawImage(video, 0, 0, canvas.width, canvas.height)
    reportProgressFor(video, index)
    if (videoTrack && typeof videoTrack.requestFrame === 'function') {
      try {
        videoTrack.requestFrame()
      } catch {
        // no frame to emit yet
      }
    }
  }

  try {
    for (let index = 0; index < measurable.length; index++) {
      if (signal?.aborted) {
        throw new DOMException('Aborted', 'AbortError')
      }
      const video = measurable[index]
      if (audioContext && audioDestination) {
        try {
          const source = audioContext.createMediaElementSource(video)
          source.connect(audioDestination)
        } catch {
          // keep video only for this part
        }
      }
      await video.play()
      await new Promise<void>((resolve, reject) => {
        const onEnded = () => {
          window.clearTimeout(timeout)
          resolve()
        }
        const onError = () => {
          window.clearTimeout(timeout)
          reject(new Error('Error playing a video part during merge'))
        }
        const timeout = window.setTimeout(() => {
          video.removeEventListener('ended', onEnded)
          video.removeEventListener('error', onError)
          reject(new Error('Timeout while merging a video part'))
        }, 120000)
        video.addEventListener('ended', onEnded, { once: true })
        video.addEventListener('error', onError, { once: true })
        const tick = () => {
          if (video.ended || video.paused) return
          drawFrame(video, index)
          schedule()
        }
        const schedule = () => {
          if (typeof video.requestVideoFrameCallback === 'function') {
            video.requestVideoFrameCallback(() => tick())
          } else {
            window.requestAnimationFrame(tick)
          }
        }
        schedule()
      })
      video.pause()
      video.removeAttribute('src')
      video.load()
    }
  } finally {
    recorder.stop()
    await new Promise<void>((resolve) => {
      recorder.onstop = () => resolve()
    })
    if (audioContext) {
      void audioContext.close().catch(() => {})
    }
  }

  onProgress?.(100)
  return new Blob(chunks, { type: mimeType })
}

/**
 * Uploads a merged blob through POST /api/media/merged and returns the
 * public URL it is served from.
 */
export async function uploadMergedMedia(blob: Blob): Promise<string> {
  const form = new FormData()
  const extension = blob.type.includes('mp4') ? 'mp4' : 'webm'
  form.append('file', blob, `studio-merged.${extension}`)
  const res = await api.post('/api/media/merged', form, {
    skipErrorHandler: true,
    skipAuthRefresh: true,
  } as Record<string, unknown>)
  const url = getMergedUploadUrl(res?.data?.data ?? res?.data)
  if (!url) {
    throw new Error('The merged video could not be uploaded')
  }
  return url
}

export async function deleteMergedMedia(name: string): Promise<void> {
  try {
    await api.delete('/api/media/merged', { params: { name } })
  } catch {
    // best effort cleanup
  }
}
