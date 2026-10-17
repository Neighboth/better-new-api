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
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'

import { getAdminSettings, updateAdminSettings } from '../../byog/api'
import type { ByogMultipliersConfig, ByogSetting } from '../../byog/types'

const defaultMultipliers: ByogMultipliersConfig = {
  high_tps_bonus: 15,
  sub_second_ttft: 10,
  high_uptime_bonus: 20,
  super_uptime_bonus: 35,
  residential_ip_bonus: 15,
  gigabit_fiber_bonus: 10,
  low_latency_bonus: 10,
  clean_reputation_bonus: 10,
  low_temp_bonus: 5,
  battery_eco_bonus: 5,
  night_owl_surge: 30,
  weekend_crunch: 20,
}

export function ByogSettingsTab() {
  const { t } = useTranslation()
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [settings, setSettings] = useState<ByogSetting>({
    id: 1,
    enabled: true,
    tier1_provider_price: 0.15,
    tier1_customer_price: 0.35,
    tier2_provider_price: 0.08,
    tier2_customer_price: 0.2,
    tier3_provider_price: 0.03,
    tier3_customer_price: 0.1,
    max_cold_start_timeout: 45,
    night_owl_surge_enabled: true,
    weekend_crunch_enabled: true,
    hardware_backends:
      '{"cuda":true,"rocm":true,"intel":true,"metal":true,"qualcomm":true,"mali":true,"cpu":true}',
    multipliers_config: JSON.stringify(defaultMultipliers),
  })

  const [multipliers, setMultipliers] =
    useState<ByogMultipliersConfig>(defaultMultipliers)

  useEffect(() => {
    loadSettings()
  }, [])

  async function loadSettings() {
    try {
      setLoading(true)
      const data = await getAdminSettings()
      if (data) {
        setSettings(data)
        if (data.multipliers_config) {
          try {
            const parsed = JSON.parse(data.multipliers_config)
            setMultipliers({ ...defaultMultipliers, ...parsed })
          } catch {
            setMultipliers(defaultMultipliers)
          }
        }
      }
    } catch (err: any) {
      toast.error(err.message || t('byog.admin.load_settings_failed'))
    } finally {
      setLoading(false)
    }
  }

  async function handleSave() {
    try {
      setSaving(true)
      const toSave: ByogSetting = {
        ...settings,
        multipliers_config: JSON.stringify(multipliers),
      }
      const updated = await updateAdminSettings(toSave)
      setSettings(updated)
      toast.success(t('byog.admin.settings_saved_success'))
    } catch (err: any) {
      toast.error(err.message || t('byog.admin.save_settings_failed'))
    } finally {
      setSaving(false)
    }
  }

  let backends: Record<string, boolean> = {}
  try {
    backends = JSON.parse(settings.hardware_backends || '{}')
  } catch {
    backends = {}
  }

  function toggleBackend(key: string, val: boolean) {
    const updated = { ...backends, [key]: val }
    setSettings({ ...settings, hardware_backends: JSON.stringify(updated) })
  }

  function updateMultiplier(key: keyof ByogMultipliersConfig, val: number) {
    setMultipliers((prev) => ({ ...prev, [key]: val }))
  }

  if (loading) {
    return (
      <div className='flex h-64 items-center justify-center'>
        <div className='text-muted-foreground'>{t('byog.loading')}</div>
      </div>
    )
  }

  return (
    <div className='space-y-6'>
      {/* Pricing Matrix */}
      <Card>
        <CardHeader>
          <CardTitle className='text-lg font-semibold'>
            {t('byog.admin.pricing_matrix_title')}
          </CardTitle>
          <CardDescription>
            {t('byog.admin.pricing_matrix_desc')}
          </CardDescription>
        </CardHeader>
        <CardContent className='space-y-6'>
          <div className='rounded-lg border bg-muted/30 p-3.5 text-xs text-muted-foreground'>
            {t(
              'byog.admin.pricing_models_note',
              'Customer sales prices are automatically derived from the Models management section. If a model is not listed under Models, a 3x multiplier of the provider acquisition cost is applied.'
            )}
          </div>

          <div className='grid grid-cols-1 gap-4 md:grid-cols-3'>
            {/* Tier 1 */}
            <div className='space-y-3 rounded-lg border p-4'>
              <div>
                <h4 className='text-sm font-semibold'>
                  {t('byog.admin.tier1_title')}
                </h4>
                <p className='text-xs text-muted-foreground'>
                  {t('byog.admin.tier1_spec')}
                </p>
              </div>
              <div className='space-y-1.5'>
                <Label className='text-xs'>
                  {t('byog.admin.provider_pay_per_1m')} ($)
                </Label>
                <Input
                  type='number'
                  step='0.01'
                  value={settings.tier1_provider_price}
                  onChange={(e) =>
                    setSettings({
                      ...settings,
                      tier1_provider_price: parseFloat(e.target.value) || 0,
                    })
                  }
                />
              </div>
            </div>

            {/* Tier 2 */}
            <div className='space-y-3 rounded-lg border p-4'>
              <div>
                <h4 className='text-sm font-semibold'>
                  {t('byog.admin.tier2_title')}
                </h4>
                <p className='text-xs text-muted-foreground'>
                  {t('byog.admin.tier2_spec')}
                </p>
              </div>
              <div className='space-y-1.5'>
                <Label className='text-xs'>
                  {t('byog.admin.provider_pay_per_1m')} ($)
                </Label>
                <Input
                  type='number'
                  step='0.01'
                  value={settings.tier2_provider_price}
                  onChange={(e) =>
                    setSettings({
                      ...settings,
                      tier2_provider_price: parseFloat(e.target.value) || 0,
                    })
                  }
                />
              </div>
            </div>

            {/* Tier 3 */}
            <div className='space-y-3 rounded-lg border p-4'>
              <div>
                <h4 className='text-sm font-semibold'>
                  {t('byog.admin.tier3_title')}
                </h4>
                <p className='text-xs text-muted-foreground'>
                  {t('byog.admin.tier3_spec')}
                </p>
              </div>
              <div className='space-y-1.5'>
                <Label className='text-xs'>
                  {t('byog.admin.provider_pay_per_1m')} ($)
                </Label>
                <Input
                  type='number'
                  step='0.01'
                  value={settings.tier3_provider_price}
                  onChange={(e) =>
                    setSettings({
                      ...settings,
                      tier3_provider_price: parseFloat(e.target.value) || 0,
                    })
                  }
                />
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Network & Multiplier Parameters */}
      <Card>
        <CardHeader>
          <CardTitle className='text-lg font-semibold'>
            {t('byog.admin.network_parameters_title')}
          </CardTitle>
          <CardDescription>
            {t('byog.admin.network_parameters_desc')}
          </CardDescription>
        </CardHeader>
        <CardContent className='space-y-6'>
          {/* General Scheduling */}
          <div className='grid grid-cols-1 gap-4 md:grid-cols-3'>
            <div className='space-y-2 rounded-lg border p-3.5'>
              <Label className='text-xs font-medium'>
                {t('byog.admin.max_cold_start_timeout')} (s)
              </Label>
              <Input
                type='number'
                value={settings.max_cold_start_timeout}
                onChange={(e) =>
                  setSettings({
                    ...settings,
                    max_cold_start_timeout: parseInt(e.target.value) || 30,
                  })
                }
              />
              <p className='text-xs text-muted-foreground'>
                {t('byog.admin.max_cold_start_timeout_hint')}
              </p>
            </div>

            <div className='flex items-center justify-between rounded-lg border p-3.5'>
              <div className='space-y-0.5 pr-2'>
                <div className='text-xs font-medium'>
                  {t('byog.admin.night_owl_surge')}
                </div>
                <div className='text-xs text-muted-foreground'>
                  {t('byog.admin.night_owl_surge_desc')}
                </div>
              </div>
              <Switch
                checked={settings.night_owl_surge_enabled}
                onCheckedChange={(checked) =>
                  setSettings({ ...settings, night_owl_surge_enabled: checked })
                }
              />
            </div>

            <div className='flex items-center justify-between rounded-lg border p-3.5'>
              <div className='space-y-0.5 pr-2'>
                <div className='text-xs font-medium'>
                  {t('byog.admin.weekend_crunch')}
                </div>
                <div className='text-xs text-muted-foreground'>
                  {t('byog.admin.weekend_crunch_desc')}
                </div>
              </div>
              <Switch
                checked={settings.weekend_crunch_enabled}
                onCheckedChange={(checked) =>
                  setSettings({ ...settings, weekend_crunch_enabled: checked })
                }
              />
            </div>
          </div>

          {/* All Multipliers Grid */}
          <div className='space-y-3 pt-2'>
            <div className='text-sm font-semibold'>
              {t(
                'byog.admin.multiplier_coefficients_title',
                'Performance & Telemetry Multipliers'
              )}
            </div>
            <div className='grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3'>
              {/* High TPS */}
              <div className='space-y-1.5 rounded-lg border p-3'>
                <Label className='text-xs font-medium'>
                  {t('byog.admin.multiplier_high_tps', 'High TPS Bonus (TPS > 40)')} (+%)
                </Label>
                <Input
                  type='number'
                  value={multipliers.high_tps_bonus}
                  onChange={(e) =>
                    updateMultiplier(
                      'high_tps_bonus',
                      parseFloat(e.target.value) || 0
                    )
                  }
                />
              </div>

              {/* Sub-Second TTFT */}
              <div className='space-y-1.5 rounded-lg border p-3'>
                <Label className='text-xs font-medium'>
                  {t('byog.admin.multiplier_sub_second_ttft', 'Fast TTFT Bonus (TTFT < 1.0s)')} (+%)
                </Label>
                <Input
                  type='number'
                  value={multipliers.sub_second_ttft}
                  onChange={(e) =>
                    updateMultiplier(
                      'sub_second_ttft',
                      parseFloat(e.target.value) || 0
                    )
                  }
                />
              </div>

              {/* High Uptime (7d) */}
              <div className='space-y-1.5 rounded-lg border p-3'>
                <Label className='text-xs font-medium'>
                  {t('byog.admin.multiplier_high_uptime', 'High Uptime (7+ Days)')} (+%)
                </Label>
                <Input
                  type='number'
                  value={multipliers.high_uptime_bonus}
                  onChange={(e) =>
                    updateMultiplier(
                      'high_uptime_bonus',
                      parseFloat(e.target.value) || 0
                    )
                  }
                />
              </div>

              {/* Super Uptime (30d) */}
              <div className='space-y-1.5 rounded-lg border p-3'>
                <Label className='text-xs font-medium'>
                  {t('byog.admin.multiplier_super_uptime', 'Super Uptime (30+ Days)')} (+%)
                </Label>
                <Input
                  type='number'
                  value={multipliers.super_uptime_bonus}
                  onChange={(e) =>
                    updateMultiplier(
                      'super_uptime_bonus',
                      parseFloat(e.target.value) || 0
                    )
                  }
                />
              </div>

              {/* Residential IP */}
              <div className='space-y-1.5 rounded-lg border p-3'>
                <Label className='text-xs font-medium'>
                  {t('byog.admin.multiplier_residential_ip', 'Residential IP Bonus')} (+%)
                </Label>
                <Input
                  type='number'
                  value={multipliers.residential_ip_bonus}
                  onChange={(e) =>
                    updateMultiplier(
                      'residential_ip_bonus',
                      parseFloat(e.target.value) || 0
                    )
                  }
                />
              </div>

              {/* Gigabit Fiber */}
              <div className='space-y-1.5 rounded-lg border p-3'>
                <Label className='text-xs font-medium'>
                  {t('byog.admin.multiplier_gigabit_fiber', 'Gigabit Fiber (1000M Down / 500M Up)')} (+%)
                </Label>
                <Input
                  type='number'
                  value={multipliers.gigabit_fiber_bonus}
                  onChange={(e) =>
                    updateMultiplier(
                      'gigabit_fiber_bonus',
                      parseFloat(e.target.value) || 0
                    )
                  }
                />
              </div>

              {/* Low Latency */}
              <div className='space-y-1.5 rounded-lg border p-3'>
                <Label className='text-xs font-medium'>
                  {t('byog.admin.multiplier_low_latency', 'Low Latency (Ping < 30ms)')} (+%)
                </Label>
                <Input
                  type='number'
                  value={multipliers.low_latency_bonus}
                  onChange={(e) =>
                    updateMultiplier(
                      'low_latency_bonus',
                      parseFloat(e.target.value) || 0
                    )
                  }
                />
              </div>

              {/* Clean Reputation */}
              <div className='space-y-1.5 rounded-lg border p-3'>
                <Label className='text-xs font-medium'>
                  {t('byog.admin.multiplier_clean_reputation', 'Clean Reputation (Score 100)')} (+%)
                </Label>
                <Input
                  type='number'
                  value={multipliers.clean_reputation_bonus}
                  onChange={(e) =>
                    updateMultiplier(
                      'clean_reputation_bonus',
                      parseFloat(e.target.value) || 0
                    )
                  }
                />
              </div>

              {/* Low Temperature */}
              <div className='space-y-1.5 rounded-lg border p-3'>
                <Label className='text-xs font-medium'>
                  {t('byog.admin.multiplier_low_temp', 'Low Temperature (GPU < 55°C / CPU < 50°C)')} (+%)
                </Label>
                <Input
                  type='number'
                  value={multipliers.low_temp_bonus}
                  onChange={(e) =>
                    updateMultiplier(
                      'low_temp_bonus',
                      parseFloat(e.target.value) || 0
                    )
                  }
                />
              </div>

              {/* Battery Eco */}
              <div className='space-y-1.5 rounded-lg border p-3'>
                <Label className='text-xs font-medium'>
                  {t('byog.admin.multiplier_battery_eco', 'Battery Eco (Mobile Charging & < 38°C)')} (+%)
                </Label>
                <Input
                  type='number'
                  value={multipliers.battery_eco_bonus}
                  onChange={(e) =>
                    updateMultiplier(
                      'battery_eco_bonus',
                      parseFloat(e.target.value) || 0
                    )
                  }
                />
              </div>

              {/* Night Owl Surge % */}
              <div className='space-y-1.5 rounded-lg border p-3'>
                <Label className='text-xs font-medium'>
                  {t('byog.admin.multiplier_night_owl', 'Night Owl Surge Rate')} (+%)
                </Label>
                <Input
                  type='number'
                  value={multipliers.night_owl_surge}
                  onChange={(e) =>
                    updateMultiplier(
                      'night_owl_surge',
                      parseFloat(e.target.value) || 0
                    )
                  }
                />
              </div>

              {/* Weekend Crunch % */}
              <div className='space-y-1.5 rounded-lg border p-3'>
                <Label className='text-xs font-medium'>
                  {t('byog.admin.multiplier_weekend_crunch', 'Weekend Crunch Rate')} (+%)
                </Label>
                <Input
                  type='number'
                  value={multipliers.weekend_crunch}
                  onChange={(e) =>
                    updateMultiplier(
                      'weekend_crunch',
                      parseFloat(e.target.value) || 0
                    )
                  }
                />
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Universal Hardware Compatibility Matrix */}
      <Card>
        <CardHeader>
          <CardTitle className='text-lg font-semibold'>
            {t('byog.admin.hardware_backends_title')}
          </CardTitle>
          <CardDescription>
            {t('byog.admin.hardware_backends_desc')}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className='grid grid-cols-1 gap-3 md:grid-cols-2 lg:grid-cols-3'>
            {[
              { id: 'cuda', label: 'NVIDIA CUDA (Compute Capability 5.0+)' },
              { id: 'rocm', label: 'AMD ROCm / HIP & Vulkan / DirectML' },
              {
                id: 'intel',
                label:
                  'Intel Iris Xe, UHD & Arc (OneAPI / Vulkan / Shared VRAM)',
              },
              {
                id: 'metal',
                label: 'Apple Silicon Metal (M1-M4 & A15-A18 Pro Unified)',
              },
              {
                id: 'qualcomm',
                label: 'Qualcomm Snapdragon (Adreno & Hexagon NPU / QNN)',
              },
              { id: 'mali', label: 'ARM Mali & Google Tensor (Vulkan Engine)' },
              {
                id: 'cpu',
                label: 'CPU Vector Extensions (AVX2, AVX-512, ARM NEON/SVE)',
              },
            ].map((hw) => (
              <div
                key={hw.id}
                className='flex items-center justify-between rounded-lg border p-3 transition-colors hover:bg-muted/30'
              >
                <div className='space-y-0.5 pr-2'>
                  <div className='text-xs font-medium'>{hw.label}</div>
                  <div className='text-xs text-muted-foreground'>
                    {backends[hw.id] !== false
                      ? t('byog.admin.backend_enabled')
                      : t('byog.admin.backend_disabled')}
                  </div>
                </div>
                <Switch
                  checked={backends[hw.id] !== false}
                  onCheckedChange={(val) => toggleBackend(hw.id, val)}
                />
              </div>
            ))}
          </div>
        </CardContent>
      </Card>

      {/* Action Footer */}
      <div className='flex justify-end pt-2'>
        <Button onClick={handleSave} disabled={saving} className='px-6'>
          {saving ? t('byog.saving') : t('byog.admin.save_settings')}
        </Button>
      </div>
    </div>
  )
}
