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
import { useMemo } from 'react'
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

interface ResourceSlidersProps {
  vramGB: number
  ramGB: number
  cores: number
  diskGB: number
  onVramChange: (val: number) => void
  onRamChange: (val: number) => void
  onCoresChange: (val: number) => void
  onDiskChange: (val: number) => void
}

export function ResourceSliders({
  vramGB,
  ramGB,
  cores,
  diskGB,
  onVramChange,
  onRamChange,
  onCoresChange,
  onDiskChange,
}: ResourceSlidersProps) {
  const { t } = useTranslation()

  // Calculate earning multiplier based on allocations
  const { multiplier, prestigeTier, tierColor, estimatedEarningsDaily } =
    useMemo(() => {
      let baseMult = 1.0
      baseMult += (vramGB / 24) * 0.8
      baseMult += (ramGB / 64) * 0.3
      baseMult += (cores / 32) * 0.4
      baseMult += (diskGB / 100) * 0.2

      let tier = t('byog.prestige_bronze')
      let color = 'border-amber-600/40 text-amber-500 bg-amber-500/10'

      if (vramGB >= 24) {
        tier = t('byog.prestige_diamond')
        color =
          'border-cyan-500/50 text-cyan-400 bg-cyan-500/20 shadow-[0_0_15px_rgba(6,182,212,0.4)]'
      } else if (vramGB >= 16) {
        tier = t('byog.prestige_gold')
        color =
          'border-yellow-500/50 text-yellow-400 bg-yellow-500/15 shadow-[0_0_10px_rgba(234,179,8,0.3)]'
      } else if (vramGB >= 8) {
        tier = t('byog.prestige_silver')
        color = 'border-slate-400/50 text-slate-300 bg-slate-400/15'
      }

      // Daily token estimate formula
      const dailyTokens = Math.round(baseMult * 120000)

      return {
        multiplier: baseMult.toFixed(2),
        prestigeTier: tier,
        tierColor: color,
        estimatedEarningsDaily: dailyTokens,
      }
    }, [vramGB, ramGB, cores, diskGB, t])

  return (
    <Card className='border-border/80 bg-card/60 relative overflow-hidden backdrop-blur-sm'>
      <CardHeader>
        <div className='flex flex-wrap items-center justify-between gap-2'>
          <div>
            <CardTitle className='flex items-center gap-2 text-lg font-bold'>
              <Zap className='text-primary h-5 w-5' />
              {t('byog.resource_allocation_title')}
            </CardTitle>
            <CardDescription>
              {t('byog.resource_allocation_desc')}
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
              {t('byog.estimated_multiplier')}
            </span>
            <div className='text-primary mt-1 flex items-baseline gap-1 text-2xl font-black'>
              {multiplier}x
              <Sparkles className='text-primary h-4 w-4 animate-pulse' />
            </div>
          </div>
          <div>
            <span className='text-muted-foreground text-xs font-medium'>
              {t('byog.estimated_daily_tokens')}
            </span>
            <div className='text-foreground mt-1 text-2xl font-black'>
              +{estimatedEarningsDaily.toLocaleString()}
            </div>
          </div>
          <div className='col-span-2 md:col-span-1'>
            <span className='text-muted-foreground text-xs font-medium'>
              {t('byog.hardware_rank')}
            </span>
            <div className='text-foreground mt-1 text-sm font-semibold'>
              {prestigeTier}
            </div>
          </div>
        </div>

        {/* Sliders */}
        <div className='space-y-5'>
          {/* VRAM Slider */}
          <div className='space-y-2'>
            <div className='flex justify-between text-sm font-medium'>
              <Label className='flex items-center gap-1.5'>
                <Cpu className='text-primary h-4 w-4' />
                {t('byog.vram_allocation')}
              </Label>
              <span className='text-primary font-mono font-bold'>
                {vramGB} GB
              </span>
            </div>
            <Slider
              value={[vramGB]}
              min={0}
              max={32}
              step={1}
              onValueChange={(val) =>
                onVramChange(Array.isArray(val) ? val[0] : (val as number))
              }
            />
          </div>

          {/* RAM Slider */}
          <div className='space-y-2'>
            <div className='flex justify-between text-sm font-medium'>
              <Label className='flex items-center gap-1.5'>
                <HardDrive className='text-primary h-4 w-4' />
                {t('byog.ram_allocation')}
              </Label>
              <span className='text-primary font-mono font-bold'>
                {ramGB} GB
              </span>
            </div>
            <Slider
              value={[ramGB]}
              min={2}
              max={64}
              step={2}
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
                {t('byog.core_allocation')}
              </Label>
              <span className='text-primary font-mono font-bold'>
                {cores} {t('byog.threads')}
              </span>
            </div>
            <Slider
              value={[cores]}
              min={2}
              max={32}
              step={2}
              onValueChange={(val) =>
                onCoresChange(Array.isArray(val) ? val[0] : (val as number))
              }
            />
          </div>

          {/* Disk Slider */}
          <div className='space-y-2'>
            <div className='flex justify-between text-sm font-medium'>
              <Label className='flex items-center gap-1.5'>
                <HardDrive className='text-primary h-4 w-4' />
                {t('byog.disk_allocation')}
              </Label>
              <span className='text-primary font-mono font-bold'>
                {diskGB} GB
              </span>
            </div>
            <Slider
              value={[diskGB]}
              min={10}
              max={250}
              step={5}
              onValueChange={(val) =>
                onDiskChange(Array.isArray(val) ? val[0] : (val as number))
              }
            />
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
