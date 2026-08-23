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
import { useMemo, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import {
  CodeBlock,
  CodeBlockCopyButton,
} from '@/components/ai-elements/code-block'
import { Loader } from '@/components/ai-elements/loader'
import { MessageContent } from '@/components/ai-elements/message'
import {
  Reasoning,
  ReasoningContent,
  ReasoningTrigger,
} from '@/components/ai-elements/reasoning'
import { Response } from '@/components/ai-elements/response'
import { Shimmer } from '@/components/ai-elements/shimmer'
import {
  Source,
  Sources,
  SourcesContent,
  SourcesTrigger,
} from '@/components/ai-elements/sources'
import { cn } from '@/lib/utils'

import { getToolLabelKey, MESSAGE_STATUS } from '../../constants'
import {
  buildMessageRenderItems,
  getMessageAlignmentClass,
  getMessageContentState,
  isErrorMessage,
  parseThinkTags,
  type MessageAlignment,
} from '../../lib'
import { getMessageContentStyles } from '../../lib/message/message-styles'
import type { Message, ThoughtBlock } from '../../types'
import { MessageError } from './message-error'
import { MessageMetadata } from './message-metadata'
import { PlaygroundMessageAttachments } from './playground-message-attachments'
import { PlaygroundPlanTable } from './playground-plan-table'
import { PlaygroundToolsSummary } from './playground-tools-summary'

function getThoughtDuration(thought: ThoughtBlock): number | undefined {
  if (!thought.startedAt || !thought.completedAt) {
    return undefined
  }
  return Math.max(
    0,
    Math.round((thought.completedAt - thought.startedAt) / 1000)
  )
}

type PlaygroundMessageContentProps = {
  actions: ReactNode
  alignment: MessageAlignment
  errorActions?: ReactNode
  isSourceVisible?: boolean
  message: Message
  versionContent: string
}

export function PlaygroundMessageContent({
  actions,
  alignment,
  errorActions,
  isSourceVisible = false,
  message,
  versionContent,
}: PlaygroundMessageContentProps) {
  const { t } = useTranslation()
  const {
    hasReasoning,
    hasSources,
    reasoningContent,
    showLoader,
    showMessageContent,
    sources,
  } = getMessageContentState(message, versionContent)
  const isError = isErrorMessage(message)
  const attachments = message.attachments ?? []
  const hasAttachments = attachments.length > 0
  const isMessageFinal =
    message.status !== MESSAGE_STATUS.LOADING &&
    message.status !== MESSAGE_STATUS.STREAMING
  const toolEvents = message.toolEvents ?? []
  const planSteps = message.plan ?? []
  const activeToolLabel = message.activeTool
    ? t(getToolLabelKey(message.activeTool))
    : null

  const renderItems = useMemo(
    () => (isError ? [] : buildMessageRenderItems(message)),
    [isError, message]
  )
  const hasInlinePlan = renderItems.some((item) => item.kind === 'plan')

  return (
    <div
      className={cn(
        'flex w-full min-w-0 flex-col',
        getMessageAlignmentClass(alignment)
      )}
    >
      {hasSources && (
        <Sources>
          <SourcesTrigger count={sources.length} />
          <SourcesContent>
            {sources.map((source) => (
              <Source
                href={source.href}
                key={`${source.href}-${source.title}`}
                title={source.title}
              />
            ))}
          </SourcesContent>
        </Sources>
      )}

      {hasReasoning && (
        <Reasoning
          defaultOpen
          duration={message.reasoning?.duration}
          isStreaming={message.isReasoningStreaming}
        >
          <ReasoningTrigger />
          <ReasoningContent>{reasoningContent}</ReasoningContent>
        </Reasoning>
      )}

      {!isError && toolEvents.length > 0 && (
        <PlaygroundToolsSummary events={toolEvents} />
      )}

      {showLoader && (
        <div className='flex items-center gap-2 py-2'>
          <Loader />
          <Shimmer className='text-sm' duration={1}>
            {activeToolLabel
              ? t('Using {{tool}}…', { tool: activeToolLabel })
              : t('Responding...')}
          </Shimmer>
        </div>
      )}

      {isError && (
        <>
          <MessageError message={message} className='mb-2' />
          <MessageMetadata alignment={alignment} message={message} />
          {errorActions}
        </>
      )}

      {!isError && planSteps.length > 0 && !hasInlinePlan && (
        <PlaygroundPlanTable steps={planSteps} />
      )}

      {!isError && hasAttachments && (
        <PlaygroundMessageAttachments attachments={attachments} />
      )}

      {!isError && (showMessageContent || hasAttachments) && (
        <>
          {showMessageContent && isSourceVisible && (
            <CodeBlock
              code={versionContent}
              className='my-0 group-[.is-assistant]:w-full group-[.is-assistant]:max-w-[78ch]'
              collapsedLines={24}
              defaultCollapsed={false}
              language='markdown'
              maxExpandedLines={48}
              showLineNumbers
              showToolbar
              title={t('Raw response')}
            >
              <CodeBlockCopyButton />
            </CodeBlock>
          )}
          {showMessageContent &&
            !isSourceVisible &&
            renderItems.map((item) => {
              if (item.kind === 'text') {
                const text =
                  message.from === 'assistant'
                    ? parseThinkTags(item.text).visibleContent
                    : item.text
                if (!text) {
                  return null
                }
                return (
                  <MessageContent
                    className={cn(getMessageContentStyles())}
                    key={item.key}
                    variant='flat'
                  >
                    <Response final={isMessageFinal}>{text}</Response>
                  </MessageContent>
                )
              }

              if (item.kind === 'thought') {
                return (
                  <Reasoning
                    defaultOpen
                    duration={getThoughtDuration(item.thought)}
                    isStreaming={false}
                    key={item.key}
                  >
                    <ReasoningTrigger />
                    <ReasoningContent>{item.thought.content}</ReasoningContent>
                  </Reasoning>
                )
              }

              if (item.kind === 'tool-running') {
                return (
                  <div
                    aria-live='polite'
                    className='text-muted-foreground flex items-center gap-2 py-1 text-sm'
                    key={item.key}
                  >
                    <Loader size={14} />
                    <span>
                      {t('Using {{tool}}…', {
                        tool: t(getToolLabelKey(item.event.name)),
                      })}
                    </span>
                  </div>
                )
              }

              return planSteps.length > 0 ? (
                <PlaygroundPlanTable key={item.key} steps={planSteps} />
              ) : null
            })}
          <MessageMetadata alignment={alignment} message={message} />
          {actions}
        </>
      )}
    </div>
  )
}
