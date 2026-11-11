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
import { useCallback, useRef, useState } from 'react'

import { toGeneratedImageAttachments } from '@/features/playground/lib/message/image-message-utils'
import { parseRequestErrorDetails } from '@/features/playground/lib/streaming/request-error-utils'

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
  group?: string
  size?: string
  quality?: string
  count?: number
}

type VideoParams = {
  prompt: string
  model: string
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

export function useStudioGeneration() {
  const [items, setItems] = useState<StudioItem[]>([])
  const [isGenerating, setIsGenerating] = useState(false)
  const abortRef = useRef<AbortController | null>(null)

  const patchItem = useCallback((id: string, patch: Partial<StudioItem>) => {
    setItems((prev) =>
      prev.map((item) => (item.id === id ? { ...item, ...patch } : item))
    )
  }, [])

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
        mode: 'image',
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
        patchItem(id, {
          status: 'ready',
          urls: attachments.map((attachment) => attachment.url),
        })
      } catch (error) {
        if (controller.signal.aborted) {
          patchItem(id, { status: 'error', error: 'Cancelled' })
          return
        }
        patchItem(id, { status: 'error', error: errorMessageOf(error) })
      } finally {
        setIsGenerating(false)
        abortRef.current = null
      }
    },
    [patchItem, startItem]
  )

  const generateVideo = useCallback(
    async (params: VideoParams) => {
      const id = startItem({
        mode: 'video',
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

        const finished = await pollVideoTask(taskId, controller.signal)
        if (!finished.result_url) {
          patchItem(id, {
            status: 'error',
            error: finished.fail_reason || 'The video could not be generated',
          })
          return
        }
        patchItem(id, { status: 'ready', urls: [finished.result_url] })
      } catch (error) {
        if (controller.signal.aborted) {
          patchItem(id, { status: 'error', error: 'Cancelled' })
          return
        }
        patchItem(id, { status: 'error', error: errorMessageOf(error) })
      } finally {
        setIsGenerating(false)
        abortRef.current = null
      }
    },
    [patchItem, startItem]
  )

  const stop = useCallback(() => {
    abortRef.current?.abort()
  }, [])

  const removeItem = useCallback((id: string) => {
    setItems((prev) => prev.filter((item) => item.id !== id))
  }, [])

  const clearItems = useCallback((mode?: StudioItem['mode']) => {
    setItems((prev) =>
      mode ? prev.filter((item) => item.mode !== mode) : []
    )
  }, [])

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
