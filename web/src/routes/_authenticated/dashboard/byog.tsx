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
import { createFileRoute, redirect } from '@tanstack/react-router'

import { ByogUserDashboard } from '@/features/byog'
import { useSystemConfigStore } from '@/stores/system-config-store'

/**
 * The BYOG module can be switched off in the System Settings. Hiding the link
 * of the sidebar is not enough: the address of the page is easy to guess, so
 * the route itself refuses to load when the module is disabled.
 */
async function isByogEnabled(): Promise<boolean | undefined> {
  const cached = useSystemConfigStore.getState().config.byogEnabled
  if (cached !== undefined) return cached

  try {
    const response = await fetch('/api/status', {
      headers: { Accept: 'application/json' },
    })
    if (!response.ok) return undefined
    const payload = (await response.json()) as {
      data?: { byog_enabled?: boolean }
    }
    return payload?.data?.byog_enabled
  } catch {
    return undefined
  }
}

export const Route = createFileRoute('/_authenticated/dashboard/byog')({
  beforeLoad: async () => {
    const enabled = await isByogEnabled()
    if (enabled === false) {
      throw redirect({ to: '/dashboard' })
    }
  },
  component: ByogUserDashboard,
})
