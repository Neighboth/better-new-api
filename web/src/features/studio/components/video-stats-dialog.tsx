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

import { useEffect, useState } from 'react'
import { X } from 'lucide-react'

export interface VideoStatsDialogProps {
  videoEl: HTMLVideoElement | null
  src: string
  selectedQualityLabel?: string
  selectedFpsLabel?: string
  nativeWidth?: number
  nativeHeight?: number
  nativeFps?: number
  isLooping: boolean
  onClose: () => void
}

interface StatsState {
  currentRes: string
  viewportRes: string
  currentTime: string
  duration: string
  bufferSec: string
  droppedFrames: string
  playbackRate: string
  status: string
  isTranscoded: boolean
}

function formatTimeSec(seconds: number): string {
  if (isNaN(seconds) || seconds < 0) return '00:00.00'
  const m = Math.floor(seconds / 60)
  const s = Math.floor(seconds % 60)
  const ms = Math.floor((seconds % 1) * 100)
  return `${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}.${ms.toString().padStart(2, '0')}`
}

export function VideoStatsDialog({
  videoEl,
  src,
  selectedQualityLabel,
  selectedFpsLabel,
  nativeWidth,
  nativeHeight,
  nativeFps,
  isLooping,
  onClose,
}: VideoStatsDialogProps) {
  const [stats, setStats] = useState<StatsState>({
    currentRes: '--',
    viewportRes: '--',
    currentTime: '00:00.00',
    duration: '00:00.00',
    bufferSec: '0.00 s',
    droppedFrames: '0 / 0',
    playbackRate: '1.0x',
    status: 'Hazır',
    isTranscoded: false,
  })

  useEffect(() => {
    if (!videoEl) return

    const updateStats = () => {
      // Buffer health
      let bufferSec = 0
      const ct = videoEl.currentTime
      if (videoEl.buffered.length > 0) {
        for (let i = 0; i < videoEl.buffered.length; i++) {
          if (videoEl.buffered.start(i) <= ct && ct <= videoEl.buffered.end(i)) {
            bufferSec = videoEl.buffered.end(i) - ct
            break
          }
        }
      }

      // Dropped frames
      let droppedStr = '0 / 0'
      const qualityFn = (videoEl as unknown as { getVideoPlaybackQuality?: () => { droppedVideoFrames?: number; totalVideoFrames?: number } }).getVideoPlaybackQuality
      if (typeof qualityFn === 'function') {
        const q = qualityFn.call(videoEl)
        droppedStr = `${q.droppedVideoFrames ?? 0} / ${q.totalVideoFrames ?? 0}`
      }

      // Status
      let st = 'Oynatılıyor'
      if (videoEl.ended) st = 'Bitti'
      else if (videoEl.paused) st = 'Duraklatıldı'

      // Transcode check
      const isTranscoded = src.includes('height=') || src.includes('fps=')

      setStats({
        currentRes: `${videoEl.videoWidth || nativeWidth || 0}x${videoEl.videoHeight || nativeHeight || 0}`,
        viewportRes: `${videoEl.clientWidth}x${videoEl.clientHeight}`,
        currentTime: formatTimeSec(videoEl.currentTime),
        duration: formatTimeSec(videoEl.duration),
        bufferSec: `${bufferSec.toFixed(2)} s`,
        droppedFrames: droppedStr,
        playbackRate: `${videoEl.playbackRate}x`,
        status: st,
        isTranscoded,
      })
    }

    updateStats()
    const timer = setInterval(updateStats, 200)
    return () => clearInterval(timer)
  }, [videoEl, src, nativeWidth, nativeHeight])

  return (
    <div
      className="absolute top-2 left-2 z-50 min-w-[230px] max-w-[270px] rounded-lg bg-zinc-900/85 backdrop-blur-xl p-2.5 text-[11px] font-mono text-zinc-100 shadow-2xl border border-white/10 select-none pointer-events-auto dark-custom-scrollbar studio-stats-box"
      onClick={(e) => e.stopPropagation()}
    >
      <div className="flex items-center justify-between border-b border-white/10 pb-1 mb-1.5">
        <span className="text-zinc-200 font-semibold tracking-wide uppercase text-[10px]">
          Geliştirici Bilgisi
        </span>
        <button
          type="button"
          onClick={onClose}
          className="text-zinc-400 hover:text-white transition-colors p-0.5 rounded-md hover:bg-white/10"
          title="Kapat"
        >
          <X className="w-3.5 h-3.5" />
        </button>
      </div>

      <div className="space-y-1">
        <div className="flex justify-between">
          <span className="text-zinc-400">Çözünürlük:</span>
          <span className="text-zinc-100 font-medium">
            {stats.currentRes} {selectedQualityLabel ? `[${selectedQualityLabel}]` : ''}
          </span>
        </div>
        <div className="flex justify-between">
          <span className="text-zinc-400">Görünüm:</span>
          <span className="text-zinc-100">{stats.viewportRes}</span>
        </div>
        <div className="flex justify-between">
          <span className="text-zinc-400">Kare Hızı:</span>
          <span className="text-zinc-100 font-medium">
            {selectedFpsLabel || (nativeFps ? `${nativeFps} FPS` : '30 FPS')}
          </span>
        </div>
        <div className="flex justify-between">
          <span className="text-zinc-400">Zaman / Süre:</span>
          <span className="text-zinc-100">
            {stats.currentTime} / {stats.duration}
          </span>
        </div>
        <div className="flex justify-between">
          <span className="text-zinc-400">Tampon Bellek:</span>
          <span className="text-emerald-400">{stats.bufferSec}</span>
        </div>
        <div className="flex justify-between">
          <span className="text-zinc-400">Bırakılan Kare:</span>
          <span className="text-zinc-100">{stats.droppedFrames}</span>
        </div>
        <div className="flex justify-between">
          <span className="text-zinc-400">Oynatma Hızı:</span>
          <span className="text-zinc-100">{stats.playbackRate}</span>
        </div>
        <div className="flex justify-between">
          <span className="text-zinc-400">Döngü Modu:</span>
          <span className={isLooping ? 'text-emerald-400 font-medium' : 'text-zinc-400'}>
            {isLooping ? 'Açık' : 'Kapalı'}
          </span>
        </div>
        <div className="flex justify-between">
          <span className="text-zinc-400">Oynatıcı Durumu:</span>
          <span className="text-zinc-100">{stats.status}</span>
        </div>
        <div className="flex justify-between pt-1 border-t border-white/10">
          <span className="text-zinc-400">Yayın Tipi:</span>
          <span className={stats.isTranscoded ? 'text-amber-400' : 'text-blue-400'}>
            {stats.isTranscoded ? 'Transcode' : 'Orijinal'}
          </span>
        </div>
      </div>
    </div>
  )
}
