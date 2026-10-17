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
import {
  Apple,
  Check,
  Copy,
  Download,
  KeyRound,
  Laptop,
  Monitor,
  Plus,
  QrCode,
  RefreshCw,
  Smartphone,
} from 'lucide-react'
import { QRCodeSVG } from 'qrcode.react'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

import { getUserNodes, getUserPairingCode } from '../api'
import type { ByogStatusData } from '../types'

interface PairModalProps {
  onPaired: () => void
  statusData?: ByogStatusData | null
}

export function PairModal({ onPaired }: PairModalProps) {
  const { t } = useTranslation()
  const [open, setDialogOpen] = useState(false)
  const [sessionCode, setSessionCode] = useState('')
  const [sessionServerUrl, setSessionServerUrl] = useState('')
  const [appName, setAppName] = useState('')
  const [copiedCode, setCopiedCode] = useState(false)
  const [isLoadingCode, setIsLoadingCode] = useState(false)
  const initialNodeCountRef = useRef<number>(-1)

  const fetchSessionCode = async () => {
    setIsLoadingCode(true)
    try {
      const data = await getUserPairingCode()
      setSessionCode(data.pairing_code)
      setSessionServerUrl(data.server_url)
      setAppName(data.app_name)
    } catch {
      toast.error(t('byog.failed_to_get_pairing_code', 'Failed to retrieve pairing code'))
    } finally {
      setIsLoadingCode(false)
    }
  }

  useEffect(() => {
    if (!open) {
      initialNodeCountRef.current = -1
      return
    }

    fetchSessionCode()

    getUserNodes()
      .then((nodes) => {
        initialNodeCountRef.current = nodes.length
      })
      .catch(() => {
        initialNodeCountRef.current = 0
      })

    const interval = setInterval(async () => {
      try {
        const nodes = await getUserNodes()
        if (
          initialNodeCountRef.current >= 0 &&
          nodes.length > initialNodeCountRef.current
        ) {
          toast.success(
            t('byog.pair_success_toast', 'Device paired successfully!')
          )
          setDialogOpen(false)
          onPaired()
        }
      } catch {
        // Ignore background polling errors
      }
    }, 2500)

    return () => clearInterval(interval)
  }, [open])

  const handleDownload = (platform: string, arch: string) => {
    const downloadUrl = `/api/v1/worker/download/${platform}/${arch}`
    const a = document.createElement('a')
    a.href = downloadUrl
    a.setAttribute('download', '')
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
  }

  const copyCode = () => {
    if (!sessionCode) return
    navigator.clipboard.writeText(sessionCode)
    setCopiedCode(true)
    setTimeout(() => setCopiedCode(false), 2000)
    toast.success(t('common.copied', 'Copied to clipboard'))
  }

  const qrPayload = JSON.stringify({
    server_url: sessionServerUrl || window.location.origin,
    pairing_code: sessionCode,
  })

  return (
    <Dialog open={open} onOpenChange={setDialogOpen}>
      <DialogTrigger
        render={
          <Button className='gap-2'>
            <Plus className='h-4 w-4' />
            {t('byog.pair_new_device_button', 'Link Device')}
          </Button>
        }
      />

      <DialogContent className='max-w-xl'>
        <DialogHeader>
          <DialogTitle className='flex items-center gap-2'>
            <KeyRound className='text-primary h-5 w-5' />
            {t('byog.connect_new_compute_node', 'Link Compute Node')}
          </DialogTitle>
          <DialogDescription>
            {t(
              'byog.connect_new_compute_node_desc',
              'Connect your smartphone, tablet, or PC to contribute hardware resources and earn rewards.'
            )}
          </DialogDescription>
        </DialogHeader>

        <Tabs defaultValue='pair' className='w-full'>
          <TabsList className='grid w-full grid-cols-2'>
            <TabsTrigger value='pair' className='gap-2'>
              <QrCode className='h-4 w-4' />
              {t('byog.pairing_tab', 'Pair Device')}
            </TabsTrigger>
            <TabsTrigger value='download' className='gap-2'>
              <Download className='h-4 w-4' />
              {t('byog.download_client_tab', 'Download Client')}
            </TabsTrigger>
          </TabsList>

          {/* Unified Pairing Tab */}
          <TabsContent value='pair' className='space-y-4 pt-4'>
            <div className='bg-card flex flex-col items-center justify-center space-y-4 rounded-xl border p-5 text-center'>
              <div className='relative rounded-xl bg-white p-3 shadow-md'>
                {isLoadingCode || !sessionCode ? (
                  <div className='flex h-[150px] w-[150px] items-center justify-center'>
                    <RefreshCw className='h-6 w-6 animate-spin text-slate-500' />
                  </div>
                ) : (
                  <QRCodeSVG value={qrPayload} size={150} level='M' />
                )}
              </div>

              <div className='text-muted-foreground text-xs max-w-sm'>
                {t(
                  'byog.qr_pairing_hint',
                  'Scan this QR code with the mobile worker app to connect instantly.'
                )}
              </div>
            </div>

            {/* Prominent Pairing Code Box */}
            <div className='rounded-xl border bg-slate-900/40 p-4 text-center space-y-2'>
              <div className='text-xs font-medium text-muted-foreground uppercase tracking-wider'>
                {t('byog.enter_pairing_code_label', 'Pairing Code')}
              </div>
              <div className='flex items-center justify-center gap-3'>
                <span className='font-mono text-xl sm:text-2xl font-bold tracking-widest text-primary selection:bg-primary/20'>
                  {sessionCode || '---- ---- ---- ----'}
                </span>
                <Button
                  size='sm'
                  variant='outline'
                  className='h-8 w-8 p-0'
                  onClick={copyCode}
                  disabled={!sessionCode}
                >
                  {copiedCode ? (
                    <Check className='h-4 w-4 text-emerald-500' />
                  ) : (
                    <Copy className='h-4 w-4' />
                  )}
                </Button>
              </div>
              <p className='text-muted-foreground text-xs'>
                {t(
                  'byog.pairing_code_instructions',
                  'Enter this code into your desktop or mobile worker app to pair.'
                )}
              </p>
            </div>

            {/* Live Polling Status */}
            <div className='flex items-center justify-center gap-2 text-xs text-muted-foreground pt-1'>
              <span className='relative flex h-2 w-2'>
                <span className='animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75'></span>
                <span className='relative inline-flex rounded-full h-2 w-2 bg-emerald-500'></span>
              </span>
              <span>
                {t('byog.waiting_for_connection', 'Waiting for device connection...')}
              </span>
            </div>
          </TabsContent>

          {/* Download Client Tab */}
          <TabsContent value='download' className='space-y-3 pt-4'>
            <div className='grid grid-cols-1 gap-2.5 sm:grid-cols-2'>
              {/* Windows */}
              <div className='bg-card flex items-center justify-between rounded-lg border p-3'>
                <div className='flex items-center gap-2.5'>
                  <Monitor className='h-5 w-5 text-blue-500' />
                  <div>
                    <div className='text-sm font-semibold'>
                      Windows (x64)
                    </div>
                    <div className='text-muted-foreground text-xs'>
                      .exe (System Tray Daemon)
                    </div>
                  </div>
                </div>
                <Button
                  size='sm'
                  variant='outline'
                  onClick={() => handleDownload('windows', 'amd64')}
                >
                  <Download className='h-3.5 w-3.5' />
                </Button>
              </div>

              {/* Linux */}
              <div className='bg-card flex items-center justify-between rounded-lg border p-3'>
                <div className='flex items-center gap-2.5'>
                  <Laptop className='h-5 w-5 text-amber-500' />
                  <div>
                    <div className='text-sm font-semibold'>
                      Linux (x86_64)
                    </div>
                    <div className='text-muted-foreground text-xs'>
                      Headless / systemd
                    </div>
                  </div>
                </div>
                <Button
                  size='sm'
                  variant='outline'
                  onClick={() => handleDownload('linux', 'amd64')}
                >
                  <Download className='h-3.5 w-3.5' />
                </Button>
              </div>

              {/* macOS */}
              <div className='bg-card flex items-center justify-between rounded-lg border p-3'>
                <div className='flex items-center gap-2.5'>
                  <Apple className='h-5 w-5 text-slate-300' />
                  <div>
                    <div className='text-sm font-semibold'>
                      macOS (Apple Silicon)
                    </div>
                    <div className='text-muted-foreground text-xs'>
                      ARM64 Binary
                    </div>
                  </div>
                </div>
                <Button
                  size='sm'
                  variant='outline'
                  onClick={() => handleDownload('darwin', 'arm64')}
                >
                  <Download className='h-3.5 w-3.5' />
                </Button>
              </div>

              {/* Mobile Android */}
              <div className='bg-card flex items-center justify-between rounded-lg border p-3'>
                <div className='flex items-center gap-2.5'>
                  <Smartphone className='h-5 w-5 text-emerald-500' />
                  <div>
                    <div className='text-sm font-semibold'>
                      Android & Tablets
                    </div>
                    <div className='text-muted-foreground text-xs'>
                      APK Package
                    </div>
                  </div>
                </div>
                <Button
                  size='sm'
                  variant='outline'
                  onClick={() => handleDownload('android', 'apk')}
                >
                  <Download className='h-3.5 w-3.5' />
                </Button>
              </div>
            </div>
          </TabsContent>
        </Tabs>

        <DialogFooter>
          <Button variant='outline' onClick={() => setDialogOpen(false)}>
            {t('common.close', 'Close')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
