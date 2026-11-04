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
import { useNavigate, useParams } from '@tanstack/react-router'
import {
  Activity,
  Cpu,
  Gift,
  Layers,
  Settings,
  ShieldAlert,
} from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

import { ByogBannedNodesTab } from './components/banned-nodes-tab'
import { ByogModelsTab } from './components/models-tab'
import { ByogNodesTab } from './components/nodes-tab'
import { ByogRewardsTab } from './components/rewards-tab'
import { ByogRulesTab } from './components/rules-tab'
import { ByogSettingsTab } from './components/settings-tab'

export type ByogAdminSection =
  | 'nodes'
  | 'settings'
  | 'rules'
  | 'models'
  | 'banned'
  | 'rewards'

interface ByogAdminProps {
  initialSection?: ByogAdminSection
}

export function ByogAdmin({ initialSection }: ByogAdminProps) {
  const { t } = useTranslation()
  const params = useParams({ strict: false }) as { section?: string }
  const navigate = useNavigate()

  // The page opens with the globe of the network, the map of the nodes is
  // the first view of the management.
  const currentSection = (params.section ||
    initialSection ||
    'nodes') as ByogAdminSection
  const [activeTab, setActiveTab] = useState<ByogAdminSection>(currentSection)

  useEffect(() => {
    if (params.section) {
      setActiveTab(params.section as ByogAdminSection)
    }
  }, [params.section])

  function handleTabChange(tab: string) {
    const section = tab as ByogAdminSection
    setActiveTab(section)
    navigate({
      to: '/byog-admin/$section',
      params: { section },
    })
  }

  return (
    <div className='h-full flex-1 overflow-y-auto space-y-6 p-4 md:p-6'>
      <div className='flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between'>
        <div className='space-y-1'>
          <h2 className='text-2xl font-bold tracking-tight'>
            {t('byog.admin.page_title')}
          </h2>
          <p className='text-muted-foreground text-sm'>
            {t('byog.admin.page_description')}
          </p>
        </div>
      </div>

      <Tabs
        value={activeTab}
        onValueChange={handleTabChange}
        className='space-y-6'
      >
        <div className='w-full overflow-x-auto pb-2 scrollbar-thin'>
          <TabsList className='inline-flex h-auto w-max min-w-full justify-start gap-1 p-1 md:grid md:w-full md:grid-cols-6'>
            <TabsTrigger value='nodes' className='flex-1 whitespace-nowrap px-3 py-2 gap-2 text-xs'>
              <Activity className='h-4 w-4' />
              {t('byog.admin.tab_nodes')}
            </TabsTrigger>
            <TabsTrigger value='settings' className='flex-1 whitespace-nowrap px-3 py-2 gap-2 text-xs'>
              <Settings className='h-4 w-4' />
              {t('byog.admin.tab_settings')}
            </TabsTrigger>
            <TabsTrigger value='rules' className='flex-1 whitespace-nowrap px-3 py-2 gap-2 text-xs'>
              <Cpu className='h-4 w-4' />
              {t('byog.admin.tab_rules')}
            </TabsTrigger>
            <TabsTrigger value='models' className='flex-1 whitespace-nowrap px-3 py-2 gap-2 text-xs'>
              <Layers className='h-4 w-4' />
              {t('byog.admin.tab_models')}
            </TabsTrigger>
            <TabsTrigger value='banned' className='flex-1 whitespace-nowrap px-3 py-2 gap-2 text-xs'>
              <ShieldAlert className='h-4 w-4' />
              {t('byog.admin.tab_banned')}
            </TabsTrigger>
            <TabsTrigger value='rewards' className='flex-1 whitespace-nowrap px-3 py-2 gap-2 text-xs'>
              <Gift className='h-4 w-4' />
              {t('byog.admin.tab_rewards')}
            </TabsTrigger>
          </TabsList>
        </div>

        <TabsContent value='nodes' className='space-y-4'>
          <ByogNodesTab />
        </TabsContent>

        <TabsContent value='settings' className='space-y-4'>
          <ByogSettingsTab />
        </TabsContent>

        <TabsContent value='rules' className='space-y-4'>
          <ByogRulesTab />
        </TabsContent>

        <TabsContent value='models' className='space-y-4'>
          <ByogModelsTab />
        </TabsContent>

        <TabsContent value='banned' className='space-y-4'>
          <ByogBannedNodesTab />
        </TabsContent>

        <TabsContent value='rewards' className='space-y-4'>
          <ByogRewardsTab />
        </TabsContent>
      </Tabs>
    </div>
  )
}
