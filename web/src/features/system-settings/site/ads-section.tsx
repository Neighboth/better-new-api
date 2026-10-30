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
import { Checkbox } from '@/components/ui/checkbox'
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
import {
  AD_SURFACES,
  parsePlacements,
  parseSurfaces,
  serializeSurfaces,
  type AdSurface,
} from '@/features/ads/lib/placements'
import { api } from '@/lib/api'

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

type CustomAdDraft = {
  id: string
  image: string
  url: string
  /**
   * Surfaces of the ad. An ad of an older panel carries no list and belongs to
   * the blog, which is the surface that existed when the ad was written.
   */
  placements: AdSurface[]
}

const adsSchema = z.object({
  AdsEnabled: z.boolean(),
  AdsMode: z.enum(['adsense', 'custom', 'both']),
  AdsSurfaces: z.string(),
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
    const parsed = JSON.parse(raw) as (Partial<CustomAdDraft> & {
      id?: string
    })[]
    if (!Array.isArray(parsed)) return []
    return parsed
      .filter((ad) => ad && typeof ad.id === 'string')
      .map((ad) => ({
        id: ad.id as string,
        image: typeof ad.image === 'string' ? ad.image : '',
        url: typeof ad.url === 'string' ? ad.url : '',
        placements: parsePlacements(
          (ad as { placements?: unknown }).placements
        ),
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
  const surfaces = parseSurfaces(form.watch('AdsSurfaces') ?? 'blog')

  const toggleSurface = (surface: AdSurface, enabled: boolean) => {
    const next = new Set(surfaces)
    if (enabled) next.add(surface)
    else next.delete(surface)
    // At least one surface stays on: the panel of the ads that draws nothing
    // is a switch that an administrator flips without seeing a change.
    if (next.size === 0) next.add('blog')
    form.setValue('AdsSurfaces', serializeSurfaces([...next]), {
      shouldDirty: true,
    })
  }

  const [logKeyword, setLogKeyword] = useState('')
  const [logPage, setLogPage] = useState(1)

  const statsQuery = useQuery({
    queryKey: ['admin-ad-stats', logKeyword, logPage],
    queryFn: async () => {
      const res = await api.get('/api/blog/manage/ads/stats', {
        params: { keyword: logKeyword, page: logPage, page_size: 15 },
      })
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
        logs?: {
          id: number
          ad_id: string
          is_adsense: boolean
          ip: string
          referrer: string
          user_agent: string
          is_member: boolean
          user_id: number
          username: string
          created_at: string
        }[]
        logs_total?: number
        page?: number
        page_size?: number
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
      if (ad.placements.length === 0) {
        toast.error(t('Every ad needs at least one surface'))
        return
      }
    }

    const customAdsJson = JSON.stringify(customAds)
    const entries: [keyof AdsFormValues, string][] = [
      ['AdsEnabled', String(data.AdsEnabled)],
      ['AdsMode', data.AdsMode],
      ['AdsSurfaces', data.AdsSurfaces],
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
      {
        id: `sponsor-ad-${current.length + 1}`,
        image: '',
        url: '',
        placements: [surfaces[0] ?? 'blog'],
      },
    ])
  }

  const updateCustomAd = (
    index: number,
    field: keyof CustomAdDraft,
    value: string | AdSurface[]
  ) => {
    setCustomAds((current) =>
      current.map((ad, i) => (i === index ? { ...ad, [field]: value } : ad))
    )
  }

  /** Adds or removes one surface of one ad. */
  const togglePlacement = (
    index: number,
    surface: AdSurface,
    enabled: boolean
  ) => {
    setCustomAds((current) =>
      current.map((ad, i) => {
        if (i !== index) return ad
        const next = new Set(ad.placements)
        if (enabled) next.add(surface)
        else next.delete(surface)
        // The surfaces of the installation bound the ad: an ad that the panel
        // keeps on a switch that is off would never draw and the administrator
        // would look for it on a page that never receives it.
        const allowed = AD_SURFACES.filter(
          (item) => next.has(item) && surfaces.includes(item)
        )
        return {
          ...ad,
          placements: allowed.length > 0 ? allowed : [surfaces[0] ?? 'blog'],
        }
      })
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
                      'Show ads on the surfaces that are on below: the banners above and below a post of the blog and two ads between the cards of the dashboard.'
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
              <div className='space-y-3'>
                <div>
                  <Label className='text-base'>{t('Ad surfaces')}</Label>
                  <FormDescription>
                    {t(
                      'Choose where the ads of the installation may appear. The blog draws the banners above and below a post, the console draws two ads between the cards of the dashboard. Every ad also picks its own surfaces below.'
                    )}
                  </FormDescription>
                </div>
                <div className='grid gap-2 sm:grid-cols-2'>
                  <SettingsSwitchItem>
                    <SettingsSwitchContent>
                      <FormLabel>{t('Blog')}</FormLabel>
                      <FormDescription>
                        {t('Banners above and below the posts of the blog.')}
                      </FormDescription>
                    </SettingsSwitchContent>
                    <Switch
                      checked={surfaces.includes('blog')}
                      onCheckedChange={(value: boolean) =>
                        toggleSurface('blog', value)
                      }
                    />
                  </SettingsSwitchItem>
                  <SettingsSwitchItem>
                    <SettingsSwitchContent>
                      <FormLabel>{t('Console')}</FormLabel>
                      <FormDescription>
                        {t(
                          'Two ads side by side between the cards of the dashboard.'
                        )}
                      </FormDescription>
                    </SettingsSwitchContent>
                    <Switch
                      checked={surfaces.includes('console')}
                      onCheckedChange={(value: boolean) =>
                        toggleSurface('console', value)
                      }
                    />
                  </SettingsSwitchItem>
                </div>
              </div>

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
                                updateCustomAd(
                                  index,
                                  'image',
                                  event.target.value
                                )
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
                        <div>
                          <Label className='text-xs'>
                            {t('Where this ad appears')}
                          </Label>
                          <div className='mt-1.5 flex flex-wrap items-center gap-4'>
                            {AD_SURFACES.map((surface) => {
                              const allowed = surfaces.includes(surface)
                              return (
                                <label
                                  key={surface}
                                  className='flex items-center gap-2 text-xs'
                                  title={
                                    allowed
                                      ? undefined
                                      : t(
                                          'Turn this surface on to place the ad there.'
                                        )
                                  }
                                >
                                  <Checkbox
                                    checked={
                                      allowed && ad.placements.includes(surface)
                                    }
                                    disabled={!allowed}
                                    onCheckedChange={(value) =>
                                      togglePlacement(
                                        index,
                                        surface,
                                        value === true
                                      )
                                    }
                                  />
                                  <span className={allowed ? '' : 'opacity-50'}>
                                    {surface === 'blog'
                                      ? t('Blog')
                                      : t('Console')}
                                  </span>
                                </label>
                              )
                            })}
                          </div>
                        </div>
                      </CardContent>
                    </Card>
                  ))}
                </div>
              )}

              <div className='space-y-3 border-t pt-4'>
                <div className='flex items-center justify-between'>
                  <Label className='text-base'>{t('Ad impressions')}</Label>
                  <div className='flex items-center gap-2'>
                    <Button
                      type='button'
                      size='sm'
                      variant='outline'
                      onClick={async () => {
                        if (
                          !confirm(
                            t('Are you sure you want to clear all ad stats?')
                          )
                        )
                          return
                        try {
                          const res = await api.delete(
                            '/api/blog/manage/ads/stats/clear'
                          )
                          if (res.data?.success) {
                            toast.success(t('Ad stats cleared successfully'))
                            statsQuery.refetch()
                          } else {
                            toast.error(
                              res.data?.message || t('Failed to clear stats')
                            )
                          }
                        } catch {
                          toast.error(t('Failed to clear stats'))
                        }
                      }}
                    >
                      <Trash2 className='me-1 h-4 w-4' />
                      {t('Clear Stats')}
                    </Button>
                    <Button
                      type='button'
                      size='sm'
                      variant='outline'
                      onClick={async () => {
                        try {
                          const response = await api.get(
                            '/api/blog/manage/ads/impressions.csv',
                            {
                              responseType: 'blob',
                            }
                          )
                          const url = window.URL.createObjectURL(
                            new Blob([response.data])
                          )
                          const link = document.createElement('a')
                          link.href = url
                          link.setAttribute('download', 'ad-impressions.csv')
                          document.body.appendChild(link)
                          link.click()
                          link.remove()
                          window.URL.revokeObjectURL(url)
                          toast.success(t('CSV downloaded successfully'))
                        } catch {
                          toast.error(t('Failed to download CSV'))
                        }
                      }}
                    >
                      <Download className='me-1 h-4 w-4' />
                      {t('Download CSV')}
                    </Button>
                  </div>
                </div>
                {statsQuery.isLoading && (
                  <p className='text-muted-foreground text-sm'>
                    {t('Loading...')}
                  </p>
                )}
                {stats && (
                  <div className='space-y-4'>
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

                    <div className='space-y-3 border-t pt-4'>
                      <div className='flex items-center justify-between gap-3'>
                        <Label className='text-sm font-semibold'>
                          {t('Detailed Impression Logs')}
                        </Label>
                        <Input
                          placeholder={t(
                            'Search IP, Referrer, User Agent, Username...'
                          )}
                          value={logKeyword}
                          onChange={(e) => {
                            setLogKeyword(e.target.value)
                            setLogPage(1)
                          }}
                          className='h-8 max-w-xs text-xs'
                        />
                      </div>

                      <Table>
                        <TableHeader>
                          <TableRow>
                            <TableHead>{t('Time')}</TableHead>
                            <TableHead>{t('Ad ID')}</TableHead>
                            <TableHead>{t('IP')}</TableHead>
                            <TableHead>{t('Member / User')}</TableHead>
                            <TableHead>{t('Referrer / Source')}</TableHead>
                            <TableHead>{t('User Agent')}</TableHead>
                          </TableRow>
                        </TableHeader>
                        <TableBody>
                          {stats.logs?.map((log) => (
                            <TableRow key={log.id}>
                              <TableCell className='text-muted-foreground text-xs whitespace-nowrap tabular-nums'>
                                {new Date(log.created_at).toLocaleString()}
                              </TableCell>
                              <TableCell className='font-mono text-xs'>
                                {log.is_adsense ? 'AdSense' : log.ad_id}
                              </TableCell>
                              <TableCell className='font-mono text-xs whitespace-nowrap'>
                                {log.ip || '-'}
                              </TableCell>
                              <TableCell className='text-xs'>
                                {log.is_member ? (
                                  <span className='font-medium text-blue-500'>
                                    {log.username || `User #${log.user_id}`}
                                  </span>
                                ) : (
                                  <span className='text-muted-foreground'>
                                    {t('Guest')}
                                  </span>
                                )}
                              </TableCell>
                              <TableCell
                                className='text-muted-foreground max-w-[180px] truncate text-xs'
                                title={log.referrer}
                              >
                                {log.referrer || '-'}
                              </TableCell>
                              <TableCell
                                className='text-muted-foreground max-w-[200px] truncate text-xs'
                                title={log.user_agent}
                              >
                                {log.user_agent || '-'}
                              </TableCell>
                            </TableRow>
                          ))}
                          {(!stats.logs || stats.logs.length === 0) && (
                            <TableRow>
                              <TableCell
                                colSpan={6}
                                className='text-muted-foreground py-4 text-center text-xs'
                              >
                                {t('No detailed logs available.')}
                              </TableCell>
                            </TableRow>
                          )}
                        </TableBody>
                      </Table>

                      {(stats.logs_total ?? 0) > 15 && (
                        <div className='text-muted-foreground flex items-center justify-between pt-2 text-xs'>
                          <span>
                            {t('Page')} {logPage} /{' '}
                            {Math.ceil((stats.logs_total ?? 0) / 15)}
                          </span>
                          <div className='flex items-center gap-2'>
                            <Button
                              type='button'
                              variant='outline'
                              size='sm'
                              disabled={logPage <= 1}
                              onClick={() =>
                                setLogPage((p) => Math.max(1, p - 1))
                              }
                            >
                              {t('Previous')}
                            </Button>
                            <Button
                              type='button'
                              variant='outline'
                              size='sm'
                              disabled={logPage * 15 >= (stats.logs_total ?? 0)}
                              onClick={() => setLogPage((p) => p + 1)}
                            >
                              {t('Next')}
                            </Button>
                          </div>
                        </div>
                      )}
                    </div>
                  </div>
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
