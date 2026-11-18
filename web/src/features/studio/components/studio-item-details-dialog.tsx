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

/**
 * Read-only detail view for one Studio generation: prompt, model settings,
 * uploaded references, start/end frames and negative prompt.
 */
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { CheckIcon, CopyIcon, XIcon } from 'lucide-react'

import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { toast } from 'sonner'

import type { StudioItem, StudioItemDetails } from '../types'

function Section({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <section className='space-y-1'>
      <p className='text-[10px] font-semibold uppercase tracking-wide text-muted-foreground'>{label}</p>
      {children}
    </section>
  )
}

function SettingRow({ label, value }: { label: string; value?: string | number }) {
  if (value === undefined || value === null || value === '') return null
  return (
    <div className='flex items-baseline justify-between gap-4 border-b border-border/40 py-1 text-xs'>
      <span className='text-muted-foreground'>{label}</span>
      <span className='text-right font-medium break-all'>{String(value)}</span>
    </div>
  )
}

function FrameImage({ src, alt }: { src?: string; alt: string }) {
  if (!src) return null
  return (
    <img
      src={src}
      alt={alt}
      className='max-h-36 rounded-md border border-border/60 object-contain bg-muted/40'
    />
  )
}

function ReferenceStrip({ urls }: { urls?: string[] }) {
  if (!urls || urls.length === 0) return null
  return (
    <div className='flex flex-wrap gap-2'>
      {urls.map((url) => (
        <img
          key={url}
          src={url}
          alt='reference'
          className='size-20 rounded-md border border-border/60 object-cover bg-muted/40'
          loading='lazy'
        />
      ))}
    </div>
  )
}

const settingLabelKeys: Record<string, string> = {
  model: 'Model',
  duration: 'Duration',
  ratio: 'Ratio',
  quality: 'Quality',
  size: 'Size',
  count: 'Count',
  group: 'Group',
}

function detailRows(item: StudioItem, details?: StudioItemDetails): Array<[string, string | number | undefined]> {
  const rows: Array<[string, string | number | undefined]> = []
  const settings = (item.settings ?? {}) as Record<string, string | number | undefined>
  for (const key of ['model', 'duration', 'ratio', 'quality', 'size', 'count', 'group'] as const) {
    const value = settings[key]
    if (value !== undefined && value !== '') rows.push([settingLabelKeys[key] ?? key, value])
  }
  if (details?.engine) rows.push(['Engine', details.engine])
  if (details?.seed !== undefined) rows.push(['Seed', details.seed])
  return rows
}

export function StudioItemDetailsDialog(props: {
  item: StudioItem | null
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const [copied, setCopied] = useState(false)
  const item = props.item
  if (!item) return null
  const details = item.details
  const rows = detailRows(item, details)
  const created = item.createdAt ? new Date(item.createdAt).toLocaleString() : ''

  const copyPrompt = async () => {
    try {
      await navigator.clipboard.writeText(item.prompt)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 1500)
    } catch {
      toast.error(t('Failed to copy prompt'))
    }
  }

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='w-[94vw] sm:w-[86vw] md:max-w-2xl max-h-[85vh] flex flex-col overflow-hidden rounded-2xl p-0 gap-0'>
        <DialogHeader className='p-3 border-b border-border/60 flex flex-row items-center justify-between space-y-0'>
          <DialogTitle className='text-sm font-semibold'>{t('Generation details')}</DialogTitle>
          <DialogDescription className='sr-only'>{t('Settings and inputs of this generation')}</DialogDescription>
          <Button
            variant='ghost'
            size='icon'
            className='size-7 text-muted-foreground hover:text-foreground'
            onClick={() => props.onOpenChange(false)}
            aria-label={t('Close')}
          >
            <XIcon className='size-4' />
          </Button>
        </DialogHeader>
        <div className='flex-1 overflow-y-auto p-4 space-y-4 text-xs'>
          {/* Prompt + copy */}
          <Section label={t('Prompt')}>
            <div className='flex items-start gap-2 rounded-md border border-border/60 bg-muted/30 p-2'>
              <p className='flex-1 whitespace-pre-wrap leading-snug'>{item.prompt}</p>
              <Button
                variant='ghost'
                size='icon'
                className='size-6 shrink-0 text-muted-foreground hover:text-foreground'
                onClick={copyPrompt}
                aria-label={t('Copy prompt')}
                title={t('Copy prompt')}
              >
                {copied ? <CheckIcon className='size-3.5' /> : <CopyIcon className='size-3.5' />}
              </Button>
            </div>
          </Section>

          {/* Metadata rows */}
          {rows.length > 0 && (
            <Section label={t('Settings')}>
              <div className='rounded-md border border-border/60 px-2 py-1'>
                {rows.map(([label, value]) => (
                  <SettingRow key={label} label={t(String(label))} value={value} />
                ))}
              </div>
            </Section>
          )}

          {/* Negative prompt */}
          {details?.negativePrompt && (
            <Section label={t('Negative prompt')}>
              <p className='whitespace-pre-wrap rounded-md border border-border/60 bg-muted/30 p-2 leading-snug'>
                {details.negativePrompt}
              </p>
            </Section>
          )}

          {/* Frames */}
          {(details?.startFrame || details?.endFrame) && (
            <Section label={t('Frames')}>
              <div className='flex flex-wrap gap-3'>
                {details?.startFrame && (
                  <div className='space-y-1'>
                    <p className='text-[10px] text-muted-foreground'>{t('Start frame')}</p>
                    <FrameImage src={details.startFrame} alt='start frame' />
                  </div>
                )}
                {details?.endFrame && (
                  <div className='space-y-1'>
                    <p className='text-[10px] text-muted-foreground'>{t('End frame')}</p>
                    <FrameImage src={details.endFrame} alt='end frame' />
                  </div>
                )}
              </div>
            </Section>
          )}

          {/* Reference images */}
          {details?.referenceImages && details.referenceImages.length > 0 && (
            <Section label={t('Reference images')}>
              <ReferenceStrip urls={details.referenceImages} />
            </Section>
          )}

          {/* Chained plan */}

          {/* Footer meta */}
          <div className='flex flex-wrap items-center gap-2 pt-1 text-[10px] text-muted-foreground'>
            {created && <span>{created}</span>}
            {item.taskId && <span className='font-mono'>{item.taskId}</span>}
            {item.localOnly && <Badge variant='outline' className='text-[9px]'>{t('Stored in this browser only')}</Badge>}
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}
