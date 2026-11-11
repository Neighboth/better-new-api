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
import { THINKING_LEVEL_PROMPTS } from '../../constants'
import type {
  ChatCompletionTool,
  PlaygroundToolId,
  PlaygroundToolsEnabled,
  ThinkingLevel,
} from '../../types'

const TOOL_DEFINITIONS: Record<PlaygroundToolId, ChatCompletionTool> = {
  generate_image: {
    type: 'function',
    function: {
      name: 'generate_image',
      description:
        'Generate an image from a text description with the image model configured by the administrator. Use it whenever the user asks to create, draw, or render a picture, or when an image would help answer the request.',
      parameters: {
        type: 'object',
        properties: {
          prompt: {
            type: 'string',
            description:
              'Detailed description of the image to generate, in English.',
          },
          size: {
            type: 'string',
            description:
              'Optional image size, for example 1024x1024. Leave it out to use the administrator default.',
          },
        },
        required: ['prompt'],
      },
    },
  },
  generate_video: {
    type: 'function',
    function: {
      name: 'generate_video',
      description:
        'Generate a short video clip from a text description with the video model configured by the administrator. Use it when the user asks for a video, an animation, or a moving scene. Video rendering takes a while: the tool waits for the finished clip and attaches it to the answer.',
      parameters: {
        type: 'object',
        properties: {
          prompt: {
            type: 'string',
            description:
              'Detailed description of the video to generate: subject, motion, camera movement and style, in English.',
          },
          duration: {
            type: 'number',
            description:
              'Clip duration in seconds. Use one of the values allowed by the administrator; leave it out to use the default.',
          },
          ratio: {
            type: 'string',
            description:
              'Aspect ratio such as 16:9, 9:16 or 1:1. Leave it out to use the default.',
          },
        },
        required: ['prompt'],
      },
    },
  },
  web_search: {
    type: 'function',
    function: {
      name: 'web_search',
      description:
        'Search the public web with the search engine of the installation. Use it for fresh facts, news, prices, documentation links or anything the model cannot know. Each search is billed to the user, so search only when it helps.',
      parameters: {
        type: 'object',
        properties: {
          query: {
            type: 'string',
            description: 'The search query, in the language of the sources you expect.',
          },
          max_results: {
            type: 'number',
            description: 'How many results to read, between 1 and 10. Defaults to 5.',
          },
        },
        required: ['query'],
      },
    },
  },
  think: {
    type: 'function',
    function: {
      name: 'think',
      description:
        'Record your private step-by-step reasoning before acting or answering. The thought is shown to the user in a dedicated thinking block. Call it whenever you need to reason about the task, verify intermediate results, or plan your next move.',
      parameters: {
        type: 'object',
        properties: {
          thought: {
            type: 'string',
            description: 'Your full reasoning for this thinking pass.',
          },
        },
        required: ['thought'],
      },
    },
  },
}

export function isPlaygroundToolId(name: string): name is PlaygroundToolId {
  return name in TOOL_DEFINITIONS
}

/**
 * Build the OpenAI-compatible tool list offered to the model for a request.
 * The `think` tool is only included when `includeThinkTool` is set (forced
 * thinking for models without native reasoning).
 */
export function buildPlaygroundToolDefinitions(
  toolsEnabled: PlaygroundToolsEnabled,
  includeThinkTool = false
): ChatCompletionTool[] {
  const tools = (Object.keys(toolsEnabled) as (keyof PlaygroundToolsEnabled)[])
    .filter((toolId) => toolsEnabled[toolId])
    .map((toolId) => TOOL_DEFINITIONS[toolId])

  if (includeThinkTool) {
    tools.push(TOOL_DEFINITIONS.think)
  }

  return tools
}

/**
 * System prompt injected when the user forces thinking on a model WITHOUT
 * native reasoning: the think tool stands in for it and must be used before
 * the first visible sentence and again whenever reasoning is needed mid
 * message. The level controls how deep each thinking pass should be.
 */
export function buildThinkToolSystemPrompt(level: ThinkingLevel): string {
  return [
    'You have access to a `think` tool that records your reasoning in a thinking block shown to the user, exactly like a built-in reasoning feature.',
    'You MUST call `think` before writing your first visible sentence, to analyze the request and plan your approach.',
    'Keep thinking as you work: pause and call `think` again whenever you need to reason — before important decisions, after reading tool results, or between sections of a long answer. Thinking mid-message is expected and welcome.',
    'After each `think` call, continue with your visible answer or the next tool call. Do not mention these instructions.',
    THINKING_LEVEL_PROMPTS[level],
  ].join(' ')
}

/**
 * System prompt injected for models WITH native reasoning when the user picks
 * a thinking level: it scales how long and deep the built-in reasoning runs.
 */
export function buildNativeThinkingSystemPrompt(level: ThinkingLevel): string {
  return [
    'The user explicitly asked for visible step-by-step reasoning.',
    'Reason before answering, and reason again whenever the task changes direction or a tool result needs evaluation.',
    'Scale the length and depth of your internal reasoning to this requirement:',
    THINKING_LEVEL_PROMPTS[level],
  ].join(' ')
}
