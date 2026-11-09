import { describe, expect, test } from 'vitest'

import {
  CHANNEL_TYPE_AEPI,
  CHANNEL_TYPE_OPTIONS,
} from '../../constants'
import { getChannelTypeConfig } from '../channel-type-config'
import { getChannelTypeIcon } from '../channel-utils'

describe('Aepi Media channel', () => {
  test('registers selection, ordering, and icon metadata', () => {
    const option = CHANNEL_TYPE_OPTIONS.find(
      (item) => item.value === CHANNEL_TYPE_AEPI
    )

    expect(option).toEqual({
      value: CHANNEL_TYPE_AEPI,
      label: 'Aepi Media',
    })

    expect(getChannelTypeIcon(CHANNEL_TYPE_AEPI)).toBe('Aepi')

    const config = getChannelTypeConfig(CHANNEL_TYPE_AEPI)
    expect(config.icon).toBe('Aepi')
    expect(config.defaultBaseUrl).toBe('https://videoapi.aerobox.eu.cc')
  })
})
