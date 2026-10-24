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
  CheckCircle2,
  Cpu,
  Gauge,
  HardDrive,
  Link2,
  Play,
  RefreshCw,
  Server,
  Sliders,
  Terminal,
  Zap,
} from 'lucide-react'

import { LanguageSwitcher } from '@/components/language-switcher'
import { ThemeSwitch } from '@/components/theme-switch'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { cn } from '@/lib/utils'

import { ResourceSliders } from './resource-sliders'
import type { ByogNode } from '../types'

interface WorkerState {
  node_id: string
  device_name: string
  status: string
  model_status: string
  download_status: string
  download_progress: number
  current_tps: number
  models: string
  disks?: Array<{
    drive: string
    label: string
    total_gb: number
    free_gb: number
    allocated_gb?: number
  }>
  physical_ram_mb?: number
  cpu_cores?: number
}

interface PermissionState {
  notifications: boolean
  battery: boolean
  autostart: boolean
  bgactivity: boolean
  autostart_supported: boolean
  bgactivity_supported: boolean
}

interface AndroidBridge {
  getInitialState(): string
  getWorkerStatus(): string
  getPermissionStatus(): string
  isOnboardingDone(): boolean
  completeOnboarding(): void
  requestNotificationPermission(): void
  requestBatteryOptimization(): void
  openAutoStartSettings(): void
  openBackgroundActivitySettings(): void
  installDownloadedUpdate(): void
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
  { key: 'resources', icon: Sliders },
  { key: 'settings', icon: Server },
  { key: 'test', icon: Terminal },
] as const

type PageKey = (typeof PAGES)[number]['key'] | 'permissions'

export function ClientPage() {
  const { t, i18n } = useTranslation()
  const [page, setPage] = useState<PageKey>('status')
  const [nodeId, setNodeId] = useState('')
  const [state, setState] = useState<WorkerState | null>(null)
  const [permissions, setPermissions] = useState<PermissionState | null>(null)
  const [serverUrl, setServerUrl] = useState('')
  const [code, setCode] = useState('')
  const [prompt, setPrompt] = useState('')
  const [answer, setAnswer] = useState('')
  const [message, setMessage] = useState('')
  const [busy, setBusy] = useState(false)
  const [onboardingDone, setOnboardingDone] = useState(true)
  const [onboardingStep, setOnboardingStep] = useState(0)
  const [updateVersion, setUpdateVersion] = useState('')

  // Resource allocations
  const [vramGB, setVramGB] = useState(4)
  const [ramGB, setRamGB] = useState(8)
  const [cores, setCores] = useState(4)
  const [diskGB, setDiskGB] = useState(20)
  const [hotStart, setHotStart] = useState(true)

  const bridge = getBridge()
  const [localApi, setLocalApi] = useState('')
  const [localToken, setLocalToken] = useState('')

  // Sync language changes across app, localStorage, and uninstaller registry
  useEffect(() => {
    const onLangChange = (lng: string) => {
      window.localStorage.setItem('i18nextLng', lng)
      if (localApi) {
        fetch(`${localApi}/settings?token=${localToken}`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ language: lng }),
        }).catch(() => {})
      }
    }
    i18n.on('languageChanged', onLangChange)
    return () => {
      i18n.off('languageChanged', onLangChange)
    }
  }, [i18n, localApi, localToken])

  useEffect(() => {
    const target = window as unknown as Record<string, unknown>
    target.showUpdatePrompt = (version: string) => setUpdateVersion(version)
    return () => {
      delete target.showUpdatePrompt
    }
  }, [])

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
        // bridge fallback
      }
      try {
        const raw = bridge.getPermissionStatus()
        setPermissions(JSON.parse(raw) as PermissionState)
        setOnboardingDone(bridge.isOnboardingDone())
      } catch {
        // ignore
      }
      return
    }

    if (localApi) {
      try {
        const response = await fetch(`${localApi}/state?token=${localToken}`)
        const payload = (await response.json()) as Record<string, unknown>
        if (typeof payload.server_url === 'string' && payload.server_url) {
          setServerUrl(payload.server_url)
        }
        if (typeof payload.node_id === 'string' && payload.node_id) {
          setNodeId(payload.node_id)
        }
        setState({
          node_id: String(payload.node_id ?? ''),
          device_name: String(payload.device_name ?? payload.app_name ?? ''),
          status: payload.connected ? 'online' : 'offline',
          model_status: String(payload.model_status ?? ''),
          download_status: String(payload.model_status ?? ''),
          download_progress: Number(payload.download_progress ?? 0),
          current_tps: Number(payload.current_tps ?? 0),
          models: String(payload.model_name ?? ''),
          disks: payload.disks as WorkerState['disks'],
          physical_ram_mb: payload.physical_ram_mb as number,
          cpu_cores: payload.cpu_cores as number,
        })
      } catch {
        // ignore
      }
      return
    }

    if (!nodeId) {
      return
    }
    try {
      const response = await fetch(`/api/v1/worker/status?node_id=${encodeURIComponent(nodeId)}`)
      const payload = await response.json()
      if (payload.success && payload.data) {
        setState(payload.data)
      }
    } catch {
      // ignore
    }
  }, [bridge, localApi, localToken, nodeId])

  useEffect(() => {
    void loadState()
    const timer = setInterval(() => void loadState(), 4000)
    return () => clearInterval(timer)
  }, [loadState])

  const pair = async () => {
    if (!code.trim()) return
    setBusy(true)
    setMessage('')
    try {
      if (bridge) {
        bridge.connect(code.trim(), serverUrl || window.location.origin)
        setMessage(t('byog.client_paired'))
        return
      }
      if (localApi) {
        const response = await fetch(`${localApi}/pair?token=${localToken}`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ code: code.trim(), server_url: serverUrl }),
        })
        const payload = await response.json()
        if (payload.success) {
          setMessage(t('byog.client_paired'))
          void loadState()
        } else {
          setMessage(payload.message ?? t('byog.client_pair_failed'))
        }
        return
      }
      const response = await fetch('/api/v1/worker/pair', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          node_id: nodeId,
          pairing_code: code.trim(),
        }),
      })
      const payload = await response.json()
      if (payload.success && payload.data?.node_id) {
        window.localStorage.setItem('byog_node_id', payload.data.node_id)
        setNodeId(payload.data.node_id)
        setMessage(t('byog.client_paired'))
        void loadState()
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
        setAnswer(payload.data?.reply ?? payload.data?.content ?? JSON.stringify(payload.data))
      } else {
        setMessage(payload.message ?? t('byog.client_test_failed'))
      }
    } catch {
      setMessage(t('byog.client_test_failed'))
    } finally {
      setBusy(false)
    }
  }

  const statusLabel = () => {
    if (!state) return t('byog.client_offline')
    if (state.status === 'online') return t('byog.client_online')
    if (state.download_progress > 0 && state.download_progress < 100) return t('byog.client_downloading')
    return state.status
  }

  // Simulated node representation for ResourceSliders
  const simulatedNode: ByogNode = {
    id: 1,
    node_id: nodeId || 'local-worker',
    user_id: 1,
    pairing_code: code,
    device_name: state?.device_name || 'Worker Node',
    device_type: bridge ? 'mobile' : 'desktop',
    os: 'Windows / Native',
    arch: 'x64',
    hwid: 'hwid',
    cpu_model: 'Host Processor',
    gpu_model: 'Host GPU',
    npu_model: '',
    physical_ram_mb: state?.physical_ram_mb || 16384,
    swap_ram_mb: 2048,
    vram_mb: 8192,
    shared_vram_mb: 0,
    tee_supported: true,
    tee_type: 'sgx',
    bandwidth_down_mbps: 100,
    bandwidth_up_mbps: 50,
    ping_ms: 20,
    network_type: 'ethernet',
    ip_address: '127.0.0.1',
    ip_type: 'residential',
    country: '',
    city: '',
    latitude: 0,
    longitude: 0,
    status: (state?.status === 'online' ? 'online' : 'offline') as any,
    operating_mode: 'mode_b',
    power_profile: 'full_power',
    schedule_config: '',
    allocated_disk_gb: diskGB,
    total_disk_gb: 120,
    free_disk_gb: state?.disks?.[0]?.free_gb || 40,
    disks: state?.disks,
    allocated_vram_gb: vramGB,
    allocated_ram_gb: ramGB,
    allocated_cores: cores,
    hot_start_enabled: hotStart,
    wifi_only_download: false,
    monthly_data_limit_mb: 0,
    monthly_data_used_mb: 0,
    run_only_while_charging: false,
    battery_cutoff_enabled: false,
    battery_cutoff_percent: 20,
    max_battery_temp: 45,
    battery_temp_supported: false,
    cpu_temp: 40,
    gpu_temp: 42,
    battery_temp: 30,
  }

  const pages = bridge ? [...PAGES, { key: 'permissions' as PageKey, icon: Zap }] : PAGES

  return (
    <div className='flex h-screen w-full flex-col bg-background text-foreground antialiased selection:bg-primary/20'>
      {/* Top Header matching Console layout */}
      <header className='flex h-14 shrink-0 items-center justify-between border-b border-border bg-card px-4'>
        <div className='flex items-center gap-3'>
          <div className='flex items-center gap-2 font-bold text-foreground'>
            <div className='flex h-8 w-8 items-center justify-center rounded-lg bg-primary text-primary-foreground font-black'>
              B
            </div>
            <span className='text-sm sm:text-base font-semibold tracking-tight'>
              {state?.device_name || 'BYOG Worker'}
            </span>
          </div>
          <Badge
            variant={state?.status === 'online' ? 'default' : 'secondary'}
            className='text-[11px] font-semibold'
          >
            {statusLabel()}
          </Badge>
        </div>

        <div className='flex items-center gap-2'>
          <LanguageSwitcher />
          <ThemeSwitch />
        </div>
      </header>

      {/* Main Container: Sidebar + Content */}
      <div className='flex min-h-0 flex-1 overflow-hidden'>
        {/* Left Sidebar matching AppSidebar style */}
        <aside className='flex w-56 shrink-0 flex-col border-r border-border bg-sidebar text-sidebar-foreground p-3 gap-1'>
          <div className='px-3 py-1.5 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground'>
            {t('byog.client_navigation', 'Menu')}
          </div>
          {pages.map((item) => {
            const Icon = item.icon
            const active = page === item.key
            return (
              <button
                key={item.key}
                type='button'
                onClick={() => setPage(item.key)}
                className={cn(
                  'flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors text-left',
                  active
                    ? 'bg-sidebar-accent text-sidebar-accent-foreground font-semibold shadow-xs'
                    : 'text-sidebar-foreground/70 hover:bg-sidebar-accent/50 hover:text-sidebar-foreground'
                )}
              >
                <Icon className={cn('h-4 w-4', active ? 'text-primary' : 'text-muted-foreground')} />
                <span>
                  {item.key === 'permissions'
                    ? t('byog.client_permissions', 'İzinler')
                    : item.key === 'resources'
                    ? t('byog.resources_tab', 'Kaynaklar & Donanım')
                    : t(`byog.client_${item.key}`, item.key)}
                </span>
              </button>
            )
          })}

          <div className='mt-auto border-t border-border/50 pt-3 px-3 text-xs text-muted-foreground'>
            <div className='font-mono font-medium truncate'>{state?.device_name || nodeId || 'BYOG Worker'}</div>
            <div className='text-[10px] opacity-70'>v1.9.1 • Local Node</div>
          </div>
        </aside>

        {/* Content Area */}
        <main className='flex-1 overflow-y-auto bg-background p-6'>
          {updateVersion && (
            <div className='mb-4 flex items-center justify-between gap-3 rounded-xl border border-primary/20 bg-primary/10 px-4 py-2.5 text-sm'>
              <span>{t('byog.client_update_available', { version: updateVersion })}</span>
              <Button size='sm' onClick={() => bridge?.installDownloadedUpdate()}>
                {t('byog.client_update_install', 'Güncelle')}
              </Button>
            </div>
          )}

          {page === 'status' && (
            <div className='space-y-4 max-w-4xl'>
              <Card>
                <CardHeader>
                  <CardTitle className='flex items-center justify-between text-base sm:text-lg'>
                    <span>{t('byog.client_status', 'İstemci Durumu')}</span>
                    <Button variant='ghost' size='icon' onClick={() => void loadState()}>
                      <RefreshCw className='h-4 w-4' />
                    </Button>
                  </CardTitle>
                  <CardDescription>
                    {t('byog.client_status_desc', 'Cihazınızın ağ bağlantısı ve model yükleme bilgileri.')}
                  </CardDescription>
                </CardHeader>
                <CardContent className='grid gap-4 text-sm'>
                  <div className='flex items-center gap-3'>
                    <Server className='text-muted-foreground h-4 w-4' />
                    <span className='text-muted-foreground w-28'>{t('byog.client_server', 'Sunucu')}:</span>
                    <span className='font-mono truncate'>{serverUrl || window.location.origin}</span>
                  </div>
                  <div className='flex items-center gap-3'>
                    <Cpu className='text-muted-foreground h-4 w-4' />
                    <span className='text-muted-foreground w-28'>{t('byog.client_model', 'Model')}:</span>
                    <span className='font-mono truncate'>{state?.models || '-'}</span>
                  </div>
                  <div className='flex items-center gap-3'>
                    <Gauge className='text-muted-foreground h-4 w-4' />
                    <span className='text-muted-foreground w-28'>{t('byog.client_speed', 'Hız')}:</span>
                    <span className='font-mono font-semibold'>{(state?.current_tps ?? 0).toFixed(1)} tps</span>
                  </div>
                  {state && state.download_progress > 0 && state.download_progress < 100 && (
                    <div className='space-y-1.5 pt-2'>
                      <div className='flex justify-between text-xs text-muted-foreground'>
                        <span>{t('byog.downloading_model', 'Model İndiriliyor...')}</span>
                        <span className='font-mono font-bold'>{state.download_progress}%</span>
                      </div>
                      <div className='bg-muted h-2 w-full overflow-hidden rounded-full'>
                        <div
                          className='h-full rounded-full bg-primary transition-all duration-300'
                          style={{ width: `${state.download_progress}%` }}
                        />
                      </div>
                    </div>
                  )}
                </CardContent>
              </Card>

              {/* Quick Resources Summary */}
              <ResourceSliders
                node={simulatedNode}
                vramGB={vramGB}
                ramGB={ramGB}
                cores={cores}
                diskGB={diskGB}
                hotStartEnabled={hotStart}
                onVramChange={setVramGB}
                onRamChange={setRamGB}
                onCoresChange={setCores}
                onDiskChange={setDiskGB}
                onHotStartChange={setHotStart}
              />
            </div>
          )}

          {page === 'pairing' && (
            <Card className='max-w-xl'>
              <CardHeader>
                <CardTitle className='text-base sm:text-lg'>{t('byog.client_pairing', 'Cihaz Eşleştirme')}</CardTitle>
                <CardDescription>
                  {t('byog.enter_pairing_code_instructions', 'Web panelinden aldığınız eşleştirme kodunu girin.')}
                </CardDescription>
              </CardHeader>
              <CardContent className='space-y-4'>
                <div className='flex gap-2'>
                  <Input
                    value={code}
                    onChange={(event) => setCode(event.target.value)}
                    placeholder={t('byog.enter_pairing_code_label', 'Eşleştirme Kodu')}
                    className='font-mono uppercase tracking-wider'
                  />
                  <Button onClick={pair} disabled={busy || !code.trim()}>
                    {t('byog.client_connect', 'Bağlan')}
                  </Button>
                </div>
                {message && <p className='text-sm text-primary font-medium'>{message}</p>}
              </CardContent>
            </Card>
          )}

          {page === 'resources' && (
            <div className='max-w-4xl'>
              <ResourceSliders
                node={simulatedNode}
                vramGB={vramGB}
                ramGB={ramGB}
                cores={cores}
                diskGB={diskGB}
                hotStartEnabled={hotStart}
                onVramChange={setVramGB}
                onRamChange={setRamGB}
                onCoresChange={setCores}
                onDiskChange={setDiskGB}
                onHotStartChange={setHotStart}
              />
            </div>
          )}

          {page === 'settings' && (
            <Card className='max-w-xl'>
              <CardHeader>
                <CardTitle className='text-base sm:text-lg'>{t('byog.client_settings', 'İstemci Ayarları')}</CardTitle>
                <CardDescription>
                  {t('byog.client_settings_desc', 'Yerel yapılandırma ve sunucu adresi.')}
                </CardDescription>
              </CardHeader>
              <CardContent className='grid gap-4 text-sm'>
                <div className='flex items-center justify-between border-b border-border/50 pb-3'>
                  <span className='text-muted-foreground'>{t('byog.client_server', 'Sunucu Adresi')}</span>
                  <span className='font-mono font-medium truncate'>{serverUrl || window.location.origin}</span>
                </div>
                <div className='flex items-center justify-between border-b border-border/50 pb-3'>
                  <span className='text-muted-foreground'>{t('byog.client_node', 'Düğüm Kimliği')}</span>
                  <span className='font-mono font-medium truncate'>{nodeId || '-'}</span>
                </div>
                <div className='flex items-center justify-between'>
                  <span className='text-muted-foreground'>{t('byog.client_language', 'Uygulama Dili')}</span>
                  <LanguageSwitcher />
                </div>
              </CardContent>
            </Card>
          )}

          {page === 'test' && (
            <Card className='max-w-2xl'>
              <CardHeader>
                <CardTitle className='text-base sm:text-lg'>{t('byog.client_test', 'Model Testi')}</CardTitle>
                <CardDescription>
                  {t('byog.client_test_desc', 'Yerel modelinize doğrudan soru sorarak yanıt hızını ve çıktıyı test edin.')}
                </CardDescription>
              </CardHeader>
              <CardContent className='space-y-4'>
                <Input
                  value={prompt}
                  onChange={(event) => setPrompt(event.target.value)}
                  placeholder={t('byog.client_test_prompt', 'Test için bir mesaj yazın...')}
                />
                <Button onClick={runTest} disabled={busy || !prompt.trim()}>
                  <Play className='mr-2 h-4 w-4' />
                  {t('byog.client_run_test', 'Çalıştır')}
                </Button>
                {answer && (
                  <div className='space-y-1.5'>
                    <span className='text-xs font-semibold text-muted-foreground'>
                      {t('byog.model_output', 'Model Yanıtı:')}
                    </span>
                    <pre className='bg-muted max-h-72 overflow-auto rounded-xl p-3.5 text-sm font-mono whitespace-pre-wrap border border-border/60'>
                      {answer}
                    </pre>
                  </div>
                )}
                {message && <p className='text-sm text-destructive font-medium'>{message}</p>}
              </CardContent>
            </Card>
          )}

          {page === 'permissions' && bridge && (
            <Card className='max-w-xl'>
              <CardHeader>
                <CardTitle className='text-base sm:text-lg'>{t('byog.client_permissions', 'Mobil İzinler')}</CardTitle>
                <CardDescription>
                  {t('byog.client_permissions_desc', 'Arka planda kesintisiz çalışabilmesi için gerekli izinler.')}
                </CardDescription>
              </CardHeader>
              <CardContent className='grid gap-4'>
                <div className='flex items-center justify-between border-b border-border/50 pb-3'>
                  <div className='flex items-center gap-2'>
                    <Bell className='h-4 w-4 text-muted-foreground' />
                    <span>{t('byog.perm_notifications', 'Bildirimler')}</span>
                  </div>
                  {permissions?.notifications ? (
                    <Badge variant='outline' className='text-emerald-500 border-emerald-500/30 gap-1'>
                      <CheckCircle2 className='h-3 w-3' /> {t('byog.granted', 'Verildi')}
                    </Badge>
                  ) : (
                    <Button size='sm' variant='outline' onClick={() => bridge.requestNotificationPermission()}>
                      {t('byog.grant', 'İzin Ver')}
                    </Button>
                  )}
                </div>
                <div className='flex items-center justify-between'>
                  <div className='flex items-center gap-2'>
                    <Battery className='h-4 w-4 text-muted-foreground' />
                    <span>{t('byog.perm_battery', 'Pil Optimizasyonu Muafiyeti')}</span>
                  </div>
                  {permissions?.battery ? (
                    <Badge variant='outline' className='text-emerald-500 border-emerald-500/30 gap-1'>
                      <CheckCircle2 className='h-3 w-3' /> {t('byog.granted', 'Verildi')}
                    </Badge>
                  ) : (
                    <Button size='sm' variant='outline' onClick={() => bridge.requestBatteryOptimization()}>
                      {t('byog.grant', 'İzin Ver')}
                    </Button>
                  )}
                </div>
              </CardContent>
            </Card>
          )}
        </main>
      </div>
    </div>
  )
}
