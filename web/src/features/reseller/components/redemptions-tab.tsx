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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Check,
  Copy,
  Loader2,
  Plus,
  Ticket,
  Trash2,
  Wallet,
} from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Dialog } from '@/components/dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { getSelf } from '@/lib/api'
import {
  formatQuota,
  formatRedemptionQuota,
  formatTimestamp,
  parseQuotaFromDollars,
} from '@/lib/format'
import { useAuthStore } from '@/stores/auth-store'

import {
  createResellerRedemption,
  deleteResellerRedemption,
  fetchResellerRedemptions,
} from '../api'
import type { ResellerRedemption } from '../types'

export function RedemptionsTab() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const currentUser = useAuthStore((s) => s.auth.user)

  const [page, setPage] = useState(1)
  const [keyword, setKeyword] = useState('')
  const [createDialogOpen, setCreateDialogOpen] = useState(false)
  const [generatedKeys, setGeneratedKeys] = useState<string[]>([])
  const [copiedKey, setCopiedKey] = useState<string | null>(null)

  // Creation form state
  const [codeName, setCodeName] = useState('')
  const [dollarAmount, setDollarAmount] = useState<number>(5)
  const [codeCount, setCodeCount] = useState<number>(1)

  // Deletion state
  const [deleteId, setDeleteId] = useState<number | null>(null)

  const refreshUserQuota = async () => {
    try {
      const res = await getSelf()
      if (res?.success && res?.data) {
        useAuthStore.getState().auth.setUser(res.data)
      }
    } catch {
      // ignore
    }
  }

  const { data, isLoading } = useQuery({
    queryKey: ['reseller-redemptions', page, keyword],
    queryFn: () => fetchResellerRedemptions(page, 10, keyword),
  })

  const codeQuotaValue = parseQuotaFromDollars(dollarAmount)
  const totalCost = codeQuotaValue * codeCount
  const availableBalance = currentUser?.quota ?? 0

  const hasEnoughBalance = availableBalance >= totalCost

  const createMutation = useMutation({
    mutationFn: async () => {
      return createResellerRedemption({
        name: codeName.trim() || `${dollarAmount}$ Code`,
        quota: codeQuotaValue,
        count: codeCount,
        type: 0,
      })
    },
    onSuccess: async (res) => {
      toast.success(t('Redemption codes created successfully'))
      setGeneratedKeys(res.keys || [])
      queryClient.invalidateQueries({ queryKey: ['reseller-redemptions'] })
      // Refresh current user's profile to reflect deducted balance immediately
      await refreshUserQuota()
    },
    onError: (err: Error) => {
      toast.error(err.message || t('Failed to create redemption codes'))
    },
  })

  const deleteMutation = useMutation({
    mutationFn: async (id: number) => {
      return deleteResellerRedemption(id)
    },
    onSuccess: async () => {
      toast.success(t('Redemption code deleted and quota refunded'))
      setDeleteId(null)
      queryClient.invalidateQueries({ queryKey: ['reseller-redemptions'] })
      // Refresh current user's profile to reflect refunded quota immediately
      await refreshUserQuota()
    },
    onError: (err: Error) => {
      toast.error(err.message || t('Failed to delete redemption code'))
    },
  })

  const handleCopy = (text: string) => {
    navigator.clipboard.writeText(text)
    setCopiedKey(text)
    toast.success(t('Copied to clipboard'))
    setTimeout(() => setCopiedKey(null), 2000)
  }

  const handleCopyAll = () => {
    navigator.clipboard.writeText(generatedKeys.join('\n'))
    toast.success(t('All codes copied to clipboard'))
  }

  return (
    <div className='space-y-6'>
      {/* Reseller Balances Header */}
      <Card>
        <CardContent className='flex items-center justify-between gap-3 py-3.5'>
          <div className='flex items-center gap-3'>
            <Wallet className='h-5 w-5' />
            <div>
              <p className='text-muted-foreground text-xs'>
                {t('Available Quota')}
              </p>
              <p className='font-mono text-lg font-bold'>
                {formatQuota(currentUser?.quota ?? 0)}
              </p>
            </div>
          </div>
          <Button size='sm' onClick={() => setCreateDialogOpen(true)}>
            <Plus className='mr-1.5 h-4 w-4' />
            {t('New Codes')}
          </Button>
        </CardContent>
      </Card>

      {/* Redemption Codes Table Card */}
      <Card>
        <CardHeader>
          <CardTitle className='flex items-center gap-2 text-base'>
            <Ticket className='text-primary h-4 w-4' />
            {t('Issued Redemption Codes')}
          </CardTitle>
          <CardDescription className='text-xs'>
            {t('Issued Redemption Codes')}
          </CardDescription>
        </CardHeader>
        <CardContent className='space-y-4'>
          <div className='flex max-w-sm gap-2'>
            <Input
              placeholder={t('Search code name or key...')}
              value={keyword}
              onChange={(e) => {
                setKeyword(e.target.value)
                setPage(1)
              }}
            />
          </div>

          <div className='rounded-md border'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('Code / Key')}</TableHead>
                  <TableHead>{t('Name')}</TableHead>
                  <TableHead>{t('Value')}</TableHead>
                  <TableHead>{t('Status')}</TableHead>
                  <TableHead>{t('Used By')}</TableHead>
                  <TableHead>{t('Created At')}</TableHead>
                  <TableHead className='text-right'>{t('Actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {isLoading && (
                  <TableRow>
                    <TableCell
                      colSpan={7}
                      className='text-muted-foreground py-8 text-center'
                    >
                      <Loader2 className='mx-auto h-6 w-6 animate-spin' />
                    </TableCell>
                  </TableRow>
                )}
                {!isLoading && (data?.items?.length ?? 0) === 0 && (
                  <TableRow>
                    <TableCell
                      colSpan={7}
                      className='text-muted-foreground py-8 text-center'
                    >
                      {t('No redemption codes found.')}
                    </TableCell>
                  </TableRow>
                )}
                {!isLoading &&
                  data?.items.map((item: ResellerRedemption) => (
                    <TableRow key={item.id}>
                      <TableCell className='font-mono text-xs'>
                        <div className='flex items-center gap-1.5'>
                          <span>{item.key}</span>
                          <Button
                            variant='ghost'
                            size='icon'
                            className='h-6 w-6'
                            onClick={() => handleCopy(item.key)}
                          >
                            {copiedKey === item.key ? (
                              <Check className='h-3 w-3 text-emerald-500' />
                            ) : (
                              <Copy className='h-3 w-3' />
                            )}
                          </Button>
                        </div>
                      </TableCell>
                      <TableCell className='text-xs font-medium'>
                        {item.name}
                      </TableCell>
                      <TableCell className='font-mono text-xs font-medium'>
                        {formatRedemptionQuota(item.quota, item.type, t)}
                      </TableCell>
                      <TableCell>
                        {item.status === 1 ? (
                          <Badge
                            variant='outline'
                            className='text-[10px]'
                          >
                            {t('Unused')}
                          </Badge>
                        ) : (
                          <Badge
                            variant='secondary'
                            className='text-[10px]'
                          >
                            {t('Redeemed')}
                          </Badge>
                        )}
                      </TableCell>
                      <TableCell className='text-xs'>
                        {item.status !== 1 && item.used_user_id ? (
                          <div className='flex items-center gap-1.5'>
                            <span className='text-foreground font-medium'>
                              {item.used_username || `ID: ${item.used_user_id}`}
                            </span>
                          </div>
                        ) : (
                          <span className='text-muted-foreground'>-</span>
                        )}
                      </TableCell>
                      <TableCell className='text-muted-foreground text-xs'>
                        {formatTimestamp(item.created_time)}
                      </TableCell>
                      <TableCell className='text-right'>
                        {item.status === 1 && (
                          <Button
                            variant='ghost'
                            size='icon'
                            className='text-destructive hover:bg-destructive/10 h-7 w-7'
                            onClick={() => setDeleteId(item.id)}
                          >
                            <Trash2 className='h-3.5 w-3.5' />
                          </Button>
                        )}
                      </TableCell>
                    </TableRow>
                  ))}
              </TableBody>
            </Table>
          </div>
        </CardContent>
      </Card>

      {/* Creation Modal */}
      <Dialog
        open={createDialogOpen}
        onOpenChange={(open) => {
          setCreateDialogOpen(open)
          if (!open) setGeneratedKeys([])
        }}
        title={
          generatedKeys.length > 0
            ? t('Generated Codes')
            : t('Generate Redemption Codes')
        }
        description={
          generatedKeys.length > 0
            ? t(
                'Save these codes now. You can copy and distribute them to your customers.'
              )
            : t(
                'Specify the dollar amount and number of redemption codes to generate.'
              )
        }
      >
        {generatedKeys.length > 0 ? (
          <div className='space-y-4 py-2'>
            <div className='bg-muted/40 max-h-60 space-y-1.5 overflow-y-auto rounded-md border p-3 font-mono text-xs'>
              {generatedKeys.map((key) => (
                <div key={key} className='flex items-center justify-between'>
                  <span>{key}</span>
                  <Button
                    variant='ghost'
                    size='icon'
                    className='h-5 w-5'
                    onClick={() => handleCopy(key)}
                  >
                    {copiedKey === key ? (
                      <Check className='h-3 w-3 text-emerald-500' />
                    ) : (
                      <Copy className='h-3 w-3' />
                    )}
                  </Button>
                </div>
              ))}
            </div>
            <div className='flex justify-end gap-2'>
              <Button variant='outline' onClick={handleCopyAll}>
                <Copy className='mr-2 h-4 w-4' />
                {t('Copy All')}
              </Button>
              <Button onClick={() => setCreateDialogOpen(false)}>
                {t('Done')}
              </Button>
            </div>
          </div>
        ) : (
          <div className='space-y-4 py-2'>
            <div className='space-y-1.5'>
              <Label htmlFor='code_name' className='text-xs font-medium'>
                {t('Batch / Code Name')}
              </Label>
              <Input
                id='code_name'
                placeholder={t('e.g. Summer Promo 10$')}
                value={codeName}
                onChange={(e) => setCodeName(e.target.value)}
              />
            </div>

            <div className='grid grid-cols-2 gap-4'>
              <div className='space-y-1.5'>
                <Label htmlFor='amount_input' className='text-xs font-medium'>
                  {t('Amount per Code ($)')}
                </Label>
                <Input
                  id='amount_input'
                  type='number'
                  min={0.1}
                  step={0.1}
                  value={dollarAmount}
                  onChange={(e) =>
                    setDollarAmount(
                      Math.max(0.01, Number.parseFloat(e.target.value) || 0)
                    )
                  }
                />
              </div>

              <div className='space-y-1.5'>
                <Label htmlFor='code_count' className='text-xs font-medium'>
                  {t('Quantity')}
                </Label>
                <Input
                  id='code_count'
                  type='number'
                  min={1}
                  max={100}
                  value={codeCount}
                  onChange={(e) =>
                    setCodeCount(
                      Math.min(
                        100,
                        Math.max(1, Number.parseInt(e.target.value, 10) || 1)
                      )
                    )
                  }
                />
              </div>
            </div>

            <div className='bg-muted/50 space-y-1 rounded-lg p-3 text-xs'>
              <div className='text-muted-foreground flex justify-between'>
                <span>{t('Total Cost')}:</span>
                <span className='text-foreground font-mono font-medium'>
                  {formatQuota(totalCost)}
                </span>
              </div>
              <div className='text-muted-foreground flex justify-between'>
                <span>{t('Your Balance')}:</span>
                <span className='text-foreground font-mono font-medium'>
                  {formatQuota(availableBalance)}
                </span>
              </div>
              {!hasEnoughBalance && (
                <p className='text-destructive pt-1 text-[11px]'>
                  {t('Insufficient balance to create these codes.')}
                </p>
              )}
            </div>

            <div className='flex justify-end gap-2 pt-2'>
              <Button
                variant='outline'
                onClick={() => setCreateDialogOpen(false)}
              >
                {t('Cancel')}
              </Button>
              <Button
                onClick={() => createMutation.mutate()}
                disabled={!hasEnoughBalance || createMutation.isPending}
              >
                {createMutation.isPending && (
                  <Loader2 className='mr-2 h-4 w-4 animate-spin' />
                )}
                {t('Create Codes')}
              </Button>
            </div>
          </div>
        )}
      </Dialog>

      {/* Delete Confirmation */}
      <ConfirmDialog
        open={deleteId !== null}
        onOpenChange={(open) => !open && setDeleteId(null)}
        title={t('Delete Redemption Code')}
        desc={t(
          'Are you sure you want to delete this unused redemption code? Its quota will be refunded directly back to your balance.'
        )}
        confirmText={t('Delete & Refund')}
        handleConfirm={() => {
          if (deleteId) deleteMutation.mutate(deleteId)
        }}
      />
    </div>
  )
}
