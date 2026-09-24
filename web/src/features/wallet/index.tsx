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
import { useState, useEffect, useCallback, useMemo, useRef } from 'react'
import { ShieldCheck } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Badge } from '@/components/ui/badge'
import { useStatus } from '@/hooks/use-status'
import { useSystemConfig } from '@/hooks/use-system-config'
import { getSelf } from '@/lib/api'
import { formatQuota } from '@/lib/format'
import { getBillingPoolInfo, getUserBalancePackages, type UserBalancePackage } from './api'

import { AffiliateRewardsCard } from './components/affiliate-rewards-card'
import { BillingHistoryDialog } from './components/dialogs/billing-history-dialog'
import { BillingPriorityCard } from './components/billing-priority-card'
import { CreemConfirmDialog } from './components/dialogs/creem-confirm-dialog'
import { PaymentConfirmDialog } from './components/dialogs/payment-confirm-dialog'
import { TransferDialog } from './components/dialogs/transfer-dialog'
import { RechargeFormCard } from './components/recharge-form-card'
import { SubscriptionPlansCard } from './components/subscription-plans-card'
import { WalletStatsCard } from './components/wallet-stats-card'
import { DEFAULT_DISCOUNT_RATE, PAYMENT_TYPES } from './constants'
import {
  useTopupInfo,
  usePayment,
  useAffiliate,
  useRedemption,
  useCreemPayment,
  useWaffoPayment,
  useWaffoPancakePayment,
} from './hooks'
import {
  getDefaultPaymentType,
  getMinTopupAmount,
  dispatchSelectedPayment,
} from './lib'
import type {
  UserWalletData,
  PaymentMethod,
  PresetAmount,
  CreemProduct,
  WaffoPayMethod,
} from './types'

interface WalletProps {
  initialShowHistory?: boolean
}

export function Wallet(props: WalletProps) {
  const { t } = useTranslation()
  const [user, setUser] = useState<UserWalletData | null>(null)
  const [userLoading, setUserLoading] = useState(true)
  const [topupAmount, setTopupAmount] = useState(0)
  const [selectedPreset, setSelectedPreset] = useState<number | null>(null)
  const [selectedPaymentMethod, setSelectedPaymentMethod] =
    useState<PaymentMethod>()
  const [selectedWaffoMethodIndex, setSelectedWaffoMethodIndex] = useState<
    number | null
  >(null)
  const [paymentLoading, setPaymentLoading] = useState<string | null>(null)
  const [confirmDialogOpen, setConfirmDialogOpen] = useState(false)
  const [transferDialogOpen, setTransferDialogOpen] = useState(false)
  const [billingDialogOpen, setBillingDialogOpen] = useState(false)
  const [redemptionCode, setRedemptionCode] = useState('')
  const [creemDialogOpen, setCreemDialogOpen] = useState(false)
  const [selectedCreemProduct, setSelectedCreemProduct] =
    useState<CreemProduct | null>(null)
  const [showSubscriptionPanel, setShowSubscriptionPanel] = useState(true)
  const [enabledBillingTypes, setEnabledBillingTypes] = useState<{
    requests?: boolean
    tokens?: boolean
    subscription?: boolean
    wallet?: boolean
  }>({
    requests: true,
    tokens: true,
    subscription: true,
    wallet: true,
  })
  const [balancePackages, setBalancePackages] = useState<UserBalancePackage[]>([])

  const { status } = useStatus()
  const { currency } = useSystemConfig()
  const { topupInfo, presetAmounts, loading: topupLoading } = useTopupInfo()

  // Calculate effective exchange rate - when display type is USD, use rate of 1
  const effectiveUsdExchangeRate = useMemo(() => {
    return currency?.quotaDisplayType === 'USD'
      ? 1
      : currency?.usdExchangeRate || 1
  }, [currency?.quotaDisplayType, currency?.usdExchangeRate])
  const {
    amount: paymentAmount,
    calculating,
    processing,
    calculatePaymentAmount,
    processPayment,
  } = usePayment()
  const {
    affiliateLink,
    loading: affiliateLoading,
    transferQuota,
    transferring,
  } = useAffiliate()
  const { redeeming, redeemCode } = useRedemption()
  const { processing: creemProcessing, processCreemPayment } = useCreemPayment()
  const { processing: waffoProcessing, processWaffoPayment } = useWaffoPayment()
  const { processing: pancakeProcessing, processWaffoPancakePayment } =
    useWaffoPancakePayment()

  // Fetch and refresh user data and packages
  const fetchPoolsAndPackages = useCallback(async () => {
    try {
      const [poolRes, pkgRes] = await Promise.all([
        getBillingPoolInfo().catch(() => null),
        getUserBalancePackages().catch(() => null),
      ])
      if (poolRes?.success && poolRes.data?.enabled_billing_types) {
        setEnabledBillingTypes(poolRes.data.enabled_billing_types)
      }
      if (pkgRes?.success && Array.isArray(pkgRes.data)) {
        setBalancePackages(pkgRes.data)
      }
    } catch {
      // ignore
    }
  }, [])

  const fetchUser = useCallback(async () => {
    try {
      setUserLoading(true)
      const response = await getSelf()
      if (response.success && response.data) {
        setUser(response.data as UserWalletData)
      }
      await fetchPoolsAndPackages()
    } catch (error) {
      // eslint-disable-next-line no-console
      console.error('Failed to fetch user data:', error)
    } finally {
      setUserLoading(false)
    }
  }, [fetchPoolsAndPackages])

  useEffect(() => {
    fetchUser()
  }, [fetchUser])

  useEffect(() => {
    if (props.initialShowHistory) {
      setBillingDialogOpen(true)
      window.history.replaceState({}, '', window.location.pathname)
    }
  }, [props.initialShowHistory])

  // Initialize topup amount when topup info is loaded
  const topupAmountInitializedRef = useRef(false)
  useEffect(() => {
    if (topupInfo && !topupAmountInitializedRef.current) {
      topupAmountInitializedRef.current = true
      const minTopup = getMinTopupAmount(topupInfo)
      setTopupAmount(minTopup)

      // Calculate initial payment amount with default payment type
      const defaultPaymentType = getDefaultPaymentType(topupInfo)
      calculatePaymentAmount(minTopup, defaultPaymentType)
    }
  }, [topupInfo, calculatePaymentAmount])

  // Get current payment type (selected or default)
  const getCurrentPaymentType = useCallback(() => {
    return selectedPaymentMethod?.type || getDefaultPaymentType(topupInfo)
  }, [selectedPaymentMethod, topupInfo])

  // Handle preset selection
  const handleSelectPreset = (preset: PresetAmount) => {
    setTopupAmount(preset.value)
    setSelectedPreset(preset.value)
    calculatePaymentAmount(preset.value, getCurrentPaymentType())
  }

  // Handle topup amount change
  const handleTopupAmountChange = (amount: number) => {
    setTopupAmount(amount)
    setSelectedPreset(null)
    calculatePaymentAmount(amount, getCurrentPaymentType())
  }

  // Handle payment method selection
  const handlePaymentMethodSelect = async (method: PaymentMethod) => {
    setSelectedPaymentMethod(method)
    setSelectedWaffoMethodIndex(null)
    setPaymentLoading(method.type)

    try {
      // Validate minimum topup
      const minTopup = getMinTopupAmount(topupInfo)
      if (topupAmount < minTopup) {
        return
      }

      // Calculate payment amount and show confirmation dialog
      await calculatePaymentAmount(topupAmount, method.type)
      setConfirmDialogOpen(true)
    } finally {
      setPaymentLoading(null)
    }
  }

  // Handle payment confirmation
  const handlePaymentConfirm = async () => {
    if (!selectedPaymentMethod) return

    const success = await dispatchSelectedPayment(
      selectedPaymentMethod,
      topupAmount,
      selectedWaffoMethodIndex,
      {
        regular: processPayment,
        waffo: processWaffoPayment,
        waffoPancake: processWaffoPancakePayment,
      }
    )

    if (success) {
      setConfirmDialogOpen(false)
      await fetchUser()
    }
  }

  // Handle redemption
  const handleRedeem = async () => {
    if (!redemptionCode) return

    const success = await redeemCode(redemptionCode)
    if (success) {
      setRedemptionCode('')
      await fetchUser()
    }
  }

  // Handle transfer
  const handleTransfer = async (amount: number) => {
    const success = await transferQuota(amount)
    if (success) {
      await fetchUser()
    }
    return success
  }

  // Handle Creem product selection
  const handleCreemProductSelect = (product: CreemProduct) => {
    setSelectedCreemProduct(product)
    setCreemDialogOpen(true)
  }

  // Handle Creem payment confirmation
  const handleCreemConfirm = async () => {
    if (!selectedCreemProduct) return

    const success = await processCreemPayment(selectedCreemProduct.productId)
    if (success) {
      setCreemDialogOpen(false)
      setSelectedCreemProduct(null)
      await fetchUser()
    }
  }

  const handleWaffoMethodSelect = async (
    method: WaffoPayMethod,
    index: number
  ) => {
    const loadingKey = `waffo-${index}`
    setSelectedPaymentMethod({
      name: method.name,
      type: PAYMENT_TYPES.WAFFO,
      icon: method.icon,
    })
    setSelectedWaffoMethodIndex(index)
    setPaymentLoading(loadingKey)

    try {
      await calculatePaymentAmount(topupAmount, PAYMENT_TYPES.WAFFO)
      setConfirmDialogOpen(true)
    } finally {
      setPaymentLoading(null)
    }
  }

  // Get discount rate for current topup amount
  const getDiscountRate = useCallback(() => {
    return topupInfo?.discount?.[topupAmount] || DEFAULT_DISCOUNT_RATE
  }, [topupInfo, topupAmount])

  const handleSubscriptionAvailabilityChange = useCallback(
    (available: boolean) => {
      setShowSubscriptionPanel(available)
    },
    []
  )

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>{t('Wallet')}</SectionPageLayout.Title>
        <SectionPageLayout.Content>
          <div className='mx-auto flex w-full max-w-7xl flex-col gap-4 sm:gap-5'>
            <WalletStatsCard
              user={user}
              loading={userLoading}
              enabledBillingTypes={enabledBillingTypes}
            />

            {balancePackages.length > 0 && (
              <div className='rounded-lg border bg-card p-4 shadow-xs'>
                <div className='flex items-center gap-2 mb-3'>
                  <ShieldCheck className='h-4 w-4 text-primary' />
                  <h3 className='text-sm font-semibold'>{t('Model-Restricted Balances')}</h3>
                  <Badge variant='outline' className='text-xs'>
                    {balancePackages.length} {t('Active')}
                  </Badge>
                </div>
                <div className='grid gap-3 sm:grid-cols-2 lg:grid-cols-3'>
                  {balancePackages.map((pkg) => (
                    <div key={pkg.id} className='rounded-md border p-3 bg-muted/20 space-y-2'>
                      <div className='flex items-center justify-between'>
                        <span className='font-medium text-xs truncate'>{pkg.name}</span>
                        <Badge
                          variant={pkg.model_filter_mode === 'whitelist' ? 'default' : 'secondary'}
                          className='text-[10px] uppercase font-mono'
                        >
                          {pkg.model_filter_mode}
                        </Badge>
                      </div>
                      <div className='text-xs font-mono font-bold'>
                        {pkg.type === 0
                          ? formatQuota(pkg.remaining_amount)
                          : pkg.type === 1
                          ? `${pkg.remaining_amount.toLocaleString()} ${t('requests')}`
                          : `${pkg.remaining_amount.toLocaleString()} ${t('tokens')}`}
                        <span className='text-[10px] text-muted-foreground font-normal ml-1'>
                          / {pkg.type === 0 ? formatQuota(pkg.initial_amount) : pkg.initial_amount.toLocaleString()}
                        </span>
                      </div>
                      <div className='text-[11px] text-muted-foreground line-clamp-2'>
                        <span className='font-semibold'>{t('Models')}: </span>
                        {pkg.models}
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}

            {(enabledBillingTypes.wallet !== false ||
              (enabledBillingTypes.subscription !== false && showSubscriptionPanel)) && (
              <div
                className={
                  showSubscriptionPanel &&
                  enabledBillingTypes.subscription !== false &&
                  enabledBillingTypes.wallet !== false
                    ? 'grid gap-4 xl:grid-cols-[minmax(0,1.05fr)_minmax(360px,0.95fr)] xl:items-start'
                    : 'grid gap-4'
                }
              >
                {enabledBillingTypes.wallet !== false && (
                  <div id='wallet-add-funds' className='scroll-mt-4'>
                    <RechargeFormCard
                      topupInfo={topupInfo}
                      presetAmounts={presetAmounts}
                      selectedPreset={selectedPreset}
                      onSelectPreset={handleSelectPreset}
                      topupAmount={topupAmount}
                      onTopupAmountChange={handleTopupAmountChange}
                      paymentAmount={paymentAmount}
                      calculating={calculating}
                      onPaymentMethodSelect={handlePaymentMethodSelect}
                      paymentLoading={paymentLoading}
                      redemptionCode={redemptionCode}
                      onRedemptionCodeChange={setRedemptionCode}
                      onRedeem={handleRedeem}
                      redeeming={redeeming}
                      topupLink={topupInfo?.topup_link}
                      loading={topupLoading}
                      priceRatio={(status?.price as number) || 1}
                      usdExchangeRate={effectiveUsdExchangeRate}
                      onOpenBilling={() => setBillingDialogOpen(true)}
                      creemProducts={topupInfo?.creem_products}
                      enableCreemTopup={topupInfo?.enable_creem_topup}
                      onCreemProductSelect={handleCreemProductSelect}
                      enableWaffoTopup={topupInfo?.enable_waffo_topup}
                      waffoPayMethods={topupInfo?.waffo_pay_methods}
                      waffoMinTopup={topupInfo?.waffo_min_topup}
                      onWaffoMethodSelect={handleWaffoMethodSelect}
                      enableWaffoPancakeTopup={
                        topupInfo?.enable_waffo_pancake_topup
                      }
                    />
                  </div>
                )}

                {enabledBillingTypes.subscription !== false && (
                  <SubscriptionPlansCard
                    topupInfo={topupInfo}
                    onAvailabilityChange={handleSubscriptionAvailabilityChange}
                    userQuota={user?.quota}
                    onPurchaseSuccess={fetchUser}
                  />
                )}
              </div>
            )}

            <BillingPriorityCard user={user} onPriorityUpdated={fetchUser} />

            <AffiliateRewardsCard
              user={user}
              affiliateLink={affiliateLink}
              onTransfer={() => setTransferDialogOpen(true)}
              complianceConfirmed={
                topupInfo?.payment_compliance_confirmed !== false
              }
              loading={affiliateLoading}
            />
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <PaymentConfirmDialog
        open={confirmDialogOpen}
        onOpenChange={setConfirmDialogOpen}
        onConfirm={handlePaymentConfirm}
        topupAmount={topupAmount}
        paymentAmount={paymentAmount}
        paymentMethod={selectedPaymentMethod}
        calculating={calculating}
        processing={processing || waffoProcessing || pancakeProcessing}
        discountRate={getDiscountRate()}
        usdExchangeRate={effectiveUsdExchangeRate}
      />

      <TransferDialog
        open={transferDialogOpen}
        onOpenChange={setTransferDialogOpen}
        onConfirm={handleTransfer}
        availableQuota={user?.aff_quota ?? 0}
        transferring={transferring}
      />

      <BillingHistoryDialog
        open={billingDialogOpen}
        onOpenChange={setBillingDialogOpen}
      />

      <CreemConfirmDialog
        open={creemDialogOpen}
        onOpenChange={setCreemDialogOpen}
        onConfirm={handleCreemConfirm}
        product={selectedCreemProduct}
        processing={creemProcessing}
      />
    </>
  )
}
