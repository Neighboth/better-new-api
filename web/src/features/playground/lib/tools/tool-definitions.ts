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
        'Generate an image from a text description with the currently selected model. Use it whenever the user asks to create, draw, or render a picture, or when an image would help answer the request.',
      parameters: {
        type: 'object',
        properties: {
          prompt: {
            type: 'string',
            description:
              'Detailed description of the image to generate, in English.',
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
        'Search the web for up-to-date information. Use it for current events, fresh facts, or anything that may have changed recently.',
      parameters: {
        type: 'object',
        properties: {
          query: {
            type: 'string',
            description: 'The search query.',
          },
          max_results: {
            type: 'number',
            description: 'Maximum number of results to return (1-10).',
          },
        },
        required: ['query'],
      },
    },
  },
  fetch_page: {
    type: 'function',
    function: {
      name: 'fetch_page',
      description:
        'Fetch the readable text content of a web page. Use it when the user shares a URL or when a search result needs to be read in full.',
      parameters: {
        type: 'object',
        properties: {
          url: {
            type: 'string',
            description: 'The absolute URL of the page to fetch.',
          },
        },
        required: ['url'],
      },
    },
  },
  update_plan: {
    type: 'function',
    function: {
      name: 'update_plan',
      description:
        'Create or replace the step-by-step plan for the current task. Call it before starting multi-step work and again whenever a step is finished or the plan changes. Each call replaces the whole plan, so always include every step with its current status.',
      parameters: {
        type: 'object',
        properties: {
          steps: {
            type: 'array',
            description: 'The full list of plan steps with their status.',
            items: {
              type: 'object',
              properties: {
                title: {
                  type: 'string',
                  description: 'Short description of the step.',
                },
                status: {
                  type: 'string',
                  enum: ['pending', 'in_progress', 'completed'],
                  description:
                    'Step status. At most one step should be in_progress.',
                },
              },
              required: ['title', 'status'],
            },
          },
        },
        required: ['steps'],
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
