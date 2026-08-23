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
  Globe02Icon,
  Image02Icon,
  Link04Icon,
  Task01Icon,
  ToolsIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { PromptInputButton } from '@/components/ai-elements/prompt-input'
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'

import {
  getToolProviderKeys,
  setToolProviderKeys,
  type ToolProviderKeys,
} from '../../lib/tools/provider-keys'
import type { PlaygroundToolId, PlaygroundToolsEnabled } from '../../types'

type PlaygroundToolsMenuProps = {
  disabled?: boolean
  toolsEnabled: PlaygroundToolsEnabled
  onToolsEnabledChange: (tool: PlaygroundToolId, value: boolean) => void
}

const TOOL_ITEMS: {
  id: PlaygroundToolId
  labelKey: string
  descriptionKey: string
  icon: ReactNode
}[] = [
  {
    id: 'generate_image',
    labelKey: 'Image generation',
    descriptionKey: 'Let the model generate images',
    icon: <HugeiconsIcon icon={Image02Icon} size={16} strokeWidth={2} />,
  },
  {
    id: 'web_search',
    labelKey: 'Web search',
    descriptionKey: 'Let the model search the web',
    icon: <HugeiconsIcon icon={Globe02Icon} size={16} strokeWidth={2} />,
  },
  {
    id: 'fetch_page',
    labelKey: 'Page fetch',
    descriptionKey: 'Let the model read web pages',
    icon: <HugeiconsIcon icon={Link04Icon} size={16} strokeWidth={2} />,
  },
  {
    id: 'update_plan',
    labelKey: 'Plan steps',
    descriptionKey: 'Let the model keep a visible task plan',
    icon: <HugeiconsIcon icon={Task01Icon} size={16} strokeWidth={2} />,
  },
]

const API_KEY_FIELDS: {
  id: keyof ToolProviderKeys
  labelKey: string
  placeholder: string
}[] = [
  {
    id: 'tavily',
    labelKey: 'Tavily API key',
    placeholder: 'tvly-…',
  },
  {
    id: 'firecrawl',
    labelKey: 'Firecrawl API key',
    placeholder: 'fc-…',
  },
]

export function PlaygroundToolsMenu(props: PlaygroundToolsMenuProps) {
  const { t } = useTranslation()
  const [apiKeys, setApiKeys] = useState<ToolProviderKeys>(getToolProviderKeys)
  const enabledCount = TOOL_ITEMS.filter(
    (item) => props.toolsEnabled[item.id]
  ).length

  const updateApiKey = (id: keyof ToolProviderKeys, value: string) => {
    const next = { ...apiKeys, [id]: value }
    setApiKeys(next)
    setToolProviderKeys(next)
  }

  return (
    <DropdownMenu>
      <Tooltip>
        <TooltipTrigger
          render={
            <DropdownMenuTrigger
              render={
                <PromptInputButton
                  aria-label={t('Tools')}
                  className='text-muted-foreground hover:text-foreground hover:bg-muted/70 font-medium'
                  disabled={props.disabled}
                  variant='ghost'
                />
              }
            />
          }
        >
          <HugeiconsIcon icon={ToolsIcon} size={16} strokeWidth={2} />
          <span className='hidden sm:inline'>
            {t('Tools')}
            {enabledCount > 0 ? ` (${enabledCount})` : ''}
          </span>
        </TooltipTrigger>
        <TooltipContent>
          <p>{t('Choose which tools the model may use')}</p>
        </TooltipContent>
      </Tooltip>
      <DropdownMenuContent align='start' className='w-64'>
        <DropdownMenuLabel>{t('Model tools')}</DropdownMenuLabel>
        {TOOL_ITEMS.map((item) => (
          <DropdownMenuCheckboxItem
            checked={props.toolsEnabled[item.id]}
            key={item.id}
            onCheckedChange={(checked) =>
              props.onToolsEnabledChange(item.id, checked === true)
            }
          >
            <span className='flex items-center gap-2'>
              {item.icon}
              <span className='flex flex-col'>
                <span>{t(item.labelKey)}</span>
                <span className='text-muted-foreground text-xs'>
                  {t(item.descriptionKey)}
                </span>
              </span>
            </span>
          </DropdownMenuCheckboxItem>
        ))}
        <DropdownMenuSeparator />
        <div className='flex flex-col gap-2 px-2 py-1.5'>
          <span className='text-muted-foreground text-xs font-medium'>
            {t('API keys (optional)')}
          </span>
          {API_KEY_FIELDS.map((field) => (
            <label
              className='flex flex-col gap-1 text-xs'
              key={field.id}
              onKeyDown={(event) => event.stopPropagation()}
            >
              <span className='text-muted-foreground'>{t(field.labelKey)}</span>
              <Input
                autoComplete='off'
                className='h-7 text-xs'
                onChange={(event) =>
                  updateApiKey(field.id, event.target.value)
                }
                placeholder={field.placeholder}
                type='password'
                value={apiKeys[field.id] ?? ''}
              />
            </label>
          ))}
          <span className='text-muted-foreground text-[11px]'>
            {t('Keys are stored in this browser only')}
          </span>
        </div>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
