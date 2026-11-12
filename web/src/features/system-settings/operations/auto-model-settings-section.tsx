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
import { RotateCcw } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { MultiSelect } from '@/components/multi-select'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { getAllModels } from '@/features/channels/api'
import { api } from '@/lib/api'

import {
  SettingsDescription,
  SettingsForm,
  SettingsFormGrid,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOptionsBulk } from '../hooks/use-update-option'

/**
 * One ranking of the Auto Model: an ordered list of model ids, best first.
 *
 * The router follows the order of every list. A dimension that the
 * administrator leaves empty is unknown to the router and it falls back to the
 * general ranking, so an installation may fill only the lists it cares about.
 */
type Rankings = Record<string, string[]>

export type AutoModelSettingsDefaults = {
  enabled?: boolean
  routingModel?: string
  rankings?: Rankings
  systemPrompt?: string
  defaultBudgetTokens?: number | string
}

/** The label of a dimension, translated where a translation exists. */
const DIMENSION_LABELS: Record<string, string> = {
  smartest_first: 'Smartest to dumbest',
  dumbest_first: 'Dumbest to smartest',
  coding: 'Best for coding',
  code_review: 'Best for code review',
  reasoning: 'Best for reasoning',
  math: 'Best for math',
  science: 'Best for science',
  creative_writing: 'Best for creative writing',
  translation: 'Best for translation',
  summarization: 'Best for summarization',
  vision: 'Best for images (vision)',
  tool_calling: 'Best for tool calling',
  agentic: 'Best for agents',
  long_context: 'Best for long context',
  instruction_following: 'Best at following instructions',
  json_output: 'Best for JSON output',
  sql: 'Best for SQL',
  debugging: 'Best at debugging',
  data_analysis: 'Best for data analysis',
  multilingual: 'Best for many languages',
  education: 'Best for teaching',
  legal: 'Best for legal work',
  medical: 'Best for medical work',
  finance: 'Best for finance',
  roleplay: 'Best for roleplay',
  reliability: 'Most reliable',
  cheapest: 'Cheapest',
  fastest: 'Fastest',
  stt: 'Best for speech to text',
  tts: 'Best for text to speech',
  image_generation: 'Best for image generation',
  video_generation: 'Best for video generation',
}

type AutoModelMeta = {
  enabled: boolean
  default_prompt: string
  dimensions: string[]
  model_name: string
}

export function AutoModelSettingsSection({
  defaultValues,
}: {
  defaultValues: AutoModelSettingsDefaults
}) {
  const { t } = useTranslation()
  const updateOptions = useUpdateOptionsBulk()

  const [enabled, setEnabled] = useState(defaultValues.enabled ?? false)
  const [routingModel, setRoutingModel] = useState(defaultValues.routingModel ?? '')
  const [rankings, setRankings] = useState<Rankings>(
    defaultValues.rankings ?? {}
  )
  const [prompt, setPrompt] = useState(defaultValues.systemPrompt ?? '')
  const [promptTouched, setPromptTouched] = useState(
    Boolean(defaultValues.systemPrompt)
  )
  const [budget, setBudget] = useState(String(defaultValues.defaultBudgetTokens ?? 0))

  // The default prompt and the dimensions live on the server: the card shows
  // the default when the administrator never edited the prompt, and the reset
  // button restores it, so the two never drift apart.
  const { data: meta } = useQuery({
    queryKey: ['auto-model-meta'],
    queryFn: async () => {
      const res = await api.get<{ success: boolean; data: AutoModelMeta }>(
        '/api/option/auto_model_meta'
      )
      return res.data.data
    },
    staleTime: 10 * 60_000,
  })

  const { data: modelsData } = useQuery({
    queryKey: ['all-models-for-auto'],
    queryFn: getAllModels,
    staleTime: 5 * 60_000,
  })

  const modelOptions = useMemo(() => {
    const raw = modelsData?.data ?? []
    return raw
      .map((entry) => (typeof entry === 'string' ? entry : (entry?.id ?? '')))
      .filter((name) => name && name !== 'auto')
      .sort((a, b) => a.localeCompare(b))
      .map((name) => ({ value: name, label: name }))
  }, [modelsData])

  const dimensions = meta?.dimensions ?? Object.keys(DIMENSION_LABELS)
  const defaultPrompt = meta?.default_prompt ?? ''

  useEffect(() => {
    // An empty override means "the default prompt is in force": the field
    // shows that prompt, which is what the administrator edits.
    if (!promptTouched && defaultPrompt) {
      setPrompt(defaultPrompt)
    }
  }, [defaultPrompt, promptTouched])

  const save = async () => {
    const cleanRankings: Rankings = {}
    for (const dimension of dimensions) {
      const list = (rankings[dimension] ?? []).filter(Boolean)
      if (list.length > 0) cleanRankings[dimension] = list
    }
    const effectivePrompt =
      prompt.trim() === defaultPrompt.trim() ? '' : prompt

    try {
      await updateOptions.mutateAsync({
        'auto_model_setting.enabled': String(Boolean(enabled)),
        'auto_model_setting.routing_model': routingModel,
        'auto_model_setting.rankings': JSON.stringify(cleanRankings),
        'auto_model_setting.system_prompt': effectivePrompt,
        'auto_model_setting.default_budget_tokens': String(
          Math.max(0, Math.floor(Number(budget) || 0))
        ),
      })
      toast.success(t('Auto model settings saved'))
    } catch {
      toast.error(t('Failed to save settings'))
    }
  }

  const resetPrompt = () => {
    setPrompt(defaultPrompt)
    setPromptTouched(false)
  }

  return (
    <SettingsSection title={t('Auto Model')}>
      <SettingsForm
        onSubmit={(event) => {
          event.preventDefault()
          void save()
        }}
      >
        <SettingsPageFormActions
          isSaving={updateOptions.isPending}
          onSave={() => void save()}
        />

        <SettingsSwitchItem>
          <SettingsSwitchContent>
            <Label>{t('Enable the auto model')}</Label>
            <SettingsDescription>
              {t(
                'Adds a model named "auto" to the catalogue. Every request that asks for it is routed to the best model of the task and billed as that model; the auto model itself has no price.'
              )}
            </SettingsDescription>
          </SettingsSwitchContent>
          <Switch
            checked={enabled}
            onCheckedChange={(checked) => setEnabled(checked)}
          />
        </SettingsSwitchItem>

        {enabled && (
          <>
            <SettingsFormGrid>
              <div className='space-y-1'>
                <Label>{t('Routing model')}</Label>
                <SettingsDescription>
                  {t(
                    'The model that reads the request and names the model to use. Leave it empty to route with the rankings alone.'
                  )}
                </SettingsDescription>
                <MultiSelect
                  emptyText={t('No model matches the search')}
                  onChange={(values) => setRoutingModel(values[values.length - 1] ?? '')}
                  options={modelOptions}
                  placeholder={t('Search models...')}
                  selected={routingModel ? [routingModel] : []}
                />
              </div>

              <div className='space-y-1'>
                <Label>{t('Default token budget')}</Label>
                <SettingsDescription>
                  {t(
                    'Optional budget of one routed request, in tokens. 0 means no budget; the router then picks the best model of the task.'
                  )}
                </SettingsDescription>
                <Input
                  inputMode='numeric'
                  min={0}
                  onChange={(event) => setBudget(event.target.value)}
                  type='number'
                  value={budget}
                />
              </div>
            </SettingsFormGrid>

            <SettingsDescription className='text-sm'>
              {t(
                'The router sees the whole catalogue of this site: every usable model with its current price, and your rankings. It picks the cheapest model that still fits the task and your rankings. Large catalogues are compacted automatically so the routing model never overflows its context window.'
              )}
            </SettingsDescription>

            <div className='space-y-2'>
              <div>
                <Label>{t('Rankings')}</Label>
                <SettingsDescription>
                  {t(
                    'One ordered list per task, best first. The router follows the order; a list that stays empty is unknown and the general ranking is used instead.'
                  )}
                </SettingsDescription>
              </div>
              <SettingsFormGrid className='gap-y-4'>
                {dimensions.map((dimension) => (
                  <div className='min-w-0 space-y-1' key={dimension}>
                    <Label className='text-xs font-medium'>
                      {t(DIMENSION_LABELS[dimension] ?? dimension)}
                    </Label>
                    <MultiSelect
                      emptyText={t('No model matches the search')}
                      onChange={(values) =>
                        setRankings((current) => ({
                          ...current,
                          [dimension]: values,
                        }))
                      }
                      options={modelOptions}
                      placeholder={t('Search models...')}
                      selected={rankings[dimension] ?? []}
                    />
                  </div>
                ))}
              </SettingsFormGrid>
            </div>

            <div className='space-y-2'>
              <div className='flex flex-wrap items-center justify-between gap-2'>
                <div className='min-w-0'>
                  <Label>{t('Routing system prompt')}</Label>
                  <SettingsDescription>
                    {t(
                      'The prompt the routing model reads. The placeholders are filled by the gateway: {{candidates}}, {{rankings}}, {{request_profile}}, {{preference}}, {{budget}}, {{request}}.'
                    )}
                  </SettingsDescription>
                </div>
                <Button onClick={resetPrompt} size='sm' type='button' variant='outline'>
                  <RotateCcw className='mr-1 size-3.5' />
                  {t('Reset to the default prompt')}
                </Button>
              </div>
              <Textarea
                className='font-mono text-xs'
                onChange={(event) => {
                  setPrompt(event.target.value)
                  setPromptTouched(true)
                }}
                rows={14}
                value={prompt}
              />
            </div>

            <SettingsDescription className='text-sm'>
              {t(
                'Clients can tune one call with extra parameters: auto_budget (tokens), auto_price_min and auto_price_max (USD per 1M input tokens), auto_preference (fast, smart, cheap or balanced) and auto_instruction (free text for the router).'
              )}
            </SettingsDescription>
          </>
        )}
      </SettingsForm>
    </SettingsSection>
  )
}
