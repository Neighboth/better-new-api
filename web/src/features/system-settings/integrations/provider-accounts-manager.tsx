import {
  CheckCircle2,
  Loader2,
  LogOut,
  Plus,
  Trash2,
  Wand2,
  XCircle,
} from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
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
  updateProviderAccounts,
  type DeploymentAccountInput,
  type DeploymentAccountSummary,
} from '@/features/models/api'
import {
  PROVIDER_TIERS,
  type DeploymentProviderId,
} from '@/features/models/lib/hardware-catalog'
import { api } from '@/lib/api'

interface EditableAccount extends DeploymentAccountInput {
  configured?: boolean
  email?: string
  eligible_gpus?: string[]
}

const MASK = '••••••••'

export function ProviderAccountsManager({
  provider,
  initialAccounts,
  onAccountsUpdated,
}: {
  provider: DeploymentProviderId
  initialAccounts: DeploymentAccountSummary[]
  onAccountsUpdated: (
    provider: DeploymentProviderId,
    next: DeploymentAccountSummary[]
  ) => void
}) {
  const { t } = useTranslation()
  const tiers = PROVIDER_TIERS[provider] ?? []
  const defaultTier = tiers[0]?.id ?? 'standard'

  const toEditable = (list: DeploymentAccountSummary[]): EditableAccount[] =>
    list.map((a, idx) => ({
      id: a.id || `acc-${idx + 1}`,
      label: a.label || `#${idx + 1}`,
      tier: a.tier || defaultTier,
      enabled: a.enabled ?? true,
      configured: a.configured,
      email: a.email,
      eligible_gpus: a.eligible_gpus,
      username: a.username ?? '',
      org: a.org ?? '',
      project_id: a.project_id ?? '',
      region: a.region ?? 'us-central1',
      token_id: a.has_token_id ? MASK : '',
      token_secret: a.has_token_secret ? MASK : '',
      api_key: a.has_api_key ? MASK : '',
      token: a.has_token ? MASK : '',
    }))

  const [accounts, setAccounts] = useState<EditableAccount[]>(() =>
    toEditable(initialAccounts)
  )
  const [saving, setSaving] = useState(false)
  const [testingId, setTestingId] = useState<string | null>(null)
  const [testResults, setTestResults] = useState<
    Record<string, { ok: boolean; msg: string }>
  >({})

  // Per-account Colab OAuth state
  const [oauthTargetId, setOauthTargetId] = useState<string | null>(null)
  const [oauthSessionId, setOauthSessionId] = useState('')
  const [oauthUrl, setOauthUrl] = useState('')
  const [oauthCallback, setOauthCallback] = useState('')
  const [oauthExchanging, setOauthExchanging] = useState(false)

  useEffect(() => {
    setAccounts(toEditable(initialAccounts))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [initialAccounts])

  const addAccount = () => {
    const nextIdx = accounts.length + 1
    setAccounts((prev) => [
      ...prev,
      {
        id: `acc-${Date.now().toString(36)}-${nextIdx}`,
        label: `#${nextIdx}`,
        tier: defaultTier,
        enabled: true,
        username: '',
        org: '',
        project_id: '',
        region: 'us-central1',
        token_id: '',
        token_secret: '',
        api_key: '',
        token: '',
      },
    ])
  }

  const updateField = (
    id: string,
    patch: Partial<EditableAccount>
  ) => {
    setAccounts((prev) =>
      prev.map((a) => (a.id === id ? { ...a, ...patch } : a))
    )
  }

  const removeAccount = (id: string) => {
    setAccounts((prev) => prev.filter((a) => a.id !== id))
  }

  const handleSaveAccounts = async () => {
    setSaving(true)
    try {
      const payload: DeploymentAccountInput[] = accounts.map((a, idx) => ({
        id: a.id,
        label: a.label.trim() || `#${idx + 1}`,
        tier: a.tier || defaultTier,
        enabled: a.enabled,
        token_id: a.token_id,
        token_secret: a.token_secret,
        username: a.username,
        api_key: a.api_key,
        token: a.token,
        org: a.org,
        project_id: a.project_id,
        region: a.region,
      }))
      const res = await updateProviderAccounts(provider, payload)
      if (res?.success && res.data) {
        onAccountsUpdated(provider, res.data)
        toast.success(t('Accounts saved'))
      } else {
        toast.error(res?.message || t('Failed to save accounts'))
      }
    } catch (e: any) {
      toast.error(e?.message || t('Failed to save accounts'))
    } finally {
      setSaving(false)
    }
  }

  const handleTestAccount = async (acc: EditableAccount) => {
    setTestingId(acc.id)
    try {
      if (provider === 'kaggle') {
        const res = await api.post('/api/deployments/kaggle/test-connection', {
          account_id: acc.id,
          username: acc.username,
          api_key: acc.api_key,
        })
        if (res.data?.success) {
          setTestResults((p) => ({
            ...p,
            [acc.id]: { ok: true, msg: t('Connected to Kaggle') },
          }))
        } else {
          setTestResults((p) => ({
            ...p,
            [acc.id]: {
              ok: false,
              msg: res.data?.message || t('Connection failed'),
            },
          }))
        }
      } else if (provider === 'colab') {
        const res = await api.post('/api/deployments/colab/test-connection', {
          account_id: acc.id,
        })
        if (res.data?.success) {
          const gpus: string[] = res.data?.data?.eligible_gpus ?? []
          setTestResults((p) => ({
            ...p,
            [acc.id]: {
              ok: true,
              msg:
                gpus.length > 0
                  ? `${t('Connected')} (${gpus.join(', ')})`
                  : t('Connected to Google Colab'),
            },
          }))
        } else {
          setTestResults((p) => ({
            ...p,
            [acc.id]: {
              ok: false,
              msg: res.data?.message || t('Connection failed'),
            },
          }))
        }
      } else if (provider === 'huggingface') {
        const res = await api.post(
          '/api/deployments/huggingface/test-connection',
          { token: acc.token === MASK ? '' : acc.token }
        )
        setTestResults((p) => ({
          ...p,
          [acc.id]: {
            ok: Boolean(res.data?.success),
            msg: res.data?.success
              ? t('Connected')
              : res.data?.message || t('Connection failed'),
          },
        }))
      } else if (provider === 'runpod') {
        const res = await api.post('/api/deployments/runpod/test-connection', {
          api_key: acc.api_key === MASK ? '' : acc.api_key,
        })
        setTestResults((p) => ({
          ...p,
          [acc.id]: {
            ok: Boolean(res.data?.success),
            msg: res.data?.success
              ? t('Connected')
              : res.data?.message || t('Connection failed'),
          },
        }))
      } else if (provider === 'hostinger') {
        const res = await api.post(
          '/api/deployments/hostinger/test-connection',
          { api_key: acc.api_key === MASK ? '' : acc.api_key }
        )
        setTestResults((p) => ({
          ...p,
          [acc.id]: {
            ok: Boolean(res.data?.success),
            msg: res.data?.success
              ? t('Connected')
              : res.data?.message || t('Connection failed'),
          },
        }))
      } else if (provider === 'ionet') {
        const res = await api.post('/api/deployments/settings/test-connection', {
          api_key: acc.api_key === MASK ? '' : acc.api_key,
        })
        setTestResults((p) => ({
          ...p,
          [acc.id]: {
            ok: Boolean(res.data?.success),
            msg: res.data?.success
              ? t('Connected')
              : res.data?.message || t('Connection failed'),
          },
        }))
      }
    } catch (e: any) {
      setTestResults((p) => ({
        ...p,
        [acc.id]: { ok: false, msg: e?.message || t('Connection failed') },
      }))
    } finally {
      setTestingId(null)
    }
  }

  const handleStartColabAccountOAuth = async (accountId: string) => {
    try {
      const res = await api.post('/api/deployments/colab/oauth/start')
      if (res.data?.success && res.data?.data) {
        setOauthTargetId(accountId)
        setOauthSessionId(res.data.data.session_id)
        setOauthUrl(res.data.data.auth_url)
        setOauthCallback('')
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

  const handleExchangeColabAccountOAuth = async (acc: EditableAccount) => {
    if (!oauthSessionId || !oauthCallback.trim()) return
    setOauthExchanging(true)
    try {
      const res = await api.post('/api/deployments/colab/oauth/exchange', {
        session_id: oauthSessionId,
        callback: oauthCallback.trim(),
        account_id: acc.id,
        label: acc.label,
        tier: acc.tier,
      })
      if (res.data?.success) {
        if (res.data?.data?.accounts) {
          onAccountsUpdated(provider, res.data.data.accounts)
        }
        setOauthTargetId(null)
        setOauthUrl('')
        setOauthCallback('')
        toast.success(t('Successfully connected to Google Colab'))
      } else {
        toast.error(
          res.data?.message || t('Failed to exchange OAuth credentials')
        )
      }
    } catch (e: any) {
      toast.error(e?.message || t('Failed to exchange OAuth credentials'))
    } finally {
      setOauthExchanging(false)
    }
  }

  const handleDisconnectColabAccount = async (accountId: string) => {
    try {
      const res = await api.post('/api/deployments/colab/oauth/disconnect', {
        account_id: accountId,
      })
      if (res.data?.data?.accounts) {
        onAccountsUpdated(provider, res.data.data.accounts)
      }
      toast.success(t('Google Colab disconnected'))
    } catch (e: any) {
      toast.error(e?.message || t('Disconnect failed'))
    }
  }

  return (
    <div className='rounded-lg border border-border/60 bg-muted/15 p-3 space-y-3'>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <div>
          <div className='text-sm font-medium'>
            {t('Multi-Account & Plan Tier Configuration')}
          </div>
          <div className='text-xs text-muted-foreground'>
            {t(
              'Add multiple accounts (#1, #2, ...) and set each account plan tier to filter eligible GPUs/TPUs.'
            )}
          </div>
        </div>
        <div className='flex items-center gap-2'>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={addAccount}
          >
            <Plus className='h-3.5 w-3.5 mr-1' />
            {t('Add Account')}
          </Button>
          <Button
            type='button'
            size='sm'
            disabled={saving}
            onClick={handleSaveAccounts}
          >
            {saving && <Loader2 className='h-3.5 w-3.5 mr-1.5 animate-spin' />}
            {t('Save Accounts')}
          </Button>
        </div>
      </div>

      {accounts.length === 0 ? (
        <div className='text-xs text-muted-foreground py-2'>
          {t('No extra accounts configured yet. Click "Add Account" to add #1, #2, etc.')}
        </div>
      ) : (
        <div className='space-y-2.5'>
          {accounts.map((acc, idx) => {
            const tr = testResults[acc.id]
            const tierMeta = tiers.find((x) => x.id === acc.tier)
            return (
              <div
                key={acc.id}
                className='rounded-md border bg-card p-3 space-y-2.5'
              >
                <div className='flex flex-wrap items-center justify-between gap-2'>
                  <div className='flex items-center gap-2'>
                    <Badge variant='secondary' className='font-mono text-xs'>
                      #{idx + 1}
                    </Badge>
                    <Input
                      value={acc.label}
                      onChange={(e) =>
                        updateField(acc.id, { label: e.target.value })
                      }
                      placeholder={`#${idx + 1}`}
                      className='h-7 w-32 text-xs'
                    />
                    {acc.configured && (
                      <Badge
                        variant='outline'
                        className='text-[10px] border-emerald-500/40 text-emerald-600 dark:text-emerald-400 bg-emerald-500/10'
                      >
                        {t('Configured')}
                      </Badge>
                    )}
                    {acc.email && (
                      <span className='text-xs text-muted-foreground'>
                        {acc.email}
                      </span>
                    )}
                  </div>
                  <div className='flex items-center gap-2'>
                    <Switch
                      checked={acc.enabled}
                      onCheckedChange={(checked) =>
                        updateField(acc.id, { enabled: checked })
                      }
                    />
                    <Button
                      type='button'
                      variant='ghost'
                      size='icon-sm'
                      className='text-destructive hover:text-destructive'
                      onClick={() => removeAccount(acc.id)}
                    >
                      <Trash2 className='h-3.5 w-3.5' />
                    </Button>
                  </div>
                </div>

                {tiers.length > 1 && (
                  <div className='space-y-1'>
                    <label className='text-[11px] font-medium text-muted-foreground'>
                      {t('Account Plan / Hardware Tier')}
                    </label>
                    <Select
                      value={acc.tier}
                      onValueChange={(v) =>
                        updateField(acc.id, { tier: v ?? defaultTier })
                      }
                    >
                      <SelectTrigger className='h-8 w-full text-xs'>
                        <SelectValue>
                          {tierMeta ? t(tierMeta.label) : acc.tier}
                        </SelectValue>
                      </SelectTrigger>
                      <SelectContent alignItemWithTrigger={false} align='start'>
                        {tiers.map((tier) => (
                          <SelectItem key={tier.id} value={tier.id}>
                            {t(tier.label)}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    {tierMeta && (
                      <p className='text-[11px] text-muted-foreground'>
                        {t(tierMeta.description)}
                      </p>
                    )}
                  </div>
                )}

                {provider === 'modal' && (
                  <div className='grid gap-2 sm:grid-cols-2'>
                    <Input
                      type='password'
                      placeholder='ak-...'
                      value={acc.token_id ?? ''}
                      onChange={(e) =>
                        updateField(acc.id, { token_id: e.target.value })
                      }
                      className='h-8 text-xs'
                    />
                    <Input
                      type='password'
                      placeholder='as-...'
                      value={acc.token_secret ?? ''}
                      onChange={(e) =>
                        updateField(acc.id, { token_secret: e.target.value })
                      }
                      className='h-8 text-xs'
                    />
                  </div>
                )}

                {provider === 'kaggle' && (
                  <div className='flex flex-wrap gap-2'>
                    <Input
                      placeholder={t('Kaggle Username')}
                      value={acc.username ?? ''}
                      onChange={(e) =>
                        updateField(acc.id, { username: e.target.value })
                      }
                      className='h-8 text-xs flex-1 min-w-[140px]'
                    />
                    <Input
                      type='password'
                      placeholder='KGAT_... or kaggle.json key'
                      value={acc.api_key ?? ''}
                      onChange={(e) =>
                        updateField(acc.id, { api_key: e.target.value })
                      }
                      className='h-8 text-xs flex-1 min-w-[180px]'
                    />
                    <Button
                      type='button'
                      variant='outline'
                      size='sm'
                      className='h-8 text-xs'
                      disabled={testingId === acc.id}
                      onClick={() => handleTestAccount(acc)}
                    >
                      {testingId === acc.id && (
                        <Loader2 className='h-3 w-3 mr-1 animate-spin' />
                      )}
                      {t('Test')}
                    </Button>
                  </div>
                )}

                {provider === 'huggingface' && (
                  <div className='flex flex-wrap gap-2'>
                    <Input
                      type='password'
                      placeholder='hf_...'
                      value={acc.token ?? ''}
                      onChange={(e) =>
                        updateField(acc.id, { token: e.target.value })
                      }
                      className='h-8 text-xs flex-1 min-w-[180px]'
                    />
                    <Input
                      placeholder={t('Organization (Optional)')}
                      value={acc.org ?? ''}
                      onChange={(e) =>
                        updateField(acc.id, { org: e.target.value })
                      }
                      className='h-8 text-xs w-40'
                    />
                    <Button
                      type='button'
                      variant='outline'
                      size='sm'
                      className='h-8 text-xs'
                      disabled={testingId === acc.id}
                      onClick={() => handleTestAccount(acc)}
                    >
                      {testingId === acc.id && (
                        <Loader2 className='h-3 w-3 mr-1 animate-spin' />
                      )}
                      {t('Test')}
                    </Button>
                  </div>
                )}

                {(provider === 'runpod' ||
                  provider === 'hostinger' ||
                  provider === 'ionet') && (
                  <div className='flex gap-2'>
                    <Input
                      type='password'
                      placeholder={t('API Key')}
                      value={acc.api_key ?? ''}
                      onChange={(e) =>
                        updateField(acc.id, { api_key: e.target.value })
                      }
                      className='h-8 text-xs flex-1'
                    />
                    <Button
                      type='button'
                      variant='outline'
                      size='sm'
                      className='h-8 text-xs'
                      disabled={testingId === acc.id}
                      onClick={() => handleTestAccount(acc)}
                    >
                      {testingId === acc.id && (
                        <Loader2 className='h-3 w-3 mr-1 animate-spin' />
                      )}
                      {t('Test')}
                    </Button>
                  </div>
                )}

                {provider === 'colab' && (
                  <div className='space-y-2'>
                    <div className='flex flex-wrap items-center gap-2'>
                      <Button
                        type='button'
                        variant='outline'
                        size='sm'
                        className='h-7 text-xs'
                        onClick={() => handleStartColabAccountOAuth(acc.id)}
                      >
                        <Wand2 className='h-3 w-3 mr-1' />
                        {t('OAuth Login')}
                      </Button>
                      {acc.configured && (
                        <>
                          <Button
                            type='button'
                            variant='outline'
                            size='sm'
                            className='h-7 text-xs'
                            disabled={testingId === acc.id}
                            onClick={() => handleTestAccount(acc)}
                          >
                            {testingId === acc.id && (
                              <Loader2 className='h-3 w-3 mr-1 animate-spin' />
                            )}
                            {t('Test')}
                          </Button>
                          <Button
                            type='button'
                            variant='ghost'
                            size='sm'
                            className='h-7 text-xs text-destructive hover:text-destructive'
                            onClick={() => handleDisconnectColabAccount(acc.id)}
                          >
                            <LogOut className='h-3 w-3 mr-1' />
                            {t('Disconnect')}
                          </Button>
                        </>
                      )}
                    </div>

                    {oauthTargetId === acc.id && oauthUrl && (
                      <div className='space-y-1.5 pt-1 border-t'>
                        <a
                          href={oauthUrl}
                          target='_blank'
                          rel='noopener noreferrer'
                          className='text-[11px] text-primary hover:underline block'
                        >
                          {t(
                            'If the window did not open, click here to log in (New Tab)'
                          )}
                        </a>
                        <div className='flex gap-2'>
                          <Input
                            placeholder='http://localhost:8085/callback?code=...'
                            value={oauthCallback}
                            onChange={(e) => setOauthCallback(e.target.value)}
                            className='h-8 text-xs flex-1'
                          />
                          <Button
                            type='button'
                            size='sm'
                            className='h-8 text-xs'
                            disabled={oauthExchanging || !oauthCallback.trim()}
                            onClick={() => handleExchangeColabAccountOAuth(acc)}
                          >
                            {oauthExchanging && (
                              <Loader2 className='h-3 w-3 mr-1 animate-spin' />
                            )}
                            {t('Verify and Get')}
                          </Button>
                        </div>
                      </div>
                    )}
                  </div>
                )}

                {tr && (
                  <p
                    className={`flex items-center gap-1 text-xs ${
                      tr.ok
                        ? 'text-emerald-600 dark:text-emerald-400'
                        : 'text-destructive'
                    }`}
                  >
                    {tr.ok ? (
                      <CheckCircle2 className='h-3.5 w-3.5' />
                    ) : (
                      <XCircle className='h-3.5 w-3.5' />
                    )}
                    {tr.msg}
                  </p>
                )}
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
