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
 * Resolves the id the polling endpoint understands.
 *
 * The submission response carries the id of the upstream provider (or the
 * public id of this installation in `X-New-Api-Task-Id`), but `/api/task/self`
 * is read by the id of the task row this installation stored — the two are not
 * always the same. The row is therefore the source of truth: it is looked up
 * right after the submit and only when it cannot be found yet the id of the
 * response is used as a fallback.
 */
export async function resolveSubmittedTaskIdWithLookup(
  response: unknown,
  options: {
    model?: string
    submittedAt?: number
    signal?: AbortSignal
    minCreatedAt?: number
  } = {}
): Promise<string> {
  const direct = resolveSubmittedTaskId(response)
  const submittedAt = options.submittedAt ?? Date.now()

  for (let attempt = 0; attempt < 3; attempt += 1) {
    try {
      const task = await findLatestVideoTask({
        model: options.model,
        sinceMs: submittedAt,
        signal: options.signal,
        minCreatedAt: options.minCreatedAt,
      })
      if (task?.task_id) return task.task_id
    } catch (error) {
      if (options.signal?.aborted) throw error
    }
    if (options.signal?.aborted) return direct
    await wait(1200, options.signal)
  }

  return direct
}

/**
 * Poll one video task until it finishes. The video relay is asynchronous: the
 * submit call returns a task id, the dashboard polls `/api/task/self` for the
 * status and the final media URL.
 *
 * `onProgress` is called with the raw progress string on every poll that
 * reports one ("0%", "42%", "100%"); callers render it when it is present and
 * hide the percentage entirely when the provider sends none.
 */
export async function pollVideoTask(
  taskId: string,
  signal?: AbortSignal,
  rescue?: (minCreatedAt: number) => Promise<string>,
  onProgress?: (progress: string) => void
): Promise<VideoTaskState> {
  const startedAt = Date.now()
  let last: VideoTaskState | null = null
  let currentId = taskId
  let emptyPolls = 0
  // Unix seconds of the first poll that actually found the task. A rescue
  // callback must never return a task created before this anchor; otherwise
  // the poll can jump backwards onto an unrelated older task and the UI
  // shows its progress ("88%" -> "14%").
  let anchorCreatedAt = 0
  let maxProgress = 0

  while (Date.now() - startedAt < MAX_WAIT_MS) {
    if (signal?.aborted) {
      throw new DOMException('Aborted', 'AbortError')
    }

    try {
      last = await getVideoTask(currentId, signal)
    } catch (error) {
      if (signal?.aborted) throw error
      last = null
    }

    if (!last) {
      emptyPolls += 1
      // The id we poll does not exist in this installation (the upstream id was
      // used instead of the id of the task row). Ask for the real one once and
      // continue polling with it.
      if (rescue && emptyPolls === 2) {
        try {
          const rescued = await rescue(anchorCreatedAt)
          if (rescued && rescued !== currentId) {
            currentId = rescued
            emptyPolls = 0
            anchorCreatedAt = 0
            maxProgress = 0
            continue
          }
        } catch (error) {
          if (signal?.aborted) throw error
        }
      }
    } else {
      emptyPolls = 0
    }

    if (last) {
      if (anchorCreatedAt === 0) {
        const createdAt = Number(last.created_at ?? 0)
        if (Number.isFinite(createdAt) && createdAt > 0) {
          anchorCreatedAt = createdAt
        }
      }
      if (onProgress && last.progress) {
        const percent = Number.parseFloat(String(last.progress))
        if (Number.isFinite(percent)) {
          // Never report a progress lower than what was already shown for
          // this poll session. Providers occasionally report a stale row.
          if (percent >= maxProgress) {
            maxProgress = percent
            onProgress(last.progress)
          }
        } else {
          onProgress(last.progress)
        }
      }
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
