import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Download, Loader2, Terminal, Trash2 } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

import { ConfirmDialog } from '@/components/confirm-dialog'
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
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Separator } from '@/components/ui/separator'
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
  getDeploymentAccounts,
  type DeploymentAccountSummary,
} from '@/features/models/api'
import {
  getHardwareForTier,
  PROVIDER_LABELS,
  type DeploymentProviderId,
} from '@/features/models/lib/hardware-catalog'
import { api } from '@/lib/api'
import { formatCurrencyFromUSD } from '@/lib/currency'
import { formatTimestamp, formatTokens } from '@/lib/format'
import { useSystemConfigStore } from '@/stores/system-config-store'

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

export interface FineTuningGPURow {
  id: string
  label: string
  kind?: 'cpu' | 'gpu' | 'tpu'
  providers?: DeploymentProviderId[]
  price_per_second: number
  enabled: boolean
}

interface AdminFineTuningJobRow {
  id: number
  job_id: string
  user_id: number
  username?: string
  model: string
  fine_tuned_model: string
  status: string
  progress?: number
  trained_tokens: number
  gpu?: string
  method?: string
  task_type?: string
  ram_tier?: string
  estimated_cost_usd: number
  has_model_file?: boolean
  created_at: number
  finished_at: number
  expires_at?: number
  files_purged_at?: number
}

interface AdminFineTuningListPayload {
  jobs: AdminFineTuningJobRow[]
  total_jobs: number
  total_revenue_usd: number
  total_trained_tokens: number
}

const DEFAULT_PRICE_BY_HW_ID: Record<string, number> = {
  CPU: 0.0005,
  'cpu-basic': 0.0004,
  'cpu-upgrade': 0.0008,
  T4: 0.0015,
  't4-small': 0.0015,
  't4-medium': 0.002,
  NVIDIA_TESLA_T4: 0.0015,
  NvidiaTeslaT4: 0.002,
  L4: 0.0025,
  l4x1: 0.0025,
  NVIDIA_L4: 0.0025,
  NvidiaL4: 0.0025,
  A10G: 0.003,
  'a10g-small': 0.003,
  'a10g-large': 0.004,
  TPU_V5E_1: 0.0025,
  TPU_V2_8: 0.003,
  TPU_V6E_1: 0.004,
  TpuV5E8: 0.0035,
  TpuV6E8: 0.005,
  'RTX-3090': 0.0018,
  'RTX-4090': 0.0025,
  L40S: 0.004,
  l40sx1: 0.0045,
  'A100-40GB': 0.0045,
  NVIDIA_TESLA_A100: 0.005,
  'A100-80GB': 0.006,
  NVIDIA_TESLA_A100_80GB: 0.006,
  A100: 0.006,
  'a100-large': 0.006,
  'RTX-PRO-6000': 0.008,
  NVIDIA_RTX_PRO_6000: 0.008,
  H100: 0.01,
  'H100-SXM': 0.01,
  'H100-80GB': 0.01,
  NVIDIA_H100_80GB: 0.01,
  h100x1: 0.01,
  H200: 0.012,
  h200x1: 0.012,
  B200: 0.015,
  B300: 0.018,
}

const DEFAULT_FT_GPU_ROWS: FineTuningGPURow[] = [
  { id: 'CPU', label: 'CPU (Standard)', kind: 'cpu', price_per_second: 0.0005, enabled: true },
  { id: 'NVIDIA_TESLA_T4', label: 'GPU: NVIDIA Tesla T4 (16GB)', kind: 'gpu', price_per_second: 0.0015, enabled: true },
  { id: 'NVIDIA_L4', label: 'GPU: NVIDIA L4 (24GB)', kind: 'gpu', price_per_second: 0.0025, enabled: true },
  { id: 'NVIDIA_A10G', label: 'GPU: NVIDIA A10G (24GB)', kind: 'gpu', price_per_second: 0.003, enabled: true },
  { id: 'NVIDIA_TESLA_A100', label: 'GPU: NVIDIA A100 (40GB)', kind: 'gpu', price_per_second: 0.006, enabled: true },
  { id: 'NVIDIA_H100_80GB', label: 'GPU: NVIDIA H100 (80GB)', kind: 'gpu', price_per_second: 0.01, enabled: true },
]

function parseFineTuningGPURows(raw?: string, legacyBasePrice = 0.002): FineTuningGPURow[] {
  const trimmed = (raw ?? '').trim()
  if (trimmed.startsWith('[')) {
    try {
      const parsed = JSON.parse(trimmed)
      if (Array.isArray(parsed) && parsed.length > 0) {
        return parsed.map((item) => ({
          id: String(item.id ?? ''),
          label: String(item.label ?? item.id ?? ''),
          kind: item.kind,
          price_per_second: Number(item.price_per_second) || 0,
          enabled: item.enabled !== false,
        }))
      }
    } catch {
      // fall through
    }
  }
  if (trimmed.startsWith('{')) {
    try {
      const legacy = JSON.parse(trimmed) as Record<string, number>
      return DEFAULT_FT_GPU_ROWS.map((row) =>
        row.id in legacy
          ? {
              ...row,
              price_per_second: Number(((legacy[row.id] ?? 1) * legacyBasePrice).toFixed(6)),
              enabled: (legacy[row.id] ?? 1) >= 0,
            }
          : row
      )
    } catch {
      // fall through
    }
  }
  return DEFAULT_FT_GPU_ROWS.map((r) => ({ ...r }))
}

const schema = z.object({
  fineTuningEnabled: z.boolean().optional(),
  retentionDays: z.number().int().min(1).max(365),
  expiryWarningDays: z.number().int().min(0).max(60),
  colabNormalRamMultiplier: z.number().min(0.1).max(20),
  colabHighRamMultiplier: z.number().min(0.1).max(20),
  fineTuningGpuRows: z
    .array(
      z.object({
        id: z.string(),
        label: z.string(),
        kind: z.enum(['cpu', 'gpu', 'tpu']).optional(),
        price_per_second: z.number().min(0),
        enabled: z.boolean(),
      })
    )
    .optional(),
})

type Values = z.infer<typeof schema>

export function FineTuningSettingsSection({
  defaultValues,
}: {
  defaultValues: {
    fineTuningEnabled?: boolean
    fineTuningPricePerSecond?: number
    fineTuningGpuMultipliers?: string
    colabNormalRamMultiplier?: number
    colabHighRamMultiplier?: number
    retentionDays?: number
    expiryWarningDays?: number
    enabledProviders: Record<DeploymentProviderId, boolean>
  }
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const updateOption = useUpdateOption()
  const currencyConfig = useSystemConfigStore((s) => s.config.currency)
  void currencyConfig

  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: {
      fineTuningEnabled: defaultValues.fineTuningEnabled ?? false,
      retentionDays: defaultValues.retentionDays ?? 30,
      expiryWarningDays: defaultValues.expiryWarningDays ?? 3,
      colabNormalRamMultiplier: defaultValues.colabNormalRamMultiplier ?? 1,
      colabHighRamMultiplier: defaultValues.colabHighRamMultiplier ?? 1.5,
      fineTuningGpuRows: parseFineTuningGPURows(
        defaultValues.fineTuningGpuMultipliers,
        defaultValues.fineTuningPricePerSecond ?? 0.002
      ),
    },
  })

  const { isDirty, isSubmitting } = form.formState
  const savedGpuRows = form.watch('fineTuningGpuRows') ?? DEFAULT_FT_GPU_ROWS
  const [accountsByProvider, setAccountsByProvider] = useState<
    Record<string, DeploymentAccountSummary[]>
  >({})
  const [logTarget, setLogTarget] = useState<AdminFineTuningJobRow | null>(null)
  const [deleteTarget, setDeleteTarget] =
    useState<AdminFineTuningJobRow | null>(null)

  useEffect(() => {
    void getDeploymentAccounts()
      .then((res) => {
        if (res?.success && res.data) {
          setAccountsByProvider(res.data)
        }
      })
      .catch(() => {})
  }, [])

  const gpuRows = useMemo<FineTuningGPURow[]>(() => {
    const orderedProviders: DeploymentProviderId[] = [
      'colab',
      'kaggle',
      'modal',
      'huggingface',
      'runpod',
      'hostinger',
      'ionet',
    ]
    const savedById = new Map(savedGpuRows.map((r) => [r.id, r]))
    const byId = new Map<string, FineTuningGPURow>()

    for (const provider of orderedProviders) {
      if (!defaultValues.enabledProviders[provider]) continue
      const accounts = accountsByProvider[provider] ?? []
      const activeAccounts = accounts.filter((a) => a.enabled)
      const tiers =
        activeAccounts.length > 0
          ? activeAccounts.map((a) => a.tier)
          : accounts.length === 0
            ? [undefined]
            : []
      for (const tier of tiers) {
        for (const hw of getHardwareForTier(provider, tier)) {
          const existing = byId.get(hw.id)
          if (existing) {
            if (!existing.providers?.includes(provider)) {
              existing.providers = [...(existing.providers ?? []), provider]
            }
            continue
          }
          const saved = savedById.get(hw.id)
          byId.set(hw.id, {
            id: hw.id,
            label: hw.label,
            kind: hw.kind,
            providers: [provider],
            price_per_second:
              saved?.price_per_second ??
              DEFAULT_PRICE_BY_HW_ID[hw.id] ??
              0.002,
            enabled: saved ? saved.enabled : true,
          })
        }
      }
    }

    if (byId.size > 0) {
      return Array.from(byId.values())
    }
    return savedGpuRows
  }, [defaultValues.enabledProviders, accountsByProvider, savedGpuRows])

  const updateGpuRow = (
    rowId: string,
    patch: Partial<Pick<FineTuningGPURow, 'price_per_second' | 'enabled'>>
  ) => {
    const next = gpuRows.map((r) =>
      r.id === rowId
        ? {
            id: r.id,
            label: r.label,
            kind: r.kind,
            price_per_second: patch.price_per_second ?? r.price_per_second,
            enabled: patch.enabled ?? r.enabled,
          }
        : {
            id: r.id,
            label: r.label,
            kind: r.kind,
            price_per_second: r.price_per_second,
            enabled: r.enabled,
          }
    )
    form.setValue('fineTuningGpuRows', next, { shouldDirty: true })
  }

  const adminJobsQuery = useQuery({
    queryKey: ['admin-fine-tuning-jobs'],
    queryFn: async () => {
      const res = await api.get<{
        success: boolean
        data?: AdminFineTuningListPayload
      }>('/api/fine_tuning/admin/jobs')
      return (
        res.data?.data ?? {
          jobs: [],
          total_jobs: 0,
          total_revenue_usd: 0,
          total_trained_tokens: 0,
        }
      )
    },
    refetchInterval: 5000,
  })

  const logsQuery = useQuery({
    queryKey: ['admin-fine-tuning-logs', logTarget?.job_id],
    queryFn: async () => {
      if (!logTarget) return null
      const res = await api.get<{
        success: boolean
        data?: { logs: string; status: string; progress: number }
      }>(`/api/user/fine_tuning/jobs/${logTarget.job_id}/logs`)
      return res.data?.data ?? null
    },
    enabled: Boolean(logTarget),
    refetchInterval: 2000,
  })

  const deleteJobMutation = useMutation({
    mutationFn: async (jobId: string) => {
      const res = await api.delete(`/api/fine_tuning/admin/jobs/${jobId}`)
      if (!res.data?.success) {
        throw new Error(res.data?.message || t('Failed to delete fine-tuning job'))
      }
    },
    onSuccess: () => {
      toast.success(t('Fine-tuning model deleted'))
      setDeleteTarget(null)
      void queryClient.invalidateQueries({ queryKey: ['admin-fine-tuning-jobs'] })
    },
    onError: (err: Error) => {
      toast.error(err.message || t('Failed to delete fine-tuning job'))
    },
  })

  const handleDownloadModel = async (jobId: string) => {
    try {
      const res = await api.get(`/api/user/fine_tuning/jobs/${jobId}/download`, {
        responseType: 'blob',
      })
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

  async function onSubmit(values: Values) {
    try {
      await updateOption.mutateAsync({
        key: 'model_deployment.fine_tuning.enabled',
        value: String(Boolean(values.fineTuningEnabled)),
      })
      await updateOption.mutateAsync({
        key: 'model_deployment.fine_tuning.retention_days',
        value: String(values.retentionDays ?? 30),
      })
      await updateOption.mutateAsync({
        key: 'model_deployment.fine_tuning.expiry_warning_days',
        value: String(values.expiryWarningDays ?? 3),
      })
      await updateOption.mutateAsync({
        key: 'model_deployment.fine_tuning.colab_normal_ram_multiplier',
        value: String(values.colabNormalRamMultiplier ?? 1),
      })
      await updateOption.mutateAsync({
        key: 'model_deployment.fine_tuning.colab_high_ram_multiplier',
        value: String(values.colabHighRamMultiplier ?? 1.5),
      })
      const persistedRows = gpuRows.map((r) => ({
        id: r.id,
        label: r.label,
        kind: r.kind,
        price_per_second: r.price_per_second,
        enabled: r.enabled,
      }))
      await updateOption.mutateAsync({
        key: 'model_deployment.fine_tuning.gpu_multipliers',
        value: JSON.stringify(persistedRows),
      })
      form.reset(values)
      toast.success(t('Fine-Tuning settings saved successfully'))
    } catch {
      toast.error(t('Failed to save Fine-Tuning settings'))
    }
  }

  const jobsData = adminJobsQuery.data

  return (
    <>
      <SettingsSection title={t('Fine-Tuning Settings')}>
        <Form {...form}>
          <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
            <FormField
              control={form.control}
              name='fineTuningEnabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Enable Fine-Tuning')}</FormLabel>
                    <FormDescription>
                      {t(
                        'Enable OpenAI-compatible /v1/fine_tuning/jobs endpoints and user Fine-tuning UI.'
                      )}
                    </FormDescription>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch
                      checked={Boolean(field.value)}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </SettingsSwitchItem>
              )}
            />

            {/* Storage Retention & Pre-Expiry Email Warning */}
            <FormField
              control={form.control}
              name='retentionDays'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Model Retention Period (Days)')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={1}
                      max={365}
                      value={field.value ?? 30}
                      onChange={(e) =>
                        field.onChange(Math.max(1, Number(e.target.value) || 30))
                      }
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Trained model archives are automatically purged from server storage after this many days (default: 30 days).'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='expiryWarningDays'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Expiry Reminder Email (Days Before)')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={0}
                      max={60}
                      value={field.value ?? 3}
                      onChange={(e) =>
                        field.onChange(Math.max(0, Number(e.target.value) || 0))
                      }
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Send a download reminder email this many days before deletion (default: 3 days). Customize the template in SMTP Email settings.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            {/* Hardware Pricing Table */}
            <FormItem data-settings-form-span='full' className='space-y-3'>
              <FormLabel>{t('Fine-Tuning Hardware Pricing')}</FormLabel>
              <FormDescription>
                {t(
                  'Configure per-second USD pricing and toggle availability for CPU, GPU, and TPU options unlocked by your enabled deployment accounts.'
                )}
              </FormDescription>
              <div className='overflow-x-auto rounded-lg border'>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('Accelerator')}</TableHead>
                      <TableHead className='w-[200px]'>
                        {t('Price per Second (USD/s)')}
                      </TableHead>
                      <TableHead className='w-[100px] text-right'>
                        {t('Enabled')}
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {gpuRows.map((row) => (
                      <TableRow
                        key={row.id}
                        className={!row.enabled ? 'opacity-55' : undefined}
                      >
                        <TableCell>
                          <div className='flex flex-wrap items-center gap-1.5'>
                            <span className='text-sm font-medium'>
                              {row.label}
                            </span>
                            {row.providers?.map((p) => (
                              <Badge
                                key={p}
                                variant='secondary'
                                className='px-1.5 py-0 text-[10px] font-normal'
                              >
                                {PROVIDER_LABELS[p] ?? p}
                              </Badge>
                            ))}
                          </div>
                          <div className='text-muted-foreground font-mono text-[11px]'>
                            {row.id}
                          </div>
                        </TableCell>
                        <TableCell>
                          <div className='relative flex items-center'>
                            <span className='text-muted-foreground pointer-events-none absolute left-2.5 text-xs'>
                              $
                            </span>
                            <Input
                              type='number'
                              step='0.0001'
                              min='0'
                              disabled={!row.enabled}
                              value={row.price_per_second}
                              onChange={(e) => {
                                updateGpuRow(row.id, {
                                  price_per_second:
                                    e.target.value === ''
                                      ? 0
                                      : Math.max(0, Number(e.target.value)),
                                })
                              }}
                              className='h-8 pl-6 font-mono text-xs'
                            />
                          </div>
                        </TableCell>
                        <TableCell className='text-right'>
                          <Switch
                            checked={row.enabled}
                            onCheckedChange={(checked) => {
                              updateGpuRow(row.id, { enabled: checked })
                            }}
                          />
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>

              {defaultValues.enabledProviders.colab && (
                <div className='bg-muted/20 grid grid-cols-1 gap-4 rounded-lg border p-3.5 sm:grid-cols-2'>
                  <FormField
                    control={form.control}
                    name='colabNormalRamMultiplier'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>
                          {t('Colab Normal RAM Multiplier')}
                        </FormLabel>
                        <FormControl>
                          <Input
                            type='number'
                            step='0.05'
                            min='0.1'
                            max='20'
                            value={field.value ?? 1}
                            onChange={(e) =>
                              field.onChange(
                                e.target.value === ''
                                  ? 1
                                  : Math.max(0.1, Number(e.target.value))
                              )
                            }
                            className='h-8 font-mono text-xs'
                          />
                        </FormControl>
                        <FormDescription className='text-xs'>
                          {t(
                            'Pricing multiplier applied to standard Colab RAM runtimes (default 1.0x).'
                          )}
                        </FormDescription>
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='colabHighRamMultiplier'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Colab High RAM Multiplier')}</FormLabel>
                        <FormControl>
                          <Input
                            type='number'
                            step='0.05'
                            min='0.1'
                            max='20'
                            value={field.value ?? 1.5}
                            onChange={(e) =>
                              field.onChange(
                                e.target.value === ''
                                  ? 1.5
                                  : Math.max(0.1, Number(e.target.value))
                              )
                            }
                            className='h-8 font-mono text-xs'
                          />
                        </FormControl>
                        <FormDescription className='text-xs'>
                          {t(
                            'Pricing multiplier applied when High-RAM is selected on Colab (default 1.5x).'
                          )}
                        </FormDescription>
                      </FormItem>
                    )}
                  />
                </div>
              )}
            </FormItem>

            <SettingsPageFormActions
              onSave={form.handleSubmit(onSubmit)}
              onReset={() => form.reset()}
              isSaving={isSubmitting || updateOption.isPending}
              isSaveDisabled={!isDirty}
              isResetDisabled={!isDirty}
            />
          </SettingsForm>
        </Form>

        <Separator className='my-6' />

        {/* Mini Admin Fine-Tuning Management Panel */}
        <Card>
          <CardHeader className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
            <div>
              <CardTitle className='text-base'>
                {t('Fine-Tuning Jobs Management')}
              </CardTitle>
              <CardDescription className='mt-1 text-xs'>
                {t(
                  'Monitor all user fine-tuning jobs, revenue, training logs, model archives, and storage cleanup.'
                )}
              </CardDescription>
            </div>
            <div className='flex flex-wrap items-center gap-2'>
              <Badge variant='outline'>
                {t('Jobs')}: {jobsData?.total_jobs ?? 0}
              </Badge>
              <Badge variant='secondary'>
                {t('Revenue')}:{' '}
                {formatCurrencyFromUSD(jobsData?.total_revenue_usd ?? 0)}
              </Badge>
              <Badge variant='outline'>
                {t('Trained Tokens')}:{' '}
                {formatTokens(jobsData?.total_trained_tokens ?? 0)}
              </Badge>
            </div>
          </CardHeader>
          <CardContent>
            <div className='overflow-x-auto'>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('User')}</TableHead>
                    <TableHead>{t('Model')}</TableHead>
                    <TableHead>{t('Accelerator')}</TableHead>
                    <TableHead>{t('Status')}</TableHead>
                    <TableHead>{t('Revenue')}</TableHead>
                    <TableHead>{t('Expires')}</TableHead>
                    <TableHead className='text-right'>{t('Actions')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {adminJobsQuery.isLoading ? (
                    <TableRow>
                      <TableCell colSpan={7} className='py-8 text-center'>
                        <Loader2 className='text-muted-foreground mx-auto h-5 w-5 animate-spin' />
                      </TableCell>
                    </TableRow>
                  ) : (jobsData?.jobs ?? []).length === 0 ? (
                    <TableRow>
                      <TableCell
                        colSpan={7}
                        className='text-muted-foreground py-8 text-center text-sm'
                      >
                        {t('No fine-tuning jobs yet.')}
                      </TableCell>
                    </TableRow>
                  ) : (
                    (jobsData?.jobs ?? []).map((job) => (
                      <TableRow key={job.job_id}>
                        <TableCell className='text-xs'>
                          <div className='font-medium'>
                            {job.username || `#${job.user_id}`}
                          </div>
                          <div className='text-muted-foreground font-mono text-[11px]'>
                            #{job.user_id}
                          </div>
                        </TableCell>
                        <TableCell className='font-mono text-xs'>
                          <div className='max-w-[240px] truncate font-medium'>
                            {job.fine_tuned_model || job.job_id}
                          </div>
                          <div className='text-muted-foreground text-[11px]'>
                            {job.job_id}
                          </div>
                        </TableCell>
                        <TableCell className='text-xs'>
                          {job.gpu || 'CPU'}
                        </TableCell>
                        <TableCell>
                          <Badge
                            variant={
                              job.status === 'succeeded'
                                ? 'default'
                                : job.status === 'failed'
                                  ? 'destructive'
                                  : 'secondary'
                            }
                          >
                            {job.status === 'running' && job.progress
                              ? `${t('Running')} (${job.progress}%)`
                              : t(
                                  job.status.charAt(0).toUpperCase() +
                                    job.status.slice(1)
                                )}
                          </Badge>
                        </TableCell>
                        <TableCell className='font-mono text-xs tabular-nums'>
                          {formatCurrencyFromUSD(job.estimated_cost_usd)}
                        </TableCell>
                        <TableCell className='text-muted-foreground text-xs whitespace-nowrap'>
                          {job.files_purged_at
                            ? t('Expired')
                            : job.expires_at
                              ? formatTimestamp(job.expires_at)
                              : '—'}
                        </TableCell>
                        <TableCell className='text-right'>
                          <div className='flex items-center justify-end gap-1'>
                            <Button
                              variant='ghost'
                              size='icon'
                              title={t('View Console')}
                              onClick={() => setLogTarget(job)}
                            >
                              <Terminal className='h-4 w-4' />
                            </Button>
                            {job.status === 'succeeded' &&
                              job.has_model_file && (
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
                              onClick={() => setDeleteTarget(job)}
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
      </SettingsSection>

      {/* Logs Modal */}
      <Dialog
        open={Boolean(logTarget)}
        onOpenChange={(open) => {
          if (!open) setLogTarget(null)
        }}
      >
        <DialogContent className='flex max-h-[85vh] flex-col sm:max-w-3xl'>
          <DialogHeader>
            <DialogTitle className='flex items-center gap-2'>
              <Terminal className='h-4 w-4' />
              {t('Training Console')}
            </DialogTitle>
            <DialogDescription className='font-mono text-xs'>
              {logTarget?.fine_tuned_model || logTarget?.job_id} ·{' '}
              {logTarget?.username || `#${logTarget?.user_id}`}
            </DialogDescription>
          </DialogHeader>
          <pre className='h-96 overflow-y-auto rounded-lg border bg-zinc-950 p-3.5 font-mono text-xs leading-relaxed whitespace-pre-wrap break-words text-zinc-100'>
            {logsQuery.isLoading
              ? t('Loading logs...')
              : logsQuery.data?.logs ||
                t('Waiting for training worker output...')}
          </pre>
          <DialogFooter>
            <Button variant='outline' onClick={() => setLogTarget(null)}>
              {t('Close')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete Confirm */}
      <ConfirmDialog
        open={Boolean(deleteTarget)}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null)
        }}
        title={t('Delete fine-tuned model?')}
        desc={t(
          'Are you sure you want to delete "{{model}}"? Training logs, checkpoints, and saved model files will be permanently removed.',
          {
            model: deleteTarget?.fine_tuned_model || deleteTarget?.job_id || '',
          }
        )}
        destructive
        isLoading={deleteJobMutation.isPending}
        handleConfirm={() => {
          if (deleteTarget) {
            deleteJobMutation.mutate(deleteTarget.job_id)
          }
        }}
        confirmText={t('Delete')}
      />
    </>
  )
}
