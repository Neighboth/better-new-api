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
import { useMemo } from 'react'
import type { Resolver } from 'react-hook-form'
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'

import { FormDirtyIndicator } from '../components/form-dirty-indicator'
import { FormNavigationGuard } from '../components/form-navigation-guard'
import {
  SettingsForm,
  SettingsFormGrid,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useSettingsForm } from '../hooks/use-settings-form'
import { useUpdateOption } from '../hooks/use-update-option'

export const SYSTEM_INFO_LANGUAGES = [
  { key: 'tr', flag: '🇹🇷', label: 'Türkçe' },
  { key: 'en', flag: '🇬🇧', label: 'English' },
  { key: 'zh_CN', flag: '🇨🇳', label: '简体中文' },
  { key: 'zh_TW', flag: '🇹🇼', label: '繁體中文' },
  { key: 'fr', flag: '🇫🇷', label: 'Français' },
  { key: 'ru', flag: '🇷🇺', label: 'Русский' },
  { key: 'ja', flag: '🇯🇵', label: '日本語' },
  { key: 'vi', flag: '🇻🇳', label: 'Tiếng Việt' },
] as const

const systemInfoSchema = z.object({
  SystemName: z.string().optional(),
  ServerAddress: z.string().optional(),
  Logo: z.string().url().optional().or(z.literal('')),
  Footer: z.string().optional(),
  About: z.string().optional(),
  HomePageContent: z.string().optional(),
  'legal.user_agreement': z.string().optional(),
  'legal.privacy_policy': z.string().optional(),

  // Multilingual System Info (8 Languages)
  SystemName_tr: z.string().optional(),
  SystemName_en: z.string().optional(),
  SystemName_zh_CN: z.string().optional(),
  SystemName_zh_TW: z.string().optional(),
  SystemName_fr: z.string().optional(),
  SystemName_ru: z.string().optional(),
  SystemName_ja: z.string().optional(),
  SystemName_vi: z.string().optional(),

  Footer_tr: z.string().optional(),
  Footer_en: z.string().optional(),
  Footer_zh_CN: z.string().optional(),
  Footer_zh_TW: z.string().optional(),
  Footer_fr: z.string().optional(),
  Footer_ru: z.string().optional(),
  Footer_ja: z.string().optional(),
  Footer_vi: z.string().optional(),

  About_tr: z.string().optional(),
  About_en: z.string().optional(),
  About_zh_CN: z.string().optional(),
  About_zh_TW: z.string().optional(),
  About_fr: z.string().optional(),
  About_ru: z.string().optional(),
  About_ja: z.string().optional(),
  About_vi: z.string().optional(),

  HomePageContent_tr: z.string().optional(),
  HomePageContent_en: z.string().optional(),
  HomePageContent_zh_CN: z.string().optional(),
  HomePageContent_zh_TW: z.string().optional(),
  HomePageContent_fr: z.string().optional(),
  HomePageContent_ru: z.string().optional(),
  HomePageContent_ja: z.string().optional(),
  HomePageContent_vi: z.string().optional(),

  'legal.user_agreement_tr': z.string().optional(),
  'legal.user_agreement_en': z.string().optional(),
  'legal.user_agreement_zh_CN': z.string().optional(),
  'legal.user_agreement_zh_TW': z.string().optional(),
  'legal.user_agreement_fr': z.string().optional(),
  'legal.user_agreement_ru': z.string().optional(),
  'legal.user_agreement_ja': z.string().optional(),
  'legal.user_agreement_vi': z.string().optional(),

  'legal.privacy_policy_tr': z.string().optional(),
  'legal.privacy_policy_en': z.string().optional(),
  'legal.privacy_policy_zh_CN': z.string().optional(),
  'legal.privacy_policy_zh_TW': z.string().optional(),
  'legal.privacy_policy_fr': z.string().optional(),
  'legal.privacy_policy_ru': z.string().optional(),
  'legal.privacy_policy_ja': z.string().optional(),
  'legal.privacy_policy_vi': z.string().optional(),
})

export type SystemInfoFormValues = z.infer<typeof systemInfoSchema>

type SystemInfoSectionProps = {
  defaultValues: Record<string, any>
}

function normalizeValue(value: unknown): string {
  if (value === undefined || value === null) return ''
  return typeof value === 'string' ? value : String(value)
}

export function SystemInfoSection({ defaultValues }: SystemInfoSectionProps) {
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

  const normalizedDefaults: Record<string, string> = {
    SystemName: normalizeValue(defaultValues.SystemName),
    ServerAddress: normalizeValue(defaultValues.ServerAddress),
    Logo: normalizeValue(defaultValues.Logo),
    Footer: normalizeValue(defaultValues.Footer),
    About: normalizeValue(defaultValues.About),
    HomePageContent: normalizeValue(defaultValues.HomePageContent),
    'legal.user_agreement': normalizeValue(
      defaultValues['legal.user_agreement'] ?? defaultValues.legal?.user_agreement
    ),
    'legal.privacy_policy': normalizeValue(
      defaultValues['legal.privacy_policy'] ?? defaultValues.legal?.privacy_policy
    ),
  }

  // Populate localized defaults
  SYSTEM_INFO_LANGUAGES.forEach((lang) => {
    normalizedDefaults[`SystemName_${lang.key}`] = normalizeValue(defaultValues[`SystemName_${lang.key}`])
    normalizedDefaults[`Footer_${lang.key}`] = normalizeValue(defaultValues[`Footer_${lang.key}`])
    normalizedDefaults[`About_${lang.key}`] = normalizeValue(defaultValues[`About_${lang.key}`])
    normalizedDefaults[`HomePageContent_${lang.key}`] = normalizeValue(defaultValues[`HomePageContent_${lang.key}`])
    normalizedDefaults[`legal.user_agreement_${lang.key}`] = normalizeValue(defaultValues[`legal.user_agreement_${lang.key}`])
    normalizedDefaults[`legal.privacy_policy_${lang.key}`] = normalizeValue(defaultValues[`legal.privacy_policy_${lang.key}`])
  })

  const { form, handleSubmit, handleReset, isDirty, isSubmitting } =
    useSettingsForm<SystemInfoFormValues>({
      resolver: zodResolver(systemInfoSchema) as Resolver<
        SystemInfoFormValues,
        unknown,
        SystemInfoFormValues
      >,
      defaultValues: normalizedDefaults as any,
      onSubmit: async (data, changedFields) => {
        // Fallback root fields if empty
        if (!data.SystemName) {
          data.SystemName = data.SystemName_en || data.SystemName_tr || 'New API'
          changedFields['SystemName'] = data.SystemName
        }

        for (const [key, value] of Object.entries(changedFields)) {
          let v = normalizeValue(value)
          if (key === 'ServerAddress') {
            v = v.replace(/\/+$/, '')
          }
          await updateOption.mutateAsync({
            key,
            value: v,
          })
        }
      },
    })

  return (
    <>
      <FormNavigationGuard when={isDirty} />

      <SettingsSection title={t('System Information')}>
        <Form {...form}>
          <SettingsForm onSubmit={handleSubmit}>
            <SettingsPageFormActions
              onSave={handleSubmit}
              onReset={handleReset}
              isSaving={isSubmitting || updateOption.isPending}
              isResetDisabled={!isDirty}
            />
            <FormDirtyIndicator isDirty={isDirty} />

            {/* Language-Neutral Global Settings */}
            <SettingsFormGrid>
              <FormField
                control={form.control}
                name='ServerAddress'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Server Address')}</FormLabel>
                    <FormControl>
                      <Input placeholder='https://yourdomain.com' {...field} />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'The public URL of your server, used for OAuth callbacks, webhooks, and other external integrations'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='Logo'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Logo URL')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t('https://example.com/logo.png')}
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('URL to your logo image (optional)')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SettingsFormGrid>

            {/* Multilingual System Content Tabs */}
            <div className='rounded-lg border p-4 space-y-4 bg-card shadow-xs mt-6'>
              <div>
                <h4 className='font-semibold text-sm'>{t('Multilingual System Content')}</h4>
                <p className='text-muted-foreground text-xs mt-1'>
                  {t(
                    'Configure system name, footer, about, homepage content and legal policies for each language supported by the system.'
                  )}
                </p>
              </div>

              <Tabs defaultValue={defaultTab} className='w-full'>
                <TabsList className='grid w-full grid-cols-4 sm:grid-cols-8 gap-1 h-auto p-1'>
                  {SYSTEM_INFO_LANGUAGES.map((lang) => (
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

                {SYSTEM_INFO_LANGUAGES.map((lang) => (
                  <TabsContent key={lang.key} value={lang.key} className='space-y-4 pt-3'>
                    <SettingsFormGrid>
                      <FormField
                        control={form.control}
                        name={`SystemName_${lang.key}` as any}
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>
                              {t('System Name')} ({lang.label})
                            </FormLabel>
                            <FormControl>
                              <Input
                                placeholder={t('New API')}
                                {...field}
                                value={field.value ?? ''}
                              />
                            </FormControl>
                            <FormDescription>
                              {t('The name displayed across the application')}
                            </FormDescription>
                            <FormMessage />
                          </FormItem>
                        )}
                      />

                      <FormField
                        control={form.control}
                        name={`Footer_${lang.key}` as any}
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>
                              {t('Footer')} ({lang.label})
                            </FormLabel>
                            <FormControl>
                              <Textarea
                                placeholder={t(
                                  '© 2025 Your Company. All rights reserved.'
                                )}
                                rows={3}
                                {...field}
                                value={field.value ?? ''}
                              />
                            </FormControl>
                            <FormDescription>
                              {t('Footer text displayed at the bottom of pages')}
                            </FormDescription>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                    </SettingsFormGrid>

                    <FormField
                      control={form.control}
                      name={`About_${lang.key}` as any}
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>
                            {t('About')} ({lang.label})
                          </FormLabel>
                          <FormControl>
                            <Textarea
                              placeholder={t(
                                'Enter HTML code (e.g., <p>About us...</p>) or a URL (e.g., https://example.com) to embed as iframe'
                              )}
                              rows={4}
                              {...field}
                              value={field.value ?? ''}
                            />
                          </FormControl>
                          <FormDescription>
                            {t(
                              'Supports HTML markup or iframe embedding. Enter HTML code directly, or provide a complete URL to automatically embed it as an iframe.'
                            )}
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    <FormField
                      control={form.control}
                      name={`HomePageContent_${lang.key}` as any}
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>
                            {t('Home Page Content')} ({lang.label})
                          </FormLabel>
                          <FormControl>
                            <Textarea
                              placeholder={t('Welcome to our New API...')}
                              rows={6}
                              {...field}
                              value={field.value ?? ''}
                            />
                          </FormControl>
                          <FormDescription>
                            {t(
                              'Content displayed on the home page (supports Markdown)'
                            )}
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    <SettingsFormGrid>
                      <FormField
                        control={form.control}
                        name={`legal.user_agreement_${lang.key}` as any}
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>
                              {t('User Agreement')} ({lang.label})
                            </FormLabel>
                            <FormControl>
                              <Textarea
                                placeholder={t(
                                  'Provide Markdown, HTML, or an external URL for the user agreement'
                                )}
                                rows={5}
                                {...field}
                                value={field.value ?? ''}
                              />
                            </FormControl>
                            <FormDescription>
                              {t(
                                'Leave empty to disable the agreement requirement. Supports Markdown, HTML, or a full URL to redirect users.'
                              )}
                            </FormDescription>
                            <FormMessage />
                          </FormItem>
                        )}
                      />

                      <FormField
                        control={form.control}
                        name={`legal.privacy_policy_${lang.key}` as any}
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>
                              {t('Privacy Policy')} ({lang.label})
                            </FormLabel>
                            <FormControl>
                              <Textarea
                                placeholder={t(
                                  'Provide Markdown, HTML, or an external URL for the privacy policy'
                                )}
                                rows={5}
                                {...field}
                                value={field.value ?? ''}
                              />
                            </FormControl>
                            <FormDescription>
                              {t(
                                'Leave empty to disable the privacy policy requirement. Supports Markdown, HTML, or a full URL to redirect users.'
                              )}
                            </FormDescription>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                    </SettingsFormGrid>
                  </TabsContent>
                ))}
              </Tabs>
            </div>
          </SettingsForm>
        </Form>
      </SettingsSection>
    </>
  )
}
