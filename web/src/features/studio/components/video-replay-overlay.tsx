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

import { RotateCcw } from 'lucide-react'

export interface VideoReplayOverlayProps {
  onReplay: () => void
}

export function VideoReplayOverlay({ onReplay }: VideoReplayOverlayProps) {
  return (
    <div
      onClick={(e) => {
        e.stopPropagation()
        onReplay()
      }}
      className="absolute inset-0 z-40 flex items-center justify-center bg-black/45 backdrop-blur-[2px] cursor-pointer animate-in fade-in duration-200"
    >
      <button
        type="button"
        onClick={(e) => {
          e.stopPropagation()
          onReplay()
        }}
        className="group relative flex items-center justify-center w-16 h-16 md:w-20 md:h-20 rounded-full bg-zinc-900/90 text-white border border-white/25 shadow-2xl hover:scale-110 active:scale-95 hover:bg-zinc-800 transition-all duration-200"
        title="Yeniden Başlat"
      >
        <RotateCcw className="w-8 h-8 md:w-10 md:h-10 text-white transition-transform group-hover:-rotate-45 duration-300" />
      </button>
    </div>
  )
}
