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
import { Award, Cpu, HardDrive, Sparkles, Zap } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Label } from '@/components/ui/label'
import { Slider } from '@/components/ui/slider'
import { Switch } from '@/components/ui/switch'

import type { ByogNode } from '../types'

interface ResourceSlidersProps {
  node?: ByogNode | null
  vramGB: number
  ramGB: number
  cores: number
  diskGB: number
  hotStartEnabled?: boolean
  onVramChange: (val: number) => void
  onRamChange: (val: number) => void
  onCoresChange: (val: number) => void
  onDiskChange: (val: number) => void
  onHotStartChange?: (val: boolean) => void
}

export function ResourceSliders({
  node,
  vramGB,
  ramGB,
  cores,
  diskGB,
  hotStartEnabled,
  onVramChange,
  onRamChange,
  onCoresChange,
  onDiskChange,
  onHotStartChange,
}: ResourceSlidersProps) {
  const { t } = useTranslation()
  const [diskAllocations, setDiskAllocations] = useState<Record<string, number>>({})

  const isMobile = node?.device_type === 'mobile' || node?.device_type === 'tablet'

  // Dynamic limits bounded by the device's actual hardware specs
  const maxVram = useMemo(() => {
    if (!node) return 32
    if (isMobile) {
      return 0
    }
    const totalVramMB = (node.vram_mb || 0) + (node.shared_vram_mb || 0)
    return totalVramMB > 0 ? Math.max(1, Math.round(totalVramMB / 1024)) : 8
  }, [node, isMobile])

  // Enforce system headroom on RAM (16GB -> max 14GB, 4GB -> max 3GB, 2GB -> max 1.5GB)
  const maxRam = useMemo(() => {
    if (!node) return 14
    const totalRamGB = (node.physical_ram_mb || (isMobile ? 4096 : 8192)) / 1024
    if (totalRamGB <= 2) return 1.5
    if (totalRamGB <= 4) return 3
    if (totalRamGB <= 6) return 4.5
    if (totalRamGB <= 8) return 6.5
    if (totalRamGB <= 16) return 14
    return Math.max(2, Math.floor(totalRamGB - 2))
  }, [node])

  // Enforce system headroom on Cores (8 cores -> max 7, 18 cores -> max 16)
  const maxCores = useMemo(() => {
    if (!node) return 7
    const total = node.cpu_cores || (node.allocated_cores ? Math.max(node.allocated_cores, 8) : 8)
    if (total <= 2) return 1
    if (total <= 4) return total - 1
    if (total <= 8) return 7
    if (total <= 18) return Math.min(16, total - 1)
    return Math.max(1, total - 2)
  }, [node])

  // Strictly bound max disk to actual free space (no arbitrary 500 GB when 40 GB free)
  const maxDisk = useMemo(() => {
    if (!node) return 40
    if (node.free_disk_gb && node.free_disk_gb > 0) {
      return node.free_disk_gb
    }
    if (node.total_disk_gb && node.total_disk_gb > 0) {
      return Math.max(1, Math.round(node.total_disk_gb * 0.7))
    }
    return 40
  }, [node])

  // Calculate earning multiplier based on allocations
  const { multiplier, prestigeTier, tierColor, estimatedEarningsDaily } =
    useMemo(() => {
      let baseMult = 1.0
      if (!isMobile) {
        baseMult += (vramGB / 24) * 0.8
      } else {
        baseMult += (ramGB / 16) * 0.8
      }
      baseMult += (ramGB / 64) * 0.3
      baseMult += (cores / 32) * 0.4
      baseMult += (diskGB / 100) * 0.2

      let tier = t('byog.prestige_bronze', 'Bronze')
      let color = 'border-amber-600/40 text-amber-500 bg-amber-500/10'

      const effectiveScore = isMobile ? (ramGB * 1.5) : vramGB
      if (effectiveScore >= 24) {
        tier = t('byog.prestige_diamond', 'Diamond')
        color =
          'border-cyan-500/50 text-cyan-400 bg-cyan-500/20 shadow-[0_0_15px_rgba(6,182,212,0.4)]'
      } else if (effectiveScore >= 16) {
        tier = t('byog.prestige_gold', 'Gold')
        color =
          'border-yellow-500/50 text-yellow-400 bg-yellow-500/15 shadow-[0_0_10px_rgba(234,179,8,0.3)]'
      } else if (effectiveScore >= 8) {
        tier = t('byog.prestige_silver', 'Silver')
        color = 'border-slate-400/50 text-slate-300 bg-slate-400/15'
      }

      const dailyTokens = Math.round(baseMult * 120000)

      return {
        multiplier: baseMult.toFixed(2),
        prestigeTier: tier,
        tierColor: color,
        estimatedEarningsDaily: dailyTokens,
      }
    }, [vramGB, ramGB, cores, diskGB, isMobile, t])

  return (
    <Card className='border-border/80 bg-card/60 relative overflow-hidden backdrop-blur-sm'>
      <CardHeader>
        <div className='flex flex-wrap items-center justify-between gap-2'>
          <div>
            <CardTitle className='flex items-center gap-2 text-lg font-bold'>
              <Zap className='text-primary h-5 w-5' />
              {t('byog.resource_allocation_title', 'Resource Allocation')}
            </CardTitle>
            <CardDescription>
              {t(
                'byog.resource_allocation_desc',
                'Adjust how much hardware power this node contributes.'
              )}
            </CardDescription>
          </div>
          <Badge
            variant='outline'
            className={`flex items-center gap-1.5 px-3 py-1 font-semibold transition-all ${tierColor}`}
          >
            <Award className='h-4 w-4' />
            {prestigeTier}
          </Badge>
        </div>
      </CardHeader>

      <CardContent className='space-y-6'>
        {/* Dynamic Multiplier & Reward Stats Bar */}
        <div className='border-primary/20 bg-primary/5 grid grid-cols-2 gap-3 rounded-xl border p-4 md:grid-cols-3'>
          <div>
            <span className='text-muted-foreground text-xs font-medium'>
              {t('byog.estimated_multiplier', 'Estimated Multiplier')}
            </span>
            <div className='text-primary mt-1 flex items-baseline gap-1 text-2xl font-black'>
              {multiplier}x
              <Sparkles className='text-primary h-4 w-4 animate-pulse' />
            </div>
          </div>
          <div>
            <span className='text-muted-foreground text-xs font-medium'>
              {t('byog.estimated_daily_tokens', 'Est. Daily Tokens')}
            </span>
            <div className='text-foreground mt-1 text-2xl font-black'>
              +{estimatedEarningsDaily.toLocaleString()}
            </div>
          </div>
          <div className='col-span-2 md:col-span-1'>
            <span className='text-muted-foreground text-xs font-medium'>
              {t('byog.hardware_rank', 'Hardware Rank')}
            </span>
            <div className='text-foreground mt-1 text-sm font-semibold'>
              {prestigeTier}
            </div>
          </div>
        </div>

        {/* Sliders */}
        <div className='space-y-5'>
          {/* VRAM Slider */}
          {!isMobile ? (
            <div className='space-y-2'>
              <div className='flex justify-between text-sm font-medium'>
                <Label className='flex items-center gap-1.5'>
                  <Cpu className='text-primary h-4 w-4' />
                  {t('byog.vram_allocation', 'VRAM Allocation')}
                </Label>
                <span className='text-primary font-mono font-bold'>
                  {Math.min(vramGB, maxVram)} GB / {maxVram} GB
                </span>
              </div>
              <Slider
                value={[Math.min(vramGB, maxVram)]}
                min={0}
                max={maxVram}
                step={1}
                onValueChange={(val) =>
                  onVramChange(Array.isArray(val) ? val[0] : (val as number))
                }
              />
            </div>
          ) : (
            <div className='rounded-lg border bg-muted/20 p-2.5 flex items-center justify-between text-xs'>
              <span className='text-muted-foreground font-medium flex items-center gap-1.5'>
                <Cpu className='h-3.5 w-3.5 text-primary' />
                {t('byog.vram_allocation', 'VRAM Tahsisi')}:
              </span>
              <span className='font-mono font-semibold text-foreground'>
                0 GB ({t('byog.uma_shared', 'Ayrık VRAM Yok / Paylaşımlı Sistem Belleği UMA')})
              </span>
            </div>
          )}

          {/* RAM Slider */}
          <div className='space-y-2'>
            <div className='flex justify-between text-sm font-medium'>
              <Label className='flex items-center gap-1.5'>
                <HardDrive className='text-primary h-4 w-4' />
                {t('byog.ram_allocation', 'RAM Allocation')}
              </Label>
              <span className='text-primary font-mono font-bold'>
                {Math.min(ramGB, maxRam)} GB / {maxRam} GB
              </span>
            </div>
            <Slider
              value={[Math.min(ramGB, maxRam)]}
              min={1}
              max={maxRam}
              step={1}
              onValueChange={(val) =>
                onRamChange(Array.isArray(val) ? val[0] : (val as number))
              }
            />
          </div>

          {/* Cores Slider */}
          <div className='space-y-2'>
            <div className='flex justify-between text-sm font-medium'>
              <Label className='flex items-center gap-1.5'>
                <Cpu className='text-primary h-4 w-4' />
                {t('byog.core_allocation', 'CPU Cores Allocation')}
              </Label>
              <span className='text-primary font-mono font-bold'>
                {Math.min(cores, maxCores)} / {maxCores} {t('byog.threads', 'Cores')}
              </span>
            </div>
            <Slider
              value={[Math.min(cores, maxCores)]}
              min={1}
              max={maxCores}
              step={1}
              onValueChange={(val) =>
                onCoresChange(Array.isArray(val) ? val[0] : (val as number))
              }
            />
          </div>

          {/* Disk Partitions / Disk Slider */}
          {node?.disks && node.disks.length > 0 ? (
            <div className='space-y-4 rounded-xl border border-border/80 bg-muted/10 p-3.5'>
              <div className='flex items-center justify-between text-sm font-semibold text-foreground'>
                <span className='flex items-center gap-1.5'>
                  <HardDrive className='text-primary h-4 w-4' />
                  {t('byog.partitions_title', 'Sabit Disk Bölümleri')} ({node.disks.length})
                </span>
                <span className='font-mono text-primary text-xs font-bold'>
                  {t('byog.total_allocated', 'Toplam Tahsis')}: {diskGB} GB
                </span>
              </div>
              {node.disks.map((d) => {
                const dMax = Math.max(1, d.free_gb)
                const currentVal = diskAllocations[d.drive] ?? Math.min(Math.max(1, d.allocated_gb || 5), dMax)
                return (
                  <div key={d.drive} className='space-y-1.5 border-t border-border/40 pt-2.5'>
                    <div className='flex justify-between text-xs font-medium'>
                      <span className='text-foreground font-semibold'>{d.label}</span>
                      <span className='text-primary font-mono font-bold'>
                        {currentVal} GB / {dMax} GB ({t('byog.free', 'Boş')}: {d.free_gb} GB, {t('byog.total', 'Toplam')}: {d.total_gb} GB)
                      </span>
                    </div>
                    <Slider
                      value={[currentVal]}
                      min={1}
                      max={dMax}
                      step={1}
                      onValueChange={(val) => {
                        const num = Array.isArray(val) ? val[0] : (val as number)
                        const updated = { ...diskAllocations, [d.drive]: num }
                        setDiskAllocations(updated)
                        const sum = Object.values(updated).reduce((a, b) => a + b, 0)
                        onDiskChange(sum)
                      }}
                    />
                  </div>
                )
              })}
            </div>
          ) : (
            <div className='space-y-2'>
              <div className='flex justify-between text-sm font-medium'>
                <Label className='flex items-center gap-1.5'>
                  <HardDrive className='text-primary h-4 w-4' />
                  {t('byog.disk_allocation', 'Disk Cache Allocation')}
                </Label>
                <span className='text-primary font-mono font-bold'>
                  {Math.min(diskGB, maxDisk)} GB / {maxDisk} GB {node?.free_disk_gb ? `(${t('byog.free', 'Boş')}: ${node.free_disk_gb} GB)` : ''}
                </span>
              </div>
              <Slider
                value={[Math.min(Math.max(1, diskGB), maxDisk)]}
                min={1}
                max={maxDisk}
                step={maxDisk > 50 ? 5 : 1}
                onValueChange={(val) =>
                  onDiskChange(Array.isArray(val) ? val[0] : (val as number))
                }
              />
            </div>
          )}
        </div>

        {/* Hot Start Switch (Keep model warm in memory) */}
        <div className='flex items-center justify-between rounded-xl border border-border/80 bg-background/50 p-4 shadow-sm'>
          <div className='space-y-0.5 pr-4'>
            <div className='flex items-center gap-2 text-sm font-semibold'>
              <Sparkles className='h-4 w-4 text-amber-500' />
              <span>{t('byog.hot_start_label', 'Hot Start (Modeli Bellekte Hazır Tut)')}</span>
            </div>
            <p className='text-xs text-muted-foreground'>
              {t(
                'byog.hot_start_desc',
                'Modeli sürekli VRAM veya sistem belleğinde sıcak tutarak ilk token yanıt süresini (TTFT) minimuma indirir.'
              )}
            </p>
          </div>
          <Switch
            checked={!!hotStartEnabled}
            onCheckedChange={onHotStartChange}
          />
        </div>
      </CardContent>
    </Card>
  )
}
