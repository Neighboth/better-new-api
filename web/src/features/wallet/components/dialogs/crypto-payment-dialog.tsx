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
import { Check, Clock, Coins, Copy, ExternalLink, History, Info, Loader2, Send, XCircle } from 'lucide-react'
import { useState, useEffect, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { formatQuota } from '@/lib/format'

import {
  createCryptoDeposit,
  submitCryptoTxHash,
  getUserCryptoDeposits,
  type CryptoDepositItem,
} from '../../api'
import type { TopupInfo } from '../../types'

interface CryptoPaymentDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  topupInfo: TopupInfo | null
  topupAmount: number
  onSuccess?: () => void
}

export function CryptoPaymentDialog({
  open,
  onOpenChange,
  topupInfo,
  topupAmount,
  onSuccess,
}: CryptoPaymentDialogProps) {
  const { t } = useTranslation()
  const wallets = topupInfo?.crypto_wallets || []

  const [selectedWalletId, setSelectedWalletId] = useState<string>('')
  const [txHash, setTxHash] = useState('')
  const [senderAddress, setSenderAddress] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [copiedAddress, setCopiedAddress] = useState(false)
  const [copiedRef, setCopiedRef] = useState(false)
  const [submittedDeposit, setSubmittedDeposit] = useState<{
    id: number
    reference_code: string
    network: string
    amount: number
  } | null>(null)

  const [activeTab, setActiveTab] = useState<'pay' | 'history'>('pay')
  const [historyList, setHistoryList] = useState<CryptoDepositItem[]>([])
  const [historyLoading, setHistoryLoading] = useState(false)

  useEffect(() => {
    if (wallets.length > 0 && !selectedWalletId) {
      setSelectedWalletId(wallets[0].id)
    }
  }, [wallets, selectedWalletId])

  const selectedWallet = wallets.find((w) => w.id === selectedWalletId) || wallets[0]

  const fetchHistory = useCallback(async () => {
    try {
      setHistoryLoading(true)
      const res = await getUserCryptoDeposits(0, 20)
      if (res?.success && res.data?.items) {
        setHistoryList(res.data.items)
      }
    } catch {
      // ignore
    } finally {
      setHistoryLoading(false)
    }
  }, [])

  useEffect(() => {
    if (open && activeTab === 'history') {
      fetchHistory()
    }
  }, [open, activeTab, fetchHistory])

  const handleCopy = (text: string, type: 'address' | 'ref') => {
    navigator.clipboard.writeText(text)
    if (type === 'address') {
      setCopiedAddress(true)
      setTimeout(() => setCopiedAddress(false), 2000)
    } else {
      setCopiedRef(true)
      setTimeout(() => setCopiedRef(false), 2000)
    }
    toast.success(t('Panoya kopyalandı'))
  }

  const handleSubmit = async () => {
    if (!selectedWallet) {
      toast.error(t('Lütfen bir kripto ağı seçin'))
      return
    }
    const cleanTx = txHash.trim()
    if (!cleanTx) {
      toast.error(t('Lütfen transferinizin İşlem Özetini (TxID / Hash) girin'))
      return
    }

    try {
      setSubmitting(true)

      // Step 1: Create deposit record
      const createRes = await createCryptoDeposit({
        amount: topupAmount,
        network: selectedWallet.network,
        wallet_id: selectedWallet.id,
      })

      if (!createRes?.success || !createRes.data?.id) {
        toast.error(createRes?.message || t('Ödeme kaydı oluşturulamadı'))
        return
      }

      const depositId = createRes.data.id
      const refCode = createRes.data.reference_code

      // Step 2: Submit TxHash
      const submitRes = await submitCryptoTxHash({
        id: depositId,
        tx_hash: cleanTx,
        sender_address: senderAddress.trim(),
      })

      if (!submitRes?.success) {
        toast.error(submitRes?.message || t('TxID kaydedilemedi'))
        return
      }

      setSubmittedDeposit({
        id: depositId,
        reference_code: refCode,
        network: selectedWallet.network,
        amount: topupAmount,
      })
      setTxHash('')
      setSenderAddress('')
      toast.success(t('Kripto ödeme bildiriminiz başarıyla iletildi!'))
      if (onSuccess) onSuccess()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : t('İşlem sırasında bir hata oluştu')
      toast.error(msg)
    } finally {
      setSubmitting(false)
    }
  }

  const getExplorerUrl = (network: string, hash: string) => {
    const n = network.toLowerCase()
    const cleanHash = hash.trim()
    if (n.includes('trc') || n.includes('tron')) {
      return `https://tronscan.org/#/transaction/${cleanHash}`
    }
    if (n.includes('bep') || n.includes('bsc') || n.includes('binance')) {
      return `https://bscscan.com/tx/${cleanHash}`
    }
    if (n.includes('erc') || n.includes('eth') || n.includes('ethereum')) {
      return `https://etherscan.io/tx/${cleanHash}`
    }
    if (n.includes('sol')) {
      return `https://solscan.io/tx/${cleanHash}`
    }
    if (n.includes('btc') || n.includes('bitcoin')) {
      return `https://mempool.space/tx/${cleanHash}`
    }
    return ''
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-w-xl max-h-[90vh] overflow-y-auto sm:max-w-2xl'>
        <DialogHeader>
          <div className='flex items-center gap-2'>
            <div className='p-2 rounded-lg bg-amber-500/10 text-amber-500'>
              <Coins className='h-5 w-5' />
            </div>
            <div>
              <DialogTitle>{t('Kripto Para ile Bakiye Yükleme')}</DialogTitle>
              <DialogDescription>
                {t('Kripto ağını seçip adrese transfer yapın ve TxID bilgisini iletin.')}
              </DialogDescription>
            </div>
          </div>
        </DialogHeader>

        <Tabs
          value={activeTab}
          onValueChange={(val) => setActiveTab(val as 'pay' | 'history')}
          className='w-full mt-2'
        >
          <TabsList className='grid grid-cols-2 w-full'>
            <TabsTrigger value='pay' className='gap-2'>
              <Send className='h-4 w-4' />
              {t('Ödeme Bildirimi')}
            </TabsTrigger>
            <TabsTrigger value='history' className='gap-2'>
              <History className='h-4 w-4' />
              {t('Ödeme Geçmişim')}
            </TabsTrigger>
          </TabsList>

          <TabsContent value='pay' className='space-y-4 pt-3'>
            {submittedDeposit ? (
              <div className='rounded-lg border bg-card p-6 text-center space-y-4 animate-in fade-in'>
                <div className='mx-auto w-12 h-12 rounded-full bg-emerald-500/10 text-emerald-500 flex items-center justify-center'>
                  <Check className='h-6 w-6' />
                </div>
                <div>
                  <h3 className='text-base font-semibold text-foreground'>
                    {t('Ödeme Bildiriminiz Alındı!')}
                  </h3>
                  <p className='text-xs text-muted-foreground mt-1'>
                    {t('Transferiniz ağ üzerinde ve yönetici tarafından incelendikten sonra bakiyeniz hesabınıza tanımlanacaktır.')}
                  </p>
                </div>

                <div className='bg-muted/50 rounded-lg p-4 max-w-sm mx-auto space-y-2 text-left'>
                  <div className='flex justify-between items-center text-xs'>
                    <span className='text-muted-foreground'>{t('Referans Kodu')}:</span>
                    <div className='flex items-center gap-1.5'>
                      <span className='font-mono font-bold text-foreground text-sm'>
                        {submittedDeposit.reference_code}
                      </span>
                      <Button
                        type='button'
                        variant='ghost'
                        size='icon'
                        className='h-6 w-6'
                        onClick={() => handleCopy(submittedDeposit.reference_code, 'ref')}
                      >
                        {copiedRef ? <Check className='h-3 w-3 text-emerald-500' /> : <Copy className='h-3 w-3' />}
                      </Button>
                    </div>
                  </div>
                  <div className='flex justify-between text-xs'>
                    <span className='text-muted-foreground'>{t('Ağ')}:</span>
                    <span className='font-medium text-foreground'>{submittedDeposit.network}</span>
                  </div>
                  <div className='flex justify-between text-xs'>
                    <span className='text-muted-foreground'>{t('Tutar')}:</span>
                    <span className='font-semibold text-emerald-600 dark:text-emerald-400'>
                      ${submittedDeposit.amount}
                    </span>
                  </div>
                </div>

                <div className='flex justify-center gap-3 pt-2'>
                  <Button
                    type='button'
                    variant='outline'
                    onClick={() => {
                      setSubmittedDeposit(null)
                      setActiveTab('history')
                    }}
                  >
                    {t('Geçmişi Görüntüle')}
                  </Button>
                  <Button
                    type='button'
                    onClick={() => setSubmittedDeposit(null)}
                  >
                    {t('Yeni Bildirim Yap')}
                  </Button>
                </div>
              </div>
            ) : wallets.length === 0 ? (
              <Alert>
                <Info className='h-4 w-4' />
                <AlertDescription>
                  {t('Sistemde tanımlı kripto cüzdanı bulunamadı. Lütfen yönetici ile iletişime geçin.')}
                </AlertDescription>
              </Alert>
            ) : (
              <>
                {/* Network Selection */}
                <div className='space-y-2'>
                  <Label className='text-xs font-medium'>{t('1. Kripto Ağı Seçin')}</Label>
                  <div className='grid grid-cols-2 sm:grid-cols-3 gap-2'>
                    {wallets.map((w) => (
                      <Button
                        key={w.id}
                        type='button'
                        variant={selectedWalletId === w.id ? 'default' : 'outline'}
                        className='h-auto py-2 px-3 justify-start text-left flex flex-col items-start gap-0.5'
                        onClick={() => setSelectedWalletId(w.id)}
                      >
                        <span className='text-xs font-semibold truncate max-w-full'>{w.network}</span>
                        {w.min_deposit ? (
                          <span className='text-[10px] opacity-70'>Min: ${w.min_deposit}</span>
                        ) : null}
                      </Button>
                    ))}
                  </div>
                </div>

                {/* Target Address & QR Box */}
                {selectedWallet && (
                  <div className='rounded-lg border bg-muted/20 p-4 space-y-3'>
                    <div className='flex items-center justify-between'>
                      <span className='text-xs font-semibold uppercase tracking-wider text-muted-foreground'>
                        {t('Transfer Adresi')} ({selectedWallet.network})
                      </span>
                      <Badge variant='outline' className='text-[11px] font-mono'>
                        {t('Yüklenecek')}: ${topupAmount}
                      </Badge>
                    </div>

                    <div className='flex items-center gap-2 bg-background rounded-md border p-2'>
                      <code className='text-xs font-mono break-all flex-1 select-all'>
                        {selectedWallet.address}
                      </code>
                      <Button
                        type='button'
                        variant='secondary'
                        size='sm'
                        className='h-7 gap-1 px-2.5 text-xs'
                        onClick={() => handleCopy(selectedWallet.address, 'address')}
                      >
                        {copiedAddress ? (
                          <>
                            <Check className='h-3.5 w-3.5 text-emerald-500' />
                            {t('Kopyalandı')}
                          </>
                        ) : (
                          <>
                            <Copy className='h-3.5 w-3.5' />
                            {t('Kopyala')}
                          </>
                        )}
                      </Button>
                    </div>

                    {selectedWallet.qr_code && (
                      <div className='flex justify-center pt-1'>
                        <img
                          src={selectedWallet.qr_code}
                          alt='QR Code'
                          className='h-28 w-28 object-contain rounded border bg-white p-1'
                        />
                      </div>
                    )}

                    {topupInfo?.crypto_instructions && (
                      <p className='text-xs text-muted-foreground whitespace-pre-wrap leading-relaxed'>
                        {topupInfo.crypto_instructions}
                      </p>
                    )}
                  </div>
                )}

                {/* TxID & Sender Address Inputs */}
                <div className='space-y-3 pt-1'>
                  <div className='space-y-1.5'>
                    <div className='flex items-center justify-between'>
                      <Label htmlFor='crypto-tx-hash' className='text-xs font-medium'>
                        {t('2. İşlem Özeti (TxID / Transaction Hash)')}{' '}
                        <span className='text-destructive'>*</span>
                      </Label>
                      <span className='text-[11px] text-muted-foreground'>
                        {t('Gönderimi doğrulayabilmemiz için zorunludur')}
                      </span>
                    </div>
                    <Input
                      id='crypto-tx-hash'
                      placeholder={t('Örnek: 0x8a92... veya b41c...')}
                      value={txHash}
                      onChange={(e) => setTxHash(e.target.value)}
                      className='font-mono text-xs'
                    />
                  </div>

                  <div className='space-y-1.5'>
                    <Label htmlFor='crypto-sender-address' className='text-xs font-medium'>
                      {t('Gönderen Cüzdan Adresi (İsteğe bağlı)')}
                    </Label>
                    <Input
                      id='crypto-sender-address'
                      placeholder={t('Transferi gönderdiğiniz cüzdan adresi')}
                      value={senderAddress}
                      onChange={(e) => setSenderAddress(e.target.value)}
                      className='font-mono text-xs'
                    />
                  </div>
                </div>

                <div className='flex justify-end gap-2 pt-3 border-t'>
                  <Button
                    type='button'
                    variant='outline'
                    onClick={() => onOpenChange(false)}
                    disabled={submitting}
                  >
                    {t('Kapat')}
                  </Button>
                  <Button
                    type='button'
                    onClick={handleSubmit}
                    disabled={submitting || !txHash.trim()}
                    className='gap-2 bg-amber-600 hover:bg-amber-700 text-white'
                  >
                    {submitting ? (
                      <>
                        <Loader2 className='h-4 w-4 animate-spin' />
                        {t('İletiliyor...')}
                      </>
                    ) : (
                      <>
                        <Send className='h-4 w-4' />
                        {t('Ödemeyi Bildir')}
                      </>
                    )}
                  </Button>
                </div>
              </>
            )}
          </TabsContent>

          <TabsContent value='history' className='space-y-3 pt-3'>
            {historyLoading ? (
              <div className='py-8 text-center text-xs text-muted-foreground flex items-center justify-center gap-2'>
                <Loader2 className='h-4 w-4 animate-spin' />
                {t('Kripto ödemeleri yükleniyor...')}
              </div>
            ) : historyList.length === 0 ? (
              <div className='rounded-lg border border-dashed p-8 text-center text-xs text-muted-foreground'>
                {t('Henüz herhangi bir kripto ödeme bildiriminiz bulunmuyor.')}
              </div>
            ) : (
              <div className='space-y-2.5 max-h-[50vh] overflow-y-auto pr-1'>
                {historyList.map((item) => {
                  const explorerUrl = getExplorerUrl(item.network, item.tx_hash)
                  return (
                    <div
                      key={item.id}
                      className='rounded-lg border bg-card p-3.5 space-y-2 text-xs'
                    >
                      <div className='flex items-center justify-between'>
                        <div className='flex items-center gap-2'>
                          <Badge variant='outline' className='font-mono text-[11px] font-bold'>
                            {item.reference_code}
                          </Badge>
                          <span className='font-semibold text-foreground'>
                            {item.network}
                          </span>
                        </div>
                        <div className='flex items-center gap-2'>
                          <span className='font-bold text-emerald-600 dark:text-emerald-400'>
                            ${item.amount}
                          </span>
                          {item.status === 'pending' && (
                            <Badge variant='secondary' className='gap-1 text-amber-600 dark:text-amber-400 bg-amber-500/10'>
                              <Clock className='h-3 w-3' />
                              {t('Beklemede')}
                            </Badge>
                          )}
                          {item.status === 'approved' && (
                            <Badge variant='secondary' className='gap-1 text-emerald-600 dark:text-emerald-400 bg-emerald-500/10'>
                              <Check className='h-3 w-3' />
                              {t('Onaylandı')}
                            </Badge>
                          )}
                          {item.status === 'rejected' && (
                            <Badge variant='destructive' className='gap-1'>
                              <XCircle className='h-3 w-3' />
                              {t('Reddedildi')}
                            </Badge>
                          )}
                        </div>
                      </div>

                      <div className='flex flex-wrap items-center gap-x-4 gap-y-1 text-muted-foreground text-[11px]'>
                        <span>
                          {t('Tarih')}: {new Date(item.create_time * 1000).toLocaleString()}
                        </span>
                        {item.quota > 0 && (
                          <span>
                            {t('Tanımlanan Bakiye')}: {formatQuota(item.quota)}
                          </span>
                        )}
                      </div>

                      {item.tx_hash && (
                        <div className='flex items-center gap-1.5 pt-1 border-t text-[11px]'>
                          <span className='text-muted-foreground'>TxID:</span>
                          <span className='font-mono truncate max-w-[280px] select-all'>
                            {item.tx_hash}
                          </span>
                          {explorerUrl && (
                            <a
                              href={explorerUrl}
                              target='_blank'
                              rel='noreferrer'
                              className='text-primary hover:underline inline-flex items-center gap-0.5 ml-auto'
                            >
                              {t('Explorer')}
                              <ExternalLink className='h-3 w-3' />
                            </a>
                          )}
                        </div>
                      )}

                      {item.status === 'rejected' && item.reject_reason && (
                        <div className='text-destructive text-[11px] bg-destructive/10 p-2 rounded'>
                          {t('Red Sebebi')}: {item.reject_reason}
                        </div>
                      )}
                    </div>
                  )
                })}
              </div>
            )}
          </TabsContent>
        </Tabs>
      </DialogContent>
    </Dialog>
  )
}
