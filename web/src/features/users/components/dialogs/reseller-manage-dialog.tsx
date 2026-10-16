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
import { Loader2, Store, Globe } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { api } from '@/lib/api'

import type { User } from '../../types'

interface ResellerManageDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  user: User
}

interface ResellerConfigData {
  id: number
  user_id: number
  child_panel_enabled: boolean
  custom_domain: string
  site_name: string
  logo_url: string
  favicon_url: string
  seo_title: string
  seo_description: string
  seo_keywords: string
}

export function ResellerManageDialog({
  open,
  onOpenChange,
  user,
}: ResellerManageDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [childPanelEnabled, setChildPanelEnabled] = useState(false)

  const { data, isLoading } = useQuery({
    queryKey: ['admin-reseller-config', user.id],
    queryFn: async () => {
      const res = await api.get(`/api/admin/reseller/${user.id}`)
      if (!res.data?.success) {
        throw new Error(res.data?.message || 'Failed to fetch reseller config')
      }
      return res.data.data as ResellerConfigData
    },
    enabled: open,
  })

  useEffect(() => {
    if (data) {
      setChildPanelEnabled(Boolean(data.child_panel_enabled))
    }
  }, [data])

  const updateMutation = useMutation({
    mutationFn: async (enabled: boolean) => {
      const res = await api.put(`/api/admin/reseller/${user.id}`, {
        child_panel_enabled: enabled,
      })
      if (!res.data?.success) {
        throw new Error(res.data?.message || 'Failed to update reseller settings')
      }
      return res.data.data
    },
    onSuccess: () => {
      toast.success(t('Reseller settings updated successfully'))
      queryClient.invalidateQueries({ queryKey: ['admin-reseller-config', user.id] })
      onOpenChange(false)
    },
    onError: (err: any) => {
      toast.error(err.message || t('Failed to update reseller settings'))
    },
  })

  const handleSave = () => {
    updateMutation.mutate(childPanelEnabled)
  }

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={t('Manage Reseller')}
      description={t('Configure reseller permissions and features for {{username}}', {
        username: user.username,
      })}
    >
      <div className='space-y-6 py-2'>
        {isLoading ? (
          <div className='flex items-center justify-center py-8'>
            <Loader2 className='h-6 w-6 animate-spin text-muted-foreground' />
          </div>
        ) : (
          <div className='space-y-4'>
            <div className='flex items-center justify-between rounded-lg border p-4 shadow-sm'>
              <div className='space-y-1'>
                <div className='flex items-center gap-2'>
                  <Store className='h-4 w-4 text-primary' />
                  <Label htmlFor='child-panel-toggle' className='text-sm font-semibold'>
                    {t('Child Panel Feature')}
                  </Label>
                </div>
                <p className='text-xs text-muted-foreground'>
                  {t(
                    'Allows this reseller to host their own white-label portal on a custom domain with independent branding.'
                  )}
                </p>
              </div>
              <Switch
                id='child-panel-toggle'
                checked={childPanelEnabled}
                onCheckedChange={setChildPanelEnabled}
                disabled={updateMutation.isPending}
              />
            </div>

            {data?.custom_domain && (
              <div className='rounded-md bg-muted/50 p-3 text-xs'>
                <div className='flex items-center gap-1.5 font-medium text-muted-foreground'>
                  <Globe className='h-3.5 w-3.5' />
                  <span>{t('Configured Custom Domain')}:</span>
                </div>
                <p className='mt-1 font-mono text-foreground'>{data.custom_domain}</p>
              </div>
            )}
          </div>
        )}

        <div className='flex justify-end gap-2 pt-2'>
          <Button
            variant='outline'
            onClick={() => onOpenChange(false)}
            disabled={updateMutation.isPending}
          >
            {t('Cancel')}
          </Button>
          <Button
            onClick={handleSave}
            disabled={isLoading || updateMutation.isPending}
          >
            {updateMutation.isPending && (
              <Loader2 className='mr-2 h-4 w-4 animate-spin' />
            )}
            {t('Save changes')}
          </Button>
        </div>
      </div>
    </Dialog>
  )
}
