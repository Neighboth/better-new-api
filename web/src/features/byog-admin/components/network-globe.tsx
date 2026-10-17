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
import { geoNaturalEarth1, geoPath } from 'd3-geo'
import { useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import * as topojson from 'topojson-client'
import worldDataRaw from 'world-atlas/countries-110m.json'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import type { ByogNode } from '@/features/byog/types'

interface NetworkGlobeProps {
  nodes: ByogNode[]
  className?: string
}

interface CountryFeature {
  type: string
  id: string | number
  properties: {
    name: string
  }
  geometry: any
}

// Major fallback coordinates for known tech hubs / countries
const COUNTRY_COORDINATES: Record<string, [number, number]> = {
  turkey: [35.2433, 38.9637],
  tr: [35.2433, 38.9637],
  'united states of america': [-95.7129, 37.0902],
  usa: [-95.7129, 37.0902],
  us: [-95.7129, 37.0902],
  germany: [10.4515, 51.1657],
  de: [10.4515, 51.1657],
  france: [2.2137, 46.2276],
  fr: [2.2137, 46.2276],
  'united kingdom': [-3.436, 55.3781],
  uk: [-3.436, 55.3781],
  gb: [-3.436, 55.3781],
  japan: [138.2529, 36.2048],
  jp: [138.2529, 36.2048],
  china: [104.1954, 35.8617],
  cn: [104.1954, 35.8617],
  singapore: [103.8198, 1.3521],
  sg: [103.8198, 1.3521],
  brazil: [-51.9253, -14.235],
  br: [-51.9253, -14.235],
  australia: [133.7751, -25.2744],
  au: [133.7751, -25.2744],
  canada: [-106.3468, 56.1304],
  ca: [-106.3468, 56.1304],
  india: [78.9629, 20.5937],
  in: [78.9629, 20.5937],
  korea: [127.7669, 35.9078],
  kr: [127.7669, 35.9078],
  netherlands: [5.2913, 52.1326],
  nl: [5.2913, 52.1326],
}

// Map short display names for tiny labels
const COUNTRY_SHORT_NAMES: Record<string, string> = {
  'United States of America': 'USA',
  'United Kingdom': 'UK',
  'Russian Federation': 'Russia',
  'Democratic Republic of the Congo': 'DR Congo',
  'Central African Republic': 'CAR',
}

const BASE_WIDTH = 960
const BASE_HEIGHT = 480

export function NetworkGlobe({ nodes, className }: NetworkGlobeProps) {
  const { t } = useTranslation()
  const svgRef = useRef<SVGSVGElement | null>(null)

  // Zoom and pan state
  const [zoom, setZoom] = useState(1)
  const [pan, setPan] = useState({ x: 0, y: 0 })
  const [isDragging, setIsDragging] = useState(false)
  const dragStartRef = useRef({ x: 0, y: 0 })

  // Hover state
  const [hoveredCountry, setHoveredCountry] = useState<string | null>(null)
  const [hoveredNode, setHoveredNode] = useState<ByogNode | null>(null)
  const [tooltipPos, setTooltipPos] = useState<{ x: number; y: number } | null>(
    null
  )

  // Generate GeoJSON and projection
  const { countries, pathGenerator, projection } = useMemo(() => {
    const worldData = worldDataRaw as any
    const geo = topojson.feature(
      worldData,
      worldData.objects.countries
    ) as unknown as { features: CountryFeature[] }

    const proj = geoNaturalEarth1().fitSize(
      [BASE_WIDTH, BASE_HEIGHT],
      geo as any
    )
    const path = geoPath(proj)

    return {
      countries: geo.features,
      pathGenerator: path,
      projection: proj,
    }
  }, [])

  // Aggregate stats
  const onlineNodes = nodes.filter((n) => n.status === 'online')
  const aggregateTps = nodes.reduce((acc, n) => acc + (n.current_tps || 0), 0)

  // Pre-calculate country node counts
  const nodesByCountry = useMemo(() => {
    const map = new Map<string, ByogNode[]>()
    for (const node of nodes) {
      const c = (node.country || 'Global').toLowerCase().trim()
      const list = map.get(c) || []
      list.push(node)
      map.set(c, list)
    }
    return map
  }, [nodes])

  // Get nodes for a specific country feature
  function getNodesForCountry(countryName: string) {
    const lower = countryName.toLowerCase().trim()
    const direct = nodesByCountry.get(lower)
    if (direct && direct.length > 0) return direct

    for (const [key, list] of nodesByCountry.entries()) {
      if (lower.includes(key) || key.includes(lower)) {
        return list
      }
    }
    return []
  }

  // Pointer drag controls
  const handlePointerDown = (e: React.PointerEvent<SVGSVGElement>) => {
    if (e.button !== 0) return
    setIsDragging(true)
    dragStartRef.current = {
      x: e.clientX - pan.x,
      y: e.clientY - pan.y,
    }
    ;(e.currentTarget as Element).setPointerCapture(e.pointerId)
  }

  const handlePointerMove = (e: React.PointerEvent<SVGSVGElement>) => {
    if (isDragging) {
      setPan({
        x: e.clientX - dragStartRef.current.x,
        y: e.clientY - dragStartRef.current.y,
      })
    }

    if (svgRef.current) {
      const rect = svgRef.current.getBoundingClientRect()
      setTooltipPos({
        x: e.clientX - rect.left,
        y: e.clientY - rect.top,
      })
    }
  }

  const handlePointerUp = (e: React.PointerEvent<SVGSVGElement>) => {
    setIsDragging(false)
    try {
      ;(e.currentTarget as Element).releasePointerCapture(e.pointerId)
    } catch {
      // Ignore
    }
  }

  const handleWheel = (e: React.WheelEvent<SVGSVGElement>) => {
    e.preventDefault()
    const zoomFactor = e.deltaY < 0 ? 1.15 : 0.85
    setZoom((prev) => Math.min(8, Math.max(0.8, prev * zoomFactor)))
  }

  const handleZoomIn = () => setZoom((prev) => Math.min(8, prev * 1.3))
  const handleZoomOut = () => setZoom((prev) => Math.max(0.8, prev / 1.3))
  const handleReset = () => {
    setZoom(1)
    setPan({ x: 0, y: 0 })
  }

  // Calculate coordinates for nodes
  const nodeMarkers = useMemo(() => {
    return nodes.map((node) => {
      let lon = node.longitude
      let lat = node.latitude

      if ((!lon && !lat) || (lon === 0 && lat === 0)) {
        const cKey = (node.country || '').toLowerCase().trim()
        if (COUNTRY_COORDINATES[cKey]) {
          const coords = COUNTRY_COORDINATES[cKey]
          lon = coords[0]
          lat = coords[1]
        } else {
          lon = 0
          lat = 20
        }
      }

      const projected = projection([lon, lat])
      return {
        node,
        x: projected ? projected[0] : 0,
        y: projected ? projected[1] : 0,
      }
    })
  }, [nodes, projection])

  // Active country for tooltip
  const activeCountryNodes = hoveredCountry
    ? getNodesForCountry(hoveredCountry)
    : []

  return (
    <Card
      className={`relative overflow-hidden border border-border/80 bg-card/60 backdrop-blur-md ${className || ''}`}
    >
      {/* Top Header Overlay - Zero Icons */}
      <div className='pointer-events-none absolute top-4 right-4 left-4 z-10 flex flex-wrap items-center justify-between gap-3'>
        <div className='pointer-events-auto rounded-lg border border-border/80 bg-background/80 px-3 py-1.5 shadow-sm backdrop-blur-md'>
          <div className='text-xs font-bold tracking-tight'>
            {t('byog.admin.network_topology_globe')}
          </div>
          <div className='text-[10px] text-muted-foreground'>
            {t('byog.admin.interactive_depin_mesh')}
          </div>
        </div>

        <div className='pointer-events-auto flex items-center gap-2'>
          <div className='flex items-center gap-1 rounded-lg border border-border/80 bg-background/80 p-0.5 shadow-sm backdrop-blur-md'>
            <Button
              variant='ghost'
              size='sm'
              className='h-7 px-2 font-mono text-xs'
              onClick={handleZoomIn}
              title='Zoom in'
            >
              +
            </Button>
            <Button
              variant='ghost'
              size='sm'
              className='h-7 px-2 font-mono text-xs'
              onClick={handleZoomOut}
              title='Zoom out'
            >
              -
            </Button>
            <Button
              variant='ghost'
              size='sm'
              className='h-7 px-2 text-xs'
              onClick={handleReset}
            >
              {t('byog.reset_view')}
            </Button>
          </div>

          <Badge
            variant='outline'
            className='border-emerald-500/30 bg-emerald-500/10 px-2.5 py-1 text-xs font-medium text-emerald-600 dark:text-emerald-400 shadow-sm backdrop-blur-md'
          >
            {onlineNodes.length} / {nodes.length} {t('byog.admin.nodes_online')}
          </Badge>
        </div>
      </div>

      {/* SVG Map Container */}
      <div
        className={`relative flex h-[500px] w-full items-center justify-center overflow-hidden select-none ${
          isDragging ? 'cursor-grabbing' : 'cursor-grab'
        }`}
      >
        <svg
          ref={svgRef}
          viewBox={`0 0 ${BASE_WIDTH} ${BASE_HEIGHT}`}
          className='h-full w-full'
          onPointerDown={handlePointerDown}
          onPointerMove={handlePointerMove}
          onPointerUp={handlePointerUp}
          onPointerCancel={handlePointerUp}
          onWheel={handleWheel}
        >
          {/* Subtle grid pattern */}
          <defs>
            <pattern
              id='geo-grid'
              width='60'
              height='60'
              patternUnits='userSpaceOnUse'
            >
              <path
                d='M 60 0 L 0 0 0 60'
                fill='none'
                stroke='currentColor'
                strokeWidth='0.5'
                className='text-border/20'
              />
            </pattern>
          </defs>
          <rect
            width={BASE_WIDTH}
            height={BASE_HEIGHT}
            fill='url(#geo-grid)'
            className='pointer-events-none'
          />

          {/* Map Group with Pan and Zoom */}
          <g
            transform={`translate(${pan.x}, ${pan.y}) scale(${zoom})`}
            transform-origin='center'
            style={{ transition: isDragging ? 'none' : 'transform 0.1s ease-out' }}
          >
            {/* Country Polygons */}
            {countries.map((feature) => {
              const countryName = feature.properties.name || ''
              const countryNodes = getNodesForCountry(countryName)
              const hasNodes = countryNodes.length > 0
              const isHovered = hoveredCountry === countryName
              const pathData = pathGenerator(feature as any)
              if (!pathData) return null

              const centroid = pathGenerator.centroid(feature as any)
              const [cx, cy] = centroid
              const shortName =
                COUNTRY_SHORT_NAMES[countryName] || countryName

              return (
                <g key={feature.id || countryName}>
                  {/* Distinct Country Shape */}
                  <path
                    d={pathData}
                    className={`cursor-pointer transition-colors duration-150 ${
                      isHovered
                        ? 'fill-primary/25 stroke-primary stroke-[1.5]'
                        : hasNodes
                          ? 'fill-emerald-500/20 stroke-emerald-500/60 stroke-[1]'
                          : 'fill-muted/20 hover:fill-muted/40 stroke-border/90 stroke-[0.75] dark:fill-card/40 dark:stroke-border/70 dark:hover:fill-muted/30'
                    }`}
                    onMouseEnter={() => setHoveredCountry(countryName)}
                    onMouseLeave={() => setHoveredCountry(null)}
                  />

                  {/* Tiny Country Label on the map */}
                  {cx && cy && !isNaN(cx) && !isNaN(cy) && (
                    <text
                      x={cx}
                      y={cy}
                      textAnchor='middle'
                      dominantBaseline='central'
                      className={`pointer-events-none select-none font-mono text-[7px] transition-opacity ${
                        isHovered
                          ? 'fill-primary font-bold opacity-100 text-[8px]'
                          : hasNodes
                            ? 'fill-emerald-500 dark:fill-emerald-400 font-semibold opacity-90'
                            : 'fill-muted-foreground/60 opacity-60'
                      }`}
                    >
                      {shortName}
                    </text>
                  )}
                </g>
              )
            })}

            {/* Inter-Node Mesh Network Connecting Lines */}
            {nodeMarkers.length >= 2 && (
              <g className='pointer-events-none'>
                {nodeMarkers.map((marker, i) => {
                  if (i === 0) return null
                  const prev = nodeMarkers[i - 1]
                  if (!marker.x || !marker.y || !prev.x || !prev.y) return null
                  return (
                    <line
                      key={`mesh-line-${i}`}
                      x1={prev.x}
                      y1={prev.y}
                      x2={marker.x}
                      y2={marker.y}
                      stroke='currentColor'
                      strokeWidth='1'
                      strokeDasharray='3 3'
                      className='text-emerald-500/30'
                    />
                  )
                })}
              </g>
            )}

            {/* DePIN Node Beacons with Ping Ripples */}
            {nodeMarkers.map(({ node, x, y }) => {
              if (!x || !y) return null
              const isOnline = node.status === 'online'
              const isHovered = hoveredNode?.id === node.id

              return (
                <g
                  key={`node-${node.id}`}
                  transform={`translate(${x}, ${y})`}
                  className='cursor-pointer'
                  onMouseEnter={() => setHoveredNode(node)}
                  onMouseLeave={() => setHoveredNode(null)}
                >
                  {/* Outer Pulsing Ring */}
                  {isOnline && (
                    <circle
                      r='9'
                      className='animate-ping fill-emerald-500/40 opacity-75'
                    />
                  )}

                  {/* Halo */}
                  <circle
                    r={isHovered ? '8' : '6'}
                    className={`transition-all ${
                      isOnline
                        ? 'fill-emerald-500/20 stroke-emerald-500 stroke-[1.5]'
                        : 'fill-amber-500/20 stroke-amber-500 stroke-[1.5]'
                    }`}
                  />

                  {/* Core Center Dot */}
                  <circle
                    r={isHovered ? '4' : '3'}
                    className={
                      isOnline
                        ? 'fill-emerald-500 shadow-lg'
                        : 'fill-amber-500 shadow-lg'
                    }
                  />
                </g>
              )
            })}
          </g>
        </svg>

        {/* Hover Details Tooltip */}
        {tooltipPos && (hoveredCountry || hoveredNode) && (
          <div
            className='pointer-events-none absolute z-30 min-w-[200px] max-w-xs -translate-x-1/2 -translate-y-full rounded-lg border border-border/90 bg-popover/95 p-3 text-popover-foreground shadow-xl backdrop-blur-md'
            style={{
              left: `${Math.min(
                Math.max(tooltipPos.x, 110),
                (svgRef.current?.clientWidth || BASE_WIDTH) - 110
              )}px`,
              top: `${Math.max(tooltipPos.y - 12, 10)}px`,
            }}
          >
            {hoveredNode ? (
              <div className='space-y-1.5'>
                <div className='flex items-center justify-between gap-2 border-b border-border/50 pb-1'>
                  <span className='font-semibold text-xs'>
                    {hoveredNode.device_name}
                  </span>
                  <Badge
                    variant={
                      hoveredNode.status === 'online' ? 'default' : 'secondary'
                    }
                    className='text-[10px]'
                  >
                    {hoveredNode.status}
                  </Badge>
                </div>
                <div className='font-mono text-[11px] text-muted-foreground'>
                  {hoveredNode.gpu_model || hoveredNode.cpu_model || 'Standard CPU'}
                </div>
                <div className='grid grid-cols-2 gap-2 pt-1 font-mono text-[11px]'>
                  <div>
                    <span className='text-muted-foreground'>TPS: </span>
                    <span className='font-semibold text-emerald-500'>
                      {hoveredNode.current_tps.toFixed(1)}
                    </span>
                  </div>
                  <div>
                    <span className='text-muted-foreground'>Ping: </span>
                    <span>{hoveredNode.ping_ms} ms</span>
                  </div>
                  <div>
                    <span className='text-muted-foreground'>RAM: </span>
                    <span>
                      {(hoveredNode.physical_ram_mb / 1024).toFixed(1)} GB
                    </span>
                  </div>
                  <div>
                    <span className='text-muted-foreground'>VRAM: </span>
                    <span>{(hoveredNode.vram_mb / 1024).toFixed(1)} GB</span>
                  </div>
                </div>
                <div className='text-[10px] text-muted-foreground pt-0.5'>
                  {hoveredNode.city ? `${hoveredNode.city}, ` : ''}
                  {hoveredNode.country || 'Global'}
                </div>
              </div>
            ) : (
              <div className='space-y-1.5'>
                <div className='flex items-center justify-between gap-2 border-b border-border/50 pb-1'>
                  <span className='font-semibold text-xs'>{hoveredCountry}</span>
                  <Badge variant='outline' className='text-[10px]'>
                    {activeCountryNodes.length}{' '}
                    {activeCountryNodes.length === 1
                      ? t('byog.node')
                      : t('byog.nodes')}
                  </Badge>
                </div>
                {activeCountryNodes.length > 0 ? (
                  <div className='space-y-1 pt-1 text-[11px]'>
                    <div className='flex justify-between font-mono'>
                      <span className='text-muted-foreground'>
                        {t('byog.admin.aggregate_tps')}:
                      </span>
                      <span className='font-semibold text-emerald-500'>
                        {activeCountryNodes
                          .reduce((acc, n) => acc + (n.current_tps || 0), 0)
                          .toFixed(1)}{' '}
                        TPS
                      </span>
                    </div>
                    <div className='flex justify-between font-mono'>
                      <span className='text-muted-foreground'>
                        {t('byog.admin.avg_ping')}:
                      </span>
                      <span>
                        {(
                          activeCountryNodes.reduce(
                            (acc, n) => acc + (n.ping_ms || 0),
                            0
                          ) / activeCountryNodes.length
                        ).toFixed(0)}{' '}
                        ms
                      </span>
                    </div>
                    <div className='max-h-20 overflow-y-auto border-t border-border/40 pt-1 text-[10px] text-muted-foreground'>
                      {activeCountryNodes.map((n) => (
                        <div key={n.id} className='truncate'>
                          • {n.device_name} ({n.gpu_model || n.cpu_model})
                        </div>
                      ))}
                    </div>
                  </div>
                ) : (
                  <div className='text-[10px] text-muted-foreground pt-1'>
                    {t('byog.admin.no_nodes_in_region')}
                  </div>
                )}
              </div>
            )}
          </div>
        )}
      </div>

      {/* Bottom Telemetry Bar - Zero Icons */}
      <div className='pointer-events-none absolute right-4 bottom-4 left-4 z-10 flex flex-wrap items-center justify-between gap-2 border-t border-border/40 pt-2 text-[11px] text-muted-foreground'>
        <div className='flex items-center gap-2'>
          <span className='inline-block h-2 w-2 rounded-full bg-emerald-500 animate-pulse' />
          <span>{t('byog.admin.p2p_mesh_live')}</span>
        </div>
        <div className='flex items-center gap-3 font-mono'>
          <span className='font-semibold text-emerald-500'>
            {aggregateTps.toFixed(1)} TPS
          </span>
          <span>{t('byog.admin.bandwidth_optimized_tunneling')}</span>
        </div>
      </div>
    </Card>
  )
}
