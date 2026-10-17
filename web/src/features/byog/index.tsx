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
import { Cpu, Gift, RefreshCw, Server, Settings2 } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

import {
  deleteNode,
  getByogStatus,
  getUserNodes,
  getUserRewards,
  updateNode,
} from './api'
import { BannedScreen } from './components/banned-screen'
import { MobileSettingsCard } from './components/mobile-settings-card'
import { NodeCard } from './components/node-card'
import { PairModal } from './components/pair-modal'
import { ResourceSliders } from './components/resource-sliders'
import { RewardStoreTab } from './components/reward-store-tab'
import { ScheduleMatrix } from './components/schedule-matrix'
import type { ByogNode, ByogRewardItem, ByogStatusData } from './types'

export function ByogUserDashboard() {
  const { t } = useTranslation()
  const [statusData, setStatusData] = useState<ByogStatusData | null>(null)
  const [nodes, setNodes] = useState<ByogNode[]>([])
  const [rewards, setRewards] = useState<ByogRewardItem[]>([])
  const [loading, setLoading] = useState(true)

  // Configure Dialog state
  const [selectedNode, setSelectedNode] = useState<ByogNode | null>(null)
  const [configOpen, setConfigOpen] = useState(false)
  const [isSaving, setIsSaving] = useState(false)

  // Rename Dialog state
  const [renameNode, setRenameNode] = useState<ByogNode | null>(null)
  const [newName, setNewName] = useState('')
  const [renameOpen, setRenameOpen] = useState(false)

  const fetchData = async () => {
    try {
      setLoading(true)
      const [status, userNodes, rewardItems] = await Promise.all([
        getByogStatus(),
        getUserNodes(),
        getUserRewards(),
      ])
      setStatusData(status)
      setNodes(userNodes)
      setRewards(rewardItems)
    } catch (err: any) {
      toast.error(err.message || t('byog.fetch_error'))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchData()
  }, [])

  // If user is banned from BYOG, show Section 3.8 Banned Screen
  if (statusData?.is_banned) {
    return <BannedScreen banInfo={statusData.ban_info} />
  }

  const handleSaveConfig = async () => {
    if (!selectedNode) return
    try {
      setIsSaving(true)
      await updateNode(selectedNode.node_id, selectedNode)
      toast.success(t('byog.node_config_saved_success'))
      setConfigOpen(false)
      fetchData()
    } catch (err: any) {
      toast.error(err.message || t('byog.save_error'))
    } finally {
      setIsSaving(false)
    }
  }

  const handleSaveRename = async () => {
    if (!renameNode || !newName.trim()) return
    try {
      await updateNode(renameNode.node_id, { device_name: newName.trim() })
      toast.success(t('byog.device_renamed_success'))
      setRenameOpen(false)
      fetchData()
    } catch (err: any) {
      toast.error(err.message || t('byog.rename_error'))
    }
  }

  const handleDeleteNode = async (nodeId: string) => {
    if (!confirm(t('byog.confirm_delete_device'))) return
    try {
      await deleteNode(nodeId)
      toast.success(t('byog.device_deleted_success'))
      fetchData()
    } catch (err: any) {
      toast.error(err.message || t('byog.delete_error'))
    }
  }

  const userAvailableTokens =
    statusData?.available_tokens ??
    nodes.reduce((acc, n) => acc + (n.total_tokens_processed || 0), 0)

  return (
    <div className='h-full w-full max-w-full flex-1 overflow-x-hidden overflow-y-auto space-y-6 p-3 sm:p-4 md:p-6'>
      {/* Top Banner & Header */}
      <div className='flex flex-col sm:flex-row sm:items-center justify-between gap-3 sm:gap-4'>
        <div className='min-w-0 flex-1'>
          <h1 className='text-xl sm:text-2xl font-bold tracking-tight flex items-center gap-2 flex-wrap'>
            <Cpu className='h-5 w-5 sm:h-6 sm:w-6 text-primary shrink-0' />
            <span className='truncate'>{t('byog.title')}</span>
            <Badge className='bg-primary text-primary-foreground text-xs font-medium shrink-0'>
              {t('byog.earn_badge')}
            </Badge>
          </h1>
          <p className='text-xs sm:text-sm text-muted-foreground mt-1 max-w-2xl line-clamp-2 sm:line-clamp-none'>
            {t('byog.subtitle')}
          </p>
        </div>

        <div className='flex items-center gap-2 shrink-0 self-start sm:self-auto'>
          <Button
            variant='outline'
            size='sm'
            onClick={fetchData}
            className='gap-1.5 text-xs'
          >
            <RefreshCw className={`h-3.5 w-3.5 ${loading ? 'animate-spin' : ''}`} />
            {t('Refresh')}
          </Button>
          <PairModal onPaired={fetchData} />
        </div>
      </div>

      <Tabs defaultValue='nodes' className='space-y-6'>
        <TabsList className='grid w-full max-w-md grid-cols-2 h-auto p-1'>
          <TabsTrigger value='nodes' className='gap-1.5 text-xs py-2 px-2 truncate'>
            <Server className='h-4 w-4 shrink-0' />
            <span className='truncate'>{t('byog.my_compute_nodes_tab')} ({nodes.length})</span>
          </TabsTrigger>
          <TabsTrigger value='rewards' className='gap-1.5 text-xs py-2 px-2 truncate'>
            <Gift className='h-4 w-4 shrink-0' />
            <span className='truncate'>{t('byog.reward_store_tab')}</span>
          </TabsTrigger>
        </TabsList>

        {/* My Compute Nodes Tab */}
        <TabsContent value='nodes' className='space-y-6'>
          {nodes.length === 0 ? (
            <div className='border-border/80 flex flex-col items-center justify-center rounded-2xl border border-dashed p-12 text-center'>
              <div className='bg-primary/10 text-primary mb-4 flex h-16 w-16 items-center justify-center rounded-2xl'>
                <Cpu className='h-8 w-8' />
              </div>
              <h3 className='text-foreground text-lg font-bold'>
                {t('byog.no_nodes_connected_title')}
              </h3>
              <p className='text-muted-foreground mt-1 mb-6 max-w-md text-sm'>
                {t('byog.no_nodes_connected_desc')}
              </p>
              <PairModal onPaired={fetchData} />
            </div>
          ) : (
            <div className='grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3'>
              {nodes.map((n) => (
                <NodeCard
                  key={n.node_id}
                  node={n}
                  onEditName={(node) => {
                    setRenameNode(node)
                    setNewName(node.device_name)
                    setRenameOpen(true)
                  }}
                  onConfigure={(node) => {
                    setSelectedNode({ ...node })
                    setConfigOpen(true)
                  }}
                  onDelete={handleDeleteNode}
                />
              ))}
            </div>
          )}
        </TabsContent>

        {/* Reward Store Tab */}
        <TabsContent value='rewards'>
          <RewardStoreTab
            rewards={rewards}
            userQuota={userAvailableTokens}
            onRewardClaimed={fetchData}
          />
        </TabsContent>
      </Tabs>

      {/* Node Configuration Dialog */}
      <Dialog open={configOpen} onOpenChange={setConfigOpen}>
        <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-3xl'>
          <DialogHeader>
            <DialogTitle className='flex items-center gap-2'>
              <Settings2 className='text-primary h-5 w-5' />
              {t('byog.configure_node_title', {
                name: selectedNode?.device_name,
              })}
            </DialogTitle>
            <DialogDescription>
              {t('byog.configure_node_desc')}
            </DialogDescription>
          </DialogHeader>

          {selectedNode && (
            <div className='space-y-6 py-2'>
              {/* Gamified Resource Sliders */}
              <ResourceSliders
                vramGB={selectedNode.allocated_vram_gb}
                ramGB={selectedNode.allocated_ram_gb}
                cores={selectedNode.allocated_cores}
                diskGB={selectedNode.allocated_disk_gb}
                onVramChange={(v) =>
                  setSelectedNode({ ...selectedNode, allocated_vram_gb: v })
                }
                onRamChange={(r) =>
                  setSelectedNode({ ...selectedNode, allocated_ram_gb: r })
                }
                onCoresChange={(c) =>
                  setSelectedNode({ ...selectedNode, allocated_cores: c })
                }
                onDiskChange={(d) =>
                  setSelectedNode({ ...selectedNode, allocated_disk_gb: d })
                }
              />

              {/* Mode Selection & Power Matrix */}
              <ScheduleMatrix
                operatingMode={selectedNode.operating_mode}
                powerProfile={selectedNode.power_profile}
                onOperatingModeChange={(m) =>
                  setSelectedNode({ ...selectedNode, operating_mode: m })
                }
                onPowerProfileChange={(p) =>
                  setSelectedNode({ ...selectedNode, power_profile: p })
                }
              />

              {/* Mobile Governance (if mobile/tablet) */}
              {(selectedNode.device_type === 'mobile' ||
                selectedNode.device_type === 'tablet') && (
                <MobileSettingsCard
                  wifiOnly={selectedNode.wifi_only_download}
                  dataLimitMB={selectedNode.monthly_data_limit_mb}
                  chargingOnly={selectedNode.run_only_while_charging}
                  batteryCutoffEnabled={selectedNode.battery_cutoff_enabled}
                  batteryCutoffPercent={selectedNode.battery_cutoff_percent}
                  maxBatteryTemp={selectedNode.max_battery_temp}
                  batteryTempSupported={selectedNode.battery_temp_supported}
                  onWifiOnlyChange={(w) =>
                    setSelectedNode({ ...selectedNode, wifi_only_download: w })
                  }
                  onDataLimitChange={(dl) =>
                    setSelectedNode({
                      ...selectedNode,
                      monthly_data_limit_mb: dl,
                    })
                  }
                  onChargingOnlyChange={(ch) =>
                    setSelectedNode({
                      ...selectedNode,
                      run_only_while_charging: ch,
                    })
                  }
                  onBatteryCutoffEnabledChange={(be) =>
                    setSelectedNode({
                      ...selectedNode,
                      battery_cutoff_enabled: be,
                    })
                  }
                  onBatteryCutoffPercentChange={(bp) =>
                    setSelectedNode({
                      ...selectedNode,
                      battery_cutoff_percent: bp,
                    })
                  }
                  onMaxBatteryTempChange={(mt) =>
                    setSelectedNode({ ...selectedNode, max_battery_temp: mt })
                  }
                />
              )}
            </div>
          )}

          <DialogFooter>
            <Button variant='outline' onClick={() => setConfigOpen(false)}>
              {t('common.cancel')}
            </Button>
            <Button onClick={handleSaveConfig} disabled={isSaving}>
              {isSaving ? t('common.saving') : t('common.save')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Rename Dialog */}
      <Dialog open={renameOpen} onOpenChange={setRenameOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('byog.rename_device_title')}</DialogTitle>
            <DialogDescription>
              {t('byog.rename_device_desc')}
            </DialogDescription>
          </DialogHeader>
          <div className='space-y-2 py-2'>
            <Label htmlFor='node-rename'>{t('byog.device_name_label')}</Label>
            <Input
              id='node-rename'
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
            />
          </div>
          <DialogFooter>
            <Button variant='outline' onClick={() => setRenameOpen(false)}>
              {t('common.cancel')}
            </Button>
            <Button onClick={handleSaveRename}>{t('common.save')}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
