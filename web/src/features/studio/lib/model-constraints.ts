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
 * Per-model option rules for the Studio.
 *
 * Administrators can attach conditional rules to a model. Every rule matches
 * on up to three dimensions (selected duration, selected quality, selected
 * aspect ratio); all non-empty dimensions must hold. Effects narrow the
 * offered durations/qualities/ratios and/or override the reference-image cap.
 * Multiple matching rules all apply — a later rule overrides an earlier one
 * per field. `max_reference_images` uses a sentinel: 0 = no override,
 * -1 = reference images disabled, >=1 = override the cap.
 */

export type ModelConstraint = {
  durations?: number[]
  qualities?: string[]
  ratios?: string[]
  durations_only?: number[]
  qualities_only?: string[]
  ratios_only?: string[]
  max_reference_images?: number
}

export type ModelRuleInput = {
  reference_image?: boolean
  max_reference_images?: number
  constraints?: ModelConstraint[]
  durations?: number[]
  qualities?: string[]
  ratios?: string[]
}

export type EffectiveModelOptions = {
  durations: number[]
  qualities: string[]
  ratios: string[]
  /** 0 = reference images not allowed for this selection. */
  maxReferenceImages: number
  referencesAllowed: boolean
}

export const DEFAULT_REFERENCE_IMAGE_CAP = 5

function intersectStrings(base: string[], only: string[] | undefined): string[] {
  if (!only || only.length === 0) return base
  const allowed = new Set(only)
  return base.filter((value) => allowed.has(value))
}

function intersectNumbers(base: number[], only: number[] | undefined): number[] {
  if (!only || only.length === 0) return base
  const allowed = new Set(only)
  return base.filter((value) => allowed.has(value))
}

function ruleMatches(
  rule: ModelConstraint,
  duration: number | undefined,
  quality: string | undefined,
  ratio: string | undefined
): boolean {
  if (rule.durations?.length) {
    if (duration === undefined || !rule.durations.includes(duration)) return false
  }
  if (rule.qualities?.length) {
    if (quality === undefined || !rule.qualities.includes(quality)) return false
  }
  if (rule.ratios?.length) {
    if (ratio === undefined || !rule.ratios.includes(ratio)) return false
  }
  return true
}

export function resolveEffectiveModelOptions(
  model: ModelRuleInput,
  duration?: number,
  quality?: string,
  ratio?: string
): EffectiveModelOptions {
  let durations = [...(model.durations ?? [])]
  let qualities = [...(model.qualities ?? [])]
  let ratios = [...(model.ratios ?? [])]
  let maxRefs: number
  if (!model.reference_image) {
    maxRefs = 0
  } else if (typeof model.max_reference_images === 'number' && model.max_reference_images > 0) {
    maxRefs = model.max_reference_images
  } else {
    maxRefs = DEFAULT_REFERENCE_IMAGE_CAP
  }

  for (const rule of model.constraints ?? []) {
    if (!ruleMatches(rule, duration, quality, ratio)) continue
    durations = intersectNumbers(durations, rule.durations_only)
    qualities = intersectStrings(qualities, rule.qualities_only)
    ratios = intersectStrings(ratios, rule.ratios_only)
    if (typeof rule.max_reference_images === 'number') {
      if (rule.max_reference_images === -1) maxRefs = 0
      else if (rule.max_reference_images >= 1) maxRefs = rule.max_reference_images
    }
  }
  if (!model.reference_image) maxRefs = 0
  return {
    durations,
    qualities,
    ratios,
    maxReferenceImages: maxRefs,
    referencesAllowed: maxRefs > 0,
  }
}

/** Picks the closest still-valid value for `selected` within `options`,
 * keeping the selection when it is still allowed. Used to auto-correct the
 * studio's duration/quality/ratio selectors when rules narrow the lists. */
export function pickStillValid<T>(selected: T | undefined, options: T[]): T | undefined {
  if (selected !== undefined && options.includes(selected)) return selected
  return options[0]
}
