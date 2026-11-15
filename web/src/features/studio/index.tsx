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
  ImageIcon,
  Loader2Icon,
  PlusIcon,
  SparklesIcon,
  Trash2Icon,
  VideoIcon,
  XIcon,
} from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
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
import { Textarea } from '@/components/ui/textarea'
import { getPlaygroundRuntimeConfig } from '@/features/playground/api'
import {
  EMPTY_PLAYGROUND_RUNTIME_CONFIG,
  type MediaModelRuntime,
} from '@/features/playground/types'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'

import { useStudioGeneration } from './hooks/use-studio-generation'
import type { StudioItem, StudioMode } from './types'

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

function ResultCard(props: { item: StudioItem; onRemove: () => void }) {
  const { t } = useTranslation()
  const { item } = props
  const progressPercent = item.progress ? progressPercentOf(item.progress) : null

  return (
    <div className='group relative overflow-hidden rounded-xl border border-border/70 bg-card shadow-xs transition hover:border-border'>
      <div className='absolute start-2 top-2 z-10 flex items-center gap-1.5'>
        <Badge
          className='gap-1 border-border/50 bg-background/80 backdrop-blur text-[10px]'
          variant='outline'
        >
          {item.mode === 'video' ? (
            <VideoIcon className='size-2.5' />
          ) : (
            <ImageIcon className='size-2.5' />
          )}
          {item.mode === 'video'
            ? t('Video')
            : item.mode === 'image'
              ? t('Görsel')
              : item.mode === 'music'
                ? t('Müzik')
                : t('Upscale')}
        </Badge>
      </div>

      <div className='relative flex min-h-36 max-h-52 items-center justify-center overflow-hidden bg-muted/20'>
        {item.status === 'pending' && (
          <div className='flex flex-col items-center justify-center gap-2 p-6'>
            <Loader2Icon className='size-5 animate-spin text-muted-foreground' />
            <p className='text-center text-xs text-muted-foreground'>
              {item.mode === 'video' ? t('Video üretiliyor...') : t('Görsel üretiliyor...')}
            </p>
            {item.mode === 'video' && progressPercent !== null && (
              <p className='font-mono text-[10px] text-muted-foreground'>
                {progressPercent}%
              </p>
            )}
          </div>
        )}

        {item.status === 'error' && (
          <div className='p-4 text-center text-destructive text-xs'>
            {item.error || t('Üretim başarısız oldu')}
          </div>
        )}

        {item.status === 'ready' &&
          item.urls.map((url) =>
            item.mode === 'video' ? (
              <video
                className='max-h-52 w-full object-contain bg-black'
                controls
                key={url}
                playsInline
                preload='metadata'
                src={url}
              />
            ) : (
              <img
                alt={item.prompt}
                className='max-h-52 w-full object-contain'
                key={url}
                src={url}
              />
            )
          )}
      </div>

      <div className='space-y-1.5 p-2.5'>
        <p className='line-clamp-2 text-xs font-medium'>{item.prompt}</p>
        <SettingsChips settings={item.settings} />

        <div className='flex items-center justify-between gap-2 pt-1 border-t'>
          <div className='flex items-center gap-1'>
            {item.status === 'ready' &&
              item.urls.map((url, index) => (
                <a
                  key={url}
                  download
                  href={url}
                  rel='noreferrer'
                  target='_blank'
                  className='inline-flex items-center gap-1 h-6 px-2 rounded-md border bg-background hover:bg-muted text-[11px] font-medium text-foreground transition'
                >
                  <DownloadIcon className='size-3' />
                  {t('İndir')}
                  {item.urls.length > 1 ? ` ${index + 1}` : ''}
                </a>
              ))}
          </div>
          <Button
            aria-label={t('Sil')}
            onClick={props.onRemove}
            size='icon'
            variant='ghost'
            className='size-6 text-muted-foreground hover:text-destructive'
          >
            <XIcon className='size-3' />
          </Button>
        </div>
      </div>
    </div>
  )
}

export function Studio() {
  const { t } = useTranslation()
  const user = useAuthStore((state) => state.auth.user)
  const [mode, setMode] = useState<StudioMode>('video')
  const [prompt, setPrompt] = useState('')
  const [isDragging, setIsDragging] = useState(false)

  const [imageModel, setImageModel] = useState('')
  const [imageSize, setImageSize] = useState('')
  const [imageQuality, setImageQuality] = useState('')
  const [imageCount] = useState(1)

  const [videoModel, setVideoModel] = useState('')
  const [videoDuration, setVideoDuration] = useState('')
  const [videoRatio, setVideoRatio] = useState('')
  const [videoQuality, setVideoQuality] = useState('')
  const [referenceImage, setReferenceImage] = useState('')

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
    () => runtime.music?.models || [{ model: 'music' }],
    [runtime.music]
  )
  const upscaleModels = useMemo<MediaModelRuntime[]>(
    () => runtime.upscale?.models || [{ model: 'upscale' }, { model: 'image-upscale' }],
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
  const currentModelDisplayName = currentModelRuntime?.display_name || currentModelName || t('Model Seç')

  useEffect(() => {
    if (selectedImageModel && selectedImageModel.model !== imageModel) {
      setImageModel(selectedImageModel.model)
      setImageSize('')
      setImageQuality('')
    }
  }, [selectedImageModel, imageModel])

  useEffect(() => {
    if (selectedVideoModel && selectedVideoModel.model !== videoModel) {
      setVideoModel(selectedVideoModel.model)
      setVideoDuration('')
      setVideoRatio('')
      setVideoQuality('')
    }
  }, [selectedVideoModel, videoModel])

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
    videoDuration || videoDurations[0] || runtime.video.default_duration || 0
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

  const modeItems = useMemo(
    () => items.filter((item) => item.mode === mode),
    [items, mode]
  )

  const canGenerate =
    prompt.trim().length > 0 &&
    !isGenerating &&
    Boolean(currentModelName)

  const handleGenerate = () => {
    if (!canGenerate) return
    const text = prompt.trim()

    if (mode === 'image' || mode === 'upscale') {
      void generateImage({
        prompt: text,
        model: currentModelName,
        group: user?.group,
        size: mode === 'image' ? size : undefined,
        quality: mode === 'image' ? quality : undefined,
        count: imageCount,
      })
      return
    }

    void generateVideo({
      prompt: text,
      model: currentModelName,
      group: user?.group,
      duration: mode === 'video' ? duration : undefined,
      ratio: mode === 'video' ? ratio : undefined,
      quality: mode === 'video' ? videoQualityValue : undefined,
      imageUrl: referenceImage || undefined,
    })
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleGenerate()
    }
  }

  const handleReferenceFile = (file: File | undefined) => {
    if (!file) {
      setReferenceImage('')
      return
    }
    const reader = new FileReader()
    reader.onload = () => {
      if (typeof reader.result === 'string') {
        setReferenceImage(reader.result)
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
      handleReferenceFile(file)
    }
  }

  if (runtimeData && !runtime.studio.enabled) {
    return (
      <div className='flex h-full flex-col items-center justify-center gap-2 p-8 text-center text-muted-foreground'>
        <SparklesIcon className='size-8' />
        <p className='text-sm'>
          {t('Stüdyo yönetici tarafından devre dışı bırakıldı.')}
        </p>
      </div>
    )
  }

  const latestItem = modeItems[0]

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
            {t('Medya dosyasını buraya bırakın')}
          </p>
        </div>
      )}

      {/* Main Canvas Area */}
      <div className='mx-auto flex w-full max-w-4xl flex-1 flex-col justify-between p-4 md:p-8'>
        {/* Stage Area */}
        <div className='flex flex-1 flex-col justify-center py-6'>
          {latestItem ? (
            <div className='space-y-6'>
              {/* Active / Latest Generation Display */}
              <div className='relative mx-auto w-full max-w-2xl overflow-hidden rounded-2xl border border-border/80 bg-card shadow-sm'>
                <div className='relative flex min-h-[200px] max-h-[360px] items-center justify-center overflow-hidden bg-muted/20'>
                  {latestItem.status === 'pending' && (
                    <div className='flex flex-col items-center justify-center gap-3 p-6 text-center'>
                      <Loader2Icon className='size-7 animate-spin text-muted-foreground' />
                      <div className='space-y-1'>
                        <p className='text-sm font-medium'>
                          {latestItem.mode === 'video'
                            ? t('Video üretiliyor...')
                            : t('Görsel üretiliyor...')}
                        </p>
                        <p className='line-clamp-2 max-w-sm text-xs text-muted-foreground'>
                          &ldquo;{latestItem.prompt}&rdquo;
                        </p>
                      </div>
                      {latestItem.mode === 'video' && latestItem.progress && (
                        <div className='w-full max-w-xs space-y-1'>
                          <Progress
                            value={progressPercentOf(latestItem.progress) ?? 25}
                            className='h-1.5'
                          />
                          <p className='font-mono text-[11px] text-muted-foreground'>
                            {latestItem.progress}
                          </p>
                        </div>
                      )}
                      {isGenerating && (
                        <Button
                          onClick={stop}
                          size='sm'
                          variant='outline'
                          className='mt-1 h-7 rounded-lg text-xs'
                        >
                          {t('Durdur')}
                        </Button>
                      )}
                    </div>
                  )}

                  {latestItem.status === 'error' && (
                    <div className='flex flex-col items-center gap-2 p-6 text-center text-destructive'>
                      <span className='flex size-8 items-center justify-center rounded-full bg-destructive/10 text-xs font-bold'>
                        !
                      </span>
                      <p className='text-xs font-medium'>
                        {latestItem.error || t('Üretim başarısız oldu')}
                      </p>
                    </div>
                  )}

                  {latestItem.status === 'ready' &&
                    latestItem.urls.map((url) =>
                      latestItem.mode === 'video' ? (
                        <video
                          autoPlay
                          className='max-h-[360px] w-full object-contain bg-black'
                          controls
                          key={url}
                          loop
                          playsInline
                          src={url}
                        />
                      ) : (
                        <img
                          alt={latestItem.prompt}
                          className='max-h-[360px] w-full object-contain'
                          key={url}
                          src={url}
                        />
                      )
                    )}
                </div>

                {/* Metadata & Download Bar */}
                <div className='flex flex-wrap items-center justify-between gap-3 border-t border-border/60 bg-card p-3'>
                  <div className='min-w-0 flex-1 space-y-1'>
                    <p className='truncate text-xs font-medium leading-relaxed'>
                      {latestItem.prompt}
                    </p>
                    <SettingsChips settings={latestItem.settings} />
                  </div>
                  {latestItem.status === 'ready' && latestItem.urls[0] && (
                    <a
                      download
                      href={latestItem.urls[0]}
                      rel='noreferrer'
                      target='_blank'
                      className='inline-flex items-center gap-1.5 h-8 px-3 rounded-lg border bg-background hover:bg-muted text-xs font-medium text-foreground transition shadow-xs'
                    >
                      <DownloadIcon className='size-3.5' />
                      {t('İndir')}
                    </a>
                  )}
                </div>
              </div>

              {/* History / Past Creations Gallery */}
              {modeItems.length > 1 && (
                <div className='space-y-3 pt-4'>
                  <div className='flex items-center justify-between border-b border-border/60 pb-2'>
                    <h3 className='text-xs font-semibold uppercase tracking-wider text-muted-foreground'>
                      {t('Önceki Üretimler')} ({modeItems.length})
                    </h3>
                    <Button
                      onClick={() => clearItems(mode)}
                      size='sm'
                      variant='ghost'
                      className='h-7 text-xs text-muted-foreground hover:text-destructive'
                    >
                      <Trash2Icon className='mr-1 size-3' />
                      {t('Temizle')}
                    </Button>
                  </div>
                  <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-3'>
                    {modeItems.slice(1).map((item) => (
                      <ResultCard
                        item={item}
                        key={item.id}
                        onRemove={() => removeItem(item.id)}
                      />
                    ))}
                  </div>
                </div>
              )}
            </div>
          ) : (
            /* Google Flow Empty State: Pixel Flower + Minimal Title */
            <div className='my-auto flex flex-col items-center justify-center gap-6 py-12 text-center select-none'>
              <PixelFlowerIcon className='size-20 transition-transform duration-300 hover:scale-105' />
              <h1 className='text-xl sm:text-2xl font-normal tracking-tight text-foreground/90 max-w-lg'>
                {t('Oluşturmaya başlayın veya medya dosyası bırakın')}
              </h1>
            </div>
          )}
        </div>

        {/* Google Flow Floating Bottom Input Capsule */}
        <div className='sticky bottom-6 z-20 w-full pt-4'>
          <div className='relative rounded-3xl border border-border/60 bg-card/95 shadow-2xl backdrop-blur-xl p-3.5 transition-all focus-within:border-border'>
            {/* Attached Reference Chip (if any) */}
            {referenceImage && (
              <div className='mb-2.5 flex items-center gap-2 rounded-xl bg-muted/60 p-1.5 pr-2.5 w-fit border border-border/40'>
                <img
                  src={referenceImage}
                  alt='Reference'
                  className='size-7 rounded-lg object-cover'
                />
                <span className='text-xs text-muted-foreground font-medium'>
                  {t('Referans Görsel')}
                </span>
                <button
                  type='button'
                  onClick={() => setReferenceImage('')}
                  className='text-muted-foreground hover:text-foreground transition'
                >
                  <XIcon className='size-3.5' />
                </button>
              </div>
            )}

            {/* Prompt Textarea */}
            <div className='w-full px-2 py-1'>
              <Textarea
                value={prompt}
                onChange={(e) => setPrompt(e.target.value)}
                onKeyDown={handleKeyDown}
                placeholder={t('Ne oluşturmak istiyorsunuz?')}
                className='min-h-[48px] max-h-[140px] resize-none border-0 bg-transparent px-2 py-1 text-sm focus-visible:ring-0 leading-relaxed placeholder:text-muted-foreground/50'
                rows={1}
              />
            </div>

            {/* Controls Row */}
            <div className='mt-2.5 flex items-center justify-between gap-2 pt-1 border-t border-border/30'>
              {/* Left Action Buttons */}
              <div className='flex items-center gap-1.5'>
                {/* Plus file attach button */}
                <label
                  htmlFor='studio-file-upload'
                  className='flex size-8 cursor-pointer items-center justify-center rounded-full bg-muted/80 hover:bg-muted text-muted-foreground hover:text-foreground transition'
                  title={t('Medya ekle')}
                >
                  <PlusIcon className='size-4' />
                  <input
                    id='studio-file-upload'
                    type='file'
                    accept='image/*,video/*'
                    className='hidden'
                    onChange={(e) => handleReferenceFile(e.target.files?.[0])}
                  />
                </label>
              </div>

              {/* Right Controls: Model Pill & Send Arrow */}
              <div className='flex items-center gap-2'>
                {/* Model & Config Pill with Popover */}
                <Popover>
                  <PopoverTrigger
                    className='inline-flex h-8 items-center gap-1.5 rounded-full bg-muted/70 hover:bg-muted border border-border/50 px-3 text-xs font-medium text-foreground transition'
                  >
                    <span className='max-w-[150px] truncate'>
                      {currentModelDisplayName}
                    </span>
                    {mode === 'video' && ratio && (
                      <>
                        <span className='text-muted-foreground/60'>⬝</span>
                        <span className='text-muted-foreground'>{ratio}</span>
                      </>
                    )}
                    {mode === 'video' && duration > 0 && (
                      <>
                        <span className='text-muted-foreground/60'>⬝</span>
                        <span className='text-muted-foreground'>{duration}s</span>
                      </>
                    )}
                    {mode === 'video' && videoQualityValue && (
                      <>
                        <span className='text-muted-foreground/60'>⬝</span>
                        <span className='text-muted-foreground'>{videoQualityValue}</span>
                      </>
                    )}
                    {mode === 'image' && size && (
                      <>
                        <span className='text-muted-foreground/60'>⬝</span>
                        <span className='text-muted-foreground'>{size}</span>
                      </>
                    )}
                  </PopoverTrigger>

                  <PopoverContent
                    align='end'
                    side='top'
                    className='w-72 space-y-2.5 rounded-xl border bg-card p-3 shadow-lg backdrop-blur-md'
                  >
                    {/* Mode selection tabs */}
                    <div className='space-y-1'>
                      <Label className='text-[11px] text-muted-foreground'>{t('Tür')}</Label>
                      <div className='flex flex-wrap gap-1 rounded-lg bg-muted/60 p-1 border'>
                        {runtime.video.enabled && (
                          <button
                            type='button'
                            onClick={() => setMode('video')}
                            className={cn(
                              'flex-1 h-6 px-2 inline-flex items-center justify-center gap-1 rounded-md text-[11px] font-medium transition',
                              mode === 'video'
                                ? 'bg-background text-foreground shadow-xs'
                                : 'text-muted-foreground hover:text-foreground'
                            )}
                          >
                            <VideoIcon className='size-3' />
                            {t('Video')}
                          </button>
                        )}
                        {runtime.image.enabled && (
                          <button
                            type='button'
                            onClick={() => setMode('image')}
                            className={cn(
                              'flex-1 h-6 px-2 inline-flex items-center justify-center gap-1 rounded-md text-[11px] font-medium transition',
                              mode === 'image'
                                ? 'bg-background text-foreground shadow-xs'
                                : 'text-muted-foreground hover:text-foreground'
                            )}
                          >
                            <ImageIcon className='size-3' />
                            {t('Görsel')}
                          </button>
                        )}
                        {runtime.music?.enabled && (
                          <button
                            type='button'
                            onClick={() => setMode('music')}
                            className={cn(
                              'flex-1 h-6 px-2 inline-flex items-center justify-center gap-1 rounded-md text-[11px] font-medium transition',
                              mode === 'music'
                                ? 'bg-background text-foreground shadow-xs'
                                : 'text-muted-foreground hover:text-foreground'
                            )}
                          >
                            {t('Müzik')}
                          </button>
                        )}
                        {runtime.upscale?.enabled && (
                          <button
                            type='button'
                            onClick={() => setMode('upscale')}
                            className={cn(
                              'flex-1 h-6 px-2 inline-flex items-center justify-center gap-1 rounded-md text-[11px] font-medium transition',
                              mode === 'upscale'
                                ? 'bg-background text-foreground shadow-xs'
                                : 'text-muted-foreground hover:text-foreground'
                            )}
                          >
                            {t('Upscale')}
                          </button>
                        )}
                      </div>
                    </div>

                    {/* Model select */}
                    <div className='space-y-1'>
                      <Label className='text-[11px] text-muted-foreground'>{t('Model')}</Label>
                      <Select
                        onValueChange={(val) => {
                          const v = String(val)
                          if (mode === 'video') setVideoModel(v)
                          else if (mode === 'image') setImageModel(v)
                          else if (mode === 'music') setMusicModel(v)
                          else if (mode === 'upscale') setUpscaleModel(v)
                        }}
                        value={currentModelName}
                      >
                        <SelectTrigger className='h-7 text-xs rounded-md'>
                          <SelectValue placeholder={t('Model Seç')} />
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

                    {/* Video parameters */}
                    {mode === 'video' && (
                      <div className='grid grid-cols-3 gap-1.5 pt-0.5'>
                        {videoRatios.length > 0 && (
                          <div className='space-y-0.5'>
                            <Label className='text-[10px] text-muted-foreground'>{t('Oran')}</Label>
                            <Select onValueChange={(val) => setVideoRatio(val ?? '')} value={ratio}>
                              <SelectTrigger className='h-7 text-xs rounded-md'>
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
                        )}
                        {videoDurations.length > 0 && (
                          <div className='space-y-0.5'>
                            <Label className='text-[10px] text-muted-foreground'>{t('Süre')}</Label>
                            <Select onValueChange={(val) => setVideoDuration(val ?? '')} value={String(duration)}>
                              <SelectTrigger className='h-7 text-xs rounded-md'>
                                <SelectValue />
                              </SelectTrigger>
                              <SelectContent>
                                {videoDurations.map((d) => (
                                  <SelectItem key={d} value={String(d)} className='text-xs'>
                                    {d}s
                                  </SelectItem>
                                ))}
                              </SelectContent>
                            </Select>
                          </div>
                        )}
                        {videoQualities.length > 0 && (
                          <div className='space-y-0.5'>
                            <Label className='text-[10px] text-muted-foreground'>{t('Kalite')}</Label>
                            <Select onValueChange={(val) => setVideoQuality(val ?? '')} value={videoQualityValue}>
                              <SelectTrigger className='h-7 text-xs rounded-md'>
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
                        )}
                      </div>
                    )}

                    {/* Image parameters */}
                    {mode === 'image' && (
                      <div className='grid grid-cols-2 gap-1.5 pt-0.5'>
                        {imageSizes.length > 0 && (
                          <div className='space-y-0.5'>
                            <Label className='text-[10px] text-muted-foreground'>{t('Boyut')}</Label>
                            <Select onValueChange={(val) => setImageSize(val ?? '')} value={size}>
                              <SelectTrigger className='h-7 text-xs rounded-md'>
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
                        )}
                        {imageQualities.length > 0 && (
                          <div className='space-y-0.5'>
                            <Label className='text-[10px] text-muted-foreground'>{t('Kalite')}</Label>
                            <Select onValueChange={(val) => setImageQuality(val ?? '')} value={quality}>
                              <SelectTrigger className='h-7 text-xs rounded-md'>
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
                        )}
                      </div>
                    )}
                  </PopoverContent>
                </Popover>

                {/* Circular Send Arrow Button */}
                <button
                  type='button'
                  disabled={!canGenerate}
                  onClick={handleGenerate}
                  className='flex size-8 items-center justify-center rounded-full bg-foreground text-background hover:bg-foreground/90 disabled:opacity-30 disabled:pointer-events-none transition shadow-sm'
                >
                  {isGenerating ? (
                    <Loader2Icon className='size-4 animate-spin' />
                  ) : (
                    <ArrowRightIcon className='size-4' />
                  )}
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
