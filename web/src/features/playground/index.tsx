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
import { useQuery } from '@tanstack/react-query'

import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { getPlaygroundRuntimeConfig } from './api'
import { PlaygroundChat } from './components/chat/playground-chat'
import { PlaygroundInput } from './components/input/playground-input'
import { PlaygroundToolStatus } from './components/playground-tool-status'
import { DEFAULT_TOOLS_ENABLED } from './constants'
import { EMPTY_PLAYGROUND_RUNTIME_CONFIG } from './types'
import {
  useChatHandler,
  usePlaygroundConversation,
  usePlaygroundOptions,
  usePlaygroundState,
} from './hooks'

export function Playground() {
  const { t } = useTranslation()
  const {
    config,
    parameterEnabled,
    thinking,
    messages,
    isLoadingMessages,
    models,
    groups,
    updateMessages,
    setModels,
    setGroups,
    updateConfig,
    updateParameterEnabled,
    clearMessages,
  } = usePlaygroundState()

  const { data: runtimeConfig } = useQuery({
    queryKey: ['playground-runtime-config'],
    queryFn: ({ signal }) => getPlaygroundRuntimeConfig(signal),
    staleTime: 60_000,
  })
  const runtime = runtimeConfig ?? EMPTY_PLAYGROUND_RUNTIME_CONFIG

  // The administrator decides which tools exist: video generation and web
  // search only show up when they are enabled and configured server side.
  const toolsEnabled = useMemo(
    () => ({
      ...DEFAULT_TOOLS_ENABLED,
      generate_video:
        DEFAULT_TOOLS_ENABLED.generate_video &&
        runtime.video.enabled &&
        Boolean(runtime.video.model),
      web_search:
        DEFAULT_TOOLS_ENABLED.web_search &&
        runtime.web_search.playground &&
        Boolean(runtime.web_search.configured),
    }),
    [runtime]
  )

  // A tool that is switched off must be described as switched off, otherwise
  // the model keeps answering "I can only generate images and videos".
  const toolUnavailableReasons = useMemo(() => {
    const reasons: Partial<
      Record<'generate_image' | 'generate_video' | 'web_search', string>
    > = {}
    if (!runtime.video.enabled) {
      reasons.generate_video = t('video generation is disabled')
    } else if (!runtime.video.model && runtime.video.models.length === 0) {
      reasons.generate_video = t('no video model is configured')
    }
    if (!runtime.web_search.playground) {
      reasons.web_search = t('the administrator turned web search off here')
    } else if (!runtime.web_search.configured) {
      reasons.web_search = t('the SearXNG URL is not configured')
    }
    if (!runtime.image.enabled) {
      reasons.generate_image = t('image generation is disabled')
    } else if (!runtime.image.model && runtime.image.models.length === 0) {
      reasons.generate_image = t('no image model is configured')
    }
    return reasons
  }, [runtime, t])

  const { sendChat, stopGeneration, isGenerating } = useChatHandler({
    config,
    parameterEnabled,
    toolsEnabled,
    toolUnavailableReasons,
    models,
    thinkingEnabled: false,
    thinkingLevel: thinking.level,
    runtime,
    onMessageUpdate: updateMessages,
  })

  const {
    editingMessageKey,
    handleSendMessage,
    handleRegenerateMessage,
    handleEditMessage,
    handleEditOpenChange,
    applyEdit,
    handleDeleteMessage,
  } = usePlaygroundConversation({
    messages,
    updateMessages,
    sendChat,
  })

  const handleNewChat = () => {
    handleEditOpenChange(false)
    stopGeneration()
    clearMessages()
  }

  const isBusy = isGenerating

  const { isLoadingModels } = usePlaygroundOptions({
    currentGroup: config.group,
    currentModel: config.model,
    setGroups,
    setModels,
    updateConfig,
  })

  return (
    <div className='relative flex size-full min-h-0 flex-col overflow-hidden'>
      {/* Full-width scroll container: scrolling works even over side whitespace */}
      <div className='flex min-h-0 flex-1 flex-col overflow-hidden'>
        <PlaygroundChat
          messages={messages}
          isLoadingMessages={isLoadingMessages}
          onRegenerateMessage={handleRegenerateMessage}
          onEditMessage={handleEditMessage}
          onDeleteMessage={handleDeleteMessage}
          onSelectPrompt={(prompt) => handleSendMessage({ text: prompt })}
          isGenerating={isBusy}
          editingKey={editingMessageKey}
          onCancelEdit={handleEditOpenChange}
          onSaveEdit={(newContent) => applyEdit(newContent, false)}
          onSaveEditAndSubmit={(newContent) => applyEdit(newContent, true)}
        />
      </div>

      {/* Input area: center content and constrain to the same container width */}
      <div className='mx-auto w-full max-w-4xl'>
        <PlaygroundToolStatus runtime={runtime} toolsEnabled={toolsEnabled} />
        <PlaygroundInput
          config={config}
          disabled={isBusy}
          groups={groups}
          groupValue={config.group}
          isGenerating={isBusy}
          isModelLoading={isLoadingModels}
          modelValue={config.model}
          models={models}
          onGroupChange={(value) => updateConfig('group', value)}
          onConfigChange={updateConfig}
          onModelChange={(value) => updateConfig('model', value)}
          onNewChat={handleNewChat}
          onParameterEnabledChange={updateParameterEnabled}
          onStop={stopGeneration}
          onSubmit={handleSendMessage}
          parameterEnabled={parameterEnabled}
        />
      </div>
    </div>
  )
}
