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

const robotsPolicies = [
  'allow_all',
  'block_all',
  'block_ai',
  'custom',
] as const

const seoSchema = z.object({
  SEOTitlePrefix: z.string(),
  SEODescription: z.string(),
  SEOKeywords: z.string(),
  SEOSocialImage: z.string(),
  RobotsPolicy: z.enum(robotsPolicies),
  RobotsCustomRules: z.string(),
  SitemapCustomUrls: z.string(),
  LLMSTxt: z.string(),
  LLMSFullTxt: z.string(),
  GoogleAnalyticsId: z.string().optional(),
  UmamiWebsiteId: z.string().optional(),
  UmamiScriptUrl: z.string().optional(),
  ClarityProjectId: z.string().optional(),
  SEOTitlePrefix_tr: z.string().optional(),
  SEOTitlePrefix_en: z.string().optional(),
  SEOTitlePrefix_zh_CN: z.string().optional(),
  SEODescription_tr: z.string().optional(),
  SEODescription_en: z.string().optional(),
  SEODescription_zh_CN: z.string().optional(),
  SEOKeywords_tr: z.string().optional(),
  SEOKeywords_en: z.string().optional(),
  SEOKeywords_zh_CN: z.string().optional(),
  PrivacyPolicy_tr: z.string().optional(),
  PrivacyPolicy_en: z.string().optional(),
  PrivacyPolicy_zh_CN: z.string().optional(),
  TermsOfService_tr: z.string().optional(),
  TermsOfService_en: z.string().optional(),
  TermsOfService_zh_CN: z.string().optional(),
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
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const form = useForm<SEOFormValues>({
    resolver: zodResolver(seoSchema),
    defaultValues,
  })

  useEffect(() => {
    form.reset(defaultValues)
  }, [defaultValues, form])

  const robotsPolicy = form.watch('RobotsPolicy')

  const onSubmit = async (data: SEOFormValues) => {
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

          <FormField
            control={form.control}
            name='SEOTitlePrefix'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Title prefix (optional)')}</FormLabel>
                <FormControl>
                  <Input
                    placeholder={t('e.g. AI Gateway')}
                    autoComplete='off'
                    {...field}
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
            name='SEODescription'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Meta description')}</FormLabel>
                <FormControl>
                  <Textarea
                    rows={2}
                    placeholder={t('Shown in search results and link previews')}
                    {...field}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='SEOKeywords'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Meta keywords')}</FormLabel>
                <FormControl>
                  <Input
                    placeholder={t('comma, separated, keywords')}
                    autoComplete='off'
                    {...field}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

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
                    'Leave empty to disable. Injects Google Analytics tracking script dynamically into the public pages.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

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
                  {t(
                    'Leave empty to disable. Privacy-friendly self-hosted or cloud Umami website ID.'
                  )}
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
                  {t(
                    'Defaults to https://analytics.umami.is/script.js if empty.'
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

          <div className='rounded-lg border p-4 space-y-4'>
            <div>
              <h4 className='font-medium text-sm'>{t('Multilingual SEO & Legal Details')}</h4>
              <p className='text-muted-foreground text-xs'>
                {t('Configure custom title prefix, description, keywords, privacy policy and terms per language for search engines.')}
              </p>
            </div>

            <Tabs defaultValue='tr' className='w-full'>
              <TabsList className='grid w-full grid-cols-3'>
                <TabsTrigger value='tr'>🇹🇷 Türkçe</TabsTrigger>
                <TabsTrigger value='en'>🇬🇧 English</TabsTrigger>
                <TabsTrigger value='zh_CN'>🇨🇳 中文</TabsTrigger>
              </TabsList>

              {(['tr', 'en', 'zh_CN'] as const).map((lang) => (
                <TabsContent key={lang} value={lang} className='space-y-3 pt-2'>
                  <FormField
                    control={form.control}
                    name={`SEOTitlePrefix_${lang}` as any}
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Title Prefix')} ({lang})</FormLabel>
                        <FormControl>
                          <Input {...field} value={field.value ?? ''} placeholder={t('AI Model Hub')} />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name={`SEODescription_${lang}` as any}
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('SEO Description')} ({lang})</FormLabel>
                        <FormControl>
                          <Textarea rows={2} {...field} value={field.value ?? ''} placeholder={t('Search description')} />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name={`SEOKeywords_${lang}` as any}
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('SEO Keywords')} ({lang})</FormLabel>
                        <FormControl>
                          <Input {...field} value={field.value ?? ''} placeholder='ai, api, openai, claude' />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name={`PrivacyPolicy_${lang}` as any}
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Privacy Policy')} ({lang})</FormLabel>
                        <FormControl>
                          <Textarea rows={3} {...field} value={field.value ?? ''} placeholder={t('Privacy policy content in this language...')} />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name={`TermsOfService_${lang}` as any}
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Terms of Service')} ({lang})</FormLabel>
                        <FormControl>
                          <Textarea rows={3} {...field} value={field.value ?? ''} placeholder={t('Terms of service content in this language...')} />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </TabsContent>
              ))}
            </Tabs>
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

          <FormField
            control={form.control}
            name='LLMSTxt'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('llms.txt content')}</FormLabel>
                <FormControl>
                  <Textarea
                    rows={6}
                    placeholder={t(
                      'Leave empty to serve 404 for /llms.txt'
                    )}
                    className='font-mono text-xs'
                    {...field}
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
                    placeholder={t(
                      'Leave empty to serve 404 for /llms-full.txt'
                    )}
                    className='font-mono text-xs'
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Served at /llms-full.txt (also /full-llms.txt). Leave empty to disable it.'
                  )}{' '}
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
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
