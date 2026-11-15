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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  CheckCircle2,
  CircleX,
  Cloud,
  Copy,
  Eye,
  Gauge,
  Loader2,
  Pencil,
  Plus,
  RefreshCw,
  Trash2,
  Waypoints,
} from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { api } from '@/lib/api'
import {
  Area,
  AreaChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip as ChartTooltip,
  XAxis,
  YAxis,
} from 'recharts'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { SectionPageLayout } from '@/components/layout'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { formatTimestampRelative } from '@/lib/format'
import { cn } from '@/lib/utils'

import {
  assignProxiesEvenly,
  clearAssignments,
  createProxy,
  deleteProxy,
  getProxyDetail,
  listChannels,
  listProxies,
  rebalanceAllChannels,
  testProxy,
  unassignProxy,
  updateProxy,
  type ProxyPoolChannel,
  type ProxyPoolProxy,
} from './api'

type ProxyFormState = {
  id?: number
  name: string
  url: string
  enabled: boolean
}

const EMPTY_FORM: ProxyFormState = { name: '', url: '', enabled: true }

export function ProxyPool() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  const [dialogOpen, setDialogOpen] = useState(false)
  const [form, setForm] = useState<ProxyFormState>(EMPTY_FORM)
  const [deleteTarget, setDeleteTarget] = useState<ProxyPoolProxy | null>(null)
  const [clearTarget, setClearTarget] = useState<ProxyPoolChannel | null>(null)
  const [testingId, setTestingId] = useState<number | null>(null)
  const [detailTarget, setDetailTarget] = useState<ProxyPoolProxy | null>(null)

  const [proxyTab, setProxyTab] = useState<'line' | 'cloudflare'>('line')
  const [cfAccountId, setCfAccountId] = useState('')
  const [cfApiToken, setCfApiToken] = useState('')
  const [cfWorkerName, setCfWorkerName] = useState('better-api-proxy')
  const [cfDeploying, setCfDeploying] = useState(false)

  const copyWorkerScript = async () => {
    try {
      const res = await api.get('/api/proxy-pool/cloudflare-script')
      const script = res.data?.data?.script
      if (script) {
        await navigator.clipboard.writeText(script)
        toast.success(t('Cloudflare Worker scripti panoya kopyalandı'))
      } else {
        toast.error(t('Script alınamadı'))
      }
    } catch {
      toast.error(t('Script alınamadı'))
    }
  }

  const handleDeployWorker = async () => {
    if (!cfAccountId.trim() || !cfApiToken.trim()) {
      toast.error(t('Account ID ve API Token gereklidir'))
      return
    }
    setCfDeploying(true)
    try {
      const res = await api.post('/api/proxy-pool/deploy-cloudflare-worker', {
        account_id: cfAccountId.trim(),
        api_token: cfApiToken.trim(),
        worker_name: cfWorkerName.trim() || 'better-api-proxy',
      })
      if (res.data?.success) {
        toast.success(t('Cloudflare Worker başarıyla deploy edildi ve proxy olarak eklendi'))
        setDialogOpen(false)
        invalidateAll()
      } else {
        toast.error(res.data?.message || t('Deploy başarısız'))
      }
    } catch (err: any) {
      toast.error(err?.response?.data?.message || t('Deploy başarısız'))
    } finally {
      setCfDeploying(false)
    }
  }

  const detailQuery = useQuery({
    queryKey: ['proxy-pool-detail', detailTarget?.id],
    queryFn: async () => {
      if (!detailTarget) return null
      const res = await getProxyDetail(detailTarget.id)
      if (!res.success) {
        throw new Error(res.message || 'failed to load detail')
      }
      return res.data
    },
    enabled: !!detailTarget,
  })

  const proxiesQuery = useQuery({
    queryKey: ['proxy-pool-proxies'],
    queryFn: async () => {
      const res = await listProxies()
      if (!res.success) {
        throw new Error(res.message || 'failed to load proxies')
      }
      return Array.isArray(res.data) ? res.data : []
    },
  })

  const channelsQuery = useQuery({
    queryKey: ['proxy-pool-channels'],
    queryFn: async () => {
      const res = await listChannels()
      if (!res.success) {
        throw new Error(res.message || 'failed to load channels')
      }
      return Array.isArray(res.data) ? res.data : []
    },
  })

  const invalidateAll = () => {
    queryClient.invalidateQueries({ queryKey: ['proxy-pool-proxies'] })
    queryClient.invalidateQueries({ queryKey: ['proxy-pool-channels'] })
  }

  const saveMutation = useMutation({
    mutationFn: async (state: ProxyFormState) => {
      if (state.id !== undefined) {
        return updateProxy(state.id, {
          name: state.name,
          url: state.url,
          enabled: state.enabled,
        })
      }
      return createProxy({ name: state.name, url: state.url, enabled: state.enabled })
    },
    onSuccess: (res) => {
      if (!res.success) {
        toast.error(res.message || t('Proxy could not be saved'))
        return
      }
      toast.success(
        form.id !== undefined ? t('Proxy updated') : t('Proxy added')
      )
      setDialogOpen(false)
      invalidateAll()
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Proxy could not be saved'))
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => deleteProxy(id),
    onSuccess: (res) => {
      if (!res.success) {
        toast.error(res.message || t('Proxy could not be deleted'))
        return
      }
      toast.success(t('Proxy deleted'))
      setDeleteTarget(null)
      invalidateAll()
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Proxy could not be deleted'))
    },
  })

  const assignMutation = useMutation({
    mutationFn: (channelId: number) => assignProxiesEvenly(channelId),
    onSuccess: (res) => {
      if (!res.success) {
        toast.error(res.message || t('Proxies could not be assigned'))
        return
      }
      const data = res.data
      toast.success(
        t('{{assigned}} key(s) bound to {{proxies}} proxies ({{perProxy}} keys per proxy)', {
          assigned: data.assigned,
          proxies: data.proxy_count,
          perProxy: data.per_proxy,
        })
      )
      invalidateAll()
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Proxies could not be assigned'))
    },
  })

  const clearMutation = useMutation({
    mutationFn: (channelId: number) => clearAssignments(channelId),
    onSuccess: (res) => {
      if (!res.success) {
        toast.error(res.message || t('Assignments could not be cleared'))
        return
      }
      toast.success(t('Assignments cleared'))
      setClearTarget(null)
      invalidateAll()
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Assignments could not be cleared'))
    },
  })

  const unassignMutation = useMutation({
    mutationFn: (id: number) => unassignProxy(id),
    onSuccess: (res) => {
      if (!res.success) {
        toast.error(res.message || t('Proxy eşlemesi kaldırılamadı'))
        return
      }
      toast.success(t('Proxy eşlemesi kaldırıldı'))
      queryClient.invalidateQueries({ queryKey: ['proxy-pool-detail'] })
      invalidateAll()
    },
    onError: (err: Error) => {
      toast.error(err.message || t('Proxy eşlemesi kaldırılamadı'))
    },
  })

  const rebalanceMutation = useMutation({
    mutationFn: () => rebalanceAllChannels(),
    onSuccess: (res) => {
      if (!res.success) {
        toast.error(res.message || t('Yeniden dengeleme başarısız'))
        return
      }
      const data = res.data
      toast.success(
        t('{{channels}} kanal, {{keys}} anahtar {{proxies}} proxy arasında eşit olarak dengelendi', {
          channels: data.rebalanced_channels,
          keys: data.total_keys_assigned,
          proxies: data.proxy_count,
        })
      )
      invalidateAll()
    },
    onError: (err: Error) => {
      toast.error(err.message || t('Yeniden dengeleme başarısız'))
    },
  })

  const handleTest = async (id: number) => {
    setTestingId(id)
    try {
      const res = await testProxy(id)
      const data = res.data
      if (data.ok) {
        toast.success(
          t('Proxy reachable — egress IP {{ip}} ({{latency}} ms)', {
            ip: data.egress_ip ?? '?',
            latency: data.latency_ms ?? 0,
          })
        )
      } else {
        toast.error(t('Proxy unreachable: {{error}}', { error: data.error ?? 'unknown' }))
      }
      invalidateAll()
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Proxy could not be tested')
      )
    } finally {
      setTestingId(null)
    }
  }

  const openCreateDialog = () => {
    setForm(EMPTY_FORM)
    setDialogOpen(true)
  }

  const openEditDialog = (proxy: ProxyPoolProxy) => {
    setForm({ id: proxy.id, name: proxy.name, url: proxy.url, enabled: proxy.enabled })
    setDialogOpen(true)
  }

  const submitForm = () => {
    const url = form.url.trim()
    if (!url) {
      toast.error(t('Proxy URL is required'))
      return
    }
    saveMutation.mutate({ ...form, url })
  }

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>
          <span className='inline-flex min-w-0 items-center gap-2'>
            <span className='truncate'>{t('Proxy Pool')}</span>
            <Badge variant='outline' className='shrink-0'>
              Admin
            </Badge>
          </span>
        </SectionPageLayout.Title>
        <SectionPageLayout.Content>
          <div className='mx-auto flex w-full max-w-6xl flex-col gap-6'>
            <Card>
              <CardHeader className='flex flex-row flex-wrap items-center justify-between gap-3'>
                <div>
                  <CardTitle>
                    {t('Proxies')}
                  </CardTitle>
                  <CardDescription className='mt-1.5'>
                    {t(
                      'Register http, https, socks5 or socks5h proxies. Only enabled proxies are used for key assignments.'
                    )}
                  </CardDescription>
                </div>
                <Button onClick={openCreateDialog} className='shrink-0'>
                  <Plus className='mr-2 h-4 w-4' />
                  {t('Add proxy')}
                </Button>
              </CardHeader>
            <CardContent>
              {proxiesQuery.isLoading ? (
                <div className='flex h-24 items-center justify-center text-muted-foreground text-sm'>
                  <Loader2 className='mr-2 h-4 w-4 animate-spin' />
                  {t('Loading')}
                </div>
              ) : proxiesQuery.isError ? (
                <div className='rounded-lg border border-destructive/40 bg-destructive/5 p-4 text-sm'>
                  {t('Proxies could not be loaded')}
                </div>
              ) : (proxiesQuery.data ?? []).length === 0 ? (
                <div className='flex flex-col items-center justify-center gap-2 rounded-lg border border-dashed p-8 text-center'>
                  <Waypoints className='h-8 w-8 text-muted-foreground' />
                  <p className='text-sm font-medium'>{t('No proxies in the pool yet')}</p>
                  <p className='text-muted-foreground text-xs'>
                    {t('Add your first proxy to start routing keys through it.')}
                  </p>
                </div>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('Name')}</TableHead>
                      <TableHead>{t('URL')}</TableHead>
                      <TableHead>{t('Status')}</TableHead>
                      <TableHead>{t('Last test')}</TableHead>
                      <TableHead className='w-[150px] text-right'>
                        {t('Actions')}
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {(proxiesQuery.data ?? []).map((proxy) => (
                      <TableRow key={proxy.id}>
                        <TableCell className='font-medium'>
                          <div className='flex items-center gap-2'>
                            <span>{proxy.name || t('Unnamed')}</span>
                            {proxy.assigned_keys !== undefined && proxy.assigned_keys > 0 ? (
                              <Badge
                                variant='outline'
                                className='border-amber-300 bg-amber-100 text-amber-800 dark:border-amber-500/40 dark:bg-amber-500/10 dark:text-amber-300 font-medium text-[11px]'
                              >
                                {proxy.assigned_keys} / Eşli
                              </Badge>
                            ) : (
                              <Badge
                                variant='secondary'
                                className='bg-muted text-muted-foreground font-normal text-[11px]'
                              >
                                {t('Yedek')}
                              </Badge>
                            )}
                          </div>
                        </TableCell>
                        <TableCell
                          className='max-w-[280px] truncate font-mono text-xs'
                          title={proxy.url}
                        >
                          {proxy.url}
                        </TableCell>
                        <TableCell>
                          {proxy.enabled ? (
                            <Badge variant='outline' className='text-emerald-600'>
                              {t('Enabled')}
                            </Badge>
                          ) : (
                            <Badge variant='secondary'>{t('Disabled')}</Badge>
                          )}
                        </TableCell>
                        <TableCell>
                          {proxy.last_tested_at === 0 ? (
                            <span className='text-muted-foreground text-xs'>
                              {t('Never')}
                            </span>
                          ) : proxy.last_test_status === 'ok' ? (
                            <span
                              className='text-xs text-emerald-600'
                              title={
                                proxy.last_test_error ||
                                formatTimestampRelative(proxy.last_tested_at)
                              }
                            >
                              {t('OK')} · {formatTimestampRelative(proxy.last_tested_at)}
                            </span>
                          ) : (
                            <span
                              className='text-xs text-destructive'
                              title={proxy.last_test_error}
                            >
                              {t('Fail')} · {formatTimestampRelative(proxy.last_tested_at)}
                            </span>
                          )}
                        </TableCell>
                        <TableCell>
                          <div className='flex justify-end gap-1'>
                            <TooltipProvider>
                              <Tooltip>
                                <TooltipTrigger
                                  render={
                                    <Button
                                      size='icon'
                                      variant='ghost'
                                      disabled={testingId === proxy.id}
                                      onClick={() => void handleTest(proxy.id)}
                                      aria-label={t('Test proxy speed')}
                                    >
                                      {testingId === proxy.id ? (
                                        <Loader2 className='h-4 w-4 animate-spin' />
                                      ) : (
                                        <Gauge className='h-4 w-4' />
                                      )}
                                    </Button>
                                  }
                                />
                                <TooltipContent>{t('Test speed / egress')}</TooltipContent>
                              </Tooltip>
                              <Tooltip>
                                <TooltipTrigger
                                  render={
                                    <Button
                                      size='icon'
                                      variant='ghost'
                                      onClick={() => setDetailTarget(proxy)}
                                      aria-label={t('Proxy details')}
                                    >
                                      <Eye className='h-4 w-4' />
                                    </Button>
                                  }
                                />
                                <TooltipContent>{t('Proxy details & usage')}</TooltipContent>
                              </Tooltip>
                              <Tooltip>
                                <TooltipTrigger
                                  render={
                                    <Button
                                      size='icon'
                                      variant='ghost'
                                      onClick={() => openEditDialog(proxy)}
                                      aria-label={t('Edit proxy')}
                                    >
                                      <Pencil className='h-4 w-4' />
                                    </Button>
                                  }
                                />
                                <TooltipContent>{t('Edit')}</TooltipContent>
                              </Tooltip>
                              <Tooltip>
                                <TooltipTrigger
                                  render={
                                    <Button
                                      size='icon'
                                      variant='ghost'
                                      onClick={() => setDeleteTarget(proxy)}
                                      aria-label={t('Delete proxy')}
                                    >
                                      <Trash2 className='h-4 w-4' />
                                    </Button>
                                  }
                                />
                                <TooltipContent>{t('Delete')}</TooltipContent>
                              </Tooltip>
                            </TooltipProvider>
                          </div>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardHeader className='flex flex-row flex-wrap items-center justify-between gap-3'>
              <div>
                <CardTitle>
                  {t('Multi-key channel assignments')}
                </CardTitle>
                <CardDescription className='mt-1.5'>
                  {t(
                    'Keys are handed out round-robin across enabled proxies. If a channel has more keys than you have proxies, each proxy serves multiple keys.'
                  )}
                </CardDescription>
              </div>
              <Button
                variant='outline'
                size='sm'
                disabled={
                  rebalanceMutation.isPending ||
                  (proxiesQuery.data ?? []).filter((p) => p.enabled).length === 0
                }
                onClick={() => rebalanceMutation.mutate()}
                className='shrink-0'
              >
                {rebalanceMutation.isPending ? (
                  <Loader2 className='mr-1.5 h-3.5 w-3.5 animate-spin' />
                ) : (
                  <RefreshCw className='mr-1.5 h-3.5 w-3.5' />
                )}
                {t('Yeniden Dengele')}
              </Button>
            </CardHeader>
            <CardContent>
              {channelsQuery.isLoading ? (
                <div className='flex h-24 items-center justify-center text-muted-foreground text-sm'>
                  <Loader2 className='mr-2 h-4 w-4 animate-spin' />
                  {t('Loading')}
                </div>
              ) : channelsQuery.isError ? (
                <div className='rounded-lg border border-destructive/40 bg-destructive/5 p-4 text-sm'>
                  {t('Channels could not be loaded')}
                </div>
              ) : (channelsQuery.data ?? []).length === 0 ? (
                <p className='text-muted-foreground text-sm'>
                  {t('No multi-key channels found. Create one from the Channels page first.')}
                </p>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('Channel')}</TableHead>
                      <TableHead>{t('Keys')}</TableHead>
                      <TableHead>{t('Assigned')}</TableHead>
                      <TableHead className='w-[220px] text-right'>
                        {t('Actions')}
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {(channelsQuery.data ?? []).map((channel) => (
                      <TableRow key={channel.id}>
                        <TableCell>
                          <div className='font-medium'>{channel.name}</div>
                          <div className='text-muted-foreground text-xs'>#{channel.id}</div>
                        </TableCell>
                        <TableCell>{channel.multi_key_size}</TableCell>
                        <TableCell>
                          <Badge
                            variant={
                              channel.assigned_keys === 0 ? 'secondary' : 'outline'
                            }
                            className={cn(
                              channel.assigned_keys === channel.multi_key_size
                                ? 'text-emerald-600'
                                : undefined
                            )}
                          >
                            {channel.assigned_keys}/{channel.multi_key_size}
                          </Badge>
                        </TableCell>
                        <TableCell>
                          <div className='flex justify-end gap-1'>
                            <Button
                              size='sm'
                              variant='outline'
                              disabled={
                                assignMutation.isPending ||
                                (proxiesQuery.data ?? []).filter((p) => p.enabled).length === 0
                              }
                              onClick={() => assignMutation.mutate(channel.id)}
                            >
                              {assignMutation.isPending ? (
                                <Loader2 className='mr-1 h-3.5 w-3.5 animate-spin' />
                              ) : assignedIconFor(channel)}
                              {t('Assign evenly')}
                            </Button>
                            <Button
                              size='sm'
                              variant='outline'
                              disabled={clearMutation.isPending || channel.assigned_keys === 0}
                              onClick={() => setClearTarget(channel)}
                            >
                              <Trash2 className='mr-1.5 h-3.5 w-3.5 text-muted-foreground' />
                              {t('Dağıtımı Geri Al')}
                            </Button>
                          </div>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </CardContent>
          </Card>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className='max-w-lg'>
          <DialogHeader>
            <DialogTitle>
              {form.id !== undefined ? t('Edit proxy') : t('Add proxy')}
            </DialogTitle>
            <DialogDescription>
              {t('HTTP/HTTPS/SOCKS5 proxy veya Cloudflare Worker proxy ekleyin.')}
            </DialogDescription>
          </DialogHeader>

          {form.id === undefined && (
            <Tabs value={proxyTab} onValueChange={(v) => setProxyTab(v as any)} className='w-full'>
              <TabsList className='grid grid-cols-2 w-full'>
                <TabsTrigger value='line'>{t('Proxy Satırı')}</TabsTrigger>
                <TabsTrigger value='cloudflare'>{t('Cloudflare Worker')}</TabsTrigger>
              </TabsList>

              <TabsContent value='line' className='mt-3'>
                <div className='flex flex-col gap-4 py-1'>
                  <div className='flex flex-col gap-2'>
                    <Label htmlFor='proxy-name'>{t('Name')}</Label>
                    <Input
                      id='proxy-name'
                      value={form.name}
                      onChange={(event) => setForm({ ...form, name: event.target.value })}
                      placeholder={t('Residential DE 01')}
                      autoComplete='off'
                    />
                  </div>
                  <div className='flex flex-col gap-2'>
                    <Label htmlFor='proxy-url'>{t('URL')}</Label>
                    <Input
                      id='proxy-url'
                      value={form.url}
                      onChange={(event) => setForm({ ...form, url: event.target.value })}
                      placeholder='http://user:password@host:port'
                      autoComplete='off'
                      spellCheck={false}
                    />
                  </div>
                  <div className='flex items-center justify-between rounded-lg border p-3'>
                    <div>
                      <Label htmlFor='proxy-enabled' className='cursor-pointer'>
                        {t('Enabled')}
                      </Label>
                      <p className='text-muted-foreground text-xs'>
                        {t('Disabled proxies keep their URLs but receive no keys.')}
                      </p>
                    </div>
                    <Switch
                      id='proxy-enabled'
                      checked={form.enabled}
                      onCheckedChange={(checked) => setForm({ ...form, enabled: checked })}
                    />
                  </div>
                </div>
              </TabsContent>

              <TabsContent value='cloudflare' className='mt-3 space-y-4'>
                <div className='rounded-xl border p-3 bg-muted/20 space-y-2'>
                  <div className='flex items-center justify-between'>
                    <div className='text-xs font-semibold'>{t('1. Worker Scripti')}</div>
                    <Button size='sm' variant='outline' type='button' onClick={() => void copyWorkerScript()}>
                      <Copy className='me-1.5 h-3.5 w-3.5' />
                      {t('Scripti Kopyala')}
                    </Button>
                  </div>
                  <p className='text-xs text-muted-foreground'>
                    {t('Cloudflare panelinde yeni bir Worker açıp bu scripti yapıştırarak yayınlayabilirsiniz.')}
                  </p>
                </div>

                <div className='rounded-xl border p-3 space-y-3'>
                  <div className='text-xs font-semibold'>{t('2. Tek Tıkla Otomatik Deploy')}</div>
                  <div className='space-y-2'>
                    <Label className='text-xs'>{t('Cloudflare Account ID')}</Label>
                    <Input
                      className='text-xs font-mono'
                      value={cfAccountId}
                      onChange={(e) => setCfAccountId(e.target.value)}
                      placeholder='e.g. 1a2b3c4d5e...'
                    />
                  </div>
                  <div className='space-y-2'>
                    <Label className='text-xs'>{t('Cloudflare API Token')}</Label>
                    <Input
                      className='text-xs font-mono'
                      type='password'
                      value={cfApiToken}
                      onChange={(e) => setCfApiToken(e.target.value)}
                      placeholder='Workers Edit izinli token'
                    />
                  </div>
                  <div className='space-y-2'>
                    <Label className='text-xs'>{t('Worker Adı')}</Label>
                    <Input
                      className='text-xs'
                      value={cfWorkerName}
                      onChange={(e) => setCfWorkerName(e.target.value)}
                      placeholder='better-api-proxy'
                    />
                  </div>
                  <Button
                    type='button'
                    className='w-full'
                    disabled={cfDeploying}
                    onClick={() => void handleDeployWorker()}
                  >
                    {cfDeploying ? (
                      <>
                        <Loader2 className='me-2 h-4 w-4 animate-spin' />
                        {t('Deploy Ediliyor...')}
                      </>
                    ) : (
                      <>
                        <Cloud className='me-2 h-4 w-4' />
                        {t('Cloudflare Workers\'a Deploy Et & Bağla')}
                      </>
                    )}
                  </Button>
                </div>

                <div className='space-y-2'>
                  <Label className='text-xs'>{t('Veya Var Olan Worker URL\'si Girin')}</Label>
                  <Input
                    className='text-xs font-mono'
                    value={form.url}
                    onChange={(e) => setForm({ ...form, url: e.target.value, name: form.name || 'Cloudflare Worker' })}
                    placeholder='https://my-proxy.workers.dev'
                  />
                </div>
              </TabsContent>
            </Tabs>
          )}

          {form.id !== undefined && (
            <div className='flex flex-col gap-4 py-2'>
              <div className='flex flex-col gap-2'>
                <Label htmlFor='proxy-name'>{t('Name')}</Label>
                <Input
                  id='proxy-name'
                  value={form.name}
                  onChange={(event) => setForm({ ...form, name: event.target.value })}
                  placeholder={t('Residential DE 01')}
                  autoComplete='off'
                />
              </div>
              <div className='flex flex-col gap-2'>
                <Label htmlFor='proxy-url'>{t('URL')}</Label>
                <Input
                  id='proxy-url'
                  value={form.url}
                  onChange={(event) => setForm({ ...form, url: event.target.value })}
                  placeholder='http://user:password@host:port'
                  autoComplete='off'
                  spellCheck={false}
                />
              </div>
              <div className='flex items-center justify-between rounded-lg border p-3'>
                <div>
                  <Label htmlFor='proxy-enabled' className='cursor-pointer'>
                    {t('Enabled')}
                  </Label>
                  <p className='text-muted-foreground text-xs'>
                    {t('Disabled proxies keep their URLs but receive no keys.')}
                  </p>
                </div>
                <Switch
                  id='proxy-enabled'
                  checked={form.enabled}
                  onCheckedChange={(checked) => setForm({ ...form, enabled: checked })}
                />
              </div>
            </div>
          )}

          <DialogFooter>
            <Button variant='ghost' onClick={() => setDialogOpen(false)}>
              {t('Cancel')}
            </Button>
            {proxyTab === 'line' || form.id !== undefined || form.url.trim() ? (
              <Button disabled={saveMutation.isPending} onClick={submitForm}>
                {saveMutation.isPending && (
                  <Loader2 className='mr-2 h-4 w-4 animate-spin' />
                )}
                {t('Save')}
              </Button>
            ) : null}
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null)
        }}
        title={t('Delete proxy?')}
        desc={t(
          'Deleting a proxy also removes its key assignments. Keys bound to it will use the channel’s default path again.'
        )}
        destructive
        isLoading={deleteMutation.isPending}
        handleConfirm={() => {
          if (deleteTarget) {
            deleteMutation.mutate(deleteTarget.id)
          }
        }}
        confirmText={t('Delete')}
      />

      <ConfirmDialog
        open={clearTarget !== null}
        onOpenChange={(open) => {
          if (!open) setClearTarget(null)
        }}
        title={t('Clear assignments?')}
        desc={t(
          'All per-key proxy assignments of “{{name}}” will be removed. Keys will fall back to the channel’s default path.',
          { name: clearTarget?.name }
        )}
        destructive
        isLoading={clearMutation.isPending}
        handleConfirm={() => {
          if (clearTarget) {
            clearMutation.mutate(clearTarget.id)
          }
        }}
        confirmText={t('Clear')}
      />

      <Dialog
        open={detailTarget !== null}
        onOpenChange={(open) => {
          if (!open) setDetailTarget(null)
        }}
      >
        <DialogContent className='max-w-2xl max-h-[85vh] overflow-y-auto'>
          <DialogHeader>
            <DialogTitle className='flex items-center gap-2'>
              <span>{detailTarget?.name || t('Proxy Details')}</span>
              {detailQuery.data?.assigned_keys !== undefined &&
                detailQuery.data.assigned_keys > 0 && (
                  <Badge
                    variant='outline'
                    className='border-amber-300 bg-amber-100 text-amber-800 dark:border-amber-500/40 dark:bg-amber-500/10 dark:text-amber-300 font-medium text-xs'
                  >
                    {detailQuery.data.assigned_keys} / Eşli
                  </Badge>
                )}
            </DialogTitle>
            <DialogDescription
              className='font-mono text-xs truncate'
              title={detailTarget?.url}
            >
              {detailTarget?.url}
            </DialogDescription>
          </DialogHeader>

          {detailQuery.isLoading ? (
            <div className='flex h-32 items-center justify-center text-muted-foreground text-sm'>
              <Loader2 className='mr-2 h-4 w-4 animate-spin' />
              {t('Loading details...')}
            </div>
          ) : detailQuery.data ? (
            <div className='flex flex-col gap-4 py-2'>
              {/* Stat Cards */}
              <div className='grid grid-cols-2 sm:grid-cols-4 gap-3'>
                <div className='rounded-lg border bg-muted/20 p-3'>
                  <div className='text-muted-foreground text-xs font-medium'>
                    {t('Ping / Hız')}
                  </div>
                  <div className='mt-1 text-base font-bold font-mono'>
                    {detailQuery.data.proxy.latency_ms
                      ? `${detailQuery.data.proxy.latency_ms} ms`
                      : '-'}
                  </div>
                </div>
                <div className='rounded-lg border bg-muted/20 p-3'>
                  <div className='text-muted-foreground text-xs font-medium'>
                    {t('Çıkış IP (Egress)')}
                  </div>
                  <div
                    className='mt-1 text-sm font-semibold font-mono truncate'
                    title={detailQuery.data.proxy.egress_ip}
                  >
                    {detailQuery.data.proxy.egress_ip || '-'}
                  </div>
                </div>
                <div className='rounded-lg border bg-muted/20 p-3'>
                  <div className='text-muted-foreground text-xs font-medium'>
                    {t('Eşli Anahtarlar')}
                  </div>
                  <div className='mt-1 text-base font-bold font-mono text-amber-600 dark:text-amber-400'>
                    {detailQuery.data.assigned_keys}
                  </div>
                </div>
                <div className='rounded-lg border bg-muted/20 p-3'>
                  <div className='text-muted-foreground text-xs font-medium'>
                    {t('Durum')}
                  </div>
                  <div className='mt-1 text-sm font-semibold'>
                    {detailQuery.data.proxy.enabled ? (
                      <span className='text-emerald-600 dark:text-emerald-400'>
                        {t('Aktif')}
                      </span>
                    ) : (
                      <span className='text-muted-foreground'>{t('Pasif')}</span>
                    )}
                  </div>
                </div>
              </div>

              {/* 30 Day Usage Chart */}
              <div className='rounded-lg border p-4'>
                <div className='text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-3'>
                  {t('Son 30 Günlük Veri Kullanımı (MB)')}
                </div>
                <div className='h-44 w-full'>
                  <ResponsiveContainer width='100%' height='100%'>
                    <AreaChart
                      data={detailQuery.data.usage_history}
                      margin={{ top: 5, right: 10, left: -20, bottom: 0 }}
                    >
                      <CartesianGrid strokeDasharray='3 3' className='stroke-muted' />
                      <XAxis dataKey='date' tick={{ fontSize: 10 }} tickLine={false} />
                      <YAxis tick={{ fontSize: 10 }} unit='M' tickLine={false} />
                      <ChartTooltip
                        formatter={(val: any) => [
                          `${Number(val).toFixed(2)} MB`,
                          t('Kullanım'),
                        ]}
                      />
                      <Area
                        type='monotone'
                        dataKey='mb'
                        stroke='#3b82f6'
                        fill='#3b82f6'
                        fillOpacity={0.2}
                      />
                    </AreaChart>
                  </ResponsiveContainer>
                </div>
              </div>

              {/* Assigned Keys Table */}
              <div className='rounded-lg border p-4'>
                <div className='flex items-center justify-between mb-2'>
                  <div className='text-xs font-semibold uppercase tracking-wider text-muted-foreground'>
                    {t('Atanmış Kanallar ve Anahtarlar')}
                  </div>
                  {detailQuery.data.assigned_keys > 0 && (
                    <Button
                      size='sm'
                      variant='outline'
                      className='text-destructive border-destructive/30 hover:bg-destructive/10 h-7 text-xs'
                      disabled={unassignMutation.isPending}
                      onClick={() => unassignMutation.mutate(detailTarget!.id)}
                    >
                      {unassignMutation.isPending ? (
                        <Loader2 className='mr-1.5 h-3 w-3 animate-spin' />
                      ) : (
                        <Trash2 className='mr-1.5 h-3 w-3' />
                      )}
                      {t('Eşlemeyi Kaldır')}
                    </Button>
                  )}
                </div>
                {detailQuery.data.assignments.length === 0 ? (
                  <p className='text-muted-foreground text-xs py-3 text-center'>
                    {t('Bu proxy şu anda herhangi bir kanal anahtarına atanmamış.')}
                  </p>
                ) : (
                  <div className='max-h-48 overflow-y-auto'>
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead className='text-xs'>{t('Kanal Adı')}</TableHead>
                          <TableHead className='text-xs'>{t('Kanal ID')}</TableHead>
                          <TableHead className='text-xs'>
                            {t('Anahtar Sırası')}
                          </TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {detailQuery.data.assignments.map((item, idx) => (
                          <TableRow
                            key={`${item.channel_id}-${item.key_index}-${idx}`}
                          >
                            <TableCell className='text-xs font-medium'>
                              {item.channel_name || `#${item.channel_id}`}
                            </TableCell>
                            <TableCell className='text-xs font-mono text-muted-foreground'>
                              #{item.channel_id}
                            </TableCell>
                            <TableCell className='text-xs font-mono'>
                              {t('Anahtar')} #{item.key_index + 1}
                            </TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </div>
                )}
              </div>
            </div>
          ) : null}

          <DialogFooter>
            <Button variant='outline' onClick={() => setDetailTarget(null)}>
              {t('Kapat')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}

function assignedIconFor(channel: ProxyPoolChannel) {
  return channel.assigned_keys === channel.multi_key_size ? (
    <CheckCircle2 className='mr-1 h-3.5 w-3.5 text-emerald-600' />
  ) : (
    <CircleX className='mr-1 h-3.5 w-3.5 text-muted-foreground' />
  )
}
