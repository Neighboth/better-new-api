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
import { CheckCircle2, ExternalLink, Loader2, ShieldAlert } from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { IconDiscord } from '@/assets/brand-icons'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { createOAuthFlow } from '@/features/auth/api'
import {
  OAUTH_BIND_CALLBACK_MESSAGE,
  OAUTH_BIND_RESULT_MESSAGE,
} from '@/features/auth/constants'
import { watchOAuthPopupClosed } from '@/features/auth/lib/oauth-bind-window'
import {
  getOAuthSessionStorage,
  markOAuthBindPopup,
} from '@/features/auth/lib/oauth-callback-mode'
import { api } from '@/lib/api'
import { buildDiscordOAuthUrl } from '@/lib/oauth'
import { cn } from '@/lib/utils'

export interface StudioDiscordGateStatus {
  active: boolean
  prerequisites_ready: boolean
  bot_enabled: boolean
  oauth_enabled: boolean
  gated_models: string[]
  invite_url: string
  guild_id: string
  discord_client_id: string
  discord_linked: boolean
  in_server: boolean
  check_error?: string
}

interface PendingDiscordBinding {
  state: string
  popup: Window
  stopCloseWatcher: () => void
}

interface OAuthBindingCallback {
  type: typeof OAUTH_BIND_CALLBACK_MESSAGE
  provider: string
  state: string
  code?: string
  error?: string
  errorDescription?: string
}

interface StudioDiscordGateDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  modelDisplayName: string
  status: StudioDiscordGateStatus | null
  onRefreshStatus: () => void | Promise<void>
}

export function StudioDiscordGateDialog({
  open,
  onOpenChange,
  modelDisplayName,
  status,
  onRefreshStatus,
}: StudioDiscordGateDialogProps) {
  const { t } = useTranslation()
  const [connecting, setConnecting] = useState(false)
  const pendingRef = useRef<PendingDiscordBinding | null>(null)

  const clearPending = useCallback((expected?: PendingDiscordBinding) => {
    const pending = pendingRef.current
    if (!pending || (expected && pending !== expected)) return
    pending.stopCloseWatcher()
    pendingRef.current = null
    setConnecting(false)
  }, [])

  useEffect(() => {
    if (typeof window === 'undefined') return
    const handleMessage = async (event: MessageEvent<unknown>) => {
      if (event.origin !== window.location.origin) return
      const message = event.data as Partial<OAuthBindingCallback> | null
      const pending = pendingRef.current
      if (
        !message ||
        message.type !== OAUTH_BIND_CALLBACK_MESSAGE ||
        !pending ||
        message.provider !== 'discord' ||
        message.state !== pending.state ||
        event.source !== pending.popup
      ) {
        return
      }

      clearPending(pending)
      let success = false
      let resultMessage = t('OAuth failed')
      try {
        if (!message.code && !message.error) {
          throw new Error(t('Missing code'))
        }
        const params: Record<string, string> = { state: message.state }
        if (message.code) params.code = message.code
        if (message.error) params.error = message.error
        if (message.errorDescription) {
          params.error_description = message.errorDescription
        }
        const response = await api.get('/api/oauth/discord', {
          params,
          skipBusinessError: true,
        })
        success = Boolean(response.data?.success)
        resultMessage = response.data?.message || resultMessage
        if (success) {
          toast.success(t('Discord account connected successfully!'))
          await onRefreshStatus()
        } else {
          toast.error(resultMessage)
        }
      } catch (err: unknown) {
        resultMessage =
          (err as { response?: { data?: { message?: string } } })?.response
            ?.data?.message ||
          (err instanceof Error ? err.message : resultMessage)
        toast.error(resultMessage)
      }

      pending.popup.postMessage(
        {
          type: OAUTH_BIND_RESULT_MESSAGE,
          provider: 'discord',
          state: message.state,
          success,
          message: resultMessage,
        },
        window.location.origin
      )
    }

    window.addEventListener('message', handleMessage)
    return () => window.removeEventListener('message', handleMessage)
  }, [clearPending, onRefreshStatus, t])

  useEffect(
    () => () => {
      const pending = pendingRef.current
      clearPending(pending ?? undefined)
      if (pending && !pending.popup.closed) pending.popup.close()
    },
    [clearPending]
  )

  const handleConnectDiscord = async () => {
    const clientId = status?.discord_client_id?.trim()
    if (!clientId) {
      toast.error(t('Discord OAuth is not configured by the administrator'))
      return
    }
    const previous = pendingRef.current
    if (previous) {
      clearPending(previous)
      if (!previous.popup.closed) previous.popup.close()
    }
    const popup = window.open('', '_blank')
    if (!popup) {
      toast.error(t('OAuth pop-up was blocked'))
      return
    }
    setConnecting(true)
    const pending: PendingDiscordBinding = {
      state: '',
      popup,
      stopCloseWatcher: () => undefined,
    }
    pending.stopCloseWatcher = watchOAuthPopupClosed(popup, () =>
      clearPending(pending)
    )
    pendingRef.current = pending
    try {
      const state = await createOAuthFlow('discord', 'bind')
      if (pendingRef.current !== pending || popup.closed) return
      if (!markOAuthBindPopup(getOAuthSessionStorage(popup), 'discord', state)) {
        throw new Error('OAuth bind popup storage is unavailable')
      }
      pending.state = state
      popup.location.replace(buildDiscordOAuthUrl(clientId, state))
    } catch {
      const isCurrent = pendingRef.current === pending
      clearPending(pending)
      if (!popup.closed) popup.close()
      if (isCurrent) toast.error(t('Failed to initialize OAuth'))
    }
  }

  const discordLinked = Boolean(status?.discord_linked)
  const inServer = Boolean(status?.in_server)
  const inviteUrl = status?.invite_url?.trim() || ''

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-md'>
        <DialogHeader>
          <DialogTitle className='flex items-center gap-2 text-base'>
            <IconDiscord className='size-5 text-[#5865F2]' />
            {t('Discord Server Required')}
          </DialogTitle>
          <DialogDescription>
            {t(
              'To generate with "{{model}}", please connect your Discord account and join our Discord server.',
              { model: modelDisplayName }
            )}
          </DialogDescription>
        </DialogHeader>

        <div className='space-y-3 py-2'>
          {/* Step 1: Connect Discord Account */}
          <div
            className={cn(
              'rounded-xl border p-3.5 transition-colors',
              discordLinked
                ? 'border-emerald-500/30 bg-emerald-500/5'
                : 'border-border bg-muted/20'
            )}
          >
            <div className='flex items-start justify-between gap-3'>
              <div className='space-y-1'>
                <div className='flex items-center gap-2 text-sm font-medium'>
                  {discordLinked ? (
                    <CheckCircle2 className='size-4 shrink-0 text-emerald-600 dark:text-emerald-400' />
                  ) : (
                    <span className='bg-primary/10 text-primary flex size-5 shrink-0 items-center justify-center rounded-full text-xs font-semibold'>
                      1
                    </span>
                  )}
                  <span>{t('Connect your Discord account')}</span>
                </div>
                <p className='text-muted-foreground pl-7 text-xs'>
                  {discordLinked
                    ? t('Your Discord account is connected.')
                    : t(
                        'Link your Discord account so we can verify your server membership automatically.'
                      )}
                </p>
              </div>
              {!discordLinked && (
                <Button
                  type='button'
                  size='sm'
                  disabled={connecting}
                  onClick={() => void handleConnectDiscord()}
                  className='shrink-0 gap-1.5'
                >
                  {connecting ? (
                    <Loader2 className='size-3.5 animate-spin' />
                  ) : (
                    <IconDiscord className='size-3.5' />
                  )}
                  {t('Connect Discord')}
                </Button>
              )}
            </div>
          </div>

          {/* Step 2: Join Discord Server */}
          <div
            className={cn(
              'rounded-xl border p-3.5 transition-colors',
              inServer
                ? 'border-emerald-500/30 bg-emerald-500/5'
                : discordLinked
                  ? 'border-border bg-muted/20'
                  : 'border-border/60 bg-muted/10 opacity-70'
            )}
          >
            <div className='flex items-start justify-between gap-3'>
              <div className='space-y-1'>
                <div className='flex items-center gap-2 text-sm font-medium'>
                  {inServer ? (
                    <CheckCircle2 className='size-4 shrink-0 text-emerald-600 dark:text-emerald-400' />
                  ) : (
                    <span className='bg-primary/10 text-primary flex size-5 shrink-0 items-center justify-center rounded-full text-xs font-semibold'>
                      2
                    </span>
                  )}
                  <span>{t('Join our Discord server')}</span>
                </div>
                <p className='text-muted-foreground pl-7 text-xs'>
                  {inServer
                    ? t('Server membership verified!')
                    : t(
                        'Join the server using the invite link below. Membership is checked automatically every second.'
                      )}
                </p>
              </div>
              {!inServer && inviteUrl && (
                <Button
                  type='button'
                  size='sm'
                  variant={discordLinked ? 'default' : 'outline'}
                  disabled={!discordLinked}
                  onClick={() =>
                    window.open(inviteUrl, '_blank', 'noopener,noreferrer')
                  }
                  className='shrink-0 gap-1.5'
                >
                  <ExternalLink className='size-3.5' />
                  {t('Join Server')}
                </Button>
              )}
            </div>
          </div>

          {!inServer && (
            <div className='text-muted-foreground flex items-center gap-2 px-1 text-xs'>
              <Loader2 className='size-3.5 shrink-0 animate-spin' />
              <span>
                {discordLinked
                  ? t('Waiting for you to join the Discord server — checking every second...')
                  : t('Waiting for Discord account connection — checking every second...')}
              </span>
            </div>
          )}

          {status?.check_error && (
            <div className='text-amber-700 dark:text-amber-400 flex items-start gap-2 rounded-lg bg-amber-500/10 px-3 py-2 text-xs'>
              <ShieldAlert className='mt-0.5 size-3.5 shrink-0' />
              <span>{t('Temporarily unable to verify with Discord; retrying...')}</span>
            </div>
          )}
        </div>

        <DialogFooter>
          <Button
            type='button'
            variant='outline'
            onClick={() => onOpenChange(false)}
          >
            {t('Close')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
