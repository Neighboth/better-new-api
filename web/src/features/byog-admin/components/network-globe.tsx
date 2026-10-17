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
import createGlobe from 'cobe'
import { Activity, Globe as GlobeIcon, Move, Zap } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Card } from '@/components/ui/card'
import type { ByogNode } from '@/features/byog/types'

interface NetworkGlobeProps {
  nodes: ByogNode[]
  className?: string
}

const GLOBAL_TECH_HUBS: [number, number, string][] = [
  [41.0082, 28.9784, 'Istanbul'],
  [37.7749, -122.4194, 'San Francisco'],
  [50.1109, 8.6821, 'Frankfurt'],
  [35.6762, 139.6503, 'Tokyo'],
  [51.5074, -0.1278, 'London'],
  [1.3521, 103.8198, 'Singapore'],
  [-33.8688, 151.2093, 'Sydney'],
  [37.5665, 126.978, 'Seoul'],
  [43.6532, -79.3832, 'Toronto'],
  [-23.5505, -46.6333, 'Sao Paulo'],
]

export function NetworkGlobe({ nodes, className }: NetworkGlobeProps) {
  const { t } = useTranslation()
  const canvasRef = useRef<HTMLCanvasElement | null>(null)
  const containerRef = useRef<HTMLDivElement | null>(null)

  const isDraggingRef = useRef(false)
  const lastMousePos = useRef<{ x: number; y: number }>({ x: 0, y: 0 })
  const phiRef = useRef(0)
  const thetaRef = useRef(0.2)
  const [isInteracting, setIsInteracting] = useState(false)

  const onlineNodes = nodes.filter((n) => n.status === 'online')
  const totalTps = nodes.reduce((acc, n) => acc + (n.current_tps || 0), 0)

  useEffect(() => {
    const canvas = canvasRef.current
    const container = containerRef.current
    if (!canvas || !container) return

    let width = container.clientWidth || 600
    const height = 460

    // Construct node markers
    const markers = (nodes.length > 0 ? nodes : [{ status: 'online' } as any]).map(
      (node, idx) => {
        const hub = GLOBAL_TECH_HUBS[idx % GLOBAL_TECH_HUBS.length]
        const location: [number, number] = [hub[0], hub[1]]
        const isOnline = node.status === 'online'
        const color: [number, number, number] = isOnline
          ? [0.05, 0.85, 0.95] // Glowing Cyan
          : [0.95, 0.65, 0.15] // Amber for idle/offline
        return {
          location,
          size: isOnline ? 0.08 : 0.05,
          color,
          id: `node-${idx}`,
        }
      }
    )

    // Build arcs connecting primary hubs
    const arcs = []
    if (markers.length >= 2) {
      for (let i = 0; i < Math.min(markers.length - 1, 5); i++) {
        arcs.push({
          from: markers[i].location,
          to: markers[(i + 1) % markers.length].location,
          color: [0.15, 0.5, 0.95] as [number, number, number],
          id: `arc-${i}`,
        })
      }
    }

    let globeInstance: any = null
    try {
      globeInstance = createGlobe(canvas, {
        devicePixelRatio: 2,
        width: width * 2,
        height: height * 2,
        phi: phiRef.current,
        theta: thetaRef.current,
        dark: 1,
        diffuse: 1.25,
        mapSamples: 16000,
        mapBrightness: 6,
        baseColor: [0.22, 0.28, 0.38],
        markerColor: [0.05, 0.85, 0.95],
        glowColor: [0.12, 0.3, 0.75],
        markers,
        arcs,
        scale: 1.05,
      })
    } catch {
      // Fallback if WebGL context creation fails
      return
    }

    let animId: number
    const animate = () => {
      if (!isDraggingRef.current) {
        phiRef.current += 0.003
      }
      if (globeInstance) {
        globeInstance.update({
          phi: phiRef.current,
          theta: thetaRef.current,
        })
      }
      animId = requestAnimationFrame(animate)
    }
    animId = requestAnimationFrame(animate)

    const handleResize = () => {
      if (!container || !globeInstance) return
      width = container.clientWidth
      globeInstance.update({
        width: width * 2,
        height: height * 2,
      })
    }
    window.addEventListener('resize', handleResize)

    return () => {
      cancelAnimationFrame(animId)
      window.removeEventListener('resize', handleResize)
      if (globeInstance) {
        globeInstance.destroy()
      }
    }
  }, [nodes])

  // Pointer drag controls for smooth 3D globe rotation
  const handlePointerDown = (e: React.PointerEvent) => {
    isDraggingRef.current = true
    setIsInteracting(true)
    lastMousePos.current = { x: e.clientX, y: e.clientY }
    ;(e.target as HTMLElement).setPointerCapture(e.pointerId)
  }

  const handlePointerMove = (e: React.PointerEvent) => {
    if (!isDraggingRef.current) return
    const deltaX = e.clientX - lastMousePos.current.x
    const deltaY = e.clientY - lastMousePos.current.y
    lastMousePos.current = { x: e.clientX, y: e.clientY }

    phiRef.current += deltaX * 0.007
    thetaRef.current = Math.max(
      -Math.PI / 3,
      Math.min(Math.PI / 3, thetaRef.current + deltaY * 0.007)
    )
  }

  const handlePointerUp = (e: React.PointerEvent) => {
    isDraggingRef.current = false
    setIsInteracting(false)
    try {
      ;(e.target as HTMLElement).releasePointerCapture(e.pointerId)
    } catch {
      // Ignore if pointer capture already lost
    }
  }

  return (
    <Card
      className={`border-border/80 bg-card/60 relative overflow-hidden backdrop-blur-md ${className || ''}`}
    >
      {/* Top Header Overlay */}
      <div className='pointer-events-none absolute top-4 right-4 left-4 z-10 flex flex-wrap items-center justify-between gap-3'>
        <div className='flex items-center gap-2.5 rounded-lg border border-border/80 bg-background/80 px-3 py-1.5 backdrop-blur-md shadow-sm'>
          <GlobeIcon className='text-primary h-4 w-4' />
          <div>
            <div className='text-xs font-bold tracking-tight'>
              {t('byog.admin.network_topology_globe')}
            </div>
            <div className='text-[10px] text-muted-foreground'>
              {t('byog.admin.interactive_depin_mesh')}
            </div>
          </div>
        </div>

        <div className='flex items-center gap-2'>
          <Badge
            variant='outline'
            className='gap-1.5 border-border/80 bg-background/80 px-2.5 py-1 text-xs font-medium backdrop-blur-md shadow-sm'
          >
            <Move className='h-3 w-3 text-muted-foreground' />
            {t('byog.admin.drag_to_rotate')}
          </Badge>
          <Badge
            variant='outline'
            className='gap-1.5 border-emerald-500/30 bg-emerald-500/10 px-2.5 py-1 text-xs font-medium text-emerald-600 dark:text-emerald-400 backdrop-blur-md shadow-sm'
          >
            <Activity className='h-3 w-3' />
            {onlineNodes.length} / {nodes.length} {t('byog.admin.nodes_online')}
          </Badge>
        </div>
      </div>

      {/* Interactive 3D WebGL Canvas Container */}
      <div
        ref={containerRef}
        className={`relative flex h-[460px] w-full items-center justify-center overflow-hidden transition-all ${
          isInteracting ? 'cursor-grabbing' : 'cursor-grab'
        }`}
        onPointerDown={handlePointerDown}
        onPointerMove={handlePointerMove}
        onPointerUp={handlePointerUp}
        onPointerCancel={handlePointerUp}
      >
        <canvas
          ref={canvasRef}
          style={{
            width: '100%',
            height: '460px',
            maxWidth: '100%',
            aspectRatio: 1,
            touchAction: 'none',
          }}
        />
      </div>

      {/* Bottom Telemetry Bar */}
      <div className='pointer-events-none absolute right-4 bottom-4 left-4 z-10 flex flex-wrap items-center justify-between gap-2 border-t border-border/40 pt-2 text-[11px] text-muted-foreground'>
        <div className='flex items-center gap-2'>
          <span className='inline-block h-2 w-2 rounded-full bg-emerald-500 animate-pulse' />
          <span>{t('byog.admin.p2p_mesh_live')}</span>
        </div>
        <div className='flex items-center gap-3'>
          <div className='flex items-center gap-1 font-mono'>
            <Zap className='h-3 w-3 text-amber-500' />
            <span>{totalTps.toFixed(1)} TPS</span>
          </div>
          <div>{t('byog.admin.bandwidth_optimized_tunneling')}</div>
        </div>
      </div>
    </Card>
  )
}
