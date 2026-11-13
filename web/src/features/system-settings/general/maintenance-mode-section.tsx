/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

For commercial licensing, please contact support@quantumnous.com
*/
import { useTranslation } from 'react-i18next'
import { useEffect, useMemo, useRef } from 'react'
import { useForm } from 'react-hook-form'
import * as z from 'zod'
import { zodResolver } from '@hookform/resolvers/zod'

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
import { Textarea } from '@/components/ui/textarea'

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const maintenanceSchema = z.object({
  maintenanceModeEnabled: z.boolean(),
  maintenanceModeETA: z.string().optional(),
  maintenanceModeMessage: z.string().optional(),
})

type MaintenanceFormValues = z.infer<typeof maintenanceSchema>

type MaintenanceModeSectionProps = {
  defaultValues: {
    MaintenanceModeEnabled: string
    MaintenanceModeETA: string
    MaintenanceModeMessage: string
  }
}

function unixToLocalInput(unix: number): string {
  if (!Number.isFinite(unix) || unix <= 0) return ''
  const d = new Date(unix * 1000)
  if (Number.isNaN(d.getTime())) return ''
  const pad = (n: number) => String(n).padStart(2, '0')
  return (
    d.getFullYear() +
    '-' +
    pad(d.getMonth() + 1) +
    '-' +
    pad(d.getDate()) +
    'T' +
    pad(d.getHours()) +
    ':' +
    pad(d.getMinutes())
  )
}

function localInputToUnix(value: string): number {
  const trimmed = value.trim()
  if (!trimmed) return 0
  const d = new Date(trimmed)
  if (Number.isNaN(d.getTime())) return 0
  return Math.floor(d.getTime() / 1000)
}

export function MaintenanceModeSection({
  defaultValues,
}: MaintenanceModeSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const initialRef = useRef(defaultValues)
  const defaultsSig = useMemo(
    () =>
      JSON.stringify([
        defaultValues.MaintenanceModeEnabled,
        defaultValues.MaintenanceModeETA,
        defaultValues.MaintenanceModeMessage,
      ]),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [
      defaultValues.MaintenanceModeEnabled,
      defaultValues.MaintenanceModeETA,
      defaultValues.MaintenanceModeMessage,
    ],
  )

  const form = useForm<MaintenanceFormValues>({
    resolver: zodResolver(maintenanceSchema),
    defaultValues: {
      maintenanceModeEnabled:
        String(defaultValues.MaintenanceModeEnabled).toLowerCase() === 'true',
      maintenanceModeETA: unixToLocalInput(
        Number(defaultValues.MaintenanceModeETA) || 0,
      ),
      maintenanceModeMessage: defaultValues.MaintenanceModeMessage ?? '',
    },
  })

  useEffect(() => {
    form.reset({
      maintenanceModeEnabled:
        String(defaultValues.MaintenanceModeEnabled).toLowerCase() === 'true',
      maintenanceModeETA: unixToLocalInput(
        Number(defaultValues.MaintenanceModeETA) || 0,
      ),
      maintenanceModeMessage: defaultValues.MaintenanceModeMessage ?? '',
    })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [defaultsSig])

  const onSubmit = async (values: MaintenanceFormValues) => {
    const entries: [string, string][] = [
      [
        'MaintenanceModeEnabled',
        values.maintenanceModeEnabled ? 'true' : 'false',
      ],
      [
        'MaintenanceModeETA',
        String(localInputToUnix(values.maintenanceModeETA ?? '')),
      ],
      ['MaintenanceModeMessage', values.maintenanceModeMessage ?? ''],
    ]

    for (const [key, value] of entries) {
      const initial = initialRef.current as Record<string, unknown>
      if (String(initial[key] ?? '') !== value) {
        await updateOption.mutateAsync({ key, value })
      }
    }
  }

  return (
    <SettingsSection title={t('System maintenance')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
          />

          <FormField
            control={form.control}
            name='maintenanceModeEnabled'
            render={({ field }) => (
              <FormItem>
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel className='text-sm font-semibold'>
                      {t('Enable maintenance mode')}
                    </FormLabel>
                    <FormDescription>
                      {t(
                        'Blocks every visitor with a maintenance screen and answers API calls with 503. Admins can still sign in to disable it.',
                      )}
                    </FormDescription>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </SettingsSwitchItem>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='maintenanceModeETA'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Expected end (optional)')}</FormLabel>
                <FormControl>
                  <Input
                    type='datetime-local'
                    className='max-w-xs'
                    {...field}
                    value={field.value ?? ''}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'If set, the maintenance screen shows the target date and a live countdown for visitors.',
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='maintenanceModeMessage'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Visitor message (optional)')}</FormLabel>
                <FormControl>
                  <Textarea
                    rows={3}
                    placeholder="We'll be back shortly. The installation is undergoing scheduled maintenance."
                    {...field}
                    value={field.value ?? ''}
                  />
                </FormControl>
                <FormDescription>
                  {t('Shown to every visitor on the maintenance screen.')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
