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
