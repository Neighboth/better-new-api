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
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { CreditCard, Eye, EyeOff, Info, Loader2, Save, ShieldCheck } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

import { updateResellerConfig } from '../api'
import type { ResellerConfig } from '../types'

interface EPayTabProps {
  config: ResellerConfig
}

export function EPayTab({ config }: EPayTabProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  const [partnerId, setPartnerId] = useState(config.epay_partner_id || '')
  const [partnerKey, setPartnerKey] = useState(config.epay_key || config.epay_partner_key || '')
  const [gatewayUrl, setGatewayUrl] = useState(config.epay_url || config.epay_gateway_url || '')
  const [showKey, setShowKey] = useState(false)

  useEffect(() => {
    setPartnerId(config.epay_partner_id || '')
    setPartnerKey(config.epay_key || config.epay_partner_key || '')
    setGatewayUrl(config.epay_url || config.epay_gateway_url || '')
  }, [config])

  const saveMutation = useMutation({
    mutationFn: async () => {
      return updateResellerConfig({
        epay_partner_id: partnerId.trim(),
        epay_key: partnerKey.trim(),
        epay_partner_key: partnerKey.trim(),
        epay_url: gatewayUrl.trim(),
        epay_gateway_url: gatewayUrl.trim(),
      })
    },
    onSuccess: () => {
      toast.success(t('Payment gateway settings saved successfully'))
      queryClient.invalidateQueries({ queryKey: ['reseller-self-config'] })
    },
    onError: (err: any) => {
      toast.error(err.message || t('Failed to save payment gateway settings'))
    },
  })

  return (
    <div className='space-y-6'>
      <Alert className='border-blue-500/50 bg-blue-500/10 text-blue-700 dark:text-blue-400'>
        <Info className='h-5 w-5' />
        <AlertTitle className='font-semibold'>{t('Payment Gateway (EPay)')}</AlertTitle>
        <AlertDescription className='text-xs mt-1 leading-relaxed'>
          {t(
            'Configure your own EPay gateway to accept customer payments directly through your child panel. Customers topping up balances on your custom domain will be directed through your personal payment merchant.'
          )}
        </AlertDescription>
      </Alert>

      <Card>
        <CardHeader>
          <div className='flex items-center gap-2'>
            <CreditCard className='h-5 w-5 text-primary' />
            <CardTitle>{t('EPay Merchant Configuration')}</CardTitle>
          </div>
          <CardDescription>
            {t('Enter your EPay partner credentials and gateway endpoint below.')}
          </CardDescription>
        </CardHeader>
        <CardContent className='space-y-4'>
          <div className='grid gap-2'>
            <Label htmlFor='epay-partner-id'>{t('EPay Partner ID')}</Label>
            <Input
              id='epay-partner-id'
              placeholder='e.g. 1000'
              value={partnerId}
              onChange={(e) => setPartnerId(e.target.value)}
            />
            <p className='text-xs text-muted-foreground'>
              {t('Your unique merchant identifier provided by your EPay service provider.')}
            </p>
          </div>

          <div className='grid gap-2'>
            <Label htmlFor='epay-partner-key'>{t('EPay Partner Key')}</Label>
            <div className='relative'>
              <Input
                id='epay-partner-key'
                type={showKey ? 'text' : 'password'}
                placeholder='e.g. your_secret_merchant_key'
                value={partnerKey}
                onChange={(e) => setPartnerKey(e.target.value)}
                className='pr-10'
              />
              <button
                type='button'
                onClick={() => setShowKey(!showKey)}
                className='absolute right-2.5 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground'
                tabIndex={-1}
              >
                {showKey ? <EyeOff className='h-4 w-4' /> : <Eye className='h-4 w-4' />}
              </button>
            </div>
            <p className='text-xs text-muted-foreground'>
              {t('Merchant secret key used for MD5 request signing and transaction verification.')}
            </p>
          </div>

          <div className='grid gap-2'>
            <Label htmlFor='epay-gateway-url'>{t('EPay Gateway URL')}</Label>
            <Input
              id='epay-gateway-url'
              placeholder='https://pay.example.com/'
              value={gatewayUrl}
              onChange={(e) => setGatewayUrl(e.target.value)}
            />
            <p className='text-xs text-muted-foreground'>
              {t('Base URL of the EPay gateway (must start with https:// or http://).')}
            </p>
          </div>

          <div className='rounded-lg border border-border/60 bg-muted/40 p-3 flex items-start gap-2.5 text-xs text-muted-foreground mt-4'>
            <ShieldCheck className='h-4 w-4 text-emerald-600 mt-0.5 shrink-0' />
            <span>
              {t(
                'Payment credentials are encrypted and stored safely. They will only be invoked when users process top-ups on your assigned child panel domain.'
              )}
            </span>
          </div>

          <div className='flex justify-end pt-4'>
            <Button
              onClick={() => saveMutation.mutate()}
              disabled={saveMutation.isPending}
              className='gap-2'
            >
              {saveMutation.isPending ? (
                <Loader2 className='h-4 w-4 animate-spin' />
              ) : (
                <Save className='h-4 w-4' />
              )}
              {t('Save Payment Settings')}
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
