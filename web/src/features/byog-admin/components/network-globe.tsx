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
import { useEffect, useRef } from 'react'

import type { ByogNode } from '@/features/byog/types'

interface NetworkGlobeProps {
  nodes: ByogNode[]
  className?: string
}

export function NetworkGlobe({ nodes, className }: NetworkGlobeProps) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null)

  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return
    const ctx = canvas.getContext('2d')
    if (!ctx) return

    let animationFrameId: number
    let rotation = 0

    // Resize canvas
    const width = (canvas.width = canvas.parentElement?.clientWidth || 600)
    const height = (canvas.height = 420)
    const radius = Math.min(width, height) * 0.38
    const centerX = width / 2
    const centerY = height / 2

    // Predefined city coordinates for demonstration when lat/lon not provided
    const sampleCoords = [
      { lat: 41.0082, lon: 28.9784, label: 'Istanbul' },
      { lat: 37.7749, lon: -122.4194, label: 'San Francisco' },
      { lat: 51.5074, lon: -0.1278, label: 'London' },
      { lat: 35.6762, lon: 139.6503, label: 'Tokyo' },
      { lat: 52.52, lon: 13.405, label: 'Berlin' },
      { lat: 1.3521, lon: 103.8198, label: 'Singapore' },
      { lat: -33.8688, lon: 151.2093, label: 'Sydney' },
    ]

    const render = () => {
      ctx.clearRect(0, 0, width, height)

      // 1. Draw Globe Base Atmosphere Glow
      const glowGrad = ctx.createRadialGradient(
        centerX,
        centerY,
        radius * 0.8,
        centerX,
        centerY,
        radius * 1.25
      )
      glowGrad.addColorStop(0, 'rgba(14, 165, 233, 0.15)')
      glowGrad.addColorStop(0.5, 'rgba(14, 165, 233, 0.05)')
      glowGrad.addColorStop(1, 'rgba(0, 0, 0, 0)')

      ctx.fillStyle = glowGrad
      ctx.beginPath()
      ctx.arc(centerX, centerY, radius * 1.25, 0, Math.PI * 2)
      ctx.fill()

      // 2. Draw Sphere Body
      const sphereGrad = ctx.createRadialGradient(
        centerX - radius * 0.3,
        centerY - radius * 0.3,
        radius * 0.1,
        centerX,
        centerY,
        radius
      )
      sphereGrad.addColorStop(0, 'rgba(30, 41, 59, 0.85)')
      sphereGrad.addColorStop(1, 'rgba(15, 23, 42, 0.95)')

      ctx.fillStyle = sphereGrad
      ctx.beginPath()
      ctx.arc(centerX, centerY, radius, 0, Math.PI * 2)
      ctx.fill()
      ctx.strokeStyle = 'rgba(56, 189, 248, 0.3)'
      ctx.lineWidth = 1.5
      ctx.stroke()

      // 3. Draw Rotating Latitude & Longitude Wireframe Rings
      ctx.lineWidth = 0.8
      for (let lat = -60; lat <= 60; lat += 30) {
        const phi = (lat * Math.PI) / 180
        const rLat = radius * Math.cos(phi)
        const yLat = centerY - radius * Math.sin(phi)

        ctx.strokeStyle = 'rgba(56, 189, 248, 0.12)'
        ctx.beginPath()
        ctx.ellipse(centerX, yLat, rLat, rLat * 0.25, 0, 0, Math.PI * 2)
        ctx.stroke()
      }

      // Longitude Meridians
      for (let lon = 0; lon < 360; lon += 45) {
        const theta = ((lon + rotation) * Math.PI) / 180
        const xOffset = Math.sin(theta) * radius

        if (Math.cos(theta) > 0) {
          ctx.strokeStyle = 'rgba(56, 189, 248, 0.15)'
          ctx.beginPath()
          ctx.ellipse(
            centerX,
            centerY,
            Math.abs(xOffset),
            radius,
            0,
            0,
            Math.PI * 2
          )
          ctx.stroke()
        }
      }

      // 4. Project and Draw Active Nodes on the Globe
      const projectedNodes: {
        x: number
        y: number
        visible: boolean
        label: string
        status: string
      }[] = []

      const targetList = nodes.length > 0 ? nodes : sampleCoords

      targetList.forEach((item, idx) => {
        const lat =
          'latitude' in item && item.latitude
            ? item.latitude
            : sampleCoords[idx % sampleCoords.length].lat
        const lon =
          'longitude' in item && item.longitude
            ? item.longitude
            : sampleCoords[idx % sampleCoords.length].lon
        const label = 'device_name' in item ? item.device_name : item.label
        const status = 'status' in item ? item.status : 'online'

        const phi = (lat * Math.PI) / 180
        const theta = ((lon + rotation) * Math.PI) / 180

        // 3D coordinates on unit sphere
        const x3d = Math.cos(phi) * Math.sin(theta)
        const y3d = -Math.sin(phi)
        const z3d = Math.cos(phi) * Math.cos(theta)

        const visible = z3d > 0 // Front-facing check

        const screenX = centerX + x3d * radius
        const screenY = centerY + y3d * radius

        projectedNodes.push({ x: screenX, y: screenY, visible, label, status })

        if (visible) {
          // Node glowing point
          const pointGrad = ctx.createRadialGradient(
            screenX,
            screenY,
            1,
            screenX,
            screenY,
            8
          )
          pointGrad.addColorStop(0, '#38bdf8')
          pointGrad.addColorStop(0.5, 'rgba(56, 189, 248, 0.4)')
          pointGrad.addColorStop(1, 'rgba(56, 189, 248, 0)')

          ctx.fillStyle = pointGrad
          ctx.beginPath()
          ctx.arc(screenX, screenY, 8, 0, Math.PI * 2)
          ctx.fill()

          ctx.fillStyle = status === 'gamer_mode' ? '#c084fc' : '#38bdf8'
          ctx.beginPath()
          ctx.arc(screenX, screenY, 3, 0, Math.PI * 2)
          ctx.fill()

          // Device Label
          ctx.fillStyle = 'rgba(255, 255, 255, 0.85)'
          ctx.font = '10px monospace'
          ctx.fillText(label, screenX + 8, screenY + 3)
        }
      })

      // 5. Draw P2P Data Arcs & Routing Transfer Beams between visible peers
      for (let i = 0; i < projectedNodes.length; i++) {
        for (let j = i + 1; j < projectedNodes.length; j++) {
          const n1 = projectedNodes[i]
          const n2 = projectedNodes[j]
          if (n1.visible && n2.visible) {
            ctx.strokeStyle = 'rgba(56, 189, 248, 0.25)'
            ctx.lineWidth = 1
            ctx.beginPath()
            ctx.moveTo(n1.x, n1.y)
            const midX = (n1.x + n2.x) / 2
            const midY = (n1.y + n2.y) / 2 - 25 // Arc upward
            ctx.quadraticCurveTo(midX, midY, n2.x, n2.y)
            ctx.stroke()
          }
        }
      }

      rotation += 0.35
      animationFrameId = requestAnimationFrame(render)
    }

    render()

    return () => {
      cancelAnimationFrame(animationFrameId)
    }
  }, [nodes])

  return (
    <div
      className={`border-primary/20 relative flex items-center justify-center overflow-hidden rounded-2xl border bg-slate-950/80 shadow-2xl ${className}`}
    >
      <div className='absolute top-3 left-4 z-10 flex items-center gap-2'>
        <span className='h-2.5 w-2.5 animate-ping rounded-full bg-emerald-500' />
        <span className='text-muted-foreground font-mono text-xs font-semibold tracking-wider uppercase'>
          DePIN Mesh Live Telemetry
        </span>
      </div>
      <canvas ref={canvasRef} className='max-w-full' />
    </div>
  )
}
