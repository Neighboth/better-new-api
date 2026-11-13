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
import { type Table } from '@tanstack/react-table'
import { Merge, Power, PowerOff, SlidersHorizontal, Tag, Trash2 } from 'lucide-react'
import { toast } from 'sonner'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { DataTableBulkActions as BulkActionsToolbar } from '@/components/data-table'
import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import {
  ADMIN_PERMISSION_ACTIONS,
  ADMIN_PERMISSION_RESOURCES,
  hasPermission,
} from '@/lib/admin-permissions'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'

import { BatchEditChannelsDialog } from './batch-edit-channels-dialog'
import { mergeChannels } from '../api'
import {
  handleBatchDelete,
  handleBatchDisable,
  handleBatchEnable,
  handleBatchSetTag,
} from '../lib'
import type { Channel } from '../types'

interface DataTableBulkActionsProps<TData> {
  table: Table<TData>
}

export function DataTableBulkActions<TData>({
  table,
}: DataTableBulkActionsProps<TData>) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [showTagDialog, setShowTagDialog] = useState(false)
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)
  const [showBatchEditDialog, setShowBatchEditDialog] = useState(false)
  const [showMergeConfirm, setShowMergeConfirm] = useState(false)
  const [tagValue, setTagValue] = useState('')
  const [mergeBusy, setMergeBusy] = useState(false)
  const [mergeDiffs, setMergeDiffs] = useState<string[] | null>(null)
  const [mergePreviewLoading, setMergePreviewLoading] = useState(false)

  const currentUser = useAuthStore((s) => s.auth.user)
  const canEditSensitive = hasPermission(
    currentUser,
    ADMIN_PERMISSION_RESOURCES.CHANNEL,
    ADMIN_PERMISSION_ACTIONS.SENSITIVE_WRITE
  )

  const selectedRows = table.getFilteredSelectedRowModel().rows
  const selectedIds = selectedRows.reduce<number[]>((ids, row) => {
    const id = (row.original as Channel).id

    if (typeof id === 'number') {
      ids.push(id)
    }

    return ids
  }, [])

  const handleClearSelection = () => {
    table.resetRowSelection()
  }

  const handleEnableAll = () => {
    handleBatchEnable(selectedIds, queryClient, handleClearSelection)
  }

  const handleDisableAll = () => {
    handleBatchDisable(selectedIds, queryClient, handleClearSelection)
  }

  const handleDeleteAll = () => {
    if (!canEditSensitive) return
    handleBatchDelete(selectedIds, queryClient, () => {
      setShowDeleteConfirm(false)
      handleClearSelection()
    })
  }

  const handleSetTag = () => {
    handleBatchSetTag(selectedIds, tagValue || null, queryClient, () => {
      setShowTagDialog(false)
      setTagValue('')
      handleClearSelection()
    })
  }

  // Dry-run preview: as soon as the merge dialog opens, ask the server to
  // compare the selected channels without merging. Differences appear inside
  // the dialog before the administrator commits to anything.
  useEffect(() => {
    if (!showMergeConfirm) return
    if (selectedIds.length < 2) {
      setMergeDiffs(null)
      return
    }
    let cancelled = false
    setMergePreviewLoading(true)
    setMergeDiffs(null)
    void mergeChannels({ ids: selectedIds, dryRun: true })
      .then((response) => {
        if (cancelled) return
        if (!response.success) {
          // The backend refused even the dry run (e.g. a multi-key channel
          // is selected); show the message as a single differing row so the
          // user learns about it inside the dialog.
          setMergeDiffs([response.message ?? t('Could not merge the channels')])
          return
        }
        const data = response.data ?? {}
        setMergeDiffs(Array.isArray(data.differing_fields) ? (data.differing_fields as string[]) : [])
      })
      .catch(() => {
        if (!cancelled) setMergeDiffs([t('Could not compare the channels')])
      })
      .finally(() => {
        if (!cancelled) setMergePreviewLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [showMergeConfirm, selectedIds.join(',')])

  const handleMergeAll = async () => {
    if (mergeBusy || selectedIds.length < 2) return
    setMergeBusy(true)
    try {
      const response = await mergeChannels({ ids: selectedIds })
      if (!response.success) {
        toast.error(response.message || t('Could not merge the channels'))
        return
      }
      toast.success(
        t('Channels merged into one multi-key channel {{name}}.', {
          name: response.data?.name ?? '',
        })
      )
      setShowMergeConfirm(false)
      setMergeDiffs(null)
      queryClient.invalidateQueries({ queryKey: ['channels'] })
      handleClearSelection()
    } catch {
      toast.error(t('Could not merge the channels'))
    } finally {
      setMergeBusy(false)
    }
  }

  return (
    <>
      <BulkActionsToolbar table={table} entityName='channel'>
        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant='outline'
                size='icon'
                onClick={handleEnableAll}
                className='size-8'
                aria-label={t('Enable selected channels')}
                title={t('Enable selected channels')}
              />
            }
          >
            <Power />
            <span className='sr-only'>{t('Enable selected channels')}</span>
          </TooltipTrigger>
          <TooltipContent>
            <p>{t('Enable selected channels')}</p>
          </TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant='outline'
                size='icon'
                onClick={handleDisableAll}
                className='size-8'
                aria-label={t('Disable selected channels')}
                title={t('Disable selected channels')}
              />
            }
          >
            <PowerOff />
            <span className='sr-only'>{t('Disable selected channels')}</span>
          </TooltipTrigger>
          <TooltipContent>
            <p>{t('Disable selected channels')}</p>
          </TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant='outline'
                size='icon'
                onClick={() => setShowTagDialog(true)}
                className='size-8'
                aria-label={t('Set tag for selected channels')}
                title={t('Set tag for selected channels')}
              />
            }
          >
            <Tag />
            <span className='sr-only'>
              {t('Set tag for selected channels')}
            </span>
          </TooltipTrigger>
          <TooltipContent>
            <p>{t('Set tag for selected channels')}</p>
          </TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant='outline'
                size='icon'
                onClick={() => setShowBatchEditDialog(true)}
                className='size-8'
                aria-label={t('Batch Edit Channels')}
                title={t('Batch Edit Channels')}
              />
            }
          >
            <SlidersHorizontal />
            <span className='sr-only'>{t('Batch Edit Channels')}</span>
          </TooltipTrigger>
          <TooltipContent>
            <p>{t('Batch Edit Channels (Auto Ban, Status Mapping, Ratio, Groups...)')}</p>
          </TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant='outline'
                size='icon'
                onClick={() => setShowMergeConfirm(true)}
                disabled={selectedIds.length < 2}
                className={cn('size-8', selectedIds.length < 2 && 'cursor-not-allowed opacity-50')}
                aria-label={t('Merge channels')}
                title={t('Merge channels')}
              />
            }
          >
            <Merge />
            <span className='sr-only'>{t('Merge channels')}</span>
          </TooltipTrigger>
          <TooltipContent>
            <p>
              {t('Merge identical channels into one multi-key channel')}
            </p>
          </TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant='destructive'
                size='icon'
                onClick={() => {
                  if (!canEditSensitive) return
                  setShowDeleteConfirm(true)
                }}
                aria-disabled={!canEditSensitive}
                className={cn(
                  'size-8',
                  !canEditSensitive && 'cursor-not-allowed opacity-50'
                )}
                aria-label={t('Delete selected channels')}
                title={
                  canEditSensitive
                    ? t('Delete selected channels')
                    : t('No permission to perform this action')
                }
              />
            }
          >
            <Trash2 />
            <span className='sr-only'>{t('Delete selected channels')}</span>
          </TooltipTrigger>
          <TooltipContent>
            <p>
              {canEditSensitive
                ? t('Delete selected channels')
                : t('No permission to perform this action')}
            </p>
          </TooltipContent>
        </Tooltip>
      </BulkActionsToolbar>

      {/* Set Tag Dialog */}
      <Dialog
        open={showTagDialog}
        onOpenChange={setShowTagDialog}
        title={t('Set Tag')}
        description={
          <>
            {t('Set a tag for')}
            {selectedIds.length}{' '}
            {t('selected channel(s). Leave empty to remove tag.')}
          </>
        }
        contentHeight='auto'
        bodyClassName='space-y-4'
        footer={
          <>
            <Button
              variant='outline'
              onClick={() => {
                setShowTagDialog(false)
                setTagValue('')
              }}
            >
              {t('Cancel')}
            </Button>
            <Button onClick={handleSetTag}>{t('Set Tag')}</Button>
          </>
        }
      >
        <div className='grid gap-4 py-4'>
          <div className='grid gap-2'>
            <Label htmlFor='tag'>{t('Tag')}</Label>
            <Input
              id='tag'
              placeholder={t('Enter tag name (optional)')}
              value={tagValue}
              onChange={(e) => setTagValue(e.target.value)}
            />
          </div>
        </div>
      </Dialog>

      {/* Merge Confirmation Dialog */}
      <Dialog
        open={showMergeConfirm}
        onOpenChange={(open) => {
          if (!mergeBusy) {
            setShowMergeConfirm(open)
            if (!open) setMergeDiffs(null)
          }
        }}
        title={t('Merge channels into one?')}
        description={t('The selected channels must have identical settings except their API key.')}
        contentHeight='auto'
        footer={
          <>
            <Button
              variant='outline'
              onClick={() => {
                setShowMergeConfirm(false)
                setMergeDiffs(null)
              }}
              disabled={mergeBusy}
            >
              {t('Cancel')}
            </Button>
            <Button onClick={handleMergeAll} disabled={mergeBusy || (mergeDiffs !== null && mergeDiffs.length > 0)}>
              {mergeBusy ? t('Merging…') : mergeDiffs !== null && mergeDiffs.length > 0 ? t('Cannot merge — fix the differences') : t('Merge')}
            </Button>
          </>
        }
      >
        <div className='grid gap-3 py-2'>
          <ul className='max-h-36 space-y-1 overflow-y-auto text-sm'>
            {selectedRows.slice(0, 10).map((row) => {
              const channel = row.original as Channel
              return (
                <li
                  key={channel.id}
                  className='flex items-center justify-between gap-3 rounded-md border px-2 py-1'
                >
                  <span className='truncate'>{channel.name}</span>
                  <span className='text-muted-foreground shrink-0 text-xs'>
                    #{channel.id}
                  </span>
                </li>
              )
            })}
            {selectedRows.length > 10 && (
              <li className='text-muted-foreground text-xs'>
                {t('+{{count}} more', { count: selectedRows.length - 10 })}
              </li>
            )}
          </ul>

          {mergePreviewLoading && (
            <p className='text-muted-foreground text-sm'>{t('Checking the channels…')}</p>
          )}
          {mergeDiffs !== null && mergeDiffs.length > 0 && (
            <div className='rounded-md border border-destructive/40 bg-destructive/5 p-3 text-sm'>
              <p className='font-semibold text-destructive'>{t('These settings differ:')}</p>
              <ul className='mt-1 max-h-36 list-disc space-y-0.5 overflow-y-auto pe-2'>
                {mergeDiffs.map((label) => (
                  <li key={label}>{label}</li>
                ))}
              </ul>
            </div>
          )}
          {mergeDiffs !== null && mergeDiffs.length === 0 && (
            <div className='rounded-md border border-emerald-500/40 bg-emerald-500/5 p-3 text-sm'>
              <p className='font-semibold text-emerald-700 dark:text-emerald-400'>{t('All settings are identical (only the API keys differ).')}</p>
              <p className='mt-1 text-muted-foreground'>
                {t('The merged channel becomes one multi-key channel (random mode). The original channels will be deleted.')}
              </p>
            </div>
          )}
        </div>
      </Dialog>

      {/* Batch Edit Channels Dialog */}
      <Dialog
        open={showBatchEditDialog}
        onOpenChange={setShowBatchEditDialog}
        title={t('Batch Edit Channels')}
        description={t('Update several channels at once.')}
        contentHeight='auto'
        bodyClassName='max-h-[72vh] overflow-y-auto pr-1'
        footer={null}
      >
        <BatchEditChannelsDialog
          channels={selectedRows.map((row) => row.original as Channel)}
          onClose={() => setShowBatchEditDialog(false)}
          onSaved={() => {
            queryClient.invalidateQueries({ queryKey: ['channels'] })
            handleClearSelection()
          }}
        />
      </Dialog>

      {/* Delete Confirmation Dialog */}
      <Dialog
        open={showDeleteConfirm}
        onOpenChange={setShowDeleteConfirm}
        title={t('Delete Channels?')}
        description={
          <>
            {t('Are you sure you want to delete')}
            {selectedIds.length}{' '}
            {t('channel(s)? This action cannot be undone.')}
          </>
        }
        contentHeight='auto'
        footer={
          <>
            <Button
              variant='outline'
              onClick={() => setShowDeleteConfirm(false)}
            >
              {t('Cancel')}
            </Button>
            <Button
              variant='destructive'
              onClick={handleDeleteAll}
              disabled={!canEditSensitive}
            >
              {t('Delete')}
            </Button>
          </>
        }
      >
        {' '}
      </Dialog>
    </>
  )
}
