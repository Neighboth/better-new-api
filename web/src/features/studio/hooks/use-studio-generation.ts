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
import { nanoid } from 'nanoid'
import { useCallback, useEffect, useRef, useState } from 'react'

import { toGeneratedImageAttachments } from '@/features/playground/lib/message/image-message-utils'
import { parseRequestErrorDetails } from '@/features/playground/lib/streaming/request-error-utils'
import { api } from '@/lib/api'
import { findLatestVideoTask } from '@/features/playground/api'

import {
  generateImages as submitImageGeneration,
  generateVideo as submitVideoGeneration,
  pollVideoTask,
  resolveSubmittedTaskIdWithLookup,
} from '../api'
import { mergeStudioItems, isStudioItem, stripVolatileMediaFields, STUDIO_ITEMS_PURGED_EVENT } from '../lib/studio-sync'
import { putStudioVideo, deleteStudioVideo } from '../lib/studio-indexeddb'
import type { StudioItem } from '../types'

/** Streams a remote or same-origin media URL into this browser's IndexedDB. */
async function importVideoToIndexedDB(itemId: string, url: string): Promise<boolean> {
  try {
    if (!url) return false
    let target = url
    if (!url.startsWith('/') && url.startsWith('http')) {
      target = `/api/media/cache?nostore=1&url=${encodeURIComponent(url)}`
    }
    const response = await fetch(target)
    if (!response.ok) return false
    const blob = await response.blob()
    if (blob.size === 0) return false
    await putStudioVideo(itemId, blob)
    // Free the server-side copy so the user's quota is not consumed.
    if (url.startsWith('/uploads/')) {
      try {
        await api.delete('/api/media/file', { params: { url } })
      } catch {
        // best effort — the file remains on the server but playback still works from IndexedDB
      }
    }
    return true
  } catch {
    return false
  }
}

/** Downloads a video into IndexedDB when the user's server-side storage is
 * full (or the server copy is gone), so new generations survive even though
 * they will not be stored on the server. */
async function storageStatsFull(): Promise<boolean> {
  try {
    const res = await api.get('/api/media/storage', { skipErrorHandler: true } as Record<string, unknown>)
    const used = Number(res?.data?.data?.used_bytes ?? 0)
    const limit = Number(res?.data?.data?.limit_bytes ?? 0)
    return limit > 0 && used >= limit
  } catch {
    return false
  }
}

/** When the user is over the storage cap, import the finished video into this
 * browser's IndexedDB and drop the server-side reference. Returns true when
 * the item now lives in IndexedDB. */
async function maybePersistLocalOnly(itemId: string, urls: string[]): Promise<string[] | null> {
  if (urls.length === 0) return null
  if (!(await storageStatsFull())) return null
  const target = urls[0]
  if (typeof target !== 'string' || !target) return null
  const ok = await importVideoToIndexedDB(itemId, target)
  return ok ? [] : null
}

type ImageParams = {
  prompt: string
  model: string
  mode?: StudioItem['mode']
  group?: string
  size?: string
  quality?: string
  count?: number
  /** Reference images attached by the user — they anchor the prompt rewrite. */
  referenceImages?: string[]
  /** Effective cap for reference images (0 = disabled). */
  referenceCap?: number
  /** Extra metadata forwarded to the relay (studio_optimize, optimizer_model…). */
  metadata?: Record<string, unknown>
}

type VideoParams = {
  prompt: string
  model: string
  mode?: StudioItem['mode']
  group?: string
  duration?: number
  ratio?: string
  quality?: string
  imageUrl?: string
  startFrame?: string
  endFrame?: string
  /** Image reference images (uploads) — capped by the model's rules. */
  referenceImages?: string[]
  /** Audio reference clips — appended to the job's reference list after images. */
  audioReferences?: string[]
  /** Video reference clips — appended after image and audio references. */
  videoReferences?: string[]
  /** Effective cap for image references from the model's option rules (0 = disabled). */
  referenceCap?: number
  negativePrompt?: string
  /** Extra metadata forwarded to the upstream adapter (audio flag, edit operation…). */
  metadata?: Record<string, unknown>
}

type ChainContext = {
  controller: AbortController
  itemId: string
  patchItem: (id: string, patch: Partial<StudioItem>) => void
  model: string
  group?: string
  ratio?: string
  imageUrl?: string
  referenceImages?: string[]
  negativePrompt?: string
  metadata?: Record<string, unknown>
}

async function runVideoChunk(
  ctx: ChainContext,
  chunkPrompt: string,
  chunkStartFrame: string | undefined,
  chunkEndFrame: string | undefined,
  preferredDuration: string | number | undefined,
  preferredQuality: string | undefined,
  negativePrompt?: string,
  referenceImages?: string[]
): Promise<{ url: string; duration: number; quality: string }> {
  const { controller, itemId, patchItem } = ctx
  // Fallback plan: preferred -> smaller supported durations; 1080p -> 720p
  const durationAttempts: number[] = []
  const preferredNum = Number(preferredDuration)
  if (Number.isFinite(preferredNum) && preferredNum > 0) {
    durationAttempts.push(Math.floor(preferredNum))
  }
  for (const candidate of [30, 20, 10]) {
    if (!durationAttempts.includes(candidate) && (preferredDuration == null || candidate < Number(preferredDuration))) {
      durationAttempts.push(candidate)
    }
  }

  const qualityAttempts: string[] = []
  if (preferredQuality) qualityAttempts.push(preferredQuality)
  if (!qualityAttempts.includes('1080p')) qualityAttempts.push('1080p')
  if (!qualityAttempts.includes('720p')) qualityAttempts.push('720p')

  let lastErr: unknown = null
  const referenceImagesForChunk = referenceImages ?? ctx.referenceImages

  for (const curQuality of qualityAttempts) {
    for (const curDuration of durationAttempts) {
      if (controller.signal.aborted) {
        throw new Error('Cancelled')
      }

      try {
        const submitted = await submitVideoGeneration(
          {
            model: ctx.model,
            group: ctx.group,
            prompt: chunkPrompt,
            duration: curDuration,
            ratio: ctx.ratio,
            quality: curQuality,
            image_url: ctx.imageUrl,
            start_frame: chunkStartFrame,
            end_frame: chunkEndFrame,
            reference_images: referenceImagesForChunk,
            negative_prompt: negativePrompt ?? ctx.negativePrompt,
            metadata: {
              ...ctx.metadata,
              start_frame: chunkStartFrame,
              end_frame: chunkEndFrame,
              reference_images: referenceImagesForChunk,
              negative_prompt: negativePrompt ?? ctx.negativePrompt,
            },
          },
          controller.signal
        )

        const submittedAt = Date.now()
        const taskId = await resolveSubmittedTaskIdWithLookup(submitted, {
          model: ctx.model,
          submittedAt,
          signal: controller.signal,
        })

        if (!taskId) {
          throw new Error('The video task has no id')
        }

        patchItem(itemId, { taskId })

        const finished = await pollVideoTask(
          taskId,
          controller.signal,
          (minCreatedAt) =>
            resolveSubmittedTaskIdWithLookup(submitted, {
              model: ctx.model,
              submittedAt,
              signal: controller.signal,
              minCreatedAt,
            }),
          (progress) => {
            const parsed = Number.parseFloat(String(progress))
            const clamped = Number.isFinite(parsed)
              ? Math.min(Math.max(parsed, 0), 99)
              : 0
            patchItem(itemId, { progress: `${clamped}%` })
          },
          0,
          (status, queuePosition) => {
            patchItem(itemId, { queuePosition: status === 'queued' ? queuePosition : undefined })
          }
        )

        if (finished.result_url) {
          return { url: finished.result_url, duration: curDuration, quality: curQuality }
        }

        throw new Error(finished.fail_reason || 'Video chunk failed')
      } catch (attemptErr) {
        if (controller.signal.aborted) throw attemptErr
        lastErr = attemptErr
        // Continue to next fallback attempt
      }
    }
  }

  throw lastErr || new Error('All duration and quality attempts failed for chunk')
}

function errorMessageOf(error: unknown): string {
  const { errorMessage } = parseRequestErrorDetails(error)
  return errorMessage || 'The generation failed'
}

const STORAGE_KEY = 'better_studio_generations'
const SYNCED_IDS_KEY = 'better_studio_synced_ids'

function loadSyncedIds(): Set<string> {
  try {
    const raw = localStorage.getItem(SYNCED_IDS_KEY)
    if (!raw) return new Set()
    const parsed = JSON.parse(raw)
    return Array.isArray(parsed) ? new Set(parsed.filter((id): id is string => typeof id === 'string')) : new Set()
  } catch {
    return new Set()
  }
}

function persistSyncedIds(ids: Set<string>) {
  try {
    localStorage.setItem(SYNCED_IDS_KEY, JSON.stringify([...ids]))
  } catch {
    // ignore storage error
  }
}

export function useStudioGeneration() {
  const [items, setItems] = useState<StudioItem[]>(() => {
    try {
      const saved = localStorage.getItem(STORAGE_KEY)
      return saved ? JSON.parse(saved) : []
    } catch {
      return []
    }
  })
  // Mirrors the items state so the stop() callback can read the freshest
  // taskId without being recreated on every progress tick.
  const itemsRef = useRef(items)
  useEffect(() => {
    itemsRef.current = items
  }, [items])
  const [isGenerating, setIsGenerating] = useState(false)
  const abortRef = useRef<AbortController | null>(null)
  const controllersRef = useRef<Map<string, AbortController>>(new Map())
  const activePollingTasks = useRef<Set<string>>(new Set())
  const hasMountedResumeRef = useRef(false)
  const syncedIdsRef = useRef<Set<string>>(new Set())

  // Seed the tombstone watermark from the previous session.
  useEffect(() => {
    syncedIdsRef.current = loadSyncedIds()
  }, [])

  // Debounced persistence to avoid lag on frequent progress updates. The
  // local copy keeps working offline; the server copy is the account-wide
  // truth so the history follows the user across browsers and devices.
  // Progress itself is pushed sparsely (at most once per 10 s per item) so
  // a second browser sees near-live progress instead of starting at 0 while
  // a fresh PUT per poll tick would hammer the endpoint.
  const lastSyncedRef = useRef<Map<string, string>>(new Map())
  const lastProgressPushRef = useRef<Map<string, number>>(new Map())
  useEffect(() => {
    const timer = setTimeout(() => {
      try {
        localStorage.setItem(STORAGE_KEY, JSON.stringify(items))
      } catch {
        // ignore storage error
      }
      const changed: StudioItem[] = []
      for (const item of items) {
        if (!item?.id) continue
        const payload = stripVolatileMediaFields(item)
        const serialized = JSON.stringify(payload)
        const baseChanged = lastSyncedRef.current.get(item.id) !== serialized
        const hasLiveProgress = item.status === 'pending' && typeof item.progress === 'string' && item.progress !== ''
        const progressDue =
          hasLiveProgress && Date.now() - (lastProgressPushRef.current.get(item.id) ?? 0) > 10_000
        if (!baseChanged && !progressDue) {
          continue
        }
        if (hasLiveProgress) {
          payload.progress = item.progress
          if (item.queuePosition !== undefined) {
            payload.queuePosition = item.queuePosition
          }
        }
        changed.push(payload)
        lastSyncedRef.current.set(item.id, serialized)
        if (hasLiveProgress) {
          lastProgressPushRef.current.set(item.id, Date.now())
        }
      }
      if (changed.length > 0) {
        const remaining = [...changed]
        while (remaining.length > 0) {
          const batch = remaining.splice(0, 50)
          for (const item of batch) {
            syncedIdsRef.current.add(item.id)
          }
          persistSyncedIds(syncedIdsRef.current)
          void api
            .put('/api/studio/items', { items: batch })
            .catch(() => {
              // Sync is best-effort; the next edit re-sends the payload.
            })
        }
      }
    }, 600)
    return () => clearTimeout(timer)
  }, [items])

  // On mount: pull the account-wide history and merge it with the local
  // working set. Union by id; the server copy wins when both exist.
  const hasFetchedRemoteRef = useRef(false)
  useEffect(() => {
    if (hasFetchedRemoteRef.current) return
    hasFetchedRemoteRef.current = true
    void (async () => {
      try {
        const res = await api.get('/api/studio/items', { skipErrorHandler: true } as Record<string, unknown>)
        const rows = (res?.data?.data?.items ?? res?.data?.items) as Array<{ item_id?: string; payload?: unknown }> | undefined
        const remote = Array.isArray(rows)
          ? rows
              .filter((row) => isStudioItem(row.payload))
              .map((row) => ({
                item_id: row.item_id ?? String((row.payload as StudioItem).id),
                payload: row.payload as StudioItem,
              }))
          : []
        const remoteIds = new Set(remote.map((row) => row.item_id))
        setItems((prev) => {
          // Local items that were synced before but are absent from the server
          // were deleted on another device — drop them here as well. Items
          // never synced stay local.
          const doomed = prev.filter((item) => syncedIdsRef.current.has(item.id) && !remoteIds.has(item.id))
          for (const item of doomed) {
            syncedIdsRef.current.delete(item.id)
          }
          for (const id of remoteIds) {
            syncedIdsRef.current.add(id)
          }
          persistSyncedIds(syncedIdsRef.current)
          const next = prev.filter((item) => !doomed.some((d) => d.id === item.id))
          return mergeStudioItems(next, remote)
        })
      } catch {
        // offline or endpoint unavailable — local history still works
      }
    })()
  }, [])

  // Server-side quota sweeps dispatch this event with the removed URLs —
  // drop the matching local cards so the popup is consistent immediately.
  useEffect(() => {
    const handler = (event: Event) => {
      const detail = (event as CustomEvent<{ urls?: string[] }>).detail
      const urls = Array.isArray(detail?.urls) ? detail.urls : []
      if (urls.length === 0) return
      const urlSet = new Set(urls)
      setItems((prev) => {
        const removed = prev.filter((item) => item.urls.some((u) => u && urlSet.has(u)))
        for (const item of removed) {
          syncedIdsRef.current.delete(item.id)
          lastSyncedRef.current.delete(item.id)
          if (item.localOnly) {
            void deleteStudioVideo(item.id).catch(() => {})
          }
        }
        if (removed.length > 0) {
          persistSyncedIds(syncedIdsRef.current)
        }
        return prev.filter((item) => !item.urls.some((u) => u && urlSet.has(u)))
      })
    }
    window.addEventListener(STUDIO_ITEMS_PURGED_EVENT, handler)
    return () => window.removeEventListener(STUDIO_ITEMS_PURGED_EVENT, handler)
  }, [])

  const patchItem = useCallback((id: string, patch: Partial<StudioItem>) => {
    setItems((prev) =>
      prev.map((item) => (item.id === id ? { ...item, ...patch } : item))
    )
  }, [])

  // Auto-resume background polling for any pending video items on mount once
  useEffect(() => {
    if (hasMountedResumeRef.current) return
    hasMountedResumeRef.current = true

    const pendingVideos = items.filter(
      (item) => item.status === 'pending' && item.mode === 'video'
    )

    if (pendingVideos.length > 0) {
      setIsGenerating(true)
    }

    pendingVideos.forEach((item) => {
      if (activePollingTasks.current.has(item.id)) return
      // Items started in this tab are already tracked in controllersRef —
      // their generation is in flight locally and a second poller would
      // fight the in-flight one over the progress value.
      if (controllersRef.current.has(item.id)) return
      activePollingTasks.current.add(item.id)
      // Stale items whose submit never reached the server would otherwise
      // poll /api/task/self for 45 minutes — give them a clear error
      // immediately instead.
      const itemAgeMs = Date.now() - (item.createdAt ?? Date.now())
      if (!item.taskId && itemAgeMs > 2 * 60 * 60 * 1000) {
        patchItem(item.id, { status: 'error', error: 'The generation was interrupted before the video task was created' })
        activePollingTasks.current.delete(item.id)
        return
      }

      const controller = new AbortController()
      controllersRef.current.set(item.id, controller)

      const resumePolling = async () => {
        try {
          let currentTaskId = item.taskId
          if (!currentTaskId) {
            // The task may take a long time to become visible — the network can
            // be down for hours (airplane mode) and the submit may have
            // finished upstream while the id never reached this item. Keep the
            // card pending and watch for the task row instead of failing fast:
            // after ~45 minutes of uninterrupted misses it still reports the
            // honest "could not be found" outcome.
            const lookupDeadline = Date.now() + 45 * 60 * 1000
            while (!controller.signal.aborted && !currentTaskId && Date.now() < lookupDeadline) {
              try {
                const foundTask = await findLatestVideoTask({
                  model: item.settings?.model as string,
                  sinceMs: item.createdAt,
                  signal: controller.signal,
                  minCreatedAt: Math.floor((item.createdAt ?? Date.now()) / 1000),
                })
                if (foundTask?.task_id) {
                  currentTaskId = foundTask.task_id
                  patchItem(item.id, { taskId: currentTaskId })
                  break
                }
              } catch {
                // network not ready; keep waiting
              }
              await new Promise((resolve) => setTimeout(resolve, 5000))
            }
          }

          if (!currentTaskId) {
            patchItem(item.id, {
              status: 'error',
              error: 'The video task could not be found',
            })
            return
          }

          const initialProg = item.progress ? Number.parseFloat(item.progress) : 0
          const finished = await pollVideoTask(
            currentTaskId,
            controller.signal,
            undefined,
            (progress) => patchItem(item.id, { progress }),
            initialProg,
            (status, queuePosition) => {
              patchItem(item.id, { queuePosition: status === 'queued' ? queuePosition : undefined })
            }
          )

          if (finished.result_url) {
            const previousUrls = Array.isArray(item.urls)
              ? item.urls.filter((url) => typeof url === 'string' && url)
              : []
            const mergedUrls = previousUrls.includes(finished.result_url)
              ? previousUrls
              : [...previousUrls, finished.result_url]
            patchItem(item.id, { status: 'ready', urls: mergedUrls })
          } else {
            const isStillRunning = ['in_progress', 'running', 'pending', 'queued'].includes(
              (finished.status || '').toLowerCase()
            )
            patchItem(item.id, {
              status: 'error',
              error:
                finished.fail_reason ||
                (isStillRunning
                  ? 'Video is still processing upstream. You can check it under Previous Generations.'
                  : 'The video could not be generated'),
            })
          }
        } catch {
          if (controller.signal.aborted) {
            patchItem(item.id, { status: 'error', error: 'Cancelled' })
            return
          }
        } finally {
          activePollingTasks.current.delete(item.id)
          controllersRef.current.delete(item.id)
          setIsGenerating(false)
        }
      }

      void resumePolling()
    })
  }, [items, patchItem])

  const startItem = useCallback(
    (item: Omit<StudioItem, 'id' | 'createdAt' | 'status' | 'urls'>) => {
      const id = nanoid()
      setItems((prev) => [
        { ...item, id, createdAt: Date.now(), status: 'pending', urls: [] },
        ...prev,
      ])
      return id
    },
    []
  )

  const generateImage = useCallback(
    async (params: ImageParams, onStarted?: (id: string) => void) => {
      const id = startItem({
        mode: params.mode || 'image',
        prompt: params.prompt,
        settings: {
          model: params.model,
          size: params.size,
          quality: params.quality,
          count: params.count,
        },
        details: {
          referenceImages: params.referenceImages?.length ? params.referenceImages : undefined,
        },
      })
      onStarted?.(id)
      const controller = new AbortController()
      abortRef.current = controller
      controllersRef.current.set(id, controller)
      setIsGenerating(true)

      try {
        // Server-side optimizer: the browser just calls the relay and waits.
        const referenceCap = Math.max(0, params.referenceCap ?? 5)
        const imageRefs = referenceCap === 0 ? [] : (params.referenceImages ?? [])
        const response = await submitImageGeneration(
          {
            model: params.model,
            group: params.group,
            prompt: params.prompt,
            n: params.count && params.count > 0 ? params.count : 1,
            size: params.size,
            quality: params.quality,
            metadata: {
              ...params.metadata,
              studio_optimize: true,
              studio_item_id: id,
              reference_images: imageRefs,
              reference_image_count: imageRefs.length,
            },
          },
          controller.signal
        )
        const attachments = toGeneratedImageAttachments(response, params.prompt)
        if (attachments.length === 0) {
          patchItem(id, {
            status: 'error',
            error: 'The model did not return an image',
          })
          return
        }
        const urls = attachments.map((attachment) => attachment.url).filter(Boolean)
        patchItem(id, {
          status: 'ready',
          urls,
        })
        return id
      } catch (error) {
        if (controller.signal.aborted) {
          patchItem(id, { status: 'error', error: 'Cancelled' })
          return id
        }
        patchItem(id, { status: 'error', error: errorMessageOf(error) })
        return id
      } finally {
        setIsGenerating(false)
        abortRef.current = null
        controllersRef.current.delete(id)
      }
    },
    [patchItem, startItem]
  )

  const generateVideo = useCallback(
    async (params: VideoParams, onStarted?: (id: string) => void) => {
      const id = startItem({
        mode: params.mode || 'video',
        prompt: params.prompt,
        settings: {
          model: params.model,
          duration: params.duration,
          ratio: params.ratio,
          quality: params.quality,
        },
        details: {
          negativePrompt: params.negativePrompt?.trim() || undefined,
          startFrame: params.startFrame,
          endFrame: params.endFrame,
          referenceImages: params.referenceImages?.length ? params.referenceImages : undefined,
        },
      })
      onStarted?.(id)
      const controller = new AbortController()
      abortRef.current = controller
      controllersRef.current.set(id, controller)
      setIsGenerating(true)

      try {
        // The prompt optimization, web/image search and the reference upload
        // wiring all happen server-side now — the browser only polls the
        // task. Cap the reference lists client-side too so we never POST
        // more than the model accepts; the server enforces the same caps.
        const referenceCap = Math.max(0, params.referenceCap ?? 5)
        const imageRefs = referenceCap === 0 ? [] : (params.referenceImages ?? [])
        const audioRefs = params.audioReferences ?? []
        const videoRefs = params.videoReferences ?? []
        const allReferences = [...imageRefs, ...audioRefs, ...videoRefs]
        const result = await runVideoChunk(
          {
            controller,
            itemId: id,
            patchItem,
            model: params.model,
            group: params.group,
            ratio: params.ratio,
            imageUrl: params.imageUrl,
            referenceImages: allReferences,
            negativePrompt: params.negativePrompt?.trim() || undefined,
            metadata: {
              ...params.metadata,
              studio_optimize: true,
              studio_item_id: id,
              reference_images: allReferences,
              reference_image_count: imageRefs.length,
              reference_audio_count: audioRefs.length,
              reference_video_count: videoRefs.length,
            },
          },
          params.prompt,
          params.startFrame,
          params.endFrame,
          params.duration,
          params.quality,
          params.negativePrompt?.trim() || undefined,
          allReferences
        )

        const localOnlyUrls = await maybePersistLocalOnly(id, [result.url])
        patchItem(id, {
          status: 'ready',
          urls: localOnlyUrls ?? [result.url],
          localOnly: localOnlyUrls !== null,
          progress: '100%',
        })
        return id
      } catch (error) {
        if (controller.signal.aborted) {
          patchItem(id, { status: 'error', error: 'Cancelled' })
          return id
        }
        patchItem(id, { status: 'error', error: errorMessageOf(error) })
        return id
      } finally {
        setIsGenerating(false)
        abortRef.current = null
        controllersRef.current.delete(id)
      }
    },
    [patchItem, startItem]
  )

  const stop = useCallback((id?: string) => {
    // Best-effort: ask the server to cancel the task if it never started.
    // The endpoint only allows QUEUED (never admitted) tasks — jobs already
    // running upstream return 409 and their result will still be stored,
    // which the caller sees as the final history item.
    const cancelTaskOnServer = (taskId?: string) => {
      if (!taskId) return
      void api.post(`/api/task/self/${encodeURIComponent(taskId)}/cancel`).catch(() => {})
    }
    if (id) {
      const item = itemsRef.current.find((entry) => entry.id === id)
      cancelTaskOnServer(item?.taskId)
      controllersRef.current.get(id)?.abort()
      controllersRef.current.delete(id)
      activePollingTasks.current.delete(id)
      patchItem(id, { status: 'error', error: 'Cancelled' })
    } else {
      for (const item of itemsRef.current) {
        cancelTaskOnServer(item.taskId)
      }
      abortRef.current?.abort()
      controllersRef.current.forEach((ctrl) => ctrl.abort())
      controllersRef.current.clear()
      activePollingTasks.current.clear()
    }
    setIsGenerating(false)
  }, [patchItem])

  const removeItem = useCallback((id: string) => {
    controllersRef.current.get(id)?.abort()
    controllersRef.current.delete(id)
    activePollingTasks.current.delete(id)
    setIsGenerating(false)
    lastSyncedRef.current.delete(id)
    syncedIdsRef.current.delete(id)
    persistSyncedIds(syncedIdsRef.current)
    setItems((prev) => {
      const itemToDelete = prev.find((item) => item.id === id)
      if (itemToDelete?.urls?.length) {
        itemToDelete.urls.forEach((u) => {
          if (!u) return
          // /uploads/merged/ is gone with the stitching feature; legacy merged
          // URLs still route through the same delete endpoint, which knows them.
          if (u.startsWith('/uploads/videos/') || u.startsWith('/uploads/merged/')) {
            void api.delete('/api/media/file', { params: { url: u } }).catch(() => {})
            return
          }
          if (itemToDelete.localOnly) {
            return // the IndexedDB blob is removed below
          }
          void api.delete('/api/media/cache', { params: { url: u } }).catch(() => {})
        })
      }
      if (itemToDelete?.localOnly) {
        void deleteStudioVideo(id).catch(() => {})
      }
      void api.delete(`/api/studio/items/${encodeURIComponent(id)}`).catch(() => {})
      return prev.filter((item) => item.id !== id)
    })
  }, [])

  const clearItems = useCallback((mode?: StudioItem['mode']) => {
    setIsGenerating(false)
    if (mode) {
      items.filter((i) => i.mode === mode).forEach((i) => {
        controllersRef.current.get(i.id)?.abort()
        controllersRef.current.delete(i.id)
        activePollingTasks.current.delete(i.id)
        lastSyncedRef.current.delete(i.id)
        syncedIdsRef.current.delete(i.id)
        persistSyncedIds(syncedIdsRef.current)
        if (i.localOnly) void deleteStudioVideo(i.id).catch(() => {})
        void api.delete(`/api/studio/items/${encodeURIComponent(i.id)}`).catch(() => {})
      })
      setItems((prev) => prev.filter((item) => item.mode !== mode))
    } else {
      controllersRef.current.forEach((ctrl) => ctrl.abort())
      controllersRef.current.clear()
      activePollingTasks.current.clear()
      lastSyncedRef.current.clear()
      items.forEach((i) => {
        syncedIdsRef.current.delete(i.id)
        if (i.localOnly) void deleteStudioVideo(i.id).catch(() => {})
        void api.delete(`/api/studio/items/${encodeURIComponent(i.id)}`).catch(() => {})
      })
      persistSyncedIds(syncedIdsRef.current)
      setItems([])
    }
  }, [items])

  return {
    items,
    isGenerating,
    generateImage,
    generateVideo,
    stop,
    removeItem,
    clearItems,
  }
}
