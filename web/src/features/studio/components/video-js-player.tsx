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
 * Studio video player built on video.js.
 *
 * Renditions: the studio never stores a rendition other than the generated
 * original (encoding is out of scope for this build), so the quality menu
 * carries exactly one level — the original. It is wired through a real
 * MenuButton so the menu can grow renditions later without UI changes.
 */
import { useEffect, useRef } from 'react'
import videojs from 'video.js'

import 'video.js/dist/video-js.css'

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
  /** Label of the original quality, e.g. "720p" — shown in the quality menu. */
  qualityLabel?: string
  onError?: () => void
}

const MenuButton: any = videojs.getComponent('MenuButton')
const MenuItem: any = videojs.getComponent('MenuItem')

class QualityMenuButton extends MenuButton {
  constructor(player: any, options?: any) {
    super(player, options)
    ;(this as any).controlText((options?.label as string) ?? 'Quality')
  }

  buildCSSClass(): string {
    return `vjs-icon-cog vjs-quality-button ${super.buildCSSClass()}`
  }

  createItems() {
    const label = ((this as any).options_?.label as string) ?? 'Original'
    return [
      new MenuItem((this as any).player_, {
        label,
        selectable: true,
        selected: true,
      }),
    ]
  }
}

videojs.registerComponent('QualityMenuButton', QualityMenuButton as any)

type VjsPlayer = ReturnType<typeof videojs>

export function VideoJsPlayer(props: VideoJsPlayerProps) {
  const videoRef = useRef<HTMLVideoElement | null>(null)
  const playerRef = useRef<VjsPlayer | null>(null)
  const onErrorRef = useRef(props.onError)
  onErrorRef.current = props.onError

  useEffect(() => {
    const element = videoRef.current
    if (!element) return

    const label = props.qualityLabel?.trim() || 'Original'
    const player = videojs(element, {
      autoplay: props.autoPlay ?? false,
      controls: props.controls ?? true,
      loop: props.loop ?? false,
      muted: props.muted ?? false,
      preload: props.preload ?? 'auto',
      playsInline: true,
      fluid: false,
      fill: false,
      responsive: false,
      sources: [{ src: props.src, type: props.type ?? 'video/mp4' }],
    })
    playerRef.current = player

    // Attach the quality menu button (one level — the original; more will
    // appear here the day a second real rendition exists).
    try {
      const controlBar = (player as any).getChild('ControlBar')
      if (controlBar) {
        controlBar.addChild('QualityMenuButton', { label })
      }
    } catch {
      // player works without the quality menu
    }

    player.on('error', () => {
      onErrorRef.current?.()
    })

    return () => {
      if (playerRef.current) {
        playerRef.current.dispose()
        playerRef.current = null
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    const player = playerRef.current as any
    if (!player) return
    // Src swap (fallback renditions appear/disappear or a new task completes).
    player.src({ src: props.src, type: props.type ?? 'video/mp4' })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [props.src])

  return (
    <video
      ref={videoRef}
      className={props.className}
      playsInline
    />
  )
}
