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
import { Check, Coins } from 'lucide-react'
import { useCallback } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { api } from '@/lib/api'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'
import { useSystemConfigStore } from '@/stores/system-config-store'

export function CurrencySwitcher() {
  const { t } = useTranslation()
  const user = useAuthStore((s) => s.auth.user)
  const setUser = useAuthStore((s) => s.auth.setUser)
  const displayCurrencies = useSystemConfigStore(
    (s) => s.config.currency.displayCurrencies
  )
  const selectedCurrencyCode = useSystemConfigStore(
    (s) => s.config.currency.selectedCurrencyCode
  )
  const setSelectedCurrencyCode = useSystemConfigStore(
    (s) => s.setSelectedCurrencyCode
  )

  const handleSelectCurrency = useCallback(
    async (code: string) => {
      const normalized = code.toUpperCase()
      setSelectedCurrencyCode(normalized)
      if (user) {
        try {
          const parsedSetting =
            typeof user.setting === 'string'
              ? JSON.parse(user.setting || '{}')
              : { ...(user.setting ?? {}) }
          setUser({
            ...user,
            setting: { ...parsedSetting, preferred_currency: normalized },
          })
          await api.put('/api/user/self', { preferred_currency: normalized })
        } catch {
          // Best-effort persistence; don't block the UI on failure
        }
      }
    },
    [setSelectedCurrencyCode, setUser, user]
  )

  if (!displayCurrencies || displayCurrencies.length <= 1) {
    return null
  }

  const active =
    displayCurrencies.find(
      (c) =>
        c.code.toUpperCase() === (selectedCurrencyCode ?? '').toUpperCase()
    ) ?? displayCurrencies[0]

  return (
    <DropdownMenu modal={false}>
      <DropdownMenuTrigger
        render={
          <Button
            variant='ghost'
            size='icon'
            className='h-9 w-9'
            title={t('Display Currency')}
            aria-label={t('Display Currency')}
          />
        }
      >
        <Coins aria-hidden='true' className='h-4 w-4' />
        <span className='sr-only'>{t('Display Currency')}</span>
      </DropdownMenuTrigger>
      <DropdownMenuContent align='end' className='min-w-[180px]'>
        {displayCurrencies.map((option) => {
          const isSelected =
            option.code.toUpperCase() === active.code.toUpperCase()
          return (
            <DropdownMenuItem
              key={option.code}
              onClick={() => void handleSelectCurrency(option.code)}
            >
              <span aria-hidden='true' className='me-2 text-base leading-none'>
                {option.icon || option.symbol || option.code}
              </span>
              <span>
                {option.name
                  ? `${option.code} · ${option.name}`
                  : `${option.code}${option.symbol ? ` (${option.symbol})` : ''}`}
              </span>
              <Check
                size={14}
                className={cn('ms-auto', !isSelected && 'hidden')}
              />
            </DropdownMenuItem>
          )
        })}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
