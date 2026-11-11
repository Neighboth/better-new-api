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
import { useMutation, useQueryClient, type QueryClient } from '@tanstack/react-query'
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

type SystemOptionEntry = { key: string; value: string }

function normalizeOptionValue(raw: string | boolean | number): string {
  if (typeof raw === 'boolean') return raw ? 'true' : 'false'
  return String(raw)
}

/**
 * Writes the saved values into the cached option list before the refetch
 * finishes.
 *
 * The settings cards read their values from the `system-options` query, so a
 * save that only invalidates the query leaves the page showing the previous
 * response until the request comes back — the switch visually flipped back
 * ("the save bounces"). Updating the cache first makes the saved value the
 * visible one immediately, and the refetch then confirms it.
 */
function applyOptionsToCache(
  queryClient: QueryClient,
  values: Record<string, string | boolean | number>
) {
  queryClient.setQueryData<SystemOptionEntry[] | undefined>(
    ['system-options'],
    (current) => {
      if (!current) return current
      const next = current.map((option) =>
        option.key in values
          ? { ...option, value: normalizeOptionValue(values[option.key]) }
          : option
      )
      const known = new Set(next.map((option) => option.key))
      for (const [key, value] of Object.entries(values)) {
        if (!known.has(key)) {
          next.push({ key, value: normalizeOptionValue(value) })
        }
      }
      return next
    }
  )
}

export function useUpdateOption() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (request: UpdateOptionRequest) => updateSystemOption(request),
    onSuccess: (data, variables) => {
      if (data.success) {
        applyOptionsToCache(queryClient, { [variables.key]: variables.value })
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
        applyOptionsToCache(queryClient, variables)
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
