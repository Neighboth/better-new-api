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

import { getSubmittableInputText } from '../input/input-control-utils'
import { toGeneratedImageAttachments } from '../message/image-message-utils'

describe('toGeneratedImageAttachments', () => {
  it('uses the hosted url when the provider returns one', () => {
    const attachments = toGeneratedImageAttachments(
      { data: [{ url: 'https://cdn.example.com/a.png' }] },
      'a red cat'
    )

    expect(attachments).toEqual([
      {
        url: 'https://cdn.example.com/a.png',
        mediaType: 'image/png',
        filename: 'a red cat',
      },
    ])
  })

  it('converts base64 payloads into data urls', () => {
    const attachments = toGeneratedImageAttachments(
      { data: [{ b64_json: 'AAA' }] },
      'a red cat'
    )

    expect(attachments[0]?.url).toBe('data:image/png;base64,AAA')
  })

  it('returns nothing when the response carries no image', () => {
    expect(toGeneratedImageAttachments({}, 'prompt')).toEqual([])
    expect(toGeneratedImageAttachments({ data: [{}] }, 'prompt')).toEqual([])
  })
})

describe('getSubmittableInputText', () => {
  it('allows submitting attachments without text', () => {
    expect(getSubmittableInputText({ text: '   ', files: [{}] })).toBe('')
  })

  it('rejects empty submissions', () => {
    expect(getSubmittableInputText({ text: '  ', files: [] })).toBeNull()
  })

  it('rejects submissions while the input is disabled', () => {
    expect(getSubmittableInputText({ text: 'hello' }, true)).toBeNull()
  })
})
