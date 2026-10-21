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
import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Activity, Cpu, Gauge, Link2, Server, Terminal } from 'lucide-react'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'

interface WorkerState {
  node_id: string
  device_name: string
  status: string
  model_status: string
  download_status: string
  download_progress: number
  current_tps: number
  models: string
}

const PAGES = [
  { key: 'status', icon: Activity },
  { key: 'pairing', icon: Link2 },
  { key: 'settings', icon: Server },
  { key: 'test', icon: Terminal },
] as const

/**
 * The page of the client of the local computing network.
 *
 * The page is shown by the client of the installation, it uses the theme of the
 * site and it does not need the session of the user.
 */
export function ClientPage() {
  const { t } = useTranslation()
  const [page, setPage] = useState<(typeof PAGES)[number]['key']>('status')
  const [nodeId, setNodeId] = useState('')
  const [state, setState] = useState<WorkerState | null>(null)
  const [code, setCode] = useState('')
  const [prompt, setPrompt] = useState('Hello! Introduce yourself in one sentence.')
  const [answer, setAnswer] = useState('')
  const [message, setMessage] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    const search = new URLSearchParams(window.location.search)
    const fromQuery = search.get('node') ?? ''
    const stored = window.localStorage.getItem('byog_node_id') ?? ''
    setNodeId(fromQuery || stored)
  }, [])

  const loadState = useCallback(async () => {
    if (!nodeId) {
      return
    }
    try {
      const response = await fetch(
        `/api/v1/worker/status?node_id=${encodeURIComponent(nodeId)}`
      )
      const payload = await response.json()
      if (payload.success) {
        setState(payload.data)
      }
    } catch {
      // The client shows the state it knows when the server does not answer.
    }
  }, [nodeId])

  useEffect(() => {
    loadState()
    const timer = window.setInterval(loadState, 4000)
    return () => window.clearInterval(timer)
  }, [loadState])

  const pair = async () => {
    setBusy(true)
    setMessage('')
    try {
      const response = await fetch('/api/v1/worker/register', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          pairing_code: code.trim(),
          device_name: 'BYOG Worker',
          device_type: 'desktop',
          os: 'windows',
          arch: 'amd64',
        }),
      })
      const payload = await response.json()
      if (payload.success && payload.data?.node_id) {
        window.localStorage.setItem('byog_node_id', payload.data.node_id)
        setNodeId(payload.data.node_id)
        setMessage(t('byog.client_paired'))
        loadState()
      } else {
        setMessage(payload.message ?? t('byog.client_pair_failed'))
      }
    } catch {
      setMessage(t('byog.client_pair_failed'))
    } finally {
      setBusy(false)
    }
  }

  const runTest = async () => {
    setBusy(true)
    setAnswer('')
    try {
      const response = await fetch('/api/v1/worker/test-inference', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          node_id: nodeId,
          prompt,
          model: state?.models?.split(',')[0] ?? '',
        }),
      })
      const payload = await response.json()
      if (payload.success) {
        setAnswer(
          payload.data?.reply ?? payload.data?.content ?? JSON.stringify(payload.data)
        )
      } else {
        setMessage(payload.message ?? t('byog.client_test_failed'))
      }
    } catch {
      setMessage(t('byog.client_test_failed'))
    } finally {
      setBusy(false)
    }
  }

  const statusLabel = () => {
    if (!state) {
      return t('byog.client_offline')
    }
    if (state.status === 'online') {
      return t('byog.client_online')
    }
    if (state.download_status) {
      return t('byog.client_downloading')
    }
    return state.status
  }

  return (
    <div className='bg-background flex h-screen w-full overflow-hidden text-left'>
      <aside className='bg-card flex w-52 shrink-0 flex-col gap-1 border-r p-3'>
        <div className='text-primary mb-2 px-3 text-lg font-bold'>
          {t('byog.client_title')}
        </div>
        {PAGES.map((item) => {
          const Icon = item.icon
          const active = page === item.key
          return (
            <button
              key={item.key}
              type='button'
              onClick={() => setPage(item.key)}
              className={`flex items-center gap-2 rounded-lg px-3 py-2 text-sm transition-colors ${
                active
                  ? 'bg-primary/10 text-primary font-semibold'
                  : 'text-muted-foreground hover:bg-muted'
              }`}
            >
              <Icon className='h-4 w-4' />
              {t(`byog.client_${item.key}`)}
            </button>
          )
        })}
        <div className='mt-auto px-3 text-xs text-muted-foreground'>
          {state?.device_name ?? nodeId ?? '-'}
        </div>
      </aside>

      <main className='flex-1 overflow-y-auto p-6'>
        {page === 'status' && (
          <Card>
            <CardHeader>
              <CardTitle className='flex items-center gap-2'>
                {t('byog.client_status')}
                <Badge variant={state?.status === 'online' ? 'default' : 'secondary'}>
                  {statusLabel()}
                </Badge>
              </CardTitle>
            </CardHeader>
            <CardContent className='grid gap-3'>
              <div className='flex items-center gap-2 text-sm'>
                <Server className='text-muted-foreground h-4 w-4' />
                <span className='text-muted-foreground w-24'>
                  {t('byog.client_server')}
                </span>
                <span className='truncate'>{window.location.origin}</span>
              </div>
              <div className='flex items-center gap-2 text-sm'>
                <Cpu className='text-muted-foreground h-4 w-4' />
                <span className='text-muted-foreground w-24'>
                  {t('byog.client_model')}
                </span>
                <span className='truncate'>{state?.models || '-'}</span>
              </div>
              <div className='flex items-center gap-2 text-sm'>
                <Gauge className='text-muted-foreground h-4 w-4' />
                <span className='text-muted-foreground w-24'>
                  {t('byog.client_speed')}
                </span>
                <span>{(state?.current_tps ?? 0).toFixed(1)}</span>
              </div>
              {state?.download_status && (
                <div className='bg-muted h-2 w-full overflow-hidden rounded-full'>
                  <div
                    className='bg-primary h-full'
                    style={{ width: `${state.download_progress}%` }}
                  />
                </div>
              )}
            </CardContent>
          </Card>
        )}

        {page === 'pairing' && (
          <Card>
            <CardHeader>
              <CardTitle>{t('byog.client_pairing')}</CardTitle>
            </CardHeader>
            <CardContent className='grid gap-3'>
              <p className='text-muted-foreground text-sm'>
                {t('byog.enter_pairing_code_instructions')}
              </p>
              <div className='flex gap-2'>
                <Input
                  value={code}
                  onChange={(event) => setCode(event.target.value)}
                  placeholder={t('byog.enter_pairing_code_label')}
                />
                <Button onClick={pair} disabled={busy || !code.trim()}>
                  {t('byog.client_connect')}
                </Button>
              </div>
              {message && <p className='text-sm'>{message}</p>}
            </CardContent>
          </Card>
        )}

        {page === 'settings' && (
          <Card>
            <CardHeader>
              <CardTitle>{t('byog.client_settings')}</CardTitle>
            </CardHeader>
            <CardContent className='grid gap-3 text-sm'>
              <div className='flex gap-2'>
                <span className='text-muted-foreground w-24'>
                  {t('byog.client_server')}
                </span>
                <span>{window.location.origin}</span>
              </div>
              <div className='flex gap-2'>
                <span className='text-muted-foreground w-24'>
                  {t('byog.client_node')}
                </span>
                <span className='truncate'>{nodeId || '-'}</span>
              </div>
            </CardContent>
          </Card>
        )}

        {page === 'test' && (
          <Card>
            <CardHeader>
              <CardTitle>{t('byog.client_test')}</CardTitle>
            </CardHeader>
            <CardContent className='grid gap-3'>
              <Input
                value={prompt}
                onChange={(event) => setPrompt(event.target.value)}
                placeholder={t('byog.client_test_prompt')}
              />
              <Button onClick={runTest} disabled={busy || !nodeId}>
                {t('byog.client_run_test')}
              </Button>
              {answer && (
                <pre className='bg-muted max-h-64 overflow-auto rounded-lg p-3 text-sm whitespace-pre-wrap'>
                  {answer}
                </pre>
              )}
              {message && <p className='text-sm'>{message}</p>}
            </CardContent>
          </Card>
        )}
      </main>
    </div>
  )
}
