import { zodResolver } from '@hookform/resolvers/zod'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

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
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'

import {
  SettingsForm,
  SettingsFormGrid,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const schema = z.object({
  enabled: z.boolean(),
  tokenId: z.string().optional(),
  tokenSecret: z.string().optional(),
  workspace: z.string().optional(),
  sharedVolumePath: z.string().optional(),
  idleTimeoutSeconds: z.number().min(0).optional(),
  priority: z.string().optional(),
})

type Values = z.infer<typeof schema>

export function ModalDeploymentSettingsSection({
  defaultValues,
}: {
  defaultValues: {
    enabled?: boolean
    tokenId?: string
    tokenSecret?: string
    workspace?: string
    sharedVolumePath?: string
    idleTimeoutSeconds?: number
    priority?: string
  }
}) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: {
      enabled: defaultValues.enabled ?? false,
      tokenId: defaultValues.tokenId ?? '',
      tokenSecret: defaultValues.tokenSecret ?? '',
      workspace: defaultValues.workspace ?? '',
      sharedVolumePath: defaultValues.sharedVolumePath ?? '/vol/models',
      idleTimeoutSeconds: defaultValues.idleTimeoutSeconds ?? 300,
      priority: defaultValues.priority ?? 'modal_first',
    },
  })

  const { isDirty, isSubmitting } = form.formState

  async function onSubmit(values: Values) {
    try {
      await updateOption.mutateAsync({
        key: 'model_deployment.modal.enabled',
        value: String(values.enabled),
      })
      await updateOption.mutateAsync({
        key: 'model_deployment.modal.token_id',
        value: String(values.tokenId ?? ''),
      })
      await updateOption.mutateAsync({
        key: 'model_deployment.modal.token_secret',
        value: String(values.tokenSecret ?? ''),
      })
      await updateOption.mutateAsync({
        key: 'model_deployment.modal.workspace',
        value: String(values.workspace ?? ''),
      })
      await updateOption.mutateAsync({
        key: 'model_deployment.modal.shared_volume_path',
        value: String(values.sharedVolumePath ?? '/vol/models'),
      })
      await updateOption.mutateAsync({
        key: 'model_deployment.modal.idle_timeout_seconds',
        value: String(values.idleTimeoutSeconds ?? 300),
      })
      await updateOption.mutateAsync({
        key: 'model_deployment.modal.priority',
        value: String(values.priority ?? 'modal_first'),
      })

      form.reset(values)
      toast.success(t('Modal deployment settings saved successfully'))
    } catch {
      toast.error(t('Failed to save Modal deployment settings'))
    }
  }

  return (
    <SettingsSection title={t('Modal GPU Serverless Deployment')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <FormField
            control={form.control}
            name='enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable Modal GPU Deployments')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Deploy open-source LLMs, diffusion, and video generation directly on Modal serverless GPU infrastructure.'
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
            )}
          />

          <SettingsFormGrid>
            <FormField
              control={form.control}
              name='tokenId'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Modal Token ID')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder='ak-...'
                      {...field}
                      value={field.value ?? ''}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Your Modal API Token ID (MODAL_TOKEN_ID)')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='tokenSecret'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Modal Token Secret')}</FormLabel>
                  <FormControl>
                    <Input
                      type='password'
                      placeholder='as-...'
                      {...field}
                      value={field.value ?? ''}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Your Modal API Token Secret (MODAL_TOKEN_SECRET)')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='workspace'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Workspace / Environment')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder='main'
                      {...field}
                      value={field.value ?? ''}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Modal workspace or environment name (default: main)')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='sharedVolumePath'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Shared Storage Volume Path')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder='/vol/models'
                      {...field}
                      value={field.value ?? ''}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      '1TB persistent shared volume path so containers do not redownload weights per request and save GPU costs'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='idleTimeoutSeconds'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Inactive Sleep Timeout (Seconds)')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      placeholder='300'
                      value={field.value ?? 300}
                      onChange={(e) => field.onChange(Number(e.target.value))}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Puts GPU containers to sleep when unused for this time to eliminate idle billing costs.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='priority'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Routing Priority')}</FormLabel>
                  <Select
                    value={field.value ?? 'modal_first'}
                    onValueChange={field.onChange}
                  >
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      <SelectItem value='modal_first'>
                        {t('Prioritize Modal GPU over Channels')}
                      </SelectItem>
                      <SelectItem value='channel_first'>
                        {t('Prioritize Upstream Channels over Modal')}
                      </SelectItem>
                      <SelectItem value='fallback'>
                        {t('Use Modal as Fallback Only')}
                      </SelectItem>
                    </SelectContent>
                  </Select>
                  <FormDescription>
                    {t('Select whether requests route to Modal first or upstream channels')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </SettingsFormGrid>

          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={isSubmitting || updateOption.isPending}
            isSaveDisabled={!isDirty}
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
