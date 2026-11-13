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
import { TitledCard } from '@/components/ui/titled-card'
import { formatQuota } from '@/lib/format'
import { Layers, Gift, Wallet as WalletIcon } from 'lucide-react'

import type { UserWalletData } from '../types'
import type { UserBalancePackage } from '../api'

interface WalletBreakdownCardProps {
  user: UserWalletData | null
  packages?: UserBalancePackage[]
}

/**
 * Balance breakdown for the wallet page.
 * Shows topped-up balance, invitation rewards (and which channels they
 * cannot use) and the available total. Visual language matches the
 * "Add Funds" card: icon badge on the left, title + description to the
 * right.
 */
export function WalletBreakdownCard({ user, packages }: WalletBreakdownCardProps) {
  const { t } = useTranslation()

  const quota = user?.quota ?? 0
  const referralQuota = user?.referral_quota ?? 0
  const referralDisplay = user?.referral_display ?? 0
  const totalDisplay = quota + referralDisplay
  const excluded = user?.referral_excluded_channels ?? []

  return (
    <TitledCard
      title={t('Balance Breakdown')}
      description={t('Where the displayed balance comes from')}
      icon={<Layers className='h-4 w-4' />}
      iconTone='chart-2'
      disableHoverEffect
    >
      <dl className='grid gap-3 text-sm sm:grid-cols-3'>
        <div className='flex items-start gap-2'>
          <WalletIcon className='text-muted-foreground mt-0.5 h-4 w-4 shrink-0' />
          <div className='min-w-0'>
            <dt className='text-muted-foreground text-[11px] font-medium uppercase tracking-wider'>
              {t('Topped-up balance')}
            </dt>
            <dd className='font-mono text-base font-bold'>{formatQuota(quota)}</dd>
            <p className='text-muted-foreground text-[11px]'>
              {t('usable for all models')}
            </p>
          </div>
        </div>
        <div className='flex items-start gap-2'>
          <Gift className='text-muted-foreground mt-0.5 h-4 w-4 shrink-0' />
          <div className='min-w-0'>
            <dt className='text-muted-foreground text-[11px] font-medium uppercase tracking-wider'>
              {t('Invitation rewards')}
            </dt>
            <dd className='font-mono text-base font-bold'>
              {formatQuota(referralQuota)}
              <span className='text-muted-foreground ml-2 text-xs font-semibold'>
                {t('shown as')} {formatQuota(referralDisplay)}
              </span>
            </dd>
            <p className='text-muted-foreground text-[11px]'>
              {t('Spent before topped-up balance.')}
            </p>
          </div>
        </div>
        <div className='flex items-start gap-2'>
          <div className='bg-primary/10 text-primary mt-0.5 flex h-4 w-4 shrink-0 items-center justify-center rounded-sm text-[10px]'>
            Σ
          </div>
          <div className='min-w-0'>
            <dt className='text-muted-foreground text-[11px] font-medium uppercase tracking-wider'>
              {t('Total available')}
            </dt>
            <dd className='font-mono text-base font-bold'>
              {formatQuota(totalDisplay)}
            </dd>
          </div>
        </div>
      </dl>

      <div className='mt-3 border-t pt-3'>
        <div className='flex items-center justify-between gap-2'>
          <div className='text-muted-foreground text-[11px] font-medium uppercase tracking-wider'>
            {t('Channels unusable for invitation rewards')}
          </div>
        </div>
        <div className='mt-1 flex flex-wrap gap-1.5'>
          {excluded.length > 0 ? (
            excluded.map((ch) => (
              <Badge
                key={ch.id}
                variant='secondary'
                className='text-[11px] font-normal'
              >
                {ch.name}
              </Badge>
            ))
          ) : (
            <span className='text-muted-foreground text-xs'>{t('None')}</span>
          )}
        </div>
      </div>

      <div className='mt-3 border-t pt-3'>
        <div className='flex items-center justify-between gap-2'>
          <div className='text-muted-foreground text-[11px] font-medium uppercase tracking-wider'>
            {t('Redemption packages')}
          </div>
          {packages && packages.length > 0 ? (
            <Badge variant='outline' className='text-[10px] uppercase'>
              {packages.length} {t('Active')}
            </Badge>
          ) : null}
        </div>
        {packages && packages.length > 0 ? (
          <ul className='mt-2 grid gap-2 text-xs'>
            {packages.map((pkg) => (
              <li
                key={pkg.id}
                className='rounded-md border bg-muted/20 px-3 py-2'
              >
                <div className='flex items-center justify-between gap-2'>
                  <span className='truncate font-medium'>{pkg.name}</span>
                  <Badge
                    variant={pkg.model_filter_mode === 'whitelist' ? 'default' : 'secondary'}
                    className='text-[10px] uppercase font-mono'
                  >
                    {pkg.model_filter_mode}
                  </Badge>
                </div>
                <div className='mt-1 font-mono text-[13px] font-semibold'>
                  {pkg.type === 0
                    ? formatQuota(pkg.remaining_amount)
                    : `${pkg.remaining_amount.toLocaleString()} ${pkg.type === 1 ? t('requests') : t('tokens')}`}
                  <span className='text-muted-foreground ml-2 text-[10px] font-normal'>
                    /{' '}
                    {pkg.type === 0
                      ? formatQuota(pkg.initial_amount)
                      : pkg.initial_amount.toLocaleString()}
                  </span>
                </div>
                <div className='mt-1 text-muted-foreground line-clamp-2'>
                  <span className='font-semibold'>{t('Models')}:</span>{' '}
                  {pkg.models}
                </div>
              </li>
            ))}
          </ul>
        ) : (
          <p className='text-muted-foreground mt-1 text-xs'>{t('None')}</p>
        )}
      </div>
    </TitledCard>
  )
}
