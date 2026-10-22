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
import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  Activity,
  Battery,
  Bell,
  Cpu,
  Gauge,
  Link2,
  Play,
  RefreshCw,
  Server,
  Terminal,
  Zap,
} from 'lucide-react'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'

interface WorkerState {
  node_id: string
  device_name: string
  status: string
  model_status: string
  download_status: string
  download_progress: number
  current_tps: number
  models: string
}

interface PermissionState {
  notifications: boolean
  battery: boolean
  autostart: boolean
  bgactivity: boolean
  autostart_supported: boolean
  bgactivity_supported: boolean
}

/**
 * The bridge of the application of Android.
 *
 * The page is shown by the client of the installation, the application of
 * Android gives the state of the device with it.
 */
interface AndroidBridge {
  getInitialState(): string
  getWorkerStatus(): string
  getPermissionStatus(): string
  requestNotificationPermission(): void
  requestBatteryOptimization(): void
  openAutoStartSettings(): void
  openBackgroundActivitySettings(): void
  connect(pairingCode: string, serverUrl: string): void
}

function getBridge(): AndroidBridge | null {
  if (typeof window === 'undefined') {
    return null
  }
  const bridge = (window as unknown as Record<string, unknown>).AndroidBridge
  return bridge ? (bridge as AndroidBridge) : null
}

const PAGES = [
  { key: 'status', icon: Activity },
  { key: 'pairing', icon: Link2 },
  { key: 'settings', icon: Server },
  { key: 'test', icon: Terminal },
] as const

// The languages of the installation.
const LANGUAGES = ['tr', 'en', 'zh_CN', 'zh_TW', 'fr', 'ru', 'ja', 'vi']

type PageKey = (typeof PAGES)[number]['key'] | 'permissions'

/**
 * The page of the client of the local computing network.
 *
 * The page is shown by the client of the installation, it uses the theme of the
 * site and it does not need the session of the user. The client of the
 * installation gives the state of the device with the server of the computer,
 * the browsers ask the server of the installation for it.
 */
export function ClientPage() {
  const { t, i18n } = useTranslation()
  const [page, setPage] = useState<PageKey>('status')
  const [nodeId, setNodeId] = useState('')
  const [state, setState] = useState<WorkerState | null>(null)
  const [permissions, setPermissions] = useState<PermissionState | null>(null)
  const [serverUrl, setServerUrl] = useState('')
  const [language, setLanguage] = useState('')
  const [code, setCode] = useState('')
  const [prompt, setPrompt] = useState('')
  const [answer, setAnswer] = useState('')
  const [message, setMessage] = useState('')
  const [busy, setBusy] = useState(false)

  const bridge = getBridge()
  // The client of the installation runs a small server on the computer of the
  // user, the page asks the state of the client from it.
  const [localApi, setLocalApi] = useState('')
  const [localToken, setLocalToken] = useState('')

  useEffect(() => {
    const search = new URLSearchParams(window.location.search)
    const fromQuery = search.get('node') ?? ''
    const stored = window.localStorage.getItem('byog_node_id') ?? ''
    setNodeId(fromQuery || stored)
    setServerUrl(window.location.origin)
    setLocalApi(search.get('local') ?? '')
    setLocalToken(search.get('token') ?? '')
  }, [])

  const loadState = useCallback(async () => {
    // The application of Android knows the state of the device, the client of
    // the installation has a server of its own, the browsers ask the server of
    // the installation for it.
    if (bridge) {
      try {
        const raw = bridge.getInitialState()
        const initial = JSON.parse(raw) as Record<string, unknown>
        if (typeof initial.server_url === 'string' && initial.server_url) {
          setServerUrl(initial.server_url)
        }
        if (typeof initial.node_id === 'string' && initial.node_id) {
          setNodeId(initial.node_id)
        }
        setState({
          node_id: String(initial.node_id ?? ''),
          device_name: String(initial.app_name ?? ''),
          status: initial.is_paired ? 'online' : 'offline',
          model_status: String(initial.model_status ?? ''),
          download_status: String(initial.model_status ?? ''),
          download_progress: Number(initial.download_progress ?? 0),
          current_tps: Number(initial.current_tps ?? 0),
          models: String(initial.assigned_model ?? ''),
        })
      } catch {
        // The application of Android does not answer, the state stays as it is.
      }
      try {
        const raw = bridge.getPermissionStatus()
        setPermissions(JSON.parse(raw) as PermissionState)
      } catch {
        // The permissions are not known.
      }
      return
    }

    if (localApi) {
      try {
        const response = await fetch(`${localApi}/state`, {
          headers: { 'X-Client-Key': localToken },
        })
        const payload = await response.json()
        if (payload.success !== false) {
          setNodeId(payload.node_id ?? '')
          setServerUrl(payload.server_url ?? '')
          setLanguage(payload.language ?? '')
          setState({
            node_id: payload.node_id ?? '',
            device_name: payload.device_name ?? '',
            status: payload.connected ? 'online' : 'offline',
            model_status: payload.model_status ?? '',
            download_status: '',
            download_progress: payload.download_progress ?? 0,
            current_tps: payload.current_tps ?? 0,
            models: payload.model_name ?? '',
          })
        }
      } catch {
        // The client of the installation does not answer.
      }
      return
    }

    if (!nodeId) {
      return
    }
    try {
      const response = await fetch(
        `/api/v1/worker/status?node_id=${encodeURIComponent(nodeId)}`
      )
      const payload = await response.json()
      if (payload.success) {
        setState(payload.data)
      }
    } catch {
      // The client shows the state it knows when the server does not answer.
    }
  }, [bridge, localApi, localToken, nodeId])

  useEffect(() => {
    loadState()
    const timer = window.setInterval(loadState, 4000)
    return () => window.clearInterval(timer)
  }, [loadState])

  const pair = async () => {
    setBusy(true)
    setMessage('')
    if (bridge) {
      // The application of Android starts the worker with the code.
      bridge.connect(code.trim(), serverUrl)
      setMessage(t('byog.client_paired'))
      setBusy(false)
      return
    }
    if (localApi) {
      try {
        const response = await fetch(`${localApi}/pair`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'X-Client-Key': localToken,
          },
          body: JSON.stringify({ code: code.trim(), server_url: serverUrl }),
        })
        const payload = await response.json()
        setMessage(
          payload.success ? t('byog.client_paired') : payload.message
        )
        loadState()
      } catch {
        setMessage(t('byog.client_pair_failed'))
      } finally {
        setBusy(false)
      }
      return
    }
    try {
      const response = await fetch('/api/v1/worker/register', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          pairing_code: code.trim(),
          device_name: 'BYOG Worker',
          device_type: 'desktop',
          os: 'windows',
          arch: 'amd64',
        }),
      })
      const payload = await response.json()
      if (payload.success && payload.data?.node_id) {
        window.localStorage.setItem('byog_node_id', payload.data.node_id)
        setNodeId(payload.data.node_id)
        setMessage(t('byog.client_paired'))
        loadState()
      } else {
        setMessage(payload.message ?? t('byog.client_pair_failed'))
      }
    } catch {
      setMessage(t('byog.client_pair_failed'))
    } finally {
      setBusy(false)
    }
  }

  const runTest = async () => {
    setBusy(true)
    setAnswer('')
    try {
      let response: Response
      if (localApi) {
        response = await fetch(`${localApi}/test`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'X-Client-Key': localToken,
          },
          body: JSON.stringify({ prompt }),
        })
      } else {
        response = await fetch('/api/v1/worker/test-inference', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            node_id: nodeId,
            prompt,
            model: state?.models?.split(',')[0] ?? '',
          }),
        })
      }
      const payload = await response.json()
      if (payload.success) {
        setAnswer(
          payload.data?.reply ?? payload.data?.content ?? JSON.stringify(payload.data)
        )
      } else {
        setMessage(payload.message ?? t('byog.client_test_failed'))
      }
    } catch {
      setMessage(t('byog.client_test_failed'))
    } finally {
      setBusy(false)
    }
  }

  const changeLanguage = async (event: React.ChangeEvent<HTMLSelectElement>) => {
    const value = event.target.value
    setLanguage(value)
    if (localApi) {
      try {
        await fetch(`${localApi}/settings`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'X-Client-Key': localToken,
          },
          body: JSON.stringify({ language: value }),
        })
      } catch {
        // The client of the installation does not answer.
      }
    }
    await i18n.changeLanguage(value)
  }

  const statusLabel = () => {    if (!state) {
      return t('byog.client_offline')
    }
    if (state.status === 'online') {
      return t('byog.client_online')
    }
    if (state.download_progress > 0 && state.download_progress < 100) {
      return t('byog.client_downloading')
    }
    return state.status
  }

  const pages = bridge
    ? [...PAGES, { key: 'permissions' as PageKey, icon: Zap }]
    : PAGES

  return (
    <div className='bg-background flex h-screen w-full overflow-hidden text-left'>
      <aside className='bg-card flex w-52 shrink-0 flex-col gap-1 border-r p-3'>
        <div className='text-primary mb-2 px-3 text-lg font-bold'>
          {t('byog.client_title')}
        </div>
        {pages.map((item) => {
          const Icon = item.icon
          const active = page === item.key
          return (
            <button
              key={item.key}
              type='button'
              onClick={() => setPage(item.key)}
              className={`flex items-center gap-2 rounded-lg px-3 py-2 text-sm transition-colors ${
                active
                  ? 'bg-primary/10 text-primary font-semibold'
                  : 'text-muted-foreground hover:bg-muted'
              }`}
            >
              <Icon className='h-4 w-4' />
              {item.key === 'permissions'
                ? t('byog.client_permissions')
                : t(`byog.client_${item.key}`)}
            </button>
          )
        })}
        <div className='mt-auto px-3 text-xs text-muted-foreground'>
          {state?.device_name || nodeId || '-'}
        </div>
      </aside>

      <main className='flex-1 overflow-y-auto p-6'>
        {page === 'status' && (
          <Card>
            <CardHeader>
              <CardTitle className='flex items-center gap-2'>
                {t('byog.client_status')}
                <Badge variant={state?.status === 'online' ? 'default' : 'secondary'}>
                  {statusLabel()}
                </Badge>
                <Button
                  variant='ghost'
                  size='icon'
                  className='ml-auto'
                  onClick={() => loadState()}
                >
                  <RefreshCw className='h-4 w-4' />
                </Button>
              </CardTitle>
            </CardHeader>
            <CardContent className='grid gap-3'>
              <div className='flex items-center gap-2 text-sm'>
                <Server className='text-muted-foreground h-4 w-4' />
                <span className='text-muted-foreground w-24'>
                  {t('byog.client_server')}
                </span>
                <span className='truncate'>{serverUrl || window.location.origin}</span>
              </div>
              <div className='flex items-center gap-2 text-sm'>
                <Cpu className='text-muted-foreground h-4 w-4' />
                <span className='text-muted-foreground w-24'>
                  {t('byog.client_model')}
                </span>
                <span className='truncate'>{state?.models || '-'}</span>
              </div>
              <div className='flex items-center gap-2 text-sm'>
                <Gauge className='text-muted-foreground h-4 w-4' />
                <span className='text-muted-foreground w-24'>
                  {t('byog.client_speed')}
                </span>
                <span>{(state?.current_tps ?? 0).toFixed(1)}</span>
              </div>
              {state && state.download_progress > 0 && (
                <div className='bg-muted h-2 w-full overflow-hidden rounded-full'>
                  <div
                    className='bg-primary h-full'
                    style={{ width: `${state.download_progress}%` }}
                  />
                </div>
              )}
            </CardContent>
          </Card>
        )}

        {page === 'pairing' && (
          <Card>
            <CardHeader>
              <CardTitle>{t('byog.client_pairing')}</CardTitle>
            </CardHeader>
            <CardContent className='grid gap-3'>
              <p className='text-muted-foreground text-sm'>
                {t('byog.enter_pairing_code_instructions')}
              </p>
              <div className='flex gap-2'>
                <Input
                  value={code}
                  onChange={(event) => setCode(event.target.value)}
                  placeholder={t('byog.enter_pairing_code_label')}
                />
                <Button onClick={pair} disabled={busy || !code.trim()}>
                  {t('byog.client_connect')}
                </Button>
              </div>
              {message && <p className='text-sm'>{message}</p>}
            </CardContent>
          </Card>
        )}

        {page === 'settings' && (
          <Card>
            <CardHeader>
              <CardTitle>{t('byog.client_settings')}</CardTitle>
            </CardHeader>
            <CardContent className='grid gap-3 text-sm'>
              <div className='flex gap-2'>
                <span className='text-muted-foreground w-24'>
                  {t('byog.client_server')}
                </span>
                <span className='truncate'>
                  {serverUrl || window.location.origin}
                </span>
              </div>
              <div className='flex gap-2'>
                <span className='text-muted-foreground w-24'>
                  {t('byog.client_node')}
                </span>
                <span className='truncate'>{nodeId || '-'}</span>
              </div>
              {language && (
                <div className='flex gap-2'>
                  <span className='text-muted-foreground w-24'>
                    {t('byog.client_language')}
                  </span>
                  <span className='truncate'>{language}</span>
                </div>
              )}
              {localApi && (
                <div className='flex items-center gap-2'>
                  <span className='text-muted-foreground w-24'>
                    {t('byog.client_language')}
                  </span>
                  <select
                    value={language}
                    onChange={changeLanguage}
                    className='border-input bg-background h-9 rounded-md border px-3 text-sm'
                  >
                    {LANGUAGES.map((code) => (
                      <option key={code} value={code}>
                        {code}
                      </option>
                    ))}
                  </select>
                </div>
              )}
            </CardContent>
          </Card>
        )}

        {page === 'test' && (
          <Card>
            <CardHeader>
              <CardTitle>{t('byog.client_test')}</CardTitle>
            </CardHeader>
            <CardContent className='grid gap-3'>
              <Input
                value={prompt}
                onChange={(event) => setPrompt(event.target.value)}
                placeholder={t('byog.client_test_prompt')}
              />
              <Button onClick={runTest} disabled={busy || !nodeId}>
                <Play className='mr-2 h-4 w-4' />
                {t('byog.client_run_test')}
              </Button>
              {answer && (
                <pre className='bg-muted max-h-64 overflow-auto rounded-lg p-3 text-sm whitespace-pre-wrap'>
                  {answer}
                </pre>
              )}
              {message && <p className='text-sm'>{message}</p>}
            </CardContent>
          </Card>
        )}

        {page === 'permissions' && bridge && (
          <Card>
            <CardHeader>
              <CardTitle>{t('byog.client_permissions')}</CardTitle>
            </CardHeader>
            <CardContent className='grid gap-3'>
              <div className='flex items-center gap-3 text-sm'>
                <Bell className='text-muted-foreground h-4 w-4' />
                <span className='flex-1'>{t('byog.client_permission_notifications')}</span>
                <Badge
                  variant={
                    permissions?.notifications ? 'default' : 'secondary'
                  }
                >
                  {permissions?.notifications
                    ? t('byog.client_permission_granted')
                    : t('byog.client_permission_needed')}
                </Badge>
                {!permissions?.notifications && (
                  <Button
                    size='sm'
                    onClick={() => bridge.requestNotificationPermission()}
                  >
                    {t('byog.client_permission_grant')}
                  </Button>
                )}
              </div>
              <div className='flex items-center gap-3 text-sm'>
                <Battery className='text-muted-foreground h-4 w-4' />
                <span className='flex-1'>{t('byog.client_permission_battery')}</span>
                <Badge variant={permissions?.battery ? 'default' : 'secondary'}>
                  {permissions?.battery
                    ? t('byog.client_permission_granted')
                    : t('byog.client_permission_needed')}
                </Badge>
                {!permissions?.battery && (
                  <Button
                    size='sm'
                    onClick={() => bridge.requestBatteryOptimization()}
                  >
                    {t('byog.client_permission_grant')}
                  </Button>
                )}
              </div>
              {permissions?.autostart_supported && (
                <div className='flex items-center gap-3 text-sm'>
                  <Zap className='text-muted-foreground h-4 w-4' />
                  <span className='flex-1'>
                    {t('byog.client_permission_autostart')}
                  </span>
                  <Badge
                    variant={
                      permissions.autostart ? 'default' : 'secondary'
                    }
                  >
                    {permissions.autostart
                      ? t('byog.client_permission_granted')
                      : t('byog.client_permission_needed')}
                  </Badge>
                  <Button size='sm' onClick={() => bridge.openAutoStartSettings()}>
                    {t('byog.client_permission_open')}
                  </Button>
                </div>
              )}
              {permissions?.bgactivity_supported && (
                <div className='flex items-center gap-3 text-sm'>
                  <Activity className='text-muted-foreground h-4 w-4' />
                  <span className='flex-1'>
                    {t('byog.client_permission_background')}
                  </span>
                  <Badge
                    variant={
                      permissions.bgactivity ? 'default' : 'secondary'
                    }
                  >
                    {permissions.bgactivity
                      ? t('byog.client_permission_granted')
                      : t('byog.client_permission_needed')}
                  </Badge>
                  <Button
                    size='sm'
                    onClick={() => bridge.openBackgroundActivitySettings()}
                  >
                    {t('byog.client_permission_open')}
                  </Button>
                </div>
              )}
            </CardContent>
          </Card>
        )}
      </main>
    </div>
  )
}
