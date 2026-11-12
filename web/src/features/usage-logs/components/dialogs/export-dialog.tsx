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
import { Download, Loader2 } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { CaptchaDialog } from '@/features/auth/components/captcha-dialog'
import { useCaptcha } from '@/features/auth/hooks/use-captcha'
import { api } from '@/lib/api'

import {
  buildExportArchive,
  buildExportFile,
  downloadBytes,
  exportFileName,
  LOG_EXPORT_FORMATS,
  type LogExportFormat,
  type LogExportPackage,
  type LogExportRow,
} from '../../lib/export'

type ExportResponse = {
  success: boolean
  code?: string
  message?: string
  next_allowed_at?: number
  data?: {
    rows: LogExportRow[]
    total: number
    returned: number
    trimmed: number
    max_bytes: number
    next_allowed_at: number
  }
}

const MIME_TYPES: Record<LogExportFormat, string> = {
  csv: 'text/csv;charset=utf-8',
  json: 'application/json;charset=utf-8',
  txt: 'text/plain;charset=utf-8',
  log: 'text/plain;charset=utf-8',
  sql: 'application/sql;charset=utf-8',
  parquet: 'application/vnd.apache.parquet',
}

/** The formats a package may hold. */
const PACKAGE_VALUES: LogExportPackage[] = ['none', 'zip']

type ExportDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** The filters of the page, as the query of the export request. */
  query: Record<string, string>
  /** The endpoint, which differs between the admin view and the own logs. */
  endpoint?: string
}

/**
 * The download of the usage logs.
 *
 * The dialog asks for the format and the packaging, opens the captcha of the
 * installation, asks the server for the rows of the filter and writes the file
 * in the browser. The cooldown of ten minutes and the cap of thirty megabytes
 * belong to the server; the dialog shows both.
 */
export function UsageLogsExportDialog({
  open,
  onOpenChange,
  query,
  endpoint = '/api/log/export',
}: ExportDialogProps) {
  const { t } = useTranslation()
  const { isCaptchaEnabled, providers } = useCaptcha()
  const [format, setFormat] = useState<LogExportFormat>('csv')
  const [packageAs, setPackageAs] = useState<LogExportPackage>('none')
  const [loading, setLoading] = useState(false)
  const [captchaOpen, setCaptchaOpen] = useState(false)
  const [cooldownUntil, setCooldownUntil] = useState(0)
  const [now, setNow] = useState(() => Date.now())

  // A cooldown that survives a reload: the server keeps the real one, the
  // browser only remembers what it was told so the button does not look
  // available while it is not.
  const storageKey = `usage_logs_export_next_${endpoint}`

  useEffect(() => {
    try {
      const raw = window.localStorage.getItem(storageKey)
      const value = Number(raw)
      if (Number.isFinite(value) && value > 0) setCooldownUntil(value)
    } catch {
      /* a browser without storage simply has no local cooldown */
    }
  }, [storageKey])

  useEffect(() => {
    if (cooldownUntil <= Date.now()) return
    const timer = window.setInterval(() => setNow(Date.now()), 1000)
    return () => window.clearInterval(timer)
  }, [cooldownUntil])

  const remainingSeconds = useMemo(() => {
    const until = cooldownUntil - now
    return until > 0 ? Math.ceil(until / 1000) : 0
  }, [cooldownUntil, now])

  const rememberCooldown = (untilSeconds: number) => {
    const until = untilSeconds * 1000
    setCooldownUntil(until)
    setNow(Date.now())
    try {
      window.localStorage.setItem(storageKey, String(until))
    } catch {
      /* ignore */
    }
  }

  const runExport = async (captchaToken?: string, provider?: string) => {
    setLoading(true)
    try {
      const params = new URLSearchParams(query)
      if (captchaToken) {
        params.set('turnstile', captchaToken)
        params.set('captcha_provider', provider ?? '')
      }
      const res = await api.get<ExportResponse>(
        `${endpoint}?${params.toString()}`,
        { skipErrorHandler: true }
      )
      const payload = res.data
      if (!payload.success) {
        if (payload.code === 'export_cooldown' && payload.next_allowed_at) {
          rememberCooldown(payload.next_allowed_at)
          toast.error(t('The next export is available in a few minutes.'))
          return
        }
        toast.error(payload.message || t('The export failed'))
        return
      }

      const data = payload.data
      if (!data) return
      if (data.rows.length === 0) {
        toast.error(t('There is nothing to export for these filters.'))
        return
      }

      const stamp = new Date()
      const bytes =
        packageAs === 'zip'
          ? buildExportArchive(data.rows, format, stamp)
          : buildExportFile(data.rows, format)
      downloadBytes(
        bytes,
        exportFileName(format, packageAs, stamp),
        packageAs === 'zip' ? 'application/zip' : MIME_TYPES[format]
      )
      rememberCooldown(data.next_allowed_at)

      if (data.trimmed > 0) {
        toast.message(
          t(
            'The file holds the newest {{rows}} rows; {{trimmed}} older rows were left out to stay inside {{size}} MB.',
            {
              rows: data.returned,
              trimmed: data.trimmed,
              size: Math.round(data.max_bytes / (1024 * 1024)),
            }
          )
        )
      } else {
        toast.success(t('{{rows}} rows exported.', { rows: data.returned }))
      }
    } catch (error) {
      const message = error instanceof Error ? error.message : ''
      toast.error(message || t('The export failed'))
    } finally {
      setLoading(false)
    }
  }

  const handleDownload = () => {
    if (remainingSeconds > 0) return
    if (isCaptchaEnabled) {
      setCaptchaOpen(true)
      return
    }
    void runExport()
  }

  return (
    <>
      <Dialog
        open={open}
        onOpenChange={onOpenChange}
        title={t('Export the usage logs')}
        description={t(
          'The file is written from the rows that match the filters of this page.'
        )}
        contentClassName='max-w-md'
        headerClassName='text-left'
        contentHeight='auto'
        bodyClassName='space-y-4'
        footer={
          <div className='flex w-full items-center justify-end gap-2'>
            <Button
              onClick={() => onOpenChange(false)}
              type='button'
              variant='outline'
            >
              {t('Cancel')}
            </Button>
            <Button
              disabled={loading || remainingSeconds > 0}
              onClick={handleDownload}
              type='button'
            >
              {loading ? (
                <Loader2 className='mr-1 size-4 animate-spin' />
              ) : (
                <Download className='mr-1 size-4' />
              )}
              {remainingSeconds > 0
                ? t('Available in {{seconds}}s', { seconds: remainingSeconds })
                : t('Download')}
            </Button>
          </div>
        }
      >
        <div className='space-y-3'>
          <div className='space-y-1'>
            <Label className='text-xs'>{t('Format')}</Label>
            <Select
              onValueChange={(value) => setFormat(value as LogExportFormat)}
              value={format}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                {LOG_EXPORT_FORMATS.map((value) => (
                  <SelectItem key={value} value={value}>
                    {value === 'sql'
                      ? t('SQL dump (DB)')
                      : value.toUpperCase()}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className='space-y-1'>
            <Label className='text-xs'>{t('Packaging')}</Label>
            <Select
              onValueChange={(value) =>
                setPackageAs(value as LogExportPackage)
              }
              value={packageAs}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                {PACKAGE_VALUES.map((value) => (
                  <SelectItem key={value} value={value}>
                    {value === 'none' ? t('No archive') : t('ZIP archive')}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <p className='text-muted-foreground text-xs'>
            {t(
              'A download is limited to 30 MB: when the filter matches more, the oldest rows are left out. After a download the next one is available in ten minutes.'
            )}
          </p>
        </div>
      </Dialog>

      <CaptchaDialog
        open={captchaOpen}
        providers={providers}
        onOpenChange={setCaptchaOpen}
        onVerified={(token, provider) => {
          void runExport(token, provider)
        }}
      />
    </>
  )
}
