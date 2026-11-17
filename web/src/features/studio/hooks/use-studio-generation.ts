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

import { toast } from 'sonner'
import { t } from 'i18next'

import {
  generateImages as submitImageGeneration,
  generateVideo as submitVideoGeneration,
  pollVideoTask,
  resolveSubmittedTaskIdWithLookup,
} from '../api'
import {
  optimizeImagePrompt,
  optimizeVideoPrompt,
  planVideoSequence,
  refinePlanForStep,
  type VideoPlan,
} from '../lib/video-plan'
import { mergeVideoParts, uploadMergedMedia, deleteMergedMedia } from '../lib/media-merge'
import { captureVideoFrames } from '../lib/video-frames'
import type { StudioItem } from '../types'

type ImageParams = {
  prompt: string
  model: string
  mode?: StudioItem['mode']
  group?: string
  size?: string
  quality?: string
  count?: number
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
  referenceImages?: string[]
  negativePrompt?: string
  videoExtension?: string
  /** Max duration (seconds) one clip may run — clamped to 2..30, defaults to 30. */
  chunkSeconds?: number
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
  videoExtension?: string
}

async function runVideoChunk(
  ctx: ChainContext,
  chunkPrompt: string,
  chunkStartFrame: string | undefined,
  chunkEndFrame: string | undefined,
  preferredDuration: string | number | undefined,
  preferredQuality: string | undefined,
  chunkIndex: number,
  totalChunks: number,
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
  let referenceImagesForChunk = chunkIndex === 0 ? ctx.referenceImages : undefined
  if (referenceImages && referenceImages.length > 0) {
    referenceImagesForChunk = referenceImages
  }

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
            image_url: chunkIndex === 0 ? ctx.imageUrl : undefined,
            start_frame: chunkStartFrame,
            end_frame: chunkEndFrame,
            reference_images: referenceImagesForChunk,
            negative_prompt: negativePrompt ?? ctx.negativePrompt,
            metadata: {
              start_frame: chunkStartFrame,
              end_frame: chunkEndFrame,
              reference_images: referenceImages ?? ctx.referenceImages,
              negative_prompt: negativePrompt ?? ctx.negativePrompt,
              video_extension: ctx.videoExtension,
              chunk_index: chunkIndex,
              total_chunks: totalChunks,
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
            // The upstream reports a raw percent string ("12%"); parse it
            // before doing any arithmetic (raw strings produce NaN).
            const chunkWeight = 100 / totalChunks
            const parsed = Number.parseFloat(String(progress))
            const clamped = Number.isFinite(parsed)
              ? Math.min(Math.max(parsed, 0), 100)
              : 0
            const overallProgress = Math.round((chunkIndex * chunkWeight) + ((clamped / 100) * chunkWeight))
            patchItem(itemId, { progress: `${Math.min(Math.max(overallProgress, 0), 99)}%` })
          },
          0
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

type VideoChainConfig = {
  id: string
  baseSettings: Record<string, string | number | undefined>
  model: string
  group?: string
  ratio?: string
  quality?: string
  prompt: string
  negativePrompt?: string
  imageUrl?: string
  /** Reference images carried into every step (aepi reference_images). */
  userReferenceImages: string[]
  startFrame?: string
  endFrame?: string
  videoExtension?: string
  totalScenes: number
  chunkSeconds: number
  /** A plan restored from a persisted item (page-reload resume); created when absent. */
  plan?: VideoPlan
  /** Clips completed so far, in order; mutated in place so an interrupted chain can resume. */
  accumulatedUrls: string[]
  controller: AbortController
}

async function runVideoChain(
  config: VideoChainConfig,
  patchItem: (id: string, patch: Partial<StudioItem>) => void
): Promise<string[]> {
  const { controller } = config
  const totalScenes = Math.max(1, config.totalScenes)
  const chunkDurationSec = Math.max(2, Math.min(30, Math.floor(config.chunkSeconds) || 30))
  const totalSeconds = totalScenes * chunkDurationSec
  const accumulatedUrls = config.accumulatedUrls
  const startAt = Math.max(0, Math.min(accumulatedUrls.length, totalScenes))

  const planned =
    config.plan ??
    (await planVideoSequence(
      {
        prompt: config.prompt,
        totalSeconds,
        chunkSeconds: chunkDurationSec,
        ratio: config.ratio,
        quality: config.quality,
        negativePrompt: config.negativePrompt,
        userReferenceImages: config.userReferenceImages.length ? config.userReferenceImages : undefined,
      },
      totalScenes,
      controller.signal
    ))
  let workingPlan = planned

  const writeStep = (step: number, progress?: string, title = workingPlan.title) =>
    patchItem(config.id, {
      ...(progress ? { progress } : {}),
      step: { current: step, total: totalScenes, title, plan: workingPlan },
    })

  patchItem(config.id, {
    settings: { ...config.baseSettings },
    step: { current: Math.min(startAt + 1, totalScenes), total: totalScenes, title: workingPlan.title, plan: workingPlan },
  })

  // Re-hydrate reference material from the clips persisted so far. Frames are
  // always derivable from the stored URLs, so a reload loses nothing.
  const accumulatedFrames: string[] = []
  let currentStartFrame = config.startFrame
  const resumeFrom = accumulatedUrls.at(-1)
  if (startAt > 0 && resumeFrom) {
    try {
      const captured = await captureVideoFrames(resumeFrom, [0.2, 0.5, 0.8], 0.85)
      accumulatedFrames.push(...captured.frames)
      currentStartFrame = captured.lastFrame
    } catch {
      currentStartFrame = resumeFrom
    }
  }

  for (let i = startAt; i < totalScenes; i++) {
    if (controller.signal.aborted) {
      patchItem(config.id, { status: 'error', error: 'Cancelled', urls: accumulatedUrls })
      return accumulatedUrls
    }
    const stepNumber = i + 1
    const isFinalChunk = i === totalScenes - 1
    writeStep(stepNumber, `${Math.round((i / totalScenes) * 100)}%`)

    let stepFrames: string[] = []
    const previousClipUrl = accumulatedUrls.at(-1)
    if (i > 0 && previousClipUrl) {
      try {
        const captured = await captureVideoFrames(previousClipUrl, [0.2, 0.5, 0.8], 0.85)
        stepFrames = captured.frames
        accumulatedFrames.push(...captured.frames)
        currentStartFrame = captured.lastFrame
      } catch {
        currentStartFrame = previousClipUrl
      }
    }

    const refined = await refinePlanForStep(
      {
        prompt: config.prompt,
        totalSeconds,
        chunkSeconds: chunkDurationSec,
        ratio: config.ratio,
        quality: config.quality,
        negativePrompt: config.negativePrompt,
        userReferenceImages: config.userReferenceImages.length ? config.userReferenceImages : undefined,
        plan: workingPlan,
        stepIndex: i,
        frames: stepFrames,
      },
      controller.signal
    )
    workingPlan = refined.plan
    writeStep(stepNumber, undefined, workingPlan.title)

    const chunkEndFrame = isFinalChunk ? config.endFrame : undefined
    const stepReferences = accumulatedFrames.slice(-8)
    const chunkRes = await runVideoChunk(
      {
        controller,
        itemId: config.id,
        patchItem,
        model: config.model,
        group: config.group,
        ratio: config.ratio,
        imageUrl: config.imageUrl,
        referenceImages: config.userReferenceImages,
        negativePrompt: config.negativePrompt,
        videoExtension: config.videoExtension,
      },
      refined.stepPrompt,
      currentStartFrame,
      chunkEndFrame,
      chunkDurationSec,
      config.quality,
      i,
      totalScenes,
      refined.negativePrompt || config.negativePrompt,
      stepReferences.length ? stepReferences : undefined
    )

    accumulatedUrls.push(chunkRes.url)
    if (chunkRes.url.startsWith('http')) {
      void api.post('/api/media/cache', { url: chunkRes.url }).catch(() => {})
    }
    patchItem(config.id, {
      urls: [...accumulatedUrls],
      step: { current: stepNumber, total: totalScenes, title: workingPlan.title, plan: workingPlan },
    })
  }

  patchItem(config.id, {
    status: 'ready',
    urls: accumulatedUrls,
    progress: '100%',
    step: { current: totalScenes, total: totalScenes, title: workingPlan.title, plan: workingPlan },
  })
  return accumulatedUrls
}

async function optimizeStudioPrompt(
  prompt: string,
  mode: StudioItem['mode'],
  signal: AbortSignal
): Promise<string> {
  const trimmed = prompt.trim()
  if (!trimmed) return prompt
  if (mode === 'image') {
    return optimizeImagePrompt(trimmed, signal)
  }
  return optimizeVideoPrompt(trimmed, signal)
}

function errorMessageOf(error: unknown): string {
  const { errorMessage } = parseRequestErrorDetails(error)
  return errorMessage || 'The generation failed'
}

const STORAGE_KEY = 'better_studio_generations'

export function useStudioGeneration() {
  const [items, setItems] = useState<StudioItem[]>(() => {
    try {
      const saved = localStorage.getItem(STORAGE_KEY)
      return saved ? JSON.parse(saved) : []
    } catch {
      return []
    }
  })
  const [isGenerating, setIsGenerating] = useState(false)
  const [mergeJobs, setMergeJobs] = useState<Record<string, number>>({})
  const abortRef = useRef<AbortController | null>(null)
  const mergingRef = useRef<Set<string>>(new Set())
  const controllersRef = useRef<Map<string, AbortController>>(new Map())
  const activePollingTasks = useRef<Set<string>>(new Set())
  const hasMountedResumeRef = useRef(false)

  // Debounced persistence to avoid lag on frequent progress updates
  useEffect(() => {
    const timer = setTimeout(() => {
      try {
        localStorage.setItem(STORAGE_KEY, JSON.stringify(items))
      } catch {
        // ignore storage error
      }
    }, 500)
    return () => clearTimeout(timer)
  }, [items])

  const patchItem = useCallback((id: string, patch: Partial<StudioItem>) => {
    setItems((prev) =>
      prev.map((item) => (item.id === id ? { ...item, ...patch } : item))
    )
  }, [])

  // Stitch every part of a finished multi-part chain into one file in the
  // background so the user gets a single video instead of N separate parts.
  const autoMergeItem = useCallback(
    (id: string, urls: string[], opts?: { autoDownload?: boolean }) => {
      const parts = urls.filter((url) => typeof url === 'string' && url)
      if (parts.length < 2) return
      if (mergingRef.current.has(id)) return
      mergingRef.current.add(id)
      setMergeJobs((prev) => ({ ...prev, [id]: 0 }))
      void (async () => {
        try {
          const blob = await mergeVideoParts(parts, {
            onProgress: (progress) => {
              setMergeJobs((prev) => ({ ...prev, [id]: progress }))
            },
          })
          const url = await uploadMergedMedia(blob)
          patchItem(id, { urls: [url, ...parts], merged: true })
          if (opts?.autoDownload) {
            const anchor = document.createElement('a')
            anchor.href = url
            anchor.download = 'studio-video.webm'
            anchor.rel = 'noreferrer'
            document.body.appendChild(anchor)
            anchor.click()
            anchor.remove()
          }
        } catch {
          toast.error(t('Video parts could not be merged into one file'))
        } finally {
          mergingRef.current.delete(id)
          setMergeJobs((prev) => {
            const next = { ...prev }
            delete next[id]
            return next
          })
        }
      })()
    },
    [patchItem]
  )

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
      activePollingTasks.current.add(item.id)

      const controller = new AbortController()
      controllersRef.current.set(item.id, controller)

      // Multi-part chained generations resume from the persisted plan. A plain
      // re-poll of the in-flight task is not enough: the remaining parts were
      // never submitted, so the interrupted sequence is completed here instead.
      const resumeChain = item.step?.plan && (item.step?.total ?? 1) > 1
      if (resumeChain) {
        void (async () => {
          try {
            const plan = item.step?.plan
            const settings = (item.settings ?? {}) as Record<string, string | number | undefined>
            const model = typeof settings.model === 'string' ? settings.model : ''
            const baseSettings: Record<string, string | number | undefined> = {
              model: settings.model,
              duration: settings.duration,
              ratio: settings.ratio,
              quality: settings.quality,
            }
            const persistedUrls = Array.isArray(item.urls)
              ? item.urls.filter((url) => typeof url === 'string' && url)
              : []
            const totalScenes = Math.max(1, plan?.steps?.length ?? item.step?.total ?? persistedUrls.length + 1)
            const totalDurationMatch = /\((\d+)x(\d+)s\)/.exec(String(settings.duration ?? ''))
            const chunkSeconds = totalDurationMatch ? Number(totalDurationMatch[2]) : 30
            const negativePrompt = plan?.negative_prompt?.trim() || undefined
            const accumulatedUrls = [...persistedUrls]

            // If a task was in flight when the page reloaded, join it first so
            // its part is not submitted twice; otherwise continue from the plan.
            if (item.taskId && accumulatedUrls.length < totalScenes) {
              try {
                const inFlightProgress = item.progress ? Number.parseFloat(String(item.progress)) : 0
                const finished = await pollVideoTask(
                  item.taskId,
                  controller.signal,
                  undefined,
                  (progress) => {
                    const chunkWeight = 100 / totalScenes
                    const parsed = Number.parseFloat(String(progress))
                    const clamped = Number.isFinite(parsed)
                      ? Math.min(Math.max(parsed, 0), 100)
                      : 0
                    const overall = Math.round(accumulatedUrls.length * chunkWeight + (clamped / 100) * chunkWeight)
                    patchItem(item.id, { progress: `${Math.min(Math.max(overall, 0), 99)}%` })
                  },
                  Number.isFinite(inFlightProgress) ? inFlightProgress : 0
                )
                if (finished.result_url && !accumulatedUrls.includes(finished.result_url)) {
                  accumulatedUrls.push(finished.result_url)
                  patchItem(item.id, { urls: [...accumulatedUrls] })
                  if (finished.result_url.startsWith('http')) {
                    void api.post('/api/media/cache', { url: finished.result_url }).catch(() => {})
                  }
                }
              } catch {
                // The in-flight task was lost upstream; the plan continues from the completed clips.
              }
            }

            const resumedUrls = await runVideoChain(
              {
                id: item.id,
                baseSettings,
                model,
                group: undefined,
                ratio: typeof settings.ratio === 'string' ? settings.ratio : undefined,
                quality: typeof settings.quality === 'string' ? settings.quality : undefined,
                prompt: item.prompt,
                negativePrompt,
                imageUrl: undefined,
                userReferenceImages: [],
                startFrame: undefined,
                endFrame: undefined,
                videoExtension: undefined,
                totalScenes,
                chunkSeconds,
                plan,
                accumulatedUrls,
                controller,
              },
              patchItem
            )
            autoMergeItem(item.id, resumedUrls)
          } catch (error) {
            if (controller.signal.aborted) {
              patchItem(item.id, { status: 'error', error: 'Cancelled' })
            } else {
              const kept = Array.isArray(item.urls)
                ? item.urls.filter((url) => typeof url === 'string' && url)
                : []
              const message = errorMessageOf(error)
              if (kept.length > 0) {
                patchItem(item.id, {
                  status: 'error',
                  error: `${message} The ${kept.length} completed part(s) are kept in this item.`,
                  urls: kept,
                })
              } else {
                patchItem(item.id, { status: 'error', error: message })
              }
            }
          } finally {
            activePollingTasks.current.delete(item.id)
            controllersRef.current.delete(item.id)
            setIsGenerating(false)
          }
        })()
        return
      }

      const resumePolling = async () => {
        try {
          let currentTaskId = item.taskId
          if (!currentTaskId) {
            for (let attempt = 0; attempt < 6 && !currentTaskId; attempt++) {
              if (controller.signal.aborted) return
              const foundTask = await findLatestVideoTask({
                model: item.settings?.model as string,
                sinceMs: item.createdAt,
                signal: controller.signal,
              })
              if (foundTask?.task_id) {
                currentTaskId = foundTask.task_id
                patchItem(item.id, { taskId: currentTaskId })
                break
              }
              await new Promise((resolve) => setTimeout(resolve, 2000))
            }
          }

          if (!currentTaskId) {
            if (Date.now() - item.createdAt > 3 * 60 * 1000) {
              patchItem(item.id, {
                status: 'error',
                error: 'The video task could not be found',
              })
            }
            return
          }

          const initialProg = item.progress ? Number.parseFloat(item.progress) : 0
          const finished = await pollVideoTask(
            currentTaskId,
            controller.signal,
            undefined,
            (progress) => patchItem(item.id, { progress }),
            initialProg
          )

          if (finished.result_url) {
            const previousUrls = Array.isArray(item.urls)
              ? item.urls.filter((url) => typeof url === 'string' && url)
              : []
            const mergedUrls = previousUrls.includes(finished.result_url)
              ? previousUrls
              : [...previousUrls, finished.result_url]
            patchItem(item.id, { status: 'ready', urls: mergedUrls })
            if (finished.result_url.startsWith('http')) {
              void api.post('/api/media/cache', { url: finished.result_url }).catch(() => {})
            }
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
  }, [items, patchItem, autoMergeItem])

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
      })
      onStarted?.(id)
      const controller = new AbortController()
      abortRef.current = controller
      controllersRef.current.set(id, controller)
      setIsGenerating(true)

      try {
        const finalPrompt = await optimizeStudioPrompt(
          params.prompt,
          params.mode || 'image',
          controller.signal
        )
        const response = await submitImageGeneration(
          {
            model: params.model,
            group: params.group,
            prompt: finalPrompt,
            n: params.count && params.count > 0 ? params.count : 1,
            size: params.size,
            quality: params.quality,
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
        urls.forEach((u) => {
          if (u.startsWith('http')) {
            void api.post('/api/media/cache', { url: u }).catch(() => {})
          }
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
      // Determine if chained multi-chunk extension is requested
      const targetTotalSeconds = params.videoExtension && params.videoExtension !== 'off'
        ? Number.parseInt(params.videoExtension, 10) || 0
        : 0
      const requestedChunkSeconds = Number(params.chunkSeconds)
      const chunkDurationSec = Number.isFinite(requestedChunkSeconds) && requestedChunkSeconds > 0
        ? Math.max(2, Math.min(30, Math.floor(requestedChunkSeconds)))
        : 30
      const totalChunks = targetTotalSeconds > chunkDurationSec
        ? Math.max(1, Math.ceil(targetTotalSeconds / chunkDurationSec))
        : 1

      const id = startItem({
        mode: params.mode || 'video',
        prompt: params.prompt,
        settings: {
          model: params.model,
          duration: targetTotalSeconds > 0 ? `${targetTotalSeconds}s (${totalChunks}x${chunkDurationSec}s)` : params.duration,
          ratio: params.ratio,
          quality: params.quality,
        },
      })
      onStarted?.(id)
      const controller = new AbortController()
      abortRef.current = controller
      controllersRef.current.set(id, controller)
      setIsGenerating(true)
      // Chunks of a multi-part extension that finished before a failure — kept
      // on the item so a broken chain never loses the completed parts.
      const accumulatedUrls: string[] = []

      try {
        const baseSettings: Record<string, string | number | undefined> = {
          model: params.model,
          duration: targetTotalSeconds > 0
            ? `${targetTotalSeconds}s (${totalChunks}x${chunkDurationSec}s)`
            : params.duration,
          ratio: params.ratio,
          quality: params.quality,
        }

        // If no multi-chunk extension is requested, run as a single planned generation
        if (targetTotalSeconds <= chunkDurationSec) {
          const finalPrompt = await optimizeVideoPrompt(params.prompt, controller.signal)
          const singleChunkDuration = params.duration || (targetTotalSeconds > 0 ? targetTotalSeconds : undefined)
          const result = await runVideoChunk(
            {
              controller,
              itemId: id,
              patchItem,
              model: params.model,
              group: params.group,
              ratio: params.ratio,
              imageUrl: params.imageUrl,
              referenceImages: params.referenceImages,
              negativePrompt: params.negativePrompt,
              videoExtension: params.videoExtension,
            },
            finalPrompt,
            params.startFrame,
            params.endFrame,
            singleChunkDuration,
            params.quality,
            0,
            1,
            params.negativePrompt,
            params.referenceImages
          )

          patchItem(id, { status: 'ready', urls: [result.url], progress: '100%' })
          if (result.url && result.url.startsWith('http')) {
            void api.post('/api/media/cache', { url: result.url }).catch(() => {})
          }
          return id
        }

        // Multi-chunk chained generation, planned as one continuous take.
        // A vision-capable prompt optimizer (mandatory) writes the global
        // visual bible + per-step directions as JSON, and every step is refined
        // against frames captured from the ACTUAL previous clip. The chain is
        // persisted on the item, so a page reload resumes it automatically.
        const userReferenceImages = [
          ...(params.referenceImages ?? []),
          ...(params.imageUrl ? [params.imageUrl] : []),
        ].filter(Boolean)

        const chainUrls = await runVideoChain(
          {
            id,
            baseSettings,
            model: params.model,
            group: params.group,
            ratio: params.ratio,
            quality: params.quality,
            prompt: params.prompt,
            negativePrompt: params.negativePrompt,
            imageUrl: params.imageUrl,
            userReferenceImages,
            startFrame: params.startFrame,
            endFrame: params.endFrame,
            videoExtension: params.videoExtension,
            totalScenes: totalChunks,
            chunkSeconds: chunkDurationSec,
            accumulatedUrls,
            controller,
          },
          patchItem
        )
        autoMergeItem(id, chainUrls)
        return id
      } catch (error) {
        if (controller.signal.aborted) {
          patchItem(id, { status: 'error', error: 'Cancelled' })
          return id
        }
        const completedParts = accumulatedUrls.length
        if (targetTotalSeconds > chunkDurationSec && completedParts > 0) {
          patchItem(id, {
            status: 'error',
            error: `${errorMessageOf(error)} The ${completedParts} completed part(s) are kept in this item.`,
            urls: accumulatedUrls,
          })
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
    [patchItem, startItem, autoMergeItem]
  )

  const stop = useCallback((id?: string) => {
    if (id) {
      controllersRef.current.get(id)?.abort()
      controllersRef.current.delete(id)
      activePollingTasks.current.delete(id)
      patchItem(id, { status: 'error', error: 'Cancelled' })
    } else {
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
    setItems((prev) => {
      const itemToDelete = prev.find((item) => item.id === id)
      if (itemToDelete?.urls?.length) {
        itemToDelete.urls.forEach((u) => {
          if (!u) return
          if (u.startsWith('/uploads/merged/')) {
            void deleteMergedMedia(u.slice('/uploads/merged/'.length))
            return
          }
          void api.delete('/api/media/cache', { params: { url: u } }).catch(() => {})
        })
      }
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
      })
      setItems((prev) => prev.filter((item) => item.mode !== mode))
    } else {
      controllersRef.current.forEach((ctrl) => ctrl.abort())
      controllersRef.current.clear()
      activePollingTasks.current.clear()
      setItems([])
    }
  }, [items])

  return {
    items,
    isGenerating,
    mergeJobs,
    generateImage,
    generateVideo,
    stop,
    removeItem,
    clearItems,
    mergeItemParts: autoMergeItem,
  }
}
