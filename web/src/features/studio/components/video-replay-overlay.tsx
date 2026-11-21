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

import { useTranslation } from 'react-i18next'
import { RotateCcw } from 'lucide-react'

export interface VideoReplayOverlayProps {
  onReplay: () => void
}

export function VideoReplayOverlay({ onReplay }: VideoReplayOverlayProps) {
  const { t } = useTranslation()
  return (
    <div
      onClick={(e) => {
        e.stopPropagation()
        onReplay()
      }}
      className="absolute inset-0 z-20 flex items-center justify-center bg-black/20 backdrop-blur-[1px] rounded-inherit overflow-hidden cursor-pointer animate-in fade-in duration-200 studio-replay-overlay"
    >
      <button
        type="button"
        onClick={(e) => {
          e.stopPropagation()
          onReplay()
        }}
        className="group relative flex items-center justify-center w-14 h-14 md:w-16 md:h-16 rounded-full bg-zinc-900/85 hover:bg-zinc-800/95 text-white border border-white/20 shadow-2xl hover:scale-110 active:scale-95 transition-all duration-200"
        title={t('Replay')}
      >
        <RotateCcw className="w-7 h-7 md:w-8 md:h-8 text-white transition-transform group-hover:-rotate-45 duration-300" />
      </button>
    </div>
  )
}
