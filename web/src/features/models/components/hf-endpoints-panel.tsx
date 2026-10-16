import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Plus } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

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
import { Switch } from '@/components/ui/switch'

import {
  createHfEndpoint,
  deleteHfEndpoint,
  listHfEndpoints,
} from '../api'

const HF_HARDWARE = [
  'cpu-basic',
  't4-small',
  'a10g-small',
  'a100-large',
] as const

export function HfEndpointsPanel() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [name, setName] = useState('')
  const [repository, setRepository] = useState('')
  const [hardware, setHardware] = useState<string>('t4-small')
  const [task, setTask] = useState('text-generation')
  const [isProtected, setIsProtected] = useState(true)

  const endpointsQuery = useQuery({
    queryKey: ['hf-endpoints'],
    queryFn: () => listHfEndpoints(),
    staleTime: 30 * 1000,
  })

  const createMutation = useMutation({
    mutationFn: () =>
      createHfEndpoint({
        name: name.trim(),
        repository: repository.trim(),
        hardware,
        task: task.trim() || 'text-generation',
        protected: isProtected,
      }),
    onSuccess: (data) => {
      if (data?.success) {
        toast.success(t('Endpoint created successfully'))
        setName('')
        setRepository('')
        setDialogOpen(false)
        queryClient.invalidateQueries({ queryKey: ['hf-endpoints'] })
        return
      }
      toast.error(data?.message || t('Failed to create endpoint'))
    },
    onError: (err: Error) => {
      toast.error(err.message || t('Failed to create endpoint'))
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (endpoint: { name: string; namespace?: string }) =>
      deleteHfEndpoint(endpoint.name, endpoint.namespace),
    onSuccess: (data) => {
      if (data?.success) {
        toast.success(t('Endpoint deleted successfully'))
        queryClient.invalidateQueries({ queryKey: ['hf-endpoints'] })
        return
      }
      toast.error(data?.message || t('Failed to delete endpoint'))
    },
    onError: (err: Error) => {
      toast.error(err.message || t('Failed to delete endpoint'))
    },
  })

  const endpoints = endpointsQuery.data?.data ?? []
  const canCreate =
    !createMutation.isPending && name.trim() !== '' && repository.trim() !== ''

  return (
    <div className='flex h-full min-h-0 flex-col gap-4'>
      <div className='flex justify-end'>
        <Button onClick={() => setDialogOpen(true)} size='sm'>
          <Plus className='h-4 w-4' />
          {t('Create endpoint')}
        </Button>
      </div>

      <div className='rounded-lg border p-4 space-y-3'>
        {endpointsQuery.isLoading && (
          <p className='text-muted-foreground text-sm'>{t('Loading...')}</p>
        )}
        {endpointsQuery.isError && (
          <p className='text-destructive text-sm'>
            {t('Failed to load endpoints')}
          </p>
        )}
        {!endpointsQuery.isLoading &&
          !endpointsQuery.isError &&
          endpoints.length === 0 && (
            <p className='text-muted-foreground text-sm'>
              {t('No endpoints yet.')}
            </p>
          )}
        {endpoints.map((ep) => (
          <div
            key={`${ep.namespace}/${ep.name}`}
            className='flex items-center justify-between gap-3 rounded-md border px-3 py-2'
          >
            <div className='min-w-0'>
              <div className='truncate font-mono text-sm'>
                {ep.namespace ? `${ep.namespace}/` : ''}
                {ep.name}
              </div>
              <div className='text-muted-foreground truncate text-xs'>
                {[ep.model?.repository, ep.status?.state]
                  .filter(Boolean)
                  .join(' - ')}
              </div>
            </div>
            <Button
              variant='destructive'
              size='sm'
              disabled={deleteMutation.isPending}
              onClick={() => {
                if (
                  window.confirm(
                    t('Delete endpoint {{name}}?', { name: ep.name })
                  )
                ) {
                  deleteMutation.mutate({
                    name: ep.name,
                    namespace: ep.namespace,
                  })
                }
              }}
            >
              {t('Delete')}
            </Button>
          </div>
        ))}
      </div>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('Create Hugging Face endpoint')}</DialogTitle>
          </DialogHeader>
          <div className='grid gap-4 py-2'>
            <div className='space-y-1'>
              <label className='text-sm font-medium'>
                {t('Endpoint name')}
              </label>
              <Input
                placeholder='my-llm'
                value={name}
                onChange={(e) => setName(e.target.value)}
              />
            </div>
            <div className='space-y-1'>
              <label className='text-sm font-medium'>
                {t('Model repository')}
              </label>
              <Input
                placeholder='meta-llama/Llama-3-8B'
                value={repository}
                onChange={(e) => setRepository(e.target.value)}
              />
            </div>
            <div className='grid gap-4 sm:grid-cols-2'>
              <div className='space-y-1'>
                <label className='text-sm font-medium'>{t('Hardware')}</label>
                <Select
                  value={hardware}
                  onValueChange={(v) => setHardware(v ?? 't4-small')}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {HF_HARDWARE.map((hw) => (
                      <SelectItem key={hw} value={hw}>
                        {hw}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className='space-y-1'>
                <label className='text-sm font-medium'>{t('Task')}</label>
                <Input
                  placeholder='text-generation'
                  value={task}
                  onChange={(e) => setTask(e.target.value)}
                />
              </div>
            </div>
            <div className='flex items-center justify-between gap-4'>
              <label className='text-sm font-medium'>
                {t('Protected endpoint (token required)')}
              </label>
              <Switch
                checked={isProtected}
                onCheckedChange={setIsProtected}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant='outline' onClick={() => setDialogOpen(false)}>
              {t('Cancel')}
            </Button>
            <Button
              onClick={() => createMutation.mutate()}
              disabled={!canCreate}
            >
              {t('Create endpoint')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
