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
import { Power, PowerOff, SlidersHorizontal, Tag, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { DataTableBulkActions as BulkActionsToolbar } from '@/components/data-table'
import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
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

import { batchEditChannels } from '../api'
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
  const [batchEditLoading, setBatchEditLoading] = useState(false)
  const [tagValue, setTagValue] = useState('')

  // Batch edit field toggles & values
  const [editAutoBan, setEditAutoBan] = useState(false)
  const [autoBanVal, setAutoBanVal] = useState<'1' | '0'>('1')

  const [editStatusCodeMapping, setEditStatusCodeMapping] = useState(false)
  const [statusCodeMappingVal, setStatusCodeMappingVal] = useState('')

  const [editModelRatio, setEditModelRatio] = useState(false)
  const [modelRatioVal, setModelRatioVal] = useState('')

  const [editModelMapping, setEditModelMapping] = useState(false)
  const [modelMappingVal, setModelMappingVal] = useState('')

  const [editGroups, setEditGroups] = useState(false)
  const [groupsVal, setGroupsVal] = useState('')

  const [editPriority, setEditPriority] = useState(false)
  const [priorityVal, setPriorityVal] = useState('')

  const [editWeight, setEditWeight] = useState(false)
  const [weightVal, setWeightVal] = useState('')
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

  const handleBatchEditSubmit = async () => {
    setBatchEditLoading(true)
    try {
      const payload: Parameters<typeof batchEditChannels>[0] = { ids: selectedIds }
      if (editAutoBan) payload.auto_ban = Number(autoBanVal)
      if (editStatusCodeMapping) payload.status_code_mapping = statusCodeMappingVal
      if (editModelRatio) payload.model_ratio_override = modelRatioVal
      if (editModelMapping) payload.model_mapping = modelMappingVal
      if (editGroups) payload.groups = groupsVal
      if (editPriority && priorityVal.trim()) payload.priority = Number(priorityVal)
      if (editWeight && weightVal.trim()) payload.weight = Number(weightVal)

      const res = await batchEditChannels(payload)
      if (res?.success) {
        toast.success(t('Successfully updated selected channels'))
        queryClient.invalidateQueries({ queryKey: ['channels'] })
        setShowBatchEditDialog(false)
        handleClearSelection()
      } else {
        toast.error(res?.message || t('Failed to update channels'))
      }
    } catch (e: any) {
      toast.error(e?.message || t('Failed to update channels'))
    } finally {
      setBatchEditLoading(false)
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

      {/* Batch Edit Channels Dialog */}
      <Dialog
        open={showBatchEditDialog}
        onOpenChange={setShowBatchEditDialog}
        title={t('Batch Edit Channels')}
        description={
          <>
            {t('Configure properties for {{count}} selected channel(s). Only checked fields will be updated.', {
              count: selectedIds.length,
            })}
          </>
        }
        contentHeight='auto'
        bodyClassName='space-y-4 max-h-[70vh] overflow-y-auto pr-1'
        footer={
          <>
            <Button
              variant='outline'
              onClick={() => setShowBatchEditDialog(false)}
              disabled={batchEditLoading}
            >
              {t('Cancel')}
            </Button>
            <Button onClick={handleBatchEditSubmit} disabled={batchEditLoading}>
              {batchEditLoading ? t('Updating...') : t('Apply Changes')}
            </Button>
          </>
        }
      >
        <div className='space-y-4 py-2'>
          {/* Auto Ban */}
          <div className='rounded-lg border p-3 space-y-2'>
            <div className='flex items-center space-x-2'>
              <Checkbox
                id='batch-autoban'
                checked={editAutoBan}
                onCheckedChange={(c) => setEditAutoBan(Boolean(c))}
              />
              <Label htmlFor='batch-autoban' className='text-xs font-semibold cursor-pointer'>
                {t('Update Auto Ban (Otomatik Yasaklama)')}
              </Label>
            </div>
            {editAutoBan && (
              <div className='pt-1 pl-6'>
                <Select
                  value={autoBanVal}
                  onValueChange={(v) => setAutoBanVal(v as '1' | '0')}
                >
                  <SelectTrigger className='text-xs h-8'>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value='1'>{t('Automatic ban enabled (default)')}</SelectItem>
                    <SelectItem value='0'>{t('Automatic ban disabled')}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            )}
          </div>

          {/* Status Code Mapping */}
          <div className='rounded-lg border p-3 space-y-2'>
            <div className='flex items-center space-x-2'>
              <Checkbox
                id='batch-status-code'
                checked={editStatusCodeMapping}
                onCheckedChange={(c) => setEditStatusCodeMapping(Boolean(c))}
              />
              <Label htmlFor='batch-status-code' className='text-xs font-semibold cursor-pointer'>
                {t('Update Status Code Mapping (Durum Kodu Eşlemesi)')}
              </Label>
            </div>
            {editStatusCodeMapping && (
              <div className='pt-1 pl-6 space-y-1'>
                <Input
                  className='text-xs font-mono'
                  placeholder='e.g. {"403": "500"}'
                  value={statusCodeMappingVal}
                  onChange={(e) => setStatusCodeMappingVal(e.target.value)}
                />
                <p className='text-[10px] text-muted-foreground'>
                  {t('Format: {"original_code": "mapped_code"}')}
                </p>
              </div>
            )}
          </div>

          {/* Model Ratio / Price Multiplier */}
          <div className='rounded-lg border p-3 space-y-2'>
            <div className='flex items-center space-x-2'>
              <Checkbox
                id='batch-model-ratio'
                checked={editModelRatio}
                onCheckedChange={(c) => setEditModelRatio(Boolean(c))}
              />
              <Label htmlFor='batch-model-ratio' className='text-xs font-semibold cursor-pointer'>
                {t('Update Price Multipliers (Fiyat Çarpanı)')}
              </Label>
            </div>
            {editModelRatio && (
              <div className='pt-1 pl-6 space-y-1'>
                <Input
                  className='text-xs font-mono'
                  placeholder='e.g. {"gpt-4o": 0.8}'
                  value={modelRatioVal}
                  onChange={(e) => setModelRatioVal(e.target.value)}
                />
                <p className='text-[10px] text-muted-foreground'>
                  {t('Optional per-model ratio multiplier in JSON format')}
                </p>
              </div>
            )}
          </div>

          {/* Model Mapping */}
          <div className='rounded-lg border p-3 space-y-2'>
            <div className='flex items-center space-x-2'>
              <Checkbox
                id='batch-model-mapping'
                checked={editModelMapping}
                onCheckedChange={(c) => setEditModelMapping(Boolean(c))}
              />
              <Label htmlFor='batch-model-mapping' className='text-xs font-semibold cursor-pointer'>
                {t('Update Model Mapping (Model Eşlemesi)')}
              </Label>
            </div>
            {editModelMapping && (
              <div className='pt-1 pl-6 space-y-1'>
                <Input
                  className='text-xs font-mono'
                  placeholder='e.g. {"gpt-4": "gpt-4o"}'
                  value={modelMappingVal}
                  onChange={(e) => setModelMappingVal(e.target.value)}
                />
                <p className='text-[10px] text-muted-foreground'>
                  {t('Format: {"requested_model": "target_model"}')}
                </p>
              </div>
            )}
          </div>

          {/* Groups */}
          <div className='rounded-lg border p-3 space-y-2'>
            <div className='flex items-center space-x-2'>
              <Checkbox
                id='batch-groups'
                checked={editGroups}
                onCheckedChange={(c) => setEditGroups(Boolean(c))}
              />
              <Label htmlFor='batch-groups' className='text-xs font-semibold cursor-pointer'>
                {t('Update User Groups (Gruplar)')}
              </Label>
            </div>
            {editGroups && (
              <div className='pt-1 pl-6 space-y-1'>
                <Input
                  className='text-xs'
                  placeholder='e.g. default,vip'
                  value={groupsVal}
                  onChange={(e) => setGroupsVal(e.target.value)}
                />
                <p className='text-[10px] text-muted-foreground'>
                  {t('Comma-separated list of user groups allowed to access these channels')}
                </p>
              </div>
            )}
          </div>

          {/* Priority & Weight */}
          <div className='rounded-lg border p-3 space-y-2'>
            <div className='flex items-center space-x-2'>
              <Checkbox
                id='batch-priority'
                checked={editPriority || editWeight}
                onCheckedChange={(c) => {
                  const b = Boolean(c)
                  setEditPriority(b)
                  setEditWeight(b)
                }}
              />
              <Label htmlFor='batch-priority' className='text-xs font-semibold cursor-pointer'>
                {t('Update Priority & Weight (Öncelik ve Ağırlık)')}
              </Label>
            </div>
            {(editPriority || editWeight) && (
              <div className='grid grid-cols-2 gap-3 pt-1 pl-6'>
                <div className='space-y-1'>
                  <Label className='text-xs'>{t('Priority')}</Label>
                  <Input
                    type='number'
                    className='text-xs font-mono'
                    placeholder='0'
                    value={priorityVal}
                    onChange={(e) => setPriorityVal(e.target.value)}
                  />
                </div>
                <div className='space-y-1'>
                  <Label className='text-xs'>{t('Weight')}</Label>
                  <Input
                    type='number'
                    min={0}
                    className='text-xs font-mono'
                    placeholder='0'
                    value={weightVal}
                    onChange={(e) => setWeightVal(e.target.value)}
                  />
                </div>
              </div>
            )}
          </div>
        </div>
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
