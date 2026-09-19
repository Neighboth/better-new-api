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
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { api } from '@/lib/api'

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
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
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

const createEmailSchema = (t: (key: string) => string) =>
  z.object({
    SMTPServer: z.string(),
    SMTPPort: z.string().refine((value) => {
      const trimmed = value.trim()
      if (!trimmed) return true
      return /^\d+$/.test(trimmed)
    }, t('Port must be a positive integer')),
    SMTPAccount: z.string(),
    SMTPFrom: z.string().refine((value) => {
      const trimmed = value.trim()
      if (!trimmed) return true
      return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(trimmed)
    }, t('Enter a valid email or leave blank')),
    SMTPToken: z.string(),
    SMTPSSLEnabled: z.boolean(),
    SMTPStartTLSEnabled: z.boolean(),
    SMTPInsecureSkipVerify: z.boolean(),
    SMTPForceAuthLogin: z.boolean(),
    EmailSubject_verification_tr: z.string().optional(),
    EmailBody_verification_tr: z.string().optional(),
    EmailSubject_verification_en: z.string().optional(),
    EmailBody_verification_en: z.string().optional(),
    EmailSubject_verification_zh_CN: z.string().optional(),
    EmailBody_verification_zh_CN: z.string().optional(),
    EmailSubject_password_reset_tr: z.string().optional(),
    EmailBody_password_reset_tr: z.string().optional(),
    EmailSubject_password_reset_en: z.string().optional(),
    EmailBody_password_reset_en: z.string().optional(),
    EmailSubject_password_reset_zh_CN: z.string().optional(),
    EmailBody_password_reset_zh_CN: z.string().optional(),
  })

type EmailFormValues = z.infer<ReturnType<typeof createEmailSchema>>

type EmailSettingsSectionProps = {
  defaultValues: EmailFormValues
}

type SmtpSecurityMode = 'none' | 'ssl_tls' | 'starttls'

function getSmtpSecurityMode(values: {
  SMTPSSLEnabled: boolean
  SMTPStartTLSEnabled: boolean
}): SmtpSecurityMode {
  if (values.SMTPSSLEnabled) return 'ssl_tls'
  if (values.SMTPStartTLSEnabled) return 'starttls'
  return 'none'
}

export function EmailSettingsSection({
  defaultValues,
}: EmailSettingsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const emailSchema = createEmailSchema(t)

  const [testEmail, setTestEmail] = useState('')
  const [isSendingTest, setIsSendingTest] = useState(false)

  const form = useForm<EmailFormValues>({
    resolver: zodResolver(emailSchema),
    defaultValues,
  })

  useResetForm(form, defaultValues)

  const handleSendTestEmail = async () => {
    if (!testEmail || !testEmail.includes('@')) {
      toast.error(t('Please enter a valid email address'))
      return
    }
    setIsSendingTest(true)
    try {
      const res = await api.post('/api/test_email', { email: testEmail })
      if (res.data?.success) {
        toast.success(t('Test email sent successfully! Please check your inbox and spam folder.'))
      } else {
        toast.error(res.data?.message || t('Failed to send test email'))
      }
    } catch (err: any) {
      toast.error(err.response?.data?.message || err.message || t('Failed to send test email'))
    } finally {
      setIsSendingTest(false)
    }
  }

  const onSubmit = async (values: EmailFormValues) => {
    const securityMode = getSmtpSecurityMode(values)
    const sanitized = {
      SMTPServer: values.SMTPServer.trim(),
      SMTPPort: values.SMTPPort.trim(),
      SMTPAccount: values.SMTPAccount.trim(),
      SMTPFrom: values.SMTPFrom.trim(),
      SMTPToken: values.SMTPToken.trim(),
      SMTPSSLEnabled: securityMode === 'ssl_tls',
      SMTPStartTLSEnabled: securityMode === 'starttls',
      SMTPInsecureSkipVerify: values.SMTPInsecureSkipVerify,
      SMTPForceAuthLogin: values.SMTPForceAuthLogin,
    }

    const initial = {
      SMTPServer: defaultValues.SMTPServer.trim(),
      SMTPPort: defaultValues.SMTPPort.trim(),
      SMTPAccount: defaultValues.SMTPAccount.trim(),
      SMTPFrom: defaultValues.SMTPFrom.trim(),
      SMTPToken: defaultValues.SMTPToken.trim(),
      SMTPSSLEnabled: defaultValues.SMTPSSLEnabled,
      SMTPStartTLSEnabled: defaultValues.SMTPStartTLSEnabled,
      SMTPInsecureSkipVerify: defaultValues.SMTPInsecureSkipVerify,
      SMTPForceAuthLogin: defaultValues.SMTPForceAuthLogin,
    }

    const updates: Array<{ key: string; value: string | boolean }> = []

    if (sanitized.SMTPServer !== initial.SMTPServer) {
      updates.push({ key: 'SMTPServer', value: sanitized.SMTPServer })
    }

    if (sanitized.SMTPPort !== initial.SMTPPort) {
      updates.push({ key: 'SMTPPort', value: sanitized.SMTPPort })
    }

    if (sanitized.SMTPAccount !== initial.SMTPAccount) {
      updates.push({ key: 'SMTPAccount', value: sanitized.SMTPAccount })
    }

    if (sanitized.SMTPFrom !== initial.SMTPFrom) {
      updates.push({ key: 'SMTPFrom', value: sanitized.SMTPFrom })
    }

    if (sanitized.SMTPToken && sanitized.SMTPToken !== initial.SMTPToken) {
      updates.push({ key: 'SMTPToken', value: sanitized.SMTPToken })
    }

    if (sanitized.SMTPSSLEnabled !== initial.SMTPSSLEnabled) {
      updates.push({
        key: 'SMTPSSLEnabled',
        value: sanitized.SMTPSSLEnabled,
      })
    }

    if (sanitized.SMTPStartTLSEnabled !== initial.SMTPStartTLSEnabled) {
      updates.push({
        key: 'SMTPStartTLSEnabled',
        value: sanitized.SMTPStartTLSEnabled,
      })
    }

    if (sanitized.SMTPInsecureSkipVerify !== initial.SMTPInsecureSkipVerify) {
      updates.push({
        key: 'SMTPInsecureSkipVerify',
        value: sanitized.SMTPInsecureSkipVerify,
      })
    }

    if (sanitized.SMTPForceAuthLogin !== initial.SMTPForceAuthLogin) {
      updates.push({
        key: 'SMTPForceAuthLogin',
        value: sanitized.SMTPForceAuthLogin,
      })
    }

    const templateKeys = [
      'EmailSubject_verification_tr',
      'EmailBody_verification_tr',
      'EmailSubject_verification_en',
      'EmailBody_verification_en',
      'EmailSubject_verification_zh_CN',
      'EmailBody_verification_zh_CN',
      'EmailSubject_password_reset_tr',
      'EmailBody_password_reset_tr',
      'EmailSubject_password_reset_en',
      'EmailBody_password_reset_en',
      'EmailSubject_password_reset_zh_CN',
      'EmailBody_password_reset_zh_CN',
    ] as const

    for (const key of templateKeys) {
      const val = values[key] ?? ''
      const initVal = defaultValues[key] ?? ''
      if (val !== initVal) {
        updates.push({ key, value: val })
      }
    }

    for (const update of updates) {
      await updateOption.mutateAsync(update)
    }
  }

  const emailLangs = [
    { key: 'tr', label: 'Türkçe' },
    { key: 'en', label: 'English' },
    { key: 'zh_CN', label: '中文' },
  ] as const

  return (
    <SettingsSection title={t('SMTP Email')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)} autoComplete='off'>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            saveLabel='Save SMTP settings'
          />
          <FormField
            control={form.control}
            name='SMTPServer'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('SMTP Host')}</FormLabel>
                <FormControl>
                  <Input
                    autoComplete='off'
                    placeholder={t('smtp.example.com')}
                    {...field}
                    onChange={(event) => field.onChange(event.target.value)}
                  />
                </FormControl>
                <FormDescription>
                  {t('Hostname or IP of your SMTP provider')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <div className='grid gap-6 md:grid-cols-2'>
            <FormField
              control={form.control}
              name='SMTPPort'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Port')}</FormLabel>
                  <FormControl>
                    <Input
                      autoComplete='off'
                      type='number'
                      placeholder='587'
                      {...field}
                      onChange={(event) => field.onChange(event.target.value)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Common ports include 25, 465, and 587')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormItem>
              <FormLabel>{t('SMTP encryption')}</FormLabel>
              <FormControl>
                <RadioGroup
                  value={getSmtpSecurityMode({
                    SMTPSSLEnabled: form.watch('SMTPSSLEnabled'),
                    SMTPStartTLSEnabled: form.watch('SMTPStartTLSEnabled'),
                  })}
                  onValueChange={(value) => {
                    const mode = value as SmtpSecurityMode
                    form.setValue('SMTPSSLEnabled', mode === 'ssl_tls', {
                      shouldDirty: true,
                    })
                    form.setValue('SMTPStartTLSEnabled', mode === 'starttls', {
                      shouldDirty: true,
                    })
                  }}
                  className='gap-3'
                >
                  <div className='flex items-center gap-2'>
                    <RadioGroupItem value='none' id='smtp-security-none' />
                    <Label
                      htmlFor='smtp-security-none'
                      className='cursor-pointer font-normal'
                    >
                      {t('No encryption')}
                    </Label>
                  </div>
                  <div className='flex items-center gap-2'>
                    <RadioGroupItem
                      value='ssl_tls'
                      id='smtp-security-ssl-tls'
                    />
                    <Label
                      htmlFor='smtp-security-ssl-tls'
                      className='cursor-pointer font-normal'
                    >
                      {t('SSL/TLS')}
                    </Label>
                  </div>
                  <div className='flex items-center gap-2'>
                    <RadioGroupItem
                      value='starttls'
                      id='smtp-security-starttls'
                    />
                    <Label
                      htmlFor='smtp-security-starttls'
                      className='cursor-pointer font-normal'
                    >
                      {t('STARTTLS')}
                    </Label>
                  </div>
                </RadioGroup>
              </FormControl>
              <FormDescription>
                {t('Choose one SMTP transport security mode')}
              </FormDescription>
            </FormItem>

            <FormField
              control={form.control}
              name='SMTPInsecureSkipVerify'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>
                      {t('Skip SMTP TLS certificate verification')}
                    </FormLabel>
                    <FormDescription>
                      {t(
                        'Allow self-signed or hostname-mismatched SMTP certificates'
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

            <FormField
              control={form.control}
              name='SMTPForceAuthLogin'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Force AUTH LOGIN')}</FormLabel>
                    <FormDescription>
                      {t('Force SMTP authentication using AUTH LOGIN method')}
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

          <FormField
            control={form.control}
            name='SMTPAccount'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Username')}</FormLabel>
                <FormControl>
                  <Input
                    autoComplete='off'
                    placeholder={t('noreply@example.com')}
                    {...field}
                    onChange={(event) => field.onChange(event.target.value)}
                  />
                </FormControl>
                <FormDescription>
                  {t('Account used when authenticating with the SMTP server')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='SMTPFrom'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('From Address')}</FormLabel>
                <FormControl>
                  <Input
                    autoComplete='off'
                    placeholder={t('New API <noreply@example.com>')}
                    {...field}
                    onChange={(event) => field.onChange(event.target.value)}
                  />
                </FormControl>
                <FormDescription>
                  {t('Display name and email used in outgoing messages')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='SMTPToken'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Password / Access Token')}</FormLabel>
                <FormControl>
                  <Input
                    autoComplete='off'
                    type='password'
                    placeholder={t('Enter new token to update')}
                    {...field}
                    onChange={(event) => field.onChange(event.target.value)}
                  />
                </FormControl>
                <FormDescription>
                  {t('Leave blank to keep the existing credential')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          {/* Test Email Section */}
          <div className='rounded-lg border p-4 space-y-3 bg-muted/20 mt-4'>
            <div className='font-medium text-sm'>{t('Test SMTP Configuration')}</div>
            <p className='text-xs text-muted-foreground'>
              {t('Send a test verification email to confirm your SMTP configuration is working properly.')}
            </p>
            <div className='flex gap-2 max-w-md'>
              <Input
                type='email'
                placeholder={t('test@example.com')}
                value={testEmail}
                onChange={(e) => setTestEmail(e.target.value)}
              />
              <Button
                type='button'
                variant='outline'
                disabled={isSendingTest}
                onClick={handleSendTestEmail}
              >
                {isSendingTest ? t('Sending...') : t('Send Test Email')}
              </Button>
            </div>
          </div>

          {/* Multilingual Email Templates Section */}
          <div className='space-y-4 pt-6 border-t mt-6'>
            <div>
              <h3 className='text-base font-semibold'>{t('Email Templates')}</h3>
              <p className='text-xs text-muted-foreground mt-1'>
                {t('Customize responsive HTML emails. If left blank, modern responsive built-in templates will be used.')}
              </p>
            </div>

            <Tabs defaultValue='verification' className='w-full'>
              <TabsList className='mb-4'>
                <TabsTrigger value='verification'>{t('Verification Code Email')}</TabsTrigger>
                <TabsTrigger value='password_reset'>{t('Password Reset Email')}</TabsTrigger>
              </TabsList>

              {/* Verification Code Templates */}
              <TabsContent value='verification' className='space-y-4'>
                <div className='text-xs text-muted-foreground p-3 bg-muted/40 rounded-md'>
                  <span className='font-semibold'>{t('Available Variables:')}</span>{' '}
                  <code className='bg-background px-1 py-0.5 rounded'>{'{{.SiteName}}'}</code>,{' '}
                  <code className='bg-background px-1 py-0.5 rounded'>{'{{.Email}}'}</code>,{' '}
                  <code className='bg-background px-1 py-0.5 rounded'>{'{{.Code}}'}</code>,{' '}
                  <code className='bg-background px-1 py-0.5 rounded'>{'{{.Year}}'}</code>
                </div>

                <Tabs defaultValue='tr' className='w-full'>
                  <TabsList className='mb-3'>
                    {emailLangs.map((lang) => (
                      <TabsTrigger key={lang.key} value={lang.key}>
                        {lang.label}
                      </TabsTrigger>
                    ))}
                  </TabsList>

                  {emailLangs.map((lang) => (
                    <TabsContent key={lang.key} value={lang.key} className='space-y-4'>
                      <FormField
                        control={form.control}
                        name={`EmailSubject_verification_${lang.key}` as any}
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('Subject')} ({lang.label})</FormLabel>
                            <FormControl>
                              <Input
                                placeholder={t('Leave blank to use default subject')}
                                {...field}
                                value={field.value ?? ''}
                              />
                            </FormControl>
                            <FormMessage />
                          </FormItem>
                        )}
                      />

                      <FormField
                        control={form.control}
                        name={`EmailBody_verification_${lang.key}` as any}
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('HTML Content')} ({lang.label})</FormLabel>
                            <FormControl>
                              <Textarea
                                rows={6}
                                placeholder={t('Leave blank to use default modern responsive HTML template')}
                                {...field}
                                value={field.value ?? ''}
                              />
                            </FormControl>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                    </TabsContent>
                  ))}
                </Tabs>
              </TabsContent>

              {/* Password Reset Templates */}
              <TabsContent value='password_reset' className='space-y-4'>
                <div className='text-xs text-muted-foreground p-3 bg-muted/40 rounded-md'>
                  <span className='font-semibold'>{t('Available Variables:')}</span>{' '}
                  <code className='bg-background px-1 py-0.5 rounded'>{'{{.SiteName}}'}</code>,{' '}
                  <code className='bg-background px-1 py-0.5 rounded'>{'{{.Email}}'}</code>,{' '}
                  <code className='bg-background px-1 py-0.5 rounded'>{'{{.ResetUrl}}'}</code>,{' '}
                  <code className='bg-background px-1 py-0.5 rounded'>{'{{.Year}}'}</code>
                </div>

                <Tabs defaultValue='tr' className='w-full'>
                  <TabsList className='mb-3'>
                    {emailLangs.map((lang) => (
                      <TabsTrigger key={lang.key} value={lang.key}>
                        {lang.label}
                      </TabsTrigger>
                    ))}
                  </TabsList>

                  {emailLangs.map((lang) => (
                    <TabsContent key={lang.key} value={lang.key} className='space-y-4'>
                      <FormField
                        control={form.control}
                        name={`EmailSubject_password_reset_${lang.key}` as any}
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('Subject')} ({lang.label})</FormLabel>
                            <FormControl>
                              <Input
                                placeholder={t('Leave blank to use default subject')}
                                {...field}
                                value={field.value ?? ''}
                              />
                            </FormControl>
                            <FormMessage />
                          </FormItem>
                        )}
                      />

                      <FormField
                        control={form.control}
                        name={`EmailBody_password_reset_${lang.key}` as any}
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('HTML Content')} ({lang.label})</FormLabel>
                            <FormControl>
                              <Textarea
                                rows={6}
                                placeholder={t('Leave blank to use default modern responsive HTML template')}
                                {...field}
                                value={field.value ?? ''}
                              />
                            </FormControl>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                    </TabsContent>
                  ))}
                </Tabs>
              </TabsContent>
            </Tabs>
          </div>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
