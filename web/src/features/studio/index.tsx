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
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

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
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import { getPlaygroundRuntimeConfig } from '@/features/playground/api'
import { EMPTY_PLAYGROUND_RUNTIME_CONFIG } from '@/features/playground/types'
import { useAuthStore } from '@/stores/auth-store'

import { useStudioGeneration } from './hooks/use-studio-generation'
import type { StudioItem, StudioMode } from './types'

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

function ResultCard(props: {
  item: StudioItem
  onRemove: () => void
}) {
  const { t } = useTranslation()
  const { item } = props

  return (
    <div className='bg-card space-y-3 rounded-xl border p-3'>
      <div className='bg-muted/40 flex min-h-40 items-center justify-center overflow-hidden rounded-lg'>
        {item.status === 'pending' && (
          <div className='text-muted-foreground flex items-center gap-2 py-12 text-sm'>
            <Loader2Icon className='size-4 animate-spin' />
            {item.mode === 'video'
              ? t('Rendering the video, this can take a few minutes…')
              : t('Generating the image…')}
          </div>
        )}
        {item.status === 'error' && (
          <p className='text-destructive px-4 py-12 text-center text-sm'>
            {item.error || t('The generation failed')}
          </p>
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
                className='max-h-[420px] w-full object-contain'
                key={url}
                src={url}
              />
            )
          )}
      </div>

      <p className='line-clamp-2 text-sm'>{item.prompt}</p>
      <SettingsChips settings={item.settings} />

      <div className='flex items-center justify-between gap-2'>
        <div className='flex gap-2'>
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
  )
}

export function Studio() {
  const { t } = useTranslation()
  const user = useAuthStore((state) => state.auth.user)
  const [mode, setMode] = useState<StudioMode>('image')
  const [prompt, setPrompt] = useState('')

  const [imageSize, setImageSize] = useState('')
  const [imageQuality, setImageQuality] = useState('')
  const [imageCount, setImageCount] = useState(1)

  const [videoDuration, setVideoDuration] = useState(0)
  const [videoRatio, setVideoRatio] = useState('')
  const [videoQuality, setVideoQuality] = useState('')
  const [referenceImage, setReferenceImage] = useState('')

  const { data: runtimeData } = useQuery({
    queryKey: ['playground-runtime-config'],
    queryFn: ({ signal }) => getPlaygroundRuntimeConfig(signal),
    staleTime: 60_000,
  })
  const runtime = runtimeData ?? EMPTY_PLAYGROUND_RUNTIME_CONFIG

  const imageModel = runtime.image.model
  const videoModel = runtime.video.model
  const size = imageSize || runtime.image.default_size
  const quality = imageQuality || runtime.image.default_quality
  const duration = videoDuration || runtime.video.default_duration
  const ratio = videoRatio || runtime.video.default_ratio
  const videoQualityValue = videoQuality || runtime.video.default_quality

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
      ? runtime.image.enabled && Boolean(imageModel)
      : runtime.video.enabled && Boolean(videoModel))

  const handleGenerate = () => {
    if (!canGenerate) return
    const text = prompt.trim()

    if (mode === 'image') {
      void generateImage({
        prompt: text,
        model: imageModel,
        group: user?.group,
        size,
        quality,
        count: imageCount,
      })
      return
    }

    void generateVideo({
      prompt: text,
      model: videoModel,
      group: user?.group,
      duration,
      ratio,
      quality: videoQualityValue,
      imageUrl: referenceImage || undefined,
    })
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

  return (
    <div className='mx-auto flex w-full max-w-7xl flex-col gap-5 p-4 md:p-6'>
      <header className='flex flex-wrap items-center justify-between gap-3'>
        <div className='space-y-1'>
          <h1 className='flex items-center gap-2 text-xl font-semibold'>
            <SparklesIcon className='size-5' />
            {t('Studio')}
          </h1>
          <p className='text-muted-foreground text-sm'>
            {t(
              'Create images and videos with the models configured by the administrator. Everything you generate stays in this page.'
            )}
          </p>
        </div>

        <div className='flex items-center gap-2'>
          <Tabs
            onValueChange={(value) => setMode(value as StudioMode)}
            value={mode}
          >
            <TabsList>
              <TabsTrigger value='image'>
                <ImageIcon className='mr-1 size-4' />
                {t('Image')}
              </TabsTrigger>
              <TabsTrigger value='video'>
                <VideoIcon className='mr-1 size-4' />
                {t('Video')}
              </TabsTrigger>
            </TabsList>
          </Tabs>
          <Button
            onClick={() => clearItems(mode)}
            size='sm'
            variant='outline'
            disabled={modeItems.length === 0}
          >
            <Trash2Icon className='size-4' />
            {t('Clear')}
          </Button>
        </div>
      </header>

      <div className='grid gap-5 lg:grid-cols-[360px_1fr]'>
        <section className='bg-card h-fit space-y-4 rounded-xl border p-4 lg:sticky lg:top-4'>
          <div className='space-y-2'>
            <Label htmlFor='studio-prompt'>{t('Prompt')}</Label>
            <Textarea
              className='min-h-32 resize-y'
              id='studio-prompt'
              onChange={(event) => setPrompt(event.target.value)}
              placeholder={
                mode === 'video'
                  ? t(
                      'Describe the scene, the motion and the camera movement…'
                    )
                  : t('Describe the image you want to create…')
              }
              value={prompt}
            />
          </div>

          {mode === 'image' ? (
            <div className='space-y-3'>
              <div className='space-y-1'>
                <Label>{t('Model')}</Label>
                <p className='bg-muted/50 rounded-md px-2 py-1.5 font-mono text-xs'>
                  {imageModel || t('Not configured')}
                </p>
              </div>

              <div className='space-y-1'>
                <Label>{t('Size')}</Label>
                <Select
                  onValueChange={(value) => setImageSize(String(value))}
                  value={size}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {runtime.image.sizes.map((option) => (
                      <SelectItem key={option} value={option}>
                        {option}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className='space-y-1'>
                <Label>{t('Quality')}</Label>
                <Select
                  onValueChange={(value) => setImageQuality(String(value))}
                  value={quality}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {runtime.image.qualities.map((option) => (
                      <SelectItem key={option} value={option}>
                        {option}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className='space-y-1'>
                <Label>{t('Images')}</Label>
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
            <div className='space-y-3'>
              <div className='space-y-1'>
                <Label>{t('Model')}</Label>
                <p className='bg-muted/50 rounded-md px-2 py-1.5 font-mono text-xs'>
                  {videoModel || t('Not configured')}
                </p>
              </div>

              <div className='space-y-1'>
                <Label>{t('Duration')}</Label>
                <Select
                  onValueChange={(value) => setVideoDuration(Number(value))}
                  value={String(duration)}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {runtime.video.durations.map((option) => (
                      <SelectItem key={option} value={String(option)}>
                        {option}s
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className='space-y-1'>
                <Label>{t('Aspect ratio')}</Label>
                <Select
                  onValueChange={(value) => setVideoRatio(String(value))}
                  value={ratio}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {runtime.video.ratios.map((option) => (
                      <SelectItem key={option} value={option}>
                        {option}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className='space-y-1'>
                <Label>{t('Quality')}</Label>
                <Select
                  onValueChange={(value) => setVideoQuality(String(value))}
                  value={videoQualityValue}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {runtime.video.qualities.map((option) => (
                      <SelectItem key={option} value={option}>
                        {option}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              {runtime.video.reference_image && (
                <div className='space-y-1'>
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
                    <div className='flex items-center gap-2'>
                      <img
                        alt={t('Reference image')}
                        className='size-14 rounded border object-cover'
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
              className='flex-1'
              disabled={!canGenerate}
              onClick={handleGenerate}
            >
              {isGenerating ? (
                <Loader2Icon className='size-4 animate-spin' />
              ) : (
                <SparklesIcon className='size-4' />
              )}
              {t('Generate')}
            </Button>
            {isGenerating && (
              <Button onClick={stop} variant='outline'>
                {t('Stop')}
              </Button>
            )}
          </div>

          {mode === 'image' && !runtime.image.enabled && (
            <p className='text-muted-foreground text-xs'>
              {t('Image generation is disabled by the administrator.')}
            </p>
          )}
          {mode === 'video' && !runtime.video.enabled && (
            <p className='text-muted-foreground text-xs'>
              {t('Video generation is disabled by the administrator.')}
            </p>
          )}
        </section>

        <section className='space-y-4'>
          {modeItems.length === 0 ? (
            <div className='text-muted-foreground flex flex-col items-center justify-center gap-2 rounded-xl border border-dashed p-12 text-center'>
              {mode === 'video' ? (
                <VideoIcon className='size-8' />
              ) : (
                <ImageIcon className='size-8' />
              )}
              <p className='text-sm'>
                {mode === 'video'
                  ? t('Your generated videos will appear here.')
                  : t('Your generated images will appear here.')}
              </p>
            </div>
          ) : (
            <div className='grid gap-4 sm:grid-cols-2 xl:grid-cols-3'>
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
  )
}
