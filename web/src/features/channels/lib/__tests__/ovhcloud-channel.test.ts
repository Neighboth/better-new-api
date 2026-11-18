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
import { describe, expect, test } from 'vitest'

import {
  CHANNEL_TYPE_OVCLOUD_ANON,
  CHANNEL_TYPE_OPTIONS,
  MODEL_FETCHABLE_TYPES,
} from '../../constants'
import { getChannelTypeConfig } from '../channel-type-config'
import { getChannelTypeIcon } from '../channel-utils'

describe('OHVCloud (Anon) channel', () => {
  test('registers selection, fetch permissions and icon metadata', () => {
    const option = CHANNEL_TYPE_OPTIONS.find(
      (item) => item.value === CHANNEL_TYPE_OVCLOUD_ANON
    )

    expect(option).toEqual({
      value: 69,
      label: 'OHVCloud (Anon)',
    })

    expect(getChannelTypeIcon(CHANNEL_TYPE_OVCLOUD_ANON)).toBe('OvhCloud')

    expect(MODEL_FETCHABLE_TYPES.has(CHANNEL_TYPE_OVCLOUD_ANON)).toBe(true)

    const config = getChannelTypeConfig(CHANNEL_TYPE_OVCLOUD_ANON)
    expect(config.icon).toBe('OvhCloud')
    expect(config.defaultBaseUrl).toBe(
      'https://oai.endpoints.kepler.ai.cloud.ovh.net/v1'
    )
  })
})
