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
import { useCallback, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { RefreshCw } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  CardDescription,
} from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  clearAdminCrashReports,
  getAdminCrashReports,
  getAdminDownloadErrors,
  getAdminObservabilitySummary,
} from '@/features/byog/api'
import type {
  ByogCrashReportItem,
  ByogCrashReportsPage,
  ByogDownloadErrorItem,
  ByogObservabilitySummary,
} from '@/features/byog/types'
import dayjs from '@/lib/dayjs'
import { formatNumber } from '@/lib/format'
import { ADMIN_PERMISSION_RESOURCES, hasPermission } from '@/lib/admin-permissions'
import { useAuthStore } from '@/stores/auth-store'

export function ByogObservabilityTab() {
  const { t } = useTranslation()
  const user = useAuthStore((state) => state.auth.user)
  const canClear = hasPermission(user, ADMIN_PERMISSION_RESOURCES.BYOG, 'write')
  const [clearing, setClearing] = useState(false)
  const [confirmClear, setConfirmClear] = useState(false)
  const [summary, setSummary] = useState<ByogObservabilitySummary | null>(null)
  const [crashes, setCrashes] = useState<ByogCrashReportsPage | null>(null)
  const [errors, setErrors] = useState<ByogDownloadErrorItem[]>([])
  const [selected, setSelected] = useState<ByogCrashReportItem | null>(null)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(true)
  const [failed, setFailed] = useState(false)
  const seq = useRef(0)
  const load = useCallback(async () => {
    const current = ++seq.current
    setLoading(true)
    try {
      const [health, reports, nodes] = await Promise.all([
        getAdminObservabilitySummary(),
        getAdminCrashReports({ page, page_size: 20 }),
        getAdminDownloadErrors(),
      ])
      if (current !== seq.current) return
      setSummary(health)
      setCrashes(reports)
      setErrors(nodes.items)
      setFailed(false)
    } catch {
      if (current === seq.current) setFailed(true)
    } finally {
      if (current === seq.current) setLoading(false)
    }
  }, [page])
  useEffect(() => {
    load()
    const timer = setInterval(load, 15000)
    return () => {
      clearInterval(timer)
      seq.current++
    }
  }, [load])
  const health = summary?.node_health
  const cards = health
    ? ([
        [t('Total'), health.total],
        [t('Online'), health.online],
        [t('Stale'), health.stale],
        [t('Offline'), health.offline],
        [t('Paused'), health.paused],
        [t('Banned'), health.banned],
        [t('Downloading'), health.downloading],
        [t('Errors'), health.errors],
        [t('Last 24 Hours'), summary.crash_reports_24h],
        [t('Last 7 Days'), summary.crash_reports_7d],
        [t('Total crashes'), summary.crash_reports_total],
      ] as const)
    : []
  return (
    <div className='space-y-4'>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <div>
          <h3 className='text-lg font-semibold'>{t('Observability')}</h3>
          <p className='text-sm text-muted-foreground'>
            {t('Node Health')} · {t('Crash Reports')}
            <br />
            {t('Health counts may overlap; online means a recent heartbeat.')}
          </p>
        </div>
        <Button variant='outline' size='sm' onClick={load} disabled={loading}>
          <RefreshCw className={loading ? 'size-4 animate-spin' : 'size-4'} />
          {t('Refresh')}
        </Button>
      </div>
      {loading && (
        <p role='status' className='text-sm text-muted-foreground'>
          {t('Loading...')}
        </p>
      )}
      {failed && (
        <div role='alert' className='flex items-center gap-2 text-destructive'>
          {t('Failed to load')}{' '}
          <Button variant='outline' onClick={load}>
            {t('Retry')}
          </Button>
        </div>
      )}
      <div className='grid grid-cols-2 gap-3 md:grid-cols-4 lg:grid-cols-6'>
        {cards.map(([label, count]) => (
          <Card key={label}>
            <CardHeader className='pb-2'>
              <CardTitle className='text-sm text-muted-foreground'>
                {label}
              </CardTitle>
            </CardHeader>
            <CardContent className='text-2xl font-semibold tabular-nums'>
              {formatNumber(count)}
            </CardContent>
          </Card>
        ))}
      </div>
      {summary && (
        <Card>
          <CardHeader>
            <CardTitle className='text-base'>{t('Error codes')}</CardTitle>
          </CardHeader>
          <CardContent className='flex flex-wrap gap-2'>
            {summary.top_error_codes?.length
              ? summary.top_error_codes.map((code) => (
                  <Badge key={code.name} variant='outline'>
                    {code.name}: {formatNumber(code.count)}
                  </Badge>
                ))
              : t('No Data')}
          </CardContent>
        </Card>
      )}
      <Card>
        <CardHeader>
          <CardTitle className='text-base'>{t('Crash Reports')}</CardTitle>
          {canClear && <Button variant='destructive' size='sm' disabled={clearing || !crashes?.total} onClick={() => setConfirmClear(true)}>{t('Archive and delete crash reports')}</Button>}
          <CardDescription>
            {crashes ? `${t('Total')}: ${formatNumber(crashes.total)}` : ''}
          </CardDescription>
        </CardHeader>
        <CardContent className='overflow-x-auto'>
          {crashes?.items.length ? (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('Created At')}</TableHead>
                  <TableHead>{t('Device')}</TableHead>
                  <TableHead>{t('Owner')}</TableHead>
                  <TableHead>{t('Error Code')}</TableHead>
                  <TableHead>{t('Error Message')}</TableHead>
                  <TableHead>{t('Details')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {crashes.items.map((report) => (
                  <TableRow key={report.id}>
                    <TableCell>
                      {dayjs
                        .unix(report.created_at)
                        .format('YYYY-MM-DD HH:mm:ss')}
                    </TableCell>
                    <TableCell>
                      {report.device_name || report.node_id}
                    </TableCell>
                    <TableCell>
                      {report.username || `#${report.user_id}`}
                    </TableCell>
                    <TableCell>
                      <Badge variant='outline'>{report.error_code}</Badge>
                    </TableCell>
                    <TableCell className='max-w-64 truncate'>
                      {report.error_message}
                    </TableCell>
                    <TableCell>
                      <Button
                        size='sm'
                        variant='outline'
                        onClick={() => setSelected(report)}
                      >
                        {t('Details')}
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          ) : (
            !loading && (
              <p className='py-6 text-center text-sm text-muted-foreground'>
                {t('No Data')}
              </p>
            )
          )}
          <div className='mt-4 flex items-center justify-end gap-2'>
            <Button
              variant='outline'
              size='sm'
              disabled={page <= 1 || loading}
              onClick={() => setPage((p) => p - 1)}
            >
              {t('Previous')}
            </Button>
            <span>{page}</span>
            <Button
              variant='outline'
              size='sm'
              disabled={
                loading || !crashes || page * crashes.page_size >= crashes.total
              }
              onClick={() => setPage((p) => p + 1)}
            >
              {t('Next')}
            </Button>
          </div>
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle className='text-base'>{t('Download Errors')}</CardTitle>
        </CardHeader>
        <CardContent className='overflow-x-auto'>
          {errors.length ? (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('Device')}</TableHead>
                  <TableHead>{t('Owner')}</TableHead>
                  <TableHead>{t('Model')}</TableHead>
                  <TableHead>{t('Model Status')}</TableHead>
                  <TableHead>{t('Download Progress')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {errors.map((node) => (
                  <TableRow key={node.node_id}>
                    <TableCell>{node.device_name || node.node_id}</TableCell>
                    <TableCell>{node.username || `#${node.user_id}`}</TableCell>
                    <TableCell>{node.download_model_id || '-'}</TableCell>
                    <TableCell>
                      {node.model_status} / {node.download_status}
                    </TableCell>
                    <TableCell>
                      {node.download_progress}% ({node.downloaded_mb} /{' '}
                      {node.total_mb} MB)
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          ) : (
            !loading && (
              <p className='py-6 text-center text-sm text-muted-foreground'>
                {t('No Data')}
              </p>
            )
          )}
        </CardContent>
      </Card>
      <Dialog open={confirmClear} onOpenChange={setConfirmClear}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('Archive and delete crash reports')}</DialogTitle>
            <DialogDescription>{t('All crash reports will be saved to the server log directory and deleted from the database. Live node errors are not cleared.')}</DialogDescription>
          </DialogHeader>
          <Button variant='destructive' disabled={clearing} onClick={async () => {
            setClearing(true)
            try {
              await clearAdminCrashReports()
              setConfirmClear(false)
              setSelected(null)
              if (page === 1) await load()
              else setPage(1)
            } catch { setFailed(true) }
            finally { setClearing(false) }
          }}>{t('Delete')}</Button>
        </DialogContent>
      </Dialog>
      <Dialog
        open={!!selected}
        onOpenChange={(open) => {
          if (!open) setSelected(null)
        }}
      >
        <DialogContent className='max-h-[85vh] overflow-y-auto'>
          <DialogHeader>
            <DialogTitle>{t('Crash Reports')}</DialogTitle>
            <DialogDescription>
              {selected?.device_name || selected?.node_id} ·{' '}
              {selected?.error_code}
            </DialogDescription>
          </DialogHeader>
          {selected && (
            <div className='space-y-3 text-sm'>
              <p>{selected.error_message}</p>
              <p>
                <strong>{t('Model')}:</strong> {selected.model_id || '-'}
              </p>
              <p>
                <strong>{t('Memory Usage')}:</strong> {selected.memory_usage_mb}{' '}
                MB
              </p>
              <p>
                <strong>{t('Recommendation')}:</strong>{' '}
                {selected.recommendation || '-'}
              </p>
              <div>
                <strong>{t('Stack Trace')}</strong>
                <pre className='mt-1 max-h-64 overflow-auto whitespace-pre-wrap break-all rounded bg-muted p-3'>
                  {selected.stack_trace || '-'}
                </pre>
              </div>
            </div>
          )}
        </DialogContent>
      </Dialog>
    </div>
  )
}
