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
import { useMemo } from 'react'

import { RichContent } from '@/components/rich-content'
import { VideoJsPlayer } from '@/features/studio/components/video-js-player'
import { cn } from '@/lib/utils'

type ContentBlock =
  | { type: 'markdown'; content: string }
  | { type: 'youtube'; videoId: string }
  | { type: 'video'; src: string }

const VIDEO_EXT_RE = /\.(mp4|webm|mov|m4v|mkv|ogv)(?:[?#][^\s)]*)?$/i

/**
 * Extracts a YouTube 11-character video ID from any standard YouTube URL
 * (watch?v=, youtu.be/, /embed/, /shorts/, /live/) or a bare 11-char ID.
 */
export function extractYouTubeVideoId(input: string): string | null {
  const raw = input.trim()
  if (!raw) return null
  if (/^[a-zA-Z0-9_-]{11}$/.test(raw)) {
    return raw
  }
  try {
    const parsed = new URL(raw)
    const host = parsed.hostname.replace(/^www\./, '').toLowerCase()
    if (host === 'youtu.be') {
      const seg = parsed.pathname.split('/').filter(Boolean)[0] ?? ''
      return /^[a-zA-Z0-9_-]{11}$/.test(seg) ? seg : null
    }
    if (host === 'youtube.com' || host === 'm.youtube.com' || host === 'youtube-nocookie.com') {
      const vParam = parsed.searchParams.get('v')
      if (vParam && /^[a-zA-Z0-9_-]{11}$/.test(vParam)) {
        return vParam
      }
      const parts = parsed.pathname.split('/').filter(Boolean)
      if (
        parts.length >= 2 &&
        (parts[0] === 'embed' || parts[0] === 'shorts' || parts[0] === 'live' || parts[0] === 'v') &&
        /^[a-zA-Z0-9_-]{11}$/.test(parts[1])
      ) {
        return parts[1]
      }
    }
  } catch {
    return null
  }
  return null
}

function isSafeMediaSrc(raw: string): boolean {
  const s = raw.trim()
  if (!s) return false
  return s.startsWith('https://') || s.startsWith('http://') || s.startsWith('/')
}

function matchLineEmbed(line: string): ContentBlock | null {
  const trimmed = line.trim()
  if (!trimmed) return null

  // 1. Explicit directive: ::youtube[URL_OR_ID]
  const ytDirective = trimmed.match(/^::youtube\[([^\]]+)\]\s*$/i)
  if (ytDirective) {
    const id = extractYouTubeVideoId(ytDirective[1])
    if (id) return { type: 'youtube', videoId: id }
  }

  // 2. Explicit directive: ::video[URL]
  const videoDirective = trimmed.match(/^::video\[([^\]]+)\]\s*$/i)
  if (videoDirective) {
    const src = videoDirective[1].trim()
    if (isSafeMediaSrc(src)) return { type: 'video', src }
  }

  // 3. Markdown image/embed syntax: ![youtube](URL) or ![video](URL) or ![...](*.mp4)
  const mdEmbed = trimmed.match(/^!\[([^\]]*)\]\(([^)\s]+)(?:\s+"[^"]*")?\)\s*$/)
  if (mdEmbed) {
    const alt = mdEmbed[1].trim().toLowerCase()
    const target = mdEmbed[2].trim()
    if (alt === 'youtube' || alt === 'yt') {
      const id = extractYouTubeVideoId(target)
      if (id) return { type: 'youtube', videoId: id }
    }
    if (alt === 'video' || VIDEO_EXT_RE.test(target)) {
      if (isSafeMediaSrc(target)) return { type: 'video', src: target }
    }
  }

  // 4. Raw HTML <video ... src="..."> tag on its own line
  const htmlVideo = trimmed.match(/^<video\b[^>]*\bsrc=["']([^"']+)["'][^>]*>(?:<\/video>)?\s*$/i)
  if (htmlVideo) {
    const src = htmlVideo[1].trim()
    if (isSafeMediaSrc(src)) return { type: 'video', src }
  }

  // 5. Standalone YouTube URL on its own line
  if (/^https?:\/\/(?:www\.|m\.)?(?:youtube\.com|youtu\.be|youtube-nocookie\.com)\/\S+$/i.test(trimmed)) {
    const id = extractYouTubeVideoId(trimmed)
    if (id) return { type: 'youtube', videoId: id }
  }

  // 6. Standalone direct video URL (.mp4/.webm/.mov/.m4v) on its own line
  if (/^(?:https?:\/\/|\/)\S+$/.test(trimmed) && VIDEO_EXT_RE.test(trimmed)) {
    return { type: 'video', src: trimmed }
  }

  return null
}

function parseBlogMarkdownBlocks(markdown: string): ContentBlock[] {
  const lines = markdown.split(/\r?\n/)
  const blocks: ContentBlock[] = []
  let mdBuffer: string[] = []
  let inFence = false
  let fenceMarker = ''

  const flushMarkdown = () => {
    const joined = mdBuffer.join('\n')
    if (joined.trim().length > 0) {
      blocks.push({ type: 'markdown', content: joined })
    }
    mdBuffer = []
  }

  for (const line of lines) {
    const fenceMatch = line.match(/^\s*(```+|~~~+)/)
    if (fenceMatch) {
      const marker = fenceMatch[1][0]
      if (!inFence) {
        inFence = true
        fenceMarker = marker
      } else if (marker === fenceMarker) {
        inFence = false
        fenceMarker = ''
      }
      mdBuffer.push(line)
      continue
    }

    if (!inFence) {
      const embed = matchLineEmbed(line)
      if (embed) {
        flushMarkdown()
        blocks.push(embed)
        continue
      }
    }

    mdBuffer.push(line)
  }

  flushMarkdown()
  return blocks
}

export interface BlogMarkdownProps {
  content: string
  className?: string
}

/**
 * Renders Blog Markdown with full support for:
 * - Standard Markdown (via RichContent)
 * - Official responsive YouTube embeds (`::youtube[URL_OR_ID]` or bare YouTube URL)
 * - Full Studio Video.js player (`::video[URL]`, `![video](URL)`, or direct `.mp4/.webm/.mov` URL)
 */
export function BlogMarkdown({ content, className }: BlogMarkdownProps) {
  const blocks = useMemo(() => parseBlogMarkdownBlocks(content || ''), [content])

  if (blocks.length === 0) {
    return null
  }

  return (
    <div className={cn('space-y-4', className)}>
      {blocks.map((block, idx) => {
        if (block.type === 'youtube') {
          return (
            <div
              key={`yt-${idx}-${block.videoId}`}
              className='my-4 aspect-video w-full overflow-hidden rounded-lg border bg-black shadow-sm'
            >
              <iframe
                src={`https://www.youtube.com/embed/${block.videoId}?rel=0`}
                title={`YouTube video ${block.videoId}`}
                className='h-full w-full border-0'
                allow='accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share'
                referrerPolicy='strict-origin-when-cross-origin'
                loading='lazy'
                allowFullScreen
              />
            </div>
          )
        }

        if (block.type === 'video') {
          return (
            <div
              key={`vid-${idx}-${block.src}`}
              className='my-4 w-full overflow-hidden rounded-lg border bg-black shadow-sm'
            >
              <VideoJsPlayer
                src={block.src}
                className='aspect-video w-full'
                controls
                playsInline
                preload='metadata'
              />
            </div>
          )
        }

        return (
          <RichContent
            key={`md-${idx}`}
            content={block.content}
            mode='markdown'
            className='max-w-none'
          />
        )
      })}
    </div>
  )
}
