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
import { Info } from 'lucide-react'
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
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
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

const MULTIPLIER_ITEMS: Array<{
  key: keyof ByogMultipliersConfig
  labelKey: string
  defaultLabel: string
  descKey: string
  defaultDesc: string
}> = [
  {
    key: 'high_tps_bonus',
    labelKey: 'byog.admin.multiplier_high_tps',
    defaultLabel: 'High TPS Bonus (TPS > 40)',
    descKey: 'byog.admin.multiplier_high_tps_desc',
    defaultDesc:
      'Bonus awarded to worker nodes delivering sustained throughput exceeding 40 Tokens Per Second.',
  },
  {
    key: 'sub_second_ttft',
    labelKey: 'byog.admin.multiplier_sub_second_ttft',
    defaultLabel: 'Fast TTFT Bonus (TTFT < 1.0s)',
    descKey: 'byog.admin.multiplier_sub_second_ttft_desc',
    defaultDesc:
      'Incentive granted when Time To First Token is under 1.0 second for ultra-responsive streaming.',
  },
  {
    key: 'high_uptime_bonus',
    labelKey: 'byog.admin.multiplier_high_uptime',
    defaultLabel: 'High Uptime (7+ Days)',
    descKey: 'byog.admin.multiplier_high_uptime_desc',
    defaultDesc:
      'Reward for nodes maintaining continuous network availability for 7 or more consecutive days.',
  },
  {
    key: 'super_uptime_bonus',
    labelKey: 'byog.admin.multiplier_super_uptime',
    defaultLabel: 'Super Uptime (30+ Days)',
    descKey: 'byog.admin.multiplier_super_uptime_desc',
    defaultDesc:
      'Top-tier reliability bonus for nodes connected without interruption for 30 or more days.',
  },
  {
    key: 'residential_ip_bonus',
    labelKey: 'byog.admin.multiplier_residential_ip',
    defaultLabel: 'Residential IP Bonus',
    descKey: 'byog.admin.multiplier_residential_ip_desc',
    defaultDesc:
      'Boost applied to residential broadband nodes to broaden geographical decentralization and anti-datacenter diversity.',
  },
  {
    key: 'gigabit_fiber_bonus',
    labelKey: 'byog.admin.multiplier_gigabit_fiber',
    defaultLabel: 'Gigabit Fiber (1000M Down / 500M Up)',
    descKey: 'byog.admin.multiplier_gigabit_fiber_desc',
    defaultDesc:
      'Bonus for high-bandwidth nodes with >= 1000 Mbps download and >= 500 Mbps upload speeds.',
  },
  {
    key: 'low_latency_bonus',
    labelKey: 'byog.admin.multiplier_low_latency',
    defaultLabel: 'Low Latency (Ping < 30ms)',
    descKey: 'byog.admin.multiplier_low_latency_desc',
    defaultDesc:
      'Reward for low latency edge nodes with ping times strictly below 30ms.',
  },
  {
    key: 'clean_reputation_bonus',
    labelKey: 'byog.admin.multiplier_clean_reputation',
    defaultLabel: 'Clean Reputation (Score 100)',
    descKey: 'byog.admin.multiplier_clean_reputation_desc',
    defaultDesc:
      'Bonus awarded to nodes maintaining a flawless 100/100 reputation score without failed audits.',
  },
  {
    key: 'low_temp_bonus',
    labelKey: 'byog.admin.multiplier_low_temp',
    defaultLabel: 'Low Temperature (GPU < 55°C / CPU < 50°C)',
    descKey: 'byog.admin.multiplier_low_temp_desc',
    defaultDesc:
      'Efficiency incentive for nodes operating at safe, cool thermal thresholds under compute load.',
  },
  {
    key: 'battery_eco_bonus',
    labelKey: 'byog.admin.multiplier_battery_eco',
    defaultLabel: 'Battery Eco (Mobile Charging & < 38°C)',
    descKey: 'byog.admin.multiplier_battery_eco_desc',
    defaultDesc:
      'Safety multiplier for mobile and tablet nodes running while plugged in with battery temps below 38°C.',
  },
  {
    key: 'night_owl_surge',
    labelKey: 'byog.admin.multiplier_night_owl',
    defaultLabel: 'Night Owl Surge Rate',
    descKey: 'byog.admin.multiplier_night_owl_desc',
    defaultDesc:
      'Dynamic surge multiplier active during off-peak night hours (01:00 - 08:00).',
  },
  {
    key: 'weekend_crunch',
    labelKey: 'byog.admin.multiplier_weekend_crunch',
    defaultLabel: 'Weekend Crunch Rate',
    descKey: 'byog.admin.multiplier_weekend_crunch_desc',
    defaultDesc:
      'Surge multiplier active throughout weekends to support peak compute loads.',
  },
]

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
    client_download_url_windows: '',
    client_download_url_linux: '',
    client_download_url_darwin: '',
    client_download_url_android: '',
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
              {MULTIPLIER_ITEMS.map((item) => (
                <div
                  key={item.key}
                  className='space-y-1.5 rounded-lg border bg-card p-3'
                >
                  <div className='flex items-center justify-between gap-1'>
                    <Label className='truncate text-xs font-medium'>
                      {t(item.labelKey, item.defaultLabel)} (+%)
                    </Label>
                    <Popover>
                      <PopoverTrigger asChild>
                        <button
                          type='button'
                          className='text-muted-foreground hover:text-foreground shrink-0 rounded p-1 transition-colors'
                          aria-label={t('common.info', 'Info')}
                        >
                          <Info className='h-3.5 w-3.5' />
                        </button>
                      </PopoverTrigger>
                      <PopoverContent className='w-72 p-3 text-xs leading-relaxed shadow-lg'>
                        <div className='mb-1 font-semibold text-foreground'>
                          {t(item.labelKey, item.defaultLabel)}
                        </div>
                        <div className='text-muted-foreground'>
                          {t(item.descKey, item.defaultDesc)}
                        </div>
                      </PopoverContent>
                    </Popover>
                  </div>
                  <Input
                    type='number'
                    value={multipliers[item.key]}
                    onChange={(e) =>
                      updateMultiplier(
                        item.key,
                        parseFloat(e.target.value) || 0
                      )
                    }
                  />
                </div>
              ))}
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

      {/* Worker Client Download Links */}
      <Card>
        <CardHeader>
          <CardTitle className='text-lg font-semibold'>
            {t('byog.admin.worker_downloads_title', 'Worker Client Download Links')}
          </CardTitle>
          <CardDescription>
            {t(
              'byog.admin.worker_downloads_desc',
              'Configure direct download URLs for your worker client packages (e.g. S3, Cloudflare R2, or self-hosted release server). If left empty, the server will check for files in the ./worker/binaries/ directory.'
            )}
          </CardDescription>
        </CardHeader>
        <CardContent className='space-y-4'>
          <div className='grid grid-cols-1 gap-4 md:grid-cols-2'>
            <div className='space-y-1.5'>
              <Label className='text-xs font-medium'>
                {t('byog.admin.worker_download_windows', 'Windows Client URL (.exe)')}
              </Label>
              <Input
                placeholder='https://downloads.example.com/byog-worker-windows-amd64.exe'
                value={settings.client_download_url_windows || ''}
                onChange={(e) =>
                  setSettings({
                    ...settings,
                    client_download_url_windows: e.target.value,
                  })
                }
              />
            </div>

            <div className='space-y-1.5'>
              <Label className='text-xs font-medium'>
                {t('byog.admin.worker_download_linux', 'Linux Client URL (Binary / Archive)')}
              </Label>
              <Input
                placeholder='https://downloads.example.com/byog-worker-linux-amd64'
                value={settings.client_download_url_linux || ''}
                onChange={(e) =>
                  setSettings({
                    ...settings,
                    client_download_url_linux: e.target.value,
                  })
                }
              />
            </div>

            <div className='space-y-1.5'>
              <Label className='text-xs font-medium'>
                {t('byog.admin.worker_download_darwin', 'macOS Client URL (Metal Binary)')}
              </Label>
              <Input
                placeholder='https://downloads.example.com/byog-worker-darwin-arm64'
                value={settings.client_download_url_darwin || ''}
                onChange={(e) =>
                  setSettings({
                    ...settings,
                    client_download_url_darwin: e.target.value,
                  })
                }
              />
            </div>

            <div className='space-y-1.5'>
              <Label className='text-xs font-medium'>
                {t('byog.admin.worker_download_android', 'Android Package URL (.apk)')}
              </Label>
              <Input
                placeholder='https://downloads.example.com/byog-worker.apk'
                value={settings.client_download_url_android || ''}
                onChange={(e) =>
                  setSettings({
                    ...settings,
                    client_download_url_android: e.target.value,
                  })
                }
              />
            </div>
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
