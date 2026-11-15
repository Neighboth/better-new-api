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
import { useQueryClient } from '@tanstack/react-query'
import {
  AlertTriangle,
  CheckCircle2,
  KeyRound,
  Loader2,
  RefreshCw,
  Trash2,
  XCircle,
} from 'lucide-react'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { ScrollArea } from '@/components/ui/scroll-area'
import { cn } from '@/lib/utils'

import { deleteMultiKeys, testAllKeysOfChannel } from '../../api'
import { channelsQueryKeys } from '../../lib'
import type { KeyTestOutcome } from '../../types'
import { useChannels } from '../channels-provider'

type MultiKeyTestDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function MultiKeyTestDialog({
  open,
  onOpenChange,
}: MultiKeyTestDialogProps) {
  const { t } = useTranslation()
  const { currentRow } = useChannels()
  const queryClient = useQueryClient()

  const [isLoading, setIsLoading] = useState(false)
  const [results, setResults] = useState<KeyTestOutcome[]>([])
  const [selectedIndices, setSelectedIndices] = useState<Set<number>>(new Set())
  const [confirmDeleteType, setConfirmDeleteType] = useState<
    'failed' | 'selected' | null
  >(null)
  const [isDeleting, setIsDeleting] = useState(false)

  const runTests = useCallback(async () => {
    if (!currentRow?.id) return
    setIsLoading(true)
    setSelectedIndices(new Set())
    try {
      const res = await testAllKeysOfChannel(currentRow.id)
      if (res.success && Array.isArray(res.data)) {
        setResults(res.data)
      } else {
        toast.error(res.message || t('Failed to test keys'))
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t('Failed to test keys'))
    } finally {
      setIsLoading(false)
    }
  }, [currentRow?.id, t])

  useEffect(() => {
    if (open && currentRow) {
      void runTests()
    } else {
      setResults([])
      setSelectedIndices(new Set())
    }
  }, [open, currentRow, runTests])

  const workingCount = useMemo(
    () => results.filter((r) => r.status === 'success').length,
    [results]
  )
  const failingCount = useMemo(
    () => results.filter((r) => r.status === 'error').length,
    [results]
  )
  const failedIndices = useMemo(
    () => results.filter((r) => r.status === 'error').map((r) => r.index),
    [results]
  )

  const allSelected = useMemo(
    () => results.length > 0 && selectedIndices.size === results.length,
    [results.length, selectedIndices.size]
  )

  const toggleSelectAll = () => {
    if (allSelected) {
      setSelectedIndices(new Set())
    } else {
      setSelectedIndices(new Set(results.map((r) => r.index)))
    }
  }

  const toggleSelectKey = (index: number) => {
    const next = new Set(selectedIndices)
    if (next.has(index)) {
      next.delete(index)
    } else {
      next.add(index)
    }
    setSelectedIndices(next)
  }

  const handleDelete = async () => {
    if (!currentRow?.id || !confirmDeleteType) return
    const indicesToDelete =
      confirmDeleteType === 'failed'
        ? failedIndices
        : Array.from(selectedIndices)

    if (indicesToDelete.length === 0) {
      setConfirmDeleteType(null)
      return
    }

    setIsDeleting(true)
    try {
      const res = await deleteMultiKeys(currentRow.id, indicesToDelete)
      if (res.success) {
        toast.success(res.message || t('Keys deleted successfully'))
        queryClient.invalidateQueries({ queryKey: channelsQueryKeys.lists() })
        setConfirmDeleteType(null)
        // Refresh tests after deletion
        void runTests()
      } else {
        toast.error(res.message || t('Failed to delete keys'))
      }
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : t('Failed to delete keys')
      )
    } finally {
      setIsDeleting(false)
    }
  }

  return (
    <>
      <Dialog
        open={open}
        onOpenChange={onOpenChange}
        title={
          <div className='flex items-center gap-2'>
            <KeyRound className='size-5 text-primary' />
            <span>{t('Test Channel Keys')}</span>
            {currentRow && (
              <span className='font-normal text-muted-foreground text-sm truncate max-w-[280px]'>
                - {currentRow.name}
              </span>
            )}
          </div>
        }
        description={t(
          'Runs a parallel connectivity test for every key in this multi-key channel to detect dead or expired keys.'
        )}
        contentClassName='max-w-3xl sm:max-w-3xl max-h-[90vh] flex flex-col p-6'
      >
        <div className='flex flex-col gap-4 py-2 flex-1 min-h-0'>
          {/* Summary Stats */}
          <div className='grid grid-cols-3 gap-3'>
            <div className='rounded-xl border border-border/80 bg-card/60 p-3 flex flex-col gap-0.5'>
              <span className='text-[11px] font-medium text-muted-foreground'>
                {t('Total Keys')}
              </span>
              <span className='text-xl font-bold'>{results.length}</span>
            </div>

            <div className='rounded-xl border border-emerald-500/20 bg-emerald-500/5 p-3 flex flex-col gap-0.5'>
              <div className='flex items-center gap-1.5 text-emerald-600 dark:text-emerald-400'>
                <CheckCircle2 className='size-3.5' />
                <span className='text-[11px] font-medium'>{t('Working')}</span>
              </div>
              <span className='text-xl font-bold text-emerald-600 dark:text-emerald-400'>
                {workingCount}
              </span>
            </div>

            <div className='rounded-xl border border-destructive/20 bg-destructive/5 p-3 flex flex-col gap-0.5'>
              <div className='flex items-center gap-1.5 text-destructive'>
                <XCircle className='size-3.5' />
                <span className='text-[11px] font-medium'>{t('Failed')}</span>
              </div>
              <span className='text-xl font-bold text-destructive'>
                {failingCount}
              </span>
            </div>
          </div>

          {/* Action Toolbar */}
          <div className='flex items-center justify-between gap-2 border-b border-border/60 pb-2.5'>
            <div className='flex items-center gap-2'>
              <Checkbox
                checked={allSelected}
                onCheckedChange={toggleSelectAll}
                disabled={isLoading || results.length === 0}
                id='select-all-keys'
              />
              <label
                htmlFor='select-all-keys'
                className='text-xs font-medium cursor-pointer select-none text-muted-foreground'
              >
                {t('Select All')}
                {selectedIndices.size > 0 && ` (${selectedIndices.size})`}
              </label>
            </div>

            <div className='flex items-center gap-2'>
              <Button
                variant='outline'
                size='sm'
                onClick={() => void runTests()}
                disabled={isLoading}
                className='h-8 text-xs gap-1.5'
              >
                <RefreshCw
                  className={cn('size-3.5', isLoading && 'animate-spin')}
                />
                {t('Retest All')}
              </Button>

              {failingCount > 0 && (
                <Button
                  variant='destructive'
                  size='sm'
                  onClick={() => setConfirmDeleteType('failed')}
                  disabled={isLoading || isDeleting}
                  className='h-8 text-xs gap-1.5'
                >
                  <Trash2 className='size-3.5' />
                  {t('Delete All Failed ({{count}})', { count: failingCount })}
                </Button>
              )}

              {selectedIndices.size > 0 && (
                <Button
                  variant='outline'
                  size='sm'
                  onClick={() => setConfirmDeleteType('selected')}
                  disabled={isLoading || isDeleting}
                  className='h-8 text-xs gap-1.5 text-destructive border-destructive/40 hover:bg-destructive/10'
                >
                  <Trash2 className='size-3.5' />
                  {t('Delete Selected ({{count}})', {
                    count: selectedIndices.size,
                  })}
                </Button>
              )}
            </div>
          </div>

          {/* Results List */}
          <div className='flex-1 min-h-[280px] max-h-[420px] rounded-xl border border-border/70 overflow-hidden bg-background/50 flex flex-col'>
            {isLoading ? (
              <div className='flex-1 flex flex-col items-center justify-center p-8 gap-3 text-muted-foreground'>
                <Loader2 className='size-8 animate-spin text-primary' />
                <p className='text-xs font-medium'>
                  {t('Testing keys in parallel...')}
                </p>
              </div>
            ) : results.length === 0 ? (
              <div className='flex-1 flex flex-col items-center justify-center p-8 text-center text-muted-foreground gap-2'>
                <KeyRound className='size-8 opacity-30' />
                <p className='text-xs'>{t('No keys found in this channel.')}</p>
              </div>
            ) : (
              <ScrollArea className='flex-1'>
                <div className='divide-y divide-border/50'>
                  {results.map((item) => {
                    const isSuccess = item.status === 'success'
                    const isChecked = selectedIndices.has(item.index)
                    return (
                      <div
                        key={item.index}
                        onClick={() => toggleSelectKey(item.index)}
                        className={cn(
                          'p-3 flex items-start gap-3 hover:bg-muted/40 transition-colors cursor-pointer text-xs',
                          isChecked && 'bg-primary/5',
                          !isSuccess && 'bg-destructive/[0.02]'
                        )}
                      >
                        <Checkbox
                          checked={isChecked}
                          onCheckedChange={() => toggleSelectKey(item.index)}
                          onClick={(e) => e.stopPropagation()}
                          className='mt-0.5'
                        />

                        <span className='font-mono text-muted-foreground font-semibold min-w-[28px] text-[11px]'>
                          #{item.index + 1}
                        </span>

                        <div className='flex-1 min-w-0 space-y-1'>
                          <div className='flex items-center gap-2 flex-wrap'>
                            <span className='font-mono font-medium truncate max-w-[360px] text-[12px]'>
                              {item.key}
                            </span>

                            {isSuccess ? (
                              <span className='inline-flex items-center gap-1 px-1.5 py-0.5 rounded-full text-[10px] font-medium bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'>
                                <CheckCircle2 className='size-3' />
                                {t('Working')}
                              </span>
                            ) : (
                              <span className='inline-flex items-center gap-1 px-1.5 py-0.5 rounded-full text-[10px] font-medium bg-destructive/10 text-destructive'>
                                <XCircle className='size-3' />
                                {t('Failed')}
                              </span>
                            )}

                            {item.latency_ms > 0 && (
                              <span className='text-[10px] font-mono text-muted-foreground px-1.5 py-0.2 rounded bg-muted/60'>
                                {item.latency_ms} ms
                              </span>
                            )}
                          </div>

                          {!isSuccess && item.error && (
                            <p className='text-[11px] text-destructive/90 break-words font-mono bg-destructive/5 p-1.5 rounded-md border border-destructive/20'>
                              {item.error}
                            </p>
                          )}
                        </div>
                      </div>
                    )
                  })}
                </div>
              </ScrollArea>
            )}
          </div>
        </div>
      </Dialog>

      {/* Confirmation Dialog for Deletions */}
      <ConfirmDialog
        open={confirmDeleteType !== null}
        onOpenChange={(open) => !open && setConfirmDeleteType(null)}
        destructive
        isLoading={isDeleting}
        title={
          <div className='flex items-center gap-2 text-destructive'>
            <AlertTriangle className='size-5' />
            <span>{t('Confirm Key Deletion')}</span>
          </div>
        }
        desc={
          confirmDeleteType === 'failed'
            ? t(
                'Are you sure you want to permanently delete all {{count}} failing key(s) from this channel? This action cannot be undone.',
                { count: failedIndices.length }
              )
            : t(
                'Are you sure you want to permanently delete {{count}} selected key(s) from this channel? This action cannot be undone.',
                { count: selectedIndices.size }
              )
        }
        handleConfirm={handleDelete}
      />
    </>
  )
}
