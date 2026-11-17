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

function loadVideoElement(src: string): Promise<HTMLVideoElement> {
  return new Promise((resolve, reject) => {
    const video = document.createElement('video')
    video.muted = true
    video.playsInline = true
    video.preload = 'auto'
    video.crossOrigin = 'anonymous'
    const timer = window.setTimeout(() => {
      cleanup()
      reject(new Error('Timeout loading part for merge'))
    }, 60000)
    const cleanup = () => {
      window.clearTimeout(timer)
      video.removeAttribute('src')
      video.load()
    }
    video.onerror = () => {
      cleanup()
      reject(new Error('Video loading error during merge'))
    }
    video.onloadedmetadata = () => {
      window.clearTimeout(timer)
      resolve(video)
    }
    video.src = playableMediaUrl(src)
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
  const first = await loadVideoElement(parts[0])
  const canvas = document.createElement('canvas')
  canvas.width = first.videoWidth || 1280
  canvas.height = first.videoHeight || 720
  const ctx = canvas.getContext('2d')
  if (!ctx) {
    first.removeAttribute('src')
    first.load()
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

  const stream = canvas.captureStream(30)
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

  let partDurations: number[] = []
  let durationSum = 0

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
  }

  try {
    // Measure durations up front for accurate progress reporting.
    const measurable: HTMLVideoElement[] = [first]
    for (let i = 1; i < parts.length; i++) {
      measurable.push(await loadVideoElement(parts[i]))
    }
    partDurations = measurable.map((video) => (Number.isFinite(video.duration) && video.duration > 0 ? video.duration : 10))
    durationSum = partDurations.reduce((sum, d) => sum + d, 0)

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
