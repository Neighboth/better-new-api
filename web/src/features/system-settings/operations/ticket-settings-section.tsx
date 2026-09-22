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
import { useForm, type Resolver } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormLabel,
} from '@/components/ui/form'
import { Switch } from '@/components/ui/switch'

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const schema = z.object({
  enabled: z.boolean(),
  liveSupportEnabled: z.boolean(),
  notifyAdminOnNewTicket: z.boolean(),
  notifyUserOnReply: z.boolean(),
})

type Values = z.infer<typeof schema>

export function TicketSettingsSection({
  defaultValues,
}: {
  defaultValues: {
    enabled: boolean
    liveSupportEnabled: boolean
    notifyAdminOnNewTicket: boolean
    notifyUserOnReply: boolean
  }
}) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const form = useForm<Values>({
    resolver: zodResolver(schema) as unknown as Resolver<Values>,
    defaultValues: {
      enabled: defaultValues.enabled,
      liveSupportEnabled: defaultValues.liveSupportEnabled,
      notifyAdminOnNewTicket: defaultValues.notifyAdminOnNewTicket,
      notifyUserOnReply: defaultValues.notifyUserOnReply,
    },
  })

  const onSubmit = async (values: Values) => {
    try {
      await updateOption.mutateAsync({
        key: 'ticket_setting.enabled',
        value: values.enabled,
      })
      await updateOption.mutateAsync({
        key: 'ticket_setting.live_support_enabled',
        value: values.liveSupportEnabled,
      })
      await updateOption.mutateAsync({
        key: 'ticket_setting.notify_admin_on_new_ticket',
        value: values.notifyAdminOnNewTicket,
      })
      await updateOption.mutateAsync({
        key: 'ticket_setting.notify_user_on_reply',
        value: values.notifyUserOnReply,
      })

      form.reset(values)
      toast.success(t('Ticket and support settings saved successfully'))
    } catch {
      toast.error(t('Failed to save settings'))
    }
  }

  return (
    <SettingsSection title={t('Support & Tickets')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <FormField
            control={form.control}
            name='enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable Support Ticket System')}</FormLabel>
                  <FormDescription>
                    {t('Allow registered users to open and manage support tickets from their dashboard.')}
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

          <FormField
            control={form.control}
            name='liveSupportEnabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable Live Chat Widget')}</FormLabel>
                  <FormDescription>
                    {t('Display a floating live support widget in the bottom-right corner for quick assistance.')}
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

          <FormField
            control={form.control}
            name='notifyAdminOnNewTicket'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Notify Admin on New Ticket')}</FormLabel>
                  <FormDescription>
                    {t('Send an email notification to the system administrator whenever a new ticket is opened.')}
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

          <FormField
            control={form.control}
            name='notifyUserOnReply'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Notify User on Support Reply')}</FormLabel>
                  <FormDescription>
                    {t('Send an email notification to the user when support responds to their ticket.')}
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

          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            isSaveDisabled={!form.formState.isDirty}
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
