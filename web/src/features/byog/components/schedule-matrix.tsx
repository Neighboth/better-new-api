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
import { BatteryCharging, Clock, Gauge, Shield, Users, Zap } from 'lucide-react'
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
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'

interface ScheduleMatrixProps {
  operatingMode: 'mode_b' | 'mode_a'
  powerProfile: 'full_power' | 'half_power' | 'active_hours' | 'dual_schedule'
  scheduleConfig?: string
  onOperatingModeChange: (mode: 'mode_b' | 'mode_a') => void
  onPowerProfileChange: (
    profile: 'full_power' | 'half_power' | 'active_hours' | 'dual_schedule'
  ) => void
  onScheduleConfigChange?: (config: string) => void
}

export function ScheduleMatrix({
  operatingMode,
  powerProfile,
  scheduleConfig,
  onOperatingModeChange,
  onPowerProfileChange,
  onScheduleConfigChange,
}: ScheduleMatrixProps) {
  const { t } = useTranslation()

  const config = useMemo(() => {
    try {
      if (scheduleConfig) {
        return JSON.parse(scheduleConfig)
      }
    } catch {
      // fallback
    }
    return {
      start_time: '09:00',
      end_time: '18:00',
      peak_start: '08:00',
      peak_end: '22:00',
    }
  }, [scheduleConfig])

  const updateSchedule = (key: string, value: string) => {
    const updated = { ...config, [key]: value }
    onScheduleConfigChange?.(JSON.stringify(updated))
  }

  return (
    <div className='space-y-6'>
      {/* Section 4.3 Mode Selection */}
      <Card className='border-border/80'>
        <CardHeader>
          <CardTitle className='text-base font-semibold'>
            {t('byog.operating_mode_title')}
          </CardTitle>
          <CardDescription>{t('byog.operating_mode_desc')}</CardDescription>
        </CardHeader>
        <CardContent>
          <RadioGroup
            value={operatingMode}
            onValueChange={(val) =>
              onOperatingModeChange(val as 'mode_b' | 'mode_a')
            }
            className='grid grid-cols-1 gap-4 md:grid-cols-2'
          >
            {/* Mode B: Community Pool & Earn - DEFAULT & RECOMMENDED */}
            <Label
              htmlFor='mode-b'
              className={`hover:border-primary/50 relative flex cursor-pointer flex-col rounded-xl border p-4 transition-all ${
                operatingMode === 'mode_b'
                  ? 'border-primary bg-primary/5 ring-primary/20 ring-2'
                  : 'border-border'
              }`}
            >
              <div className='flex items-center justify-between'>
                <div className='flex items-center gap-2 font-semibold'>
                  <RadioGroupItem value='mode_b' id='mode-b' />
                  <Users className='text-primary h-4 w-4' />
                  {t('byog.mode_b_title')}
                </div>
                <Badge
                  variant='default'
                  className='bg-primary text-primary-foreground text-[10px] font-bold tracking-wide uppercase'
                >
                  {t('byog.recommended_badge')}
                </Badge>
              </div>
              <p className='text-muted-foreground mt-2 text-xs leading-relaxed'>
                {t('byog.mode_b_desc')}
              </p>
            </Label>

            {/* Mode A: Private Mode */}
            <Label
              htmlFor='mode-a'
              className={`hover:border-primary/50 relative flex cursor-pointer flex-col rounded-xl border p-4 transition-all ${
                operatingMode === 'mode_a'
                  ? 'border-primary bg-primary/5 ring-primary/20 ring-2'
                  : 'border-border'
              }`}
            >
              <div className='flex items-center gap-2 font-semibold'>
                <RadioGroupItem value='mode_a' id='mode-a' />
                <Shield className='text-muted-foreground h-4 w-4' />
                {t('byog.mode_a_title')}
              </div>
              <p className='text-muted-foreground mt-2 text-xs leading-relaxed'>
                {t('byog.mode_a_desc')}
              </p>
            </Label>
          </RadioGroup>
        </CardContent>
      </Card>

      {/* Section 4.3 Advanced Power & Schedule Matrix */}
      <Card className='border-border/80'>
        <CardHeader>
          <CardTitle className='flex items-center gap-2 text-base font-semibold'>
            <Clock className='text-primary h-4 w-4' />
            {t('byog.power_schedule_matrix_title')}
          </CardTitle>
          <CardDescription>
            {t('byog.power_schedule_matrix_desc')}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <RadioGroup
            value={powerProfile}
            onValueChange={(val) =>
              onPowerProfileChange(
                val as
                  | 'full_power'
                  | 'half_power'
                  | 'active_hours'
                  | 'dual_schedule'
              )
            }
            className='grid grid-cols-1 gap-3 sm:grid-cols-2'
          >
            {/* Option 1: Always Full Power */}
            <Label
              htmlFor='p-full'
              className={`flex cursor-pointer items-start gap-3 rounded-lg border p-3 transition-colors ${
                powerProfile === 'full_power'
                  ? 'border-primary bg-primary/5'
                  : 'border-border'
              }`}
            >
              <RadioGroupItem
                value='full_power'
                id='p-full'
                className='mt-0.5'
              />
              <div>
                <div className='flex items-center gap-1.5 text-sm font-medium'>
                  <Zap className='h-4 w-4 text-amber-500' />
                  {t('byog.profile_full_power_title')}
                </div>
                <div className='text-muted-foreground text-xs'>
                  {t('byog.profile_full_power_desc')}
                </div>
              </div>
            </Label>

            {/* Option 2: Always Half Power / Eco */}
            <Label
              htmlFor='p-half'
              className={`flex cursor-pointer items-start gap-3 rounded-lg border p-3 transition-colors ${
                powerProfile === 'half_power'
                  ? 'border-primary bg-primary/5'
                  : 'border-border'
              }`}
            >
              <RadioGroupItem
                value='half_power'
                id='p-half'
                className='mt-0.5'
              />
              <div>
                <div className='flex items-center gap-1.5 text-sm font-medium'>
                  <Gauge className='h-4 w-4 text-emerald-500' />
                  {t('byog.profile_half_power_title')}
                </div>
                <div className='text-muted-foreground text-xs'>
                  {t('byog.profile_half_power_desc')}
                </div>
              </div>
            </Label>

            {/* Option 3: Active Hours Only */}
            <Label
              htmlFor='p-hours'
              className={`flex cursor-pointer items-start gap-3 rounded-lg border p-3 transition-colors ${
                powerProfile === 'active_hours'
                  ? 'border-primary bg-primary/5'
                  : 'border-border'
              }`}
            >
              <RadioGroupItem
                value='active_hours'
                id='p-hours'
                className='mt-0.5'
              />
              <div>
                <div className='flex items-center gap-1.5 text-sm font-medium'>
                  <Clock className='h-4 w-4 text-blue-500' />
                  {t('byog.profile_active_hours_title')}
                </div>
                <div className='text-muted-foreground text-xs'>
                  {t('byog.profile_active_hours_desc')}
                </div>
              </div>
            </Label>

            {/* Option 4: Custom Dual-Schedule Automation */}
            <Label
              htmlFor='p-dual'
              className={`flex cursor-pointer items-start gap-3 rounded-lg border p-3 transition-colors ${
                powerProfile === 'dual_schedule'
                  ? 'border-primary bg-primary/5'
                  : 'border-border'
              }`}
            >
              <RadioGroupItem
                value='dual_schedule'
                id='p-dual'
                className='mt-0.5'
              />
              <div>
                <div className='flex items-center gap-1.5 text-sm font-medium'>
                  <BatteryCharging className='h-4 w-4 text-purple-500' />
                  {t('byog.profile_dual_schedule_title')}
                </div>
                <div className='text-muted-foreground text-xs'>
                  {t('byog.profile_dual_schedule_desc')}
                </div>
              </div>
            </Label>
          </RadioGroup>

          {/* Time Picker Panel for Active Hours */}
          {powerProfile === 'active_hours' && (
            <div className='mt-4 rounded-xl border border-blue-500/20 bg-blue-500/5 p-4 space-y-3'>
              <div className='flex items-center gap-2 text-sm font-semibold text-blue-600 dark:text-blue-400'>
                <Clock className='h-4 w-4' />
                <span>{t('byog.active_hours_settings', 'Aktif Çalışma Saatleri')}</span>
              </div>
              <p className='text-xs text-muted-foreground'>
                {t(
                  'byog.active_hours_desc_hint',
                  'Cihazınız yalnızca bu zaman aralığında ağa hizmet verir ve token işler.'
                )}
              </p>
              <div className='grid grid-cols-1 sm:grid-cols-2 gap-4 pt-1'>
                <div className='space-y-1.5'>
                  <Label htmlFor='active-start' className='text-xs font-medium'>
                    {t('byog.active_hours_start', 'Başlangıç Saati')}
                  </Label>
                  <Input
                    id='active-start'
                    type='time'
                    value={config.start_time || '09:00'}
                    onChange={(e) => updateSchedule('start_time', e.target.value)}
                  />
                </div>
                <div className='space-y-1.5'>
                  <Label htmlFor='active-end' className='text-xs font-medium'>
                    {t('byog.active_hours_end', 'Bitiş Saati')}
                  </Label>
                  <Input
                    id='active-end'
                    type='time'
                    value={config.end_time || '18:00'}
                    onChange={(e) => updateSchedule('end_time', e.target.value)}
                  />
                </div>
              </div>
            </div>
          )}

          {/* Time Picker Panel for Dual Schedule */}
          {powerProfile === 'dual_schedule' && (
            <div className='mt-4 rounded-xl border border-purple-500/20 bg-purple-500/5 p-4 space-y-3'>
              <div className='flex items-center gap-2 text-sm font-semibold text-purple-600 dark:text-purple-400'>
                <BatteryCharging className='h-4 w-4' />
                <span>{t('byog.dual_schedule_settings', 'İkili Zaman Programı Ayarları')}</span>
              </div>
              <p className='text-xs text-muted-foreground'>
                {t(
                  'byog.dual_schedule_desc_hint',
                  'Belirlenen gündüz saatlerinde Tam Güç, geri kalan saatlerde ise Eko Güç modunda çalışır.'
                )}
              </p>
              <div className='grid grid-cols-1 sm:grid-cols-2 gap-4 pt-1'>
                <div className='space-y-1.5'>
                  <Label htmlFor='dual-start' className='text-xs font-medium'>
                    {t('byog.dual_peak_start', 'Gündüz / Tam Güç Başlangıç')}
                  </Label>
                  <Input
                    id='dual-start'
                    type='time'
                    value={config.peak_start || '08:00'}
                    onChange={(e) => updateSchedule('peak_start', e.target.value)}
                  />
                </div>
                <div className='space-y-1.5'>
                  <Label htmlFor='dual-end' className='text-xs font-medium'>
                    {t('byog.dual_peak_end', 'Gündüz / Tam Güç Bitiş')}
                  </Label>
                  <Input
                    id='dual-end'
                    type='time'
                    value={config.peak_end || '22:00'}
                    onChange={(e) => updateSchedule('peak_end', e.target.value)}
                  />
                </div>
              </div>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
