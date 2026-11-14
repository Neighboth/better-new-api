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
  Loader2,
  Network,
  Plus,
  TestTube2,
  Trash2,
  Waypoints,
  Zap,
} from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
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
  listChannels,
  listProxies,
  testProxy,
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
    <SectionPageLayout>
      <SectionPageLayout.Title>
        <span className='inline-flex min-w-0 items-center gap-2'>
          <Waypoints className='h-5 w-5 shrink-0' />
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
                <CardTitle className='flex items-center gap-2'>
                  <Network className='h-4 w-4' />
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
                          {proxy.name || t('Unnamed')}
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
                                      aria-label={t('Test proxy')}
                                    >
                                      {testingId === proxy.id ? (
                                        <Loader2 className='h-4 w-4 animate-spin' />
                                      ) : (
                                        <TestTube2 className='h-4 w-4' />
                                      )}
                                    </Button>
                                  }
                                />
                                <TooltipContent>{t('Test egress')}</TooltipContent>
                              </Tooltip>
                            </TooltipProvider>
                            <Button
                              size='sm'
                              variant='ghost'
                              onClick={() => openEditDialog(proxy)}
                            >
                              {t('Edit')}
                            </Button>
                            <Button
                              size='icon'
                              variant='ghost'
                              onClick={() => setDeleteTarget(proxy)}
                              aria-label={t('Delete proxy')}
                            >
                              <Trash2 className='h-4 w-4' />
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

          <Card>
            <CardHeader>
              <CardTitle className='flex items-center gap-2'>
                <Zap className='h-4 w-4' />
                {t('Multi-key channel assignments')}
              </CardTitle>
              <CardDescription className='mt-1.5'>
                {t(
                  'Keys are handed out round-robin across enabled proxies. If a channel has more keys than you have proxies, each proxy serves multiple keys.'
                )}
              </CardDescription>
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
                              variant='ghost'
                              disabled={clearMutation.isPending || channel.assigned_keys === 0}
                              onClick={() => setClearTarget(channel)}
                            >
                              <Trash2 className='h-4 w-4' />
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

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {form.id !== undefined ? t('Edit proxy') : t('Add proxy')}
            </DialogTitle>
            <DialogDescription>
              {t('http://user:pass@host:port, https://…, socks5://… or socks5h://…')}
            </DialogDescription>
          </DialogHeader>
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
          <DialogFooter>
            <Button variant='ghost' onClick={() => setDialogOpen(false)}>
              {t('Cancel')}
            </Button>
            <Button disabled={saveMutation.isPending} onClick={submitForm}>
              {saveMutation.isPending && (
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
    </SectionPageLayout>
  )
}

function assignedIconFor(channel: ProxyPoolChannel) {
  return channel.assigned_keys === channel.multi_key_size ? (
    <CheckCircle2 className='mr-1 h-3.5 w-3.5 text-emerald-600' />
  ) : (
    <CircleX className='mr-1 h-3.5 w-3.5 text-muted-foreground' />
  )
}
