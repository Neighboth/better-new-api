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
import { Loader2, RefreshCw } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { JsonEditor } from '@/components/json-editor'
import { MultiSelect } from '@/components/multi-select'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

import {
  batchEditChannels,
  fetchUpstreamModels,
  getAllModels,
  getGroups,
  type BatchEditChannelsParams,
} from '../api'
import type { Channel } from '../types'
import { ModelMappingEditor } from './model-mapping-editor'
import { PriceMultiplierEditor } from './price-multiplier-editor'

type FieldKey =
  | 'models'
  | 'groups'
  | 'tag'
  | 'priorityWeight'
  | 'autoBan'
  | 'modelRatio'
  | 'modelMapping'
  | 'paramOverride'
  | 'headerOverride'
  | 'statusCodeMapping'
  | 'remark'
  | 'testModel'

const FIELD_KEYS: FieldKey[] = [
  'models',
  'groups',
  'tag',
  'priorityWeight',
  'autoBan',
  'modelRatio',
  'modelMapping',
  'paramOverride',
  'headerOverride',
  'statusCodeMapping',
  'remark',
  'testModel',
]

function Section(props: {
  id: FieldKey
  title: string
  hint?: string
  enabled: boolean
  onToggle: (enabled: boolean) => void
  children: React.ReactNode
}) {
  return (
    <div className='rounded-lg border p-3'>
      <div className='flex items-start gap-2'>
        <Checkbox
          checked={props.enabled}
          id={`batch-field-${props.id}`}
          onCheckedChange={(checked) => props.onToggle(Boolean(checked))}
        />
        <div className='space-y-0.5'>
          <Label
            className='cursor-pointer text-xs font-semibold'
            htmlFor={`batch-field-${props.id}`}
          >
            {props.title}
          </Label>
          {props.hint && (
            <p className='text-muted-foreground text-[11px]'>{props.hint}</p>
          )}
        </div>
      </div>
      {props.enabled && <div className='space-y-2 pt-2 pl-6'>{props.children}</div>}
    </div>
  )
}

export function BatchEditChannelsDialog(props: {
  channels: Channel[]
  onClose: () => void
  onSaved: () => void
}) {
  const { t } = useTranslation()
  const [saving, setSaving] = useState(false)
  const [activeTab, setActiveTab] = useState('basic')
  const [enabled, setEnabled] = useState<Record<FieldKey, boolean>>(() =>
    Object.fromEntries(FIELD_KEYS.map((key) => [key, false])) as Record<
      FieldKey,
      boolean
    >
  )

  const [models, setModels] = useState('')
  const [groups, setGroups] = useState<string[]>([])
  const [tag, setTag] = useState('')
  const [priority, setPriority] = useState('')
  const [weight, setWeight] = useState('')
  const [autoBan, setAutoBan] = useState<'1' | '0'>('1')
  const [modelRatio, setModelRatio] = useState('')
  const [modelMapping, setModelMapping] = useState('')
  const [paramOverride, setParamOverride] = useState('')
  const [headerOverride, setHeaderOverride] = useState('')
  const [statusCodeMapping, setStatusCodeMapping] = useState('')
  const [remark, setRemark] = useState('')
  const [testModel, setTestModel] = useState('')
  const [fetchingModels, setFetchingModels] = useState(false)

  const ids = props.channels.map((channel) => channel.id)

  const { data: groupsData } = useQuery({
    queryKey: ['groups'],
    queryFn: getGroups,
  })
  const { data: allModelsData } = useQuery({
    queryKey: ['all-models-for-batch'],
    queryFn: getAllModels,
    staleTime: 5 * 60_000,
  })

  const modelOptions = useMemo(
    () =>
      (allModelsData?.data ?? [])
        .map((entry) => (typeof entry === 'string' ? entry : entry?.id ?? ''))
        .filter(Boolean)
        .sort((a, b) => a.localeCompare(b)),
    [allModelsData]
  )

  const channelModelList = useMemo(
    () =>
      models
        .split(',')
        .map((model) => model.trim())
        .filter(Boolean),
    [models]
  )

  const groupOptions = useMemo(
    () => (groupsData?.data ?? []).map((group) => ({ value: group, label: group })),
    [groupsData]
  )

  // Fetching the model list of the upstream only makes sense when every
  // selected channel talks to the same address: the answer of one channel
  // would not describe the others otherwise.
  const sharedBaseUrl = useMemo(() => {
    const urls = new Set(
      props.channels.map((channel) => (channel.base_url ?? '').trim())
    )
    if (urls.size !== 1) return null
    return [...urls][0]
  }, [props.channels])

  const baseUrlWarning =
    sharedBaseUrl === null
      ? t(
          'The selected channels use different base URLs, so the model list cannot be fetched from one of them.'
        )
      : sharedBaseUrl === ''
        ? t(
            'The selected channels use the default base URL of their channel type.'
          )
        : ''

  const toggle = (key: FieldKey) => (value: boolean) =>
    setEnabled((prev) => ({ ...prev, [key]: value }))

  const fetchModels = async () => {
    if (sharedBaseUrl === null || props.channels.length === 0) return
    // Any channel of the selection describes the same upstream, pick one.
    const channel = props.channels[Math.floor(Math.random() * props.channels.length)]
    setFetchingModels(true)
    try {
      const response = await fetchUpstreamModels(channel.id)
      if (!response.success || !response.data?.length) {
        toast.error(response.message || t('No models returned by the upstream'))
        return
      }
      setModels(response.data.join(','))
      setEnabled((prev) => ({ ...prev, models: true }))
      toast.success(
        t('Fetched {{count}} models from the upstream', {
          count: response.data.length,
        })
      )
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Failed to fetch models')
      )
    } finally {
      setFetchingModels(false)
    }
  }

  const submit = async () => {
    const payload: BatchEditChannelsParams = { ids }
    if (enabled.models) payload.models = models
    if (enabled.groups) payload.groups = groups.join(',')
    if (enabled.tag) payload.tag = tag
    if (enabled.priorityWeight) {
      if (priority.trim() !== '') payload.priority = Number(priority)
      if (weight.trim() !== '') payload.weight = Number(weight)
    }
    if (enabled.autoBan) payload.auto_ban = Number(autoBan)
    if (enabled.modelRatio) payload.model_ratio_override = modelRatio
    if (enabled.modelMapping) payload.model_mapping = modelMapping
    if (enabled.paramOverride) payload.param_override = paramOverride
    if (enabled.headerOverride) payload.header_override = headerOverride
    if (enabled.statusCodeMapping)
      payload.status_code_mapping = statusCodeMapping
    if (enabled.remark) payload.remark = remark
    if (enabled.testModel) payload.test_model = testModel

    if (Object.keys(payload).length === 1) {
      toast.error(t('Enable at least one field to update'))
      return
    }

    setSaving(true)
    try {
      const res = await batchEditChannels(payload)
      if (res?.success) {
        toast.success(
          t('Successfully updated {{count}} channel(s)', { count: ids.length })
        )
        props.onSaved()
        props.onClose()
      } else {
        toast.error(res?.message || t('Failed to update channels'))
      }
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Failed to update channels')
      )
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className='space-y-3'>
      <p className='text-muted-foreground text-xs'>
        {t(
          'Configure properties for {{count}} selected channel(s). Only the checked fields are written.',
          { count: ids.length }
        )}
      </p>

      <Tabs onValueChange={setActiveTab} value={activeTab}>
        <TabsList className='w-full'>
          <TabsTrigger value='basic'>{t('Basic')}</TabsTrigger>
          <TabsTrigger value='models'>{t('Models')}</TabsTrigger>
          <TabsTrigger value='pricing'>{t('Pricing')}</TabsTrigger>
          <TabsTrigger value='overrides'>{t('Overrides')}</TabsTrigger>
          <TabsTrigger value='advanced'>{t('Advanced')}</TabsTrigger>
        </TabsList>

        <TabsContent className='space-y-3 pt-3' value='basic'>
          <Section
            enabled={enabled.groups}
            hint={t('Replaces the user groups that may use these channels.')}
            id='groups'
            onToggle={toggle('groups')}
            title={t('User groups')}
          >
            <MultiSelect
              allowCreate
              onChange={setGroups}
              options={groupOptions}
              placeholder={t('Select the user groups')}
              selected={groups}
            />
          </Section>

          <Section
            enabled={enabled.tag}
            hint={t('Groups the channels in the channel list.')}
            id='tag'
            onToggle={toggle('tag')}
            title={t('Tag')}
          >
            <Input
              onChange={(event) => setTag(event.target.value)}
              placeholder={t('Enter tag name (optional)')}
              value={tag}
            />
          </Section>

          <Section
            enabled={enabled.priorityWeight}
            hint={t('Higher priority is tried first, the weight splits the load.')}
            id='priorityWeight'
            onToggle={toggle('priorityWeight')}
            title={t('Priority & weight')}
          >
            <div className='grid grid-cols-2 gap-3'>
              <div className='space-y-1'>
                <Label className='text-xs'>{t('Priority')}</Label>
                <Input
                  className='font-mono text-xs'
                  onChange={(event) => setPriority(event.target.value)}
                  placeholder='0'
                  type='number'
                  value={priority}
                />
              </div>
              <div className='space-y-1'>
                <Label className='text-xs'>{t('Weight')}</Label>
                <Input
                  className='font-mono text-xs'
                  min={0}
                  onChange={(event) => setWeight(event.target.value)}
                  placeholder='0'
                  type='number'
                  value={weight}
                />
              </div>
            </div>
          </Section>

          <Section
            enabled={enabled.autoBan}
            hint={t(
              'Automatically disable a channel when its upstream reports an error.'
            )}
            id='autoBan'
            onToggle={toggle('autoBan')}
            title={t('Automatic ban')}
          >
            <Select
              onValueChange={(value) => setAutoBan(value as '1' | '0')}
              value={autoBan}
            >
              <SelectTrigger className='h-8 text-xs'>
                {/* The label is rendered explicitly: the trigger used to fall
                    back to the raw "1"/"0" value before the item list was
                    mounted. */}
                <SelectValue>
                  {autoBan === '1'
                    ? t('Automatic ban enabled')
                    : t('Automatic ban disabled')}
                </SelectValue>
              </SelectTrigger>
              <SelectContent>
                <SelectItem value='1'>{t('Automatic ban enabled')}</SelectItem>
                <SelectItem value='0'>{t('Automatic ban disabled')}</SelectItem>
              </SelectContent>
            </Select>
          </Section>
          <Section
            enabled={enabled.remark}
            hint={t('A free note shown in the channel list.')}
            id='remark'
            onToggle={toggle('remark')}
            title={t('Remark')}
          >
            <Input
              maxLength={255}
              onChange={(event) => setRemark(event.target.value)}
              placeholder={t('Short note about this channel')}
              value={remark}
            />
          </Section>
        </TabsContent>

        <TabsContent className='space-y-3 pt-3' value='models'>
          <Section
            enabled={enabled.models}
            hint={t(
              'Replaces the model list of every selected channel. Separate the models with a comma.'
            )}
            id='models'
            onToggle={toggle('models')}
            title={t('Models')}
          >
            <MultiSelect
              allowCreate
              onChange={(values) => setModels(values.join(','))}
              options={modelOptions.map((model) => ({
                value: model,
                label: model,
              }))}
              placeholder={t('Type a model name and press comma')}
              selected={channelModelList}
            />
            <div className='flex flex-wrap items-center gap-2'>
              <Button
                disabled={
                  fetchingModels || sharedBaseUrl === null || ids.length === 0
                }
                onClick={() => void fetchModels()}
                size='sm'
                type='button'
                variant='outline'
              >
                {fetchingModels ? (
                  <Loader2 className='size-3.5 animate-spin' />
                ) : (
                  <RefreshCw className='size-3.5' />
                )}
                {t('Fetch models')}
              </Button>
              {baseUrlWarning && (
                <span className='text-muted-foreground text-[11px]'>
                  {baseUrlWarning}
                </span>
              )}
            </div>
          </Section>

          <Section
            enabled={enabled.testModel}
            hint={t('The model the health check of these channels uses.')}
            id='testModel'
            onToggle={toggle('testModel')}
            title={t('Test model')}
          >
            <Input
              className='font-mono text-xs'
              list='batch-test-models'
              onChange={(event) => setTestModel(event.target.value)}
              placeholder='gpt-4o-mini'
              value={testModel}
            />
            <datalist id='batch-test-models'>
              {modelOptions.map((model) => (
                <option key={model} value={model} />
              ))}
            </datalist>
          </Section>

          <Section
            enabled={enabled.modelMapping}
            hint={t('Maps the requested model name to the name of the upstream.')}
            id='modelMapping'
            onToggle={toggle('modelMapping')}
            title={t('Model mapping')}
          >
            <ModelMappingEditor
              onChange={setModelMapping}
              value={modelMapping}
            />
          </Section>
        </TabsContent>

        <TabsContent className='space-y-3 pt-3' value='pricing'>
          <Section
            enabled={enabled.modelRatio}
            hint={t(
              'Per-model price multiplier of these channels, for example 0.5 makes a model cost half.'
            )}
            id='modelRatio'
            onToggle={toggle('modelRatio')}
            title={t('Price multiplier')}
          >
            <PriceMultiplierEditor
              models={channelModelList}
              onChange={setModelRatio}
              showQuickSet
              value={modelRatio}
            />
          </Section>
        </TabsContent>

        <TabsContent className='space-y-3 pt-3' value='overrides'>
          <Section
            enabled={enabled.paramOverride}
            hint={t(
              'Rewrites request parameters before the request reaches the upstream.'
            )}
            id='paramOverride'
            onToggle={toggle('paramOverride')}
            title={t('Parameter override')}
          >
            <JsonEditor
              emptyMessage={t('No parameter override configured.')}
              keyLabel={t('Path')}
              keyPlaceholder='temperature'
              onChange={setParamOverride}
              value={paramOverride}
              valueLabel={t('Value')}
            />
          </Section>

          <Section
            enabled={enabled.headerOverride}
            hint={t('Rewrites the HTTP headers sent to the upstream.')}
            id='headerOverride'
            onToggle={toggle('headerOverride')}
            title={t('Header override')}
          >
            <JsonEditor
              emptyMessage={t('No header override configured.')}
              keyLabel={t('Header')}
              keyPlaceholder='User-Agent'
              onChange={setHeaderOverride}
              value={headerOverride}
              valueLabel={t('Value')}
            />
          </Section>
        </TabsContent>

        <TabsContent className='space-y-3 pt-3' value='advanced'>
          <Section
            enabled={enabled.statusCodeMapping}
            hint={t('Maps the status codes of the upstream to other codes.')}
            id='statusCodeMapping'
            onToggle={toggle('statusCodeMapping')}
            title={t('Status code mapping')}
          >
            <JsonEditor
              emptyMessage={t('No status code mappings configured.')}
              keyLabel={t('Original Code')}
              keyPlaceholder='400'
              onChange={setStatusCodeMapping}
              template={{ '400': '500', '429': '503' }}
              value={statusCodeMapping}
              valueLabel={t('Mapped Code')}
              valueType='string'
            />
          </Section>
        </TabsContent>
      </Tabs>

      <div className='flex justify-end gap-2 border-t pt-3'>
        <Button onClick={props.onClose} variant='outline'>
          {t('Cancel')}
        </Button>
        <Button disabled={saving} onClick={() => void submit()}>
          {saving ? t('Updating...') : t('Apply Changes')}
        </Button>
      </div>
    </div>
  )
}
