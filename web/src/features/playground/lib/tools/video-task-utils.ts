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
import { getVideoTask } from '../../api'
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
