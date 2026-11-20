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

import { useState, useRef, useEffect } from 'react'
import {
  Settings,
  ChevronRight,
  ChevronLeft,
  Check,
} from 'lucide-react'
import type { QualityItem, FpsItem } from './video-context-menu'

export interface VideoSettingsPopoverProps {
  isLooping: boolean
  showStats: boolean
  playbackSpeed: number
  qualities: QualityItem[]
  selectedQuality: number | null
  fpsList: FpsItem[]
  selectedFps: number | null
  onToggleLoop: () => void
  onToggleStats: () => void
  onSelectQuality: (h: number | null) => void
  onSelectFps: (f: number | null) => void
  onSelectSpeed: (s: number) => void
}

const SPEED_OPTIONS = [0.25, 0.5, 0.75, 1, 1.25, 1.5, 1.75, 2]

export function VideoSettingsPopover({
  isLooping,
  showStats,
  playbackSpeed,
  qualities,
  selectedQuality,
  fpsList,
  selectedFps,
  onToggleLoop,
  onToggleStats,
  onSelectQuality,
  onSelectFps,
  onSelectSpeed,
}: VideoSettingsPopoverProps) {
  const [isOpen, setIsOpen] = useState(false)
  const [view, setView] = useState<'main' | 'quality' | 'fps' | 'speed'>('main')
  const rootRef = useRef<HTMLDivElement>(null)

  // Close on outside click
  useEffect(() => {
    if (!isOpen) return
    const handleClickOutside = (e: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) {
        setIsOpen(false)
        setView('main')
      }
    }
    document.addEventListener('pointerdown', handleClickOutside)
    return () => document.removeEventListener('pointerdown', handleClickOutside)
  }, [isOpen])

  // Clean labels without parentheses
  const currentQualityLabel =
    selectedQuality === null
      ? 'Otomatik'
      : `${selectedQuality}p`

  const currentFpsLabel =
    selectedFps === null
      ? 'Orijinal'
      : `${selectedFps} FPS`

  return (
    <div ref={rootRef} className="relative inline-flex items-center">
      {/* Gear Button */}
      <button
        type="button"
        onClick={() => {
          setIsOpen((prev) => !prev)
          setView('main')
        }}
        className={`w-7 h-7 md:w-8 md:h-8 rounded-full flex items-center justify-center text-white/80 hover:text-white hover:bg-white/10 transition-colors ${
          isOpen ? 'bg-white/15 text-white' : ''
        }`}
        title="Ayarlar"
        aria-label="Ayarlar"
      >
        <Settings className={`w-4 h-4 md:w-4.5 md:h-4.5 transition-transform duration-300 ${isOpen ? 'rotate-45' : ''}`} />
      </button>

      {/* Popover: match control bar translucency, compact, no left icons */}
      {isOpen && (
        <div
          onClick={(e) => e.stopPropagation()}
          className="absolute bottom-full right-0 mb-2 w-46 rounded-2xl bg-zinc-900/75 backdrop-blur-xl border border-white/10 p-1 shadow-2xl text-zinc-100 text-[11px] sm:text-xs select-none z-50 max-h-[calc(100vh-60px)] overflow-y-auto animate-in fade-in zoom-in-95 duration-150"
        >
          {view === 'main' && (
            <div className="space-y-0.5">
              {/* Kalite */}
              <button
                type="button"
                onClick={() => setView('quality')}
                className="w-full flex items-center justify-between px-2 py-1.5 rounded-xl hover:bg-white/10 text-left transition-colors"
              >
                <span>Kalite</span>
                <div className="flex items-center gap-1 text-[11px] text-zinc-400">
                  <span className="truncate max-w-[75px]">{currentQualityLabel}</span>
                  <ChevronRight className="w-3.5 h-3.5 text-zinc-400" />
                </div>
              </button>

              {/* Kare Hızı (FPS) */}
              <button
                type="button"
                onClick={() => setView('fps')}
                className="w-full flex items-center justify-between px-2 py-1.5 rounded-xl hover:bg-white/10 text-left transition-colors"
              >
                <span>Kare Hızı</span>
                <div className="flex items-center gap-1 text-[11px] text-zinc-400">
                  <span className="truncate max-w-[65px]">{currentFpsLabel}</span>
                  <ChevronRight className="w-3.5 h-3.5 text-zinc-400" />
                </div>
              </button>

              {/* Hız */}
              <button
                type="button"
                onClick={() => setView('speed')}
                className="w-full flex items-center justify-between px-2 py-1.5 rounded-xl hover:bg-white/10 text-left transition-colors"
              >
                <span>Oynatma hızı</span>
                <div className="flex items-center gap-1 text-[11px] text-zinc-400">
                  <span>{playbackSpeed === 1 ? 'Normal' : `${playbackSpeed}x`}</span>
                  <ChevronRight className="w-3.5 h-3.5 text-zinc-400" />
                </div>
              </button>

              <div className="h-px bg-white/10 my-0.5" />

              {/* Döngü Toggle */}
              <div
                onClick={onToggleLoop}
                className="w-full flex items-center justify-between px-2 py-1.5 rounded-xl hover:bg-white/10 cursor-pointer transition-colors"
              >
                <span>Döngü</span>
                <div
                  className={`w-7 h-4 rounded-full p-0.5 transition-colors flex items-center ${
                    isLooping ? 'bg-emerald-500 justify-end' : 'bg-zinc-700/80 justify-start'
                  }`}
                >
                  <div className="w-3 h-3 rounded-full bg-white shadow-xs" />
                </div>
              </div>

              {/* Geliştirici Bilgisi Toggle */}
              <div
                onClick={onToggleStats}
                className="w-full flex items-center justify-between px-2 py-1.5 rounded-xl hover:bg-white/10 cursor-pointer transition-colors"
              >
                <span>Geliştirici bilgisi</span>
                <div
                  className={`w-7 h-4 rounded-full p-0.5 transition-colors flex items-center ${
                    showStats ? 'bg-amber-500 justify-end' : 'bg-zinc-700/80 justify-start'
                  }`}
                >
                  <div className="w-3 h-3 rounded-full bg-white shadow-xs" />
                </div>
              </div>
            </div>
          )}

          {/* Submenu: Quality */}
          {view === 'quality' && (
            <div className="space-y-0.5">
              <button
                type="button"
                onClick={() => setView('main')}
                className="w-full flex items-center gap-1.5 px-2 py-1.5 border-b border-white/10 font-medium text-zinc-200 hover:text-white transition-colors"
              >
                <ChevronLeft className="w-3.5 h-3.5" />
                <span>Kalite</span>
              </button>
              <div className="max-h-48 overflow-y-auto pt-0.5 space-y-0.5">
                <button
                  type="button"
                  onClick={() => {
                    onSelectQuality(null)
                    setView('main')
                  }}
                  className="w-full flex items-center justify-between px-2 py-1 rounded-lg hover:bg-white/10 text-left transition-colors"
                >
                  <span>Otomatik</span>
                  {selectedQuality === null && <Check className="w-3.5 h-3.5 text-blue-400" />}
                </button>
                {qualities.map((item) => (
                  <button
                    key={item.height}
                    type="button"
                    onClick={() => {
                      onSelectQuality(item.isOriginal ? null : item.height)
                      setView('main')
                    }}
                    className="w-full flex items-center justify-between px-2 py-1 rounded-lg hover:bg-white/10 text-left transition-colors"
                  >
                    <span>{item.label}</span>
                    {selectedQuality === item.height && <Check className="w-3.5 h-3.5 text-blue-400" />}
                  </button>
                ))}
              </div>
            </div>
          )}

          {/* Submenu: FPS */}
          {view === 'fps' && (
            <div className="space-y-0.5">
              <button
                type="button"
                onClick={() => setView('main')}
                className="w-full flex items-center gap-1.5 px-2 py-1.5 border-b border-white/10 font-medium text-zinc-200 hover:text-white transition-colors"
              >
                <ChevronLeft className="w-3.5 h-3.5" />
                <span>Kare Hızı</span>
              </button>
              <div className="max-h-48 overflow-y-auto pt-0.5 space-y-0.5">
                <button
                  type="button"
                  onClick={() => {
                    onSelectFps(null)
                    setView('main')
                  }}
                  className="w-full flex items-center justify-between px-2 py-1 rounded-lg hover:bg-white/10 text-left transition-colors"
                >
                  <span>Orijinal</span>
                  {selectedFps === null && <Check className="w-3.5 h-3.5 text-blue-400" />}
                </button>
                {fpsList.map((item) => (
                  <button
                    key={item.fps}
                    type="button"
                    onClick={() => {
                      onSelectFps(item.isOriginal ? null : item.fps)
                      setView('main')
                    }}
                    className="w-full flex items-center justify-between px-2 py-1 rounded-lg hover:bg-white/10 text-left transition-colors"
                  >
                    <span>{item.label}</span>
                    {selectedFps === item.fps && <Check className="w-3.5 h-3.5 text-blue-400" />}
                  </button>
                ))}
              </div>
            </div>
          )}

          {/* Submenu: Speed */}
          {view === 'speed' && (
            <div className="space-y-0.5">
              <button
                type="button"
                onClick={() => setView('main')}
                className="w-full flex items-center gap-1.5 px-2 py-1.5 border-b border-white/10 font-medium text-zinc-200 hover:text-white transition-colors"
              >
                <ChevronLeft className="w-3.5 h-3.5" />
                <span>Oynatma hızı</span>
              </button>
              <div className="max-h-48 overflow-y-auto pt-0.5 space-y-0.5">
                {SPEED_OPTIONS.map((rate) => (
                  <button
                    key={rate}
                    type="button"
                    onClick={() => {
                      onSelectSpeed(rate)
                      setView('main')
                    }}
                    className="w-full flex items-center justify-between px-2 py-1 rounded-lg hover:bg-white/10 text-left transition-colors"
                  >
                    <span>{rate === 1 ? 'Normal' : `${rate}x`}</span>
                    {playbackSpeed === rate && <Check className="w-3.5 h-3.5 text-blue-400" />}
                  </button>
                ))}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
