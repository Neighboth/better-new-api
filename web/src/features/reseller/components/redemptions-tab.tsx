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
import { Check, Coins, Copy, FileText, Loader2, MessageSquareCode, Plus, Ticket, Trash2, Wallet } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Dialog } from '@/components/dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
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
import { formatQuota, formatRedemptionQuota, formatTimestamp, parseQuotaFromDollars } from '@/lib/format'
import { useAuthStore } from '@/stores/auth-store'

import {
  createResellerRedemption,
  deleteResellerRedemption,
  fetchRedeemedUserLogs,
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
  const [codeType, setCodeType] = useState<number>(0) // 0: Quota, 1: Requests, 2: Tokens
  const [codeName, setCodeName] = useState('')
  const [dollarAmount, setDollarAmount] = useState<number>(5)
  const [unitCount, setUnitCount] = useState<number>(100)
  const [codeCount, setCodeCount] = useState<number>(1)

  // Deletion state
  const [deleteId, setDeleteId] = useState<number | null>(null)

  // User request logs dialog state
  const [logsUserId, setLogsUserId] = useState<number | null>(null)
  const [logsUsername, setLogsUsername] = useState<string>('')
  const [logsPage, setLogsPage] = useState(1)

  const { data: logsData, isLoading: logsLoading } = useQuery({
    queryKey: ['redeemed-user-logs', logsUserId, logsPage],
    queryFn: () =>
      logsUserId
        ? fetchRedeemedUserLogs(logsUserId, logsPage, 10)
        : Promise.resolve({ items: [], total: 0 }),
    enabled: !!logsUserId,
  })

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

  const codeQuotaValue =
    codeType === 0 ? parseQuotaFromDollars(dollarAmount) : unitCount
  const totalCost = codeQuotaValue * codeCount

  const availableBalance =
    codeType === 0
      ? (currentUser?.quota ?? 0)
      : codeType === 1
      ? ((currentUser as any)?.requests_balance ?? 0)
      : ((currentUser as any)?.tokens_balance ?? 0)

  const hasEnoughBalance = availableBalance >= totalCost

  const createMutation = useMutation({
    mutationFn: async () => {
      const typeLabel =
        codeType === 0
          ? `${dollarAmount}$`
          : codeType === 1
          ? `${unitCount} Req`
          : `${unitCount} Tok`
      return createResellerRedemption({
        name: codeName.trim() || `${typeLabel} Code`,
        quota: codeQuotaValue,
        count: codeCount,
        type: codeType,
      })
    },
    onSuccess: async (res) => {
      toast.success(t('Redemption codes created successfully'))
      setGeneratedKeys(res.keys || [])
      queryClient.invalidateQueries({ queryKey: ['reseller-redemptions'] })
      // Refresh current user's profile to reflect deducted balance immediately
      await refreshUserQuota()
    },
    onError: (err: any) => {
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
    onError: (err: any) => {
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
      <div className='grid grid-cols-1 md:grid-cols-3 gap-4'>
        <Card>
          <CardContent className='flex items-center gap-3 py-3.5'>
            <div className='rounded-full bg-primary/10 p-2.5 text-primary'>
              <Wallet className='h-5 w-5' />
            </div>
            <div>
              <p className='text-xs text-muted-foreground'>{t('Available Quota')}</p>
              <p className='text-lg font-bold font-mono'>{formatQuota(currentUser?.quota ?? 0)}</p>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardContent className='flex items-center gap-3 py-3.5'>
            <div className='rounded-full bg-sky-500/10 p-2.5 text-sky-500'>
              <MessageSquareCode className='h-5 w-5' />
            </div>
            <div>
              <p className='text-xs text-muted-foreground'>{t('Available Requests')}</p>
              <p className='text-lg font-bold font-mono'>{((currentUser as any)?.requests_balance ?? 0).toLocaleString()} {t('req')}</p>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardContent className='flex items-center justify-between gap-3 py-3.5'>
            <div className='flex items-center gap-3'>
              <div className='rounded-full bg-amber-500/10 p-2.5 text-amber-500'>
                <Coins className='h-5 w-5' />
              </div>
              <div>
                <p className='text-xs text-muted-foreground'>{t('Available Tokens')}</p>
                <p className='text-lg font-bold font-mono'>{((currentUser as any)?.tokens_balance ?? 0).toLocaleString()} {t('tokens')}</p>
              </div>
            </div>
            <Button size='sm' onClick={() => setCreateDialogOpen(true)}>
              <Plus className='mr-1.5 h-4 w-4' />
              {t('New Codes')}
            </Button>
          </CardContent>
        </Card>
      </div>

      {/* Redemption Codes Table Card */}
      <Card>
        <CardHeader>
          <CardTitle className='text-base flex items-center gap-2'>
            <Ticket className='h-4 w-4 text-primary' />
            {t('Issued Redemption Codes')}
          </CardTitle>
          <CardDescription className='text-xs'>
            {t('Redemption codes generated from your quota, request, or token balances. Unused codes can be refunded.')}
          </CardDescription>
        </CardHeader>
        <CardContent className='space-y-4'>
          <div className='flex gap-2 max-w-sm'>
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
                  <TableHead>{t('Type')}</TableHead>
                  <TableHead>{t('Value')}</TableHead>
                  <TableHead>{t('Status')}</TableHead>
                  <TableHead>{t('Used By')}</TableHead>
                  <TableHead>{t('Created At')}</TableHead>
                  <TableHead className='text-right'>{t('Actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {isLoading ? (
                  <TableRow>
                    <TableCell colSpan={8} className='py-8 text-center text-muted-foreground'>
                      <Loader2 className='mx-auto h-6 w-6 animate-spin' />
                    </TableCell>
                  </TableRow>
                ) : (data?.items?.length ?? 0) === 0 ? (
                  <TableRow>
                    <TableCell colSpan={8} className='py-8 text-center text-muted-foreground'>
                      {t('No redemption codes found.')}
                    </TableCell>
                  </TableRow>
                ) : (
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
                      <TableCell className='text-xs font-medium'>{item.name}</TableCell>
                      <TableCell>
                        {item.type === 1 ? (
                          <Badge variant='outline' className='bg-sky-500/10 text-sky-600 border-sky-500/20 text-[10px]'>
                            {t('Requests')}
                          </Badge>
                        ) : item.type === 2 ? (
                          <Badge variant='outline' className='bg-amber-500/10 text-amber-600 border-amber-500/20 text-[10px]'>
                            {t('Tokens')}
                          </Badge>
                        ) : (
                          <Badge variant='outline' className='bg-purple-500/10 text-purple-600 border-purple-500/20 text-[10px]'>
                            {t('Balance')}
                          </Badge>
                        )}
                      </TableCell>
                      <TableCell className='text-xs font-semibold text-emerald-600 dark:text-emerald-400 font-mono'>
                        {formatRedemptionQuota(item.quota, item.type, t)}
                      </TableCell>
                      <TableCell>
                        {item.status === 1 ? (
                          <Badge variant='outline' className='bg-emerald-500/10 text-emerald-600 border-emerald-500/20 text-[10px]'>
                            {t('Unused')}
                          </Badge>
                        ) : (
                          <Badge variant='outline' className='bg-muted text-muted-foreground text-[10px]'>
                            {t('Redeemed')}
                          </Badge>
                        )}
                      </TableCell>
                      <TableCell className='text-xs'>
                        {item.status !== 1 && item.used_user_id ? (
                          <div className='flex items-center gap-1.5'>
                            <span className='font-medium text-foreground'>
                              {item.used_username || `ID: ${item.used_user_id}`}
                            </span>
                            <Button
                              variant='ghost'
                              size='sm'
                              className='h-6 px-1.5 text-[11px] gap-1 text-primary hover:bg-primary/10'
                              onClick={() => {
                                setLogsUserId(item.used_user_id!)
                                setLogsUsername(item.used_username || `User #${item.used_user_id}`)
                                setLogsPage(1)
                              }}
                              title={t('View Request Logs')}
                            >
                              <FileText className='h-3 w-3' />
                              <span>{t('Logs')}</span>
                            </Button>
                          </div>
                        ) : (
                          <span className='text-muted-foreground'>-</span>
                        )}
                      </TableCell>
                      <TableCell className='text-xs text-muted-foreground'>
                        {formatTimestamp(item.created_time)}
                      </TableCell>
                      <TableCell className='text-right'>
                        {item.status === 1 && (
                          <Button
                            variant='ghost'
                            size='icon'
                            className='h-7 w-7 text-destructive hover:bg-destructive/10'
                            onClick={() => setDeleteId(item.id)}
                          >
                            <Trash2 className='h-3.5 w-3.5' />
                          </Button>
                        )}
                      </TableCell>
                    </TableRow>
                  ))
                )}
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
        title={generatedKeys.length > 0 ? t('Generated Codes') : t('Generate Redemption Codes')}
        description={
          generatedKeys.length > 0
            ? t('Save these codes now. You can copy and distribute them to your customers.')
            : t('Specify the dollar amount and number of redemption codes to generate.')
        }
      >
        {generatedKeys.length > 0 ? (
          <div className='space-y-4 py-2'>
            <div className='max-h-60 overflow-y-auto rounded-md border bg-muted/40 p-3 font-mono text-xs space-y-1.5'>
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
              <Button onClick={() => setCreateDialogOpen(false)}>{t('Done')}</Button>
            </div>
          </div>
        ) : (
          <div className='space-y-4 py-2'>
            {/* Code Type Selection */}
            <div className='space-y-1.5'>
              <Label className='text-xs font-medium'>{t('Code Type')}</Label>
              <div className='grid grid-cols-3 gap-2'>
                <Button
                  type='button'
                  variant={codeType === 0 ? 'default' : 'outline'}
                  size='sm'
                  className='text-xs'
                  onClick={() => setCodeType(0)}
                >
                  <Wallet className='mr-1.5 h-3.5 w-3.5' />
                  {t('Balance ($)')}
                </Button>
                <Button
                  type='button'
                  variant={codeType === 1 ? 'default' : 'outline'}
                  size='sm'
                  className='text-xs'
                  onClick={() => setCodeType(1)}
                >
                  <MessageSquareCode className='mr-1.5 h-3.5 w-3.5' />
                  {t('Requests')}
                </Button>
                <Button
                  type='button'
                  variant={codeType === 2 ? 'default' : 'outline'}
                  size='sm'
                  className='text-xs'
                  onClick={() => setCodeType(2)}
                >
                  <Coins className='mr-1.5 h-3.5 w-3.5' />
                  {t('Tokens')}
                </Button>
              </div>
            </div>

            <div className='space-y-1.5'>
              <Label htmlFor='code_name' className='text-xs font-medium'>
                {t('Batch / Code Name')}
              </Label>
              <Input
                id='code_name'
                placeholder={
                  codeType === 0
                    ? t('e.g. Summer Promo 10$')
                    : codeType === 1
                    ? t('e.g. 500 Requests Pack')
                    : t('e.g. 1M Tokens Pack')
                }
                value={codeName}
                onChange={(e) => setCodeName(e.target.value)}
              />
            </div>

            <div className='grid grid-cols-2 gap-4'>
              <div className='space-y-1.5'>
                <Label htmlFor='amount_input' className='text-xs font-medium'>
                  {codeType === 0
                    ? t('Amount per Code ($)')
                    : codeType === 1
                    ? t('Requests per Code')
                    : t('Tokens per Code')}
                </Label>
                {codeType === 0 ? (
                  <Input
                    id='amount_input'
                    type='number'
                    min={0.1}
                    step={0.1}
                    value={dollarAmount}
                    onChange={(e) =>
                      setDollarAmount(Math.max(0.01, parseFloat(e.target.value) || 0))
                    }
                  />
                ) : (
                  <Input
                    id='amount_input'
                    type='number'
                    min={1}
                    step={1}
                    value={unitCount}
                    onChange={(e) =>
                      setUnitCount(Math.max(1, parseInt(e.target.value) || 1))
                    }
                  />
                )}
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
                      Math.min(100, Math.max(1, parseInt(e.target.value) || 1))
                    )
                  }
                />
              </div>
            </div>

            <div className='rounded-lg bg-muted/50 p-3 text-xs space-y-1'>
              <div className='flex justify-between text-muted-foreground'>
                <span>{t('Total Cost')}:</span>
                <span className='font-mono font-medium text-foreground'>
                  {codeType === 0
                    ? formatQuota(totalCost)
                    : codeType === 1
                    ? `${totalCost.toLocaleString()} ${t('req')}`
                    : `${totalCost.toLocaleString()} ${t('tokens')}`}
                </span>
              </div>
              <div className='flex justify-between text-muted-foreground'>
                <span>{t('Your Balance')}:</span>
                <span className='font-mono font-medium text-foreground'>
                  {codeType === 0
                    ? formatQuota(availableBalance)
                    : codeType === 1
                    ? `${availableBalance.toLocaleString()} ${t('req')}`
                    : `${availableBalance.toLocaleString()} ${t('tokens')}`}
                </span>
              </div>
              {!hasEnoughBalance && (
                <p className='text-destructive text-[11px] pt-1'>
                  {t('Insufficient balance to create these codes.')}
                </p>
              )}
            </div>

            <div className='flex justify-end gap-2 pt-2'>
              <Button variant='outline' onClick={() => setCreateDialogOpen(false)}>
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
        desc={t('Are you sure you want to delete this unused redemption code? Its quota will be refunded directly back to your balance.')}
        confirmText={t('Delete & Refund')}
        handleConfirm={() => {
          if (deleteId) deleteMutation.mutate(deleteId)
        }}
      />

      {/* Redeemed User Request Logs Dialog */}
      <Dialog
        open={logsUserId !== null}
        onOpenChange={(open) => {
          if (!open) setLogsUserId(null)
        }}
        title={`${t('User Request Logs')}: ${logsUsername}`}
        description={t('Recent API request logs for this user.')}
      >
        <div className='space-y-4 py-2'>
          <div className='rounded-md border max-h-96 overflow-y-auto'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className='text-xs'>{t('Time')}</TableHead>
                  <TableHead className='text-xs'>{t('Model')}</TableHead>
                  <TableHead className='text-xs'>{t('Tokens')}</TableHead>
                  <TableHead className='text-xs'>{t('Quota')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {logsLoading ? (
                  <TableRow>
                    <TableCell colSpan={4} className='py-8 text-center text-muted-foreground'>
                      <Loader2 className='mx-auto h-5 w-5 animate-spin' />
                    </TableCell>
                  </TableRow>
                ) : (logsData?.items?.length ?? 0) === 0 ? (
                  <TableRow>
                    <TableCell colSpan={4} className='py-8 text-center text-xs text-muted-foreground'>
                      {t('No request logs found for this user.')}
                    </TableCell>
                  </TableRow>
                ) : (
                  logsData?.items.map((log: any) => (
                    <TableRow key={log.id}>
                      <TableCell className='text-[11px] text-muted-foreground whitespace-nowrap'>
                        {formatTimestamp(log.created_at)}
                      </TableCell>
                      <TableCell className='text-[11px] font-mono'>
                        {log.model_name || '-'}
                      </TableCell>
                      <TableCell className='text-[11px]'>
                        {(log.prompt_tokens || 0) + (log.completion_tokens || 0)}
                      </TableCell>
                      <TableCell className='text-[11px] font-semibold text-emerald-600 dark:text-emerald-400'>
                        {formatQuota(log.quota)}
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </div>
          <div className='flex justify-end'>
            <Button variant='outline' size='sm' onClick={() => setLogsUserId(null)}>
              {t('Close')}
            </Button>
          </div>
        </div>
      </Dialog>
    </div>
  )
}
