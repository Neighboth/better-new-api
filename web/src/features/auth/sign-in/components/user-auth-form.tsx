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
import { Link } from '@tanstack/react-router'
import axios from 'axios'
import { Loader2, LogIn } from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'
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
import { login, wechatLoginByCode } from '@/features/auth/api'
import { CaptchaDialog } from '@/features/auth/components/captcha-dialog'
import {
  OtherLoginDialog,
  OtherLoginTrigger,
} from '@/features/auth/components/other-login-dialog'
import { TermsFooter } from '@/features/auth/components/terms-footer'
import { loginFormSchema } from '@/features/auth/constants'
import { useAuthRedirect } from '@/features/auth/hooks/use-auth-redirect'
import { useCaptcha } from '@/features/auth/hooks/use-captcha'
import { beginPasskeyLogin, finishPasskeyLogin } from '@/features/auth/passkey'
import type { AuthFormProps } from '@/features/auth/types'
import { useStatus } from '@/hooks/use-status'
import { isAuthBundle } from '@/lib/api'
import {
  buildAssertionResult,
  prepareCredentialRequestOptions,
  isPasskeySupported as detectPasskeySupport,
} from '@/lib/passkey'
import { getServerErrorMessageKey } from '@/lib/server-error-message'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'

export function UserAuthForm({
  className,
  redirectTo,
  ...props
}: AuthFormProps) {
  const { t } = useTranslation()
  const [isLoading, setIsLoading] = useState(false)
  const [wechatCode, setWeChatCode] = useState('')
  const [passkeySupported, setPasskeySupported] = useState(false)
  const [isPasskeyLoading, setIsPasskeyLoading] = useState(false)
  const [isWeChatDialogOpen, setIsWeChatDialogOpen] = useState(false)
  const [isWeChatSubmitting, setIsWeChatSubmitting] = useState(false)
  const [isOtherLoginOpen, setIsOtherLoginOpen] = useState(false)
  const [isCaptchaOpen, setIsCaptchaOpen] = useState(false)
  const pendingFormData = useRef<z.infer<typeof loginFormSchema> | null>(null)
  const loginFailedMessage = t('Login failed')

  const { status } = useStatus()
  const { isCaptchaEnabled, providers } = useCaptcha()
  const { handleLoginSuccess, redirectTo2FA } = useAuthRedirect()
  const setPending2FAFlowToken = useAuthStore(
    (state) => state.auth.setPending2FAFlowToken
  )

  const passkeyLoginEnabled = Boolean(
    status?.passkey_login ?? status?.data?.passkey_login
  )
  const passwordLoginEnabled =
    (status?.password_login_enabled ??
      status?.data?.password_login_enabled ??
      true) !== false
  const hasWeChatLogin = Boolean(status?.wechat_login)
  const hasOAuthLogin = Boolean(
    status?.github_oauth ||
    status?.discord_oauth ||
    status?.oidc_enabled ||
    status?.linuxdo_oauth ||
    status?.telegram_oauth ||
    (status?.custom_oauth_providers?.length ?? 0) > 0
  )
  const hasOtherLoginMethods =
    passkeyLoginEnabled || hasWeChatLogin || hasOAuthLogin

  useEffect(() => {
    detectPasskeySupport()
      .then(setPasskeySupported)
      .catch(() => setPasskeySupported(false))
  }, [])

  const form = useForm<z.infer<typeof loginFormSchema>>({
    resolver: zodResolver(loginFormSchema),
    defaultValues: {
      username: '',
      password: '',
    },
  })

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

  async function onSubmit(data: z.infer<typeof loginFormSchema>) {
    if (isCaptchaEnabled) {
      pendingFormData.current = data
      setIsCaptchaOpen(true)
      return
    }
    await doLogin(data, '', '')
  }

  async function doLogin(
    data: z.infer<typeof loginFormSchema>,
    captchaToken: string,
    captchaProvider: string
  ) {
    setIsLoading(true)
    try {
      const res = await login({
        username: data.username,
        password: data.password,
        turnstile: captchaToken,
        captcha_provider: captchaProvider,
      })

      if (res.success) {
        if (res.data && 'require_2fa' in res.data && res.data.require_2fa) {
          if (!res.data.flow_token) {
            throw new Error(t('Login flow expired. Please sign in again.'))
          }
          setPending2FAFlowToken(res.data.flow_token)
          redirectTo2FA()
          return
        }

        if (!isAuthBundle(res.data)) {
          throw new Error(t('Login failed'))
        }
        await handleLoginSuccess(res.data, redirectTo)
        toast.success(t('Welcome back!'))
      } else {
        if (getServerErrorMessageKey(res)) return
        toast.error(res?.message || loginFailedMessage)
        reopenCaptcha(data)
      }
    } catch (error: unknown) {
      if (axios.isAxiosError(error)) {
        reopenCaptcha(data)
        return
      }
      toast.error(error instanceof Error ? error.message : loginFailedMessage)
      reopenCaptcha(data)
    } finally {
      setIsLoading(false)
    }
  }

  /** A rejected login invalidates the captcha token, so re-open the dialog. */
  function reopenCaptcha(data: z.infer<typeof loginFormSchema>) {
    if (!isCaptchaEnabled) return
    pendingFormData.current = data
    setIsCaptchaOpen(true)
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
        await handleLoginSuccess(res.data, redirectTo)
        toast.success(t('Signed in via WeChat'))
        handleWeChatDialogChange(false)
      } else {
        if (getServerErrorMessageKey(res)) return
        toast.error(res?.message || loginFailedMessage)
      }
    } catch (error: unknown) {
      if (getServerErrorMessageKey(error)) return
      toast.error(loginFailedMessage)
    } finally {
      setIsWeChatSubmitting(false)
    }
  }

  async function handlePasskeyLogin() {
    if (!passkeySupported) {
      toast.error(t('Passkey is not supported on this device'))
      return
    }

    if (!navigator?.credentials) {
      toast.error(t('Passkey is not available in this browser'))
      return
    }

    setIsPasskeyLoading(true)
    try {
      const begin = await beginPasskeyLogin()
      if (!begin.success) {
        if (getServerErrorMessageKey(begin)) return
        throw new Error(begin.message || t('Failed to start Passkey login'))
      }

      const publicKey = prepareCredentialRequestOptions(
        begin.data?.options ?? begin.data
      )
      const flowToken = begin.data?.flow_token
      if (!flowToken) {
        throw new Error(t('Login flow expired. Please sign in again.'))
      }

      const credential = (await navigator.credentials.get({
        publicKey,
      })) as PublicKeyCredential | null

      if (!credential) {
        toast.info(t('Passkey login was cancelled'))
        return
      }

      const assertion = buildAssertionResult(credential)
      if (!assertion) {
        throw new Error(t('Invalid Passkey response'))
      }

      const finish = await finishPasskeyLogin(flowToken, assertion)
      if (!finish.success) {
        if (getServerErrorMessageKey(finish)) return
        throw new Error(finish.message || t('Failed to complete Passkey login'))
      }

      if (!isAuthBundle(finish.data)) {
        throw new Error(t('Missing user data from Passkey login response'))
      }

      await handleLoginSuccess(finish.data, redirectTo)
      toast.success(t('Signed in with Passkey'))
      setIsOtherLoginOpen(false)
    } catch (error: unknown) {
      if (getServerErrorMessageKey(error)) return
      if (error instanceof DOMException && error.name === 'NotAllowedError') {
        toast.info(t('Passkey login was cancelled or timed out'))
      } else if (error instanceof Error) {
        toast.error(error.message)
      } else {
        toast.error(t('Passkey login failed'))
      }
    } finally {
      setIsPasskeyLoading(false)
    }
  }

  return (
    <Form {...form}>
      <form
        onSubmit={form.handleSubmit(onSubmit)}
        className={cn('grid gap-4', className)}
        {...props}
      >
        {passwordLoginEnabled && (
          <>
            {/* Username Field */}
            <FormField
              control={form.control}
              name='username'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Username or Email')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t('Enter your username or email')}
                      {...field}
                    />
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
                <FormItem className='relative'>
                  <FormLabel>{t('Password')}</FormLabel>
                  <FormControl>
                    <PasswordInput
                      placeholder={t('Enter password')}
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                  <Link
                    to='/forgot-password'
                    className='text-muted-foreground absolute end-0 -top-0.5 z-10 text-sm font-medium hover:opacity-75'
                  >
                    {t('Forgot password?')}
                  </Link>
                </FormItem>
              )}
            />

            {/* Submit Button */}
            <Button
              type='submit'
              className='mt-2 w-full justify-center gap-2'
              disabled={isLoading}
            >
              {isLoading ? <Loader2 className='animate-spin' /> : <LogIn />}
              {t('Sign in')}
            </Button>
          </>
        )}

        {/* Legal text + alternative methods */}
        <TermsFooter variant='sign-in' status={status} />
        {hasOtherLoginMethods && (
          <OtherLoginTrigger
            onClick={() => setIsOtherLoginOpen(true)}
            disabled={isLoading}
          />
        )}
      </form>

      <OtherLoginDialog
        open={isOtherLoginOpen}
        onOpenChange={setIsOtherLoginOpen}
        variant='sign-in'
        status={status}
        redirectTo={redirectTo}
        disabled={isLoading}
        passkeyEnabled={passkeyLoginEnabled}
        passkeySupported={passkeySupported}
        isPasskeyLoading={isPasskeyLoading}
        onPasskey={handlePasskeyLogin}
        onWeChatLogin={hasWeChatLogin ? handleOpenWeChatDialog : undefined}
        isWeChatLoading={isWeChatSubmitting}
      />

      <CaptchaDialog
        open={isCaptchaOpen}
        onOpenChange={setIsCaptchaOpen}
        providers={providers}
        onVerified={(token, provider) => {
          const data = pendingFormData.current
          if (data) {
            void doLogin(data, token, provider)
          }
        }}
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
                className='gap-2'
              >
                {isWeChatSubmitting ? (
                  <Loader2 className='h-4 w-4 animate-spin' />
                ) : null}
                {t('Confirm')}
              </Button>
            </>
          }
        >
          {wechatQrCodeUrl ? (
            <div className='flex justify-center'>
              <img
                src={wechatQrCodeUrl}
                alt={t('WeChat QR code')}
                className='h-40 w-40 rounded-md border object-cover'
              />
            </div>
          ) : (
            <p className='text-muted-foreground text-sm'>
              {t('WeChat QR code is not configured yet.')}
            </p>
          )}
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
