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

import { useState } from 'react'
import {
  Play,
  Pause,
  Repeat,
  Activity,
  Sliders,
  Layers,
  Clock,
  Download,
  Copy,
  Maximize,
  Minimize,
  ChevronRight,
  ChevronLeft,
  Check,
} from 'lucide-react'

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
  const [view, setView] = useState<'main' | 'quality' | 'fps' | 'speed'>('main')

  // Selected labels
  const currentQualityLabel =
    selectedQuality === null
      ? (qualities.find((q) => q.isOriginal)?.label || 'Otomatik')
      : (qualities.find((q) => q.height === selectedQuality)?.label || `${selectedQuality}p`)

  const currentFpsLabel =
    selectedFps === null
      ? (fpsList.find((f) => f.isOriginal)?.label || 'Orijinal')
      : `${selectedFps} FPS`

  return (
    <div
      style={{ left: `${x}px`, top: `${y}px` }}
      onClick={(e) => e.stopPropagation()}
      className="absolute z-50 w-64 rounded-xl bg-zinc-900/95 backdrop-blur-xl border border-white/15 p-1.5 shadow-2xl text-zinc-200 text-xs select-none animate-in fade-in zoom-in-95 duration-150"
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
            className="w-full flex items-center gap-2.5 px-2.5 py-2 rounded-lg hover:bg-white/10 text-left transition-colors"
          >
            {isPlaying ? (
              <>
                <Pause className="w-4 h-4 text-blue-400" />
                <span>Durdur</span>
              </>
            ) : (
              <>
                <Play className="w-4 h-4 text-blue-400" />
                <span>Oynat</span>
              </>
            )}
          </button>

          {/* 2. Loop */}
          <button
            type="button"
            onClick={() => {
              onToggleLoop()
              onClose()
            }}
            className="w-full flex items-center justify-between px-2.5 py-2 rounded-lg hover:bg-white/10 text-left transition-colors"
          >
            <div className="flex items-center gap-2.5">
              <Repeat className={`w-4 h-4 ${isLooping ? 'text-emerald-400' : 'text-zinc-400'}`} />
              <span>Döngü</span>
            </div>
            {isLooping && <Check className="w-3.5 h-3.5 text-emerald-400" />}
          </button>

          {/* 3. Stats for Nerds */}
          <button
            type="button"
            onClick={() => {
              onToggleStats()
              onClose()
            }}
            className="w-full flex items-center justify-between px-2.5 py-2 rounded-lg hover:bg-white/10 text-left transition-colors"
          >
            <div className="flex items-center gap-2.5">
              <Activity className={`w-4 h-4 ${showStats ? 'text-amber-400' : 'text-zinc-400'}`} />
              <span>Geliştirici bilgisi</span>
            </div>
            {showStats && <Check className="w-3.5 h-3.5 text-amber-400" />}
          </button>

          <div className="h-px bg-white/10 my-1" />

          {/* 4. Quality */}
          <button
            type="button"
            onClick={() => setView('quality')}
            className="w-full flex items-center justify-between px-2.5 py-2 rounded-lg hover:bg-white/10 text-left transition-colors"
          >
            <div className="flex items-center gap-2.5">
              <Sliders className="w-4 h-4 text-indigo-400" />
              <span>Kalite</span>
            </div>
            <div className="flex items-center gap-1 text-[11px] text-zinc-400">
              <span className="truncate max-w-[90px]">{currentQualityLabel}</span>
              <ChevronRight className="w-3.5 h-3.5 text-zinc-400" />
            </div>
          </button>

          {/* 5. FPS */}
          <button
            type="button"
            onClick={() => setView('fps')}
            className="w-full flex items-center justify-between px-2.5 py-2 rounded-lg hover:bg-white/10 text-left transition-colors"
          >
            <div className="flex items-center gap-2.5">
              <Layers className="w-4 h-4 text-cyan-400" />
              <span>Kare Hızı (FPS)</span>
            </div>
            <div className="flex items-center gap-1 text-[11px] text-zinc-400">
              <span className="truncate max-w-[80px]">{currentFpsLabel}</span>
              <ChevronRight className="w-3.5 h-3.5 text-zinc-400" />
            </div>
          </button>

          {/* 6. Speed */}
          <button
            type="button"
            onClick={() => setView('speed')}
            className="w-full flex items-center justify-between px-2.5 py-2 rounded-lg hover:bg-white/10 text-left transition-colors"
          >
            <div className="flex items-center gap-2.5">
              <Clock className="w-4 h-4 text-purple-400" />
              <span>Oynatma hızı</span>
            </div>
            <div className="flex items-center gap-1 text-[11px] text-zinc-400">
              <span>{playbackSpeed === 1 ? 'Normal' : `${playbackSpeed}x`}</span>
              <ChevronRight className="w-3.5 h-3.5 text-zinc-400" />
            </div>
          </button>

          <div className="h-px bg-white/10 my-1" />

          {/* 7. Download */}
          <button
            type="button"
            onClick={() => {
              onDownload()
              onClose()
            }}
            className="w-full flex items-center gap-2.5 px-2.5 py-2 rounded-lg hover:bg-white/10 text-left transition-colors"
          >
            <Download className="w-4 h-4 text-zinc-400" />
            <span>Videoyu indir</span>
          </button>

          {/* 8. Copy URL */}
          <button
            type="button"
            onClick={() => {
              onCopyUrl()
              onClose()
            }}
            className="w-full flex items-center gap-2.5 px-2.5 py-2 rounded-lg hover:bg-white/10 text-left transition-colors"
          >
            <Copy className="w-4 h-4 text-zinc-400" />
            <span>Video bağlantısını kopyala</span>
          </button>

          {/* 9. Copy URL at current time */}
          <button
            type="button"
            onClick={() => {
              onCopyUrlAtTime()
              onClose()
            }}
            className="w-full flex items-center gap-2.5 px-2.5 py-2 rounded-lg hover:bg-white/10 text-left transition-colors"
          >
            <Clock className="w-4 h-4 text-zinc-400" />
            <span>Geçerli süredeki bağlantıyı kopyala</span>
          </button>

          <div className="h-px bg-white/10 my-1" />

          {/* 10. Fullscreen */}
          <button
            type="button"
            onClick={() => {
              onToggleFullscreen()
              onClose()
            }}
            className="w-full flex items-center gap-2.5 px-2.5 py-2 rounded-lg hover:bg-white/10 text-left transition-colors"
          >
            {isFullscreen ? (
              <>
                <Minimize className="w-4 h-4 text-zinc-400" />
                <span>Tam ekrandan çık</span>
              </>
            ) : (
              <>
                <Maximize className="w-4 h-4 text-zinc-400" />
                <span>Tam ekran</span>
              </>
            )}
          </button>
        </div>
      )}

      {/* View: Quality */}
      {view === 'quality' && (
        <div className="space-y-0.5">
          <button
            type="button"
            onClick={() => setView('main')}
            className="w-full flex items-center gap-2 px-2.5 py-2 border-b border-white/10 font-medium text-zinc-300 hover:text-white transition-colors"
          >
            <ChevronLeft className="w-4 h-4" />
            <span>Kalite Seçimi</span>
          </button>
          <div className="max-h-56 overflow-y-auto pt-1 space-y-0.5">
            <button
              type="button"
              onClick={() => {
                onSelectQuality(null)
                onClose()
              }}
              className="w-full flex items-center justify-between px-2.5 py-1.5 rounded-lg hover:bg-white/10 text-left transition-colors"
            >
              <span>Otomatik (Orijinal)</span>
              {selectedQuality === null && <Check className="w-3.5 h-3.5 text-blue-400" />}
            </button>
            {qualities.map((item) => (
              <button
                key={item.height}
                type="button"
                onClick={() => {
                  onSelectQuality(item.isOriginal ? null : item.height)
                  onClose()
                }}
                className="w-full flex items-center justify-between px-2.5 py-1.5 rounded-lg hover:bg-white/10 text-left transition-colors"
              >
                <span>{item.label}</span>
                {selectedQuality === item.height && <Check className="w-3.5 h-3.5 text-blue-400" />}
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
            className="w-full flex items-center gap-2 px-2.5 py-2 border-b border-white/10 font-medium text-zinc-300 hover:text-white transition-colors"
          >
            <ChevronLeft className="w-4 h-4" />
            <span>Kare Hızı (FPS)</span>
          </button>
          <div className="max-h-56 overflow-y-auto pt-1 space-y-0.5">
            <button
              type="button"
              onClick={() => {
                onSelectFps(null)
                onClose()
              }}
              className="w-full flex items-center justify-between px-2.5 py-1.5 rounded-lg hover:bg-white/10 text-left transition-colors"
            >
              <span>Orijinal FPS</span>
              {selectedFps === null && <Check className="w-3.5 h-3.5 text-blue-400" />}
            </button>
            {fpsList.map((item) => (
              <button
                key={item.fps}
                type="button"
                onClick={() => {
                  onSelectFps(item.isOriginal ? null : item.fps)
                  onClose()
                }}
                className="w-full flex items-center justify-between px-2.5 py-1.5 rounded-lg hover:bg-white/10 text-left transition-colors"
              >
                <span>{item.label}</span>
                {selectedFps === item.fps && <Check className="w-3.5 h-3.5 text-blue-400" />}
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
            className="w-full flex items-center gap-2 px-2.5 py-2 border-b border-white/10 font-medium text-zinc-300 hover:text-white transition-colors"
          >
            <ChevronLeft className="w-4 h-4" />
            <span>Oynatma Hızı</span>
          </button>
          <div className="max-h-56 overflow-y-auto pt-1 space-y-0.5">
            {SPEED_OPTIONS.map((rate) => (
              <button
                key={rate}
                type="button"
                onClick={() => {
                  onSelectSpeed(rate)
                  onClose()
                }}
                className="w-full flex items-center justify-between px-2.5 py-1.5 rounded-lg hover:bg-white/10 text-left transition-colors"
              >
                <span>{rate === 1 ? 'Normal' : `${rate}x`}</span>
                {playbackSpeed === rate && <Check className="w-3.5 h-3.5 text-blue-400" />}
              </button>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
