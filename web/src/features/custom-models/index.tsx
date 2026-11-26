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
  FolderOpen,
  Loader2,
  Pencil,
  Plus,
  Search,
  Square,
  Terminal,
  Trash2,
  X,
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
import { Textarea } from '@/components/ui/textarea'
import { api } from '@/lib/api'
import { formatCurrencyFromUSD } from '@/lib/currency'
import { formatTimestamp } from '@/lib/format'
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
  dataset_bytes?: number
  estimated_cost_usd: number
  has_model_file?: boolean
  created_at: number
  finished_at: number
  expires_at?: number
  files_purged_at?: number
}

interface FineTuningListResponse {
  enabled: boolean
  price_per_second: number
  gpu_multipliers: Record<string, number>
  gpu_options?: FineTuningGPUOption[]
  colab_enabled?: boolean
  colab_normal_ram_multiplier?: number
  colab_high_ram_multiplier?: number
  retention_days?: number
  expiry_warning_days?: number
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
  system_prompt?: string
  created_at: number
  updated_at: number
}

interface StagedModelUpload {
  upload_id: string
  filename: string
  size: number
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
    datasetAccept: '.jsonl,.json,.parquet,.csv,.txt',
    datasetHintKey:
      'Supported: JSONL (OpenAI chat/messages), JSON, Parquet, or CSV',
    modelAccept: '.safetensors,.bin,.pt,.zip,.tar,.gz',
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
    modelAccept: '.safetensors,.bin,.pt,.zip,.tar,.gz',
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

function parseModelBillions(
  modelName: string,
  paramSizeHint?: string,
  modelFileBytes = 0
): number {
  const re = /(?:^|[^0-9a-z])(\d+(?:\.\d+)?)\s*([bm])(?:[^a-z]|$)/gi
  const parseHint = (raw?: string): number => {
    const s = (raw ?? '').trim()
    if (!s) return 0
    let best = 0
    let m: RegExpExecArray | null
    re.lastIndex = 0
    while ((m = re.exec(s)) !== null) {
      let v = Number(m[1])
      if (!Number.isFinite(v) || v <= 0) continue
      if (m[2].toLowerCase() === 'm') v /= 1000
      if (v > best && v <= 2000) best = v
    }
    return best
  }
  const fromParam = parseHint(paramSizeHint)
  if (fromParam > 0) return fromParam
  const fromName = parseHint(modelName)
  if (fromName > 0) return fromName
  const lower = modelName.toLowerCase()
  if ((lower.includes('deepseek-r1') || lower.includes('deepseek-v3')) && !lower.includes('distill')) {
    return 671
  }
  if (lower.includes('deepseek-v2.5') || lower.includes('deepseek-v2')) {
    return lower.includes('lite') ? 16 : 236
  }
  if (lower.includes('flux')) return 12
  if (lower.includes('sdxl') || lower.includes('xl')) return 6.5
  if (lower.includes('sd3')) return 8
  if (lower.includes('stable-diffusion') || lower.includes('cyberrealistic') || lower.includes('sd1.5')) return 1.2
  if (lower.includes('whisper-large')) return 1.55
  if (modelFileBytes > 0) {
    return Math.min(700, Math.max(0.25, modelFileBytes / 2_000_000_000))
  }
  return 7
}

function getAcceleratorSpeedFactor(gpuId: string): number {
  const trimmed = gpuId.trim()
  const exact: Record<string, number> = {
    NvidiaTeslaT4: 1.85, // Kaggle T4 x2
    't4-medium': 1.35,
    T4: 1.0,
    NVIDIA_TESLA_T4: 1.0,
    't4-small': 1.0,
    l4x4: 7.8,
    L4: 2.1,
    NVIDIA_L4: 2.1,
    NvidiaL4: 2.1,
    l4x1: 2.1,
    'a10g-large': 3.4,
    A10G: 2.4,
    NVIDIA_A10G: 2.4,
    'a10g-small': 2.4,
    'RTX-3090': 2.3,
    'RTX-4090': 4.0,
    l40sx4: 15.5,
    L40S: 4.2,
    l40sx1: 4.2,
    'A100-40GB': 5.4,
    NVIDIA_TESLA_A100: 5.4,
    'A100-80GB': 6.2,
    NVIDIA_TESLA_A100_80GB: 6.2,
    A100: 6.2,
    'a100-large': 6.2,
    a100x2: 12.0,
    a100x4: 23.0,
    a100x8: 44.0,
    'RTX-PRO-6000': 7.0,
    NVIDIA_RTX_PRO_6000: 7.0,
    H100: 12.0,
    'H100-SXM': 12.0,
    'H100-80GB': 12.0,
    NVIDIA_H100_80GB: 12.0,
    h100x1: 12.0,
    h100x4: 45.0,
    h100x8: 85.0,
    H200: 14.5,
    h200x1: 14.5,
    h200x8: 105.0,
    B200: 18.0,
    B300: 24.0,
    TPU_V2_8: 3.6,
    TPU_V5E_1: 3.0,
    TPU_V6E_1: 4.2,
    TpuV5E8: 10.5,
    TpuV6E8: 16.0,
    'cpu-upgrade': 0.12,
    CPU: 0.07,
    'cpu-basic': 0.07,
  }
  if (trimmed in exact) return exact[trimmed]
  const upper = trimmed.toUpperCase()
  if (upper.includes('B300')) return 24.0
  if (upper.includes('B200')) return 18.0
  if (upper.includes('H200')) return 14.5
  if (upper.includes('H100')) return 12.0
  if (upper.includes('A100')) return 5.8
  if (upper.includes('RTX-PRO-6000') || upper.includes('RTX_PRO_6000')) return 7.0
  if (upper.includes('L40S')) return 4.2
  if (upper.includes('4090')) return 4.0
  if (upper.includes('TPU')) return 3.5
  if (upper.includes('A10G') || upper.includes('3090')) return 2.4
  if (upper.includes('L4')) return 2.1
  if (upper.includes('T4')) return 1.0
  if (upper.includes('CPU')) return 0.07
  return 1.0
}

function estimateDurationSeconds(params: {
  gpuId: string
  taskType: FineTuningTaskType
  method: string
  epochs: number
  datasetBytes: number
  datasetSamples: number
  modelName: string
  modelParamHint?: string
  modelFileBytes?: number
}): number {
  const safeEpochs = Math.max(1, params.epochs)
  const datasetBytes = Math.max(0, params.datasetBytes)
  const modelB = parseModelBillions(
    params.modelName,
    params.modelParamHint,
    params.modelFileBytes ?? 0
  )
  const speed = getAcceleratorSpeedFactor(params.gpuId)
  const taskFactor =
    params.taskType === 'image'
      ? 1.85
      : params.taskType === 'vision'
        ? 1.6
        : params.taskType === 'audio'
          ? 1.35
          : 1.0
  const methodFactor =
    params.method === 'full'
      ? 2.2
      : params.method === 'dpo'
        ? 1.65
        : params.method === 'dreambooth'
          ? 1.45
          : params.method === 'sft'
            ? 1.15
            : params.method === 'qlora'
              ? 1.1
              : 1.0

  const samples =
    params.datasetSamples > 0
      ? params.datasetSamples
      : datasetBytes > 0
        ? Math.max(16, Math.floor(datasetBytes / 180))
        : 100

  const tokensFromBytes = datasetBytes / 4
  const tokensFromSamples =
    params.taskType === 'image' ? samples * 320 : samples * 24
  const estTokensPerEpoch = Math.max(tokensFromBytes, tokensFromSamples)

  const setupSec = Math.min(2400, 60 + modelB * 18)
  const tokenComputePerEpoch =
    ((estTokensPerEpoch / 650) * (modelB / 7) * methodFactor * taskFactor) /
    speed
  const stepsPerEpoch = Math.ceil(samples / 16)
  const stepOverheadPerEpoch =
    (stepsPerEpoch * (0.28 + 0.09 * modelB) * taskFactor) / Math.sqrt(speed)
  const exportSec = Math.min(600, 30 + modelB * 3)

  const total = Math.round(
    setupSec + safeEpochs * (tokenComputePerEpoch + stepOverheadPerEpoch) + exportSec
  )
  return Math.min(14 * 86400, Math.max(45, total))
}

function formatDurationEstimate(totalSeconds: number, t: (key: string, opts?: any) => string): string {
  const mins = Math.max(1, Math.round(totalSeconds / 60))
  if (mins < 60) {
    return `${mins} ${t('min')}`
  }
  const hours = Math.floor(mins / 60)
  const remMins = mins % 60
  return remMins > 0 ? `${hours}h ${remMins}m` : `${hours}h`
}

async function countDatasetSamplesFromFile(file: File): Promise<number> {
  const ext = file.name.split('.').pop()?.toLowerCase() ?? ''
  if (!['jsonl', 'csv', 'txt', 'json'].includes(ext)) {
    return Math.max(16, Math.floor(file.size / 256))
  }
  // Stream-scan up to 64MB in chunks to count exact lines without blowing browser RAM
  const limitBytes = Math.min(file.size, 64 * 1024 * 1024)
  if (ext === 'json' && file.size <= 16 * 1024 * 1024) {
    try {
      const parsed = JSON.parse(await file.text())
      if (Array.isArray(parsed) && parsed.length > 0) return parsed.length
    } catch {
      // fall back to newline count
    }
  }
  const chunkSize = 2 * 1024 * 1024
  let offset = 0
  let nonEmptyLines = 0
  let sawCharsOnLine = false
  while (offset < limitBytes) {
    const slice = file.slice(offset, Math.min(limitBytes, offset + chunkSize))
    const buf = new Uint8Array(await slice.arrayBuffer())
    for (let i = 0; i < buf.length; i++) {
      const b = buf[i]
      if (b === 10) {
        if (sawCharsOnLine) nonEmptyLines++
        sawCharsOnLine = false
      } else if (b !== 13 && b !== 32 && b !== 9) {
        sawCharsOnLine = true
      }
    }
    offset += chunkSize
  }
  if (sawCharsOnLine) nonEmptyLines++
  if (ext === 'csv' && nonEmptyLines > 1) nonEmptyLines--
  if (limitBytes < file.size && nonEmptyLines > 0) {
    nonEmptyLines = Math.round(nonEmptyLines * (file.size / limitBytes))
  }
  return Math.max(1, nonEmptyLines)
}

function formatBytesHuman(bytes: number): string {
  if (bytes >= 1024 * 1024 * 1024) {
    return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`
  }
  if (bytes >= 1024 * 1024) {
    return `${(bytes / (1024 * 1024)).toFixed(2)} MB`
  }
  return `${Math.max(1, Math.round(bytes / 1024))} KB`
}

export function CustomModelsPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const currencyConfig = useSystemConfigStore((s) => s.config.currency)
  void currencyConfig

  // Fine-tuning state
  const [ftCreateOpen, setFtCreateOpen] = useState(false)
  const [hfPickerOpen, setHfPickerOpen] = useState(false)
  const [ftTaskType, setFtTaskType] = useState<FineTuningTaskType>('text')
  const [ftBaseModel, setFtBaseModel] = useState('')
  const [ftModelParamSize, setFtModelParamSize] = useState('')
  const [stagedUpload, setStagedUpload] = useState<StagedModelUpload | null>(
    null
  )
  const [uploadingModel, setUploadingModel] = useState(false)
  const [uploadPercent, setUploadPercent] = useState(0)
  const [ftMethod, setFtMethod] = useState<string>('lora')
  const [ftSuffix, setFtSuffix] = useState('')
  const [ftGpu, setFtGpu] = useState('')
  const [ftRamTier, setFtRamTier] = useState<'normal' | 'high'>('normal')
  const [ftEpochs, setFtEpochs] = useState(3)
  const [ftFile, setFtFile] = useState<File | null>(null)
  const [ftDatasetSamples, setFtDatasetSamples] = useState(0)
  const [deleteFtTarget, setDeleteFtTarget] =
    useState<FineTuningJobItem | null>(null)
  const [consoleJob, setConsoleJob] = useState<FineTuningJobItem | null>(null)
  const modelFileInputRef = useRef<HTMLInputElement | null>(null)
  const stagedUploadRef = useRef<StagedModelUpload | null>(null)
  const consoleEndRef = useRef<HTMLDivElement | null>(null)
  const consoleViewportRef = useRef<HTMLPreElement | null>(null)
  const stickToBottomRef = useRef(true)

  // Auto model alias state
  const [aliasDialogOpen, setAliasDialogOpen] = useState(false)
  const [editingAlias, setEditingAlias] = useState<UserModelAlias | null>(null)
  const [aliasName, setAliasName] = useState('')
  const [targetModel, setTargetModel] = useState('')
  const [aliasDesc, setAliasDesc] = useState('')
  const [aliasSystemPrompt, setAliasSystemPrompt] = useState('')
  const [deleteAliasTarget, setDeleteAliasTarget] =
    useState<UserModelAlias | null>(null)

  useEffect(() => {
    stagedUploadRef.current = stagedUpload
  }, [stagedUpload])

  const cleanupStagedUpload = async (uploadId?: string) => {
    const id = uploadId ?? stagedUploadRef.current?.upload_id
    if (!id) return
    try {
      await api.delete(`/api/user/fine_tuning/staged_model/${id}`)
    } catch {
      // ignore cleanup error
    }
  }

  // Clean up any abandoned staged model upload on unmount
  useEffect(() => {
    return () => {
      if (stagedUploadRef.current?.upload_id) {
        void api
          .delete(
            `/api/user/fine_tuning/staged_model/${stagedUploadRef.current.upload_id}`
          )
          .catch(() => {})
      }
    }
  }, [])

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
          retention_days: 30,
          expiry_warning_days: 3,
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
  const datasetBytes = ftFile?.size ?? 0
  const activeModelName = stagedUpload?.filename || ftBaseModel.trim()
  const estimatedModelBillions = parseModelBillions(
    activeModelName,
    ftModelParamSize,
    stagedUpload?.size ?? 0
  )
  const estimatedDurationSeconds = estimateDurationSeconds({
    gpuId: ftGpu || 'CPU',
    taskType: ftTaskType,
    method: ftMethod,
    epochs: ftEpochs,
    datasetBytes,
    datasetSamples: ftDatasetSamples,
    modelName: activeModelName,
    modelParamHint: ftModelParamSize,
    modelFileBytes: stagedUpload?.size ?? 0,
  })
  const effectivePricePerSecond =
    (selectedGpuObj?.price_per_second ?? 0.0015) * activeRamMultiplier
  const estimatedRatePerMin = effectivePricePerSecond * 60
  const estimatedTotalCostUSD =
    effectivePricePerSecond * estimatedDurationSeconds

  const handleDatasetFileSelected = async (file: File | null) => {
    setFtFile(file)
    if (!file) {
      setFtDatasetSamples(0)
      return
    }
    try {
      const count = await countDatasetSamplesFromFile(file)
      setFtDatasetSamples(count)
    } catch {
      setFtDatasetSamples(Math.max(16, Math.floor(file.size / 180)))
    }
  }

  const handleLocalModelSelected = async (file: File | null) => {
    if (!file) return
    if (file.name.toLowerCase().endsWith('.gguf')) {
      toast.error(
        t(
          'GGUF files are inference-only and cannot be fine-tuned. Please select a .safetensors, .pt, or .bin file.'
        )
      )
      return
    }
    // Clear any previously entered/selected model and delete any earlier staged upload
    const prevUploadId = stagedUpload?.upload_id
    setFtBaseModel('')
    setFtModelParamSize('')
    setStagedUpload(null)
    if (prevUploadId) {
      void cleanupStagedUpload(prevUploadId)
    }

    setUploadingModel(true)
    setUploadPercent(0)
    try {
      const fd = new FormData()
      fd.append('model_file', file)
      const res = await api.post<{
        success: boolean
        message?: string
        data?: StagedModelUpload
      }>('/api/user/fine_tuning/upload_model', fd, {
        headers: { 'Content-Type': undefined },
        onUploadProgress: (evt) => {
          if (evt.total && evt.total > 0) {
            setUploadPercent(Math.min(100, Math.round((evt.loaded / evt.total) * 100)))
          }
        },
      })
      if (!res.data?.success || !res.data.data) {
        throw new Error(res.data?.message || t('Model upload failed'))
      }
      setUploadPercent(100)
      setStagedUpload(res.data.data)
      toast.success(t('Model uploaded and ready for training'))
    } catch (err: any) {
      toast.error(err?.response?.data?.message || err?.message || t('Model upload failed'))
    } finally {
      setUploadingModel(false)
      if (modelFileInputRef.current) {
        modelFileInputRef.current.value = ''
      }
    }
  }

  const handleCloseFtDialog = (open: boolean) => {
    if (!open) {
      // Lock dialog while uploading
      if (uploadingModel) return
      if (stagedUpload?.upload_id) {
        void cleanupStagedUpload(stagedUpload.upload_id)
        setStagedUpload(null)
      }
      setFtCreateOpen(false)
    } else {
      setFtCreateOpen(true)
    }
  }

  const createFtMutation = useMutation({
    mutationFn: async () => {
      const formData = new FormData()
      formData.append('task_type', ftTaskType)
      formData.append('method', ftMethod)
      formData.append('suffix', ftSuffix.trim())
      formData.append('gpu', ftGpu)
      formData.append('ram_tier', showColabRamSelector ? ftRamTier : 'normal')
      formData.append('epochs', String(ftEpochs))

      if (stagedUpload) {
        formData.append('model_source', 'upload')
        formData.append('staged_upload_id', stagedUpload.upload_id)
        formData.append('model', stagedUpload.filename)
      } else {
        const trimmed = ftBaseModel.trim()
        const isUrl =
          trimmed.startsWith('http://') || trimmed.startsWith('https://')
        formData.append('model_source', isUrl ? 'url' : 'huggingface')
        if (isUrl) {
          formData.append('model_url', trimmed)
        }
        formData.append('model', trimmed)
      }
      if (ftModelParamSize.trim()) {
        formData.append('model_param_size', ftModelParamSize.trim())
      }
      if (ftFile) {
        formData.append('file', ftFile)
      }
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
      setStagedUpload(null)
      setFtCreateOpen(false)
      setFtBaseModel('')
      setFtModelParamSize('')
      setFtSuffix('')
      setFtFile(null)
      setFtDatasetSamples(0)
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
        system_prompt: aliasSystemPrompt.trim(),
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
      setAliasSystemPrompt('')
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
    setAliasSystemPrompt('')
    setAliasDialogOpen(true)
  }

  const openEditAlias = (item: UserModelAlias) => {
    setEditingAlias(item)
    setAliasName(item.alias)
    setTargetModel(item.target_model)
    setAliasDesc(item.description ?? '')
    setAliasSystemPrompt(item.system_prompt ?? '')
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
      const url = window.URL.createObjectURL(new Blob([res.data]))
      const a = document.createElement('a')
      a.href = url
      a.download = `${jobId}.zip`
      document.body.appendChild(a)
      a.click()
      document.body.removeChild(a)
      window.URL.revokeObjectURL(url)
    } catch {
      toast.error(t('Failed to download model file'))
    }
  }

  const handleHfPicked = (picked: HfPickedModel) => {
    if (stagedUpload?.upload_id) {
      void cleanupStagedUpload(stagedUpload.upload_id)
      setStagedUpload(null)
    }
    setFtBaseModel(
      picked.filename ? `${picked.repoId}/${picked.filename}` : picked.repoId
    )
    setFtModelParamSize(picked.parameterSize ?? '')
    // Never overwrite the user's chosen task type when picking a model
    setHfPickerOpen(false)
  }

  const ftData = ftQuery.data
  const fineTuningEnabled = Boolean(ftData?.enabled)
  const taskCfg = TASK_TYPE_CONFIG[ftTaskType]
  const hasValidModelInput = Boolean(stagedUpload || ftBaseModel.trim())

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
                      <div className='flex flex-wrap items-center gap-1.5'>
                        <span>{item.alias}</span>
                        {item.system_prompt?.trim() && (
                          <Badge
                            variant='secondary'
                            className='font-sans text-[10px] font-normal'
                            title={item.system_prompt}
                          >
                            {t('System Prompt')}
                          </Badge>
                        )}
                      </div>
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
              'Create and manage fine-tuning jobs from here or directly via the OpenAI-compatible API endpoint POST /v1/fine_tuning/jobs. Trained model archives (.zip) are retained for {{days}} days.',
              { days: ftData?.retention_days ?? 30 }
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
                        {job.expires_at && !job.files_purged_at
                          ? ` · ${t('Expires')} ${formatTimestamp(job.expires_at)}`
                          : ''}
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
                            title={t('Download (.zip)')}
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
      <Dialog
        open={ftCreateOpen}
        onOpenChange={handleCloseFtDialog}
        disablePointerDismissal={uploadingModel}
      >
        <DialogContent
          className='max-h-[90vh] overflow-y-auto sm:max-w-xl'
          showCloseButton={!uploadingModel}
        >
          <DialogHeader>
            <DialogTitle>{t('New Fine-Tuning Job')}</DialogTitle>
            <DialogDescription>
              {t('Estimated rate: {{rate}} / min.', {
                rate: formatCurrencyFromUSD(estimatedRatePerMin),
              })}
            </DialogDescription>
          </DialogHeader>
          <div className='space-y-4 py-2'>
            {/* Task Type */}
            <div className='space-y-1.5'>
              <Label>{t('Task Type')}</Label>
              <Select
                value={ftTaskType}
                disabled={uploadingModel}
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

            {/* Base Model (Hugging Face ID / URL + Browse + Local Upload Folder button) */}
            <div className='space-y-1.5'>
              <Label>{t('Base Model')}</Label>
              <div className='flex items-center gap-2'>
                <Input
                  placeholder={t(
                    'Hugging Face repo ID or direct download URL (e.g. Qwen/Qwen2.5-7B-Instruct)'
                  )}
                  disabled={uploadingModel}
                  value={stagedUpload ? '' : ftBaseModel}
                  onChange={(e) => {
                    if (stagedUpload?.upload_id) {
                      void cleanupStagedUpload(stagedUpload.upload_id)
                      setStagedUpload(null)
                    }
                    setFtModelParamSize('')
                    setFtBaseModel(e.target.value)
                  }}
                />
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  disabled={uploadingModel}
                  className='shrink-0'
                  onClick={() => setHfPickerOpen(true)}
                >
                  <Search className='mr-1.5 h-3.5 w-3.5' />
                  {t('Browse')}
                </Button>
                <Button
                  type='button'
                  variant='outline'
                  size='icon'
                  disabled={uploadingModel}
                  title={t('Upload model file from device (.safetensors, .pt, .bin)')}
                  className='shrink-0'
                  onClick={() => modelFileInputRef.current?.click()}
                >
                  {uploadingModel ? (
                    <Loader2 className='h-4 w-4 animate-spin' />
                  ) : (
                    <FolderOpen className='h-4 w-4' />
                  )}
                </Button>
                <input
                  ref={modelFileInputRef}
                  type='file'
                  accept={taskCfg.modelAccept}
                  className='hidden'
                  onChange={(e) =>
                    void handleLocalModelSelected(e.target.files?.[0] ?? null)
                  }
                />
              </div>

              {uploadingModel && (
                <div className='bg-muted/40 space-y-1.5 rounded-lg border p-2.5'>
                  <div className='flex items-center justify-between text-xs'>
                    <span className='flex items-center gap-1.5 font-medium'>
                      <Loader2 className='h-3.5 w-3.5 animate-spin' />
                      {t('Uploading model file to server...')}
                    </span>
                    <span className='font-mono font-semibold tabular-nums'>
                      {uploadPercent}%
                    </span>
                  </div>
                  <div className='bg-muted h-1.5 w-full overflow-hidden rounded-full'>
                    <div
                      className='bg-primary h-full transition-all duration-150'
                      style={{ width: `${uploadPercent}%` }}
                    />
                  </div>
                </div>
              )}

              {stagedUpload && !uploadingModel && (
                <div className='bg-muted/40 flex items-center justify-between gap-2 rounded-lg border px-3 py-2 text-xs'>
                  <div className='min-w-0 truncate'>
                    <span className='font-medium'>{stagedUpload.filename}</span>{' '}
                    <span className='text-muted-foreground'>
                      ({formatBytesHuman(stagedUpload.size)})
                    </span>
                  </div>
                  <Button
                    type='button'
                    variant='ghost'
                    size='icon'
                    className='h-6 w-6 shrink-0'
                    title={t('Remove uploaded model')}
                    onClick={() => {
                      void cleanupStagedUpload(stagedUpload.upload_id)
                      setStagedUpload(null)
                    }}
                  >
                    <X className='h-3.5 w-3.5' />
                  </Button>
                </div>
              )}
            </div>

            {/* Method + Accelerator */}
            <div className='grid grid-cols-1 gap-3 sm:grid-cols-2'>
              <div className='space-y-1.5'>
                <Label>{t('Fine-Tuning Method')}</Label>
                <Select
                  value={ftMethod}
                  disabled={uploadingModel}
                  onValueChange={(v) => {
                    if (v) setFtMethod(v)
                  }}
                >
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
                <Select
                  value={ftGpu}
                  disabled={uploadingModel}
                  onValueChange={(v) => {
                    if (v) setFtGpu(v)
                  }}
                >
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
                  disabled={uploadingModel}
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
                  disabled={uploadingModel}
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
                  disabled={uploadingModel}
                  value={ftSuffix}
                  onChange={(e) => setFtSuffix(e.target.value)}
                />
              </div>
            </div>

            {/* Dataset file upload */}
            <div className='space-y-1.5'>
              <Label>{t('Training Dataset File')}</Label>
              <Input
                type='file'
                disabled={uploadingModel}
                accept={taskCfg.datasetAccept}
                onChange={(e) =>
                  void handleDatasetFileSelected(e.target.files?.[0] ?? null)
                }
              />
              <p className='text-muted-foreground text-xs'>
                {t(taskCfg.datasetHintKey)}
                {ftFile
                  ? ` · ${formatBytesHuman(ftFile.size)}${ftDatasetSamples > 0 ? ` · ${ftDatasetSamples.toLocaleString()} ${t('samples')}` : ''}`
                  : ''}
              </p>
            </div>

            {/* Dataset-Size & Model-Size Aware Live Estimated Total Cost */}
            <div className='bg-muted/30 flex items-center justify-between rounded-lg border px-3.5 py-2.5'>
              <div className='space-y-0.5'>
                <div className='text-xs font-medium'>
                  {t('Estimated Total Cost')}
                </div>
                <div className='text-muted-foreground text-[11px]'>
                  {t(
                    '{{rate}} / min · ~{{duration}} (~{{model}}B, {{epochs}} epochs, {{dataset}})',
                    {
                      rate: formatCurrencyFromUSD(estimatedRatePerMin),
                      duration: formatDurationEstimate(estimatedDurationSeconds, t),
                      model:
                        estimatedModelBillions >= 10
                          ? Math.round(estimatedModelBillions)
                          : Number(estimatedModelBillions.toFixed(1)),
                      epochs: ftEpochs,
                      dataset: ftFile
                        ? ftDatasetSamples > 0
                          ? `${ftDatasetSamples.toLocaleString()} ${t('samples')} (${formatBytesHuman(ftFile.size)})`
                          : formatBytesHuman(ftFile.size)
                        : t('no dataset selected'),
                    }
                  )}
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
              disabled={uploadingModel}
              onClick={() => handleCloseFtDialog(false)}
            >
              {t('Cancel')}
            </Button>
            <Button
              disabled={
                uploadingModel ||
                createFtMutation.isPending ||
                !hasValidModelInput ||
                !ftFile
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
        <DialogContent className='flex max-h-[85vh] flex-col sm:max-w-3xl'>
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
            className='h-96 overflow-y-auto rounded-lg border bg-zinc-950 p-3.5 font-mono text-xs leading-relaxed whitespace-pre-wrap break-words text-zinc-100'
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

      {/* Hugging Face Picker for Fine-Tuning (excludes inference-only GGUF) */}
      <HfModelPickerDialog
        open={hfPickerOpen}
        onOpenChange={setHfPickerOpen}
        onSelect={handleHfPicked}
        apiBasePath='/api/user/fine_tuning/hf'
        excludeGguf
      />

      {/* Auto Model Alias Create / Edit Dialog (with optional System Prompt) */}
      <Dialog open={aliasDialogOpen} onOpenChange={setAliasDialogOpen}>
        <DialogContent className='sm:max-w-lg'>
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
            <div className='space-y-1.5'>
              <Label>{t('System Prompt (Optional)')}</Label>
              <Textarea
                rows={4}
                placeholder={t(
                  'Automatically injected as the system prompt on every request using this auto model (standard input token pricing applies).'
                )}
                value={aliasSystemPrompt}
                onChange={(e) => setAliasSystemPrompt(e.target.value)}
              />
              <p className='text-muted-foreground text-xs'>
                {t(
                  'Injected prompt tokens are included in standard prompt token billing on each request.'
                )}
              </p>
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
