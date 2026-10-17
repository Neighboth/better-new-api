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
  | 'settings'
  | 'rules'
  | 'models'
  | 'nodes'
  | 'banned'
  | 'rewards'

interface ByogAdminProps {
  initialSection?: ByogAdminSection
}

export function ByogAdmin({ initialSection }: ByogAdminProps) {
  const { t } = useTranslation()
  const params = useParams({ strict: false }) as { section?: string }
  const navigate = useNavigate()

  const currentSection = (params.section ||
    initialSection ||
    'settings') as ByogAdminSection
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
        <div className='overflow-x-auto pb-1'>
          <TabsList className='grid w-full min-w-[640px] grid-cols-3 md:grid-cols-6'>
            <TabsTrigger value='settings' className='gap-2 text-xs'>
              <Settings className='h-4 w-4' />
              {t('byog.admin.tab_settings')}
            </TabsTrigger>
            <TabsTrigger value='rules' className='gap-2 text-xs'>
              <Cpu className='h-4 w-4' />
              {t('byog.admin.tab_rules')}
            </TabsTrigger>
            <TabsTrigger value='models' className='gap-2 text-xs'>
              <Layers className='h-4 w-4' />
              {t('byog.admin.tab_models')}
            </TabsTrigger>
            <TabsTrigger value='nodes' className='gap-2 text-xs'>
              <Activity className='h-4 w-4' />
              {t('byog.admin.tab_nodes')}
            </TabsTrigger>
            <TabsTrigger value='banned' className='gap-2 text-xs'>
              <ShieldAlert className='h-4 w-4' />
              {t('byog.admin.tab_banned')}
            </TabsTrigger>
            <TabsTrigger value='rewards' className='gap-2 text-xs'>
              <Gift className='h-4 w-4' />
              {t('byog.admin.tab_rewards')}
            </TabsTrigger>
          </TabsList>
        </div>

        <TabsContent value='settings' className='space-y-4'>
          <ByogSettingsTab />
        </TabsContent>

        <TabsContent value='rules' className='space-y-4'>
          <ByogRulesTab />
        </TabsContent>

        <TabsContent value='models' className='space-y-4'>
          <ByogModelsTab />
        </TabsContent>

        <TabsContent value='nodes' className='space-y-4'>
          <ByogNodesTab />
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
