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
  Activity,
  Battery,
  BatteryCharging,
  Database,
  Eye,
  Globe,
  HardDrive,
  Laptop,
  Monitor,
  RefreshCw,
  Search,
  Server,
  ShieldAlert,
  ShieldCheck,
  Smartphone,
  Tablet,
  Thermometer,
  Wifi,
  Zap,
} from 'lucide-react'
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

import { getAdminNodes } from '../../byog/api'
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
    } catch (err: any) {
      // silent or initial toast
    } finally {
      setLoading(false)
    }
  }

  function handleInspect(node: ByogNode) {
    setSelectedNode(node)
    setInspectOpen(true)
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

  function getDeviceIcon(type: string) {
    switch (type) {
      case 'mobile':
        return <Smartphone className='h-4 w-4 text-emerald-500' />
      case 'tablet':
        return <Tablet className='h-4 w-4 text-emerald-400' />
      case 'laptop':
        return <Laptop className='h-4 w-4 text-blue-400' />
      default:
        return <Monitor className='h-4 w-4 text-indigo-400' />
    }
  }

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
      {/* 3D Network Globe */}
      <NetworkGlobe nodes={nodes} />

      {/* Aggregate Metrics Bar */}
      <div className='grid grid-cols-2 gap-4 md:grid-cols-4'>
        <Card className='flex items-center gap-3 p-4'>
          <Activity className='h-8 w-8 text-emerald-500' />
          <div>
            <div className='text-2xl font-bold'>
              {onlineNodes.length} / {nodes.length}
            </div>
            <div className='text-muted-foreground text-xs'>
              {t('byog.admin.online_nodes_total')}
            </div>
          </div>
        </Card>

        <Card className='flex items-center gap-3 p-4'>
          <Zap className='h-8 w-8 text-amber-500' />
          <div>
            <div className='text-2xl font-bold'>{aggregateTps.toFixed(1)}</div>
            <div className='text-muted-foreground text-xs'>
              {t('byog.admin.aggregate_tps')}
            </div>
          </div>
        </Card>

        <Card className='flex items-center gap-3 p-4'>
          <HardDrive className='h-8 w-8 text-blue-500' />
          <div>
            <div className='text-2xl font-bold'>
              {totalVramGb.toFixed(0)} GB
            </div>
            <div className='text-muted-foreground text-xs'>
              {t('byog.admin.total_vram_capacity')}
            </div>
          </div>
        </Card>

        <Card className='flex items-center gap-3 p-4'>
          <Smartphone className='h-8 w-8 text-purple-500' />
          <div>
            <div className='text-2xl font-bold'>
              {desktopCount}D / {mobileCount}M
            </div>
            <div className='text-muted-foreground text-xs'>
              {t('byog.admin.desktop_vs_mobile')}
            </div>
          </div>
        </Card>
      </div>

      {/* Filterable Nodes Table */}
      <Card>
        <CardHeader>
          <div className='flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between'>
            <div>
              <CardTitle className='flex items-center gap-2'>
                <Server className='text-primary h-5 w-5' />
                {t('byog.admin.connected_nodes_title')}
              </CardTitle>
              <CardDescription>
                {t('byog.admin.connected_nodes_desc')}
              </CardDescription>
            </div>
            <div className='flex items-center gap-2'>
              <div className='relative w-48 sm:w-64'>
                <Search className='text-muted-foreground absolute top-2.5 left-2.5 h-4 w-4' />
                <Input
                  className='pl-8 text-xs'
                  placeholder={t('byog.admin.search_nodes')}
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                />
              </div>
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
              <Button variant='outline' size='icon' onClick={loadNodes}>
                <RefreshCw className='h-4 w-4' />
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
                        <div className='flex items-center gap-2'>
                          {getDeviceIcon(node.device_type)}
                          <div>
                            <div className='text-sm font-semibold'>
                              {node.device_name}
                            </div>
                            <div className='text-muted-foreground font-mono text-[11px]'>
                              {node.node_id.slice(0, 12)}...
                            </div>
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
                        <div>VRAM: {(node.vram_mb / 1024).toFixed(1)} GB</div>
                      </TableCell>
                      <TableCell className='text-xs'>
                        <div className='flex items-center gap-1 font-mono'>
                          <Wifi className='text-muted-foreground h-3 w-3' />
                          {node.ping_ms} ms
                        </div>
                        <div className='text-muted-foreground text-[10px]'>
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
                        <Button
                          variant='ghost'
                          size='sm'
                          onClick={() => handleInspect(node)}
                          className='gap-1.5 text-xs'
                        >
                          <Eye className='h-3.5 w-3.5' />
                          {t('byog.admin.inspect')}
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Deep Inspect Modal */}
      <Dialog open={inspectOpen} onOpenChange={setInspectOpen}>
        <DialogContent className='max-h-[90vh] max-w-3xl overflow-y-auto'>
          <DialogHeader>
            <DialogTitle className='flex items-center gap-2'>
              <Server className='text-primary h-5 w-5' />
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
                <div className='flex items-center gap-3'>
                  {selectedNode.tee_supported ? (
                    <ShieldCheck className='h-6 w-6 text-emerald-500' />
                  ) : (
                    <ShieldAlert className='h-6 w-6 text-amber-500' />
                  )}
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
                    (Ignored)
                  </div>
                </div>

                <div className='bg-muted/10 space-y-1 rounded-lg border p-3'>
                  <div className='text-muted-foreground text-xs'>
                    {t('byog.admin.vram_memory')}
                  </div>
                  <div className='text-sm font-semibold'>
                    {(selectedNode.vram_mb / 1024).toFixed(1)} GB
                  </div>
                  {selectedNode.shared_vram_mb > 0 && (
                    <div className='text-muted-foreground text-[10px]'>
                      Shared: {(selectedNode.shared_vram_mb / 1024).toFixed(1)}{' '}
                      GB
                    </div>
                  )}
                </div>
              </div>

              {/* Telemetry & Thermals */}
              <div className='grid grid-cols-2 gap-4 md:grid-cols-4'>
                <div className='bg-muted/10 space-y-1 rounded-lg border p-3'>
                  <div className='text-muted-foreground flex items-center gap-1 text-xs'>
                    <Thermometer className='h-3.5 w-3.5 text-red-500' />
                    {t('byog.admin.cpu_temp')}
                  </div>
                  <div className='text-base font-semibold'>
                    {selectedNode.cpu_temp}°C
                  </div>
                </div>

                <div className='bg-muted/10 space-y-1 rounded-lg border p-3'>
                  <div className='text-muted-foreground flex items-center gap-1 text-xs'>
                    <Thermometer className='h-3.5 w-3.5 text-amber-500' />
                    {t('byog.admin.gpu_temp')}
                  </div>
                  <div className='text-base font-semibold'>
                    {selectedNode.gpu_temp}°C
                  </div>
                </div>

                {selectedNode.battery_temp_supported && (
                  <div className='bg-muted/10 space-y-1 rounded-lg border p-3'>
                    <div className='text-muted-foreground flex items-center gap-1 text-xs'>
                      <Thermometer className='h-3.5 w-3.5 text-blue-500' />
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
                    <div className='text-muted-foreground flex items-center gap-1 text-xs'>
                      {selectedNode.is_charging ? (
                        <BatteryCharging className='h-3.5 w-3.5 text-emerald-500' />
                      ) : (
                        <Battery className='text-muted-foreground h-3.5 w-3.5' />
                      )}
                      {t('byog.admin.battery_status')}
                    </div>
                    <div className='text-base font-semibold'>
                      {selectedNode.battery_percent}%{' '}
                      {selectedNode.is_charging && `(${t('byog.charging')})`}
                    </div>
                  </div>
                )}
              </div>

              {/* Network & Geographic Info */}
              <div className='bg-muted/10 space-y-2 rounded-xl border p-4'>
                <div className='flex items-center gap-1.5 text-sm font-semibold'>
                  <Globe className='text-primary h-4 w-4' />
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

              {/* Models Loaded & Cumulative Stats */}
              <div className='grid grid-cols-1 gap-4 md:grid-cols-2'>
                <div className='bg-muted/10 space-y-2 rounded-xl border p-4'>
                  <div className='flex items-center gap-1.5 text-sm font-semibold'>
                    <Database className='h-4 w-4 text-blue-500' />
                    {t('byog.admin.installed_models')}
                  </div>
                  <div className='bg-background max-h-32 overflow-y-auto rounded border p-2.5 font-mono text-xs'>
                    {selectedNode.installed_models ||
                      t('byog.admin.no_models_installed')}
                  </div>
                </div>

                <div className='bg-muted/10 space-y-2 rounded-xl border p-4'>
                  <div className='flex items-center gap-1.5 text-sm font-semibold'>
                    <Zap className='h-4 w-4 text-amber-500' />
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
