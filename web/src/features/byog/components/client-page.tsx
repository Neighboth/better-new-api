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
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  Activity,
  AlertTriangle,
  Battery,
  Bell,
  CheckCircle2,
  ChevronRight,
  Cpu,
  Gauge,
  Link2,
  LogOut,
  Play,
  QrCode,
  RefreshCw,
  Server,
  Sliders,
  Terminal,
  Zap,
} from 'lucide-react'

import { LanguageSwitcher } from '@/components/language-switcher'
import { ThemeSwitch } from '@/components/theme-switch'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarInset,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarProvider,
  SidebarRail,
  SidebarTrigger,
} from '@/components/ui/sidebar'
import { useSystemConfig } from '@/hooks/use-system-config'
import { cn } from '@/lib/utils'

import { ResourceSliders } from './resource-sliders'
import type { ByogNode } from '../types'

interface DiskInfo {
  drive: string
  label: string
  total_gb: number
  free_gb: number
  allocated_gb?: number
}

interface WorkerState {
  node_id: string
  device_name: string
  status: string
  model_status: string
  download_status: string
  download_progress: number
  current_tps: number
  last_ttft_ms?: number
  models: string
  assigned_model?: string
  disks?: DiskInfo[]
  physical_ram_mb?: number
  cpu_cores?: number
  favicon?: string
  favicon_base64?: string
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
  disconnect(): void
  unpair(): void
  openQRScanner(): void
  scanQrCode(): void
  isSystemDarkMode(): boolean
}

function getBridge(): AndroidBridge | null {
  if (typeof window === 'undefined') {
    return null
  }
  const bridge = (window as unknown as Record<string, unknown>).AndroidBridge
  return bridge ? (bridge as AndroidBridge) : null
}

const PAGES = [
  { key: 'status', labelKey: 'byog.client_status', fallback: 'Durum', icon: Activity },
  { key: 'resources', labelKey: 'byog.resources_tab', fallback: 'Kaynaklar & Donanım', icon: Sliders },
  { key: 'test', labelKey: 'byog.client_test', fallback: 'Test', icon: Terminal },
] as const

type PageKey = (typeof PAGES)[number]['key']

export function ClientPage() {
  const { t, i18n } = useTranslation()
  const { logo: systemLogo, systemName } = useSystemConfig()
  const [page, setPage] = useState<PageKey>(() => {
    if (typeof window !== 'undefined') {
      const tab = new URLSearchParams(window.location.search).get('tab')
      if (tab === 'resources' || tab === 'test') return tab
    }
    return 'status'
  })
  const [nodeId, setNodeId] = useState('')
  const [state, setState] = useState<WorkerState | null>(null)
  const [permissions, setPermissions] = useState<PermissionState | null>(null)
  const [serverUrl, setServerUrl] = useState('')
  const [code, setCode] = useState('')
  const [prompt, setPrompt] = useState('')
  const [answer, setAnswer] = useState('')
  const [message, setMessage] = useState('')
  const [busy, setBusy] = useState(false)
  const [isRefreshing, setIsRefreshing] = useState(false)
  const [onboardingDone, setOnboardingDone] = useState(() => {
    if (typeof window !== 'undefined' && new URLSearchParams(window.location.search).get('onboarding') === '1') {
      return false
    }
    return true
  })
  const [onboardingStep, setOnboardingStep] = useState(0)
  const [updateVersion, setUpdateVersion] = useState('')
  const [disconnectDialogOpen, setDisconnectDialogOpen] = useState(() => {
    if (typeof window !== 'undefined' && new URLSearchParams(window.location.search).get('disconnect') === '1') {
      return true
    }
    return false
  })

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
        const isPaired = Boolean(initial.is_paired && initial.node_id)
        setState({
          node_id: String(initial.node_id ?? ''),
          device_name: String(initial.app_name ?? ''),
          status: isPaired ? 'online' : 'offline',
          model_status: String(initial.model_status ?? ''),
          download_status: String(initial.model_status ?? ''),
          download_progress: Number(initial.download_progress ?? 0),
          current_tps: Number(initial.current_tps ?? 0),
          models: String(initial.assigned_model ?? ''),
          assigned_model: String(initial.assigned_model ?? ''),
          disks: initial.disks as WorkerState['disks'],
          physical_ram_mb: initial.physical_ram_mb as number,
          cpu_cores: initial.cpu_cores as number,
          favicon: initial.favicon as string,
          favicon_base64: initial.favicon_base64 as string,
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
    const target = window as unknown as Record<string, unknown>
    target.showUpdatePrompt = (version: string) => setUpdateVersion(version)
    target.handleScannedQr = (qrText: string) => {
      if (qrText) {
        setCode(qrText.trim().toUpperCase())
      }
    }
    target.onPairSuccess = () => {
      setBusy(false)
      setMessage(t('byog.client_paired', 'Cihaz başarıyla eşleştirildi'))
      void loadState()
    }
    target.onPairError = (errMsg?: string) => {
      setBusy(false)
      setMessage(errMsg || t('byog.client_pair_failed', 'Eşleştirme başarısız oldu'))
    }
    return () => {
      delete target.showUpdatePrompt
      delete target.handleScannedQr
      delete target.onPairSuccess
      delete target.onPairError
    }
  }, [t, loadState])

  useEffect(() => {
    const search = new URLSearchParams(window.location.search)
    const fromQuery = search.get('node') ?? ''
    const stored = window.localStorage.getItem('byog_node_id') ?? ''
    setNodeId(fromQuery || stored)
    setServerUrl(window.location.origin)
    setLocalApi(search.get('local') ?? '')
    setLocalToken(search.get('token') ?? '')
    if (search.get('onboarding') === '1') {
      setOnboardingDone(false)
    }
  }, [])

  useEffect(() => {
    void loadState()
    const timer = setInterval(() => void loadState(), 4000)
    return () => clearInterval(timer)
  }, [loadState])

  const handleManualRefresh = async () => {
    setIsRefreshing(true)
    await loadState()
    setTimeout(() => setIsRefreshing(false), 600)
  }

  const pair = async () => {
    if (!code.trim()) return
    setBusy(true)
    setMessage('')
    try {
      if (bridge) {
        bridge.connect(code.trim(), serverUrl || window.location.origin)
        setTimeout(() => {
          setBusy(false)
        }, 8000)
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
          setMessage(t('byog.client_paired', 'Cihaz başarıyla eşleştirildi'))
          void loadState()
        } else {
          setMessage(payload.message ?? t('byog.client_pair_failed', 'Eşleştirme başarısız oldu'))
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
        setMessage(t('byog.client_paired', 'Cihaz başarıyla eşleştirildi'))
        void loadState()
      } else {
        setMessage(payload.message ?? t('byog.client_pair_failed', 'Eşleştirme başarısız oldu'))
      }
    } catch {
      setMessage(t('byog.client_pair_failed', 'Eşleştirme başarısız oldu'))
    } finally {
      if (!bridge) {
        setBusy(false)
      }
    }
  }

  const disconnectDevice = async () => {
    setBusy(true)
    try {
      if (bridge) {
        bridge.disconnect()
      } else if (localApi) {
        await fetch(`${localApi}/pair?token=${localToken}`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ code: '', server_url: '' }),
        }).catch(() => {})
      }
      window.localStorage.removeItem('byog_node_id')
      setNodeId('')
      setState(null)
      setCode('')
      setMessage('')
      setPage('status')
    } catch {
      // ignore
    } finally {
      setBusy(false)
      setDisconnectDialogOpen(false)
      void loadState()
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
        setMessage(payload.message ?? t('byog.client_test_failed', 'Test başarısız oldu'))
      }
    } catch {
      setMessage(t('byog.client_test_failed', 'Test başarısız oldu'))
    } finally {
      setBusy(false)
    }
  }

  const statusLabel = () => {
    if (!state) return t('byog.client_offline', 'Çevrimdışı')
    if (state.status === 'online') return t('byog.client_online', 'Çevrimiçi')
    if (state.download_progress > 0 && state.download_progress < 100) return t('byog.client_downloading', 'İndiriliyor')
    return state.status
  }

  // Calculate accurate total free disk space across all available storage volumes
  const totalFreeDiskGB = useMemo(() => {
    if (state?.disks && state.disks.length > 0) {
      return state.disks.reduce((sum, d) => sum + (d.free_gb || 0), 0)
    }
    return 40
  }, [state?.disks])

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
    total_disk_gb: state?.disks?.reduce((s, d) => s + (d.total_gb || 0), 0) || 120,
    free_disk_gb: totalFreeDiskGB,
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
  } as unknown as ByogNode

  // Determine paired status
  const isPaired = Boolean((nodeId && state?.status === 'online') || (bridge ? (state?.status === 'online' && nodeId) : nodeId))
  const isModelReady = state?.model_status === 'ready' || (state && state.download_progress >= 100)

  // Header Brand Logo: use custom site logo/favicon or favicon_base64
  const brandLogo = useMemo(() => {
    if (state?.favicon_base64) {
      return state.favicon_base64.startsWith('data:')
        ? state.favicon_base64
        : `data:image/png;base64,${state.favicon_base64}`
    }
    if (state?.favicon) {
      return state.favicon
    }
    if (systemLogo) {
      return systemLogo
    }
    return '/logo.png'
  }, [state?.favicon, state?.favicon_base64, systemLogo])

  // --- VIEW 1: Onboarding / Permissions (Mobile only, shown sequentially before pairing) ---
  const isTestingOnboarding = typeof window !== 'undefined' && new URLSearchParams(window.location.search).get('onboarding') === '1'
  if ((bridge && !onboardingDone) || (isTestingOnboarding && !onboardingDone)) {
    const handleNextStep = () => {
      if (onboardingStep === 0) {
        setOnboardingStep(1)
      } else if (onboardingStep === 1) {
        if (permissions?.autostart_supported) {
          setOnboardingStep(2)
        } else {
          bridge?.completeOnboarding()
          setOnboardingDone(true)
        }
      } else {
        bridge?.completeOnboarding()
        setOnboardingDone(true)
      }
    }

    return (
      <div className='flex h-screen w-full flex-col bg-background text-foreground antialiased'>
      <header className='flex h-14 shrink-0 items-center justify-between border-b border-border bg-card px-4'>
        <div className='flex items-center gap-2 min-w-0'>
          <img
            src={brandLogo}
            alt='Logo'
            className='h-7 w-7 rounded-lg object-contain shrink-0'
            onError={(e) => {
              ;(e.currentTarget as HTMLElement).style.display = 'none'
            }}
          />
          <span className='text-sm font-semibold tracking-tight truncate max-w-[160px] sm:max-w-none'>
            {state?.device_name || systemName || 'BYOG Worker'}
          </span>
        </div>
        <div className='flex items-center gap-1.5 shrink-0'>
          <LanguageSwitcher />
          <ThemeSwitch />
        </div>
      </header>

        <div className='flex flex-1 items-center justify-center p-4 sm:p-6'>
          <Card className='w-full max-w-lg border-border/80 shadow-lg'>
            <CardHeader className='text-center pb-4'>
              <div className='mx-auto mb-3 flex size-12 items-center justify-center rounded-2xl bg-primary/10 text-primary'>
                <Zap className='size-6' />
              </div>
              <CardTitle className='text-xl sm:text-2xl font-bold'>
                {t('byog.permissions_setup_title', 'Cihaz İzinleri')}
              </CardTitle>
              <CardDescription className='text-sm sm:text-base'>
                {t(
                  'byog.permissions_setup_desc',
                  'BYOG Worker düğümünün arka planda yapay zeka çıkarımı yapabilmesi için aşağıdaki sistem izinleri gereklidir.'
                )}
              </CardDescription>
            </CardHeader>

            <CardContent className='space-y-6'>
              {/* Step 0: Notifications */}
              {onboardingStep === 0 && (
                <div className='space-y-4 rounded-xl border border-border/60 bg-muted/40 p-5'>
                  <div className='flex items-center gap-3'>
                    <div className='flex size-10 items-center justify-center rounded-xl bg-primary/15 text-primary'>
                      <Bell className='size-5' />
                    </div>
                    <div>
                      <h4 className='font-semibold text-foreground'>
                        {t('byog.perm_notif_title', '1. Bildirim İzni')}
                      </h4>
                      <p className='text-xs text-muted-foreground'>
                        {t(
                          'byog.perm_notif_desc',
                          'Arka plan servisinin işletim sistemi tarafından sonlandırılmasını önlemek ve model indirme ilerlemesini bildirmek için gereklidir.'
                        )}
                      </p>
                    </div>
                  </div>

                  <div className='pt-2 flex flex-wrap items-center justify-between gap-2'>
                    {permissions?.notifications ? (
                      <Badge variant='outline' className='text-emerald-500 border-emerald-500/30 gap-1.5 py-1 px-3'>
                        <CheckCircle2 className='size-4' /> {t('byog.granted', 'İzin Verildi')}
                      </Badge>
                    ) : (
                      <Button
                        size='sm'
                        onClick={() => {
                          bridge?.requestNotificationPermission()
                          setTimeout(() => void loadState(), 1500)
                        }}
                      >
                        {t('byog.grant_notification', 'Bildirimlere İzin Ver')}
                      </Button>
                    )}

                    <Button onClick={handleNextStep} disabled={!permissions?.notifications && !isTestingOnboarding}>
                      {t('byog.continue', 'Devam Et')} <ChevronRight className='size-4 ml-1' />
                    </Button>
                  </div>
                </div>
              )}

              {/* Step 1: Battery Optimization */}
              {onboardingStep === 1 && (
                <div className='space-y-4 rounded-xl border border-border/60 bg-muted/40 p-5'>
                  <div className='flex items-center gap-3'>
                    <div className='flex size-10 items-center justify-center rounded-xl bg-amber-500/15 text-amber-500'>
                      <Battery className='size-5' />
                    </div>
                    <div>
                      <h4 className='font-semibold text-foreground'>
                        {t('byog.perm_battery_title', '2. Pil Optimizasyonu Muafiyeti')}
                      </h4>
                      <p className='text-xs text-muted-foreground'>
                        {t(
                          'byog.perm_battery_desc',
                          'Cihaz ekranı kapalıyken çıkarım işlemlerinin kesilmemesi için pil tasarrufu kısıtlamasından muaf tutulması zorunludur.'
                        )}
                      </p>
                    </div>
                  </div>

                  <div className='pt-2 flex flex-wrap items-center justify-between gap-2'>
                    {permissions?.battery ? (
                      <Badge variant='outline' className='text-emerald-500 border-emerald-500/30 gap-1.5 py-1 px-3'>
                        <CheckCircle2 className='size-4' /> {t('byog.granted', 'İzin Verildi')}
                      </Badge>
                    ) : (
                      <Button
                        size='sm'
                        onClick={() => {
                          bridge?.requestBatteryOptimization()
                          setTimeout(() => void loadState(), 1500)
                        }}
                      >
                        {t('byog.grant_battery', 'Muafiyet İste')}
                      </Button>
                    )}

                    <Button onClick={handleNextStep} disabled={!permissions?.battery && !isTestingOnboarding}>
                      {t('byog.continue', 'Devam Et')} <ChevronRight className='size-4 ml-1' />
                    </Button>
                  </div>
                </div>
              )}

              {/* Step 2: Autostart (Xiaomi / HyperOS, Huawei, Honor, Samsung, Oppo, Vivo, Transsion, etc.) */}
              {onboardingStep === 2 && (
                <div className='space-y-4 rounded-xl border border-border/60 bg-muted/40 p-5'>
                  <div className='flex items-center gap-3'>
                    <div className='flex size-10 items-center justify-center rounded-xl bg-indigo-500/15 text-indigo-500'>
                      <Zap className='size-5' />
                    </div>
                    <div>
                      <h4 className='font-semibold text-foreground'>
                        {t('byog.perm_autostart_title', '3. Otomatik Başlatma & Arka Plan')}
                      </h4>
                      <p className='text-xs text-muted-foreground'>
                        {t(
                          'byog.perm_autostart_desc',
                          'Cihaz yeniden başlatıldığında veya ekran kapandığında düğümün arka planda kesintisiz çalışabilmesi için üreticinizin sistem yöneticisinden Otomatik Başlatma veya Kısıtlanmamış Pil kullanımını açın.'
                        )}
                      </p>
                    </div>
                  </div>

                  <div className='pt-2 flex flex-wrap items-center justify-between gap-2'>
                    <Button
                      size='sm'
                      variant='outline'
                      onClick={() => {
                        bridge?.openAutoStartSettings()
                        setTimeout(() => void loadState(), 1500)
                      }}
                    >
                      {t('byog.open_settings', 'Ayarları Aç')}
                    </Button>

                    <Button onClick={handleNextStep}>
                      {t('byog.complete_setup', 'Kurulumu Tamamla')} <ChevronRight className='size-4 ml-1' />
                    </Button>
                  </div>
                </div>
              )}
            </CardContent>
          </Card>
        </div>
      </div>
    )
  }

  // --- VIEW 2: Unpaired Screen (Centered pairing view without sidebar) ---
  if (!isPaired) {
    return (
      <div className='flex h-screen w-full flex-col bg-background text-foreground antialiased'>
      <header className='flex h-14 shrink-0 items-center justify-between border-b border-border bg-card px-4'>
        <div className='flex items-center gap-2 min-w-0'>
          <img
            src={brandLogo}
            alt='Logo'
            className='h-7 w-7 rounded-lg object-contain shrink-0'
            onError={(e) => {
              ;(e.currentTarget as HTMLElement).style.display = 'none'
            }}
          />
          <span className='text-sm font-semibold tracking-tight truncate max-w-[160px] sm:max-w-none'>
            {state?.device_name || systemName || 'BYOG Worker'}
          </span>
        </div>
        <div className='flex items-center gap-1.5 shrink-0'>
          <LanguageSwitcher />
          <ThemeSwitch />
        </div>
      </header>

        <div className='flex flex-1 items-center justify-center p-4 sm:p-6'>
          <Card className='w-full max-w-md border-border/80 shadow-xl'>
            <CardHeader className='text-center pb-4'>
              <div className='mx-auto mb-3 flex size-12 items-center justify-center rounded-2xl bg-primary/10 text-primary'>
                <Link2 className='size-6' />
              </div>
              <CardTitle className='text-xl sm:text-2xl font-bold'>
                {t('byog.client_pairing', 'Cihaz Eşleştirme')}
              </CardTitle>
              <CardDescription className='text-sm sm:text-base'>
                {t(
                  'byog.enter_pairing_code_instructions',
                  'Web konsolundan aldığınız eşleştirme kodunu (XXXX-XXXX-XXXX-XXXX) girin veya kamerayı açarak QR kodu taratın.'
                )}
              </CardDescription>
            </CardHeader>

            <CardContent className='space-y-4'>
              <div className='flex items-center gap-2'>
                {bridge && (
                  <Button
                    type='button'
                    variant='outline'
                    size='icon'
                    onClick={() => bridge.scanQrCode()}
                    title={t('byog.scan_qr', 'QR Kod Tara')}
                    className='shrink-0 h-11 w-11 rounded-lg border-primary/30 text-primary hover:bg-primary/10'
                  >
                    <QrCode className='size-5' />
                  </Button>
                )}

                <Input
                  value={code}
                  onChange={(event) => {
                    let val = event.target.value.toUpperCase().replaceAll(/[^A-Z0-9-]/g, '')
                    const raw = val.replaceAll('-', '')
                    if (raw.length === 16 && !val.includes('-')) {
                      val = raw.match(/.{1,4}/g)?.join('-') ?? val
                    }
                    setCode(val)
                  }}
                  placeholder={t('byog.enter_pairing_code_label', 'EŞLEŞTİRME KODU (XXXX-XXXX-XXXX-XXXX)')}
                  className='h-11 font-mono uppercase tracking-widest text-center text-sm sm:text-base font-bold min-w-0 flex-1'
                  maxLength={32}
                  disabled={busy}
                />

                <Button
                  onClick={pair}
                  disabled={busy || !code.trim()}
                  className='shrink-0 h-11 px-5 font-semibold'
                >
                  {busy ? <RefreshCw className='size-4 animate-spin' /> : t('byog.client_connect', 'Eşleştir')}
                </Button>
              </div>

              {message && (
                <div
                  className={cn(
                    'rounded-lg p-3 text-center text-sm font-medium',
                    message.includes('başarı') || message.includes('paired')
                      ? 'bg-emerald-500/10 text-emerald-500 border border-emerald-500/20'
                      : 'bg-destructive/10 text-destructive border border-destructive/20'
                  )}
                >
                  {message}
                </div>
              )}
            </CardContent>
          </Card>
        </div>
      </div>
    )
  }

  // --- VIEW 3: Main Application Screen (Paired) ---
  return (
    <SidebarProvider defaultOpen={true} className='flex-col h-screen w-full bg-background text-foreground antialiased selection:bg-primary/20'>
      {/* Top Header */}
      <header className='flex h-14 shrink-0 items-center justify-between border-b border-border bg-card px-4'>
        <div className='flex items-center gap-2 min-w-0'>
          <SidebarTrigger variant='ghost' className='size-8 md:hidden shrink-0' />
          <div className='flex items-center gap-2 font-bold text-foreground min-w-0'>
            <img
              src={brandLogo}
              alt='Logo'
              className='h-7 w-7 rounded-lg object-contain shrink-0'
              onError={(e) => {
                ;(e.currentTarget as HTMLElement).style.display = 'none'
              }}
            />
            <span className='text-sm font-semibold tracking-tight truncate max-w-[120px] sm:max-w-none'>
              {state?.device_name || systemName || 'BYOG Worker'}
            </span>
          </div>
          <Badge
            variant={state?.status === 'online' ? 'default' : 'secondary'}
            className='text-[10px] font-semibold hidden sm:inline-flex shrink-0'
          >
            {statusLabel()}
          </Badge>
        </div>

        <div className='flex items-center gap-1.5 shrink-0'>
          <LanguageSwitcher />
          <ThemeSwitch />
        </div>
      </header>

      {/* Main Container: Left Sidebar + Content */}
      <div className='flex min-h-0 w-full flex-1 overflow-hidden'>
        <Sidebar collapsible='icon' className='top-14 h-[calc(100vh-3.5rem)] border-r border-border'>
          <SidebarContent className='py-2'>
            <SidebarGroup>
              <SidebarGroupLabel className='px-3 py-1.5 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground'>
                {t('byog.client_navigation', 'Menü')}
              </SidebarGroupLabel>
              <SidebarGroupContent>
                <SidebarMenu>
                  {PAGES.map((item) => {
                    const Icon = item.icon
                    const active = page === item.key
                    return (
                      <SidebarMenuItem key={item.key}>
                        <SidebarMenuButton
                          isActive={active}
                          onClick={() => setPage(item.key)}
                          tooltip={t(item.labelKey, item.fallback)}
                          className={cn(
                            'flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors cursor-pointer',
                            active
                              ? 'bg-sidebar-accent text-sidebar-accent-foreground font-semibold shadow-xs'
                              : 'text-sidebar-foreground/70 hover:bg-sidebar-accent/50 hover:text-sidebar-foreground'
                          )}
                        >
                          <Icon className={cn('h-4 w-4', active ? 'text-primary' : 'text-muted-foreground')} />
                          <span>{t(item.labelKey, item.fallback)}</span>
                        </SidebarMenuButton>
                      </SidebarMenuItem>
                    )
                  })}
                </SidebarMenu>
              </SidebarGroupContent>
            </SidebarGroup>
          </SidebarContent>

          <SidebarFooter className='p-3 border-t border-border/50'>
            <button
              type='button'
              onClick={() => setDisconnectDialogOpen(true)}
              className='flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-sm font-medium text-destructive hover:bg-destructive/10 transition-colors text-left cursor-pointer'
            >
              <LogOut className='size-4' />
              <span>{t('byog.disconnect_device', 'Bağlantıyı Kes')}</span>
            </button>

            <div className='px-3 pt-2 text-xs text-muted-foreground border-t border-border/30'>
              <div className='font-mono font-medium truncate'>{state?.device_name || nodeId || 'BYOG Worker'}</div>
              <div className='text-[10px] opacity-70'>v1.9.2 • Local Node</div>
            </div>
          </SidebarFooter>
          <SidebarRail />
        </Sidebar>

        {/* Content Area */}
        <SidebarInset className='flex-1 overflow-y-auto bg-background p-4 sm:p-6'>
          {updateVersion && (
            <div className='mb-4 flex items-center justify-between gap-3 rounded-xl border border-primary/20 bg-primary/10 px-4 py-2.5 text-sm'>
              <span>{t('byog.client_update_available', { version: updateVersion })}</span>
              <Button size='sm' onClick={() => bridge?.installDownloadedUpdate()}>
                {t('byog.client_update_install', 'Güncelle')}
              </Button>
            </div>
          )}

          {/* TAB 1: Durum (Status) - Single neat card without duplicate sliders */}
          {page === 'status' && (
            <div className='space-y-4 max-w-3xl'>
              <Card>
                <CardHeader>
                  <CardTitle className='flex items-center justify-between text-base sm:text-lg'>
                    <span>{t('byog.client_status', 'İstemci Durumu')}</span>
                    <Button
                      variant='ghost'
                      size='icon'
                      onClick={handleManualRefresh}
                      disabled={isRefreshing}
                      title={t('byog.refresh', 'Yenile')}
                    >
                      <RefreshCw className={cn('h-4 w-4', isRefreshing && 'animate-spin')} />
                    </Button>
                  </CardTitle>
                  <CardDescription>
                    {t('byog.client_status_desc', 'Cihazınızın ağ bağlantısı ve model yükleme bilgileri.')}
                  </CardDescription>
                </CardHeader>

                <CardContent className='grid gap-4 text-sm overflow-hidden'>
                  <div className='flex items-center gap-3 min-w-0'>
                    <Server className='text-muted-foreground h-4 w-4 shrink-0' />
                    <span className='text-muted-foreground w-24 sm:w-36 shrink-0'>{t('byog.client_node_name', 'Node Adı')}:</span>
                    <span className='font-mono font-semibold truncate min-w-0 flex-1'>
                      {state?.device_name || nodeId || 'BYOG Worker'}
                    </span>
                  </div>

                  <div className='flex items-center gap-3 min-w-0'>
                    <Link2 className='text-muted-foreground h-4 w-4 shrink-0' />
                    <span className='text-muted-foreground w-24 sm:w-36 shrink-0'>{t('byog.client_server', 'URL')}:</span>
                    <span className='font-mono truncate min-w-0 flex-1'>{serverUrl || window.location.origin}</span>
                  </div>

                  <div className='flex items-center gap-3 min-w-0'>
                    <Cpu className='text-muted-foreground h-4 w-4 shrink-0' />
                    <span className='text-muted-foreground w-24 sm:w-36 shrink-0'>{t('byog.installed_models', 'Modeller')}:</span>
                    <span className='font-mono truncate font-medium min-w-0 flex-1'>
                      {state?.models || state?.assigned_model || '—'}
                    </span>
                  </div>

                  <div className='flex items-center gap-3 min-w-0'>
                    <Gauge className='text-muted-foreground h-4 w-4 shrink-0' />
                    <span className='text-muted-foreground w-24 sm:w-36 shrink-0'>{t('byog.client_speed', 'TPS')}:</span>
                    <span className='font-mono font-bold text-primary truncate min-w-0 flex-1'>
                      {(state?.current_tps ?? 0).toFixed(1)} tps
                    </span>
                  </div>

                  <div className='flex items-center gap-3 min-w-0'>
                    <Activity className='text-muted-foreground h-4 w-4 shrink-0' />
                    <span className='text-muted-foreground w-24 sm:w-36 shrink-0'>{t('byog.client_ttft', 'TTFT')}:</span>
                    <span className='font-mono font-semibold truncate min-w-0 flex-1'>
                      {state?.last_ttft_ms ? `${state.last_ttft_ms} ms` : '—'}
                    </span>
                  </div>

                  {/* Download progress bar if downloading */}
                  {state && state.download_progress > 0 && state.download_progress < 100 && (
                    <div className='space-y-1.5 pt-3 border-t border-border/50'>
                      <div className='flex justify-between text-xs text-muted-foreground'>
                        <span>{t('byog.downloading_model', 'Model İndiriliyor...')}</span>
                        <span className='font-mono font-bold text-primary'>{state.download_progress}%</span>
                      </div>
                      <div className='bg-muted h-2.5 w-full overflow-hidden rounded-full'>
                        <div
                          className='h-full rounded-full bg-primary transition-all duration-300'
                          style={{ width: `${state.download_progress}%` }}
                        />
                      </div>
                    </div>
                  )}
                </CardContent>
              </Card>
            </div>
          )}

          {/* TAB 2: Kaynaklar & Donanım (Resources & Hardware) */}
          {page === 'resources' && (
            <div className='max-w-4xl space-y-4'>
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

          {/* TAB 3: Test (Requires model to be ready) */}
          {page === 'test' && (
            <Card className='max-w-2xl'>
              <CardHeader>
                <CardTitle className='text-base sm:text-lg'>{t('byog.client_test', 'Model Testi')}</CardTitle>
                <CardDescription>
                  {t('byog.client_test_desc', 'Yerel modelinize doğrudan soru sorarak yanıt hızını ve çıktıyı test edin.')}
                </CardDescription>
              </CardHeader>

              <CardContent className='space-y-4'>
                {!isModelReady ? (
                  <div className='flex items-center gap-3 rounded-xl border border-amber-500/30 bg-amber-500/10 p-4 text-sm text-amber-500'>
                    <AlertTriangle className='size-5 shrink-0' />
                    <span>
                      {t(
                        'byog.model_download_required_for_test',
                        'Test yapabilmek için atanmış modelin cihazınıza indirilmesi tamamlanmalıdır.'
                      )}
                    </span>
                  </div>
                ) : null}

                <Input
                  value={prompt}
                  onChange={(event) => setPrompt(event.target.value)}
                  placeholder={t('byog.client_test_prompt', 'Test için bir mesaj yazın...')}
                  disabled={!isModelReady || busy}
                />

                <Button onClick={runTest} disabled={!isModelReady || busy || !prompt.trim()}>
                  {busy ? <RefreshCw className='mr-2 h-4 w-4 animate-spin' /> : <Play className='mr-2 h-4 w-4' />}
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
        </SidebarInset>
      </div>

      {/* Disconnect Confirmation Dialog */}
      <AlertDialog open={disconnectDialogOpen} onOpenChange={setDisconnectDialogOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle className='text-destructive flex items-center gap-2'>
              <AlertTriangle className='size-5' />
              {t('byog.disconnect_confirm_title', 'Cihaz Bağlantısını Kes')}
            </AlertDialogTitle>
            <AlertDialogDescription className='leading-relaxed'>
              {t(
                'byog.disconnect_confirm_desc',
                'Bu cihazın ağ bağlantısını kesmek istediğinize emin misiniz? Arka plan çıkarım işlemleri durdurulacak, cihaz siteden kaldırılacak ve tekrar bağlanmak için yeniden eşleştirmeniz gerekecektir.'
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={busy}>{t('common.cancel', 'Vazgeç')}</AlertDialogCancel>
            <AlertDialogAction
              onClick={disconnectDevice}
              disabled={busy}
              className='bg-destructive text-destructive-foreground hover:bg-destructive/90'
            >
              {busy ? <RefreshCw className='size-4 animate-spin' /> : t('byog.disconnect_device', 'Bağlantıyı Kes')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </SidebarProvider>
  )
}
