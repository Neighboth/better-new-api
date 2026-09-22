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
  fallback_chat_models: z.string().optional().default(''),
  fallback_image_models: z.string().optional().default(''),
  fallback_tts_models: z.string().optional().default(''),
  fallback_stt_models: z.string().optional().default(''),
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
      fallback_chat_models: 'relay_fallback_setting.fallback_chat_models',
      fallback_image_models: 'relay_fallback_setting.fallback_image_models',
      fallback_tts_models: 'relay_fallback_setting.fallback_tts_models',
      fallback_stt_models: 'relay_fallback_setting.fallback_stt_models',
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
            name='fallback_chat_models'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Chat Fallback Models')}</FormLabel>
                <FormControl>
                  <Input
                    {...field}
                    placeholder='gpt-4o-mini, gemini-2.0-flash, claude-3-5-haiku'
                  />
                </FormControl>
                <FormDescription>
                  {t('Fallback models for chat and text completions. If left blank, general fallback models will be used.')}
                </FormDescription>
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='fallback_image_models'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Image Fallback Models')}</FormLabel>
                <FormControl>
                  <Input
                    {...field}
                    placeholder='dall-e-3, flux-schnell, sdxl'
                  />
                </FormControl>
                <FormDescription>
                  {t('Separate fallback models for image generation requests so image models do not fall back to chat models.')}
                </FormDescription>
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='fallback_tts_models'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('TTS (Speech) Fallback Models')}</FormLabel>
                <FormControl>
                  <Input
                    {...field}
                    placeholder='tts-1, tts-1-hd'
                  />
                </FormControl>
                <FormDescription>
                  {t('Fallback models for text-to-speech requests.')}
                </FormDescription>
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='fallback_stt_models'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('STT (Audio Transcription) Fallback Models')}</FormLabel>
                <FormControl>
                  <Input
                    {...field}
                    placeholder='whisper-1'
                  />
                </FormControl>
                <FormDescription>
                  {t('Fallback models for audio transcription and translation requests.')}
                </FormDescription>
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='fallback_models'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('General Fallback Models (Legacy)')}</FormLabel>
                <FormControl>
                  <Input
                    {...field}
                    placeholder='gpt-4o-mini, gemini-2.0-flash'
                  />
                </FormControl>
                <FormDescription>
                  {t('Global fallback models used as default for text chat when specific chat fallback is unset.')}
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
                  {t('Prepended to system message. You can use variables ${modelid}, ${model_id}, or ${model} which will be automatically replaced with the requested model ID.')}
                </FormDescription>
              </FormItem>
            )}
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}