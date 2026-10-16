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
import { useCallback, useEffect, useRef, useState } from 'react'

import { getDeploymentSettings, testDeploymentConnection } from '../api'
import {
  parseDeploymentProviders,
  type DeploymentProviderStatus,
} from '../lib/deployment-providers'

interface ConnectionState {
  loading: boolean
  ok: boolean | null
  error: string | null
}

// NOTE: no connection cache on purpose. The settings form saves ~20 keys,
// each triggering a refetch; a stale cache would keep showing a passing
// connection after the key was changed to an invalid one.

type LoadingPhase = 'idle' | 'settings' | 'connection' | 'done'

export function useModelDeploymentSettings() {
  const [loading, setLoading] = useState(true)
  const [loadingPhase, setLoadingPhase] = useState<LoadingPhase>('settings')
  const [settings, setSettings] = useState<Record<string, unknown>>({
    'model_deployment.ionet.enabled': false,
  })
  const [connectionState, setConnectionState] = useState<ConnectionState>({
    loading: false,
    ok: null,
    error: null,
  })
  const initialLoadRef = useRef(true)

  // Parallel fetch: settings + connection test (when enabled).
  // Always live-tests io.net: a fake key must fail loudly instead of
  // passing on a stale cached result.
  const fetchAll = useCallback(async () => {
    setLoading(true)
    setLoadingPhase('settings')

    try {
      // Step 1: Fetch settings first (usually fast)
      // Backend returns { providers: [{provider, enabled, configured, can_connect}] }
      const response = (await getDeploymentSettings()) as unknown as {
        data?: {
          providers?: DeploymentProviderStatus[]
        }
      }
      const { ionetEnabled, modalEnabled, hfEnabled, anyEnabled } =
        parseDeploymentProviders(response?.data?.providers)

      setSettings({
        'model_deployment.ionet.enabled': ionetEnabled,
        'model_deployment.modal.enabled': modalEnabled,
        'model_deployment.huggingface.enabled': hfEnabled,
        'model_deployment.enabled': anyEnabled,
      })

      if (!anyEnabled) {
        // Not enabled, done
        setConnectionState({ loading: false, ok: null, error: null })
        setLoadingPhase('done')
        setLoading(false)
        return
      }

      // Step 2: Check connection (check cache first)
      // Only io.net has a generic list/test endpoint. Modal/HF-only setups
      // must not be blocked by an io.net connection test.
      if (!ionetEnabled) {
        setConnectionState({ loading: false, ok: true, error: null })
        setLoadingPhase('done')
        setLoading(false)
        return
      }
      // Test connection live.
      setLoadingPhase('connection')
      setConnectionState({ loading: true, ok: null, error: null })

      try {
        const connResponse = await testDeploymentConnection()
        if (connResponse?.success) {
          setConnectionState({ loading: false, ok: true, error: null })
        } else {
          const message = connResponse?.message || 'Connection failed'
          setConnectionState({ loading: false, ok: false, error: message })
        }
      } catch (error: unknown) {
        const errMsg =
          error instanceof Error ? error.message : 'Connection failed'
        setConnectionState({ loading: false, ok: false, error: errMsg })
      }
    } catch {
      // Settings fetch failed, use defaults
      setConnectionState({ loading: false, ok: null, error: null })
    } finally {
      setLoadingPhase('done')
      setLoading(false)
    }
  }, [])

  // Initial load
  useEffect(() => {
    if (initialLoadRef.current) {
      initialLoadRef.current = false
      fetchAll()
    }
  }, [fetchAll])

  const isIoNetEnabled = Boolean(settings['model_deployment.ionet.enabled'])
  const isModalEnabled = Boolean(settings['model_deployment.modal.enabled'])
  const isHfEnabled = Boolean(settings['model_deployment.huggingface.enabled'])
  const isAnyEnabled = Boolean(
    settings['model_deployment.enabled'] ??
      isIoNetEnabled ??
      isModalEnabled ??
      isHfEnabled
  )

  // Manual retry (always live).
  const testConnection = useCallback(async () => {
    setConnectionState({ loading: true, ok: null, error: null })
    setLoadingPhase('connection')

    try {
      const response = await testDeploymentConnection()
      if (response?.success) {
        setConnectionState({ loading: false, ok: true, error: null })
        return
      }
      const message = response?.message || 'Connection failed'
      setConnectionState({ loading: false, ok: false, error: message })
    } catch (error: unknown) {
      const errMsg =
        error instanceof Error ? error.message : 'Connection failed'
      setConnectionState({ loading: false, ok: false, error: errMsg })
    } finally {
      setLoadingPhase('done')
    }
  }, [])

  // Refresh all (always live, so a changed key is re-validated).
  const refresh = useCallback(() => {
    return fetchAll()
  }, [fetchAll])

  // Refresh on window focus (useful after saving settings in another page).
  // Intentionally live: settings may have changed since the last check.
  useEffect(() => {
    const handler = () => {
      fetchAll()
    }
    window.addEventListener('focus', handler)
    return () => window.removeEventListener('focus', handler)
  }, [fetchAll])

  return {
    loading,
    loadingPhase,
    settings,
    isIoNetEnabled,
    isModalEnabled,
    isHfEnabled,
    isAnyEnabled,
    refresh,
    connectionLoading: connectionState.loading,
    connectionOk: connectionState.ok,
    connectionError: connectionState.error,
    testConnection,
  }
}
