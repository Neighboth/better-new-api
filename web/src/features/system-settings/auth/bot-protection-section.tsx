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
import { ArrowDown, ArrowUp } from 'lucide-react'
import { zodResolver } from '@hookform/resolvers/zod'
import { useEffect, useRef, useState } from 'react'
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
  'turnstile',
  'recaptcha',
  'hcaptcha',
  'image',
] as const

const botProtectionSchema = z.object({
  CaptchaType: z.enum(['off', ...captchaProviders]),
  CaptchaProviderOrder: z.string().optional(),
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
  const captchaEnabled = selectedProvider !== 'off'

  // Remember the last real provider so re-enabling restores it.
  const lastProviderRef = useRef<(typeof captchaProviders)[number]>(
    defaultValues.CaptchaType === 'off'
      ? 'turnstile'
      : (defaultValues.CaptchaType as (typeof captchaProviders)[number])
  )
  useEffect(() => {
    if (selectedProvider !== 'off') {
      lastProviderRef.current = selectedProvider
    }
  }, [selectedProvider])

  const [providerOrder, setProviderOrder] = useState<string[]>(() => {
    const raw = defaultValues.CaptchaProviderOrder || 'turnstile,recaptcha,hcaptcha,image'
    const items = raw.split(',').map((s: string) => s.trim().toLowerCase()).filter(Boolean)
    const defaultList = ['turnstile', 'recaptcha', 'hcaptcha', 'image']
    return Array.from(new Set([...items, ...defaultList]))
  })

  const moveOrder = (index: number, direction: 'up' | 'down') => {
    const targetIndex = direction === 'up' ? index - 1 : index + 1
    if (targetIndex < 0 || targetIndex >= providerOrder.length) return
    const newOrder = [...providerOrder]
    const temp = newOrder[index]
    newOrder[index] = newOrder[targetIndex]
    newOrder[targetIndex] = temp
    setProviderOrder(newOrder)
    form.setValue('CaptchaProviderOrder', newOrder.join(','), { shouldDirty: true })
    if (targetIndex === 0 && newOrder[0] !== 'off') {
      form.setValue('CaptchaType', newOrder[0] as any, { shouldDirty: true })
    }
  }

  const toggleCaptcha = (enabled: boolean) => {
    form.setValue(
      'CaptchaType',
      enabled ? lastProviderRef.current : 'off',
      { shouldDirty: true }
    )
  }

  const openCaptchaTest = () => {
    const values = form.getValues()
    if (values.CaptchaType === 'off') {
      toast.error(t('Configure a captcha provider first'))
      return
    }
    const siteKeys: Record<string, string | undefined> = {
      turnstile: values.TurnstileSiteKey,
      recaptcha: values.RecaptchaSiteKey,
      hcaptcha: values.HCaptchaSiteKey,
      image: '',
    }
    setTestProviders([
      { type: values.CaptchaType, siteKey: siteKeys[values.CaptchaType] ?? '' },
    ])
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

          <SettingsSwitchItem>
            <SettingsSwitchContent>
              <Label>{t('Enable captcha')}</Label>
              <p className='text-muted-foreground text-sm'>
                {t(
                  'Protect login, registration, email verification and password reset with a captcha. The challenge appears in a popup when the user submits the form.'
                )}
              </p>
            </SettingsSwitchContent>
            <Switch
              checked={captchaEnabled}
              onCheckedChange={toggleCaptcha}
            />
          </SettingsSwitchItem>

          {captchaEnabled && (
            <>
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <Label>{t('Enable captcha fallback')}</Label>
                  <p className='text-muted-foreground text-sm'>
                    {t(
                      'If the selected captcha fails to load or its quota runs out, automatically fall back to the other configured providers, ending with the built-in image captcha. Select each provider below to enter its keys.'
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

              {/* Captcha Provider Fallback Sequence */}
              <div className='rounded-lg border p-4 space-y-3 bg-muted/20'>
                <div>
                  <Label className='text-sm font-semibold'>
                    {t('Captcha Fallback Sıralaması')}
                  </Label>
                  <p className='text-xs text-muted-foreground mt-0.5'>
                    {t(
                      'Varsayılan sıralama: Turnstile -> reCaptcha -> hCaptcha -> Dahili Resim. Bir sağlayıcı hata verirse sıradakine otomatik geçilir. Sıralamayı ok butonlarıyla özelleştirebilirsiniz.'
                    )}
                  </p>
                </div>
                <div className='space-y-2'>
                  {providerOrder.map((providerKey, idx) => (
                    <div
                      key={providerKey}
                      className='flex items-center justify-between rounded-md border bg-card px-3 py-2 text-xs shadow-sm'
                    >
                      <div className='flex items-center gap-2'>
                        <span className='font-mono font-bold text-muted-foreground w-4'>
                          {idx + 1}.
                        </span>
                        <span className='font-medium text-foreground'>
                          {t(
                            providerLabels[
                              providerKey as keyof typeof providerLabels
                            ] || providerKey
                          )}
                        </span>
                        {idx === 0 && (
                          <span className='text-[10px] rounded bg-primary/10 text-primary px-1.5 py-0.5 font-semibold'>
                            {t('İlk Denenen (Varsayılan)')}
                          </span>
                        )}
                      </div>
                      <div className='flex items-center gap-1'>
                        <Button
                          type='button'
                          variant='ghost'
                          size='icon'
                          className='h-6 w-6'
                          disabled={idx === 0}
                          onClick={() => moveOrder(idx, 'up')}
                        >
                          <ArrowUp className='h-3.5 w-3.5' />
                        </Button>
                        <Button
                          type='button'
                          variant='ghost'
                          size='icon'
                          className='h-6 w-6'
                          disabled={idx === providerOrder.length - 1}
                          onClick={() => moveOrder(idx, 'down')}
                        >
                          <ArrowDown className='h-3.5 w-3.5' />
                        </Button>
                      </div>
                    </div>
                  ))}
                </div>
              </div>

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

              <div>
                <Button
                  type='button'
                  variant='outline'
                  onClick={openCaptchaTest}
                >
                  {t('Test captcha')}
                </Button>
                <p className='text-muted-foreground mt-2 text-sm'>
                  {t(
                    'Opens the verification popup for the selected provider, exactly as your users will see it.'
                  )}
                </p>
              </div>
            </>
          )}
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
