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
import { useMutation, useQueryClient } from '@tanstack/react-query'
import i18next from 'i18next'
import { toast } from 'sonner'

import { updateSystemOption, updateSystemOptionsBulk } from '../api'
import type { UpdateOptionRequest } from '../types'

// Configuration keys that require status refresh
const STATUS_RELATED_KEYS = new Set([
  'SystemName',
  'Footer',
  'Logo',
  'LogoDark',
  'Favicon',
  'HideHeaderSiteName',
  'HeaderLogoHeight',
  'ServerAddress',
  'general_setting.system_name',
  'general_setting.footer',
  'HeaderNavModules',
  'SidebarModulesAdmin',
  'CustomNavItems',
  'Notice',
  'LogConsumeEnabled',
  'QuotaPerUnit',
  'USDExchangeRate',
  'DisplayInCurrencyEnabled',
  'DisplayTokenStatEnabled',
  'general_setting.quota_display_type',
  'general_setting.custom_currency_symbol',
  'general_setting.custom_currency_exchange_rate',
  'oidc.display_name',
  'byog_setting.enabled',
  'AnnouncementBars',
])

const isStatusRelated = (key: string) =>
  STATUS_RELATED_KEYS.has(key) ||
  key.startsWith('SystemName_') ||
  key.startsWith('Footer_')

export function useUpdateOption() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (request: UpdateOptionRequest) => updateSystemOption(request),
    onSuccess: (data, variables) => {
      if (data.success) {
        // Always refresh system-options and system-config
        queryClient.invalidateQueries({ queryKey: ['system-options'] })
        queryClient.invalidateQueries({ queryKey: ['system-config'] })

        // If updating frontend-display-related config, also refresh status
        if (isStatusRelated(variables.key)) {
          queryClient.invalidateQueries({ queryKey: ['status'] })
          try {
            window.localStorage.removeItem('status')
          } catch {
            /* empty */
          }
        }

        toast.success(i18next.t('Setting updated successfully'))
      } else {
        toast.error(data.message || i18next.t('Failed to update setting'))
      }
    },
    onError: (error: Error) => {
      toast.error(error.message || i18next.t('Failed to update setting'))
    },
  })
}

/**
 * Saves a whole settings card at once.
 *
 * The per-field mutation fires one request per switch, which is why a card
 * sometimes stored only half of the changes (a request fails, the page
 * refetches the options in between, and the remaining requests write stale
 * values). This mutation sends every key of the card in a single transaction
 * and refetches the caches once at the end.
 */
export function useUpdateOptionsBulk() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (options: Record<string, string | boolean | number>) => {
      const data = await updateSystemOptionsBulk(options)
      // The endpoint answers 200 with success:false for a rejected key (for
      // example a payment compliance field). Surface it as a failure so the
      // card shows the error instead of a silent no-op.
      if (!data.success) {
        throw new Error(data.message || i18next.t('Failed to update setting'))
      }
      return data
    },
    onSuccess: (data, variables) => {
      if (data.success) {
        queryClient.invalidateQueries({ queryKey: ['system-options'] })
        queryClient.invalidateQueries({ queryKey: ['system-config'] })
        queryClient.invalidateQueries({ queryKey: ['playground-runtime-config'] })
        if (Object.keys(variables).some(isStatusRelated)) {
          queryClient.invalidateQueries({ queryKey: ['status'] })
          try {
            window.localStorage.removeItem('status')
          } catch {
            /* empty */
          }
        }
      }
    },
  })
}
