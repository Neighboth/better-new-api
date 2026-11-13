import { zodResolver } from '@hookform/resolvers/zod'
import { useForm, type Resolver } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
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
  SettingsDescription,
  SettingsForm,
  SettingsFormGrid,
  SettingsSwitchContent,
  SettingsSwitchRow,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOptionsBulk } from '../hooks/use-update-option'

const schema = z.object({
  searxngUrl: z.string().optional(),
  onlineSearchPrice: z.coerce.number().min(0).max(1000).optional(),
  onlineSearchMax: z.coerce.number().int().min(1).max(10).optional(),
  playgroundEnabled: z.boolean().optional(),
  discordEnabled: z.boolean().optional(),
  liveEnabled: z.boolean().optional(),
  blogEnabled: z.boolean().optional(),
})

type Values = z.infer<typeof schema>

export function OnlineSearchSettingsSection({
  defaultValues,
}: {
  defaultValues: {
    searxngUrl?: string
    onlineSearchPrice?: number
    onlineSearchMax?: number
    playgroundEnabled?: boolean
    discordEnabled?: boolean
    liveEnabled?: boolean
    blogEnabled?: boolean
  }
}) {
  const { t } = useTranslation()
  const updateOptions = useUpdateOptionsBulk()

  const form = useForm<Values>({
    resolver: zodResolver(schema) as unknown as Resolver<Values>,
    defaultValues: {
      searxngUrl: defaultValues.searxngUrl ?? '',
      onlineSearchPrice: defaultValues.onlineSearchPrice ?? 0,
      onlineSearchMax: defaultValues.onlineSearchMax ?? 5,
      playgroundEnabled: defaultValues.playgroundEnabled ?? false,
      discordEnabled: defaultValues.discordEnabled ?? false,
      liveEnabled: defaultValues.liveEnabled ?? false,
      blogEnabled: defaultValues.blogEnabled ?? false,
    },
  })

  const onSubmit = async (values: Values) => {
    try {
      // One request for the whole card: the toggles and the search engine
      // must be stored together, a partial save made the page look like the
      // save was ignored (and the old values came back on the next refetch).
      await updateOptions.mutateAsync({
        'online_search_setting.searxng_url': String(values.searxngUrl ?? ''),
        'online_search_setting.price_per_search': String(
          values.onlineSearchPrice ?? 0
        ),
        'online_search_setting.max_searches': String(values.onlineSearchMax ?? 5),
        'playground_setting.web_search_playground_enabled': String(
          Boolean(values.playgroundEnabled)
        ),
        'playground_setting.web_search_discord_enabled': String(
          Boolean(values.discordEnabled)
        ),
        'playground_setting.web_search_live_enabled': String(
          Boolean(values.liveEnabled)
        ),
        'playground_setting.web_search_blog_enabled': String(
          Boolean(values.blogEnabled)
        ),
      })

      form.reset(values)
      toast.success(t('Web search settings saved successfully'))
    } catch {
      toast.error(t('Failed to save settings'))
    }
  }

  const playgroundEnabled = form.watch('playgroundEnabled')
  const discordEnabled = form.watch('discordEnabled')
  const liveEnabled = form.watch('liveEnabled')
  const blogEnabled = form.watch('blogEnabled')

  return (
    <SettingsSection title={t('Web Search')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsFormGrid>
            <FormField
              control={form.control}
              name='searxngUrl'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('SearXNG URL')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder='https://searxng.example.com'
                      {...field}
                      value={field.value ?? ''}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Self-hosted SearXNG instance used by the search tools. Leave empty to disable web search everywhere.'
                    )}
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
                  <FormLabel>{t('Price per search (USD)')}</FormLabel>
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
                    {t(
                      'Charged to the user balance for every search call made by a tool. Set 0 to make the search free.'
                    )}
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
                  <FormLabel>{t('Max search results')}</FormLabel>
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
                    {t('Maximum SearXNG results returned per search (1-10).')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </SettingsFormGrid>

          <div className='space-y-1'>
            <Label>{t('Where the search tool is available')}</Label>
            <SettingsDescription>
              {t(
                'Tick a surface to give its assistant the web search tool. Unticked surfaces never search the web.'
              )}
            </SettingsDescription>
          </div>

          <SettingsSwitchRow>
            <SettingsSwitchContent>
              <Label>{t('Playground')}</Label>
              <SettingsDescription>
                {t(
                  'Adds the web search tool to the playground chat. Every search is billed at the price above.'
                )}
              </SettingsDescription>
            </SettingsSwitchContent>
            <Switch
              checked={Boolean(playgroundEnabled)}
              onCheckedChange={(checked) =>
                form.setValue('playgroundEnabled', checked, {
                  shouldDirty: true,
                })
              }
            />
          </SettingsSwitchRow>

          <SettingsSwitchRow>
            <SettingsSwitchContent>
              <Label>{t('Discord bot')}</Label>
              <SettingsDescription>
                {t('Gives the Discord bot the same web search tool.')}
              </SettingsDescription>
            </SettingsSwitchContent>
            <Switch
              checked={Boolean(discordEnabled)}
              onCheckedChange={(checked) =>
                form.setValue('discordEnabled', checked, {
                  shouldDirty: true,
                })
              }
            />
          </SettingsSwitchRow>
          {discordEnabled && (
            <p className='text-muted-foreground border-amber-500/40 bg-amber-500/5 rounded-md border px-3 py-2 text-xs'>
              {t(
                'Discord users are not signed in to this site, so the search price cannot always be charged: searches made by unlinked accounts may be free.'
              )}
            </p>
          )}

          <SettingsSwitchRow>
            <SettingsSwitchContent>
              <Label>{t('Live support')}</Label>
              <SettingsDescription>
                {t(
                  'Lets the AI live support model search the web when a visitor asks about current information.'
                )}
              </SettingsDescription>
            </SettingsSwitchContent>
            <Switch
              checked={Boolean(liveEnabled)}
              onCheckedChange={(checked) =>
                form.setValue('liveEnabled', checked, { shouldDirty: true })
              }
            />
          </SettingsSwitchRow>
          {liveEnabled && (
            <p className='text-muted-foreground border-amber-500/40 bg-amber-500/5 rounded-md border px-3 py-2 text-xs'>
              {t(
                'Visitors of the live support are often not logged in, so the search price may not be charged for their searches.'
              )}
            </p>
          )}

          <SettingsSwitchRow>
            <SettingsSwitchContent>
              <Label>{t('Blog generation')}</Label>
              <SettingsDescription>
                {t(
                  'Lets the AI blog writer search the web and read pages while it drafts an article, so posts carry up-to-date sources.'
                )}
              </SettingsDescription>
            </SettingsSwitchContent>
            <Switch
              checked={Boolean(blogEnabled)}
              onCheckedChange={(checked) =>
                form.setValue('blogEnabled', checked, { shouldDirty: true })
              }
            />
          </SettingsSwitchRow>
          {blogEnabled && (
            <p className='text-muted-foreground border-amber-500/40 bg-amber-500/5 rounded-md border px-3 py-2 text-xs'>
              {t(
                'Blog posts may take much longer to generate because the model searches and reads pages. Every search and every page is billed at the prices above.'
              )}
            </p>
          )}

          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOptions.isPending}
            isSaveDisabled={!form.formState.isDirty}
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
