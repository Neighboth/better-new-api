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

import type { PlanStep, PlanStepStatus } from '../../types'

const PLAN_STEP_STATUSES: readonly PlanStepStatus[] = [
  'pending',
  'in_progress',
  'completed',
]

function isPlanStepStatus(value: unknown): value is PlanStepStatus {
  return (
    typeof value === 'string' &&
    (PLAN_STEP_STATUSES as readonly string[]).includes(value)
  )
}

/**
 * Normalize the model-provided plan payload into displayable steps. Invalid
 * entries are dropped, missing statuses default to pending, and only the first
 * in_progress step keeps that status so the table stays unambiguous.
 */
export function normalizePlanSteps(input: unknown): PlanStep[] {
  if (!Array.isArray(input)) {
    return []
  }

  let hasInProgress = false

  return input
    .map((item): PlanStep | null => {
      if (!item || typeof item !== 'object') {
        return null
      }

      const record = item as Record<string, unknown>
      const title = typeof record.title === 'string' ? record.title.trim() : ''
      if (!title) {
        return null
      }

      let status: PlanStepStatus = isPlanStepStatus(record.status)
        ? record.status
        : 'pending'
      if (status === 'in_progress') {
        if (hasInProgress) {
          status = 'pending'
        } else {
          hasInProgress = true
        }
      }

      return { id: nanoid(), title, status }
    })
    .filter((step): step is PlanStep => Boolean(step))
}

export function togglePlanStep(steps: PlanStep[], stepId: string): PlanStep[] {
  return steps.map((step) =>
    step.id === stepId
      ? {
          ...step,
          status: step.status === 'completed' ? 'pending' : 'completed',
        }
      : step
  )
}

export function renamePlanStep(
  steps: PlanStep[],
  stepId: string,
  title: string
): PlanStep[] {
  const trimmed = title.trim()
  if (!trimmed) {
    return steps
  }

  return steps.map((step) =>
    step.id === stepId ? { ...step, title: trimmed } : step
  )
}

export function removePlanStep(steps: PlanStep[], stepId: string): PlanStep[] {
  return steps.filter((step) => step.id !== stepId)
}

export function appendPlanStep(steps: PlanStep[], title: string): PlanStep[] {
  const trimmed = title.trim()
  if (!trimmed) {
    return steps
  }

  return [...steps, { id: nanoid(), title: trimmed, status: 'pending' }]
}

export function getPlanProgress(steps: PlanStep[]): {
  completed: number
  total: number
} {
  return {
    completed: steps.filter((step) => step.status === 'completed').length,
    total: steps.length,
  }
}
