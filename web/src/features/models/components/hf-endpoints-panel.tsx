import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { Plus } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { DataTablePage, useDataTable } from '@/components/data-table'
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
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { useMediaQuery } from '@/hooks'
import { useTableUrlState } from '@/hooks/use-table-url-state'
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
import { getDeploymentStatusOptions } from '../constants'
import {
  normalizeHfStatus,
  useHfEndpointsColumns,
  type HfEndpointRow,
} from './hf-endpoints-columns'

const route = getRouteApi('/_authenticated/models/$section')

const HF_HARDWARE = [
  'cpu-basic',
  't4-small',
  'a10g-small',
  'a100-large',
] as const

export function HfEndpointsPanel() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const isMobile = useMediaQuery('(max-width: 640px)')

  const {
    globalFilter,
    onGlobalFilterChange,
    columnFilters,
    onColumnFiltersChange,
    pagination,
    onPaginationChange,
    ensurePageInRange,
  } = useTableUrlState({
    search: route.useSearch(),
    navigate: route.useNavigate(),
    pagination: {
      pageKey: 'hPage',
      pageSizeKey: 'hPageSize',
      defaultPage: 1,
      defaultPageSize: isMobile ? 8 : 10,
    },
    globalFilter: { enabled: true, key: 'hFilter' },
    columnFilters: [
      { columnId: 'status', searchKey: 'hStatus', type: 'array' },
    ],
  })

  const keyword = globalFilter ?? ''
  const statusFilter =
    (columnFilters.find((f) => f.id === 'status')?.value as string[]) || []
  const activeStatus =
    statusFilter.length > 0 && !statusFilter.includes('all')
      ? statusFilter[0]
      : undefined

  const [dialogOpen, setDialogOpen] = useState(false)
  const [name, setName] = useState('')
  const [repository, setRepository] = useState('')
  const [hardware, setHardware] = useState<string>('t4-small')
  const [task, setTask] = useState('text-generation')
  const [isProtected, setIsProtected] = useState(true)

  const [detailsOpen, setDetailsOpen] = useState(false)
  const [detailsEndpoint, setDetailsEndpoint] =
    useState<HfEndpointRow | null>(null)
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<HfEndpointRow | null>(null)
  const [isDeleting, setIsDeleting] = useState(false)

  const endpointsQuery = useQuery({
    queryKey: ['hf-endpoints'],
    queryFn: () => listHfEndpoints(),
    staleTime: 30 * 1000,
  })

  const allEndpoints = useMemo<HfEndpointRow[]>(
    () => endpointsQuery.data?.data ?? [],
    [endpointsQuery.data]
  )

  const filtered = useMemo(() => {
    const kw = keyword.trim().toLowerCase()
    return allEndpoints.filter((ep) => {
      if (activeStatus && normalizeHfStatus(ep.status?.state) !== activeStatus) {
        return false
      }
      if (!kw) return true
      const haystack =
        `${ep.namespace ?? ''}/${ep.name} ${ep.model?.repository ?? ''}`.toLowerCase()
      return haystack.includes(kw)
    })
  }, [allEndpoints, keyword, activeStatus])

  const totalCount = filtered.length
  const paged = useMemo(() => {
    const start = pagination.pageIndex * pagination.pageSize
    return filtered.slice(start, start + pagination.pageSize)
  }, [filtered, pagination.pageIndex, pagination.pageSize])

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

  const handleDelete = async () => {
    if (!deleteTarget) return
    setIsDeleting(true)
    try {
      const res = await deleteHfEndpoint(
        deleteTarget.name,
        deleteTarget.namespace
      )
      if (res?.success) {
        toast.success(t('Deleted successfully'))
        queryClient.invalidateQueries({ queryKey: ['hf-endpoints'] })
      } else {
        toast.error(res?.message || t('Delete failed'))
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t('Delete failed'))
    } finally {
      setIsDeleting(false)
      setDeleteOpen(false)
      setDeleteTarget(null)
    }
  }

  const columns = useHfEndpointsColumns({
    onViewDetails: (endpoint) => {
      setDetailsEndpoint(endpoint)
      setDetailsOpen(true)
    },
    onDelete: (endpoint) => {
      setDeleteTarget(endpoint)
      setDeleteOpen(true)
    },
  })

  const { table } = useDataTable({
    data: paged,
    columns,
    totalCount,
    columnFilters,
    pagination,
    globalFilter,
    onColumnFiltersChange,
    onPaginationChange,
    onGlobalFilterChange,
    manualPagination: true,
    manualFiltering: true,
    withSortedRowModel: false,
    ensurePageInRange,
  })

  const statusFilterOptions = useMemo(() => {
    return [...getDeploymentStatusOptions(t)].map((opt) => ({
      label: opt.label,
      value: opt.value,
    }))
  }, [t])

  const canCreate =
    !createMutation.isPending && name.trim() !== '' && repository.trim() !== ''

  return (
    <>
      <div className='flex h-full min-h-0 flex-col gap-4'>
        <div className='flex justify-end'>
          <Button onClick={() => setDialogOpen(true)} size='sm'>
            <Plus className='h-4 w-4' />
            {t('Create endpoint')}
          </Button>
        </div>
        <div className='min-h-0 flex-1'>
          <DataTablePage
            table={table}
            columns={columns}
            isLoading={endpointsQuery.isLoading}
            isFetching={endpointsQuery.isFetching}
            emptyTitle={t('No Deployments Found')}
            emptyDescription={t(
              'No deployments available. Create one to get started.'
            )}
            skeletonKeyPrefix='hf-endpoint-skeleton'
            applyHeaderSize
            toolbarProps={{
              searchPlaceholder: t('Search deployments...'),
              searchDebounceMs: 500,
              filters: [
                {
                  columnId: 'status',
                  title: t('Status'),
                  options: statusFilterOptions,
                  singleSelect: true,
                },
              ],
            }}
          />
        </div>
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

      <Dialog
        open={detailsOpen}
        onOpenChange={(open) => {
          setDetailsOpen(open)
          if (!open) setDetailsEndpoint(null)
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('Endpoint details')}</DialogTitle>
          </DialogHeader>
          <pre className='max-h-96 overflow-auto rounded-md bg-muted p-3 font-mono text-xs'>
            {JSON.stringify(detailsEndpoint, null, 2)}
          </pre>
        </DialogContent>
      </Dialog>

      <AlertDialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Delete endpoint')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('Delete endpoint {{name}}?', {
                name: deleteTarget ? deleteTarget.name : '',
              })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={handleDelete} disabled={isDeleting}>
              {isDeleting ? t('Deleting...') : t('Delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
