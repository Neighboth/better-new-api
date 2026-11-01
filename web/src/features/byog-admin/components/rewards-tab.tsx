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
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Upload, X } from 'lucide-react'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'

import {
  approveAdminClaim,
  createAdminReward,
  deleteAdminReward,
  getAdminClaims,
  getAdminRewards,
  rejectAdminClaim,
  updateAdminReward,
} from '../../byog/api'
import type { ByogRewardClaim, ByogRewardItem } from '../../byog/types'

export function ByogRewardsTab() {
  const { t } = useTranslation()
  const [subTab, setSubTab] = useState<'claims' | 'catalog'>('claims')
  const [rewards, setRewards] = useState<ByogRewardItem[]>([])
  const [claims, setClaims] = useState<ByogRewardClaim[]>([])
  const [loading, setLoading] = useState(true)

  // Item modal
  const [itemModalOpen, setItemModalOpen] = useState(false)
  const [editingItem, setEditingItem] =
    useState<Partial<ByogRewardItem> | null>(null)
  const fileInputRef = useRef<HTMLInputElement | null>(null)

  function handleImageUpload(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file) return
    if (!file.type.startsWith('image/')) {
      toast.error('Lütfen geçerli bir resim dosyası seçin.')
      return
    }
    const reader = new FileReader()
    reader.onload = (event) => {
      const img = new window.Image()
      img.onload = () => {
        // Target 400x250 aspect ratio (16:10)
        const targetWidth = 400
        const targetHeight = 250
        const canvas = document.createElement('canvas')
        canvas.width = targetWidth
        canvas.height = targetHeight
        const ctx = canvas.getContext('2d')
        if (ctx) {
          ctx.drawImage(img, 0, 0, targetWidth, targetHeight)
          const dataUrl = canvas.toDataURL('image/jpeg', 0.88)
          setEditingItem((prev) => (prev ? { ...prev, image_url: dataUrl } : null))
          toast.success(
            t(
              'byog.admin.reward_image_uploaded',
              'Görsel yüklendi ve 400x250 boyutuna optimize edildi.'
            )
          )
        }
      }
      img.src = event.target?.result as string
    }
    reader.readAsDataURL(file)
  }

  // Action status
  const [actioningId, setActioningId] = useState<number | null>(null)

  useEffect(() => {
    loadData()
  }, [])

  async function loadData() {
    try {
      setLoading(true)
      const [rewardData, claimData] = await Promise.all([
        getAdminRewards(),
        getAdminClaims(),
      ])
      setRewards(rewardData || [])
      setClaims(claimData || [])
    } catch (err: any) {
      toast.error(err.message || t('byog.admin.load_rewards_failed'))
    } finally {
      setLoading(false)
    }
  }

  function handleOpenCreateItem() {
    setEditingItem({
      title: '',
      description: '',
      category: 'digital',
      required_tokens: 50000000,
      stock: 10,
      is_active: true,
    })
    setItemModalOpen(true)
  }

  function handleOpenEditItem(item: ByogRewardItem) {
    setEditingItem({ ...item })
    setItemModalOpen(true)
  }

  async function handleSaveItem() {
    if (!editingItem || !editingItem.title) return
    try {
      if (editingItem.id) {
        await updateAdminReward(editingItem.id, editingItem)
        toast.success(t('byog.admin.reward_updated_success'))
      } else {
        await createAdminReward(editingItem)
        toast.success(t('byog.admin.reward_created_success'))
      }
      setItemModalOpen(false)
      loadData()
    } catch (err: any) {
      toast.error(err.message || t('byog.admin.save_reward_failed'))
    }
  }

  async function handleDeleteItem(id: number) {
    if (!window.confirm(t('byog.admin.confirm_delete_reward'))) return
    try {
      await deleteAdminReward(id)
      toast.success(t('byog.admin.reward_deleted_success'))
      loadData()
    } catch (err: any) {
      toast.error(err.message || t('byog.admin.delete_reward_failed'))
    }
  }

  async function handleApproveClaim(id: number) {
    try {
      setActioningId(id)
      await approveAdminClaim(id)
      toast.success(t('byog.admin.claim_approved_success'))
      loadData()
    } catch (err: any) {
      toast.error(err.message || t('byog.admin.action_claim_failed'))
    } finally {
      setActioningId(null)
    }
  }

  async function handleRejectClaim(id: number) {
    if (!window.confirm(t('byog.admin.confirm_reject_claim'))) return
    try {
      setActioningId(id)
      await rejectAdminClaim(id)
      toast.success(t('byog.admin.claim_rejected_success'))
      loadData()
    } catch (err: any) {
      toast.error(err.message || t('byog.admin.action_claim_failed'))
    } finally {
      setActioningId(null)
    }
  }

  function exportClaimsJson() {
    const dataStr =
      'data:text/json;charset=utf-8,' +
      encodeURIComponent(JSON.stringify(claims, null, 2))
    const downloadAnchor = document.createElement('a')
    downloadAnchor.setAttribute('href', dataStr)
    downloadAnchor.setAttribute('download', `byog-claims-${Date.now()}.json`)
    document.body.appendChild(downloadAnchor)
    downloadAnchor.click()
    downloadAnchor.remove()
  }

  function getClaimStatusBadge(status: string) {
    switch (status) {
      case 'approved':
        return (
          <Badge className='border-emerald-500/20 bg-emerald-500/10 text-emerald-500'>
            {t('byog.admin.status_approved')}
          </Badge>
        )
      case 'rejected':
        return (
          <Badge variant='destructive'>
            {t('byog.admin.status_rejected')}
          </Badge>
        )
      default:
        return (
          <Badge
            variant='outline'
            className='border-amber-500 text-amber-500'
          >
            {t('byog.admin.status_pending')}
          </Badge>
        )
    }
  }

  return (
    <div className='space-y-6'>
      <Tabs value={subTab} onValueChange={(val: any) => setSubTab(val)}>
        <div className='flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between'>
          <TabsList>
            <TabsTrigger value='claims'>
              {t('byog.admin.reward_claims')} (
              {claims.filter((c) => c.status === 'pending').length}{' '}
              {t('byog.admin.pending_suffix')})
            </TabsTrigger>
            <TabsTrigger value='catalog'>
              {t('byog.admin.reward_catalog')} ({rewards.length})
            </TabsTrigger>
          </TabsList>

          <div className='flex items-center gap-2'>
            {subTab === 'catalog' ? (
              <Button onClick={handleOpenCreateItem}>
                {t('byog.admin.add_reward_item')}
              </Button>
            ) : (
              <Button
                variant='outline'
                size='sm'
                onClick={exportClaimsJson}
              >
                {t('byog.admin.export_json')}
              </Button>
            )}
            <Button variant='outline' size='sm' onClick={loadData}>
              {t('byog.refresh')}
            </Button>
          </div>
        </div>

        {/* Claims View */}
        <TabsContent value='claims' className='space-y-4 pt-4'>
          <Card>
            <CardHeader>
              <CardTitle className='text-lg'>
                {t('byog.admin.pending_claims_title')}
              </CardTitle>
              <CardDescription>
                {t('byog.admin.pending_claims_desc')}
              </CardDescription>
            </CardHeader>
            <CardContent>
              {loading ? (
                <div className='text-muted-foreground flex h-32 items-center justify-center'>
                  {t('byog.loading')}
                </div>
              ) : claims.length === 0 ? (
                <div className='text-muted-foreground py-12 text-center'>
                  {t('byog.admin.no_claims_found')}
                </div>
              ) : (
                <div className='overflow-x-auto rounded-md border'>
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>{t('byog.admin.claim_id')}</TableHead>
                        <TableHead>{t('byog.admin.user_id')}</TableHead>
                        <TableHead>{t('byog.admin.reward_title')}</TableHead>
                        <TableHead>{t('byog.admin.tokens_burned')}</TableHead>
                        <TableHead>{t('byog.admin.ticket_id')}</TableHead>
                        <TableHead>{t('byog.admin.status')}</TableHead>
                        <TableHead>{t('byog.admin.date')}</TableHead>
                        <TableHead className='text-right'>
                          {t('byog.admin.actions')}
                        </TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {claims.map((claim) => (
                        <TableRow key={claim.id}>
                          <TableCell className='font-mono text-xs'>
                            #{claim.id}
                          </TableCell>
                          <TableCell>
                            <span className='text-xs font-medium'>
                              {claim.user_name || `#${claim.user_id}`}
                            </span>
                          </TableCell>
                          <TableCell className='text-xs font-medium'>
                            {claim.reward_title}
                          </TableCell>
                          <TableCell className='font-mono text-xs font-semibold text-emerald-500'>
                            {claim.tokens_burned.toLocaleString()}
                          </TableCell>
                          <TableCell className='font-mono text-xs'>
                            {claim.ticket_id ? `#${claim.ticket_id}` : '-'}
                          </TableCell>
                          <TableCell>
                            {getClaimStatusBadge(claim.status)}
                          </TableCell>
                          <TableCell className='text-muted-foreground text-xs'>
                            {new Date(
                              claim.created_at * 1000
                            ).toLocaleDateString()}
                          </TableCell>
                          <TableCell className='text-right'>
                            {claim.status === 'pending' ? (
                              <div className='flex items-center justify-end gap-1.5'>
                                <Button
                                  variant='outline'
                                  size='sm'
                                  className='h-7 px-2 text-xs text-emerald-500 hover:bg-emerald-500/10 hover:text-emerald-600'
                                  disabled={actioningId === claim.id}
                                  onClick={() => handleApproveClaim(claim.id)}
                                >
                                  {t('byog.admin.approve')}
                                </Button>
                                <Button
                                  variant='outline'
                                  size='sm'
                                  className='text-destructive hover:text-destructive hover:bg-destructive/10 h-7 px-2 text-xs'
                                  disabled={actioningId === claim.id}
                                  onClick={() => handleRejectClaim(claim.id)}
                                >
                                  {t('byog.admin.reject')}
                                </Button>
                              </div>
                            ) : (
                              <span className='text-muted-foreground text-xs'>
                                -
                              </span>
                            )}
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        {/* Catalog View */}
        <TabsContent value='catalog' className='space-y-4 pt-4'>
          <Card>
            <CardHeader>
              <CardTitle className='text-lg'>
                {t('byog.admin.catalog_title')}
              </CardTitle>
              <CardDescription>{t('byog.admin.catalog_desc')}</CardDescription>
            </CardHeader>
            <CardContent>
              {loading ? (
                <div className='text-muted-foreground flex h-32 items-center justify-center'>
                  {t('byog.loading')}
                </div>
              ) : rewards.length === 0 ? (
                <div className='text-muted-foreground py-12 text-center'>
                  {t('byog.admin.no_items_in_catalog')}
                </div>
              ) : (
                <div className='grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3'>
                  {rewards.map((item) => (
                    <div
                      key={item.id}
                      className='bg-muted/10 group relative space-y-3 rounded-xl border p-4'
                    >
                      <div className='flex items-start justify-between'>
                        <div className='space-y-1'>
                          <h4 className='text-sm font-semibold'>
                            {item.title}
                          </h4>
                          <Badge
                            variant='outline'
                            className='text-[10px] capitalize'
                          >
                            {item.category}
                          </Badge>
                        </div>
                        <div className='flex items-center gap-1.5'>
                          <Button
                            variant='outline'
                            size='sm'
                            className='h-7 px-2 text-xs'
                            onClick={() => handleOpenEditItem(item)}
                          >
                            {t('common.edit')}
                          </Button>
                          <Button
                            variant='outline'
                            size='sm'
                            className='text-destructive hover:text-destructive h-7 px-2 text-xs'
                            onClick={() => handleDeleteItem(item.id)}
                          >
                            {t('common.delete')}
                          </Button>
                        </div>
                      </div>

                      <p className='text-muted-foreground line-clamp-2 text-xs'>
                        {item.description}
                      </p>

                      <div className='flex items-center justify-between border-t pt-2 text-xs'>
                        <div>
                          <span className='text-muted-foreground'>
                            {t('byog.admin.required_tokens')}:
                          </span>{' '}
                          <span className='text-primary font-semibold'>
                            {(item.required_tokens / 1000000).toFixed(0)}M
                          </span>
                        </div>
                        <div>
                          <span className='text-muted-foreground'>
                            {t('byog.admin.stock')}:
                          </span>{' '}
                          <span className='font-mono font-medium'>
                            {item.stock}
                          </span>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>

      {/* Item Dialog */}
      <Dialog open={itemModalOpen} onOpenChange={setItemModalOpen}>
        <DialogContent className='max-w-xl max-h-[88vh] flex flex-col p-6 overflow-hidden'>
          <DialogHeader className='shrink-0 pb-2 border-b'>
            <DialogTitle>
              {editingItem?.id
                ? t('byog.admin.edit_reward_item')
                : t('byog.admin.create_reward_item')}
            </DialogTitle>
            <DialogDescription>
              {t('byog.admin.reward_dialog_desc')}
            </DialogDescription>
          </DialogHeader>

          {editingItem && (
            <div className='flex-1 overflow-y-auto pr-1 space-y-4 py-3 min-h-0'>
              <div className='space-y-2'>
                <Label>{t('byog.admin.item_title')}</Label>
                <Input
                  value={editingItem.title || ''}
                  onChange={(e) =>
                    setEditingItem({ ...editingItem, title: e.target.value })
                  }
                  placeholder='e.g. Steam $20 Gift Card'
                />
              </div>

              <div className='space-y-2'>
                <Label>{t('byog.admin.category')}</Label>
                <Select
                  value={editingItem.category || 'digital'}
                  onValueChange={(val) =>
                    setEditingItem({
                      ...editingItem,
                      category: val || 'digital',
                    })
                  }
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value='digital'>
                      {t('byog.admin.category_digital')}
                    </SelectItem>
                    <SelectItem value='physical'>
                      {t('byog.admin.category_physical')}
                    </SelectItem>
                    <SelectItem value='gift_card'>
                      {t('byog.admin.category_gift_card')}
                    </SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div className='space-y-2'>
                <Label>{t('byog.admin.required_tokens')}</Label>
                <Input
                  type='number'
                  value={editingItem.required_tokens || 0}
                  onChange={(e) =>
                    setEditingItem({
                      ...editingItem,
                      required_tokens: parseInt(e.target.value) || 0,
                    })
                  }
                />
                <span className='text-muted-foreground text-xs'>
                  {((editingItem.required_tokens || 0) / 1000000).toFixed(1)}M
                  Tokens
                </span>
              </div>

              <div className='space-y-2'>
                <Label>{t('byog.admin.stock')}</Label>
                <Input
                  type='number'
                  value={editingItem.stock || 0}
                  onChange={(e) =>
                    setEditingItem({
                      ...editingItem,
                      stock: parseInt(e.target.value) || 0,
                    })
                  }
                />
              </div>

              {/* Reward Image Upload / URL & Guidelines (Item 24 & Upgraded) */}
              <div className='space-y-2'>
                <div className='flex items-center justify-between'>
                  <Label>{t('byog.admin.reward_image', 'Ürün Görseli')}</Label>
                  <span className='text-muted-foreground text-[11px] font-medium'>
                    {t('byog.admin.reward_exact_size', 'Tam Boyut: 400x250 px (16:10)')}
                  </span>
                </div>
                <div className='flex gap-2 items-center'>
                  <Input
                    value={editingItem.image_url || ''}
                    onChange={(e) =>
                      setEditingItem({
                        ...editingItem,
                        image_url: e.target.value,
                      })
                    }
                    placeholder={t(
                      'byog.admin.reward_image_placeholder',
                      'https://.../product-image.png veya dosya yükleyin'
                    )}
                    className='text-xs flex-1'
                  />
                  <input
                    ref={fileInputRef}
                    type='file'
                    accept='image/*'
                    className='hidden'
                    onChange={handleImageUpload}
                  />
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    className='gap-1.5 shrink-0 text-xs'
                    onClick={() => fileInputRef.current?.click()}
                  >
                    <Upload className='size-3.5' />
                    {t('byog.admin.reward_upload_image', 'Görsel Yükle')}
                  </Button>
                </div>
                <p className='text-muted-foreground text-[11px]'>
                  {t(
                    'byog.admin.reward_size_hint',
                    'Önerilen tam boyut 400x250 pikseldir (16:10 en-boy oranı). Dosya yükleyebilir veya doğrudan görsel URL\'si yapıştırabilirsiniz.'
                  )}
                </p>
                {editingItem.image_url ? (
                  <div className='relative mt-2 inline-block'>
                    <div className='relative h-28 w-44 overflow-hidden rounded-lg border bg-muted/40 shadow-xs'>
                      <img
                        src={editingItem.image_url}
                        alt={t('byog.admin.preview', 'Önizleme')}
                        className='h-full w-full object-cover'
                        onError={(e) => {
                          ;(e.target as HTMLElement).style.display = 'none'
                        }}
                      />
                    </div>
                    <Button
                      type='button'
                      variant='destructive'
                      size='icon'
                      className='absolute -top-2 -right-2 size-6 rounded-full shadow-sm'
                      onClick={() => setEditingItem({ ...editingItem, image_url: '' })}
                      title={t('byog.admin.remove_image', 'Görseli Kaldır')}
                    >
                      <X className='size-3' />
                    </Button>
                  </div>
                ) : null}
              </div>

              <div className='space-y-2'>
                <Label>{t('byog.admin.description')}</Label>
                <Textarea
                  value={editingItem.description || ''}
                  onChange={(e) =>
                    setEditingItem({
                      ...editingItem,
                      description: e.target.value,
                    })
                  }
                  placeholder='Item details, delivery terms...'
                />
              </div>

              <div className='flex items-center justify-between rounded-lg border p-3'>
                <Label className='text-sm font-medium'>
                  {t('byog.admin.item_is_active')}
                </Label>
                <Switch
                  checked={editingItem.is_active ?? true}
                  onCheckedChange={(val) =>
                    setEditingItem({ ...editingItem, is_active: val })
                  }
                />
              </div>
            </div>
          )}

          <DialogFooter className='pt-3 border-t shrink-0'>
            <Button variant='outline' onClick={() => setItemModalOpen(false)}>
              {t('byog.cancel')}
            </Button>
            <Button onClick={handleSaveItem}>{t('byog.save')}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
