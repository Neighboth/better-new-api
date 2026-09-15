import { zodResolver } from '@hookform/resolvers/zod'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

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

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import { useUpdateOption } from '../hooks/use-update-option'

const searchSchema = z.object({
  PlaygroundSearchFallbackEnabled: z.boolean(),
  SearchTavilyKey: z.string(),
  SearchTavilyAnonymous: z.boolean(),
  SearchFirecrawlKey: z.string(),
  SearchFirecrawlAnonymous: z.boolean(),
  SearchSearxngHost: z.string().url().or(z.literal('')),
})

type SearchFormValues = z.infer<typeof searchSchema>

type SearchSettingsSectionProps = {
  defaultValues: SearchFormValues
}

export function SearchSettingsSection({
  defaultValues,
}: SearchSettingsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const form = useForm({
    resolver: zodResolver(searchSchema),
    defaultValues,
  })

  useResetForm(form, defaultValues)

  const onSubmit = async (data: SearchFormValues) => {
    const updates = Object.entries(data).filter(
      ([key, value]) => value !== defaultValues[key as keyof SearchFormValues]
    )

    for (const [key, value] of updates) {
      await updateOption.mutateAsync({ key, value })
    }
  }

  return (
    <SettingsSection title={t('Playground Search API')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
          />
          <FormField
            control={form.control}
            name='PlaygroundSearchFallbackEnabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable Search Fallback')}</FormLabel>
                  <FormDescription>
                    {t(
                      'If enabled, automatically fallback to other providers if one search provider fails.'
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

          <div className='flex flex-col gap-4 mt-6'>
            <h4 className='text-sm font-semibold'>{t('Tavily Search')}</h4>
            <FormField
              control={form.control}
              name='SearchTavilyKey'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Tavily API Key')}</FormLabel>
                  <FormControl>
                    <Input {...field} autoComplete='off' />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='SearchTavilyAnonymous'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Allow Anonymous Request (Tavily)')}</FormLabel>
                    <FormDescription>
                      {t('Try anonymous request first before using API key.')}
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
          </div>

          <div className='flex flex-col gap-4 mt-6'>
            <h4 className='text-sm font-semibold'>{t('Firecrawl Search')}</h4>
            <FormField
              control={form.control}
              name='SearchFirecrawlKey'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Firecrawl API Key')}</FormLabel>
                  <FormControl>
                    <Input {...field} autoComplete='off' />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='SearchFirecrawlAnonymous'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Allow Anonymous Request (Firecrawl)')}</FormLabel>
                    <FormDescription>
                      {t('Try anonymous request first before using API key.')}
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
          </div>

          <div className='flex flex-col gap-4 mt-6'>
            <h4 className='text-sm font-semibold'>{t('SearXNG Search')}</h4>
            <FormField
              control={form.control}
              name='SearchSearxngHost'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('SearXNG Host URL')}</FormLabel>
                  <FormControl>
                    <Input placeholder='https://searxng.example.com' {...field} autoComplete='off' />
                  </FormControl>
                  <FormDescription>
                    <span className='font-semibold text-orange-500'>
                      {t('You must enable format=json in your SearXNG settings.')}
                    </span>
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
