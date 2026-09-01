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
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Plus, Edit, Trash2, Shield, Loader2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { getLobeIcon } from '@/lib/lobe-icon'

import { deleteVendor, getVendors } from '../../models/api'
import { VendorMutateDialog } from '../../models/components/dialogs/vendor-mutate-dialog'
import { vendorsQueryKeys } from '../../models/lib'
import type { Vendor } from '../../models/types'

export function VendorSettingsSection() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [selectedVendor, setSelectedVendor] = useState<Vendor | null>(null)
  const [deletingId, setDeletingId] = useState<number | null>(null)

  const { data, isLoading } = useQuery({
    queryKey: vendorsQueryKeys.lists(),
    queryFn: () => getVendors({ page_size: 100 }),
  })

  const vendors = data?.data?.items || []

  const handleCreate = () => {
    setSelectedVendor(null)
    setDialogOpen(true)
  }

  const handleEdit = (vendor: Vendor) => {
    setSelectedVendor(vendor)
    setDialogOpen(true)
  }

  const handleDelete = async (vendor: Vendor) => {
    if (vendor.is_builtin) {
      toast.error(t('Built-in vendors cannot be deleted.'))
      return
    }
    if (!window.confirm(t('Are you sure you want to delete vendor {{name}}?', { name: vendor.name }))) {
      return
    }

    setDeletingId(vendor.id)
    try {
      const res = await deleteVendor(vendor.id)
      if (res.success) {
        toast.success(t('Vendor deleted successfully'))
        queryClient.invalidateQueries({ queryKey: vendorsQueryKeys.lists() })
      } else {
        toast.error(res.message || t('Failed to delete vendor'))
      }
    } catch (err: unknown) {
      toast.error((err as Error)?.message || t('Failed to delete vendor'))
    } finally {
      setDeletingId(null)
    }
  }

  return (
    <Card>
      <CardHeader>
        <div className='flex items-center justify-between'>
          <div>
            <CardTitle>{t('Vendor Management')}</CardTitle>
            <CardDescription>
              {t('Manage AI model vendors, logos, and model name keyword filters ("If contains"). Built-in vendors are protected.')}
            </CardDescription>
          </div>
          <Button onClick={handleCreate} size='sm'>
            <Plus className='mr-1.5 h-4 w-4' />
            {t('Add Vendor')}
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <div className='flex h-32 items-center justify-center'>
            <Loader2 className='h-6 w-6 animate-spin text-muted-foreground' />
          </div>
        ) : vendors.length === 0 ? (
          <div className='py-8 text-center text-sm text-muted-foreground'>
            {t('No vendors found.')}
          </div>
        ) : (
          <div className='rounded-md border overflow-x-auto max-h-[70vh] overflow-y-auto'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className='w-[60px]'>{t('Icon')}</TableHead>
                  <TableHead>{t('Name')}</TableHead>
                  <TableHead>{t('If Contains Filters')}</TableHead>
                  <TableHead>{t('Type')}</TableHead>
                  <TableHead className='text-right'>{t('Actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {vendors.map((v) => {
                  const iconEl = v.icon ? (
                    v.icon.startsWith('/') || v.icon.startsWith('http') ? (
                      <img src={v.icon} alt={v.name} className='h-5 w-5 object-contain' />
                    ) : (
                      getLobeIcon(v.icon, 18)
                    )
                  ) : null

                  return (
                    <TableRow key={v.id}>
                      <TableCell>{iconEl}</TableCell>
                      <TableCell className='font-medium'>
                        {v.name}
                        {v.description && (
                          <div className='text-xs text-muted-foreground'>{v.description}</div>
                        )}
                      </TableCell>
                      <TableCell className='max-w-[200px] truncate text-xs text-muted-foreground'>
                        {v.keywords || '-'}
                      </TableCell>
                      <TableCell>
                        {v.is_builtin ? (
                          <Badge variant='secondary' className='gap-1 text-xs'>
                            <Shield className='h-3 w-3' />
                            {t('Built-in')}
                          </Badge>
                        ) : (
                          <Badge variant='outline' className='text-xs'>
                            {t('Custom')}
                          </Badge>
                        )}
                      </TableCell>
                      <TableCell className='text-right'>
                        <div className='flex justify-end gap-1'>
                          <Button
                            variant='ghost'
                            size='icon'
                            className='h-8 w-8'
                            onClick={() => handleEdit(v)}
                            title={t('Edit')}
                          >
                            <Edit className='h-4 w-4' />
                          </Button>
                          <Button
                            variant='ghost'
                            size='icon'
                            className='h-8 w-8 text-destructive hover:text-destructive'
                            onClick={() => handleDelete(v)}
                            disabled={v.is_builtin || deletingId === v.id}
                            title={v.is_builtin ? t('Built-in vendors cannot be deleted') : t('Delete')}
                          >
                            <Trash2 className='h-4 w-4' />
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
          </div>
        )}

        <VendorMutateDialog
          open={dialogOpen}
          onOpenChange={setDialogOpen}
          currentVendor={selectedVendor}
        />
      </CardContent>
    </Card>
  )
}
