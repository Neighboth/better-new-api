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

import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

type BlogSectionProps = {
  defaultValues: {
    BlogEnabled: boolean
  }
}

export function BlogSection({ defaultValues }: BlogSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const [blogEnabled, setBlogEnabled] = useState(defaultValues.BlogEnabled)

  useEffect(() => {
    setBlogEnabled(defaultValues.BlogEnabled)
  }, [defaultValues])

  return (
    <SettingsSection title={t('Blog')}>
      <SettingsForm onSubmit={(event) => event.preventDefault()}>
        <SettingsSwitchItem>
          <SettingsSwitchContent>
            <Label>{t('Enable blog')}</Label>
            <p className='text-muted-foreground text-sm'>
              {t(
                'Show a Blogs link in the header and serve public blog pages at /blog. Reading posts requires no account.'
              )}
            </p>
          </SettingsSwitchContent>
          <Switch
            checked={blogEnabled}
            onCheckedChange={(checked) => {
              setBlogEnabled(checked)
              void updateOption.mutateAsync({
                key: 'BlogEnabled',
                value: String(checked),
              })
            }}
          />
        </SettingsSwitchItem>
        <p className='text-muted-foreground text-sm'>
          {t('Manage posts from the Blog section in the sidebar.')}
        </p>
      </SettingsForm>
    </SettingsSection>
  )
}
