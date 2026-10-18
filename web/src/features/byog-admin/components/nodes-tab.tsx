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
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

import { toast } from 'sonner'
import { deleteAdminNode, getAdminNodes } from '../../byog/api'
import type { ByogNode } from '../../byog/types'
import { NetworkGlobe } from './network-globe'

export function ByogNodesTab() {
  const { t } = useTranslation()
  const [nodes, setNodes] = useState<ByogNode[]>([])
  const [loading, setLoading] = useState(true)
  const [search, setSearch] = useState('')
  const [typeFilter, setTypeFilter] = useState('all')
  const [selectedNode, setSelectedNode] = useState<ByogNode | null>(null)
  const [inspectOpen, setInspectOpen] = useState(false)

  useEffect(() => {
    loadNodes()
    const interval = setInterval(loadNodes, 15000)
    return () => clearInterval(interval)
  }, [])

  async function loadNodes() {
    try {
      const data = await getAdminNodes()
      setNodes(data || [])
    } catch {
      // Keep silent on background polling
    } finally {
      setLoading(false)
    }
  }

  function handleInspect(node: ByogNode) {
    setSelectedNode(node)
    setInspectOpen(true)
  }

  async function handleDeleteNode(nodeId: string) {
    if (
      !confirm(
        t(
          'byog.confirm_delete_device',
          'Bu cihazı ağdan ve veritabanından kalıcı olarak silmek istediğinizden emin misiniz?'
        )
      )
    )
      return
    try {
      await deleteAdminNode(nodeId)
      setNodes((prev) => prev.filter((n) => n.node_id !== nodeId && String(n.id) !== nodeId))
      toast.success(
        t('byog.device_deleted_success', 'Cihaz başarıyla silindi')
      )
      loadNodes()
    } catch (err: any) {
      toast.error(err.message || 'Silme işlemi başarısız')
    }
  }

  // Stats calculation
  const onlineNodes = nodes.filter((n) => n.status === 'online')
  const aggregateTps = onlineNodes.reduce(
    (acc, n) => acc + (n.current_tps || 0),
    0
  )
  const totalVramGb = nodes.reduce((acc, n) => acc + (n.vram_mb || 0) / 1024, 0)
  const mobileCount = nodes.filter(
    (n) => n.device_type === 'mobile' || n.device_type === 'tablet'
  ).length
  const desktopCount = nodes.length - mobileCount

  const filteredNodes = nodes.filter((n) => {
    const matchesSearch =
      n.device_name?.toLowerCase().includes(search.toLowerCase()) ||
      n.node_id?.toLowerCase().includes(search.toLowerCase()) ||
      n.gpu_model?.toLowerCase().includes(search.toLowerCase()) ||
      n.cpu_model?.toLowerCase().includes(search.toLowerCase())
    const matchesType = typeFilter === 'all' || n.device_type === typeFilter
    return matchesSearch && matchesType
  })

  function getStatusBadge(status: string) {
    switch (status) {
      case 'online':
        return (
          <Badge className='border-emerald-500/20 bg-emerald-500/10 text-emerald-500'>
            {t('byog.status_online')}
          </Badge>
        )
      case 'gamer_mode':
        return (
          <Badge className='border-purple-500/20 bg-purple-500/10 text-purple-400'>
            {t('byog.status_gamer_mode')}
          </Badge>
        )
      case 'throttled':
        return (
          <Badge className='border-amber-500/20 bg-amber-500/10 text-amber-500'>
            {t('byog.status_throttled')}
          </Badge>
        )
      case 'banned':
        return <Badge variant='destructive'>{t('byog.status_banned')}</Badge>
      default:
        return <Badge variant='outline'>{t('byog.status_offline')}</Badge>
    }
  }

  return (
    <div className='space-y-6'>
      {/* Interactive World Map */}
      <NetworkGlobe nodes={nodes} />

      {/* Aggregate Metrics Bar - Zero Icons */}
      <div className='grid grid-cols-2 gap-4 md:grid-cols-4'>
        <Card className='space-y-1 p-4'>
          <div className='text-2xl font-bold'>
            {onlineNodes.length} / {nodes.length}
          </div>
          <div className='text-muted-foreground text-xs'>
            {t('byog.admin.online_nodes_total')}
          </div>
        </Card>

        <Card className='space-y-1 p-4'>
          <div className='text-2xl font-bold'>{aggregateTps.toFixed(1)}</div>
          <div className='text-muted-foreground text-xs'>
            {t('byog.admin.aggregate_tps')}
          </div>
        </Card>

        <Card className='space-y-1 p-4'>
          <div className='text-2xl font-bold'>
            {totalVramGb.toFixed(0)} GB
          </div>
          <div className='text-muted-foreground text-xs'>
            {t('byog.admin.total_vram_capacity')}
          </div>
        </Card>

        <Card className='space-y-1 p-4'>
          <div className='text-2xl font-bold'>
            {desktopCount}D / {mobileCount}M
          </div>
          <div className='text-muted-foreground text-xs'>
            {t('byog.admin.desktop_vs_mobile')}
          </div>
        </Card>
      </div>

      {/* Filterable Nodes Table */}
      <Card>
        <CardHeader>
          <div className='flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between'>
            <div>
              <CardTitle>
                {t('byog.admin.connected_nodes_title')}
              </CardTitle>
              <CardDescription>
                {t('byog.admin.connected_nodes_desc')}
              </CardDescription>
            </div>
            <div className='flex items-center gap-2'>
              <Input
                className='w-44 text-xs sm:w-60'
                placeholder={t('byog.admin.search_nodes')}
                value={search}
                onChange={(e) => setSearch(e.target.value)}
              />
              <Select
                value={typeFilter}
                onValueChange={(val) => setTypeFilter(val || 'all')}
              >
                <SelectTrigger className='w-32 text-xs'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='all'>
                    {t('byog.admin.all_types')}
                  </SelectItem>
                  <SelectItem value='desktop'>
                    {t('byog.admin.desktop')}
                  </SelectItem>
                  <SelectItem value='laptop'>
                    {t('byog.admin.laptop')}
                  </SelectItem>
                  <SelectItem value='mobile'>
                    {t('byog.admin.mobile')}
                  </SelectItem>
                  <SelectItem value='tablet'>
                    {t('byog.admin.tablet')}
                  </SelectItem>
                </SelectContent>
              </Select>
              <Button variant='outline' size='sm' onClick={loadNodes}>
                {t('byog.refresh')}
              </Button>
            </div>
          </div>
        </CardHeader>
        <CardContent>
          {loading ? (
            <div className='text-muted-foreground flex h-32 items-center justify-center'>
              {t('byog.loading')}
            </div>
          ) : filteredNodes.length === 0 ? (
            <div className='text-muted-foreground py-12 text-center'>
              {t('byog.admin.no_nodes_found')}
            </div>
          ) : (
            <div className='overflow-x-auto rounded-md border'>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('byog.admin.device')}</TableHead>
                    <TableHead>{t('byog.admin.hardware')}</TableHead>
                    <TableHead>{t('byog.admin.ram_vram')}</TableHead>
                    <TableHead>{t('byog.admin.network_ping')}</TableHead>
                    <TableHead>{t('byog.admin.operating_mode')}</TableHead>
                    <TableHead>{t('byog.admin.telemetry')}</TableHead>
                    <TableHead>{t('byog.admin.status')}</TableHead>
                    <TableHead className='text-right'>
                      {t('byog.admin.actions')}
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {filteredNodes.map((node) => (
                    <TableRow key={node.id}>
                      <TableCell>
                        <div>
                          <div className='text-sm font-semibold'>
                            {node.device_name}
                          </div>
                          <div className='flex items-center gap-1.5 pt-0.5'>
                            <Badge
                              variant='outline'
                              className='px-1 py-0 font-mono text-[10px] uppercase'
                            >
                              {node.device_type}
                            </Badge>
                            <span className='text-muted-foreground font-mono text-[11px]'>
                              {node.node_id.slice(0, 12)}...
                            </span>
                          </div>
                        </div>
                      </TableCell>
                      <TableCell>
                        <div className='text-xs'>
                          <div
                            className='max-w-[160px] truncate font-medium'
                            title={node.gpu_model || node.cpu_model}
                          >
                            {node.gpu_model || node.cpu_model}
                          </div>
                          <div className='text-muted-foreground text-[10px]'>
                            {node.os} ({node.arch})
                          </div>
                        </div>
                      </TableCell>
                      <TableCell className='text-muted-foreground text-xs'>
                        <div>
                          RAM: {(node.physical_ram_mb / 1024).toFixed(1)} GB
                        </div>
                        <div>
                          {node.device_type === 'mobile' || node.device_type === 'tablet'
                            ? 'VRAM: 0 GB (UMA)'
                            : `VRAM: ${(node.vram_mb / 1024).toFixed(1)} GB`}
                        </div>
                      </TableCell>
                      <TableCell className='text-xs font-mono'>
                        <div>{node.ping_ms} ms</div>
                        <div className='text-muted-foreground font-sans text-[10px]'>
                          {node.country || 'Global'}
                        </div>
                      </TableCell>
                      <TableCell>
                        <Badge variant='outline' className='text-xs capitalize'>
                          {node.operating_mode === 'mode_b'
                            ? t('byog.mode_b_short')
                            : t('byog.mode_a_short')}
                        </Badge>
                      </TableCell>
                      <TableCell className='text-xs'>
                        <div className='text-primary font-mono font-medium'>
                          {node.current_tps.toFixed(1)} TPS
                        </div>
                        <div className='text-muted-foreground text-[10px]'>
                          {node.current_ttft_ms} ms TTFT
                        </div>
                      </TableCell>
                      <TableCell>{getStatusBadge(node.status)}</TableCell>
                      <TableCell className='text-right'>
                        <div className='flex items-center justify-end gap-1.5'>
                          <Button
                            variant='outline'
                            size='sm'
                            onClick={() => handleInspect(node)}
                            className='h-7 px-2.5 text-xs'
                          >
                            {t('byog.admin.inspect')}
                          </Button>
                          <Button
                            variant='ghost'
                            size='sm'
                            onClick={() => handleDeleteNode(node.node_id)}
                            className='h-7 px-2 text-xs text-destructive hover:text-destructive hover:bg-destructive/10'
                          >
                            {t('common.delete', 'Sil')}
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Deep Inspect Modal - Zero Decorative Icons */}
      <Dialog open={inspectOpen} onOpenChange={setInspectOpen}>
        <DialogContent className='max-h-[90vh] max-w-3xl overflow-y-auto'>
          <DialogHeader>
            <DialogTitle>
              {t('byog.admin.deep_inspect_title', {
                name: selectedNode?.device_name,
              })}
            </DialogTitle>
            <DialogDescription className='font-mono text-xs'>
              HWID: {selectedNode?.hwid} • Node ID: {selectedNode?.node_id}
            </DialogDescription>
          </DialogHeader>

          {selectedNode && (
            <div className='space-y-6 py-4'>
              {/* TEE Attestation & Security Status */}
              <div className='bg-muted/20 flex items-center justify-between rounded-xl border p-4'>
                <div>
                  <div className='text-sm font-semibold'>
                    {selectedNode.tee_supported
                      ? t('byog.admin.tee_attested')
                      : t('byog.admin.tee_unsupported')}
                  </div>
                  <div className='text-muted-foreground text-xs'>
                    {selectedNode.tee_supported
                      ? t('byog.admin.tee_type_label', {
                          type: selectedNode.tee_type,
                        })
                      : t('byog.admin.standard_isolation')}
                  </div>
                </div>
                <Badge variant='outline' className='font-mono text-xs'>
                  Reputation: {selectedNode.reputation_score}/100
                </Badge>
              </div>

              {/* Hardware Specifications */}
              <div className='grid grid-cols-2 gap-4 md:grid-cols-4'>
                <div className='bg-muted/10 space-y-1 rounded-lg border p-3'>
                  <div className='text-muted-foreground text-xs'>
                    {t('byog.admin.cpu_model')}
                  </div>
                  <div
                    className='truncate text-sm font-semibold'
                    title={selectedNode.cpu_model}
                  >
                    {selectedNode.cpu_model || 'N/A'}
                  </div>
                </div>

                <div className='bg-muted/10 space-y-1 rounded-lg border p-3'>
                  <div className='text-muted-foreground text-xs'>
                    {t('byog.admin.gpu_model')}
                  </div>
                  <div
                    className='truncate text-sm font-semibold'
                    title={selectedNode.gpu_model}
                  >
                    {selectedNode.gpu_model || 'CPU Only'}
                  </div>
                </div>

                <div className='bg-muted/10 space-y-1 rounded-lg border p-3'>
                  <div className='text-muted-foreground text-xs'>
                    {t('byog.admin.physical_ram')}
                  </div>
                  <div className='text-sm font-semibold'>
                    {(selectedNode.physical_ram_mb / 1024).toFixed(1)} GB
                  </div>
                  <div className='text-muted-foreground text-[10px]'>
                    Swap: {(selectedNode.swap_ram_mb / 1024).toFixed(1)} GB
                  </div>
                </div>

                <div className='bg-muted/10 space-y-1 rounded-lg border p-3'>
                  <div className='text-muted-foreground text-xs'>
                    {t('byog.admin.vram_memory')}
                  </div>
                  <div className='text-sm font-semibold'>
                    {selectedNode.device_type === 'mobile' || selectedNode.device_type === 'tablet'
                      ? '0 GB (UMA)'
                      : `${(selectedNode.vram_mb / 1024).toFixed(1)} GB`}
                  </div>
                  {selectedNode.shared_vram_mb > 0 && selectedNode.device_type !== 'mobile' && selectedNode.device_type !== 'tablet' && (
                    <div className='text-muted-foreground text-[10px]'>
                      Shared: {(selectedNode.shared_vram_mb / 1024).toFixed(1)} GB
                    </div>
                  )}
                </div>
              </div>

              {/* Operating Profile & Resource Allocations */}
              <div className='bg-muted/10 space-y-3 rounded-xl border p-4'>
                <div className='text-sm font-semibold'>
                  {t('byog.admin.operating_and_allocation', 'Çalışma Modu ve Kaynak Dağılımı')}
                </div>
                <div className='grid grid-cols-2 gap-4 text-xs md:grid-cols-4'>
                  <div>
                    <span className='text-muted-foreground'>{t('byog.mode_label', 'Mod')}: </span>
                    <span className='font-medium capitalize'>
                      {selectedNode.operating_mode === 'mode_b' ? t('byog.mode_b_short', 'Havuz (Mode B)') : t('byog.mode_a_short', 'Özel (Mode A)')}
                    </span>
                  </div>
                  <div>
                    <span className='text-muted-foreground'>{t('byog.power_profile', 'Güç Profili')}: </span>
                    <span className='font-medium capitalize'>{selectedNode.power_profile.replace('_', ' ')}</span>
                  </div>
                  <div>
                    <span className='text-muted-foreground'>{t('byog.allocated_ram', 'Ayrılan RAM')}: </span>
                    <span className='font-mono font-medium'>{selectedNode.allocated_ram_gb} GB</span>
                  </div>
                  <div>
                    <span className='text-muted-foreground'>{t('byog.allocated_vram', 'Ayrılan VRAM')}: </span>
                    <span className='font-mono font-medium'>
                      {selectedNode.device_type === 'mobile' || selectedNode.device_type === 'tablet' ? '0 GB (UMA)' : `${selectedNode.allocated_vram_gb} GB`}
                    </span>
                  </div>
                  <div>
                    <span className='text-muted-foreground'>{t('byog.allocated_cores', 'Ayrılan Çekirdek')}: </span>
                    <span className='font-mono font-medium'>{selectedNode.allocated_cores}</span>
                  </div>
                  <div>
                    <span className='text-muted-foreground'>{t('byog.allocated_disk', 'Ayrılan Disk')}: </span>
                    <span className='font-mono font-medium'>{selectedNode.allocated_disk_gb} GB</span>
                  </div>
                  <div>
                    <span className='text-muted-foreground'>{t('byog.hot_start_label', 'Sıcak Başlatma')}: </span>
                    <span className='font-medium'>{selectedNode.hot_start_enabled ? t('common.enabled', 'Aktif') : t('common.disabled', 'Pasif')}</span>
                  </div>
                  <div>
                    <span className='text-muted-foreground'>{t('byog.last_heartbeat', 'Son Sinyal')}: </span>
                    <span className='font-mono'>{Math.max(0, Math.floor(Date.now() / 1000 - selectedNode.last_heartbeat_at))}s önce</span>
                  </div>
                </div>
              </div>

              {/* Mobile Governance (if mobile/tablet) */}
              {(selectedNode.device_type === 'mobile' || selectedNode.device_type === 'tablet') && (
                <div className='bg-muted/10 space-y-3 rounded-xl border p-4'>
                  <div className='text-sm font-semibold'>
                    {t('byog.admin.mobile_governance', 'Mobil Cihaz ve Batarya Yönetimi')}
                  </div>
                  <div className='grid grid-cols-2 gap-4 text-xs md:grid-cols-4'>
                    <div>
                      <span className='text-muted-foreground'>{t('byog.wifi_only', 'Sadece Wi-Fi')}: </span>
                      <span>{selectedNode.wifi_only_download ? t('common.yes', 'Evet') : t('common.no', 'Hayır')}</span>
                    </div>
                    <div>
                      <span className='text-muted-foreground'>{t('byog.data_limit', 'Mobil Veri Sınırı')}: </span>
                      <span>{selectedNode.monthly_data_limit_mb > 0 ? `${selectedNode.monthly_data_limit_mb} MB` : t('common.unlimited', 'Sınırsız')}</span>
                    </div>
                    <div>
                      <span className='text-muted-foreground'>{t('byog.charging_only', 'Sadece Şarjdayken')}: </span>
                      <span>{selectedNode.run_only_while_charging ? t('common.yes', 'Evet') : t('common.no', 'Hayır')}</span>
                    </div>
                    <div>
                      <span className='text-muted-foreground'>{t('byog.battery_cutoff', 'Pil Kesme Eşiği')}: </span>
                      <span>{selectedNode.battery_cutoff_enabled ? `%${selectedNode.battery_cutoff_percent}` : t('common.disabled', 'Devre Dışı')}</span>
                    </div>
                    <div>
                      <span className='text-muted-foreground'>{t('byog.max_temp_limit', 'Sıcaklık Sınırı')}: </span>
                      <span>{selectedNode.max_battery_temp}°C</span>
                    </div>
                    <div>
                      <span className='text-muted-foreground'>{t('byog.data_used', 'Kullanılan Veri')}: </span>
                      <span>{selectedNode.monthly_data_used_mb} MB</span>
                    </div>
                  </div>
                </div>
              )}

              {/* Telemetry & Thermals */}
              <div className='grid grid-cols-2 gap-4 md:grid-cols-4'>
                <div className='bg-muted/10 space-y-1 rounded-lg border p-3'>
                  <div className='text-muted-foreground text-xs'>
                    {t('byog.admin.cpu_temp')}
                  </div>
                  <div className='text-base font-semibold'>
                    {selectedNode.cpu_temp}°C
                  </div>
                </div>

                <div className='bg-muted/10 space-y-1 rounded-lg border p-3'>
                  <div className='text-muted-foreground text-xs'>
                    {t('byog.admin.gpu_temp')}
                  </div>
                  <div className='text-base font-semibold'>
                    {selectedNode.gpu_temp}°C
                  </div>
                </div>

                {selectedNode.battery_temp_supported && (
                  <div className='bg-muted/10 space-y-1 rounded-lg border p-3'>
                    <div className='text-muted-foreground text-xs'>
                      {t('byog.admin.battery_temp')}
                    </div>
                    <div className='text-base font-semibold'>
                      {selectedNode.battery_temp}°C
                    </div>
                  </div>
                )}

                {(selectedNode.device_type === 'mobile' ||
                  selectedNode.device_type === 'tablet' ||
                  selectedNode.battery_percent > 0) && (
                  <div className='bg-muted/10 space-y-1 rounded-lg border p-3'>
                    <div className='text-muted-foreground text-xs'>
                      {t('byog.admin.battery_status')}
                    </div>
                    <div className='text-base font-semibold flex items-center gap-1.5'>
                      <span>{selectedNode.battery_percent}%</span>
                      {selectedNode.is_charging ? (
                        <span className='text-emerald-500 font-normal text-xs'>({t('byog.charging', 'Şarjda')})</span>
                      ) : (
                        <span className='text-muted-foreground font-normal text-xs'>({t('byog.not_charging', 'Pilde')})</span>
                      )}
                    </div>
                  </div>
                )}
              </div>

              {/* Network & Geographic Info */}
              <div className='bg-muted/10 space-y-2 rounded-xl border p-4'>
                <div className='text-sm font-semibold'>
                  {t('byog.admin.network_geo_title')}
                </div>
                <div className='grid grid-cols-2 gap-4 text-xs md:grid-cols-4'>
                  <div>
                    <span className='text-muted-foreground'>
                      {t('byog.admin.ip_address')}:
                    </span>{' '}
                    <span className='font-mono'>{selectedNode.ip_address}</span>
                  </div>
                  <div>
                    <span className='text-muted-foreground'>
                      {t('byog.admin.ip_type')}:
                    </span>{' '}
                    <span className='capitalize'>{selectedNode.ip_type}</span>
                  </div>
                  <div>
                    <span className='text-muted-foreground'>
                      {t('byog.admin.bandwidth')}:
                    </span>{' '}
                    <span>
                      {selectedNode.bandwidth_down_mbps} /{' '}
                      {selectedNode.bandwidth_up_mbps} Mbps
                    </span>
                  </div>
                  <div>
                    <span className='text-muted-foreground'>
                      {t('byog.admin.ping')}:
                    </span>{' '}
                    <span className='font-mono'>{selectedNode.ping_ms} ms</span>
                  </div>
                  <div>
                    <span className='text-muted-foreground'>
                      {t('byog.admin.location')}:
                    </span>{' '}
                    <span>
                      {selectedNode.city ? `${selectedNode.city}, ` : ''}
                      {selectedNode.country}
                    </span>
                  </div>
                  <div>
                    <span className='text-muted-foreground'>
                      {t('byog.admin.coordinates')}:
                    </span>{' '}
                    <span className='font-mono'>
                      {selectedNode.latitude.toFixed(2)},{' '}
                      {selectedNode.longitude.toFixed(2)}
                    </span>
                  </div>
                </div>
              </div>

              {/* Models Loaded, Download Queue & Cumulative Stats */}
              <div className='grid grid-cols-1 gap-4 md:grid-cols-3'>
                <div className='bg-muted/10 space-y-2 rounded-xl border p-4'>
                  <div className='text-sm font-semibold'>
                    {t('byog.admin.installed_models')}
                  </div>
                  <div className='bg-background max-h-32 overflow-y-auto rounded border p-2.5 font-mono text-xs'>
                    {selectedNode.installed_models ||
                      t('byog.admin.no_models_installed')}
                  </div>
                </div>

                <div className='bg-muted/10 space-y-2 rounded-xl border p-4'>
                  <div className='text-sm font-semibold'>
                    {t('byog.admin.download_queue', 'İndirme Kuyruğu')}
                  </div>
                  <div className='bg-background max-h-32 overflow-y-auto rounded border p-2.5 font-mono text-xs'>
                    {selectedNode.download_queue && selectedNode.download_queue !== '[]'
                      ? selectedNode.download_queue
                      : t('byog.admin.queue_empty', 'Kuyruk Boş')}
                  </div>
                </div>

                <div className='bg-muted/10 space-y-2 rounded-xl border p-4'>
                  <div className='text-sm font-semibold'>
                    {t('byog.admin.cumulative_contributions')}
                  </div>
                  <div className='space-y-1.5 text-xs'>
                    <div className='flex justify-between'>
                      <span className='text-muted-foreground'>
                        {t('byog.admin.total_tokens_processed')}:
                      </span>
                      <span className='font-mono font-medium'>
                        {selectedNode.total_tokens_processed.toLocaleString()}
                      </span>
                    </div>
                    <div className='flex justify-between'>
                      <span className='text-muted-foreground'>
                        {t('byog.admin.total_earned_credits')}:
                      </span>
                      <span className='font-mono font-medium'>
                        {selectedNode.total_earned_credits.toFixed(4)}
                      </span>
                    </div>
                    <div className='flex justify-between'>
                      <span className='text-muted-foreground'>
                        {t('byog.admin.uptime')}:
                      </span>
                      <span className='font-mono'>
                        {(selectedNode.uptime_seconds / 3600).toFixed(1)} hrs
                      </span>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          )}
        </DialogContent>
      </Dialog>
    </div>
  )
}
