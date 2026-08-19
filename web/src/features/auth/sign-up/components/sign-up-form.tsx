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
import { Loader2 } from 'lucide-react'
import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import type { z } from 'zod'

import { Dialog } from '@/components/dialog'
import { PasswordInput } from '@/components/password-input'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { register, wechatLoginByCode } from '@/features/auth/api'
import { CaptchaDialog } from '@/features/auth/components/captcha-dialog'
import {
  OtherLoginDialog,
  OtherLoginTrigger,
} from '@/features/auth/components/other-login-dialog'
import { TermsFooter } from '@/features/auth/components/terms-footer'
import { registerFormSchema } from '@/features/auth/constants'
import { useAuthRedirect } from '@/features/auth/hooks/use-auth-redirect'
import { useCaptcha } from '@/features/auth/hooks/use-captcha'
import { useEmailVerification } from '@/features/auth/hooks/use-email-verification'
import {
  getAffiliateCode,
  saveAffiliateCode,
} from '@/features/auth/lib/storage'
import { useStatus } from '@/hooks/use-status'
import { isAuthBundle } from '@/lib/api'
import { getServerErrorMessageKey } from '@/lib/server-error-message'
import { cn } from '@/lib/utils'

type CaptchaAction = 'register' | 'send-code'

export function SignUpForm({
  className,
  ...props
}: React.HTMLAttributes<HTMLFormElement>) {
  const { t } = useTranslation()
  const [isLoading, setIsLoading] = useState(false)
  const [verificationCode, setVerificationCode] = useState('')
  const [wechatCode, setWeChatCode] = useState('')
  const [isWeChatDialogOpen, setIsWeChatDialogOpen] = useState(false)
  const [isWeChatSubmitting, setIsWeChatSubmitting] = useState(false)
  const [isOtherLoginOpen, setIsOtherLoginOpen] = useState(false)
  const [isCaptchaOpen, setIsCaptchaOpen] = useState(false)
  const pendingCaptchaAction = useRef<CaptchaAction>('register')

  const { status } = useStatus()
  const { isCaptchaEnabled, providers } = useCaptcha()
  const { redirectToLogin, handleLoginSuccess } = useAuthRedirect()
  const {
    isSending: isSendingCode,
    secondsLeft,
    isActive,
    sendCode,
  } = useEmailVerification()

  const form = useForm<z.infer<typeof registerFormSchema>>({
    resolver: zodResolver(registerFormSchema),
    defaultValues: {
      username: '',
      email: '',
      password: '',
      confirmPassword: '',
    },
  })

  const emailValue = form.watch('email')
  const emailVerificationRequired = !!status?.email_verification
  const oauthRegisterEnabled =
    status?.oauth_register_enabled ??
    status?.data?.oauth_register_enabled ??
    true
  const hasWeChatLogin = Boolean(status?.wechat_login)
  const hasOAuthLogin = Boolean(
    status?.github_oauth ||
    status?.discord_oauth ||
    status?.oidc_enabled ||
    status?.linuxdo_oauth ||
    status?.telegram_oauth ||
    (status?.custom_oauth_providers?.length ?? 0) > 0
  )
  const hasOtherLoginMethods = hasWeChatLogin || hasOAuthLogin

  const wechatQrCodeUrl = useMemo(() => {
    return (
      status?.wechat_qrcode ||
      status?.wechat_qr_code ||
      status?.wechat_qrcode_image_url ||
      status?.wechat_qr_code_image_url ||
      status?.wechat_account_qrcode_image_url ||
      status?.WeChatAccountQRCodeImageURL ||
      status?.data?.wechat_qrcode ||
      status?.data?.WeChatAccountQRCodeImageURL ||
      ''
    )
  }, [status])

  useEffect(() => {
    const aff = new URLSearchParams(window.location.search).get('aff')?.trim()
    if (aff) {
      saveAffiliateCode(aff)
    }
  }, [])

  async function onSubmit(data: z.infer<typeof registerFormSchema>) {
    // Validate email verification if required
    if (emailVerificationRequired) {
      if (!data.email) {
        toast.error(t('Please enter your email'))
        return
      }
      if (!verificationCode) {
        toast.error(t('Please enter the verification code'))
        return
      }
    }

    if (isCaptchaEnabled) {
      pendingCaptchaAction.current = 'register'
      setIsCaptchaOpen(true)
      return
    }
    await doRegister(data, '', '')
  }

  async function doRegister(
    data: z.infer<typeof registerFormSchema>,
    captchaToken: string,
    captchaProvider: string
  ) {
    setIsLoading(true)
    try {
      const res = await register({
        username: data.username,
        password: data.password,
        email: data.email || undefined,
        verification_code: verificationCode || undefined,
        aff_code: getAffiliateCode(),
        turnstile: captchaToken,
        captcha_provider: captchaProvider,
      })

      if (res?.success) {
        toast.success(t('Account created! Please sign in'))
        redirectToLogin()
      } else {
        toast.error(res?.message || t('Failed to create account'))
        reopenCaptcha('register')
      }
    } catch {
      // Errors are handled by global interceptor
      reopenCaptcha('register')
    } finally {
      setIsLoading(false)
    }
  }

  /** A rejected request invalidates the captcha token, so re-open the dialog. */
  function reopenCaptcha(action: CaptchaAction) {
    if (!isCaptchaEnabled) return
    pendingCaptchaAction.current = action
    setIsCaptchaOpen(true)
  }

  async function handleSendVerificationCode() {
    if (isCaptchaEnabled) {
      if (!emailValue) {
        toast.error(t('Please enter your email first'))
        return
      }
      pendingCaptchaAction.current = 'send-code'
      setIsCaptchaOpen(true)
      return
    }
    await sendCode(emailValue || '')
  }

  function handleCaptchaVerified(token: string, provider: string) {
    if (pendingCaptchaAction.current === 'send-code') {
      void sendCode(emailValue || '', token, provider)
      return
    }
    void form.handleSubmit((data) => doRegister(data, token, provider))()
  }

  const handleOpenWeChatDialog = () => {
    setIsOtherLoginOpen(false)
    setIsWeChatDialogOpen(true)
  }

  const handleWeChatDialogChange = (open: boolean) => {
    setIsWeChatDialogOpen(open)
    if (!open) {
      setWeChatCode('')
      setIsWeChatSubmitting(false)
    }
  }

  async function handleWeChatLogin() {
    if (!wechatCode.trim()) {
      toast.error(t('Please enter the verification code'))
      return
    }

    setIsWeChatSubmitting(true)
    try {
      const res = await wechatLoginByCode(wechatCode)
      if (res?.success && isAuthBundle(res.data)) {
        await handleLoginSuccess(res.data)
        toast.success(t('Signed in via WeChat'))
        handleWeChatDialogChange(false)
      } else {
        if (getServerErrorMessageKey(res)) return
        toast.error(res?.message || t('Login failed'))
      }
    } catch (error: unknown) {
      if (getServerErrorMessageKey(error)) return
      toast.error(t('Login failed'))
    } finally {
      setIsWeChatSubmitting(false)
    }
  }

  let verificationCodeAction: ReactNode = t('Send code')
  if (isActive) {
    verificationCodeAction = t('Resend ({{seconds}}s)', {
      seconds: secondsLeft,
    })
  } else if (isSendingCode) {
    verificationCodeAction = <Loader2 className='h-4 w-4 animate-spin' />
  }

  return (
    <Form {...form}>
      <form
        onSubmit={form.handleSubmit(onSubmit)}
        className={cn('grid gap-4', className)}
        {...props}
      >
        {/* Username Field */}
        <FormField
          control={form.control}
          name='username'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Username')}</FormLabel>
              <FormControl>
                <Input placeholder={t('Enter your username')} {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        {/* Password Field */}
        <FormField
          control={form.control}
          name='password'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Password')}</FormLabel>
              <FormControl>
                <PasswordInput placeholder={t('Enter password')} {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        {/* Confirm Password Field */}
        <FormField
          control={form.control}
          name='confirmPassword'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Confirm password')}</FormLabel>
              <FormControl>
                <PasswordInput placeholder={t('Confirm password')} {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        {/* Email Verification Section */}
        {emailVerificationRequired && (
          <>
            {/* Email Field */}
            <FormField
              control={form.control}
              name='email'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>
                    {t('Email (required for verification)')}
                  </FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t('name@example.com')}
                      type='email'
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            {/* Verification Code Field */}
            <div className='flex items-end gap-2'>
              <div className='flex-1'>
                <Input
                  placeholder={t('Verification code')}
                  value={verificationCode}
                  onChange={(e) => setVerificationCode(e.target.value)}
                />
              </div>
              <Button
                variant='outline'
                type='button'
                disabled={isLoading || isSendingCode || isActive || !emailValue}
                onClick={handleSendVerificationCode}
              >
                {verificationCodeAction}
              </Button>
            </div>
          </>
        )}

        {/* Submit Button */}
        <Button
          type='submit'
          className='mt-2 w-full justify-center gap-2'
          disabled={isLoading}
        >
          {isLoading ? <Loader2 className='h-4 w-4 animate-spin' /> : null}
          {t('Create account')}
        </Button>

        {/* Legal text + alternative methods */}
        <TermsFooter variant='sign-up' status={status} />
        {hasOtherLoginMethods && oauthRegisterEnabled && (
          <OtherLoginTrigger
            onClick={() => setIsOtherLoginOpen(true)}
            disabled={isLoading}
          />
        )}
      </form>

      <OtherLoginDialog
        open={isOtherLoginOpen}
        onOpenChange={setIsOtherLoginOpen}
        variant='sign-up'
        status={status}
        disabled={isLoading}
        onWeChatLogin={hasWeChatLogin ? handleOpenWeChatDialog : undefined}
        isWeChatLoading={isWeChatSubmitting}
      />

      <CaptchaDialog
        open={isCaptchaOpen}
        onOpenChange={setIsCaptchaOpen}
        providers={providers}
        onVerified={handleCaptchaVerified}
      />

      {hasWeChatLogin && (
        <Dialog
          open={isWeChatDialogOpen}
          onOpenChange={handleWeChatDialogChange}
          title={t('WeChat sign in')}
          description={t(
            'Scan the QR code to follow the official account and reply with “验证码” to receive your verification code.'
          )}
          contentClassName='max-w-sm'
          headerClassName='text-left'
          contentHeight='auto'
          bodyClassName='space-y-4'
          footer={
            <>
              <Button
                type='button'
                variant='outline'
                onClick={() => handleWeChatDialogChange(false)}
                disabled={isWeChatSubmitting}
              >
                {t('Cancel')}
              </Button>
              <Button
                type='button'
                onClick={handleWeChatLogin}
                disabled={isWeChatSubmitting || !wechatCode.trim()}
              >
                {isWeChatSubmitting ? (
                  <Loader2 className='mr-2 h-4 w-4 animate-spin' />
                ) : null}
                {t('Sign in')}
              </Button>
            </>
          }
        >
          <div className='flex flex-col items-center gap-3'>
            {wechatQrCodeUrl ? (
              <img
                src={wechatQrCodeUrl}
                alt={t('WeChat QR code')}
                className='h-40 w-40 rounded-md border object-cover'
              />
            ) : (
              <p className='text-muted-foreground text-sm'>
                {t('WeChat QR code is not configured yet.')}
              </p>
            )}
          </div>
          <div className='grid gap-2'>
            <Label htmlFor='wechat-code'>{t('Verification code')}</Label>
            <Input
              id='wechat-code'
              placeholder={t('Enter the code you received')}
              value={wechatCode}
              onChange={(event) => setWeChatCode(event.target.value)}
              disabled={isWeChatSubmitting}
              onKeyDown={(event) => {
                if (event.key === 'Enter') {
                  event.preventDefault()
                  void handleWeChatLogin()
                }
              }}
            />
          </div>
        </Dialog>
      )}
    </Form>
  )
}
