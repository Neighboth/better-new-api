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
import { Coins, Edit2, Plus, Trash2 } from 'lucide-react'
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
import { Label } from '@/components/ui/label'

export type CryptoWalletConfig = {
  id: string
  network: string
  address: string
  qr_code?: string
  min_deposit?: number
}

type CryptoWalletsVisualEditorProps = {
  value: string
  onChange: (value: string) => void
}

const COMMON_NETWORKS = [
  'USDT (TRC20)',
  'USDT (ERC20)',
  'USDT (BEP20)',
  'USDC (ERC20)',
  'USDC (Solana)',
  'BTC (Bitcoin)',
  'ETH (Ethereum)',
  'SOL (Solana)',
  'TON (The Open Network)',
  'LTC (Litecoin)',
]

export function CryptoWalletsVisualEditor({
  value,
  onChange,
}: CryptoWalletsVisualEditorProps) {
  const { t } = useTranslation()

  const wallets = React.useMemo<CryptoWalletConfig[]>(() => {
    try {
      const parsed = JSON.parse(value || '[]')
      return Array.isArray(parsed) ? parsed : []
    } catch {
      return []
    }
  }, [value])

  const [dialogOpen, setDialogOpen] = React.useState(false)
  const [editingIndex, setEditingIndex] = React.useState<number | null>(null)
  const [draft, setDraft] = React.useState<CryptoWalletConfig>({
    id: '',
    network: 'USDT (TRC20)',
    address: '',
    qr_code: '',
    min_deposit: 1,
  })

  const handleOpenAdd = () => {
    setEditingIndex(null)
    setDraft({
      id: `wallet-${Date.now().toString(36)}`,
      network: 'USDT (TRC20)',
      address: '',
      qr_code: '',
      min_deposit: 1,
    })
    setDialogOpen(true)
  }

  const handleOpenEdit = (index: number) => {
    setEditingIndex(index)
    setDraft({ ...wallets[index] })
    setDialogOpen(true)
  }

  const handleRemove = (index: number) => {
    const next = wallets.filter((_, i) => i !== index)
    onChange(JSON.stringify(next))
    toast.success(t('Wallet removed'))
  }

  const handleSave = () => {
    if (!draft.network.trim()) {
      toast.error(t('Please enter network name'))
      return
    }
    if (!draft.address.trim()) {
      toast.error(t('Please enter wallet address'))
      return
    }

    const cleanDraft = {
      ...draft,
      id: draft.id || `wallet-${Date.now().toString(36)}`,
      network: draft.network.trim(),
      address: draft.address.trim(),
      qr_code: draft.qr_code?.trim() || '',
      min_deposit: Number(draft.min_deposit) || 1,
    }

    let next: CryptoWalletConfig[]
    if (editingIndex !== null) {
      next = wallets.map((w, i) => (i === editingIndex ? cleanDraft : w))
    } else {
      next = [...wallets, cleanDraft]
    }

    onChange(JSON.stringify(next))
    setDialogOpen(false)
    toast.success(t('Wallet saved'))
  }

  return (
    <div className='space-y-4'>
      <div className='flex items-center justify-between'>
        <div>
          <h4 className='text-sm font-semibold'>{t('Crypto Wallets & Networks')}</h4>
          <p className='text-xs text-muted-foreground'>
            {t('Configure the networks and wallet addresses that users can transfer funds to.')}
          </p>
        </div>
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={handleOpenAdd}
          className='gap-1.5'
        >
          <Plus className='h-4 w-4' />
          {t('Add Wallet')}
        </Button>
      </div>

      {wallets.length === 0 ? (
        <div className='rounded-lg border border-dashed p-6 text-center text-xs text-muted-foreground space-y-2'>
          <Coins className='mx-auto h-8 w-8 opacity-40' />
          <p>{t('No crypto wallets configured yet.')}</p>
          <Button type='button' variant='outline' size='sm' onClick={handleOpenAdd}>
            <Plus className='h-3.5 w-3.5 mr-1' />
            {t('Add First Wallet')}
          </Button>
        </div>
      ) : (
        <div className='grid gap-3 sm:grid-cols-2'>
          {wallets.map((wallet, index) => (
            <div
              key={index}
              className='rounded-lg border bg-card p-4 space-y-3 relative group'
            >
              <div className='flex items-center justify-between'>
                <div className='flex items-center gap-2'>
                  <Badge variant='outline' className='font-semibold'>
                    {wallet.network}
                  </Badge>
                  {wallet.min_deposit ? (
                    <span className='text-[10px] text-muted-foreground'>
                      Min: ${wallet.min_deposit}
                    </span>
                  ) : null}
                </div>
                <div className='flex items-center gap-1'>
                  <Button
                    type='button'
                    variant='ghost'
                    size='icon'
                    className='h-7 w-7'
                    onClick={() => handleOpenEdit(index)}
                  >
                    <Edit2 className='h-3.5 w-3.5' />
                  </Button>
                  <Button
                    type='button'
                    variant='ghost'
                    size='icon'
                    className='h-7 w-7 text-destructive hover:bg-destructive/10'
                    onClick={() => handleRemove(index)}
                  >
                    <Trash2 className='h-3.5 w-3.5' />
                  </Button>
                </div>
              </div>

              <div>
                <Label className='text-[10px] uppercase text-muted-foreground font-semibold'>
                  {t('Wallet Address')}
                </Label>
                <code className='block text-xs font-mono break-all bg-muted/40 p-1.5 rounded mt-0.5 select-all'>
                  {wallet.address}
                </code>
              </div>

              {wallet.qr_code && (
                <div className='flex items-center gap-2 pt-1 border-t text-[11px] text-muted-foreground'>
                  <span>QR: {wallet.qr_code}</span>
                </div>
              )}
            </div>
          ))}
        </div>
      )}

      {/* Edit / Add Dialog */}
      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className='max-w-md'>
          <DialogHeader>
            <DialogTitle>
              {editingIndex !== null ? t('Edit Wallet') : t('Add New Crypto Wallet')}
            </DialogTitle>
            <DialogDescription>
              {t('Specify the network and deposit address where users can send funds.')}
            </DialogDescription>
          </DialogHeader>

          <div className='space-y-4 py-2'>
            <div className='space-y-1.5'>
              <Label className='text-xs font-medium'>{t('Network Type')}</Label>
              <Input
                placeholder='e.g. USDT (TRC20)'
                value={draft.network}
                onChange={(e) => setDraft((prev) => ({ ...prev, network: e.target.value }))}
              />
              <div className='flex flex-wrap gap-1 mt-1.5'>
                {COMMON_NETWORKS.map((net) => (
                  <button
                    key={net}
                    type='button'
                    className='text-[10px] px-2 py-0.5 rounded bg-muted hover:bg-accent text-muted-foreground hover:text-foreground'
                    onClick={() => setDraft((prev) => ({ ...prev, network: net }))}
                  >
                    {net}
                  </button>
                ))}
              </div>
            </div>

            <div className='space-y-1.5'>
              <Label className='text-xs font-medium'>{t('Deposit Address')}</Label>
              <Input
                placeholder='0x... or T...'
                value={draft.address}
                className='font-mono text-xs'
                onChange={(e) => setDraft((prev) => ({ ...prev, address: e.target.value }))}
              />
            </div>

            <div className='grid grid-cols-2 gap-3'>
              <div className='space-y-1.5'>
                <Label className='text-xs font-medium'>{t('Minimum Deposit ($)')}</Label>
                <Input
                  type='number'
                  min='0'
                  value={draft.min_deposit || 1}
                  onChange={(e) =>
                    setDraft((prev) => ({ ...prev, min_deposit: Number(e.target.value) || 0 }))
                  }
                />
              </div>

              <div className='space-y-1.5'>
                <Label className='text-xs font-medium'>{t('QR Code URL (Optional)')}</Label>
                <Input
                  placeholder='https://.../qr.png'
                  value={draft.qr_code || ''}
                  onChange={(e) => setDraft((prev) => ({ ...prev, qr_code: e.target.value }))}
                />
              </div>
            </div>
          </div>

          <DialogFooter>
            <Button type='button' variant='outline' onClick={() => setDialogOpen(false)}>
              {t('Cancel')}
            </Button>
            <Button type='button' onClick={handleSave}>
              {t('Save')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
