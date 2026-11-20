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
import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  ArrowUpIcon,
  CheckIcon,
  ChevronDownIcon,
  DownloadIcon,
  EyeIcon,
  EyeOffIcon,
  HistoryIcon,
  ImageIcon,
  InfoIcon,
  Loader2Icon,
  PlusIcon,
  SparklesIcon,
  FileAudioIcon,
  Square as SquareIcon,
  Trash2Icon,
  TriangleAlertIcon,
  VideoIcon,
  XIcon,
  FolderIcon,
} from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { api } from '@/lib/api'

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
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
import type { StudioItem, StudioMode } from './types'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { getPlaygroundRuntimeConfig } from '@/features/playground/api'
import {
  EMPTY_PLAYGROUND_RUNTIME_CONFIG,
  type MediaModelRuntime,
} from '@/features/playground/types'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'
import { Switch } from '@/components/ui/switch'

import { useStudioGeneration } from './hooks/use-studio-generation'
import {
  resolveEffectiveModelOptions,
  pickStillValid,
  type EffectiveModelOptions,
} from './lib/model-constraints'
import { getStudioVideo } from './lib/studio-indexeddb'
import { StudioItemDetailsDialog } from './components/studio-item-details-dialog'
import { VideoJsPlayer } from './components/video-js-player'
import { STUDIO_ITEMS_PURGED_EVENT } from './lib/studio-sync'

/** Serves media through local cache proxy to avoid 12-hour upstream expiration. */
function getPlayableMediaUrl(url?: string): string {
  if (!url) return ''
  if (url.startsWith('http://') || url.startsWith('https://')) {
    return `/api/media/cache?url=${encodeURIComponent(url)}`
  }
  return url
}

/**
 * Resolves the playback URL for an item: IndexedDB-local when the server
 * storage is full, otherwise through the media proxy (or the raw same-origin
 * path). Callers must hydrate `localOnly` URLs separately with useLocalOnlyUrls.
 */
function getPlayableItemUrl(item: StudioItem, localUrls: Record<string, string>): string {
  if (item.localOnly) {
    return localUrls[item.id] ?? ''
  }
  return getPlayableMediaUrl(item.urls[0])
}

/** Hydrates object URLs for items whose bytes live in this browser's IndexedDB. */
function useLocalOnlyUrls(items: StudioItem[]): Record<string, string> {
  const [localUrls, setLocalUrls] = useState<Record<string, string>>({})

  useEffect(() => {
    let cancelled = false
    const pending = items.filter((item) => item.localOnly && !(item.urls?.length) && !localUrls[item.id])
    if (pending.length === 0) return
    void (async () => {
      for (const item of pending) {
        try {
          const blob = await getStudioVideo(item.id)
          if (!blob) continue
          const objectUrl = URL.createObjectURL(blob)
          if (cancelled) {
            URL.revokeObjectURL(objectUrl)
            continue
          }
          setLocalUrls((prev) => ({ ...prev, [item.id]: objectUrl }))
        } catch {
          // blob missing — item stays unplayable in this browser
        }
      }
    })()
    return () => {
      cancelled = true
    }
  }, [items, localUrls])

  // Drop and revoke URLs for items that disappeared from the history.
  useEffect(() => {
    setLocalUrls((prev) => {
      const ids = new Set(items.map((item) => item.id))
      const next = { ...prev }
      for (const id of Object.keys(next)) {
        if (!ids.has(id)) {
          URL.revokeObjectURL(next[id])
          delete next[id]
        }
      }
      return next
    })
  }, [items])

  // Revoke everything when the component unmounts.
  useEffect(
    () => () => {
      Object.values(localUrls).forEach((url) => URL.revokeObjectURL(url))
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    []
  )

  return localUrls
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

function progressPercentOf(raw: any): number | null {
  if (typeof raw === 'number') {
    return Math.min(100, Math.max(0, Math.round(raw)))
  }
  if (!raw) return null
  const str = String(raw)
  const match = str.match(/(\d+(?:[.,]\d+)?)/)
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

function formatBytes(value?: number): string {
  if (!Number.isFinite(value ?? NaN) || !value) return '0 MB'
  if (value >= 1024 * 1024 * 1024) return `${(value / (1024 * 1024 * 1024)).toFixed(2)} GB`
  return `${(value / (1024 * 1024)).toFixed(1)} MB`
}

interface VideoStorageQuotaProps {
  onConfirmOldest: () => void
  deleting: boolean
}

/** Shows the per-user video storage quota and offers to free space by deleting the oldest videos. Compact single-row design. */
function VideoStorageQuota({ onConfirmOldest, deleting }: VideoStorageQuotaProps) {
  const { t } = useTranslation()
  const { data } = useQuery({
    queryKey: ['media-storage'],
    queryFn: async () => {
      const res = await api.get('/api/media/storage', { skipErrorHandler: true } as Record<string, unknown>)
      return (res?.data?.data ?? null) as { used_bytes?: number; limit_bytes?: number; count?: number } | null
    },
    staleTime: 30_000,
  })
  const used = Number(data?.used_bytes ?? 0)
  const limit = Number(data?.limit_bytes ?? 1 << 30)
  const ratio = limit > 0 ? Math.min(1, used / limit) : 0
  const over = limit > 0 && used >= limit
  const percent = Math.round(ratio * 100)

  return (
    <div className='border-b border-border/60 bg-muted/30 px-3 sm:px-4 py-1.5 flex items-center gap-2'>
      <FolderIcon className='size-3 shrink-0 text-muted-foreground' />
      <div className='min-w-0 flex-1'>
        <div className='flex items-center justify-between gap-2 text-[10px] leading-tight'>
          <span className='text-muted-foreground truncate'>
            {t('Video storage')}{' '}
            <span className={over ? 'text-amber-600 dark:text-amber-400 font-medium' : 'text-foreground/80 font-medium'}>
              {formatBytes(used)} / {formatBytes(limit)}
            </span>
          </span>
          <span className='text-[9px] text-muted-foreground tabular-nums shrink-0'>{percent}%</span>
        </div>
        <div className='mt-0.5 h-1 w-full overflow-hidden rounded-full bg-muted'>
          <div
            className={cn('h-full rounded-full transition-all', over ? 'bg-amber-500' : 'bg-primary')}
            style={{ width: `${percent}%` }}
          />
        </div>
        {over && (
          <p className='mt-0.5 text-[9px] leading-tight text-amber-600 dark:text-amber-400'>
            {t('Server storage is full — new videos will be kept only in this browser (IndexedDB). Existing videos are not affected.')}
          </p>
        )}
      </div>
      <button
        type='button'
        className='shrink-0 inline-flex items-center gap-1 rounded-md px-1.5 py-1 text-[9px] font-medium text-muted-foreground hover:text-destructive hover:bg-destructive/10 transition-colors disabled:opacity-40 disabled:pointer-events-none'
        disabled={deleting || used <= 0}
        onClick={onConfirmOldest}
        title={t('Delete the oldest videos to free up space')}
      >
        {deleting ? <Loader2Icon className='size-3 animate-spin' /> : <Trash2Icon className='size-3' />}
        <span className='hidden xs:inline sm:inline'>{t('Delete oldest')}</span>
      </button>
    </div>
  )
}

/** Reference-image chip strip: cap + source from the model's option rules. */
// Deterministic 48-bit-ish hash of a URL/data-URL string — used for React
// keys on unique reference entries.
function referenceKey(url: string): string {
  let hash = 0
  for (let i = 0; i < url.length; i += 113) {
    hash = (hash * 31 + url.charCodeAt(i)) >>> 0
  }
  return hash.toString(36) + ':' + url.length.toString(36)
}

export function Studio() {
  const { t } = useTranslation()
  const user = useAuthStore((state) => state.auth.user)
  const [mode, setMode] = useState<StudioMode>('video')
  const [prompt, setPrompt] = useState('')
  const [isDragging, setIsDragging] = useState(false)
  const [attachedMedia, setAttachedMedia] = useState<AttachedMedia | null>(null)
  const [popoverOpen, setPopoverOpen] = useState(false)
  const [deleteConfirmItemId, setDeleteConfirmItemId] = useState<string | null>(null)
  const [detailsItemId, setDetailsItemId] = useState<string | null>(null)
  const queryClient = useQueryClient()
  const [confirmOldestOpen, setConfirmOldestOpen] = useState(false)
  const [confirmAllOpen, setConfirmAllOpen] = useState(false)
  const [totalToWipe, setTotalToWipe] = useState({ bytes: 0, count: 0 })
  const [deletingStorage, setDeletingStorage] = useState(false)

  const runDeleteStorage = async (all: boolean) => {
    setDeletingStorage(true)
    try {
      const res = await api.delete(`/api/media/storage${all ? '?all=1' : ''}`, {
        skipErrorHandler: true,
      } as Record<string, unknown>)
      const freed = Number(res?.data?.data?.freed_bytes ?? 0)
      const deletedUrls = (res?.data?.data?.deleted_urls ?? []) as string[]
      await queryClient.invalidateQueries({ queryKey: ['media-storage'] })
      if (deletedUrls.length > 0) {
        window.dispatchEvent(
          new CustomEvent(STUDIO_ITEMS_PURGED_EVENT, { detail: { urls: deletedUrls } })
        )
      }
      toast.success(t('Freed {{freed}} of storage by deleting oldest videos.', { freed: formatBytes(freed) }))
    } catch (err) {
      const response = (err as { response?: { status?: number; data?: Record<string, unknown> } })?.response
      if (
        response?.status === 409 &&
        response.data?.code === 'would_delete_all_stored'
      ) {
        setTotalToWipe({
          bytes: Number(response.data.total_bytes ?? 0),
          count: Number(response.data.total_count ?? 0),
        })
        setConfirmAllOpen(true)
      } else {
        toast.error(t('Failed to delete old videos.'))
      }
    } finally {
      setDeletingStorage(false)
    }
  }

  // Track active stage item during this session (stored in localStorage so returning to tab keeps finished item on stage)
  const [activeSessionItemId, setActiveSessionItemIdState] = useState<string | null>(() => {
    try {
      return localStorage.getItem('studio_active_item_id') || null
    } catch {
      return null
    }
  })

  const setActiveSessionItemId = useCallback((id: string | null) => {
    setActiveSessionItemIdState(id)
    try {
      if (id) {
        localStorage.setItem('studio_active_item_id', id)
      } else {
        localStorage.removeItem('studio_active_item_id')
      }
    } catch {}
  }, [])
  const [historyOpen, setHistoryOpen] = useState(false)
  const [historyTab, setHistoryTab] = useState<'all' | StudioMode>('all')
  // Items whose media URL failed to load (e.g. an expired upstream link).
  // Broken videos render an explicit state instead of an endless 0:00 player.
  const [brokenMediaIds, setBrokenMediaIds] = useState<ReadonlySet<string>>(new Set())
  const markMediaBroken = (id: string) =>
    setBrokenMediaIds((prev) => {
      if (prev.has(id)) return prev
      const next = new Set(prev)
      next.add(id)
      return next
    })

  const fileInputRef = useRef<HTMLInputElement>(null)

  const [imageModel, setImageModel] = useState('')
  const [imageSize, setImageSize] = useState('')
  const [imageQuality, setImageQuality] = useState('')
  const [imageCount] = useState(1)

  const [videoModel, setVideoModel] = useState('')
  const [videoDuration, setVideoDuration] = useState('')
  const [videoRatio, setVideoRatio] = useState('')
  const [videoQuality, setVideoQuality] = useState('')
  const [audioEnabled, setAudioEnabled] = useState(true)
  const [referenceImages, setReferenceImages] = useState<string[]>([])
  const [audioReferences, setAudioReferences] = useState<string[]>([])
  const [videoReferences, setVideoReferences] = useState<string[]>([])
  const [negativePrompt, setNegativePrompt] = useState('')
  const [startFrame, setStartFrame] = useState('')
  const [endFrame, setEndFrame] = useState('')
  const [showAdvancedOptions, setShowAdvancedOptions] = useState(false)
  const [optimizerPickerOpen, setOptimizerPickerOpen] = useState(false)
  const [optimizerModel, setOptimizerModel] = useState<string>(() => {
    try {
      return localStorage.getItem('studio_optimizer_model') ?? ''
    } catch {
      return ''
    }
  })

  const { data: runtimeData } = useQuery({
    queryKey: ['playground-runtime-config'],
    queryFn: ({ signal }) => getPlaygroundRuntimeConfig(signal),
    staleTime: 60_000,
  })
  const runtime = runtimeData ?? EMPTY_PLAYGROUND_RUNTIME_CONFIG

  // The administrator may configure several optimizer models; when more than
  // one is set the composer shows a picker (first is the default).
  const optimizerModels = useMemo<string[]>(() => {
    const fromList = (runtime.studio?.prompt_optimizer_models ?? [])
      .map((value) => value.trim())
      .filter(Boolean)
    if (fromList.length > 0) return fromList
    const single = runtime.studio?.prompt_optimizer_model?.trim()
    return single ? single.split(',').map((value) => value.trim()).filter(Boolean) : []
  }, [runtime.studio])

  // Keep the stored selection valid against the configured list.
  useEffect(() => {
    if (optimizerModels.length === 0) {
      if (optimizerModel) setOptimizerModel('')
      return
    }
    if (!optimizerModel || !optimizerModels.includes(optimizerModel)) {
      setOptimizerModel(optimizerModels[0])
    }
  }, [optimizerModels, optimizerModel])

  useEffect(() => {
    try {
      localStorage.setItem('studio_optimizer_model', optimizerModel)
    } catch {
      // quota — best effort
    }
  }, [optimizerModel])

  // Automatically fall back to video if selected mode is disabled
  useEffect(() => {
    if (!runtime.image.enabled && mode === 'image') {
      setMode('video')
    }
  }, [runtime.image.enabled, mode])

  const imageModels = useMemo<MediaModelRuntime[]>(() => {
    if (runtime.image.models?.length) return runtime.image.models
    if (runtime.image.model) return [{ model: runtime.image.model }]
    return []
  }, [runtime.image])
  const videoModels = useMemo<MediaModelRuntime[]>(() => {
    if (runtime.video.models?.length) return runtime.video.models
    if (runtime.video.model) return [{ model: runtime.video.model }]
    return []
  }, [runtime.video])

  const selectedImageModel = useMemo(() => {
    const match = imageModels.find((entry) => entry.model === imageModel)
    return match ?? imageModels[0]
  }, [imageModels, imageModel])

  const selectedVideoModel = useMemo(() => {
    const match = videoModels.find((entry) => entry.model === videoModel)
    return match ?? videoModels[0]
  }, [videoModels, videoModel])

  const currentModelRuntime = useMemo(() => {
    switch (mode) {
      case 'video':
        return selectedVideoModel
      case 'image':
        return selectedImageModel
      default:
        return undefined
    }
  }, [mode, selectedVideoModel, selectedImageModel])

  const currentModelName = currentModelRuntime?.model || ''
  const currentModelDisplayName = currentModelRuntime?.display_name || currentModelName || t('Select Model')

  useEffect(() => {
    if (selectedImageModel && selectedImageModel.model !== imageModel) {
      setImageModel(selectedImageModel.model)
      setImageSize('')
      setImageQuality('')
    }
  }, [selectedImageModel, imageModel])

  const appliedVideoModelRef = useRef('')
  useEffect(() => {
    if (!selectedVideoModel) return
    const model = selectedVideoModel.model
    if (appliedVideoModelRef.current === model) {
      if (!videoDuration && selectedVideoModel.default_duration) {
        setVideoDuration(String(selectedVideoModel.default_duration))
      }
      return
    }
    appliedVideoModelRef.current = model
    setVideoModel(model)
    setVideoDuration(
      selectedVideoModel.default_duration ? String(selectedVideoModel.default_duration) : ''
    )
    setVideoRatio('')
    setVideoQuality('')
  }, [selectedVideoModel, videoDuration])

  // Apply default model from runtime config on initial load
  const defaultModelAppliedRef = useRef(false)
  useEffect(() => {
    if (defaultModelAppliedRef.current) return
    const def = runtime.studio?.default_model?.trim()
    if (!def) return

    if (videoModels.some((m) => m.model === def)) {
      setMode('video')
      setVideoModel(def)
      defaultModelAppliedRef.current = true
    } else if (imageModels.some((m) => m.model === def)) {
      setMode('image')
      setImageModel(def)
      defaultModelAppliedRef.current = true
    }
  }, [runtime.studio?.default_model, videoModels, imageModels])

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

  // ── Effective options after the model's conditional rules ────────────────
  // Rules narrow durations/qualities/ratios and the reference cap per
  // selection ("15s: no 1080p", "at 1080p: max 2 references", "at 10s: no
  // references"). Everything downstream renders and submits these.
  const effectiveVideoOptions = useMemo<EffectiveModelOptions>(
    () =>
      resolveEffectiveModelOptions(
        selectedVideoModel ?? {},
        duration > 0 ? duration : undefined,
        videoQualityValue || undefined,
        ratio || undefined
      ),
    [selectedVideoModel, duration, videoQualityValue, ratio]
  )
  const effectiveImageOptions = useMemo<EffectiveModelOptions>(
    () =>
      resolveEffectiveModelOptions(
        selectedImageModel ?? {},
        undefined,
        imageQuality || selectedImageModel?.default_quality || runtime.image.default_quality || undefined,
        undefined
      ),
    [selectedImageModel, imageQuality, runtime.image.default_quality]
  )
  const activeEffective = mode === 'video' ? effectiveVideoOptions : effectiveImageOptions
  const offeredDurations = effectiveVideoOptions.durations.length
    ? effectiveVideoOptions.durations
    : videoDurations
  const offeredRatios = effectiveVideoOptions.ratios.length
    ? effectiveVideoOptions.ratios
    : videoRatios
  const offeredVideoQualities = effectiveVideoOptions.qualities.length
    ? effectiveVideoOptions.qualities
    : videoQualities
  const offeredImageQualities = effectiveImageOptions.qualities.length
    ? effectiveImageOptions.qualities
    : imageQualities
  const referenceCap = activeEffective.maxReferenceImages
  const referencesAllowed = activeEffective.referencesAllowed
  const audioReferenceCap = selectedVideoModel?.audio_reference
    ? Math.max(1, selectedVideoModel.max_audio_references ?? 3)
    : 0
  const videoReferenceCap = selectedVideoModel?.video_reference
    ? Math.max(1, selectedVideoModel.max_video_references ?? 2)
    : 0

  // Keep the selected values inside the rule-narrowed lists; if the rules
  // disallow the current selection, fall back to the first offered option.
  useEffect(() => {
    const picked = pickStillValid(
      videoDuration ? Number(videoDuration) : undefined,
      effectiveVideoOptions.durations
    )
    if (picked !== undefined && String(picked) !== videoDuration) {
      setVideoDuration(String(picked))
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [effectiveVideoOptions.durations])
  useEffect(() => {
    if (videoQualityValue && effectiveVideoOptions.qualities.length > 0 &&
        !effectiveVideoOptions.qualities.includes(videoQualityValue)) {
      setVideoQuality(effectiveVideoOptions.qualities[0])
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [effectiveVideoOptions.qualities])
  useEffect(() => {
    if (ratio && effectiveVideoOptions.ratios.length > 0 &&
        !effectiveVideoOptions.ratios.includes(ratio)) {
      setVideoRatio(effectiveVideoOptions.ratios[0])
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [effectiveVideoOptions.ratios])

  // Trim reference images when the rules shrink the cap (e.g. switching to a
  // duration at which references are disabled).
  useEffect(() => {
    if (referenceImages.length > referenceCap) {
      setReferenceImages((prev) => prev.slice(0, referenceCap))
      toast.info(
        t('Reference images are not available at these settings for this model.')
      )
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [referenceCap])
  useEffect(() => {
    if (audioReferenceCap > 0) {
      setAudioReferences((prev) => prev.slice(0, audioReferenceCap))
    } else if (audioReferences.length > 0) {
      setAudioReferences([])
      toast.info(t('This model does not accept audio references.'))
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [audioReferenceCap])
  useEffect(() => {
    if (videoReferenceCap > 0) {
      setVideoReferences((prev) => prev.slice(0, videoReferenceCap))
    } else if (videoReferences.length > 0) {
      setVideoReferences([])
      toast.info(t('This model does not accept video references.'))
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [videoReferenceCap])

  const {
    items,
    isGenerating,
    generateImage,
    generateVideo,
    stop,
    removeItem,
  } = useStudioGeneration()

  // IndexedDB-backed object URLs for items kept in this browser (over-quota storage).
  const localOnlyUrls = useLocalOnlyUrls(items)

  // Lifecycle for page reloads: if an active item has already finished and been viewed, archive it to history popup on reload
  const reloadCheckedRef = useRef(false)
  useEffect(() => {
    if (reloadCheckedRef.current || items.length === 0) return
    reloadCheckedRef.current = true

    const storedId = (() => {
      try {
        return localStorage.getItem('studio_active_item_id')
      } catch {
        return null
      }
    })()
    if (!storedId) return

    const item = items.find((i) => i.id === storedId)
    if (!item) return

    // If item is still pending, keep it visible!
    if (item.status === 'pending') return

    // If item is completed, check if it was seen before this reload
    try {
      const seen = sessionStorage.getItem('studio_seen_completed_' + item.id) === '1'
      if (seen) {
        // Page was reloaded after item was already viewed: archive to popup!
        setActiveSessionItemId(null)
      } else {
        // First view after completion (e.g. user started generation, closed tab, returned)
        sessionStorage.setItem('studio_seen_completed_' + item.id, '1')
      }
    } catch {}
  }, [items, setActiveSessionItemId])

  const activeItem = useMemo(() => {
    // If an item is active/pending, always display it on the stage even after page refresh
    const pendingItem = items.find((item) => item.status === 'pending')
    if (pendingItem) return pendingItem

    if (activeSessionItemId) {
      const match = items.find((item) => item.id === activeSessionItemId)
      if (match) return match
    }

    return null
  }, [items, activeSessionItemId])

  // Track viewing completed item so subsequent reloads archive it
  useEffect(() => {
    if (activeItem && (activeItem.status === 'ready' || activeItem.status === 'error')) {
      try {
        sessionStorage.setItem('studio_seen_completed_' + activeItem.id, '1')
      } catch {}
    }
  }, [activeItem])

  // Only the active stage item being pending or an active generation blocks the send button
  const hasActivePending = isGenerating || activeItem?.status === 'pending'

  const singleVideoUrl = (() => {
    if (activeItem?.localOnly) {
      return localOnlyUrls[activeItem.id] || undefined
    }
    const urls = activeItem?.urls ?? []
    return urls.length > 0 ? urls[0] : undefined
  })()

  const editModeOnly = mode === 'video' && selectedVideoModel?.edit_mode === 'only'
  const editReady = !editModeOnly || attachedMedia?.type === 'video'
  const audioRenderAvailable = Boolean(selectedVideoModel?.audio)
  const canGenerate =
    prompt.trim().length > 0 && !hasActivePending && Boolean(currentModelName) && editReady

  const handleGenerate = async () => {
    if (!canGenerate || hasActivePending) return
    const text = prompt.trim()
    // The rules may have switched a modality off after the files were picked —
    // gate each list again at submission time.
    const imageRefs = referencesAllowed ? referenceImages : []
    const audioRefs = audioReferenceCap > 0 ? audioReferences : []
    const videoRefs = videoReferenceCap > 0 ? videoReferences : []
    const editMode = selectedVideoModel?.edit_mode ?? 'none'
    const audioOfModel = Boolean(selectedVideoModel?.audio)

    if (mode === 'image') {
      await generateImage(
        {
          prompt: text,
          model: currentModelName,
          mode,
          group: user?.group,
          size,
          quality,
          count: imageCount,
          referenceImages: imageRefs,
          referenceCap,
          metadata: optimizerModel ? { optimizer_model: optimizerModel } : undefined,
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
        duration: duration || undefined,
        ratio: ratio || undefined,
        quality: videoQualityValue || undefined,
        imageUrl: attachedMedia?.url || undefined,
        startFrame: startFrame.trim() || undefined,
        endFrame: endFrame.trim() || undefined,
        negativePrompt: negativePrompt.trim() || undefined,
        referenceImages: imageRefs,
        audioReferences: audioRefs,
        videoReferences: videoRefs,
        referenceCap,
        metadata: {
          ...(optimizerModel ? { optimizer_model: optimizerModel } : {}),
          ...(audioOfModel ? { audio: audioEnabled } : {}),
          ...(editMode && editMode !== 'none' && attachedMedia?.type === 'video'
            ? { operation: 'edit' }
            : {}),
        },
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

  const readFileAsDataURL = (file: File) =>
    new Promise<string>((resolve, reject) => {
      const reader = new FileReader()
      reader.onload = () =>
        typeof reader.result === 'string' ? resolve(reader.result) : reject(new Error('read failed'))
      reader.onerror = () => reject(reader.error ?? new Error('read failed'))
      reader.readAsDataURL(file)
    })

  // Adds reference files (image/audio/video) to the matching chip list,
  // honoring the model's per-modality cap and the current option rules.
  const handleReferenceFiles = async (
    files: FileList | File[] | null,
    kind: 'image' | 'audio' | 'video'
  ) => {
    if (!files || files.length === 0) return
    const capByKind = {
      image: referenceCap,
      audio: audioReferenceCap,
      video: videoReferenceCap,
    } as const
    const currentByKind = {
      image: referenceImages,
      audio: audioReferences,
      video: videoReferences,
    } as const
    const setterByKind: Record<'image' | 'audio' | 'video', React.Dispatch<React.SetStateAction<string[]>>> = {
      image: setReferenceImages,
      audio: setAudioReferences,
      video: setVideoReferences,
    }
    const cap = capByKind[kind]
    const current = currentByKind[kind]
    const setter = setterByKind[kind]
    const MAX_REFERENCE_FILE_BYTES = 8 << 20
    const rejectedForSize = [...files].filter((file) => file.size > MAX_REFERENCE_FILE_BYTES).length
    if (rejectedForSize > 0) {
      toast.error(
        t('Reference files must be under 8 MB each ({{count}} skipped).', {
          count: rejectedForSize,
        })
      )
    }
    const filesWithinLimit = [...files].filter(
      (file) => file.size <= MAX_REFERENCE_FILE_BYTES
    )
    if (cap <= 0) {
      const message = {
        image: t('This model does not accept reference images at these settings.'),
        audio: t('This model does not accept audio references.'),
        video: t('This model does not accept video references.'),
      }[kind]
      toast.error(message)
      return
    }
    const room = Math.max(0, cap - current.length)
    const accepted = filesWithinLimit.slice(0, room)
    if (accepted.length === 0) {
      toast.info(
        t('The model accepts at most {{max}} reference file(s) at these settings.', { max: cap })
      )
      return
    }
    try {
      const urls = await Promise.all(accepted.map(readFileAsDataURL))
      setter((prev) => [...new Set([...prev, ...urls])].slice(0, cap))
    } catch {
      toast.error(t('Failed to read one of the files.'))
      return
    }
    if (filesWithinLimit.length > accepted.length) {
      toast.info(
        t('The model accepts at most {{max}} reference file(s) at these settings.', { max: cap })
      )
    }
  }

  // The '+' button of the composer covers every reference role:
  //  - exactly one image/video picked -> the single attachment (frame source / edit source)
  //  - everything else (audio or multiple files) -> the modality reference lists
  //    attached to the generation as reference_images / reference_audio_count / reference_video_count
  const handleMediaPick = (files: FileList | null) => {
    if (!files || files.length === 0) return
    const list = [...files]
    const images = list.filter((f) => f.type.startsWith('image/'))
    const videos = list.filter((f) => f.type.startsWith('video/'))
    const audios = list.filter((f) => f.type.startsWith('audio/'))
    if (list.length === 1 && (images.length === 1 || videos.length === 1)) {
      // Single attachment keeps the legacy frame/edit-source behavior.
      const file = list[0]
      const reader = new FileReader()
      reader.onload = () => {
        if (typeof reader.result === 'string') {
          setAttachedMedia({
            url: reader.result,
            name: file.name,
            type: images.length === 1 ? 'image' : 'video',
          })
        }
      }
      reader.readAsDataURL(file)
      return
    }
    if (images.length > 0) void handleReferenceFiles(images, 'image')
    if (audios.length > 0) void handleReferenceFiles(audios, 'audio')
    if (videos.length > 0) void handleReferenceFiles(videos, 'video')
  }

  // Paste files (Ctrl+V / Cmd+V) straight into the composer textarea —
  // images become image references, audios/videos land in their own lists.
  const handlePromptPaste = (e: React.ClipboardEvent<HTMLTextAreaElement>) => {
    const files = [...e.clipboardData.items]
      .filter((item) => item.kind === 'file')
      .map((item) => item.getAsFile())
      .filter((file): file is File => Boolean(file))
    if (files.length === 0) return
    e.preventDefault()
    const images = files.filter((f) => f.type.startsWith('image/'))
    const audios = files.filter((f) => f.type.startsWith('audio/'))
    const videos = files.filter((f) => f.type.startsWith('video/'))
    if (images.length > 0) void handleReferenceFiles(images, 'image')
    if (audios.length > 0) void handleReferenceFiles(audios, 'audio')
    if (videos.length > 0) void handleReferenceFiles(videos, 'video')
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
    const files = e.dataTransfer.files
    if (files?.length) {
      handleMediaPick(files)
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
        <div className='flex flex-1 flex-col justify-center py-6 pb-20'>
          {activeItem ? (
            <div className='space-y-6'>
              {/* Active / Latest Generation Display Card */}
              <div className='relative isolate z-0 mx-auto w-full max-w-2xl overflow-hidden rounded-2xl border border-border/80 bg-card shadow-sm'>
                {/* Top action buttons: Previous Generations & Dismiss/Clear */}
                <div className='absolute end-3 top-3 z-10 flex items-center gap-1.5'>
                  <Button
                    size='sm'
                    variant='ghost'
                    onClick={() => setHistoryOpen(true)}
                    className='h-7 px-2.5 text-xs text-muted-foreground hover:text-foreground bg-background/80 backdrop-blur-sm border border-border/50 hover:bg-background rounded-lg gap-1.5 shadow-xs'
                    title={t('History')}
                  >
                    <HistoryIcon className='size-3.5' />
                    <span>{t('History')}</span>
                  </Button>
                  <Button
                    size='sm'
                    variant='ghost'
                    onClick={() => {
                      if (activeItem && activeItem.status === 'pending') {
                        stop(activeItem.id)
                      }
                      setActiveSessionItemId(null)
                    }}
                    className={cn(
                      'h-7 px-2.5 text-xs backdrop-blur-sm border border-border/50 rounded-lg gap-1.5 shadow-xs transition',
                      activeItem.status === 'pending'
                        ? 'text-destructive hover:text-destructive bg-destructive/10 hover:bg-destructive/20 border-destructive/30 font-medium'
                        : 'text-muted-foreground hover:text-foreground bg-background/80 hover:bg-background'
                    )}
                    title={activeItem.status === 'pending' ? t('Stop') : t('Hide')}
                  >
                    {activeItem.status === 'pending' ? (
                      <>
                        <SquareIcon className='size-3 fill-current' />
                        <span>{t('Stop')}</span>
                      </>
                    ) : (
                      <>
                        <EyeOffIcon className='size-3.5' />
                        <span>{t('Hide')}</span>
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
                            : t('Generating image...')}
                        </p>
                        <p className='line-clamp-2 max-w-sm text-xs text-muted-foreground'>
                          &ldquo;{activeItem.prompt}&rdquo;
                        </p>
                      </div>
                      {activeItem.mode === 'video' && activeItem.queuePosition !== undefined && (
                        <div className='inline-flex flex-col items-center gap-1 rounded-full border border-border/60 bg-background/85 px-4 py-1.5 text-[11px] font-medium text-muted-foreground shadow-sm backdrop-blur'>
                          <span className='inline-flex items-center gap-2'>
                            <Loader2Icon className='size-3 animate-spin' />
                            {t('In queue — you are in line')}
                          </span>
                          <span className='text-[10px] text-muted-foreground/80'>
                            {activeItem.queuePosition <= 1
                              ? t('In queue — you are next')
                              : t('In queue — you are #{{position}} in line', {
                                  position: activeItem.queuePosition,
                                })}
                          </span>
                        </div>
                      )}
                      {activeItem.mode === 'video' && (
                        <div className='w-full max-w-xs flex items-center gap-3'>
                          <Progress
                            value={progressPercentOf(activeItem.progress ?? '') ?? 0}
                            className='h-1.5 flex-1'
                          />
                          <span className='font-mono text-xs text-muted-foreground shrink-0 min-w-[36px] text-right'>
                            {activeItem.progress || '0%'}
                          </span>
                        </div>
                      )}
                    </div>
                  )}

                  {activeItem.status === 'error' && (
                    <div className='flex w-full flex-col items-center justify-center gap-3 p-6'>
                      <span className='flex size-8 items-center justify-center rounded-full bg-destructive/10 text-xs font-bold text-destructive'>
                        !
                      </span>
                      <p className='text-xs font-medium text-destructive max-w-md text-center'>
                        {activeItem.error || t('Generation failed')}
                      </p>
                    </div>
                  )}

                  {activeItem.status === 'ready' && (
                    <div className='relative w-full p-2 flex flex-col items-center justify-center gap-2'>
                      {activeItem.mode === 'video' ? (
                        (() => {
                          if (activeItem.localOnly && !singleVideoUrl) {
                            // The video is opening from this browser's IndexedDB — a
                            // brief local read.
                            return (
                              <div className='flex flex-col items-center justify-center gap-3 p-10 text-center'>
                                <Loader2Icon className='size-9 animate-spin text-muted-foreground' />
                                <p className='text-sm font-medium'>{t('Loading local video…')}</p>
                              </div>
                            )
                          }
                          if (brokenMediaIds.has(activeItem.id)) {
                            return (
                              <div className='flex flex-col items-center justify-center gap-3 p-10 text-center'>
                                <VideoIcon className='size-10 text-muted-foreground/40' />
                                <p className='text-sm font-medium text-muted-foreground'>
                                  {t('The video could not be loaded.')}
                                </p>
                                <p className='max-w-xs text-[11px] text-muted-foreground/70'>
                                  {t('The upstream link may have expired, or the file could not be stored on the server. Regenerate to create a fresh copy.')}
                                </p>
                                <button
                                  type='button'
                                  onClick={() =>
                                    setBrokenMediaIds((prev) => {
                                      const next = new Set(prev)
                                      next.delete(activeItem.id)
                                      return next
                                    })
                                  }
                                  className='h-8 rounded-lg border bg-background px-4 text-xs font-medium text-foreground transition hover:bg-muted'
                                >
                                  {t('Retry')}
                                </button>
                              </div>
                            )
                          }
                          return singleVideoUrl ? (
                            <VideoJsPlayer
                              key={singleVideoUrl}
                              autoPlay
                              loop={false}
                              className='max-h-[380px] w-full object-contain bg-black rounded-lg'
                              src={getPlayableItemUrl(activeItem, localOnlyUrls) || singleVideoUrl}
                              qualityLabel={activeItem.settings?.quality != null ? String(activeItem.settings.quality) : undefined}
                              onError={() => markMediaBroken(activeItem.id)}
                            />
                          ) : (
                            <p className='max-w-sm py-8 text-center text-sm font-medium text-muted-foreground'>
                              {t('The video could not be loaded.')}
                            </p>
                          )
                        })()
                      ) : (
                        <img
                          alt={activeItem.prompt}
                          className='max-h-[380px] w-full object-contain rounded-lg'
                          src={getPlayableItemUrl(activeItem, localOnlyUrls)}
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
                  {(activeItem.status === 'ready' || (activeItem.status === 'error' && activeItem.urls.length > 0)) && (
                    <div className='flex items-center gap-1.5'>
                      {(() => {
                        const stagePlayable = getPlayableItemUrl(activeItem, localOnlyUrls)
                        return (
                          <a
                            href={stagePlayable || undefined}
                            download
                            rel='noreferrer'
                            target='_blank'
                            className='inline-flex items-center gap-1.5 h-8 px-3 rounded-lg border bg-background hover:bg-muted text-xs font-medium text-foreground transition shadow-xs aria-disabled:pointer-events-none aria-disabled:opacity-50'
                            aria-disabled={!stagePlayable}
                          >
                            <DownloadIcon className='size-3.5' />
                            {t('Download')}
                          </a>
                        )
                      })()}
                    </div>
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
        <div className='sticky bottom-6 z-30 w-full pt-4'>
          {/* Attached media & references strip, above the capsule */}
          {(referenceImages.length > 0 ||
            audioReferences.length > 0 ||
            videoReferences.length > 0 ||
            attachedMedia) && (
            <div className='mb-2 flex max-w-full items-center gap-2 overflow-x-auto rounded-xl border border-border/60 bg-card/90 p-1.5 backdrop-blur-md w-fit'>
              {attachedMedia && (
                <div className='flex shrink-0 items-center gap-2 rounded-lg border border-border/60 bg-background/60 py-1 pe-2 ps-1.5'>
                  {(() => {
                    if (attachedMedia.type === 'image') {
                      return (
                        <img
                          src={attachedMedia.url}
                          alt={t('Reference Image')}
                          className='size-7 rounded-md object-cover'
                        />
                      )
                    }
                    if (attachedMedia.type === 'audio') {
                      return (
                        <div className='flex size-7 items-center justify-center rounded-md bg-muted'>
                          <FileAudioIcon className='size-4 text-foreground' />
                        </div>
                      )
                    }
                    return (
                      <div className='flex size-7 items-center justify-center rounded-md bg-muted'>
                        <VideoIcon className='size-4 text-foreground' />
                      </div>
                    )
                  })()}
                  <span className='max-w-[140px] truncate text-xs font-medium text-foreground'>
                    {attachedMedia.name
                      ? attachedMedia.name
                      : (() => {
                          switch (attachedMedia.type) {
                            case 'audio':
                              return t('Reference Audio')
                            case 'video':
                              return t('Reference Video')
                            default:
                              return t('Reference Image')
                          }
                        })()}
                  </span>
                  <button
                    type='button'
                    aria-label={t('Remove reference')}
                    onClick={() => setAttachedMedia(null)}
                    className='text-muted-foreground transition hover:text-foreground'
                  >
                    <XIcon className='size-3.5' />
                  </button>
                </div>
              )}
              {referenceImages.map((url, index) => (
                <div
                  key={`img-${referenceKey(url)}`}
                  className='relative flex size-14 shrink-0 items-end overflow-hidden rounded-lg border border-border/60'
                >
                  <img
                    src={url}
                    alt={t('Reference image {{index}}', { index: index + 1 })}
                    className='absolute inset-0 size-full object-cover'
                  />
                  <span className='absolute start-1 top-1 rounded bg-black/60 px-1 text-[9px] font-semibold text-white'>
                    {index + 1}
                  </span>
                  <button
                    type='button'
                    aria-label={t('Remove reference image {{index}}', { index: index + 1 })}
                    onClick={() =>
                      setReferenceImages((prev) => prev.filter((_, i) => i !== index))
                    }
                    className='absolute end-0.5 top-0.5 rounded-md bg-black/60 p-0.5 text-white transition hover:bg-black/80'
                  >
                    <XIcon className='size-3' />
                  </button>
                </div>
              ))}
              {audioReferences.map((_url, index) => (
                <div
                  key={`aud-${referenceKey(_url)}`}
                  className='relative flex size-14 shrink-0 flex-col items-center justify-center gap-0.5 overflow-hidden rounded-lg border border-border/60 bg-muted'
                >
                  <FileAudioIcon className='size-5 text-foreground/80' />
                  <span className='text-[9px] font-semibold text-muted-foreground'>
                    {t('Audio {{index}}', { index: index + 1 })}
                  </span>
                  <button
                    type='button'
                    aria-label={t('Remove audio reference {{index}}', { index: index + 1 })}
                    onClick={() =>
                      setAudioReferences((prev) => prev.filter((_, i) => i !== index))
                    }
                    className='absolute end-0.5 top-0.5 rounded-md bg-black/60 p-0.5 text-white transition hover:bg-black/80'
                  >
                    <XIcon className='size-3' />
                  </button>
                </div>
              ))}
              {videoReferences.map((_url, index) => (
                <div
                  key={`vid-${referenceKey(_url)}`}
                  className='relative flex size-14 shrink-0 flex-col items-center justify-center gap-0.5 overflow-hidden rounded-lg border border-border/60 bg-muted'
                >
                  <VideoIcon className='size-5 text-foreground/80' />
                  <span className='text-[9px] font-semibold text-muted-foreground'>
                    {t('Video {{index}}', { index: index + 1 })}
                  </span>
                  <button
                    type='button'
                    aria-label={t('Remove video reference {{index}}', { index: index + 1 })}
                    onClick={() =>
                      setVideoReferences((prev) => prev.filter((_, i) => i !== index))
                    }
                    className='absolute end-0.5 top-0.5 rounded-md bg-black/60 p-0.5 text-white transition hover:bg-black/80'
                  >
                    <XIcon className='size-3' />
                  </button>
                </div>
              ))}
              <span className='shrink-0 ps-1 text-[10px] font-medium tabular-nums text-muted-foreground'>
                {referenceImages.length + audioReferences.length + videoReferences.length} /{' '}
                {referenceCap + audioReferenceCap + videoReferenceCap}
              </span>
            </div>
          )}

          <div className='w-full rounded-2xl border border-border/80 bg-card/95 shadow-2xl backdrop-blur-xl transition focus-within:border-border'>
            {/* Tabs: Video | Image */}
            <div className='relative flex items-center gap-1 border-b border-border/50 px-3 pt-2'>
              {runtime.video.enabled && (
                <button
                  type='button'
                  className={cn(
                    'relative flex items-center gap-1.5 px-3 pb-2.5 pt-1.5 text-[13px] font-medium transition-colors',
                    mode === 'video' ? 'text-foreground' : 'text-muted-foreground hover:text-foreground'
                  )}
                  onClick={() => setMode('video')}
                >
                  <VideoIcon className='size-3.5' />
                  {t('Video')}
                  {mode === 'video' && (
                    <span className='absolute bottom-[-1px] left-2 right-2 h-0.5 rounded-full bg-foreground' />
                  )}
                </button>
              )}
              {runtime.image.enabled && (
                <button
                  type='button'
                  className={cn(
                    'relative flex items-center gap-1.5 px-3 pb-2.5 pt-1.5 text-[13px] font-medium transition-colors',
                    mode === 'image' ? 'text-foreground' : 'text-muted-foreground hover:text-foreground'
                  )}
                  onClick={() => setMode('image')}
                >
                  <ImageIcon className='size-3.5' />
                  {t('Image')}
                  {mode === 'image' && (
                    <span className='absolute bottom-[-1px] left-2 right-2 h-0.5 rounded-full bg-foreground' />
                  )}
                </button>
              )}
              <div className='ml-auto pe-1 pb-1.5 text-[10px] tabular-nums text-muted-foreground/70'>
                {prompt.length > 0 ? t('{{count}} / 15000 characters', { count: prompt.length }) : ''}
              </div>
            </div>

            {/* Prompt input */}
            <div className='px-4 pb-1 pt-3'>
              <textarea
                value={prompt}
                onChange={(e) => setPrompt(e.target.value.slice(0, 15000))}
                onKeyDown={handleKeyDown}
                onPaste={handlePromptPaste}
                placeholder={t('What do you want to create?')}
                className='h-12 sm:h-14 w-full resize-none border-0 bg-transparent p-0 text-sm leading-6 text-foreground placeholder:text-muted-foreground/60 focus:outline-none focus:ring-0'
                rows={2}
                maxLength={15000}
              />
            </div>

            {/* Bottom toolbar */}
            <div className='flex min-w-0 items-center gap-1.5 overflow-hidden px-2.5 pb-2.5 pt-1'>
              {/* Left Action: Plus file attach button */}
              <label
                htmlFor='studio-file-upload'
                className='flex size-8 shrink-0 cursor-pointer items-center justify-center rounded-lg border border-border/70 bg-muted/40 text-muted-foreground transition hover:border-border hover:bg-muted hover:text-foreground'
                title={t('Add media file')}
              >
                <PlusIcon className='size-4' />
                <input
                  id='studio-file-upload'
                  ref={fileInputRef}
                  type='file'
                  accept='image/*,video/*,audio/*'
                  multiple
                  className='hidden'
                  onChange={(e) => {
                    handleMediaPick(e.target.files)
                    e.target.value = ''
                  }}
                />
              </label>

              <div className='flex-1' />

            {/* Right Controls: Model Pill & Send Arrow */}
            <div className='flex min-w-0 shrink items-center gap-1.5 sm:gap-2'>
              {/* Model Pill Trigger displaying ONLY model name per requirements */}
              <Popover open={popoverOpen} onOpenChange={setPopoverOpen}>
                <PopoverTrigger
                  className='inline-flex h-8 min-w-0 shrink items-center gap-1 rounded-lg border border-border/70 bg-muted/40 px-2.5 text-xs font-medium text-foreground transition hover:border-border hover:bg-muted'
                >
                  <span className='max-w-[72px] min-w-0 truncate min-[420px]:max-w-[120px] sm:max-w-[140px]'>
                    {currentModelDisplayName}
                  </span>
                  {currentModelRuntime?.tag && (
                    <span className='hidden min-[420px]:inline rounded bg-primary/10 px-1.5 py-0.2 text-[9px] font-semibold text-primary uppercase border border-primary/20 shrink-0'>
                      {currentModelRuntime.tag}
                    </span>
                  )}
                  <ChevronDownIcon className='size-3.5 shrink-0 text-muted-foreground' />
                </PopoverTrigger>

                <PopoverContent
                  align='end'
                  side='top'
                  className='w-80 rounded-2xl border border-border/80 bg-card p-3.5 shadow-xl backdrop-blur-md space-y-3'
                >
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
                      }}
                      value={currentModelName}
                    >
                      <SelectTrigger className='h-9 w-full rounded-xl border border-border/60 bg-background px-3 text-xs font-medium'>
                        <SelectValue placeholder={t('Select Model')} />
                      </SelectTrigger>
                      <SelectContent>
                        {(mode === 'video' ? videoModels : imageModels).map((m) => (
                          <SelectItem key={m.model} value={m.model} className='text-xs'>
                            <div className='flex items-center justify-between w-full gap-2'>
                              <span>{m.display_name || m.model}</span>
                              {m.tag && (
                                <span className='rounded bg-primary/10 px-1.5 py-0.2 text-[9px] font-semibold text-primary uppercase border border-primary/20 shrink-0'>
                                  {m.tag}
                                </span>
                              )}
                            </div>
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>

                  {/* Video parameters (Oran, Süre, Kalite) */}
                  {mode === 'video' && (
                    <>
                    <div className='grid grid-cols-3 gap-2 pt-0.5'>
                      <div className='space-y-1'>
                        <Label className='text-xs font-medium text-muted-foreground'>{t('Oran')}</Label>
                        <Select onValueChange={(val) => setVideoRatio(val ?? '')} value={ratio}>
                          <SelectTrigger className='h-8 w-full rounded-xl border border-border/60 bg-background px-2.5 text-xs font-medium'>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            {offeredRatios.map((r) => (
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
                            {offeredDurations.map((d) => (
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
                            {offeredVideoQualities.map((q) => (
                              <SelectItem key={q} value={q} className='text-xs'>
                                {q}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </div>
                    </div>

                    {/* Advanced Controls Collapsible */}
                    <div className='space-y-1.5 pt-2 border-t border-border/40'>
                      <button
                        type='button'
                        onClick={() => setShowAdvancedOptions(!showAdvancedOptions)}
                        className='flex items-center justify-between w-full text-xs font-medium text-muted-foreground hover:text-foreground transition'
                      >
                        <span>{t('Advanced Frame & Negative Options')}</span>
                        <span className='text-[10px] font-mono'>{showAdvancedOptions ? '▲' : '▼'}</span>
                      </button>
                      {showAdvancedOptions && (
                        <div className='space-y-2 pt-1'>
                          {/* Edit-mode hint for edit-capable video models */}
                          {selectedVideoModel?.edit_mode && selectedVideoModel.edit_mode !== 'none' && (
                            <p className='rounded-lg border border-border/60 bg-muted/30 px-2 py-1.5 text-[10px] leading-snug text-muted-foreground'>
                              {selectedVideoModel.edit_mode === 'only'
                                ? t('This model edits an input video — attach the video to edit with the + button before generating.')
                                : t('This model can edit an input video — attach a video with the + button to edit it, or generate from text or image as usual.')}
                            </p>
                          )}

                          {/* Generate audio — disabled with reason when the model can't */}
                          <div className='flex items-center justify-between gap-2 px-1 py-1.5'>
                            <div className='min-w-0'>
                              <Label
                                className='text-[11px] text-foreground/90'
                                htmlFor='studio-generate-audio'
                              >
                                {t('Generate audio')}
                              </Label>
                              <p className='text-[9px] leading-snug text-muted-foreground'>
                                {audioRenderAvailable
                                  ? t('The model renders an audio track with the video.')
                                  : t('This model does not generate audio.')}
                              </p>
                            </div>
                            <Switch
                              checked={audioEnabled}
                              disabled={!audioRenderAvailable}
                              id='studio-generate-audio'
                              onCheckedChange={setAudioEnabled}
                            />
                          </div>

                          <div className='space-y-1'>
                            <Label className='text-[11px] text-muted-foreground'>{t('Negative Prompt')}</Label>
                            <Input
                              value={negativePrompt}
                              onChange={(e) => setNegativePrompt(e.target.value)}
                              placeholder={t('Avoid blur, bad anatomy, text...')}
                              className='h-7 text-xs'
                            />
                          </div>
                          <div className='grid grid-cols-2 gap-2'>
                            {/* Start Frame File Upload with Thumbnail */}
                            <div className='space-y-1'>
                              <Label className='text-[11px] text-muted-foreground'>{t('Start Frame')}</Label>
                              {startFrame ? (
                                <div className='relative flex items-center gap-1.5 rounded-lg border border-border/70 p-1 bg-muted/30'>
                                  <img
                                    src={startFrame}
                                    alt='Start frame'
                                    className='size-7 rounded object-cover border border-border/40'
                                  />
                                  <span className='text-[10px] text-muted-foreground truncate flex-1 font-mono'>
                                    {t('Start Frame')}
                                  </span>
                                  <button
                                    type='button'
                                    onClick={() => setStartFrame('')}
                                    className='text-muted-foreground hover:text-destructive p-0.5'
                                    title={t('Clear')}
                                  >
                                    <XIcon className='size-3.5' />
                                  </button>
                                </div>
                              ) : (
                                <label className='flex h-7 items-center justify-center gap-1.5 rounded-lg border border-dashed border-border/80 hover:border-border hover:bg-muted/40 cursor-pointer text-[10px] text-muted-foreground transition px-2'>
                                  <PlusIcon className='size-3' />
                                  <span>{t('Upload Image')}</span>
                                  <input
                                    type='file'
                                    accept='image/*'
                                    className='hidden'
                                    onChange={(e) => {
                                      const file = e.target.files?.[0]
                                      if (file) {
                                        const reader = new FileReader()
                                        reader.onload = () => {
                                          if (typeof reader.result === 'string') {
                                            setStartFrame(reader.result)
                                          }
                                        }
                                        reader.readAsDataURL(file)
                                      }
                                    }}
                                  />
                                </label>
                              )}
                            </div>

                            {/* End Frame File Upload with Thumbnail */}
                            <div className='space-y-1'>
                              <Label className='text-[11px] text-muted-foreground'>{t('End Frame')}</Label>
                              {endFrame ? (
                                <div className='relative flex items-center gap-1.5 rounded-lg border border-border/70 p-1 bg-muted/30'>
                                  <img
                                    src={endFrame}
                                    alt='End frame'
                                    className='size-7 rounded object-cover border border-border/40'
                                  />
                                  <span className='text-[10px] text-muted-foreground truncate flex-1 font-mono'>
                                    {t('End Frame')}
                                  </span>
                                  <button
                                    type='button'
                                    onClick={() => setEndFrame('')}
                                    className='text-muted-foreground hover:text-destructive p-0.5'
                                    title={t('Clear')}
                                  >
                                    <XIcon className='size-3.5' />
                                  </button>
                                </div>
                              ) : (
                                <label className='flex h-7 items-center justify-center gap-1.5 rounded-lg border border-dashed border-border/80 hover:border-border hover:bg-muted/40 cursor-pointer text-[10px] text-muted-foreground transition px-2'>
                                  <PlusIcon className='size-3' />
                                  <span>{t('Upload Image')}</span>
                                  <input
                                    type='file'
                                    accept='image/*'
                                    className='hidden'
                                    onChange={(e) => {
                                      const file = e.target.files?.[0]
                                      if (file) {
                                        const reader = new FileReader()
                                        reader.onload = () => {
                                          if (typeof reader.result === 'string') {
                                            setEndFrame(reader.result)
                                          }
                                        }
                                        reader.readAsDataURL(file)
                                      }
                                    }}
                                  />
                                </label>
                              )}
                            </div>
                          </div>
                        </div>
                      )}
                    </div>
                    </>
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
                            {offeredImageQualities.map((q) => (
                              <SelectItem key={q} value={q} className='text-xs'>
                                {q}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </div>
                    </div>
                  )}

                  {/* Mandatory prompt optimizer notice when it is not configured */}
                  {!runtime.studio?.prompt_optimizer_model?.trim() && (
                    <div className='mt-2 flex items-start gap-2 rounded-lg border border-amber-500/40 bg-amber-500/10 p-2 text-[11px] leading-snug text-amber-700 dark:text-amber-400'>
                      <TriangleAlertIcon className='mt-0.5 size-3.5 shrink-0' />
                      <span>
                        {t('A prompt optimizer model is required for every Studio generation. Set it under System Settings → Playground & Studio.')}
                      </span>
                    </div>
                  )}
                </PopoverContent>
              </Popover>

              {/* Prompt optimizer model picker — only when several models are configured */}
              {optimizerModels.length > 1 && (
                <Popover open={optimizerPickerOpen} onOpenChange={setOptimizerPickerOpen}>
                  <PopoverTrigger
                    className='hidden sm:flex h-8 max-w-[160px] shrink-0 items-center gap-1.5 rounded-lg border border-border/70 bg-muted/40 px-2.5 text-xs font-medium text-muted-foreground transition hover:border-border hover:bg-muted hover:text-foreground'
                    title={t('Prompt optimizer model')}
                  >
                    <span className='truncate font-mono'>
                      {optimizerModel || optimizerModels[0]}
                    </span>
                    <ChevronDownIcon className='size-3 shrink-0' />
                  </PopoverTrigger>
                  <PopoverContent
                    align='end'
                    side='top'
                    className='w-64 rounded-2xl border border-border/80 bg-card p-2 shadow-xl'
                  >
                    <p className='px-2 pb-1 pt-1 text-[10px] font-semibold uppercase tracking-wide text-muted-foreground'>
                      {t('Prompt optimizer model')}
                    </p>
                    {optimizerModels.map((model) => (
                      <button
                        key={model}
                        type='button'
                        onClick={() => {
                          setOptimizerModel(model)
                          setOptimizerPickerOpen(false)
                        }}
                        className={cn(
                          'flex w-full items-center justify-between gap-2 rounded-lg px-2 py-1.5 text-left text-xs font-mono transition hover:bg-muted',
                          optimizerModel === model ? 'text-foreground' : 'text-muted-foreground'
                        )}
                      >
                        <span className='truncate'>{model}</span>
                        {optimizerModel === model && <CheckIcon className='size-3.5 shrink-0' />}
                      </button>
                    ))}
                  </PopoverContent>
                </Popover>
              )}

              {/* Circular Send Arrow Button */}
              <button
                type='button'
                disabled={!canGenerate || hasActivePending}
                title={(() => {
                  if (hasActivePending) {
                    return t('An active generation is in progress. Please wait for it to complete or clear it from the stage.')
                  }
                  if (!prompt.trim()) return t('Please enter a prompt')
                  if (!currentModelName) return t('Please select a model')
                  return t('Generate')
                })()}
                onClick={() => void handleGenerate()}
                className='flex size-8 sm:size-9 shrink-0 items-center justify-center rounded-full bg-foreground text-background hover:bg-foreground/90 disabled:opacity-30 disabled:cursor-not-allowed transition shadow-sm'
              >
                {isGenerating || hasActivePending ? (
                  <Loader2Icon className='size-4 animate-spin' />
                ) : (
                  <ArrowUpIcon className='size-4' />
                )}
              </button>
            </div>
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
                {t('Your previous video and image creations.')}
              </DialogDescription>
            </div>
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
              </TabsList>
            </Tabs>
          </div>

          <VideoStorageQuota
            onConfirmOldest={() => setConfirmOldestOpen(true)}
            deleting={deletingStorage}
          />

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
                      <div className='flex items-center gap-1.5'>
                        <Badge
                          className='gap-1 border-border/50 bg-background/80 text-[10px]'
                          variant='outline'
                        >
                          {item.mode === 'video' ? (
                            <VideoIcon className='size-2.5' />
                          ) : (
                            <ImageIcon className='size-2.5' />
                          )}
                          {item.mode === 'video' ? t('Video') : t('Image')}
                        </Badge>
                        {item.localOnly && (
                          <Badge
                            variant='outline'
                            className='gap-1 border-amber-500/40 bg-amber-500/10 text-[9px] text-amber-600 dark:text-amber-400'
                          >
                            {t('Stored in this browser')}
                          </Badge>
                        )}
                      </div>
                      <span className='text-[10px] text-muted-foreground font-mono'>
                        {new Date(item.createdAt).toLocaleDateString()}
                      </span>
                    </div>

                    <div className='relative my-2 flex min-h-36 max-h-48 items-center justify-center overflow-hidden bg-muted/20 rounded-md mx-2'>
                      {(() => {
                        const playable = item.localOnly
                          ? (localOnlyUrls[item.id] ?? '')
                          : getPlayableMediaUrl(item.urls[0])
                        if (item.status === 'pending') {
                          return (
                            <div className='flex flex-col items-center justify-center gap-1.5 p-4 text-center'>
                              <Loader2Icon className='size-5 animate-spin text-muted-foreground' />
                              <span className='text-xs text-muted-foreground'>{t('Generating...')}</span>
                            </div>
                          )
                        }
                        if (item.localOnly && !playable) {
                          return (
                            <div className='flex flex-col items-center justify-center gap-1.5 p-4 text-center'>
                              <Loader2Icon className='size-5 animate-spin text-muted-foreground' />
                              <span className='text-[10px] text-muted-foreground'>{t('Loading local video…')}</span>
                            </div>
                          )
                        }
                        if (item.status === 'error' && playable && !brokenMediaIds.has(item.id)) {
                          return (
                            <VideoJsPlayer
                              key={playable}
                              controls
                              preload='metadata'
                              src={playable}
                              onError={() => markMediaBroken(item.id)}
                              qualityLabel={item.settings?.quality != null ? String(item.settings.quality) : undefined}
                              className='max-h-48 w-full object-contain bg-black rounded'
                            />
                          )
                        }
                        if (item.status === 'error') {
                          return (
                            <div className='p-3 text-center text-destructive text-xs'>
                              {item.error || t('Generation failed')}
                            </div>
                          )
                        }
                        if (brokenMediaIds.has(item.id)) {
                          return (
                            <div className='flex h-full flex-col items-center justify-center gap-1.5 p-4 text-center'>
                              <VideoIcon className='size-6 text-muted-foreground/40' />
                              <span className='text-[10px] text-muted-foreground'>{t('The video could not be loaded.')}</span>
                            </div>
                          )
                        }
                        if (item.status === 'ready' && playable) {
                          return item.mode === 'video' ? (
                            <VideoJsPlayer
                              key={playable}
                              controls
                              preload='metadata'
                              src={playable}
                              onError={() => markMediaBroken(item.id)}
                              qualityLabel={item.settings?.quality != null ? String(item.settings.quality) : undefined}
                              className='max-h-48 w-full object-contain bg-black rounded'
                            />
                          ) : (
                            <img
                              alt={item.prompt}
                              src={playable}
                              className='max-h-48 w-full object-contain rounded'
                            />
                          )
                        }
                        return null
                      })()}
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
                          {item.status === 'ready' && (item.urls[0] || item.localOnly) && (
                            <a
                              download
                              href={(item.localOnly ? localOnlyUrls[item.id] : getPlayableMediaUrl(item.urls[0])) || undefined}
                              rel='noreferrer'
                              target='_blank'
                              className='inline-flex items-center justify-center size-7 rounded-md border bg-background hover:bg-muted text-foreground transition aria-disabled:pointer-events-none aria-disabled:opacity-50'
                              aria-disabled={!(item.localOnly ? localOnlyUrls[item.id] : item.urls[0])}
                              title={t('Download')}
                            >
                              <DownloadIcon className='size-3.5' />
                            </a>
                          )}
                          <Button
                            size='icon'
                            variant='ghost'
                            onClick={() => setDetailsItemId(item.id)}
                            className='size-7 text-muted-foreground hover:text-foreground'
                            title={t('Details')}
                            aria-label={t('Details')}
                          >
                            <InfoIcon className='size-3.5' />
                          </Button>
                          <Button
                            size='icon'
                            variant='ghost'
                            onClick={() => setDeleteConfirmItemId(item.id)}
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

      {/* Delete Confirmation Alert Dialog */}
      <AlertDialog
        open={Boolean(deleteConfirmItemId)}
        onOpenChange={(open) => !open && setDeleteConfirmItemId(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Delete Generation')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('Are you sure you want to permanently delete this generation? This action cannot be undone.')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction
              className='bg-destructive text-destructive-foreground hover:bg-destructive/90'
              onClick={() => {
                if (deleteConfirmItemId) {
                  removeItem(deleteConfirmItemId)
                  if (activeSessionItemId === deleteConfirmItemId) {
                    setActiveSessionItemId(null)
                  }
                  setDeleteConfirmItemId(null)
                }
              }}
            >
              {t('Delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Delete Oldest Videos Confirmation Alert Dialog */}
      <AlertDialog open={confirmOldestOpen} onOpenChange={setConfirmOldestOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Delete the oldest videos?')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('The oldest stored videos are deleted until at least 256 MB are freed. This cannot be undone.')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                setConfirmOldestOpen(false)
                void runDeleteStorage(false)
              }}
              className='bg-destructive text-destructive-foreground hover:bg-destructive/90'
            >
              {t('Delete oldest')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Delete ALL Videos Confirmation Alert Dialog */}
      <AlertDialog open={confirmAllOpen} onOpenChange={setConfirmAllOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Delete ALL stored videos?')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                'Freeing that much space would delete your entire stored library ({{count}} videos · {{size}}). Delete everything?',
                { count: totalToWipe.count, size: formatBytes(totalToWipe.bytes) }
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                setConfirmAllOpen(false)
                void runDeleteStorage(true)
              }}
              className='bg-destructive text-destructive-foreground hover:bg-destructive/90'
            >
              {t('Delete all')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Item details dialog */}
      <StudioItemDetailsDialog
        item={detailsItemId ? (items.find((i) => i.id === detailsItemId) ?? null) : null}
        open={Boolean(detailsItemId)}
        onOpenChange={(open) => !open && setDetailsItemId(null)}
      />
    </div>
  )
}
