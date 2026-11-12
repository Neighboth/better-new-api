import { useQuery } from '@tanstack/react-query'
import { Braces, LayoutList, PlusIcon, XIcon } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { JsonCodeEditor } from '@/components/json-code-editor'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { MultiSelect } from '@/components/multi-select'
import { getAllModels } from '@/features/channels/api'

import {
  SettingsDescription,
  SettingsForm,
  SettingsFormGrid,
  SettingsSwitchContent,
  SettingsSwitchItem,
  SettingsSwitchRow,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOptionsBulk } from '../hooks/use-update-option'

/**
 * One image or video model with the options the studio offers for it.
 *
 * Every model has its own quality levels, aspect ratios, sizes and durations:
 * one video model renders 5 and 10 second clips while the next one offers 4,
 * 8 and 12 seconds. The studio shows exactly the options of the selected
 * model.
 */
export type MediaModelConfig = {
  model: string
  sizes?: string[]
  qualities?: string[]
  ratios?: string[]
  durations?: number[]
  reference_image?: boolean
  default_size?: string
  default_quality?: string
  default_ratio?: string
}

export type PlaygroundSettingsDefaults = {
  imageEnabled?: boolean
  imageModels?: MediaModelConfig[]
  videoEnabled?: boolean
  videoModels?: MediaModelConfig[]
  studioEnabled?: boolean
  imageDefaultCount?: number
  imageModel?: string
  imageSizes?: string
  imageQualities?: string
  imageDefaultSize?: string
  imageDefaultQuality?: string
  videoModel?: string
  videoDurations?: string
  videoRatios?: string
  videoQualities?: string
  videoDefaultRatio?: string
  videoDefaultQuality?: string
  videoReferenceImage?: boolean
}

function splitLegacyList(raw?: string): string[] {
  if (!raw) return []
  return raw
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)
}

/**
 * Builds the model list of the runtime configuration.
 *
 * Installations that configured the studio before the per-model options
 * existed only have the single-model fields; their values become the first
 * entry of the list so nothing is lost when the page is opened.
 */
export function normalizeImageModels(
  defaults: PlaygroundSettingsDefaults
): MediaModelConfig[] {
  if (defaults.imageModels && defaults.imageModels.length > 0) {
    return defaults.imageModels
  }
  if (!defaults.imageModel?.trim()) return []
  return [
    {
      model: defaults.imageModel.trim(),
      sizes: splitLegacyList(defaults.imageSizes),
      qualities: splitLegacyList(defaults.imageQualities),
      default_size: defaults.imageDefaultSize,
      default_quality: defaults.imageDefaultQuality,
    },
  ]
}

export function normalizeVideoModels(
  defaults: PlaygroundSettingsDefaults
): MediaModelConfig[] {
  if (defaults.videoModels && defaults.videoModels.length > 0) {
    return defaults.videoModels
  }
  if (!defaults.videoModel?.trim()) return []
  return [
    {
      model: defaults.videoModel.trim(),
      durations: splitLegacyList(defaults.videoDurations)
        .map((value) => Number(value))
        .filter((value) => Number.isFinite(value) && value > 0),
      ratios: splitLegacyList(defaults.videoRatios),
      qualities: splitLegacyList(defaults.videoQualities),
      default_ratio: defaults.videoDefaultRatio,
      default_quality: defaults.videoDefaultQuality,
      reference_image: defaults.videoReferenceImage ?? true,
    },
  ]
}

type TagFieldProps = {
  label: string
  description?: string
  values: string[]
  options?: string[]
  placeholder?: string
  onChange: (values: string[]) => void
}

/** Comma separated tag editor: type + "," adds a tag, the × removes it. */
function TagField(props: TagFieldProps) {
  const { t } = useTranslation()
  const options = useMemo(
    () =>
      (props.options ?? []).map((value) => ({ value, label: value })),
    [props.options]
  )
  const allOptions = useMemo(() => {
    const merged = new Map(options.map((option) => [option.value, option]))
    for (const value of props.values) {
      if (!merged.has(value)) merged.set(value, { value, label: value })
    }
    return [...merged.values()]
  }, [options, props.values])

  return (
    <div className='space-y-1'>
      <Label className='text-xs'>{props.label}</Label>
      <MultiSelect
        allowCreate
        options={allOptions}
        selected={props.values}
        onChange={props.onChange}
        placeholder={props.placeholder ?? t('Type a value and press comma')}
      />
      {props.description && (
        <p className='text-muted-foreground text-[11px]'>
          {props.description}
        </p>
      )}
    </div>
  )
}

type ModelCardProps = {
  index: number
  mode: 'image' | 'video'
  value: MediaModelConfig
  modelOptions: string[]
  onChange: (value: MediaModelConfig) => void
  onRemove: () => void
}

function ModelCard(props: ModelCardProps) {
  const { t } = useTranslation()
  const { value, mode } = props

  const update = (patch: Partial<MediaModelConfig>) =>
    props.onChange({ ...value, ...patch })

  const sizeOptions = useMemo(() => value.sizes ?? [], [value.sizes])
  const qualityOptions = useMemo(() => value.qualities ?? [], [value.qualities])
  const ratioOptions = useMemo(() => value.ratios ?? [], [value.ratios])

  return (
    <div className='bg-muted/20 space-y-3 rounded-lg p-3'>
      <div className='flex items-start gap-2'>
        <div className='flex-1 space-y-1'>
          <Label className='text-xs'>{t('Model')}</Label>
          <Input
            className='font-mono text-xs'
            list={`playground-media-models-${mode}`}
            onChange={(event) => update({ model: event.target.value })}
            placeholder={mode === 'image' ? 'dall-e-3' : 'kling-v1'}
            value={value.model}
          />
          <datalist id={`playground-media-models-${mode}`}>
            {props.modelOptions.map((model) => (
              <option key={model} value={model} />
            ))}
          </datalist>
        </div>
        <Button
          aria-label={t('Remove model')}
          className='mt-5'
          onClick={props.onRemove}
          size='icon-sm'
          type='button'
          variant='ghost'
        >
          <XIcon className='size-4' />
        </Button>
      </div>

      {mode === 'image' ? (
        <>
          <SettingsFormGrid>
            <TagField
              label={t('Sizes')}
              onChange={(sizes) => update({ sizes })}
              placeholder='1024x1024'
              values={value.sizes ?? []}
            />
            <TagField
              label={t('Quality levels')}
              onChange={(qualities) => update({ qualities })}
              placeholder='standard, hd'
              values={value.qualities ?? []}
            />
          </SettingsFormGrid>
          <SettingsFormGrid>
            <div className='space-y-1'>
              <Label className='text-xs'>{t('Default size')}</Label>
              <Select
                onValueChange={(next) => update({ default_size: String(next) })}
                value={value.default_size || sizeOptions[0] || ''}
              >
                <SelectTrigger className='h-9'>
                  <SelectValue placeholder={t('First size')} />
                </SelectTrigger>
                <SelectContent>
                  {sizeOptions.map((size) => (
                    <SelectItem key={size} value={size}>
                      {size}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className='space-y-1'>
              <Label className='text-xs'>{t('Default quality')}</Label>
              <Select
                onValueChange={(next) =>
                  update({ default_quality: String(next) })
                }
                value={value.default_quality || qualityOptions[0] || ''}
              >
                <SelectTrigger className='h-9'>
                  <SelectValue placeholder={t('First quality')} />
                </SelectTrigger>
                <SelectContent>
                  {qualityOptions.map((quality) => (
                    <SelectItem key={quality} value={quality}>
                      {quality}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </SettingsFormGrid>
        </>
      ) : (
        <>
          <SettingsFormGrid>
            <TagField
              label={t('Durations (seconds)')}
              description={t(
                'The studio offers exactly these durations and derives the shortest and the longest clip from them.'
              )}
              onChange={(raw) =>
                update({
                  durations: raw
                    .map((item) => Number(item))
                    .filter((item) => Number.isFinite(item) && item > 0)
                    .sort((a, b) => a - b),
                })
              }
              placeholder='5, 10'
              values={(value.durations ?? []).map(String)}
            />
            <TagField
              label={t('Aspect ratios')}
              onChange={(ratios) => update({ ratios })}
              placeholder='16:9, 9:16'
              values={value.ratios ?? []}
            />
          </SettingsFormGrid>
          <SettingsFormGrid>
            <TagField
              label={t('Quality levels')}
              onChange={(qualities) => update({ qualities })}
              placeholder='standard, high'
              values={value.qualities ?? []}
            />
            <div className='space-y-1'>
              <Label className='text-xs'>{t('Default ratio')}</Label>
              <Select
                onValueChange={(next) => update({ default_ratio: String(next) })}
                value={value.default_ratio || ratioOptions[0] || ''}
              >
                <SelectTrigger className='h-9'>
                  <SelectValue placeholder={t('First ratio')} />
                </SelectTrigger>
                <SelectContent>
                  {ratioOptions.map((ratio) => (
                    <SelectItem key={ratio} value={ratio}>
                      {ratio}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </SettingsFormGrid>
          <SettingsSwitchRow>
            <SettingsSwitchContent>
              <Label className='text-sm'>{t('Accepts a reference image')}</Label>
              <SettingsDescription>
                {t('Shows the reference image picker for this model.')}
              </SettingsDescription>
            </SettingsSwitchContent>
            <Switch
              checked={Boolean(value.reference_image)}
              onCheckedChange={(checked) => update({ reference_image: checked })}
            />
          </SettingsSwitchRow>
        </>
      )}
    </div>
  )
}

export function PlaygroundSettingsSection({
  defaultValues,
}: {
  defaultValues: PlaygroundSettingsDefaults
}) {
  const { t } = useTranslation()
  const updateOptions = useUpdateOptionsBulk()

  const [imageEnabled, setImageEnabled] = useState<boolean>(
    defaultValues.imageEnabled ?? true
  )
  const [videoEnabled, setVideoEnabled] = useState<boolean>(
    defaultValues.videoEnabled ?? false
  )
  const [studioEnabled, setStudioEnabled] = useState<boolean>(
    defaultValues.studioEnabled ?? true
  )
  const [imageModels, setImageModels] = useState<MediaModelConfig[]>(
    normalizeImageModels(defaultValues)
  )
  const [videoModels, setVideoModels] = useState<MediaModelConfig[]>(
    normalizeVideoModels(defaultValues)
  )

  // Whole-page JSON mode: the same options, edited as one document.
  const [jsonMode, setJsonMode] = useState(false)
  const [jsonDraft, setJsonDraft] = useState('')
  const [jsonError, setJsonError] = useState('')

  const currentSettingsObject = () => ({
    image_enabled: imageEnabled,
    image_models: imageModels.filter((model) => model.model.trim() !== ''),
    video_enabled: videoEnabled,
    video_models: videoModels.filter((model) => model.model.trim() !== ''),
    studio_enabled: studioEnabled,
    image_model: imageModels[0]?.model ?? '',
    video_model: videoModels[0]?.model ?? '',
  })

  const openJsonMode = () => {
    setJsonDraft(JSON.stringify(currentSettingsObject(), null, 2))
    setJsonError('')
    setJsonMode(true)
  }

  const saveJson = async () => {
    let parsed: Record<string, unknown>
    try {
      const value: unknown = JSON.parse(jsonDraft)
      if (!value || typeof value !== 'object' || Array.isArray(value)) {
        setJsonError(t('The JSON must be an object'))
        return
      }
      parsed = value as Record<string, unknown>
    } catch (error) {
      setJsonError(
        error instanceof Error ? error.message : t('Invalid JSON document')
      )
      return
    }

    const payload: Record<string, string> = {}
    for (const [key, value] of Object.entries(parsed)) {
      if (!(key in currentSettingsObject())) continue
      payload[`playground_setting.${key}`] =
        typeof value === 'object' && value !== null
          ? JSON.stringify(value)
          : String(value)
    }
    if (Object.keys(payload).length === 0) {
      setJsonError(t('No playground setting found in the document'))
      return
    }

    try {
      await updateOptions.mutateAsync(payload)
      setJsonError('')
      toast.success(t('Playground settings saved successfully'))
    } catch {
      toast.error(t('Failed to save settings'))
    }
  }

  const { data: modelsData } = useQuery({
    queryKey: ['all-models-for-media'],
    queryFn: getAllModels,
    staleTime: 5 * 60_000,
  })
  const modelOptions = useMemo(() => {
    const raw = modelsData?.data ?? []
    return raw
      .map((entry) => (typeof entry === 'string' ? entry : entry?.id ?? ''))
      .filter(Boolean)
      .sort((a, b) => a.localeCompare(b))
  }, [modelsData])

  const save = async () => {
    try {
      await updateOptions.mutateAsync({
        'playground_setting.image_enabled': String(imageEnabled),
        'playground_setting.image_models': JSON.stringify(
          imageModels.filter((model) => model.model.trim() !== '')
        ),
        'playground_setting.video_enabled': String(videoEnabled),
        'playground_setting.video_models': JSON.stringify(
          videoModels.filter((model) => model.model.trim() !== '')
        ),
        'playground_setting.studio_enabled': String(studioEnabled),
        // The per-model list replaces the single-model fields; keep them in
        // sync so older clients and the playground tool still work.
        'playground_setting.image_model': imageModels[0]?.model ?? '',
        'playground_setting.video_model': videoModels[0]?.model ?? '',
      })
      toast.success(t('Playground settings saved successfully'))
    } catch {
      toast.error(t('Failed to save settings'))
    }
  }

  const addModel = (mode: 'image' | 'video') => {
    const entry: MediaModelConfig =
      mode === 'image'
        ? {
            model: '',
            sizes: ['1024x1024', '1024x1792', '1792x1024'],
            qualities: ['standard', 'hd'],
            default_size: '1024x1024',
            default_quality: 'standard',
          }
        : {
            model: '',
            durations: [5, 10],
            ratios: ['16:9', '9:16', '1:1'],
            qualities: ['standard', 'high'],
            default_ratio: '16:9',
            default_quality: 'standard',
            reference_image: true,
          }
    if (mode === 'image') {
      setImageModels((prev) => [...prev, entry])
    } else {
      setVideoModels((prev) => [...prev, entry])
    }
  }

  return (
    <SettingsSection title={t('Playground & Studio')}>
      <SettingsForm onSubmit={(event) => {
        event.preventDefault()
        void save()
      }}>
        {!jsonMode && (
          <SettingsPageFormActions
            isSaving={updateOptions.isPending}
            onSave={() => void save()}
          />
        )}
        <div className='flex items-start justify-between gap-3'>
          <SettingsDescription className='max-w-xl'>
            {jsonMode
              ? t(
                  'Editing every playground and studio option as one JSON document.'
                )
              : t(
                  'Pick the models the playground tools and the studio offer, with their own options.'
                )}
          </SettingsDescription>
          <Button
            onClick={() => (jsonMode ? setJsonMode(false) : openJsonMode())}
            size='sm'
            type='button'
            variant='outline'
          >
            {jsonMode ? (
              <>
                <LayoutList className='mr-1 size-3.5' />
                {t('Visual Mode')}
              </>
            ) : (
              <>
                <Braces className='mr-1 size-3.5' />
                {t('JSON Mode')}
              </>
            )}
          </Button>
        </div>

        {jsonMode ? (
          <div className='space-y-2'>
            <JsonCodeEditor
              ariaLabel={t('Playground settings JSON')}
              onChange={setJsonDraft}
              value={jsonDraft}
            />
            {jsonError && (
              <p className='text-destructive text-xs'>{jsonError}</p>
            )}
            <div className='flex justify-end gap-2'>
              <Button
                onClick={openJsonMode}
                size='sm'
                type='button'
                variant='ghost'
              >
                {t('Reset')}
              </Button>
              <Button
                disabled={updateOptions.isPending}
                onClick={() => void saveJson()}
                size='sm'
                type='button'
              >
                {t('Save JSON')}
              </Button>
            </div>
          </div>
        ) : (
        <div className='space-y-4'>
          <SettingsSwitchItem>
            <SettingsSwitchContent>
              <Label>{t('Image generation')}</Label>
              <SettingsDescription>
                {t(
                  'Offers the image tool in the playground and the image tab of the studio.'
                )}
              </SettingsDescription>
            </SettingsSwitchContent>
            <Switch checked={imageEnabled} onCheckedChange={setImageEnabled} />
          </SettingsSwitchItem>

          {imageEnabled && (
            <div className='space-y-3'>
              {imageModels.length === 0 && (
                <p className='text-muted-foreground border-border rounded-lg border border-dashed p-3 text-xs'>
                  {t('No image model configured yet.')}
                </p>
              )}
              {imageModels.map((model, index) => (
                <ModelCard
                  index={index}
                  key={`image-${index}`}
                  mode='image'
                  modelOptions={modelOptions}
                  onChange={(next) =>
                    setImageModels((prev) =>
                      prev.map((item, i) => (i === index ? next : item))
                    )
                  }
                  onRemove={() =>
                    setImageModels((prev) => prev.filter((_, i) => i !== index))
                  }
                  value={model}
                />
              ))}
              <Button
                onClick={() => addModel('image')}
                size='sm'
                type='button'
                variant='outline'
              >
                <PlusIcon className='size-4' />
                {t('Add image model')}
              </Button>
            </div>
          )}

          <SettingsSwitchItem>
            <SettingsSwitchContent>
              <Label>{t('Video generation')}</Label>
              <SettingsDescription>
                {t(
                  'Offers the video tool in the playground and the video tab of the studio.'
                )}
              </SettingsDescription>
            </SettingsSwitchContent>
            <Switch checked={videoEnabled} onCheckedChange={setVideoEnabled} />
          </SettingsSwitchItem>

          {videoEnabled && (
            <div className='space-y-3'>
              {videoModels.length === 0 && (
                <p className='text-muted-foreground border-border rounded-lg border border-dashed p-3 text-xs'>
                  {t('No video model configured yet.')}
                </p>
              )}
              {videoModels.map((model, index) => (
                <ModelCard
                  index={index}
                  key={`video-${index}`}
                  mode='video'
                  modelOptions={modelOptions}
                  onChange={(next) =>
                    setVideoModels((prev) =>
                      prev.map((item, i) => (i === index ? next : item))
                    )
                  }
                  onRemove={() =>
                    setVideoModels((prev) => prev.filter((_, i) => i !== index))
                  }
                  value={model}
                />
              ))}
              <Button
                onClick={() => addModel('video')}
                size='sm'
                type='button'
                variant='outline'
              >
                <PlusIcon className='size-4' />
                {t('Add video model')}
              </Button>
            </div>
          )}

          <SettingsSwitchItem>
            <SettingsSwitchContent>
              <Label>{t('Studio page')}</Label>
              <SettingsDescription>
                {t('Shows the studio page in the dashboard menu.')}
              </SettingsDescription>
            </SettingsSwitchContent>
            <Switch checked={studioEnabled} onCheckedChange={setStudioEnabled} />
          </SettingsSwitchItem>

        </div>
        )}
      </SettingsForm>
    </SettingsSection>
  )
}
