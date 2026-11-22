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
import { ModalDeployHelper } from './components/modal-deploy-helper'
import { NotebookDeploymentsPanel } from './components/notebook-deployments-panel'
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
  const deploymentSettings = useModelDeploymentSettings()
  const { isIoNetEnabled: isIoNetAvailable } = deploymentSettings
  const [providerTab, setProviderTab] = useState('ionet')

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
            isIoNetAvailable &&
            providerTab === 'ionet' && (
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
                <DeploymentsSection
                  settings={deploymentSettings}
                  providerTab={providerTab}
                  onProviderTabChange={setProviderTab}
                />
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

type DeploymentSettings = ReturnType<typeof useModelDeploymentSettings>

function DeploymentsSection(props: {
  settings: DeploymentSettings
  providerTab: string
  onProviderTabChange: (tab: string) => void
}) {
  const queryClient = useQueryClient()
  const { t } = useTranslation()
  const navigate = useNavigate()
  const {
    loading: deploymentLoading,
    loadingPhase,
    isIoNetEnabled,
    isModalEnabled,
    isHfEnabled,
    isColabEnabled,
    isKaggleEnabled,
    isAnyEnabled,
    connectionLoading,
    connectionOk,
    connectionError,
    testConnection,
  } = props.settings

  const enabledTabs: Array<{ id: string; label: string }> = []
  if (isIoNetEnabled) enabledTabs.push({ id: 'ionet', label: 'io.net' })
  if (isHfEnabled) enabledTabs.push({ id: 'huggingface', label: 'Hugging Face' })
  if (isColabEnabled) enabledTabs.push({ id: 'colab', label: 'Google Colab' })
  if (isKaggleEnabled) enabledTabs.push({ id: 'kaggle', label: 'Kaggle' })
  if (isModalEnabled) enabledTabs.push({ id: 'modal', label: 'Modal' })

  const activeTab = enabledTabs.some((tab) => tab.id === props.providerTab)
    ? props.providerTab
    : (enabledTabs[0]?.id ?? 'ionet')

  useEffect(() => {
    if (enabledTabs.length > 0 && activeTab !== props.providerTab) {
      props.onProviderTabChange(activeTab)
    }
  }, [activeTab, enabledTabs.length, props])

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

  if (!deploymentLoading && isAnyEnabled && enabledTabs.length > 0) {
    const renderTabPanel = (tabId: string) => {
      switch (tabId) {
        case 'huggingface':
          return <HfEndpointsPanel />
        case 'colab':
          return <NotebookDeploymentsPanel provider='colab' />
        case 'kaggle':
          return <NotebookDeploymentsPanel provider='kaggle' />
        case 'modal':
          return (
            <div className='space-y-6'>
              <div className='mx-auto mt-4 max-w-md'>
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
              <div className='mx-auto max-w-3xl'>
                <ModalDeployHelper />
              </div>
            </div>
          )
        default:
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
    }

    if (enabledTabs.length === 1) {
      return renderTabPanel(enabledTabs[0].id)
    }

    return (
      <Tabs
        value={activeTab}
        onValueChange={props.onProviderTabChange}
        className='flex h-full min-h-0 flex-col gap-4'
      >
        <TabsList className='w-fit flex-wrap'>
          {enabledTabs.map((tab) => (
            <TabsTrigger key={tab.id} value={tab.id}>
              {tab.label}
            </TabsTrigger>
          ))}
        </TabsList>
        <div className='min-h-0 flex-1'>{renderTabPanel(activeTab)}</div>
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
