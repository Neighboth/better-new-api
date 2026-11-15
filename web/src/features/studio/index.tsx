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
  DownloadIcon,
  ImageIcon,
  Loader2Icon,
  SparklesIcon,
  Trash2Icon,
  VideoIcon,
  XIcon,
} from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Progress } from '@/components/ui/progress'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
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

const SUGGESTED_IMAGE_PROMPTS = [
  'A lighthouse on a cliff during a dramatic storm, oil painting',
  'Minimal product shot of headphones on concrete, studio lighting',
  'A fox curled up in a sunbeam in a library, photorealistic',
]

const SUGGESTED_VIDEO_PROMPTS = [
  'A paper plane gliding through a sunlit old library',
  'Timelapse of a thunderstorm gathering over the sea',
  'A cat leaping across rooftops at sunset, cinematic',
]

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
          className='bg-muted text-muted-foreground rounded-full px-2 py-0.5 text-[11px]'
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
    <div className='group bg-card relative overflow-hidden rounded-xl border'>

      <Badge
        className='absolute start-2 top-3 z-10 gap-1 border-background/60 bg-background/80 backdrop-blur'
        variant={item.mode === 'video' ? 'secondary' : 'outline'}
      >
        {item.mode === 'video' ? (
          <VideoIcon className='size-3' />
        ) : (
          <ImageIcon className='size-3' />
        )}
        {item.mode === 'video' ? t('Video') : t('Image')}
      </Badge>

      <div className='bg-muted/40 relative flex min-h-44 items-center justify-center overflow-hidden'>
        {item.status === 'pending' && (
          <div className='relative flex min-h-44 w-full flex-col items-center justify-center gap-3 px-6 py-12'>
            <div className='relative flex flex-col items-center gap-3'>
              <div className='relative flex size-10 items-center justify-center'>
                <Loader2Icon className='text-muted-foreground size-5 animate-spin' />
              </div>
              <p className='text-muted-foreground max-w-[16rem] text-center text-xs leading-relaxed'>
                {item.mode === 'video'
                  ? t('Rendering the video, this can take a few minutes…')
                  : t('Generating the image…')}
              </p>
              {item.mode === 'video' && progressPercent !== null && (
                <div className='w-full max-w-[14rem] space-y-1.5'>
                  <Progress
                    value={progressPercent}
                    className='h-1.5'
                  />
                  <p className='text-muted-foreground text-center font-mono text-[11px] tabular-nums'>
                    {progressPercent}%
                  </p>
                </div>
              )}
            </div>
          </div>
        )}
        {item.status === 'error' && (
          <div className='flex w-full flex-col items-center gap-2 bg-destructive/5 px-4 py-12 text-center'>
            <span className='flex size-10 items-center justify-center rounded-full bg-destructive/10 text-destructive text-lg'>
              !
            </span>
            <p className='text-destructive/90 max-w-[18rem] text-xs leading-relaxed'>
              {item.error || t('The generation failed')}
            </p>
          </div>
        )}
        {item.status === 'ready' &&
          item.urls.map((url) =>
            item.mode === 'video' ? (
              <video
                className='max-h-[420px] w-full'
                controls
                key={url}
                playsInline
                preload='metadata'
                src={url}
              />
            ) : (
              <img
                alt={item.prompt}
                className='max-h-[420px] w-full object-contain transition duration-500 group-hover:scale-[1.02]'
                key={url}
                src={url}
              />
            )
          )}
      </div>

      <div className='space-y-2 p-3.5'>
        <p className='line-clamp-2 text-sm'>{item.prompt}</p>
        <SettingsChips settings={item.settings} />

        <div className='flex items-center justify-between gap-2 pt-1'>
          <div className='flex flex-wrap gap-2'>
            {item.status === 'ready' &&
              item.urls.map((url, index) => (
                <Button
                  key={url}
                  size='sm'
                  variant='outline'
                  render={
                    <a
                      download
                      href={url}
                      rel='noreferrer'
                      target='_blank'
                      aria-label={t('Download')}
                    />
                  }
                >
                  <DownloadIcon className='size-3.5' />
                  {t('Download')}
                  {item.urls.length > 1 ? ` ${index + 1}` : ''}
                </Button>
              ))}
          </div>
          <Button
            aria-label={t('Remove')}
            onClick={props.onRemove}
            size='icon-sm'
            variant='ghost'
          >
            <XIcon className='size-4' />
          </Button>
        </div>
      </div>
    </div>
  )
}

/** Select of one option of the current media model. */
function OptionSelect(props: {
  label: string
  value: string
  options: string[]
  format?: (value: string) => string
  onChange: (value: string) => void
  disabled?: boolean
}) {
  if (props.options.length === 0) return null
  return (
    <div className='space-y-1'>
      <Label className='text-xs'>{props.label}</Label>
      <Select
        disabled={props.disabled}
        onValueChange={(next) => props.onChange(String(next))}
        value={props.value || String(props.options[0])}
      >
        <SelectTrigger className='h-9 w-full'>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {props.options.map((option) => (
            <SelectItem key={option} value={option}>
              {props.format ? props.format(option) : option}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  )
}

export function Studio() {
  const { t } = useTranslation()
  const user = useAuthStore((state) => state.auth.user)
  const [mode, setMode] = useState<StudioMode>('video')
  const [prompt, setPrompt] = useState('')

  const [imageModel, setImageModel] = useState('')
  const [imageSize, setImageSize] = useState('')
  const [imageQuality, setImageQuality] = useState('')
  const [imageCount, setImageCount] = useState(1)

  const [videoModel, setVideoModel] = useState('')
  const [videoDuration, setVideoDuration] = useState('')
  const [videoRatio, setVideoRatio] = useState('')
  const [videoQuality, setVideoQuality] = useState('')
  const [referenceImage, setReferenceImage] = useState('')

  const { data: runtimeData } = useQuery({
    queryKey: ['playground-runtime-config'],
    queryFn: ({ signal }) => getPlaygroundRuntimeConfig(signal),
    staleTime: 60_000,
  })
  const runtime = runtimeData ?? EMPTY_PLAYGROUND_RUNTIME_CONFIG

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

  // Keep a valid selection when the configuration arrives or the mode
  // changes; every model brings its own options, so the chosen values reset
  // to the defaults of the newly selected model.
  const selectedImageModel = useMemo(() => {
    const match = imageModels.find((entry) => entry.model === imageModel)
    return match ?? imageModels[0]
  }, [imageModels, imageModel])
  const selectedVideoModel = useMemo(() => {
    const match = videoModels.find((entry) => entry.model === videoModel)
    return match ?? videoModels[0]
  }, [videoModels, videoModel])

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
  const supportsReference =
    selectedVideoModel?.reference_image ?? runtime.video.reference_image

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
    (mode === 'image'
      ? runtime.image.enabled && Boolean(selectedImageModel)
      : runtime.video.enabled && Boolean(selectedVideoModel))

  const handleGenerate = () => {
    if (!canGenerate) return
    const text = prompt.trim()

    if (mode === 'image') {
      void generateImage({
        prompt: text,
        model: selectedImageModel?.model ?? '',
        group: user?.group,
        size,
        quality,
        count: imageCount,
      })
      return
    }

    void generateVideo({
      prompt: text,
      model: selectedVideoModel?.model ?? '',
      group: user?.group,
      duration,
      ratio,
      quality: videoQualityValue,
      imageUrl: referenceImage || undefined,
    })
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

  if (runtimeData && !runtime.studio.enabled) {
    return (
      <div className='text-muted-foreground flex h-full flex-col items-center justify-center gap-2 p-8 text-center'>
        <SparklesIcon className='size-8' />
        <p className='text-sm'>
          {t('The studio is disabled by the administrator.')}
        </p>
      </div>
    )
  }

  const modeDisabled =
    mode === 'image' ? !runtime.image.enabled : !runtime.video.enabled

  const latestItem = modeItems[0]

  return (
    <div className='flex h-full min-h-0 w-full flex-col overflow-y-auto overscroll-contain bg-background'>
      <div className='mx-auto flex w-full max-w-[1400px] flex-col gap-6 p-4 md:p-6'>
        {/* Google Flow Top Docked Control Toolbar */}
        <div className='flex flex-wrap items-center justify-between gap-3 rounded-2xl border bg-card/60 p-2.5 backdrop-blur-md shadow-sm'>
          {/* Mode Selector Pill */}
          <div
            aria-label={t('Output type')}
            className='inline-flex items-center rounded-xl bg-muted/60 p-1 border'
            role='tablist'
          >
            {(
              [
                { value: 'video', label: t('Video'), icon: VideoIcon },
                { value: 'image', label: t('Image'), icon: ImageIcon },
              ] as const
            ).map((entry) => (
              <button
                aria-selected={mode === entry.value}
                className={cn(
                  'inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-medium transition-all',
                  mode === entry.value
                    ? 'bg-background text-foreground shadow-sm'
                    : 'text-muted-foreground hover:text-foreground'
                )}
                key={entry.value}
                onClick={() => setMode(entry.value as StudioMode)}
                role='tab'
                type='button'
              >
                <entry.icon className='size-3.5' />
                {entry.label}
              </button>
            ))}
          </div>

          {/* Model and Generation Parameter Pills */}
          <div className='flex flex-wrap items-center gap-2'>
            {/* Model Pill */}
            <div className='min-w-[160px]'>
              {mode === 'image' ? (
                <Select
                  onValueChange={(value) => {
                    setImageModel(String(value))
                    setImageSize('')
                    setImageQuality('')
                  }}
                  value={selectedImageModel?.model ?? ''}
                >
                  <SelectTrigger className='h-8 text-xs rounded-lg'>
                    <SelectValue placeholder={t('Model')} />
                  </SelectTrigger>
                  <SelectContent>
                    {imageModels.map((entry) => (
                      <SelectItem key={entry.model} value={entry.model} className='text-xs'>
                        {entry.model}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              ) : (
                <Select
                  onValueChange={(value) => {
                    setVideoModel(String(value))
                    setVideoDuration('')
                    setVideoRatio('')
                    setVideoQuality('')
                  }}
                  value={selectedVideoModel?.model ?? ''}
                >
                  <SelectTrigger className='h-8 text-xs rounded-lg'>
                    <SelectValue placeholder={t('Model')} />
                  </SelectTrigger>
                  <SelectContent>
                    {videoModels.map((entry) => (
                      <SelectItem key={entry.model} value={entry.model} className='text-xs'>
                        {entry.model}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              )}
            </div>

            {/* Mode-specific Option Pills */}
            {mode === 'video' ? (
              <>
                {videoRatios.length > 0 && (
                  <Select onValueChange={setVideoRatio} value={ratio}>
                    <SelectTrigger className='h-8 text-xs rounded-lg w-[100px]'>
                      <SelectValue placeholder={t('Ratio')} />
                    </SelectTrigger>
                    <SelectContent>
                      {videoRatios.map((r) => (
                        <SelectItem key={r} value={r} className='text-xs'>
                          {r}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
                {videoDurations.length > 0 && (
                  <Select onValueChange={setVideoDuration} value={String(duration)}>
                    <SelectTrigger className='h-8 text-xs rounded-lg w-[90px]'>
                      <SelectValue placeholder={t('Duration')} />
                    </SelectTrigger>
                    <SelectContent>
                      {videoDurations.map((d) => (
                        <SelectItem key={d} value={String(d)} className='text-xs'>
                          {d}s
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
                {videoQualities.length > 0 && (
                  <Select onValueChange={setVideoQuality} value={videoQualityValue}>
                    <SelectTrigger className='h-8 text-xs rounded-lg w-[95px]'>
                      <SelectValue placeholder={t('Quality')} />
                    </SelectTrigger>
                    <SelectContent>
                      {videoQualities.map((q) => (
                        <SelectItem key={q} value={q} className='text-xs'>
                          {q}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
              </>
            ) : (
              <>
                {imageSizes.length > 0 && (
                  <Select onValueChange={setImageSize} value={size}>
                    <SelectTrigger className='h-8 text-xs rounded-lg w-[110px]'>
                      <SelectValue placeholder={t('Size')} />
                    </SelectTrigger>
                    <SelectContent>
                      {imageSizes.map((s) => (
                        <SelectItem key={s} value={s} className='text-xs'>
                          {s}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
                {imageQualities.length > 0 && (
                  <Select onValueChange={setImageQuality} value={quality}>
                    <SelectTrigger className='h-8 text-xs rounded-lg w-[95px]'>
                      <SelectValue placeholder={t('Quality')} />
                    </SelectTrigger>
                    <SelectContent>
                      {imageQualities.map((q) => (
                        <SelectItem key={q} value={q} className='text-xs'>
                          {q}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
              </>
            )}

            <Button
              disabled={modeItems.length === 0}
              onClick={() => clearItems(mode)}
              size='sm'
              variant='ghost'
              className='h-8 px-2.5 text-xs text-muted-foreground hover:text-destructive'
            >
              <Trash2Icon className='size-3.5 mr-1' />
              {t('Clear')}
            </Button>
          </div>
        </div>

        {/* Central Creative Stage / Showcase Hero */}
        {latestItem ? (
          <div className='relative overflow-hidden rounded-2xl border bg-card/40 shadow-sm'>
            <div className='bg-muted/30 relative flex min-h-[380px] max-h-[560px] items-center justify-center overflow-hidden'>
              {latestItem.status === 'pending' && (
                <div className='flex flex-col items-center justify-center gap-4 p-8 text-center'>
                  <div className='relative flex size-14 items-center justify-center rounded-2xl bg-primary/10 text-primary shadow-sm'>
                    <Loader2Icon className='size-7 animate-spin' />
                  </div>
                  <div className='space-y-1.5'>
                    <p className='text-sm font-medium'>
                      {latestItem.mode === 'video'
                        ? t('Generating cinematic video with AI...')
                        : t('Rendering image...')}
                    </p>
                    <p className='text-xs text-muted-foreground max-w-md line-clamp-2'>
                      &ldquo;{latestItem.prompt}&rdquo;
                    </p>
                  </div>
                  {latestItem.mode === 'video' && latestItem.progress && (
                    <div className='w-full max-w-xs space-y-1'>
                      <Progress value={progressPercentOf(latestItem.progress) ?? 20} className='h-2' />
                      <p className='text-[11px] font-mono text-muted-foreground'>
                        {latestItem.progress}
                      </p>
                    </div>
                  )}
                  {isGenerating && (
                    <Button onClick={stop} size='sm' variant='outline' className='mt-2'>
                      {t('Cancel')}
                    </Button>
                  )}
                </div>
              )}

              {latestItem.status === 'error' && (
                <div className='flex flex-col items-center gap-3 p-8 text-center'>
                  <div className='flex size-12 items-center justify-center rounded-full bg-destructive/10 text-destructive text-xl font-bold'>
                    !
                  </div>
                  <p className='text-sm font-medium text-destructive'>
                    {latestItem.error || t('The generation failed')}
                  </p>
                </div>
              )}

              {latestItem.status === 'ready' &&
                latestItem.urls.map((url) =>
                  latestItem.mode === 'video' ? (
                    <video
                      autoPlay
                      className='max-h-[560px] w-full rounded-xl object-contain bg-black/90'
                      controls
                      key={url}
                      loop
                      playsInline
                      src={url}
                    />
                  ) : (
                    <img
                      alt={latestItem.prompt}
                      className='max-h-[560px] w-full rounded-xl object-contain'
                      key={url}
                      src={url}
                    />
                  )
                )}
            </div>

            {/* Hero metadata & download bar */}
            <div className='flex flex-wrap items-center justify-between gap-3 border-t bg-card/80 p-4'>
              <div className='space-y-1 min-w-0 flex-1'>
                <p className='text-sm font-medium leading-relaxed truncate'>{latestItem.prompt}</p>
                <SettingsChips settings={latestItem.settings} />
              </div>
              {latestItem.status === 'ready' && latestItem.urls[0] && (
                <Button asChild size='sm' variant='outline'>
                  <a download href={latestItem.urls[0]} rel='noreferrer' target='_blank'>
                    <DownloadIcon className='size-3.5 mr-1.5' />
                    {t('Download')}
                  </a>
                </Button>
              )}
            </div>
          </div>
        ) : (
          /* Empty State Inspired by Google Flow */
          <div className='relative flex min-h-[320px] flex-col items-center justify-center rounded-2xl border border-dashed bg-card/30 p-8 text-center'>
            <div className='flex size-16 items-center justify-center rounded-2xl bg-gradient-to-tr from-blue-500/10 to-indigo-500/20 text-blue-500 shadow-sm mb-4'>
              {mode === 'video' ? <VideoIcon className='size-8' /> : <ImageIcon className='size-8' />}
            </div>
            <h2 className='text-lg font-semibold tracking-tight'>
              {mode === 'video' ? t('Google Flow Video Studio') : t('Generative Image Studio')}
            </h2>
            <p className='text-xs text-muted-foreground max-w-md mt-1 mb-6'>
              {mode === 'video'
                ? t('Describe your vision, camera motion, and cinematic scene to bring it to life with generative video models.')
                : t('Describe what you want to create and let advanced image diffusion models produce high quality renders.')}
            </p>
            <div className='flex flex-wrap justify-center gap-2 max-w-2xl'>
              {(mode === 'video' ? SUGGESTED_VIDEO_PROMPTS : SUGGESTED_IMAGE_PROMPTS).map(
                (suggestion) => (
                  <button
                    key={suggestion}
                    type='button'
                    onClick={() => setPrompt(suggestion)}
                    className='rounded-full border border-zinc-700/50 bg-card/60 px-3.5 py-1.5 text-xs text-zinc-300 hover:bg-zinc-800 hover:text-white transition shadow-sm'
                  >
                    ✨ {suggestion}
                  </button>
                )
              )}
            </div>
          </div>
        )}

        {/* Google Flow Floating Bottom Prompt Composer Bar */}
        <div className='relative rounded-2xl border bg-card/90 p-4 shadow-xl backdrop-blur-xl'>
          {supportsReference && mode === 'video' && (
            <div className='mb-3 flex items-center gap-2'>
              <label
                htmlFor='studio-ref-upload'
                className='inline-flex cursor-pointer items-center gap-1.5 rounded-lg border border-zinc-700/50 bg-zinc-800/40 px-2.5 py-1 text-xs text-zinc-300 hover:bg-zinc-800'
              >
                <span>📷</span>
                <span>{referenceImage ? t('Change Reference Image') : t('Add Reference Image (Image-to-Video)')}</span>
                <input
                  id='studio-ref-upload'
                  type='file'
                  accept='image/*'
                  className='hidden'
                  onChange={(e) => handleReferenceFile(e.target.files?.[0])}
                />
              </label>
              {referenceImage && (
                <div className='flex items-center gap-2'>
                  <img
                    src={referenceImage}
                    alt='Reference'
                    className='size-7 rounded border object-cover'
                  />
                  <Button
                    size='sm'
                    variant='ghost'
                    className='h-6 px-1.5 text-xs text-muted-foreground hover:text-destructive'
                    onClick={() => setReferenceImage('')}
                  >
                    {t('Remove')}
                  </Button>
                </div>
              )}
            </div>
          )}

          <div className='flex items-end gap-3'>
            <div className='flex-1'>
              <Textarea
                id='studio-prompt'
                value={prompt}
                onChange={(e) => setPrompt(e.target.value)}
                placeholder={
                  mode === 'video'
                    ? t('Describe camera movement, lighting, subjects, and cinematic scene actions...')
                    : t('Describe the image details, artistic style, camera lens, and composition...')
                }
                className='min-h-[72px] resize-none border-0 bg-transparent p-0 text-sm focus-visible:ring-0 leading-relaxed placeholder:text-muted-foreground/60'
              />
            </div>

            <div className='flex items-center gap-2 shrink-0'>
              {isGenerating ? (
                <Button onClick={stop} variant='outline' size='sm' className='h-10 px-4'>
                  {t('Stop')}
                </Button>
              ) : (
                <Button
                  disabled={!canGenerate}
                  onClick={handleGenerate}
                  className='h-10 rounded-xl px-5 gap-2 bg-blue-600 hover:bg-blue-700 text-white font-medium shadow-sm transition'
                >
                  <SparklesIcon className='size-4' />
                  {t('Generate')}
                </Button>
              )}
            </div>
          </div>
        </div>

        {/* Gallery / Previous Creations Grid */}
        {modeItems.length > 1 && (
          <div className='space-y-4 pt-2'>
            <div className='flex items-center justify-between border-b pb-2'>
              <h3 className='text-xs font-semibold tracking-wider uppercase text-muted-foreground flex items-center gap-2'>
                {mode === 'video' ? <VideoIcon className='size-3.5' /> : <ImageIcon className='size-3.5' />}
                {t('Creation History')} ({modeItems.length})
              </h3>
            </div>
            <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4'>
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
    </div>
  )
}
