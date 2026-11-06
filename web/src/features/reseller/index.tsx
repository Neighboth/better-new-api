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
import { LayoutDashboard, Store, Ticket } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { RedemptionsTab } from './components/redemptions-tab'
import { SummaryTab } from './components/summary-tab'

export function ResellerPortal() {
  const { t } = useTranslation()
  return (
    <SectionPageLayout>
      <SectionPageLayout.Title><div className='flex items-center gap-3'><Store className='h-6 w-6 text-primary' /><span>{t('Reseller Portal')}</span></div></SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <Tabs defaultValue='summary' className='space-y-6'>
          <TabsList className='grid w-full max-w-lg grid-cols-2'>
            <TabsTrigger value='summary' className='gap-2'><LayoutDashboard className='h-4 w-4' />{t('Dashboard')}</TabsTrigger>
            <TabsTrigger value='redemptions' className='gap-2'><Ticket className='h-4 w-4' />{t('Redemption Codes')}</TabsTrigger>
          </TabsList>
          <TabsContent value='summary'><SummaryTab /></TabsContent>
          <TabsContent value='redemptions'><RedemptionsTab /></TabsContent>
        </Tabs>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
