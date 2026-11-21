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

import { useState, useRef, useLayoutEffect } from 'react'
import { useTranslation } from 'react-i18next'
import {
  ChevronRight,
  ChevronLeft,
  Check,
} from 'lucide-react'
import { cn } from '@/lib/utils'

export interface QualityItem {
  height: number
  label: string
  isOriginal: boolean
}

export interface FpsItem {
  fps: number
  label: string
  isOriginal: boolean
}

export interface VideoContextMenuProps {
  x: number
  y: number
  maxHeight?: number
  isWide?: boolean
  isPlaying: boolean
  isLooping: boolean
  showStats: boolean
  isFullscreen: boolean
  playbackSpeed: number
  qualities: QualityItem[]
  selectedQuality: number | null
  fpsList: FpsItem[]
  selectedFps: number | null
  onTogglePlay: () => void
  onToggleLoop: () => void
  onToggleStats: () => void
  onToggleFullscreen: () => void
  onDownload: () => void
  onCopyUrl: () => void
  onCopyUrlAtTime: () => void
  onSelectQuality: (h: number | null) => void
  onSelectFps: (f: number | null) => void
  onSelectSpeed: (s: number) => void
  onClose: () => void
}

const SPEED_OPTIONS = [0.25, 0.5, 0.75, 1, 1.25, 1.5, 1.75, 2]

export function VideoContextMenu({
  x,
  y,
  maxHeight,
  isWide = false,
  isPlaying,
  isLooping,
  showStats,
  isFullscreen,
  playbackSpeed,
  qualities,
  selectedQuality,
  fpsList,
  selectedFps,
  onTogglePlay,
  onToggleLoop,
  onToggleStats,
  onToggleFullscreen,
  onDownload,
  onCopyUrl,
  onCopyUrlAtTime,
  onSelectQuality,
  onSelectFps,
  onSelectSpeed,
  onClose,
}: VideoContextMenuProps) {
  const { t } = useTranslation()
  const [view, setView] = useState<'main' | 'quality' | 'fps' | 'speed'>('main')
  const menuRef = useRef<HTMLDivElement>(null)

  // Dynamically ensure menu never overflows the container bottom when container is tall enough
  useLayoutEffect(() => {
    if (!menuRef.current) return
    const el = menuRef.current
    const parent = el.parentElement
    if (!parent) return

    const parentHeight = parent.clientHeight
    const pad = 8
    const actualHeight = el.scrollHeight

    // Can the entire menu fit within the parent container?
    if (parentHeight - pad * 2 >= actualHeight) {
      // Yes! It can fit completely without scrolling.
      // Check if it overflows the bottom edge:
      const bottomEdge = el.offsetTop + actualHeight
      if (bottomEdge > parentHeight - pad) {
        // Shift UPWARDS so bottom fits within parentHeight - pad
        const adjustedTop = Math.max(pad, parentHeight - actualHeight - pad)
        el.style.top = `${adjustedTop}px`
      }
      // Never constrain maxHeight and eliminate all scrollbars
      el.style.maxHeight = ''
      el.classList.remove('overflow-y-auto', 'dark-custom-scrollbar')
      el.classList.add('overflow-hidden')
    } else {
      // Container is physically too short to show the menu (e.g. tiny 180px card).
      // Keep within bounds and enable scrolling.
      const available = parentHeight - el.offsetTop - pad
      el.style.maxHeight = `${Math.max(60, available)}px`
      el.classList.add('overflow-y-auto', 'dark-custom-scrollbar')
      el.classList.remove('overflow-hidden')
    }
  }, [view, y])

  // Clean labels without parentheses
  const currentQualityLabel =
    selectedQuality === null
      ? t('Auto')
      : `${selectedQuality}p`

  const currentFpsLabel =
    selectedFps === null
      ? t('Original')
      : `${selectedFps} FPS`

  return (
    <div
      ref={menuRef}
      style={{
        left: `${x}px`,
        top: `${y}px`,
        maxHeight: maxHeight ? `${maxHeight}px` : undefined,
      }}
      onClick={(e) => e.stopPropagation()}
      className={cn(
        "absolute z-50 rounded-lg bg-zinc-900/85 backdrop-blur-xl border border-white/10 p-1 shadow-2xl text-zinc-100 select-none studio-context-menu text-[11px] sm:text-xs animate-in fade-in zoom-in-95 duration-150",
        isWide ? "is-wide w-58 sm:w-60" : "w-48 sm:w-52",
        maxHeight ? "overflow-y-auto dark-custom-scrollbar" : "overflow-hidden"
      )}
    >
      {view === 'main' && (
        <div className="space-y-0.5">
          {/* 1. Play / Pause */}
          <button
            type="button"
            onClick={() => {
              onTogglePlay()
              onClose()
            }}
            className="w-full flex items-center justify-between px-2 py-1.25 rounded-md hover:bg-white/10 text-left transition-colors studio-menu-button"
          >
            <span className="whitespace-nowrap">{isPlaying ? t('Pause') : t('Play')}</span>
          </button>

          {/* 2. Loop */}
          <button
            type="button"
            onClick={() => {
              onToggleLoop()
              onClose()
            }}
            className="w-full flex items-center justify-between px-2 py-1.25 rounded-md hover:bg-white/10 text-left transition-colors studio-menu-button"
          >
            <span className="whitespace-nowrap">{t('Loop')}</span>
            {isLooping && <Check className="w-3.5 h-3.5 text-white studio-menu-icon" />}
          </button>

          {/* 3. Stats for Nerds */}
          <button
            type="button"
            onClick={() => {
              onToggleStats()
              onClose()
            }}
            className="w-full flex items-center justify-between px-2 py-1.25 rounded-md hover:bg-white/10 text-left transition-colors studio-menu-button"
          >
            <span className="whitespace-nowrap">{t('Stats for nerds')}</span>
            {showStats && <Check className="w-3.5 h-3.5 text-white studio-menu-icon" />}
          </button>

          <div className="h-px bg-white/10 my-0.5" />

          {/* 4. Quality */}
          <button
            type="button"
            onClick={() => setView('quality')}
            className="w-full flex items-center justify-between px-2 py-1.25 rounded-md hover:bg-white/10 text-left transition-colors studio-menu-button"
          >
            <span className="whitespace-nowrap">{t('Quality')}</span>
            <div className="flex items-center gap-1 text-[11px] text-zinc-400 shrink-0">
              <span className="truncate max-w-[70px]">{currentQualityLabel}</span>
              <ChevronRight className="w-3.5 h-3.5 text-zinc-400 studio-menu-icon" />
            </div>
          </button>

          {/* 5. FPS */}
          <button
            type="button"
            onClick={() => setView('fps')}
            className="w-full flex items-center justify-between px-2 py-1.25 rounded-md hover:bg-white/10 text-left transition-colors studio-menu-button"
          >
            <span className="whitespace-nowrap">{t('Frame Rate')}</span>
            <div className="flex items-center gap-1 text-[11px] text-zinc-400 shrink-0">
              <span className="truncate max-w-[65px]">{currentFpsLabel}</span>
              <ChevronRight className="w-3.5 h-3.5 text-zinc-400 studio-menu-icon" />
            </div>
          </button>

          {/* 6. Speed */}
          <button
            type="button"
            onClick={() => setView('speed')}
            className="w-full flex items-center justify-between px-2 py-1.25 rounded-md hover:bg-white/10 text-left transition-colors studio-menu-button"
          >
            <span className="whitespace-nowrap">{t('Playback speed')}</span>
            <div className="flex items-center gap-1 text-[11px] text-zinc-400 shrink-0">
              <span>{playbackSpeed === 1 ? t('Normal') : `${playbackSpeed}x`}</span>
              <ChevronRight className="w-3.5 h-3.5 text-zinc-400 studio-menu-icon" />
            </div>
          </button>

          <div className="h-px bg-white/10 my-0.5" />

          {/* 7. Download */}
          <button
            type="button"
            onClick={() => {
              onDownload()
              onClose()
            }}
            className="w-full flex items-center px-2 py-1.25 rounded-md hover:bg-white/10 text-left transition-colors studio-menu-button"
          >
            <span className="whitespace-nowrap">{t('Download video')}</span>
          </button>

          {/* 8. Copy URL */}
          <button
            type="button"
            onClick={() => {
              onCopyUrl()
              onClose()
            }}
            className="w-full flex items-center px-2 py-1.25 rounded-md hover:bg-white/10 text-left transition-colors studio-menu-button"
          >
            <span className="whitespace-nowrap">{t('Copy video URL')}</span>
          </button>

          {/* 9. Copy URL at current time */}
          <button
            type="button"
            onClick={() => {
              onCopyUrlAtTime()
              onClose()
            }}
            className="w-full flex items-center px-2 py-1.25 rounded-md hover:bg-white/10 text-left transition-colors studio-menu-button"
          >
            <span className="whitespace-nowrap">{t('Copy video URL at current time')}</span>
          </button>

          <div className="h-px bg-white/10 my-0.5" />

          {/* 10. Fullscreen */}
          <button
            type="button"
            onClick={() => {
              onToggleFullscreen()
              onClose()
            }}
            className="w-full flex items-center px-2 py-1.25 rounded-md hover:bg-white/10 text-left transition-colors studio-menu-button"
          >
            <span className="whitespace-nowrap">{isFullscreen ? t('Exit full screen') : t('Full screen')}</span>
          </button>
        </div>
      )}

      {/* View: Quality */}
      {view === 'quality' && (
        <div className="space-y-0.5">
          <button
            type="button"
            onClick={() => setView('main')}
            className="w-full flex items-center gap-1.5 px-2 py-1.25 border-b border-white/10 font-medium text-zinc-200 hover:text-white transition-colors studio-menu-button"
          >
            <ChevronLeft className="w-3.5 h-3.5 studio-menu-icon" />
            <span className="whitespace-nowrap">{t('Quality')}</span>
          </button>
          <div className="max-h-36 sm:max-h-44 overflow-y-auto dark-custom-scrollbar pt-0.5 space-y-0.5">
            <button
              type="button"
              onClick={() => {
                onSelectQuality(null)
                onClose()
              }}
              className="w-full flex items-center justify-between px-2 py-1 rounded-md hover:bg-white/10 text-left transition-colors studio-menu-button"
            >
              <span className="whitespace-nowrap">{t('Auto')}</span>
              {selectedQuality === null && <Check className="w-3.5 h-3.5 text-white studio-menu-icon" />}
            </button>
            {qualities.map((item) => (
              <button
                key={item.height}
                type="button"
                onClick={() => {
                  onSelectQuality(item.isOriginal ? null : item.height)
                  onClose()
                }}
                className="w-full flex items-center justify-between px-2 py-1 rounded-md hover:bg-white/10 text-left transition-colors studio-menu-button"
              >
                <span className="whitespace-nowrap">{item.isOriginal ? `${item.label} (${t('Original')})` : item.label}</span>
                {selectedQuality === item.height && <Check className="w-3.5 h-3.5 text-white studio-menu-icon" />}
              </button>
            ))}
          </div>
        </div>
      )}

      {/* View: FPS */}
      {view === 'fps' && (
        <div className="space-y-0.5">
          <button
            type="button"
            onClick={() => setView('main')}
            className="w-full flex items-center gap-1.5 px-2 py-1.25 border-b border-white/10 font-medium text-zinc-200 hover:text-white transition-colors studio-menu-button"
          >
            <ChevronLeft className="w-3.5 h-3.5 studio-menu-icon" />
            <span className="whitespace-nowrap">{t('Frame Rate')}</span>
          </button>
          <div className="max-h-36 sm:max-h-44 overflow-y-auto dark-custom-scrollbar pt-0.5 space-y-0.5">
            <button
              type="button"
              onClick={() => {
                onSelectFps(null)
                onClose()
              }}
              className="w-full flex items-center justify-between px-2 py-1 rounded-md hover:bg-white/10 text-left transition-colors studio-menu-button"
            >
              <span className="whitespace-nowrap">{t('Original')}</span>
              {selectedFps === null && <Check className="w-3.5 h-3.5 text-white studio-menu-icon" />}
            </button>
            {fpsList.map((item) => (
              <button
                key={item.fps}
                type="button"
                onClick={() => {
                  onSelectFps(item.isOriginal ? null : item.fps)
                  onClose()
                }}
                className="w-full flex items-center justify-between px-2 py-1 rounded-md hover:bg-white/10 text-left transition-colors studio-menu-button"
              >
                <span className="whitespace-nowrap">{item.label}</span>
                {selectedFps === item.fps && <Check className="w-3.5 h-3.5 text-white studio-menu-icon" />}
              </button>
            ))}
          </div>
        </div>
      )}

      {/* View: Speed */}
      {view === 'speed' && (
        <div className="space-y-0.5">
          <button
            type="button"
            onClick={() => setView('main')}
            className="w-full flex items-center gap-1.5 px-2 py-1.25 border-b border-white/10 font-medium text-zinc-200 hover:text-white transition-colors studio-menu-button"
          >
            <ChevronLeft className="w-3.5 h-3.5 studio-menu-icon" />
            <span className="whitespace-nowrap">{t('Playback speed')}</span>
          </button>
          <div className="max-h-36 sm:max-h-44 overflow-y-auto dark-custom-scrollbar pt-0.5 space-y-0.5">
            {SPEED_OPTIONS.map((rate) => (
              <button
                key={rate}
                type="button"
                onClick={() => {
                  onSelectSpeed(rate)
                  onClose()
                }}
                className="w-full flex items-center justify-between px-2 py-1 rounded-md hover:bg-white/10 text-left transition-colors studio-menu-button"
              >
                <span className="whitespace-nowrap">{rate === 1 ? t('Normal') : `${rate}x`}</span>
                {playbackSpeed === rate && <Check className="w-3.5 h-3.5 text-white studio-menu-icon" />}
              </button>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
