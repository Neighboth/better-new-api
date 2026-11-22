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
import { getRouteApi, useNavigate } from '@tanstack/react-router'
import { Plus } from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'

import { DeploymentAccessGuard } from './components/deployment-access-guard'
import { ModelsDialogs } from './components/models-dialogs'
import { ModelsPrimaryButtons } from './components/models-primary-buttons'
import { ModelsProvider, useModels } from './components/models-provider'
import { ModelsTable } from './components/models-table'
import { UnifiedDeploymentsPanel } from './components/notebook-deployments-panel'
import { useModelDeploymentSettings } from './hooks/use-model-deployment-settings'
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

  const [createDeploymentOpen, setCreateDeploymentOpen] = useState(false)
  const deploymentSettings = useModelDeploymentSettings()
  const {
    loading: deploymentLoading,
    loadingPhase,
    isAnyEnabled,
    enabledProviders,
    providerAccounts,
    connectionLoading,
    connectionOk,
    connectionError,
    testConnection,
  } = deploymentSettings

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
            isAnyEnabled && (
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
              ) : !deploymentLoading && isAnyEnabled ? (
                <UnifiedDeploymentsPanel
                  enabledProviders={enabledProviders}
                  providerAccounts={providerAccounts}
                  createOpen={createDeploymentOpen}
                  onCreateOpenChange={setCreateDeploymentOpen}
                />
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
                  <UnifiedDeploymentsPanel
                    enabledProviders={enabledProviders}
                    providerAccounts={providerAccounts}
                    createOpen={createDeploymentOpen}
                    onCreateOpenChange={setCreateDeploymentOpen}
                  />
                </DeploymentAccessGuard>
              )}
            </div>
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <ModelsDialogs />
    </>
  )
}

export function Models() {
  return (
    <ModelsProvider>
      <ModelsContent />
    </ModelsProvider>
  )
}
