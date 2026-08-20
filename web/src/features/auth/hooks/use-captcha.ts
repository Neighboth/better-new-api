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
import { useStatus } from '@/hooks/use-status'

export type CaptchaType =
  | 'off'
  | 'turnstile'
  | 'recaptcha'
  | 'hcaptcha'
  | 'image'

export type ActiveCaptchaType = Exclude<CaptchaType, 'off'>

export type CaptchaProviderOption = {
  type: ActiveCaptchaType
  siteKey: string
}

const FALLBACK_ORDER: ActiveCaptchaType[] = [
  'turnstile',
  'hcaptcha',
  'recaptcha',
  'image',
]

/**
 * Resolves the captcha provider configured on the server, plus the ordered
 * fallback chain used when a provider widget fails to load. Falls back to the
 * legacy turnstile flags when the backend does not report captcha_type yet.
 */
export function useCaptcha() {
  const { status } = useStatus()

  let captchaType = (status?.captcha_type as CaptchaType | undefined) ?? 'off'
  if (!status?.captcha_type) {
    captchaType =
      status?.turnstile_check && status?.turnstile_site_key
        ? 'turnstile'
        : 'off'
  }

  const siteKeys: Record<ActiveCaptchaType, string> = {
    turnstile: (status?.turnstile_site_key as string | undefined) ?? '',
    recaptcha: (status?.recaptcha_site_key as string | undefined) ?? '',
    hcaptcha: (status?.hcaptcha_site_key as string | undefined) ?? '',
    image: '',
  }

  const toProvider = (type: ActiveCaptchaType): CaptchaProviderOption => ({
    type,
    siteKey: siteKeys[type],
  })
  const isUsable = (provider: CaptchaProviderOption) =>
    provider.type === 'image' || Boolean(provider.siteKey)

  const providers: CaptchaProviderOption[] = []
  if (captchaType !== 'off') {
    if (status?.captcha_fallback) {
      // Fallback enabled: fixed try-order, independent of the selected type.
      for (const type of FALLBACK_ORDER) {
        const candidate = toProvider(type)
        if (isUsable(candidate)) {
          providers.push(candidate)
        }
      }
    } else {
      const primary = toProvider(captchaType as ActiveCaptchaType)
      if (isUsable(primary)) {
        providers.push(primary)
      }
    }
  }

  const siteKey =
    captchaType === 'off' ? '' : siteKeys[captchaType as ActiveCaptchaType]
  const isCaptchaEnabled = providers.length > 0

  return { captchaType, siteKey, isCaptchaEnabled, providers }
}
