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
import { Link } from '@tanstack/react-router'
import { CopyIcon, DownloadIcon, SparklesIcon } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { VideoJsPlayer } from '@/features/studio/components/video-js-player'

import { copyImageAttachment, downloadImageAttachment } from '../../lib'
import type { MessageAttachment } from '../../types'

type PlaygroundMessageAttachmentsProps = {
  attachments: MessageAttachment[]
  /** The studio hint under generated media, shown inside the box by default. */
  showStudioHint?: boolean
  /** Suppresses video attachments when the video generation tool failed. */
  hasVideoToolError?: boolean
}

function isVideoAttachment(attachment: MessageAttachment): boolean {
  if (attachment.mediaType?.startsWith('video')) return true
  return /\.(mp4|webm|mov|m4v)(\?|$)/i.test(attachment.url)
}

function isPlayableMediaUrl(raw?: string): boolean {
  const url = (raw ?? '').trim()
  if (!url) return false
  return (
    url.startsWith('http://') ||
    url.startsWith('https://') ||
    url.startsWith('/') ||
    url.startsWith('blob:') ||
    url.startsWith('data:')
  )
}

function getPlayableVideoUrl(url: string): string {
  const trimmed = url.trim()
  if (trimmed.startsWith('http://') || trimmed.startsWith('https://')) {
    return `/api/media/cache?url=${encodeURIComponent(trimmed)}`
  }
  return trimmed
}

export function PlaygroundMessageAttachments(
  props: PlaygroundMessageAttachmentsProps
) {
  const { t } = useTranslation()
  const [preview, setPreview] = useState<MessageAttachment | null>(null)
  const [brokenVideoUrls, setBrokenVideoUrls] = useState<Set<string>>(
    () => new Set()
  )

  const markVideoBroken = (url: string) => {
    setBrokenVideoUrls((prev) => {
      if (prev.has(url)) return prev
      const next = new Set(prev)
      next.add(url)
      return next
    })
  }

  const visibleAttachments = props.attachments.filter((attachment) => {
    if (!isPlayableMediaUrl(attachment.url)) return false
    if (isVideoAttachment(attachment)) {
      if (props.hasVideoToolError) return false
      if (brokenVideoUrls.has(attachment.url)) return false
    }
    return true
  })

  if (visibleAttachments.length === 0) {
    return null
  }

  const handleDownload = async (
    attachment: MessageAttachment,
    index: number
  ) => {
    try {
      await downloadImageAttachment(attachment, index)
    } catch {
      toast.error(t('Could not download the image'))
    }
  }

  const handleCopy = async (attachment: MessageAttachment) => {
    try {
      const result = await copyImageAttachment(attachment)
      toast.success(
        result === 'image' ? t('Image copied') : t('Image link copied')
      )
    } catch {
      toast.error(t('Could not copy the image'))
    }
  }

  const studioHintFooter =
    props.showStudioHint !== false ? (
      <div className='border-border/60 bg-muted/20 flex flex-wrap items-center gap-1.5 border-t px-3 py-2 text-xs text-muted-foreground'>
        <SparklesIcon className='size-3.5 shrink-0' />
        <span>
          {t('Want more control over the result?')}{' '}
          <Link
            className='text-foreground font-medium underline underline-offset-2'
            to='/studio'
          >
            {t('Open Studio')}
          </Link>{' '}
          {t(
            'to set the model, size, duration, ratio and quality of images and videos.'
          )}
        </span>
      </div>
    ) : null

  return (
    <>
      <div className='mb-3 flex flex-col gap-2.5'>
        {visibleAttachments.map((attachment, index) => {
          const isVideo = isVideoAttachment(attachment)

          if (isVideo) {
            const playableSrc = getPlayableVideoUrl(attachment.url)
            return (
              <div
                className='border-border/70 bg-card w-full max-w-2xl overflow-hidden rounded-xl border shadow-xs'
                key={attachment.url}
              >
                <div className='relative bg-black'>
                  <VideoJsPlayer
                    key={playableSrc}
                    autoPlay={false}
                    loop={false}
                    className='max-h-[380px] w-full bg-black object-contain'
                    src={playableSrc}
                    onError={() => markVideoBroken(attachment.url)}
                  />
                </div>
                {studioHintFooter}
              </div>
            )
          }

          return (
            <div
              className='border-border/70 bg-card w-fit max-w-full overflow-hidden rounded-xl border shadow-xs'
              key={attachment.url}
            >
              <div className='group relative'>
                <button
                  className='block cursor-zoom-in'
                  onClick={() => setPreview(attachment)}
                  type='button'
                >
                  <img
                    alt={attachment.filename || t('Attachment')}
                    className='max-h-64 max-w-full object-contain'
                    src={attachment.url}
                  />
                  <span className='sr-only'>{t('Preview image')}</span>
                </button>
                <div className='absolute top-1.5 right-1.5 flex gap-1 opacity-0 transition-opacity group-hover:opacity-100 focus-within:opacity-100'>
                  <Button
                    aria-label={t('Copy image')}
                    className='bg-background/80 size-7 backdrop-blur'
                    onClick={() => void handleCopy(attachment)}
                    size='icon-sm'
                    type='button'
                    variant='outline'
                  >
                    <CopyIcon className='size-3.5' />
                  </Button>
                  <Button
                    aria-label={t('Download image')}
                    className='bg-background/80 size-7 backdrop-blur'
                    onClick={() => void handleDownload(attachment, index)}
                    size='icon-sm'
                    type='button'
                    variant='outline'
                  >
                    <DownloadIcon className='size-3.5' />
                  </Button>
                </div>
              </div>
              {studioHintFooter}
            </div>
          )
        })}
      </div>

      <Dialog
        open={preview !== null}
        onOpenChange={(open) => !open && setPreview(null)}
      >
        <DialogContent className='sm:max-w-3xl'>
          <DialogHeader>
            <DialogTitle className='truncate'>
              {preview?.filename || t('Attachment')}
            </DialogTitle>
          </DialogHeader>
          {preview && (
            <img
              alt={preview.filename || t('Attachment')}
              className='max-h-[70vh] w-full object-contain'
              src={preview.url}
            />
          )}
        </DialogContent>
      </Dialog>
    </>
  )
}
