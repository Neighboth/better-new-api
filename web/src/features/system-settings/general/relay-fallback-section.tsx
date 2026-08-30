import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { useForm } from 'react-hook-form'
import * as z from 'zod'

import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
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
import { useResetForm } from '../hooks/use-reset-form'
import { useUpdateOption } from '../hooks/use-update-option'

const fallbackSchema = z.object({
  enable_fallback: z.boolean(),
  fallback_models: z.string(),
  fallback_system_prompt: z.string(),
})

type FallbackFormValues = z.infer<typeof fallbackSchema>

type RelayFallbackSectionProps = {
  defaultValues: FallbackFormValues
}

export function RelayFallbackSection({
  defaultValues,
}: RelayFallbackSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const form = useForm({
    resolver: zodResolver(fallbackSchema),
    defaultValues,
  })

  useResetForm(form, defaultValues)

  const onSubmit = async (data: FallbackFormValues) => {
    const keyMap: Record<keyof FallbackFormValues, string> = {
      enable_fallback: 'relay_fallback_setting.enable_fallback',
      fallback_models: 'relay_fallback_setting.fallback_models',
      fallback_system_prompt: 'relay_fallback_setting.fallback_system_prompt',
    }

    const updates = Object.entries(data).filter(
      ([key, value]) => value !== defaultValues[key as keyof FallbackFormValues]
    )

    for (const [key, value] of updates) {
      const optionKey = keyMap[key as keyof FallbackFormValues]
      await updateOption.mutateAsync({ key: optionKey, value })
    }
  }

  return (
    <SettingsSection title={t('Relay Fallback')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
          />
          <FormField
            control={form.control}
            name='enable_fallback'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable Relay Fallback')}</FormLabel>
                  <FormDescription>
                    {t('When the requested model fails or has no price configured, try the fallback models below.')}
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
            name='fallback_models'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Fallback Models')}</FormLabel>
                <FormControl>
                  <Input
                    {...field}
                    placeholder='gpt-4o-mini, gemini-2.0-flash'
                  />
                </FormControl>
                <FormDescription>
                  {t('Comma-separated model names attempted in order after the original model.')}
                </FormDescription>
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='fallback_system_prompt'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Fallback System Prompt')}</FormLabel>
                <FormControl>
                  <Textarea
                    {...field}
                    rows={4}
                    placeholder={t('Optional system prompt applied to every relay attempt')}
                  />
                </FormControl>
                <FormDescription>
                  {t('Prepended to the system message; useful for routing or safety constraints.')}
                </FormDescription>
              </FormItem>
            )}
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}