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
import { useQuery } from '@tanstack/react-query'
import {
  ArrowRightIcon,
  DownloadIcon,
  EyeIcon,
  FileAudioIcon,
  HistoryIcon,
  ImageIcon,
  Loader2Icon,
  MusicIcon,
  PlusIcon,
  SparklesIcon,
  Square as SquareIcon,
  Trash2Icon,
  VideoIcon,
  XIcon,
} from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import { Progress } from '@/components/ui/progress'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { getPlaygroundRuntimeConfig } from '@/features/playground/api'
import {
  EMPTY_PLAYGROUND_RUNTIME_CONFIG,
  type MediaModelRuntime,
} from '@/features/playground/types'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'

import { useStudioGeneration } from './hooks/use-studio-generation'
import type { StudioMode } from './types'

/** Serves media through local cache proxy to avoid 12-hour upstream expiration. */
function getPlayableMediaUrl(url?: string): string {
  if (!url) return ''
  if (url.startsWith('http://') || url.startsWith('https://')) {
    return `/api/media/cache?url=${encodeURIComponent(url)}`
  }
  return url
}

/** Pixel Flower emblem in classic 8-bit Google Flow style. */
function PixelFlowerIcon({ className }: { className?: string }) {
  return (
    <svg
      className={cn('size-16 text-foreground', className)}
      viewBox='0 0 32 32'
      fill='currentColor'
      xmlns='http://www.w3.org/2000/svg'
      shapeRendering='crispEdges'
    >
      {/* Top Petal */}
      <rect x='14' y='2' width='4' height='2' />
      <rect x='12' y='4' width='2' height='2' />
      <rect x='18' y='4' width='2' height='2' />
      <rect x='12' y='6' width='2' height='2' />
      <rect x='18' y='6' width='2' height='2' />
      {/* Center opening frame */}
      <rect x='14' y='8' width='4' height='4' />
      {/* Bottom Petal */}
      <rect x='12' y='12' width='2' height='2' />
      <rect x='18' y='12' width='2' height='2' />
      <rect x='12' y='14' width='2' height='2' />
      <rect x='18' y='14' width='2' height='2' />
      <rect x='14' y='16' width='4' height='2' />
      {/* Left Petal */}
      <rect x='6' y='8' width='2' height='4' />
      <rect x='8' y='6' width='2' height='2' />
      <rect x='8' y='12' width='2' height='2' />
      <rect x='10' y='6' width='2' height='2' />
      <rect x='10' y='12' width='2' height='2' />
      {/* Right Petal */}
      <rect x='24' y='8' width='2' height='4' />
      <rect x='22' y='6' width='2' height='2' />
      <rect x='22' y='12' width='2' height='2' />
      <rect x='20' y='6' width='2' height='2' />
      <rect x='20' y='12' width='2' height='2' />
      {/* Stem */}
      <rect x='15' y='16' width='2' height='12' />
      {/* Left Leaf */}
      <rect x='11' y='20' width='4' height='2' />
      <rect x='9' y='22' width='4' height='2' />
      {/* Right Leaf */}
      <rect x='17' y='20' width='4' height='2' />
      <rect x='19' y='22' width='4' height='2' />
      {/* Base */}
      <rect x='13' y='28' width='6' height='2' />
    </svg>
  )
}

function SettingsChips(props: {
  settings: Record<string, string | number | undefined>
}) {
  const entries = Object.entries(props.settings).filter(
    ([, value]) => value !== undefined && value !== ''
  )
  if (entries.length === 0) return null

  return (
    <div className='flex flex-wrap gap-1.5'>
      {entries.map(([key, value]) => (
        <span
          className='rounded-md border border-border/60 bg-muted/50 px-2 py-0.5 text-[11px] font-mono text-muted-foreground'
          key={key}
        >
          {key}: {String(value)}
        </span>
      ))}
    </div>
  )
}

function progressPercentOf(raw: string): number | null {
  const match = raw.match(/(\d+(?:[.,]\d+)?)/)
  if (!match) return null
  const value = Number(match[1].replace(',', '.'))
  if (!Number.isFinite(value)) return null
  return Math.min(100, Math.max(0, Math.round(value)))
}

type AttachedMedia = {
  url: string
  name: string
  type: 'image' | 'video' | 'audio'
}

export function Studio() {
  const { t } = useTranslation()
  const user = useAuthStore((state) => state.auth.user)
  const [mode, setMode] = useState<StudioMode>('video')
  const [prompt, setPrompt] = useState('')
  const [isDragging, setIsDragging] = useState(false)
  const [attachedMedia, setAttachedMedia] = useState<AttachedMedia | null>(null)
  const [popoverOpen, setPopoverOpen] = useState(false)

  // Track active stage item during this session (resets to null on page refresh so Google Flow empty state appears)
  const [activeSessionItemId, setActiveSessionItemId] = useState<string | null>(null)
  const [historyOpen, setHistoryOpen] = useState(false)
  const [historyTab, setHistoryTab] = useState<'all' | StudioMode>('all')

  const fileInputRef = useRef<HTMLInputElement>(null)

  const [imageModel, setImageModel] = useState('')
  const [imageSize, setImageSize] = useState('')
  const [imageQuality, setImageQuality] = useState('')
  const [imageCount] = useState(1)

  const [videoModel, setVideoModel] = useState('')
  const [videoDuration, setVideoDuration] = useState('')
  const [videoRatio, setVideoRatio] = useState('')
  const [videoQuality, setVideoQuality] = useState('')

  const [musicModel, setMusicModel] = useState('')
  const [upscaleModel, setUpscaleModel] = useState('')

  const { data: runtimeData } = useQuery({
    queryKey: ['playground-runtime-config'],
    queryFn: ({ signal }) => getPlaygroundRuntimeConfig(signal),
    staleTime: 60_000,
  })
  const runtime = runtimeData ?? EMPTY_PLAYGROUND_RUNTIME_CONFIG

  // Automatically fall back to video if selected mode is disabled
  useEffect(() => {
    if (!runtime.image.enabled && mode === 'image') {
      setMode('video')
    }
    if (!runtime.music?.enabled && mode === 'music') {
      setMode('video')
    }
    if (!runtime.upscale?.enabled && mode === 'upscale') {
      setMode('video')
    }
  }, [runtime.image.enabled, runtime.music?.enabled, runtime.upscale?.enabled, mode])

  const imageModels = useMemo<MediaModelRuntime[]>(
    () =>
      runtime.image.models?.length
        ? runtime.image.models
        : runtime.image.model
          ? [{ model: runtime.image.model }]
          : [],
    [runtime.image]
  )
  const videoModels = useMemo<MediaModelRuntime[]>(
    () =>
      runtime.video.models?.length
        ? runtime.video.models
        : runtime.video.model
          ? [{ model: runtime.video.model }]
          : [],
    [runtime.video]
  )
  const musicModels = useMemo<MediaModelRuntime[]>(
    () => runtime.music?.models || [{ model: 'music', display_name: 'music' }],
    [runtime.music]
  )
  const upscaleModels = useMemo<MediaModelRuntime[]>(
    () => runtime.upscale?.models || [{ model: 'upscale', display_name: 'upscale' }, { model: 'image-upscale', display_name: 'image-upscale' }],
    [runtime.upscale]
  )

  const selectedImageModel = useMemo(() => {
    const match = imageModels.find((entry) => entry.model === imageModel)
    return match ?? imageModels[0]
  }, [imageModels, imageModel])

  const selectedVideoModel = useMemo(() => {
    const match = videoModels.find((entry) => entry.model === videoModel)
    return match ?? videoModels[0]
  }, [videoModels, videoModel])

  const selectedMusicModel = useMemo(() => {
    const match = musicModels.find((entry) => entry.model === musicModel)
    return match ?? musicModels[0]
  }, [musicModels, musicModel])

  const selectedUpscaleModel = useMemo(() => {
    const match = upscaleModels.find((entry) => entry.model === upscaleModel)
    return match ?? upscaleModels[0]
  }, [upscaleModels, upscaleModel])

  const currentModelRuntime = useMemo(() => {
    switch (mode) {
      case 'video':
        return selectedVideoModel
      case 'image':
        return selectedImageModel
      case 'music':
        return selectedMusicModel
      case 'upscale':
        return selectedUpscaleModel
      default:
        return undefined
    }
  }, [mode, selectedVideoModel, selectedImageModel, selectedMusicModel, selectedUpscaleModel])

  const currentModelName = currentModelRuntime?.model || ''
  const currentModelDisplayName = currentModelRuntime?.display_name || currentModelName || t('Select Model')

  useEffect(() => {
    if (selectedImageModel && selectedImageModel.model !== imageModel) {
      setImageModel(selectedImageModel.model)
      setImageSize('')
      setImageQuality('')
    }
  }, [selectedImageModel, imageModel])

  useEffect(() => {
    if (selectedVideoModel) {
      if (selectedVideoModel.model !== videoModel) {
        setVideoModel(selectedVideoModel.model)
        setVideoDuration(
          selectedVideoModel.default_duration
            ? String(selectedVideoModel.default_duration)
            : ''
        )
        setVideoRatio('')
        setVideoQuality('')
      } else if (!videoDuration && selectedVideoModel.default_duration) {
        setVideoDuration(String(selectedVideoModel.default_duration))
      }
    }
  }, [selectedVideoModel, videoModel, videoDuration])

  useEffect(() => {
    if (selectedMusicModel && selectedMusicModel.model !== musicModel) {
      setMusicModel(selectedMusicModel.model)
    }
  }, [selectedMusicModel, musicModel])

  useEffect(() => {
    if (selectedUpscaleModel && selectedUpscaleModel.model !== upscaleModel) {
      setUpscaleModel(selectedUpscaleModel.model)
    }
  }, [selectedUpscaleModel, upscaleModel])

  // Clear incompatible attachments when switching between media modes
  useEffect(() => {
    if (attachedMedia) {
      if (mode === 'music' && attachedMedia.type !== 'audio') {
        setAttachedMedia(null)
      } else if (mode !== 'music' && attachedMedia.type === 'audio') {
        setAttachedMedia(null)
      }
    }
  }, [mode, attachedMedia])

  const imageSizes = selectedImageModel?.sizes?.length
    ? selectedImageModel.sizes
    : runtime.image.sizes
  const imageQualities = selectedImageModel?.qualities?.length
    ? selectedImageModel.qualities
    : runtime.image.qualities
  const size =
    imageSize || selectedImageModel?.default_size || runtime.image.default_size
  const quality =
    imageQuality ||
    selectedImageModel?.default_quality ||
    runtime.image.default_quality

  const videoDurations = selectedVideoModel?.durations?.length
    ? selectedVideoModel.durations
    : runtime.video.durations
  const videoRatios = selectedVideoModel?.ratios?.length
    ? selectedVideoModel.ratios
    : runtime.video.ratios
  const videoQualities = selectedVideoModel?.qualities?.length
    ? selectedVideoModel.qualities
    : runtime.video.qualities
  const duration = Number(
    videoDuration ||
      selectedVideoModel?.default_duration ||
      runtime.video.default_duration ||
      videoDurations[0] ||
      0
  )
  const ratio =
    videoRatio || selectedVideoModel?.default_ratio || runtime.video.default_ratio
  const videoQualityValue =
    videoQuality ||
    selectedVideoModel?.default_quality ||
    runtime.video.default_quality

  const {
    items,
    isGenerating,
    generateImage,
    generateVideo,
    stop,
    removeItem,
    clearItems,
  } = useStudioGeneration()

  const [isMobileScreen, setIsMobileScreen] = useState(false)

  useEffect(() => {
    const checkMobile = () => {
      setIsMobileScreen(window.innerWidth < 640)
    }
    checkMobile()
    window.addEventListener('resize', checkMobile)
    return () => window.removeEventListener('resize', checkMobile)
  }, [])

  const activeItem = useMemo(() => {
    if (activeSessionItemId) {
      const match = items.find((item) => item.id === activeSessionItemId)
      if (match) return match
    }
    // If an item is active/pending, always display it on the stage even after page refresh
    const pendingItem = items.find((item) => item.status === 'pending')
    if (pendingItem) return pendingItem
    return null
  }, [items, activeSessionItemId])

  // Only the active stage item being pending or an active generation blocks the send button
  const hasActivePending = isGenerating || activeItem?.status === 'pending'

  const canGenerate =
    prompt.trim().length > 0 &&
    !hasActivePending &&
    Boolean(currentModelName)

  const handleGenerate = async () => {
    if (!canGenerate || hasActivePending) return
    const text = prompt.trim()

    if (mode === 'image' || mode === 'upscale') {
      await generateImage(
        {
          prompt: text,
          model: currentModelName,
          mode,
          group: user?.group,
          size: mode === 'image' ? size : undefined,
          quality: mode === 'image' ? quality : undefined,
          count: imageCount,
        },
        (id) => setActiveSessionItemId(id)
      )
      return
    }

    await generateVideo(
      {
        prompt: text,
        model: currentModelName,
        mode,
        group: user?.group,
        duration: mode === 'video' ? duration : undefined,
        ratio: mode === 'video' ? ratio : undefined,
        quality: mode === 'video' ? videoQualityValue : undefined,
        imageUrl: attachedMedia?.url || undefined,
      },
      (id) => setActiveSessionItemId(id)
    )
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      if (canGenerate && !hasActivePending) {
        void handleGenerate()
      }
    }
  }

  const handleFileAttach = (file: File | undefined) => {
    if (!file) {
      setAttachedMedia(null)
      return
    }
    if (mode === 'music' && !file.type.startsWith('audio/')) {
      toast.error(t('Only audio files can be attached for music'))
      return
    }
    if (mode !== 'music' && file.type.startsWith('audio/')) {
      toast.error(t('Please select an image or video'))
      return
    }
    const reader = new FileReader()
    reader.onload = () => {
      if (typeof reader.result === 'string') {
        let fileType: AttachedMedia['type'] = 'image'
        if (file.type.startsWith('video/')) fileType = 'video'
        else if (file.type.startsWith('audio/')) fileType = 'audio'
        setAttachedMedia({
          url: reader.result,
          name: file.name,
          type: fileType,
        })
      }
    }
    reader.readAsDataURL(file)
  }

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault()
    setIsDragging(true)
  }
  const handleDragLeave = (e: React.DragEvent) => {
    e.preventDefault()
    setIsDragging(false)
  }
  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault()
    setIsDragging(false)
    const file = e.dataTransfer.files?.[0]
    if (file) {
      if (mode === 'music' && !file.type.startsWith('audio/')) {
        toast.error(t('Only audio files can be attached for music'))
        return
      }
      handleFileAttach(file)
    }
  }

  const filteredHistoryItems = useMemo(() => {
    if (historyTab === 'all') return items
    return items.filter((item) => item.mode === historyTab)
  }, [items, historyTab])

  if (runtimeData && !runtime.studio.enabled) {
    return (
      <div className='flex h-full flex-col items-center justify-center gap-2 p-8 text-center text-muted-foreground'>
        <SparklesIcon className='size-8' />
        <p className='text-sm'>
          {t('Studio is disabled by administrator.')}
        </p>
      </div>
    )
  }

  return (
    <div
      onDragOver={handleDragOver}
      onDragLeave={handleDragLeave}
      onDrop={handleDrop}
      className='relative flex h-full min-h-0 w-full flex-col overflow-y-auto overscroll-contain bg-background text-foreground'
    >
      {/* File Drop Overlay */}
      {isDragging && (
        <div className='absolute inset-0 z-50 flex items-center justify-center bg-background/85 backdrop-blur-sm border-2 border-dashed border-primary/50 m-4 rounded-3xl pointer-events-none'>
          <p className='text-sm font-medium'>
            {t('Drop media file here')}
          </p>
        </div>
      )}

      {/* Main Canvas Area */}
      <div className='mx-auto flex w-full max-w-4xl flex-1 flex-col justify-between p-4 md:p-8'>
        {/* Stage Area */}
        <div className='flex flex-1 flex-col justify-center py-6'>
          {activeItem ? (
            <div className='space-y-6'>
              {/* Active / Latest Generation Display Card */}
              <div className='relative mx-auto w-full max-w-2xl overflow-hidden rounded-2xl border border-border/80 bg-card shadow-sm'>
                {/* Top action buttons: Previous Generations & Dismiss/Clear */}
                <div className='absolute end-3 top-3 z-10 flex items-center gap-1.5'>
                  <Button
                    size='sm'
                    variant='ghost'
                    onClick={() => setHistoryOpen(true)}
                    className='h-7 px-2.5 text-xs text-muted-foreground hover:text-foreground bg-background/80 backdrop-blur-sm border border-border/50 hover:bg-background rounded-lg gap-1.5 shadow-xs'
                    title={t('Previous Generations')}
                  >
                    <HistoryIcon className='size-3.5' />
                    <span>{t('Previous Generations')}</span>
                  </Button>
                  <Button
                    size='sm'
                    variant='ghost'
                    onClick={() => {
                      if (activeItem) {
                        stop(activeItem.id)
                        removeItem(activeItem.id)
                      }
                      setActiveSessionItemId(null)
                    }}
                    className={cn(
                      'h-7 px-2.5 text-xs backdrop-blur-sm border border-border/50 rounded-lg gap-1.5 shadow-xs transition',
                      activeItem.status === 'pending'
                        ? 'text-destructive hover:text-destructive bg-destructive/10 hover:bg-destructive/20 border-destructive/30 font-medium'
                        : 'text-muted-foreground hover:text-foreground bg-background/80 hover:bg-background'
                    )}
                    title={activeItem.status === 'pending' ? t('Stop') : t('Delete All')}
                  >
                    {activeItem.status === 'pending' ? (
                      <>
                        <SquareIcon className='size-3 fill-current' />
                        <span>{t('Stop')}</span>
                      </>
                    ) : (
                      <>
                        <Trash2Icon className='size-3.5' />
                        <span>{t('Delete All')}</span>
                      </>
                    )}
                  </Button>
                </div>

                <div className='relative flex min-h-[220px] max-h-[380px] items-center justify-center overflow-hidden bg-muted/20'>
                  {activeItem.status === 'pending' && (
                    <div className='flex flex-col items-center justify-center gap-3 p-6 text-center'>
                      <Loader2Icon className='size-7 animate-spin text-muted-foreground' />
                      <div className='space-y-1'>
                        <p className='text-sm font-medium'>
                          {activeItem.mode === 'video'
                            ? t('Generating video...')
                            : activeItem.mode === 'music'
                              ? t('Generating music...')
                              : t('Generating image...')}
                        </p>
                        <p className='line-clamp-2 max-w-sm text-xs text-muted-foreground'>
                          &ldquo;{activeItem.prompt}&rdquo;
                        </p>
                      </div>
                      {activeItem.mode === 'video' && activeItem.progress && (
                        <div className='w-full max-w-xs space-y-1'>
                          <Progress
                            value={progressPercentOf(activeItem.progress) ?? 25}
                            className='h-1.5'
                          />
                          <p className='font-mono text-[11px] text-muted-foreground'>
                            {activeItem.progress}
                          </p>
                        </div>
                      )}
                      <Button
                        onClick={() => {
                          stop(activeItem.id)
                          removeItem(activeItem.id)
                          setActiveSessionItemId(null)
                        }}
                        size='sm'
                        variant='outline'
                        className='mt-1 h-7 rounded-lg text-xs gap-1.5 text-destructive hover:text-destructive hover:bg-destructive/10 border-destructive/30'
                      >
                        <SquareIcon className='size-3 fill-current' />
                        {t('Stop')}
                      </Button>
                    </div>
                  )}

                  {activeItem.status === 'error' && (
                    <div className='flex flex-col items-center gap-2 p-8 text-center text-destructive'>
                      <span className='flex size-8 items-center justify-center rounded-full bg-destructive/10 text-xs font-bold text-destructive'>
                        !
                      </span>
                      <p className='text-xs font-medium max-w-md'>
                        {activeItem.error || t('Generation failed')}
                      </p>
                    </div>
                  )}

                  {activeItem.status === 'ready' && (
                    <div className='w-full p-2 flex items-center justify-center'>
                      {activeItem.mode === 'music' ? (
                        <div className='w-full max-w-md p-6 flex flex-col items-center gap-4'>
                          <div className='size-14 rounded-full bg-muted flex items-center justify-center'>
                            <MusicIcon className='size-7 text-foreground' />
                          </div>
                          <audio
                            controls
                            className='w-full'
                            src={getPlayableMediaUrl(activeItem.urls[0])}
                          />
                        </div>
                      ) : activeItem.mode === 'video' ? (
                        <video
                          autoPlay
                          className='max-h-[380px] w-full object-contain bg-black rounded-lg'
                          controls
                          loop
                          playsInline
                          src={getPlayableMediaUrl(activeItem.urls[0])}
                        />
                      ) : (
                        <img
                          alt={activeItem.prompt}
                          className='max-h-[380px] w-full object-contain rounded-lg'
                          src={getPlayableMediaUrl(activeItem.urls[0])}
                        />
                      )}
                    </div>
                  )}
                </div>

                {/* Metadata & Download Bar */}
                <div className='flex flex-wrap items-center justify-between gap-3 border-t border-border/60 bg-card p-3.5'>
                  <div className='min-w-0 flex-1 space-y-1.5'>
                    <p className='truncate text-xs font-medium leading-relaxed'>
                      {activeItem.prompt}
                    </p>
                    <SettingsChips settings={activeItem.settings} />
                  </div>
                  {activeItem.status === 'ready' && activeItem.urls[0] && (
                    <a
                      download
                      href={getPlayableMediaUrl(activeItem.urls[0])}
                      rel='noreferrer'
                      target='_blank'
                      className='inline-flex items-center gap-1.5 h-8 px-3 rounded-lg border bg-background hover:bg-muted text-xs font-medium text-foreground transition shadow-xs'
                    >
                      <DownloadIcon className='size-3.5' />
                      {t('Download')}
                    </a>
                  )}
                </div>
              </div>
            </div>
          ) : (
            /* Google Flow Empty State: Pixel Flower + Subtitle + Önceki Üretimler Button */
            <div className='my-auto flex flex-col items-center justify-center gap-4 py-12 text-center select-none'>
              <PixelFlowerIcon className='size-20 transition-transform duration-300 hover:scale-105' />
              <h1 className='text-xl sm:text-2xl font-normal tracking-tight text-foreground/90 max-w-lg'>
                {t('Start creating or drop media files')}
              </h1>
              {items.length > 0 && (
                <Button
                  variant='outline'
                  size='sm'
                  onClick={() => setHistoryOpen(true)}
                  className='mt-2 inline-flex items-center gap-1.5 rounded-full border border-border/70 bg-card hover:bg-muted text-xs font-medium px-4 py-1.5 h-8 text-muted-foreground hover:text-foreground shadow-xs transition'
                >
                  <HistoryIcon className='size-3.5' />
                  <span>{t('Previous Generations')}</span>
                  <span className='rounded-full bg-muted px-1.5 py-0.2 text-[10px] font-mono text-foreground'>
                    {items.length}
                  </span>
                </Button>
              )}
            </div>
          )}
        </div>

        {/* Google Flow Floating Bottom Input Capsule */}
        <div className='sticky bottom-6 z-20 w-full pt-4'>
          {/* Attached Reference Chip (if any) positioned cleanly above the capsule */}
          {attachedMedia && (
            <div className='mb-2 flex items-center gap-2 rounded-xl bg-card/90 border border-border/60 p-1.5 pr-2.5 w-fit shadow-md backdrop-blur-md'>
              {attachedMedia.type === 'image' ? (
                <img
                  src={attachedMedia.url}
                  alt='Reference'
                  className='size-7 rounded-lg object-cover'
                />
              ) : attachedMedia.type === 'audio' ? (
                <div className='size-7 rounded-lg bg-background flex items-center justify-center'>
                  <FileAudioIcon className='size-4 text-foreground' />
                </div>
              ) : (
                <div className='size-7 rounded-lg bg-background flex items-center justify-center'>
                  <VideoIcon className='size-4 text-foreground' />
                </div>
              )}
              <span className='max-w-[200px] truncate text-xs text-muted-foreground font-medium'>
                {attachedMedia.name ||
                  (attachedMedia.type === 'audio'
                    ? t('Reference Audio')
                    : attachedMedia.type === 'video'
                      ? t('Reference Video')
                      : t('Reference Image'))}
              </span>
              <button
                type='button'
                onClick={() => setAttachedMedia(null)}
                className='text-muted-foreground hover:text-foreground transition'
              >
                <XIcon className='size-3.5' />
              </button>
            </div>
          )}

          <div className='relative flex items-center gap-1.5 sm:gap-2.5 rounded-full border border-border/70 bg-card/95 shadow-2xl backdrop-blur-xl px-2.5 sm:px-3 py-1.5 sm:py-2 transition-all focus-within:border-border'>
            {/* Left Action: Plus file attach button */}
            <label
              htmlFor='studio-file-upload'
              className='flex size-8 sm:size-9 shrink-0 cursor-pointer items-center justify-center rounded-full bg-muted/80 hover:bg-muted text-muted-foreground hover:text-foreground transition shadow-xs'
              title={mode === 'music' ? t('Add audio file') : t('Add media file')}
            >
              <PlusIcon className='size-4' />
              <input
                id='studio-file-upload'
                ref={fileInputRef}
                type='file'
                accept={mode === 'music' ? 'audio/*' : 'image/*,video/*'}
                className='hidden'
                onChange={(e) => handleFileAttach(e.target.files?.[0])}
              />
            </label>

            {/* Prompt Input / Textarea */}
            <div className='flex-1 min-w-0 flex items-center'>
              <textarea
                value={prompt}
                onChange={(e) => setPrompt(e.target.value)}
                onKeyDown={handleKeyDown}
                placeholder={
                  mode === 'music'
                    ? (isMobileScreen ? t('Describe melody...') : t('Describe lyrics, style or melody...'))
                    : (isMobileScreen ? t('Describe your idea...') : t('What do you want to create?'))
                }
                className='w-full resize-none border-0 bg-transparent py-2 px-1.5 sm:px-2 text-xs sm:text-sm focus:outline-none focus:ring-0 leading-5 placeholder:text-muted-foreground/60 overflow-hidden align-middle my-auto placeholder:truncate whitespace-nowrap'
                rows={1}
                style={{ height: '36px', maxHeight: '100px' }}
                onInput={(e) => {
                  const target = e.currentTarget
                  target.style.height = '36px'
                  target.style.height = `${Math.min(target.scrollHeight, 100)}px`
                }}
              />
            </div>

            {/* Right Controls: Model Pill & Send Arrow */}
            <div className='flex items-center gap-1.5 sm:gap-2 shrink-0'>
              {/* Model Pill Trigger displaying ONLY model name per requirements */}
              <Popover open={popoverOpen} onOpenChange={setPopoverOpen}>
                <PopoverTrigger
                  className='inline-flex h-8 sm:h-9 items-center gap-1 sm:gap-1.5 rounded-full bg-muted/70 hover:bg-muted border border-border/50 px-2.5 sm:px-3.5 text-xs font-medium text-foreground transition shadow-xs'
                >
                  <span className='max-w-[85px] sm:max-w-[140px] truncate text-[11px] sm:text-xs'>
                    {currentModelDisplayName}
                  </span>
                </PopoverTrigger>

                <PopoverContent
                  align='end'
                  side='top'
                  className='w-80 rounded-2xl border border-border/80 bg-card p-3.5 shadow-xl backdrop-blur-md space-y-3'
                >
                  {/* Mode selection segmented control */}
                  <div className='space-y-1.5'>
                    <Label className='text-xs font-medium text-foreground/80'>{t('Type')}</Label>
                    <div className='flex items-center rounded-xl bg-muted/60 p-1 border border-border/40 gap-1'>
                      {runtime.video.enabled && (
                        <button
                          type='button'
                          onClick={() => setMode('video')}
                          className={cn(
                            'flex-1 h-7 rounded-lg text-xs font-medium transition flex items-center justify-center gap-1.5',
                            mode === 'video'
                              ? 'bg-background text-foreground shadow-xs font-semibold'
                              : 'text-muted-foreground hover:text-foreground'
                          )}
                        >
                          <VideoIcon className='size-3.5' />
                          <span>{t('Video')}</span>
                        </button>
                      )}
                      {runtime.image.enabled && (
                        <button
                          type='button'
                          onClick={() => setMode('image')}
                          className={cn(
                            'flex-1 h-7 rounded-lg text-xs font-medium transition flex items-center justify-center gap-1.5',
                            mode === 'image'
                              ? 'bg-background text-foreground shadow-xs font-semibold'
                              : 'text-muted-foreground hover:text-foreground'
                          )}
                        >
                          <ImageIcon className='size-3.5' />
                          <span>{t('Image')}</span>
                        </button>
                      )}
                      {runtime.music?.enabled && (
                        <button
                          type='button'
                          onClick={() => setMode('music')}
                          className={cn(
                            'flex-1 h-7 rounded-lg text-xs font-medium transition flex items-center justify-center gap-1.5',
                            mode === 'music'
                              ? 'bg-background text-foreground shadow-xs font-semibold'
                              : 'text-muted-foreground hover:text-foreground'
                          )}
                        >
                          <span>{t('Music')}</span>
                        </button>
                      )}
                      {runtime.upscale?.enabled && (
                        <button
                          type='button'
                          onClick={() => setMode('upscale')}
                          className={cn(
                            'flex-1 h-7 rounded-lg text-xs font-medium transition flex items-center justify-center gap-1.5',
                            mode === 'upscale'
                              ? 'bg-background text-foreground shadow-xs font-semibold'
                              : 'text-muted-foreground hover:text-foreground'
                          )}
                        >
                          <span>{t('Upscale')}</span>
                        </button>
                      )}
                    </div>
                  </div>

                  {/* Model select */}
                  <div className='space-y-1.5'>
                    <Label className='text-xs font-medium text-foreground/80'>{t('Model')}</Label>
                    <Select
                      onValueChange={(val) => {
                        const v = String(val)
                        if (mode === 'video') {
                          setVideoModel(v)
                          const target = videoModels.find((m) => m.model === v)
                          if (target?.default_duration) {
                            setVideoDuration(String(target.default_duration))
                          }
                        } else if (mode === 'image') setImageModel(v)
                        else if (mode === 'music') setMusicModel(v)
                        else if (mode === 'upscale') setUpscaleModel(v)
                      }}
                      value={currentModelName}
                    >
                      <SelectTrigger className='h-9 w-full rounded-xl border border-border/60 bg-background px-3 text-xs font-medium'>
                        <SelectValue placeholder={t('Select Model')} />
                      </SelectTrigger>
                      <SelectContent>
                        {(mode === 'video'
                          ? videoModels
                          : mode === 'image'
                            ? imageModels
                            : mode === 'music'
                              ? musicModels
                              : upscaleModels
                        ).map((m) => (
                          <SelectItem key={m.model} value={m.model} className='text-xs'>
                            {m.display_name || m.model}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>

                  {/* Video parameters (Oran, Süre, Kalite) */}
                  {mode === 'video' && (
                    <div className='grid grid-cols-3 gap-2 pt-0.5'>
                      <div className='space-y-1'>
                        <Label className='text-xs font-medium text-muted-foreground'>{t('Oran')}</Label>
                        <Select onValueChange={(val) => setVideoRatio(val ?? '')} value={ratio}>
                          <SelectTrigger className='h-8 w-full rounded-xl border border-border/60 bg-background px-2.5 text-xs font-medium'>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            {videoRatios.map((r) => (
                              <SelectItem key={r} value={r} className='text-xs'>
                                {r}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </div>

                      <div className='space-y-1'>
                        <Label className='text-xs font-medium text-muted-foreground'>{t('Duration')}</Label>
                        <Select onValueChange={(val) => setVideoDuration(val ?? '')} value={String(duration)}>
                          <SelectTrigger className='h-8 w-full rounded-xl border border-border/60 bg-background px-2.5 text-xs font-medium'>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            {videoDurations.map((d) => (
                              <SelectItem key={d} value={String(d)} className='text-xs'>
                                {d}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </div>

                      <div className='space-y-1'>
                        <Label className='text-xs font-medium text-muted-foreground'>{t('Quality')}</Label>
                        <Select onValueChange={(val) => setVideoQuality(val ?? '')} value={videoQualityValue}>
                          <SelectTrigger className='h-8 w-full rounded-xl border border-border/60 bg-background px-2.5 text-xs font-medium'>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            {videoQualities.map((q) => (
                              <SelectItem key={q} value={q} className='text-xs'>
                                {q}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </div>
                    </div>
                  )}

                  {/* Image parameters */}
                  {mode === 'image' && (
                    <div className='grid grid-cols-2 gap-2 pt-0.5'>
                      <div className='space-y-1'>
                        <Label className='text-xs font-medium text-muted-foreground'>{t('Size')}</Label>
                        <Select onValueChange={(val) => setImageSize(val ?? '')} value={size}>
                          <SelectTrigger className='h-8 w-full rounded-xl border border-border/60 bg-background px-2.5 text-xs font-medium'>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            {imageSizes.map((s) => (
                              <SelectItem key={s} value={s} className='text-xs'>
                                {s}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </div>

                      <div className='space-y-1'>
                        <Label className='text-xs font-medium text-muted-foreground'>{t('Quality')}</Label>
                        <Select onValueChange={(val) => setImageQuality(val ?? '')} value={quality}>
                          <SelectTrigger className='h-8 w-full rounded-xl border border-border/60 bg-background px-2.5 text-xs font-medium'>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            {imageQualities.map((q) => (
                              <SelectItem key={q} value={q} className='text-xs'>
                                {q}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </div>
                    </div>
                  )}
                </PopoverContent>
              </Popover>

              {/* Circular Send Arrow Button */}
              <button
                type='button'
                disabled={!canGenerate || hasActivePending}
                title={
                  hasActivePending
                    ? t('An active generation is in progress. Please wait for it to complete or clear it from the stage.')
                    : !prompt.trim()
                    ? t('Please enter a prompt')
                    : !currentModelName
                    ? t('Please select a model')
                    : t('Generate')
                }
                onClick={() => void handleGenerate()}
                className='flex size-9 shrink-0 items-center justify-center rounded-full bg-foreground text-background hover:bg-foreground/90 disabled:opacity-30 disabled:cursor-not-allowed transition shadow-sm'
              >
                {isGenerating || hasActivePending ? (
                  <Loader2Icon className='size-4 animate-spin' />
                ) : (
                  <ArrowRightIcon className='size-4' />
                )}
              </button>
            </div>
          </div>
        </div>
      </div>

      {/* Previous Generations Dialog (Item 11) */}
      <Dialog open={historyOpen} onOpenChange={setHistoryOpen}>
        <DialogContent className='w-[96vw] sm:w-[95vw] md:max-w-6xl lg:max-w-7xl h-[90vh] sm:h-[85vh] max-h-[92vh] flex flex-col p-0 overflow-hidden rounded-2xl'>
          <DialogHeader className='p-3 sm:p-4 border-b flex flex-row items-center justify-between space-y-0'>
            <div>
              <DialogTitle className='text-sm sm:text-base font-semibold'>
                {t('Previous Generations')}
              </DialogTitle>
              <DialogDescription className='text-xs text-muted-foreground'>
                {t('Your previous video, image, and music creations.')}
              </DialogDescription>
            </div>
            {items.length > 0 && (
              <Button
                variant='ghost'
                size='sm'
                onClick={() => {
                  clearItems()
                  setActiveSessionItemId(null)
                }}
                className='h-8 text-xs text-muted-foreground hover:text-destructive gap-1.5'
              >
                <Trash2Icon className='size-3.5' />
                <span>{t('Clear All')}</span>
              </Button>
            )}
          </DialogHeader>

          {/* Filter tabs */}
          <div className='px-3 sm:px-4 pt-2.5 sm:pt-3 border-b pb-2 sm:pb-2.5 flex items-center justify-between gap-2 overflow-x-auto no-scrollbar'>
            <Tabs value={historyTab} onValueChange={(val) => setHistoryTab(val as typeof historyTab)} className='w-full sm:w-auto'>
              <TabsList className='inline-flex h-8 w-auto min-w-full sm:min-w-0 justify-start gap-1 p-1'>
                <TabsTrigger value='all' className='text-xs px-2.5 sm:px-3 shrink-0'>
                  {t('All')} ({items.length})
                </TabsTrigger>
                <TabsTrigger value='video' className='text-xs px-2.5 sm:px-3 shrink-0'>
                  {t('Video')} ({items.filter((i) => i.mode === 'video').length})
                </TabsTrigger>
                <TabsTrigger value='image' className='text-xs px-2.5 sm:px-3 shrink-0'>
                  {t('Image')} ({items.filter((i) => i.mode === 'image').length})
                </TabsTrigger>
                <TabsTrigger value='music' className='text-xs px-2.5 sm:px-3 shrink-0'>
                  {t('Music')} ({items.filter((i) => i.mode === 'music').length})
                </TabsTrigger>
                <TabsTrigger value='upscale' className='text-xs px-2.5 sm:px-3 shrink-0'>
                  {t('Upscale')} ({items.filter((i) => i.mode === 'upscale').length})
                </TabsTrigger>
              </TabsList>
            </Tabs>
          </div>

          {/* Items Grid */}
          <div className='flex-1 overflow-y-auto p-4'>
            {filteredHistoryItems.length === 0 ? (
              <div className='py-12 text-center text-xs text-muted-foreground'>
                {t('No previous generations yet.')}
              </div>
            ) : (
              <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4'>
                {filteredHistoryItems.map((item) => (
                  <div
                    key={item.id}
                    className='group relative flex flex-col justify-between overflow-hidden rounded-xl border border-border/70 bg-card shadow-xs transition hover:border-border'
                  >
                    <div className='p-2 pb-0 flex items-center justify-between gap-2'>
                      <Badge
                        className='gap-1 border-border/50 bg-background/80 text-[10px]'
                        variant='outline'
                      >
                        {item.mode === 'video' ? (
                          <VideoIcon className='size-2.5' />
                        ) : item.mode === 'music' ? (
                          <MusicIcon className='size-2.5' />
                        ) : (
                          <ImageIcon className='size-2.5' />
                        )}
                        {item.mode === 'video'
                          ? t('Video')
                          : item.mode === 'image'
                            ? t('Image')
                            : item.mode === 'music'
                              ? t('Music')
                              : t('Upscale')}
                      </Badge>
                      <span className='text-[10px] text-muted-foreground font-mono'>
                        {new Date(item.createdAt).toLocaleDateString()}
                      </span>
                    </div>

                    <div className='relative my-2 flex min-h-36 max-h-48 items-center justify-center overflow-hidden bg-muted/20 rounded-md mx-2'>
                      {item.status === 'pending' && (
                        <div className='flex flex-col items-center justify-center gap-1.5 p-4 text-center'>
                          <Loader2Icon className='size-5 animate-spin text-muted-foreground' />
                          <span className='text-xs text-muted-foreground'>{t('Generating...')}</span>
                        </div>
                      )}
                      {item.status === 'error' && (
                        <div className='p-3 text-center text-destructive text-xs'>
                          {item.error || t('Generation failed')}
                        </div>
                      )}
                      {item.status === 'ready' && item.urls[0] && (
                        item.mode === 'music' ? (
                          <div className='w-full p-2 flex flex-col items-center justify-center gap-2'>
                            <MusicIcon className='size-6 text-foreground' />
                            <audio controls className='w-full' src={getPlayableMediaUrl(item.urls[0])} />
                          </div>
                        ) : item.mode === 'video' ? (
                          <video
                            controls
                            playsInline
                            preload='metadata'
                            src={getPlayableMediaUrl(item.urls[0])}
                            className='max-h-48 w-full object-contain bg-black rounded'
                          />
                        ) : (
                          <img
                            alt={item.prompt}
                            src={getPlayableMediaUrl(item.urls[0])}
                            className='max-h-48 w-full object-contain rounded'
                          />
                        )
                      )}
                    </div>

                    <div className='space-y-2 p-2.5 pt-0'>
                      <p className='line-clamp-2 text-xs font-medium' title={item.prompt}>
                        {item.prompt}
                      </p>
                      <SettingsChips settings={item.settings} />

                      <div className='flex items-center justify-between gap-1 pt-2 border-t border-border/40'>
                        <Button
                          size='sm'
                          variant='outline'
                          onClick={() => {
                            setActiveSessionItemId(item.id)
                            setHistoryOpen(false)
                          }}
                          className='h-7 px-2 text-[11px] gap-1'
                        >
                          <EyeIcon className='size-3' />
                          {t('Show on Stage')}
                        </Button>

                        <div className='flex items-center gap-1'>
                          {item.status === 'ready' && item.urls[0] && (
                            <a
                              download
                              href={getPlayableMediaUrl(item.urls[0])}
                              rel='noreferrer'
                              target='_blank'
                              className='inline-flex items-center justify-center size-7 rounded-md border bg-background hover:bg-muted text-foreground transition'
                              title={t('Download')}
                            >
                              <DownloadIcon className='size-3.5' />
                            </a>
                          )}
                          <Button
                            size='icon'
                            variant='ghost'
                            onClick={() => {
                              removeItem(item.id)
                              if (activeSessionItemId === item.id) {
                                setActiveSessionItemId(null)
                              }
                            }}
                            className='size-7 text-muted-foreground hover:text-destructive'
                            title={t('Delete')}
                          >
                            <Trash2Icon className='size-3.5' />
                          </Button>
                        </div>
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </DialogContent>
      </Dialog>
    </div>
  )
}
