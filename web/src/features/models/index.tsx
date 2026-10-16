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
import { useQueryClient } from '@tanstack/react-query'
import { getRouteApi, useNavigate } from '@tanstack/react-router'
import { AlertCircle, Plus, Server, Settings } from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'

import { listDeployments } from './api'
import { DeploymentAccessGuard } from './components/deployment-access-guard'
import { DeploymentsTable } from './components/deployments-table'
import { HfEndpointsPanel } from './components/hf-endpoints-panel'
import { CreateDeploymentDrawer } from './components/dialogs/create-deployment-drawer'
import { ModelsDialogs } from './components/models-dialogs'
import { ModelsPrimaryButtons } from './components/models-primary-buttons'
import { ModelsProvider, useModels } from './components/models-provider'
import { ModelsTable } from './components/models-table'
import { useModelDeploymentSettings } from './hooks/use-model-deployment-settings'
import { deploymentsQueryKeys } from './lib'
import {
  type ModelsSectionId,
  MODELS_DEFAULT_SECTION,
  MODELS_SECTION_IDS,
} from './section-registry'

const route = getRouteApi('/_authenticated/models/$section')

const SECTION_META: Record<ModelsSectionId, { titleKey: string }> = {
  metadata: {
    titleKey: 'Metadata',
  },
  deployments: {
    titleKey: 'Deployments',
  },
}

function ModelsContent() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { tabCategory, setTabCategory } = useModels()
  const params = route.useParams()
  const activeSection = (params.section ??
    MODELS_DEFAULT_SECTION) as ModelsSectionId

  // Deployment create dialog state
  const [createDeploymentOpen, setCreateDeploymentOpen] = useState(false)
  const { isIoNetEnabled: isIoNetAvailable } = useModelDeploymentSettings()

  // keep context state in sync (for components that rely on it)
  useEffect(() => {
    if (tabCategory !== activeSection) {
      setTabCategory(activeSection)
    }
  }, [activeSection, setTabCategory, tabCategory])

  const handleSectionChange = useCallback(
    (section: string) => {
      void navigate({
        to: '/models/$section',
        params: { section: section as ModelsSectionId },
      })
    },
    [navigate]
  )

  const meta = SECTION_META[activeSection] ?? SECTION_META.metadata

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>{t(meta.titleKey)}</SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          {activeSection === 'metadata' ? (
            <ModelsPrimaryButtons />
          ) : (
            isIoNetAvailable && (
              <Button onClick={() => setCreateDeploymentOpen(true)} size='sm'>
                <Plus className='h-4 w-4' />
                {t('Create deployment')}
              </Button>
            )
          )}
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <div className='flex h-full min-h-0 flex-col gap-4'>
            <Tabs value={activeSection} onValueChange={handleSectionChange}>
              <TabsList className='max-w-full flex-wrap justify-start group-data-horizontal/tabs:h-auto'>
                {MODELS_SECTION_IDS.map((section) => (
                  <TabsTrigger key={section} value={section}>
                    {t(SECTION_META[section].titleKey)}
                  </TabsTrigger>
                ))}
              </TabsList>
            </Tabs>
            <div className='min-h-0 flex-1'>
              {activeSection === 'metadata' ? (
                <ModelsTable />
              ) : (
                <DeploymentsSection />
              )}
            </div>
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <ModelsDialogs />
      <CreateDeploymentDrawer
        open={createDeploymentOpen}
        onOpenChange={setCreateDeploymentOpen}
      />
    </>
  )
}

function DeploymentsSection() {
  const queryClient = useQueryClient()
  const { t } = useTranslation()
  const navigate = useNavigate()
  const {
    loading: deploymentLoading,
    loadingPhase,
    isIoNetEnabled,
    isHfEnabled,
    isAnyEnabled,
    connectionLoading,
    connectionOk,
    connectionError,
    testConnection,
  } = useModelDeploymentSettings()
  const [hfTab, setHfTab] = useState('ionet')

  // Prefetch deployments list while connection check is in progress.
  useEffect(() => {
    if (isIoNetEnabled && loadingPhase === 'connection') {
      const defaultParams = { p: 1, page_size: 10 }
      queryClient.prefetchQuery({
        queryKey: deploymentsQueryKeys.list(defaultParams),
        queryFn: () => listDeployments(defaultParams),
        staleTime: 30 * 1000,
      })
    }
  }, [isIoNetEnabled, loadingPhase, queryClient])

  const handleGoToSettings = () => {
    navigate({
      to: '/system-settings/models/$section',
      params: { section: 'model-deployment' },
    })
  }

  // io.net disabled but Modal/Hugging Face enabled: those providers have no
  // container list API here; deployments live on the provider side and route
  // automatically.
  if (!deploymentLoading && isAnyEnabled && !isIoNetEnabled && !isHfEnabled) {
    return (
      <div className='mx-auto mt-8 max-w-md'>
        <div className='text-center'>
          <div className='mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-2xl bg-emerald-100 dark:bg-emerald-900/20'>
            <Server className='h-8 w-8 text-emerald-600 dark:text-emerald-400' />
          </div>
          <h3 className='mb-6 text-xl font-semibold'>
            {t('Serverless deployments active')}
          </h3>
        </div>
        <div className='space-y-4'>
          <Alert variant='default'>
            <AlertCircle className='h-4 w-4' />
            <AlertTitle>{t('Active providers')}</AlertTitle>
            <AlertDescription>
              {t(
                'Modal deployments are managed on the provider side and route automatically by priority; no container list is shown here.'
              )}
            </AlertDescription>
          </Alert>
          <Button onClick={handleGoToSettings} className='w-full'>
            <Settings className='mr-2 h-4 w-4' />
            {t('Go to settings')}
          </Button>
        </div>
      </div>
    )
  }

  // Hugging Face enabled without io.net: show the endpoint manager instead
  // of the io.net container table.
  if (!deploymentLoading && isHfEnabled && !isIoNetEnabled) {
    return <HfEndpointsPanel />
  }

  // Both enabled: provider tabs above the io.net table.
  if (!deploymentLoading && isHfEnabled && isIoNetEnabled) {
    return (
      <Tabs
        value={hfTab}
        onValueChange={setHfTab}
        className='flex h-full min-h-0 flex-col'
      >
        <TabsList className='w-fit'>
          <TabsTrigger value='ionet'>io.net</TabsTrigger>
          <TabsTrigger value='huggingface'>Hugging Face</TabsTrigger>
        </TabsList>
        {hfTab === 'huggingface' ? (
          <HfEndpointsPanel />
        ) : (
          <DeploymentAccessGuard
            loading={deploymentLoading}
            loadingPhase={loadingPhase}
            isEnabled={isAnyEnabled}
            connectionLoading={connectionLoading}
            connectionOk={connectionOk}
            connectionError={connectionError}
            onRetry={testConnection}
          >
            <DeploymentsTable />
          </DeploymentAccessGuard>
        )}
      </Tabs>
    )
  }

  return (
    <DeploymentAccessGuard
      loading={deploymentLoading}
      loadingPhase={loadingPhase}
      isEnabled={isAnyEnabled}
      connectionLoading={connectionLoading}
      connectionOk={connectionOk}
      connectionError={connectionError}
      onRetry={testConnection}
    >
      <DeploymentsTable />
    </DeploymentAccessGuard>
  )
}

export function Models() {
  return (
    <ModelsProvider>
      <ModelsContent />
    </ModelsProvider>
  )
}
