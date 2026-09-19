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
import { useEffect, useMemo } from 'react'
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

import { SettingsForm } from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

export const SEO_LANGUAGES = [
  { key: 'tr', flag: '🇹🇷', label: 'Türkçe' },
  { key: 'en', flag: '🇬🇧', label: 'English' },
  { key: 'zh_CN', flag: '🇨🇳', label: '简体中文' },
  { key: 'zh_TW', flag: '🇹🇼', label: '繁體中文' },
  { key: 'fr', flag: '🇫🇷', label: 'Français' },
  { key: 'ru', flag: '🇷🇺', label: 'Русский' },
  { key: 'ja', flag: '🇯🇵', label: '日本語' },
  { key: 'vi', flag: '🇻🇳', label: 'Tiếng Việt' },
] as const

export type SeoLanguageKey = (typeof SEO_LANGUAGES)[number]['key']

const robotsPolicies = [
  'allow_all',
  'block_all',
  'block_ai',
  'custom',
] as const

const seoSchema = z.object({
  SEOTitlePrefix: z.string().optional(),
  SEODescription: z.string().optional(),
  SEOKeywords: z.string().optional(),
  SEOSocialImage: z.string().optional(),
  RobotsPolicy: z.enum(robotsPolicies),
  RobotsCustomRules: z.string().optional(),
  SitemapCustomUrls: z.string().optional(),
  LLMSTxt: z.string().optional(),
  LLMSFullTxt: z.string().optional(),
  GoogleAnalyticsId: z.string().optional(),
  UmamiWebsiteId: z.string().optional(),
  UmamiScriptUrl: z.string().optional(),
  ClarityProjectId: z.string().optional(),

  // 8 Languages for Title Prefix
  SEOTitlePrefix_tr: z.string().optional(),
  SEOTitlePrefix_en: z.string().optional(),
  SEOTitlePrefix_zh_CN: z.string().optional(),
  SEOTitlePrefix_zh_TW: z.string().optional(),
  SEOTitlePrefix_fr: z.string().optional(),
  SEOTitlePrefix_ru: z.string().optional(),
  SEOTitlePrefix_ja: z.string().optional(),
  SEOTitlePrefix_vi: z.string().optional(),

  // 8 Languages for Meta Description
  SEODescription_tr: z.string().optional(),
  SEODescription_en: z.string().optional(),
  SEODescription_zh_CN: z.string().optional(),
  SEODescription_zh_TW: z.string().optional(),
  SEODescription_fr: z.string().optional(),
  SEODescription_ru: z.string().optional(),
  SEODescription_ja: z.string().optional(),
  SEODescription_vi: z.string().optional(),

  // 8 Languages for Keywords
  SEOKeywords_tr: z.string().optional(),
  SEOKeywords_en: z.string().optional(),
  SEOKeywords_zh_CN: z.string().optional(),
  SEOKeywords_zh_TW: z.string().optional(),
  SEOKeywords_fr: z.string().optional(),
  SEOKeywords_ru: z.string().optional(),
  SEOKeywords_ja: z.string().optional(),
  SEOKeywords_vi: z.string().optional(),

  // 8 Languages for Privacy Policy
  PrivacyPolicy_tr: z.string().optional(),
  PrivacyPolicy_en: z.string().optional(),
  PrivacyPolicy_zh_CN: z.string().optional(),
  PrivacyPolicy_zh_TW: z.string().optional(),
  PrivacyPolicy_fr: z.string().optional(),
  PrivacyPolicy_ru: z.string().optional(),
  PrivacyPolicy_ja: z.string().optional(),
  PrivacyPolicy_vi: z.string().optional(),

  // 8 Languages for Terms of Service
  TermsOfService_tr: z.string().optional(),
  TermsOfService_en: z.string().optional(),
  TermsOfService_zh_CN: z.string().optional(),
  TermsOfService_zh_TW: z.string().optional(),
  TermsOfService_fr: z.string().optional(),
  TermsOfService_ru: z.string().optional(),
  TermsOfService_ja: z.string().optional(),
  TermsOfService_vi: z.string().optional(),
})

type SEOFormValues = z.infer<typeof seoSchema>

type SEOSectionProps = {
  defaultValues: SEOFormValues
}

const robotsPolicyLabels: Record<(typeof robotsPolicies)[number], string> = {
  allow_all: 'Allow every crawler (fully open)',
  block_all: 'Block every crawler',
  block_ai: 'Allow search engines, block AI training crawlers',
  custom: 'Custom rules (paste robots.txt content below)',
}

export function SEOSection({ defaultValues }: SEOSectionProps) {
  const { t, i18n } = useTranslation()
  const updateOption = useUpdateOption()

  const defaultTab = useMemo(() => {
    const l = (i18n.language || 'tr').toLowerCase()
    if (l.startsWith('tr')) return 'tr'
    if (l.startsWith('zh')) {
      return l.includes('tw') || l.includes('hk') || l.includes('hant') ? 'zh_TW' : 'zh_CN'
    }
    if (l.startsWith('fr')) return 'fr'
    if (l.startsWith('ru')) return 'ru'
    if (l.startsWith('ja')) return 'ja'
    if (l.startsWith('vi')) return 'vi'
    return 'en'
  }, [i18n.language])

  const form = useForm<SEOFormValues>({
    resolver: zodResolver(seoSchema),
    defaultValues,
  })

  useEffect(() => {
    form.reset(defaultValues)
  }, [defaultValues, form])

  const robotsPolicy = form.watch('RobotsPolicy')

  const onSubmit = async (data: SEOFormValues) => {
    // If base fields are empty, sync from active/en/tr tab
    if (!data.SEOTitlePrefix) {
      data.SEOTitlePrefix = data.SEOTitlePrefix_en || data.SEOTitlePrefix_tr || ''
    }
    if (!data.SEODescription) {
      data.SEODescription = data.SEODescription_en || data.SEODescription_tr || ''
    }
    if (!data.SEOKeywords) {
      data.SEOKeywords = data.SEOKeywords_en || data.SEOKeywords_tr || ''
    }

    const updates = Object.entries(data).filter(
      ([key, value]) => value !== defaultValues[key as keyof SEOFormValues]
    )

    for (const [key, value] of updates) {
      await updateOption.mutateAsync({ key, value: value ?? '' })
    }
  }

  return (
    <SettingsSection title={t('SEO')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)} autoComplete='off'>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
          />

          {/* Multilingual SEO & Legal Details Tabs at the TOP */}
          <div className='rounded-lg border p-4 space-y-4 bg-card shadow-xs'>
            <div>
              <h4 className='font-semibold text-sm'>{t('Multilingual SEO & Legal Details')}</h4>
              <p className='text-muted-foreground text-xs mt-1'>
                {t(
                  'Configure custom title prefix, description, keywords, privacy policy and terms per language for search engines.'
                )}
              </p>
            </div>

            <Tabs defaultValue={defaultTab} className='w-full'>
              <TabsList className='grid w-full grid-cols-4 sm:grid-cols-8 gap-1 h-auto p-1'>
                {SEO_LANGUAGES.map((lang) => (
                  <TabsTrigger
                    key={lang.key}
                    value={lang.key}
                    className='text-xs py-1.5 px-2 flex items-center justify-center gap-1.5'
                  >
                    <span>{lang.flag}</span>
                    <span className='truncate'>{lang.label}</span>
                  </TabsTrigger>
                ))}
              </TabsList>

              {SEO_LANGUAGES.map((lang) => (
                <TabsContent key={lang.key} value={lang.key} className='space-y-4 pt-3'>
                  <FormField
                    control={form.control}
                    name={`SEOTitlePrefix_${lang.key}` as any}
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>
                          {t('Title Prefix')} ({lang.label})
                        </FormLabel>
                        <FormControl>
                          <Input
                            {...field}
                            value={field.value ?? ''}
                            placeholder={t('e.g. AI Gateway')}
                            autoComplete='off'
                          />
                        </FormControl>
                        <FormDescription>
                          {t(
                            'Appended to the site name in the browser tab and search results as "Site Name - Prefix".'
                          )}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name={`SEODescription_${lang.key}` as any}
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>
                          {t('SEO Description')} ({lang.label})
                        </FormLabel>
                        <FormControl>
                          <Textarea
                            rows={2}
                            {...field}
                            value={field.value ?? ''}
                            placeholder={t('Shown in search results and link previews')}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name={`SEOKeywords_${lang.key}` as any}
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>
                          {t('SEO Keywords')} ({lang.label})
                        </FormLabel>
                        <FormControl>
                          <Input
                            {...field}
                            value={field.value ?? ''}
                            placeholder='ai, api, openai, claude'
                            autoComplete='off'
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name={`PrivacyPolicy_${lang.key}` as any}
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>
                          {t('Privacy Policy')} ({lang.label})
                        </FormLabel>
                        <FormControl>
                          <Textarea
                            rows={4}
                            {...field}
                            value={field.value ?? ''}
                            placeholder={t('Privacy policy content in this language...')}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name={`TermsOfService_${lang.key}` as any}
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>
                          {t('Terms of Service')} ({lang.label})
                        </FormLabel>
                        <FormControl>
                          <Textarea
                            rows={4}
                            {...field}
                            value={field.value ?? ''}
                            placeholder={t('Terms of service content in this language...')}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </TabsContent>
              ))}
            </Tabs>
          </div>

          {/* Language-Neutral Global SEO & Crawlers */}
          <FormField
            control={form.control}
            name='SEOSocialImage'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Social banner image URL (og:image)')}</FormLabel>
                <FormControl>
                  <Input
                    placeholder='https://example.com/banner.png'
                    autoComplete='off'
                    {...field}
                    value={field.value ?? ''}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Used for social media link previews (Open Graph / Twitter card). The site icon comes from the Logo setting above.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <div className='grid gap-6 md:grid-cols-2'>
            <FormField
              control={form.control}
              name='GoogleAnalyticsId'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Google Analytics ID')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder='G-XXXXXXXXXX'
                      autoComplete='off'
                      {...field}
                      value={field.value ?? ''}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Leave empty to disable. Injects Google Analytics tracking script dynamically into public pages.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='ClarityProjectId'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Microsoft Clarity Project ID')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder='xxxxxxxxxx'
                      autoComplete='off'
                      {...field}
                      value={field.value ?? ''}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Leave empty to disable. Injects Microsoft Clarity analytics script into public pages.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <div className='grid gap-6 md:grid-cols-2'>
            <FormField
              control={form.control}
              name='UmamiWebsiteId'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Umami Website ID')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder='xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx'
                      autoComplete='off'
                      {...field}
                      value={field.value ?? ''}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Leave empty to disable. Privacy-friendly self-hosted or cloud Umami website ID.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='UmamiScriptUrl'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Umami Script URL')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder='https://analytics.umami.is/script.js'
                      autoComplete='off'
                      {...field}
                      value={field.value ?? ''}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Defaults to https://analytics.umami.is/script.js if empty.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <FormField
            control={form.control}
            name='RobotsPolicy'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Crawler policy (robots.txt)')}</FormLabel>
                <Select value={field.value} onValueChange={field.onChange}>
                  <FormControl>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent alignItemWithTrigger={false}>
                    {robotsPolicies.map((policy) => (
                      <SelectItem key={policy} value={policy}>
                        {t(robotsPolicyLabels[policy])}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <FormDescription>
                  {t('Preview:')}{' '}
                  <a
                    href='/robots.txt'
                    target='_blank'
                    rel='noreferrer'
                    className='hover:text-primary underline underline-offset-4'
                  >
                    /robots.txt
                  </a>
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          {robotsPolicy === 'custom' && (
            <FormField
              control={form.control}
              name='RobotsCustomRules'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Custom robots.txt rules')}</FormLabel>
                  <FormControl>
                    <Textarea
                      rows={6}
                      placeholder={'User-agent: *\nDisallow: /console'}
                      className='font-mono text-xs'
                      {...field}
                      value={field.value ?? ''}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          )}

          <FormField
            control={form.control}
            name='SitemapCustomUrls'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Extra sitemap URLs')}</FormLabel>
                <FormControl>
                  <Textarea
                    rows={3}
                    placeholder={'/custom/abc\nhttps://example.com/page'}
                    className='font-mono text-xs'
                    {...field}
                    value={field.value ?? ''}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'One URL per line. Relative paths (starting with /) are resolved against the server address. Published blog posts are included automatically.'
                  )}{' '}
                  {t('Preview:')}{' '}
                  <a
                    href='/sitemap.xml'
                    target='_blank'
                    rel='noreferrer'
                    className='hover:text-primary underline underline-offset-4'
                  >
                    /sitemap.xml
                  </a>
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <div className='grid gap-6 md:grid-cols-2'>
            <FormField
              control={form.control}
              name='LLMSTxt'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('llms.txt content')}</FormLabel>
                  <FormControl>
                    <Textarea
                      rows={6}
                      placeholder={t('Leave empty to serve 404 for /llms.txt')}
                      className='font-mono text-xs'
                      {...field}
                      value={field.value ?? ''}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Served at /llms.txt. Leave empty to disable it.')}{' '}
                    <a
                      href='/llms.txt'
                      target='_blank'
                      rel='noreferrer'
                      className='hover:text-primary underline underline-offset-4'
                    >
                      /llms.txt
                    </a>
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='LLMSFullTxt'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('llms-full.txt content')}</FormLabel>
                  <FormControl>
                    <Textarea
                      rows={6}
                      placeholder={t('Leave empty to serve 404 for /llms-full.txt')}
                      className='font-mono text-xs'
                      {...field}
                      value={field.value ?? ''}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Served at /llms-full.txt (also /full-llms.txt). Leave empty to disable it.')}{' '}
                    <a
                      href='/llms-full.txt'
                      target='_blank'
                      rel='noreferrer'
                      className='hover:text-primary underline underline-offset-4'
                    >
                      /llms-full.txt
                    </a>
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
