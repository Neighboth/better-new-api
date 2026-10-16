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
import { KeyRound, Loader2, LogIn } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'

import { OAuthProviders } from '../components/oauth-providers'
import type { SystemStatus } from '../types'

type OtherLoginDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  variant: 'sign-in' | 'sign-up'
  status: SystemStatus | null
  redirectTo?: string
  disabled?: boolean
  passkeyEnabled?: boolean
  passkeySupported?: boolean
  isPasskeyLoading?: boolean
  onPasskey?: () => void
  onWeChatLogin?: () => void
  isWeChatLoading?: boolean
}

/**
 * Popup listing the alternative sign-in/sign-up methods (OAuth providers,
 * Passkey). Opened from the "Other sign-in options" button below the auth
 * forms so the main form stays minimal.
 */
export function OtherLoginDialog({
  open,
  onOpenChange,
  variant,
  status,
  redirectTo,
  disabled,
  passkeyEnabled,
  passkeySupported,
  isPasskeyLoading,
  onPasskey,
  onWeChatLogin,
  isWeChatLoading,
}: OtherLoginDialogProps) {
  const { t } = useTranslation()

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={t('Other sign-in options')}
      description={t('Choose one of the available methods below.')}
      contentClassName='max-w-sm'
      headerClassName='text-left'
      contentHeight='auto'
      bodyClassName='space-y-4'
    >
      {variant === 'sign-in' && passkeyEnabled && (
        <div className='space-y-1'>
          <Button
            type='button'
            variant='outline'
            disabled={disabled || isPasskeyLoading || !passkeySupported}
            onClick={onPasskey}
            className='h-11 w-full justify-center gap-2 rounded-lg'
          >
            {isPasskeyLoading ? (
              <Loader2 className='h-4 w-4 animate-spin' />
            ) : (
              <KeyRound className='h-4 w-4' />
            )}
            {t('Sign in with Passkey')}
          </Button>
          {!passkeySupported && (
            <p className='text-muted-foreground text-xs'>
              {t('Passkey is not supported on this device.')}
            </p>
          )}
        </div>
      )}

      <OAuthProviders
        status={status}
        redirectTo={redirectTo}
        disabled={disabled}
        onWeChatLogin={onWeChatLogin}
        isWeChatLoading={isWeChatLoading}
      />
    </Dialog>
  )
}

/** Full-width outline button opening the alternative methods dialog. */
export function OtherLoginTrigger({
  onClick,
  disabled,
}: {
  onClick: () => void
  disabled?: boolean
}) {
  const { t } = useTranslation()
  return (
    <Button
      type='button'
      variant='outline'
      className='w-full justify-center gap-2'
      onClick={onClick}
      disabled={disabled}
    >
      <LogIn className='h-4 w-4' />
      {t('Other sign-in options')}
    </Button>
  )
}
