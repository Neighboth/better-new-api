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
import {
  ArrowDown,
  ArrowRight,
  ArrowUp,
  Check,
  Coins,
  Crown,
  Info,
  Layers,
  Loader2,
  MessageSquareCode,
  RotateCcw,
  WalletCards,
} from 'lucide-react'
import { useState, useEffect, useMemo, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Switch } from '@/components/ui/switch'
import { TitledCard } from '@/components/ui/titled-card'
import { formatQuota } from '@/lib/format'
import { cn } from '@/lib/utils'

import { getBillingPoolInfo, updateBillingPriority } from '../api'
import type { UserWalletData } from '../types'

interface BillingPriorityCardProps {
  user: UserWalletData | null
  onPriorityUpdated?: () => void
}

type BillingType = 'requests' | 'subscription' | 'tokens' | 'wallet'

interface PoolMeta {
  type: BillingType
  titleKey: string
  descriptionKey: string
  shortTitleKey: string
  icon: typeof WalletCards
  toneColor: string
}

const ALL_POOLS: PoolMeta[] = [
  {
    type: 'requests',
    titleKey: 'Requests Pool',
    descriptionKey: 'Requests Pool Description',
    shortTitleKey: 'Requests',
    icon: MessageSquareCode,
    toneColor: 'text-sky-500 bg-sky-500/10 border-sky-500/20',
  },
  {
    type: 'subscription',
    titleKey: 'Subscription Pool',
    descriptionKey: 'Subscription Pool Description',
    shortTitleKey: 'Subscription',
    icon: Crown,
    toneColor: 'text-amber-500 bg-amber-500/10 border-amber-500/20',
  },
  {
    type: 'tokens',
    titleKey: 'Tokens Pool',
    descriptionKey: 'Tokens Pool Description',
    shortTitleKey: 'Tokens',
    icon: Coins,
    toneColor: 'text-emerald-500 bg-emerald-500/10 border-emerald-500/20',
  },
  {
    type: 'wallet',
    titleKey: 'Standard Wallet Pool',
    descriptionKey: 'Standard Wallet Pool Description',
    shortTitleKey: 'Wallet',
    icon: WalletCards,
    toneColor: 'text-purple-500 bg-purple-500/10 border-purple-500/20',
  },
]

export function BillingPriorityCard({
  user,
  onPriorityUpdated,
}: BillingPriorityCardProps) {
  const { t } = useTranslation()
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)

  // Ordered list of pools
  const [poolOrder, setPoolOrder] = useState<BillingType[]>([
    'requests',
    'subscription',
    'tokens',
    'wallet',
  ])

  // Set of enabled pool types
  const [activePools, setActivePools] = useState<Set<BillingType>>(
    new Set(['requests', 'subscription', 'tokens', 'wallet'])
  )

  // System-wide enabled pool types set by administrator
  const [systemEnabled, setSystemEnabled] = useState<Record<BillingType, boolean>>({
    requests: true,
    subscription: true,
    tokens: true,
    wallet: true,
  })

  // Load priority and system options from API
  const loadPriorityInfo = useCallback(async () => {
    try {
      setLoading(true)
      const res = await getBillingPoolInfo()
      if (res.success && res.data) {
        if (res.data.enabled_billing_types) {
          setSystemEnabled({
            requests: res.data.enabled_billing_types.requests !== false,
            subscription: res.data.enabled_billing_types.subscription !== false,
            tokens: res.data.enabled_billing_types.tokens !== false,
            wallet: res.data.enabled_billing_types.wallet !== false,
          })
        }

        const serverPriority = res.data.billing_priority || user?.billing_priority
        if (serverPriority && Array.isArray(serverPriority) && serverPriority.length > 0) {
          const validTypes = serverPriority.filter((p): p is BillingType =>
            ALL_POOLS.some((meta) => meta.type === p)
          )
          const activeSet = new Set<BillingType>(validTypes)
          setActivePools(activeSet)

          // Add any missing types to the end of the order
          const fullOrder = [...validTypes]
          ALL_POOLS.forEach((meta) => {
            if (!fullOrder.includes(meta.type)) {
              fullOrder.push(meta.type)
            }
          })
          setPoolOrder(fullOrder)
        }
      }
    } catch {
      // Ignore initial load error
    } finally {
      setLoading(false)
    }
  }, [user?.billing_priority])

  useEffect(() => {
    loadPriorityInfo()
  }, [loadPriorityInfo])

  // Move a pool item up in the order
  const handleMoveUp = (type: BillingType) => {
    setPoolOrder((prev) => {
      const visible = prev.filter((p) => systemEnabled[p] !== false)
      const vIdx = visible.indexOf(type)
      if (vIdx <= 0) return prev
      const prevType = visible[vIdx - 1]
      const next = [...prev]
      const idxA = next.indexOf(type)
      const idxB = next.indexOf(prevType)
      next[idxA] = prevType
      next[idxB] = type
      return next
    })
  }

  // Move a pool item down in the order
  const handleMoveDown = (type: BillingType) => {
    setPoolOrder((prev) => {
      const visible = prev.filter((p) => systemEnabled[p] !== false)
      const vIdx = visible.indexOf(type)
      if (vIdx >= visible.length - 1) return prev
      const nextType = visible[vIdx + 1]
      const next = [...prev]
      const idxA = next.indexOf(type)
      const idxB = next.indexOf(nextType)
      next[idxA] = nextType
      next[idxB] = type
      return next
    })
  }

  // Toggle activation of a pool
  const handleTogglePool = (type: BillingType) => {
    setActivePools((prev) => {
      const next = new Set(prev)
      if (next.has(type)) {
        if (next.size <= 1) {
          toast.warning(t('At least one billing pool must remain active'))
          return prev
        }
        next.delete(type)
      } else {
        next.add(type)
      }
      return next
    })
  }

  // Active cascade sequence
  const activeChain = useMemo(() => {
    return poolOrder.filter(
      (type) => activePools.has(type) && systemEnabled[type]
    )
  }, [poolOrder, activePools, systemEnabled])

  // Save changes to backend
  const handleSave = async () => {
    if (activeChain.length === 0) {
      toast.error(t('At least one billing pool must be active'))
      return
    }

    try {
      setSaving(true)
      const res = await updateBillingPriority(activeChain)
      if (res.success) {
        toast.success(t('Billing priority updated successfully'))
        onPriorityUpdated?.()
      } else {
        toast.error(res.message || t('Failed to update billing priority'))
      }
    } catch {
      toast.error(t('Failed to update billing priority'))
    } finally {
      setSaving(false)
    }
  }

  // Reset to default order
  const handleResetDefault = () => {
    setPoolOrder(['requests', 'subscription', 'tokens', 'wallet'])
    setActivePools(new Set(['requests', 'subscription', 'tokens', 'wallet']))
    toast.info(t('Reset to default order. Click Save to apply.'))
  }

  const getBalanceInfo = (type: BillingType) => {
    switch (type) {
      case 'requests':
        return `${(user?.requests_balance ?? 0).toLocaleString()} ${t('requests')}`
      case 'subscription':
        return t('Active Plan Quota')
      case 'tokens':
        return `${(user?.tokens_balance ?? 0).toLocaleString()} ${t('tokens')}`
      case 'wallet':
        return formatQuota(user?.quota ?? 0)
    }
  }

  const visiblePools = useMemo(() => {
    return poolOrder.filter((type) => systemEnabled[type] !== false)
  }, [poolOrder, systemEnabled])

  if (visiblePools.length <= 1) {
    return null
  }

  return (
    <TitledCard
      title={t('Spending Priority & Budget Pools')}
      description={t(
        'Configure block-based spending priority across your balance pools. Requests cascade through active pools in order.'
      )}
      icon={<Layers className='h-4 w-4' />}
    >
      <div className='space-y-5'>
        {/* Cascade Pipeline Preview */}
        <div className='bg-muted/40 rounded-lg border p-3 sm:p-4'>
          <div className='text-muted-foreground mb-2 flex items-center justify-between text-xs font-semibold uppercase tracking-wider'>
            <span>{t('Active Cascade Pipeline')}</span>
            <span className='text-[11px] font-normal lowercase'>
              {activeChain.length} {t('pools active')}
            </span>
          </div>

          <div className='flex flex-wrap items-center gap-2 pt-1'>
            {activeChain.map((type, idx) => {
              const meta = ALL_POOLS.find((p) => p.type === type)!
              const Icon = meta.icon
              return (
                <div key={type} className='flex items-center gap-2'>
                  <div
                    className={cn(
                      'flex items-center gap-1.5 rounded-md border px-2.5 py-1.5 text-xs font-medium shadow-xs',
                      meta.toneColor
                    )}
                  >
                    <span className='font-mono font-bold'>{idx + 1}.</span>
                    <Icon className='h-3.5 w-3.5' />
                    <span>{t(meta.shortTitleKey)}</span>
                    <Badge variant='outline' className='ml-1 text-[10px] px-1 py-0 bg-background/50'>
                      {getBalanceInfo(type)}
                    </Badge>
                  </div>
                  {idx < activeChain.length - 1 && (
                    <ArrowRight className='text-muted-foreground/60 h-3.5 w-3.5 shrink-0' />
                  )}
                </div>
              )
            })}
            {activeChain.length === 0 && (
              <span className='text-destructive text-xs font-medium'>
                {t('No pools active. Requests will be blocked!')}
              </span>
            )}
          </div>
        </div>

        {/* Priority Blocks List */}
        <div className='space-y-2.5'>
          {visiblePools.map((type, index) => {
            const meta = ALL_POOLS.find((p) => p.type === type)!
            const Icon = meta.icon
            const isActive = activePools.has(type)
            const activeIndex = activeChain.indexOf(type)

            return (
              <Card
                key={type}
                className={cn(
                  'transition-all duration-150',
                  isActive
                    ? 'border-border shadow-xs'
                    : 'border-dashed opacity-60 bg-muted/20'
                )}
              >
                <CardContent className='flex flex-col gap-3 p-3.5 sm:flex-row sm:items-center sm:justify-between'>
                  <div className='flex items-center gap-3 min-w-0'>
                    {/* Index badge */}
                    <div
                      className={cn(
                        'flex h-7 w-7 shrink-0 items-center justify-center rounded-md text-xs font-bold font-mono',
                        isActive
                          ? 'bg-primary text-primary-foreground'
                          : 'bg-muted text-muted-foreground'
                      )}
                    >
                      {isActive ? `#${activeIndex + 1}` : '—'}
                    </div>

                    {/* Icon & Details */}
                    <div className='min-w-0 flex-1'>
                      <div className='flex items-center gap-2'>
                        <Icon className='text-muted-foreground h-4 w-4 shrink-0' />
                        <span className='text-sm font-semibold truncate'>
                          {t(meta.titleKey)}
                        </span>
                        {!isActive && (
                          <Badge variant='outline' className='text-[10px] text-muted-foreground'>
                            {t('Inactive')}
                          </Badge>
                        )}
                      </div>
                      <p className='text-muted-foreground mt-0.5 text-xs line-clamp-1'>
                        {t(meta.descriptionKey)}
                      </p>
                    </div>
                  </div>

                  {/* Right side: Balance, Reorder Buttons, Toggle */}
                  <div className='flex items-center justify-between gap-3 sm:justify-end shrink-0 pl-10 sm:pl-0'>
                    <div className='text-left sm:text-right'>
                      <div className='text-[11px] text-muted-foreground uppercase font-medium'>
                        {t('Balance')}
                      </div>
                      <div className='text-xs font-mono font-semibold'>
                        {getBalanceInfo(type)}
                      </div>
                    </div>

                    <div className='flex items-center gap-1 border-l pl-3'>
                      <Button
                        variant='ghost'
                        size='icon'
                        className='h-7 w-7'
                        disabled={index === 0 || !isActive}
                        onClick={() => handleMoveUp(type)}
                        title={t('Move Up')}
                      >
                        <ArrowUp className='h-3.5 w-3.5' />
                      </Button>
                      <Button
                        variant='ghost'
                        size='icon'
                        className='h-7 w-7'
                        disabled={index === visiblePools.length - 1 || !isActive}
                        onClick={() => handleMoveDown(type)}
                        title={t('Move Down')}
                      >
                        <ArrowDown className='h-3.5 w-3.5' />
                      </Button>

                      <div className='ml-2 flex items-center gap-1.5'>
                        <Switch
                          checked={isActive}
                          onCheckedChange={() => handleTogglePool(type)}
                        />
                      </div>
                    </div>
                  </div>
                </CardContent>
              </Card>
            )
          })}
        </div>

        {/* Informational guide */}
        <Alert className='bg-primary/5 border-primary/10 py-2.5'>
          <Info className='h-4 w-4 text-primary shrink-0' />
          <AlertDescription className='text-xs text-muted-foreground leading-relaxed'>
            {t(
              'Requests first draw from pool #1. If empty or insufficient, requests fall back to #2, and so forth. Pools not included in your active cascade will be preserved and will not be charged, even if balance is available.'
            )}
          </AlertDescription>
        </Alert>

        {/* Action Buttons */}
        <div className='flex items-center justify-between pt-1'>
          <Button
            variant='outline'
            size='sm'
            onClick={handleResetDefault}
            disabled={saving || loading}
          >
            <RotateCcw className='mr-1.5 h-3.5 w-3.5' />
            {t('Reset Default')}
          </Button>

          <Button
            size='sm'
            onClick={handleSave}
            disabled={saving || loading || activeChain.length === 0}
          >
            {saving ? (
              <Loader2 className='mr-1.5 h-3.5 w-3.5 animate-spin' />
            ) : (
              <Check className='mr-1.5 h-3.5 w-3.5' />
            )}
            {t('Save Priority')}
          </Button>
        </div>
      </div>
    </TitledCard>
  )
}
