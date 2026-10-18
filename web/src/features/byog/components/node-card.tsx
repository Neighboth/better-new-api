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
  Edit2,
  Gamepad2,
  HardDrive,
  Laptop,
  MoreVertical,
  Pause,
  Server,
  Settings2,
  Smartphone,
  Tablet,
  Thermometer,
  Trash2,
  Zap,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardFooter, CardHeader } from '@/components/ui/card'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'

import type { ByogNode } from '../types'

interface NodeCardProps {
  node: ByogNode
  onEditName: (node: ByogNode) => void
  onConfigure: (node: ByogNode) => void
  onDelete: (nodeId: string) => void
}

export function NodeCard({
  node,
  onEditName,
  onConfigure,
  onDelete,
}: NodeCardProps) {
  const { t } = useTranslation()

  const getDeviceIcon = () => {
    switch (node.device_type) {
      case 'mobile':
        return <Smartphone className='text-primary h-5 w-5' />
      case 'tablet':
        return <Tablet className='text-primary h-5 w-5' />
      case 'laptop':
        return <Laptop className='text-primary h-5 w-5' />
      default:
        return <Server className='text-primary h-5 w-5' />
    }
  }

  const getStatusBadge = () => {
    switch (node.status) {
      case 'online':
        return (
          <Badge
            variant='outline'
            className='gap-1 border-emerald-500/40 bg-emerald-500/10 text-xs font-semibold text-emerald-500'
          >
            <span className='h-1.5 w-1.5 animate-pulse rounded-full bg-emerald-500' />
            {t('byog.status_online')}
          </Badge>
        )
      case 'gamer_mode':
        return (
          <Badge
            variant='outline'
            className='gap-1 border-purple-500/40 bg-purple-500/10 text-xs font-semibold text-purple-400'
          >
            <Gamepad2 className='h-3 w-3' />
            {t('byog.status_gamer_mode')}
          </Badge>
        )
      case 'paused':
        return (
          <Badge
            variant='outline'
            className='gap-1 border-amber-500/40 bg-amber-500/10 text-xs font-semibold text-amber-500'
          >
            <Pause className='h-3 w-3' />
            {t('byog.status_paused')}
          </Badge>
        )
      case 'throttled':
        return (
          <Badge
            variant='outline'
            className='gap-1 border-rose-500/40 bg-rose-500/10 text-xs font-semibold text-rose-500'
          >
            <Thermometer className='h-3 w-3' />
            {t('byog.status_throttled')}
          </Badge>
        )
      default:
        return (
          <Badge
            variant='outline'
            className='border-muted text-muted-foreground text-xs'
          >
            {t('byog.status_offline')}
          </Badge>
        )
    }
  }

  const getModelStatusInfo = () => {
    let installed: string[] = []
    let queue: string[] = []
    try {
      if (node.installed_models) installed = JSON.parse(node.installed_models)
    } catch {}
    try {
      if (node.download_queue) queue = JSON.parse(node.download_queue)
    } catch {}

    if (installed && installed.length > 0) {
      return {
        model: installed[0],
        status:
          node.current_tps > 0
            ? t('byog.model_working', 'Üretimde')
            : t('byog.model_ready', 'Hazır (Beklemede)'),
        badgeClass: 'border-emerald-500/40 bg-emerald-500/10 text-emerald-500',
      }
    }
    if (queue && queue.length > 0) {
      return {
        model: queue[0],
        status: t('byog.model_downloading', 'İndiriliyor...'),
        badgeClass: 'border-amber-500/40 bg-amber-500/10 text-amber-500 animate-pulse',
      }
    }
    return {
      model: t('byog.no_model_assigned', 'Model Kuyruğu Bekleniyor'),
      status: t('byog.model_idle', 'Boşta'),
      badgeClass: 'border-slate-500/40 bg-slate-500/10 text-muted-foreground',
    }
  }

  const modelInfo = getModelStatusInfo()

  return (
    <Card className='border-border/80 hover:border-primary/40 relative flex flex-col overflow-hidden transition-all hover:shadow-md'>
      <CardHeader className='pb-3'>
        <div className='flex items-start justify-between gap-2'>
          <div className='flex items-center gap-3'>
            <div className='border-border bg-muted/40 flex h-10 w-10 items-center justify-center rounded-xl border'>
              {getDeviceIcon()}
            </div>
            <div>
              <div className='text-foreground flex items-center gap-1.5 text-base font-bold'>
                {node.device_name}
                <button
                  type='button'
                  onClick={() => onEditName(node)}
                  className='text-muted-foreground/60 hover:text-foreground p-0.5 transition-colors'
                  title={t('common.edit')}
                >
                  <Edit2 className='h-3.5 w-3.5' />
                </button>
              </div>
              <div className='text-muted-foreground max-w-[180px] truncate font-mono text-xs'>
                {node.gpu_model || node.cpu_model || node.node_id.slice(0, 12)}
              </div>
            </div>
          </div>

          <div className='flex items-center gap-1.5'>
            {getStatusBadge()}
            <DropdownMenu>
              <DropdownMenuTrigger
                render={
                  <Button
                    variant='ghost'
                    size='icon'
                    className='text-muted-foreground h-8 w-8'
                  />
                }
              >
                <MoreVertical className='h-4 w-4' />
              </DropdownMenuTrigger>
              <DropdownMenuContent align='end'>
                <DropdownMenuItem
                  onClick={() => onConfigure(node)}
                  className='gap-2'
                >
                  <Settings2 className='h-4 w-4' />
                  {t('byog.configure_node')}
                </DropdownMenuItem>
                <DropdownMenuItem
                  onClick={() => onEditName(node)}
                  className='gap-2'
                >
                  <Edit2 className='h-4 w-4' />
                  {t('byog.rename_device')}
                </DropdownMenuItem>
                <DropdownMenuItem
                  onClick={() => onDelete(node.node_id)}
                  className='text-destructive gap-2'
                >
                  <Trash2 className='h-4 w-4' />
                  {t('common.delete')}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </div>
      </CardHeader>

      <CardContent className='flex-1 space-y-3 pb-3'>
        {/* Mode Tag */}
        <div className='flex items-center justify-between text-xs'>
          <span className='text-muted-foreground'>{t('byog.mode_label')}:</span>
          {node.operating_mode === 'mode_b' ? (
            <Badge
              variant='secondary'
              className='bg-primary/10 text-primary border-primary/20 text-[10px]'
            >
              {t('byog.mode_b_short')}
            </Badge>
          ) : (
            <Badge variant='outline' className='text-[10px]'>
              {t('byog.mode_a_short')}
            </Badge>
          )}
        </div>

        {/* Active Model Status */}
        <div className='bg-muted/40 flex items-center justify-between rounded-lg border p-2 text-xs'>
          <div className='flex flex-col'>
            <span className='text-muted-foreground text-[10px] font-semibold uppercase'>
              {t('byog.active_model', 'Aktif Model')}
            </span>
            <span
              className='text-foreground max-w-[170px] truncate font-mono text-xs font-medium'
              title={modelInfo.model}
            >
              {modelInfo.model}
            </span>
          </div>
          <Badge variant='outline' className={`text-[10px] ${modelInfo.badgeClass}`}>
            {modelInfo.status}
          </Badge>
        </div>

        {/* Telemetry Metrics Grid */}
        <div className='bg-muted/20 grid grid-cols-2 gap-2 rounded-lg border p-2.5 text-xs'>
          <div>
            <span className='text-muted-foreground text-[11px]'>
              {t('byog.speed')}:
            </span>
            <div className='text-foreground flex items-center gap-1 font-mono font-semibold'>
              <Activity className='h-3 w-3 text-emerald-500' />
              {node.current_tps
                ? `${node.current_tps.toFixed(1)} TPS`
                : '-- TPS'}
            </div>
          </div>
          <div>
            <span className='text-muted-foreground text-[11px]'>
              {t('byog.first_token_ttft')}:
            </span>
            <div className='text-foreground flex items-center gap-1 font-mono font-semibold'>
              <Zap className='h-3 w-3 text-amber-500' />
              {node.current_ttft_ms ? `${node.current_ttft_ms} ms` : '< 500 ms'}
            </div>
          </div>
          <div>
            <span className='text-muted-foreground text-[11px]'>
              {t('byog.physical_ram')}:
            </span>
            <div className='text-foreground flex items-center gap-1 font-mono font-semibold'>
              <HardDrive className='text-primary h-3 w-3' />
              {(node.physical_ram_mb / 1024).toFixed(1)} GB
            </div>
          </div>
          <div>
            <span className='text-muted-foreground text-[11px]'>
              {t('byog.total_tokens')}:
            </span>
            <div className='text-foreground font-mono font-semibold'>
              +{node.total_tokens_processed.toLocaleString()}
            </div>
          </div>
        </div>

        {/* Battery & Thermal info if Mobile */}
        {(node.device_type === 'mobile' || node.device_type === 'tablet') && (
          <div className='border-border/40 text-muted-foreground flex items-center justify-between border-t pt-2 text-xs'>
            <div className='flex items-center gap-1.5'>
              {node.is_charging ? (
                <BatteryCharging className='h-3.5 w-3.5 text-emerald-500' />
              ) : (
                <Battery className='h-3.5 w-3.5' />
              )}
              <span>{node.battery_percent}%</span>
            </div>
            {node.battery_temp > 0 && (
              <div className='flex items-center gap-1'>
                <Thermometer className='h-3.5 w-3.5 text-amber-500' />
                <span>{node.battery_temp.toFixed(1)}°C</span>
              </div>
            )}
          </div>
        )}
      </CardContent>

      <CardFooter className='border-border/40 border-t pt-3'>
        <Button
          variant='outline'
          size='sm'
          className='w-full gap-2'
          onClick={() => onConfigure(node)}
        >
          <Settings2 className='h-3.5 w-3.5' />
          {t('byog.configure_node')}
        </Button>
      </CardFooter>
    </Card>
  )
}
