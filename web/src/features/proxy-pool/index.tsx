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
  AlertTriangle,
  CheckCircle2,
  CircleX,
  Cloud,
  Copy,
  Eye,
  FileText,
  Gauge,
  Info,
  Loader2,
  Pencil,
  Plus,
  Power,
  PowerOff,
  RefreshCw,
  Trash2,
  Upload,
  Waypoints,
  X,
  Timer,
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
import { DISABLED_ROW_DESKTOP } from '@/components/data-table'
import { SectionPageLayout } from '@/components/layout'
import { StatusBadge } from '@/components/status-badge'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
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
import { Textarea } from '@/components/ui/textarea'
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
  batchCreateProxies,
  clearAssignments,
  createProxy,
  deleteProxy,
  getChannelDetail,
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

function getProxyProtocol(proxy: { url?: string; is_cf_worker?: boolean }): string {
  if (proxy.is_cf_worker) return 'Worker'
  const u = (proxy.url || '').toLowerCase()
  if (u.startsWith('socks5h://')) return 'Socks5H'
  if (u.startsWith('socks5://')) return 'Socks5'
  if (u.startsWith('socks4://')) return 'Socks4'
  if (u.startsWith('https://')) return 'HTTPS'
  if (u.startsWith('http://')) return 'HTTP'
  return 'HTTP'
}

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

  const [proxyTab, setProxyTab] = useState<'line' | 'cloudflare' | 'bulk'>('line')
  const [cfAccountId, setCfAccountId] = useState('')
  const [cfApiToken, setCfApiToken] = useState('')
  const [cfWorkerName, setCfWorkerName] = useState('better-api-proxy')
  const [cfDeploying, setCfDeploying] = useState(false)

  const [bulkBaseName, setBulkBaseName] = useState('Proxy')
  const [bulkText, setBulkText] = useState('')
  const [bulkEnabled, setBulkEnabled] = useState(true)
  const [isBulkSaving, setIsBulkSaving] = useState(false)
  const [bulkProgress, setBulkProgress] = useState<{ done: number; total: number } | null>(null)
  const [bulkFileName, setBulkFileName] = useState<string | null>(null)
  const [bulkFileLines, setBulkFileLines] = useState<string[] | null>(null)
  const [togglingProxyId, setTogglingProxyId] = useState<number | null>(null)
  const [currentPage, setCurrentPage] = useState(1)
  const pageSize = 20

  const [selectedProxyIds, setSelectedProxyIds] = useState<number[]>([])
  const [batchNameDialogOpen, setBatchNameDialogOpen] = useState(false)
  const [batchNameValue, setBatchNameValue] = useState('')
  const [batchDeleteDialogOpen, setBatchDeleteDialogOpen] = useState(false)
  const [isBatchTesting, setIsBatchTesting] = useState(false)
  const [isBatchRenaming, setIsBatchRenaming] = useState(false)
  const [isBatchDeleting, setIsBatchDeleting] = useState(false)

  const handleBatchTest = async () => {
    if (selectedProxyIds.length === 0) return
    setIsBatchTesting(true)
    let okCount = 0
    let failCount = 0
    try {
      await Promise.all(
        selectedProxyIds.map(async (id) => {
          try {
            const res = await testProxy(id)
            if (res.data?.ok) {
              okCount++
            } else {
              failCount++
            }
          } catch {
            failCount++
          }
        })
      )
      toast.success(
        t('Batch test completed: {{ok}} succeeded, {{fail}} failed', {
          ok: okCount,
          fail: failCount,
        })
      )
      void queryClient.invalidateQueries({ queryKey: ['proxy-pool-proxies'] })
    } finally {
      setIsBatchTesting(false)
    }
  }

  const handleBatchRename = async () => {
    const baseName = batchNameValue.trim()
    if (!baseName) {
      toast.error(t('Please enter a name'))
      return
    }
    setIsBatchRenaming(true)
    try {
      await Promise.all(
        selectedProxyIds.map(async (id, idx) => {
          const name = selectedProxyIds.length > 1 ? `${baseName} ${idx + 1}` : baseName
          await updateProxy(id, { name })
        })
      )
      toast.success(t('Renamed {{count}} proxies', { count: selectedProxyIds.length }))
      setBatchNameDialogOpen(false)
      void queryClient.invalidateQueries({ queryKey: ['proxy-pool-proxies'] })
    } catch {
      toast.error(t('Failed to rename proxies'))
    } finally {
      setIsBatchRenaming(false)
    }
  }

  const handleBatchDelete = async () => {
    setIsBatchDeleting(true)
    try {
      await Promise.all(selectedProxyIds.map((id) => deleteProxy(id)))
      toast.success(t('Deleted {{count}} proxies', { count: selectedProxyIds.length }))
      setSelectedProxyIds([])
      setBatchDeleteDialogOpen(false)
      void queryClient.invalidateQueries({ queryKey: ['proxy-pool-proxies'] })
      void queryClient.invalidateQueries({ queryKey: ['proxy-pool-channels'] })
    } catch {
      toast.error(t('Failed to delete proxies'))
    } finally {
      setIsBatchDeleting(false)
    }
  }

  const copyWorkerScript = async () => {
    try {
      const res = await api.get('/api/proxy-pool/cloudflare-script')
      const script = res.data?.data?.script
      if (script) {
        await navigator.clipboard.writeText(script)
        toast.success(t('Cloudflare Worker script copied to clipboard'))
      } else {
        toast.error(t('Failed to fetch script'))
      }
    } catch {
      toast.error(t('Failed to fetch script'))
    }
  }

  const handleDeployWorker = async () => {
    if (!cfAccountId.trim() || !cfApiToken.trim()) {
      toast.error(t('Account ID and API Token are required'))
      return
    }
    setCfDeploying(true)
    try {
      const res = await api.post('/api/proxy-pool/deploy-cloudflare-worker', {
        account_id: cfAccountId.trim(),
        api_token: cfApiToken.trim(),
        worker_name: cfWorkerName.trim() || 'better-api-proxy',
        name: form.name.trim() || 'Cloudflare Worker Proxy',
        enabled: form.enabled,
      })
      if (res.data?.success) {
        toast.success(t('Cloudflare Worker deployed and added as proxy successfully'))
        setDialogOpen(false)
        invalidateAll()
      } else {
        toast.error(res.data?.message || t('Deployment failed'))
      }
    } catch (err: any) {
      toast.error(err?.response?.data?.message || t('Deployment failed'))
    } finally {
      setCfDeploying(false)
    }
  }

  // Shared line parser for the paste area and the file picker.
  const parseBulkLines = (text: string) =>
    text
      .split('\n')
      .map((l) => l.trim())
      .filter((l) => l.length > 0 && !l.startsWith('#'))

  const handleBulkSave = async () => {
    const lines = bulkFileLines ?? parseBulkLines(bulkText)
    if (lines.length === 0) {
      toast.error(t('Please enter at least one proxy'))
      return
    }
    setIsBulkSaving(true)
    setBulkProgress({ done: 0, total: lines.length })
    // Chunked import: the browser never holds one giant request and the
    // server commits small, fast transactions instead of one all-or-nothing
    // import that can run for minutes without feedback.
    const CHUNK = 1000
    let imported = 0
    try {
      for (let start = 0; start < lines.length; start += CHUNK) {
        const slice = lines.slice(start, start + CHUNK)
        const proxies = slice.map((url, idx) => ({
          url,
          name: lines.length === 1 ? (bulkBaseName.trim() || 'Proxy') : `${bulkBaseName.trim() || 'Proxy'}-${start + idx + 1}`,
          enabled: bulkEnabled,
        }))
        const res = await batchCreateProxies({
          base_name: bulkBaseName.trim() || 'Proxy',
          enabled: bulkEnabled,
          proxies,
        })
        if (!res.success) {
          toast.error(res.message || t('Failed to import proxies'))
          return
        }
        imported += proxies.length
        setBulkProgress({ done: imported, total: lines.length })
      }
      toast.success(t('Successfully imported {{count}} proxies', { count: imported }))
      setDialogOpen(false)
      setBulkText('')
      setBulkFileLines(null)
      setBulkFileName(null)
      invalidateAll()
    } catch (err: any) {
      toast.error(err?.response?.data?.message || err?.message || t('Failed to import proxies'))
    } finally {
      setIsBulkSaving(false)
      setBulkProgress(null)
    }
  }

  const handleToggleProxyEnabled = async (proxy: ProxyPoolProxy) => {
    setTogglingProxyId(proxy.id)
    try {
      const res = await updateProxy(proxy.id, { enabled: !proxy.enabled })
      if (!res.success) {
        toast.error(res.message || t('Proxy could not be updated'))
        return
      }
      toast.success(
        !proxy.enabled ? t('Proxy enabled') : t('Proxy disabled')
      )
      invalidateAll()
    } catch (err: any) {
      toast.error(err?.message || t('Proxy could not be updated'))
    } finally {
      setTogglingProxyId(null)
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
    queryKey: ['proxy-pool-proxies', currentPage],
    queryFn: async () => {
      const res = await listProxies({
        page: currentPage,
        page_size: pageSize,
      })
      if (!res.success) {
        throw new Error(res.message || 'failed to load proxies')
      }
      if (Array.isArray(res.data)) {
        return {
          items: res.data,
          total: res.data.length,
          page: 1,
          page_size: res.data.length,
          total_pages: 1,
          enabled_count: res.data.filter((p) => p.enabled).length,
        }
      }
      return res.data
    },
  })

  const proxyList = proxiesQuery.data?.items ?? []
  const totalProxies = proxiesQuery.data?.total ?? 0
  const totalPages = proxiesQuery.data?.total_pages ?? 1
  const enabledCount = proxiesQuery.data?.enabled_count ?? proxyList.filter((p) => p.enabled).length

  const [detailChannelId, setDetailChannelId] = useState<number | null>(null)

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

  const channelDetailQuery = useQuery({
    queryKey: ['proxy-pool-channel-detail', detailChannelId],
    queryFn: async () => {
      if (detailChannelId === null) return null
      const res = await getChannelDetail(detailChannelId)
      if (!res.success) {
        throw new Error(res.message || 'failed to load channel detail')
      }
      return res.data
    },
    enabled: detailChannelId !== null,
    staleTime: 10_000,
  })
  const channelDetailData = channelDetailQuery.data

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
        toast.error(res.message || t('Failed to remove proxy assignment'))
        return
      }
      toast.success(t('Proxy assignment removed'))
      queryClient.invalidateQueries({ queryKey: ['proxy-pool-detail'] })
      invalidateAll()
    },
    onError: (err: Error) => {
      toast.error(err.message || t('Failed to remove proxy assignment'))
    },
  })

  const rebalanceMutation = useMutation({
    mutationFn: () => rebalanceAllChannels(),
    onSuccess: (res) => {
      if (!res.success) {
        toast.error(res.message || t('Rebalance failed'))
        return
      }
      const data = res.data
      toast.success(
        t('Balanced {{keys}} keys across {{proxies}} proxies for {{channels}} channels', {
          channels: data.rebalanced_channels,
          keys: data.total_keys_assigned,
          proxies: data.proxy_count,
        })
      )
      invalidateAll()
    },
    onError: (err: Error) => {
      toast.error(err.message || t('Rebalance failed'))
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
              {(() => {
                if (proxiesQuery.isLoading) {
                  return (
                    <div className='flex h-24 items-center justify-center text-muted-foreground text-sm'>
                      <Loader2 className='mr-2 h-4 w-4 animate-spin' />
                      {t('Loading')}
                    </div>
                  )
                }
                if (proxiesQuery.isError) {
                  return (
                    <div className='rounded-lg border border-destructive/40 bg-destructive/5 p-4 text-sm'>
                      {t('Proxies could not be loaded')}
                    </div>
                  )
                }
                if (totalProxies === 0) {
                  return (
                    <div className='flex flex-col items-center justify-center gap-2 rounded-lg border border-dashed p-8 text-center'>
                      <Waypoints className='h-8 w-8 text-muted-foreground' />
                      <p className='text-sm font-medium'>{t('No proxies in the pool yet')}</p>
                      <p className='text-muted-foreground text-xs'>
                        {t('Add your first proxy to start routing keys through it.')}
                      </p>
                    </div>
                  )
                }
                return (
                <>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead className='w-[40px]'>
                        <Checkbox
                          checked={
                            proxyList.length > 0 &&
                            selectedProxyIds.length === proxyList.length
                          }
                          onCheckedChange={(checked) => {
                            if (checked) {
                              setSelectedProxyIds(proxyList.map((p) => p.id))
                            } else {
                              setSelectedProxyIds([])
                            }
                          }}
                          aria-label={t('Select all')}
                        />
                      </TableHead>
                      <TableHead>{t('Name')}</TableHead>
                      <TableHead>{t('URL')}</TableHead>
                      <TableHead>{t('Type')}</TableHead>
                      <TableHead>{t('Status')}</TableHead>
                      <TableHead>{t('Last test')}</TableHead>
                      <TableHead className='w-[160px] text-right'>
                        {t('Actions')}
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {proxyList.map((proxy) => (
                      <TableRow
                        key={proxy.id}
                        className={cn(
                          !proxy.enabled && DISABLED_ROW_DESKTOP,
                          proxy.enabled && selectedProxyIds.includes(proxy.id) && 'bg-muted/40'
                        )}
                      >
                        <TableCell>
                          <Checkbox
                            checked={selectedProxyIds.includes(proxy.id)}
                            onCheckedChange={(checked) => {
                              setSelectedProxyIds((prev) =>
                                checked ? [...prev, proxy.id] : prev.filter((id) => id !== proxy.id)
                              )
                            }}
                            aria-label={`Select ${proxy.name}`}
                          />
                        </TableCell>
                        <TableCell className='font-medium'>
                          <div className='flex items-center gap-2'>
                            <span>{proxy.name || t('Unnamed')}</span>
                            {proxy.assigned_keys !== undefined && proxy.assigned_keys > 0 ? (
                              <Badge
                                variant='outline'
                                className='font-medium text-[11px]'
                              >
                                {proxy.assigned_keys} / {t('Assigned')}
                              </Badge>
                            ) : (
                              <Badge
                                variant='secondary'
                                className='font-normal text-[11px]'
                              >
                                {t('Backup')}
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
                          <div className='flex items-center gap-1.5'>
                            <Badge variant='outline' className='text-[10px] font-mono px-1.5 py-0 uppercase'>
                              {getProxyProtocol(proxy)}
                            </Badge>
                            {proxy.country_code ? (
                              <img
                                src={`https://flagcdn.com/20x15/${proxy.country_code.toLowerCase()}.png`}
                                alt={proxy.country_code}
                                title={(() => {
                                  let t1 = proxy.country_name || proxy.country_code
                                  if (proxy.city) t1 += ` - ${proxy.city}`
                                  if (proxy.isp) t1 += ` (${proxy.isp})`
                                  return t1
                                })()}
                                className='inline-block h-3.5 w-auto rounded-xs shadow-xs shrink-0'
                                onError={(e) => {
                                  ;(e.target as HTMLElement).style.display = 'none'
                                }}
                              />
                            ) : null}
                          </div>
                        </TableCell>
                        <TableCell>
                          {(() => {
                            if (proxy.enabled) {
                              return (
                                <StatusBadge
                                  label={t('Enabled')}
                                  variant='success'
                                  size='sm'
                                  copyable={false}
                                />
                              )
                            }
                            if (proxy.last_test_status === 'fail') {
                              return (
                                <StatusBadge
                                  label={t('Auto-disabled')}
                                  variant='warning'
                                  size='sm'
                                  copyable={false}
                                />
                              )
                            }
                            return (
                              <StatusBadge
                                label={t('Disabled')}
                                variant='danger'
                                size='sm'
                                copyable={false}
                              />
                            )
                          })()}
                        </TableCell>
                        <TableCell>
                          {(() => {
                            if (proxy.last_tested_at === 0) {
                              return (
                                <span className='text-muted-foreground text-xs'>
                                  {t('Never')}
                                </span>
                              )
                            }
                            if (proxy.last_test_status === 'ok') {
                              return (
                                <span
                                  className='text-xs text-emerald-600'
                                  title={
                                    proxy.last_test_error ||
                                    formatTimestampRelative(proxy.last_tested_at)
                                  }
                                >
                                  {t('OK')} · {formatTimestampRelative(proxy.last_tested_at)}
                                </span>
                              )
                            }
                            return (
                              <span
                                className='text-xs text-destructive'
                                title={proxy.last_test_error}
                              >
                                {t('Fail')} · {formatTimestampRelative(proxy.last_tested_at)}
                              </span>
                            )
                          })()}
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
                                      disabled={togglingProxyId === proxy.id}
                                      onClick={() => void handleToggleProxyEnabled(proxy)}
                                      aria-label={proxy.enabled ? t('Disable') : t('Enable')}
                                      className={
                                        proxy.enabled
                                          ? 'text-destructive hover:text-destructive'
                                          : 'text-emerald-600 hover:text-emerald-600'
                                      }
                                    >
                                      {(() => {
                                        if (togglingProxyId === proxy.id) {
                                          return <Loader2 className='h-4 w-4 animate-spin' />
                                        }
                                        if (proxy.enabled) {
                                          return <PowerOff className='h-4 w-4' />
                                        }
                                        return <Power className='h-4 w-4' />
                                      })()}
                                    </Button>
                                  }
                                />
                                <TooltipContent>
                                  {proxy.enabled ? t('Disable') : t('Enable')}
                                </TooltipContent>
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
                {totalPages > 1 && (
                  <div className='flex flex-wrap items-center justify-between gap-3 px-2 pt-4 border-t'>
                    <div className='text-xs text-muted-foreground'>
                      {t('Showing page {{page}} of {{totalPages}} ({{total}} total proxies)', {
                        page: currentPage,
                        totalPages,
                        total: totalProxies,
                      })}
                    </div>
                    <div className='flex items-center gap-2'>
                      <Button
                        variant='outline'
                        size='sm'
                        disabled={currentPage <= 1 || proxiesQuery.isLoading}
                        onClick={() => setCurrentPage((p) => Math.max(1, p - 1))}
                        className='h-8 text-xs'
                      >
                        {t('Previous')}
                      </Button>
                      <span className='text-xs font-medium px-2'>
                        {currentPage} / {totalPages}
                      </span>
                      <Button
                        variant='outline'
                        size='sm'
                        disabled={currentPage >= totalPages || proxiesQuery.isLoading}
                        onClick={() => setCurrentPage((p) => Math.min(totalPages, p + 1))}
                        className='h-8 text-xs'
                      >
                        {t('Next')}
                      </Button>
                    </div>
                  </div>
                )}
                </>
                )
              })()}
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
                  enabledCount === 0
                }
                onClick={() => rebalanceMutation.mutate()}
                className='shrink-0'
              >
                {rebalanceMutation.isPending ? (
                  <Loader2 className='mr-1.5 h-3.5 w-3.5 animate-spin' />
                ) : (
                  <RefreshCw className='mr-1.5 h-3.5 w-3.5' />
                )}
                {t('Rebalance All')}
              </Button>
            </CardHeader>
            <CardContent>
              {(() => {
                if (channelsQuery.isLoading) {
                  return (
                    <div className='flex h-24 items-center justify-center text-muted-foreground text-sm'>
                      <Loader2 className='mr-2 h-4 w-4 animate-spin' />
                      {t('Loading')}
                    </div>
                  )
                }
                if (channelsQuery.isError) {
                  return (
                    <div className='rounded-lg border border-destructive/40 bg-destructive/5 p-4 text-sm'>
                      {t('Channels could not be loaded')}
                    </div>
                  )
                }
                if ((channelsQuery.data ?? []).length === 0) {
                  return (
                    <p className='text-muted-foreground text-sm'>
                      {t('No multi-key channels found. Create one from the Channels page first.')}
                    </p>
                  )
                }
                return (
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
                    {(channelsQuery.data ?? []).map((channel) => {
                      const isAnon = channel.type === 65 || channel.type === 68 || channel.type === 69
                      const isComplete = isAnon ? channel.assigned_keys > 0 : channel.assigned_keys === channel.multi_key_size
                      return (
                      <TableRow key={channel.id}>
                        <TableCell>
                          <div className='font-medium'>{channel.name}</div>
                          <div className='text-muted-foreground text-xs'>#{channel.id}</div>
                        </TableCell>
                        <TableCell>{isAnon ? '∞' : channel.multi_key_size}</TableCell>
                        <TableCell>
                          <Badge
                            variant={
                              channel.assigned_keys === 0 ? 'secondary' : 'outline'
                            }
                            className={cn(
                              isComplete
                                ? 'text-emerald-600'
                                : undefined
                            )}
                          >
                            {channel.assigned_keys} / {isAnon ? '∞' : channel.multi_key_size}
                          </Badge>
                        </TableCell>
                        <TableCell>
                          <div className='flex justify-end gap-1'>
                            <Button
                              size='sm'
                              variant='outline'
                              onClick={() => setDetailChannelId(channel.id)}
                              aria-label={t('Channel detail')}
                              title={t('Channel detail')}
                            >
                              <Info className='size-3.5' />
                            </Button>
                            <Button
                              size='sm'
                              variant='outline'
                              disabled={
                                assignMutation.isPending ||
                                enabledCount === 0
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
                              {t('Revert Distribution')}
                            </Button>
                          </div>
                        </TableCell>
                      </TableRow>
                    )})}
                  </TableBody>
                </Table>
                )
              })()}
            </CardContent>
          </Card>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className='max-w-lg w-[95vw] sm:w-full max-h-[90vh] sm:max-h-[85vh] flex flex-col p-0 overflow-hidden'>
          <DialogHeader className='p-5 sm:p-6 pb-3 border-b shrink-0'>
            <DialogTitle>
              {form.id !== undefined ? t('Edit proxy') : t('Add proxy')}
            </DialogTitle>
            <DialogDescription>
              {t('Add an HTTP/HTTPS/SOCKS5/SOCKS5H proxy or a Cloudflare Worker proxy.')}
            </DialogDescription>
          </DialogHeader>

          <div className='flex-1 overflow-y-auto px-5 sm:px-6 py-4 space-y-4'>
            {form.id === undefined && (
              <Tabs value={proxyTab} onValueChange={(v) => setProxyTab(v as any)} className='w-full'>
                <TabsList className='grid grid-cols-3 w-full'>
                  <TabsTrigger value='line' className='text-xs'>{t('Proxy URL')}</TabsTrigger>
                  <TabsTrigger value='cloudflare' className='text-xs'>{t('Cloudflare Worker')}</TabsTrigger>
                  <TabsTrigger value='bulk' className='text-xs'>{t('Bulk Import')}</TabsTrigger>
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
                        placeholder='http://user:password@host:port, [ipv6]:port, socks5:// or socks5h://...'
                        autoComplete='off'
                        spellCheck={false}
                      />
                    </div>
                    <div className='flex items-center justify-between rounded-lg border p-3'>
                      <div>
                        <Label htmlFor='proxy-enabled' className='cursor-pointer text-xs font-medium'>
                          {t('Enabled')}
                        </Label>
                        <p className='text-muted-foreground text-[11px]'>
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
                  <div className='flex items-start gap-2.5 rounded-xl border border-amber-500/40 bg-amber-500/10 p-3 text-amber-700 dark:text-amber-400 text-xs'>
                    <AlertTriangle className='size-4 shrink-0 mt-0.5' />
                    <p className='leading-relaxed'>
                      {t('Cloudflare worker proxies may have high latency and IP instability, not recommended for high performance requirements.')}
                    </p>
                  </div>

                  <div className='rounded-xl border p-3 bg-muted/20 space-y-2'>
                    <div className='flex items-center justify-between'>
                      <div className='text-xs font-semibold'>{t('1. Worker Script')}</div>
                      <Button size='sm' variant='outline' type='button' onClick={() => void copyWorkerScript()}>
                        <Copy className='me-1.5 h-3.5 w-3.5' />
                        {t('Copy Script')}
                      </Button>
                    </div>
                    <p className='text-xs text-muted-foreground'>
                      {t('You can create a new Worker in the Cloudflare dashboard and paste this script to deploy it.')}
                    </p>
                  </div>

                  <div className='rounded-xl border p-3 space-y-3'>
                    <div className='text-xs font-semibold'>{t('2. One-Click Auto Deploy')}</div>
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
                        placeholder='Workers Edit token'
                      />
                    </div>
                    <div className='space-y-2'>
                      <Label className='text-xs'>{t('Worker Name')}</Label>
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
                          {t('Deploying...')}
                        </>
                      ) : (
                        <>
                          <Cloud className='me-2 h-4 w-4' />
                          {t('Deploy to Cloudflare Workers & Add')}
                        </>
                      )}
                    </Button>
                  </div>

                  <div className='space-y-2'>
                    <Label className='text-xs'>{t('Or Enter Existing Worker URL')}</Label>
                    <Input
                      className='text-xs font-mono'
                      value={form.url}
                      onChange={(e) => setForm({ ...form, url: e.target.value, name: form.name || 'Cloudflare Worker' })}
                      placeholder='https://my-proxy.workers.dev'
                    />
                  </div>

                  <div className='flex items-center justify-between rounded-lg border p-3'>
                    <div>
                      <Label htmlFor='cf-proxy-enabled' className='cursor-pointer text-xs font-medium'>
                        {t('Enabled')}
                      </Label>
                      <p className='text-muted-foreground text-[11px]'>
                        {t('Disabled proxies keep their URLs but receive no keys.')}
                      </p>
                    </div>
                    <Switch
                      id='cf-proxy-enabled'
                      checked={form.enabled}
                      onCheckedChange={(checked) => setForm({ ...form, enabled: checked })}
                    />
                  </div>
                </TabsContent>

                <TabsContent value='bulk' className='mt-3 space-y-4'>
                  <div className='space-y-2'>
                    <Label htmlFor='bulk-base-name' className='text-xs font-medium'>
                      {t('Base Name')}
                    </Label>
                    <Input
                      id='bulk-base-name'
                      value={bulkBaseName}
                      onChange={(e) => setBulkBaseName(e.target.value)}
                      placeholder='Proxy (Proxy-1, Proxy-2...)'
                      autoComplete='off'
                    />
                  </div>

                  <div className='space-y-2'>
                    <div className='flex items-center justify-between'>
                      <Label htmlFor='bulk-text' className='text-xs font-medium'>
                        {t('Proxies (One per line)')}
                      </Label>
                      <label className='cursor-pointer text-xs text-primary hover:underline flex items-center gap-1'>
                        <Upload className='size-3.5' />
                        <span>{t('Upload .txt file')}</span>
                        <input
                          type='file'
                          accept='.txt,text/plain'
                          className='hidden'
                          onChange={(e) => {
                            const file = e.target.files?.[0]
                            if (file) {
                              const reader = new FileReader()
                              reader.onload = () => {
                                if (typeof reader.result === 'string') {
                                  const parsed = parseBulkLines(reader.result)
                                  setBulkFileLines(parsed)
                                  setBulkFileName(file.name)
                                  setBulkText('')
                                }
                              }
                              reader.readAsText(file)
                            }
                            // allow picking the same file again after removing it
                            e.target.value = ''
                          }}
                        />
                      </label>
                    </div>
                    {bulkFileLines !== null ? (
                      <div className='rounded-md border border-border bg-muted/40 p-3 space-y-2'>
                        <div className='flex items-center gap-2'>
                          <FileText className='size-4 shrink-0 text-muted-foreground' />
                          <div className='min-w-0 flex-1'>
                            <p className='truncate text-xs font-medium'>{bulkFileName ?? t('Pasted text')}</p>
                            <p className='text-[11px] text-muted-foreground'>
                              {t('{{count}} proxies ready to import', { count: bulkFileLines.length })}
                            </p>
                          </div>
                          <button
                            type='button'
                            className='rounded p-1 text-muted-foreground hover:bg-muted hover:text-foreground'
                            onClick={() => {
                              setBulkFileLines(null)
                              setBulkFileName(null)
                            }}
                            aria-label={t('Remove file')}
                          >
                            <X className='size-4' />
                          </button>
                        </div>
                        <pre className='max-h-28 overflow-auto rounded bg-background/60 p-2 font-mono text-[10px] leading-4 text-muted-foreground'>
                          {bulkFileLines.slice(0, 6).join('\n')}
                          {bulkFileLines.length > 6 ? `\n… +${bulkFileLines.length - 6}` : ''}
                        </pre>
                      </div>
                    ) : (
                      <Textarea
                        id='bulk-text'
                        value={bulkText}
                        onChange={(e) => {
                          const value = e.target.value
                          // Very large pastes would freeze a controlled textarea —
                          // parse them straight into the file-mode summary instead.
                          if (value.length > 100_000) {
                            const parsed = parseBulkLines(value)
                            setBulkFileLines(parsed)
                            setBulkFileName(null)
                            setBulkText('')
                            return
                          }
                          setBulkText(value)
                        }}
                        placeholder="socks5://1.2.3.4:1080, socks5h://user:pass@1.2.3.4:1080, socks4://user:pass@1.2.3.4:1080, 1.2.3.4:8080:user:pass, http://1.2.3.4:8080 (one per line)"
                        rows={5}
                        className='font-mono text-xs'
                      />
                    )}
                    <p className='text-[11px] text-muted-foreground'>
                      {t('Supports socks5, socks5h, socks4, http, https, host:port:user:pass or host:port formats.')}
                    </p>
                  </div>

                  <div className='flex items-center justify-between rounded-lg border p-3'>
                    <div>
                      <Label htmlFor='bulk-enabled' className='cursor-pointer text-xs font-medium'>
                        {t('Enabled')}
                      </Label>
                      <p className='text-muted-foreground text-[11px]'>
                        {t('Disabled proxies keep their URLs but receive no keys.')}
                      </p>
                    </div>
                    <Switch
                      id='bulk-enabled'
                      checked={bulkEnabled}
                      onCheckedChange={setBulkEnabled}
                    />
                  </div>

                  <Button
                    type='button'
                    className='w-full'
                    disabled={
                      isBulkSaving ||
                      (bulkFileLines !== null ? bulkFileLines.length === 0 : !bulkText.trim())
                    }
                    onClick={() => void handleBulkSave()}
                  >
                    {isBulkSaving ? (
                      <>
                        <Loader2 className='me-2 h-4 w-4 animate-spin' />
                        {bulkProgress
                          ? t('Importing {{done}}/{{total}}', {
                              done: bulkProgress.done,
                              total: bulkProgress.total,
                            })
                          : t('Importing...')}
                      </>
                    ) : (
                      <>
                        <Plus className='me-2 h-4 w-4' />
                        {t('Import Proxies')}
                      </>
                    )}
                  </Button>
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
                    placeholder='http://user:password@host:port, [ipv6]:port, socks5:// or socks5h://...'
                    autoComplete='off'
                    spellCheck={false}
                  />
                </div>
                <div className='flex items-center justify-between rounded-lg border p-3'>
                  <div>
                    <Label htmlFor='proxy-enabled' className='cursor-pointer text-xs font-medium'>
                      {t('Enabled')}
                    </Label>
                    <p className='text-muted-foreground text-[11px]'>
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
          </div>

          <DialogFooter className='px-6 py-4 border-t bg-muted/20 flex flex-row items-center justify-end gap-3 shrink-0'>
            <Button variant='ghost' onClick={() => setDialogOpen(false)}>
              {t('Cancel')}
            </Button>
            <Button
              disabled={saveMutation.isPending || cfDeploying || isBulkSaving}
              onClick={() => {
                if (proxyTab === 'line' || form.id !== undefined) {
                  submitForm()
                } else if (proxyTab === 'cloudflare') {
                  if (form.url.trim()) {
                    submitForm()
                  } else if (cfAccountId.trim() && cfApiToken.trim()) {
                    void handleDeployWorker()
                  } else {
                    toast.error(t('Please enter a Worker URL or provide Cloudflare credentials to deploy'))
                  }
                } else if (proxyTab === 'bulk') {
                  void handleBulkSave()
                }
              }}
            >
              {(saveMutation.isPending || cfDeploying || isBulkSaving) && (
                <Loader2 className='mr-2 h-4 w-4 animate-spin' />
              )}
              {t('Save')}
            </Button>
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
        <DialogContent className='max-w-6xl w-[95vw] max-h-[90vh] overflow-y-auto'>
          <DialogHeader>
            <DialogTitle className='flex items-center gap-2'>
              <span>{detailTarget?.name || t('Proxy Details')}</span>
              {detailQuery.data?.assigned_keys !== undefined &&
                detailQuery.data.assigned_keys > 0 && (
                  <Badge
                    variant='outline'
                    className='font-medium text-xs'
                  >
                    {detailQuery.data.assigned_keys} / {t('Assigned')}
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

          {(() => {
            if (detailQuery.isLoading) {
              return (
                <div className='flex h-32 items-center justify-center text-muted-foreground text-sm'>
                  <Loader2 className='mr-2 h-4 w-4 animate-spin' />
                  {t('Loading details...')}
                </div>
              )
            }
            if (detailQuery.data) {
              const latency = detailQuery.data.proxy.latency_ms ?? 0
              let latencyBadgeClass = 'text-rose-600 border-rose-500/40'
              let latencyLabel = t('Slow')
              if (latency < 400) {
                latencyBadgeClass = 'text-emerald-600 border-emerald-500/40'
                latencyLabel = t('Fast')
              } else if (latency < 900) {
                latencyBadgeClass = 'text-amber-600 border-amber-500/40'
                latencyLabel = t('Medium')
              }
              return (
            <div className='flex flex-col gap-4 py-2'>
              {/* Stat Cards */}
              <div className='grid grid-cols-2 sm:grid-cols-4 gap-3'>
                <div className='rounded-lg border bg-muted/20 p-3'>
                  <div className='text-muted-foreground text-xs font-medium'>
                    {t('Latency / Speed')}
                  </div>
                  <div className='mt-1 text-base font-bold font-mono flex items-center gap-1.5'>
                    <span>{detailQuery.data.proxy.latency_ms ? `${detailQuery.data.proxy.latency_ms} ms` : '-'}</span>
                    {latency > 0 ? (
                      <Badge variant='outline' className={cn('text-[10px] px-1 py-0', latencyBadgeClass)}>
                        {latencyLabel}
                      </Badge>
                    ) : null}
                  </div>
                </div>
                <div className='rounded-lg border bg-muted/20 p-3'>
                  <div className='text-muted-foreground text-xs font-medium'>
                    {t('Egress IP & Location')}
                  </div>
                  <div className='mt-1 text-sm font-semibold font-mono truncate flex items-center gap-1.5'>
                    {detailQuery.data.proxy.country_code ? (
                      <img
                        src={`https://flagcdn.com/20x15/${detailQuery.data.proxy.country_code.toLowerCase()}.png`}
                        alt={detailQuery.data.proxy.country_code}
                        className='inline-block h-3.5 w-auto rounded-xs shadow-xs shrink-0'
                      />
                    ) : null}
                    <span title={detailQuery.data.proxy.egress_ip}>{detailQuery.data.proxy.egress_ip || '-'}</span>
                  </div>
                  {(detailQuery.data.proxy.country_name || detailQuery.data.proxy.city || detailQuery.data.proxy.isp) && (
                    <div
                      className='text-[11px] text-muted-foreground truncate mt-0.5'
                      title={(() => {
                        let location = ''
                        if (detailQuery.data.proxy.city) location += `${detailQuery.data.proxy.city}, `
                        location += detailQuery.data.proxy.country_name || ''
                        if (detailQuery.data.proxy.isp) location += ` (${detailQuery.data.proxy.isp})`
                        return location.trim()
                      })()}
                    >
                      {[detailQuery.data.proxy.city, detailQuery.data.proxy.country_name, detailQuery.data.proxy.isp].filter(Boolean).join(' · ')}
                    </div>
                  )}
                </div>
                <div className='rounded-lg border bg-muted/20 p-3'>
                  <div className='text-muted-foreground text-xs font-medium'>
                    {t('Protocol / Type')}
                  </div>
                  <div className='mt-1 text-sm font-semibold'>
                    <Badge variant='outline' className='text-xs uppercase font-mono'>
                      {getProxyProtocol(detailQuery.data.proxy)}
                    </Badge>
                  </div>
                </div>
                <div className='rounded-lg border bg-muted/20 p-3'>
                  <div className='text-muted-foreground text-xs font-medium'>
                    {t('Status & Keys')}
                  </div>
                  <div className='mt-1 flex items-center justify-between'>
                    <div className='text-base font-bold font-mono'>
                      {detailQuery.data.assigned_keys} {t('Keys')}
                    </div>
                    {detailQuery.data.proxy.enabled ? (
                      <span className='text-xs font-semibold text-emerald-600 dark:text-emerald-400'>{t('Enabled')}</span>
                    ) : (
                      <span className='text-xs font-semibold text-destructive'>{t('Disabled')}</span>
                    )}
                  </div>
                </div>
              </div>

              {/* 30 Day Usage Chart */}
              <div className='rounded-lg border p-4'>
                <div className='text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-3'>
                  {t('Last 30 Days Data Transfer (MB)')}
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
                          t('Usage'),
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
                    {t('Assigned Channels and Keys')}
                  </div>
                  {detailQuery.data.assigned_keys > 0 && (
                    <Button
                      size='sm'
                      variant='outline'
                      className='text-destructive border-destructive/30 hover:bg-destructive/10 h-7 text-xs'
                      disabled={unassignMutation.isPending}
                      onClick={() => {
                        if (detailTarget) {
                          unassignMutation.mutate(detailTarget.id)
                        }
                      }}
                    >
                      {unassignMutation.isPending ? (
                        <Loader2 className='mr-1.5 h-3 w-3 animate-spin' />
                      ) : (
                        <Trash2 className='mr-1.5 h-3 w-3' />
                      )}
                      {t('Unassign')}
                    </Button>
                  )}
                </div>
                {detailQuery.data.assignments.length === 0 ? (
                  <p className='text-muted-foreground text-xs py-3 text-center'>
                    {t('This proxy is not currently assigned to any channel key.')}
                  </p>
                ) : (
                  <div className='max-h-48 overflow-y-auto'>
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead className='text-xs'>{t('Channel Name')}</TableHead>
                          <TableHead className='text-xs'>{t('Channel ID')}</TableHead>
                          <TableHead className='text-xs'>
                            {t('Key Index')}
                          </TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {detailQuery.data.assignments.map((item) => (
                          <TableRow
                            key={`${item.channel_id}-${item.key_index}`}
                          >
                            <TableCell className='text-xs font-medium'>
                              {item.channel_name || `#${item.channel_id}`}
                            </TableCell>
                            <TableCell className='text-xs font-mono text-muted-foreground'>
                              #{item.channel_id}
                            </TableCell>
                            <TableCell className='text-xs font-mono'>
                              {t('Key')} #{item.key_index + 1}
                            </TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </div>
                )}
              </div>
            </div>
              )
            }
            return null
          })()}

          <DialogFooter>
            <Button variant='outline' onClick={() => setDetailTarget(null)}>
              {t('Close')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Bulk Operations Toolbar */}
      {selectedProxyIds.length > 0 && (
        <div className='fixed bottom-6 left-1/2 -translate-x-1/2 z-50 flex flex-wrap items-center gap-2 rounded-full border bg-background/95 px-4 py-2 shadow-xl backdrop-blur'>
          <span className='text-xs font-medium px-2'>
            {t('{{count}} proxies selected', { count: selectedProxyIds.length })}
          </span>
          <Button
            size='sm'
            variant='outline'
            className='h-8 text-xs'
            disabled={isBatchTesting}
            onClick={handleBatchTest}
          >
            {isBatchTesting ? (
              <Loader2 className='mr-1.5 h-3.5 w-3.5 animate-spin' />
            ) : (
              <Gauge className='mr-1.5 h-3.5 w-3.5' />
            )}
            {t('Batch Test')}
          </Button>
          <Button
            size='sm'
            variant='outline'
            className='h-8 text-xs'
            disabled={isBatchRenaming}
            onClick={() => {
              setBatchNameValue('')
              setBatchNameDialogOpen(true)
            }}
          >
            <Pencil className='mr-1.5 h-3.5 w-3.5' />
            {t('Batch Rename')}
          </Button>
          <Button
            size='sm'
            variant='destructive'
            className='h-8 text-xs'
            disabled={isBatchDeleting}
            onClick={() => setBatchDeleteDialogOpen(true)}
          >
            <Trash2 className='mr-1.5 h-3.5 w-3.5' />
            {t('Batch Delete')}
          </Button>
          <Button
            size='sm'
            variant='ghost'
            className='h-8 text-xs'
            onClick={() => setSelectedProxyIds([])}
          >
            {t('Clear')}
          </Button>
        </div>
      )}

      {/* Channel Detail Dialog */}
      <Dialog
        open={detailChannelId !== null}
        onOpenChange={(open) => {
          if (!open) setDetailChannelId(null)
        }}
      >
        <DialogContent className='max-w-3xl w-[96vw] sm:w-full max-h-[88vh] sm:max-h-[85vh] flex flex-col p-0 overflow-hidden'>
          <DialogHeader className='p-5 sm:p-6 pb-3 border-b shrink-0'>
            <DialogTitle>
              {channelDetailData?.channel.name || t('Channel detail')}
            </DialogTitle>
            <DialogDescription>
              #{channelDetailData?.channel.id ?? detailChannelId}
              {channelDetailData
                ? ` · ${t('Type {{type}}', { type: channelDetailData.channel.type })} · ${t(
                    '{{count}} keys',
                    { count: channelDetailData.channel.multi_key_size || 0 }
                  )}`
                : ''}
            </DialogDescription>
          </DialogHeader>
          <div className='flex-1 overflow-y-auto p-5 sm:p-6 space-y-5'>
            {channelDetailQuery.isLoading && (
              <div className='flex h-24 items-center justify-center text-muted-foreground'>
                <Loader2 className='me-2 size-4 animate-spin' />
                {t('Loading')}
              </div>
            )}
            {channelDetailQuery.isError && (
              <p className='rounded-lg border border-destructive/40 bg-destructive/5 p-4 text-sm'>
                {t('Channels could not be loaded')}
              </p>
            )}
            {channelDetailData && (
              <>
                <section className='space-y-2'>
                  <h3 className='text-xs font-semibold uppercase tracking-wide text-muted-foreground'>
                    {t('Assigned proxies')}
                  </h3>
                  {channelDetailData.assignments.length === 0 ? (
                    <p className='rounded-lg border border-dashed border-border/70 p-4 text-center text-xs text-muted-foreground'>
                      {t('No proxies assigned to this channel yet. Use "Assign evenly" or "Rebalance All".')}
                    </p>
                  ) : (
                    <div className='overflow-x-auto rounded-lg border border-border/60'>
                      <Table>
                        <TableHeader>
                          <TableRow>
                            <TableHead>{t('Proxy')}</TableHead>
                            <TableHead>{t('Egress')}</TableHead>
                            <TableHead>{t('Latency')}</TableHead>
                            <TableHead>{t('Last test')}</TableHead>
                            <TableHead className='text-right'>{t('Cooldown')}</TableHead>
                          </TableRow>
                        </TableHeader>
                        <TableBody>
                          {channelDetailData.assignments.map((assignment) => {
                            const cooldownSeconds =
                              channelDetailData.cooldowns[assignment.proxy_url] ?? 0
                            return (
                              <TableRow key={`${assignment.key_index}:${assignment.proxy_id}`}>
                                <TableCell>
                                  <div className='flex flex-wrap items-center gap-1.5'>
                                    <span className='text-[10px] font-mono text-muted-foreground'>
                                      #{assignment.proxy_id}
                                    </span>
                                    <span className='font-medium'>{assignment.proxy_name}</span>
                                  </div>
                                  <div
                                    className='max-w-[260px] truncate text-[10px] font-mono text-muted-foreground'
                                    title={assignment.proxy_url}
                                  >
                                    {assignment.proxy_url}
                                  </div>
                                  {assignment.country_name && (
                                    <div className='text-[10px] text-muted-foreground'>
                                      {assignment.country_name}
                                      {assignment.country_code ? ` (${assignment.country_code})` : ''}
                                    </div>
                                  )}
                                </TableCell>
                                <TableCell className='font-mono text-[10px]'>
                                  {assignment.egress_ip || '—'}
                                </TableCell>
                                <TableCell className='font-mono text-[10px]'>
                                  {assignment.latency_ms > 0 ? `${assignment.latency_ms} ms` : '—'}
                                </TableCell>
                                <TableCell>
                                  {(() => {
                                    let testBadgeVariant: 'secondary' | 'destructive' | 'outline' = 'outline'
                                    if (assignment.last_test_status === 'ok') {
                                      testBadgeVariant = 'secondary'
                                    } else if (assignment.last_test_status === 'fail') {
                                      testBadgeVariant = 'destructive'
                                    }
                                    return (
                                      <Badge
                                        variant={testBadgeVariant}
                                        className='text-[10px]'
                                        title={assignment.last_test_error || undefined}
                                      >
                                        {assignment.last_test_status || t('Not tested')}
                                      </Badge>
                                    )
                                  })()}
                                  {assignment.last_tested_at > 0 && (
                                    <div className='text-[9px] text-muted-foreground'>
                                      {formatTimestampRelative(assignment.last_tested_at)}
                                    </div>
                                  )}
                                </TableCell>
                                <TableCell className='text-right'>
                                  {cooldownSeconds > 0 ? (
                                    <Badge
                                      variant='outline'
                                      className='inline-flex items-center gap-1 border-amber-500/50 text-[10px] text-amber-600 dark:text-amber-400'
                                    >
                                      <Timer className='size-3' />
                                      {t('On cooldown · {{seconds}}s left', {
                                        seconds: cooldownSeconds,
                                      })}
                                    </Badge>
                                  ) : (
                                    <span className='text-[10px] text-muted-foreground'>—</span>
                                  )}
                                </TableCell>
                              </TableRow>
                            )
                          })}
                        </TableBody>
                      </Table>
                    </div>
                  )}
                </section>

                <section className='space-y-2'>
                  <h3 className='text-xs font-semibold uppercase tracking-wide text-muted-foreground'>
                    {t('Channel event log')}
                  </h3>
                  {channelDetailData.events.length === 0 ? (
                    <p className='rounded-lg border border-dashed border-border/70 p-4 text-center text-xs text-muted-foreground'>
                      {t('No events yet')}
                    </p>
                  ) : (
                    <ul className='max-h-56 space-y-1.5 overflow-y-auto pe-1'>
                      {channelDetailData.events.map((event) => (
                        <li
                          key={event.id}
                          className='flex items-start gap-2 rounded-md border border-border/50 bg-muted/30 px-2 py-1.5 text-[11px]'
                        >
                          <Badge
                            variant={eventKindBadgeVariant(event.kind)}
                            className='mt-0.5 shrink-0 text-[9px]'
                          >
                            {eventKindLabel(event.kind, t)}
                          </Badge>
                          <span className='min-w-0 flex-1 break-words'>{event.message}</span>
                          <span className='shrink-0 text-[9px] text-muted-foreground tabular-nums'>
                            {formatTimestampRelative(event.created_at)}
                          </span>
                        </li>
                      ))}
                    </ul>
                  )}
                </section>

                <section className='space-y-2'>
                  <h3 className='text-xs font-semibold uppercase tracking-wide text-muted-foreground'>
                    {t('Global pool events')}
                  </h3>
                  {channelDetailData.global_events.length === 0 ? (
                    <p className='rounded-lg border border-dashed border-border/70 p-4 text-center text-xs text-muted-foreground'>
                      {t('No events yet')}
                    </p>
                  ) : (
                    <ul className='max-h-40 space-y-1.5 overflow-y-auto pe-1'>
                      {channelDetailData.global_events.map((event) => (
                        <li
                          key={event.id}
                          className='flex items-start gap-2 rounded-md border border-border/50 bg-muted/30 px-2 py-1.5 text-[11px]'
                        >
                          <Badge
                            variant={eventKindBadgeVariant(event.kind)}
                            className='mt-0.5 shrink-0 text-[9px]'
                          >
                            {eventKindLabel(event.kind, t)}
                          </Badge>
                          <span className='min-w-0 flex-1 break-words'>{event.message}</span>
                          <span className='shrink-0 text-[9px] text-muted-foreground tabular-nums'>
                            {formatTimestampRelative(event.created_at)}
                          </span>
                        </li>
                      ))}
                    </ul>
                  )}
                </section>
              </>
            )}
          </div>
        </DialogContent>
      </Dialog>

      {/* Batch Rename Dialog */}
      <Dialog open={batchNameDialogOpen} onOpenChange={setBatchNameDialogOpen}>
        <DialogContent className='max-w-md'>
          <DialogHeader>
            <DialogTitle>{t('Batch Rename')}</DialogTitle>
            <DialogDescription>
              {t('Enter base name for {{count}} proxies. Sequence numbers will be appended automatically if multiple are selected.', {
                count: selectedProxyIds.length,
              })}
            </DialogDescription>
          </DialogHeader>
          <div className='py-2 space-y-2'>
            <Label htmlFor='batch-proxy-name'>{t('New Name')}</Label>
            <Input
              id='batch-proxy-name'
              placeholder={t('e.g., Proxy-TR')}
              value={batchNameValue}
              onChange={(e) => setBatchNameValue(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') {
                  e.preventDefault()
                  void handleBatchRename()
                }
              }}
            />
          </div>
          <DialogFooter>
            <Button variant='outline' onClick={() => setBatchNameDialogOpen(false)}>
              {t('Cancel')}
            </Button>
            <Button disabled={isBatchRenaming || !batchNameValue.trim()} onClick={handleBatchRename}>
              {isBatchRenaming && <Loader2 className='mr-1.5 h-3.5 w-3.5 animate-spin' />}
              {t('Save')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Batch Delete ConfirmDialog */}
      <ConfirmDialog
        open={batchDeleteDialogOpen}
        onOpenChange={setBatchDeleteDialogOpen}
        title={t('Delete selected proxies?')}
        desc={t(
          '{{count}} proxies will be deleted. Keys assigned to these proxies will revert to the default route.',
          { count: selectedProxyIds.length }
        )}
        destructive
        isLoading={isBatchDeleting}
        handleConfirm={handleBatchDelete}
        confirmText={t('Delete')}
      />
    </>
  )
}

function eventKindBadgeVariant(kind: string): 'default' | 'secondary' | 'destructive' | 'outline' {
  switch (kind) {
    case 'cooldown_429':
    case 'switch_429':
      return 'destructive'
    case 'test_fail':
    case 'auto_disabled':
      return 'destructive'
    case 'assign':
    case 'import':
    case 'auto_enabled':
    case 'test_ok':
      return 'secondary'
    default:
      return 'outline'
  }
}

function eventKindLabel(kind: string, t: (key: string) => string): string {
  switch (kind) {
    case 'assign':
      return t('Assignment')
    case 'import':
      return t('Imported')
    case 'clear':
      return t('Cleared')
    case 'unassign':
      return t('Unassigned')
    case 'cooldown_429':
      return t('429 cooldown')
    case 'switch_429':
      return t('429 switch')
    case 'test_fail':
      return t('Test failed')
    case 'test_ok':
      return t('Test ok')
    case 'auto_disabled':
      return t('Auto-disabled')
    case 'auto_enabled':
      return t('Auto-enabled')
    default:
      return kind
  }
}

function assignedIconFor(channel: ProxyPoolChannel) {
  const isAnon = channel.type === 65 || channel.type === 68 || channel.type === 69
  const isComplete = isAnon ? channel.assigned_keys > 0 : channel.assigned_keys === channel.multi_key_size
  return isComplete ? (
    <CheckCircle2 className='mr-1 h-3.5 w-3.5 text-emerald-600' />
  ) : (
    <CircleX className='mr-1 h-3.5 w-3.5 text-muted-foreground' />
  )
}
