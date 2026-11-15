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

  useEffect(() => {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(items))
    } catch {
      // ignore storage error
    }
  }, [items])

  const patchItem = useCallback((id: string, patch: Partial<StudioItem>) => {
    setItems((prev) =>
      prev.map((item) => (item.id === id ? { ...item, ...patch } : item))
    )
  }, [])

  // Auto-resume background polling for any pending video items on mount or page reload
  useEffect(() => {
    const pendingVideos = items.filter(
      (item) => item.status === 'pending' && item.mode === 'video'
    )

    pendingVideos.forEach((item) => {
      if (activePollingTasks.current.has(item.id)) return
      activePollingTasks.current.add(item.id)

      const controller = new AbortController()
      controllersRef.current.set(item.id, controller)

      const resumePolling = async () => {
        try {
          let currentTaskId = item.taskId
          if (!currentTaskId) {
            const foundTask = await findLatestVideoTask({
              model: item.settings?.model as string,
              sinceMs: item.createdAt,
              signal: controller.signal,
            })
            if (foundTask?.task_id) {
              currentTaskId = foundTask.task_id
              patchItem(item.id, { taskId: currentTaskId })
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

          const finished = await pollVideoTask(
            currentTaskId,
            controller.signal,
            undefined,
            (progress) => patchItem(item.id, { progress })
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
    async (params: ImageParams) => {
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
      const controller = new AbortController()
      abortRef.current = controller
      controllersRef.current.set(id, controller)
      setIsGenerating(true)

      try {
        const response = await submitImageGeneration(
          {
            model: params.model,
            group: params.group,
            prompt: params.prompt,
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
    async (params: VideoParams) => {
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
      const controller = new AbortController()
      abortRef.current = controller
      controllersRef.current.set(id, controller)
      setIsGenerating(true)

      try {
        const submitted = await submitVideoGeneration(
          {
            model: params.model,
            group: params.group,
            prompt: params.prompt,
            duration: params.duration,
            ratio: params.ratio,
            quality: params.quality,
            image_url: params.imageUrl,
          },
          controller.signal
        )
        const submittedAt = Date.now()
        // Reads the id from the response (body or `X-New-Api-Task-Id`) and,
        // when neither carries it, from the task list of the user: the task is
        // created and billed either way, so a stripped header must not lose it.
        const taskId = await resolveSubmittedTaskIdWithLookup(submitted, {
          model: params.model,
          submittedAt,
          signal: controller.signal,
        })
        if (!taskId) {
          patchItem(id, {
            status: 'error',
            error: 'The video task has no id',
          })
          return
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
          (progress) => patchItem(id, { progress })
        )
        if (!finished.result_url) {
          const isStillRunning = ['in_progress', 'running', 'pending', 'queued'].includes((finished.status || '').toLowerCase())
          patchItem(id, {
            status: 'error',
            error: finished.fail_reason || (isStillRunning ? 'Video is still processing upstream. You can check it under Previous Generations.' : 'The video could not be generated'),
          })
          return
        }
        patchItem(id, { status: 'ready', urls: [finished.result_url] })
        if (finished.result_url && finished.result_url.startsWith('http')) {
          void api.post('/api/media/cache', { url: finished.result_url }).catch(() => {})
        }
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
    } else {
      abortRef.current?.abort()
      controllersRef.current.forEach((ctrl) => ctrl.abort())
      controllersRef.current.clear()
      activePollingTasks.current.clear()
      setIsGenerating(false)
    }
  }, [])

  const removeItem = useCallback((id: string) => {
    controllersRef.current.get(id)?.abort()
    controllersRef.current.delete(id)
    activePollingTasks.current.delete(id)
    setItems((prev) => prev.filter((item) => item.id !== id))
  }, [])

  const clearItems = useCallback((mode?: StudioItem['mode']) => {
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
