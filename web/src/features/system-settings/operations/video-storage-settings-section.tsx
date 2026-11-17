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
import { useEffect } from 'react'
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
import { Input } from '@/components/ui/input'

import {
  SettingsForm,
} from '../components/settings-form-layout'
import {
  SettingsSection,
} from '../components/settings-section'
import { useUpdateOptionsBulk } from '../hooks/use-update-option'

const schema = z.object({
  maxVideoQuotaGb: z.coerce.number().min(0.1).max(4096),
  retentionDays: z.coerce.number().int().min(1).max(3650),
  expiryWarnDaysBefore: z.coerce.number().int().min(0).max(365),
  maxVideoAssetBytes: z.coerce.number().int().min(1024 * 1024),
})

type Values = z.infer<typeof schema>

const BYTES_PER_GB = 1024 * 1024 * 1024

export function VideoStorageSettingsSection({
  defaultValues,
}: {
  defaultValues: {
    maxVideoQuotaBytes: number
    retentionDays: number
    expiryWarnDaysBefore: number
    maxVideoAssetBytes: number
  }
}) {
  const { t } = useTranslation()
  const updateOptions = useUpdateOptionsBulk()

  const form = useForm<Values>({
    resolver: zodResolver(schema) as unknown as Resolver<Values>,
    defaultValues: {
      maxVideoQuotaGb: Math.max(0.1, defaultValues.maxVideoQuotaBytes / BYTES_PER_GB),
      retentionDays: defaultValues.retentionDays,
      expiryWarnDaysBefore: defaultValues.expiryWarnDaysBefore,
      maxVideoAssetBytes: defaultValues.maxVideoAssetBytes,
    },
  })

  useEffect(() => {
    form.reset({
      maxVideoQuotaGb: Math.max(0.1, defaultValues.maxVideoQuotaBytes / BYTES_PER_GB),
      retentionDays: defaultValues.retentionDays,
      expiryWarnDaysBefore: defaultValues.expiryWarnDaysBefore,
      maxVideoAssetBytes: defaultValues.maxVideoAssetBytes,
    })
  }, [defaultValues, form])

  const onSubmit = async (values: Values) => {
    try {
      await updateOptions.mutateAsync({
        'video_storage_setting.max_video_quota_bytes': Math.round(values.maxVideoQuotaGb * BYTES_PER_GB),
        'video_storage_setting.retention_days': values.retentionDays,
        'video_storage_setting.expiry_warn_days_before': values.expiryWarnDaysBefore,
        'video_storage_setting.max_video_asset_bytes': values.maxVideoAssetBytes,
      })
      form.reset(values)
      toast.success(t('Video storage settings saved successfully'))
    } catch {
      toast.error(t('Failed to save settings'))
    }
  }

  return (
    <SettingsSection title={t('Video Storage')}>
      <p className='-mt-2 text-xs text-muted-foreground'>
        {t('Server-side storage for generated videos: per-user quota, retention and expiry warning. Over-quota generations are kept in the browser (IndexedDB) instead of the server.')}
      </p>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <div className='grid gap-4 sm:grid-cols-2'>
            <FormField
              control={form.control}
              name='maxVideoQuotaGb'
              render={({ field }) => (
                <div className='space-y-1'>
                  <FormLabel>{t('Per-user quota (GB)')}</FormLabel>
                  <FormControl>
                    <Input type='number' step='0.1' min={0.1} {...field} />
                  </FormControl>
                  <FormDescription>
                    {t('New videos beyond this limit are kept in the browser instead of the server.')}
                  </FormDescription>
                </div>
              )}
            />
            <FormField
              control={form.control}
              name='retentionDays'
              render={({ field }) => (
                <div className='space-y-1'>
                  <FormLabel>{t('Retention (days)')}</FormLabel>
                  <FormControl>
                    <Input type='number' min={1} {...field} />
                  </FormControl>
                </div>
              )}
            />
            <FormField
              control={form.control}
              name='expiryWarnDaysBefore'
              render={({ field }) => (
                <div className='space-y-1'>
                  <FormLabel>{t('Expiry warning (days before)')}</FormLabel>
                  <FormControl>
                    <Input type='number' min={0} {...field} />
                  </FormControl>
                </div>
              )}
            />
            <FormField
              control={form.control}
              name='maxVideoAssetBytes'
              render={({ field }) => (
                <div className='space-y-1'>
                  <FormLabel>{t('Maximum single video size (bytes)')}</FormLabel>
                  <FormControl>
                    <Input type='number' min={1024 * 1024} {...field} />
                  </FormControl>
                </div>
              )}
            />
          </div>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
