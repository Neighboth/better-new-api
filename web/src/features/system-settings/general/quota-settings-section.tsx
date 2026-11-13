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
import { useQuery } from '@tanstack/react-query'
import { zodResolver } from '@hookform/resolvers/zod'
import type { ChangeEvent } from 'react'
import type { Resolver } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

import { getAllChannels } from '@/features/channels/api'
import { MultiSelect } from '@/components/multi-select'

import { Alert, AlertDescription } from '@/components/ui/alert'
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
import { Switch } from '@/components/ui/switch'
import { formatQuota } from '@/lib/format'

import { FormDirtyIndicator } from '../components/form-dirty-indicator'
import { FormNavigationGuard } from '../components/form-navigation-guard'
import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
  SettingsFormGrid,
  SettingsFormGridItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useSettingsForm } from '../hooks/use-settings-form'
import { useUpdateOption } from '../hooks/use-update-option'

const quotaSchema = z.object({
  QuotaForNewUser: z.coerce.number().min(0),
  PreConsumedQuota: z.coerce.number().min(0),
  TopUpLink: z.string(),
  quota_setting: z.object({
    enable_free_model_pre_consume: z.boolean(),
    always_cache_hit: z.boolean().optional(),
  }),
  referral_setting: z.object({
    enabled: z.boolean(),
    invitee_real: z.coerce.number().min(0),
    invitee_display: z.coerce.number().min(0),
    inviter_real: z.coerce.number().min(0),
    inviter_display: z.coerce.number().min(0),
    excluded_channel_ids: z.array(z.number()),
  }),
})

type QuotaFormValues = z.infer<typeof quotaSchema>
type QuotaInputValue = number | ''

function formatQuotaInputValue(value: QuotaInputValue): string {
  return formatQuota(value === '' ? 0 : value)
}

type QuotaSettingsSectionProps = {
  defaultValues: QuotaFormValues
  complianceConfirmed?: boolean
}

export function QuotaSettingsSection({
  defaultValues,
  complianceConfirmed = true,
}: QuotaSettingsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const handleNumberChange =
    (onChange: (value: QuotaInputValue) => void) =>
    (event: ChangeEvent<HTMLInputElement>) => {
      const value = event.currentTarget.valueAsNumber
      onChange(Number.isNaN(value) ? '' : value)
    }

  const { form, handleSubmit, isDirty, isSubmitting } =
    useSettingsForm<QuotaFormValues>({
      resolver: zodResolver(quotaSchema) as Resolver<
        QuotaFormValues,
        unknown,
        QuotaFormValues
      >,
      defaultValues,
      onSubmit: async (_data, changedFields) => {
        for (const [key, value] of Object.entries(changedFields)) {
          // The channel exclusion list is an array of ids: serialize it as
          // JSON, the backend decodes it into a typed []int.
          if (key === 'referral_setting.excluded_channel_ids') {
            await updateOption.mutateAsync({
              key,
              value: JSON.stringify(
                (Array.isArray(value) ? value : []).map((v) => Number(v))
              ),
            })
            continue
          }
          if (key.startsWith('referral_setting.')) {
            // The referral amounts are stored in raw quota units.
            if (typeof value === 'number') {
              await updateOption.mutateAsync({ key, value })
              continue
            }
            await updateOption.mutateAsync({
              key,
              value: String(value),
            })
            continue
          }
          await updateOption.mutateAsync({
            key,
            value: value as string | number | boolean,
          })
        }
      },
    })

  const referralEnabled = form.watch('referral_setting.enabled')

  // Channels for the referral-balance exclusion: the list is needed whenever
  // referral grants happen, with or without the display inflation.
  const { data: channelsData } = useQuery({
    queryKey: ['quota-channels-for-referral'],
    queryFn: () => getAllChannels(),
    staleTime: 5 * 60_000,
  })

  const channelOptions = (
    (channelsData as { data?: { items?: { id: number; name: string }[] } } |
      undefined)?.data?.items ?? []
  ).map((channel) => ({
    value: String(channel.id),
    label: `#${channel.id} · ${channel.name}`,
  }))

  return (
    <SettingsSection title={t('Quota Settings')}>
      <FormNavigationGuard when={isDirty} />

      {!complianceConfirmed ? (
        <Alert variant='destructive'>
          <AlertDescription>
            {t(
              'Non-zero invitation rewards require compliance confirmation in Payment Gateway settings.'
            )}
          </AlertDescription>
        </Alert>
      ) : null}

      <Form {...form}>
        <SettingsForm onSubmit={handleSubmit}>
          <SettingsPageFormActions
            onSave={handleSubmit}
            isSaving={updateOption.isPending || isSubmitting}
          />
          <FormDirtyIndicator isDirty={isDirty} />
          <SettingsFormGrid>
            <FormField
              control={form.control}
              name='QuotaForNewUser'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('New User Quota')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      value={field.value ?? ''}
                      onChange={handleNumberChange(field.onChange)}
                      name={field.name}
                      onBlur={field.onBlur}
                      ref={field.ref}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Initial quota given to new users ({{formattedQuota}})',
                      {
                        formattedQuota: formatQuotaInputValue(field.value),
                      }
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='PreConsumedQuota'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Pre-Consumed Quota')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      value={field.value ?? ''}
                      onChange={handleNumberChange(field.onChange)}
                      name={field.name}
                      onBlur={field.onBlur}
                      ref={field.ref}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Quota consumed before charging users')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <SettingsFormGridItem span='full'>
              <div className='space-y-4'>
                <FormField
                  control={form.control}
                  name='referral_setting.enabled'
                  render={({ field }) => (
                    <SettingsSwitchItem className='py-0'>
                      <SettingsSwitchContent>
                        <FormLabel>{t('Referral reward enhancement')}</FormLabel>
                        <FormDescription>
                          {t(
                            'Invitation rewards grant a separate referral balance instead of plain wallet quota. The wallet shows the display amount; every settlement spends the real balance. Channels can be excluded from spending this balance below.'
                          )}
                        </FormDescription>
                      </SettingsSwitchContent>
                      <FormControl>
                        <Switch
                          checked={field.value}
                          onCheckedChange={field.onChange}
                          disabled={updateOption.isPending}
                        />
                      </FormControl>
                    </SettingsSwitchItem>
                  )}
                />
                <div className='space-y-4'>
                    <div className='grid gap-4 sm:grid-cols-2'>
                      <FormField
                        control={form.control}
                        name='referral_setting.invitee_real'
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('Balance for the invited user')}</FormLabel>
                            <FormControl>
                              <Input
                                type='number'
                                min={0}
                                value={field.value ?? 0}
                                onChange={handleNumberChange(field.onChange)}
                                name={field.name}
                                onBlur={field.onBlur}
                                ref={field.ref}
                              />
                            </FormControl>
                            <FormDescription>
                              {t(
                                'Real balance the invited user is granted ({{formattedQuota}})',
                                {
                                  formattedQuota: formatQuotaInputValue(field.value),
                                }
                              )}
                            </FormDescription>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                      <FormField
                        control={form.control}
                        name='referral_setting.invitee_display'
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('Balance to show for the same reward')}</FormLabel>
                            <FormControl>
                              <Input
                                type='number'
                                min={0}
                                disabled={!referralEnabled}
                                value={field.value ?? 0}
                                onChange={handleNumberChange(field.onChange)}
                                name={field.name}
                                onBlur={field.onBlur}
                                ref={field.ref}
                              />
                            </FormControl>
                            <FormDescription>
                              {referralEnabled
                                ? t(
                                    'Display balance shown for the same reward ({{formattedQuota}})',
                                    {
                                      formattedQuota: formatQuotaInputValue(field.value),
                                    }
                                  )
                                : t(
                                    'Display balance matches the real balance while the enhancement is off'
                                  )}
                            </FormDescription>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                    </div>

                    <div className='grid gap-4 sm:grid-cols-2'>
                      <FormField
                        control={form.control}
                        name='referral_setting.inviter_real'
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('Balance for the inviter')}</FormLabel>
                            <FormControl>
                              <Input
                                type='number'
                                min={0}
                                value={field.value ?? 0}
                                onChange={handleNumberChange(field.onChange)}
                                name={field.name}
                                onBlur={field.onBlur}
                                ref={field.ref}
                              />
                            </FormControl>
                            <FormDescription>
                              {t(
                                'Real balance the inviter is granted ({{formattedQuota}})',
                                {
                                  formattedQuota: formatQuotaInputValue(field.value),
                                }
                              )}
                            </FormDescription>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                      <FormField
                        control={form.control}
                        name='referral_setting.inviter_display'
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('Balance to show for the same reward')}</FormLabel>
                            <FormControl>
                              <Input
                                type='number'
                                min={0}
                                disabled={!referralEnabled}
                                value={field.value ?? 0}
                                onChange={handleNumberChange(field.onChange)}
                                name={field.name}
                                onBlur={field.onBlur}
                                ref={field.ref}
                              />
                            </FormControl>
                            <FormDescription>
                              {referralEnabled
                                ? t(
                                    'Display balance shown to the inviter for the same reward ({{formattedQuota}})',
                                    {
                                      formattedQuota: formatQuotaInputValue(field.value),
                                    }
                                  )
                                : t(
                                    'Display balance matches the real balance while the enhancement is off'
                                  )}
                            </FormDescription>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                    </div>

                    <FormField
                      control={form.control}
                      name='referral_setting.excluded_channel_ids'
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>{t('Channels the referral balance may not use')}</FormLabel>
                          <FormControl>
                            <MultiSelect
                              emptyText={t('No channel matches the search')}
                              options={channelOptions}
                              placeholder={t('Search channels...')}
                              selected={(field.value ?? []).map(String)}
                              onChange={(values) =>
                                field.onChange(values.map((v) => Number(v)))
                              }
                            />
                          </FormControl>
                          <FormDescription>
                            {t(
                              'Requests funded by the referral balance never take these channels, not even as a fallback. Paid balance is unaffected.'
                            )}
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                </div>
              </div>
            </SettingsFormGridItem>

            <SettingsFormGridItem span='full'>
              <FormField
                control={form.control}
                name='quota_setting.enable_free_model_pre_consume'
                render={({ field }) => (
                  <SettingsSwitchItem>
                    <SettingsSwitchContent>
                      <FormLabel>{t('Pre-Consume for Free Models')}</FormLabel>
                      <FormDescription>
                        {t(
                          'When enabled, zero-cost models also pre-consume quota before final settlement.'
                        )}
                      </FormDescription>
                    </SettingsSwitchContent>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                        disabled={updateOption.isPending}
                      />
                    </FormControl>
                  </SettingsSwitchItem>
                )}
              />
            </SettingsFormGridItem>

            <SettingsFormGridItem span='full'>
              <FormField
                control={form.control}
                name='quota_setting.always_cache_hit'
                render={({ field }) => (
                  <SettingsSwitchItem>
                    <SettingsSwitchContent>
                      <FormLabel>{t('Always Cache Hit')}</FormLabel>
                      <FormDescription>
                        {t(
                          'When enabled, even if the upstream channel does not support caching, the system charges the discounted cache rate for cacheable prompts.'
                        )}
                      </FormDescription>
                    </SettingsSwitchContent>
                    <FormControl>
                      <Switch
                        checked={field.value ?? false}
                        onCheckedChange={field.onChange}
                        disabled={updateOption.isPending}
                      />
                    </FormControl>
                  </SettingsSwitchItem>
                )}
              />
            </SettingsFormGridItem>

            <FormField
              control={form.control}
              name='TopUpLink'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Top-Up Link')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t('https://example.com/topup')}
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('External link for users to purchase quota')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </SettingsFormGrid>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
