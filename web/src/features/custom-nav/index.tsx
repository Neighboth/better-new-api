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
import { useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import { AuthenticatedLayout, PublicLayout } from '@/components/layout'
import { RichContent } from '@/components/rich-content'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { isSafeCustomNavUrl } from '@/features/system-settings/maintenance/custom-nav-config'
import { useCustomNavItems } from '@/hooks/use-custom-nav-items'
import { useAuthStore } from '@/stores/auth-store'

type CustomNavPageProps = {
  navId: string
  view?: 'sidebar' | 'header'
}

export function CustomNavPage(props: CustomNavPageProps) {
  const { t } = useTranslation()
  const items = useCustomNavItems()
  const item = items.find((candidate) => candidate.id === props.navId)
  const navigate = useNavigate()
  const isSidebarView = props.view !== 'header'
  const isAuthenticated = useAuthStore((state) => !!state.auth.user)

  if (!item) {
    return (
      <PublicLayout>
        <div className='mx-auto max-w-3xl space-y-4'>
          <h1 className='text-2xl font-semibold'>{t('Not Found')}</h1>
          <Alert variant='destructive'>
            <AlertDescription>
              {t('This page is not available.')}
            </AlertDescription>
          </Alert>
        </div>
      </PublicLayout>
    )
  }

  // The sidebar layout is only meaningful for signed-in users; route
  // visitors to sign-in with a return path so the context is preserved.

  if (isSidebarView && !isAuthenticated) {
    void navigate({
      to: '/sign-in',
      search: { redirect: window.location.href },
    })
    return null
  }

  const title = item.label
  const content = item.content
  const body = (
    <div className={item.contentType === 'url' ? 'min-h-[70vh] flex-1' : ''}>
      <CustomNavContent
        contentType={item.contentType}
        content={content}
        title={title}
      />
    </div>
  )

  if (isSidebarView) {
    return (
      <AuthenticatedLayout>
        <div className='h-full min-h-0 w-full overflow-y-auto p-4'>
          {body}
        </div>
      </AuthenticatedLayout>
    )
  }

  return (
    <PublicLayout showMainContainer={false}>
      <main className='min-h-svh w-full'>
        <div className='mx-auto w-full max-w-5xl px-4 py-6'>
          {body}
        </div>
      </main>
    </PublicLayout>
  )
}

type CustomNavContentProps = {
  contentType: 'html' | 'markdown' | 'url'
  content: string
  title: string
}

function CustomNavContent(props: CustomNavContentProps) {
  const { t } = useTranslation()

  if (props.contentType === 'url') {
    if (!isSafeCustomNavUrl(props.content)) {
      return (
        <Alert variant='destructive'>
          <AlertDescription>{t('Enter a valid http(s) URL.')}</AlertDescription>
        </Alert>
      )
    }

    return (
      <iframe
        src={props.content.trim()}
        title={props.title}
        className='h-full min-h-[500px] w-full rounded-lg border'
        referrerPolicy='no-referrer'
        sandbox='allow-forms allow-popups allow-popups-to-escape-sandbox allow-scripts'
      />
    )
  }

  return (
    <RichContent
      content={props.content}
      mode={props.contentType === 'html' ? 'html' : 'markdown'}
      htmlVariant='isolated'
      className='max-w-none'
    />
  )
}
