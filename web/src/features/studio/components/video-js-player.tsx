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
 * Studio video player built on Video.js v10 default theme (@videojs/react)
 * Enhanced with custom context menu, stats for nerds, YouTube-style replay,
 * bottom-bar settings menu (Quality, FPS, Loop), and on-the-fly streaming transcoding.
 */
import '@videojs/react/video/skin.css'
import './video-js-player.css'
import { useState, useRef, useEffect, useMemo, useCallback } from 'react'
import { createPortal } from 'react-dom'
import { VideoPlayer, VideoSkin, Video } from '@videojs/react/video'
import { cn } from '@/lib/utils'
import { toast } from 'sonner'
import { VideoStatsDialog } from './video-stats-dialog'
import { VideoReplayOverlay } from './video-replay-overlay'
import { VideoContextMenu, type QualityItem, type FpsItem } from './video-context-menu'
import { VideoSettingsPopover } from './video-settings-popover'

export interface VideoJsPlayerProps {
  src: string
  type?: string
  className?: string
  autoPlay?: boolean
  loop?: boolean
  muted?: boolean
  controls?: boolean
  playsInline?: boolean
  preload?: 'auto' | 'metadata' | 'none'
  qualityLabel?: string
  onError?: () => void
}

const STANDARD_HEIGHTS = [
  { height: 2160, label: '4K' },
  { height: 1440, label: '1440p' },
  { height: 1080, label: '1080p' },
  { height: 720, label: '720p' },
  { height: 480, label: '480p' },
  { height: 360, label: '360p' },
  { height: 260, label: '260p' },
  { height: 144, label: '144p' },
]

const STANDARD_FPS = [60, 45, 30, 20, 15, 10, 5]

function getRawUrl(sourceUrl: string): string {
  try {
    if (sourceUrl.includes('url=')) {
      const parsed = new URL(sourceUrl, window.location.origin)
      const raw = parsed.searchParams.get('url')
      if (raw) return raw
    }
  } catch {}
  return sourceUrl
}

function getFullUrl(sourceUrl: string): string {
  const raw = getRawUrl(sourceUrl)
  if (raw.startsWith('http://') || raw.startsWith('https://')) {
    return raw
  }
  const cleanPath = raw.startsWith('/') ? raw : `/${raw}`
  return `${window.location.origin}${cleanPath}`
}

function buildTranscodedUrl(rawUrl: string, height: number | null, fps: number | null): string {
  if (!height && !fps) {
    if (rawUrl.startsWith('http://') || rawUrl.startsWith('https://')) {
      return `/api/media/cache?url=${encodeURIComponent(rawUrl)}`
    }
    return rawUrl
  }
  let url = `/api/media/cache?url=${encodeURIComponent(rawUrl)}`
  if (height) url += `&height=${height}`
  if (fps) url += `&fps=${fps}`
  return url
}

export function VideoJsPlayer({
  src,
  autoPlay = false,
  loop: initialLoop = false,
  muted = false,
  playsInline = true,
  preload = 'auto',
  className,
  onError,
}: VideoJsPlayerProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const videoRef = useRef<HTMLVideoElement>(null)

  // Player state
  const [currentSrc, setCurrentSrc] = useState(src)
  const [isPlaying, setIsPlaying] = useState(false)
  const [isEnded, setIsEnded] = useState(false)
  const [isLooping, setIsLooping] = useState(initialLoop)
  const [showStats, setShowStats] = useState(false)
  const [isFullscreen, setIsFullscreen] = useState(false)
  const [playbackSpeed, setPlaybackSpeed] = useState(1)

  // Context menu state
  const [contextMenu, setContextMenu] = useState<{ x: number; y: number; maxHeight?: number; isWide?: boolean } | null>(null)

  // Native metadata & selected options
  const [nativeWidth, setNativeWidth] = useState<number | undefined>(undefined)
  const [nativeHeight, setNativeHeight] = useState<number | undefined>(undefined)
  const [nativeFps, setNativeFps] = useState<number | undefined>(undefined)
  const [selectedQuality, setSelectedQuality] = useState<number | null>(null)
  const [selectedFps, setSelectedFps] = useState<number | null>(null)

  // Portal target for bottom controls bar
  const [controlsPrimaryEl, setControlsPrimaryEl] = useState<HTMLElement | null>(null)

  // Update currentSrc when external src changes
  useEffect(() => {
    setCurrentSrc(src)
    setSelectedQuality(null)
    setSelectedFps(null)
    setIsEnded(false)
  }, [src])

  // Probe media metadata on mount/src change
  useEffect(() => {
    const rawUrl = getRawUrl(src)
    if (!rawUrl) return

    let isMounted = true
    fetch(`/api/media/probe?url=${encodeURIComponent(rawUrl)}`)
      .then((res) => (res.ok ? res.json() : null))
      .then((data) => {
        if (!isMounted || !data) return
        if (data.width) setNativeWidth(data.width)
        if (data.height) setNativeHeight(data.height)
        if (data.fps) setNativeFps(Math.round(data.fps))
      })
      .catch(() => {})

    return () => {
      isMounted = false
    }
  }, [src])

  // Fullscreen change listener
  useEffect(() => {
    const handleFullscreenChange = () => {
      const isFs = Boolean(
        document.fullscreenElement &&
        containerRef.current &&
        (document.fullscreenElement === containerRef.current ||
          containerRef.current.contains(document.fullscreenElement) ||
          document.fullscreenElement.contains(containerRef.current))
      )
      setIsFullscreen(isFs)
    }

    document.addEventListener('fullscreenchange', handleFullscreenChange)
    return () => document.removeEventListener('fullscreenchange', handleFullscreenChange)
  }, [])

  // Close context menu on outside click
  useEffect(() => {
    if (!contextMenu) return
    const handleClickOutside = () => {
      setContextMenu(null)
    }
    window.addEventListener('click', handleClickOutside)
    window.addEventListener('contextmenu', handleClickOutside)
    return () => {
      window.removeEventListener('click', handleClickOutside)
      window.removeEventListener('contextmenu', handleClickOutside)
    }
  }, [contextMenu])

  // Locate .video-controls-primary for portal injection of settings button
  useEffect(() => {
    const locateControls = () => {
      if (containerRef.current) {
        const el = containerRef.current.querySelector('.video-controls-primary') as HTMLElement | null
        if (el && el !== controlsPrimaryEl) {
          setControlsPrimaryEl(el)
        }
      }
    }

    locateControls()
    const timer1 = setTimeout(locateControls, 100)
    const timer2 = setTimeout(locateControls, 500)
    return () => {
      clearTimeout(timer1)
      clearTimeout(timer2)
    }
  }, [controlsPrimaryEl])

  // Compute available qualities without parentheses
  const availableQualities: QualityItem[] = useMemo(() => {
    const origH = nativeHeight || 720
    let list = STANDARD_HEIGHTS.filter((item) => item.height <= origH)

    // If native height is non-standard (e.g. 768p, 512p, 1024p), add it
    if (!STANDARD_HEIGHTS.some((item) => item.height === origH)) {
      list.push({ height: origH, label: `${origH}p` })
      list.sort((a, b) => b.height - a.height)
    }

    return list.map((item) => ({
      ...item,
      isOriginal: item.height === origH,
      label: item.label,
    }))
  }, [nativeHeight])

  // Compute available FPS without parentheses
  const availableFps: FpsItem[] = useMemo(() => {
    const origFps = nativeFps || 30
    let list = STANDARD_FPS.filter((fps) => fps <= origFps)

    if (!STANDARD_FPS.includes(origFps)) {
      list.push(origFps)
      list.sort((a, b) => b - a)
    }

    return list.map((fps) => ({
      fps,
      isOriginal: fps === origFps,
      label: `${fps} FPS`,
    }))
  }, [nativeFps])

  // Switch quality or FPS seamlessly
  const switchQualityOrFps = useCallback(
    (newHeight: number | null, newFps: number | null) => {
      setSelectedQuality(newHeight)
      setSelectedFps(newFps)

      const video = videoRef.current
      const currentTime = video?.currentTime || 0
      const wasPlaying = video ? !video.paused : false

      const rawUrl = getRawUrl(src)
      const nextUrl = buildTranscodedUrl(rawUrl, newHeight, newFps)
      setCurrentSrc(nextUrl)

      if (video) {
        const handleRestore = () => {
          video.removeEventListener('loadeddata', handleRestore)
          video.currentTime = currentTime
          if (wasPlaying) {
            video.play().catch(() => {})
          }
        }
        video.addEventListener('loadeddata', handleRestore)
      }
    },
    [src]
  )

  const handleSelectQuality = useCallback(
    (h: number | null) => {
      switchQualityOrFps(h, selectedFps)
    },
    [selectedFps, switchQualityOrFps]
  )

  const handleSelectFps = useCallback(
    (f: number | null) => {
      switchQualityOrFps(selectedQuality, f)
    },
    [selectedQuality, switchQualityOrFps]
  )

  const handleSelectSpeed = useCallback((s: number) => {
    setPlaybackSpeed(s)
    if (videoRef.current) {
      videoRef.current.playbackRate = s
    }
  }, [])

  // Video event listeners
  const handleLoadedMetadata = () => {
    if (videoRef.current) {
      const h = videoRef.current.videoHeight
      const w = videoRef.current.videoWidth
      if (h && !nativeHeight) setNativeHeight(h)
      if (w && !nativeWidth) setNativeWidth(w)
      videoRef.current.playbackRate = playbackSpeed
    }
  }

  const handleEnded = () => {
    setIsPlaying(false)
    if (!isLooping) {
      setIsEnded(true)
    }
  }

  const handleReplay = useCallback(() => {
    setIsEnded(false)
    if (videoRef.current) {
      videoRef.current.currentTime = 0
      videoRef.current.play().catch(() => {})
      setIsPlaying(true)
    }
  }, [])

  const togglePlay = useCallback(() => {
    if (!videoRef.current) return
    if (videoRef.current.paused || videoRef.current.ended) {
      setIsEnded(false)
      videoRef.current.play().catch(() => {})
    } else {
      videoRef.current.pause()
    }
  }, [])

  const toggleLoop = useCallback(() => {
    setIsLooping((prev) => {
      const next = !prev
      if (videoRef.current) {
        videoRef.current.loop = next
      }
      return next
    })
  }, [])

  const toggleFullscreen = useCallback(() => {
    const el = containerRef.current
    if (!el) return
    if (document.fullscreenElement) {
      document.exitFullscreen().catch(() => {})
    } else {
      el.requestFullscreen().catch(() => {})
    }
  }, [])

  const handleDownload = useCallback(async () => {
    try {
      const downloadSrc = currentSrc
      const res = await fetch(downloadSrc)
      const blob = await res.blob()
      const blobUrl = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = blobUrl
      a.download = `video-${Date.now()}.mp4`
      document.body.appendChild(a)
      a.click()
      document.body.removeChild(a)
      URL.revokeObjectURL(blobUrl)
      toast.success('Video indirilmeye başlandı')
    } catch {
      window.open(currentSrc, '_blank')
    }
  }, [currentSrc])

  const handleCopyUrl = useCallback(async () => {
    const full = getFullUrl(src)
    await navigator.clipboard.writeText(full)
    toast.success('Video bağlantısı panoya kopyalandı')
  }, [src])

  const handleCopyUrlAtTime = useCallback(async () => {
    const full = getFullUrl(src)
    const time = Math.floor(videoRef.current?.currentTime || 0)
    const urlWithTime = `${full}#t=${time}`
    await navigator.clipboard.writeText(urlWithTime)
    toast.success(`Video bağlantısı (${time}s) panoya kopyalandı`)
  }, [src])

  // Custom Context Menu handler:
  // 1. Dynamic width: wide menus on wide video boxes (and in fullscreen).
  // 2. Smart positioning: if opening downwards would cause a scrollbar,
  //    checks if the full menu fits slightly higher up / upwards and opens fully without scrolling.
  //    Only scrolls if the video container itself is shorter than the menu.
  const handleContextMenu = (e: React.MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()

    const fsEl = document.fullscreenElement as HTMLElement | null
    const skinEl = containerRef.current?.querySelector('.studio-video-skin') as HTMLElement | null
    const container = fsEl || skinEl || containerRef.current
    if (!container) return

    const rect = container.getBoundingClientRect()
    const clickX = e.clientX - rect.left
    const clickY = e.clientY - rect.top

    const isWide = rect.width >= 330
    const menuWidth = isWide ? 232 : 184
    const naturalMenuHeight = 250
    const pad = 8

    // Horizontal clamping
    let x = clickX
    if (x + menuWidth > rect.width - pad) {
      x = Math.max(pad, rect.width - menuWidth - pad)
    } else {
      x = Math.max(pad, x)
    }

    // Vertical positioning:
    // Can the entire menu fit within the video player (from pad to rect.height - pad)?
    const totalAvailableHeight = rect.height - pad * 2
    let y: number
    let maxHeight: number | undefined

    if (totalAvailableHeight >= naturalMenuHeight) {
      // 1. Container has plenty of height to display the entire menu without scrolling!
      if (clickY + naturalMenuHeight <= rect.height - pad) {
        // Fits downwards from the click position
        y = clickY
      } else {
        // Shift upwards so it stays completely within container without scrolling
        y = Math.max(pad, rect.height - naturalMenuHeight - pad)
      }
      maxHeight = undefined
    } else {
      // 2. Container itself is physically too short to display the full menu (tiny card)
      const spaceBelow = rect.height - clickY - pad
      const spaceAbove = clickY - pad
      if (spaceBelow >= spaceAbove) {
        y = Math.max(pad, clickY)
        maxHeight = Math.max(60, spaceBelow)
      } else {
        maxHeight = Math.max(60, spaceAbove)
        y = Math.max(pad, clickY - maxHeight)
      }
    }

    setContextMenu({ x, y, maxHeight, isWide })
  }

  // Gracefully handle video playback errors
  const handleVideoError = () => {
    if (selectedQuality !== null || selectedFps !== null) {
      // Transcode stream error: revert to original without marking item as broken
      setSelectedQuality(null)
      setSelectedFps(null)
      setCurrentSrc(src)
      toast.error('Seçilen kalitede video akışı açılamadı, orijinal kaliteye dönüldü')
      return
    }
    // Only call outer onError if the original source itself failed
    onError?.()
  }

  return (
    <div
      ref={containerRef}
      className={cn('relative w-full h-full select-none overflow-hidden rounded-inherit group/studio-player', className)}
      onContextMenu={handleContextMenu}
    >
      <VideoPlayer>
        <VideoSkin className="studio-video-skin" onContextMenu={handleContextMenu}>
          <Video
            ref={videoRef}
            src={currentSrc}
            autoPlay={autoPlay}
            loop={isLooping}
            muted={muted}
            controls={false}
            playsInline={playsInline}
            preload={preload}
            onError={handleVideoError}
            onLoadedMetadata={handleLoadedMetadata}
            onPlay={() => {
              setIsPlaying(true)
              setIsEnded(false)
            }}
            onPause={() => setIsPlaying(false)}
            onEnded={handleEnded}
          />

          {/* 1. Large Central Replay Button (When ended && !isLooping) */}
          {isEnded && !isLooping && <VideoReplayOverlay onReplay={handleReplay} />}

          {/* 2. Stats for Nerds (Geliştirici Bilgisi) Overlay */}
          {showStats && (
            <VideoStatsDialog
              videoEl={videoRef.current}
              src={currentSrc}
              selectedQualityLabel={
                selectedQuality
                  ? `${selectedQuality}p`
                  : availableQualities.find((q) => q.isOriginal)?.label
              }
              selectedFpsLabel={selectedFps ? `${selectedFps} FPS` : undefined}
              nativeWidth={nativeWidth}
              nativeHeight={nativeHeight}
              nativeFps={nativeFps}
              isLooping={isLooping}
              onClose={() => setShowStats(false)}
            />
          )}

          {/* 3. Custom Right-Click Context Menu (Inside VideoSkin so it works in fullscreen) */}
          {contextMenu && (
            <VideoContextMenu
              x={contextMenu.x}
              y={contextMenu.y}
              maxHeight={contextMenu.maxHeight}
              isWide={contextMenu.isWide}
              isPlaying={isPlaying}
              isLooping={isLooping}
              showStats={showStats}
              isFullscreen={isFullscreen}
              playbackSpeed={playbackSpeed}
              qualities={availableQualities}
              selectedQuality={selectedQuality}
              fpsList={availableFps}
              selectedFps={selectedFps}
              onTogglePlay={togglePlay}
              onToggleLoop={toggleLoop}
              onToggleStats={() => setShowStats((prev) => !prev)}
              onToggleFullscreen={toggleFullscreen}
              onDownload={handleDownload}
              onCopyUrl={handleCopyUrl}
              onCopyUrlAtTime={handleCopyUrlAtTime}
              onSelectQuality={handleSelectQuality}
              onSelectFps={handleSelectFps}
              onSelectSpeed={handleSelectSpeed}
              onClose={() => setContextMenu(null)}
            />
          )}
        </VideoSkin>
      </VideoPlayer>

      {/* 4. Bottom Bar Custom Settings Popover via Portal */}
      {controlsPrimaryEl &&
        createPortal(
          <VideoSettingsPopover
            isLooping={isLooping}
            showStats={showStats}
            playbackSpeed={playbackSpeed}
            qualities={availableQualities}
            selectedQuality={selectedQuality}
            fpsList={availableFps}
            selectedFps={selectedFps}
            onToggleLoop={toggleLoop}
            onToggleStats={() => setShowStats((prev) => !prev)}
            onSelectQuality={handleSelectQuality}
            onSelectFps={handleSelectFps}
            onSelectSpeed={handleSelectSpeed}
          />,
          controlsPrimaryEl
        )}
    </div>
  )
}
