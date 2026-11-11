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
import { findLatestVideoTask, getVideoTask } from '../../api'
import { ERROR_MESSAGES } from '../../constants'
import type { VideoTaskState } from '../../types'

const POLL_INTERVAL_MS = 4000
const MAX_WAIT_MS = 10 * 60 * 1000

const TERMINAL_FAILURE_STATUSES = new Set([
  'failure',
  'failed',
  'error',
  'cancelled',
  'canceled',
  'expired',
])

const SUCCESS_STATUSES = new Set(['success', 'succeeded', 'completed', 'done'])

function wait(ms: number, signal?: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    if (signal?.aborted) {
      reject(new DOMException('Aborted', 'AbortError'))
      return
    }
    const timer = setTimeout(() => {
      signal?.removeEventListener('abort', onAbort)
      resolve()
    }, ms)
    const onAbort = () => {
      clearTimeout(timer)
      reject(new DOMException('Aborted', 'AbortError'))
    }
    signal?.addEventListener('abort', onAbort, { once: true })
  })
}

const TASK_ID_KEYS = ['task_id', 'taskId', 'id', 'request_id', 'requestId'] as const

/**
 * Reads the id of a freshly submitted video task.
 *
 * Providers answer with different shapes (`{task_id}`, `{id}`, `{data:{id}}`,
 * `{output:{task_id}}`, …) and the relay adds the id of this installation in
 * the `X-New-Api-Task-Id` header, which `generateVideo` copies into the body
 * as `task_id`. Without this the studio reported "The video task has no id"
 * even though the task was created and billed.
 */
export function resolveSubmittedTaskId(response: unknown): string {
  const queue: unknown[] = [response]
  const seen = new Set<unknown>()
  let depth = 0

  while (queue.length > 0 && depth < 40) {
    const current = queue.shift()
    depth += 1
    if (!current || typeof current !== 'object' || seen.has(current)) continue
    seen.add(current)

    const record = current as Record<string, unknown>
    for (const key of TASK_ID_KEYS) {
      const value = record[key]
      if (typeof value === 'string' && value.trim() !== '') {
        return value.trim()
      }
      if (typeof value === 'number' && Number.isFinite(value)) {
        return String(value)
      }
    }
    for (const value of Object.values(record)) {
      if (value && typeof value === 'object') {
        queue.push(value)
      }
    }
  }
  return ''
}

/**
 * Resolves the id of a submitted video task, falling back to the task list.
 *
 * Prefer {@link resolveSubmittedTaskId}: the response usually carries the id.
 * When neither the body nor the `X-New-Api-Task-Id` header delivered it (a
 * caching proxy, a provider that answers without an id), the created task is
 * read back from `/api/task/self` so a billed generation is never lost.
 */
export async function resolveSubmittedTaskIdWithLookup(
  response: unknown,
  options: { model?: string; submittedAt?: number; signal?: AbortSignal } = {}
): Promise<string> {
  const direct = resolveSubmittedTaskId(response)
  if (direct) return direct

  const submittedAt = options.submittedAt ?? Date.now()
  for (let attempt = 0; attempt < 3; attempt += 1) {
    try {
      const task = await findLatestVideoTask({
        model: options.model,
        sinceMs: submittedAt,
        signal: options.signal,
      })
      if (task?.task_id) return task.task_id
    } catch (error) {
      if (options.signal?.aborted) throw error
    }
    if (options.signal?.aborted) return ''
    await wait(1200, options.signal)
  }
  return ''
}

/**
 * Poll one video task until it finishes. The video relay is asynchronous: the
 * submit call returns a task id, the dashboard polls `/api/task/self` for the
 * status and the final media URL.
 */
export async function pollVideoTask(
  taskId: string,
  signal?: AbortSignal
): Promise<VideoTaskState> {
  const startedAt = Date.now()
  let last: VideoTaskState | null = null

  while (Date.now() - startedAt < MAX_WAIT_MS) {
    if (signal?.aborted) {
      throw new DOMException('Aborted', 'AbortError')
    }

    try {
      last = await getVideoTask(taskId, signal)
    } catch (error) {
      if (signal?.aborted) throw error
      last = null
    }

    if (last) {
      const status = (last.status || '').toLowerCase()
      if (last.result_url && !TERMINAL_FAILURE_STATUSES.has(status)) {
        return last
      }
      if (TERMINAL_FAILURE_STATUSES.has(status)) {
        return last
      }
      if (SUCCESS_STATUSES.has(status) && last.result_url) {
        return last
      }
    }

    await wait(POLL_INTERVAL_MS, signal)
  }

  if (last) return last
  throw new Error(ERROR_MESSAGES.VIDEO_TASK_TIMEOUT)
}
