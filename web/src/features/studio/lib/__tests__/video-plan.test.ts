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
import { describe, expect, it } from 'vitest'

import { parseVideoPlan } from '../video-plan'

const VALID_PLAN = {
  title: 'Plane over Istanbul',
  global_style:
    'White fuselage airliner with red cheatline and tail code PS-7, golden hour, dark asphalt runway with yellow dashes.',
  characters: [
    {
      name: 'Captain Demir',
      appearance: 'Turkish man, 40s, short dark hair',
      wardrobe: 'navy uniform with gold epaulettes',
      distinctive_features: 'small scar on left eyebrow',
    },
  ],
  location: {
    description: 'Coastal runway above the Bosphorus',
    continuity_notes: 'sea stays on the right side of the frame',
  },
  consistency_anchors: [
    'pure white fuselage with one red cheatline and tail code PS-7',
    'golden-hour backlight, long shadows',
  ],
  steps: [
    { step: 1, duration_seconds: 30, action: 'The aircraft rolls to the runway and lines up.', camera: 'wide tracking shot', start_state: 'parked at gate', end_state: 'accelerating down the runway', lighting: 'golden hour' },
    { step: 2, duration_seconds: 30, action: 'The aircraft lifts off and climbs.', camera: 'low angle from runway end', start_state: 'accelerating down the runway', end_state: 'gear retracted, climbing over the sea', lighting: 'golden hour' },
    { step: 3, duration_seconds: 30, action: 'The aircraft cruises above the Bosphorus.', camera: 'aerial side view', start_state: 'gear retracted, climbing over the sea', end_state: 'descending on final approach', lighting: 'golden hour' },
    { step: 4, duration_seconds: 30, action: 'The aircraft lands and brakes.', camera: 'stationary side view, panning', start_state: 'descending on final approach', end_state: 'stopped at the terminal gate', lighting: 'golden hour' },
  ],
  negative_prompt: 'scene cut, aircraft color change, different tail code',
}

describe('parseVideoPlan', () => {
  it('parses a well-formed plan and preserves the exact number of steps', () => {
    const plan = parseVideoPlan(JSON.stringify(VALID_PLAN), 4)
    expect(plan.title).toBe('Plane over Istanbul')
    expect(plan.steps).toHaveLength(4)
    expect(plan.steps.map((s) => s.step)).toEqual([1, 2, 3, 4])
    expect(plan.steps[1].action).toContain('lifts off')
    expect(plan.consistency_anchors).toHaveLength(2)
    expect(plan.characters).toHaveLength(1)
    expect(plan.characters[0]?.distinctive_features).toBe('small scar on left eyebrow')
    expect(plan.negative_prompt).toContain('scene cut')
  })

  it('parses a plan wrapped in a markdown code fence', () => {
    const raw = '```json\n' + JSON.stringify(VALID_PLAN) + '\n```'
    const plan = parseVideoPlan(raw, 4)
    expect(plan.steps).toHaveLength(4)
    expect(plan.steps[0]?.camera).toBe('wide tracking shot')
  })

  it('parses a plan with commentary text around the JSON object', () => {
    const raw = `Sure! Here is the plan:\n${JSON.stringify(VALID_PLAN)}\nHope this works.`
    const plan = parseVideoPlan(raw, 4)
    expect(plan.steps).toHaveLength(4)
  })

  it('rejects a response that is not JSON at all', () => {
    expect(() => parseVideoPlan('the plane just flies, trust me', 4)).toThrow(/valid JSON|empty response/)
  })

  it('rejects a JSON object without usable steps', () => {
    expect(() => parseVideoPlan(JSON.stringify({ title: 'x', steps: [] }), 4)).toThrow(
      /without usable steps/
    )
  })

  it('renumbers and pads the steps deterministically when the model returns fewer than requested', () => {
    const fewer = {
      ...VALID_PLAN,
      steps: [VALID_PLAN.steps[0], VALID_PLAN.steps[1]],
    }
    const plan = parseVideoPlan(JSON.stringify(fewer), 4)
    expect(plan.steps.map((s) => s.step)).toEqual([1, 2, 3, 4])
    expect(plan.steps[2]?.action).toBe(VALID_PLAN.steps[1].action)
    expect(plan.steps[3]?.action).toBe(VALID_PLAN.steps[1].action)
  })

  it('truncates extra steps deterministically when the model returns more than requested', () => {
    const more = {
      ...VALID_PLAN,
      steps: [...VALID_PLAN.steps, { ...VALID_PLAN.steps[3], step: 5 }, { ...VALID_PLAN.steps[3], step: 6 }],
    }
    const plan = parseVideoPlan(JSON.stringify(more), 4)
    expect(plan.steps).toHaveLength(4)
    expect(plan.steps.map((s) => s.step)).toEqual([1, 2, 3, 4])
  })

  it('keeps dialogue and quotes intact and tolerates snake_case and camelCase keys', () => {
    const raw = JSON.stringify({
      ...VALID_PLAN,
      negative_prompt: undefined,
      negativePrompt: 'cut scene, tail code PS-7 change',
      consistency_anchors: undefined,
      consistencyAnchors: ['turn code PS-7'],
    })
    const plan = parseVideoPlan(raw, 4)
    expect(plan.negative_prompt).toBe('cut scene, tail code PS-7 change')
    expect(plan.consistency_anchors).toEqual(['turn code PS-7'])
  })
})
