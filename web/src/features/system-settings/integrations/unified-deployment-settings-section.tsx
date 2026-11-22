import { zodResolver } from '@hookform/resolvers/zod'
import { CheckCircle2, Loader2, LogOut, Wand2, XCircle } from 'lucide-react'
import { useState } from 'react'
import { useForm, type UseFormReturn } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

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
import { Switch } from '@/components/ui/switch'
import { testDeploymentConnectionWithKey } from '@/features/models/api'
import { api } from '@/lib/api'

import { SettingsForm, SettingsFormGrid } from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const schema = z.object({
  // Global dispatch priority
  priority: z.string().optional(),

  // io.net
  ionetEnabled: z.boolean(),
  ionetApiKey: z.string().optional(),
  ionetIdleTimeoutSeconds: z.number().min(0).optional(),

  // Modal
  modalEnabled: z.boolean(),
  modalTokenId: z.string().optional(),
  modalTokenSecret: z.string().optional(),
  modalSharedVolumePath: z.string().optional(),
  modalIdleTimeoutSeconds: z.number().min(0).optional(),

  // Hugging Face Spaces
  hfEnabled: z.boolean(),
  hfToken: z.string().optional(),
  hfSpaceOrg: z.string().optional(),
  hfIdleTimeoutSeconds: z.number().min(0).optional(),

  // Hostinger GPU VPS
  hostingerEnabled: z.boolean(),
  hostingerApiKey: z.string().optional(),
  hostingerIdleTimeoutSeconds: z.number().min(0).optional(),

  // RunPod GPU Cloud
  runpodEnabled: z.boolean(),
  runpodApiKey: z.string().optional(),
  runpodIdleTimeoutSeconds: z.number().min(0).optional(),

  // Google Colab
  colabEnabled: z.boolean(),
  colabProjectId: z.string().optional(),
  colabRegion: z.string().optional(),
  colabIdleTimeoutSeconds: z.number().min(0).optional(),

  // Kaggle
  kaggleEnabled: z.boolean(),
  kaggleUsername: z.string().optional(),
  kaggleApiKey: z.string().optional(),
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
                    field.onChange(Number.isFinite(parsed) && parsed > 0 ? parsed : 1)
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

function localizeHostingerError(
  t: (key: string, options?: Record<string, unknown>) => string,
  payload:
    | { code?: string; status?: number; error?: string; message?: string }
    | undefined
): string {
  const code = payload?.code ?? ''
  const status = payload?.status
  switch (code) {
    case 'hostinger_invalid_token':
      return t(
        'Invalid Hostinger API token (HTTP {{status}}). Create a token in hPanel > Account > API and make sure it has access to GPU/VPS.',
        { status: status ?? 401 }
      )
    case 'hostinger_rejected':
      return t('Hostinger rejected the token (HTTP {{status}}).', {
        status: status ?? 0,
      })
    case 'hostinger_unreachable':
      return t('Failed to connect to Hostinger: {{error}}', {
        error: payload?.error ?? '',
      })
    case 'hostinger_unexpected_response':
      return t('Unexpected response from the Hostinger API (HTTP {{status}})', {
        status: status ?? 0,
      })
    default:
      return payload?.message || t('Connection failed')
  }
}

export function UnifiedDeploymentSettingsSection({
  defaultValues,
}: {
  defaultValues: {
    priority?: string
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

      ionetEnabled: defaultValues.ionet?.enabled ?? false,
      ionetApiKey: defaultValues.ionet?.apiKey ?? '',
      ionetIdleTimeoutSeconds: defaultValues.ionet?.idleTimeoutSeconds ?? 300,

      modalEnabled: defaultValues.modal?.enabled ?? false,
      modalTokenId: defaultValues.modal?.tokenId ?? '',
      modalTokenSecret: defaultValues.modal?.tokenSecret ?? '',
      modalSharedVolumePath:
        defaultValues.modal?.sharedVolumePath ?? '/vol/models',
      modalIdleTimeoutSeconds: defaultValues.modal?.idleTimeoutSeconds ?? 300,

      hfEnabled: defaultValues.huggingface?.enabled ?? false,
      hfToken: defaultValues.huggingface?.token ?? '',
      hfSpaceOrg: defaultValues.huggingface?.org ?? '',
      hfIdleTimeoutSeconds: defaultValues.huggingface?.idleTimeoutSeconds ?? 300,

      hostingerEnabled: defaultValues.hostinger?.enabled ?? false,
      hostingerApiKey: defaultValues.hostinger?.apiKey ?? '',
      hostingerIdleTimeoutSeconds:
        defaultValues.hostinger?.idleTimeoutSeconds ?? 300,

      runpodEnabled: defaultValues.runpod?.enabled ?? false,
      runpodApiKey: defaultValues.runpod?.apiKey ?? '',
      runpodIdleTimeoutSeconds:
        defaultValues.runpod?.idleTimeoutSeconds ?? 300,

      colabEnabled: defaultValues.colab?.enabled ?? false,
      colabProjectId: defaultValues.colab?.projectId ?? '',
      colabRegion: defaultValues.colab?.region ?? 'us-central1',
      colabIdleTimeoutSeconds: defaultValues.colab?.idleTimeoutSeconds ?? 300,

      kaggleEnabled: defaultValues.kaggle?.enabled ?? false,
      kaggleUsername: defaultValues.kaggle?.username ?? '',
      kaggleApiKey: defaultValues.kaggle?.apiKey ?? '',
      kaggleIdleTimeoutSeconds:
        defaultValues.kaggle?.idleTimeoutSeconds ?? 300,
    },
  })

  const { isDirty, isSubmitting } = form.formState
  const ionetEnabled = form.watch('ionetEnabled')
  const modalEnabled = form.watch('modalEnabled')
  const hfEnabled = form.watch('hfEnabled')

  // Colab OAuth state (matches AntiGravity OAuth flow)
  const [colabConnectedEmail, setColabConnectedEmail] = useState<string>(
    defaultValues.colab?.accountEmail ?? ''
  )
  const [colabConnected, setColabConnected] = useState<boolean>(
    Boolean(
      defaultValues.colab?.accessToken || defaultValues.colab?.refreshToken
    )
  )
  const [colabOAuthSessionId, setColabOAuthSessionId] = useState('')
  const [colabOAuthUrl, setColabOAuthUrl] = useState('')
  const [colabCallbackUrl, setColabCallbackUrl] = useState('')
  const [colabOAuthExchanging, setColabOAuthExchanging] = useState(false)

  const [ioTestState, setIoTestState] = useState<{
    loading: boolean
    ok: boolean | null
    error: string | null
  }>({ loading: false, ok: null, error: null })

  const [hfTestState, setHfTestState] = useState<{
    loading: boolean
    ok: boolean | null
    error: string | null
    username: string | null
  }>({ loading: false, ok: null, error: null, username: null })

  const [hostingerTestState, setHostingerTestState] = useState<{
    loading: boolean
    ok: boolean | null
    error: string | null
    catalog: string | null
    gpuGroups: number | null
  }>({
    loading: false,
    ok: null,
    error: null,
    catalog: null,
    gpuGroups: null,
  })

  const [runpodTestState, setRunpodTestState] = useState<{
    loading: boolean
    ok: boolean | null
    error: string | null
  }>({ loading: false, ok: null, error: null })

  const [colabTestState, setColabTestState] = useState<{
    loading: boolean
    ok: boolean | null
    error: string | null
  }>({ loading: false, ok: null, error: null })

  const [kaggleTestState, setKaggleTestState] = useState<{
    loading: boolean
    ok: boolean | null
    error: string | null
  }>({ loading: false, ok: null, error: null })

  const handleStartColabOAuth = async () => {
    try {
      const res = await api.post('/api/deployments/colab/oauth/start')
      if (res.data?.success && res.data?.data) {
        setColabOAuthSessionId(res.data.data.session_id)
        setColabOAuthUrl(res.data.data.auth_url)
        setColabCallbackUrl('')
        try {
          window.open(res.data.data.auth_url, '_blank', 'noopener,noreferrer')
        } catch {
          // ignore popup blocker
        }
      } else {
        toast.error(res.data?.message || t('Failed to start OAuth flow'))
      }
    } catch (e: any) {
      toast.error(e?.message || t('Failed to start OAuth flow'))
    }
  }

  const handleExchangeColabOAuth = async () => {
    if (!colabOAuthSessionId || !colabCallbackUrl.trim()) return
    setColabOAuthExchanging(true)
    try {
      const res = await api.post('/api/deployments/colab/oauth/exchange', {
        session_id: colabOAuthSessionId,
        callback: colabCallbackUrl.trim(),
      })
      if (res.data?.success) {
        setColabConnected(true)
        setColabConnectedEmail(res.data?.data?.email ?? '')
        if (res.data?.data?.project_id && !form.getValues('colabProjectId')) {
          form.setValue('colabProjectId', res.data.data.project_id)
        }
        setColabOAuthUrl('')
        setColabCallbackUrl('')
        toast.success(t('Successfully connected to Google Colab'))
      } else {
        toast.error(res.data?.message || t('Failed to exchange OAuth credentials'))
      }
    } catch (e: any) {
      toast.error(e?.message || t('Failed to exchange OAuth credentials'))
    } finally {
      setColabOAuthExchanging(false)
    }
  }

  const handleDisconnectColabOAuth = async () => {
    try {
      await api.post('/api/deployments/colab/oauth/disconnect')
      setColabConnected(false)
      setColabConnectedEmail('')
      setColabTestState({ loading: false, ok: null, error: null })
      toast.success(t('Google Colab disconnected'))
    } catch (e: any) {
      toast.error(e?.message || t('Disconnect failed'))
    }
  }

  const handleTestIoConnection = async () => {
    const key = form.getValues('ionetApiKey')
    if (!key) {
      toast.error(t('Please enter an io.net API key first'))
      return
    }
    setIoTestState({ loading: true, ok: null, error: null })
    try {
      const res: any = await testDeploymentConnectionWithKey(key)
      if (res.data?.success || res.success) {
        setIoTestState({ loading: false, ok: true, error: null })
        toast.success(t('Successfully connected to io.net'))
      } else {
        setIoTestState({
          loading: false,
          ok: false,
          error: res.data?.message || res.message || t('Connection failed'),
        })
        toast.error(res.data?.message || res.message || t('Connection failed'))
      }
    } catch (e: any) {
      setIoTestState({
        loading: false,
        ok: false,
        error: e?.message || t('Connection failed'),
      })
      toast.error(e?.message || t('Connection failed'))
    }
  }

  const handleTestHfConnection = async () => {
    const token = form.getValues('hfToken')
    if (!token) {
      toast.error(t('Please enter a Hugging Face token first'))
      return
    }
    setHfTestState({ loading: true, ok: null, error: null, username: null })
    try {
      const res = await api.post('/api/deployments/huggingface/test-connection', {
        token,
      })
      if (res.data?.success) {
        setHfTestState({
          loading: false,
          ok: true,
          error: null,
          username: res.data?.data?.name || null,
        })
        toast.success(
          t('Connected to Hugging Face successfully: {{name}}', {
            name: res.data?.data?.name || '',
          })
        )
      } else {
        setHfTestState({
          loading: false,
          ok: false,
          error: res.data?.message || t('Connection failed'),
          username: null,
        })
        toast.error(res.data?.message || t('Connection failed'))
      }
    } catch (e: any) {
      setHfTestState({
        loading: false,
        ok: false,
        error: e?.message || t('Connection failed'),
        username: null,
      })
      toast.error(e?.message || t('Connection failed'))
    }
  }

  const handleTestHostingerConnection = async () => {
    const key = form.getValues('hostingerApiKey')
    if (!key) {
      toast.error(t('Please enter a Hostinger API key first'))
      return
    }
    setHostingerTestState({
      loading: true,
      ok: null,
      error: null,
      catalog: null,
      gpuGroups: null,
    })
    try {
      const res = await api.post('/api/deployments/hostinger/test-connection', {
        api_key: key,
      })
      if (res.data?.success) {
        const payload = res.data?.data ?? {}
        setHostingerTestState({
          loading: false,
          ok: true,
          error: null,
          catalog: payload.catalog ?? null,
          gpuGroups: payload.gpu_groups ?? null,
        })
        toast.success(t('Connected to Hostinger successfully'))
      } else {
        const localized = localizeHostingerError(t, res.data)
        setHostingerTestState({
          loading: false,
          ok: false,
          error: localized,
          catalog: null,
          gpuGroups: null,
        })
        toast.error(localized)
      }
    } catch (e: any) {
      const localized = localizeHostingerError(t, e?.response?.data)
      const message = e?.response?.data?.message
        ? localized
        : e?.message || localized
      setHostingerTestState({
        loading: false,
        ok: false,
        error: message,
        catalog: null,
        gpuGroups: null,
      })
      toast.error(message)
    }
  }

  const handleTestRunPodConnection = async () => {
    const key = form.getValues('runpodApiKey')
    if (!key) {
      toast.error(t('Please enter a RunPod API key first'))
      return
    }
    setRunpodTestState({ loading: true, ok: null, error: null })
    try {
      const res = await api.post('/api/deployments/runpod/test-connection', {
        api_key: key,
      })
      if (res.data?.success) {
        setRunpodTestState({ loading: false, ok: true, error: null })
        toast.success(t('Connected to RunPod successfully'))
      } else {
        setRunpodTestState({
          loading: false,
          ok: false,
          error: res.data?.message || t('Connection failed'),
        })
        toast.error(res.data?.message || t('Connection failed'))
      }
    } catch (e: any) {
      setRunpodTestState({
        loading: false,
        ok: false,
        error: e?.message || t('Connection failed'),
      })
      toast.error(e?.message || t('Connection failed'))
    }
  }

  const handleTestColabConnection = async () => {
    setColabTestState({ loading: true, ok: null, error: null })
    try {
      const res = await api.post('/api/deployments/colab/test-connection', {})
      if (res.data?.success) {
        setColabTestState({ loading: false, ok: true, error: null })
        toast.success(t('Connected to Google Colab successfully'))
      } else {
        setColabTestState({
          loading: false,
          ok: false,
          error: res.data?.message || t('Connection failed'),
        })
        toast.error(res.data?.message || t('Connection failed'))
      }
    } catch (e: any) {
      setColabTestState({
        loading: false,
        ok: false,
        error: e?.message || t('Connection failed'),
      })
      toast.error(e?.message || t('Connection failed'))
    }
  }

  const handleTestKaggleConnection = async () => {
    const username = form.getValues('kaggleUsername')
    const apiKey = form.getValues('kaggleApiKey')
    if (!apiKey) {
      toast.error(t('Please enter a Kaggle API key first'))
      return
    }
    setKaggleTestState({ loading: true, ok: null, error: null })
    try {
      const res = await api.post('/api/deployments/kaggle/test-connection', {
        username,
        api_key: apiKey,
      })
      if (res.data?.success) {
        setKaggleTestState({ loading: false, ok: true, error: null })
        toast.success(t('Connected to Kaggle successfully'))
      } else {
        setKaggleTestState({
          loading: false,
          ok: false,
          error: res.data?.message || t('Connection failed'),
        })
        toast.error(res.data?.message || t('Connection failed'))
      }
    } catch (e: any) {
      setKaggleTestState({
        loading: false,
        ok: false,
        error: e?.message || t('Connection failed'),
      })
      toast.error(e?.message || t('Connection failed'))
    }
  }

  async function onSubmit(values: Values) {
    try {
      await updateOption.mutateAsync({
        key: 'model_deployment.priority',
        value: String(values.priority || 'deployments_first'),
      })

      // io.net
      await updateOption.mutateAsync({
        key: 'model_deployment.ionet.enabled',
        value: String(values.ionetEnabled),
      })
      await updateOption.mutateAsync({
        key: 'model_deployment.ionet.api_key',
        value: String(values.ionetApiKey ?? ''),
      })
      await updateOption.mutateAsync({
        key: 'model_deployment.ionet.idle_timeout_seconds',
        value: String(values.ionetIdleTimeoutSeconds ?? 0),
      })

      // Modal
      await updateOption.mutateAsync({
        key: 'model_deployment.modal.enabled',
        value: String(values.modalEnabled),
      })
      await updateOption.mutateAsync({
        key: 'model_deployment.modal.token_id',
        value: String(values.modalTokenId ?? ''),
      })
      await updateOption.mutateAsync({
        key: 'model_deployment.modal.token_secret',
        value: String(values.modalTokenSecret ?? ''),
      })
      await updateOption.mutateAsync({
        key: 'model_deployment.modal.shared_volume_path',
        value: String(values.modalSharedVolumePath ?? '/vol/models'),
      })
      await updateOption.mutateAsync({
        key: 'model_deployment.modal.idle_timeout_seconds',
        value: String(values.modalIdleTimeoutSeconds ?? 0),
      })

      // Hugging Face Spaces
      await updateOption.mutateAsync({
        key: 'model_deployment.huggingface.enabled',
        value: String(values.hfEnabled),
      })
      await updateOption.mutateAsync({
        key: 'model_deployment.huggingface.token',
        value: String(values.hfToken ?? ''),
      })
      await updateOption.mutateAsync({
        key: 'model_deployment.huggingface.api_key',
        value: String(values.hfToken ?? ''),
      })
      await updateOption.mutateAsync({
        key: 'model_deployment.huggingface.org',
        value: String(values.hfSpaceOrg ?? ''),
      })
      await updateOption.mutateAsync({
        key: 'model_deployment.huggingface.idle_timeout_seconds',
        value: String(values.hfIdleTimeoutSeconds ?? 0),
      })

      // Hostinger
      await updateOption.mutateAsync({
        key: 'model_deployment.hostinger.enabled',
        value: String(values.hostingerEnabled),
      })
      await updateOption.mutateAsync({
        key: 'model_deployment.hostinger.api_key',
        value: String(values.hostingerApiKey ?? ''),
      })
      await updateOption.mutateAsync({
        key: 'model_deployment.hostinger.idle_timeout_seconds',
        value: String(values.hostingerIdleTimeoutSeconds ?? 0),
      })

      // RunPod
      await updateOption.mutateAsync({
        key: 'model_deployment.runpod.enabled',
        value: String(values.runpodEnabled),
      })
      await updateOption.mutateAsync({
        key: 'model_deployment.runpod.api_key',
        value: String(values.runpodApiKey ?? ''),
      })
      await updateOption.mutateAsync({
        key: 'model_deployment.runpod.idle_timeout_seconds',
        value: String(values.runpodIdleTimeoutSeconds ?? 0),
      })

      // Google Colab
      await updateOption.mutateAsync({
        key: 'model_deployment.colab.enabled',
        value: String(values.colabEnabled),
      })
      await updateOption.mutateAsync({
        key: 'model_deployment.colab.project_id',
        value: String(values.colabProjectId ?? ''),
      })
      await updateOption.mutateAsync({
        key: 'model_deployment.colab.region',
        value: String(values.colabRegion ?? 'us-central1'),
      })
      await updateOption.mutateAsync({
        key: 'model_deployment.colab.idle_timeout_seconds',
        value: String(values.colabIdleTimeoutSeconds ?? 0),
      })

      // Kaggle
      await updateOption.mutateAsync({
        key: 'model_deployment.kaggle.enabled',
        value: String(values.kaggleEnabled),
      })
      await updateOption.mutateAsync({
        key: 'model_deployment.kaggle.username',
        value: String(values.kaggleUsername ?? ''),
      })
      await updateOption.mutateAsync({
        key: 'model_deployment.kaggle.api_key',
        value: String(values.kaggleApiKey ?? ''),
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
          {/* Global Dispatch Priority Card */}
          <Card className='border border-border/70 shadow-sm'>
            <CardHeader className='pb-3'>
              <CardTitle className='text-base font-semibold'>
                {t('Dispatch Priority')}
              </CardTitle>
              <CardDescription className='text-xs mt-1'>
                {t(
                  'Global routing preference between Model Deployments and upstream API channels.'
                )}
              </CardDescription>
            </CardHeader>
            <CardContent className='pt-0'>
              <FormField
                control={form.control}
                name='priority'
                render={({ field }) => (
                  <FormItem>
                    <Select
                      value={field.value ?? 'deployments_first'}
                      onValueChange={field.onChange}
                    >
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
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
                    <FormMessage />
                  </FormItem>
                )}
              />
            </CardContent>
          </Card>

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
                <FormField
                  control={form.control}
                  name='ionetApiKey'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('io.net API Key')}</FormLabel>
                      <div className='flex items-center gap-2'>
                        <FormControl>
                          <Input
                            type='password'
                            placeholder='io_sec_...'
                            {...field}
                            value={field.value ?? ''}
                          />
                        </FormControl>
                        <Button
                          type='button'
                          variant='outline'
                          size='sm'
                          onClick={handleTestIoConnection}
                          disabled={ioTestState.loading || !field.value}
                        >
                          {ioTestState.loading && (
                            <Loader2 className='mr-1.5 h-3.5 w-3.5 animate-spin' />
                          )}
                          {t('Test')}
                        </Button>
                      </div>
                      {ioTestState.ok && (
                        <div className='flex items-center gap-1.5 text-xs text-emerald-600 dark:text-emerald-400 font-medium'>
                          <CheckCircle2 className='h-3.5 w-3.5' />
                          {t('Connection verified')}
                        </div>
                      )}
                      {ioTestState.error && (
                        <div className='flex items-center gap-1.5 text-xs text-destructive font-medium'>
                          <XCircle className='h-3.5 w-3.5' />
                          {ioTestState.error}
                        </div>
                      )}
                      <FormDescription className='text-xs'>
                        {t('Enterprise API key from io.net console')}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <IdleSleepField form={form} name='ionetIdleTimeoutSeconds' />
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
                <SettingsFormGrid>
                  <FormField
                    control={form.control}
                    name='modalTokenId'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Token ID')}</FormLabel>
                        <FormControl>
                          <Input
                            placeholder='ak-...'
                            {...field}
                            value={field.value ?? ''}
                          />
                        </FormControl>
                        <FormDescription className='text-xs'>
                          {t('Modal API Token ID')}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name='modalTokenSecret'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Token Secret')}</FormLabel>
                        <FormControl>
                          <Input
                            type='password'
                            placeholder='as-...'
                            {...field}
                            value={field.value ?? ''}
                          />
                        </FormControl>
                        <FormDescription className='text-xs'>
                          {t('Modal API Token Secret')}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </SettingsFormGrid>

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
                <SettingsFormGrid>
                  <FormField
                    control={form.control}
                    name='hfToken'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Access Token')}</FormLabel>
                        <div className='flex items-center gap-2'>
                          <FormControl>
                            <Input
                              type='password'
                              placeholder='hf_...'
                              {...field}
                              value={field.value ?? ''}
                            />
                          </FormControl>
                          <Button
                            type='button'
                            variant='outline'
                            size='sm'
                            onClick={handleTestHfConnection}
                            disabled={hfTestState.loading || !field.value}
                          >
                            {hfTestState.loading && (
                              <Loader2 className='mr-1.5 h-3.5 w-3.5 animate-spin' />
                            )}
                            {t('Test')}
                          </Button>
                        </div>
                        {hfTestState.ok && (
                          <div className='flex items-center gap-1.5 text-xs text-emerald-600 dark:text-emerald-400 font-medium'>
                            <CheckCircle2 className='h-3.5 w-3.5' />
                            {t('Verified user: {{name}}', {
                              name: hfTestState.username || '',
                            })}
                          </div>
                        )}
                        {hfTestState.error && (
                          <div className='flex items-center gap-1.5 text-xs text-destructive font-medium'>
                            <XCircle className='h-3.5 w-3.5' />
                            {hfTestState.error}
                          </div>
                        )}
                        <FormDescription className='text-xs'>
                          {t('Hugging Face User Access Token (read/write)')}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name='hfSpaceOrg'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Organization / Username')}</FormLabel>
                        <FormControl>
                          <Input
                            placeholder='my-org'
                            {...field}
                            value={field.value ?? ''}
                          />
                        </FormControl>
                        <FormDescription className='text-xs'>
                          {t(
                            'Target Organization or Username for Spaces creation'
                          )}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </SettingsFormGrid>

                <IdleSleepField form={form} name='hfIdleTimeoutSeconds' />
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
                <FormField
                  control={form.control}
                  name='hostingerApiKey'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Hostinger API Key / Token')}</FormLabel>
                      <div className='flex gap-2'>
                        <FormControl>
                          <Input
                            type='password'
                            placeholder='hst_...'
                            {...field}
                            value={field.value ?? ''}
                          />
                        </FormControl>
                        <Button
                          type='button'
                          variant='outline'
                          size='sm'
                          className='shrink-0'
                          disabled={hostingerTestState.loading || !field.value}
                          onClick={handleTestHostingerConnection}
                        >
                          {hostingerTestState.loading && (
                            <Loader2 className='mr-1.5 h-3.5 w-3.5 animate-spin' />
                          )}
                          {t('Test Connection')}
                        </Button>
                      </div>
                      {hostingerTestState.ok === true && (
                        <p className='flex items-center gap-1 text-xs text-emerald-600 dark:text-emerald-400'>
                          <CheckCircle2 className='h-3.5 w-3.5' />
                          {hostingerTestState.catalog === 'gpu'
                            ? t(
                                'Successfully connected to Hostinger GPU Hosting ({{count}} GPU models in the catalog)',
                                { count: hostingerTestState.gpuGroups ?? 0 }
                              )
                            : t('Successfully connected to Hostinger')}
                        </p>
                      )}
                      {hostingerTestState.ok === false && (
                        <p className='flex items-center gap-1 text-xs text-destructive'>
                          <XCircle className='h-3.5 w-3.5' />
                          {hostingerTestState.error}
                        </p>
                      )}
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <IdleSleepField
                  form={form}
                  name='hostingerIdleTimeoutSeconds'
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
                <FormField
                  control={form.control}
                  name='runpodApiKey'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('RunPod API Key')}</FormLabel>
                      <div className='flex gap-2'>
                        <FormControl>
                          <Input
                            type='password'
                            placeholder='rpa_...'
                            {...field}
                            value={field.value ?? ''}
                          />
                        </FormControl>
                        <Button
                          type='button'
                          variant='outline'
                          size='sm'
                          className='shrink-0'
                          disabled={runpodTestState.loading || !field.value}
                          onClick={handleTestRunPodConnection}
                        >
                          {runpodTestState.loading && (
                            <Loader2 className='mr-1.5 h-3.5 w-3.5 animate-spin' />
                          )}
                          {t('Test Connection')}
                        </Button>
                      </div>
                      {runpodTestState.ok === true && (
                        <p className='flex items-center gap-1 text-xs text-emerald-600 dark:text-emerald-400'>
                          <CheckCircle2 className='h-3.5 w-3.5' />
                          {t('Successfully connected to RunPod')}
                        </p>
                      )}
                      {runpodTestState.ok === false && (
                        <p className='flex items-center gap-1 text-xs text-destructive'>
                          <XCircle className='h-3.5 w-3.5' />
                          {runpodTestState.error}
                        </p>
                      )}
                      <FormDescription className='text-xs'>
                        {t(
                          'Your personal RunPod API key from Settings -> API Keys.'
                        )}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <IdleSleepField form={form} name='runpodIdleTimeoutSeconds' />
              </CardContent>
            )}
          </Card>

          {/* Google Colab (OAuth Login like AntiGravity + Auto-Refresh) */}
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
                <div className='border-border/60 flex flex-col gap-3 rounded-lg border bg-muted/20 p-4'>
                  <div className='flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
                    <div className='space-y-1'>
                      <div className='flex items-center gap-2 font-semibold text-sm'>
                        <span>{t('Google Colab OAuth Integration')}</span>
                        {colabConnected && (
                          <Badge
                            variant='outline'
                            className='text-[11px] border-emerald-500/40 text-emerald-600 dark:text-emerald-400 bg-emerald-500/10'
                          >
                            {t('Connected & Auto-Refresh Active')}
                          </Badge>
                        )}
                      </div>
                      <div className='text-muted-foreground text-xs'>
                        {colabConnected
                          ? t('Signed in as {{email}}', {
                              email: colabConnectedEmail || 'Google Account',
                            })
                          : t(
                              'Sign in with Google in a new tab and paste the redirected localhost URL below.'
                            )}
                      </div>
                    </div>
                    <div className='flex flex-wrap items-center gap-2'>
                      <Button
                        type='button'
                        variant='default'
                        size='sm'
                        onClick={handleStartColabOAuth}
                      >
                        <Wand2 className='mr-2 h-4 w-4' />
                        {t('1. Start OAuth Login')}
                      </Button>
                      {colabConnected && (
                        <>
                          <Button
                            type='button'
                            variant='outline'
                            size='sm'
                            disabled={colabTestState.loading}
                            onClick={handleTestColabConnection}
                          >
                            {colabTestState.loading && (
                              <Loader2 className='mr-1.5 h-3.5 w-3.5 animate-spin' />
                            )}
                            {t('Test Connection')}
                          </Button>
                          <Button
                            type='button'
                            variant='ghost'
                            size='sm'
                            className='text-destructive hover:text-destructive'
                            onClick={handleDisconnectColabOAuth}
                          >
                            <LogOut className='mr-1.5 h-3.5 w-3.5' />
                            {t('Disconnect')}
                          </Button>
                        </>
                      )}
                    </div>
                  </div>

                  {colabOAuthUrl && (
                    <div className='flex flex-col gap-2 pt-2 border-t'>
                      <div className='text-xs font-medium text-primary'>
                        <a
                          href={colabOAuthUrl}
                          target='_blank'
                          rel='noopener noreferrer'
                          className='hover:underline'
                        >
                          {t(
                            'If the window did not open, click here to log in (New Tab)'
                          )}
                        </a>
                      </div>
                      <div className='flex items-center gap-2'>
                        <Input
                          placeholder='http://localhost:8085/callback?code=...'
                          value={colabCallbackUrl}
                          onChange={(e) => setColabCallbackUrl(e.target.value)}
                          className='text-xs'
                        />
                        <Button
                          type='button'
                          size='sm'
                          onClick={handleExchangeColabOAuth}
                          disabled={colabOAuthExchanging || !colabCallbackUrl}
                        >
                          {colabOAuthExchanging && (
                            <Loader2 className='mr-1.5 h-3.5 w-3.5 animate-spin' />
                          )}
                          {t('Verify and Get')}
                        </Button>
                      </div>
                    </div>
                  )}

                  {colabTestState.ok === true && (
                    <p className='flex items-center gap-1 text-xs text-emerald-600 dark:text-emerald-400'>
                      <CheckCircle2 className='h-3.5 w-3.5' />
                      {t('Successfully connected to Google Colab')}
                    </p>
                  )}
                  {colabTestState.ok === false && (
                    <p className='flex items-center gap-1 text-xs text-destructive'>
                      <XCircle className='h-3.5 w-3.5' />
                      {colabTestState.error}
                    </p>
                  )}
                </div>

                <SettingsFormGrid>
                  <FormField
                    control={form.control}
                    name='colabProjectId'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('GCP Project ID (Optional)')}</FormLabel>
                        <FormControl>
                          <Input
                            placeholder='my-gcp-project'
                            {...field}
                            value={field.value ?? ''}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name='colabRegion'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Region (Optional)')}</FormLabel>
                        <FormControl>
                          <Input
                            placeholder='us-central1'
                            {...field}
                            value={field.value ?? ''}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </SettingsFormGrid>

                <IdleSleepField form={form} name='colabIdleTimeoutSeconds' />
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
                <SettingsFormGrid>
                  <FormField
                    control={form.control}
                    name='kaggleUsername'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Kaggle Username')}</FormLabel>
                        <FormControl>
                          <Input
                            placeholder='my-kaggle-username'
                            {...field}
                            value={field.value ?? ''}
                          />
                        </FormControl>
                        <FormDescription className='text-xs'>
                          {t('Your Kaggle account username')}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name='kaggleApiKey'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Kaggle API Key / Token')}</FormLabel>
                        <div className='flex gap-2'>
                          <FormControl>
                            <Input
                              type='password'
                              placeholder='KGAT_... or kaggle.json key'
                              {...field}
                              value={field.value ?? ''}
                            />
                          </FormControl>
                          <Button
                            type='button'
                            variant='outline'
                            size='sm'
                            className='shrink-0'
                            disabled={kaggleTestState.loading || !field.value}
                            onClick={handleTestKaggleConnection}
                          >
                            {kaggleTestState.loading && (
                              <Loader2 className='mr-1.5 h-3.5 w-3.5 animate-spin' />
                            )}
                            {t('Test Connection')}
                          </Button>
                        </div>
                        {kaggleTestState.ok === true && (
                          <p className='flex items-center gap-1 text-xs text-emerald-600 dark:text-emerald-400'>
                            <CheckCircle2 className='h-3.5 w-3.5' />
                            {t('Successfully connected to Kaggle')}
                          </p>
                        )}
                        {kaggleTestState.ok === false && (
                          <p className='flex items-center gap-1 text-xs text-destructive'>
                            <XCircle className='h-3.5 w-3.5' />
                            {kaggleTestState.error}
                          </p>
                        )}
                        <FormDescription className='text-xs'>
                          {t(
                            'From Kaggle Account -> Settings -> API -> Create New Token'
                          )}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </SettingsFormGrid>

                <IdleSleepField form={form} name='kaggleIdleTimeoutSeconds' />
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
