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

/**
 * The highlight colors that a pinned model card can use in Model Square.
 *
 * `pin_color` is stored as a short stable identifier, the actual Tailwind
 * classes live here so the card outline and the "Pinned" badge can never drift
 * apart again (they were hardcoded separately before).
 */
export const PIN_COLOR_VALUES = [
  'amber',
  'emerald',
  'blue',
  'purple',
  'rose',
  'red',
  'none',
] as const

export type PinColorValue = (typeof PIN_COLOR_VALUES)[number]

export interface PinColorStyle {
  /** Border + background of the whole model card. */
  card: string
  /** Background / text / border of the "Pinned" badge. */
  badge: string
  /** Small round swatch used by the color picker. */
  dot: string
  /** Translation key of the human readable label. */
  label: string
}

const AMBER: PinColorStyle = {
  card: 'border-amber-500/30 bg-amber-500/[0.02]',
  badge:
    'bg-amber-500/10 text-amber-700 dark:text-amber-400 border-amber-500/25',
  dot: 'bg-amber-500',
  label: 'Amber / Gold Highlight',
}

const PIN_COLOR_STYLES: Record<Exclude<PinColorValue, 'none'>, PinColorStyle> = {
  amber: AMBER,
  emerald: {
    card: 'border-emerald-500/40 bg-emerald-500/[0.03]',
    badge:
      'bg-emerald-500/10 text-emerald-700 dark:text-emerald-400 border-emerald-500/25',
    dot: 'bg-emerald-500',
    label: 'Emerald / Green Highlight',
  },
  blue: {
    card: 'border-blue-500/40 bg-blue-500/[0.03]',
    badge:
      'bg-blue-500/10 text-blue-700 dark:text-blue-400 border-blue-500/25',
    dot: 'bg-blue-500',
    label: 'Blue Highlight',
  },
  purple: {
    card: 'border-purple-500/40 bg-purple-500/[0.03]',
    badge:
      'bg-purple-500/10 text-purple-700 dark:text-purple-400 border-purple-500/25',
    dot: 'bg-purple-500',
    label: 'Purple Highlight',
  },
  rose: {
    card: 'border-rose-500/40 bg-rose-500/[0.03]',
    badge: 'bg-rose-500/10 text-rose-700 dark:text-rose-400 border-rose-500/25',
    dot: 'bg-rose-500',
    label: 'Rose Highlight',
  },
  red: {
    card: 'border-red-600/50 bg-red-600/[0.04]',
    badge: 'bg-red-600/10 text-red-700 dark:text-red-400 border-red-600/30',
    dot: 'bg-red-600',
    label: 'Red Highlight',
  },
}

/**
 * Resolves a stored `pin_color` into card, badge and swatch classes.
 * Unknown / legacy values (including "gold") fall back to amber, `none`
 * removes the highlight completely.
 */
export function getPinColorStyle(color?: string | null): PinColorStyle {
  const normalized = (color || 'amber').toLowerCase()
  if (normalized === 'none') {
    return { card: '', badge: '', dot: 'bg-muted', label: '' }
  }
  return (
    PIN_COLOR_STYLES[normalized as keyof typeof PIN_COLOR_STYLES] ?? AMBER
  )
}

/** Ordered options for the pin color picker. */
export const PIN_COLOR_OPTIONS: { value: PinColorValue; style: PinColorStyle }[] =
  PIN_COLOR_VALUES.map((value) => ({
    value,
    style:
      value === 'none'
        ? {
            card: '',
            badge: '',
            dot: 'bg-muted-foreground/30',
            label: 'No Color Change (Default Card Style)',
          }
        : PIN_COLOR_STYLES[value],
  }))
