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

import { getPlaygroundRuntimeConfig } from './api'
import { PlaygroundChat } from './components/chat/playground-chat'
import { PlaygroundInput } from './components/input/playground-input'
import { DEFAULT_TOOLS_ENABLED } from './constants'
import { EMPTY_PLAYGROUND_RUNTIME_CONFIG } from './types'
import {
  useChatHandler,
  usePlaygroundConversation,
  usePlaygroundOptions,
  usePlaygroundState,
} from './hooks'

export function Playground() {
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

  const { sendChat, stopGeneration, isGenerating } = useChatHandler({
    config,
    parameterEnabled,
    toolsEnabled,
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
