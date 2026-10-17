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
import { AlertCircle, FileText, LifeBuoy, ShieldAlert } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'

import { submitBanAppeal } from '../api'
import type { ByogNodeBan } from '../types'

interface BannedScreenProps {
  banInfo?: ByogNodeBan
}

export function BannedScreen({ banInfo }: BannedScreenProps) {
  const { t } = useTranslation()
  const [appealText, setAppealText] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [dialogOpen, setDialogOpen] = useState(false)

  const handleAppeal = async () => {
    if (!appealText.trim()) {
      toast.error(t('byog.appeal_text_required'))
      return
    }

    try {
      setIsSubmitting(true)
      const res = await submitBanAppeal(banInfo?.node_id || '', appealText)
      toast.success(
        t('byog.appeal_submitted_success', { ticketId: res.ticket_id })
      )
      setDialogOpen(false)
      setAppealText('')
    } catch (err: any) {
      toast.error(err.message || t('byog.appeal_failed'))
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <div className='flex min-h-[600px] w-full items-center justify-center p-4'>
      <Card className='border-destructive/40 bg-destructive/5 max-w-2xl shadow-2xl'>
        <CardHeader className='text-center'>
          <div className='bg-destructive/10 text-destructive mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full'>
            <ShieldAlert className='h-10 w-10' />
          </div>
          <CardTitle className='text-destructive text-2xl font-bold tracking-tight'>
            {t('byog.access_restricted_title')}
          </CardTitle>
          <CardDescription className='text-muted-foreground text-base'>
            {t('byog.access_restricted_description')}
          </CardDescription>
        </CardHeader>

        <CardContent className='space-y-4'>
          <Alert
            variant='destructive'
            className='border-destructive/30 bg-destructive/10'
          >
            <AlertCircle className='h-5 w-5' />
            <AlertTitle className='font-semibold'>
              {t('byog.violation_details')}
            </AlertTitle>
            <AlertDescription className='mt-2 space-y-1 text-sm'>
              <div>
                <span className='font-medium'>{t('byog.violation_code')}:</span>{' '}
                <span className='font-mono font-bold'>
                  {banInfo?.violation_code || '#SEC-LOGPROB-492'}
                </span>
              </div>
              <div>
                <span className='font-medium'>{t('byog.ban_reason')}:</span>{' '}
                <span>
                  {banInfo?.ban_reason || t('byog.default_ban_reason')}
                </span>
              </div>
              {banInfo?.test_id && (
                <div>
                  <span className='font-medium'>{t('byog.model_id')}:</span>{' '}
                  <span className='font-mono'>{banInfo.test_id}</span>
                </div>
              )}
              {banInfo?.expected_logprob !== undefined &&
                banInfo?.actual_logprob !== undefined && (
                  <div className='grid grid-cols-2 gap-2 pt-1 font-mono text-xs'>
                    <div className='bg-background/50 rounded p-2'>
                      {t('byog.expected_logprob')}: {banInfo.expected_logprob}
                    </div>
                    <div className='bg-background/50 text-destructive rounded p-2 font-bold'>
                      {t('byog.actual_logprob')}: {banInfo.actual_logprob}
                    </div>
                  </div>
                )}
            </AlertDescription>
          </Alert>

          <div className='bg-card text-muted-foreground rounded-lg border p-4 text-xs'>
            <div className='text-foreground flex items-center gap-2 font-medium'>
              <FileText className='h-4 w-4' />
              {t('byog.zero_trust_policy_notice')}
            </div>
            <p className='mt-1 leading-relaxed'>
              {t('byog.zero_trust_policy_explanation')}
            </p>
          </div>
        </CardContent>

        <CardFooter className='border-border/50 flex justify-center border-t pt-4'>
          <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
            <DialogTrigger
              render={
                <Button variant='destructive' className='gap-2'>
                  <LifeBuoy className='h-4 w-4' />
                  {t('byog.open_appeal_ticket_button')}
                </Button>
              }
            />
            <DialogContent>
              <DialogHeader>
                <DialogTitle>{t('byog.appeal_modal_title')}</DialogTitle>
                <DialogDescription>
                  {t('byog.appeal_modal_description')}
                </DialogDescription>
              </DialogHeader>
              <div className='space-y-3 py-2'>
                <Label htmlFor='appeal-text'>
                  {t('byog.appeal_statement_label')}
                </Label>
                <Textarea
                  id='appeal-text'
                  rows={5}
                  placeholder={t('byog.appeal_placeholder')}
                  value={appealText}
                  onChange={(e) => setAppealText(e.target.value)}
                />
              </div>
              <DialogFooter>
                <Button variant='outline' onClick={() => setDialogOpen(false)}>
                  {t('common.cancel')}
                </Button>
                <Button onClick={handleAppeal} disabled={isSubmitting}>
                  {isSubmitting
                    ? t('common.submitting')
                    : t('byog.submit_appeal')}
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </CardFooter>
      </Card>
    </div>
  )
}
