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
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import { AnnouncementBarsBanner } from '@/components/announcement-bar-banner'
import { Skeleton } from '@/components/ui/skeleton'
import { useSystemConfig } from '@/hooks/use-system-config'

type AuthLayoutProps = {
  children: React.ReactNode
}

export function AuthLayout({ children }: AuthLayoutProps) {
  const { t } = useTranslation()
  const { systemName, logo, loading, hideHeaderSiteName } = useSystemConfig()

  return (
    <div className='relative flex min-h-svh flex-col'>
      <AnnouncementBarsBanner />
      <div
        aria-hidden='true'
        className='w-full shrink-0 transition-[height] duration-200'
        style={{ height: 'var(--announcement-bars-height, 0px)' }}
      />
      <div className='w-full px-4 pt-4 sm:px-8 sm:pt-8'>
        <Link
          to='/'
          className='inline-flex items-center gap-2 transition-opacity hover:opacity-80'
        >
          <div className='flex h-8 items-center'>
            {loading ? (
              <Skeleton className='h-8 w-32 rounded-md' />
            ) : (
              <img
                src={logo}
                alt={t('Logo')}
                className='h-8 w-auto max-w-48 object-contain'
              />
            )}
          </div>
          {loading ? (
            <Skeleton className='h-6 w-24' />
          ) : (
            !hideHeaderSiteName && (
              <h1 className='text-xl font-medium'>{systemName}</h1>
            )
          )}
        </Link>
      </div>
      <div className='container flex flex-1 items-center justify-center py-6'>
        <div className='mx-auto flex w-full flex-col justify-center space-y-2 px-4 sm:w-[480px] sm:p-8'>
          {children}
        </div>
      </div>
    </div>
  )
}
