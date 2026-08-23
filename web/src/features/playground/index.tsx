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
import { useCallback } from 'react'

import { PlaygroundChat } from './components/chat/playground-chat'
import { PlaygroundInput } from './components/input/playground-input'
import type { PlanTableAction } from './components/message/playground-plan-table'
import {
  useChatHandler,
  usePlaygroundConversation,
  usePlaygroundOptions,
  usePlaygroundState,
} from './hooks'
import {
  appendPlanStep,
  removePlanStep,
  renamePlanStep,
  togglePlanStep,
} from './lib'
import type { Message } from './types'

export function Playground() {
  const {
    config,
    parameterEnabled,
    toolsEnabled,
    messages,
    isLoadingMessages,
    models,
    groups,
    updateMessages,
    setModels,
    setGroups,
    updateConfig,
    updateParameterEnabled,
    updateToolsEnabled,
    clearMessages,
  } = usePlaygroundState()

  const { sendChat, stopGeneration, isGenerating } = useChatHandler({
    config,
    parameterEnabled,
    toolsEnabled,
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

  const handlePlanAction = useCallback(
    (message: Message, action: PlanTableAction) => {
      updateMessages((prev) =>
        prev.map((item) => {
          if (item.key !== message.key || !item.plan) {
            return item
          }

          let plan = item.plan
          switch (action.type) {
            case 'toggle':
              plan = togglePlanStep(plan, action.stepId)
              break
            case 'rename':
              plan = renamePlanStep(plan, action.stepId, action.title)
              break
            case 'remove':
              plan = removePlanStep(plan, action.stepId)
              break
            case 'add':
              plan = appendPlanStep(plan, action.title)
              break
          }

          return { ...item, plan }
        })
      )
    },
    [updateMessages]
  )

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
          onPlanAction={handlePlanAction}
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
          onToolsEnabledChange={updateToolsEnabled}
          parameterEnabled={parameterEnabled}
          toolsEnabled={toolsEnabled}
        />
      </div>
    </div>
  )
}
