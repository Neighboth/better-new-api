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
import { Globe, LayoutDashboard, Loader2, Store, Ticket } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Badge } from '@/components/ui/badge'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

import { fetchResellerConfig } from './api'
import { ChildPanelTab } from './components/child-panel-tab'
import { RedemptionsTab } from './components/redemptions-tab'
import { SummaryTab } from './components/summary-tab'

export function ResellerPortal() {
  const { t } = useTranslation()
  const [activeTab, setActiveTab] = useState<'summary' | 'child-panel' | 'redemptions'>('summary')

  const { data: config, isLoading, error } = useQuery({
    queryKey: ['reseller-self-config'],
    queryFn: fetchResellerConfig,
  })

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        <div className='flex items-center gap-3'>
          <Store className='h-6 w-6 text-primary' />
          <span>{t('Reseller Portal')}</span>
          {config && (
            <Badge
              variant={config.child_panel_enabled ? 'default' : 'secondary'}
              className='text-xs'
            >
              {config.child_panel_enabled
                ? t('Child Panel Active')
                : t('Child Panel Disabled')}
            </Badge>
          )}
        </div>
      </SectionPageLayout.Title>

      <SectionPageLayout.Content>
        {isLoading ? (
          <div className='flex h-64 items-center justify-center'>
            <Loader2 className='h-8 w-8 animate-spin text-muted-foreground' />
          </div>
        ) : error || !config ? (
          <div className='rounded-lg border border-destructive/30 bg-destructive/10 p-6 text-center text-sm text-destructive'>
            {t('Failed to load reseller configuration. Please try refreshing.')}
          </div>
        ) : (
          <Tabs
            value={activeTab}
            onValueChange={(val) => setActiveTab(val as any)}
            className='space-y-6'
          >
            <TabsList className='grid w-full max-w-lg grid-cols-3'>
              <TabsTrigger value='summary' className='gap-2'>
                <LayoutDashboard className='h-4 w-4' />
                <span>{t('Dashboard')}</span>
              </TabsTrigger>
              <TabsTrigger value='child-panel' className='gap-2'>
                <Globe className='h-4 w-4' />
                <span>{t('Child Panel')}</span>
              </TabsTrigger>
              <TabsTrigger value='redemptions' className='gap-2'>
                <Ticket className='h-4 w-4' />
                <span>{t('Redemption Codes')}</span>
              </TabsTrigger>
            </TabsList>

            <TabsContent value='summary' className='space-y-4'>
              <SummaryTab />
            </TabsContent>

            <TabsContent value='child-panel' className='space-y-4'>
              <ChildPanelTab config={config} />
            </TabsContent>

            <TabsContent value='redemptions' className='space-y-4'>
              <RedemptionsTab />
            </TabsContent>
          </Tabs>
        )}
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
