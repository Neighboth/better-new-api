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
  Download,
  Loader2,
  Pencil,
  Plus,
  Search,
  Square,
  Terminal,
  Trash2,
} from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import {
  HfModelPickerDialog,
  type HfPickedModel,
} from '@/components/hf-model-picker-dialog'
import { SectionPageLayout } from '@/components/layout'
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
import { api } from '@/lib/api'
import { formatCurrencyFromUSD } from '@/lib/currency'
import { useSystemConfigStore } from '@/stores/system-config-store'

interface FineTuningGPUOption {
  id: string
  label: string
  kind?: 'cpu' | 'gpu' | 'tpu'
  providers?: string[]
  supports_colab_ram?: boolean
  price_per_second: number
  enabled: boolean
}

interface FineTuningJobItem {
  id: number
  job_id: string
  model: string
  fine_tuned_model: string
  training_file: string
  status: string
  progress?: number
  trained_tokens: number
  error?: string
  provider?: string
  gpu?: string
  method?: string
  task_type?: string
  ram_tier?: string
  model_source?: string
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
  gpu_options?: FineTuningGPUOption[]
  colab_enabled?: boolean
  colab_normal_ram_multiplier?: number
  colab_high_ram_multiplier?: number
  jobs: FineTuningJobItem[]
}

interface FineTuningLogsResponse {
  job_id: string
  fine_tuned_model: string
  status: string
  progress: number
  trained_tokens: number
  error?: string
  logs: string
}

interface UserModelAlias {
  id: number
  alias: string
  target_model: string
  description?: string
  created_at: number
  updated_at: number
}

type FineTuningTaskType = 'text' | 'image' | 'audio' | 'vision'

const TASK_TYPE_CONFIG: Record<
  FineTuningTaskType,
  {
    labelKey: string
    methods: Array<{ id: string; label: string }>
    datasetAccept: string
    datasetHintKey: string
    modelAccept: string
  }
> = {
  text: {
    labelKey: 'Text / LLM',
    methods: [
      { id: 'lora', label: 'LoRA (Low-Rank Adaptation)' },
      { id: 'qlora', label: 'QLoRA (4-bit Quantized LoRA)' },
      { id: 'sft', label: 'SFT (Supervised Fine-Tuning)' },
      { id: 'dpo', label: 'DPO (Direct Preference Optimization)' },
      { id: 'full', label: 'Full Fine-Tuning' },
    ],
    datasetAccept: '.jsonl,.json,.parquet,.csv',
    datasetHintKey:
      'Supported: JSONL (OpenAI chat/messages), JSON, Parquet, or CSV',
    modelAccept: '.safetensors,.gguf,.bin,.pt,.zip,.tar,.gz',
  },
  image: {
    labelKey: 'Image / Diffusion',
    methods: [
      { id: 'lora', label: 'LoRA (Style / Character Adapter)' },
      { id: 'dreambooth', label: 'DreamBooth (Subject Fine-Tuning)' },
      { id: 'textual_inversion', label: 'Textual Inversion (Token Embedding)' },
      { id: 'full', label: 'Full UNet / Checkpoint Fine-Tuning' },
    ],
    datasetAccept: '.zip,.tar,.gz,.parquet',
    datasetHintKey:
      'Supported: ZIP/TAR archive with images + matching .txt/.jsonl captions, or Parquet',
    modelAccept: '.safetensors,.ckpt,.bin,.pt,.zip,.tar,.gz',
  },
  audio: {
    labelKey: 'Audio / Speech',
    methods: [
      { id: 'lora', label: 'LoRA (Voice / Acoustic Adapter)' },
      { id: 'full', label: 'Full Fine-Tuning' },
    ],
    datasetAccept: '.zip,.tar,.gz,.jsonl,.parquet,.csv',
    datasetHintKey:
      'Supported: ZIP/TAR audio clips + metadata.jsonl/csv, or Parquet',
    modelAccept: '.safetensors,.bin,.pt,.zip,.tar,.gz',
  },
  vision: {
    labelKey: 'Vision / Multimodal',
    methods: [
      { id: 'lora', label: 'LoRA (Vision-Language Adapter)' },
      { id: 'qlora', label: 'QLoRA (4-bit Quantized LoRA)' },
      { id: 'sft', label: 'SFT (Supervised Fine-Tuning)' },
      { id: 'full', label: 'Full Fine-Tuning' },
    ],
    datasetAccept: '.jsonl,.json,.zip,.parquet',
    datasetHintKey:
      'Supported: JSONL/JSON multimodal conversations, ZIP (images + metadata.jsonl), or Parquet',
    modelAccept: '.safetensors,.gguf,.bin,.pt,.zip,.tar,.gz',
  },
}

const KNOWN_HW_LABELS: Record<string, string> = {
  CPU: 'CPU (Standard)',
  'cpu-basic': 'CPU: Basic (x2)',
  'cpu-upgrade': 'CPU: Upgrade (x4)',
  T4: 'GPU: NVIDIA T4 (16GB)',
  't4-small': 'GPU: NVIDIA T4 Small (16GB)',
  't4-medium': 'GPU: NVIDIA T4 Medium',
  NVIDIA_TESLA_T4: 'GPU: NVIDIA Tesla T4 (16GB)',
  NvidiaTeslaT4: 'GPU: NVIDIA Tesla T4 x2',
  L4: 'GPU: NVIDIA L4 (24GB)',
  l4x1: 'GPU: NVIDIA L4 x1 (24GB)',
  NVIDIA_L4: 'GPU: NVIDIA L4 (24GB)',
  NvidiaL4: 'GPU: NVIDIA L4',
  A10G: 'GPU: NVIDIA A10G (24GB)',
  NVIDIA_A10G: 'GPU: NVIDIA A10G (24GB)',
  NVIDIA_TESLA_A100: 'GPU: NVIDIA A100 (40GB)',
  NVIDIA_TESLA_A100_80GB: 'GPU: NVIDIA A100 (80GB)',
  'A100-40GB': 'GPU: NVIDIA A100 (40GB)',
  'A100-80GB': 'GPU: NVIDIA A100 (80GB)',
  NVIDIA_H100_80GB: 'GPU: NVIDIA H100 (80GB)',
  H100: 'GPU: NVIDIA H100 (80GB)',
  H200: 'GPU: NVIDIA H200 SXM (141GB)',
  B200: 'GPU: NVIDIA B200 (180GB)',
  B300: 'GPU: NVIDIA B300 (262GB)',
  TPU_V2_8: 'TPU: v2-8',
  TPU_V5E_1: 'TPU: v5e-1',
  TPU_V6E_1: 'TPU: v6e-1',
  TpuV5E8: 'TPU: v5e-8',
  TpuV6E8: 'TPU: v6e-8',
}

export function CustomModelsPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  // Subscribe to currency store so switching currency updates all cost displays instantly
  const currencyConfig = useSystemConfigStore((s) => s.config.currency)
  void currencyConfig

  // Fine-tuning state
  const [ftCreateOpen, setFtCreateOpen] = useState(false)
  const [hfPickerOpen, setHfPickerOpen] = useState(false)
  const [ftTaskType, setFtTaskType] = useState<FineTuningTaskType>('text')
  const [ftModelSource, setFtModelSource] = useState<
    'huggingface' | 'url' | 'upload'
  >('huggingface')
  const [ftBaseModel, setFtBaseModel] = useState('')
  const [ftModelUrl, setFtModelUrl] = useState('')
  const [ftModelFile, setFtModelFile] = useState<File | null>(null)
  const [ftMethod, setFtMethod] = useState<string>('lora')
  const [ftSuffix, setFtSuffix] = useState('')
  const [ftGpu, setFtGpu] = useState('')
  const [ftRamTier, setFtRamTier] = useState<'normal' | 'high'>('normal')
  const [ftEpochs, setFtEpochs] = useState(3)
  const [ftFile, setFtFile] = useState<File | null>(null)
  const [deleteFtTarget, setDeleteFtTarget] =
    useState<FineTuningJobItem | null>(null)
  const [consoleJob, setConsoleJob] = useState<FineTuningJobItem | null>(null)
  const consoleEndRef = useRef<HTMLDivElement | null>(null)
  const consoleViewportRef = useRef<HTMLPreElement | null>(null)
  const stickToBottomRef = useRef(true)

  // Auto model alias state
  const [aliasDialogOpen, setAliasDialogOpen] = useState(false)
  const [editingAlias, setEditingAlias] = useState<UserModelAlias | null>(null)
  const [aliasName, setAliasName] = useState('')
  const [targetModel, setTargetModel] = useState('')
  const [aliasDesc, setAliasDesc] = useState('')
  const [deleteAliasTarget, setDeleteAliasTarget] =
    useState<UserModelAlias | null>(null)

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
          gpu_options: [],
          colab_enabled: false,
          colab_normal_ram_multiplier: 1,
          colab_high_ram_multiplier: 1.5,
          jobs: [],
        }
      )
    },
    refetchInterval: 3000,
  })

  const consoleLogsQuery = useQuery({
    queryKey: ['user-fine-tuning-logs', consoleJob?.job_id],
    queryFn: async () => {
      if (!consoleJob) return null
      const res = await api.get<{
        success: boolean
        data?: FineTuningLogsResponse
      }>(`/api/user/fine_tuning/jobs/${consoleJob.job_id}/logs`)
      return res.data?.data ?? null
    },
    enabled: Boolean(consoleJob),
    refetchInterval: 2000,
  })

  useEffect(() => {
    if (consoleJob && stickToBottomRef.current && consoleEndRef.current) {
      consoleEndRef.current.scrollIntoView({ behavior: 'auto' })
    }
  }, [consoleLogsQuery.data?.logs, consoleJob])

  const allGpuOptions = ftQuery.data?.gpu_options ?? []
  const enabledGpuOptions = useMemo(
    () => allGpuOptions.filter((g) => g.enabled),
    [allGpuOptions]
  )

  const gpuLabelMap = useMemo(() => {
    const map: Record<string, string> = { ...KNOWN_HW_LABELS }
    for (const opt of allGpuOptions) {
      if (opt.id && opt.label) {
        map[opt.id] = opt.label
      }
    }
    return map
  }, [allGpuOptions])

  const formatHardwareLabel = (id?: string) => {
    if (!id) return '—'
    return gpuLabelMap[id] || id
  }

  useEffect(() => {
    if (
      enabledGpuOptions.length > 0 &&
      !enabledGpuOptions.some((g) => g.id === ftGpu)
    ) {
      setFtGpu(enabledGpuOptions[0].id)
    }
  }, [enabledGpuOptions, ftGpu])

  // Keep method valid when task type switches
  useEffect(() => {
    const allowed = TASK_TYPE_CONFIG[ftTaskType].methods
    if (!allowed.some((m) => m.id === ftMethod)) {
      setFtMethod(allowed[0].id)
    }
  }, [ftTaskType, ftMethod])

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

  const selectedGpuObj = enabledGpuOptions.find((g) => g.id === ftGpu)
  const showColabRamSelector = Boolean(selectedGpuObj?.supports_colab_ram)
  const colabNormalMult = ftQuery.data?.colab_normal_ram_multiplier ?? 1
  const colabHighMult = ftQuery.data?.colab_high_ram_multiplier ?? 1.5
  const activeRamMultiplier = showColabRamSelector
    ? ftRamTier === 'high'
      ? colabHighMult
      : colabNormalMult
    : 1
  const estimatedDurationSeconds = Math.max(60, ftEpochs * 120)
  const effectivePricePerSecond =
    (selectedGpuObj?.price_per_second ?? 0.0015) * activeRamMultiplier
  const estimatedRatePerMin = effectivePricePerSecond * 60
  const estimatedTotalCostUSD =
    effectivePricePerSecond * estimatedDurationSeconds

  const createFtMutation = useMutation({
    mutationFn: async () => {
      const formData = new FormData()
      formData.append('task_type', ftTaskType)
      formData.append('model_source', ftModelSource)
      formData.append('method', ftMethod)
      formData.append('suffix', ftSuffix.trim())
      formData.append('gpu', ftGpu)
      formData.append('ram_tier', showColabRamSelector ? ftRamTier : 'normal')
      formData.append('epochs', String(ftEpochs))
      formData.append('estimated_seconds', String(estimatedDurationSeconds))
      if (ftModelSource === 'huggingface') {
        formData.append('model', ftBaseModel.trim())
      } else if (ftModelSource === 'url') {
        formData.append('model_url', ftModelUrl.trim())
        formData.append('model', ftBaseModel.trim() || ftModelUrl.trim())
      } else if (ftModelSource === 'upload' && ftModelFile) {
        formData.append('model_file', ftModelFile)
        formData.append('model', ftBaseModel.trim() || ftModelFile.name)
      }
      if (ftFile) {
        formData.append('file', ftFile)
      }
      // Do NOT set manual Content-Type; let the browser set multipart/form-data with its boundary!
      const res = await api.post('/api/user/fine_tuning/jobs', formData, {
        headers: { 'Content-Type': undefined },
      })
      if (!res.data?.success) {
        throw new Error(
          res.data?.message || t('Failed to create fine-tuning job')
        )
      }
      return res.data.data
    },
    onSuccess: () => {
      toast.success(t('Fine-tuning job created'))
      setFtCreateOpen(false)
      setFtBaseModel('')
      setFtModelUrl('')
      setFtModelFile(null)
      setFtSuffix('')
      setFtFile(null)
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
        throw new Error(
          res.data?.message || t('Failed to stop fine-tuning job')
        )
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
        throw new Error(
          res.data?.message || t('Failed to delete fine-tuning job')
        )
      }
    },
    onSuccess: () => {
      toast.success(t('Fine-tuning model deleted'))
      setDeleteFtTarget(null)
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
      setDeleteAliasTarget(null)
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
      const res = await api.get(
        `/api/user/fine_tuning/jobs/${jobId}/download`,
        {
          responseType: 'blob',
        }
      )
      const disposition = String(res.headers?.['content-disposition'] ?? '')
      const filenameMatch = disposition.match(/filename="?([^";]+)"?/i)
      const filename = filenameMatch?.[1] || `${jobId}-model.tar.gz`
      const url = window.URL.createObjectURL(new Blob([res.data]))
      const a = document.createElement('a')
      a.href = url
      a.download = filename
      document.body.appendChild(a)
      a.click()
      document.body.removeChild(a)
      window.URL.revokeObjectURL(url)
    } catch {
      toast.error(t('Failed to download model file'))
    }
  }

  const handleHfPicked = (picked: HfPickedModel) => {
    setFtBaseModel(
      picked.filename ? `${picked.repoId}/${picked.filename}` : picked.repoId
    )
    if (picked.taskType === 'image') {
      setFtTaskType('image')
    } else if (picked.taskType === 'audio') {
      setFtTaskType('audio')
    } else {
      setFtTaskType('text')
    }
    setHfPickerOpen(false)
  }

  const ftData = ftQuery.data
  const fineTuningEnabled = Boolean(ftData?.enabled)
  const taskCfg = TASK_TYPE_CONFIG[ftTaskType]

  const hasValidModelInput =
    ftModelSource === 'huggingface'
      ? Boolean(ftBaseModel.trim())
      : ftModelSource === 'url'
        ? Boolean(ftModelUrl.trim())
        : Boolean(ftModelFile)

  const renderStatusBadge = (status: string, progress?: number) => {
    switch (status) {
      case 'succeeded':
        return <Badge variant='default'>{t('Succeeded')}</Badge>
      case 'running':
        return (
          <Badge variant='secondary'>
            {progress && progress > 0
              ? `${t('Running')} (${progress}%)`
              : t('Running')}
          </Badge>
        )
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

  const autoModelsCard = (
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
                <TableHead className='text-right'>{t('Actions')}</TableHead>
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
                          onClick={() => setDeleteAliasTarget(item)}
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
  )

  const fineTuningCard = (
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
          disabled={!fineTuningEnabled || enabledGpuOptions.length === 0}
          onClick={() => setFtCreateOpen(true)}
        >
          <Plus className='mr-1.5 h-4 w-4' />
          {t('New Fine-Tuning Job')}
        </Button>
      </CardHeader>
      <CardContent className='space-y-4'>
        <div className='overflow-x-auto'>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Model')}</TableHead>
                <TableHead>{t('Base Model')}</TableHead>
                <TableHead>{t('Method')}</TableHead>
                <TableHead>{t('Accelerator')}</TableHead>
                <TableHead>{t('Status')}</TableHead>
                <TableHead>{t('Estimated Cost')}</TableHead>
                <TableHead className='text-right'>{t('Actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {ftQuery.isLoading ? (
                <TableRow>
                  <TableCell colSpan={7} className='py-8 text-center'>
                    <Loader2 className='text-muted-foreground mx-auto h-5 w-5 animate-spin' />
                  </TableCell>
                </TableRow>
              ) : (ftData?.jobs ?? []).length === 0 ? (
                <TableRow>
                  <TableCell
                    colSpan={7}
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
                    <TableCell className='text-sm'>{job.model}</TableCell>
                    <TableCell className='text-xs uppercase'>
                      <div>{job.method || 'lora'}</div>
                      {job.task_type && (
                        <div className='text-muted-foreground text-[10px] capitalize'>
                          {job.task_type}
                        </div>
                      )}
                    </TableCell>
                    <TableCell className='text-xs'>
                      <div>{formatHardwareLabel(job.gpu)}</div>
                      {job.ram_tier === 'high' && (
                        <div className='text-muted-foreground text-[10px]'>
                          {t('High RAM')}
                        </div>
                      )}
                    </TableCell>
                    <TableCell>
                      {renderStatusBadge(job.status, job.progress)}
                    </TableCell>
                    <TableCell className='text-sm tabular-nums'>
                      {formatCurrencyFromUSD(job.estimated_cost_usd)}
                    </TableCell>
                    <TableCell className='text-right'>
                      <div className='flex items-center justify-end gap-1'>
                        <Button
                          variant='ghost'
                          size='icon'
                          title={t('View Console')}
                          onClick={() => {
                            stickToBottomRef.current = true
                            setConsoleJob(job)
                          }}
                        >
                          <Terminal className='h-4 w-4' />
                        </Button>
                        {(job.status === 'queued' ||
                          job.status === 'running') && (
                          <Button
                            variant='ghost'
                            size='icon'
                            title={t('Stop')}
                            onClick={() => cancelFtMutation.mutate(job.job_id)}
                          >
                            <Square className='h-4 w-4' />
                          </Button>
                        )}
                        {job.status === 'succeeded' && job.has_model_file && (
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
                        )}
                        <Button
                          variant='ghost'
                          size='icon'
                          className='text-destructive hover:text-destructive'
                          title={t('Delete')}
                          onClick={() => setDeleteFtTarget(job)}
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
  )

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>
          <span className='truncate'>{t('Custom Models')}</span>
        </SectionPageLayout.Title>
        <SectionPageLayout.Content>
          <div className='mx-auto flex w-full max-w-6xl flex-col gap-6'>
            {fineTuningEnabled ? (
              <Tabs defaultValue='fine-tuning' className='space-y-4'>
                <TabsList>
                  <TabsTrigger value='fine-tuning'>
                    {t('Fine-Tuned Models')}
                  </TabsTrigger>
                  <TabsTrigger value='auto-models'>
                    {t('Auto Models')}
                  </TabsTrigger>
                </TabsList>

                <TabsContent value='fine-tuning' className='space-y-4'>
                  {fineTuningCard}
                </TabsContent>

                <TabsContent value='auto-models' className='space-y-4'>
                  {autoModelsCard}
                </TabsContent>
              </Tabs>
            ) : (
              autoModelsCard
            )}
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      {/* Create Fine-Tuning Job Dialog */}
      <Dialog open={ftCreateOpen} onOpenChange={setFtCreateOpen}>
        <DialogContent className='sm:max-w-xl max-h-[90vh] overflow-y-auto'>
          <DialogHeader>
            <DialogTitle>{t('New Fine-Tuning Job')}</DialogTitle>
            <DialogDescription>
              {t('Estimated rate: {{rate}} / min.', {
                rate: formatCurrencyFromUSD(estimatedRatePerMin),
              })}
            </DialogDescription>
          </DialogHeader>
          <div className='space-y-4 py-2'>
            {/* Task Type & Model Source */}
            <div className='grid grid-cols-1 gap-3 sm:grid-cols-2'>
              <div className='space-y-1.5'>
                <Label>{t('Task Type')}</Label>
                <Select
                  value={ftTaskType}
                  onValueChange={(v) => setFtTaskType(v as FineTuningTaskType)}
                >
                  <SelectTrigger>
                    <SelectValue>{t(taskCfg.labelKey)}</SelectValue>
                  </SelectTrigger>
                  <SelectContent>
                    {(
                      Object.keys(TASK_TYPE_CONFIG) as FineTuningTaskType[]
                    ).map((key) => (
                      <SelectItem key={key} value={key}>
                        {t(TASK_TYPE_CONFIG[key].labelKey)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className='space-y-1.5'>
                <Label>{t('Model Source')}</Label>
                <Select
                  value={ftModelSource}
                  onValueChange={(v) =>
                    setFtModelSource(v as 'huggingface' | 'url' | 'upload')
                  }
                >
                  <SelectTrigger>
                    <SelectValue>
                      {ftModelSource === 'huggingface'
                        ? 'Hugging Face'
                        : ftModelSource === 'url'
                          ? t('Download from URL')
                          : t('Upload Model File')}
                    </SelectValue>
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value='huggingface'>Hugging Face</SelectItem>
                    <SelectItem value='url'>
                      {t('Download from URL')}
                    </SelectItem>
                    <SelectItem value='upload'>
                      {t('Upload Model File')}
                    </SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>

            {ftModelSource === 'huggingface' && (
              <div className='space-y-1.5'>
                <Label>{t('Base Model')}</Label>
                <div className='flex items-center gap-2'>
                  <Input
                    placeholder='Qwen/Qwen2.5-7B-Instruct'
                    value={ftBaseModel}
                    onChange={(e) => setFtBaseModel(e.target.value)}
                  />
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    className='shrink-0'
                    onClick={() => setHfPickerOpen(true)}
                  >
                    <Search className='mr-1.5 h-3.5 w-3.5' />
                    {t('Browse')}
                  </Button>
                </div>
              </div>
            )}

            {ftModelSource === 'url' && (
              <div className='space-y-1.5'>
                <Label>{t('Model Download URL')}</Label>
                <Input
                  placeholder='https://huggingface.co/.../model.safetensors'
                  value={ftModelUrl}
                  onChange={(e) => setFtModelUrl(e.target.value)}
                />
              </div>
            )}

            {ftModelSource === 'upload' && (
              <div className='space-y-1.5'>
                <Label>{t('Model File')}</Label>
                <Input
                  type='file'
                  accept={taskCfg.modelAccept}
                  onChange={(e) =>
                    setFtModelFile(e.target.files?.[0] ?? null)
                  }
                />
                {ftModelFile && (
                  <p className='text-muted-foreground text-xs'>
                    {ftModelFile.name} (
                    {(ftModelFile.size / (1024 * 1024)).toFixed(2)} MB)
                  </p>
                )}
              </div>
            )}

            {/* Method + Accelerator */}
            <div className='grid grid-cols-1 gap-3 sm:grid-cols-2'>
              <div className='space-y-1.5'>
                <Label>{t('Fine-Tuning Method')}</Label>
                <Select value={ftMethod} onValueChange={setFtMethod}>
                  <SelectTrigger>
                    <SelectValue>
                      {taskCfg.methods.find((m) => m.id === ftMethod)?.label ??
                        ftMethod}
                    </SelectValue>
                  </SelectTrigger>
                  <SelectContent>
                    {taskCfg.methods.map((m) => (
                      <SelectItem key={m.id} value={m.id}>
                        {m.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className='space-y-1.5'>
                <Label>{t('Accelerator')}</Label>
                <Select value={ftGpu} onValueChange={setFtGpu}>
                  <SelectTrigger>
                    <SelectValue>
                      {selectedGpuObj
                        ? selectedGpuObj.label
                        : formatHardwareLabel(ftGpu)}
                    </SelectValue>
                  </SelectTrigger>
                  <SelectContent>
                    {enabledGpuOptions.map((g) => (
                      <SelectItem key={g.id} value={g.id}>
                        {g.label} (
                        {formatCurrencyFromUSD(g.price_per_second * 60)}/m)
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>

            {showColabRamSelector && (
              <div className='space-y-1.5'>
                <Label>{t('RAM Tier')}</Label>
                <Select
                  value={ftRamTier}
                  onValueChange={(v) => setFtRamTier(v as 'normal' | 'high')}
                >
                  <SelectTrigger>
                    <SelectValue>
                      {ftRamTier === 'high'
                        ? `${t('High RAM')} (${colabHighMult}x)`
                        : `${t('Normal RAM')} (${colabNormalMult}x)`}
                    </SelectValue>
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value='normal'>
                      {t('Normal RAM')} ({colabNormalMult}x)
                    </SelectItem>
                    <SelectItem value='high'>
                      {t('High RAM')} ({colabHighMult}x)
                    </SelectItem>
                  </SelectContent>
                </Select>
              </div>
            )}

            <div className='grid grid-cols-2 gap-3'>
              <div className='space-y-1.5'>
                <Label>{t('Epochs')}</Label>
                <Input
                  type='number'
                  min={1}
                  max={50}
                  value={ftEpochs}
                  onChange={(e) =>
                    setFtEpochs(
                      Math.min(50, Math.max(1, Number(e.target.value) || 1))
                    )
                  }
                />
              </div>
              <div className='space-y-1.5'>
                <Label>{t('Model Suffix (Optional)')}</Label>
                <Input
                  placeholder='my-assistant-v1'
                  value={ftSuffix}
                  onChange={(e) => setFtSuffix(e.target.value)}
                />
              </div>
            </div>

            {/* Dataset file upload ONLY (no inline paste to prevent browser freeze on large datasets) */}
            <div className='space-y-1.5'>
              <Label>{t('Training Dataset File')}</Label>
              <Input
                type='file'
                accept={taskCfg.datasetAccept}
                onChange={(e) => setFtFile(e.target.files?.[0] ?? null)}
              />
              <p className='text-muted-foreground text-xs'>
                {t(taskCfg.datasetHintKey)}
              </p>
            </div>

            {/* Live Estimated Total Cost */}
            <div className='bg-muted/30 flex items-center justify-between rounded-lg border px-3.5 py-2.5'>
              <div className='space-y-0.5'>
                <div className='text-xs font-medium'>
                  {t('Estimated Total Cost')}
                </div>
                <div className='text-muted-foreground text-[11px]'>
                  {t('{{rate}} / min · ~{{mins}} min ({{epochs}} epochs)', {
                    rate: formatCurrencyFromUSD(estimatedRatePerMin),
                    mins: Math.round(estimatedDurationSeconds / 60),
                    epochs: ftEpochs,
                  })}
                </div>
              </div>
              <div className='text-base font-semibold tabular-nums'>
                {formatCurrencyFromUSD(estimatedTotalCostUSD)}
              </div>
            </div>
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
                createFtMutation.isPending || !hasValidModelInput || !ftFile
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

      {/* Live Sanitized Console Log Dialog */}
      <Dialog
        open={Boolean(consoleJob)}
        onOpenChange={(open) => {
          if (!open) setConsoleJob(null)
        }}
      >
        <DialogContent className='sm:max-w-3xl max-h-[85vh] flex flex-col'>
          <DialogHeader>
            <DialogTitle className='flex items-center gap-2'>
              <Terminal className='h-4 w-4' />
              {t('Training Console')}
            </DialogTitle>
            <DialogDescription className='flex flex-wrap items-center gap-2 font-mono text-xs'>
              <span>{consoleJob?.fine_tuned_model || consoleJob?.job_id}</span>
              {consoleLogsQuery.data?.status &&
                renderStatusBadge(
                  consoleLogsQuery.data.status,
                  consoleLogsQuery.data.progress
                )}
            </DialogDescription>
          </DialogHeader>
          <pre
            ref={consoleViewportRef}
            onScroll={(e) => {
              const el = e.currentTarget
              const atBottom =
                el.scrollHeight - el.scrollTop - el.clientHeight < 40
              stickToBottomRef.current = atBottom
            }}
            className='bg-zinc-950 text-zinc-100 h-96 overflow-y-auto rounded-lg border p-3.5 font-mono text-xs leading-relaxed whitespace-pre-wrap break-words'
          >
            {consoleLogsQuery.isLoading
              ? t('Loading logs...')
              : consoleLogsQuery.data?.logs ||
                t('Waiting for training worker output...')}
            <div ref={consoleEndRef} />
          </pre>
          <DialogFooter>
            <Button variant='outline' onClick={() => setConsoleJob(null)}>
              {t('Close')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete Fine-Tuned Model Confirm Dialog */}
      <ConfirmDialog
        open={Boolean(deleteFtTarget)}
        onOpenChange={(open) => {
          if (!open) setDeleteFtTarget(null)
        }}
        title={t('Delete fine-tuned model?')}
        desc={t(
          'Are you sure you want to delete "{{model}}"? Training logs, checkpoints, and saved model files will be permanently removed.',
          {
            model:
              deleteFtTarget?.fine_tuned_model || deleteFtTarget?.job_id || '',
          }
        )}
        destructive
        isLoading={deleteFtMutation.isPending}
        handleConfirm={() => {
          if (deleteFtTarget) {
            deleteFtMutation.mutate(deleteFtTarget.job_id)
          }
        }}
        confirmText={t('Delete')}
      />

      {/* Delete Auto Model Confirm Dialog */}
      <ConfirmDialog
        open={Boolean(deleteAliasTarget)}
        onOpenChange={(open) => {
          if (!open) setDeleteAliasTarget(null)
        }}
        title={t('Delete auto model?')}
        desc={t('Are you sure you want to delete "{{alias}}"?', {
          alias: deleteAliasTarget?.alias || '',
        })}
        destructive
        isLoading={deleteAliasMutation.isPending}
        handleConfirm={() => {
          if (deleteAliasTarget) {
            deleteAliasMutation.mutate(deleteAliasTarget.id)
          }
        }}
        confirmText={t('Delete')}
      />

      {/* Hugging Face Picker for Fine-Tuning */}
      <HfModelPickerDialog
        open={hfPickerOpen}
        onOpenChange={setHfPickerOpen}
        onSelect={handleHfPicked}
        apiBasePath='/api/user/fine_tuning/hf'
      />

      {/* Auto Model Alias Create / Edit Dialog */}
      <Dialog open={aliasDialogOpen} onOpenChange={setAliasDialogOpen}>
        <DialogContent className='sm:max-w-md'>
          <DialogHeader>
            <DialogTitle>
              {editingAlias ? t('Edit Auto Model') : t('Add Auto Model')}
            </DialogTitle>
            <DialogDescription>
              {t(
                'Requests using this alias will automatically route to the selected target model.'
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
                list='user-available-models-datalist'
                placeholder='gpt-4o-mini'
                value={targetModel}
                onChange={(e) => setTargetModel(e.target.value)}
              />
              <datalist id='user-available-models-datalist'>
                {(userModelsQuery.data ?? []).map((m) => (
                  <option key={m} value={m} />
                ))}
              </datalist>
            </div>
            <div className='space-y-1.5'>
              <Label>{t('Description (Optional)')}</Label>
              <Input
                placeholder={t('Production chatbot default')}
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
    </>
  )
}
