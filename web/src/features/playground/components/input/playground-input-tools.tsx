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
import { PlusIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import {
  PromptInputButton,
  PromptInputTools,
} from '@/components/ai-elements/prompt-input'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'

import type {
  ParameterEnabled,
  PlaygroundConfig,
  PlaygroundToolId,
  PlaygroundToolsEnabled,
} from '../../types'
import { PlaygroundAttachmentMenu } from './playground-attachment-menu'
import { PlaygroundParameterPanel } from './playground-parameter-panel'
import { PlaygroundToolsMenu } from './playground-tools-menu'

type PlaygroundInputToolsProps = {
  config: PlaygroundConfig
  disabled?: boolean
  onConfigChange: <K extends keyof PlaygroundConfig>(
    key: K,
    value: PlaygroundConfig[K]
  ) => void
  onAppendText: (snippet: string) => void
  onNewChat?: () => void
  onParameterEnabledChange: (
    key: keyof ParameterEnabled,
    value: boolean
  ) => void
  parameterEnabled: ParameterEnabled
  toolsEnabled: PlaygroundToolsEnabled
  onToolsEnabledChange: (tool: PlaygroundToolId, value: boolean) => void
}

export function PlaygroundInputTools(props: PlaygroundInputToolsProps) {
  const { t } = useTranslation()

  return (
    <PromptInputTools className='bg-background/70 border-border/60 rounded-lg border p-1 shadow-xs'>
      <PlaygroundAttachmentMenu
        disabled={props.disabled}
        onAppendText={props.onAppendText}
      />

      <PlaygroundToolsMenu
        disabled={props.disabled}
        onToolsEnabledChange={props.onToolsEnabledChange}
        toolsEnabled={props.toolsEnabled}
      />

      <PlaygroundParameterPanel
        config={props.config}
        disabled={props.disabled}
        onConfigChange={props.onConfigChange}
        onParameterEnabledChange={props.onParameterEnabledChange}
        parameterEnabled={props.parameterEnabled}
      />

      <Tooltip>
        <TooltipTrigger
          render={
            <PromptInputButton
              aria-label={t('New chat')}
              className='text-muted-foreground hover:text-foreground hover:bg-muted/70 font-medium'
              disabled={props.disabled || !props.onNewChat}
              onClick={props.onNewChat}
              variant='ghost'
            >
              <PlusIcon size={16} />
              <span className='hidden sm:inline'>{t('New chat')}</span>
            </PromptInputButton>
          }
        />
        <TooltipContent>
          <p>{t('New chat')}</p>
        </TooltipContent>
      </Tooltip>
    </PromptInputTools>
  )
}
