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
import { nanoid } from 'nanoid'

import type { ToolCall, ToolCallDelta } from '../../types'

/**
 * Merge streamed tool_call deltas into complete tool calls. Providers stream
 * the id/name once and then append argument fragments keyed by `index`.
 */
export function mergeToolCallDeltas(
  current: ToolCall[],
  deltas: ToolCallDelta[]
): ToolCall[] {
  const merged = [...current]

  for (const delta of deltas) {
    const existing = merged[delta.index]
    if (!existing) {
      merged[delta.index] = {
        id: delta.id ?? `call_${nanoid()}`,
        type: 'function',
        function: {
          name: delta.function?.name ?? '',
          arguments: delta.function?.arguments ?? '',
        },
      }
      continue
    }

    merged[delta.index] = {
      id: delta.id ?? existing.id,
      type: 'function',
      function: {
        name: existing.function.name + (delta.function?.name ?? ''),
        arguments:
          existing.function.arguments + (delta.function?.arguments ?? ''),
      },
    }
  }

  return merged
}

/**
 * Parse the JSON argument payload of a tool call. Returns null when the
 * payload is malformed so the caller can report a tool error instead of
 * crashing the request loop.
 */
export function parseToolArguments(
  argumentsJson: string
): Record<string, unknown> | null {
  if (!argumentsJson.trim()) {
    return {}
  }

  try {
    const parsed: unknown = JSON.parse(argumentsJson)
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      return parsed as Record<string, unknown>
    }
  } catch {
    // fall through to null
  }

  return null
}

export function truncateToolResult(text: string, maxChars: number): string {
  if (text.length <= maxChars) {
    return text
  }

  return `${text.slice(0, maxChars)}\n\n[truncated]`
}

export function getStringArg(
  args: Record<string, unknown>,
  key: string
): string {
  const value = args[key]
  return typeof value === 'string' ? value.trim() : ''
}

export function getNumberArg(
  args: Record<string, unknown>,
  key: string,
  fallback: number,
  min: number,
  max: number
): number {
  const value = args[key]
  if (typeof value !== 'number' || !Number.isFinite(value)) {
    return fallback
  }

  return Math.min(max, Math.max(min, Math.round(value)))
}
