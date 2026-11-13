/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

For commercial licensing, please contact support@quantumnous.com
*/
import { useTranslation } from 'react-i18next'
import { Badge } from '@/components/ui/badge'
import { formatQuota } from '@/lib/format'
import { Layers, Gift, Wallet as WalletIcon, ShieldBan } from 'lucide-react'

import type { UserWalletData } from '../types'

interface WalletBreakdownCardProps {
  user: UserWalletData | null
}

/**
 * Detailed balance breakdown for the wallet page.
 *
 * Shows the user how much of the balance is topped up (usable for every
 * model and every channel), how much comes from invitation rewards (and
 * which channels invitation rewards may not use), and the overall display
 * total. Model-restricted packages are listed by the wallet page in their
 * own card below this one.
 */
export function WalletBreakdownCard({ user }: WalletBreakdownCardProps) {
  const { t } = useTranslation()

  const quota = user?.quota ?? 0
  const referralQuota = user?.referral_quota ?? 0
  const referralDisplay = user?.referral_display ?? 0
  const totalDisplay = quota + referralDisplay
  const excludedChannels = user?.referral_excluded_channels ?? []

  return (
    <div className='rounded-lg border bg-card p-4 shadow-xs'>
      <div className='flex items-center gap-2 mb-3'>
        <Layers className='h-4 w-4 text-primary' />
        <h3 className='text-sm font-semibold'>{t('Balance Breakdown')}</h3>
      </div>

      <dl className='grid gap-2.5 sm:grid-cols-2'>
        {/* Topped-up balance */}
        <div className='rounded-md border bg-muted/20 p-3 space-y-1'>
          <dt className='flex items-center gap-1.5 text-[11px] font-medium uppercase tracking-wider text-muted-foreground'>
            <WalletIcon className='h-3.5 w-3.5' />
            {t('Topped-up balance')}
          </dt>
          <dd className='text-base font-bold font-mono'>{formatQuota(quota)}</dd>
          <p className='text-[11px] text-muted-foreground'>
            {t('usable for all models and all channels')}
          </p>
        </div>

        {/* Invitation rewards */}
        <div className='rounded-md border bg-muted/20 p-3 space-y-1'>
          <dt className='flex items-center gap-1.5 text-[11px] font-medium uppercase tracking-wider text-muted-foreground'>
            <Gift className='h-3.5 w-3.5' />
            {t('Invitation rewards')}
          </dt>
          <dd className='text-base font-bold font-mono'>
            {formatQuota(referralQuota)}
            <span className='ml-2 text-xs font-semibold text-emerald-600 dark:text-emerald-400'>
              {t('shown as')} {formatQuota(referralDisplay)}
            </span>
          </dd>
          <p className='text-[11px] text-muted-foreground'>
            {t('Spent before topped-up balance on every settlement')}
          </p>
        </div>

        {/* Channels unavailable for invitation rewards */}
        <div className='rounded-md border bg-muted/20 p-3 space-y-1.5 sm:col-span-2'>
          <dt className='flex items-center gap-1.5 text-[11px] font-medium uppercase tracking-wider text-muted-foreground'>
            <ShieldBan className='h-3.5 w-3.5' />
            {t('Channels unusable for invitation rewards')}
          </dt>
          <dd>
            {excludedChannels.length > 0 ? (
              <div className='flex flex-wrap gap-1.5 pt-0.5'>
                {excludedChannels.map((ch) => (
                  <Badge key={ch.id} variant='secondary' className='text-[11px] font-normal'>
                    {ch.name}
                  </Badge>
                ))}
              </div>
            ) : (
              <span className='text-xs text-muted-foreground'>{t('None')}</span>
            )}
          </dd>
        </div>

        {/* Total available */}
        <div className='rounded-md border bg-primary/5 p-3 space-y-1 sm:col-span-2 flex flex-wrap items-center justify-between gap-2'>
          <div>
            <dt className='text-[11px] font-medium uppercase tracking-wider text-muted-foreground'>
              {t('Total available')}
            </dt>
            <dd>
              <span className='text-lg font-extrabold font-mono'>
                {formatQuota(totalDisplay)}
              </span>
            </dd>
          </div>
          <span className='text-[11px] text-muted-foreground max-w-56'>
            {t('Balance restricted to specific models is listed below.')}
          </span>
        </div>
      </dl>
    </div>
  )
}
