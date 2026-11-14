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
    <div className='group bg-card relative overflow-hidden rounded-2xl border shadow-sm transition duration-300 hover:-translate-y-0.5 hover:border-primary/40 hover:shadow-lg hover:shadow-primary/5'>
      {/* Mode ribbon */}
      <div className='pointer-events-none absolute inset-x-0 top-0 h-[3px] bg-gradient-to-r from-primary/70 via-sky-400/60 to-transparent' />
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
            <div
              aria-hidden='true'
              className='pointer-events-none absolute inset-0 animate-pulse bg-gradient-to-br from-primary/20 via-transparent to-sky-500/10'
            />
            <div
              aria-hidden='true'
              className='absolute inset-x-6 top-0 h-px animate-pulse bg-gradient-to-r from-transparent via-primary/70 to-transparent'
            />
            <div className='relative flex flex-col items-center gap-3'>
              <div className='relative flex size-12 items-center justify-center'>
                <span
                  aria-hidden='true'
                  className='absolute inset-0 animate-ping rounded-full bg-primary/15'
                />
                <Loader2Icon className='text-primary size-6 animate-spin' />
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
                    className='h-1.5 [&>div]:bg-gradient-to-r [&>div]:from-primary [&>div]:to-sky-400'
                  />
                  <p className='text-primary text-center font-mono text-[11px] tabular-nums'>
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
  const [mode, setMode] = useState<StudioMode>('image')
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

  return (
    // The authenticated layout gives every page a fixed height and hides the
    // overflow, so a page that grows must scroll itself. Without this wrapper
    // the studio was clipped: the settings card was taller than the viewport
    // and nothing on the page could be scrolled.
    <div className='flex h-full min-h-0 w-full flex-col overflow-y-auto overscroll-contain'>
      <div className='mx-auto flex w-full max-w-[1500px] flex-col gap-6 p-4 md:p-6'>
        <header className='border-primary/20 from-primary/15 via-primary/5 to-background relative flex flex-wrap items-start justify-between gap-4 overflow-hidden rounded-2xl border bg-gradient-to-br p-4 md:p-5'>
          <div aria-hidden='true' className='pointer-events-none absolute -top-16 end-10 size-48 rounded-full bg-primary/20 blur-3xl' />
          <div aria-hidden='true' className='pointer-events-none absolute -bottom-20 start-24 size-48 rounded-full bg-sky-400/15 blur-3xl' />
          <div className='relative space-y-1'>
            <h1 className='flex items-center gap-2 text-xl font-semibold'>
              <span className='bg-primary/15 text-primary flex size-9 items-center justify-center rounded-xl'>
                <SparklesIcon className='size-5' />
              </span>
              {t('Studio')}
            </h1>
            <p className='text-muted-foreground max-w-2xl text-sm'>
              {t(
                'Create images and videos with the models configured by the administrator. Everything you generate stays in this page.'
              )}
            </p>
          </div>

          <div className='relative flex flex-wrap items-center gap-2'>
            {/* Rendered as a segmented control instead of a tab list: the tab
                list inherited the transparent background of the header and the
                two modes were invisible. */}
            <div
              aria-label={t('Output type')}
              className='bg-background/80 inline-flex rounded-lg border p-1 shadow-sm'
              role='tablist'
            >
              {(
                [
                  { value: 'image', label: t('Image'), icon: ImageIcon },
                  { value: 'video', label: t('Video'), icon: VideoIcon },
                ] as const
              ).map((entry) => (
                <button
                  aria-selected={mode === entry.value}
                  className={cn(
                    'inline-flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm font-medium transition',
                    mode === entry.value
                      ? 'bg-primary text-primary-foreground shadow-md shadow-primary/30'
                      : 'text-muted-foreground hover:text-foreground'
                  )}
                  key={entry.value}
                  onClick={() => setMode(entry.value as StudioMode)}
                  role='tab'
                  type='button'
                >
                  <entry.icon className='size-4' />
                  {entry.label}
                </button>
              ))}
            </div>
            <Button
              disabled={modeItems.length === 0}
              onClick={() => clearItems(mode)}
              size='sm'
              variant='outline'
            >
              <Trash2Icon className='size-4' />
              {t('Clear')}
            </Button>
          </div>
        </header>

        <div className='grid min-w-0 items-start gap-6 lg:grid-cols-[minmax(0,380px)_minmax(0,1fr)]'>
          <section className='bg-card min-w-0 space-y-4 overflow-hidden rounded-2xl border p-4 shadow-sm'>
          <div className='-mx-4 -mt-4 flex items-center justify-between gap-2 border-b bg-gradient-to-r from-primary/10 to-transparent px-4 py-2.5'>
            <div className='flex items-center gap-2 text-xs font-semibold tracking-wide text-primary uppercase'>
              <SparklesIcon className='size-3.5' />
              {t('Composer')}
            </div>
            <span className='text-muted-foreground bg-background/80 rounded-full border px-2 py-0.5 font-mono text-[11px] tabular-nums'>
              {prompt.length} {t('characters')}
            </span>
          </div>
          <div className='space-y-2'>
            <Label htmlFor='studio-prompt' className='text-xs font-medium'>
              {t('Prompt')}
            </Label>
            <Textarea
              className='min-h-36 resize-y border-primary/10 bg-background/60 leading-relaxed shadow-inner transition focus-visible:border-primary/40 focus-visible:ring-primary/20'
              id='studio-prompt'
              onChange={(event) => setPrompt(event.target.value)}
              placeholder={
                mode === 'video'
                  ? t('Describe the scene, the motion and the camera movement…')
                  : t('Describe the image you want to create…')
              }
              value={prompt}
            />
          </div>

          <div className='space-y-1'>
            <Label className='text-xs font-medium'>{t('Model')}</Label>
            {mode === 'image' ? (
              <Select
                onValueChange={(value) => {
                  setImageModel(String(value))
                  setImageSize('')
                  setImageQuality('')
                }}
                value={selectedImageModel?.model ?? ''}
              >
                <SelectTrigger className='w-full'>
                  <SelectValue placeholder={t('Not configured')} />
                </SelectTrigger>
                <SelectContent>
                  {imageModels.map((entry) => (
                    <SelectItem key={entry.model} value={entry.model}>
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
                <SelectTrigger className='w-full'>
                  <SelectValue placeholder={t('Not configured')} />
                </SelectTrigger>
                <SelectContent>
                  {videoModels.map((entry) => (
                    <SelectItem key={entry.model} value={entry.model}>
                      {entry.model}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          </div>

          {mode === 'image' ? (
            <div className='grid gap-3 sm:grid-cols-2'>
              <OptionSelect
                label={t('Size')}
                onChange={setImageSize}
                options={imageSizes}
                value={size}
              />
              <OptionSelect
                label={t('Quality')}
                onChange={setImageQuality}
                options={imageQualities}
                value={quality}
              />
              <div className='space-y-1 sm:col-span-2'>
                <Label className='text-xs'>{t('Images')}</Label>
                <Input
                  max={4}
                  min={1}
                  onChange={(event) =>
                    setImageCount(
                      Math.min(4, Math.max(1, Number(event.target.value) || 1))
                    )
                  }
                  type='number'
                  value={imageCount}
                />
              </div>
            </div>
          ) : (
            <div className='grid gap-3 sm:grid-cols-2'>
              <OptionSelect
                format={(value) => `${value}s`}
                label={t('Duration')}
                onChange={setVideoDuration}
                options={videoDurations.map(String)}
                value={String(duration)}
              />
              <OptionSelect
                label={t('Aspect ratio')}
                onChange={setVideoRatio}
                options={videoRatios}
                value={ratio}
              />
              <OptionSelect
                label={t('Quality')}
                onChange={setVideoQuality}
                options={videoQualities}
                value={videoQualityValue}
              />
              {supportsReference && (
                <div className='space-y-1 sm:col-span-2'>
                  <Label htmlFor='studio-reference-image'>
                    {t('Reference image (optional)')}
                  </Label>
                  <Input
                    accept='image/*'
                    id='studio-reference-image'
                    onChange={(event) =>
                      handleReferenceFile(event.target.files?.[0])
                    }
                    type='file'
                  />
                  {referenceImage && (
                    <div className='flex items-center gap-2 pt-1'>
                      <img
                        alt={t('Reference image')}
                        className='size-14 rounded-md border object-cover'
                        src={referenceImage}
                      />
                      <Button
                        onClick={() => setReferenceImage('')}
                        size='sm'
                        variant='ghost'
                      >
                        {t('Remove')}
                      </Button>
                    </div>
                  )}
                </div>
              )}
            </div>
          )}

          <div className='flex gap-2'>
            <Button
              className='from-primary to-primary/75 text-primary-foreground h-11 flex-1 bg-gradient-to-r text-[15px] font-semibold shadow-md shadow-primary/25 transition hover:from-primary/90 hover:to-primary/65 hover:shadow-lg hover:shadow-primary/30'
              disabled={!canGenerate}
              onClick={handleGenerate}
            >
              {isGenerating ? (
                <Loader2Icon className='size-4 animate-spin' />
              ) : (
                <SparklesIcon className='size-4' />
              )}
              {isGenerating ? t('Generating…') : t('Generate')}
            </Button>
            {isGenerating && (
              <Button onClick={stop} variant='outline'>
                {t('Stop')}
              </Button>
            )}
          </div>

          {modeDisabled && (
            <p className='bg-muted/50 text-muted-foreground rounded-lg px-3 py-2 text-xs'>
              {mode === 'video'
                ? t('Video generation is disabled by the administrator.')
                : t('Image generation is disabled by the administrator.')}
            </p>
          )}
        </section>

        <section className='space-y-4'>
          <div className='flex items-center justify-between border-b pb-2'>
            <h2 className='text-muted-foreground flex items-center gap-2 text-[11px] font-semibold tracking-widest uppercase'>
              {mode === 'video' ? (
                <VideoIcon className='size-3.5 text-primary' />
              ) : (
                <ImageIcon className='size-3.5 text-primary' />
              )}
              {mode === 'video' ? t('Videos') : t('Images')}
            </h2>
            {modeItems.length > 0 && (
              <span className='text-muted-foreground bg-muted rounded-full px-2 py-0.5 font-mono text-[11px] tabular-nums'>
                {t('{{count}} result(s)', { count: modeItems.length })}
              </span>
            )}
          </div>

          {modeItems.length === 0 ? (
            <div className='from-primary/5 via-background to-background relative flex flex-col items-center justify-center overflow-hidden rounded-2xl border border-dashed bg-gradient-to-b p-10 text-center md:p-14'>
              <div
                aria-hidden='true'
                className='pointer-events-none absolute -top-20 left-1/2 size-64 -translate-x-1/2 rounded-full bg-primary/10 blur-3xl'
              />
              <div className='relative flex flex-col items-center gap-4'>
                <div className='from-primary/25 to-sky-400/20 ring-primary/10 flex size-20 items-center justify-center rounded-3xl bg-gradient-to-br ring-8'>
                  {mode === 'video' ? (
                    <VideoIcon className='text-primary size-9' />
                  ) : (
                    <ImageIcon className='text-primary size-9' />
                  )}
                </div>
                <div className='space-y-1'>
                  <p className='text-base font-semibold'>
                    {mode === 'video'
                      ? t('Your generated videos will appear here.')
                      : t('Your generated images will appear here.')}
                  </p>
                  <p className='text-muted-foreground mx-auto max-w-sm text-xs'>
                    {t(
                      'Try one of these prompts to get started, then tune the model and options in the composer.'
                    )}
                  </p>
                </div>
                <div className='flex flex-wrap items-center justify-center gap-2'>
                  {(mode === 'video'
                    ? SUGGESTED_VIDEO_PROMPTS
                    : SUGGESTED_IMAGE_PROMPTS
                  ).map((suggestion) => (
                    <button
                      className='text-muted-foreground hover:border-primary/40 hover:text-primary bg-background/80 rounded-full border px-3 py-1.5 text-xs shadow-sm transition'
                      key={suggestion}
                      onClick={() => setPrompt(suggestion)}
                      type='button'
                    >
                      {suggestion}
                    </button>
                  ))}
                </div>
              </div>
            </div>
          ) : (
            <div className='grid gap-4 sm:grid-cols-2 2xl:grid-cols-3'>
              {modeItems.map((item) => (
                <ResultCard
                  item={item}
                  key={item.id}
                  onRemove={() => removeItem(item.id)}
                />
              ))}
            </div>
          )}
        </section>
        </div>
      </div>
    </div>
  )
}
