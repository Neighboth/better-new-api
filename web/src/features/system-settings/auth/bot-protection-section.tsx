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
import { zodResolver } from '@hookform/resolvers/zod'
import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

import { CaptchaDialog } from '@/features/auth/components/captcha-dialog'
import type { CaptchaProviderOption } from '@/features/auth/hooks/use-captcha'

import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const captchaProviders = [
  'off',
  'turnstile',
  'recaptcha',
  'hcaptcha',
  'image',
] as const

const botProtectionSchema = z.object({
  CaptchaType: z.enum(captchaProviders),
  TurnstileSiteKey: z.string().optional(),
  TurnstileSecretKey: z.string().optional(),
  RecaptchaSiteKey: z.string().optional(),
  RecaptchaSecretKey: z.string().optional(),
  HCaptchaSiteKey: z.string().optional(),
  HCaptchaSecretKey: z.string().optional(),
})

type BotProtectionFormValues = z.infer<typeof botProtectionSchema>

type BotProtectionSectionProps = {
  defaultValues: BotProtectionFormValues
  captchaFallbackEnabled: boolean
}

const providerLabels: Record<(typeof captchaProviders)[number], string> = {
  off: 'Off (no captcha)',
  turnstile: 'Cloudflare Turnstile',
  recaptcha: 'Google reCAPTCHA v2',
  hcaptcha: 'hCaptcha',
  image: 'Image captcha (built-in)',
}

export function BotProtectionSection({
  defaultValues,
  captchaFallbackEnabled,
}: BotProtectionSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [fallbackEnabled, setFallbackEnabled] = useState(captchaFallbackEnabled)
  const [testProviders, setTestProviders] = useState<CaptchaProviderOption[]>(
    []
  )
  const [testOpen, setTestOpen] = useState(false)

  const form = useForm<BotProtectionFormValues>({
    resolver: zodResolver(botProtectionSchema),
    defaultValues,
  })

  useEffect(() => {
    form.reset(defaultValues)
  }, [defaultValues, form])

  useEffect(() => {
    setFallbackEnabled(captchaFallbackEnabled)
  }, [captchaFallbackEnabled])

  const selectedProvider = form.watch('CaptchaType')

  const openCaptchaTest = () => {
    const values = form.getValues()
    const keys: Record<string, string> = {
      turnstile: values.TurnstileSiteKey ?? '',
      recaptcha: values.RecaptchaSiteKey ?? '',
      hcaptcha: values.HCaptchaSiteKey ?? '',
      image: '',
    }
    const types: CaptchaProviderOption['type'][] = [
      'turnstile',
      'recaptcha',
      'hcaptcha',
      'image',
    ]
    const providers: CaptchaProviderOption[] = []
    const ordered =
      values.CaptchaType === 'off'
        ? types
        : [values.CaptchaType, ...types.filter((p) => p !== values.CaptchaType)]
    for (const type of ordered) {
      if (type === 'image' || keys[type]) {
        providers.push({ type, siteKey: keys[type] })
      }
    }
    if (providers.length === 0) {
      toast.error(t('Configure a captcha provider first'))
      return
    }
    setTestProviders(providers)
    setTestOpen(true)
  }

  const onSubmit = async (data: BotProtectionFormValues) => {
    const updates = Object.entries(data).filter(
      ([key, value]) =>
        value !== defaultValues[key as keyof BotProtectionFormValues]
    )

    for (const [key, value] of updates) {
      await updateOption.mutateAsync({ key, value: value ?? '' })
    }
  }

  const providerKeyFields = (
    siteKeyField:
      | 'TurnstileSiteKey'
      | 'RecaptchaSiteKey'
      | 'HCaptchaSiteKey',
    secretKeyField:
      | 'TurnstileSecretKey'
      | 'RecaptchaSecretKey'
      | 'HCaptchaSecretKey',
    providerName: string
  ) => (
    <>
      <FormField
        control={form.control}
        name={siteKeyField}
        render={({ field }) => (
          <FormItem>
            <FormLabel>{t('Site Key')}</FormLabel>
            <FormControl>
              <Input
                placeholder={t('Your {{provider}} site key', {
                  provider: providerName,
                })}
                autoComplete='off'
                {...field}
              />
            </FormControl>
            <FormMessage />
          </FormItem>
        )}
      />

      <FormField
        control={form.control}
        name={secretKeyField}
        render={({ field }) => (
          <FormItem>
            <FormLabel>{t('Secret Key')}</FormLabel>
            <FormControl>
              <Input
                type='password'
                placeholder={t('Your {{provider}} secret key', {
                  provider: providerName,
                })}
                autoComplete='new-password'
                {...field}
              />
            </FormControl>
            <FormMessage />
          </FormItem>
        )}
      />
    </>
  )

  return (
    <SettingsSection title={t('Bot Protection')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)} autoComplete='off'>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
          />

          <FormField
            control={form.control}
            name='CaptchaType'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Captcha provider')}</FormLabel>
                <Select value={field.value} onValueChange={field.onChange}>
                  <FormControl>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent alignItemWithTrigger={false}>
                    {captchaProviders.map((provider) => (
                      <SelectItem key={provider} value={provider}>
                        {t(providerLabels[provider])}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <FormDescription>
                  {t(
                    'Protect login, registration, email verification and password reset with a captcha. The challenge appears in a popup when the user submits the form.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          {selectedProvider === 'turnstile' &&
            providerKeyFields(
              'TurnstileSiteKey',
              'TurnstileSecretKey',
              'Turnstile'
            )}
          {selectedProvider === 'recaptcha' &&
            providerKeyFields(
              'RecaptchaSiteKey',
              'RecaptchaSecretKey',
              'reCAPTCHA'
            )}
          {selectedProvider === 'hcaptcha' &&
            providerKeyFields(
              'HCaptchaSiteKey',
              'HCaptchaSecretKey',
              'hCaptcha'
            )}
          {selectedProvider === 'image' && (
            <FormDescription>
              {t(
                'The image captcha is generated on the server and needs no external keys.'
              )}
            </FormDescription>
          )}

          <SettingsSwitchItem>
            <SettingsSwitchContent>
              <Label>{t('Enable captcha fallback')}</Label>
              <p className='text-muted-foreground text-sm'>
                {t(
                  'If the selected captcha fails to load or its quota runs out, automatically fall back to the other configured providers, ending with the built-in image captcha. Select each provider above to enter its keys.'
                )}
              </p>
            </SettingsSwitchContent>
            <Switch
              checked={fallbackEnabled}
              onCheckedChange={(checked) => {
                setFallbackEnabled(checked)
                void updateOption.mutateAsync({
                  key: 'CaptchaFallbackEnabled',
                  value: String(checked),
                })
              }}
            />
          </SettingsSwitchItem>

          <div>
            <Button
              type='button'
              variant='outline'
              onClick={openCaptchaTest}
              disabled={selectedProvider === 'off'}
            >
              {t('Test captcha')}
            </Button>
            <p className='text-muted-foreground mt-2 text-sm'>
              {t(
                'Opens the verification popup exactly as your users will see it, with every configured provider.'
              )}
            </p>
          </div>
        </SettingsForm>
      </Form>

      <CaptchaDialog
        open={testOpen}
        onOpenChange={setTestOpen}
        providers={testProviders}
        onVerified={() => {
          setTestOpen(false)
          toast.success(t('Captcha verified successfully'))
        }}
      />
    </SettingsSection>
  )
}
