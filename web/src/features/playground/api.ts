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
import { api } from '@/lib/api'

import { API_ENDPOINTS } from './constants'
import { EMPTY_PLAYGROUND_RUNTIME_CONFIG } from './types'
import type {
  ChatCompletionRequest,
  ChatCompletionResponse,
  ImageGenerationRequest,
  ImageGenerationResponse,
  ModelOption,
  GroupOption,
  PlaygroundRuntimeConfig,
  VideoGenerationRequest,
  VideoGenerationResponse,
  VideoTaskState,
  WebSearchResponse,
} from './types'

/**
 * Send chat completion request (non-streaming)
 */
export async function sendChatCompletion(
  payload: ChatCompletionRequest,
  signal?: AbortSignal,
  timeout?: number
): Promise<ChatCompletionResponse> {
  const config: Record<string, unknown> = {
    signal,
    skipErrorHandler: true,
  }
  if (timeout) {
    config.timeout = timeout
  }
  const res = await api.post(API_ENDPOINTS.CHAT_COMPLETIONS, payload, config)
  return res.data
}

/**
 * Generate images from a prompt
 */
export async function generateImages(
  payload: ImageGenerationRequest,
  signal?: AbortSignal
): Promise<ImageGenerationResponse> {
  const res = await api.post(API_ENDPOINTS.IMAGE_GENERATIONS, payload, {
    signal,
    skipErrorHandler: true,
  } as Record<string, unknown>)
  return res.data
}

/**
 * Get user available models
 */
export async function getUserModels(group: string): Promise<ModelOption[]> {
  const res = await api.get(API_ENDPOINTS.USER_MODELS, {
    params: { group },
  })
  const { data } = res

  if (!data.success || !Array.isArray(data.data)) {
    return []
  }

  return data.data.map((model: string) => ({
    label: model,
    value: model,
  }))
}

/**
 * Get user groups
 */
export async function getUserGroups(): Promise<GroupOption[]> {
  const res = await api.get(API_ENDPOINTS.USER_GROUPS)
  const { data } = res

  if (!data.success || !data.data) {
    return []
  }

  const groupData = data.data as Record<string, { desc: string; ratio: number }>

  // label is for button display (name only); desc is for dropdown content
  return Object.entries(groupData).map(([group, info]) => ({
    label: group,
    value: group,
    ratio: info.ratio,
    desc: info.desc,
  }))
}

/**
 * Runtime configuration of the playground, the studio and the tool toggles.
 * Falls back to the local defaults when the request fails so the playground
 * keeps working on older backends.
 */
export async function getPlaygroundRuntimeConfig(
  signal?: AbortSignal
): Promise<PlaygroundRuntimeConfig> {
  try {
    const res = await api.get('/api/playground/config', { signal })
    const data = res?.data?.data as Partial<PlaygroundRuntimeConfig> | undefined
    if (!data) return EMPTY_PLAYGROUND_RUNTIME_CONFIG
    return {
      web_search: {
        ...EMPTY_PLAYGROUND_RUNTIME_CONFIG.web_search,
        ...(data.web_search ?? {}),
      },
      image: { ...EMPTY_PLAYGROUND_RUNTIME_CONFIG.image, ...(data.image ?? {}) },
      video: { ...EMPTY_PLAYGROUND_RUNTIME_CONFIG.video, ...(data.video ?? {}) },
      studio: {
        ...EMPTY_PLAYGROUND_RUNTIME_CONFIG.studio,
        ...(data.studio ?? {}),
      },
    }
  } catch {
    return EMPTY_PLAYGROUND_RUNTIME_CONFIG
  }
}

/** Submit a video generation task (session authenticated). */
export async function generateVideo(
  payload: VideoGenerationRequest,
  signal?: AbortSignal
): Promise<VideoGenerationResponse> {
  const res = await api.post('/pg/videos/generations', payload, {
    signal,
    skipErrorHandler: true,
  } as Record<string, unknown>)
  return res.data as VideoGenerationResponse
}

/** Read one of the tasks of the current user. */
export async function getVideoTask(
  taskId: string,
  signal?: AbortSignal
): Promise<VideoTaskState | null> {
  const res = await api.get('/api/task/self', {
    params: { task_id: taskId, p: 1, page_size: 1 },
    signal,
  })
  const items = (res?.data?.data?.items ?? []) as VideoTaskState[]
  return items[0] ?? null
}

/** Run a web search through the SearXNG instance of the installation. */
export async function searchWeb(
  query: string,
  maxResults?: number,
  signal?: AbortSignal
): Promise<WebSearchResponse> {
  const res = await api.post(
    '/pg/web-search',
    { query, max_results: maxResults },
    { signal, skipErrorHandler: true } as Record<string, unknown>
  )
  return res.data as WebSearchResponse
}
