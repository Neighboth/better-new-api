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
import { useCallback, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { sendChatCompletion } from '../api'
import {
  ERROR_MESSAGES,
  MAX_MODEL_FALLBACKS,
  MAX_TOOL_ROUNDS,
} from '../constants'
import {
  applyStreamingChunk,
  applyToolEventFinish,
  applyToolEventStart,
  buildApiTranscript,
  buildChatApiPayload,
  buildPlaygroundToolDefinitions,
  buildThinkingSystemPrompt,
  executePlaygroundTool,
  isPlaygroundToolId,
  mergeToolCallDeltas,
  parseToolArguments,
  summarizeToolCallArguments,
  supportsNativeThinking,
  updateAssistantMessageWithError,
  updateLastAssistantMessage,
  parseRequestErrorDetails,
  applyChatCompletionResponse,
  completeAssistantMessage,
  hasChatCompletionChoice,
  isAssistantMessageFinal,
  isAssistantMessagePending,
} from '../lib'
import type {
  ChatCompletionMessage,
  ChatCompletionTool,
  Message,
  ModelOption,
  PlaygroundConfig,
  ParameterEnabled,
  PlaygroundToolsEnabled,
  ThinkingLevel,
  ToolCall,
  ToolCallDelta,
} from '../types'
import { useStreamRequest } from './use-stream-request'

interface UseChatHandlerOptions {
  config: PlaygroundConfig
  parameterEnabled: ParameterEnabled
  toolsEnabled: PlaygroundToolsEnabled
  /** Available chat models for silent error fallback. */
  models: ModelOption[]
  thinkingEnabled: boolean
  thinkingLevel: ThinkingLevel
  onMessageUpdate: (updater: (prev: Message[]) => Message[]) => void
}

const KNOWN_ERROR_MESSAGES = new Set<string>(Object.values(ERROR_MESSAGES))
const STREAM_UPDATE_FLUSH_MS = 50

type PendingStreamChunks = {
  generation: number
  content: string
  reasoning: string
}

// Per-round accumulator: the text and tool calls streamed in the current
// request, so a tool_call round can be appended to the API transcript.
type RoundState = {
  content: string
  toolCalls: ToolCall[]
}

function mergePendingStreamChunk(
  currentChunk: string,
  nextChunk: string
): string {
  if (!currentChunk || !nextChunk.startsWith(currentChunk)) {
    return currentChunk + nextChunk
  }

  return nextChunk
}

/**
 * Hook for handling chat message sending and receiving
 */
export function useChatHandler({
  config,
  parameterEnabled,
  toolsEnabled,
  models,
  thinkingEnabled,
  thinkingLevel,
  onMessageUpdate,
}: UseChatHandlerOptions) {
  const { t } = useTranslation()
  const { sendStreamRequest, stopStream, isStreaming } = useStreamRequest()
  const [isRequesting, setIsRequesting] = useState(false)
  const abortControllerRef = useRef<AbortController | null>(null)
  const requestGenerationRef = useRef(0)
  const transcriptRef = useRef<ChatCompletionMessage[] | null>(null)
  const toolsRef = useRef<ChatCompletionTool[]>([])
  const roundStateRef = useRef<RoundState>({ content: '', toolCalls: [] })
  // Silent error fallback: same-model retry first, then other models.
  const modelOverrideRef = useRef<string | null>(null)
  const fallbackStateRef = useRef<{
    tried: Set<string>
    retriedCurrent: boolean
  }>({
    tried: new Set(),
    retriedCurrent: false,
  })
  const currentRoundRef = useRef(0)
  const modelsRef = useRef<ModelOption[]>(models)
  useEffect(() => {
    modelsRef.current = models
  }, [models])
  // Latest round starter, used by the tool loop to kick off the next round
  // without a circular useCallback dependency.
  const startRoundRef = useRef<
    ((generation: number, round: number) => void) | null
  >(null)
  const pendingStreamChunksRef = useRef<PendingStreamChunks>({
    generation: 0,
    content: '',
    reasoning: '',
  })
  const streamFlushTimerRef = useRef<number | null>(null)

  const discardPendingStreamUpdates = useCallback((generation: number) => {
    if (streamFlushTimerRef.current !== null) {
      window.clearTimeout(streamFlushTimerRef.current)
      streamFlushTimerRef.current = null
    }
    pendingStreamChunksRef.current = {
      generation,
      content: '',
      reasoning: '',
    }
  }, [])

  const flushStreamUpdates = useCallback(
    (generation: number) => {
      if (generation !== requestGenerationRef.current) return
      if (streamFlushTimerRef.current !== null) {
        window.clearTimeout(streamFlushTimerRef.current)
        streamFlushTimerRef.current = null
      }

      const pendingChunks = pendingStreamChunksRef.current
      if (pendingChunks.generation !== generation) return
      if (!pendingChunks.reasoning && !pendingChunks.content) {
        return
      }

      pendingStreamChunksRef.current = {
        generation,
        content: '',
        reasoning: '',
      }
      onMessageUpdate((prev) => {
        if (generation !== requestGenerationRef.current) return prev
        return updateLastAssistantMessage(prev, (message) => {
          let updatedMessage = message

          if (pendingChunks.reasoning) {
            updatedMessage = applyStreamingChunk(
              updatedMessage,
              'reasoning',
              pendingChunks.reasoning
            )
          }

          if (pendingChunks.content) {
            updatedMessage = applyStreamingChunk(
              updatedMessage,
              'content',
              pendingChunks.content
            )
          }

          return updatedMessage
        })
      })
    },
    [onMessageUpdate]
  )

  const scheduleStreamFlush = useCallback(
    (generation: number) => {
      if (generation !== requestGenerationRef.current) return
      if (streamFlushTimerRef.current !== null) {
        return
      }

      streamFlushTimerRef.current = window.setTimeout(() => {
        flushStreamUpdates(generation)
      }, STREAM_UPDATE_FLUSH_MS)
    },
    [flushStreamUpdates]
  )

  useEffect(
    () => () => {
      requestGenerationRef.current += 1
      if (streamFlushTimerRef.current !== null) {
        window.clearTimeout(streamFlushTimerRef.current)
      }
      abortControllerRef.current?.abort()
      abortControllerRef.current = null
    },
    []
  )

  const getDisplayError = useCallback(
    (error: string) => {
      if (KNOWN_ERROR_MESSAGES.has(error)) {
        return t(error)
      }

      const connectionClosedSuffix = `: ${ERROR_MESSAGES.CONNECTION_CLOSED}`
      if (error.endsWith(connectionClosedSuffix)) {
        return `${error.slice(0, -ERROR_MESSAGES.CONNECTION_CLOSED.length)}${t(
          ERROR_MESSAGES.CONNECTION_CLOSED
        )}`
      }

      return error
    },
    [t]
  )

  // Handle stream update
  const handleStreamUpdate = useCallback(
    (generation: number, type: 'reasoning' | 'content', chunk: string) => {
      if (generation !== requestGenerationRef.current) return
      if (pendingStreamChunksRef.current.generation !== generation) return
      if (type === 'content') {
        // Keep the raw per-round text so a tool_call round can be replayed
        // into the API transcript with its tool_calls.
        roundStateRef.current.content = mergePendingStreamChunk(
          roundStateRef.current.content,
          chunk
        )
      }
      pendingStreamChunksRef.current[type] = mergePendingStreamChunk(
        pendingStreamChunksRef.current[type],
        chunk
      )
      scheduleStreamFlush(generation)
    },
    [scheduleStreamFlush]
  )

  // Accumulate streamed tool_call deltas for the current round
  const handleToolCallDelta = useCallback(
    (generation: number, deltas: ToolCallDelta[]) => {
      if (generation !== requestGenerationRef.current) return
      roundStateRef.current.toolCalls = mergeToolCallDeltas(
        roundStateRef.current.toolCalls,
        deltas
      )
    },
    []
  )

  // Execute the tool calls of a finished round, then continue the loop
  const runToolCalls = useCallback(
    async (generation: number, round: number) => {
      const transcript = transcriptRef.current
      if (!transcript) return

      const { content, toolCalls } = roundStateRef.current
      roundStateRef.current = { content: '', toolCalls: [] }
      transcript.push({
        role: 'assistant',
        content: content || null,
        tool_calls: toolCalls,
      })

      const abortController = new AbortController()
      abortControllerRef.current?.abort()
      abortControllerRef.current = abortController

      const isCurrent = () =>
        requestGenerationRef.current === generation &&
        !abortController.signal.aborted

      for (const call of toolCalls) {
        if (!isCurrent()) return

        const parsedArgs = parseToolArguments(call.function.arguments)
        const toolName = isPlaygroundToolId(call.function.name)
          ? call.function.name
          : null

        onMessageUpdate((prev) => {
          if (!isCurrent()) return prev
          return updateLastAssistantMessage(prev, (message) =>
            applyToolEventStart(message, {
              id: call.id,
              name: call.function.name,
              status: 'running',
              summary: summarizeToolCallArguments(parsedArgs),
              startedAt: Date.now(),
            })
          )
        })

        const finishWithError = (errorText: string) => {
          transcript.push({
            role: 'tool',
            tool_call_id: call.id,
            name: call.function.name,
            content: JSON.stringify({ error: errorText }),
          })
          onMessageUpdate((prev) => {
            if (!isCurrent()) return prev
            return updateLastAssistantMessage(prev, (message) =>
              applyToolEventFinish(message, call.id, {
                status: 'error',
                error: errorText,
              })
            )
          })
        }

        if (!toolName || !parsedArgs) {
          finishWithError(
            toolName
              ? 'Invalid tool arguments'
              : `Unknown tool: ${call.function.name}`
          )
          continue
        }

        try {
          const outcome = await executePlaygroundTool(
            toolName,
            call.function.arguments,
            {
              config,
              models: modelsRef.current,
              signal: abortController.signal,
            }
          )
          if (!isCurrent()) return

          transcript.push({
            role: 'tool',
            tool_call_id: call.id,
            name: toolName,
            content: outcome.content,
          })
          onMessageUpdate((prev) => {
            if (!isCurrent()) return prev
            return updateLastAssistantMessage(prev, (message) =>
              applyToolEventFinish(message, call.id, {
                status: 'done',
                summary: outcome.summary,
                plan: outcome.plan,
                attachments: outcome.attachments,
                sources: outcome.sources,
                thought: outcome.thought,
              })
            )
          })
        } catch (error) {
          if (!isCurrent()) return
          finishWithError(
            error instanceof Error
              ? error.message
              : t(ERROR_MESSAGES.TOOL_CALL_FAILED)
          )
        }
      }

      if (!isCurrent()) return
      startRoundRef.current?.(generation, round + 1)
    },
    [config, onMessageUpdate, t]
  )

  // Handle stream complete
  const handleStreamComplete = useCallback(
    (generation: number, round: number) => {
      if (generation !== requestGenerationRef.current) return
      flushStreamUpdates(generation)

      const toolCalls = roundStateRef.current.toolCalls
      if (
        toolCalls.length > 0 &&
        round < MAX_TOOL_ROUNDS &&
        transcriptRef.current
      ) {
        void runToolCalls(generation, round)
        return
      }

      setIsRequesting(false)
      onMessageUpdate((prev) => {
        if (generation !== requestGenerationRef.current) return prev
        return updateLastAssistantMessage(prev, (message) =>
          isAssistantMessageFinal(message)
            ? message
            : completeAssistantMessage(message)
        )
      })
    },
    [flushStreamUpdates, onMessageUpdate, runToolCalls]
  )

  /**
   * Pick the next model for silent error recovery: one immediate retry of the
   * current model (the gateway then routes to another channel hosting it),
   * then other available models. Returns null when nothing is left to try.
   */
  const pickFallbackModel = useCallback((): string | null => {
    const state = fallbackStateRef.current
    const currentModel = modelOverrideRef.current ?? config.model

    if (!state.retriedCurrent) {
      state.retriedCurrent = true
      return currentModel
    }

    // `tried` always contains the originally selected model; the rest are
    // fallbacks already attempted.
    if (state.tried.size - 1 >= MAX_MODEL_FALLBACKS) {
      return null
    }

    for (const model of modelsRef.current) {
      if (state.tried.has(model.value)) {
        continue
      }
      state.tried.add(model.value)
      return model.value
    }

    return null
  }, [config.model])

  // Handle stream error
  const handleStreamError = useCallback(
    (generation: number, error: string, errorCode?: string) => {
      if (generation !== requestGenerationRef.current) return
      flushStreamUpdates(generation)

      // Silent recovery: retry with another channel/model before the user
      // ever sees an error.
      const fallbackModel = pickFallbackModel()
      if (fallbackModel) {
        modelOverrideRef.current = fallbackModel
        roundStateRef.current.toolCalls = []
        startRoundRef.current?.(generation, currentRoundRef.current)
        return
      }

      setIsRequesting(false)
      const displayError = getDisplayError(error)
      toast.error(displayError)
      const errorTitle = t(ERROR_MESSAGES.API_REQUEST_ERROR)
      onMessageUpdate((prev) => {
        if (generation !== requestGenerationRef.current) return prev
        return updateAssistantMessageWithError(
          prev,
          displayError,
          errorCode,
          errorTitle
        )
      })
    },
    [flushStreamUpdates, getDisplayError, onMessageUpdate, pickFallbackModel, t]
  )

  // Start a streaming request for one round of the tool loop
  const startStreamRound = useCallback(
    (generation: number, round: number) => {
      const transcript = transcriptRef.current
      if (!transcript) return

      currentRoundRef.current = round
      const effectiveConfig = modelOverrideRef.current
        ? { ...config, model: modelOverrideRef.current }
        : config
      const payload = buildChatApiPayload(
        transcript,
        effectiveConfig,
        parameterEnabled,
        toolsRef.current
      )
      void sendStreamRequest(
        payload,
        (type, chunk) => handleStreamUpdate(generation, type, chunk),
        () => handleStreamComplete(generation, round),
        (error, errorCode) => handleStreamError(generation, error, errorCode),
        (deltas) => handleToolCallDelta(generation, deltas)
      )
    },
    [
      config,
      parameterEnabled,
      sendStreamRequest,
      handleStreamUpdate,
      handleStreamComplete,
      handleStreamError,
      handleToolCallDelta,
    ]
  )

  // Run a non-streaming request for one round of the tool loop
  const runNonStreamingRound = useCallback(
    async (generation: number, round: number) => {
      const transcript = transcriptRef.current
      if (!transcript) return

      currentRoundRef.current = round
      const effectiveConfig = modelOverrideRef.current
        ? { ...config, model: modelOverrideRef.current }
        : config
      const payload = buildChatApiPayload(
        transcript,
        effectiveConfig,
        parameterEnabled,
        toolsRef.current
      )
      const abortController = new AbortController()
      abortControllerRef.current?.abort()
      abortControllerRef.current = abortController
      let continued = false

      try {
        const response = await sendChatCompletion(
          payload,
          abortController.signal
        )
        if (
          abortController.signal.aborted ||
          requestGenerationRef.current !== generation
        ) {
          return
        }

        if (!hasChatCompletionChoice(response)) {
          handleStreamError(generation, ERROR_MESSAGES.API_REQUEST_ERROR)
          return
        }

        const choice = response.choices[0]
        const toolCalls = choice.message.tool_calls ?? []

        if (toolCalls.length > 0 && round < MAX_TOOL_ROUNDS) {
          const choiceContent = choice.message.content ?? ''
          const choiceReasoning = choice.message.reasoning_content
          roundStateRef.current = { content: choiceContent, toolCalls }
          onMessageUpdate((prev) => {
            if (requestGenerationRef.current !== generation) return prev
            return updateLastAssistantMessage(prev, (message) => {
              let updated = message
              if (choiceReasoning) {
                updated = applyStreamingChunk(
                  updated,
                  'reasoning',
                  choiceReasoning
                )
              }
              if (choiceContent) {
                updated = applyStreamingChunk(updated, 'content', choiceContent)
              }
              return updated
            })
          })
          continued = true
          await runToolCalls(generation, round)
          return
        }

        onMessageUpdate((prev) => {
          if (requestGenerationRef.current !== generation) return prev
          return updateLastAssistantMessage(prev, (message) => {
            const updatedMessage = applyChatCompletionResponse(
              message,
              response
            )

            return updatedMessage ?? message
          })
        })
      } catch (error: unknown) {
        if (
          abortController.signal.aborted ||
          requestGenerationRef.current !== generation
        ) {
          return
        }

        const { errorCode, errorMessage } = parseRequestErrorDetails(error)
        handleStreamError(generation, errorMessage, errorCode)
      } finally {
        // A continued round installs its own controller; only release ours.
        if (abortControllerRef.current === abortController) {
          abortControllerRef.current = null
        }
        if (!continued && requestGenerationRef.current === generation) {
          setIsRequesting(false)
        }
      }
    },
    [config, parameterEnabled, onMessageUpdate, handleStreamError, runToolCalls]
  )

  useEffect(() => {
    startRoundRef.current = (generation, round) => {
      if (config.stream) {
        startStreamRound(generation, round)
      } else {
        void runNonStreamingRound(generation, round)
      }
    }
  })

  // Send chat request (stream or non-stream based on config)
  const sendChat = useCallback(
    (messages: Message[]) => {
      const generation = requestGenerationRef.current + 1
      requestGenerationRef.current = generation
      abortControllerRef.current?.abort()
      abortControllerRef.current = null
      discardPendingStreamUpdates(generation)
      stopStream()

      // Force thinking only when the model cannot reason natively; native
      // reasoning models are left untouched either way.
      const forceThink =
        thinkingEnabled && !supportsNativeThinking(config.model)

      transcriptRef.current = buildApiTranscript(messages)
      if (forceThink) {
        transcriptRef.current.unshift({
          role: 'system',
          content: buildThinkingSystemPrompt(thinkingLevel),
        })
      }
      toolsRef.current = buildPlaygroundToolDefinitions(
        toolsEnabled,
        forceThink
      )
      roundStateRef.current = { content: '', toolCalls: [] }
      modelOverrideRef.current = null
      fallbackStateRef.current = {
        tried: new Set([config.model]),
        retriedCurrent: false,
      }
      currentRoundRef.current = 0
      setIsRequesting(true)
      startRoundRef.current?.(generation, 0)
    },
    [
      config.model,
      thinkingEnabled,
      thinkingLevel,
      toolsEnabled,
      stopStream,
      discardPendingStreamUpdates,
    ]
  )

  // Stop generation
  const stopGeneration = useCallback(() => {
    const stoppedGeneration = requestGenerationRef.current
    flushStreamUpdates(stoppedGeneration)
    const idleGeneration = stoppedGeneration + 1
    requestGenerationRef.current = idleGeneration
    discardPendingStreamUpdates(idleGeneration)
    transcriptRef.current = null
    roundStateRef.current = { content: '', toolCalls: [] }
    stopStream()
    abortControllerRef.current?.abort()
    abortControllerRef.current = null
    setIsRequesting(false)
    onMessageUpdate((prev) => {
      if (requestGenerationRef.current !== idleGeneration) return prev
      return updateLastAssistantMessage(prev, (message) =>
        isAssistantMessagePending(message)
          ? completeAssistantMessage(message)
          : message
      )
    })
  }, [
    stopStream,
    flushStreamUpdates,
    discardPendingStreamUpdates,
    onMessageUpdate,
  ])

  return {
    sendChat,
    stopGeneration,
    isGenerating: isStreaming || isRequesting,
  }
}
