import { zodResolver } from '@hookform/resolvers/zod'
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
import { Textarea } from '@/components/ui/textarea'

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
  botToken: z.string().optional(),
  botName: z.string().optional(),
  prefix: z.string().optional(),
  status: z.string().optional(),
  language: z.string().optional(),
  embedColor: z.string().optional(),
  aiSystemPrompt: z.string().optional(),
  rpgEnabled: z.boolean(),
  rpcAppId: z.string().optional(),
  rpcDetails: z.string().optional(),
  rpcState: z.string().optional(),
  rpcLargeImage: z.string().optional(),
  rpcLargeText: z.string().optional(),
  rpcSmallImage: z.string().optional(),
  rpcSmallText: z.string().optional(),
  autoReplyChannelId: z.string().optional(),
  autoReplyModel: z.string().optional(),
  onlyLinkedAccounts: z.boolean(),
})

type Values = z.infer<typeof schema>

export function DiscordBotSettingsSection({
  defaultValues,
}: {
  defaultValues: {
    enabled?: boolean
    botToken?: string
    botName?: string
    prefix?: string
    status?: string
    language?: string
    embedColor?: string
    aiSystemPrompt?: string
    rpgEnabled?: boolean
    rpcAppId?: string
    rpcDetails?: string
    rpcState?: string
    rpcLargeImage?: string
    rpcLargeText?: string
    rpcSmallImage?: string
    rpcSmallText?: string
    autoReplyChannelId?: string
    autoReplyModel?: string
    onlyLinkedAccounts?: boolean
  }
}) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: {
      enabled: defaultValues.enabled ?? false,
      botToken: defaultValues.botToken ?? '',
      botName: defaultValues.botName ?? 'MyAIBot',
      prefix: defaultValues.prefix ?? '!',
      status: defaultValues.status ?? 'Ready to help',
      language: defaultValues.language ?? 'en',
      embedColor: defaultValues.embedColor ?? '#00ff00',
      aiSystemPrompt:
        defaultValues.aiSystemPrompt ?? 'You are a helpful AI assistant.',
      rpgEnabled: defaultValues.rpgEnabled ?? true,
      rpcAppId: defaultValues.rpcAppId ?? '',
      rpcDetails: defaultValues.rpcDetails ?? '',
      rpcState: defaultValues.rpcState ?? '',
      rpcLargeImage: defaultValues.rpcLargeImage ?? '',
      rpcLargeText: defaultValues.rpcLargeText ?? '',
      rpcSmallImage: defaultValues.rpcSmallImage ?? '',
      rpcSmallText: defaultValues.rpcSmallText ?? '',
      autoReplyChannelId: defaultValues.autoReplyChannelId ?? '',
      autoReplyModel: defaultValues.autoReplyModel ?? '',
      onlyLinkedAccounts: defaultValues.onlyLinkedAccounts ?? false,
    },
  })

  const { isDirty, isSubmitting } = form.formState

  async function onSubmit(values: Values) {
    try {
      await updateOption.mutateAsync({
        key: 'discord_bot.enabled',
        value: String(values.enabled),
      })
      await updateOption.mutateAsync({
        key: 'discord_bot.token',
        value: String(values.botToken ?? ''),
      })
      await updateOption.mutateAsync({
        key: 'discord_bot.bot_token',
        value: String(values.botToken ?? ''),
      })
      await updateOption.mutateAsync({
        key: 'discord_bot.bot_name',
        value: String(values.botName ?? 'MyAIBot'),
      })
      await updateOption.mutateAsync({
        key: 'discord_bot.prefix',
        value: String(values.prefix ?? '!'),
      })
      await updateOption.mutateAsync({
        key: 'discord_bot.status',
        value: String(values.status ?? 'Ready to help'),
      })
      await updateOption.mutateAsync({
        key: 'discord_bot.language',
        value: String(values.language ?? 'en'),
      })
      await updateOption.mutateAsync({
        key: 'discord_bot.embed_color',
        value: String(values.embedColor ?? '#00ff00'),
      })
      await updateOption.mutateAsync({
        key: 'discord_bot.ai_system_prompt',
        value: String(values.aiSystemPrompt ?? ''),
      })
      await updateOption.mutateAsync({
        key: 'discord_bot.rpc_enabled',
        value: String(values.rpgEnabled),
      })
      await updateOption.mutateAsync({
        key: 'discord_bot.rpc_app_id',
        value: String(values.rpcAppId ?? ''),
      })
      await updateOption.mutateAsync({
        key: 'discord_bot.rpc_details',
        value: String(values.rpcDetails ?? ''),
      })
      await updateOption.mutateAsync({
        key: 'discord_bot.rpc_state',
        value: String(values.rpcState ?? ''),
      })
      await updateOption.mutateAsync({
        key: 'discord_bot.rpc_large_image',
        value: String(values.rpcLargeImage ?? ''),
      })
      await updateOption.mutateAsync({
        key: 'discord_bot.rpc_large_text',
        value: String(values.rpcLargeText ?? ''),
      })
      await updateOption.mutateAsync({
        key: 'discord_bot.rpc_small_image',
        value: String(values.rpcSmallImage ?? ''),
      })
      await updateOption.mutateAsync({
        key: 'discord_bot.rpc_small_text',
        value: String(values.rpcSmallText ?? ''),
      })
      await updateOption.mutateAsync({
        key: 'discord_bot.auto_reply_channel_id',
        value: String(values.autoReplyChannelId ?? ''),
      })
      await updateOption.mutateAsync({
        key: 'discord_bot.auto_reply_model',
        value: String(values.autoReplyModel ?? ''),
      })
      await updateOption.mutateAsync({
        key: 'discord_bot.only_linked_accounts',
        value: String(values.onlyLinkedAccounts),
      })

      form.reset(values)
      toast.success(t('Discord Bot settings saved successfully'))
    } catch {
      toast.error(t('Failed to save Discord Bot settings'))
    }
  }

  return (
    <SettingsSection title={t('Discord Bot Integration')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <FormField
            control={form.control}
            name='enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable Built-in Discord Bot')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Connect a Discord bot to interact with users, display models menu, generate images, and auto-reply in designated channels.'
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
              name='botToken'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Discord Bot Token')}</FormLabel>
                  <FormControl>
                    <Input
                      type='password'
                      placeholder='MTAy...'
                      {...field}
                      value={field.value ?? ''}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Bot token from Discord Developer Portal')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='botName'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Bot Name')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder='MyAIBot'
                      {...field}
                      value={field.value ?? ''}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Display name of the bot')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='prefix'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Command Prefix')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder='!'
                      {...field}
                      value={field.value ?? ''}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Prefix for commands (e.g. !, /, ?)')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='status'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Bot Activity / Status')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder='Ready to help'
                      {...field}
                      value={field.value ?? ''}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Activity status text displayed on Discord')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='language'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Default Language')}</FormLabel>
                  <Select
                    value={field.value ?? 'en'}
                    onValueChange={field.onChange}
                  >
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      <SelectItem value='tr'>Türkçe (tr)</SelectItem>
                      <SelectItem value='en'>English (en)</SelectItem>
                      <SelectItem value='zh_CN'>简体中文 (zh-CN)</SelectItem>
                      <SelectItem value='zh_TW'>繁體中文 (zh-TW)</SelectItem>
                      <SelectItem value='ja'>日本語 (ja)</SelectItem>
                      <SelectItem value='ko'>한국어 (ko)</SelectItem>
                      <SelectItem value='de'>Deutsch (de)</SelectItem>
                      <SelectItem value='fr'>Français (fr)</SelectItem>
                      <SelectItem value='es'>Español (es)</SelectItem>
                      <SelectItem value='ru'>Русский (ru)</SelectItem>
                      <SelectItem value='vi'>Tiếng Việt (vi)</SelectItem>
                      <SelectItem value='ar'>العربية (ar)</SelectItem>
                      <SelectItem value='pt'>Português (pt)</SelectItem>
                    </SelectContent>
                  </Select>
                  <FormDescription>
                    {t('Language for bot embed messages and responses')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='embedColor'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Embed Color')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder='#00ff00'
                      {...field}
                      value={field.value ?? ''}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Hex color code for Discord rich embeds')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='autoReplyChannelId'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Auto-Reply Channel ID')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder='123456789012345678'
                      {...field}
                      value={field.value ?? ''}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Specific Discord Channel ID where the bot automatically replies to all incoming messages'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='autoReplyModel'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Auto-Reply Model')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder='gpt-4o-mini'
                      {...field}
                      value={field.value ?? ''}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('AI Model to use for channel auto-replies')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </SettingsFormGrid>

          <FormField
            control={form.control}
            name='aiSystemPrompt'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('AI System Prompt')}</FormLabel>
                <FormControl>
                  <Textarea
                    placeholder='You are a helpful AI assistant.'
                    rows={4}
                    {...field}
                    value={field.value ?? ''}
                  />
                </FormControl>
                <FormDescription>
                  {t('System instructions for the AI bot when generating responses')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='rpgEnabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Rich Presence (RPC) & Status Activity')}</FormLabel>
                  <FormDescription>
                    {t('Show dynamic Rich Presence (RPC) and status activity on Discord.')}
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

          {form.watch('rpgEnabled') && (
            <div className='border-border/60 rounded-lg border bg-muted/20 p-4 space-y-4'>
              <div className='font-semibold text-sm'>{t('Discord RPC (Rich Presence) Detayları')}</div>
              <div className='grid gap-4 sm:grid-cols-2'>
                <FormField
                  control={form.control}
                  name='rpcAppId'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Application ID (Client ID)')}</FormLabel>
                      <FormControl>
                        <Input placeholder='123456789012345678' {...field} value={field.value ?? ''} />
                      </FormControl>
                      <FormDescription>{t('Discord Developer Portal Application ID')}</FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='rpcDetails'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Details (Üst Başlık)')}</FormLabel>
                      <FormControl>
                        <Input placeholder='AI Router & Hub' {...field} value={field.value ?? ''} />
                      </FormControl>
                      <FormDescription>{t('RPC üst metni (örn. AI Servisi)')}</FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='rpcState'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('State (Alt Başlık)')}</FormLabel>
                      <FormControl>
                        <Input placeholder='Online & Hazır' {...field} value={field.value ?? ''} />
                      </FormControl>
                      <FormDescription>{t('RPC alt metni (örn. Yanıt Veriyor)')}</FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='rpcLargeImage'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Large Image Key')}</FormLabel>
                      <FormControl>
                        <Input placeholder='logo_large' {...field} value={field.value ?? ''} />
                      </FormControl>
                      <FormDescription>{t('Developer Portal Rich Presence Asset Adı')}</FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='rpcLargeText'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Large Image Text')}</FormLabel>
                      <FormControl>
                        <Input placeholder='PixRouter AI' {...field} value={field.value ?? ''} />
                      </FormControl>
                      <FormDescription>{t('Büyük görselin üzerine gelince görünen metin')}</FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='rpcSmallImage'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Small Image Key')}</FormLabel>
                      <FormControl>
                        <Input placeholder='status_online' {...field} value={field.value ?? ''} />
                      </FormControl>
                      <FormDescription>{t('Küçük rozet görseli asset anahtarı')}</FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
            </div>
          )}

          <FormField
            control={form.control}
            name='onlyLinkedAccounts'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Charge Only Linked User Accounts')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Deduct quota from linked user accounts. If enabled, commands from unlinked Discord users will not be charged balance.'
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
