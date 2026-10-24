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
import { useEffect, useState } from 'react'
import { Link } from '@tanstack/react-router'
import { Check, Copy, KeyRound, ArrowLeft } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { AuthLayout } from '../auth-layout'

export function VerifyCodePage() {
  const { t } = useTranslation()
  const [copied, setCopied] = useState(false)
  const [code, setCode] = useState('')

  useEffect(() => {
    // Add noindex meta tag to prevent search engines from indexing verification codes
    let meta = document.querySelector('meta[name="robots"]')
    let created = false
    if (!meta) {
      meta = document.createElement('meta')
      meta.setAttribute('name', 'robots')
      document.head.appendChild(meta)
      created = true
    }
    meta.setAttribute('content', 'noindex, nofollow')

    // Read code from URL search parameters (?code=XXXXXX)
    const params = new URLSearchParams(window.location.search)
    const urlCode = params.get('code') || ''
    setCode(urlCode.trim())

    return () => {
      if (created && meta && meta.parentNode) {
        meta.parentNode.removeChild(meta)
      }
    }
  }, [])

  const handleCopy = async () => {
    if (!code) return
    try {
      await navigator.clipboard.writeText(code)
      setCopied(true)
      setTimeout(() => setCopied(false), 2500)
    } catch {
      // Fallback if clipboard API is restricted
      const textArea = document.createElement('textarea')
      textArea.value = code
      document.body.appendChild(textArea)
      textArea.select()
      document.execCommand('copy')
      document.body.removeChild(textArea)
      setCopied(true)
      setTimeout(() => setCopied(false), 2500)
    }
  }

  return (
    <AuthLayout>
      <div className='w-full max-w-md mx-auto space-y-6'>
        <div className='space-y-2 text-center sm:text-left'>
          <div className='inline-flex items-center justify-center p-3 mb-2 rounded-2xl bg-primary/10 text-primary'>
            <KeyRound className='w-8 h-8' />
          </div>
          <h2 className='text-2xl font-semibold tracking-tight'>
            {t('E-posta Doğrulama Kodu')}
          </h2>
          <p className='text-muted-foreground text-sm sm:text-base leading-relaxed'>
            {t('Aşağıdaki doğrulama kodunu kopyalayıp işlem yaptığınız sayfadaki alana giriniz.')}
          </p>
        </div>

        <div className='rounded-xl border bg-card p-6 shadow-sm space-y-4'>
          {code ? (
            <div className='space-y-4'>
              <div className='flex items-center justify-center rounded-lg bg-muted/60 py-5 px-4 border border-dashed border-primary/30'>
                <span className='font-mono text-3xl sm:text-4xl font-bold tracking-[0.25em] text-primary select-all'>
                  {code}
                </span>
              </div>

              <Button
                type='button'
                onClick={handleCopy}
                className='w-full h-11 text-base font-medium flex items-center justify-center gap-2 transition-all'
                variant={copied ? 'outline' : 'default'}
              >
                {copied ? (
                  <>
                    <Check className='w-5 h-5 text-green-500 animate-in zoom-in-50' />
                    <span className='text-green-600 font-semibold'>{t('Kopyalandı!')}</span>
                  </>
                ) : (
                  <>
                    <Copy className='w-5 h-5' />
                    <span>{t('Kodu Kopyala')}</span>
                  </>
                )}
              </Button>
            </div>
          ) : (
            <div className='text-center py-6 space-y-2'>
              <p className='text-muted-foreground text-sm'>
                {t('Geçerli bir doğrulama kodu bulunamadı. Lütfen e-postanızdaki bağlantıyı tekrar kontrol edin.')}
              </p>
            </div>
          )}
        </div>

        <div className='text-center sm:text-left'>
          <Link
            to='/'
            className='inline-flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground transition-colors'
          >
            <ArrowLeft className='w-4 h-4' />
            <span>{t('Ana Sayfaya Dön')}</span>
          </Link>
        </div>
      </div>
    </AuthLayout>
  )
}
