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
import { toast } from 'sonner'

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
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
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
  createAdminRule,
  deleteAdminRule,
  getAdminRules,
  updateAdminRule,
} from '../../byog/api'
import type { ByogAssignmentRule } from '../../byog/types'

export function ByogRulesTab() {
  const { t } = useTranslation()
  const [rules, setRules] = useState<ByogAssignmentRule[]>([])
  const [loading, setLoading] = useState(true)
  const [modalOpen, setModalOpen] = useState(false)
  const [editingRule, setEditingRule] =
    useState<Partial<ByogAssignmentRule> | null>(null)

  useEffect(() => {
    loadRules()
  }, [])

  async function loadRules() {
    try {
      setLoading(true)
      const data = await getAdminRules()
      setRules(data || [])
    } catch (err: any) {
      toast.error(err.message || t('byog.admin.load_rules_failed'))
    } finally {
      setLoading(false)
    }
  }

  function handleOpenCreate() {
    setEditingRule({
      name: '',
      device_class: 'all',
      min_vram_mb: 6144,
      min_physical_ram_mb: 16384,
      hardware_family: 'all',
      min_cores: 4,
      vector_instruction_set: 'all',
      target_models: 'qwen-2.5-7b-instruct',
      target_loras: '',
      priority: 10,
      is_enabled: true,
    })
    setModalOpen(true)
  }

  function handleOpenEdit(rule: ByogAssignmentRule) {
    setEditingRule({ ...rule })
    setModalOpen(true)
  }

  async function handleSave() {
    if (!editingRule) return
    try {
      if (editingRule.id) {
        await updateAdminRule(editingRule.id, editingRule)
        toast.success(t('byog.admin.rule_updated_success'))
      } else {
        await createAdminRule(editingRule)
        toast.success(t('byog.admin.rule_created_success'))
      }
      setModalOpen(false)
      loadRules()
    } catch (err: any) {
      toast.error(err.message || t('byog.admin.save_rule_failed'))
    }
  }

  async function handleDelete(id: number) {
    if (!window.confirm(t('byog.admin.confirm_delete_rule'))) return
    try {
      await deleteAdminRule(id)
      toast.success(t('byog.admin.rule_deleted_success'))
      loadRules()
    } catch (err: any) {
      toast.error(err.message || t('byog.admin.delete_rule_failed'))
    }
  }

  async function handleToggle(rule: ByogAssignmentRule) {
    try {
      await updateAdminRule(rule.id, { is_enabled: !rule.is_enabled })
      setRules((prev) =>
        prev.map((r) =>
          r.id === rule.id ? { ...r, is_enabled: !r.is_enabled } : r
        )
      )
    } catch (err: any) {
      toast.error(err.message || t('byog.admin.save_rule_failed'))
    }
  }

  return (
    <div className='space-y-6'>
      <Card>
        <CardHeader>
          <div className='flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between'>
            <div>
              <CardTitle>
                {t('byog.admin.assignment_rules_title')}
              </CardTitle>
              <CardDescription>
                {t('byog.admin.assignment_rules_desc')}
              </CardDescription>
            </div>
            <Button onClick={handleOpenCreate}>
              {t('byog.admin.add_rule')}
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          {loading ? (
            <div className='text-muted-foreground flex h-32 items-center justify-center'>
              {t('byog.loading')}
            </div>
          ) : rules.length === 0 ? (
            <div className='text-muted-foreground py-12 text-center'>
              {t('byog.admin.no_rules_found')}
            </div>
          ) : (
            <div className='overflow-x-auto rounded-md border'>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('byog.admin.rule_name')}</TableHead>
                    <TableHead>{t('byog.admin.device_class')}</TableHead>
                    <TableHead>{t('byog.admin.hardware_family')}</TableHead>
                    <TableHead>{t('byog.admin.min_ram_vram')}</TableHead>
                    <TableHead>{t('byog.admin.target_models')}</TableHead>
                    <TableHead>{t('byog.admin.priority')}</TableHead>
                    <TableHead>{t('byog.admin.status')}</TableHead>
                    <TableHead className='text-right'>
                      {t('byog.admin.actions')}
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {rules.map((rule) => (
                    <TableRow key={rule.id}>
                      <TableCell className='font-medium'>{rule.name}</TableCell>
                      <TableCell>
                        <Badge variant='outline' className='capitalize'>
                          {rule.device_class}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <span className='font-mono text-xs'>
                          {rule.hardware_family}
                        </span>
                      </TableCell>
                      <TableCell className='text-muted-foreground text-xs'>
                        RAM: {(rule.min_physical_ram_mb / 1024).toFixed(0)} GB |
                        VRAM: {(rule.min_vram_mb / 1024).toFixed(0)} GB
                      </TableCell>
                      <TableCell>
                        <div
                          className='max-w-[200px] truncate font-mono text-xs'
                          title={rule.target_models}
                        >
                          {rule.target_models}
                        </div>
                      </TableCell>
                      <TableCell>{rule.priority}</TableCell>
                      <TableCell>
                        <Switch
                          checked={rule.is_enabled}
                          onCheckedChange={() => handleToggle(rule)}
                        />
                      </TableCell>
                      <TableCell className='text-right'>
                        <div className='flex items-center justify-end gap-1.5'>
                          <Button
                            variant='outline'
                            size='sm'
                            className='h-7 px-2 text-xs'
                            onClick={() => handleOpenEdit(rule)}
                          >
                            {t('common.edit')}
                          </Button>
                          <Button
                            variant='outline'
                            size='sm'
                            className='h-7 px-2 text-xs text-destructive hover:text-destructive'
                            onClick={() => handleDelete(rule.id)}
                          >
                            {t('common.delete')}
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

      {/* Create / Edit Dialog */}
      <Dialog open={modalOpen} onOpenChange={setModalOpen}>
        <DialogContent className='max-h-[90vh] max-w-2xl overflow-y-auto'>
          <DialogHeader>
            <DialogTitle>
              {editingRule?.id
                ? t('byog.admin.edit_rule')
                : t('byog.admin.create_rule')}
            </DialogTitle>
            <DialogDescription>
              {t('byog.admin.rule_dialog_desc')}
            </DialogDescription>
          </DialogHeader>

          {editingRule && (
            <div className='grid grid-cols-1 gap-4 py-4 md:grid-cols-2'>
              <div className='space-y-2 md:col-span-2'>
                <Label>{t('byog.admin.rule_name')}</Label>
                <Input
                  value={editingRule.name || ''}
                  onChange={(e) =>
                    setEditingRule({ ...editingRule, name: e.target.value })
                  }
                  placeholder='e.g. Mid-Range Desktop 8GB VRAM'
                />
              </div>

              <div className='space-y-2'>
                <Label>{t('byog.admin.device_class')}</Label>
                <Select
                  value={editingRule.device_class || 'all'}
                  onValueChange={(val) =>
                    setEditingRule({
                      ...editingRule,
                      device_class: val || 'all',
                    })
                  }
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value='all'>
                      {t('byog.admin.all_devices')}
                    </SelectItem>
                    <SelectItem value='desktop'>
                      {t('byog.admin.desktop_server')}
                    </SelectItem>
                    <SelectItem value='laptop'>
                      {t('byog.admin.laptop')}
                    </SelectItem>
                    <SelectItem value='mobile'>
                      {t('byog.admin.mobile_tablet')}
                    </SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div className='space-y-2'>
                <Label>{t('byog.admin.hardware_family')}</Label>
                <Select
                  value={editingRule.hardware_family || 'all'}
                  onValueChange={(val) =>
                    setEditingRule({
                      ...editingRule,
                      hardware_family: val || 'all',
                    })
                  }
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value='all'>
                      {t('byog.admin.all_hardware')}
                    </SelectItem>
                    <SelectItem value='cuda'>NVIDIA CUDA</SelectItem>
                    <SelectItem value='rocm'>AMD ROCm / Vulkan</SelectItem>
                    <SelectItem value='intel'>Intel Iris / Arc</SelectItem>
                    <SelectItem value='metal'>Apple Metal</SelectItem>
                    <SelectItem value='qualcomm'>
                      Qualcomm Snapdragon / QNN
                    </SelectItem>
                    <SelectItem value='mali'>ARM Mali / Tensor</SelectItem>
                    <SelectItem value='cpu'>CPU Only</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div className='space-y-2'>
                <Label>{t('byog.admin.min_vram_mb')}</Label>
                <Input
                  type='number'
                  value={editingRule.min_vram_mb || 0}
                  onChange={(e) =>
                    setEditingRule({
                      ...editingRule,
                      min_vram_mb: parseInt(e.target.value) || 0,
                    })
                  }
                />
                <span className='text-muted-foreground text-xs'>
                  {((editingRule.min_vram_mb || 0) / 1024).toFixed(1)} GB
                </span>
              </div>

              <div className='space-y-2'>
                <Label>{t('byog.admin.min_physical_ram_mb')}</Label>
                <Input
                  type='number'
                  value={editingRule.min_physical_ram_mb || 0}
                  onChange={(e) =>
                    setEditingRule({
                      ...editingRule,
                      min_physical_ram_mb: parseInt(e.target.value) || 0,
                    })
                  }
                />
                <span className='text-muted-foreground text-xs'>
                  {((editingRule.min_physical_ram_mb || 0) / 1024).toFixed(1)}{' '}
                  GB
                </span>
              </div>

              <div className='space-y-2'>
                <Label>{t('byog.admin.min_cores')}</Label>
                <Input
                  type='number'
                  value={editingRule.min_cores || 1}
                  onChange={(e) =>
                    setEditingRule({
                      ...editingRule,
                      min_cores: parseInt(e.target.value) || 1,
                    })
                  }
                />
              </div>

              <div className='space-y-2'>
                <Label>{t('byog.admin.vector_instruction_set')}</Label>
                <Select
                  value={editingRule.vector_instruction_set || 'all'}
                  onValueChange={(val) =>
                    setEditingRule({
                      ...editingRule,
                      vector_instruction_set: val || 'all',
                    })
                  }
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value='all'>
                      {t('byog.admin.all_instructions')}
                    </SelectItem>
                    <SelectItem value='avx2'>x86_64 AVX2</SelectItem>
                    <SelectItem value='avx512'>x86_64 AVX-512</SelectItem>
                    <SelectItem value='neon'>ARM NEON</SelectItem>
                    <SelectItem value='dotprod'>
                      ARMv8.2 DotProd / I8MM
                    </SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div className='space-y-2 md:col-span-2'>
                <Label>{t('byog.admin.target_models_label')}</Label>
                <Input
                  value={editingRule.target_models || ''}
                  onChange={(e) =>
                    setEditingRule({
                      ...editingRule,
                      target_models: e.target.value,
                    })
                  }
                  placeholder='e.g. qwen-2.5-7b-instruct, mistral-7b-instruct'
                />
                <p className='text-muted-foreground text-xs'>
                  {t('byog.admin.target_models_hint')}
                </p>
              </div>

              <div className='space-y-2 md:col-span-2'>
                <Label>{t('byog.admin.target_loras_label')}</Label>
                <Input
                  value={editingRule.target_loras || ''}
                  onChange={(e) =>
                    setEditingRule({
                      ...editingRule,
                      target_loras: e.target.value,
                    })
                  }
                  placeholder='e.g. qwen-coder-adapter, sql-generator-lora'
                />
              </div>

              <div className='space-y-2'>
                <Label>{t('byog.admin.priority')}</Label>
                <Input
                  type='number'
                  value={editingRule.priority || 10}
                  onChange={(e) =>
                    setEditingRule({
                      ...editingRule,
                      priority: parseInt(e.target.value) || 10,
                    })
                  }
                />
              </div>

              <div className='flex items-center justify-between rounded-lg border p-3'>
                <Label className='text-sm font-medium'>
                  {t('byog.admin.rule_is_enabled')}
                </Label>
                <Switch
                  checked={editingRule.is_enabled ?? true}
                  onCheckedChange={(val) =>
                    setEditingRule({ ...editingRule, is_enabled: val })
                  }
                />
              </div>
            </div>
          )}

          <DialogFooter>
            <Button variant='outline' onClick={() => setModalOpen(false)}>
              {t('byog.cancel')}
            </Button>
            <Button onClick={handleSave}>{t('byog.save')}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
