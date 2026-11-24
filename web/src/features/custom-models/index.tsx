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
  ArrowRight,
  Cpu,
  Download,
  Loader2,
  Pencil,
  Plus,
  Sparkles,
  Square,
  Trash2,
} from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Main } from '@/components/layout'
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
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import { api } from '@/lib/api'
import { formatCurrencyFromUSD } from '@/lib/currency'

interface FineTuningJobItem {
  id: number
  job_id: string
  model: string
  fine_tuned_model: string
  training_file: string
  status: string
  trained_tokens: number
  error?: string
  provider?: string
  gpu?: string
  duration_seconds: number
  estimated_cost_usd: number
  has_model_file?: boolean
  created_at: number
  finished_at: number
}

interface FineTuningListResponse {
  enabled: boolean
  price_per_second: number
  gpu_multipliers: Record<string, number>
  jobs: FineTuningJobItem[]
}

interface UserModelAlias {
  id: number
  alias: string
  target_model: string
  description?: string
  created_at: number
  updated_at: number
}

export function CustomModelsPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  // Fine-tuning state
  const [ftCreateOpen, setFtCreateOpen] = useState(false)
  const [ftBaseModel, setFtBaseModel] = useState('')
  const [ftSuffix, setFtSuffix] = useState('')
  const [ftGpu, setFtGpu] = useState('NVIDIA_TESLA_T4')
  const [ftEpochs, setFtEpochs] = useState(3)
  const [ftFile, setFtFile] = useState<File | null>(null)
  const [ftInlineData, setFtInlineData] = useState('')

  // Auto model alias state
  const [aliasDialogOpen, setAliasDialogOpen] = useState(false)
  const [editingAlias, setEditingAlias] = useState<UserModelAlias | null>(null)
  const [aliasName, setAliasName] = useState('')
  const [targetModel, setTargetModel] = useState('')
  const [aliasDesc, setAliasDesc] = useState('')

  const ftQuery = useQuery({
    queryKey: ['user-fine-tuning-jobs'],
    queryFn: async () => {
      const res = await api.get<{
        success: boolean
        data?: FineTuningListResponse
      }>('/api/user/fine_tuning/jobs')
      return (
        res.data?.data ?? {
          enabled: false,
          price_per_second: 0.002,
          gpu_multipliers: {},
          jobs: [],
        }
      )
    },
    refetchInterval: 5000,
  })

  const aliasesQuery = useQuery({
    queryKey: ['user-model-aliases'],
    queryFn: async () => {
      const res = await api.get<{ success: boolean; data?: UserModelAlias[] }>(
        '/api/user/model_aliases'
      )
      return res.data?.data ?? []
    },
  })

  const userModelsQuery = useQuery({
    queryKey: ['user-available-models-for-alias'],
    queryFn: async () => {
      const res = await api.get<{ success: boolean; data?: string[] }>(
        '/api/user/models'
      )
      return Array.isArray(res.data?.data) ? res.data.data : []
    },
    staleTime: 60 * 1000,
  })

  const createFtMutation = useMutation({
    mutationFn: async () => {
      const formData = new FormData()
      formData.append('model', ftBaseModel.trim())
      formData.append('suffix', ftSuffix.trim())
      formData.append('gpu', ftGpu)
      formData.append('epochs', String(ftEpochs))
      if (ftFile) {
        formData.append('file', ftFile)
      } else {
        formData.append('training_data', ftInlineData.trim())
      }
      const res = await api.post('/api/user/fine_tuning/jobs', formData, {
        headers: { 'Content-Type': 'multipart/form-data' },
      })
      if (!res.data?.success) {
        throw new Error(res.data?.message || t('Failed to create fine-tuning job'))
      }
      return res.data.data
    },
    onSuccess: () => {
      toast.success(t('Fine-tuning job created'))
      setFtCreateOpen(false)
      setFtBaseModel('')
      setFtSuffix('')
      setFtFile(null)
      setFtInlineData('')
      void queryClient.invalidateQueries({ queryKey: ['user-fine-tuning-jobs'] })
    },
    onError: (err: Error) => {
      toast.error(err.message || t('Failed to create fine-tuning job'))
    },
  })

  const cancelFtMutation = useMutation({
    mutationFn: async (jobId: string) => {
      const res = await api.post(`/api/user/fine_tuning/jobs/${jobId}/cancel`)
      if (!res.data?.success) {
        throw new Error(res.data?.message || t('Failed to stop fine-tuning job'))
      }
    },
    onSuccess: () => {
      toast.success(t('Fine-tuning job stopped'))
      void queryClient.invalidateQueries({ queryKey: ['user-fine-tuning-jobs'] })
    },
    onError: (err: Error) => {
      toast.error(err.message || t('Failed to stop fine-tuning job'))
    },
  })

  const deleteFtMutation = useMutation({
    mutationFn: async (jobId: string) => {
      const res = await api.delete(`/api/user/fine_tuning/jobs/${jobId}`)
      if (!res.data?.success) {
        throw new Error(res.data?.message || t('Failed to delete fine-tuning job'))
      }
    },
    onSuccess: () => {
      toast.success(t('Fine-tuning model deleted'))
      void queryClient.invalidateQueries({ queryKey: ['user-fine-tuning-jobs'] })
    },
    onError: (err: Error) => {
      toast.error(err.message || t('Failed to delete fine-tuning job'))
    },
  })

  const saveAliasMutation = useMutation({
    mutationFn: async () => {
      const payload = {
        alias: aliasName.trim(),
        target_model: targetModel.trim(),
        description: aliasDesc.trim(),
      }
      const res = editingAlias
        ? await api.put(`/api/user/model_aliases/${editingAlias.id}`, payload)
        : await api.post('/api/user/model_aliases', payload)
      if (!res.data?.success) {
        throw new Error(res.data?.message || t('Failed to save auto model'))
      }
    },
    onSuccess: () => {
      toast.success(
        editingAlias ? t('Auto model updated') : t('Auto model created')
      )
      setAliasDialogOpen(false)
      setEditingAlias(null)
      setAliasName('')
      setTargetModel('')
      setAliasDesc('')
      void queryClient.invalidateQueries({ queryKey: ['user-model-aliases'] })
      void queryClient.invalidateQueries({ queryKey: ['playground-models'] })
    },
    onError: (err: Error) => {
      toast.error(err.message || t('Failed to save auto model'))
    },
  })

  const deleteAliasMutation = useMutation({
    mutationFn: async (id: number) => {
      const res = await api.delete(`/api/user/model_aliases/${id}`)
      if (!res.data?.success) {
        throw new Error(res.data?.message || t('Failed to delete auto model'))
      }
    },
    onSuccess: () => {
      toast.success(t('Auto model deleted'))
      void queryClient.invalidateQueries({ queryKey: ['user-model-aliases'] })
      void queryClient.invalidateQueries({ queryKey: ['playground-models'] })
    },
    onError: (err: Error) => {
      toast.error(err.message || t('Failed to delete auto model'))
    },
  })

  const openCreateAlias = () => {
    setEditingAlias(null)
    setAliasName('')
    setTargetModel('')
    setAliasDesc('')
    setAliasDialogOpen(true)
  }

  const openEditAlias = (item: UserModelAlias) => {
    setEditingAlias(item)
    setAliasName(item.alias)
    setTargetModel(item.target_model)
    setAliasDesc(item.description ?? '')
    setAliasDialogOpen(true)
  }

  const handleDownloadModel = async (jobId: string) => {
    try {
      const res = await api.get(`/api/user/fine_tuning/jobs/${jobId}/download`, {
        responseType: 'blob',
      })
      const url = window.URL.createObjectURL(new Blob([res.data]))
      const a = document.createElement('a')
      a.href = url
      a.download = `${jobId}-model.json`
      document.body.appendChild(a)
      a.click()
      document.body.removeChild(a)
      window.URL.revokeObjectURL(url)
    } catch {
      toast.error(t('Failed to download model file'))
    }
  }

  const ftData = ftQuery.data
  const gpuMultipliers = ftData?.gpu_multipliers ?? {
    CPU: 0.5,
    NVIDIA_TESLA_T4: 1,
    NVIDIA_L4: 1.5,
    NVIDIA_TESLA_A100: 3,
    NVIDIA_H100_80GB: 5,
  }
  const activeGpuMult = gpuMultipliers[ftGpu] ?? 1
  const estimatedRatePerMin =
    (ftData?.price_per_second ?? 0.002) * activeGpuMult * 60

  const renderStatusBadge = (status: string) => {
    switch (status) {
      case 'succeeded':
        return <Badge variant='default'>{t('Succeeded')}</Badge>
      case 'running':
        return <Badge variant='secondary'>{t('Running')}</Badge>
      case 'queued':
        return <Badge variant='outline'>{t('Queued')}</Badge>
      case 'cancelled':
        return <Badge variant='outline'>{t('Cancelled')}</Badge>
      case 'failed':
        return <Badge variant='destructive'>{t('Failed')}</Badge>
      default:
        return <Badge variant='outline'>{status}</Badge>
    }
  }

  return (
    <Main>
      <div className='mx-auto flex w-full max-w-7xl flex-col gap-6 px-3 py-4 sm:px-6 sm:py-6'>
        <div className='flex flex-col gap-1'>
          <h1 className='text-2xl font-bold tracking-tight'>
            {t('Custom Models')}
          </h1>
          <p className='text-muted-foreground text-sm'>
            {t(
              'Manage your fine-tuned models hosted on this server and private auto model aliases for your API requests.'
            )}
          </p>
        </div>

        <Tabs defaultValue='fine-tuning' className='space-y-4'>
          <TabsList>
            <TabsTrigger value='fine-tuning' className='gap-1.5'>
              <Cpu className='h-4 w-4' />
              {t('Fine-Tuned Models')}
            </TabsTrigger>
            <TabsTrigger value='auto-models' className='gap-1.5'>
              <Sparkles className='h-4 w-4' />
              {t('Auto Models')}
            </TabsTrigger>
          </TabsList>

          {/* Section 1: Fine-Tuned Models */}
          <TabsContent value='fine-tuning' className='space-y-4'>
            <Card>
              <CardHeader className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
                <div className='space-y-1'>
                  <CardTitle className='text-base font-semibold'>
                    {t('Fine-Tuned Models')}
                  </CardTitle>
                  <CardDescription>
                    {t(
                      'Create and manage fine-tuning jobs from here or directly via the OpenAI-compatible API endpoint POST /v1/fine_tuning/jobs. Trained model artifacts are hosted on this server.'
                    )}
                  </CardDescription>
                </div>
                <Button
                  size='sm'
                  disabled={!ftData?.enabled}
                  onClick={() => setFtCreateOpen(true)}
                >
                  <Plus className='mr-1.5 h-4 w-4' />
                  {t('New Fine-Tuning Job')}
                </Button>
              </CardHeader>
              <CardContent className='space-y-4'>
                {!ftData?.enabled && !ftQuery.isLoading && (
                  <p className='text-muted-foreground text-sm'>
                    {t(
                      'Fine-tuning is currently disabled by the administrator.'
                    )}
                  </p>
                )}

                <div className='overflow-x-auto'>
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>{t('Model')}</TableHead>
                        <TableHead>{t('Base Model')}</TableHead>
                        <TableHead>{t('GPU')}</TableHead>
                        <TableHead>{t('Status')}</TableHead>
                        <TableHead>{t('Estimated Cost')}</TableHead>
                        <TableHead className='text-right'>
                          {t('Actions')}
                        </TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {ftQuery.isLoading ? (
                        <TableRow>
                          <TableCell colSpan={6} className='py-8 text-center'>
                            <Loader2 className='text-muted-foreground mx-auto h-5 w-5 animate-spin' />
                          </TableCell>
                        </TableRow>
                      ) : (ftData?.jobs ?? []).length === 0 ? (
                        <TableRow>
                          <TableCell
                            colSpan={6}
                            className='text-muted-foreground py-8 text-center text-sm'
                          >
                            {t('No fine-tuning jobs yet.')}
                          </TableCell>
                        </TableRow>
                      ) : (
                        (ftData?.jobs ?? []).map((job) => (
                          <TableRow key={job.job_id}>
                            <TableCell className='font-mono text-xs font-medium'>
                              <div>{job.fine_tuned_model || job.job_id}</div>
                              <div className='text-muted-foreground text-[11px]'>
                                {job.job_id}
                              </div>
                            </TableCell>
                            <TableCell className='text-sm'>
                              {job.model}
                            </TableCell>
                            <TableCell className='text-xs'>
                              {job.gpu || 'NVIDIA_TESLA_T4'}
                            </TableCell>
                            <TableCell>{renderStatusBadge(job.status)}</TableCell>
                            <TableCell className='text-sm tabular-nums'>
                              {formatCurrencyFromUSD(job.estimated_cost_usd)}
                            </TableCell>
                            <TableCell className='text-right'>
                              <div className='flex items-center justify-end gap-1'>
                                {(job.status === 'queued' ||
                                  job.status === 'running') && (
                                  <Button
                                    variant='ghost'
                                    size='icon'
                                    title={t('Stop')}
                                    onClick={() =>
                                      cancelFtMutation.mutate(job.job_id)
                                    }
                                  >
                                    <Square className='h-4 w-4' />
                                  </Button>
                                )}
                                <Button
                                  variant='ghost'
                                  size='icon'
                                  title={t('Download')}
                                  onClick={() =>
                                    void handleDownloadModel(job.job_id)
                                  }
                                >
                                  <Download className='h-4 w-4' />
                                </Button>
                                <Button
                                  variant='ghost'
                                  size='icon'
                                  className='text-destructive hover:text-destructive'
                                  title={t('Delete')}
                                  onClick={() =>
                                    deleteFtMutation.mutate(job.job_id)
                                  }
                                >
                                  <Trash2 className='h-4 w-4' />
                                </Button>
                              </div>
                            </TableCell>
                          </TableRow>
                        ))
                      )}
                    </TableBody>
                  </Table>
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          {/* Section 2: Auto Models (Private Per-User Aliases) */}
          <TabsContent value='auto-models' className='space-y-4'>
            <Card>
              <CardHeader className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
                <div className='space-y-1'>
                  <CardTitle className='text-base font-semibold'>
                    {t('Auto Models')}
                  </CardTitle>
                  <CardDescription>
                    {t(
                      'Define private model aliases that automatically route to a target model on your API calls. Switch the target model anytime without changing your code. Only visible to your account.'
                    )}
                  </CardDescription>
                </div>
                <Button size='sm' onClick={openCreateAlias}>
                  <Plus className='mr-1.5 h-4 w-4' />
                  {t('Add Auto Model')}
                </Button>
              </CardHeader>
              <CardContent>
                <div className='overflow-x-auto'>
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>{t('Model ID (Alias)')}</TableHead>
                        <TableHead>{t('Target Model')}</TableHead>
                        <TableHead>{t('Description')}</TableHead>
                        <TableHead className='text-right'>
                          {t('Actions')}
                        </TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {aliasesQuery.isLoading ? (
                        <TableRow>
                          <TableCell colSpan={4} className='py-8 text-center'>
                            <Loader2 className='text-muted-foreground mx-auto h-5 w-5 animate-spin' />
                          </TableCell>
                        </TableRow>
                      ) : (aliasesQuery.data ?? []).length === 0 ? (
                        <TableRow>
                          <TableCell
                            colSpan={4}
                            className='text-muted-foreground py-8 text-center text-sm'
                          >
                            {t(
                              'No auto models configured yet. Create one like "auto-myapp" pointing to your preferred model.'
                            )}
                          </TableCell>
                        </TableRow>
                      ) : (
                        (aliasesQuery.data ?? []).map((item) => (
                          <TableRow key={item.id}>
                            <TableCell className='font-mono text-sm font-medium'>
                              {item.alias}
                            </TableCell>
                            <TableCell>
                              <span className='inline-flex items-center gap-1.5 font-mono text-sm'>
                                <ArrowRight className='text-muted-foreground h-3.5 w-3.5' />
                                {item.target_model}
                              </span>
                            </TableCell>
                            <TableCell className='text-muted-foreground text-sm'>
                              {item.description || '—'}
                            </TableCell>
                            <TableCell className='text-right'>
                              <div className='flex items-center justify-end gap-1'>
                                <Button
                                  variant='ghost'
                                  size='icon'
                                  title={t('Edit')}
                                  onClick={() => openEditAlias(item)}
                                >
                                  <Pencil className='h-4 w-4' />
                                </Button>
                                <Button
                                  variant='ghost'
                                  size='icon'
                                  className='text-destructive hover:text-destructive'
                                  title={t('Delete')}
                                  onClick={() =>
                                    deleteAliasMutation.mutate(item.id)
                                  }
                                >
                                  <Trash2 className='h-4 w-4' />
                                </Button>
                              </div>
                            </TableCell>
                          </TableRow>
                        ))
                      )}
                    </TableBody>
                  </Table>
                </div>
              </CardContent>
            </Card>
          </TabsContent>
        </Tabs>

        {/* Create Fine-Tuning Job Dialog */}
        <Dialog open={ftCreateOpen} onOpenChange={setFtCreateOpen}>
          <DialogContent className='sm:max-w-lg'>
            <DialogHeader>
              <DialogTitle>{t('New Fine-Tuning Job')}</DialogTitle>
              <DialogDescription>
                {t(
                  'Upload a JSONL training dataset or paste training examples directly. Estimated rate: {{rate}} / min.',
                  { rate: formatCurrencyFromUSD(estimatedRatePerMin) }
                )}
              </DialogDescription>
            </DialogHeader>
            <div className='space-y-4 py-2'>
              <div className='space-y-1.5'>
                <Label>{t('Base Model')}</Label>
                <Input
                  list='ft-base-models'
                  placeholder='Qwen/Qwen2.5-7B-Instruct'
                  value={ftBaseModel}
                  onChange={(e) => setFtBaseModel(e.target.value)}
                />
                <datalist id='ft-base-models'>
                  {(userModelsQuery.data ?? []).map((m) => (
                    <option key={m} value={m} />
                  ))}
                </datalist>
              </div>
              <div className='grid grid-cols-2 gap-3'>
                <div className='space-y-1.5'>
                  <Label>{t('GPU')}</Label>
                  <Select value={ftGpu} onValueChange={setFtGpu}>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {Object.keys(gpuMultipliers).map((g) => (
                        <SelectItem key={g} value={g}>
                          {g} ({gpuMultipliers[g]}x)
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className='space-y-1.5'>
                  <Label>{t('Epochs')}</Label>
                  <Input
                    type='number'
                    min={1}
                    max={50}
                    value={ftEpochs}
                    onChange={(e) =>
                      setFtEpochs(Math.max(1, Number(e.target.value) || 1))
                    }
                  />
                </div>
              </div>
              <div className='space-y-1.5'>
                <Label>{t('Model Suffix (Optional)')}</Label>
                <Input
                  placeholder='my-assistant-v1'
                  value={ftSuffix}
                  onChange={(e) => setFtSuffix(e.target.value)}
                />
              </div>
              <div className='space-y-1.5'>
                <Label>{t('Training Dataset File (.jsonl)')}</Label>
                <Input
                  type='file'
                  accept='.jsonl,.json,.txt'
                  onChange={(e) => setFtFile(e.target.files?.[0] ?? null)}
                />
              </div>
              {!ftFile && (
                <div className='space-y-1.5'>
                  <Label>{t('Or Paste JSONL Dataset')}</Label>
                  <Textarea
                    rows={4}
                    className='font-mono text-xs'
                    placeholder='{"messages":[{"role":"user","content":"Hi"},{"role":"assistant","content":"Hello!"}]}'
                    value={ftInlineData}
                    onChange={(e) => setFtInlineData(e.target.value)}
                  />
                </div>
              )}
            </div>
            <DialogFooter>
              <Button
                variant='outline'
                onClick={() => setFtCreateOpen(false)}
              >
                {t('Cancel')}
              </Button>
              <Button
                disabled={
                  createFtMutation.isPending ||
                  !ftBaseModel.trim() ||
                  (!ftFile && !ftInlineData.trim())
                }
                onClick={() => createFtMutation.mutate()}
              >
                {createFtMutation.isPending && (
                  <Loader2 className='mr-1.5 h-4 w-4 animate-spin' />
                )}
                {t('Start Fine-Tuning')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        {/* Create/Edit Auto Model Alias Dialog */}
        <Dialog open={aliasDialogOpen} onOpenChange={setAliasDialogOpen}>
          <DialogContent className='sm:max-w-md'>
            <DialogHeader>
              <DialogTitle>
                {editingAlias ? t('Edit Auto Model') : t('Add Auto Model')}
              </DialogTitle>
              <DialogDescription>
                {t(
                  'Requests sent to this alias using your API key will transparently route to the target model.'
                )}
              </DialogDescription>
            </DialogHeader>
            <div className='space-y-4 py-2'>
              <div className='space-y-1.5'>
                <Label>{t('Model ID (Alias)')}</Label>
                <Input
                  placeholder='auto-myapp'
                  value={aliasName}
                  onChange={(e) => setAliasName(e.target.value)}
                />
              </div>
              <div className='space-y-1.5'>
                <Label>{t('Target Model')}</Label>
                <Input
                  list='alias-target-models'
                  placeholder='opus-5.5'
                  value={targetModel}
                  onChange={(e) => setTargetModel(e.target.value)}
                />
                <datalist id='alias-target-models'>
                  {(userModelsQuery.data ?? []).map((m) => (
                    <option key={m} value={m} />
                  ))}
                </datalist>
              </div>
              <div className='space-y-1.5'>
                <Label>{t('Description (Optional)')}</Label>
                <Input
                  placeholder={t('Production app default model')}
                  value={aliasDesc}
                  onChange={(e) => setAliasDesc(e.target.value)}
                />
              </div>
            </div>
            <DialogFooter>
              <Button
                variant='outline'
                onClick={() => setAliasDialogOpen(false)}
              >
                {t('Cancel')}
              </Button>
              <Button
                disabled={
                  saveAliasMutation.isPending ||
                  !aliasName.trim() ||
                  !targetModel.trim()
                }
                onClick={() => saveAliasMutation.mutate()}
              >
                {saveAliasMutation.isPending && (
                  <Loader2 className='mr-1.5 h-4 w-4 animate-spin' />
                )}
                {t('Save')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>
    </Main>
  )
}
