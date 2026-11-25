import { zodResolver } from '@hookform/resolvers/zod'
import { useEffect, useState } from 'react'
import { useForm, type UseFormReturn } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
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
import type { DeploymentProviderId } from '@/features/models/lib/hardware-catalog'

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'
import { ProviderAccountsManager } from './provider-accounts-manager'

export interface FineTuningGPURow {
  id: string
  label: string
  price_per_second: number
  enabled: boolean
}

const DEFAULT_FT_GPU_ROWS: FineTuningGPURow[] = [
  { id: 'CPU', label: 'CPU', price_per_second: 0.0005, enabled: true },
  {
    id: 'NVIDIA_TESLA_T4',
    label: 'NVIDIA T4 (16GB)',
    price_per_second: 0.0015,
    enabled: true,
  },
  {
    id: 'NVIDIA_L4',
    label: 'NVIDIA L4 (24GB)',
    price_per_second: 0.0025,
    enabled: true,
  },
  {
    id: 'NVIDIA_A10G',
    label: 'NVIDIA A10G (24GB)',
    price_per_second: 0.003,
    enabled: true,
  },
  {
    id: 'NVIDIA_TESLA_A100',
    label: 'NVIDIA A100 (80GB)',
    price_per_second: 0.006,
    enabled: true,
  },
  {
    id: 'NVIDIA_H100_80GB',
    label: 'NVIDIA H100 (80GB)',
    price_per_second: 0.01,
    enabled: true,
  },
]

function parseFineTuningGPURows(
  raw?: string,
  legacyBasePrice = 0.002
): FineTuningGPURow[] {
  const trimmed = (raw ?? '').trim()
  if (trimmed.startsWith('[')) {
    try {
      const parsed = JSON.parse(trimmed)
      if (Array.isArray(parsed) && parsed.length > 0) {
        return parsed.map((item) => ({
          id: String(item.id ?? ''),
          label: String(item.label ?? item.id ?? ''),
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
              price_per_second: Number(
                ((legacy[row.id] ?? 1) * legacyBasePrice).toFixed(6)
              ),
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
  // Global dispatch priority & fine-tuning
  priority: z.string().optional(),
  fineTuningEnabled: z.boolean().optional(),
  fineTuningGpuRows: z
    .array(
      z.object({
        id: z.string(),
        label: z.string(),
        price_per_second: z.coerce.number().min(0),
        enabled: z.boolean(),
      })
    )
    .optional(),

  // io.net
  ionetEnabled: z.boolean(),
  ionetIdleTimeoutSeconds: z.number().min(0).optional(),

  // Modal
  modalEnabled: z.boolean(),
  modalSharedVolumePath: z.string().optional(),
  modalIdleTimeoutSeconds: z.number().min(0).optional(),

  // Hugging Face Spaces
  hfEnabled: z.boolean(),
  hfIdleTimeoutSeconds: z.number().min(0).optional(),

  // Hostinger GPU VPS
  hostingerEnabled: z.boolean(),
  hostingerIdleTimeoutSeconds: z.number().min(0).optional(),

  // RunPod GPU Cloud
  runpodEnabled: z.boolean(),
  runpodIdleTimeoutSeconds: z.number().min(0).optional(),

  // Google Colab
  colabEnabled: z.boolean(),
  colabIdleTimeoutSeconds: z.number().min(0).optional(),

  // Kaggle
  kaggleEnabled: z.boolean(),
  kaggleIdleTimeoutSeconds: z.number().min(0).optional(),
})

type Values = z.infer<typeof schema>

function IdleSleepField({
  form,
  name,
}: {
  form: UseFormReturn<Values>
  name:
    | 'ionetIdleTimeoutSeconds'
    | 'modalIdleTimeoutSeconds'
    | 'hfIdleTimeoutSeconds'
    | 'hostingerIdleTimeoutSeconds'
    | 'runpodIdleTimeoutSeconds'
    | 'colabIdleTimeoutSeconds'
    | 'kaggleIdleTimeoutSeconds'
}) {
  const { t } = useTranslation()
  const value = form.watch(name) ?? 0
  const isEnabled = value > 0

  return (
    <div className='rounded-lg border border-border/60 bg-muted/20 p-3 space-y-3'>
      <div className='flex items-center justify-between gap-4'>
        <div>
          <div className='text-sm font-medium'>
            {t('Auto-sleep when idle')}
          </div>
          <div className='text-xs text-muted-foreground'>
            {t(
              'Automatically scale down / sleep instances when inactive. Disable to keep instances running.'
            )}
          </div>
        </div>
        <Switch
          checked={isEnabled}
          onCheckedChange={(checked) => {
            form.setValue(name, checked ? 300 : 0, { shouldDirty: true })
          }}
        />
      </div>

      {isEnabled && (
        <FormField
          control={form.control}
          name={name}
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Idle Sleep Timeout (Seconds)')}</FormLabel>
              <FormControl>
                <Input
                  type='number'
                  min={1}
                  placeholder='300'
                  value={field.value ?? 300}
                  onChange={(e) => {
                    const parsed = Number.parseInt(e.target.value, 10)
                    field.onChange(
                      Number.isFinite(parsed) && parsed > 0 ? parsed : 1
                    )
                  }}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
      )}
    </div>
  )
}

export function UnifiedDeploymentSettingsSection({
  defaultValues,
}: {
  defaultValues: {
    priority?: string
    fineTuningEnabled?: boolean
    fineTuningPricePerSecond?: number
    fineTuningGpuMultipliers?: string
    ionet?: {
      enabled?: boolean
      apiKey?: string
      idleTimeoutSeconds?: number
    }
    modal?: {
      enabled?: boolean
      tokenId?: string
      tokenSecret?: string
      sharedVolumePath?: string
      idleTimeoutSeconds?: number
    }
    huggingface?: {
      enabled?: boolean
      token?: string
      org?: string
      idleTimeoutSeconds?: number
    }
    hostinger?: {
      enabled?: boolean
      apiKey?: string
      idleTimeoutSeconds?: number
    }
    runpod?: {
      enabled?: boolean
      apiKey?: string
      idleTimeoutSeconds?: number
    }
    colab?: {
      enabled?: boolean
      accessToken?: string
      refreshToken?: string
      accountEmail?: string
      projectId?: string
      region?: string
      idleTimeoutSeconds?: number
    }
    kaggle?: {
      enabled?: boolean
      username?: string
      apiKey?: string
      idleTimeoutSeconds?: number
    }
  }
}) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: {
      priority: defaultValues.priority || 'deployments_first',
      fineTuningEnabled: defaultValues.fineTuningEnabled ?? false,
      fineTuningGpuRows: parseFineTuningGPURows(
        defaultValues.fineTuningGpuMultipliers,
        defaultValues.fineTuningPricePerSecond ?? 0.002
      ),

      ionetEnabled: defaultValues.ionet?.enabled ?? false,
      ionetIdleTimeoutSeconds: defaultValues.ionet?.idleTimeoutSeconds ?? 300,

      modalEnabled: defaultValues.modal?.enabled ?? false,
      modalSharedVolumePath:
        defaultValues.modal?.sharedVolumePath ?? '/vol/models',
      modalIdleTimeoutSeconds: defaultValues.modal?.idleTimeoutSeconds ?? 300,

      hfEnabled: defaultValues.huggingface?.enabled ?? false,
      hfIdleTimeoutSeconds: defaultValues.huggingface?.idleTimeoutSeconds ?? 300,

      hostingerEnabled: defaultValues.hostinger?.enabled ?? false,
      hostingerIdleTimeoutSeconds:
        defaultValues.hostinger?.idleTimeoutSeconds ?? 300,

      runpodEnabled: defaultValues.runpod?.enabled ?? false,
      runpodIdleTimeoutSeconds:
        defaultValues.runpod?.idleTimeoutSeconds ?? 300,

      colabEnabled: defaultValues.colab?.enabled ?? false,
      colabIdleTimeoutSeconds: defaultValues.colab?.idleTimeoutSeconds ?? 300,

      kaggleEnabled: defaultValues.kaggle?.enabled ?? false,
      kaggleIdleTimeoutSeconds:
        defaultValues.kaggle?.idleTimeoutSeconds ?? 300,
    },
  })

  const { isDirty, isSubmitting } = form.formState
  const ionetEnabled = form.watch('ionetEnabled')
  const modalEnabled = form.watch('modalEnabled')
  const hfEnabled = form.watch('hfEnabled')
  const gpuRows = form.watch('fineTuningGpuRows') ?? DEFAULT_FT_GPU_ROWS

  const [accountsByProvider, setAccountsByProvider] = useState<
    Record<string, DeploymentAccountSummary[]>
  >({})

  useEffect(() => {
    void getDeploymentAccounts()
      .then((res) => {
        if (res?.success && res.data) {
          setAccountsByProvider(res.data)
        }
      })
      .catch(() => {})
  }, [])

  const handleAccountsUpdated = (
    p: DeploymentProviderId,
    next: DeploymentAccountSummary[]
  ) => {
    setAccountsByProvider((prev) => ({ ...prev, [p]: next }))
  }

  async function onSubmit(values: Values) {
    try {
      await updateOption.mutateAsync({
        key: 'model_deployment.priority',
        value: String(values.priority || 'deployments_first'),
      })
      await updateOption.mutateAsync({
        key: 'model_deployment.fine_tuning.enabled',
        value: String(Boolean(values.fineTuningEnabled)),
      })
      await updateOption.mutateAsync({
        key: 'model_deployment.fine_tuning.gpu_multipliers',
        value: JSON.stringify(values.fineTuningGpuRows ?? DEFAULT_FT_GPU_ROWS),
      })

      await updateOption.mutateAsync({
        key: 'model_deployment.ionet.enabled',
        value: String(values.ionetEnabled),
      })
      await updateOption.mutateAsync({
        key: 'model_deployment.ionet.idle_timeout_seconds',
        value: String(values.ionetIdleTimeoutSeconds ?? 0),
      })

      await updateOption.mutateAsync({
        key: 'model_deployment.modal.enabled',
        value: String(values.modalEnabled),
      })
      await updateOption.mutateAsync({
        key: 'model_deployment.modal.shared_volume_path',
        value: String(values.modalSharedVolumePath ?? '/vol/models'),
      })
      await updateOption.mutateAsync({
        key: 'model_deployment.modal.idle_timeout_seconds',
        value: String(values.modalIdleTimeoutSeconds ?? 0),
      })

      await updateOption.mutateAsync({
        key: 'model_deployment.huggingface.enabled',
        value: String(values.hfEnabled),
      })
      await updateOption.mutateAsync({
        key: 'model_deployment.huggingface.idle_timeout_seconds',
        value: String(values.hfIdleTimeoutSeconds ?? 0),
      })

      await updateOption.mutateAsync({
        key: 'model_deployment.hostinger.enabled',
        value: String(values.hostingerEnabled),
      })
      await updateOption.mutateAsync({
        key: 'model_deployment.hostinger.idle_timeout_seconds',
        value: String(values.hostingerIdleTimeoutSeconds ?? 0),
      })

      await updateOption.mutateAsync({
        key: 'model_deployment.runpod.enabled',
        value: String(values.runpodEnabled),
      })
      await updateOption.mutateAsync({
        key: 'model_deployment.runpod.idle_timeout_seconds',
        value: String(values.runpodIdleTimeoutSeconds ?? 0),
      })

      await updateOption.mutateAsync({
        key: 'model_deployment.colab.enabled',
        value: String(values.colabEnabled),
      })
      await updateOption.mutateAsync({
        key: 'model_deployment.colab.idle_timeout_seconds',
        value: String(values.colabIdleTimeoutSeconds ?? 0),
      })

      await updateOption.mutateAsync({
        key: 'model_deployment.kaggle.enabled',
        value: String(values.kaggleEnabled),
      })
      await updateOption.mutateAsync({
        key: 'model_deployment.kaggle.idle_timeout_seconds',
        value: String(values.kaggleIdleTimeoutSeconds ?? 0),
      })

      form.reset(values)
      toast.success(t('Model Deployment settings saved successfully'))
    } catch {
      toast.error(t('Failed to save Model Deployment settings'))
    }
  }

  return (
    <SettingsSection title={t('Model Deployment Settings')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          {/* Global Dispatch Priority */}
          <FormField
            control={form.control}
            name='priority'
            render={({ field }) => (
              <FormItem data-settings-form-span='full'>
                <FormLabel>{t('Dispatch Priority')}</FormLabel>
                <Select
                  value={field.value ?? 'deployments_first'}
                  onValueChange={field.onChange}
                >
                  <FormControl>
                    <SelectTrigger className='w-full'>
                      <SelectValue>
                        {(field.value ?? 'deployments_first') === 'channels_first'
                          ? t(
                              'Channels First (Prefer configured channels, fallback to deployments)'
                            )
                          : t(
                              'Deployments First (Prefer deployments, fallback to channels)'
                            )}
                      </SelectValue>
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent alignItemWithTrigger={false} align='start'>
                    <SelectItem value='deployments_first'>
                      {t(
                        'Deployments First (Prefer deployments, fallback to channels)'
                      )}
                    </SelectItem>
                    <SelectItem value='channels_first'>
                      {t(
                        'Channels First (Prefer configured channels, fallback to deployments)'
                      )}
                    </SelectItem>
                  </SelectContent>
                </Select>
                <FormDescription>
                  {t(
                    'Global routing preference between Model Deployments and upstream API channels.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='fineTuningEnabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Fine-tuning')}</FormLabel>
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

          {form.watch('fineTuningEnabled') && (
            <FormItem data-settings-form-span='full' className='space-y-2'>
              <FormLabel>{t('Fine-Tuning GPU Pricing')}</FormLabel>
              <FormDescription>
                {t(
                  'Configure per-second USD pricing and toggle availability for each accelerator.'
                )}
              </FormDescription>
              <div className='overflow-x-auto rounded-lg border'>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('GPU')}</TableHead>
                      <TableHead className='w-[200px]'>
                        {t('Price per Second (USD/s)')}
                      </TableHead>
                      <TableHead className='w-[100px] text-right'>
                        {t('Enabled')}
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {gpuRows.map((row, idx) => (
                      <TableRow
                        key={row.id}
                        className={!row.enabled ? 'opacity-55' : undefined}
                      >
                        <TableCell>
                          <div className='text-sm font-medium'>{row.label}</div>
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
                                const next = [...gpuRows]
                                next[idx] = {
                                  ...next[idx],
                                  price_per_second:
                                    e.target.value === ''
                                      ? 0
                                      : Math.max(0, Number(e.target.value)),
                                }
                                form.setValue('fineTuningGpuRows', next, {
                                  shouldDirty: true,
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
                              const next = [...gpuRows]
                              next[idx] = { ...next[idx], enabled: checked }
                              form.setValue('fineTuningGpuRows', next, {
                                shouldDirty: true,
                              })
                            }}
                          />
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            </FormItem>
          )}

          <Separator data-settings-form-span='full' />

          {/* io.net */}
          <Card className='border border-border/70 shadow-sm'>
            <CardHeader className='pb-3'>
              <div className='flex items-center justify-between'>
                <div>
                  <CardTitle className='text-base font-semibold'>
                    io.net Cloud
                  </CardTitle>
                  <CardDescription className='text-xs mt-1'>
                    {t(
                      'Deploy serverless GPU clusters and containers on io.net decentralized compute network.'
                    )}
                  </CardDescription>
                </div>
                <FormField
                  control={form.control}
                  name='ionetEnabled'
                  render={({ field }) => (
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                  )}
                />
              </div>
            </CardHeader>
            {ionetEnabled && (
              <CardContent className='pt-0 space-y-4'>
                <IdleSleepField form={form} name='ionetIdleTimeoutSeconds' />
                <ProviderAccountsManager
                  provider='ionet'
                  initialAccounts={accountsByProvider.ionet ?? []}
                  onAccountsUpdated={handleAccountsUpdated}
                />
              </CardContent>
            )}
          </Card>

          {/* Modal */}
          <Card className='border border-border/70 shadow-sm'>
            <CardHeader className='pb-3'>
              <div className='flex items-center justify-between'>
                <div>
                  <CardTitle className='text-base font-semibold'>
                    Modal Deployment
                  </CardTitle>
                  <CardDescription className='text-xs mt-1'>
                    {t(
                      'Serverless AI deployment on Modal with GPU auto-scaling, persistent cache, and idle sleep.'
                    )}
                  </CardDescription>
                </div>
                <FormField
                  control={form.control}
                  name='modalEnabled'
                  render={({ field }) => (
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                  )}
                />
              </div>
            </CardHeader>
            {modalEnabled && (
              <CardContent className='pt-0 space-y-4'>
                <FormField
                  control={form.control}
                  name='modalSharedVolumePath'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Shared Volume Path')}</FormLabel>
                      <FormControl>
                        <Input
                          placeholder='/vol/models'
                          {...field}
                          value={field.value ?? ''}
                        />
                      </FormControl>
                      <FormDescription className='text-xs'>
                        {t(
                          'Modal persistent volume for weights (saves bandwidth and GPU cost)'
                        )}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <IdleSleepField form={form} name='modalIdleTimeoutSeconds' />
                <ProviderAccountsManager
                  provider='modal'
                  initialAccounts={accountsByProvider.modal ?? []}
                  onAccountsUpdated={handleAccountsUpdated}
                />
              </CardContent>
            )}
          </Card>

          {/* Hugging Face Spaces */}
          <Card className='border border-border/70 shadow-sm'>
            <CardHeader className='pb-3'>
              <div className='flex items-center justify-between'>
                <div>
                  <CardTitle className='text-base font-semibold'>
                    Hugging Face Spaces
                  </CardTitle>
                  <CardDescription className='text-xs mt-1'>
                    {t(
                      'Deploy models directly on Hugging Face Spaces with hardware acceleration.'
                    )}
                  </CardDescription>
                </div>
                <FormField
                  control={form.control}
                  name='hfEnabled'
                  render={({ field }) => (
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                  )}
                />
              </div>
            </CardHeader>
            {hfEnabled && (
              <CardContent className='pt-0 space-y-4'>
                <IdleSleepField form={form} name='hfIdleTimeoutSeconds' />
                <ProviderAccountsManager
                  provider='huggingface'
                  initialAccounts={accountsByProvider.huggingface ?? []}
                  onAccountsUpdated={handleAccountsUpdated}
                />
              </CardContent>
            )}
          </Card>

          {/* Hostinger GPU VPS */}
          <Card className='border border-border/70 shadow-sm'>
            <CardHeader className='pb-3'>
              <div className='flex items-center justify-between'>
                <div>
                  <CardTitle className='text-base font-semibold'>
                    {t('Hostinger GPU Hosting')}
                  </CardTitle>
                  <CardDescription className='text-xs mt-1'>
                    {t(
                      'Deploy and run models on Hostinger on-demand NVIDIA GPU servers with hourly billing.'
                    )}
                  </CardDescription>
                </div>
                <FormField
                  control={form.control}
                  name='hostingerEnabled'
                  render={({ field }) => (
                    <FormItem>
                      <FormControl>
                        <Switch
                          checked={field.value}
                          onCheckedChange={field.onChange}
                        />
                      </FormControl>
                    </FormItem>
                  )}
                />
              </div>
            </CardHeader>

            {form.watch('hostingerEnabled') && (
              <CardContent className='space-y-4 pt-0'>
                <IdleSleepField
                  form={form}
                  name='hostingerIdleTimeoutSeconds'
                />
                <ProviderAccountsManager
                  provider='hostinger'
                  initialAccounts={accountsByProvider.hostinger ?? []}
                  onAccountsUpdated={handleAccountsUpdated}
                />
              </CardContent>
            )}
          </Card>

          {/* RunPod GPU Cloud */}
          <Card className='border border-border/70 shadow-sm'>
            <CardHeader className='pb-3'>
              <div className='flex items-center justify-between'>
                <div>
                  <CardTitle className='text-base font-semibold'>
                    RunPod GPU Cloud
                  </CardTitle>
                  <CardDescription className='text-xs mt-1'>
                    {t(
                      'Deploy AI models to RunPod serverless and dedicated GPU infrastructure (RTX 4090, A100, H100).'
                    )}
                  </CardDescription>
                </div>
                <FormField
                  control={form.control}
                  name='runpodEnabled'
                  render={({ field }) => (
                    <FormItem>
                      <FormControl>
                        <Switch
                          checked={field.value}
                          onCheckedChange={field.onChange}
                        />
                      </FormControl>
                    </FormItem>
                  )}
                />
              </div>
            </CardHeader>

            {form.watch('runpodEnabled') && (
              <CardContent className='space-y-4 pt-0'>
                <IdleSleepField form={form} name='runpodIdleTimeoutSeconds' />
                <ProviderAccountsManager
                  provider='runpod'
                  initialAccounts={accountsByProvider.runpod ?? []}
                  onAccountsUpdated={handleAccountsUpdated}
                />
              </CardContent>
            )}
          </Card>

          {/* Google Colab */}
          <Card className='border border-border/70 shadow-sm'>
            <CardHeader className='pb-3'>
              <div className='flex items-center justify-between'>
                <div>
                  <CardTitle className='text-base font-semibold'>
                    Google Colab
                  </CardTitle>
                  <CardDescription className='text-xs mt-1'>
                    {t(
                      'Provision Google Colab CPU, GPU, and TPU runtimes via Google OAuth with automatic token refresh.'
                    )}
                  </CardDescription>
                </div>
                <FormField
                  control={form.control}
                  name='colabEnabled'
                  render={({ field }) => (
                    <FormItem>
                      <FormControl>
                        <Switch
                          checked={field.value}
                          onCheckedChange={field.onChange}
                        />
                      </FormControl>
                    </FormItem>
                  )}
                />
              </div>
            </CardHeader>

            {form.watch('colabEnabled') && (
              <CardContent className='pt-0 space-y-4'>
                <IdleSleepField form={form} name='colabIdleTimeoutSeconds' />
                <ProviderAccountsManager
                  provider='colab'
                  initialAccounts={accountsByProvider.colab ?? []}
                  onAccountsUpdated={handleAccountsUpdated}
                />
              </CardContent>
            )}
          </Card>

          {/* Kaggle */}
          <Card className='border border-border/70 shadow-sm'>
            <CardHeader className='pb-3'>
              <div className='flex items-center justify-between'>
                <div>
                  <CardTitle className='text-base font-semibold'>
                    Kaggle
                  </CardTitle>
                  <CardDescription className='text-xs mt-1'>
                    {t(
                      'Deploy and run Hugging Face models on Kaggle Kernels with CPU, GPU (T4 x2, P100, L4), or TPU (v3-8, v5e-8, v6e-8).'
                    )}
                  </CardDescription>
                </div>
                <FormField
                  control={form.control}
                  name='kaggleEnabled'
                  render={({ field }) => (
                    <FormItem>
                      <FormControl>
                        <Switch
                          checked={field.value}
                          onCheckedChange={field.onChange}
                        />
                      </FormControl>
                    </FormItem>
                  )}
                />
              </div>
            </CardHeader>

            {form.watch('kaggleEnabled') && (
              <CardContent className='pt-0 space-y-4'>
                <IdleSleepField form={form} name='kaggleIdleTimeoutSeconds' />
                <ProviderAccountsManager
                  provider='kaggle'
                  initialAccounts={accountsByProvider.kaggle ?? []}
                  onAccountsUpdated={handleAccountsUpdated}
                />
              </CardContent>
            )}
          </Card>

          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={isSubmitting || updateOption.isPending}
            isSaveDisabled={!isDirty}
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
