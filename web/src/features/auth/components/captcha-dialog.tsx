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
import { Loader2, RefreshCw } from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { HCaptchaWidget, RecaptchaWidget } from '@/components/captcha-widget'
import { Dialog } from '@/components/dialog'
import { Turnstile } from '@/components/turnstile'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import type { CaptchaProviderOption } from '@/features/auth/hooks/use-captcha'
import { api } from '@/lib/api'

type CaptchaDialogProps = {
  open: boolean
  providers: CaptchaProviderOption[]
  onOpenChange: (open: boolean) => void
  onVerified: (token: string, provider: string) => void
}

/**
 * Human-verification dialog shown when the user submits an auth form.
 *
 * Solving the challenge never submits anything by itself — the user always
 * confirms with the Continue button. The Refresh button stays disabled until
 * the server rejects a submission, then it regenerates the challenge. If a
 * provider widget fails to load, the dialog automatically falls back to the
 * next configured provider.
 */
export function CaptchaDialog({
  open,
  providers,
  onOpenChange,
  onVerified,
}: CaptchaDialogProps) {
  const { t } = useTranslation()
  const [providerIndex, setProviderIndex] = useState(0)
  const [token, setToken] = useState('')
  const [widgetKey, setWidgetKey] = useState(0)
  const [failed, setFailed] = useState(false)
  const submittedRef = useRef(false)

  const provider = providers[Math.min(providerIndex, providers.length - 1)]

  useEffect(() => {
    if (!open) return
    setProviderIndex(0)
    setToken('')
    setWidgetKey((current) => current + 1)
    // Reopening after a submission means the server rejected us — only then
    // does the refresh button become available.
    setFailed(submittedRef.current)
    submittedRef.current = false
  }, [open])

  const handleWidgetError = useCallback(() => {
    setProviderIndex((current) => {
      if (current >= providers.length - 1) {
        toast.error(t('Failed to load captcha'))
        return current
      }
      toast.info(t('Captcha failed to load, trying another verification'))
      return current + 1
    })
    setToken('')
  }, [providers.length, t])

  const handleWidgetExpired = useCallback(() => {
    setToken('')
    setWidgetKey((current) => current + 1)
    toast.error(t('Captcha expired or failed, please try again'))
  }, [t])

  const handleRefresh = () => {
    setToken('')
    setWidgetKey((current) => current + 1)
    setFailed(false)
  }

  const handleContinue = () => {
    if (!token || !provider) return
    submittedRef.current = true
    onOpenChange(false)
    onVerified(token, provider.type)
  }

  if (!provider) return null

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={t('Human verification')}
      description={t('Please complete the verification below to continue.')}
      contentClassName='max-w-sm'
      headerClassName='text-left'
      contentHeight='auto'
      bodyClassName='space-y-4'
    >
      <div className='flex flex-col items-center gap-4'>
        {provider.type === 'turnstile' && (
          <Turnstile
            key={`turnstile-${widgetKey}`}
            siteKey={provider.siteKey}
            onVerify={setToken}
            onExpire={handleWidgetExpired}
            onError={handleWidgetError}
          />
        )}
        {provider.type === 'recaptcha' && (
          <RecaptchaWidget
            key={`recaptcha-${widgetKey}`}
            siteKey={provider.siteKey}
            onVerify={setToken}
            onExpire={handleWidgetExpired}
            onError={handleWidgetError}
          />
        )}
        {provider.type === 'hcaptcha' && (
          <HCaptchaWidget
            key={`hcaptcha-${widgetKey}`}
            siteKey={provider.siteKey}
            onVerify={setToken}
            onExpire={handleWidgetExpired}
            onError={handleWidgetError}
          />
        )}
        {provider.type === 'image' && (
          <ImageChallenge key={`image-${widgetKey}`} onChange={setToken} />
        )}
      </div>

      <div className='flex gap-2'>
        <Button
          type='button'
          variant='outline'
          className='gap-2'
          disabled={!failed}
          onClick={handleRefresh}
        >
          <RefreshCw className='h-4 w-4' />
          {t('Refresh captcha')}
        </Button>
        <Button
          type='button'
          className='flex-1'
          disabled={!token}
          onClick={handleContinue}
        >
          {t('Continue')}
        </Button>
      </div>
    </Dialog>
  )
}

type ImageChallengeProps = {
  onChange: (packedToken: string) => void
}

/**
 * Self-hosted image captcha. Reports a packed "captchaId:answer" token upward
 * once the user typed an answer, or an empty string while incomplete.
 */
function ImageChallenge({ onChange }: ImageChallengeProps) {
  const { t } = useTranslation()
  const [captchaId, setCaptchaId] = useState('')
  const [image, setImage] = useState('')
  const [answer, setAnswer] = useState('')
  const [isLoading, setIsLoading] = useState(false)

  const load = useCallback(async () => {
    setIsLoading(true)
    setAnswer('')
    onChange('')
    try {
      const res = await api.get('/api/captcha/image')
      if (res.data?.success) {
        setCaptchaId(res.data.data.captcha_id)
        setImage(res.data.data.image)
      } else {
        toast.error(res.data?.message || t('Failed to load captcha'))
      }
    } catch {
      toast.error(t('Failed to load captcha'))
    } finally {
      setIsLoading(false)
    }
  }, [onChange, t])

  useEffect(() => {
    void load()
    // Load exactly once per mount; the parent remounts us to regenerate.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  return (
    <div className='w-full space-y-3'>
      <div className='relative flex justify-center'>
        {image ? (
          <img
            src={image}
            alt={t('Captcha image')}
            className='h-[60px] w-[180px] rounded-md border'
          />
        ) : (
          <div className='bg-muted flex h-[60px] w-[180px] items-center justify-center rounded-md border'>
            <Loader2 className='h-4 w-4 animate-spin' />
          </div>
        )}
        <Button
          type='button'
          variant='ghost'
          size='icon'
          className='absolute -end-1 top-0 h-8 w-8'
          onClick={() => void load()}
          disabled={isLoading}
          aria-label={t('Load a new captcha image')}
        >
          <RefreshCw className={isLoading ? 'h-4 w-4 animate-spin' : 'h-4 w-4'} />
        </Button>
      </div>
      <div className='grid gap-2'>
        <Label htmlFor='image-captcha-answer'>{t('Enter the code shown')}</Label>
        <Input
          id='image-captcha-answer'
          value={answer}
          onChange={(event) => {
            const value = event.target.value
            setAnswer(value)
            onChange(
              captchaId && value.trim() ? `${captchaId}:${value.trim()}` : ''
            )
          }}
          autoComplete='off'
          maxLength={8}
        />
      </div>
    </div>
  )
}
