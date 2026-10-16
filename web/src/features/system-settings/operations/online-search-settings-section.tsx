import { zodResolver } from '@hookform/resolvers/zod'
import { useForm, type Resolver } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

import { Input } from '@/components/ui/input'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'

import {
  SettingsForm,
  SettingsFormGrid,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const schema = z.object({
  searxngUrl: z.string().optional(),
  onlineSearchPrice: z.coerce.number().min(0).max(1000).optional(),
  onlineSearchMax: z.coerce.number().int().min(1).max(10).optional(),
})

type Values = z.infer<typeof schema>

export function OnlineSearchSettingsSection({
  defaultValues,
}: {
  defaultValues: {
    searxngUrl?: string
    onlineSearchPrice?: number
    onlineSearchMax?: number
  }
}) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const form = useForm<Values>({
    resolver: zodResolver(schema) as unknown as Resolver<Values>,
    defaultValues: {
      searxngUrl: defaultValues.searxngUrl ?? '',
      onlineSearchPrice: defaultValues.onlineSearchPrice ?? 0,
      onlineSearchMax: defaultValues.onlineSearchMax ?? 5,
    },
  })

  const onSubmit = async (values: Values) => {
    try {
      await updateOption.mutateAsync({
        key: 'online_search_setting.searxng_url',
        value: String(values.searxngUrl ?? ''),
      })
      await updateOption.mutateAsync({
        key: 'online_search_setting.price_per_search',
        value: String(values.onlineSearchPrice ?? 0),
      })
      await updateOption.mutateAsync({
        key: 'online_search_setting.max_searches',
        value: String(values.onlineSearchMax ?? 5),
      })

      form.reset(values)
      toast.success(t('Online search settings saved successfully'))
    } catch {
      toast.error(t('Failed to save settings'))
    }
  }

  return (
    <SettingsSection title={t('Online Search (:online)')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsFormGrid>
            <FormField
              control={form.control}
              name='searxngUrl'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('SearXNG URL (:online)')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder='https://searxng.ornek.com'
                      {...field}
                      value={field.value ?? ''}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Self-hosted SearXNG instance for :online models. Leave empty to disable. Example: https://searx.be')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='onlineSearchPrice'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Price per :online search (USD)')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      step='0.001'
                      min={0}
                      placeholder='0.002'
                      {...field}
                      value={field.value ?? 0}
                      onChange={(e) => field.onChange(Number(e.target.value))}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Deducted from user balance per search result call.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='onlineSearchMax'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Max :online searches')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={1}
                      max={10}
                      {...field}
                      value={field.value ?? 5}
                      onChange={(e) => field.onChange(Number(e.target.value))}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Maximum SearXNG results per request (1-10).')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </SettingsFormGrid>

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
