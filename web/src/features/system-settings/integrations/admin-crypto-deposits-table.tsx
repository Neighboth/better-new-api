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
import { Check, CheckCircle2, Clock, ExternalLink, Loader2, RefreshCw, Search, XCircle } from 'lucide-react'
import * as React from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { formatQuota } from '@/lib/format'

import {
  approveCryptoDeposit,
  getAdminCryptoDeposits,
  rejectCryptoDeposit,
  type CryptoDepositItem,
} from '@/features/wallet/api'

export function AdminCryptoDepositsTable() {
  const { t } = useTranslation()

  const [deposits, setDeposits] = React.useState<CryptoDepositItem[]>([])
  const [total, setTotal] = React.useState(0)
  const [loading, setLoading] = React.useState(false)
  const [page, setPage] = React.useState(0)
  const [statusFilter, setStatusFilter] = React.useState<string>('all')
  const [searchKeyword, setSearchKeyword] = React.useState('')

  const [actionLoadingId, setActionLoadingId] = React.useState<number | null>(null)
  const [rejectDialogItem, setRejectDialogItem] = React.useState<CryptoDepositItem | null>(null)
  const [rejectReason, setRejectReason] = React.useState('')

  const fetchDeposits = React.useCallback(async () => {
    try {
      setLoading(true)
      const res = await getAdminCryptoDeposits(page, 20, statusFilter, searchKeyword)
      if (res?.success && res.data) {
        setDeposits(res.data.items || [])
        setTotal(res.data.total || 0)
      }
    } catch {
      toast.error(t('Kripto ödemeleri yüklenirken hata oluştu'))
    } finally {
      setLoading(false)
    }
  }, [page, statusFilter, searchKeyword, t])

  React.useEffect(() => {
    fetchDeposits()
  }, [fetchDeposits])

  const handleApprove = async (item: CryptoDepositItem) => {
    if (!confirm(t('{{user}} kullanıcısının ${{amount}} tutarındaki kripto ödemesini onaylamak ve bakiyesini yüklemek istediğinize emin misiniz?', {
      user: item.username,
      amount: item.amount,
    }))) {
      return
    }

    try {
      setActionLoadingId(item.id)
      const res = await approveCryptoDeposit(item.id)
      if (res?.success) {
        toast.success(res.message || t('Ödeme onaylandı ve bakiye yüklendi!'))
        fetchDeposits()
      } else {
        toast.error(res?.message || t('Onaylama işlemi başarısız'))
      }
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : t('İşlem başarısız'))
    } finally {
      setActionLoadingId(null)
    }
  }

  const handleConfirmReject = async () => {
    if (!rejectDialogItem) return

    try {
      setActionLoadingId(rejectDialogItem.id)
      const res = await rejectCryptoDeposit(rejectDialogItem.id, rejectReason)
      if (res?.success) {
        toast.success(t('Ödeme talebi reddedildi'))
        setRejectDialogItem(null)
        setRejectReason('')
        fetchDeposits()
      } else {
        toast.error(res?.message || t('Reddetme başarısız'))
      }
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : t('İşlem başarısız'))
    } finally {
      setActionLoadingId(null)
    }
  }

  const getExplorerUrl = (network: string, hash: string) => {
    const n = network.toLowerCase()
    const clean = hash.trim()
    if (!clean) return ''
    if (n.includes('trc') || n.includes('tron')) return `https://tronscan.org/#/transaction/${clean}`
    if (n.includes('bep') || n.includes('bsc') || n.includes('binance')) return `https://bscscan.com/tx/${clean}`
    if (n.includes('erc') || n.includes('eth') || n.includes('ethereum')) return `https://etherscan.io/tx/${clean}`
    if (n.includes('sol')) return `https://solscan.io/tx/${clean}`
    if (n.includes('btc') || n.includes('bitcoin')) return `https://mempool.space/tx/${clean}`
    return ''
  }

  return (
    <div className='space-y-4 pt-4 border-t'>
      <div className='flex flex-wrap items-center justify-between gap-3'>
        <div>
          <h4 className='text-sm font-semibold'>{t('Kripto Ödeme Bildirimleri Yönetimi')}</h4>
          <p className='text-xs text-muted-foreground'>
            {t('Kullanıcıların gönderdiği transfer bildirimlerini inceleyin, TxID kontrolü yapıp bakiyelerini onaylayın.')}
          </p>
        </div>
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={fetchDeposits}
          disabled={loading}
          className='gap-1.5'
        >
          <RefreshCw className={`h-3.5 w-3.5 ${loading ? 'animate-spin' : ''}`} />
          {t('Yenile')}
        </Button>
      </div>

      {/* Filter Toolbar */}
      <div className='flex flex-wrap items-center gap-3'>
        <div className='relative flex-1 min-w-[200px]'>
          <Search className='absolute left-2.5 top-2.5 h-3.5 w-3.5 text-muted-foreground' />
          <Input
            placeholder={t('Kullanıcı adı, Referans Kodu veya TxID ile ara...')}
            value={searchKeyword}
            onChange={(e) => setSearchKeyword(e.target.value)}
            className='h-8 pl-8 text-xs'
          />
        </div>

        <Select value={statusFilter} onValueChange={setStatusFilter}>
          <SelectTrigger className='w-36 h-8 text-xs'>
            <SelectValue placeholder={t('Tüm Durumlar')} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value='all'>{t('Tüm Durumlar')}</SelectItem>
            <SelectItem value='pending'>{t('Bekleyenler')}</SelectItem>
            <SelectItem value='approved'>{t('Onaylananlar')}</SelectItem>
            <SelectItem value='rejected'>{t('Reddedilenler')}</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {/* Table */}
      <div className='rounded-lg border bg-card overflow-hidden'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className='w-[130px]'>{t('Referans')}</TableHead>
              <TableHead className='w-[120px]'>{t('Kullanıcı')}</TableHead>
              <TableHead className='w-[140px]'>{t('Ağ')}</TableHead>
              <TableHead className='w-[110px]'>{t('Tutar')}</TableHead>
              <TableHead>{t('TxID (İşlem Özeti)')}</TableHead>
              <TableHead className='w-[110px]'>{t('Durum')}</TableHead>
              <TableHead className='w-[140px] text-right'>{t('İşlemler')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {loading ? (
              <TableRow>
                <TableCell colSpan={7} className='h-24 text-center'>
                  <div className='flex items-center justify-center gap-2 text-xs text-muted-foreground'>
                    <Loader2 className='h-4 w-4 animate-spin' />
                    {t('Yükleniyor...')}
                  </div>
                </TableCell>
              </TableRow>
            ) : deposits.length === 0 ? (
              <TableRow>
                <TableCell colSpan={7} className='h-24 text-center text-xs text-muted-foreground'>
                  {t('Kayıt bulunamadı.')}
                </TableCell>
              </TableRow>
            ) : (
              deposits.map((item) => {
                const explorer = getExplorerUrl(item.network, item.tx_hash)
                const isItemLoading = actionLoadingId === item.id
                return (
                  <TableRow key={item.id} className='text-xs'>
                    <TableCell className='font-mono font-bold'>
                      {item.reference_code}
                      <div className='text-[10px] text-muted-foreground font-normal'>
                        {new Date(item.create_time * 1000).toLocaleDateString()}
                      </div>
                    </TableCell>

                    <TableCell>
                      <span className='font-medium text-foreground'>{item.username}</span>
                      <div className='text-[10px] text-muted-foreground'>ID: {item.user_id}</div>
                    </TableCell>

                    <TableCell>
                      <Badge variant='outline' className='text-[11px] font-semibold'>
                        {item.network}
                      </Badge>
                    </TableCell>

                    <TableCell>
                      <div className='font-bold text-emerald-600 dark:text-emerald-400'>
                        ${item.amount}
                      </div>
                      <div className='text-[10px] text-muted-foreground'>
                        {formatQuota(item.quota)}
                      </div>
                    </TableCell>

                    <TableCell>
                      <div className='flex items-center gap-1.5'>
                        <span className='font-mono truncate max-w-[180px] select-all'>
                          {item.tx_hash || '-'}
                        </span>
                        {explorer && (
                          <a
                            href={explorer}
                            target='_blank'
                            rel='noreferrer'
                            className='text-primary hover:underline inline-flex items-center gap-0.5'
                            title={t('Blockchain Explorer')}
                          >
                            <ExternalLink className='h-3 w-3' />
                          </a>
                        )}
                      </div>
                      {item.sender_address && (
                        <div className='text-[10px] text-muted-foreground truncate max-w-[200px]'>
                          {t('Gönderen')}: {item.sender_address}
                        </div>
                      )}
                    </TableCell>

                    <TableCell>
                      {item.status === 'pending' && (
                        <Badge variant='secondary' className='gap-1 text-amber-600 dark:text-amber-400 bg-amber-500/10'>
                          <Clock className='h-3 w-3' />
                          {t('Beklemede')}
                        </Badge>
                      )}
                      {item.status === 'approved' && (
                        <Badge variant='secondary' className='gap-1 text-emerald-600 dark:text-emerald-400 bg-emerald-500/10'>
                          <CheckCircle2 className='h-3 w-3' />
                          {t('Onaylandı')}
                        </Badge>
                      )}
                      {item.status === 'rejected' && (
                        <Badge variant='destructive' className='gap-1' title={item.reject_reason}>
                          <XCircle className='h-3 w-3' />
                          {t('Reddedildi')}
                        </Badge>
                      )}
                    </TableCell>

                    <TableCell className='text-right'>
                      {item.status === 'pending' ? (
                        <div className='flex items-center justify-end gap-1.5'>
                          <Button
                            type='button'
                            size='sm'
                            className='h-7 px-2.5 text-xs bg-emerald-600 hover:bg-emerald-700 text-white'
                            disabled={isItemLoading}
                            onClick={() => handleApprove(item)}
                          >
                            {isItemLoading ? (
                              <Loader2 className='h-3.5 w-3.5 animate-spin' />
                            ) : (
                              t('Onayla')
                            )}
                          </Button>
                          <Button
                            type='button'
                            size='sm'
                            variant='outline'
                            className='h-7 px-2 text-xs text-destructive hover:bg-destructive/10'
                            disabled={isItemLoading}
                            onClick={() => {
                              setRejectDialogItem(item)
                              setRejectReason('')
                            }}
                          >
                            {t('Reddet')}
                          </Button>
                        </div>
                      ) : item.status === 'approved' ? (
                        <span className='text-[11px] text-muted-foreground'>{t('İşlem Tamamlandı')}</span>
                      ) : (
                        <span className='text-[11px] text-destructive truncate max-w-[120px] inline-block' title={item.reject_reason}>
                          {item.reject_reason || t('Reddedildi')}
                        </span>
                      )}
                    </TableCell>
                  </TableRow>
                )
              })
            )}
          </TableBody>
        </Table>
      </div>

      {/* Reject Modal */}
      <Dialog open={!!rejectDialogItem} onOpenChange={(open) => !open && setRejectDialogItem(null)}>
        <DialogContent className='max-w-md'>
          <DialogHeader>
            <DialogTitle>{t('Ödeme Talebini Reddet')}</DialogTitle>
            <DialogDescription>
              {t('{{user}} kullanıcısının {{ref}} referans kodlu ödemesini reddetmek üzeresiniz. Lütfen bir red sebebi belirtin.', {
                user: rejectDialogItem?.username,
                ref: rejectDialogItem?.reference_code,
              })}
            </DialogDescription>
          </DialogHeader>

          <div className='space-y-2 py-2'>
            <Input
              placeholder={t('Örn: Geçersiz TxID, cüzdanda transfer görünmüyor')}
              value={rejectReason}
              onChange={(e) => setRejectReason(e.target.value)}
            />
          </div>

          <DialogFooter>
            <Button type='button' variant='outline' onClick={() => setRejectDialogItem(null)}>
              {t('İptal')}
            </Button>
            <Button type='button' variant='destructive' onClick={handleConfirmReject}>
              {t('Reddet')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
