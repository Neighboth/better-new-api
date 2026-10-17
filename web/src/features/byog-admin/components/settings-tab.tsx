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
import { Cpu, DollarSign, Globe, Moon, Save, Sparkles, Zap } from 'lucide-react'
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
import type { ByogSetting } from '../../byog/types'

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
  })

  useEffect(() => {
    loadSettings()
  }, [])

  async function loadSettings() {
    try {
      setLoading(true)
      const data = await getAdminSettings()
      if (data) {
        setSettings(data)
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
      const updated = await updateAdminSettings(settings)
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

  if (loading) {
    return (
      <div className='flex h-64 items-center justify-center'>
        <div className='text-muted-foreground'>{t('byog.loading')}</div>
      </div>
    )
  }

  return (
    <div className='space-y-6'>
      {/* Global Module Switch */}
      <Card
        className={
          settings.enabled
            ? 'border-primary/40'
            : 'border-destructive/40 bg-destructive/5'
        }
      >
        <CardHeader>
          <div className='flex items-center justify-between'>
            <div className='space-y-1'>
              <CardTitle className='flex items-center gap-2 text-xl'>
                <Globe className='text-primary h-5 w-5' />
                {t('byog.admin.global_toggle_title')}
              </CardTitle>
              <CardDescription>
                {t('byog.admin.global_toggle_desc')}
              </CardDescription>
            </div>
            <Switch
              checked={settings.enabled}
              onCheckedChange={(checked) =>
                setSettings({ ...settings, enabled: checked })
              }
            />
          </div>
        </CardHeader>
      </Card>

      {/* Pricing Matrix */}
      <Card>
        <CardHeader>
          <CardTitle className='flex items-center gap-2'>
            <DollarSign className='h-5 w-5 text-emerald-500' />
            {t('byog.admin.pricing_matrix_title')}
          </CardTitle>
          <CardDescription>
            {t('byog.admin.pricing_matrix_desc')}
          </CardDescription>
        </CardHeader>
        <CardContent className='space-y-6'>
          <div className='grid grid-cols-1 gap-6 md:grid-cols-3'>
            {/* Tier 1 */}
            <div className='bg-muted/20 space-y-4 rounded-xl border p-4'>
              <div className='flex items-center justify-between'>
                <div>
                  <h4 className='text-primary flex items-center gap-1.5 text-sm font-semibold'>
                    <Zap className='h-4 w-4' />
                    {t('byog.admin.tier1_title')}
                  </h4>
                  <p className='text-muted-foreground text-xs'>
                    {t('byog.admin.tier1_spec')}
                  </p>
                </div>
              </div>
              <div className='space-y-3'>
                <div>
                  <Label className='text-xs'>
                    {t('byog.admin.provider_pay_per_1m')}
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
                <div>
                  <Label className='text-xs'>
                    {t('byog.admin.customer_charge_per_1m')}
                  </Label>
                  <Input
                    type='number'
                    step='0.01'
                    value={settings.tier1_customer_price}
                    onChange={(e) =>
                      setSettings({
                        ...settings,
                        tier1_customer_price: parseFloat(e.target.value) || 0,
                      })
                    }
                  />
                </div>
              </div>
            </div>

            {/* Tier 2 */}
            <div className='bg-muted/20 space-y-4 rounded-xl border p-4'>
              <div className='flex items-center justify-between'>
                <div>
                  <h4 className='flex items-center gap-1.5 text-sm font-semibold text-blue-500'>
                    <Cpu className='h-4 w-4' />
                    {t('byog.admin.tier2_title')}
                  </h4>
                  <p className='text-muted-foreground text-xs'>
                    {t('byog.admin.tier2_spec')}
                  </p>
                </div>
              </div>
              <div className='space-y-3'>
                <div>
                  <Label className='text-xs'>
                    {t('byog.admin.provider_pay_per_1m')}
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
                <div>
                  <Label className='text-xs'>
                    {t('byog.admin.customer_charge_per_1m')}
                  </Label>
                  <Input
                    type='number'
                    step='0.01'
                    value={settings.tier2_customer_price}
                    onChange={(e) =>
                      setSettings({
                        ...settings,
                        tier2_customer_price: parseFloat(e.target.value) || 0,
                      })
                    }
                  />
                </div>
              </div>
            </div>

            {/* Tier 3 */}
            <div className='bg-muted/20 space-y-4 rounded-xl border p-4'>
              <div className='flex items-center justify-between'>
                <div>
                  <h4 className='flex items-center gap-1.5 text-sm font-semibold text-amber-500'>
                    <Sparkles className='h-4 w-4' />
                    {t('byog.admin.tier3_title')}
                  </h4>
                  <p className='text-muted-foreground text-xs'>
                    {t('byog.admin.tier3_spec')}
                  </p>
                </div>
              </div>
              <div className='space-y-3'>
                <div>
                  <Label className='text-xs'>
                    {t('byog.admin.provider_pay_per_1m')}
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
                <div>
                  <Label className='text-xs'>
                    {t('byog.admin.customer_charge_per_1m')}
                  </Label>
                  <Input
                    type='number'
                    step='0.01'
                    value={settings.tier3_customer_price}
                    onChange={(e) =>
                      setSettings({
                        ...settings,
                        tier3_customer_price: parseFloat(e.target.value) || 0,
                      })
                    }
                  />
                </div>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Network & Multiplier Parameters */}
      <Card>
        <CardHeader>
          <CardTitle className='flex items-center gap-2'>
            <Zap className='h-5 w-5 text-amber-500' />
            {t('byog.admin.network_parameters_title')}
          </CardTitle>
          <CardDescription>
            {t('byog.admin.network_parameters_desc')}
          </CardDescription>
        </CardHeader>
        <CardContent className='space-y-6'>
          <div className='grid grid-cols-1 gap-6 md:grid-cols-3'>
            <div className='space-y-2'>
              <Label className='text-sm font-medium'>
                {t('byog.admin.max_cold_start_timeout')}
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
              <p className='text-muted-foreground text-xs'>
                {t('byog.admin.max_cold_start_timeout_hint')}
              </p>
            </div>

            <div className='flex items-center justify-between rounded-xl border p-4'>
              <div className='space-y-0.5'>
                <div className='flex items-center gap-1.5 text-sm font-medium'>
                  <Moon className='h-4 w-4 text-indigo-400' />
                  {t('byog.admin.night_owl_surge')}
                </div>
                <div className='text-muted-foreground text-xs'>
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

            <div className='flex items-center justify-between rounded-xl border p-4'>
              <div className='space-y-0.5'>
                <div className='flex items-center gap-1.5 text-sm font-medium'>
                  <Sparkles className='h-4 w-4 text-emerald-400' />
                  {t('byog.admin.weekend_crunch')}
                </div>
                <div className='text-muted-foreground text-xs'>
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
        </CardContent>
      </Card>

      {/* Universal Hardware Compatibility Matrix */}
      <Card>
        <CardHeader>
          <CardTitle className='flex items-center gap-2'>
            <Cpu className='h-5 w-5 text-blue-500' />
            {t('byog.admin.hardware_backends_title')}
          </CardTitle>
          <CardDescription>
            {t('byog.admin.hardware_backends_desc')}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className='grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3'>
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
                className='bg-muted/10 hover:bg-muted/20 flex items-center justify-between rounded-lg border p-3.5 transition-colors'
              >
                <div className='space-y-0.5 pr-2'>
                  <div className='text-sm font-medium'>{hw.label}</div>
                  <div className='text-muted-foreground text-xs'>
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
      <div className='flex justify-end pt-4'>
        <Button onClick={handleSave} disabled={saving} className='gap-2 px-6'>
          <Save className='h-4 w-4' />
          {saving ? t('byog.saving') : t('byog.admin.save_settings')}
        </Button>
      </div>
    </div>
  )
}
