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
  Add01Icon,
  CheckmarkCircle02Icon,
  CircleIcon,
  Delete02Icon,
  Loading03Icon,
  PencilEdit01Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import {
  Plan,
  PlanContent,
  PlanDescription,
  PlanHeader,
  PlanTitle,
} from '@/components/ai-elements/plan'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { cn } from '@/lib/utils'

import { getPlanProgress } from '../../lib'
import type { PlanStep } from '../../types'

export type PlanTableAction =
  | { type: 'toggle'; stepId: string }
  | { type: 'rename'; stepId: string; title: string }
  | { type: 'remove'; stepId: string }
  | { type: 'add'; title: string }

type PlaygroundPlanTableProps = {
  steps: PlanStep[]
  onAction?: (action: PlanTableAction) => void
}

function PlanStepStatusIcon(props: { status: PlanStep['status'] }) {
  if (props.status === 'completed') {
    return (
      <HugeiconsIcon
        className='text-primary'
        icon={CheckmarkCircle02Icon}
        size={16}
        strokeWidth={2}
      />
    )
  }

  if (props.status === 'in_progress') {
    return (
      <HugeiconsIcon
        className='text-primary animate-spin'
        icon={Loading03Icon}
        size={16}
        strokeWidth={2}
      />
    )
  }

  return (
    <HugeiconsIcon
      className='text-muted-foreground'
      icon={CircleIcon}
      size={16}
      strokeWidth={2}
    />
  )
}

export function PlaygroundPlanTable(props: PlaygroundPlanTableProps) {
  const { t } = useTranslation()
  const [editingStepId, setEditingStepId] = useState<string | null>(null)
  const [editingTitle, setEditingTitle] = useState('')
  const [newStepTitle, setNewStepTitle] = useState('')

  const progress = getPlanProgress(props.steps)
  const isStreaming = props.steps.some((step) => step.status === 'in_progress')
  const stepStatusLabels: Record<PlanStep['status'], string> = {
    completed: t('Done'),
    in_progress: t('In progress'),
    pending: t('Pending'),
  }

  const commitRename = () => {
    if (editingStepId && editingTitle.trim()) {
      props.onAction?.({
        type: 'rename',
        stepId: editingStepId,
        title: editingTitle,
      })
    }
    setEditingStepId(null)
    setEditingTitle('')
  }

  const commitAdd = () => {
    if (newStepTitle.trim()) {
      props.onAction?.({ type: 'add', title: newStepTitle })
      setNewStepTitle('')
    }
  }

  return (
    <Plan
      className='border-border/60 bg-background/80 border'
      isStreaming={isStreaming}
    >
      <PlanHeader>
        <PlanTitle>{t('Plan')}</PlanTitle>
        <PlanDescription>
          {t('{{done}}/{{total}} completed', {
            done: progress.completed,
            total: progress.total,
          })}
        </PlanDescription>
      </PlanHeader>
      <PlanContent className='p-2'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className='w-24'>{t('Status')}</TableHead>
              <TableHead>{t('Step')}</TableHead>
              {props.onAction && <TableHead className='w-20' />}
            </TableRow>
          </TableHeader>
          <TableBody>
            {props.steps.map((step) => (
              <TableRow key={step.id}>
                <TableCell className='w-24'>
                  <span className='flex items-center gap-2'>
                    {props.onAction ? (
                      <Checkbox
                        aria-label={t('Mark step as done')}
                        checked={step.status === 'completed'}
                        onCheckedChange={() =>
                          props.onAction?.({ type: 'toggle', stepId: step.id })
                        }
                      />
                    ) : (
                      <PlanStepStatusIcon status={step.status} />
                    )}
                    <span className='text-muted-foreground text-xs'>
                      {stepStatusLabels[step.status]}
                    </span>
                  </span>
                </TableCell>
                <TableCell>
                  {editingStepId === step.id ? (
                    <Input
                      aria-label={t('Edit step')}
                      autoFocus
                      className='h-7 text-sm'
                      onBlur={commitRename}
                      onChange={(event) => setEditingTitle(event.target.value)}
                      onKeyDown={(event) => {
                        if (event.key === 'Enter') commitRename()
                        if (event.key === 'Escape') setEditingStepId(null)
                      }}
                      value={editingTitle}
                    />
                  ) : (
                    <span
                      className={cn(
                        'text-sm',
                        step.status === 'completed' &&
                          'text-muted-foreground line-through'
                      )}
                    >
                      {step.title}
                    </span>
                  )}
                </TableCell>
                {props.onAction && (
                  <TableCell className='w-20'>
                    <span className='flex items-center justify-end gap-1'>
                      <Button
                        aria-label={t('Edit step')}
                        className='size-6'
                        onClick={() => {
                          setEditingStepId(step.id)
                          setEditingTitle(step.title)
                        }}
                        size='icon'
                        variant='ghost'
                      >
                        <HugeiconsIcon
                          icon={PencilEdit01Icon}
                          size={14}
                          strokeWidth={2}
                        />
                      </Button>
                      <Button
                        aria-label={t('Delete step')}
                        className='size-6'
                        onClick={() =>
                          props.onAction?.({ type: 'remove', stepId: step.id })
                        }
                        size='icon'
                        variant='ghost'
                      >
                        <HugeiconsIcon
                          icon={Delete02Icon}
                          size={14}
                          strokeWidth={2}
                        />
                      </Button>
                    </span>
                  </TableCell>
                )}
              </TableRow>
            ))}
          </TableBody>
        </Table>

        {props.onAction && (
          <div className='mt-2 flex items-center gap-2'>
            <Input
              aria-label={t('Add step')}
              className='h-8 text-sm'
              onChange={(event) => setNewStepTitle(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === 'Enter') commitAdd()
              }}
              placeholder={t('Add a step')}
              value={newStepTitle}
            />
            <Button
              aria-label={t('Add step')}
              className='size-8 shrink-0'
              disabled={!newStepTitle.trim()}
              onClick={commitAdd}
              size='icon'
              variant='outline'
            >
              <HugeiconsIcon icon={Add01Icon} size={14} strokeWidth={2} />
            </Button>
          </div>
        )}
      </PlanContent>
    </Plan>
  )
}
