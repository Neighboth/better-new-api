import { zodResolver } from '@hookform/resolvers/zod'
import { CheckCircle2, Loader2, Server, XCircle } from 'lucide-react'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
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
import { api } from '@/lib/api'
import { testDeploymentConnectionWithKey } from '@/features/models/api'

import {
  SettingsForm,
  SettingsFormGrid,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const schema = z.object({
  // io.net
  ionetEnabled: z.boolean(),
  ionetApiKey: z.string().optional(),

  // Modal
  modalEnabled: z.boolean(),
  modalTokenId: z.string().optional(),
  modalTokenSecret: z.string().optional(),
  modalWorkspace: z.string().optional(),
  modalSharedVolumePath: z.string().optional(),
  modalIdleTimeoutSeconds: z.number().min(0).optional(),
  modalPriority: z.string().optional(),

  // Hugging Face Spaces
  hfEnabled: z.boolean(),
  hfToken: z.string().optional(),
  hfSpaceOrg: z.string().optional(),
  hfHardware: z.string().optional(),
  hfPriority: z.string().optional(),
})

type Values = z.infer<typeof schema>

export function UnifiedDeploymentSettingsSection({
  defaultValues,
}: {
  defaultValues: {
    ionet?: {
      enabled?: boolean
      apiKey?: string
    }
    modal?: {
      enabled?: boolean
      tokenId?: string
      tokenSecret?: string
      workspace?: string
      sharedVolumePath?: string
      idleTimeoutSeconds?: number
      priority?: string
    }
    huggingface?: {
      enabled?: boolean
      token?: string
      org?: string
      hardware?: string
      priority?: string
    }
  }
}) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: {
      ionetEnabled: defaultValues.ionet?.enabled ?? false,
      ionetApiKey: defaultValues.ionet?.apiKey ?? '',

      modalEnabled: defaultValues.modal?.enabled ?? false,
      modalTokenId: defaultValues.modal?.tokenId ?? '',
      modalTokenSecret: defaultValues.modal?.tokenSecret ?? '',
      modalWorkspace: defaultValues.modal?.workspace ?? '',
      modalSharedVolumePath:
        defaultValues.modal?.sharedVolumePath ?? '/vol/models',
      modalIdleTimeoutSeconds: defaultValues.modal?.idleTimeoutSeconds ?? 300,
      modalPriority: defaultValues.modal?.priority ?? 'modal_first',

      hfEnabled: defaultValues.huggingface?.enabled ?? false,
      hfToken: defaultValues.huggingface?.token ?? '',
      hfSpaceOrg: defaultValues.huggingface?.org ?? '',
      hfHardware: defaultValues.huggingface?.hardware ?? 't4-small',
      hfPriority: defaultValues.huggingface?.priority ?? 'hf_first',
    },
  })

  const { isDirty, isSubmitting } = form.formState
  const ionetEnabled = form.watch('ionetEnabled')
  const modalEnabled = form.watch('modalEnabled')
  const hfEnabled = form.watch('hfEnabled')

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

  const handleTestIoConnection = async () => {
    const key = form.getValues('ionetApiKey')
    if (!key) {
      toast.error(t('Please enter an io.net API key first'))
      return
    }
    setIoTestState({ loading: true, ok: null, error: null })
    try {
      const res = await testDeploymentConnectionWithKey(key)
      if (res.success) {
        setIoTestState({ loading: false, ok: true, error: null })
        toast.success(t('Successfully connected to io.net'))
      } else {
        setIoTestState({
          loading: false,
          ok: false,
          error: res.message || t('Connection failed'),
        })
        toast.error(res.message || t('Connection failed'))
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
      if (res.success) {
        setHfTestState({
          loading: false,
          ok: true,
          error: null,
          username: res.data?.name || null,
        })
        toast.success(
          t('Connected to Hugging Face successfully: {{name}}', {
            name: res.data?.name || '',
          })
        )
      } else {
        setHfTestState({
          loading: false,
          ok: false,
          error: res.message || t('Connection failed'),
          username: null,
        })
        toast.error(res.message || t('Connection failed'))
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

  async function onSubmit(values: Values) {
    try {
      // io.net options
      await updateOption.mutateAsync({
        key: 'model_deployment.ionet.enabled',
        value: String(values.ionetEnabled),
      })
      await updateOption.mutateAsync({
        key: 'model_deployment.ionet.api_key',
        value: String(values.ionetApiKey ?? ''),
      })

      // Modal options
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
        key: 'model_deployment.modal.workspace',
        value: String(values.modalWorkspace ?? ''),
      })
      await updateOption.mutateAsync({
        key: 'model_deployment.modal.shared_volume_path',
        value: String(values.modalSharedVolumePath ?? '/vol/models'),
      })
      await updateOption.mutateAsync({
        key: 'model_deployment.modal.idle_timeout_seconds',
        value: String(values.modalIdleTimeoutSeconds ?? 300),
      })
      await updateOption.mutateAsync({
        key: 'model_deployment.modal.priority',
        value: String(values.modalPriority ?? 'modal_first'),
      })

      // Hugging Face options
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
        key: 'model_deployment.huggingface.hardware',
        value: String(values.hfHardware ?? 't4-small'),
      })
      await updateOption.mutateAsync({
        key: 'model_deployment.huggingface.priority',
        value: String(values.hfPriority ?? 'hf_first'),
      })

      form.reset(values)
      toast.success(t('Model Deployment settings saved successfully'))
    } catch {
      toast.error(t('Failed to save Model Deployment settings'))
    }
  }

  return (
    <SettingsSection
      title={t('Model Deployment Settings')}
      description={t(
        'Configure serverless GPU and container deployment providers (io.net, Modal, Hugging Face Spaces) for hosting model instances.'
      )}
    >
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          {/* Provider 1: io.net */}
          <Card className='border border-border/70 shadow-sm'>
            <CardHeader className='pb-3'>
              <div className='flex items-center justify-between'>
                <div className='flex items-center gap-2'>
                  <Server className='h-5 w-5 text-primary' />
                  <CardTitle className='text-base font-semibold'>io.net Cloud</CardTitle>
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
              <CardDescription className='text-xs'>
                {t('Deploy serverless GPU clusters and containers on io.net decentralized compute network.')}
              </CardDescription>
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
              </CardContent>
            )}
          </Card>

          {/* Provider 2: Modal */}
          <Card className='border border-border/70 shadow-sm'>
            <CardHeader className='pb-3'>
              <div className='flex items-center justify-between'>
                <div className='flex items-center gap-2'>
                  <Server className='h-5 w-5 text-indigo-500' />
                  <CardTitle className='text-base font-semibold'>Modal Deployment</CardTitle>
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
              <CardDescription className='text-xs'>
                {t('Serverless AI deployment on Modal with GPU auto-scaling, persistent cache, and idle sleep.')}
              </CardDescription>
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

                <SettingsFormGrid>
                  <FormField
                    control={form.control}
                    name='modalWorkspace'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Workspace')}</FormLabel>
                        <FormControl>
                          <Input
                            placeholder='my-org'
                            {...field}
                            value={field.value ?? ''}
                          />
                        </FormControl>
                        <FormDescription className='text-xs'>
                          {t('Modal workspace name')}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

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
                          {t('Modal persistent volume for weights (saves bandwidth and GPU cost)')}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </SettingsFormGrid>

                <SettingsFormGrid>
                  <FormField
                    control={form.control}
                    name='modalIdleTimeoutSeconds'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Idle Sleep Timeout (Seconds)')}</FormLabel>
                        <FormControl>
                          <Input
                            type='number'
                            min={0}
                            placeholder='300'
                            value={field.value ?? 300}
                            onChange={(e) =>
                              field.onChange(
                                Number.parseInt(e.target.value, 10) || 0
                              )
                            }
                          />
                        </FormControl>
                        <FormDescription className='text-xs'>
                          {t('Automatically scale down / sleep instances when inactive')}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name='modalPriority'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Dispatch Priority')}</FormLabel>
                        <Select
                          value={field.value ?? 'modal_first'}
                          onValueChange={field.onChange}
                        >
                          <FormControl>
                            <SelectTrigger>
                              <SelectValue />
                            </SelectTrigger>
                          </FormControl>
                          <SelectContent>
                            <SelectItem value='modal_first'>
                              {t('Modal First (Prefer Modal, fallback to channels)')}
                            </SelectItem>
                            <SelectItem value='channels_first'>
                              {t('Channels First (Prefer configured channels, fallback to Modal)')}
                            </SelectItem>
                          </SelectContent>
                        </Select>
                        <FormDescription className='text-xs'>
                          {t('Routing preference between Modal and external API channels')}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </SettingsFormGrid>
              </CardContent>
            )}
          </Card>

          {/* Provider 3: Hugging Face Spaces */}
          <Card className='border border-border/70 shadow-sm'>
            <CardHeader className='pb-3'>
              <div className='flex items-center justify-between'>
                <div className='flex items-center gap-2'>
                  <Server className='h-5 w-5 text-amber-500' />
                  <CardTitle className='text-base font-semibold'>Hugging Face Spaces</CardTitle>
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
              <CardDescription className='text-xs'>
                {t('Deploy models directly on Hugging Face Spaces with hardware acceleration.')}
              </CardDescription>
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
                          {t('Target Organization or Username for Spaces creation')}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </SettingsFormGrid>

                <SettingsFormGrid>
                  <FormField
                    control={form.control}
                    name='hfHardware'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Default Hardware Tier')}</FormLabel>
                        <Select
                          value={field.value ?? 't4-small'}
                          onValueChange={field.onChange}
                        >
                          <FormControl>
                            <SelectTrigger>
                              <SelectValue />
                            </SelectTrigger>
                          </FormControl>
                          <SelectContent>
                            <SelectItem value='cpu-basic'>CPU Basic (Free)</SelectItem>
                            <SelectItem value='t4-small'>Nvidia T4 (Small)</SelectItem>
                            <SelectItem value='a10g-small'>Nvidia A10G (Small)</SelectItem>
                            <SelectItem value='a100-large'>Nvidia A100 (Large)</SelectItem>
                          </SelectContent>
                        </Select>
                        <FormDescription className='text-xs'>
                          {t('Compute tier to request for new Spaces')}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name='hfPriority'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Dispatch Priority')}</FormLabel>
                        <Select
                          value={field.value ?? 'hf_first'}
                          onValueChange={field.onChange}
                        >
                          <FormControl>
                            <SelectTrigger>
                              <SelectValue />
                            </SelectTrigger>
                          </FormControl>
                          <SelectContent>
                            <SelectItem value='hf_first'>
                              {t('Hugging Face First')}
                            </SelectItem>
                            <SelectItem value='channels_first'>
                              {t('Channels First')}
                            </SelectItem>
                          </SelectContent>
                        </Select>
                        <FormDescription className='text-xs'>
                          {t('Routing preference between Hugging Face and external API channels')}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </SettingsFormGrid>
              </CardContent>
            )}
          </Card>

          {/* Unified Single Save Button */}
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
