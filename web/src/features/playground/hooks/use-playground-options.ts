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
import { useEffect, useMemo, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { getPlaygroundRuntimeConfig, getUserGroups, getUserModels } from '../api'
import {
  getGroupFallback,
  getModelFallback,
  getOptionLoadErrorMessage,
  shouldClearModelForGroup,
} from '../lib'
import type { GroupOption, ModelOption, PlaygroundConfig } from '../types'

type UsePlaygroundOptionsParams = {
  currentGroup: string
  currentModel: string
  setGroups: (groups: GroupOption[]) => void
  setModels: (models: ModelOption[]) => void
  updateConfig: <K extends keyof PlaygroundConfig>(
    key: K,
    value: PlaygroundConfig[K]
  ) => void
}

export function usePlaygroundOptions({
  currentGroup,
  currentModel,
  setGroups,
  setModels,
  updateConfig,
}: UsePlaygroundOptionsParams) {
  const { t } = useTranslation()

  const {
    data: modelsData,
    error: modelsError,
    isError: isModelsError,
    isLoading: isLoadingModels,
  } = useQuery({
    queryKey: ['playground-models', currentGroup],
    queryFn: () => getUserModels(currentGroup),
    enabled: currentGroup !== '',
  })

  const {
    data: groupsData,
    error: groupsError,
    isError: isGroupsError,
  } = useQuery({
    queryKey: ['playground-groups'],
    queryFn: getUserGroups,
  })

  useEffect(() => {
    if (!isModelsError) return

    toast.error(
      getOptionLoadErrorMessage(
        modelsError,
        t('Failed to load playground models')
      )
    )
  }, [isModelsError, modelsError, t])

  useEffect(() => {
    if (!isGroupsError) return

    toast.error(
      getOptionLoadErrorMessage(
        groupsError,
        t('Failed to load playground groups')
      )
    )
  }, [isGroupsError, groupsError, t])

  const { data: runtimeConfig, isLoading: isLoadingRuntimeConfig } = useQuery({
    queryKey: ['playground-runtime-config'],
    queryFn: ({ signal }) => getPlaygroundRuntimeConfig(signal),
    staleTime: 60 * 1000,
  })

  const appliedConfiguredDefaultRef = useRef(false)

  const filteredModels = useMemo(() => {
    if (!modelsData) return undefined
    const hiddenSet = new Set(
      (runtimeConfig?.playground?.hidden_models ?? [])
        .map((m) => m.trim())
        .filter(Boolean)
    )
    const displayNames = runtimeConfig?.playground?.model_display_names ?? {}
    return modelsData
      .filter((m) => !hiddenSet.has(m.value))
      .map((m) => {
        const display = displayNames[m.value]?.trim()
        return display ? { ...m, label: display } : m
      })
  }, [
    modelsData,
    runtimeConfig?.playground?.hidden_models,
    runtimeConfig?.playground?.model_display_names,
  ])

  useEffect(() => {
    if (!filteredModels) return

    setModels(filteredModels)
    if (isLoadingRuntimeConfig) return

    const configuredDefault = (
      runtimeConfig?.playground?.default_model ?? ''
    ).trim()
    const hasConfigured =
      Boolean(configuredDefault) &&
      filteredModels.some((m) => m.value === configuredDefault)

    if (!appliedConfiguredDefaultRef.current && hasConfigured) {
      appliedConfiguredDefaultRef.current = true
      if (currentModel !== configuredDefault) {
        updateConfig('model', configuredDefault)
      }
      return
    }

    const hasCurrent = Boolean(
      currentModel && filteredModels.some((m) => m.value === currentModel)
    )
    if (!hasCurrent && hasConfigured) {
      updateConfig('model', configuredDefault)
      return
    }
    const fallback = getModelFallback(filteredModels, currentModel)

    if (fallback) {
      updateConfig('model', fallback)
      return
    }

    if (shouldClearModelForGroup(filteredModels, currentModel)) {
      updateConfig('model', '')
    }
  }, [
    filteredModels,
    currentModel,
    isLoadingRuntimeConfig,
    runtimeConfig?.playground?.default_model,
    setModels,
    updateConfig,
  ])

  useEffect(() => {
    if (!groupsData) return

    setGroups(groupsData)
    const fallback = getGroupFallback(groupsData, currentGroup)

    if (fallback) {
      updateConfig('group', fallback)
    }
  }, [groupsData, currentGroup, setGroups, updateConfig])

  return {
    isLoadingModels,
  }
}
