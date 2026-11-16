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
  getPlaygroundRuntimeConfig,
  pollVideoTask,
  resolveSubmittedTaskIdWithLookup,
} from '../api'
import type { StudioItem } from '../types'

type ImageParams = {
  prompt: string
  model: string
  mode?: StudioItem['mode']
  group?: string
  size?: string
  quality?: string
  count?: number
  optimize?: boolean
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
  optimize?: boolean
  startFrame?: string
  endFrame?: string
  referenceImages?: string[]
  negativePrompt?: string
  videoExtension?: string
}

async function optimizeStudioPrompt(
  prompt: string,
  mode: StudioItem['mode'],
  signal: AbortSignal
): Promise<string> {
  const trimmed = prompt.trim()
  if (!trimmed) return prompt
  try {
    const runtimeConfig = await getPlaygroundRuntimeConfig(signal)
    if (
      !runtimeConfig?.studio?.prompt_optimizer_enabled ||
      !runtimeConfig?.studio?.prompt_optimizer_model
    ) {
      return prompt
    }

    let defaultSys = runtimeConfig.studio.prompt_optimizer_system_prompt
    if (!defaultSys) {
      if (mode === 'image') {
        defaultSys =
          'You are a professional image prompt optimizer. Rewrite the prompt into a descriptive, high-quality English prompt with lighting, composition, visual details, and style that generative AI models accept without safety violations. CRITICAL REQUIREMENT: If the prompt contains any Turkish text or quotes intended for rendering, DO NOT translate them — keep all Turkish quotes intact. Output ONLY the optimized prompt text without any introductory text, explanation, or quotes.'
      } else {
        defaultSys =
          'You are a professional video generation prompt optimizer and safety enhancer. Rewrite the given prompt into safe, detailed, cinematic English that generative video AI models accept without content violations. Describe camera movement, lighting, visual details, and style cleanly and vividly. CRITICAL REQUIREMENT: If the prompt contains any dialogue, spoken quotes, or speech lines, DO NOT translate the spoken dialogue — keep all spoken words/quotes in Turkish exactly as written. Output ONLY the optimized prompt text without any introductory text, explanation, or quotes around the prompt.'
      }
    }

    const chatRes = await api.post(
      '/pg/chat/completions',
      {
        model: runtimeConfig.studio.prompt_optimizer_model,
        messages: [
          { role: 'system', content: defaultSys },
          { role: 'user', content: trimmed },
        ],
        temperature: 0.7,
      },
      {
        signal,
        skipErrorHandler: true,
        skipAuthRefresh: true,
      } as Record<string, unknown>
    )
    const optimized = chatRes.data?.choices?.[0]?.message?.content?.trim()
    return optimized || prompt
  } catch (err) {
    console.warn('Studio prompt optimizer skipped or failed:', err)
    return prompt
  }
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
  const abortRef = useRef<AbortController | null>(null)
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

          const initialProg = item.progress ? parseFloat(item.progress) : 0
          const finished = await pollVideoTask(
            currentTaskId,
            controller.signal,
            undefined,
            (progress) => patchItem(item.id, { progress }),
            initialProg
          )

          if (finished.result_url) {
            patchItem(item.id, { status: 'ready', urls: [finished.result_url] })
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
        } catch (error) {
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
      })
      onStarted?.(id)
      const controller = new AbortController()
      abortRef.current = controller
      controllersRef.current.set(id, controller)
      setIsGenerating(true)

      try {
        let finalPrompt = params.prompt
        if (params.optimize !== false) {
          finalPrompt = await optimizeStudioPrompt(
            params.prompt,
            params.mode || 'image',
            controller.signal
          )
        }
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

async function captureLastFrameFromVideoUrl(videoUrl: string): Promise<string> {
  return new Promise((resolve, reject) => {
    const video = document.createElement('video')
    video.crossOrigin = 'anonymous'
    video.preload = 'auto'
    video.muted = true
    video.playsInline = true

    // Use our local media cache proxy to avoid any cross-origin tainting
    const cacheUrl = videoUrl.startsWith('data:') || videoUrl.startsWith('blob:') || videoUrl.startsWith('/api/')
      ? videoUrl
      : `/api/media/cache?url=${encodeURIComponent(videoUrl)}`

    video.src = cacheUrl

    const timer = setTimeout(() => {
      cleanup()
      reject(new Error('Timeout extracting last video frame'))
    }, 15000)

    function cleanup() {
      clearTimeout(timer)
      video.removeAttribute('src')
      video.load()
    }

    video.onloadedmetadata = () => {
      // Seek slightly before the very end to avoid EOF black frame
      const targetTime = Math.max(0, video.duration - 0.15)
      video.currentTime = targetTime
    }

    video.onseeked = () => {
      try {
        const canvas = document.createElement('canvas')
        canvas.width = video.videoWidth || 1280
        canvas.height = video.videoHeight || 720
        const ctx = canvas.getContext('2d')
        if (!ctx) {
          cleanup()
          reject(new Error('Canvas 2D context not available'))
          return
        }
        ctx.drawImage(video, 0, 0, canvas.width, canvas.height)
        const dataUrl = canvas.toDataURL('image/jpeg', 0.92)
        cleanup()
        resolve(dataUrl)
      } catch (err) {
        cleanup()
        reject(err)
      }
    }

    video.onerror = (e) => {
      cleanup()
      reject(new Error('Video loading error during frame extraction'))
    }
  })
}

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
      })
      onStarted?.(id)
      const controller = new AbortController()
      abortRef.current = controller
      controllersRef.current.set(id, controller)
      setIsGenerating(true)

      try {
        let finalPrompt = params.prompt
        if (params.optimize !== false) {
          finalPrompt = await optimizeStudioPrompt(params.prompt, 'video', controller.signal)
        }

        // Determine if chained multi-chunk extension is requested
        const targetTotalSeconds = params.videoExtension && params.videoExtension !== 'off'
          ? parseInt(params.videoExtension, 10) || 0
          : 0

        // Helper to run a single generation attempt with fallback support
        const executeChunkWithFallback = async (
          chunkPrompt: string,
          chunkStartFrame: string | undefined,
          chunkEndFrame: string | undefined,
          preferredDuration: string | number | undefined,
          preferredQuality: string | undefined,
          chunkIndex: number,
          totalChunks: number
        ): Promise<{ url: string; duration: string | number; quality: string }> => {
          // Fallback plan: 30s -> 20s -> 10s; 1080p -> 720p
          const durationAttempts: (string | number)[] = []
          if (preferredDuration) durationAttempts.push(preferredDuration)
          if (!durationAttempts.includes('30') && !durationAttempts.includes(30)) durationAttempts.push('30')
          if (!durationAttempts.includes('20') && !durationAttempts.includes(20)) durationAttempts.push('20')
          if (!durationAttempts.includes('10') && !durationAttempts.includes(10)) durationAttempts.push('10')

          const qualityAttempts: string[] = []
          if (preferredQuality) qualityAttempts.push(preferredQuality)
          if (!qualityAttempts.includes('1080p')) qualityAttempts.push('1080p')
          if (!qualityAttempts.includes('720p')) qualityAttempts.push('720p')

          let lastErr: any = null

          for (const curQuality of qualityAttempts) {
            for (const curDuration of durationAttempts) {
              if (controller.signal.aborted) {
                throw new Error('Cancelled')
              }

              try {
                const submitted = await submitVideoGeneration(
                  {
                    model: params.model,
                    group: params.group,
                    prompt: chunkPrompt,
                    duration: curDuration,
                    ratio: params.ratio,
                    quality: curQuality,
                    image_url: chunkIndex === 0 ? params.imageUrl : undefined,
                    start_frame: chunkStartFrame,
                    end_frame: chunkEndFrame,
                    reference_images: chunkIndex === 0 ? params.referenceImages : undefined,
                    negative_prompt: params.negativePrompt,
                    metadata: {
                      start_frame: chunkStartFrame,
                      end_frame: chunkEndFrame,
                      reference_images: params.referenceImages,
                      negative_prompt: params.negativePrompt,
                      video_extension: params.videoExtension,
                      chunk_index: chunkIndex,
                      total_chunks: totalChunks,
                    },
                  },
                  controller.signal
                )

                const submittedAt = Date.now()
                const taskId = await resolveSubmittedTaskIdWithLookup(submitted, {
                  model: params.model,
                  submittedAt,
                  signal: controller.signal,
                })

                if (!taskId) {
                  throw new Error('The video task has no id')
                }

                patchItem(id, { taskId })

                const finished = await pollVideoTask(
                  taskId,
                  controller.signal,
                  (minCreatedAt) =>
                    resolveSubmittedTaskIdWithLookup(submitted, {
                      model: params.model,
                      submittedAt,
                      signal: controller.signal,
                      minCreatedAt,
                    }),
                  (progress) => {
                    // Normalize progress across total chunks
                    const chunkWeight = 100 / totalChunks
                    const overallProgress = Math.round((chunkIndex * chunkWeight) + ((progress / 100) * chunkWeight))
                    patchItem(id, { progress: Math.min(overallProgress, 99) })
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

        // If no multi-chunk extension is requested, run as single generation
        if (targetTotalSeconds <= 30) {
          const singleChunkDuration = params.duration || (targetTotalSeconds > 0 ? targetTotalSeconds : undefined)
          const result = await executeChunkWithFallback(
            finalPrompt,
            params.startFrame,
            params.endFrame,
            singleChunkDuration,
            params.quality,
            0,
            1
          )

          patchItem(id, { status: 'ready', urls: [result.url], progress: 100 })
          if (result.url && result.url.startsWith('http')) {
            void api.post('/api/media/cache', { url: result.url }).catch(() => {})
          }
          return id
        }

        // Multi-chunk chained generation (e.g., 60s -> 2 chunks, 120s -> 4 chunks of 30s)
        const chunkDurationSec = 30
        const totalChunks = Math.max(1, Math.ceil(targetTotalSeconds / chunkDurationSec))
        const accumulatedUrls: string[] = []
        let currentStartFrame = params.startFrame

        for (let i = 0; i < totalChunks; i++) {
          if (controller.signal.aborted) {
            patchItem(id, { status: 'error', error: 'Cancelled' })
            return id
          }

          // In multi-chunk:
          // Chunk 0 gets user's initial startFrame (if any).
          // Intermediate chunks get the last frame of the previous chunk as startFrame.
          // ONLY the final chunk (i === totalChunks - 1) gets user's endFrame!
          const isFinalChunk = (i === totalChunks - 1)
          const chunkEndFrame = isFinalChunk ? params.endFrame : undefined

          const chunkRes = await executeChunkWithFallback(
            finalPrompt,
            currentStartFrame,
            chunkEndFrame,
            chunkDurationSec,
            params.quality,
            i,
            totalChunks
          )

          accumulatedUrls.push(chunkRes.url)
          if (chunkRes.url.startsWith('http')) {
            void api.post('/api/media/cache', { url: chunkRes.url }).catch(() => {})
          }

          if (!isFinalChunk) {
            // Extract the last frame of this chunk to pass as start_frame for the next chunk
            try {
              currentStartFrame = await captureLastFrameFromVideoUrl(chunkRes.url)
            } catch (frameErr) {
              // If frame capture fails, fallback to passing the video url itself
              currentStartFrame = chunkRes.url
            }
          }
        }

        patchItem(id, {
          status: 'ready',
          urls: accumulatedUrls,
          progress: 100,
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
          if (u) {
            void api.delete('/api/media/cache', { params: { url: u } }).catch(() => {})
          }
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
    generateImage,
    generateVideo,
    stop,
    removeItem,
    clearItems,
  }
}
