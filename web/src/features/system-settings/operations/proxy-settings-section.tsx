/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { SettingsSection } from '../components/settings-section'
import {
  SettingsForm,
  SettingsDescription,
  SettingsSwitchItem,
  SettingsSwitchContent,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { useUpdateOptionsBulk } from '../hooks/use-update-option'

export interface ProxySettingsDefaults {
  ProxyPoolEnabled?: boolean
  ProxyAutoTestIntervalMinutes?: number
  ProxyAutoDisableUnhealthy?: boolean
}

export function ProxySettingsSection({
  defaultValues,
}: {
  defaultValues?: ProxySettingsDefaults
}) {
  const { t } = useTranslation()
  const updateOptions = useUpdateOptionsBulk()

  const [enabled, setEnabled] = useState(defaultValues?.ProxyPoolEnabled ?? true)
  const [interval, setInterval] = useState(defaultValues?.ProxyAutoTestIntervalMinutes ?? 15)
  const [autoDisable, setAutoDisable] = useState(defaultValues?.ProxyAutoDisableUnhealthy ?? false)

  const handleSave = async () => {
    try {
      await updateOptions.mutateAsync({
        ProxyPoolEnabled: String(enabled),
        ProxyAutoTestIntervalMinutes: String(interval),
        ProxyAutoDisableUnhealthy: String(autoDisable),
      })
      toast.success(t('Proxy settings saved successfully'))
    } catch {
      toast.error(t('Failed to save proxy settings'))
    }
  }

  return (
    <SettingsSection title={t('Proxy Settings')}>
      <SettingsForm
        onSubmit={(e) => {
          e.preventDefault()
          void handleSave()
        }}
      >
        <SettingsPageFormActions
          isSaving={updateOptions.isPending}
          onSave={() => void handleSave()}
        />

        <div className='space-y-4'>
          <SettingsSwitchItem>
            <SettingsSwitchContent>
              <Label>{t('Enable Proxy Pool')}</Label>
              <SettingsDescription>
                {t(
                  'Activates proxy pool sticky balancing and routing for multi-key channels.'
                )}
              </SettingsDescription>
            </SettingsSwitchContent>
            <Switch checked={enabled} onCheckedChange={setEnabled} />
          </SettingsSwitchItem>

          <div className='rounded-xl border p-4 bg-card/50 space-y-2 shadow-xs'>
            <Label className='text-xs font-semibold'>
              {t('Automatic Speed Test Interval (Minutes)')}
            </Label>
            <SettingsDescription className='text-xs'>
              {t(
                'Background worker runs latency ping tests across all proxies at this interval to keep ms and speed up-to-date.'
              )}
            </SettingsDescription>
            <Input
              type='number'
              min={1}
              max={1440}
              value={interval}
              onChange={(e) => setInterval(Math.max(1, Number(e.target.value) || 15))}
              className='max-w-[200px] h-9 text-xs'
            />
          </div>

          <SettingsSwitchItem>
            <SettingsSwitchContent>
              <Label>{t('Auto-Disable Failing Proxies')}</Label>
              <SettingsDescription>
                {t(
                  'Automatically disables failing proxies when automated test encounters connection timeout or errors, and re-enables them when healthy.'
                )}
              </SettingsDescription>
            </SettingsSwitchContent>
            <Switch checked={autoDisable} onCheckedChange={setAutoDisable} />
          </SettingsSwitchItem>
        </div>
      </SettingsForm>
    </SettingsSection>
  )
}
