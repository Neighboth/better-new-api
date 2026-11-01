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
import { createFileRoute } from '@tanstack/react-router'
import { RotateCcw } from 'lucide-react'

import { Button } from '@/components/ui/button'
import { ClientPage } from '@/features/byog/components/client-page'

function ClientErrorComponent({ error }: { error?: Error }) {
  return (
    <div className='min-h-screen w-full flex flex-col items-center justify-center p-6 text-center bg-background text-foreground space-y-4 select-none'>
      <div className='p-4 rounded-2xl bg-destructive/10 text-destructive font-semibold text-lg'>
        BYOG Client Arayüzü Başlatılamadı
      </div>
      <p className='text-sm text-muted-foreground max-w-sm'>
        {error?.message || 'Arayüz yüklenirken bir sorun oluştu. Lütfen yeniden deneyin.'}
      </p>
      <Button
        onClick={() => window.location.reload()}
        className='gap-2'
        size='sm'
      >
        <RotateCcw className='size-4' />
        Sayfayı Yeniden Yükle
      </Button>
    </div>
  )
}

export const Route = createFileRoute('/client')({
  component: ClientPage,
  errorComponent: ClientErrorComponent,
})
