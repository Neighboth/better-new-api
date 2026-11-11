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
import { AiImageIcon, Search01Icon, Video01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'

import type { PlaygroundRuntimeConfig, PlaygroundToolsEnabled } from '../types'

type PlaygroundToolStatusProps = {
  toolsEnabled: PlaygroundToolsEnabled
  runtime: PlaygroundRuntimeConfig
  className?: string
}

/**
 * Shows the tools the current request really carries.
 *
 * The playground used to expose image and video generation only, so users (and
 * the models themselves) believed web search did not exist. The strip lists
 * every active tool, and explains a disabled one instead of hiding it: web
 * search that is switched on but has no SearXNG URL is a configuration
 * problem the administrator has to see.
 */
export function PlaygroundToolStatus(props: PlaygroundToolStatusProps) {
  const { t } = useTranslation()
  const search = props.runtime.web_search

  const tools = [
    {
      id: 'web_search',
      enabled: props.toolsEnabled.web_search,
      icon: Search01Icon,
      label: t('Web search'),
      pending:
        search.playground && !search.configured
          ? t('SearXNG URL is missing, so web search stays off')
          : null,
    },
    {
      id: 'generate_image',
      enabled: props.toolsEnabled.generate_image,
      icon: AiImageIcon,
      label: t('Image generation'),
      pending: null,
    },
    {
      id: 'generate_video',
      enabled: props.toolsEnabled.generate_video,
      icon: Video01Icon,
      label: t('Video generation'),
      pending: null,
    },
  ].filter((tool) => tool.enabled || tool.pending)

  if (tools.length === 0) {
    return null
  }

  return (
    <div className={cn('flex flex-wrap items-center gap-1.5 px-1 pb-1', props.className)}>
      <span className='text-muted-foreground text-[11px]'>
        {t('Tools in this chat:')}
      </span>
      {tools.map((tool) => (
        <Badge
          className={cn(
            'gap-1 text-[11px] font-normal',
            !tool.enabled && 'border-amber-500/50 text-amber-600'
          )}
          key={tool.id}
          title={tool.pending ?? undefined}
          variant={tool.enabled ? 'secondary' : 'outline'}
        >
          <HugeiconsIcon aria-hidden='true' icon={tool.icon} size={12} />
          {tool.label}
          {!tool.enabled && ' ⚠'}
        </Badge>
      ))}
    </div>
  )
}
