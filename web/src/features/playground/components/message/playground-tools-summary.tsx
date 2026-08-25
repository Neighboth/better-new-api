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
  AiImageIcon,
  ArrowDown01Icon,
  BrainIcon,
  Globe02Icon,
  Link04Icon,
  Task01Icon,
  ToolsIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'

import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import { cn } from '@/lib/utils'

import { getToolLabelKey } from '../../constants'
import type { ToolEvent } from '../../types'

const TOOL_ICONS = {
  generate_image: AiImageIcon,
  web_search: Globe02Icon,
  fetch_page: Link04Icon,
  update_plan: Task01Icon,
  think: BrainIcon,
} as const

function ToolEventIcon({ name }: { name: string }) {
  const icon =
    TOOL_ICONS[name as keyof typeof TOOL_ICONS] ?? (ToolsIcon as never)
  return <HugeiconsIcon aria-hidden='true' icon={icon} size={14} />
}

function formatDuration(event: ToolEvent): string | null {
  if (!event.startedAt || !event.completedAt) {
    return null
  }
  const seconds = (event.completedAt - event.startedAt) / 1000
  return `${seconds.toFixed(1)}s`
}

type PlaygroundToolsSummaryProps = {
  events: ToolEvent[]
}

/**
 * Compact "Used N tools" disclosure at the top of the message. Individual
 * tool calls no longer render as big blocks; details live here.
 */
export function PlaygroundToolsSummary(props: PlaygroundToolsSummaryProps) {
  const { t } = useTranslation()
  const finished = props.events.filter((event) => event.status !== 'running')

  if (finished.length === 0) {
    return null
  }

  return (
    <Collapsible className='not-prose text-primary mb-2 text-xs'>
      <CollapsibleTrigger className='text-muted-foreground hover:text-foreground flex items-center gap-2'>
        <p className='font-medium'>
          {t('Used {{count}} tools', { count: finished.length })}
        </p>
        <HugeiconsIcon aria-hidden='true' icon={ArrowDown01Icon} size={14} />
      </CollapsibleTrigger>
      <CollapsibleContent className='mt-2 flex flex-col gap-2'>
        {finished.map((event) => {
          const duration = formatDuration(event)
          return (
            <div
              className='border-border/60 flex flex-col gap-1 rounded-md border px-2.5 py-1.5'
              key={event.id}
            >
              <div className='flex items-center gap-2'>
                <ToolEventIcon name={event.name} />
                <span className='font-medium'>
                  {t(getToolLabelKey(event.name))}
                </span>
                <span
                  className={cn(
                    'text-[11px]',
                    event.status === 'error'
                      ? 'text-destructive'
                      : 'text-muted-foreground'
                  )}
                >
                  {event.status === 'error' ? t('Error') : t('Completed')}
                </span>
                {duration && (
                  <span className='text-muted-foreground ml-auto text-[11px]'>
                    {duration}
                  </span>
                )}
              </div>
              {event.summary && (
                <p className='text-muted-foreground wrap-break-word'>
                  {event.summary}
                </p>
              )}
              {event.error && (
                <p className='text-destructive wrap-break-word'>
                  {t(event.error)}
                </p>
              )}
            </div>
          )
        })}
      </CollapsibleContent>
    </Collapsible>
  )
}
