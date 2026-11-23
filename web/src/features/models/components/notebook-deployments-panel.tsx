import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Check,
  Copy,
  ExternalLink,
  Gauge,
  Loader2,
  Pencil,
  Play,
  Power,
  PowerOff,
  Terminal,
  Trash2,
  X,
} from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { BadgeListCell, DISABLED_ROW_DESKTOP } from '@/components/data-table'
import {
  SideDrawerSection,
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import { GroupBadge } from '@/components/group-badge'
import {
  HfModelPickerDialog,
  type HfPickedModel,
} from '@/components/hf-model-picker-dialog'
import { MultiSelect, type Option } from '@/components/multi-select'
import { StatusBadge, type StatusBadgeProps } from '@/components/status-badge'
import { TruncatedText } from '@/components/truncated-text'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
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
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Switch } from '@/components/ui/switch'
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
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { getGroups } from '@/features/channels/api'
import { NumericSpinnerInput } from '@/features/channels/components/numeric-spinner-input'
import { api } from '@/lib/api'
import { formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'

import {
  createNotebookDeployment,
  createNotebookDeploymentsBatch,
  deleteNotebookDeployment,
  getNotebookDeploymentLogs,
  getNotebookDeploymentTestStatus,
  listNotebookDeployments,
  startNotebookDeploymentTest,
  updateNotebookDeployment,
  type CreateNotebookDeploymentPayload,
  type DeploymentAccountSummary,
  type ModelFileSpec,
  type ModelTestResult,
  type NotebookDeploymentItem,
} from '../api'
import {
  getHardwareForTier,
  PROVIDER_LABELS,
  type DeploymentProviderId,
} from '../lib/hardware-catalog'

const TASK_TYPES = [
  { id: 'text', label: 'Text Generation (LLM)' },
  { id: 'image', label: 'Image Generation' },
  { id: 'video', label: 'Video Generation' },
  { id: 'audio', label: 'Audio / TTS' },
  { id: 'embeddings', label: 'Embeddings' },
] as const

const FILE_ROLES = [
  { id: 'main', label: 'Main / Checkpoint' },
  { id: 'text_encoder', label: 'Text Encoder' },
  { id: 'text_encoder_2', label: 'Text Encoder 2' },
  { id: 'vae', label: 'VAE' },
  { id: 'unet', label: 'UNet / Transformer' },
  { id: 'lora', label: 'LoRA' },
  { id: 'extra', label: 'Extra / Shard' },
] as const

const ACTIVE_STATUSES = new Set([
  'provisioning',
  'downloading',
  'starting',
  'testing',
])

function parseFallbacks(raw?: string): string[] {
  if (!raw) return []
  try {
    const parsed = JSON.parse(raw)
    return Array.isArray(parsed)
      ? parsed.filter((x) => typeof x === 'string')
      : []
  } catch {
    return []
  }
}

function parseModelFiles(raw?: string): ModelFileSpec[] {
  if (!raw) return []
  try {
    const parsed = JSON.parse(raw)
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return []
  }
}

function parseGroups(raw?: string): string[] {
  const s = (raw ?? 'default').trim()
  if (!s) return ['default']
  return s
    .split(',')
    .map((x) => x.trim())
    .filter(Boolean)
}

function sanitizeDeploymentName(repoId: string): string {
  const base = repoId.split('/').pop() ?? 'model'
  return base
    .toLowerCase()
    .replace(/[^a-z0-9-]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 40)
}

function DeploymentStatusCell({ item }: { item: NotebookDeploymentItem }) {
  const { t } = useTranslation()
  const status = (item.status || 'unknown').toLowerCase()
  const isDisabled = item.enabled === 2
  const isFailed = status === 'failed' || status === 'error'
  const isRunning = status === 'running'

  let variant: StatusBadgeProps['variant'] = 'neutral'
  let labelKey = 'Pending'
  if (isDisabled) {
    variant = 'neutral'
    labelKey = 'Disabled'
  } else if (isRunning) {
    variant = 'success'
    labelKey = 'Running'
  } else if (isFailed) {
    variant = 'danger'
    labelKey = 'Failed'
  } else if (ACTIVE_STATUSES.has(status)) {
    variant = 'warning'
    if (status === 'downloading') labelKey = 'Downloading'
    else if (status === 'starting') labelKey = 'Starting'
    else if (status === 'testing') labelKey = 'Testing'
    else labelKey = 'Provisioning'
  }

  return (
    <div className='flex flex-col gap-1'>
      <StatusBadge
        label={t(labelKey)}
        variant={variant}
        size='sm'
        copyable={false}
      />
      {item.status_message ? (
        <span
          className='text-[11px] text-muted-foreground max-w-[200px] truncate'
          title={item.status_message}
        >
          {item.status_message}
        </span>
      ) : null}
    </div>
  )
}

interface BulkQueueItem {
  key: string
  name: string
  modelRepo: string
  modelFile?: string
  taskType: string
}

export function UnifiedDeploymentsPanel({
  enabledProviders,
  providerAccounts = {},
  createOpen,
  onCreateOpenChange,
}: {
  enabledProviders: DeploymentProviderId[]
  providerAccounts?: Record<string, DeploymentAccountSummary[]>
  createOpen: boolean
  onCreateOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  const defaultProvider: DeploymentProviderId = enabledProviders[0] ?? 'modal'

  const [createMode, setCreateMode] = useState<'single' | 'bulk'>('single')
  const [provider, setProvider] =
    useState<DeploymentProviderId>(defaultProvider)
  const [accountId, setAccountId] = useState<string>('')
  const [hfPickerOpen, setHfPickerOpen] = useState(false)
  const [bulkPickerOpen, setBulkPickerOpen] = useState(false)

  // Single create state
  const [name, setName] = useState('')
  const [modelRepo, setModelRepo] = useState('')
  const [modelFile, setModelFile] = useState('')
  const [modelFiles, setModelFiles] = useState<ModelFileSpec[]>([])
  const [repoFileSuggestions, setRepoFileSuggestions] = useState<string[]>([])
  const [loadingRepoFiles, setLoadingRepoFiles] = useState(false)
  const [taskType, setTaskType] = useState<string>('text')
  const [hfToken, setHfToken] = useState('')

  // Routing & compute fields (shared for single & bulk)
  const [highRam, setHighRam] = useState(false)
  const [cpuCores, setCpuCores] = useState<number>(0)
  const [memoryGb, setMemoryGb] = useState<number>(0)
  const [selectedGroups, setSelectedGroups] = useState<string[]>(['default'])
  const [priority, setPriority] = useState<number>(0)
  const [weight, setWeight] = useState<number>(1)

  // Bulk create state
  const [bulkQueue, setBulkQueue] = useState<BulkQueueItem[]>([])
  const [bulkPasteText, setBulkPasteText] = useState('')

  const accountsForProvider = useMemo(
    () => providerAccounts[provider] ?? [],
    [providerAccounts, provider]
  )

  const selectedAccount = useMemo(() => {
    if (accountsForProvider.length === 0) return undefined
    return (
      accountsForProvider.find((a) => a.id === accountId) ??
      accountsForProvider.find((a) => a.enabled && a.configured) ??
      accountsForProvider[0]
    )
  }, [accountsForProvider, accountId])

  const hardwareOptions = useMemo(
    () =>
      getHardwareForTier(
        provider,
        selectedAccount?.tier,
        selectedAccount?.eligible_gpus
      ),
    [provider, selectedAccount?.tier, selectedAccount?.eligible_gpus]
  )

  const [accelerator, setAccelerator] = useState<string>(
    hardwareOptions[0]?.id ?? 'CPU'
  )
  const [fallbackAccelerators, setFallbackAccelerators] = useState<string[]>(
    []
  )

  useEffect(() => {
    if (
      enabledProviders.length > 0 &&
      !enabledProviders.includes(provider)
    ) {
      setProvider(enabledProviders[0])
    }
  }, [enabledProviders, provider])

  useEffect(() => {
    if (accountsForProvider.length > 0 && !accountId) {
      const def =
        accountsForProvider.find((a) => a.enabled && a.configured) ??
        accountsForProvider[0]
      if (def) setAccountId(def.id)
    }
  }, [accountsForProvider, accountId])

  useEffect(() => {
    const validIds = new Set(hardwareOptions.map((h) => h.id))
    if (hardwareOptions.length > 0 && !validIds.has(accelerator)) {
      setAccelerator(hardwareOptions[0].id)
    }
    setFallbackAccelerators((prev) => prev.filter((id) => validIds.has(id)))
  }, [hardwareOptions, accelerator])

  const handleProviderChange = (next: DeploymentProviderId) => {
    setProvider(next)
    const nextAccs = providerAccounts[next] ?? []
    const def =
      nextAccs.find((a) => a.enabled && a.configured) ?? nextAccs[0]
    setAccountId(def?.id ?? '')
    const nextHw = getHardwareForTier(next, def?.tier, def?.eligible_gpus)
    setAccelerator(nextHw[0]?.id ?? 'CPU')
    setFallbackAccelerators([])
  }

  const toggleFallback = (id: string) => {
    if (id === accelerator) return
    setFallbackAccelerators((prev) =>
      prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]
    )
  }

  const groupsQuery = useQuery({
    queryKey: ['channels', 'groups'],
    queryFn: getGroups,
    staleTime: 60 * 1000,
    refetchOnWindowFocus: false,
  })
  const groupOptions: Option[] = useMemo(() => {
    const raw = (groupsQuery.data as any)?.data
    const list: string[] = Array.isArray(raw) && raw.length > 0 ? raw : ['default']
    return list.map((g) => ({ label: g, value: g }))
  }, [groupsQuery.data])

  // Details & Live Console Drawer
  const [detailsItem, setDetailsItem] =
    useState<NotebookDeploymentItem | null>(null)
  const [detailsTab, setDetailsTab] = useState<'overview' | 'logs'>('overview')
  const [editGroups, setEditGroups] = useState<string[]>(['default'])
  const [editPriority, setEditPriority] = useState(0)
  const [editWeight, setEditWeight] = useState(1)
  const [editEnabled, setEditEnabled] = useState(true)
  const [savingDetails, setSavingDetails] = useState(false)
  const logsEndRef = useRef<HTMLPreElement | null>(null)

  useEffect(() => {
    if (detailsItem) {
      setEditGroups(parseGroups(detailsItem.group))
      setEditPriority(detailsItem.priority ?? 0)
      setEditWeight(detailsItem.weight ?? 1)
      setEditEnabled(detailsItem.enabled !== 2)
    }
  }, [detailsItem])

  const logsQuery = useQuery({
    queryKey: ['notebook-deployment-logs', detailsItem?.id],
    queryFn: () => getNotebookDeploymentLogs(detailsItem!.id),
    enabled: Boolean(detailsItem) && detailsTab === 'logs',
    refetchOnWindowFocus: false,
    refetchInterval:
      Boolean(detailsItem) && detailsTab === 'logs' ? 1000 : false,
  })

  useEffect(() => {
    if (detailsTab === 'logs' && logsEndRef.current) {
      logsEndRef.current.scrollTop = logsEndRef.current.scrollHeight
    }
  }, [logsQuery.data?.data?.logs, detailsTab])

  const [testTarget, setTestTarget] =
    useState<NotebookDeploymentItem | null>(null)
  const [testPrompt, setTestPrompt] = useState(
    'Hello! Describe yourself briefly.'
  )
  const [testResult, setTestResult] = useState<ModelTestResult | null>(null)
  const [testError, setTestError] = useState<string | null>(null)
  const [testing, setTesting] = useState(false)
  const [testProgress, setTestProgress] = useState(0)
  const [testStage, setTestStage] = useState('')
  const [testElapsedSec, setTestElapsedSec] = useState(0)
  const testPollCancelledRef = useRef(false)

  useEffect(() => {
    if (!testTarget) {
      testPollCancelledRef.current = true
    }
    return () => {
      testPollCancelledRef.current = true
    }
  }, [testTarget])

  const [deleteTarget, setDeleteTarget] =
    useState<NotebookDeploymentItem | null>(null)
  const [deleting, setDeleting] = useState(false)

  const listQuery = useQuery({
    queryKey: ['notebook-deployments', 'all'],
    queryFn: () => listNotebookDeployments(),
    staleTime: 5 * 1000,
    refetchOnWindowFocus: false,
    refetchInterval: (query) => {
      const rows = query.state.data?.data ?? []
      return rows.some(
        (r) =>
          r.enabled !== 2 &&
          ACTIVE_STATUSES.has((r.status || '').toLowerCase())
      )
        ? 3000
        : false
    },
  })

  const items = listQuery.data?.data ?? []

  const selectedTaskLabel =
    TASK_TYPES.find((tk) => tk.id === taskType)?.label ?? 'Text Generation (LLM)'
  const selectedHwLabel =
    hardwareOptions.find((hw) => hw.id === accelerator)?.label ?? accelerator
  const formatAccountLabel = (acc: DeploymentAccountSummary) => {
    const idPart = acc.label || `#${acc.index}`
    const extra = acc.email || acc.username
    return extra ? `${idPart} (${extra})` : idPart
  }

  const createMutation = useMutation({
    mutationFn: () =>
      createNotebookDeployment({
        provider,
        account_id: selectedAccount?.id,
        name: name.trim(),
        model_repo: modelRepo.trim(),
        model_file: modelFile.trim() || undefined,
        model_files: modelFiles.filter((mf) => mf.file.trim() !== ''),
        task_type: taskType,
        accelerator,
        fallback_accelerators: fallbackAccelerators,
        high_ram: provider === 'colab' ? highRam : false,
        cpu_cores: provider === 'modal' ? cpuCores : 0,
        memory_gb: provider === 'modal' ? memoryGb : 0,
        group: selectedGroups.join(',') || 'default',
        priority,
        weight,
        enabled: 1,
        hf_token: hfToken.trim() || undefined,
      }),
    onSuccess: (res) => {
      if (res?.success) {
        toast.success(t('Deployment started'))
        onCreateOpenChange(false)
        setName('')
        setModelRepo('')
        setModelFile('')
        setModelFiles([])
        setFallbackAccelerators([])
        setHfToken('')
        queryClient.invalidateQueries({
          queryKey: ['notebook-deployments', 'all'],
        })
        return
      }
      toast.error(res?.message || t('Failed to create deployment'))
    },
    onError: (err: Error) => {
      toast.error(err.message || t('Failed to create deployment'))
    },
  })

  const batchCreateMutation = useMutation({
    mutationFn: () => {
      const payloads: CreateNotebookDeploymentPayload[] = bulkQueue.map(
        (entry) => ({
          provider,
          account_id: selectedAccount?.id,
          name: entry.name.trim() || sanitizeDeploymentName(entry.modelRepo),
          model_repo: entry.modelRepo.trim(),
          model_file: entry.modelFile?.trim() || undefined,
          task_type: entry.taskType || 'text',
          accelerator,
          fallback_accelerators: fallbackAccelerators,
          high_ram: provider === 'colab' ? highRam : false,
          cpu_cores: provider === 'modal' ? cpuCores : 0,
          memory_gb: provider === 'modal' ? memoryGb : 0,
          group: selectedGroups.join(',') || 'default',
          priority,
          weight,
          enabled: 1,
          hf_token: hfToken.trim() || undefined,
        })
      )
      return createNotebookDeploymentsBatch(payloads)
    },
    onSuccess: (res) => {
      if (res?.success) {
        const count = res.data?.created?.length ?? 0
        const errs = res.data?.errors ?? []
        toast.success(t('Started {{count}} deployment(s)', { count }))
        if (errs.length > 0) {
          toast.warning(errs.join('; '))
        }
        onCreateOpenChange(false)
        setBulkQueue([])
        setBulkPasteText('')
        queryClient.invalidateQueries({
          queryKey: ['notebook-deployments', 'all'],
        })
        return
      }
      toast.error(res?.message || t('Failed to create deployments'))
    },
    onError: (err: Error) => {
      toast.error(err.message || t('Failed to create deployments'))
    },
  })

  const handleHfPicked = (picked: HfPickedModel) => {
    setModelRepo(picked.repoId)
    setModelFile(picked.filename ?? '')
    setTaskType(picked.taskType)
    if (picked.filename) {
      setModelFiles([{ repo: picked.repoId, file: picked.filename, role: 'main' }])
    }
    if (!name.trim()) {
      setName(sanitizeDeploymentName(picked.repoId))
    }
  }

  const handleBulkPicked = (pickedList: HfPickedModel[]) => {
    setBulkQueue((prev) => {
      const seen = new Set(
        prev.map((x) => `${x.modelRepo}::${x.modelFile ?? ''}`)
      )
      const next = [...prev]
      for (const p of pickedList) {
        const key = `${p.repoId}::${p.filename ?? ''}`
        if (seen.has(key)) continue
        seen.add(key)
        next.push({
          key: `${key}-${Date.now().toString(36)}`,
          name: sanitizeDeploymentName(p.repoId),
          modelRepo: p.repoId,
          modelFile: p.filename,
          taskType: p.taskType,
        })
      }
      return next
    })
  }

  const handleAddPastedBulk = () => {
    const lines = bulkPasteText
      .split('\n')
      .map((l) => l.trim())
      .filter(Boolean)
    if (lines.length === 0) return
    setBulkQueue((prev) => {
      const next = [...prev]
      for (const line of lines) {
        const [repoPart, taskPart] = line.split('|').map((s) => s.trim())
        if (!repoPart) continue
        next.push({
          key: `${repoPart}-${Math.random().toString(36).slice(2, 7)}`,
          name: sanitizeDeploymentName(repoPart),
          modelRepo: repoPart,
          taskType: taskPart || taskType || 'text',
        })
      }
      return next
    })
    setBulkPasteText('')
  }

  const handleFetchRepoFiles = async () => {
    const repo = modelRepo.trim()
    if (!repo) {
      toast.error(t('Enter a Hugging Face repository first'))
      return
    }
    setLoadingRepoFiles(true)
    try {
      const resp = await api.get('/api/deployments/hf/files', {
        params: { repo, token: hfToken.trim() || undefined },
      })
      const files: Array<{ filename: string }> = resp.data?.data?.files ?? []
      setRepoFileSuggestions(files.map((f) => f.filename))
      if (files.length === 0) {
        toast.info(t('No weight files found in repository'))
      }
    } catch (e: any) {
      toast.error(
        e?.response?.data?.message || t('Failed to list repository files')
      )
    } finally {
      setLoadingRepoFiles(false)
    }
  }

  const handleInlineFieldUpdate = async (
    item: NotebookDeploymentItem,
    patch: { priority?: number; weight?: number; enabled?: number }
  ) => {
    try {
      const res = await updateNotebookDeployment(item.id, patch)
      if (res?.success) {
        queryClient.invalidateQueries({
          queryKey: ['notebook-deployments', 'all'],
        })
      } else {
        toast.error(res?.message || t('Update failed'))
      }
    } catch (e: any) {
      toast.error(e?.message || t('Update failed'))
    }
  }

  const handleSaveDetails = async () => {
    if (!detailsItem) return
    setSavingDetails(true)
    try {
      const res = await updateNotebookDeployment(detailsItem.id, {
        group: editGroups.join(',') || 'default',
        priority: editPriority,
        weight: editWeight,
        enabled: editEnabled ? 1 : 2,
      })
      if (res?.success && res.data) {
        setDetailsItem(res.data)
        toast.success(t('Updated successfully'))
        queryClient.invalidateQueries({
          queryKey: ['notebook-deployments', 'all'],
        })
      } else {
        toast.error(res?.message || t('Update failed'))
      }
    } catch (e: any) {
      toast.error(e?.message || t('Update failed'))
    } finally {
      setSavingDetails(false)
    }
  }

  const handleRunTest = async () => {
    if (!testTarget) return
    testPollCancelledRef.current = false
    setTesting(true)
    setTestError(null)
    setTestResult(null)
    setTestProgress(0)
    setTestStage(t('Starting test...'))
    setTestElapsedSec(0)
    const startedAt = Date.now()
    const elapsedTimer = window.setInterval(() => {
      setTestElapsedSec(Math.floor((Date.now() - startedAt) / 1000))
    }, 1000)

    try {
      const startRes = await startNotebookDeploymentTest(testTarget.id, {
        prompt: testPrompt.trim(),
      })
      if (!startRes?.success || !startRes.data?.job_id) {
        setTestError(startRes?.message || t('Model test failed'))
        return
      }
      const jobId = startRes.data.job_id
      setTestProgress(startRes.data.progress ?? 0)
      setTestStage(startRes.data.stage || t('Starting test...'))

      let consecutivePollErrors = 0
      while (!testPollCancelledRef.current) {
        await new Promise((r) => setTimeout(r, 1200))
        if (testPollCancelledRef.current) break
        let pollRes
        try {
          pollRes = await getNotebookDeploymentTestStatus(
            testTarget.id,
            jobId
          )
          consecutivePollErrors = 0
        } catch (pollErr) {
          consecutivePollErrors += 1
          if (consecutivePollErrors >= 5) {
            throw pollErr
          }
          continue
        }
        if (!pollRes?.success || !pollRes.data) {
          setTestError(pollRes?.message || t('Model test failed'))
          break
        }
        const st = pollRes.data
        setTestProgress(st.progress ?? 0)
        if (st.stage) setTestStage(st.stage)
        if (st.status === 'completed') {
          if (st.result) setTestResult(st.result)
          setTestProgress(100)
          break
        }
        if (st.status === 'failed') {
          setTestError(st.error || t('Model test failed'))
          break
        }
      }
    } catch (err) {
      setTestError(err instanceof Error ? err.message : t('Model test failed'))
    } finally {
      window.clearInterval(elapsedTimer)
      setTesting(false)
      queryClient.invalidateQueries({
        queryKey: ['notebook-deployments', 'all'],
      })
    }
  }

  const handleDelete = async () => {
    if (!deleteTarget) return
    setDeleting(true)
    try {
      const res = await deleteNotebookDeployment(deleteTarget.id)
      if (res?.success) {
        toast.success(t('Deleted successfully'))
        if (res.data?.remote_stop_warning) {
          toast.warning(res.data.remote_stop_warning)
        }
        queryClient.invalidateQueries({
          queryKey: ['notebook-deployments', 'all'],
        })
      } else {
        toast.error(res?.message || t('Delete failed'))
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t('Delete failed'))
    } finally {
      setDeleting(false)
      setDeleteTarget(null)
    }
  }

  return (
    <TooltipProvider delay={200}>
      <div className='flex h-full min-h-0 flex-col gap-4'>
        <div className='rounded-lg border overflow-auto'>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Name')}</TableHead>
                <TableHead>{t('Provider')}</TableHead>
                <TableHead>{t('Status')}</TableHead>
                <TableHead>{t('Groups')}</TableHead>
                <TableHead>{t('Priority')}</TableHead>
                <TableHead>{t('Weight')}</TableHead>
                <TableHead>{t('Hardware')}</TableHead>
                <TableHead>{t('Created')}</TableHead>
                <TableHead>{t('Actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {listQuery.isLoading ? (
                <TableRow>
                  <TableCell colSpan={9} className='text-center py-8'>
                    <Loader2 className='h-5 w-5 animate-spin mx-auto text-muted-foreground' />
                  </TableCell>
                </TableRow>
              ) : items.length === 0 ? (
                <TableRow>
                  <TableCell
                    colSpan={9}
                    className='text-center py-10 text-muted-foreground text-sm'
                  >
                    {t('No deployments available. Create one to get started.')}
                  </TableCell>
                </TableRow>
              ) : (
                items.map((item) => {
                  const extraFiles = parseModelFiles(item.model_files)
                  const groups = parseGroups(item.group)
                  const isDisabled = item.enabled === 2
                  const providerLabel =
                    PROVIDER_LABELS[item.provider as DeploymentProviderId] ??
                    item.provider
                  const showAccLabel =
                    Boolean(item.account_label) &&
                    ((providerAccounts[item.provider]?.length ?? 0) > 1 ||
                      item.account_label !== '#1')

                  return (
                    <TableRow
                      key={item.id}
                      className={isDisabled ? DISABLED_ROW_DESKTOP : undefined}
                    >
                      <TableCell>
                        <div className='flex flex-col gap-0.5 max-w-[240px] min-w-0'>
                          <TruncatedText
                            text={item.name}
                            className={cn(
                              isDisabled
                                ? 'text-muted-foreground font-normal'
                                : 'font-medium'
                            )}
                            maxWidth='max-w-full'
                          />
                          <div className='flex items-center gap-1.5 text-xs text-muted-foreground font-mono truncate'>
                            <span className='truncate'>{item.model_repo}</span>
                            {extraFiles.length > 0 && (
                              <span className='shrink-0'>
                                (+{extraFiles.length})
                              </span>
                            )}
                          </div>
                        </div>
                      </TableCell>
                      <TableCell>
                        <div className='flex items-center gap-1.5'>
                          <StatusBadge
                            label={providerLabel}
                            variant='blue'
                            size='sm'
                            copyable={false}
                          />
                          {showAccLabel ? (
                            <StatusBadge
                              label={item.account_label!}
                              variant='neutral'
                              size='sm'
                              copyable={false}
                            />
                          ) : null}
                        </div>
                      </TableCell>
                      <TableCell>
                        <DeploymentStatusCell item={item} />
                      </TableCell>
                      <TableCell>
                        <BadgeListCell
                          items={groups.map((g) => (
                            <GroupBadge key={g} group={g} size='sm' />
                          ))}
                        />
                      </TableCell>
                      <TableCell>
                        <NumericSpinnerInput
                          value={item.priority ?? 0}
                          min={-999}
                          onChange={(val) =>
                            void handleInlineFieldUpdate(item, { priority: val })
                          }
                        />
                      </TableCell>
                      <TableCell>
                        <NumericSpinnerInput
                          value={item.weight ?? 1}
                          min={1}
                          onChange={(val) =>
                            void handleInlineFieldUpdate(item, { weight: val })
                          }
                        />
                      </TableCell>
                      <TableCell>
                        <StatusBadge
                          label={item.accelerator}
                          variant='neutral'
                          size='sm'
                          copyable={false}
                          className='font-mono'
                        />
                      </TableCell>
                      <TableCell className='font-mono text-xs text-muted-foreground whitespace-nowrap'>
                        {formatTimestampToDate(item.created_time)}
                      </TableCell>
                      <TableCell>
                        <div className='-ml-1.5 flex items-center gap-1'>
                          <Tooltip>
                            <TooltipTrigger
                              render={
                                <Button
                                  variant='ghost'
                                  size='icon-sm'
                                  onClick={() => {
                                    setDetailsTab('overview')
                                    setDetailsItem(item)
                                  }}
                                  aria-label={t('Edit')}
                                />
                              }
                            >
                              <Pencil className='size-4' />
                            </TooltipTrigger>
                            <TooltipContent>{t('Edit')}</TooltipContent>
                          </Tooltip>

                          <Tooltip>
                            <TooltipTrigger
                              render={
                                <Button
                                  variant='ghost'
                                  size='icon-sm'
                                  onClick={() => {
                                    setTestTarget(item)
                                    setTestResult(null)
                                    setTestError(null)
                                  }}
                                  aria-label={t('Test')}
                                />
                              }
                            >
                              <Gauge className='size-4' />
                            </TooltipTrigger>
                            <TooltipContent>{t('Test')}</TooltipContent>
                          </Tooltip>

                          <Tooltip>
                            <TooltipTrigger
                              render={
                                <Button
                                  variant='ghost'
                                  size='icon-sm'
                                  onClick={() => {
                                    setDetailsTab('logs')
                                    setDetailsItem(item)
                                  }}
                                  aria-label={t('Console')}
                                />
                              }
                            >
                              <Terminal className='size-4' />
                            </TooltipTrigger>
                            <TooltipContent>{t('Console')}</TooltipContent>
                          </Tooltip>

                          <Tooltip>
                            <TooltipTrigger
                              render={
                                <Button
                                  variant='ghost'
                                  size='icon-sm'
                                  onClick={() =>
                                    void handleInlineFieldUpdate(item, {
                                      enabled: isDisabled ? 1 : 2,
                                    })
                                  }
                                  aria-label={
                                    isDisabled ? t('Enable') : t('Disable')
                                  }
                                />
                              }
                            >
                              {isDisabled ? (
                                <Power className='size-4' />
                              ) : (
                                <PowerOff className='size-4' />
                              )}
                            </TooltipTrigger>
                            <TooltipContent>
                              {isDisabled ? t('Enable') : t('Disable')}
                            </TooltipContent>
                          </Tooltip>

                          <Tooltip>
                            <TooltipTrigger
                              render={
                                <Button
                                  variant='ghost'
                                  size='icon-sm'
                                  className='text-destructive hover:text-destructive'
                                  onClick={() => setDeleteTarget(item)}
                                  aria-label={t('Delete')}
                                />
                              }
                            >
                              <Trash2 className='size-4' />
                            </TooltipTrigger>
                            <TooltipContent>{t('Delete')}</TooltipContent>
                          </Tooltip>
                        </div>
                      </TableCell>
                    </TableRow>
                  )
                })
              )}
            </TableBody>
          </Table>
        </div>

        {/* Right-Side Create Deployment Drawer */}
        <Sheet open={createOpen} onOpenChange={onCreateOpenChange}>
          <SheetContent className={sideDrawerContentClassName('sm:max-w-xl')}>
            <SheetHeader className={sideDrawerHeaderClassName()}>
              <SheetTitle>{t('Create deployment')}</SheetTitle>
              <SheetDescription>
                {t(
                  'Configure provider, model, hardware, and routing settings.'
                )}
              </SheetDescription>
            </SheetHeader>

            <form
              id='unified-create-deployment-form'
              className={sideDrawerFormClassName()}
              onSubmit={(e) => {
                e.preventDefault()
                if (createMode === 'bulk') {
                  if (bulkQueue.length === 0) return
                  batchCreateMutation.mutate()
                } else {
                  if (!name.trim() || !modelRepo.trim()) return
                  createMutation.mutate()
                }
              }}
            >
              <Tabs
                value={createMode}
                onValueChange={(v) =>
                  setCreateMode((v as 'single' | 'bulk') ?? 'single')
                }
              >
                <TabsList className='w-full'>
                  <TabsTrigger value='single' className='flex-1'>
                    {t('Single Deployment')}
                  </TabsTrigger>
                  <TabsTrigger value='bulk' className='flex-1'>
                    {t('Bulk Create')}
                  </TabsTrigger>
                </TabsList>
              </Tabs>

              <SideDrawerSection>
                <div
                  className={cn(
                    'grid gap-4',
                    accountsForProvider.length > 1
                      ? 'sm:grid-cols-2'
                      : 'grid-cols-1'
                  )}
                >
                  <div className='space-y-1.5'>
                    <label className='text-xs font-medium'>
                      {t('Provider')}
                    </label>
                    <Select
                      value={provider}
                      onValueChange={(v) =>
                        handleProviderChange(
                          (v ?? defaultProvider) as DeploymentProviderId
                        )
                      }
                    >
                      <SelectTrigger className='w-full'>
                        <SelectValue>{PROVIDER_LABELS[provider]}</SelectValue>
                      </SelectTrigger>
                      <SelectContent alignItemWithTrigger={false} align='start'>
                        {enabledProviders.map((p) => (
                          <SelectItem key={p} value={p}>
                            {PROVIDER_LABELS[p]}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>

                  {accountsForProvider.length > 1 && (
                    <div className='space-y-1.5'>
                      <label className='text-xs font-medium'>
                        {t('Account')}
                      </label>
                      <Select
                        value={selectedAccount?.id ?? ''}
                        onValueChange={(v) => setAccountId(v ?? '')}
                      >
                        <SelectTrigger className='w-full'>
                          <SelectValue>
                            {selectedAccount
                              ? formatAccountLabel(selectedAccount)
                              : t('Select account')}
                          </SelectValue>
                        </SelectTrigger>
                        <SelectContent alignItemWithTrigger={false} align='start'>
                          {accountsForProvider.map((acc) => (
                            <SelectItem key={acc.id} value={acc.id}>
                              {formatAccountLabel(acc)}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                  )}
                </div>
              </SideDrawerSection>

              {createMode === 'single' ? (
                <SideDrawerSection>
                  <div className='grid gap-4 sm:grid-cols-2'>
                    <div className='space-y-1.5'>
                      <label className='text-xs font-medium'>
                        {t('Deployment Name')}
                      </label>
                      <Input
                        placeholder='my-qwen-server'
                        value={name}
                        onChange={(e) => setName(e.target.value)}
                      />
                    </div>

                    <div className='space-y-1.5'>
                      <label className='text-xs font-medium'>
                        {t('Model Type / Task')}
                      </label>
                      <Select
                        value={taskType}
                        onValueChange={(v) => setTaskType(v ?? 'text')}
                      >
                        <SelectTrigger className='w-full'>
                          <SelectValue>{t(selectedTaskLabel)}</SelectValue>
                        </SelectTrigger>
                        <SelectContent alignItemWithTrigger={false} align='start'>
                          {TASK_TYPES.map((tk) => (
                            <SelectItem key={tk.id} value={tk.id}>
                              {t(tk.label)}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                  </div>

                  <div className='space-y-1.5'>
                    <label className='text-xs font-medium'>
                      {t('Hugging Face Model')}
                    </label>
                    <div className='flex items-center gap-2'>
                      <Input
                        placeholder='Qwen/Qwen2.5-7B-Instruct'
                        value={modelRepo}
                        onChange={(e) => setModelRepo(e.target.value)}
                        className='flex-1 min-w-0'
                      />
                      <Button
                        type='button'
                        variant='outline'
                        size='sm'
                        className='shrink-0'
                        onClick={() => setHfPickerOpen(true)}
                      >
                        {t('Select from Hugging Face')}
                      </Button>
                    </div>
                    {modelFile ? (
                      <p className='text-xs text-muted-foreground font-mono'>
                        {t('Selected file')}: {modelFile}
                      </p>
                    ) : null}
                  </div>

                  <div className='space-y-2 rounded-lg border p-3 bg-muted/15'>
                    <div className='flex flex-wrap items-center justify-between gap-2'>
                      <span className='text-xs font-medium'>
                        {t('Multi-File Components (Optional)')}
                      </span>
                      <div className='flex items-center gap-2'>
                        <Button
                          type='button'
                          variant='outline'
                          size='sm'
                          className='h-7 text-xs'
                          disabled={loadingRepoFiles || !modelRepo.trim()}
                          onClick={handleFetchRepoFiles}
                        >
                          {loadingRepoFiles && (
                            <Loader2 className='h-3 w-3 mr-1 animate-spin' />
                          )}
                          {t('List Repo Files')}
                        </Button>
                        <Button
                          type='button'
                          variant='outline'
                          size='sm'
                          className='h-7 text-xs'
                          onClick={() =>
                            setModelFiles((prev) => [
                              ...prev,
                              {
                                repo: modelRepo.trim(),
                                file: '',
                                role: 'extra',
                              },
                            ])
                          }
                        >
                          {t('Add File')}
                        </Button>
                      </div>
                    </div>

                    {repoFileSuggestions.length > 0 && (
                      <div className='flex flex-wrap gap-1 pt-1'>
                        {repoFileSuggestions.slice(0, 12).map((fn) => (
                          <Button
                            key={fn}
                            type='button'
                            variant='secondary'
                            size='sm'
                            className='h-6 text-[11px] font-mono px-2'
                            onClick={() =>
                              setModelFiles((prev) => [
                                ...prev,
                                {
                                  repo: modelRepo.trim(),
                                  file: fn,
                                  role: fn.toLowerCase().includes('vae')
                                    ? 'vae'
                                    : fn.toLowerCase().includes('lora')
                                      ? 'lora'
                                      : fn.toLowerCase().includes('encoder') ||
                                          fn.toLowerCase().includes('t5') ||
                                          fn.toLowerCase().includes('clip')
                                        ? 'text_encoder'
                                        : 'main',
                                },
                              ])
                            }
                          >
                            + {fn}
                          </Button>
                        ))}
                      </div>
                    )}

                    {modelFiles.map((mf, idx) => {
                      const roleLabel =
                        FILE_ROLES.find((r) => r.id === (mf.role || 'extra'))
                          ?.label ?? 'Extra / Shard'
                      return (
                        <div
                          key={idx}
                          className='grid grid-cols-[130px_1fr_1fr_auto] items-center gap-1.5'
                        >
                          <Select
                            value={mf.role || 'extra'}
                            onValueChange={(v) =>
                              setModelFiles((prev) =>
                                prev.map((x, i) =>
                                  i === idx
                                    ? {
                                        ...x,
                                        role: (v ??
                                          'extra') as ModelFileSpec['role'],
                                      }
                                    : x
                                )
                              )
                            }
                          >
                            <SelectTrigger className='h-8 w-full text-xs'>
                              <SelectValue>{t(roleLabel)}</SelectValue>
                            </SelectTrigger>
                            <SelectContent alignItemWithTrigger={false} align='start'>
                              {FILE_ROLES.map((r) => (
                                <SelectItem key={r.id} value={r.id}>
                                  {t(r.label)}
                                </SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                          <Input
                            placeholder={t('Repo (optional)')}
                            value={mf.repo ?? ''}
                            onChange={(e) =>
                              setModelFiles((prev) =>
                                prev.map((x, i) =>
                                  i === idx ? { ...x, repo: e.target.value } : x
                                )
                              )
                            }
                            className='h-8 text-xs font-mono'
                          />
                          <Input
                            placeholder='model.safetensors'
                            value={mf.file}
                            onChange={(e) =>
                              setModelFiles((prev) =>
                                prev.map((x, i) =>
                                  i === idx ? { ...x, file: e.target.value } : x
                                )
                              )
                            }
                            className='h-8 text-xs font-mono'
                          />
                          <Button
                            type='button'
                            variant='ghost'
                            size='icon-sm'
                            onClick={() =>
                              setModelFiles((prev) =>
                                prev.filter((_, i) => i !== idx)
                              )
                            }
                          >
                            <X className='h-3.5 w-3.5' />
                          </Button>
                        </div>
                      )
                    })}
                  </div>
                </SideDrawerSection>
              ) : (
                <SideDrawerSection>
                  <div className='flex items-center justify-between gap-2'>
                    <span className='text-sm font-medium'>
                      {t('Bulk Model Queue')} ({bulkQueue.length})
                    </span>
                    <Button
                      type='button'
                      variant='outline'
                      size='sm'
                      onClick={() => setBulkPickerOpen(true)}
                    >
                      {t('Select Multiple from Hugging Face')}
                    </Button>
                  </div>

                  <div className='space-y-1.5'>
                    <label className='text-xs font-medium'>
                      {t(
                        'Or paste Hugging Face repos (one per line, optional |task)'
                      )}
                    </label>
                    <Textarea
                      rows={3}
                      placeholder={
                        'Qwen/Qwen2.5-0.5B-Instruct-GGUF|text\nturingevo/tiny-sd-safetensors|image'
                      }
                      value={bulkPasteText}
                      onChange={(e) => setBulkPasteText(e.target.value)}
                      className='text-xs font-mono'
                    />
                    <div className='flex justify-end'>
                      <Button
                        type='button'
                        variant='secondary'
                        size='sm'
                        onClick={handleAddPastedBulk}
                      >
                        {t('Add')}
                      </Button>
                    </div>
                  </div>

                  {bulkQueue.length > 0 && (
                    <div className='space-y-1.5 max-h-48 overflow-y-auto rounded-lg border p-2'>
                      {bulkQueue.map((entry, idx) => (
                        <div
                          key={entry.key}
                          className='flex items-center gap-2 text-xs'
                        >
                          <Input
                            value={entry.name}
                            onChange={(e) =>
                              setBulkQueue((prev) =>
                                prev.map((x, i) =>
                                  i === idx ? { ...x, name: e.target.value } : x
                                )
                              )
                            }
                            className='h-7 w-32 font-mono text-xs'
                          />
                          <span className='font-mono truncate flex-1'>
                            {entry.modelRepo}
                            {entry.modelFile ? ` (${entry.modelFile})` : ''}
                          </span>
                          <StatusBadge
                            label={entry.taskType}
                            variant='neutral'
                            size='sm'
                            copyable={false}
                          />
                          <Button
                            type='button'
                            variant='ghost'
                            size='icon-sm'
                            onClick={() =>
                              setBulkQueue((prev) =>
                                prev.filter((_, i) => i !== idx)
                              )
                            }
                          >
                            <X className='h-3.5 w-3.5' />
                          </Button>
                        </div>
                      ))}
                    </div>
                  )}
                </SideDrawerSection>
              )}

              <SideDrawerSection>
                <div className='space-y-1.5'>
                  <label className='text-xs font-medium'>
                    {t('Default Hardware (CPU / GPU / TPU)')}
                  </label>
                  <Select
                    value={accelerator}
                    onValueChange={(v) => {
                      const next = v ?? hardwareOptions[0]?.id ?? 'CPU'
                      setAccelerator(next)
                      setFallbackAccelerators((prev) =>
                        prev.filter((x) => x !== next)
                      )
                    }}
                  >
                    <SelectTrigger className='w-full'>
                      <SelectValue>{selectedHwLabel}</SelectValue>
                    </SelectTrigger>
                    <SelectContent alignItemWithTrigger={false} align='start'>
                      {hardwareOptions.map((hw) => (
                        <SelectItem key={hw.id} value={hw.id}>
                          {hw.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>

                {hardwareOptions.length > 1 && (
                  <div className='space-y-1.5'>
                    <label className='text-xs font-medium'>
                      {t('Alternative / Fallback Hardware (if primary fails)')}
                    </label>
                    <div className='flex flex-wrap gap-1.5 rounded-lg border p-2.5 bg-muted/20'>
                      {hardwareOptions
                        .filter((hw) => hw.id !== accelerator)
                        .map((hw) => {
                          const selected = fallbackAccelerators.includes(hw.id)
                          return (
                            <Button
                              key={hw.id}
                              type='button'
                              size='sm'
                              variant={selected ? 'default' : 'outline'}
                              className='h-7 text-xs gap-1'
                              onClick={() => toggleFallback(hw.id)}
                            >
                              {selected && <Check className='h-3 w-3' />}
                              {hw.label}
                            </Button>
                          )
                        })}
                    </div>
                  </div>
                )}

                {provider === 'colab' && (
                  <div className='space-y-1.5'>
                    <label className='text-xs font-medium'>
                      {t('RAM Profile')}
                    </label>
                    <Select
                      value={highRam ? 'high' : 'standard'}
                      onValueChange={(v) => setHighRam(v === 'high')}
                    >
                      <SelectTrigger className='w-full'>
                        <SelectValue>
                          {highRam ? t('High-RAM') : t('Standard RAM')}
                        </SelectValue>
                      </SelectTrigger>
                      <SelectContent alignItemWithTrigger={false} align='start'>
                        <SelectItem value='standard'>
                          {t('Standard RAM')}
                        </SelectItem>
                        <SelectItem value='high'>{t('High-RAM')}</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                )}

                {provider === 'modal' && (
                  <div className='grid gap-4 sm:grid-cols-2'>
                    <div className='space-y-1.5'>
                      <label className='text-xs font-medium'>
                        {t('CPU Cores')}
                      </label>
                      <Select
                        value={String(cpuCores)}
                        onValueChange={(v) => setCpuCores(Number(v) || 0)}
                      >
                        <SelectTrigger className='w-full'>
                          <SelectValue>
                            {cpuCores > 0
                              ? `${cpuCores} ${t('Cores')}`
                              : t('Default')}
                          </SelectValue>
                        </SelectTrigger>
                        <SelectContent alignItemWithTrigger={false} align='start'>
                          <SelectItem value='0'>{t('Default')}</SelectItem>
                          {[1, 2, 4, 8, 16, 32].map((c) => (
                            <SelectItem key={c} value={String(c)}>
                              {c} {t('Cores')}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                    <div className='space-y-1.5'>
                      <label className='text-xs font-medium'>
                        {t('Memory (RAM)')}
                      </label>
                      <Select
                        value={String(memoryGb)}
                        onValueChange={(v) => setMemoryGb(Number(v) || 0)}
                      >
                        <SelectTrigger className='w-full'>
                          <SelectValue>
                            {memoryGb > 0 ? `${memoryGb} GB` : t('Default')}
                          </SelectValue>
                        </SelectTrigger>
                        <SelectContent alignItemWithTrigger={false} align='start'>
                          <SelectItem value='0'>{t('Default')}</SelectItem>
                          {[2, 4, 8, 16, 32, 64, 128].map((g) => (
                            <SelectItem key={g} value={String(g)}>
                              {g} GB
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                  </div>
                )}

                <div className='space-y-1.5'>
                  <label className='text-xs font-medium'>{t('Groups')}</label>
                  <MultiSelect
                    options={groupOptions}
                    selected={selectedGroups}
                    onChange={setSelectedGroups}
                    placeholder={t('Select groups')}
                    allowCreate
                  />
                </div>

                <div className='grid gap-4 sm:grid-cols-2'>
                  <div className='space-y-1.5'>
                    <label className='text-xs font-medium'>
                      {t('Priority')}
                    </label>
                    <Input
                      type='number'
                      value={priority}
                      onChange={(e) =>
                        setPriority(Number.parseInt(e.target.value, 10) || 0)
                      }
                    />
                  </div>

                  <div className='space-y-1.5'>
                    <label className='text-xs font-medium'>
                      {t('Weight')}
                    </label>
                    <Input
                      type='number'
                      min={1}
                      value={weight}
                      onChange={(e) =>
                        setWeight(
                          Math.max(1, Number.parseInt(e.target.value, 10) || 1)
                        )
                      }
                    />
                  </div>
                </div>

                <div className='space-y-1.5'>
                  <label className='text-xs font-medium'>
                    {t('HF Token (Optional)')}
                  </label>
                  <Input
                    type='password'
                    placeholder='hf_... (for gated models)'
                    value={hfToken}
                    onChange={(e) => setHfToken(e.target.value)}
                  />
                </div>
              </SideDrawerSection>
            </form>

            <SheetFooter className={sideDrawerFooterClassName()}>
              <SheetClose asChild>
                <Button variant='outline' type='button'>
                  {t('Cancel')}
                </Button>
              </SheetClose>
              {createMode === 'bulk' ? (
                <Button
                  type='submit'
                  form='unified-create-deployment-form'
                  disabled={
                    batchCreateMutation.isPending || bulkQueue.length === 0
                  }
                >
                  {batchCreateMutation.isPending && (
                    <Loader2 className='h-4 w-4 mr-1.5 animate-spin' />
                  )}
                  {t('Create All ({{count}})', { count: bulkQueue.length })}
                </Button>
              ) : (
                <Button
                  type='submit'
                  form='unified-create-deployment-form'
                  disabled={
                    createMutation.isPending ||
                    !name.trim() ||
                    !modelRepo.trim()
                  }
                >
                  {createMutation.isPending && (
                    <Loader2 className='h-4 w-4 mr-1.5 animate-spin' />
                  )}
                  {t('Create deployment')}
                </Button>
              )}
            </SheetFooter>
          </SheetContent>
        </Sheet>

        <HfModelPickerDialog
          open={hfPickerOpen}
          onOpenChange={setHfPickerOpen}
          onSelect={handleHfPicked}
        />

        <HfModelPickerDialog
          open={bulkPickerOpen}
          onOpenChange={setBulkPickerOpen}
          onSelect={handleHfPicked}
          multiSelect
          onSelectMultiple={handleBulkPicked}
        />

        {/* Right-Side Deployment Details & Live Console Drawer */}
        <Sheet
          open={Boolean(detailsItem)}
          onOpenChange={(open) => {
            if (!open) setDetailsItem(null)
          }}
        >
          <SheetContent className={sideDrawerContentClassName('sm:max-w-xl')}>
            <SheetHeader className={sideDrawerHeaderClassName()}>
              <SheetTitle>{t('Deployment Details')}</SheetTitle>
              <SheetDescription>{detailsItem?.name}</SheetDescription>
            </SheetHeader>

            {detailsItem ? (
              <div className={sideDrawerFormClassName()}>
                <Tabs
                  value={detailsTab}
                  onValueChange={(v) =>
                    setDetailsTab((v as 'overview' | 'logs') ?? 'overview')
                  }
                >
                  <TabsList className='w-full'>
                    <TabsTrigger value='overview' className='flex-1'>
                      {t('Overview & Routing')}
                    </TabsTrigger>
                    <TabsTrigger value='logs' className='flex-1'>
                      {t('Live Console')}
                    </TabsTrigger>
                  </TabsList>

                  <TabsContent value='overview' className='space-y-4 mt-4'>
                    <SideDrawerSection>
                      <div className='grid gap-3 text-xs sm:grid-cols-2'>
                        <div>
                          <div className='text-muted-foreground mb-1'>
                            {t('Provider')}
                          </div>
                          <StatusBadge
                            label={
                              PROVIDER_LABELS[
                                detailsItem.provider as DeploymentProviderId
                              ] ?? detailsItem.provider
                            }
                            variant='blue'
                            size='sm'
                            copyable={false}
                          />
                        </div>
                        <div>
                          <div className='text-muted-foreground mb-1'>
                            {t('Status')}
                          </div>
                          <DeploymentStatusCell item={detailsItem} />
                        </div>
                        <div className='sm:col-span-2'>
                          <div className='text-muted-foreground mb-1'>
                            {t('Model')}
                          </div>
                          <div className='font-mono font-medium break-all'>
                            {detailsItem.model_repo}
                          </div>
                          {detailsItem.model_file ? (
                            <div className='font-mono text-muted-foreground mt-0.5 break-all'>
                              {detailsItem.model_file}
                            </div>
                          ) : null}
                        </div>
                        <div className='sm:col-span-2'>
                          <div className='text-muted-foreground mb-1'>
                            {t('Endpoint URL')}
                          </div>
                          {detailsItem.endpoint_url ? (
                            <div className='flex items-center gap-2 rounded-md border bg-muted/30 px-2.5 py-1.5'>
                              <span className='font-mono text-xs break-all flex-1'>
                                {detailsItem.endpoint_url}
                              </span>
                              <Button
                                type='button'
                                size='icon-sm'
                                variant='ghost'
                                onClick={() => {
                                  navigator.clipboard.writeText(
                                    detailsItem.endpoint_url ?? ''
                                  )
                                  toast.success(t('Copied'))
                                }}
                              >
                                <Copy className='h-3.5 w-3.5' />
                              </Button>
                            </div>
                          ) : (
                            <span className='text-muted-foreground italic'>
                              {t('Provisioning automatically...')}
                            </span>
                          )}
                        </div>
                        {detailsItem.error_message ? (
                          <div className='sm:col-span-2 rounded-md border border-destructive/40 bg-destructive/10 p-2.5 text-destructive break-words'>
                            {detailsItem.error_message}
                          </div>
                        ) : null}
                        {(detailsItem.high_ram ||
                          (detailsItem.cpu_cores ?? 0) > 0 ||
                          (detailsItem.memory_gb ?? 0) > 0) && (
                          <div className='sm:col-span-2 flex flex-wrap gap-2'>
                            {detailsItem.high_ram && (
                              <StatusBadge
                                label={t('High-RAM')}
                                variant='neutral'
                                size='sm'
                                copyable={false}
                              />
                            )}
                            {(detailsItem.cpu_cores ?? 0) > 0 && (
                              <StatusBadge
                                label={`${detailsItem.cpu_cores} ${t('Cores')}`}
                                variant='neutral'
                                size='sm'
                                copyable={false}
                              />
                            )}
                            {(detailsItem.memory_gb ?? 0) > 0 && (
                              <StatusBadge
                                label={`${detailsItem.memory_gb} GB RAM`}
                                variant='neutral'
                                size='sm'
                                copyable={false}
                              />
                            )}
                          </div>
                        )}
                      </div>
                    </SideDrawerSection>

                    <SideDrawerSection>
                      <div className='flex items-center justify-between'>
                        <span className='text-xs font-medium'>
                          {t('Enabled')}
                        </span>
                        <Switch
                          checked={editEnabled}
                          onCheckedChange={setEditEnabled}
                        />
                      </div>
                      <div className='space-y-1.5'>
                        <label className='text-xs font-medium'>
                          {t('Groups')}
                        </label>
                        <MultiSelect
                          options={groupOptions}
                          selected={editGroups}
                          onChange={setEditGroups}
                          placeholder={t('Select groups')}
                          allowCreate
                        />
                      </div>
                      <div className='grid gap-3 sm:grid-cols-2'>
                        <div className='space-y-1'>
                          <label className='text-xs font-medium'>
                            {t('Priority')}
                          </label>
                          <Input
                            type='number'
                            value={editPriority}
                            onChange={(e) =>
                              setEditPriority(
                                Number.parseInt(e.target.value, 10) || 0
                              )
                            }
                          />
                        </div>
                        <div className='space-y-1'>
                          <label className='text-xs font-medium'>
                            {t('Weight')}
                          </label>
                          <Input
                            type='number'
                            min={1}
                            value={editWeight}
                            onChange={(e) =>
                              setEditWeight(
                                Math.max(
                                  1,
                                  Number.parseInt(e.target.value, 10) || 1
                                )
                              )
                            }
                          />
                        </div>
                      </div>
                      <Button
                        type='button'
                        size='sm'
                        disabled={savingDetails}
                        onClick={handleSaveDetails}
                      >
                        {savingDetails && (
                          <Loader2 className='h-3.5 w-3.5 mr-1.5 animate-spin' />
                        )}
                        {t('Save Routing')}
                      </Button>
                    </SideDrawerSection>
                  </TabsContent>

                  <TabsContent value='logs' className='space-y-3 mt-4'>
                    <div className='flex items-center justify-between gap-2'>
                      <span className='text-xs text-muted-foreground'>
                        {logsQuery.data?.data?.status_message ||
                          detailsItem.status_message ||
                          t('Streaming live deployment logs...')}
                      </span>
                      <div className='flex items-center gap-1.5'>
                        <Button
                          type='button'
                          variant='outline'
                          size='sm'
                          className='h-7 text-xs'
                          onClick={() => {
                            navigator.clipboard.writeText(
                              logsQuery.data?.data?.logs ||
                                detailsItem.logs ||
                                ''
                            )
                            toast.success(t('Copied'))
                          }}
                        >
                          <Copy className='h-3 w-3 mr-1' />
                          {t('Copy')}
                        </Button>
                      </div>
                    </div>

                    <pre
                      ref={logsEndRef}
                      className='h-[380px] overflow-auto rounded-lg border bg-zinc-950 p-3 font-mono text-xs text-zinc-100 whitespace-pre-wrap break-words'
                    >
                      {logsQuery.data?.data?.logs ||
                        detailsItem.logs ||
                        t('Waiting for logs...')}
                    </pre>
                  </TabsContent>
                </Tabs>
              </div>
            ) : null}

            <SheetFooter className={sideDrawerFooterClassName()}>
              <SheetClose asChild>
                <Button variant='outline' type='button'>
                  {t('Close')}
                </Button>
              </SheetClose>
            </SheetFooter>
          </SheetContent>
        </Sheet>

        {/* Test Model Dialog */}
        <Dialog
          open={Boolean(testTarget)}
          onOpenChange={(open) => {
            if (!open) setTestTarget(null)
          }}
        >
          <DialogContent className='max-w-xl'>
            <DialogHeader>
              <DialogTitle>
                {t('Test Model')}: {testTarget?.model_repo}
              </DialogTitle>
            </DialogHeader>
            <div className='space-y-4 py-2'>
              <div className='flex gap-2'>
                <Input
                  value={testPrompt}
                  onChange={(e) => setTestPrompt(e.target.value)}
                  placeholder={t('Enter test prompt...')}
                />
                <Button disabled={testing} onClick={handleRunTest}>
                  {testing ? (
                    <Loader2 className='h-4 w-4 mr-1.5 animate-spin' />
                  ) : (
                    <Play className='h-4 w-4 mr-1.5' />
                  )}
                  {t('Test')}
                </Button>
              </div>

              {testing && (
                <div className='space-y-2 rounded-lg border p-3 bg-muted/20'>
                  <div className='flex items-center justify-between text-xs'>
                    <span className='font-medium truncate pr-2'>
                      {testStage || t('Starting test...')}
                    </span>
                    <span className='font-mono shrink-0'>
                      <span className='font-semibold'>{testProgress}%</span>
                      <span className='text-muted-foreground ml-2'>
                        {testElapsedSec}s
                      </span>
                    </span>
                  </div>
                  <div className='h-2 w-full overflow-hidden rounded-full bg-muted'>
                    <div
                      className='h-full bg-primary transition-all duration-300'
                      style={{ width: `${Math.max(2, testProgress)}%` }}
                    />
                  </div>
                </div>
              )}

              {testError ? (
                <div className='rounded-md border border-destructive/40 bg-destructive/10 p-3 text-xs text-destructive'>
                  {testError}
                </div>
              ) : null}

              {testResult ? (
                <div className='space-y-3 rounded-lg border p-3 bg-muted/20'>
                  <div className='flex items-center justify-between text-xs text-muted-foreground'>
                    <span className='capitalize font-medium'>
                      {t('Response')} ({testResult.type})
                    </span>
                    <span>{testResult.latency_ms} ms</span>
                  </div>

                  {testResult.type === 'video' && testResult.media_url ? (
                    <div className='space-y-2'>
                      <video
                        src={testResult.media_url}
                        controls
                        className='w-full rounded-md max-h-72 bg-black'
                      />
                      <a
                        href={testResult.media_url}
                        target='_blank'
                        rel='noreferrer'
                        className='text-xs text-primary underline inline-flex items-center gap-1 break-all'
                      >
                        {testResult.media_url}
                        <ExternalLink className='h-3 w-3 shrink-0' />
                      </a>
                    </div>
                  ) : testResult.type === 'image' && testResult.media_url ? (
                    <div className='space-y-2'>
                      <img
                        src={testResult.media_url}
                        alt='Generated output'
                        className='w-full rounded-md max-h-72 object-contain'
                      />
                      <a
                        href={testResult.media_url}
                        target='_blank'
                        rel='noreferrer'
                        className='text-xs text-primary underline inline-flex items-center gap-1 break-all'
                      >
                        {testResult.media_url}
                        <ExternalLink className='h-3 w-3 shrink-0' />
                      </a>
                    </div>
                  ) : testResult.type === 'audio' && testResult.media_url ? (
                    <div className='space-y-2'>
                      <audio
                        src={testResult.media_url}
                        controls
                        className='w-full'
                      />
                      <a
                        href={testResult.media_url}
                        target='_blank'
                        rel='noreferrer'
                        className='text-xs text-primary underline inline-flex items-center gap-1 break-all'
                      >
                        {testResult.media_url}
                        <ExternalLink className='h-3 w-3 shrink-0' />
                      </a>
                    </div>
                  ) : (
                    <pre className='max-h-64 overflow-auto whitespace-pre-wrap rounded-md bg-background p-3 text-xs border'>
                      {testResult.text || testResult.raw}
                    </pre>
                  )}
                </div>
              ) : null}
            </div>
          </DialogContent>
        </Dialog>

        {/* Delete Confirm */}
        <AlertDialog
          open={Boolean(deleteTarget)}
          onOpenChange={(open) => {
            if (!open) setDeleteTarget(null)
          }}
        >
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>{t('Confirm delete')}</AlertDialogTitle>
              <AlertDialogDescription>
                {t('Delete deployment {{name}}?', {
                  name: deleteTarget?.name ?? '',
                })}
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel disabled={deleting}>
                {t('Cancel')}
              </AlertDialogCancel>
              <AlertDialogAction
                onClick={handleDelete}
                disabled={deleting}
                variant='destructive'
              >
                {deleting ? t('Deleting...') : t('Delete')}
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </div>
    </TooltipProvider>
  )
}
