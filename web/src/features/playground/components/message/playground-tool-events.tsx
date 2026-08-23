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
import { useTranslation } from 'react-i18next'

import { Tool, ToolContent, ToolHeader } from '@/components/ai-elements/tool'

import { isPlaygroundToolId } from '../../lib'
import type { ToolEvent, ToolEventStatus } from '../../types'

const TOOL_EVENT_STATES: Record<
  ToolEventStatus,
  'input-available' | 'output-available' | 'output-error'
> = {
  running: 'input-available',
  done: 'output-available',
  error: 'output-error',
}

const TOOL_LABEL_KEYS: Record<string, string> = {
  generate_image: 'Image generation',
  web_search: 'Web search',
  fetch_page: 'Page fetch',
  update_plan: 'Plan update',
}

type PlaygroundToolEventsProps = {
  events: ToolEvent[]
}

export function PlaygroundToolEvents(props: PlaygroundToolEventsProps) {
  const { t } = useTranslation()

  if (props.events.length === 0) {
    return null
  }

  return (
    <div className='flex flex-col gap-1'>
      {props.events.map((event) => {
        const title = isPlaygroundToolId(event.name)
          ? t(TOOL_LABEL_KEYS[event.name])
          : event.name

        return (
          <Tool key={event.id}>
            <ToolHeader
              state={TOOL_EVENT_STATES[event.status]}
              title={title}
              type='tool-call'
            />
            <ToolContent>
              <div className='text-muted-foreground space-y-1 p-2 text-xs'>
                {event.summary && (
                  <p className='wrap-break-word'>{event.summary}</p>
                )}
                {event.error && (
                  <p className='text-destructive wrap-break-word'>
                    {t(event.error)}
                  </p>
                )}
              </div>
            </ToolContent>
          </Tool>
        )
      })}
    </div>
  )
}
