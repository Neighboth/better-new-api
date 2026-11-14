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
import { useQuery } from '@tanstack/react-query'
import {
  ArrowLeft,
  BadgeCheck,
  Coins,
  Headphones,
  MonitorSmartphone,
  RefreshCw,
  ShieldCheck,
  Sparkles,
  Ticket,
  Users,
  UsersRound,
  Wallet,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useNavigate, useParams } from '@tanstack/react-router'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Separator } from '@/components/ui/separator'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { LOG_TYPES, LOG_TYPE_ENUM } from '@/features/usage-logs/constants'
import { USER_ROLES, USER_STATUS } from '@/features/users/constants'
import {
  formatQuota,
  formatTimestamp,
  formatTimestampRelative,
} from '@/lib/format'
import { cn } from '@/lib/utils'

import {
  getUserDetail,
  type UserDetailPayload,
  type UserDetailResponse,
} from './api'

const LOG_TYPE_LABEL: Record<number, string> = Object.fromEntries(
  LOG_TYPES.map((entry) => [entry.value, entry.label])
)

function DetailRow(props: { label: string; value: React.ReactNode; mono?: boolean }) {
  return (
    <div className='grid grid-cols-[130px_minmax(0,1fr)] items-baseline gap-3 py-1.5 text-sm'>
      <dt className='text-muted-foreground text-xs'>{props.label}</dt>
      <dd className={cn('min-w-0 break-words', props.mono && 'font-mono text-xs')}>
        {props.value}
      </dd>
    </div>
  )
}

function StatChip(props: {
  icon: React.ReactNode
  label: string
  value: React.ReactNode
}) {
  return (
    <div className='bg-card flex flex-col items-center gap-1 rounded-xl border p-3 text-center shadow-sm'>
      <span className='text-primary'>{props.icon}</span>
      <span className='text-lg leading-6 font-semibold tabular-nums'>{props.value}</span>
      <span className='text-muted-foreground text-[11px]'>{props.label}</span>
    </div>
  )
}

export function UserDetail() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { id: rawId } = useParams({ from: '/_authenticated/users/$id' })
  const userId = Number(rawId)

  const { data, isLoading, isError, refetch, isFetching } = useQuery({
    queryKey: ['user-detail', userId],
    queryFn: async (): Promise<UserDetailPayload | null> => {
      const res: UserDetailResponse = await getUserDetail(userId)
      if (!res.success || !res.data) {
        throw new Error(res.message || t('User could not be loaded.'))
      }
      return res.data
    },
    enabled: Number.isFinite(userId) && userId > 0,
    staleTime: 15_000,
  })

  if (isLoading) {
    return (
      <div className='flex h-full items-center justify-center p-8'>
        <div className='text-muted-foreground flex items-center gap-2 text-sm'>
          <RefreshCw className='size-4 animate-spin' />
          {t('Loading user…')}
        </div>
      </div>
    )
  }

  if (isError || !data) {
    return (
      <div className='flex h-full flex-col items-center justify-center gap-4 p-8'>
        <p className='text-muted-foreground text-sm'>{t('User could not be loaded.')}</p>
        <Button variant='outline' onClick={() => navigate({ to: '/users' })}>
          <ArrowLeft className='size-4' />
          {t('Back to users')}
        </Button>
      </div>
    )
  }

  const user = data.user
  const roleConfig = USER_ROLES[user.role as keyof typeof USER_ROLES]
  const isDeleted = Boolean(user.DeletedAt)
  const requestsBalance = user.requests_balance
  const tokensBalance = user.tokens_balance
  const totalReferral = (user.referral_quota ?? 0) + (user.referral_display ?? 0)

  return (
    <div className='flex h-full min-h-0 w-full flex-col overflow-y-auto overscroll-contain'>
      <div className='mx-auto w-full max-w-6xl space-y-5 p-4 pb-16 md:p-6'>
        {/* Top bar */}
        <div className='flex items-center justify-between gap-2'>
          <Button
            variant='ghost'
            size='sm'
            className='-ms-2'
            onClick={() => navigate({ to: '/users' })}
          >
            <ArrowLeft className='size-4' />
            {t('Back to users')}
          </Button>
          <Button
            variant='outline'
            size='sm'
            onClick={() => void refetch()}
            disabled={isFetching}
          >
            <RefreshCw className={cn('size-4', isFetching && 'animate-spin')} />
            {t('Refresh')}
          </Button>
        </div>

        {/* Hero */}
        <div className='from-primary/10 via-background to-background relative overflow-hidden rounded-2xl border bg-gradient-to-br p-5'>
          <div
            aria-hidden='true'
            className='pointer-events-none absolute -top-16 end-6 size-48 rounded-full bg-primary/15 blur-3xl'
          />
          <div className='relative flex flex-wrap items-center gap-4'>
            <div className='from-primary to-primary/60 text-primary-foreground flex size-16 shrink-0 items-center justify-center rounded-2xl bg-gradient-to-br font-mono text-2xl font-bold shadow-md shadow-primary/25'>
              {user.username.slice(0, 2).toUpperCase()}
            </div>
            <div className='min-w-0 flex-1 space-y-1.5'>
              <div className='flex flex-wrap items-center gap-2'>
                <h1 className='text-xl font-semibold tracking-tight'>
                  {user.display_name || user.username}
                </h1>
                {roleConfig && (
                  <Badge variant='secondary' className='gap-1'>
                    <roleConfig.icon size={12} />
                    {t(roleConfig.labelKey)}
                  </Badge>
                )}
                <Badge
                  variant='outline'
                  className={cn(
                    isDeleted
                      ? 'text-destructive border-destructive/40'
                      : user.status === USER_STATUS.ENABLED
                        ? 'text-emerald-600'
                        : 'text-muted-foreground'
                  )}
                >
                  {isDeleted
                    ? t('Deleted')
                    : user.status === USER_STATUS.ENABLED
                      ? t('Enabled')
                      : t('Disabled')}
                </Badge>
                {data.two_fa_enabled && (
                  <Badge variant='outline' className='gap-1 text-emerald-600'>
                    <ShieldCheck className='size-3' />
                    2FA
                  </Badge>
                )}
              </div>
              <p className='text-muted-foreground font-mono text-xs'>
                @{user.username} · #{user.id}
                {user.email ? ` · ${user.email}` : ''}
              </p>
              {user.remark && (
                <p className='text-muted-foreground bg-muted/50 inline-block max-w-full truncate rounded-md px-2 py-1 text-xs'>
                  {user.remark}
                </p>
              )}
            </div>
            <div className='flex shrink-0 flex-col items-end gap-1 text-xs'>
              <span className='text-muted-foreground'>{t('Created At')}</span>
              <span>{user.created_at ? formatTimestamp(user.created_at) : '—'}</span>
              <span className='text-muted-foreground mt-1'>{t('Last Login')}</span>
              <span>
                {user.last_login_at ? formatTimestampRelative(user.last_login_at) : '—'}
              </span>
            </div>
          </div>
        </div>

        {/* Stats strip */}
        <div className='grid grid-cols-2 gap-3 sm:grid-cols-3 xl:grid-cols-6'>
          <StatChip
            icon={<Users className='size-5' />}
            label={t('Requests')}
            value={String(user.request_count ?? 0)}
          />
          <StatChip
            icon={<Coins className='size-5' />}
            label={t('Remaining quota')}
            value={formatQuota(user.quota ?? 0)}
          />
          <StatChip
            icon={<Wallet className='size-5' />}
            label={t('Used quota')}
            value={formatQuota(user.used_quota ?? 0)}
          />
          <StatChip
            icon={<UsersRound className='size-5' />}
            label={t('Invited users')}
            value={String(user.aff_count ?? 0)}
          />
          <StatChip
            icon={<Headphones className='size-5' />}
            label={t('Tickets')}
            value={String(data.tickets_count)}
          />
          <StatChip
            icon={<MonitorSmartphone className='size-5' />}
            label={t('Active sessions')}
            value={String(data.active_sessions_count)}
          />
        </div>

        <div className='grid items-start gap-4 xl:grid-cols-2'>
          {/* Account */}
          <Card>
            <CardHeader className='pb-3'>
              <CardTitle className='flex items-center gap-2 text-sm'>
                <BadgeCheck className='text-primary size-4' />
                {t('Account')}
              </CardTitle>
            </CardHeader>
            <CardContent className='pt-0'>
              <dl className='divide-border divide-y'>
                <DetailRow label={t('ID')} value={<span className='font-mono'>{user.id}</span>} />
                <DetailRow label={t('Username')} value={<span className='font-mono'>{user.username}</span>} />
                <DetailRow label={t('Display name')} value={user.display_name || '—'} />
                <DetailRow label={t('Email')} value={user.email || '—'} />
                <DetailRow label={t('Group')} value={<span className='font-mono'>{user.group || 'default'}</span>} />
                <DetailRow
                  label={t('Role')}
                  value={roleConfig ? t(roleConfig.labelKey) : String(user.role)}
                />
                <DetailRow
                  label={t('Created At')}
                  value={user.created_at ? formatTimestamp(user.created_at) : '—'}
                />
                <DetailRow
                  label={t('Last Login')}
                  value={user.last_login_at ? formatTimestamp(user.last_login_at) : '—'}
                />
                <DetailRow
                  label={t('Affiliate code')}
                  value={<span className='font-mono'>{user.aff_code || '—'}</span>}
                />
                <DetailRow
                  label={t('Stripe customer')}
                  value={<span className='font-mono'>{user.stripe_customer || '—'}</span>}
                />
                {user.github_id && <DetailRow label={t('GitHub')} value={<span className='font-mono'>{user.github_id}</span>} />}
                {user.oidc_id && <DetailRow label={t('OIDC')} value={<span className='font-mono'>{user.oidc_id}</span>} />}
                {user.wechat_id && <DetailRow label={t('WeChat')} value={<span className='font-mono'>{user.wechat_id}</span>} />}
                {user.telegram_id && <DetailRow label={t('Telegram')} value={<span className='font-mono'>{user.telegram_id}</span>} />}
                {user.linux_do_id && <DetailRow label={t('Linux.do')} value={<span className='font-mono'>{user.linux_do_id}</span>} />}
                <DetailRow
                  label={t('Deleted')}
                  value={isDeleted ? t('Yes') : t('No')}
                />
              </dl>
            </CardContent>
          </Card>

          {/* Quota & balance */}
          <Card>
            <CardHeader className='pb-3'>
              <CardTitle className='flex items-center gap-2 text-sm'>
                <Coins className='text-primary size-4' />
                {t('Quota & balance')}
              </CardTitle>
            </CardHeader>
            <CardContent className='pt-0'>
              <dl className='divide-border divide-y'>
                <DetailRow
                  label={t('Remaining quota')}
                  value={<span className='font-semibold'>{formatQuota(user.quota ?? 0)}</span>}
                />
                <DetailRow
                  label={t('Used quota')}
                  value={formatQuota(user.used_quota ?? 0)}
                />
                <DetailRow
                  label={t('Total quota')}
                  value={formatQuota((user.quota ?? 0) + (user.used_quota ?? 0))}
                />
                <DetailRow
                  label={t('Requests')}
                  value={String(user.request_count ?? 0)}
                />
                {requestsBalance !== undefined && (
                  <DetailRow label={t('Requests balance')} value={String(requestsBalance)} />
                )}
                {tokensBalance !== undefined && (
                  <DetailRow label={t('Tokens balance')} value={String(tokensBalance)} />
                )}
                <Separator className='my-2' />
                <DetailRow
                  label={t('Referral balance')}
                  value={
                    <span>
                      {formatQuota(user.referral_quota ?? 0)}
                      <span className='text-muted-foreground'> / {formatQuota(user.referral_display ?? 0)}</span>
                    </span>
                  }
                />
                <DetailRow
                  label={t('Referral (available to spend)')}
                  value={formatQuota(user.aff_quota ?? 0)}
                />
                <DetailRow
                  label={t('Invitation revenue (all time)')}
                  value={formatQuota(user.aff_history_quota ?? 0)}
                />
                <DetailRow
                  label={t('Total referral balance')}
                  value={formatQuota(totalReferral)}
                />
                {(user.inviter_id ?? 0) > 0 && (
                  <DetailRow
                    label={t('Invited by')}
                    value={
                      <button
                        className='text-primary font-mono underline-offset-2 hover:underline'
                        onClick={() => navigate({ to: '/users/$id', params: { id: String(user.inviter_id) } })}
                        type='button'
                      >
                        #{user.inviter_id}
                      </button>
                    }
                  />
                )}
              </dl>
            </CardContent>
          </Card>

          {/* Security */}
          <Card>
            <CardHeader className='pb-3'>
              <CardTitle className='flex items-center gap-2 text-sm'>
                <ShieldCheck className='text-primary size-4' />
                {t('Security')}
              </CardTitle>
            </CardHeader>
            <CardContent className='space-y-3 pt-0'>
              <dl className='divide-border divide-y'>
                <DetailRow
                  label={t('Two-factor authentication')}
                  value={
                    data.two_fa_enabled ? (
                      <span className='inline-flex items-center gap-1 font-medium text-emerald-600'>
                        <BadgeCheck className='size-3.5' />
                        {t('Enabled')}
                      </span>
                    ) : (
                      <span className='text-muted-foreground'>{t('Disabled')}</span>
                    )
                  }
                />
                <DetailRow
                  label={t('Active sessions')}
                  value={String(data.active_sessions_count)}
                />
              </dl>
              {data.active_sessions.length > 0 && (
                <div className='rounded-lg border'>
                  <div className='text-muted-foreground border-b bg-muted/40 px-3 py-1.5 text-[11px] font-medium'>
                    {t('Active sessions')}
                  </div>
                  <ul className='divide-y'>
                    {data.active_sessions.map((session) => (
                      <li key={session.sid} className='px-3 py-2 text-xs'>
                        <div className='flex flex-wrap items-center justify-between gap-2'>
                          <span className='font-mono'>{session.sid.slice(0, 8)}…</span>
                          <span className='text-muted-foreground'>{session.ip || '—'}</span>
                        </div>
                        <div className='mt-0.5 line-clamp-1 font-mono text-[11px] text-muted-foreground'>
                          {session.login_method} · {session.user_agent}
                        </div>
                        <div className='mt-0.5 flex justify-between text-[11px] text-muted-foreground'>
                          <span>{t('Last active')}: {formatTimestampRelative(session.last_active_at)}</span>
                          <span>{t('Expires')}: {formatTimestamp(session.expires_at)}</span>
                        </div>
                      </li>
                    ))}
                  </ul>
                </div>
              )}
              {data.oauth_bindings.length > 0 && (
                <div className='rounded-lg border'>
                  <div className='text-muted-foreground border-b bg-muted/40 px-3 py-1.5 text-[11px] font-medium'>
                    {t('OAuth bindings')}
                  </div>
                  <ul className='divide-y'>
                    {data.oauth_bindings.map((binding) => (
                      <li
                        key={binding.id}
                        className='flex flex-wrap items-center justify-between gap-2 px-3 py-2 text-xs'
                      >
                        <span className='font-mono'>{t('Provider')} #{binding.provider_id}</span>
                        <span className='max-w-[45%] truncate font-mono text-[11px]'>
                          {binding.provider_user_id}
                        </span>
                        <span className='text-muted-foreground text-[11px]'>
                          {binding.created_at ? formatTimestamp(Date.parse(binding.created_at) / 1000) : '—'}
                        </span>
                      </li>
                    ))}
                  </ul>
                </div>
              )}
            </CardContent>
          </Card>

          {/* Subscriptions */}
          <Card>
            <CardHeader className='pb-3'>
              <CardTitle className='flex items-center gap-2 text-sm'>
                <Ticket className='text-primary size-4' />
                {t('Subscriptions')}
              </CardTitle>
            </CardHeader>
            <CardContent className='pt-0'>
              {data.subscriptions.length === 0 ? (
                <p className='text-muted-foreground text-sm'>
                  {t('This user has no subscriptions.')}
                </p>
              ) : (
                <ul className='space-y-2'>
                  {data.subscriptions.map((entry) => {
                    const sub = entry.subscription
                    const used = sub.amount_total > 0 ? Math.round((sub.amount_used / sub.amount_total) * 100) : 0
                    return (
                      <li key={sub.id} className='rounded-lg border p-3 text-xs'>
                        <div className='flex flex-wrap items-center justify-between gap-2'>
                          <span className='font-medium'>
                            {t('Plan')} #{sub.plan_id}
                          </span>
                          <Badge
                            variant={sub.status === 'active' ? 'outline' : 'secondary'}
                            className={cn(sub.status === 'active' && 'text-emerald-600')}
                          >
                            {sub.status}
                          </Badge>
                        </div>
                        <div className='mt-2 grid grid-cols-2 gap-2 text-muted-foreground sm:grid-cols-4'>
                          <div>
                            <div className='text-[10px] uppercase'>{t('Quota')}</div>
                            <div className='text-foreground font-mono'>{used}%</div>
                          </div>
                          <div>
                            <div className='text-[10px] uppercase'>{t('Source')}</div>
                            <div className='text-foreground font-mono'>{sub.source}</div>
                          </div>
                          <div>
                            <div className='text-[10px] uppercase'>{t('Starts')}</div>
                            <div className='text-foreground font-mono'>{formatTimestamp(sub.start_time)}</div>
                          </div>
                          <div>
                            <div className='text-[10px] uppercase'>{t('Ends')}</div>
                            <div className='text-foreground font-mono'>{formatTimestamp(sub.end_time)}</div>
                          </div>
                        </div>
                      </li>
                    )
                  })}
                </ul>
              )}
            </CardContent>
          </Card>
        </div>

        {/* Admin permissions */}
        {user.admin_permissions && Object.keys(user.admin_permissions).length > 0 && (
          <Card>
            <CardHeader className='pb-3'>
              <CardTitle className='flex items-center gap-2 text-sm'>
                <Sparkles className='text-primary size-4' />
                {t('Admin permissions')}
              </CardTitle>
            </CardHeader>
            <CardContent className='pt-0'>
              <div className='flex flex-wrap gap-1.5'>
                {Object.entries(user.admin_permissions).flatMap(([resource, actions]) =>
                  Object.entries(actions)
                    .filter(([, granted]) => granted)
                    .map(([action]) => (
                      <Badge key={`${resource}:${action}`} variant='secondary' className='font-mono text-[10px]'>
                        {resource}:{action}
                      </Badge>
                    ))
                )}
              </div>
            </CardContent>
          </Card>
        )}

        {/* Recent usage logs */}
        <Card>
          <CardHeader className='flex-row items-center justify-between gap-2 pb-3'>
            <CardTitle className='flex items-center gap-2 text-sm'>
              <Coins className='text-primary size-4' />
              {t('Recent usage')}
            </CardTitle>
            <span className='text-muted-foreground text-xs tabular-nums'>
              {t('Last {{count}} of {{total}}', { count: data.recent_logs.length, total: data.recent_logs_total })}
            </span>
          </CardHeader>
          <CardContent className='pt-0'>
            {data.recent_logs.length === 0 ? (
              <p className='text-muted-foreground text-sm'>{t('No usage logs yet.')}</p>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('Time')}</TableHead>
                    <TableHead>{t('Type')}</TableHead>
                    <TableHead>{t('Model')}</TableHead>
                    <TableHead>{t('Channel')}</TableHead>
                    <TableHead className='text-right'>{t('Quota')}</TableHead>
                    <TableHead className='text-right'>{t('Tokens')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {data.recent_logs.map((log) => (
                    <TableRow key={log.id}>
                      <TableCell className='text-xs whitespace-nowrap'>
                        {formatTimestamp(log.created_at)}
                      </TableCell>
                      <TableCell>
                        <Badge variant='outline' className='text-[10px]'>
                          {LOG_TYPE_LABEL[log.type] ?? LOG_TYPE_ENUM.UNKNOWN}
                        </Badge>
                      </TableCell>
                      <TableCell className='max-w-[180px] truncate font-mono text-xs' title={log.model_name}>
                        {log.model_name || '—'}
                      </TableCell>
                      <TableCell className='max-w-[140px] truncate font-mono text-xs'>
                        {log.channel_name || (log.token_name ? `#${log.token_id ?? ''}` : '—')}
                      </TableCell>
                      <TableCell className='text-right font-mono text-xs'>
                        {formatQuota(log.quota)}
                      </TableCell>
                      <TableCell className='text-right font-mono text-xs'>
                        {(log.prompt_tokens ?? 0) + (log.completion_tokens ?? 0)}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
