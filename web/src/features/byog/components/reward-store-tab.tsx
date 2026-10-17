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
import { Coins, Gift, Package, ShieldCheck, Ticket } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
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
} from '@/components/ui/dialog'

import { claimReward } from '../api'
import type { ByogRewardItem } from '../types'

interface RewardStoreTabProps {
  rewards: ByogRewardItem[]
  userQuota: number
  onRewardClaimed: () => void
}

export function RewardStoreTab({
  rewards,
  userQuota,
  onRewardClaimed,
}: RewardStoreTabProps) {
  const { t } = useTranslation()
  const [selectedReward, setSelectedReward] = useState<ByogRewardItem | null>(
    null
  )
  const [isClaiming, setIsClaiming] = useState(false)
  const [confirmOpen, setConfirmOpen] = useState(false)

  const handleClaim = async () => {
    if (!selectedReward) return
    try {
      setIsClaiming(true)
      const res = await claimReward(selectedReward.id)
      toast.success(
        t('byog.reward_claimed_success', {
          item: selectedReward.title,
          ticketId: res.ticket_id,
        })
      )
      setConfirmOpen(false)
      setSelectedReward(null)
      onRewardClaimed()
    } catch (err: any) {
      toast.error(err.message || t('byog.claim_error'))
    } finally {
      setIsClaiming(false)
    }
  }

  return (
    <div className='space-y-6'>
      {/* Header bar showing user token balance */}
      <div className='border-border bg-card flex flex-wrap items-center justify-between gap-4 rounded-xl border p-4'>
        <div>
          <h3 className='flex items-center gap-2 text-lg font-bold'>
            <Gift className='text-primary h-5 w-5' />
            {t('byog.reward_store_title')}
          </h3>
          <p className='text-muted-foreground text-xs'>
            {t('byog.reward_store_description')}
          </p>
        </div>

        <div className='bg-muted/30 flex items-center gap-3 rounded-lg border px-3 py-2'>
          <Coins className='h-4 w-4 text-amber-500' />
          <div>
            <div className='text-muted-foreground text-[10px] font-bold uppercase'>
              {t('byog.available_tokens')}
            </div>
            <div className='text-foreground font-mono font-black'>
              {userQuota.toLocaleString()}
            </div>
          </div>
        </div>
      </div>

      {/* Rewards Grid */}
      <div className='grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4'>
        {rewards.map((item) => {
          const hasEnough = userQuota >= item.required_tokens
          const inStock = item.stock > 0

          return (
            <Card
              key={item.id}
              className='border-border/80 hover:border-primary/50 flex flex-col justify-between transition-all'
            >
              <CardHeader className='pb-3'>
                <div className='flex items-center justify-between gap-2'>
                  <Badge
                    variant='outline'
                    className='text-muted-foreground text-[10px] font-semibold uppercase'
                  >
                    {item.category}
                  </Badge>
                  <span className='text-muted-foreground text-xs'>
                    {t('byog.stock')}: {item.stock}
                  </span>
                </div>
                <CardTitle className='mt-2 text-base font-bold'>
                  {item.title}
                </CardTitle>
                <CardDescription className='line-clamp-2 text-xs'>
                  {item.description}
                </CardDescription>
              </CardHeader>

              <CardContent className='pb-3'>
                <div className='bg-primary/5 border-primary/20 rounded-lg border p-2.5 text-center'>
                  <span className='text-muted-foreground text-[11px]'>
                    {t('byog.required_tokens')}
                  </span>
                  <div className='text-primary font-mono text-lg font-bold'>
                    {item.required_tokens.toLocaleString()}
                  </div>
                </div>
              </CardContent>

              <CardFooter className='border-border/40 border-t pt-3'>
                <Button
                  className='w-full gap-2'
                  disabled={!hasEnough || !inStock}
                  onClick={() => {
                    setSelectedReward(item)
                    setConfirmOpen(true)
                  }}
                >
                  <Package className='h-4 w-4' />
                  {!inStock
                    ? t('byog.out_of_stock')
                    : !hasEnough
                      ? t('byog.insufficient_balance')
                      : t('byog.claim_reward_button')}
                </Button>
              </CardFooter>
            </Card>
          )
        })}
      </div>

      {/* Claim Confirmation Dialog */}
      <Dialog open={confirmOpen} onOpenChange={setConfirmOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle className='flex items-center gap-2'>
              <ShieldCheck className='text-primary h-5 w-5' />
              {t('byog.confirm_claim_title')}
            </DialogTitle>
            <DialogDescription>
              {t('byog.confirm_claim_desc', { item: selectedReward?.title })}
            </DialogDescription>
          </DialogHeader>

          {selectedReward && (
            <div className='space-y-3 py-2 text-sm'>
              <div className='flex justify-between border-b pb-2 text-xs'>
                <span className='text-muted-foreground'>
                  {t('byog.reward_item')}:
                </span>
                <span className='font-bold'>{selectedReward.title}</span>
              </div>
              <div className='flex justify-between border-b pb-2 text-xs'>
                <span className='text-muted-foreground'>
                  {t('byog.burn_deduction')}:
                </span>
                <span className='text-destructive font-mono font-bold'>
                  -{selectedReward.required_tokens.toLocaleString()}{' '}
                  {t('byog.tokens')}
                </span>
              </div>
              <div className='bg-muted/40 text-muted-foreground flex items-start gap-2 rounded-lg p-3 text-xs'>
                <Ticket className='text-primary mt-0.5 h-4 w-4 shrink-0' />
                <p>{t('byog.claim_auto_ticket_explanation')}</p>
              </div>
            </div>
          )}

          <DialogFooter>
            <Button variant='outline' onClick={() => setConfirmOpen(false)}>
              {t('common.cancel')}
            </Button>
            <Button onClick={handleClaim} disabled={isClaiming}>
              {isClaiming
                ? t('common.submitting')
                : t('byog.confirm_burn_and_claim')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
