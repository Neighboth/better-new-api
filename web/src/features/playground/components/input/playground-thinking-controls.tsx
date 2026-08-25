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
import { AiBrain01Icon, ArrowDown01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'

import { PromptInputButton } from '@/components/ai-elements/prompt-input'
import { Badge } from '@/components/ui/badge'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'

import { DEFAULT_THINKING_LEVEL, THINKING_LEVELS } from '../../constants'
import type { ThinkingLevel } from '../../types'

const LEVEL_LABEL_KEYS: Record<ThinkingLevel, string> = {
  lite: 'Lite',
  low: 'Low',
  medium: 'Medium',
  high: 'High',
  ultra: 'Ultra',
}

type PlaygroundThinkingControlsProps = {
  disabled?: boolean
  enabled: boolean
  level: ThinkingLevel
  onChange: (patch: Partial<{ enabled: boolean; level: ThinkingLevel }>) => void
}

/**
 * Thinking toggle + depth selector. Forcing only applies to models without
 * native reasoning; reasoning models already think on their own.
 */
export function PlaygroundThinkingControls(
  props: PlaygroundThinkingControlsProps
) {
  const { t } = useTranslation()

  return (
    <>
      <Tooltip>
        <TooltipTrigger
          render={
            <PromptInputButton
              aria-label={t('Thinking')}
              aria-pressed={props.enabled}
              className={cn(
                'font-medium',
                props.enabled
                  ? 'text-primary hover:text-primary'
                  : 'text-muted-foreground hover:text-foreground hover:bg-muted/70'
              )}
              disabled={props.disabled}
              onClick={() => props.onChange({ enabled: !props.enabled })}
              variant='ghost'
            >
              <HugeiconsIcon icon={AiBrain01Icon} size={16} />
              <span className='hidden sm:inline'>{t('Thinking')}</span>
            </PromptInputButton>
          }
        />
        <TooltipContent>
          <p>
            {props.enabled
              ? t(
                  'Thinking is on: models without native reasoning are asked to think step by step'
                )
              : t(
                  'Turn on to make models without native reasoning think step by step'
                )}
          </p>
        </TooltipContent>
      </Tooltip>

      {props.enabled && (
        <DropdownMenu>
          <DropdownMenuTrigger
            render={
              <PromptInputButton
                aria-label={t('Thinking level')}
                className='text-muted-foreground hover:text-foreground hover:bg-muted/70 font-medium'
                disabled={props.disabled}
                variant='ghost'
              >
                <span>{t(LEVEL_LABEL_KEYS[props.level])}</span>
                <HugeiconsIcon icon={ArrowDown01Icon} size={14} />
              </PromptInputButton>
            }
          />
          <DropdownMenuContent align='start'>
            <DropdownMenuRadioGroup
              onValueChange={(value) =>
                props.onChange({ level: value as ThinkingLevel })
              }
              value={props.level}
            >
              {THINKING_LEVELS.map((level) => (
                <DropdownMenuRadioItem key={level} value={level}>
                  <span className='flex items-center gap-2'>
                    {t(LEVEL_LABEL_KEYS[level])}
                    {level === DEFAULT_THINKING_LEVEL && (
                      <Badge variant='secondary'>{t('Recommended')}</Badge>
                    )}
                  </span>
                </DropdownMenuRadioItem>
              ))}
            </DropdownMenuRadioGroup>
          </DropdownMenuContent>
        </DropdownMenu>
      )}
    </>
  )
}
