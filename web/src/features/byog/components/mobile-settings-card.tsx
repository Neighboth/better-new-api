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
import {
  Battery,
  BatteryCharging,
  Info,
  Smartphone,
  Thermometer,
  Wifi,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription } from '@/components/ui/alert'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Slider } from '@/components/ui/slider'
import { Switch } from '@/components/ui/switch'

interface MobileSettingsCardProps {
  wifiOnly: boolean
  dataLimitMB: number
  chargingOnly: boolean
  batteryCutoffEnabled: boolean
  batteryCutoffPercent: number
  maxBatteryTemp: number
  batteryTempSupported: boolean
  onWifiOnlyChange: (val: boolean) => void
  onDataLimitChange: (val: number) => void
  onChargingOnlyChange: (val: boolean) => void
  onBatteryCutoffEnabledChange: (val: boolean) => void
  onBatteryCutoffPercentChange: (val: number) => void
  onMaxBatteryTempChange: (val: number) => void
}

export function MobileSettingsCard({
  wifiOnly,
  dataLimitMB,
  chargingOnly,
  batteryCutoffEnabled,
  batteryCutoffPercent,
  maxBatteryTemp,
  batteryTempSupported,
  onWifiOnlyChange,
  onDataLimitChange,
  onChargingOnlyChange,
  onBatteryCutoffEnabledChange,
  onBatteryCutoffPercentChange,
  onMaxBatteryTempChange,
}: MobileSettingsCardProps) {
  const { t } = useTranslation()

  return (
    <Card className='border-border/80'>
      <CardHeader>
        <CardTitle className='flex items-center gap-2 text-base font-semibold'>
          <Smartphone className='text-primary h-4 w-4' />
          {t('byog.mobile_governance_title')}
        </CardTitle>
        <CardDescription>{t('byog.mobile_governance_desc')}</CardDescription>
      </CardHeader>

      <CardContent className='space-y-6'>
        {/* 1. Download on Wi-Fi Only - DEFAULT: ON */}
        <div className='flex items-center justify-between rounded-lg border p-3.5'>
          <div className='space-y-0.5'>
            <div className='flex items-center gap-2 text-sm font-medium'>
              <Wifi className='text-primary h-4 w-4' />
              {t('byog.wifi_only_title')}
            </div>
            <p className='text-muted-foreground text-xs'>
              {t('byog.wifi_only_desc')}
            </p>
          </div>
          <Switch checked={wifiOnly} onCheckedChange={onWifiOnlyChange} />
        </div>

        {/* 2. Monthly Data Limit - DEFAULT: UNLIMITED */}
        <div className='flex flex-col gap-2 rounded-lg border p-3.5 sm:flex-row sm:items-center sm:justify-between'>
          <div className='space-y-0.5'>
            <div className='text-sm font-medium'>
              {t('byog.monthly_data_limit_title')}
            </div>
            <p className='text-muted-foreground text-xs'>
              {t('byog.monthly_data_limit_desc')}
            </p>
          </div>
          <Select
            value={dataLimitMB.toString()}
            onValueChange={(val) => onDataLimitChange(parseInt(val || '0', 10))}
          >
            <SelectTrigger className='w-[160px]'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value='0'>
                {t('byog.data_limit_unlimited')}
              </SelectItem>
              <SelectItem value='500'>500 MB</SelectItem>
              <SelectItem value='1024'>1 GB</SelectItem>
              <SelectItem value='5120'>5 GB</SelectItem>
              <SelectItem value='10240'>10 GB</SelectItem>
              <SelectItem value='25600'>25 GB</SelectItem>
            </SelectContent>
          </Select>
        </div>

        {/* 3. Run Only While Charging - DEFAULT: OFF */}
        <div className='flex items-center justify-between rounded-lg border p-3.5'>
          <div className='space-y-0.5'>
            <div className='flex items-center gap-2 text-sm font-medium'>
              <BatteryCharging className='text-primary h-4 w-4' />
              {t('byog.charging_only_title')}
            </div>
            <p className='text-muted-foreground text-xs'>
              {t('byog.charging_only_desc')}
            </p>
          </div>
          <Switch
            checked={chargingOnly}
            onCheckedChange={onChargingOnlyChange}
          />
        </div>

        {/* 4. Battery Level Cutoff - DEFAULT: OFF */}
        <div className='space-y-3 rounded-lg border p-3.5'>
          <div className='flex items-center justify-between'>
            <div className='space-y-0.5'>
              <div className='flex items-center gap-2 text-sm font-medium'>
                <Battery className='text-primary h-4 w-4' />
                {t('byog.battery_cutoff_title')}
              </div>
              <p className='text-muted-foreground text-xs'>
                {t('byog.battery_cutoff_desc')}
              </p>
            </div>
            <Switch
              checked={batteryCutoffEnabled}
              onCheckedChange={onBatteryCutoffEnabledChange}
            />
          </div>

          {batteryCutoffEnabled && (
            <div className='border-border/50 space-y-2 border-t pt-2'>
              <div className='flex justify-between text-xs font-medium'>
                <span>{t('byog.minimum_battery_threshold')}</span>
                <span className='text-primary font-mono font-bold'>
                  {batteryCutoffPercent}%
                </span>
              </div>
              <Slider
                value={[batteryCutoffPercent]}
                min={5}
                max={50}
                step={5}
                onValueChange={(val) =>
                  onBatteryCutoffPercentChange(
                    Array.isArray(val) ? val[0] : (val as number)
                  )
                }
              />
            </div>
          )}
        </div>

        {/* 5. Max Battery Temp Cutoff - DEFAULT: 50°C */}
        <div className='space-y-3 rounded-lg border p-3.5'>
          <div className='flex items-center justify-between'>
            <div className='space-y-0.5'>
              <div className='flex items-center gap-2 text-sm font-medium'>
                <Thermometer className='text-primary h-4 w-4' />
                {t('byog.max_battery_temp_title')}
              </div>
              <p className='text-muted-foreground text-xs'>
                {t('byog.max_battery_temp_desc')}
              </p>
            </div>
            <span className='text-primary font-mono text-sm font-bold'>
              {maxBatteryTemp}°C
            </span>
          </div>

          <Slider
            value={[maxBatteryTemp]}
            min={40}
            max={60}
            step={1}
            onValueChange={(val) =>
              onMaxBatteryTempChange(
                Array.isArray(val) ? val[0] : (val as number)
              )
            }
          />

          {!batteryTempSupported && (
            <Alert className='border-muted bg-muted/40 py-2 text-xs'>
              <Info className='text-muted-foreground h-3.5 w-3.5' />
              <AlertDescription className='text-muted-foreground text-xs'>
                {t('byog.battery_temp_sensor_missing_note')}
              </AlertDescription>
            </Alert>
          )}
        </div>
      </CardContent>
    </Card>
  )
}
