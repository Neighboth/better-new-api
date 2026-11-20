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
 * Studio video player built on Video.js v10 default theme (@videojs/react).
 */
import '@videojs/react/video/skin.css'
import { VideoPlayer, VideoSkin, Video } from '@videojs/react/video'

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
  qualityLabel?: string
  onError?: () => void
}

export function VideoJsPlayer({
  src,
  autoPlay = false,
  loop = false,
  muted = false,
  controls = true,
  playsInline = true,
  preload = 'auto',
  className,
  onError,
}: VideoJsPlayerProps) {
  return (
    <VideoPlayer>
      <VideoSkin className={className}>
        <Video
          src={src}
          autoPlay={autoPlay}
          loop={loop}
          muted={muted}
          controls={controls}
          playsInline={playsInline}
          preload={preload}
          onError={onError}
        />
      </VideoSkin>
    </VideoPlayer>
  )
}
