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
import { Download, Loader2, Plus, Trash2, Upload } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useForm, type Resolver } from 'react-hook-form'
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
  _draftKey?: string
  id: string
  image: string
  images?: Record<string, string>
  popupImages?: Record<string, string>
  /** Seconds the skip button of the popup ad stays disabled. */
  popupSkipSeconds?: number
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
      .map((ad, idx) => ({
        _draftKey: `draft-ad-${idx}-${Math.random().toString(36).slice(2, 9)}`,
        id: ad.id as string,
        image: typeof ad.image === 'string' ? ad.image : '',
        images: typeof (ad as { images?: unknown }).images === 'object' && (ad as { images?: Record<string, string> }).images ? ((ad as { images: Record<string, string> }).images) : {},
        popupImages: typeof (ad as { popup_images?: unknown }).popup_images === 'object' && (ad as { popup_images?: Record<string, string> }).popup_images ? ((ad as { popup_images: Record<string, string> }).popup_images) : {},
        popupSkipSeconds: typeof (ad as { popup_skip_seconds?: unknown }).popup_skip_seconds === 'number' && Number.isFinite(Number((ad as { popup_skip_seconds?: number }).popup_skip_seconds)) ? (ad as { popup_skip_seconds?: number }).popup_skip_seconds : 0,
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

const AD_LANGUAGES = [
  { code: 'tr', flag: '🇹🇷', label: 'TR' },
  { code: 'en', flag: '🇺🇸', label: 'EN' },
  { code: 'zh_CN', flag: '🇨🇳', label: 'ZH' },
  { code: 'zh_TW', flag: '🇹🇼', label: 'TW' },
  { code: 'fr', flag: '🇫🇷', label: 'FR' },
  { code: 'ru', flag: '🇷🇺', label: 'RU' },
  { code: 'ja', flag: '🇯🇵', label: 'JA' },
  { code: 'vi', flag: '🇻🇳', label: 'VI' },
] as const

function LanguageImageGrid(props: {
  t: (key: string, options?: Record<string, unknown>) => string
  header: string
  hint: string
  required?: boolean
  values: Record<string, string | undefined>
  uploadingSlot: { index: number; lang: string } | null
  adIndex: number
  popup: boolean
  onImageChange: (lang: string, url: string) => void
  onUpload: (langKey: string) => void
}) {
  const uploadingLang =
    props.uploadingSlot && props.uploadingSlot.index === props.adIndex
      ? props.uploadingSlot.lang
      : null
  return (
    <div className='bg-muted/20 space-y-2 rounded-lg border border-border/50 p-2.5'>
      <div className='flex items-center justify-between gap-2'>
        <div>
          <p className='text-xs font-semibold text-foreground'>
            {props.header}
            {props.required && <span className='ml-1 text-destructive'>*</span>}
          </p>
          <p className='text-[10px] text-muted-foreground'>{props.hint}</p>
        </div>
        {props.required && (
          <span className='text-[10px] text-muted-foreground'>
            {props.t('At least one image required')}
          </span>
        )}
      </div>
      <div className='grid grid-cols-1 gap-2 sm:grid-cols-2'>
        {AD_LANGUAGES.map((lang) => {
          const langKey = props.popup ? `popup:${lang.code}` : lang.code
          const isUploadingThis = uploadingLang === langKey
          return (
            <div key={langKey} className='flex items-center gap-1.5'>
              <span className='flex w-10 shrink-0 items-center gap-1 text-xs'>
                <span>{lang.flag}</span>
                <span className='font-mono font-bold'>{lang.label}</span>
              </span>
              <Input
                value={props.values[lang.code] || ''}
                onChange={(e) => props.onImageChange(lang.code, e.target.value)}
                placeholder={`${lang.label} URL...`}
                className='h-7 text-xs flex-1'
              />
              <Button
                type='button'
                variant='outline'
                size='icon'
                className='h-7 w-7 shrink-0'
                disabled={isUploadingThis}
                onClick={() => props.onUpload(langKey)}
                title={`${lang.label} ${props.t('Upload')}`}
              >
                {isUploadingThis ? (
                  <Loader2 className='h-3.5 w-3.5 animate-spin' />
                ) : (
                  <Upload className='h-3.5 w-3.5' />
                )}
              </Button>
            </div>
          )
        })}
      </div>
    </div>
  )
}

export function AdsSection({ defaultValues }: AdsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const fileInputRef = useRef<HTMLInputElement>(null)
  const [uploadingSlot, setUploadingSlot] = useState<{ index: number; lang: string } | null>(null)

  const form = useForm<AdsFormValues>({
    // The numeric field coerces its input, so the resolver is cast to the form
    // type the rest of the card already uses.
    resolver: zodResolver(adsSchema) as unknown as Resolver<AdsFormValues>,
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
      const placements = ad.placements ?? []
      const bannerCount = Object.values(ad.images ?? {}).filter(
        (img) => img && img.trim() !== ''
      ).length
      const popupCount = Object.values(ad.popupImages ?? {}).filter(
        (img) => img && img.trim() !== ''
      ).length
      if (placements.includes('blog') || placements.includes('console')) {
        // The banner of the blog and the console wants at least one image;
        // the missing languages fall back to the English one.
        if (bannerCount === 0) {
          toast.error(t('Every ad needs at least one banner image'))
          return
        }
      }
      if (placements.includes('popup')) {
        // The popup draws a tall rectangle of its own; one image is enough,
        // the missing languages fall back to the one that exists.
        if (popupCount === 0) {
          toast.error(t('The popup ad needs at least one popup image'))
          return
        }
        const skip = Number(ad.popupSkipSeconds ?? 0)
        if (!Number.isFinite(skip) || skip < 0 || skip > 60) {
          toast.error(t('Popup skip delay must be between 0 and 60 seconds'))
          return
        }
      }
      if (!ad.url.trim()) {
        toast.error(t('Every ad needs a target URL'))
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

    const sanitizedAds = customAds.map((ad) => {
      const popupImages = Object.fromEntries(
        Object.entries(ad.popupImages ?? {}).filter(([, v]) => v && v.trim() !== '')
      )
      const images = Object.fromEntries(
        Object.entries(ad.images ?? {}).filter(([, v]) => v && v.trim() !== '')
      )
      const anyImg =
        images['tr'] ||
        images['en'] ||
        Object.values(images).find((v) => v && v.trim() !== '') ||
        Object.values(popupImages).find((v) => v && v.trim() !== '') ||
        ''
      return {
        id: ad.id,
        image: anyImg,
        images,
        popup_images: popupImages,
        popup_skip_seconds: Math.max(0, Math.min(60, Math.floor(Number(ad.popupSkipSeconds) || 0))),
        url: ad.url,
        placements: ad.placements,
      }
    })

    const customAdsJson = JSON.stringify(sanitizedAds)
    const entries: [keyof AdsFormValues, string][] = [
      ['AdsEnabled', String(data.AdsEnabled)],
      ['AdsMode', data.AdsMode],
      // The top-level surface switches are gone: an ad decides where it
      // appears, and every surface is reachable. The stored option is set
      // to all surfaces so an old install behaves the same.
      ['AdsSurfaces', 'blog,console,popup'],
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
        _draftKey: `draft-ad-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`,
        id: `sponsor-ad-${current.length + 1}`,
        image: '',
        images: {},
        popupImages: {},
        popupSkipSeconds: 0,
        url: '',
        placements: ['blog'],
      },
    ])
  }

  const updateCustomAd = (
    index: number,
    field: keyof CustomAdDraft,
    value: string | number | Record<string, string> | AdSurface[]
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
        return {
          ...ad,
          placements: AD_SURFACES.filter((item) => next.has(item)),
        }
      })
    )
  }

  const removeCustomAd = (index: number) => {
    setCustomAds((current) => current.filter((_, i) => i !== index))
  }

  const handleFileUpload = async (index: number, langKey: string, file: File) => {
    const isPopup = langKey.startsWith('popup:')
    const lang = isPopup ? langKey.slice('popup:'.length) : langKey
    const formData = new FormData()
    formData.append('file', file)
    setUploadingSlot({ index, lang: langKey })
    try {
      const res = await api.post('/api/blog/manage/ads/upload', formData, {
        headers: { 'Content-Type': 'multipart/form-data' },
      })
      if (res.data?.success && res.data?.data) {
        const ad = customAds[index]
        if (isPopup) {
          const updatedImages = { ...(ad?.popupImages || {}), [lang]: res.data.data }
          updateCustomAd(index, 'popupImages' as keyof CustomAdDraft, updatedImages)
        } else {
          const updatedImages = { ...(ad?.images || {}), [lang]: res.data.data }
          updateCustomAd(index, 'images' as keyof CustomAdDraft, updatedImages)
        }
        toast.success(t('Image uploaded successfully'))
      } else {
        toast.error(res.data?.message || t('Failed to upload image'))
      }
    } catch {
      toast.error(t('Failed to upload image'))
    } finally {
      setUploadingSlot(null)
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
                      'Show ads on the surfaces that are on below: the banners above and below a post of the blog, the ads between the cards of the dashboard and the banner that opens over a page.'
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
                      'Each ad needs an ID, images and a target site. The image stretches to fill the banner slot, and clicking it opens the site in a new tab. The ID only shows here and in the stats.'
                    )}
                  </FormDescription>
                  {customAds.length === 0 && (
                    <p className='text-muted-foreground text-sm'>
                      {t('No custom ads yet.')}
                    </p>
                  )}
                  {customAds.map((ad, index) => (
                    <Card key={ad._draftKey || `ad-slot-${index}`} className='py-3'>
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
                        {(ad.placements.includes('blog') ||
                          ad.placements.includes('console')) && (
                          <LanguageImageGrid
                            t={t}
                            required
                            header={t('Banner images (Blog & Console)')}
                            hint={t('Banner strip for the blog and the console (1000 x 200). Missing languages fall back to English.')}
                            values={ad.images ?? {}}
                            uploadingSlot={uploadingSlot}
                            adIndex={index}
                            popup={false}
                            onImageChange={(lang, url) => {
                              const updatedImages = { ...(ad.images || {}), [lang]: url }
                              if (!url) delete updatedImages[lang]
                              updateCustomAd(index, 'images' as keyof CustomAdDraft, updatedImages)
                            }}
                            onUpload={(langKey) => {
                              setUploadingSlot({ index, lang: langKey })
                              fileInputRef.current?.click()
                            }}
                          />
                        )}
                        {ad.placements.includes('popup') && (
                          <LanguageImageGrid
                            t={t}
                            required
                            header={t('Popup images')}
                            hint={t('Tall rectangle required for the popup (1600 x 900). The popup never uses banner images.')}
                            values={ad.popupImages ?? {}}
                            uploadingSlot={uploadingSlot}
                            adIndex={index}
                            popup
                            onImageChange={(lang, url) => {
                              const updatedImages = { ...(ad.popupImages || {}), [lang]: url }
                              if (!url) delete updatedImages[lang]
                              updateCustomAd(index, 'popupImages' as keyof CustomAdDraft, updatedImages)
                            }}
                            onUpload={(langKey) => {
                              setUploadingSlot({ index, lang: langKey })
                              fileInputRef.current?.click()
                            }}
                          />
                        )}
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
                            {AD_SURFACES.map((surface) => (
                              <label
                                key={surface}
                                className='flex items-center gap-2 text-xs'
                              >
                                <Checkbox
                                  checked={ad.placements.includes(surface)}
                                  onCheckedChange={(value) =>
                                    togglePlacement(
                                      index,
                                      surface,
                                      value === true
                                    )
                                  }
                                />
                                <span>
                                  {surface === 'blog'
                                    ? t('Blog')
                                    : surface === 'console'
                                      ? t('Console')
                                      : t('Popup')}
                                </span>
                              </label>
                            ))}
                          </div>
                        </div>
                        {ad.placements.includes('popup') && (
                          <div>
                            <Label className='text-xs'>
                              {t('Popup skip delay (seconds)')}
                            </Label>
                            <div className='mt-1 flex items-center gap-2'>
                              <Input
                                className='max-w-28'
                                inputMode='numeric'
                                type='number'
                                min={0}
                                max={60}
                                value={ad.popupSkipSeconds ?? 0}
                                onChange={(event) =>
                                  updateCustomAd(
                                    index,
                                    'popupSkipSeconds',
                                    Math.max(
                                      0,
                                      Math.min(60, Math.floor(Number(event.target.value) || 0))
                                    )
                                  )
                                }
                              />
                              <span className='text-[10px] text-muted-foreground'>
                                {t('0 = skip button active immediately')}
                              </span>
                            </div>
                          </div>
                        )}
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
          if (file && uploadingSlot !== null) {
            void handleFileUpload(uploadingSlot.index, uploadingSlot.lang, file)
          }
          e.target.value = ''
        }}
      />
    </SettingsSection>
  )
}
