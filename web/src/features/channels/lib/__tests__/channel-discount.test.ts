import { describe, expect, test } from 'vitest'

import {
  CHANNEL_FORM_DEFAULT_VALUES,
  transformFormDataToCreatePayload,
  transformFormDataToUpdatePayload,
} from '../channel-form'

describe('channel model_ratio_override passthrough', () => {
  test('create payload carries the discount map', () => {
    const payload = transformFormDataToCreatePayload({
      ...CHANNEL_FORM_DEFAULT_VALUES,
      name: 'discounted',
      type: 1,
      key: 'sk-test',
      models: 'gpt-4o',
      group: ['default'],
      status: 1,
      model_ratio_override: '{"gpt-4o":0.5}',
    })
    expect(payload.channel.model_ratio_override).toBe('{"gpt-4o":0.5}')
  })

  test('update payload carries the discount map', () => {
    const payload = transformFormDataToUpdatePayload(
      {
        ...CHANNEL_FORM_DEFAULT_VALUES,
        name: 'discounted',
        type: 1,
        models: 'gpt-4o',
        group: ['default'],
        status: 1,
        model_ratio_override: '{"gpt-4o":0.5}',
      },
      7
    )
    expect(
      (payload as Record<string, unknown>).model_ratio_override
    ).toBe('{"gpt-4o":0.5}')
  })

  test('empty discount clears to empty string on update', () => {
    const payload = transformFormDataToUpdatePayload(
      {
        ...CHANNEL_FORM_DEFAULT_VALUES,
        name: 'plain',
        type: 1,
        models: 'gpt-4o',
        group: ['default'],
        status: 1,
        model_ratio_override: '',
      },
      7
    )
    expect(
      (payload as Record<string, unknown>).model_ratio_override
    ).toBe('')
  })
})
