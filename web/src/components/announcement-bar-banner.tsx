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
import React, { memo, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { useSystemConfig } from '@/hooks/use-system-config'
import { pickLocalizedValue } from '@/lib/content-language'

export interface AnnouncementBarItem {
  id: string
  enabled: boolean
  text: string
  translations?: Record<string, string>
  bg_color: string
  text_color: string
  height?: number
  font_size?: string
  font_weight?: string
  font_family?: string
  button_enabled?: boolean
  button_text?: string
  button_translations?: Record<string, string>
  button_url?: string
  button_bg_color?: string
  button_text_color?: string
  closable?: boolean
}

interface SingleBarProps {
  bar: AnnouncementBarItem
  onClose: (id: string) => void
}

// Number of mounted banner hosts; the layout variable is only removed after the
// last host unmounts.
let announcementBarInstances = 0

/**
 * Parses simple **bold** / *italic* / _italic_ / ~~strike~~ markdown tags into
 * React elements. Shared with the admin live preview so that what the operator
 * sees in System Settings is exactly what the site renders.
 */
export function renderAnnouncementContent(rawText: string): React.ReactNode {
  if (!rawText) return null
  // Match **bold** or *italic* / _italic_
  const parts: React.ReactNode[] = []
  const regex = /(\*\*[^*]+\*\*|~~[^~]+~~|_[^_]+_|\*[^*]+\*)/g
  let lastIdx = 0
  let match: RegExpExecArray | null

  while ((match = regex.exec(rawText)) !== null) {
    if (match.index > lastIdx) {
      parts.push(rawText.substring(lastIdx, match.index))
    }
    const token = match[0]
    if (token.startsWith('**') && token.endsWith('**')) {
      parts.push(
        <strong key={match.index} className='font-bold'>
          {token.slice(2, -2)}
        </strong>
      )
    } else if (token.startsWith('~~') && token.endsWith('~~')) {
      parts.push(
        <span key={match.index} className='line-through'>
          {token.slice(2, -2)}
        </span>
      )
    } else if (token.startsWith('_') && token.endsWith('_')) {
      parts.push(
        <em key={match.index} className='italic'>
          {token.slice(1, -1)}
        </em>
      )
    } else if (token.startsWith('*') && token.endsWith('*')) {
      parts.push(
        <em key={match.index} className='italic'>
          {token.slice(1, -1)}
        </em>
      )
    } else {
      parts.push(token)
    }
    lastIdx = regex.lastIndex
  }
  if (lastIdx < rawText.length) {
    parts.push(rawText.substring(lastIdx))
  }
  return parts.length > 0 ? parts : rawText
}

const SingleBar = memo(function SingleBar({ bar, onClose }: SingleBarProps) {
  const { i18n } = useTranslation()
  const containerRef = useRef<HTMLDivElement>(null)
  const textRef = useRef<HTMLSpanElement>(null)
  const [isOverflowing, setIsOverflowing] = useState(false)

  const activeText = useMemo(() => {
    const localized = pickLocalizedValue(bar.translations, i18n.language)
    return localized || bar.text || ''
  }, [bar.translations, bar.text, i18n.language])

  const activeButtonText = useMemo(() => {
    const localized = pickLocalizedValue(bar.button_translations, i18n.language)
    return localized || bar.button_text || ''
  }, [bar.button_translations, bar.button_text, i18n.language])

  useEffect(() => {
    const checkOverflow = () => {
      if (containerRef.current && textRef.current) {
        setIsOverflowing(textRef.current.scrollWidth > containerRef.current.clientWidth)
      }
    }

    checkOverflow()
    const timer = setTimeout(checkOverflow, 50)
    window.addEventListener('resize', checkOverflow)
    return () => {
      clearTimeout(timer)
      window.removeEventListener('resize', checkOverflow)
    }
  }, [activeText])

  const barHeight = Math.max(28, Math.min(bar.height || 38, 80))
  const bgColor = bar.bg_color || '#8b5cf6'
  const textColor = bar.text_color || '#ffffff'
  const btnBg = bar.button_bg_color || '#000000'
  const btnText = bar.button_text_color || '#ffffff'
  const hasButton = Boolean(bar.button_enabled && activeButtonText?.trim() && bar.button_url?.trim())
  const isClosable = bar.closable !== false

  const fontSize = bar.font_size || '13px'
  const fontWeight = bar.font_weight || '500'
  const fontFamily = bar.font_family || 'inherit'

  const handleButtonClick = (e: React.MouseEvent) => {
    e.stopPropagation()
    if (!bar.button_url) return
    if (bar.button_url.startsWith('http://') || bar.button_url.startsWith('https://')) {
      window.open(bar.button_url, '_blank', 'noopener,noreferrer')
    } else {
      window.location.href = bar.button_url
    }
  }

  const formattedContent = useMemo(() => renderAnnouncementContent(activeText), [activeText])

  return (
    <div
      style={{
        backgroundColor: bgColor,
        color: textColor,
        minHeight: `${barHeight}px`,
        height: `${barHeight}px`,
        fontSize,
        fontWeight,
        fontFamily,
      }}
      className='relative w-full z-40 flex items-center justify-between px-3 leading-normal transition-colors select-none shadow-sm'
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
                animationDuration: `${Math.max(12, Math.round(activeText.length * 0.25))}s`,
              }}
            >
              <span ref={textRef} className='pr-12 inline-flex items-center gap-2'>
                {formattedContent}
              </span>
              <span className='pr-12 inline-flex items-center gap-2' aria-hidden='true'>
                {formattedContent}
              </span>
            </div>
          </div>
        ) : (
          <span ref={textRef} className='text-center truncate px-2'>
            {formattedContent}
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
            <span>{activeButtonText}</span>
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
        return parsed.filter((b) => b.enabled !== false && Boolean(b.text?.trim() || Object.keys(b.translations || {}).length > 0))
      }
    } catch {
      /* ignore */
    }
    return []
  }, [announcementBars])

  const activeBars = useMemo(() => {
    return bars.filter((b) => !dismissedIds.includes(b.id))
  }, [bars, dismissedIds])

  // Publish the exact rendered height of the stacked bars as a CSS variable.
  // The fixed header and every page (both the container pages and the
  // full-bleed ones such as the home hero) offset themselves with it, so the
  // content can never slide underneath the bars.
  // useLayoutEffect runs before the browser paints, which prevents a frame in
  // which the bars would cover the page.
  useLayoutEffect(() => {
    if (typeof document === 'undefined') return
    announcementBarInstances += 1
    return () => {
      announcementBarInstances -= 1
      // Another banner (for example while the router swaps layouts) may still
      // be mounted; only the last one clears the variable.
      if (announcementBarInstances <= 0) {
        announcementBarInstances = 0
        document.documentElement.style.removeProperty('--announcement-bars-height')
      }
    }
  }, [])

  const barsHeight = useMemo(
    () =>
      activeBars.reduce(
        (sum, b) => sum + Math.max(28, Math.min(b.height || 38, 80)),
        0
      ),
    [activeBars]
  )

  useLayoutEffect(() => {
    if (typeof document === 'undefined') return
    document.documentElement.style.setProperty(
      '--announcement-bars-height',
      `${barsHeight}px`
    )
  }, [barsHeight])

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
    <aside
      aria-label='Site Announcements'
      className='fixed inset-x-0 top-0 z-[60] w-full shrink-0 flex flex-col shadow-sm'
    >
      {activeBars.map((bar) => (
        <SingleBar key={bar.id} bar={bar} onClose={handleClose} />
      ))}
    </aside>
  )
}
