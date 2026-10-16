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
import {
  ArrowDown01Icon,
  CheckmarkCircle02Icon,
  CircleIcon,
  Loading03Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'

import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import { cn } from '@/lib/utils'

import { getPlanProgress } from '../../lib'
import type { PlanStep } from '../../types'

function PlanStepStatusIcon({ status }: { status: PlanStep['status'] }) {
  if (status === 'completed') {
    return (
      <HugeiconsIcon
        aria-hidden='true'
        className='text-primary shrink-0'
        icon={CheckmarkCircle02Icon}
        size={14}
      />
    )
  }
  if (status === 'in_progress') {
    return (
      <HugeiconsIcon
        aria-hidden='true'
        className='text-primary shrink-0 animate-spin'
        icon={Loading03Icon}
        size={14}
      />
    )
  }
  return (
    <HugeiconsIcon
      aria-hidden='true'
      className='text-muted-foreground shrink-0'
      icon={CircleIcon}
      size={14}
    />
  )
}

type PlaygroundPlanTableProps = {
  steps: PlanStep[]
}

/**
 * Read-only view of the model's plan. Only the model edits it through the
 * update_plan tool; the user just follows the progress.
 */
export function PlaygroundPlanTable(props: PlaygroundPlanTableProps) {
  const { t } = useTranslation()
  const progress = getPlanProgress(props.steps)

  return (
    <Collapsible
      className='not-prose border-border/60 mb-2 w-full rounded-md border text-xs'
      defaultOpen
    >
      <CollapsibleTrigger className='flex w-full items-center gap-2 px-2.5 py-1.5'>
        <span className='font-medium'>{t('Plan')}</span>
        <span className='text-muted-foreground'>
          {t('{{done}}/{{total}} completed', {
            done: progress.completed,
            total: progress.total,
          })}
        </span>
        <HugeiconsIcon
          aria-hidden='true'
          className='text-muted-foreground ml-auto'
          icon={ArrowDown01Icon}
          size={14}
        />
      </CollapsibleTrigger>
      <CollapsibleContent>
        <ul className='flex flex-col gap-1 px-2.5 pb-2'>
          {props.steps.map((step) => (
            <li className='flex items-center gap-2' key={step.id}>
              <PlanStepStatusIcon status={step.status} />
              <span
                className={cn(
                  'wrap-break-word',
                  step.status === 'completed' &&
                    'text-muted-foreground line-through'
                )}
              >
                {step.title}
              </span>
            </li>
          ))}
        </ul>
      </CollapsibleContent>
    </Collapsible>
  )
}
