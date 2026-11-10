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
import { ExternalLink, X } from 'lucide-react'
import { memo, useEffect, useMemo, useRef, useState } from 'react'

import { useSystemConfig } from '@/hooks/use-system-config'

export interface AnnouncementBarItem {
  id: string
  enabled: boolean
  text: string
  bg_color: string
  text_color: string
  height?: number
  button_enabled?: boolean
  button_text?: string
  button_url?: string
  button_bg_color?: string
  button_text_color?: string
  closable?: boolean
}

interface SingleBarProps {
  bar: AnnouncementBarItem
  onClose: (id: string) => void
}

const SingleBar = memo(function SingleBar({ bar, onClose }: SingleBarProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const textRef = useRef<HTMLSpanElement>(null)
  const [isOverflowing, setIsOverflowing] = useState(false)

  useEffect(() => {
    const checkOverflow = () => {
      if (containerRef.current && textRef.current) {
        setIsOverflowing(textRef.current.scrollWidth > containerRef.current.clientWidth)
      }
    }

    checkOverflow()
    window.addEventListener('resize', checkOverflow)
    return () => window.removeEventListener('resize', checkOverflow)
  }, [bar.text])

  const barHeight = Math.max(28, Math.min(bar.height || 38, 80))
  const bgColor = bar.bg_color || '#8b5cf6'
  const textColor = bar.text_color || '#ffffff'
  const btnBg = bar.button_bg_color || '#000000'
  const btnText = bar.button_text_color || '#ffffff'
  const hasButton = Boolean(bar.button_enabled && bar.button_text?.trim() && bar.button_url?.trim())
  const isClosable = bar.closable !== false

  const handleButtonClick = (e: React.MouseEvent) => {
    e.stopPropagation()
    if (!bar.button_url) return
    if (bar.button_url.startsWith('http://') || bar.button_url.startsWith('https://')) {
      window.open(bar.button_url, '_blank', 'noopener,noreferrer')
    } else {
      window.location.href = bar.button_url
    }
  }

  return (
    <div
      style={{
        backgroundColor: bgColor,
        color: textColor,
        minHeight: `${barHeight}px`,
        height: `${barHeight}px`,
      }}
      className='relative w-full z-40 flex items-center justify-between px-3 text-xs sm:text-sm font-medium transition-colors select-none shadow-sm'
    >
      {/* Center content / Marquee area */}
      <div
        ref={containerRef}
        className='flex-1 min-w-0 h-full overflow-hidden flex items-center justify-center relative'
      >
        {isOverflowing ? (
          <div className='w-full overflow-hidden group'>
            <div
              className='inline-flex items-center whitespace-nowrap animate-marquee hover:[animation-play-state:paused]'
              style={{
                animationDuration: `${Math.max(12, Math.round(bar.text.length * 0.25))}s`,
              }}
            >
              <span ref={textRef} className='pr-12 inline-flex items-center gap-2'>
                {bar.text}
              </span>
              <span className='pr-12 inline-flex items-center gap-2' aria-hidden='true'>
                {bar.text}
              </span>
            </div>
          </div>
        ) : (
          <span ref={textRef} className='text-center truncate px-2'>
            {bar.text}
          </span>
        )}
      </div>

      {/* Action button & Close button */}
      <div className='flex items-center gap-2 shrink-0 z-10 pl-2'>
        {hasButton && (
          <button
            type='button'
            onClick={handleButtonClick}
            style={{
              backgroundColor: btnBg,
              color: btnText,
            }}
            className='inline-flex items-center justify-center gap-1.5 rounded-full px-3 sm:px-4 py-1 text-xs font-semibold shadow-sm transition-transform active:scale-95 hover:opacity-90 shrink-0 cursor-pointer'
          >
            <span>{bar.button_text}</span>
            {bar.button_url?.startsWith('http') && <ExternalLink className='h-3 w-3 opacity-80' />}
          </button>
        )}

        {isClosable && (
          <button
            type='button'
            onClick={() => onClose(bar.id)}
            aria-label='Dismiss announcement'
            className='p-1 rounded-full opacity-70 hover:opacity-100 hover:bg-black/10 dark:hover:bg-white/10 transition-opacity'
          >
            <X className='h-3.5 w-3.5' />
          </button>
        )}
      </div>
    </div>
  )
})

export function AnnouncementBarsBanner() {
  const { announcementBars } = useSystemConfig()
  const [dismissedIds, setDismissedIds] = useState<string[]>(() => {
    try {
      if (typeof window !== 'undefined') {
        const stored = sessionStorage.getItem('dismissed_announcement_bars')
        return stored ? JSON.parse(stored) : []
      }
    } catch {
      /* ignore */
    }
    return []
  })

  const bars: AnnouncementBarItem[] = useMemo(() => {
    if (!announcementBars) return []
    try {
      const parsed = JSON.parse(announcementBars)
      if (Array.isArray(parsed)) {
        return parsed.filter((b) => b.enabled !== false && Boolean(b.text?.trim()))
      }
    } catch {
      /* ignore */
    }
    return []
  }, [announcementBars])

  const activeBars = useMemo(() => {
    return bars.filter((b) => !dismissedIds.includes(b.id))
  }, [bars, dismissedIds])

  const handleClose = (id: string) => {
    setDismissedIds((prev) => {
      const next = [...prev, id]
      try {
        sessionStorage.setItem('dismissed_announcement_bars', JSON.stringify(next))
      } catch {
        /* ignore */
      }
      return next
    })
  }

  if (activeBars.length === 0) return null

  return (
    <aside aria-label='Site Announcements' className='w-full shrink-0 flex flex-col'>
      {activeBars.map((bar) => (
        <SingleBar key={bar.id} bar={bar} onClose={handleClose} />
      ))}
    </aside>
  )
}
