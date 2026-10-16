import type { ColumnDef } from '@tanstack/react-table'
import { Eye, Info, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { DataTableRowActionMenu } from '@/components/data-table/core/row-action-menu'
import { StatusBadge } from '@/components/status-badge'
import { TableId } from '@/components/table-id'
import { Button } from '@/components/ui/button'
import {
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
} from '@/components/ui/dropdown-menu'
import { formatTimestampToDate } from '@/lib/format'

import { getDeploymentStatusConfig } from '../constants'

export interface HfEndpointRow {
  name: string
  namespace?: string
  status?: { state?: string }
  model?: { repository?: string; task?: string }
  compute?: { accelerator?: string; instanceType?: string; instanceSize?: string }
  endpointUrl?: string
  createdAt?: string
}

export function normalizeHfStatus(raw: unknown): string {
  const s = String(raw ?? '')
    .toLowerCase()
    .replaceAll(/[_\s]+/g, '')
  if (s.includes('run')) return 'running'
  if (s.includes('paus') || s.includes('scaledtozero')) return 'paused'
  if (s.includes('fail') || s.includes('error')) return 'failed'
  if (s.includes('build') || s.includes('pending') || s.includes('updat')) {
    return 'building'
  }
  return s || 'unknown'
}

function toTimestamp(value: unknown): number | undefined {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string' && value.trim()) {
    const asNumber = Number(value)
    if (Number.isFinite(asNumber) && value.trim() !== '') return asNumber
    const parsed = Date.parse(value)
    if (!Number.isNaN(parsed)) return Math.floor(parsed / 1000)
  }
  return undefined
}

export function useHfEndpointsColumns(opts: {
  onViewDetails: (endpoint: HfEndpointRow) => void
  onDelete: (endpoint: HfEndpointRow) => void
}): ColumnDef<HfEndpointRow>[] {
  const { t } = useTranslation()
  const STATUS = getDeploymentStatusConfig(t)

  return [
    {
      accessorKey: 'name',
      header: t('ID'),
      meta: { mobileHidden: true },
      cell: ({ row }) => <TableId value={row.original.name} />,
      size: 120,
    },
    {
      id: 'endpoint',
      accessorFn: (row) =>
        row.namespace ? `${row.namespace}/${row.name}` : row.name,
      header: t('Name'),
      meta: { mobileTitle: true },
      cell: ({ getValue }) => {
        const name = String(getValue() || '-') || '-'
        return (
          <StatusBadge
            label={name}
            variant='neutral'
            copyText={name}
            size='sm'
            className='-ml-1.5 font-mono'
          />
        )
      },
      minSize: 220,
    },
    {
      id: 'status',
      accessorFn: (row) => normalizeHfStatus(row.status?.state),
      header: t('Status'),
      meta: { mobileBadge: true },
      cell: ({ row }) => {
        const raw =
          typeof row.original.status?.state === 'string' &&
          row.original.status.state.trim()
            ? row.original.status.state.trim()
            : t('Unknown')
        const key = normalizeHfStatus(row.original.status?.state)
        const config = STATUS[key] || {
          label: raw,
          variant: 'neutral' as const,
        }
        return (
          <StatusBadge
            label={config.label}
            variant={config.variant}
            size='sm'
            copyable={false}
            className='-ml-1.5'
          />
        )
      },
      filterFn: (row, id, value) => {
        if (
          !Array.isArray(value) ||
          value.length === 0 ||
          value.includes('all')
        ) {
          return true
        }
        return value.includes(String(row.getValue(id)))
      },
      size: 160,
      enableSorting: false,
    },
    {
      id: 'provider',
      header: t('Provider'),
      cell: () => (
        <StatusBadge
          label='Hugging Face'
          autoColor='Hugging Face'
          size='sm'
          copyable={false}
          className='-ml-1.5'
        />
      ),
      size: 140,
      enableSorting: false,
    },
    {
      id: 'model',
      accessorFn: (row) => row.model?.repository || '',
      header: t('Model'),
      cell: ({ row }) => {
        const repo = row.original.model?.repository || ''
        if (!repo) {
          return <span className='text-muted-foreground text-xs'>-</span>
        }
        return (
          <StatusBadge
            label={repo}
            variant='neutral'
            copyText={repo}
            size='sm'
            className='-ml-1.5 font-mono'
          />
        )
      },
      minSize: 220,
      enableSorting: false,
    },
    {
      id: 'hardware',
      accessorFn: (row) =>
        [row.compute?.instanceType, row.compute?.instanceSize]
          .filter(Boolean)
          .join(' '),
      header: t('Hardware'),
      meta: { mobileHidden: true },
      cell: ({ row }) => {
        const hw = [row.original.compute?.instanceType, row.original.compute?.instanceSize]
          .filter(Boolean)
          .join(' ')
        if (!hw) {
          return <span className='text-muted-foreground text-xs'>-</span>
        }
        return (
          <StatusBadge
            label={hw}
            variant='neutral'
            copyText={hw}
            size='sm'
            className='-ml-1.5 font-mono'
          />
        )
      },
      minSize: 180,
      enableSorting: false,
    },
    {
      id: 'created',
      accessorFn: (row) => row.createdAt || '',
      header: t('Created'),
      meta: { mobileHidden: true },
      cell: ({ row }) => (
        <div className='min-w-[140px] font-mono text-sm'>
          {formatTimestampToDate(toTimestamp(row.original.createdAt))}
        </div>
      ),
      size: 180,
    },
    {
      id: 'actions',
      header: () => t('Actions'),
      enableHiding: false,
      enableSorting: false,
      cell: ({ row }) => (
        <div className='-ml-2.5 flex items-center gap-1'>
          <Button
            variant='ghost'
            size='icon-sm'
            onClick={() => opts.onViewDetails(row.original)}
            aria-label={t('View details')}
          >
            <Eye />
          </Button>
          <DataTableRowActionMenu ariaLabel={t('Open menu')}>
            <DropdownMenuItem onClick={() => opts.onViewDetails(row.original)}>
              {t('View details')}
              <DropdownMenuShortcut>
                <Info size={16} />
              </DropdownMenuShortcut>
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              onClick={() => opts.onDelete(row.original)}
              className='text-destructive focus:text-destructive'
            >
              {t('Delete')}
              <DropdownMenuShortcut>
                <Trash2 size={16} />
              </DropdownMenuShortcut>
            </DropdownMenuItem>
          </DataTableRowActionMenu>
        </div>
      ),
      meta: { pinned: 'right' as const },
    },
  ]
}
