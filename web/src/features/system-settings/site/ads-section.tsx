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
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery } from '@tanstack/react-query'
import { Download, Plus, Trash2, Upload } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
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
import { api } from '@/lib/api'

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

type CustomAdDraft = { id: string; image: string; url: string }

const adsSchema = z.object({
  AdsEnabled: z.boolean(),
  AdsMode: z.enum(['adsense', 'custom', 'both']),
  AdSenseClientId: z.string().optional(),
  AdSenseSlotId: z.string().optional(),
  CustomAds: z.string(),
})

type AdsFormValues = z.infer<typeof adsSchema>

type AdsSectionProps = {
  defaultValues: AdsFormValues
}

function parseCustomAds(raw: string): CustomAdDraft[] {
  try {
    const parsed = JSON.parse(raw) as CustomAdDraft[]
    if (!Array.isArray(parsed)) return []
    return parsed
      .filter((ad) => ad && typeof ad.id === 'string')
      .map((ad) => ({
        id: ad.id,
        image: typeof ad.image === 'string' ? ad.image : '',
        url: typeof ad.url === 'string' ? ad.url : '',
      }))
  } catch {
    return []
  }
}

const AD_ID_PATTERN = /^[a-z0-9][a-z0-9-]{1,63}$/

export function AdsSection({ defaultValues }: AdsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const fileInputRef = useRef<HTMLInputElement>(null)
  const [uploadingIndex, setUploadingIndex] = useState<number | null>(null)

  const form = useForm<AdsFormValues>({
    resolver: zodResolver(adsSchema),
    defaultValues,
  })

  const [customAds, setCustomAds] = useState<CustomAdDraft[]>(() =>
    parseCustomAds(defaultValues.CustomAds)
  )

  useEffect(() => {
    form.reset(defaultValues)
    setCustomAds(parseCustomAds(defaultValues.CustomAds))
  }, [defaultValues, form])

  const adsEnabled = form.watch('AdsEnabled')
  const adsMode = form.watch('AdsMode')

  const statsQuery = useQuery({
    queryKey: ['admin-ad-stats'],
    queryFn: async () => {
      const res = await api.get('/api/blog/manage/ads/stats')
      if (!res.data?.success) {
        throw new Error(res.data?.message || 'failed')
      }
      return res.data.data as {
        items: {
          ad_id: string
          is_adsense: boolean
          impressions: number
          unique_ips: number
        }[]
        total: number
      }
    },
  })

  const onSubmit = async (data: AdsFormValues) => {
    for (const ad of customAds) {
      if (!AD_ID_PATTERN.test(ad.id)) {
        toast.error(t('Ad ID must be lowercase letters, digits and dashes'))
        return
      }
      if (!ad.image.trim() || !ad.url.trim()) {
        toast.error(t('Every ad needs an image URL and a target URL'))
        return
      }
    }
    const ids = new Set<string>()
    for (const ad of customAds) {
      if (ids.has(ad.id)) {
        toast.error(t('Ad IDs must be unique'))
        return
      }
      ids.add(ad.id)
    }

    const customAdsJson = JSON.stringify(customAds)
    const entries: [keyof AdsFormValues, string][] = [
      ['AdsEnabled', String(data.AdsEnabled)],
      ['AdsMode', data.AdsMode],
      ['AdSenseClientId', data.AdSenseClientId ?? ''],
      ['AdSenseSlotId', data.AdSenseSlotId ?? ''],
      ['CustomAds', customAdsJson],
    ]
    const defaults = { ...defaultValues, CustomAds: defaultValues.CustomAds }
    for (const [key, value] of entries) {
      if (String(defaults[key] ?? '') !== value) {
        await updateOption.mutateAsync({ key, value })
      }
    }
    toast.success(t('Settings saved'))
  }

  const addCustomAd = () => {
    setCustomAds((current) => [
      ...current,
      { id: `sponsor-ad-${current.length + 1}`, image: '', url: '' },
    ])
  }

  const updateCustomAd = (
    index: number,
    field: keyof CustomAdDraft,
    value: string
  ) => {
    setCustomAds((current) =>
      current.map((ad, i) => (i === index ? { ...ad, [field]: value } : ad))
    )
  }

  const removeCustomAd = (index: number) => {
    setCustomAds((current) => current.filter((_, i) => i !== index))
  }

  const handleFileUpload = async (index: number, file: File) => {
    const formData = new FormData()
    formData.append('file', file)
    setUploadingIndex(index)
    try {
      const res = await api.post('/api/blog/manage/ads/upload', formData, {
        headers: { 'Content-Type': 'multipart/form-data' },
      })
      if (res.data?.success && res.data?.data) {
        updateCustomAd(index, 'image', res.data.data)
        toast.success(t('Image uploaded successfully'))
      } else {
        toast.error(res.data?.message || t('Failed to upload image'))
      }
    } catch {
      toast.error(t('Failed to upload image'))
    } finally {
      setUploadingIndex(null)
    }
  }

  const showAdsenseFields = adsMode === 'adsense' || adsMode === 'both'
  const showCustomAds = adsMode === 'custom' || adsMode === 'both'
  const stats = statsQuery.data

  return (
    <SettingsSection title={t('Ads')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)} autoComplete='off'>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
          />

          <FormField
            control={form.control}
            name='AdsEnabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable ads')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Show ad banners at the top and bottom of every blog post. Two slots above, two below.'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <Switch
                  checked={field.value}
                  onCheckedChange={field.onChange}
                />
              </SettingsSwitchItem>
            )}
          />

          {adsEnabled && (
            <>
              <FormField
                control={form.control}
                name='AdsMode'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Ad source')}</FormLabel>
                    <Select value={field.value} onValueChange={field.onChange}>
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent alignItemWithTrigger={false}>
                        <SelectItem value='adsense'>
                          {t('Google AdSense only')}
                        </SelectItem>
                        <SelectItem value='custom'>
                          {t('Custom image ads only')}
                        </SelectItem>
                        <SelectItem value='both'>{t('Both')}</SelectItem>
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />

              {showAdsenseFields && (
                <>
                  <FormField
                    control={form.control}
                    name='AdSenseClientId'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>
                          {t('AdSense publisher ID (ca-pub-...)')}
                        </FormLabel>
                        <FormControl>
                          <Input
                            placeholder='ca-pub-xxxxxxxxxxxxxxxx'
                            autoComplete='off'
                            {...field}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='AdSenseSlotId'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('AdSense ad slot ID')}</FormLabel>
                        <FormControl>
                          <Input placeholder='1234567890' {...field} />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </>
              )}

              {showCustomAds && (
                <div className='space-y-3'>
                  <div className='flex items-center justify-between'>
                    <Label className='text-base'>{t('Custom image ads')}</Label>
                    <Button
                      type='button'
                      size='sm'
                      variant='outline'
                      onClick={addCustomAd}
                    >
                      <Plus className='me-1 h-4 w-4' />
                      {t('Add ad')}
                    </Button>
                  </div>
                  <FormDescription>
                    {t(
                      'Each ad needs an image URL, a target URL and an ID. The image is stretched to exactly fill the banner slot, and clicks open the URL in a new tab. The ID is only visible here and in impression stats.'
                    )}
                  </FormDescription>
                  {customAds.length === 0 && (
                    <p className='text-muted-foreground text-sm'>
                      {t('No custom ads yet.')}
                    </p>
                  )}
                  {customAds.map((ad, index) => (
                    <Card key={ad.id || `ad-slot-${index}`} className='py-3'>
                      <CardContent className='grid gap-3 px-3'>
                        <div className='flex items-center justify-between gap-2'>
                          <div className='flex-1'>
                            <Label className='text-xs'>
                              {t('Ad ID (internal)')}
                            </Label>
                            <Input
                              value={ad.id}
                              onChange={(event) =>
                                updateCustomAd(index, 'id', event.target.value)
                              }
                              placeholder='sponsor-ad-1'
                            />
                          </div>
                          <Button
                            type='button'
                            variant='ghost'
                            size='icon'
                            className='mt-4'
                            onClick={() => removeCustomAd(index)}
                            aria-label={t('Delete')}
                          >
                            <Trash2 className='h-4 w-4' />
                          </Button>
                        </div>
                        <div>
                          <Label className='text-xs'>{t('Image URL')}</Label>
                          <div className='flex items-center gap-2'>
                            <Input
                              value={ad.image}
                              onChange={(event) =>
                                updateCustomAd(index, 'image', event.target.value)
                              }
                              placeholder='https://example.com/banner.png or upload'
                            />
                            <Button
                              type='button'
                              variant='outline'
                              size='sm'
                              disabled={uploadingIndex === index}
                              onClick={() => {
                                setUploadingIndex(index)
                                fileInputRef.current?.click()
                              }}
                            >
                              <Upload className='me-1 h-4 w-4' />
                              {t('Upload')}
                            </Button>
                          </div>
                        </div>
                        <div>
                          <Label className='text-xs'>{t('Target URL')}</Label>
                          <Input
                            value={ad.url}
                            onChange={(event) =>
                              updateCustomAd(index, 'url', event.target.value)
                            }
                            placeholder='https://example.com'
                          />
                        </div>
                      </CardContent>
                    </Card>
                  ))}
                </div>
              )}

              <div className='space-y-3 border-t pt-4'>
                <div className='flex items-center justify-between'>
                  <Label className='text-base'>{t('Ad impressions')}</Label>
                  <Button
                    type='button'
                    size='sm'
                    variant='outline'
                    onClick={() =>
                      (window.location.href = '/api/blog/manage/ads/impressions.csv')
                    }
                  >
                    <Download className='me-1 h-4 w-4' />
                    {t('Download CSV')}
                  </Button>
                </div>
                {statsQuery.isLoading && (
                  <p className='text-muted-foreground text-sm'>
                    {t('Loading...')}
                  </p>
                )}
                {stats && (
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>{t('Ad')}</TableHead>
                        <TableHead className='text-right'>
                          {t('Impressions')}
                        </TableHead>
                        <TableHead className='text-right'>
                          {t('Unique IPs')}
                        </TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {stats.items.map((row) => (
                        <TableRow key={`${row.is_adsense}-${row.ad_id}`}>
                          <TableCell className='font-mono text-xs'>
                            {row.is_adsense ? 'AdSense' : row.ad_id}
                          </TableCell>
                          <TableCell className='text-right'>
                            {row.impressions}
                          </TableCell>
                          <TableCell className='text-right'>
                            {row.unique_ips}
                          </TableCell>
                        </TableRow>
                      ))}
                      {stats.items.length === 0 && (
                        <TableRow>
                          <TableCell
                            colSpan={3}
                            className='text-muted-foreground text-center'
                          >
                            {t('No impressions yet.')}
                          </TableCell>
                        </TableRow>
                      )}
                    </TableBody>
                  </Table>
                )}
              </div>
            </>
          )}
        </SettingsForm>
      </Form>
      <input
        type='file'
        ref={fileInputRef}
        className='hidden'
        accept='image/*'
        onChange={(e) => {
          const file = e.target.files?.[0]
          if (file && uploadingIndex !== null) {
            void handleFileUpload(uploadingIndex, file)
          }
          e.target.value = ''
        }}
      />
    </SettingsSection>
  )
}
