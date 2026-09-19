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
import { Coins, Crown, MessageSquareCode, WalletCards, ShieldAlert } from 'lucide-react'
import type { Resolver } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

import { Alert, AlertDescription } from '@/components/ui/alert'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
} from '@/components/ui/form'
import { Switch } from '@/components/ui/switch'

import { FormDirtyIndicator } from '../components/form-dirty-indicator'
import { FormNavigationGuard } from '../components/form-navigation-guard'
import {
  SettingsForm,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useSettingsForm } from '../hooks/use-settings-form'
import { useUpdateOption } from '../hooks/use-update-option'

const budgetPoolsSchema = z.object({
  EnableBillingRequests: z.boolean(),
  EnableBillingTokens: z.boolean(),
  EnableBillingSubscription: z.boolean(),
  EnableBillingWallet: z.boolean(),
})

export type BudgetPoolsFormValues = z.infer<typeof budgetPoolsSchema>

type BudgetPoolsSectionProps = {
  defaultValues: BudgetPoolsFormValues
}

export function BudgetPoolsSection({ defaultValues }: BudgetPoolsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const { form, handleSubmit, isDirty, isSubmitting } =
    useSettingsForm<BudgetPoolsFormValues>({
      resolver: zodResolver(budgetPoolsSchema) as Resolver<
        BudgetPoolsFormValues,
        unknown,
        BudgetPoolsFormValues
      >,
      defaultValues,
      onSubmit: async (_data, changedFields) => {
        for (const [key, value] of Object.entries(changedFields)) {
          await updateOption.mutateAsync({
            key,
            value: value as boolean,
          })
        }
      },
    })

  return (
    <SettingsSection title={t('Budget Pools Management')}>
      <p className='text-xs text-muted-foreground mb-4'>
        {t(
          'Configure which balance and spending pools are enabled system-wide. Disabling a pool prevents users from consuming quota from that specific balance type.'
        )}
      </p>
      <FormNavigationGuard when={isDirty} />

      <Alert className='border-primary/20 bg-primary/5 mb-4'>
        <ShieldAlert className='h-4 w-4 text-primary shrink-0' />
        <AlertDescription className='text-xs leading-relaxed text-muted-foreground'>
          {t(
            'Users can prioritize their individual pool spending order from their Wallet dashboard. When a pool is disabled here, it is completely bypassed during API settlement.'
          )}
        </AlertDescription>
      </Alert>

      <Form {...form}>
        <SettingsForm onSubmit={handleSubmit}>
          <SettingsPageFormActions
            onSave={handleSubmit}
            isSaving={updateOption.isPending || isSubmitting}
          />
          <FormDirtyIndicator isDirty={isDirty} />

          <div className='grid gap-4 sm:grid-cols-2'>
            {/* Requests Pool */}
            <Card className='border shadow-xs'>
              <CardHeader className='pb-3'>
                <div className='flex items-center justify-between'>
                  <div className='flex items-center gap-2'>
                    <div className='p-2 rounded-lg bg-sky-500/10 text-sky-500'>
                      <MessageSquareCode className='h-5 w-5' />
                    </div>
                    <div>
                      <CardTitle className='text-base'>{t('Requests Pool')}</CardTitle>
                      <CardDescription className='text-xs'>
                        {t('Fixed API Request Count')}
                      </CardDescription>
                    </div>
                  </div>
                  <FormField
                    control={form.control}
                    name='EnableBillingRequests'
                    render={({ field }) => (
                      <FormItem>
                        <FormControl>
                          <Switch
                            checked={field.value}
                            onCheckedChange={field.onChange}
                            disabled={updateOption.isPending}
                          />
                        </FormControl>
                      </FormItem>
                    )}
                  />
                </div>
              </CardHeader>
              <CardContent>
                <p className='text-xs text-muted-foreground leading-relaxed'>
                  {t(
                    'Allows users to spend from their per-request quota balance. 1 successful API call deducts 1 request quota without calculating token prices.'
                  )}
                </p>
              </CardContent>
            </Card>

            {/* Tokens Pool */}
            <Card className='border shadow-xs'>
              <CardHeader className='pb-3'>
                <div className='flex items-center justify-between'>
                  <div className='flex items-center gap-2'>
                    <div className='p-2 rounded-lg bg-emerald-500/10 text-emerald-500'>
                      <Coins className='h-5 w-5' />
                    </div>
                    <div>
                      <CardTitle className='text-base'>{t('Tokens Pool')}</CardTitle>
                      <CardDescription className='text-xs'>
                        {t('Token Count Balances')}
                      </CardDescription>
                    </div>
                  </div>
                  <FormField
                    control={form.control}
                    name='EnableBillingTokens'
                    render={({ field }) => (
                      <FormItem>
                        <FormControl>
                          <Switch
                            checked={field.value}
                            onCheckedChange={field.onChange}
                            disabled={updateOption.isPending}
                          />
                        </FormControl>
                      </FormItem>
                    )}
                  />
                </div>
              </CardHeader>
              <CardContent>
                <p className='text-xs text-muted-foreground leading-relaxed'>
                  {t(
                    'Allows users to consume prompt and completion tokens directly from their dedicated token pool balance.'
                  )}
                </p>
              </CardContent>
            </Card>

            {/* Subscription Pool */}
            <Card className='border shadow-xs'>
              <CardHeader className='pb-3'>
                <div className='flex items-center justify-between'>
                  <div className='flex items-center gap-2'>
                    <div className='p-2 rounded-lg bg-amber-500/10 text-amber-500'>
                      <Crown className='h-5 w-5' />
                    </div>
                    <div>
                      <CardTitle className='text-base'>{t('Subscription Pool')}</CardTitle>
                      <CardDescription className='text-xs'>
                        {t('Active Recurring Plan')}
                      </CardDescription>
                    </div>
                  </div>
                  <FormField
                    control={form.control}
                    name='EnableBillingSubscription'
                    render={({ field }) => (
                      <FormItem>
                        <FormControl>
                          <Switch
                            checked={field.value}
                            onCheckedChange={field.onChange}
                            disabled={updateOption.isPending}
                          />
                        </FormControl>
                      </FormItem>
                    )}
                  />
                </div>
              </CardHeader>
              <CardContent>
                <p className='text-xs text-muted-foreground leading-relaxed'>
                  {t(
                    'Allows recurring subscription plan quotas to be prioritized before standard pay-as-you-go balance.'
                  )}
                </p>
              </CardContent>
            </Card>

            {/* Standard Wallet Pool */}
            <Card className='border shadow-xs'>
              <CardHeader className='pb-3'>
                <div className='flex items-center justify-between'>
                  <div className='flex items-center gap-2'>
                    <div className='p-2 rounded-lg bg-purple-500/10 text-purple-500'>
                      <WalletCards className='h-5 w-5' />
                    </div>
                    <div>
                      <CardTitle className='text-base'>{t('Standard Wallet Pool')}</CardTitle>
                      <CardDescription className='text-xs'>
                        {t('Monetary Quota Balance')}
                      </CardDescription>
                    </div>
                  </div>
                  <FormField
                    control={form.control}
                    name='EnableBillingWallet'
                    render={({ field }) => (
                      <FormItem>
                        <FormControl>
                          <Switch
                            checked={field.value}
                            onCheckedChange={field.onChange}
                            disabled={updateOption.isPending}
                          />
                        </FormControl>
                      </FormItem>
                    )}
                  />
                </div>
              </CardHeader>
              <CardContent>
                <p className='text-xs text-muted-foreground leading-relaxed'>
                  {t(
                    'Standard USD wallet balance loaded via payment gateways, redemption codes, or admin top-ups.'
                  )}
                </p>
              </CardContent>
            </Card>
          </div>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
