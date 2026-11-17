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
 * Deterministic cleanup of a negative prompt against the positive scene
 * description. Video models will happily follow a contradictory instruction
 * ("fixed camera" in the positive, "camera movement" in the negative) and
 * produce garbage, so anything in the negative that fights the positive is
 * dropped here — regardless of what the model or the user wrote.
 */

const CONTRADICTION_RULES: Array<{ positive: RegExp; drop: RegExp[] }> = [
  // The main prompt asks for a stationary camera — kill "camera movement" negatives.
  {
    positive: /\b(static|fixed|stationary|locked|immobile)\s+(camera|shot|frame|angle)|\b(camera|shot|angle)\s+(static|fixed|stationary|locked)\b/i,
    drop: [/\b(camera shake|camera movement|camera rotation|camera pan|camera drift|camera float|handheld|moving camera|unstable camera|shaky camera|camera motion)\b/i],
  },
  // The main prompt explicitly wants rain — never drop "rain" negatives out.
  {
    positive: /\b(rain|raining|downpour|drizzle)\b/i,
    drop: [/\b(no rain|rainless|dry weather|dry|clear weather|no precipitation|sunny)\b/i],
  },
  // Explicit vehicles in the main prompt — negatives forbidding them are self-defeating.
  {
    positive: /\b(car|vehicle|suv|sedan|truck|bus|motorcycle|scooter|bicycle|automobile|taxi)\b/i,
    drop: [/\b(no (car|vehicle|suv|sedan|truck|bus|traffic)|vehicle-free|no traffic|no cars|empty road|no automobiles)\b/i],
  },
  // Explicit people/characters — negatives banning people produce empty scenes.
  {
    positive: /\b(person|people|man|woman|child|pedestrian|driver|crowd|character|figure|silhouette)\b/i,
    drop: [/\b(empty scene|uninhabited|no people|no persons|no humans|nobody|depopulated|no characters)\b/i],
  },
  // Explicit low-light / night or daylight — opposite negatives dropped.
  {
    positive: /\b(night|dark|dusk|twilight|night-time|nighttime|glow|neon|lamp|moonlight|streetlight|street lamp)\b/i,
    drop: [/\b(daylight|bright (sun|sunlight)|sunny|daytime|high noon|clear blue sky|blinding light)\b/i],
  },
  {
    positive: /\b(day|daytime|bright|sun|sunlight|sunny|noon|midday)\b/i,
    drop: [/\b(night scene|night mode|darkness|pitch[- ]black|moonlit|candlelight|midnight)\b/i],
  },
  // Wet surface — "dry" negatives would contradict the visual anchor.
  {
    positive: /\b(wet|rain-soaked|puddle|standing water|wet asphalt|wet ground|wet road)\b/i,
    drop: [/\b(dry (road|ground|asphalt|surface)|bone[- ]dry)\b/i],
  },
]

/**
 * Splits a negative prompt into candidate clauses. Handles comma, newline,
 * semicolon, bullet and hyphen separators so mixed formatting survives.
 */
function splitNegativeClauses(negative: string): string[] {
  return negative
    .split(/[\n;,•·|]+|\s+-\s+|\s—\s/g)
    .map((part) => part.trim())
    .filter((part) => part.length > 0)
}

export function removeNegativeContradictions(mainPrompt: string, negativePrompt: string): string {
  const main = mainPrompt || ''
  const clauses = splitNegativeClauses(negativePrompt || '')
  if (clauses.length === 0) return ''
  const kept: string[] = []
  for (const clause of clauses) {
    let drop = false
    for (const rule of CONTRADICTION_RULES) {
      if (rule.positive.test(main) && rule.drop.some((pattern) => pattern.test(clause))) {
        drop = true
        break
      }
    }
    if (!drop) kept.push(clause)
  }
  return kept.join(', ')
}
