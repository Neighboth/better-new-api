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

const DEFAULT_ORDER: ActiveCaptchaType[] = [
  'turnstile',
  'recaptcha',
  'hcaptcha',
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

  // Parse custom order from status if provided, else use DEFAULT_ORDER
  const rawOrder = (status?.captcha_provider_order as string | undefined) || ''
  const parsedOrder = rawOrder
    ? (rawOrder
        .split(',')
        .map((s) => s.trim().toLowerCase())
        .filter((s) =>
          ['turnstile', 'recaptcha', 'hcaptcha', 'image'].includes(s)
        ) as ActiveCaptchaType[])
    : []

  const orderChain = Array.from(
    new Set([...parsedOrder, ...DEFAULT_ORDER])
  ) as ActiveCaptchaType[]

  const providers: CaptchaProviderOption[] = []
  if (captchaType !== 'off') {
    // If custom order is configured, respect the order directly;
    // Otherwise put chosen primary first, then fallback in default order.
    const sequence =
      parsedOrder.length > 0
        ? orderChain
        : [
            captchaType as ActiveCaptchaType,
            ...orderChain.filter((t) => t !== captchaType),
          ]

    for (const type of sequence) {
      // If fallback is not explicitly disabled or if status.captcha_fallback is true (or undefined/default)
      if (providers.length === 0 || status?.captcha_fallback !== false) {
        const candidate = toProvider(type)
        if (isUsable(candidate) && !providers.some((p) => p.type === candidate.type)) {
          providers.push(candidate)
        }
      }
    }
  }

  const siteKey =
    captchaType === 'off' ? '' : siteKeys[captchaType as ActiveCaptchaType]
  const isCaptchaEnabled = providers.length > 0

  return { captchaType, siteKey, isCaptchaEnabled, providers }
}
