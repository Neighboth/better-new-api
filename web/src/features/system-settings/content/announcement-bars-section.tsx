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
import { ArrowDown, ArrowUp, ExternalLink, Plus, Save, Trash2, X } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
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
  { name: 'Pastel Mint', bg: '#d1fae5', text: '#064e3b' },
  { name: 'Pastel Lavender', bg: '#ede9fe', text: '#4c1d95' },
]

export function AnnouncementBarsSection({
  initialData,
}: {
  initialData?: string
}) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [saving, setSaving] = useState(false)

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
      text: t('🔥 Summer Sale Up To 50% OFF! Check out our latest updates.'),
      bg_color: '#8b5cf6',
      text_color: '#ffffff',
      height: 38,
      button_enabled: true,
      button_text: t('Shop Now'),
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
            return (
              <Card key={bar.id || index} className='relative overflow-hidden border shadow-sm'>
                {/* Visual Live Preview of this Bar */}
                <div
                  style={{
                    backgroundColor: bar.bg_color || '#8b5cf6',
                    color: bar.text_color || '#ffffff',
                    height: `${Math.max(28, Math.min(bar.height || 38, 80))}px`,
                  }}
                  className='w-full flex items-center justify-between px-4 text-xs font-medium border-b select-none transition-colors'
                >
                  <div className='truncate flex-1 text-center pr-2'>
                    {bar.text || t('Announcement bar preview text...')}
                  </div>
                  <div className='flex items-center gap-2 shrink-0'>
                    {bar.button_enabled && (
                      <span
                        style={{
                          backgroundColor: bar.button_bg_color || '#000000',
                          color: bar.button_text_color || '#ffffff',
                        }}
                        className='inline-flex items-center gap-1 rounded-full px-3 py-0.5 text-[11px] font-semibold shadow-xs'
                      >
                        {bar.button_text || t('Button')}
                        <ExternalLink className='h-2.5 w-2.5 opacity-80' />
                      </span>
                    )}
                    {bar.closable !== false && (
                      <span className='opacity-60'>
                        <X className='h-3.5 w-3.5' />
                      </span>
                    )}
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

                  {/* Announcement Text */}
                  <div className='space-y-1.5'>
                    <Label className='text-xs font-medium'>{t('Announcement Text')}</Label>
                    <Input
                      value={bar.text}
                      onChange={(e) => handleUpdateBar(index, { text: e.target.value })}
                      placeholder={t('Enter announcement message...')}
                      className='text-xs'
                    />
                    <p className='text-[11px] text-muted-foreground'>
                      {t('If the text exceeds screen width, it automatically scrolls in a smooth marquee.')}
                    </p>
                  </div>

                  {/* Height & Colors Grid */}
                  <div className='grid grid-cols-1 gap-4 sm:grid-cols-3'>
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
                  <div className='flex flex-wrap items-center gap-1.5 pt-1'>
                    <span className='text-[11px] text-muted-foreground me-1'>
                      {t('Color Presets:')}
                    </span>
                    {PRESET_COLORS.map((preset) => (
                      <button
                        key={preset.name}
                        type='button'
                        onClick={() =>
                          handleUpdateBar(index, { bg_color: preset.bg, text_color: preset.text })
                        }
                        style={{ backgroundColor: preset.bg, color: preset.text }}
                        className='rounded px-2 py-0.5 text-[10px] font-medium border shadow-xs hover:opacity-80 transition-opacity'
                      >
                        {preset.name}
                      </button>
                    ))}
                  </div>

                  {/* Button Settings */}
                  <div className='rounded-lg border bg-muted/20 p-3 space-y-3'>
                    <div className='flex items-center justify-between'>
                      <div>
                        <Label className='text-xs font-semibold'>{t('Action Button')}</Label>
                        <p className='text-[11px] text-muted-foreground'>
                          {t('Show a pill-shaped action button redirecting users to a URL.')}
                        </p>
                      </div>
                      <Switch
                        checked={bar.button_enabled}
                        onCheckedChange={(val) => handleUpdateBar(index, { button_enabled: val })}
                      />
                    </div>

                    {bar.button_enabled && (
                      <div className='grid grid-cols-1 gap-3 sm:grid-cols-2 pt-2 border-t'>
                        <div className='space-y-1.5'>
                          <Label className='text-xs font-medium'>{t('Button Text')}</Label>
                          <Input
                            value={bar.button_text || ''}
                            onChange={(e) =>
                              handleUpdateBar(index, { button_text: e.target.value })
                            }
                            placeholder='Shop Now / Visit Now'
                            className='text-xs'
                          />
                        </div>

                        <div className='space-y-1.5'>
                          <Label className='text-xs font-medium'>{t('Button URL')}</Label>
                          <Input
                            value={bar.button_url || ''}
                            onChange={(e) =>
                              handleUpdateBar(index, { button_url: e.target.value })
                            }
                            placeholder='https://example.com/promo'
                            className='text-xs'
                          />
                        </div>

                        <div className='space-y-1.5'>
                          <Label className='text-xs font-medium'>{t('Button Background')}</Label>
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
                    )}
                  </div>

                  {/* Closable switch */}
                  <div className='flex items-center justify-between pt-1'>
                    <div className='space-y-0.5'>
                      <Label className='text-xs font-medium'>{t('Allow Close (Dismiss X Button)')}</Label>
                      <p className='text-[11px] text-muted-foreground'>
                        {t('Allows users to hide this banner for their current session.')}
                      </p>
                    </div>
                    <Switch
                      checked={bar.closable !== false}
                      onCheckedChange={(val) => handleUpdateBar(index, { closable: val })}
                    />
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
