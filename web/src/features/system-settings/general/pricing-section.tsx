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
import { Plus, RefreshCw, Trash2 } from 'lucide-react'
import { useState } from 'react'
import type { Resolver } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

import { Button } from '@/components/ui/button'
import { api } from '@/lib/api'

import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import {
  DEFAULT_CURRENCY_CONFIG,
  type DisplayCurrencyOption,
} from '@/stores/system-config-store'

import { FormDirtyIndicator } from '../components/form-dirty-indicator'
import { FormNavigationGuard } from '../components/form-navigation-guard'
import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useSettingsForm } from '../hooks/use-settings-form'
import { useUpdateOption } from '../hooks/use-update-option'
import { safeNumberFieldProps } from '../utils/numeric-field'

const createPricingSchema = (t: (key: string) => string) =>
  z
    .object({
      QuotaPerUnit: z.coerce.number().min(0, t('Value must be at least 0')),
      USDExchangeRate: z.coerce
        .number()
        .min(0.0001, t('Exchange rate must be greater than 0')),
      DisplayInCurrencyEnabled: z.boolean(),
      DisplayTokenStatEnabled: z.boolean(),
      general_setting: z.object({
        quota_display_type: z.enum(['USD', 'CNY', 'TOKENS', 'CUSTOM']),
        custom_currency_symbol: z.string().max(8).optional(),
        custom_currency_exchange_rate: z.coerce
          .number()
          .min(0.0001, t('Exchange rate must be greater than 0'))
          .optional(),
        auto_exchange_rates: z.boolean().optional(),
        display_currencies: z
          .array(
            z.object({
              code: z.string(),
              symbol: z.string(),
              name: z.string().optional(),
              exchange_rate: z.coerce.number().min(0),
              auto_rate: z.boolean().optional(),
            })
          )
          .optional(),
      }),
    })
    .superRefine((data, ctx) => {
      const displayType = data.general_setting.quota_display_type

      if (displayType === 'CUSTOM') {
        if (!data.general_setting.custom_currency_symbol?.trim()) {
          ctx.addIssue({
            code: z.ZodIssueCode.custom,
            path: ['general_setting', 'custom_currency_symbol'],
            message: t('Custom currency symbol is required'),
          })
        }

        if (data.general_setting.custom_currency_exchange_rate == null) {
          ctx.addIssue({
            code: z.ZodIssueCode.custom,
            path: ['general_setting', 'custom_currency_exchange_rate'],
            message: t('Exchange rate is required'),
          })
        }
      }
    })

type PricingFormValues = z.infer<ReturnType<typeof createPricingSchema>>

type PricingSectionProps = {
  defaultValues: PricingFormValues
}

export function PricingSection({ defaultValues }: PricingSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const pricingSchema = createPricingSchema(t)

  const { form, handleSubmit, handleReset, isDirty, isSubmitting } =
    useSettingsForm<PricingFormValues>({
      resolver: zodResolver(pricingSchema) as Resolver<
        PricingFormValues,
        unknown,
        PricingFormValues
      >,
      defaultValues,
      onSubmit: async (_data, changedFields) => {
        for (const [key, value] of Object.entries(changedFields)) {
          if (value === undefined || value === null) continue
          if (key === 'general_setting.display_currencies') {
            await updateOption.mutateAsync({
              key,
              value: JSON.stringify(value),
            })
            continue
          }
          if (typeof value === 'object') continue

          let serialized: string | boolean = value as string | boolean

          if (typeof value === 'boolean') {
            serialized = String(value)
          } else if (typeof value === 'number') {
            serialized = Number.isFinite(value) ? String(value) : '0'
          }

          await updateOption.mutateAsync({
            key,
            value: serialized,
          })
        }
      },
    })

  const displayType = form.watch('general_setting.quota_display_type') ?? 'USD'
  const displayInCurrencyEnabled = form.watch('DisplayInCurrencyEnabled')
  const displayCurrencies =
    (form.watch('general_setting.display_currencies') as
      | DisplayCurrencyOption[]
      | undefined) ?? []
  const [fetchingRates, setFetchingRates] = useState(false)

  const handleFetchLiveRates = async () => {
    try {
      setFetchingRates(true)
      const res = await api.get('/api/currency/rates?refresh=true')
      if (res.data?.success && res.data?.data) {
        const rates = res.data.data as Record<string, number>
        const updated = displayCurrencies.map((c) => {
          const code = c.code.trim().toUpperCase()
          if (code === 'USD' || code === 'TOKENS') return { ...c, exchange_rate: 1 }
          if (c.auto_rate !== false && rates[code] && rates[code] > 0) {
            return { ...c, exchange_rate: Number(rates[code].toFixed(4)) }
          }
          return c
        })
        form.setValue('general_setting.display_currencies', updated, {
          shouldDirty: true,
        })
        toast.success(t('Live exchange rates updated'))
      } else {
        toast.error(res.data?.message || t('Failed to fetch exchange rates'))
      }
    } catch {
      toast.error(t('Failed to fetch exchange rates'))
    } finally {
      setFetchingRates(false)
    }
  }
  const showTokensOnlyOption = displayType === 'TOKENS'
  const showQuotaPerUnit =
    displayType === 'TOKENS' ||
    defaultValues.QuotaPerUnit !== DEFAULT_CURRENCY_CONFIG.quotaPerUnit
  const showDisplayInCurrencyOption = displayInCurrencyEnabled === false

  return (
    <>
      <FormNavigationGuard when={isDirty} />

      <SettingsSection title={t('Pricing & Display')}>
        <Form {...form}>
          <SettingsForm onSubmit={handleSubmit}>
            <SettingsPageFormActions
              onSave={handleSubmit}
              onReset={handleReset}
              isSaving={updateOption.isPending || isSubmitting}
              isResetDisabled={!isDirty}
            />
            <FormDirtyIndicator isDirty={isDirty} />
            {showQuotaPerUnit && (
              <FormField
                control={form.control}
                name='QuotaPerUnit'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Quota Per Unit')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        step='0.01'
                        value={field.value as number}
                        disabled
                        name={field.name}
                        onBlur={field.onBlur}
                        ref={field.ref}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Number of tokens per unit quota')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            )}

            <FormField
              control={form.control}
              name='general_setting.quota_display_type'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Display Mode')}</FormLabel>
                  <Select
                    items={[
                      { value: 'USD', label: t('USD') },
                      { value: 'CNY', label: t('CNY') },
                      { value: 'CUSTOM', label: t('Custom Currency') },
                      { value: 'TOKENS', label: t('Tokens Only') },
                    ]}
                    value={field.value}
                    onValueChange={field.onChange}
                  >
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue placeholder={t('Select display mode')} />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent alignItemWithTrigger={false}>
                      <SelectGroup>
                        <SelectItem value='USD'>{t('USD')}</SelectItem>
                        <SelectItem value='CNY'>{t('CNY')}</SelectItem>
                        <SelectItem value='CUSTOM'>
                          {t('Custom Currency')}
                        </SelectItem>
                        {showTokensOnlyOption && (
                          <SelectItem value='TOKENS'>
                            {t('Tokens Only')}
                          </SelectItem>
                        )}
                      </SelectGroup>
                    </SelectContent>
                  </Select>
                  <FormDescription>
                    {t('Choose how quota values are shown to users')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            {displayType !== 'TOKENS' && (
              <FormField
                control={form.control}
                name='USDExchangeRate'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {displayType === 'CNY'
                        ? t('CNY per USD')
                        : displayType === 'USD'
                          ? t('USD Exchange Rate')
                          : t('USD Exchange Rate')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        step='0.01'
                        {...safeNumberFieldProps(field)}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Real exchange rate between USD and your payment gateway currency'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            )}

            {displayType === 'CUSTOM' && (
              <div className='grid gap-4 sm:grid-cols-2'>
                <FormField
                  control={form.control}
                  name='general_setting.custom_currency_symbol'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Custom Currency Symbol')}</FormLabel>
                      <FormControl>
                        <Input
                          type='text'
                          value={field.value ?? ''}
                          onChange={field.onChange}
                          name={field.name}
                          onBlur={field.onBlur}
                          ref={field.ref}
                          maxLength={8}
                          placeholder={t('e.g. ¥ or HK$')}
                        />
                      </FormControl>
                      <FormDescription>
                        {t('Prefix used when displaying prices')}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='general_setting.custom_currency_exchange_rate'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Units per USD')}</FormLabel>
                      <FormControl>
                        <Input
                          type='number'
                          step='0.01'
                          value={field.value ?? ''}
                          onChange={(e) =>
                            field.onChange(
                              e.target.value === ''
                                ? undefined
                                : e.target.valueAsNumber
                            )
                          }
                          name={field.name}
                          onBlur={field.onBlur}
                          ref={field.ref}
                          placeholder={t('e.g. 8 means 1 USD = 8 units')}
                        />
                      </FormControl>
                      <FormDescription>
                        {t('Conversion rate from USD to your custom currency')}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
            )}

            {showDisplayInCurrencyOption && (
              <FormField
                control={form.control}
                name='DisplayInCurrencyEnabled'
                render={({ field }) => (
                  <SettingsSwitchItem>
                    <SettingsSwitchContent>
                      <FormLabel>{t('Display in Currency')}</FormLabel>
                      <FormDescription>
                        {displayType === 'TOKENS'
                          ? t(
                              'Tokens-only mode will show raw quota values regardless of this toggle.'
                            )
                          : t('Show prices in currency instead of quota.')}
                      </FormDescription>
                    </SettingsSwitchContent>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                  </SettingsSwitchItem>
                )}
              />
            )}

            <FormField
              control={form.control}
              name='DisplayTokenStatEnabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Display Token Statistics')}</FormLabel>
                    <FormDescription>
                      {t('Show token usage statistics in the UI')}
                    </FormDescription>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </SettingsSwitchItem>
              )}
            />

            <div className='space-y-4 rounded-xl border p-4'>
              <div className='flex flex-wrap items-center justify-between gap-2'>
                <div>
                  <h4 className='text-sm font-medium'>
                    {t('Multi-Currency Display Options')}
                  </h4>
                  <p className='text-muted-foreground text-xs'>
                    {t(
                      'Configure multiple display currencies for the header switcher. Leave empty to use the single default currency above.'
                    )}
                  </p>
                </div>
                <div className='flex items-center gap-2'>
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    disabled={fetchingRates || displayCurrencies.length === 0}
                    onClick={() => void handleFetchLiveRates()}
                  >
                    <RefreshCw
                      className={`mr-1.5 h-3.5 w-3.5 ${fetchingRates ? 'animate-spin' : ''}`}
                    />
                    {t('Fetch Live Rates')}
                  </Button>
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    onClick={() => {
                      const next =
                        displayCurrencies.length === 0
                          ? [
                              {
                                code: 'USD',
                                symbol: '$',
                                name: 'US Dollar',
                                exchange_rate: 1,
                                auto_rate: true,
                              },
                              {
                                code: 'TRY',
                                symbol: '₺',
                                name: 'Turkish Lira',
                                exchange_rate: 0,
                                auto_rate: true,
                              },
                            ]
                          : [
                              ...displayCurrencies,
                              {
                                code: '',
                                symbol: '',
                                name: '',
                                exchange_rate: 0,
                                auto_rate: true,
                              },
                            ]
                      form.setValue('general_setting.display_currencies', next, {
                        shouldDirty: true,
                      })
                    }}
                  >
                    <Plus className='mr-1.5 h-3.5 w-3.5' />
                    {t('Add Currency')}
                  </Button>
                </div>
              </div>

              <FormField
                control={form.control}
                name='general_setting.auto_exchange_rates'
                render={({ field }) => (
                  <SettingsSwitchItem>
                    <SettingsSwitchContent>
                      <FormLabel>{t('Auto-Fetch Exchange Rates')}</FormLabel>
                      <FormDescription>
                        {t(
                          'Automatically update currency exchange rates from public live rates API.'
                        )}
                      </FormDescription>
                    </SettingsSwitchContent>
                    <FormControl>
                      <Switch
                        checked={field.value ?? true}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                  </SettingsSwitchItem>
                )}
              />

              {displayCurrencies.length > 0 && (
                <div className='space-y-2'>
                  {displayCurrencies.map((item, idx) => (
                    <div
                      key={idx}
                      className='grid grid-cols-12 items-center gap-2 rounded-lg border bg-muted/20 p-2.5'
                    >
                      <div className='col-span-6 sm:col-span-2'>
                        <Input
                          placeholder='USD / TRY'
                          value={item.code}
                          onChange={(e) => {
                            const next = [...displayCurrencies]
                            next[idx] = {
                              ...next[idx],
                              code: e.target.value.toUpperCase(),
                            }
                            form.setValue('general_setting.display_currencies', next, {
                              shouldDirty: true,
                            })
                          }}
                          className='h-8 font-mono text-xs'
                        />
                      </div>
                      <div className='col-span-6 sm:col-span-2'>
                        <Input
                          placeholder='$ / ₺'
                          value={item.symbol}
                          onChange={(e) => {
                            const next = [...displayCurrencies]
                            next[idx] = { ...next[idx], symbol: e.target.value }
                            form.setValue('general_setting.display_currencies', next, {
                              shouldDirty: true,
                            })
                          }}
                          className='h-8 text-xs'
                        />
                      </div>
                      <div className='col-span-6 sm:col-span-3'>
                        <Input
                          placeholder={t('Display Name')}
                          value={item.name ?? ''}
                          onChange={(e) => {
                            const next = [...displayCurrencies]
                            next[idx] = { ...next[idx], name: e.target.value }
                            form.setValue('general_setting.display_currencies', next, {
                              shouldDirty: true,
                            })
                          }}
                          className='h-8 text-xs'
                        />
                      </div>
                      <div className='col-span-6 sm:col-span-2'>
                        <Input
                          type='number'
                          step='0.0001'
                          placeholder='1.0'
                          value={item.exchange_rate ?? 0}
                          disabled={item.auto_rate !== false}
                          onChange={(e) => {
                            const next = [...displayCurrencies]
                            next[idx] = {
                              ...next[idx],
                              exchange_rate: Number(e.target.value) || 0,
                            }
                            form.setValue('general_setting.display_currencies', next, {
                              shouldDirty: true,
                            })
                          }}
                          className='h-8 font-mono text-xs'
                        />
                      </div>
                      <div className='col-span-9 sm:col-span-2 flex items-center gap-1.5'>
                        <Switch
                          checked={item.auto_rate !== false}
                          onCheckedChange={(checked) => {
                            const next = [...displayCurrencies]
                            next[idx] = { ...next[idx], auto_rate: checked }
                            form.setValue('general_setting.display_currencies', next, {
                              shouldDirty: true,
                            })
                          }}
                        />
                        <span className='text-muted-foreground text-[11px]'>
                          {t('Auto')}
                        </span>
                      </div>
                      <div className='col-span-3 sm:col-span-1 flex justify-end'>
                        <Button
                          type='button'
                          variant='ghost'
                          size='icon'
                          className='h-8 w-8 text-destructive'
                          onClick={() => {
                            const next = displayCurrencies.filter(
                              (_, i) => i !== idx
                            )
                            form.setValue(
                              'general_setting.display_currencies',
                              next,
                              { shouldDirty: true }
                            )
                          }}
                        >
                          <Trash2 className='h-3.5 w-3.5' />
                        </Button>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </SettingsForm>
        </Form>
      </SettingsSection>
    </>
  )
}
