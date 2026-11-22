import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Check,
  Copy,
  ExternalLink,
  Eye,
  Loader2,
  Play,
  Search,
  Trash2,
} from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  HfModelPickerDialog,
  type HfPickedModel,
} from '@/components/hf-model-picker-dialog'
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
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogFooter,
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
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { formatTimestampToDate } from '@/lib/format'

import {
  createNotebookDeployment,
  deleteNotebookDeployment,
  listNotebookDeployments,
  testNotebookDeployment,
  updateNotebookDeployment,
  type ModelTestResult,
  type NotebookDeploymentItem,
} from '../api'
import {
  PROVIDER_HARDWARE_CATALOG,
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

function parseFallbacks(raw?: string): string[] {
  if (!raw) return []
  try {
    const parsed = JSON.parse(raw)
    return Array.isArray(parsed) ? parsed.filter((x) => typeof x === 'string') : []
  } catch {
    return []
  }
}

export function UnifiedDeploymentsPanel({
  enabledProviders,
  createOpen,
  onCreateOpenChange,
}: {
  enabledProviders: DeploymentProviderId[]
  createOpen: boolean
  onCreateOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  const defaultProvider: DeploymentProviderId =
    enabledProviders[0] ?? 'modal'

  const [provider, setProvider] =
    useState<DeploymentProviderId>(defaultProvider)
  const [hfPickerOpen, setHfPickerOpen] = useState(false)
  const [name, setName] = useState('')
  const [modelRepo, setModelRepo] = useState('')
  const [modelFile, setModelFile] = useState('')
  const [taskType, setTaskType] = useState<string>('text')
  const [accelerator, setAccelerator] = useState<string>(
    PROVIDER_HARDWARE_CATALOG[defaultProvider][0]?.id ?? 'CPU'
  )
  const [fallbackAccelerators, setFallbackAccelerators] = useState<string[]>(
    []
  )
  const [endpointUrlInput, setEndpointUrlInput] = useState('')
  const [hfToken, setHfToken] = useState('')

  useEffect(() => {
    if (
      enabledProviders.length > 0 &&
      !enabledProviders.includes(provider)
    ) {
      const next = enabledProviders[0]
      setProvider(next)
      setAccelerator(PROVIDER_HARDWARE_CATALOG[next][0]?.id ?? 'CPU')
      setFallbackAccelerators([])
    }
  }, [enabledProviders, provider])

  const handleProviderChange = (next: DeploymentProviderId) => {
    setProvider(next)
    setAccelerator(PROVIDER_HARDWARE_CATALOG[next][0]?.id ?? 'CPU')
    setFallbackAccelerators([])
  }

  const toggleFallback = (id: string) => {
    if (id === accelerator) return
    setFallbackAccelerators((prev) =>
      prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]
    )
  }

  const [detailsItem, setDetailsItem] =
    useState<NotebookDeploymentItem | null>(null)
  const [editEndpointUrl, setEditEndpointUrl] = useState('')
  const [savingEndpoint, setSavingEndpoint] = useState(false)

  const [testTarget, setTestTarget] =
    useState<NotebookDeploymentItem | null>(null)
  const [testPrompt, setTestPrompt] = useState(
    'Hello! Describe yourself briefly.'
  )
  const [testResult, setTestResult] = useState<ModelTestResult | null>(null)
  const [testError, setTestError] = useState<string | null>(null)
  const [testing, setTesting] = useState(false)

  const [deleteTarget, setDeleteTarget] =
    useState<NotebookDeploymentItem | null>(null)
  const [deleting, setDeleting] = useState(false)

  const listQuery = useQuery({
    queryKey: ['notebook-deployments', 'all'],
    queryFn: () => listNotebookDeployments(),
    staleTime: 15 * 1000,
  })

  const items = listQuery.data?.data ?? []
  const hardwareOptions = PROVIDER_HARDWARE_CATALOG[provider] ?? []

  const createMutation = useMutation({
    mutationFn: () =>
      createNotebookDeployment({
        provider,
        name: name.trim(),
        model_repo: modelRepo.trim(),
        model_file: modelFile.trim() || undefined,
        task_type: taskType,
        accelerator,
        fallback_accelerators: fallbackAccelerators,
        endpoint_url: endpointUrlInput.trim() || undefined,
        hf_token: hfToken.trim() || undefined,
      }),
    onSuccess: (res) => {
      if (res?.success) {
        toast.success(t('Deployment created successfully'))
        onCreateOpenChange(false)
        setName('')
        setModelRepo('')
        setModelFile('')
        setFallbackAccelerators([])
        setEndpointUrlInput('')
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

  const handleHfPicked = (picked: HfPickedModel) => {
    setModelRepo(picked.repoId)
    setModelFile(picked.filename ?? '')
    setTaskType(picked.taskType)
    if (!name.trim()) {
      const base = picked.repoId.split('/').pop() ?? 'model'
      setName(base.toLowerCase().replace(/[^a-z0-9-]+/g, '-').slice(0, 40))
    }
  }

  const handleSaveEndpointUrl = async () => {
    if (!detailsItem) return
    setSavingEndpoint(true)
    try {
      const res = await updateNotebookDeployment(detailsItem.id, {
        endpoint_url: editEndpointUrl.trim(),
      })
      if (res?.success && res.data) {
        setDetailsItem(res.data)
        toast.success(t('Endpoint URL updated'))
        queryClient.invalidateQueries({
          queryKey: ['notebook-deployments', 'all'],
        })
      } else {
        toast.error(res?.message || t('Failed to update endpoint URL'))
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t('Failed to update'))
    } finally {
      setSavingEndpoint(false)
    }
  }

  const handleRunTest = async () => {
    if (!testTarget) return
    setTesting(true)
    setTestError(null)
    setTestResult(null)
    try {
      const res = await testNotebookDeployment(testTarget.id, {
        prompt: testPrompt.trim(),
      })
      if (res?.success && res.data) {
        setTestResult(res.data)
      } else {
        setTestError(res?.message || t('Model test failed'))
      }
    } catch (err) {
      setTestError(err instanceof Error ? err.message : t('Model test failed'))
    } finally {
      setTesting(false)
    }
  }

  const handleDelete = async () => {
    if (!deleteTarget) return
    setDeleting(true)
    try {
      const res = await deleteNotebookDeployment(deleteTarget.id)
      if (res?.success) {
        toast.success(t('Deleted successfully'))
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
    <div className='flex h-full min-h-0 flex-col gap-4'>
      <div className='rounded-lg border overflow-auto'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('Name')}</TableHead>
              <TableHead>{t('Provider')}</TableHead>
              <TableHead>{t('Model')}</TableHead>
              <TableHead>{t('Task')}</TableHead>
              <TableHead>{t('Hardware')}</TableHead>
              <TableHead>{t('Status')}</TableHead>
              <TableHead>{t('Created')}</TableHead>
              <TableHead className='text-right'>{t('Actions')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {listQuery.isLoading ? (
              <TableRow>
                <TableCell colSpan={8} className='text-center py-8'>
                  <Loader2 className='h-5 w-5 animate-spin mx-auto text-muted-foreground' />
                </TableCell>
              </TableRow>
            ) : items.length === 0 ? (
              <TableRow>
                <TableCell
                  colSpan={8}
                  className='text-center py-10 text-muted-foreground text-sm'
                >
                  {t('No deployments available. Create one to get started.')}
                </TableCell>
              </TableRow>
            ) : (
              items.map((item) => {
                const fallbacks = parseFallbacks(item.fallback_accelerators)
                const providerLabel =
                  PROVIDER_LABELS[item.provider as DeploymentProviderId] ??
                  item.provider
                return (
                  <TableRow key={item.id}>
                    <TableCell className='font-medium font-mono text-xs'>
                      {item.name}
                    </TableCell>
                    <TableCell>
                      <Badge variant='outline' className='text-xs'>
                        {providerLabel}
                      </Badge>
                    </TableCell>
                    <TableCell className='font-mono text-xs'>
                      <div>{item.model_repo}</div>
                      {item.model_file ? (
                        <div className='text-[11px] text-muted-foreground'>
                          {item.model_file}
                        </div>
                      ) : null}
                    </TableCell>
                    <TableCell>
                      <Badge
                        variant='secondary'
                        className='text-xs capitalize'
                      >
                        {item.task_type}
                      </Badge>
                    </TableCell>
                    <TableCell className='font-mono text-xs'>
                      <div className='flex flex-wrap items-center gap-1'>
                        <Badge variant='default' className='text-[11px]'>
                          {item.accelerator}
                        </Badge>
                        {fallbacks.map((fb) => (
                          <Badge
                            key={fb}
                            variant='secondary'
                            className='text-[10px]'
                          >
                            {fb}
                          </Badge>
                        ))}
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge variant='outline' className='text-xs capitalize'>
                        {item.status}
                      </Badge>
                    </TableCell>
                    <TableCell className='font-mono text-xs'>
                      {formatTimestampToDate(item.created_time)}
                    </TableCell>
                    <TableCell className='text-right'>
                      <div className='flex items-center justify-end gap-1'>
                        <Button
                          variant='outline'
                          size='sm'
                          className='h-7 text-xs gap-1'
                          onClick={() => {
                            setTestTarget(item)
                            setTestResult(null)
                            setTestError(null)
                          }}
                        >
                          <Play className='h-3 w-3' />
                          {t('Test')}
                        </Button>
                        <Button
                          variant='ghost'
                          size='icon-sm'
                          onClick={() => {
                            setDetailsItem(item)
                            setEditEndpointUrl(item.endpoint_url ?? '')
                          }}
                          aria-label={t('View details')}
                        >
                          <Eye className='h-4 w-4' />
                        </Button>
                        <Button
                          variant='ghost'
                          size='icon-sm'
                          className='text-destructive hover:text-destructive'
                          onClick={() => setDeleteTarget(item)}
                          aria-label={t('Delete')}
                        >
                          <Trash2 className='h-4 w-4' />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                )
              })
            )}
          </TableBody>
        </Table>
      </div>

      {/* Single Unified Create Deployment Dialog */}
      <Dialog open={createOpen} onOpenChange={onCreateOpenChange}>
        <DialogContent className='max-w-2xl max-h-[90vh] overflow-y-auto'>
          <DialogHeader>
            <DialogTitle>{t('Create deployment')}</DialogTitle>
          </DialogHeader>

          <div className='grid gap-4 py-2'>
            <div className='grid gap-4 sm:grid-cols-2'>
              <div className='space-y-1.5'>
                <label className='text-sm font-medium'>{t('Provider')}</label>
                <Select
                  value={provider}
                  onValueChange={(v) =>
                    handleProviderChange((v ?? defaultProvider) as DeploymentProviderId)
                  }
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {enabledProviders.map((p) => (
                      <SelectItem key={p} value={p}>
                        {PROVIDER_LABELS[p]}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className='space-y-1.5'>
                <label className='text-sm font-medium'>
                  {t('Deployment Name')}
                </label>
                <Input
                  placeholder='my-qwen-server'
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                />
              </div>
            </div>

            <div className='space-y-1.5'>
              <label className='text-sm font-medium'>
                {t('Hugging Face Model')}
              </label>
              <div className='flex gap-2'>
                <Input
                  placeholder='Qwen/Qwen2.5-7B-Instruct'
                  value={modelRepo}
                  onChange={(e) => setModelRepo(e.target.value)}
                />
                <Button
                  type='button'
                  variant='outline'
                  className='shrink-0 gap-1.5'
                  onClick={() => setHfPickerOpen(true)}
                >
                  <Search className='h-4 w-4' />
                  {t('Select from Hugging Face')}
                </Button>
              </div>
              {modelFile ? (
                <p className='text-xs text-muted-foreground font-mono'>
                  {t('Selected file')}: {modelFile}
                </p>
              ) : null}
            </div>

            <div className='grid gap-4 sm:grid-cols-2'>
              <div className='space-y-1.5'>
                <label className='text-sm font-medium'>
                  {t('Model Type / Task')}
                </label>
                <Select
                  value={taskType}
                  onValueChange={(v) => setTaskType(v ?? 'text')}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {TASK_TYPES.map((tk) => (
                      <SelectItem key={tk.id} value={tk.id}>
                        {t(tk.label)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className='space-y-1.5'>
                <label className='text-sm font-medium'>
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
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {hardwareOptions.map((hw) => (
                      <SelectItem key={hw.id} value={hw.id}>
                        {hw.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>

            <div className='space-y-2'>
              <div className='text-sm font-medium'>
                {t('Alternative / Fallback Hardware (if primary fails)')}
              </div>
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

            <div className='grid gap-4 sm:grid-cols-2'>
              <div className='space-y-1.5'>
                <label className='text-sm font-medium'>
                  {t('HF Token (Optional)')}
                </label>
                <Input
                  type='password'
                  placeholder='hf_...'
                  value={hfToken}
                  onChange={(e) => setHfToken(e.target.value)}
                />
              </div>
              <div className='space-y-1.5'>
                <label className='text-sm font-medium'>
                  {t('Endpoint URL (Optional)')}
                </label>
                <Input
                  placeholder='https://xxxx.trycloudflare.com'
                  value={endpointUrlInput}
                  onChange={(e) => setEndpointUrlInput(e.target.value)}
                />
              </div>
            </div>
          </div>

          <DialogFooter>
            <Button variant='outline' onClick={() => onCreateOpenChange(false)}>
              {t('Cancel')}
            </Button>
            <Button
              disabled={
                createMutation.isPending ||
                !name.trim() ||
                !modelRepo.trim()
              }
              onClick={() => createMutation.mutate()}
            >
              {createMutation.isPending && (
                <Loader2 className='h-4 w-4 mr-1.5 animate-spin' />
              )}
              {t('Create deployment')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <HfModelPickerDialog
        open={hfPickerOpen}
        onOpenChange={setHfPickerOpen}
        onSelect={handleHfPicked}
      />

      {/* Details & Serving Script Dialog */}
      <Dialog
        open={Boolean(detailsItem)}
        onOpenChange={(open) => {
          if (!open) setDetailsItem(null)
        }}
      >
        <DialogContent className='max-w-3xl'>
          <DialogHeader>
            <DialogTitle>{t('Deployment Details')}</DialogTitle>
          </DialogHeader>
          {detailsItem ? (
            <div className='space-y-4'>
              <div className='grid gap-2 text-xs sm:grid-cols-2'>
                <div>
                  <span className='text-muted-foreground'>{t('Model')}: </span>
                  <span className='font-mono font-semibold'>
                    {detailsItem.model_repo}
                  </span>
                </div>
                <div>
                  <span className='text-muted-foreground'>
                    {t('Hardware')}:{' '}
                  </span>
                  <span className='font-mono font-semibold'>
                    {detailsItem.accelerator}
                  </span>
                </div>
                {detailsItem.remote_url ? (
                  <div className='sm:col-span-2'>
                    <span className='text-muted-foreground'>
                      {t('Remote URL')}:{' '}
                    </span>
                    <a
                      href={detailsItem.remote_url}
                      target='_blank'
                      rel='noreferrer'
                      className='text-primary underline inline-flex items-center gap-1'
                    >
                      {detailsItem.remote_url}
                      <ExternalLink className='h-3 w-3' />
                    </a>
                  </div>
                ) : null}
              </div>

              <div className='space-y-1.5'>
                <label className='text-xs font-medium'>
                  {t('Serving Endpoint URL')}
                </label>
                <div className='flex gap-2'>
                  <Input
                    value={editEndpointUrl}
                    onChange={(e) => setEditEndpointUrl(e.target.value)}
                    placeholder='https://xxxx.trycloudflare.com'
                    className='text-xs font-mono'
                  />
                  <Button
                    size='sm'
                    variant='outline'
                    disabled={savingEndpoint}
                    onClick={handleSaveEndpointUrl}
                  >
                    {savingEndpoint && (
                      <Loader2 className='h-3.5 w-3.5 mr-1 animate-spin' />
                    )}
                    {t('Save')}
                  </Button>
                </div>
              </div>

              <div className='space-y-1.5'>
                <div className='flex items-center justify-between'>
                  <label className='text-xs font-medium'>
                    {t('Generated Serving Script')}
                  </label>
                  <Button
                    size='sm'
                    variant='ghost'
                    className='h-7 text-xs gap-1'
                    onClick={() => {
                      navigator.clipboard.writeText(detailsItem.script ?? '')
                      toast.success(t('Copied'))
                    }}
                  >
                    <Copy className='h-3.5 w-3.5' />
                    {t('Copy')}
                  </Button>
                </div>
                <pre className='max-h-72 overflow-auto rounded-md bg-muted p-3 font-mono text-[11px]'>
                  {detailsItem.script}
                </pre>
              </div>
            </div>
          ) : null}
        </DialogContent>
      </Dialog>

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
  )
}
