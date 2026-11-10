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
import { ArrowDown, ArrowUp, ExternalLink, Globe, Plus, Save, Trash2, X } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { type AnnouncementBarItem } from '@/components/announcement-bar-banner'

import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const PRESET_COLORS = [
  { name: 'Purple', bg: '#8b5cf6', text: '#ffffff' },
  { name: 'Emerald', bg: '#10b981', text: '#ffffff' },
  { name: 'Amber', bg: '#f59e0b', text: '#000000' },
  { name: 'Blue', bg: '#3b82f6', text: '#ffffff' },
  { name: 'Rose', bg: '#f43f5e', text: '#ffffff' },
  { name: 'Dark', bg: '#18181b', text: '#ffffff' },
  { name: 'Red', bg: '#dc2626', text: '#ffffff' },
  { name: 'Pastel Mint', bg: '#d1fae5', text: '#064e3b' },
  { name: 'Pastel Lavender', bg: '#ede9fe', text: '#4c1d95' },
]

const LANGUAGES = [
  { code: 'tr', flag: '🇹🇷', label: 'Türkçe' },
  { code: 'en', flag: '🇬🇧', label: 'English' },
  { code: 'zh', flag: '🇨🇳', label: '简体中文' },
  { code: 'zh-TW', flag: '🇹🇼', label: '繁體中文' },
  { code: 'ja', flag: '🇯🇵', label: '日本語' },
  { code: 'ru', flag: '🇷🇺', label: 'Русский' },
  { code: 'fr', flag: '🇫🇷', label: 'Français' },
  { code: 'vi', flag: '🇻🇳', label: 'Tiếng Việt' },
] as const

export function AnnouncementBarsSection({
  initialData,
}: {
  initialData?: string
}) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [saving, setSaving] = useState(false)
  const [activeLang, setActiveLang] = useState<string>('tr')

  const [bars, setBars] = useState<AnnouncementBarItem[]>(() => {
    if (!initialData) return []
    try {
      const parsed = JSON.parse(initialData)
      return Array.isArray(parsed) ? parsed : []
    } catch {
      return []
    }
  })

  const handleAddBar = () => {
    const newBar: AnnouncementBarItem = {
      id: `bar-${Date.now()}`,
      enabled: true,
      text: '🔥 Summer Sale Up To 50% OFF! Check out our latest updates.',
      translations: {
        tr: '🔥 %50\'ye Varan Yaz İndirimi! En son güncellemelerimize göz atın.',
        en: '🔥 Summer Sale Up To 50% OFF! Check out our latest updates.',
      },
      bg_color: '#8b5cf6',
      text_color: '#ffffff',
      height: 38,
      font_size: '13px',
      font_weight: '500',
      button_enabled: true,
      button_text: 'Shop Now',
      button_translations: {
        tr: 'Hemen İncele',
        en: 'Shop Now',
      },
      button_url: 'https://example.com',
      button_bg_color: '#000000',
      button_text_color: '#ffffff',
      closable: true,
    }
    setBars((prev) => [...prev, newBar])
  }

  const handleUpdateBar = (index: number, patch: Partial<AnnouncementBarItem>) => {
    setBars((prev) => {
      const copy = [...prev]
      copy[index] = { ...copy[index], ...patch }
      return copy
    })
  }

  const handleUpdateTranslation = (
    index: number,
    lang: string,
    field: 'text' | 'button_text',
    value: string
  ) => {
    setBars((prev) => {
      const copy = [...prev]
      const bar = { ...copy[index] }
      if (field === 'text') {
        const trans = { ...(bar.translations || {}) }
        trans[lang] = value
        bar.translations = trans
        if (lang === 'en' || !bar.text) {
          bar.text = value
        }
      } else {
        const btnTrans = { ...(bar.button_translations || {}) }
        btnTrans[lang] = value
        bar.button_translations = btnTrans
        if (lang === 'en' || !bar.button_text) {
          bar.button_text = value
        }
      }
      copy[index] = bar
      return copy
    })
  }

  const handleDeleteBar = (index: number) => {
    setBars((prev) => prev.filter((_, i) => i !== index))
  }

  const handleMove = (index: number, direction: 'up' | 'down') => {
    setBars((prev) => {
      const copy = [...prev]
      const targetIndex = direction === 'up' ? index - 1 : index + 1
      if (targetIndex < 0 || targetIndex >= copy.length) return prev
      const temp = copy[index]
      copy[index] = copy[targetIndex]
      copy[targetIndex] = temp
      return copy
    })
  }

  const handleSaveAll = async () => {
    setSaving(true)
    try {
      await updateOption.mutateAsync({
        key: 'AnnouncementBars',
        value: JSON.stringify(bars),
      })
      toast.success(t('Announcement bars saved successfully'))
    } catch (err: any) {
      toast.error(err?.message || t('Failed to save announcement bars'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <SettingsSection title={t('Announcement Bars (Top Banners)')}>
      <div className='flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 -mt-2 mb-2'>
        <p className='text-sm text-muted-foreground'>
          {t(
            'Configure customizable top announcement lines, choose custom heights, colors, text, auto marquee scroll when overflowing, and action buttons.'
          )}
        </p>
        <div className='flex items-center gap-2 shrink-0'>
          <Button variant='outline' size='sm' onClick={handleAddBar}>
            <Plus className='h-4 w-4 me-1' />
            {t('Add Announcement Bar')}
          </Button>
          <Button size='sm' onClick={handleSaveAll} disabled={saving || updateOption.isPending}>
            <Save className='h-4 w-4 me-1' />
            {saving || updateOption.isPending ? t('Saving...') : t('Save Changes')}
          </Button>
        </div>
      </div>

      <div className='space-y-6 pt-2'>
        {bars.length === 0 ? (
          <div className='rounded-lg border border-dashed p-8 text-center'>
            <p className='text-sm text-muted-foreground'>
              {t('No announcement bars configured yet. Add your first top banner!')}
            </p>
            <Button variant='outline' size='sm' className='mt-4' onClick={handleAddBar}>
              <Plus className='h-4 w-4 me-1' />
              {t('Add Announcement Bar')}
            </Button>
          </div>
        ) : (
          bars.map((bar, index) => {
            const currentText =
              bar.translations?.[activeLang] ??
              (activeLang === 'en' ? bar.text : '') ??
              ''
            const currentBtnText =
              bar.button_translations?.[activeLang] ??
              (activeLang === 'en' ? bar.button_text : '') ??
              ''

            return (
              <Card key={bar.id || index} className='relative overflow-hidden border shadow-sm'>
                {/* Visual Live Preview of this Bar */}
                <div
                  style={{
                    backgroundColor: bar.bg_color || '#8b5cf6',
                    color: bar.text_color || '#ffffff',
                    height: `${Math.max(28, Math.min(bar.height || 38, 80))}px`,
                    fontSize: bar.font_size || '13px',
                    fontWeight: bar.font_weight || '500',
                  }}
                  className='w-full flex items-center justify-between px-4 text-xs font-medium border-b select-none transition-colors'
                >
                  <div className='truncate flex-1 text-center pr-2'>
                    {currentText || bar.text || t('Announcement bar preview text...')}
                  </div>
                  <div className='flex items-center gap-2 shrink-0'>
                    {bar.button_enabled && (
                      <span
                        style={{
                          backgroundColor: bar.button_bg_color || '#000000',
                          color: bar.button_text_color || '#ffffff',
                        }}
                        className='rounded-full px-3 py-0.5 text-[11px] font-semibold flex items-center gap-1 shadow-sm'
                      >
                        {currentBtnText || bar.button_text || t('Action')}
                        <ExternalLink className='h-2.5 w-2.5 opacity-80' />
                      </span>
                    )}
                    {bar.closable !== false && <X className='h-3 w-3 opacity-70' />}
                  </div>
                </div>

                <CardHeader className='pb-3 pt-4'>
                  <div className='flex items-center justify-between'>
                    <div className='flex items-center gap-2'>
                      <CardTitle className='text-sm font-semibold'>
                        {t('Bar #{{num}}', { num: index + 1 })}
                      </CardTitle>
                      <Badge variant={bar.enabled ? 'default' : 'secondary'} className='text-[10px]'>
                        {bar.enabled ? t('Active') : t('Disabled')}
                      </Badge>
                    </div>

                    <div className='flex items-center gap-1'>
                      <Button
                        variant='ghost'
                        size='icon'
                        className='h-7 w-7'
                        disabled={index === 0}
                        onClick={() => handleMove(index, 'up')}
                        title={t('Move Up')}
                      >
                        <ArrowUp className='h-3.5 w-3.5' />
                      </Button>
                      <Button
                        variant='ghost'
                        size='icon'
                        className='h-7 w-7'
                        disabled={index === bars.length - 1}
                        onClick={() => handleMove(index, 'down')}
                        title={t('Move Down')}
                      >
                        <ArrowDown className='h-3.5 w-3.5' />
                      </Button>
                      <Button
                        variant='ghost'
                        size='icon'
                        className='h-7 w-7 text-destructive hover:bg-destructive/10'
                        onClick={() => handleDeleteBar(index)}
                        title={t('Delete')}
                      >
                        <Trash2 className='h-3.5 w-3.5' />
                      </Button>
                    </div>
                  </div>
                </CardHeader>

                <CardContent className='space-y-4'>
                  <div className='flex items-center justify-between border-b pb-3'>
                    <div className='space-y-0.5'>
                      <Label className='text-xs font-medium'>{t('Enable Bar')}</Label>
                      <p className='text-[11px] text-muted-foreground'>
                        {t('Turn this announcement bar on or off.')}
                      </p>
                    </div>
                    <Switch
                      checked={bar.enabled}
                      onCheckedChange={(val) => handleUpdateBar(index, { enabled: val })}
                    />
                  </div>

                  {/* Multi-language Selector Tabs */}
                  <div className='space-y-2 border-b pb-4'>
                    <div className='flex items-center justify-between'>
                      <Label className='text-xs font-medium flex items-center gap-1.5'>
                        <Globe className='h-3.5 w-3.5 text-primary' />
                        {t('Language')} &amp; {t('Announcement Text')}
                      </Label>
                      <span className='text-[11px] text-muted-foreground'>
                        {t('Use **bold** and _italic_ for formatting')}
                      </span>
                    </div>

                    <div className='flex flex-wrap gap-1'>
                      {LANGUAGES.map((lang) => {
                        const hasVal = Boolean(
                          bar.translations?.[lang.code]?.trim() ||
                            (lang.code === 'en' && bar.text?.trim())
                        )
                        return (
                          <Button
                            key={lang.code}
                            type='button'
                            size='sm'
                            variant={activeLang === lang.code ? 'default' : 'outline'}
                            className='h-7 text-xs px-2.5 gap-1'
                            onClick={() => setActiveLang(lang.code)}
                          >
                            <span>{lang.flag}</span>
                            <span>{lang.label}</span>
                            {hasVal && (
                              <span className='size-1.5 rounded-full bg-emerald-500 ms-0.5' />
                            )}
                          </Button>
                        )
                      })}
                    </div>

                    <div className='pt-2 space-y-1.5'>
                      <Input
                        value={currentText}
                        onChange={(e) =>
                          handleUpdateTranslation(index, activeLang, 'text', e.target.value)
                        }
                        placeholder={t('Enter message for selected language (e.g. **Big Sale** 50% Off)...')}
                        className='text-xs font-medium'
                      />
                      <p className='text-[11px] text-muted-foreground'>
                        {t('If the text exceeds screen width, it automatically scrolls in a smooth marquee.')}
                      </p>
                    </div>
                  </div>

                  {/* Typography & Dimensions */}
                  <div className='grid grid-cols-1 gap-4 sm:grid-cols-4 border-b pb-4'>
                    <div className='space-y-1.5'>
                      <Label className='text-xs font-medium'>{t('Height (px)')}</Label>
                      <Input
                        type='number'
                        min={28}
                        max={80}
                        value={bar.height ?? 38}
                        onChange={(e) =>
                          handleUpdateBar(index, { height: Number(e.target.value) || 38 })
                        }
                        className='text-xs font-mono'
                      />
                    </div>

                    <div className='space-y-1.5'>
                      <Label className='text-xs font-medium'>{t('Font Size')}</Label>
                      <Select
                        value={bar.font_size || '13px'}
                        onValueChange={(val) => handleUpdateBar(index, { font_size: val || undefined })}
                      >
                        <SelectTrigger className='text-xs h-9'>
                          <SelectValue placeholder={String(t('Font Size'))} />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value='11px'>11px ({t('Extra Small')})</SelectItem>
                          <SelectItem value='12px'>12px ({t('Small')})</SelectItem>
                          <SelectItem value='13px'>13px ({t('Regular')})</SelectItem>
                          <SelectItem value='14px'>14px ({t('Medium')})</SelectItem>
                          <SelectItem value='16px'>16px ({t('Large')})</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>

                    <div className='space-y-1.5'>
                      <Label className='text-xs font-medium'>{t('Font Weight')}</Label>
                      <Select
                        value={bar.font_weight || '500'}
                        onValueChange={(val) => handleUpdateBar(index, { font_weight: val || undefined })}
                      >
                        <SelectTrigger className='text-xs h-9'>
                          <SelectValue placeholder={String(t('Font Weight'))} />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value='400'>{t('Normal (400)')}</SelectItem>
                          <SelectItem value='500'>{t('Medium (500)')}</SelectItem>
                          <SelectItem value='600'>{t('Semi-Bold (600)')}</SelectItem>
                          <SelectItem value='700'>{t('Bold (700)')}</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>

                    <div className='space-y-1.5'>
                      <Label className='text-xs font-medium'>{t('Closable Button (X)')}</Label>
                      <div className='flex items-center h-9'>
                        <Switch
                          checked={bar.closable !== false}
                          onCheckedChange={(val) => handleUpdateBar(index, { closable: val })}
                        />
                      </div>
                    </div>
                  </div>

                  {/* Colors Grid */}
                  <div className='grid grid-cols-1 gap-4 sm:grid-cols-2 border-b pb-4'>
                    <div className='space-y-1.5'>
                      <Label className='text-xs font-medium'>{t('Background Color')}</Label>
                      <div className='flex items-center gap-2'>
                        <input
                          type='color'
                          value={bar.bg_color || '#8b5cf6'}
                          onChange={(e) => handleUpdateBar(index, { bg_color: e.target.value })}
                          className='h-8 w-10 cursor-pointer rounded border p-0.5 bg-background'
                        />
                        <Input
                          value={bar.bg_color || '#8b5cf6'}
                          onChange={(e) => handleUpdateBar(index, { bg_color: e.target.value })}
                          className='text-xs font-mono'
                        />
                      </div>
                    </div>

                    <div className='space-y-1.5'>
                      <Label className='text-xs font-medium'>{t('Text Color')}</Label>
                      <div className='flex items-center gap-2'>
                        <input
                          type='color'
                          value={bar.text_color || '#ffffff'}
                          onChange={(e) => handleUpdateBar(index, { text_color: e.target.value })}
                          className='h-8 w-10 cursor-pointer rounded border p-0.5 bg-background'
                        />
                        <Input
                          value={bar.text_color || '#ffffff'}
                          onChange={(e) => handleUpdateBar(index, { text_color: e.target.value })}
                          className='text-xs font-mono'
                        />
                      </div>
                    </div>
                  </div>

                  {/* Preset Colors */}
                  <div className='space-y-1.5 border-b pb-4'>
                    <Label className='text-xs font-medium'>{t('Color Presets')}</Label>
                    <div className='flex flex-wrap gap-2 pt-1'>
                      {PRESET_COLORS.map((preset) => (
                        <button
                          key={preset.name}
                          type='button'
                          onClick={() =>
                            handleUpdateBar(index, {
                              bg_color: preset.bg,
                              text_color: preset.text,
                            })
                          }
                          style={{ backgroundColor: preset.bg, color: preset.text }}
                          className='rounded-md px-2.5 py-1 text-xs font-medium border shadow-xs transition-transform hover:scale-105 active:scale-95'
                        >
                          {t(preset.name)}
                        </button>
                      ))}
                    </div>
                  </div>

                  {/* Action Button Settings */}
                  <div className='space-y-4 pt-1'>
                    <div className='flex items-center justify-between'>
                      <div className='space-y-0.5'>
                        <Label className='text-xs font-medium'>{t('Action Button')}</Label>
                        <p className='text-[11px] text-muted-foreground'>
                          {t('Show an interactive pill button on the right side of the bar.')}
                        </p>
                      </div>
                      <Switch
                        checked={Boolean(bar.button_enabled)}
                        onCheckedChange={(val) =>
                          handleUpdateBar(index, { button_enabled: val })
                        }
                      />
                    </div>

                    {bar.button_enabled && (
                      <div className='rounded-lg border bg-muted/20 p-3 space-y-3'>
                        <div className='grid grid-cols-1 gap-3 sm:grid-cols-2'>
                          <div className='space-y-1.5'>
                            <Label className='text-xs font-medium'>
                              {t('Button Text')} ({LANGUAGES.find((l) => l.code === activeLang)?.label})
                            </Label>
                            <Input
                              value={currentBtnText}
                              onChange={(e) =>
                                handleUpdateTranslation(
                                  index,
                                  activeLang,
                                  'button_text',
                                  e.target.value
                                )
                              }
                              placeholder={t('e.g. Shop Now, Learn More...')}
                              className='text-xs'
                            />
                          </div>

                          <div className='space-y-1.5'>
                            <Label className='text-xs font-medium'>{t('Button Target URL')}</Label>
                            <Input
                              value={bar.button_url || ''}
                              onChange={(e) =>
                                handleUpdateBar(index, { button_url: e.target.value })
                              }
                              placeholder={t('https://example.com or /pricing')}
                              className='text-xs font-mono'
                            />
                          </div>
                        </div>

                        <div className='grid grid-cols-1 gap-3 sm:grid-cols-2 pt-1'>
                          <div className='space-y-1.5'>
                            <Label className='text-xs font-medium'>
                              {t('Button Background Color')}
                            </Label>
                            <div className='flex items-center gap-2'>
                              <input
                                type='color'
                                value={bar.button_bg_color || '#000000'}
                                onChange={(e) =>
                                  handleUpdateBar(index, { button_bg_color: e.target.value })
                                }
                                className='h-8 w-10 cursor-pointer rounded border p-0.5 bg-background'
                              />
                              <Input
                                value={bar.button_bg_color || '#000000'}
                                onChange={(e) =>
                                  handleUpdateBar(index, { button_bg_color: e.target.value })
                                }
                                className='text-xs font-mono'
                              />
                            </div>
                          </div>

                          <div className='space-y-1.5'>
                            <Label className='text-xs font-medium'>{t('Button Text Color')}</Label>
                            <div className='flex items-center gap-2'>
                              <input
                                type='color'
                                value={bar.button_text_color || '#ffffff'}
                                onChange={(e) =>
                                  handleUpdateBar(index, { button_text_color: e.target.value })
                                }
                                className='h-8 w-10 cursor-pointer rounded border p-0.5 bg-background'
                              />
                              <Input
                                value={bar.button_text_color || '#ffffff'}
                                onChange={(e) =>
                                  handleUpdateBar(index, { button_text_color: e.target.value })
                                }
                                className='text-xs font-mono'
                              />
                            </div>
                          </div>
                        </div>
                      </div>
                    )}
                  </div>
                </CardContent>
              </Card>
            )
          })
        )}
      </div>
    </SettingsSection>
  )
}
