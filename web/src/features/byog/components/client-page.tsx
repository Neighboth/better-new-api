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
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import jsQR from 'jsqr'
import {
  Activity,
  AlertTriangle,
  Battery,
  Bell,
  Camera,
  CheckCircle2,
  ChevronRight,
  Cpu,
  Gauge,
  Link2,
  LogOut,
  Monitor,
  Play,
  QrCode,
  RefreshCw,
  Save,
  Server,
  Sliders,
  Terminal,
  Upload,
  Zap,
} from 'lucide-react'

import { getFreshAuthHeaders } from '@/lib/api'
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
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { useSystemConfig } from '@/hooks/use-system-config'
import { cn } from '@/lib/utils'

import { ResourceSliders } from './resource-sliders'
import { ScheduleMatrix } from './schedule-matrix'
import { MobileSettingsCard } from './mobile-settings-card'
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
  device_type?: string
  status: string
  paired?: boolean
  user_id?: number
  model_status: string
  download_status: string
  download_progress: number
  current_tps: number
  last_ttft_ms?: number
  models: string
  assigned_model?: string
  disks?: DiskInfo[]
  physical_ram_mb?: number
  swap_ram_mb?: number
  cpu_cores?: number
  cpu_model?: string
  gpu_model?: string
  vram_mb?: number
  shared_vram_mb?: number
  bandwidth_down_mbps?: number
  bandwidth_up_mbps?: number
  ping_ms?: number
  download_model_id?: string
  downloaded_mb?: number
  total_mb?: number
  download_speed_bps?: number
  eta_seconds?: number
  download_queue?: string[]
  total_disk_gb?: number
  free_disk_gb?: number
  allocated_ram_gb?: number
  allocated_vram_gb?: number
  allocated_cores?: number
  allocated_disk_gb?: number
  operating_mode?: string
  power_profile?: string
  schedule_config?: string
  auto_pause_on_game?: boolean
  hot_start_enabled?: boolean
  favicon?: string
  favicon_base64?: string
  system_name?: string
  app_name?: string
  language?: string
}

// formatRemainingTime formats a duration in seconds as `1h 2m 3s`.
//
// The parts that are zero are not shown, the time of a download is the time of
// a transfer and not a number of minutes that the user has to divide.
export function formatRemainingTime(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds <= 0) return '—'
  const total = Math.round(seconds)
  const hours = Math.floor(total / 3600)
  const minutes = Math.floor((total % 3600) / 60)
  const rest = total % 60
  const parts: string[] = []
  if (hours > 0) parts.push(`${hours}h`)
  if (minutes > 0 || hours > 0) parts.push(`${minutes}m`)
  parts.push(`${rest}s`)
  return parts.join(' ')
}

// formatTransferSpeed formats a rate in bytes per second.
export function formatTransferSpeed(bytesPerSecond: number): string {
  if (!Number.isFinite(bytesPerSecond) || bytesPerSecond <= 0) return ''
  const units = ['B/s', 'KB/s', 'MB/s', 'GB/s']
  let value = bytesPerSecond
  let unit = 0
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024
    unit += 1
  }
  return `${value.toFixed(unit === 0 ? 0 : 1)} ${units[unit]}`
}

// mergeWorkerState merges a state report of the device into the state of the
// page.
//
// The reports of the broadcasts of the device and of the record of the node
// arrive in an arbitrary order, a report of an older moment must never pull
// the bar of a running download down. The identity of the download decides:
// a report of another model starts a fresh bar, a report of the same model
// only moves the bar forward.
export function mergeWorkerState(
  prev: WorkerState | null | undefined,
  incoming: Partial<WorkerState>
): WorkerState {
  const merged = {
    ...((prev ?? {}) as WorkerState),
    ...incoming,
  } as WorkerState

  if (!prev) {
    return merged
  }

  const prevModel = prev.download_model_id || ''
  const nextModel = incoming.download_model_id || ''
  const sameTransfer = !nextModel || !prevModel || nextModel === prevModel

  if (sameTransfer) {
    // The bar of the same download only moves forward.
    merged.download_progress = Math.max(
      prev.download_progress ?? 0,
      incoming.download_progress ?? 0
    )
    if (incoming.downloaded_mb !== undefined || prev.downloaded_mb !== undefined) {
      merged.downloaded_mb = Math.max(prev.downloaded_mb ?? 0, incoming.downloaded_mb ?? 0)
    }
  }

  // The real size of the model host is the authority about the total, a
  // report without a size never replaces a known size with zero.
  if (incoming.total_mb === undefined) {
    merged.total_mb = prev.total_mb ?? merged.total_mb
  } else if (incoming.total_mb <= 0 && (prev.total_mb ?? 0) > 0) {
    merged.total_mb = prev.total_mb
  }

  return merged
}

// formatMegabytes formats a size of a model.
export function formatMegabytes(megabytes: number | undefined): string {
  if (!megabytes || megabytes <= 0) return '0 MB'
  if (megabytes >= 1024) return `${(megabytes / 1024).toFixed(1)} GB`
  return `${Math.round(megabytes)} MB`
}

// mergeDiskReports joins the partitions of the record of the node with the
// partitions of the computer.
//
// The record carries the allocation that the user made, the client carries the
// free space of the moment. A partition that the client does not report is kept
// from the record, a partition that the client reports new has no allocation.
function mergeDiskReports(
  remote: DiskInfo[] | undefined,
  local: DiskInfo[] | undefined
): DiskInfo[] | undefined {
  if (!remote || remote.length === 0) return local
  if (!local || local.length === 0) return remote
  return remote.map((disk) => {
    const reported = local.find((item) => item.drive === disk.drive)
    if (!reported) return disk
    return {
      ...disk,
      label: reported.label || disk.label,
      total_gb: reported.total_gb,
      free_gb: reported.free_gb,
      allocated_gb: disk.allocated_gb ?? 0,
    }
  })
}

function formatPairingCode(val: string): string {
  const cleaned = val.toUpperCase().replace(/[^A-Z0-9]/g, '')
  if (cleaned.length === 16) {
    return cleaned.match(/.{1,4}/g)?.join('-') ?? cleaned
  }
  return val.toUpperCase().replace(/[^A-Z0-9-]/g, '')
}

function extractPairingCode(text: string): string {
  if (!text) return ''
  const trimmed = text.trim()
  if (trimmed.startsWith('{') && trimmed.endsWith('}')) {
    try {
      const parsed = JSON.parse(trimmed) as Record<string, unknown>
      const rawCode = String(parsed.pairing_code ?? parsed.pairingCode ?? parsed.code ?? '')
      if (rawCode) {
        return formatPairingCode(rawCode)
      }
    } catch {
      // ignore
    }
  }
  return formatPairingCode(trimmed)
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
  testModelInference?(prompt: string): void
  workerRequest?(operation: string, input: string): string
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

function QrScanModal({
  open,
  onOpenChange,
  onScan,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  onScan: (code: string) => void
}) {
  const { t } = useTranslation()
  const [activeTab, setActiveTab] = useState<'camera' | 'screen' | 'file'>('camera')
  const [errorMsg, setErrorMsg] = useState('')
  const [cameraActive, setCameraActive] = useState(false)
  const videoRef = useRef<HTMLVideoElement | null>(null)
  const canvasRef = useRef<HTMLCanvasElement | null>(null)
  const streamRef = useRef<MediaStream | null>(null)
  const animFrameRef = useRef<number | null>(null)
  const fileInputRef = useRef<HTMLInputElement | null>(null)

  const stopStream = useCallback(() => {
    if (animFrameRef.current) {
      cancelAnimationFrame(animFrameRef.current)
      animFrameRef.current = null
    }
    if (streamRef.current) {
      streamRef.current.getTracks().forEach((track) => track.stop())
      streamRef.current = null
    }
    if (videoRef.current) {
      videoRef.current.srcObject = null
    }
    setCameraActive(false)
  }, [])

  useEffect(() => {
    if (!open) {
      stopStream()
      setErrorMsg('')
    }
  }, [open, stopStream])

  const scanFrame = useCallback(() => {
    const video = videoRef.current
    if (!video || video.readyState !== video.HAVE_ENOUGH_DATA) {
      animFrameRef.current = requestAnimationFrame(scanFrame)
      return
    }
    if (!canvasRef.current) {
      canvasRef.current = document.createElement('canvas')
    }
    const canvas = canvasRef.current
    let w = video.videoWidth
    let h = video.videoHeight
    if (w > 1000) {
      const scale = 1000 / w
      w = 1000
      h = Math.round(h * scale)
    }
    canvas.width = w
    canvas.height = h
    const ctx = canvas.getContext('2d', { willReadFrequently: true })
    if (ctx) {
      ctx.drawImage(video, 0, 0, w, h)
      const imageData = ctx.getImageData(0, 0, w, h)
      const qrCode = jsQR(imageData.data, w, h, { inversionAttempts: 'attemptBoth' })
      if (qrCode && qrCode.data) {
        onScan(qrCode.data)
        stopStream()
        onOpenChange(false)
        return
      }
    }
    animFrameRef.current = requestAnimationFrame(scanFrame)
  }, [onScan, onOpenChange, stopStream])

  const startCamera = async () => {
    stopStream()
    setErrorMsg('')
    try {
      const stream = await navigator.mediaDevices.getUserMedia({
        video: { facingMode: 'environment' },
      })
      streamRef.current = stream
      if (videoRef.current) {
        videoRef.current.srcObject = stream
        videoRef.current.setAttribute('playsinline', 'true')
        await videoRef.current.play()
        setCameraActive(true)
        animFrameRef.current = requestAnimationFrame(scanFrame)
      }
    } catch {
      setErrorMsg(t('byog.camera_permission_denied', 'Kamera açılamadı veya izin verilmedi.'))
    }
  }

  const startScreenCapture = async () => {
    stopStream()
    setErrorMsg('')
    try {
      const stream = await navigator.mediaDevices.getDisplayMedia({
        video: true,
      })
      streamRef.current = stream
      if (videoRef.current) {
        videoRef.current.srcObject = stream
        await videoRef.current.play()
        setCameraActive(true)
        animFrameRef.current = requestAnimationFrame(scanFrame)
      }
    } catch {
      setErrorMsg(t('byog.screen_capture_denied', 'Ekran paylaşımı başlatılamadı veya iptal edildi.'))
    }
  }

  const handleFileUpload = (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    if (!file) return
    setErrorMsg('')
    const reader = new FileReader()
    reader.onload = (e) => {
      const img = new Image()
      img.onload = () => {
        const canvas = document.createElement('canvas')
        let w = img.width
        let h = img.height
        if (w > 1200) {
          const scale = 1200 / w
          w = 1200
          h = Math.round(h * scale)
        }
        canvas.width = w
        canvas.height = h
        const ctx = canvas.getContext('2d', { willReadFrequently: true })
        if (!ctx) return
        ctx.drawImage(img, 0, 0, w, h)
        const imgData = ctx.getImageData(0, 0, w, h)
        const code = jsQR(imgData.data, w, h, { inversionAttempts: 'attemptBoth' })
        if (code && code.data) {
          onScan(code.data)
          onOpenChange(false)
        } else {
          setErrorMsg(t('byog.qr_not_found_in_image', 'Yüklenen görselde geçerli bir QR kod bulunamadı.'))
        }
      }
      img.src = e.target?.result as string
    }
    reader.readAsDataURL(file)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-w-md p-6'>
        <DialogHeader>
          <DialogTitle className='flex items-center gap-2 text-lg font-bold'>
            <QrCode className='size-5 text-primary' />
            {t('byog.qr_scan_title', 'QR Kod Okut')}
          </DialogTitle>
          <DialogDescription>
            {t('byog.qr_scan_desc', 'Eşleştirme kodunu kamera, ekran yakalama veya QR resim dosyası yükleyerek okutun.')}
          </DialogDescription>
        </DialogHeader>

        <Tabs
          value={activeTab}
          onValueChange={(val) => {
            stopStream()
            setErrorMsg('')
            setActiveTab(val as 'camera' | 'screen' | 'file')
          }}
          className='w-full'
        >
          <TabsList className='grid grid-cols-3 w-full mb-4'>
            <TabsTrigger value='camera' className='flex items-center gap-1.5 text-xs sm:text-sm'>
              <Camera className='size-4' />
              {t('byog.tab_camera', 'Kamera')}
            </TabsTrigger>
            <TabsTrigger value='screen' className='flex items-center gap-1.5 text-xs sm:text-sm'>
              <Monitor className='size-4' />
              {t('byog.tab_screen', 'Ekran')}
            </TabsTrigger>
            <TabsTrigger value='file' className='flex items-center gap-1.5 text-xs sm:text-sm'>
              <Upload className='size-4' />
              {t('byog.tab_file', 'Dosya')}
            </TabsTrigger>
          </TabsList>

          <TabsContent value='camera' className='space-y-3'>
            <div className='relative aspect-video w-full overflow-hidden rounded-xl bg-muted/60 border border-border flex items-center justify-center'>
              <video ref={videoRef} className={cn('h-full w-full object-cover', !cameraActive && 'hidden')} />
              {!cameraActive && (
                <div className='text-center p-4'>
                  <Camera className='size-10 mx-auto text-muted-foreground/60 mb-2' />
                  <Button onClick={startCamera} size='sm'>
                    {t('byog.start_camera', 'Kamerayı Başlat')}
                  </Button>
                </div>
              )}
            </div>
          </TabsContent>

          <TabsContent value='screen' className='space-y-3'>
            <div className='relative aspect-video w-full overflow-hidden rounded-xl bg-muted/60 border border-border flex items-center justify-center'>
              <video ref={videoRef} className={cn('h-full w-full object-cover', !cameraActive && 'hidden')} />
              {!cameraActive && (
                <div className='text-center p-4'>
                  <Monitor className='size-10 mx-auto text-muted-foreground/60 mb-2' />
                  <Button onClick={startScreenCapture} size='sm'>
                    {t('byog.start_screen_capture', 'Ekranı Paylaş & Tara')}
                  </Button>
                </div>
              )}
            </div>
          </TabsContent>

          <TabsContent value='file' className='space-y-3'>
            <div
              onClick={() => fileInputRef.current?.click()}
              className='aspect-video w-full rounded-xl border-2 border-dashed border-border hover:border-primary/50 flex flex-col items-center justify-center cursor-pointer p-4 transition-colors'
            >
              <Upload className='size-10 text-muted-foreground/60 mb-2' />
              <span className='text-sm font-medium text-foreground'>
                {t('byog.upload_qr_image', 'QR Kod Görseli Seç')}
              </span>
              <span className='text-xs text-muted-foreground mt-1'>PNG, JPG, WEBP</span>
              <input
                ref={fileInputRef}
                type='file'
                accept='image/*'
                onChange={handleFileUpload}
                className='hidden'
              />
            </div>
          </TabsContent>
        </Tabs>

        {errorMsg && (
          <div className='rounded-lg bg-destructive/10 border border-destructive/20 p-2.5 text-xs text-destructive font-medium'>
            {errorMsg}
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}

const cleanModel = (val: unknown): string => {
  if (!val) return ''
  if (Array.isArray(val)) {
    const valid = val.map((v) => String(v).trim()).filter((v) => v && v !== 'null' && v !== 'undefined')
    return valid.join(', ')
  }
  const s = String(val).trim()
  if (s === 'null' || s === 'undefined' || s === '[]' || s === '[""]' || s === '["null"]') return ''
  if (s.startsWith('[') && s.endsWith(']')) {
    try {
      const parsed = JSON.parse(s)
      if (Array.isArray(parsed)) {
        const valid = parsed.map((v) => String(v).trim()).filter((v) => v && v !== 'null' && v !== 'undefined')
        return valid.join(', ')
      }
    } catch {}
  }
  return s
}

export function ClientPage() {
  const { t, i18n } = useTranslation()
  const { logo: systemLogo, favicon: systemFavicon, systemName } = useSystemConfig()
  const [page, setPage] = useState<PageKey>(() => {
    if (typeof window !== 'undefined') {
      const tab = new URLSearchParams(window.location.search).get('tab')
      if (tab === 'resources' || tab === 'test') return tab
    }
    return 'status'
  })
  const [nodeId, setNodeId] = useState<string>(() => {
    if (typeof window !== 'undefined') {
      const search = new URLSearchParams(window.location.search)
      return search.get('node_id') || localStorage.getItem('byog_client_node_id') || ''
    }
    return ''
  })
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
    if (typeof window !== 'undefined') {
      if (new URLSearchParams(window.location.search).get('onboarding') === '1') {
        return false
      }
      return localStorage.getItem('byog_onboarding_done') === '1'
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
  const [vramGB, setVramGB] = useState(0)
  const [ramGB, setRamGB] = useState(0)
  const [cores, setCores] = useState(0)
  const [diskGB, setDiskGB] = useState(0)
  const [diskAllocations, setDiskAllocations] = useState<Record<string, number>>({})
  const [hotStart, setHotStart] = useState(false)
  const [wifiOnly, setWifiOnly] = useState(false)
  const [dataLimitMB, setDataLimitMB] = useState(0)
  const [chargingOnly, setChargingOnly] = useState(false)
  const [batteryCutoffEnabled, setBatteryCutoffEnabled] = useState(false)
  const [batteryCutoffPercent, setBatteryCutoffPercent] = useState(20)
  const [maxBatteryTemp, setMaxBatteryTemp] = useState(50)
  const [autoPauseOnGame, setAutoPauseOnGame] = useState(true)
  const [operatingMode, setOperatingMode] = useState<'mode_a' | 'mode_b'>('mode_a')
  const [powerProfile, setPowerProfile] = useState<'full_power' | 'half_power' | 'active_hours' | 'dual_schedule'>('half_power')
  const [scheduleConfig, setScheduleConfig] = useState<string>('')
  const [qrModalOpen, setQrModalOpen] = useState(false)
  const [savingResources, setSavingResources] = useState(false)
  const [saveSuccessMsg, setSaveSuccessMsg] = useState('')
  const [allocatedInitialized, setAllocatedInitialized] = useState(false)
  const lastUserEditTimeRef = useRef(0)

  const bridge = getBridge()
  const [localApi, setLocalApi] = useState<string>(() => {
    if (typeof window !== 'undefined') {
      return new URLSearchParams(window.location.search).get('local') || ''
    }
    return ''
  })
  const [localToken, setLocalToken] = useState<string>(() => {
    if (typeof window !== 'undefined') {
      return new URLSearchParams(window.location.search).get('token') || ''
    }
    return ''
  })

  // Live Bidirectional Sync & 50% Defaults: sync allocations from state when user is not actively editing
  useEffect(() => {
    if (!state) return
    const isActivelyEditing = Date.now() - lastUserEditTimeRef.current < 3000
    if (isActivelyEditing) return

    const effectiveRam = state.allocated_ram_gb !== undefined && state.allocated_ram_gb !== null
      ? state.allocated_ram_gb
      : Math.round(((state.physical_ram_mb || 8192) / 2048) * 10) / 10
    if (effectiveRam !== ramGB) {
      setRamGB(effectiveRam)
    }

    const effectiveCores = state.allocated_cores !== undefined && state.allocated_cores !== null
      ? state.allocated_cores
      : Math.max(1, Math.floor((state.cpu_cores || 4) * 0.5))
    if (effectiveCores !== cores) {
      setCores(effectiveCores)
    }

    const maxDedicatedVram = (state.vram_mb && state.vram_mb >= 1024) ? Math.floor(state.vram_mb / 1024) : 0
    const effectiveVram = maxDedicatedVram > 0
      ? (state.allocated_vram_gb !== undefined && state.allocated_vram_gb !== null ? state.allocated_vram_gb : Math.floor(maxDedicatedVram * 0.5))
      : 0
    if (effectiveVram !== vramGB) {
      setVramGB(effectiveVram)
    }

    if (state.hot_start_enabled !== undefined && state.hot_start_enabled !== hotStart) {
      setHotStart(state.hot_start_enabled)
    }

    if (state.operating_mode && (state.operating_mode === 'mode_a' || state.operating_mode === 'mode_b') && state.operating_mode !== operatingMode) {
      setOperatingMode(state.operating_mode)
    }
    if (state.power_profile && state.power_profile !== powerProfile) {
      setPowerProfile(state.power_profile as any)
    }
    if (state.schedule_config !== undefined && state.schedule_config !== scheduleConfig) {
      setScheduleConfig(state.schedule_config)
    }
    if (state.auto_pause_on_game !== undefined && state.auto_pause_on_game !== autoPauseOnGame) {
      setAutoPauseOnGame(state.auto_pause_on_game)
    }

    if (state.disks && state.disks.length > 0) {
      const allocations: Record<string, number> = {}
      let anyAllocated = false
      for (const disk of state.disks) {
        if (disk.allocated_gb !== undefined && disk.allocated_gb !== null) {
          anyAllocated = true
          break
        }
      }
      for (const disk of state.disks) {
        allocations[disk.drive] = anyAllocated
          ? (disk.allocated_gb ?? 0)
          : Math.floor((disk.free_gb || 0) * 0.5)
      }
      setDiskAllocations(allocations)
      const sum = Object.values(allocations).reduce((a, b) => a + b, 0)
      setDiskGB(sum)
    } else if (state.allocated_disk_gb !== undefined && state.allocated_disk_gb !== diskGB) {
      setDiskGB(state.allocated_disk_gb)
    }

    if (!allocatedInitialized && (state.allocated_ram_gb !== undefined || state.physical_ram_mb)) {
      setAllocatedInitialized(true)
    }
  }, [allocatedInitialized, state, ramGB, vramGB, cores, diskGB, hotStart, operatingMode, powerProfile, scheduleConfig, autoPauseOnGame])

  // Load client state from Android bridge, local agent, or coordinator
  const loadState = useCallback(async () => {
    if (bridge) {
      try {
        const raw = bridge.getInitialState()
        if (raw) {
          const initial = JSON.parse(raw) as Record<string, unknown>
          if (typeof initial.server_url === 'string' && initial.server_url) {
            setServerUrl(initial.server_url)
          }
          if (typeof initial.node_id === 'string' && initial.node_id) {
            setNodeId(initial.node_id)
          }
          const isPaired = Boolean(initial.is_paired && initial.node_id)
          setState((prev) => ({
            ...prev,
            node_id: String(initial.node_id ?? ''),
            device_name: String(initial.device_name ?? initial.app_name ?? ''),
            system_name: String(initial.system_name ?? ''),
            app_name: String(initial.app_name ?? ''),
            status: isPaired ? 'online' : 'offline',
            paired: isPaired,
            model_status: String(initial.model_status ?? ''),
            download_status: String(initial.model_status ?? ''),
            download_progress: Number(initial.download_progress ?? 0),
            current_tps: Number(initial.current_tps ?? 0),
            models: String(initial.assigned_model ?? ''),
            assigned_model: String(initial.assigned_model ?? ''),
            disks: initial.disks as WorkerState['disks'],
            physical_ram_mb: initial.physical_ram_mb as number,
            swap_ram_mb: initial.swap_ram_mb as number,
            cpu_cores: initial.cpu_cores as number,
            cpu_model: initial.cpu_model as string,
            gpu_model: initial.gpu_model as string,
            vram_mb: initial.vram_mb as number,
            shared_vram_mb: initial.shared_vram_mb as number,
            bandwidth_down_mbps: initial.bandwidth_down_mbps as number,
            bandwidth_up_mbps: initial.bandwidth_up_mbps as number,
            ping_ms: initial.ping_ms as number,
            download_model_id: initial.download_model_id as string,
            downloaded_mb: Number(initial.downloaded_mb ?? 0),
            total_mb: Number(initial.total_mb ?? 0),
            download_speed_bps: initial.download_speed_bps as number,
            eta_seconds: initial.eta_seconds as number,
            download_queue: initial.download_queue as string[],
            allocated_disk_gb: initial.allocated_disk_gb as number,
            hot_start_enabled: initial.hot_start_enabled as boolean,
            favicon: initial.favicon as string,
            favicon_base64: initial.favicon_base64 as string,
          }))
        }
      } catch {
        // bridge fallback
      }
      try {
        const raw = bridge.getPermissionStatus()
        if (raw) {
          const perms = JSON.parse(raw) as PermissionState
          setPermissions(perms)
          if (!perms.notifications || !perms.battery) {
            setOnboardingDone(false)
          } else if (typeof bridge.isOnboardingDone === 'function') {
            setOnboardingDone(bridge.isOnboardingDone())
          } else {
            setOnboardingDone(true)
          }
        }
      } catch {
        // ignore
      }
    } else if (localApi) {
      try {
        const response = await fetch(`${localApi}/state?token=${localToken}`, {
          signal: AbortSignal.timeout(5000),
        })
        const payload = (await response.json()) as Record<string, unknown>
        if (typeof payload.server_url === 'string' && payload.server_url) {
          setServerUrl(payload.server_url)
        }
        if (typeof payload.node_id === 'string' && payload.node_id) {
          setNodeId(payload.node_id)
        }
        setState((prev) => ({
          ...prev,
          node_id: String(payload.node_id ?? ''),
          device_name: String(payload.device_name ?? payload.app_name ?? ''),
          system_name: String(payload.system_name ?? ''),
          app_name: String(payload.app_name ?? ''),
          status: payload.connected ? 'online' : 'offline',
          paired: Boolean(payload.paired),
          model_status: String(payload.model_status ?? ''),
          download_status: String(payload.model_status ?? ''),
          download_progress: (() => {
            const incoming = Number(payload.download_progress ?? 0)
            const sameModel = prev?.download_model_id && payload.download_model_id === prev.download_model_id
            return (sameModel && incoming > 0 && prev?.download_progress)
              ? Math.max(prev.download_progress, incoming)
              : incoming
          })(),
          current_tps: Number(payload.current_tps ?? 0),
          models: String(payload.model_name ?? ''),
          disks: payload.disks as WorkerState['disks'],
          physical_ram_mb: payload.physical_ram_mb as number,
          swap_ram_mb: payload.swap_ram_mb as number,
          cpu_cores: payload.cpu_cores as number,
          cpu_model: payload.cpu_model as string,
          gpu_model: payload.gpu_model as string,
          vram_mb: payload.vram_mb as number,
          shared_vram_mb: payload.shared_vram_mb as number,
          bandwidth_down_mbps: payload.bandwidth_down_mbps as number,
          bandwidth_up_mbps: payload.bandwidth_up_mbps as number,
          ping_ms: payload.ping_ms as number,
          download_model_id: payload.download_model_id as string,
          downloaded_mb: (() => {
            const incoming = Number(payload.downloaded_mb ?? 0)
            const sameModel = prev?.download_model_id && payload.download_model_id === prev.download_model_id
            return (sameModel && incoming > 0 && prev?.downloaded_mb)
              ? Math.max(prev.downloaded_mb, incoming)
              : incoming
          })(),
          total_mb: Number(payload.total_mb ?? 0) || prev?.total_mb || 0,
          download_speed_bps: payload.download_speed_bps as number,
          eta_seconds: payload.eta_seconds as number,
          download_queue: payload.download_queue as string[],
          allocated_disk_gb: payload.allocated_disk_gb as number,
          allocated_ram_gb: payload.allocated_ram_gb as number,
          allocated_vram_gb: payload.allocated_vram_gb as number,
          allocated_cores: payload.allocated_cores as number,
          operating_mode: payload.operating_mode as string,
          power_profile: payload.power_profile as string,
          schedule_config: payload.schedule_config as string,
          auto_pause_on_game: payload.auto_pause_on_game as boolean,
          hot_start_enabled: payload.hot_start_enabled as boolean,
          favicon_base64: (payload.favicon_base64 as string) || undefined,
        }))
      } catch {
        // ignore
      }
    }

    // The record of the node is the source of the allocation of the resources
    // and of the hardware that the client reported. The page reads it in every
    // mode: a client that is opened on the site and a client that is opened on
    // the computer show the same allocation.
    const targetNodeId = nodeId || state?.node_id || ''
    if (!targetNodeId) {
      return
    }
    try {
      let payload: { success: boolean; data?: Record<string, unknown>; code?: string }
      if (bridge?.workerRequest) {
        payload = JSON.parse(bridge.workerRequest('status', '{}'))
      } else {
        const url = localApi
          ? `${localApi}/worker/status?token=${encodeURIComponent(localToken)}`
          : `/api/byog/user/client/status?node_id=${encodeURIComponent(targetNodeId)}`
        const response = await fetch(url, {
          headers: localApi ? {} : await getFreshAuthHeaders(),
          signal: AbortSignal.timeout(4000),
        })
        payload = await response.json()
      }
      if (payload.success && payload.data) {
        const remote = payload.data as Record<string, unknown>
        setState((prev) => {
          const merged = {
            ...(prev ?? {}),
            ...(remote as Partial<WorkerState>),
            status: remote.status === 'online' ? 'online' : 'offline',
            paired: true,
            models:
              cleanModel(remote.models) ||
              cleanModel(remote.assigned_model) ||
              cleanModel(prev?.models) ||
              '',
            downloaded_mb: Math.max(
              Number(prev?.downloaded_mb ?? 0),
              Number(remote.downloaded_mb ?? 0)
            ),
            total_mb: Math.max(
              Number(prev?.total_mb ?? 0),
              Number(remote.total_mb ?? 0)
            ),
            download_progress: Math.max(
              Number(prev?.download_progress ?? 0),
              Number(remote.download_progress ?? 0)
            ),
          } as WorkerState
          merged.disks = mergeDiskReports(
            remote.disks as DiskInfo[] | undefined,
            prev?.disks
          )
          if (prev && (localApi || bridge)) {
            // The client of the computer is the source of the live values, the
            // record of the node is the source of the allocation above.
            if (prev.download_status) merged.download_status = prev.download_status
            if (prev.download_progress !== undefined) merged.download_progress = prev.download_progress
            if (prev.downloaded_mb !== undefined) merged.downloaded_mb = prev.downloaded_mb
            if (prev.total_mb !== undefined && prev.total_mb > 0) merged.total_mb = prev.total_mb
            if (prev.download_model_id) merged.download_model_id = prev.download_model_id
            if (prev.download_speed_bps !== undefined) merged.download_speed_bps = prev.download_speed_bps
            if (prev.eta_seconds !== undefined) merged.eta_seconds = prev.eta_seconds
            if (prev.current_tps !== undefined && prev.current_tps > 0) {
              merged.current_tps = prev.current_tps
            }
            if (prev.ping_ms !== undefined && prev.ping_ms > 0) {
              merged.ping_ms = prev.ping_ms
            }
            if ((merged.bandwidth_down_mbps === undefined || merged.bandwidth_down_mbps === 0) && prev.bandwidth_down_mbps !== undefined && prev.bandwidth_down_mbps > 0) {
              merged.bandwidth_down_mbps = prev.bandwidth_down_mbps
            }
            if ((merged.bandwidth_up_mbps === undefined || merged.bandwidth_up_mbps === 0) && prev.bandwidth_up_mbps !== undefined && prev.bandwidth_up_mbps > 0) {
              merged.bandwidth_up_mbps = prev.bandwidth_up_mbps
            }
            if (prev.download_queue) merged.download_queue = prev.download_queue
            if (prev.model_status) {
              merged.model_status = prev.model_status
            }
            if (prev.vram_mb !== undefined) merged.vram_mb = prev.vram_mb
            if (prev.shared_vram_mb !== undefined) merged.shared_vram_mb = prev.shared_vram_mb
            if (prev.gpu_model) merged.gpu_model = prev.gpu_model
            if (prev.cpu_model) merged.cpu_model = prev.cpu_model
          }
          return merged
        })
      } else if (payload?.code === 'NODE_NOT_FOUND' && !bridge) {
        // Node does not exist on server (database reset or unpaired)
        window.localStorage.removeItem('byog_node_id')
        setNodeId('')
        setState((prev) => (prev ? { ...prev, paired: false, node_id: '', status: 'offline' } : null))
      }
    } catch {
      // ignore
    }
  }, [bridge, localApi, localToken, nodeId, state?.node_id])

  // Save allocated resource sliders to backend & local bridge
  const saveResourceAllocations = useCallback(async () => {
    setSavingResources(true)
    setSaveSuccessMsg('')
    try {
      const diskList = Object.entries(diskAllocations).map(([drive, allocated]) => ({
        drive,
        allocated_gb: allocated,
      }))
      const localSavePromise = localApi
        ? fetch(`${localApi}/settings?token=${localToken}`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            signal: AbortSignal.timeout(4000),
            body: JSON.stringify({
              allocated_ram_gb: ramGB,
              allocated_vram_gb: vramGB,
              allocated_cores: cores,
              allocated_disk_gb: diskGB,
              hot_start_enabled: hotStart,
              disks: diskList,
              operating_mode: operatingMode,
              power_profile: powerProfile,
              schedule_config: scheduleConfig,
              auto_pause_on_game: autoPauseOnGame,
            }),
          }).catch(() => {})
        : Promise.resolve()

      const targetNodeId = nodeId || state?.node_id
      const remoteSavePromise = (async () => {
        if (!targetNodeId) return
        const resources = {
          node_id: targetNodeId,
          allocated_ram_gb: ramGB,
          allocated_vram_gb: vramGB,
          allocated_cores: cores,
          allocated_disk_gb: diskGB,
          hot_start_enabled: hotStart,
          disks: diskList,
          operating_mode: operatingMode,
          power_profile: powerProfile,
          schedule_config: scheduleConfig,
        }
        if (bridge?.workerRequest) {
          bridge.workerRequest('resources', JSON.stringify(resources))
        } else {
          const url = localApi
            ? `${localApi}/worker/resources?token=${encodeURIComponent(localToken)}`
            : '/api/byog/user/client/resources'
          const headers = localApi ? {} : await getFreshAuthHeaders()
          await fetch(url, {
            method: 'POST',
            headers: { ...headers, 'Content-Type': 'application/json' },
            signal: AbortSignal.timeout(4000),
            body: JSON.stringify(resources),
          }).catch(() => {})
        }
      })()

      await Promise.allSettled([localSavePromise, remoteSavePromise])
      setSaveSuccessMsg(t('byog.resources_saved', 'Kaynak ayarları başarıyla kaydedildi'))
      setTimeout(() => setSaveSuccessMsg(''), 4000)
      void loadState()
    } catch (e) {
      setMessage(String(e))
    } finally {
      setSavingResources(false)
    }
  }, [bridge, cores, diskAllocations, diskGB, hotStart, loadState, localApi, localToken, nodeId, ramGB, state?.node_id, t, vramGB, operatingMode, powerProfile, scheduleConfig, autoPauseOnGame])

  const handleVramChange = (val: number) => {
    lastUserEditTimeRef.current = Date.now()
    setVramGB(val)
  }
  const handleRamChange = (val: number) => {
    lastUserEditTimeRef.current = Date.now()
    setRamGB(val)
  }
  const handleCoresChange = (val: number) => {
    lastUserEditTimeRef.current = Date.now()
    setCores(val)
  }
  const handleDiskChange = (val: number) => {
    lastUserEditTimeRef.current = Date.now()
    setDiskGB(val)
  }
  const handleDiskAllocationsChange = (val: Record<string, number>) => {
    lastUserEditTimeRef.current = Date.now()
    setDiskAllocations(val)
    const sum = Object.values(val).reduce((a, b) => a + b, 0)
    setDiskGB(sum)
  }
  const handleHotStartChange = (val: boolean) => {
    lastUserEditTimeRef.current = Date.now()
    setHotStart(val)
  }

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
    if (state?.language && state.language !== i18n.language) {
      i18n.changeLanguage(state.language)
    }
  }, [state?.language, i18n])



  useEffect(() => {
    const target = window as unknown as Record<string, unknown>
    target.showUpdatePrompt = (version: string) => setUpdateVersion(version)
    target.updateState = (stateObj: unknown) => {
      if (!stateObj) return
      try {
        const parsed = typeof stateObj === 'string' ? JSON.parse(stateObj) : (stateObj as Record<string, unknown>)
        if (parsed && typeof parsed === 'object') {
          // The merge of the broadcast follows the identity of the download,
          // a report of an older moment never pulls the bar of the running
          // transfer down.
          setState((prev) => mergeWorkerState(prev, parsed as Partial<WorkerState>))
          if (parsed.node_id && typeof parsed.node_id === 'string') {
            setNodeId(parsed.node_id)
            window.localStorage.setItem('byog_node_id', parsed.node_id)
          }
          if (parsed.server_url && typeof parsed.server_url === 'string') {
            setServerUrl(parsed.server_url)
          }
        }
      } catch {}
    }
    target.handleScannedQr = (qrText: string) => {
      if (qrText) {
        setCode(extractPairingCode(qrText))
      }
    }
    target.refreshPermissions = () => {
      if (bridge && typeof bridge.getPermissionStatus === 'function') {
        try {
          const raw = bridge.getPermissionStatus()
          if (raw) {
            setPermissions(JSON.parse(raw) as PermissionState)
          }
        } catch {}
      }
    }
    const onFocus = () => {
      const refresh = target.refreshPermissions
      if (typeof refresh === 'function') refresh()
    }
    window.addEventListener('focus', onFocus)
    target.onPairSuccess = () => {
      setBusy(false)
      setOnboardingDone(true)
      setMessage(t('byog.client_paired', 'Cihaz başarıyla eşleştirildi'))
      void loadState()
    }
    target.onModelTestResult = (reply?: string) => {
      setBusy(false)
      if (reply) setAnswer(reply)
    }
    target.onPairError = (errMsg?: string) => {
      setBusy(false)
      setMessage(errMsg || t('byog.client_pair_failed', 'Eşleştirme başarısız oldu'))
    }
    return () => {
      window.removeEventListener('focus', onFocus)
      delete target.showUpdatePrompt
      delete target.updateState
      delete target.handleScannedQr
      delete target.refreshPermissions
      delete target.onPairSuccess
      delete target.onModelTestResult
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
    const isDownloading = state?.model_status === 'downloading'
    const intervalMs = isDownloading ? 1000 : 4000
    const timer = setInterval(() => void loadState(), intervalMs)
    return () => clearInterval(timer)
  }, [loadState, state?.model_status])

  const handleManualRefresh = async () => {
    setIsRefreshing(true)
    try {
      await loadState()
    } finally {
      setTimeout(() => setIsRefreshing(false), 400)
    }
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
        await fetch(`${localApi}/disconnect?token=${localToken}`, {
          method: 'POST',
        }).catch(() => {})
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
      if (bridge?.testModelInference) {
        bridge.testModelInference(prompt)
        return
      }
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
        response = await fetch('/api/byog/user/client/test-inference', {
          method: 'POST',
          headers: { ...(await getFreshAuthHeaders()), 'Content-Type': 'application/json' },
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
    if (state?.free_disk_gb && state.free_disk_gb > 0) {
      return state.free_disk_gb
    }
    return 0
  }, [state?.disks, state?.free_disk_gb])

  // Simulated node representation for ResourceSliders
  const simulatedNode: ByogNode = {
    id: 1,
    node_id: nodeId || 'local-worker',
    user_id: 1,
    pairing_code: code,
    device_name: state?.device_name || 'Worker Node',
    // The hardware of the computer is reported by the client. A value that the
    // client does not report is unknown here and it is not invented: a page
    // that shows eight gigabytes of video memory of a computer without a
    // graphics adapter lies about the computer of the user.
    device_type: (state?.device_type as ByogNode['device_type']) || 'desktop',
    os: '',
    arch: '',
    hwid: '',
    cpu_model: state?.cpu_model || '',
    gpu_model: state?.gpu_model || '',
    npu_model: '',
    cpu_cores: state?.cpu_cores ?? 8,
    physical_ram_mb: state?.physical_ram_mb ?? 0,
    swap_ram_mb: state?.swap_ram_mb ?? 0,
    vram_mb: state?.vram_mb ?? 0,
    shared_vram_mb: state?.shared_vram_mb ?? 0,
    tee_supported: false,
    tee_type: 'none',
    bandwidth_down_mbps: state?.bandwidth_down_mbps ?? 0,
    bandwidth_up_mbps: state?.bandwidth_up_mbps ?? 0,
    ping_ms: state?.ping_ms ?? 0,
    network_type: 'ethernet',
    ip_address: '',
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
    total_disk_gb: (state?.disks && state.disks.length > 0)
      ? state.disks.reduce((s, d) => s + (d.total_gb || 0), 0)
      : (state?.total_disk_gb || 0),
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
    cpu_temp: 0,
    gpu_temp: 0,
    battery_temp: 0,
  } as unknown as ByogNode

  // Determine paired status (Device is paired only when authenticated or confirmed by backend/bridge)
  const isPaired = Boolean(state?.paired || (state?.user_id && state.user_id > 0))
  const isModelReady = state?.model_status === 'ready' || (state && state.download_progress >= 100)

  // Header Brand Logo: prioritize custom site logo/favicon or favicon_base64
  const brandLogo = useMemo(() => {
    if (state?.favicon_base64) {
      return state.favicon_base64.startsWith('data:')
        ? state.favicon_base64
        : `data:image/png;base64,${state.favicon_base64}`
    }
    if (state?.favicon && state.favicon !== '/logo.png' && state.favicon !== 'logo.png') {
      return state.favicon
    }
    if (systemFavicon && systemFavicon !== '/favicon.ico' && systemFavicon !== 'favicon.ico') {
      return systemFavicon
    }
    if (systemLogo && systemLogo !== '/logo.png' && systemLogo !== 'logo.png') {
      return systemLogo
    }
    if (state?.favicon) {
      return state.favicon
    }
    return '/logo.png'
  }, [state?.favicon, state?.favicon_base64, systemFavicon, systemLogo])

  const effectiveSystemName = useMemo(() => {
    if (systemName && systemName !== 'New API' && systemName !== 'BYOG Worker') {
      return systemName
    }
    if (state?.system_name && state.system_name !== 'New API' && state.system_name !== 'BYOG Worker') {
      return state.system_name
    }
    if (state?.app_name && state.app_name !== 'New API') {
      return state.app_name
    }
    return 'BYOG Worker'
  }, [systemName, state?.system_name, state?.app_name])

  // --- VIEW 1: Onboarding / Permissions (Mobile only, shown sequentially before pairing) ---
  const isTestingOnboarding = typeof window !== 'undefined' && new URLSearchParams(window.location.search).get('onboarding') === '1'
  const needsMandatoryPermissions = Boolean(
    bridge && permissions && (!permissions.notifications || !permissions.battery)
  )

  useEffect(() => {
    if (bridge && permissions) {
      if (!permissions.notifications) {
        setOnboardingStep(0)
      } else if (!permissions.battery && onboardingStep === 0) {
        setOnboardingStep(1)
      }
    }
  }, [bridge, permissions, onboardingStep])

  if ((bridge && (!onboardingDone || needsMandatoryPermissions)) || (isTestingOnboarding && !onboardingDone)) {
    const handleNextStep = () => {
      let curPerms = permissions
      if (bridge && typeof bridge.getPermissionStatus === 'function') {
        try {
          const raw = bridge.getPermissionStatus()
          if (raw) {
            curPerms = JSON.parse(raw) as PermissionState
            setPermissions(curPerms)
          }
        } catch {}
      }

      if (onboardingStep === 0) {
        if (!curPerms?.notifications && !isTestingOnboarding) {
          toast.error(
            t(
              'byog.notification_permission_required',
              'Bildirim izni zorunludur. Devam etmek için lütfen izin verin.'
            )
          )
          return
        }
        setOnboardingStep(1)
      } else if (onboardingStep === 1) {
        if (!curPerms?.battery && !isTestingOnboarding) {
          toast.error(
            t(
              'byog.battery_permission_required',
              'Pil optimizasyonu muafiyeti zorunludur. Devam etmek için lütfen izin verin.'
            )
          )
          return
        }
        if (curPerms?.autostart_supported) {
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
              {effectiveSystemName}
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

                    <Button
                      onClick={handleNextStep}
                      disabled={!permissions?.notifications && !isTestingOnboarding}
                      className={!permissions?.notifications && !isTestingOnboarding ? 'opacity-50 cursor-not-allowed' : ''}
                    >
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

                    <Button
                      onClick={handleNextStep}
                      disabled={!permissions?.battery && !isTestingOnboarding}
                      className={!permissions?.battery && !isTestingOnboarding ? 'opacity-50 cursor-not-allowed' : ''}
                    >
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
            {effectiveSystemName}
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
                <Button
                  type='button'
                  variant='outline'
                  size='icon'
                  onClick={() => {
                    if (bridge) {
                      bridge.scanQrCode()
                    } else {
                      setQrModalOpen(true)
                    }
                  }}
                  title={t('byog.scan_qr', 'QR Kod Tara')}
                  className='shrink-0 h-11 w-11 rounded-lg border-primary/30 text-primary hover:bg-primary/10'
                >
                  <QrCode className='size-5' />
                </Button>

                <Input
                  value={code}
                  autoComplete='off'
                  autoCorrect='off'
                  autoCapitalize='off'
                  spellCheck={false}
                  data-form-type='other'
                  data-lpignore='true'
                  onChange={(event) => {
                    const rawInput = event.target.value
                    if (rawInput.trim().startsWith('{') && rawInput.includes('pairing_code')) {
                      setCode(extractPairingCode(rawInput))
                      return
                    }
                    let val = rawInput.toUpperCase().replaceAll(/[^A-Z0-9-]/g, '')
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

        <QrScanModal
          open={qrModalOpen}
          onOpenChange={setQrModalOpen}
          onScan={(scanned) => setCode(extractPairingCode(scanned))}
        />
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
              {effectiveSystemName}
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
              <div className='font-mono font-medium truncate'>{effectiveSystemName}</div>
              <div className='text-[10px] opacity-70 font-mono truncate'>Node: {nodeId || state?.node_id || '—'}</div>
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
                      {cleanModel(state?.models) ||
                       cleanModel((state as unknown as Record<string, unknown>)?.installed_models) ||
                       cleanModel(state?.assigned_model) ||
                       (state?.download_model_id ? `${state.download_model_id} (${t('byog.client_downloading', 'İndiriliyor')})` : '—')}
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

                  <div className='flex items-center gap-3 min-w-0'>
                    <Activity className='text-muted-foreground h-4 w-4 shrink-0' />
                    <span className='text-muted-foreground w-24 sm:w-36 shrink-0'>{t('byog.client_ping', 'Gecikme (Ping)')}:</span>
                    <span className='font-mono font-semibold truncate min-w-0 flex-1'>
                      {state?.ping_ms ? `${Math.round(state.ping_ms)} ms` : '—'}
                    </span>
                  </div>

                  <div className='flex items-center gap-3 min-w-0'>
                    <Gauge className='text-muted-foreground h-4 w-4 shrink-0' />
                    <span className='text-muted-foreground w-24 sm:w-36 shrink-0'>{t('byog.client_bandwidth', 'Hız (İndir/Yükle)')}:</span>
                    <span className='font-mono font-semibold truncate min-w-0 flex-1'>
                      ↓ {state?.bandwidth_down_mbps ? state.bandwidth_down_mbps.toFixed(1) : '—'} Mbps / ↑ {state?.bandwidth_up_mbps ? state.bandwidth_up_mbps.toFixed(1) : '—'} Mbps
                    </span>
                  </div>

                  {/* Download progress bar if downloading */}
                  {state && state.download_progress > 0 && state.download_progress < 100 && (
                    <div className='space-y-1.5 pt-3 border-t border-border/50'>
                      <div className='flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground'>
                        <span className='min-w-0 truncate'>
                          {state.download_model_id
                            ? t('byog.downloading_model_named', {
                                defaultValue: '{{model}} indiriliyor...',
                                model: state.download_model_id,
                              })
                            : t('byog.downloading_model', 'Model İndiriliyor...')}
                        </span>
                        <span className='flex items-center gap-2 font-mono'>
                          {state.download_speed_bps ? (
                            <span className='text-muted-foreground'>
                              {formatTransferSpeed(state.download_speed_bps)}
                            </span>
                          ) : null}
                          <span className='text-primary font-bold'>
                            {state.download_progress}%
                          </span>
                        </span>
                      </div>
                      <div className='bg-muted h-2.5 w-full overflow-hidden rounded-full'>
                        <div
                          className='h-full rounded-full bg-primary transition-all duration-300'
                          style={{ width: `${state.download_progress}%` }}
                        />
                      </div>
                      <div className='flex flex-wrap items-center justify-between gap-2 text-[11px] text-muted-foreground'>
                        <span className='font-mono'>
                          {(() => {
                            const dl = state.downloaded_mb || 0
                            let total = state.total_mb || 0
                            if (total <= 0 && state.download_progress > 0 && dl > 0) {
                              total = Math.round(dl / (state.download_progress / 100))
                            }
                            if (dl <= 0 && total <= 0) {
                              return `${state.download_progress}%`
                            }
                            return `${formatMegabytes(dl)} / ${total > 0 ? formatMegabytes(total) : '—'}`
                          })()}
                        </span>
                        {(() => {
                          let eta = state.eta_seconds
                          if ((!eta || eta <= 0) && (state.download_speed_bps && state.download_speed_bps > 0) && (state.total_mb && state.total_mb > (state.downloaded_mb || 0))) {
                            const remainingBytes = (state.total_mb - (state.downloaded_mb || 0)) * 1024 * 1024
                            eta = Math.round(remainingBytes / (state.download_speed_bps / 8))
                          }
                          return eta && eta > 0 ? (
                            <span className='font-mono'>
                              {t('byog.download_remaining', 'Kalan süre')}:{' '}
                              {formatRemainingTime(eta)}
                            </span>
                          ) : null
                        })()}
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
                diskAllocations={diskAllocations}
                onVramChange={handleVramChange}
                onRamChange={handleRamChange}
                onCoresChange={handleCoresChange}
                onDiskChange={handleDiskChange}
                onDiskAllocationsChange={handleDiskAllocationsChange}
                onHotStartChange={handleHotStartChange}
              />

              {/* Mode Selection & Power Matrix */}
              <ScheduleMatrix
                operatingMode={operatingMode}
                powerProfile={powerProfile}
                scheduleConfig={scheduleConfig}
                onOperatingModeChange={(m) => {
                  lastUserEditTimeRef.current = Date.now()
                  setOperatingMode(m)
                }}
                onPowerProfileChange={(p) => {
                  lastUserEditTimeRef.current = Date.now()
                  setPowerProfile(p)
                }}
                onScheduleConfigChange={(sc) => {
                  lastUserEditTimeRef.current = Date.now()
                  setScheduleConfig(sc)
                }}
              />

              <MobileSettingsCard
                deviceType={state?.device_type || 'desktop'}
                wifiOnly={wifiOnly}
                dataLimitMB={dataLimitMB}
                chargingOnly={chargingOnly}
                batteryCutoffEnabled={batteryCutoffEnabled}
                batteryCutoffPercent={batteryCutoffPercent}
                maxBatteryTemp={maxBatteryTemp}
                batteryTempSupported={true}
                autoPauseOnGame={autoPauseOnGame}
                onWifiOnlyChange={setWifiOnly}
                onDataLimitChange={setDataLimitMB}
                onChargingOnlyChange={setChargingOnly}
                onBatteryCutoffEnabledChange={setBatteryCutoffEnabled}
                onBatteryCutoffPercentChange={setBatteryCutoffPercent}
                onMaxBatteryTempChange={setMaxBatteryTemp}
                onAutoPauseOnGameChange={setAutoPauseOnGame}
              />

              <div className='flex items-center justify-between pt-2'>
                {saveSuccessMsg ? (
                  <span className='text-sm text-emerald-500 font-medium flex items-center gap-1.5'>
                    <CheckCircle2 className='size-4' /> {saveSuccessMsg}
                  </span>
                ) : <div />}

                <Button
                  onClick={saveResourceAllocations}
                  disabled={savingResources}
                  className='ml-auto font-semibold'
                >
                  {savingResources ? (
                    <RefreshCw className='size-4 mr-2 animate-spin' />
                  ) : (
                    <Save className='size-4 mr-2' />
                  )}
                  {t('common.save', 'Kaydet')}
                </Button>
              </div>
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
                  autoComplete='off'
                  autoCorrect='off'
                  autoCapitalize='off'
                  spellCheck={false}
                  data-form-type='other'
                  data-lpignore='true'
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
