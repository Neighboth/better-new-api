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
  Smartphone,
} from 'lucide-react'
import { QRCodeSVG } from 'qrcode.react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button, buttonVariants } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

import { pairNode } from '../api'
import type { ByogStatusData } from '../types'

interface PairModalProps {
  onPaired: () => void
  statusData?: ByogStatusData | null
}

export function PairModal({ onPaired }: PairModalProps) {
  const { t } = useTranslation()
  const [open, setDialogOpen] = useState(false)
  const [pairingCode, setPairingCode] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [copiedLink, setCopiedLink] = useState(false)

  const handleDownload = (platform: string, arch: string) => {
    const downloadUrl = `/api/v1/worker/download/${platform}/${arch}`
    const a = document.createElement('a')
    a.href = downloadUrl
    a.setAttribute('download', '')
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
  }

  const handlePair = async () => {
    if (!pairingCode.trim()) {
      toast.error(t('byog.enter_pairing_code_error'))
      return
    }

    try {
      setIsSubmitting(true)
      await pairNode(pairingCode.trim().toUpperCase())
      toast.success(t('byog.pair_success_toast'))
      setDialogOpen(false)
      setPairingCode('')
      onPaired()
    } catch (err: any) {
      toast.error(err.message || t('byog.pair_failed_toast'))
    } finally {
      setIsSubmitting(false)
    }
  }

  const copyUrl = () => {
    navigator.clipboard.writeText(window.location.origin + '/dashboard/byog')
    setCopiedLink(true)
    setTimeout(() => setCopiedLink(false), 2000)
    toast.success(t('common.copied'))
  }

  return (
    <Dialog open={open} onOpenChange={setDialogOpen}>
      <DialogTrigger
        render={
          <Button className='gap-2'>
            <Plus className='h-4 w-4' />
            {t('byog.pair_new_device_button')}
          </Button>
        }
      />

      <DialogContent className='max-w-xl'>
        <DialogHeader>
          <DialogTitle className='flex items-center gap-2'>
            <KeyRound className='text-primary h-5 w-5' />
            {t('byog.connect_new_compute_node')}
          </DialogTitle>
          <DialogDescription>
            {t('byog.connect_new_compute_node_desc')}
          </DialogDescription>
        </DialogHeader>

        <Tabs defaultValue='code' className='w-full'>
          <TabsList className='grid w-full grid-cols-2'>
            <TabsTrigger value='code' className='gap-2'>
              <KeyRound className='h-4 w-4' />
              {t('byog.pairing_code_tab')}
            </TabsTrigger>
            <TabsTrigger value='download' className='gap-2'>
              <Download className='h-4 w-4' />
              {t('byog.download_client_tab')}
            </TabsTrigger>
          </TabsList>

          {/* Pairing Code Tab */}
          <TabsContent value='code' className='space-y-4 pt-4'>
            <div className='bg-card flex flex-col items-center justify-center space-y-3 rounded-xl border p-4 text-center'>
              <div className='rounded-lg bg-white p-3 shadow-sm'>
                <QRCodeSVG
                  value={window.location.origin + '/dashboard/byog'}
                  size={120}
                  level='M'
                />
              </div>
              <div className='text-muted-foreground text-xs'>
                {t('byog.qr_pairing_hint')}
              </div>
            </div>

            <div className='space-y-2'>
              <Label htmlFor='p-code' className='text-sm font-medium'>
                {t('byog.enter_pairing_code_label')}
              </Label>
              <div className='flex gap-2'>
                <Input
                  id='p-code'
                  placeholder='9K4L-28XM-73PQ-51ZR'
                  className='text-center font-mono text-base font-bold tracking-widest uppercase'
                  value={pairingCode}
                  onChange={(e) => setPairingCode(e.target.value)}
                />
                <Button onClick={handlePair} disabled={isSubmitting}>
                  {isSubmitting ? t('Loading') : t('Link Device')}
                </Button>
              </div>
              <p className='text-muted-foreground text-xs'>
                {t('byog.pairing_code_instructions')}
              </p>
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
                      Windows (x64 / ARM64)
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
                      Linux (x86_64 / ARM64)
                    </div>
                    <div className='text-muted-foreground text-xs'>
                      systemd / headless
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
                      macOS (Apple Silicon / Intel)
                    </div>
                    <div className='text-muted-foreground text-xs'>
                      Metal Framework Bundle
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
                      APK Package / Foreground Service
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
            {t('Close')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
