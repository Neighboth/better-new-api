import { useQuery } from '@tanstack/react-query'
import { Braces, LayoutList, PlusIcon, XIcon } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { nanoid } from 'nanoid'
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
import { Textarea } from '@/components/ui/textarea'
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
  /** Client-side React row identity only; it is stripped before persisting. */
  _rowId?: string
  model: string
  tag?: string
  sizes?: string[]
  qualities?: string[]
  ratios?: string[]
  durations?: number[]
  reference_image?: boolean
  default_size?: string
  default_quality?: string
  default_ratio?: string
  default_duration?: number
}

export type PlaygroundSettingsDefaults = {
  imageEnabled?: boolean
  imageModels?: MediaModelConfig[]
  videoEnabled?: boolean
  videoModels?: MediaModelConfig[]
  studioEnabled?: boolean
  studioDefaultModel?: string
  promptOptimizerEnabled?: boolean
  promptOptimizerModel?: string
  promptOptimizerSystemPrompt?: string
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
  videoDefaultDuration?: number
  videoReferenceImage?: boolean
}

function splitLegacyList(raw?: string): string[] {
  if (!raw) return []
  return raw
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)
}

/** Assigns a stable row identity for React keys when the entry lacks one. */
function ensureRowIds(models: MediaModelConfig[]): MediaModelConfig[] {
  return models.map((model) =>
    model._rowId ? model : { ...model, _rowId: nanoid() }
  )
}

/** Strips the client-only row identity right before persisting. */
function stripRowIds(models: MediaModelConfig[]): MediaModelConfig[] {
  return models.map(({ _rowId, ...rest }) => rest)
}

/**
 * Builds the model list of the runtime configuration.
 *
 * Installations that configured the studio before the per-model options
 * existed only have the single-model fields; their values become the first
 * entry of the list so nothing is lost when the page is opened.
 */
function normalizeImageModels(
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

function normalizeVideoModels(
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
      default_duration: defaults.videoDefaultDuration,
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
  const durationOptions = useMemo(() => value.durations ?? [], [value.durations])

  return (
    <div className='border border-border rounded-xl p-4 bg-card/50 space-y-3 shadow-xs'>
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
        <div className='w-28 sm:w-36 space-y-1'>
          <Label className='text-xs'>{t('Tag (optional)')}</Label>
          <Input
            className='text-xs'
            placeholder='Fast, Pro, HQ...'
            value={value.tag ?? ''}
            onChange={(event) => update({ tag: event.target.value })}
          />
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
            <div className='grid grid-cols-1 sm:grid-cols-3 gap-3'>
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
              <div className='space-y-1'>
                <Label className='text-xs'>{t('Default duration')}</Label>
                <Select
                  onValueChange={(next) =>
                    update({ default_duration: Number(next) || undefined })
                  }
                  value={(() => {
                    if (value.default_duration) return String(value.default_duration)
                    if (durationOptions[0] != null) return String(durationOptions[0])
                    return ''
                  })()}
                >
                  <SelectTrigger className='h-9'>
                    <SelectValue placeholder={t('First duration')} />
                  </SelectTrigger>
                  <SelectContent>
                    {durationOptions.map((duration) => (
                      <SelectItem key={duration} value={String(duration)}>
                        {duration}s
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
  const [studioDefaultModel, setStudioDefaultModel] = useState<string>(
    defaultValues.studioDefaultModel ?? ''
  )
  const [imageModels, setImageModels] = useState<MediaModelConfig[]>(() =>
    ensureRowIds(normalizeImageModels(defaultValues))
  )
  const [videoModels, setVideoModels] = useState<MediaModelConfig[]>(() =>
    ensureRowIds(normalizeVideoModels(defaultValues))
  )
  const [promptOptimizerModel, setPromptOptimizerModel] = useState<string>(
    defaultValues.promptOptimizerModel ?? ''
  )
  const [promptOptimizerSystemPrompt, setPromptOptimizerSystemPrompt] = useState<string>(
    defaultValues.promptOptimizerSystemPrompt ??
      'You are a professional video generation prompt optimizer and safety enhancer. Rewrite the given prompt into safe, detailed, cinematic English that generative video AI models accept without content violations. Describe camera movement, lighting, visual details, and style cleanly and vividly. CRITICAL REQUIREMENT: If the prompt contains any dialogue, spoken quotes, or speech lines, DO NOT translate the spoken dialogue — keep all spoken words/quotes in Turkish exactly as written. Output ONLY the optimized prompt text without any introductory text, explanation, or quotes around the prompt.'
  )

  // Whole-page JSON mode: the same options, edited as one document.
  const [jsonMode, setJsonMode] = useState(false)
  const [jsonDraft, setJsonDraft] = useState('')
  const [jsonError, setJsonError] = useState('')

  const currentSettingsObject = () => ({
    image_enabled: imageEnabled,
    image_models: stripRowIds(
      imageModels.filter((model) => model.model.trim() !== '')
    ),
    video_enabled: videoEnabled,
    video_models: stripRowIds(
      videoModels.filter((model) => model.model.trim() !== '')
    ),
    studio_enabled: studioEnabled,
    studio_default_model: studioDefaultModel,
    prompt_optimizer_enabled: true,
    prompt_optimizer_model: promptOptimizerModel,
    prompt_optimizer_system_prompt: promptOptimizerSystemPrompt,
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

    const optimizerModel = String(parsed.prompt_optimizer_model ?? '').trim()
    if (!optimizerModel) {
      setJsonError(t('A prompt optimizer model is required for Studio generations'))
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
    payload['playground_setting.prompt_optimizer_enabled'] = 'true'
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
    if (!promptOptimizerModel.trim()) {
      toast.error(t('A prompt optimizer model is required for Studio generations'))
      return
    }
    try {
      await updateOptions.mutateAsync({
        'playground_setting.image_enabled': String(imageEnabled),
        'playground_setting.image_models': JSON.stringify(
          stripRowIds(imageModels.filter((model) => model.model.trim() !== ''))
        ),
        'playground_setting.video_enabled': String(videoEnabled),
        'playground_setting.video_models': JSON.stringify(
          stripRowIds(videoModels.filter((model) => model.model.trim() !== ''))
        ),
        'playground_setting.studio_enabled': String(studioEnabled),
        'playground_setting.studio_default_model': studioDefaultModel,
        'playground_setting.prompt_optimizer_enabled': 'true',
        'playground_setting.prompt_optimizer_model': promptOptimizerModel.trim(),
        'playground_setting.prompt_optimizer_system_prompt': promptOptimizerSystemPrompt,
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
    const rowId = nanoid()
    const entry: MediaModelConfig =
      mode === 'image'
        ? {
            _rowId: rowId,
            model: '',
            sizes: ['1024x1024', '1024x1792', '1792x1024'],
            qualities: ['standard', 'hd'],
            default_size: '1024x1024',
            default_quality: 'standard',
          }
        : {
            _rowId: rowId,
            model: '',
            durations: [5, 10],
            ratios: ['16:9', '9:16', '1:1'],
            qualities: ['standard', 'high'],
            default_ratio: '16:9',
            default_duration: 5,
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
                  key={model._rowId}
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
                  key={model._rowId}
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

          <div className='border border-border rounded-xl p-4 bg-card/50 space-y-4 shadow-xs'>
            <SettingsSwitchRow>
              <SettingsSwitchContent>
                <Label className='text-sm font-medium'>
                  {t('Generation Assistant Model')}
                  <span className='text-destructive' aria-hidden='true'> *</span>
                </Label>
                <SettingsDescription>
                  {t('Required — always on. Assists every generation in the Studio — it rewrites prompts, plans multi-part (chained) videos as one continuous take, and refines each part against frames captured from the actual previous clip. Dialogue and quotes are preserved verbatim, in their original language. The model must support image (vision) input.')}
                </SettingsDescription>
              </SettingsSwitchContent>
              <span className='shrink-0 rounded-md border border-border bg-muted/40 px-2 py-1 text-[10px] font-semibold uppercase tracking-wide text-muted-foreground'>
                {t('Always on')}
              </span>
            </SettingsSwitchRow>

            <div className='space-y-3 pt-2 border-t border-border/40'>
              <div className='space-y-1.5'>
                <Label className='text-xs font-medium'>
                  {t('Assistant Model')}
                  <span className='text-destructive' aria-hidden='true'> *</span>
                </Label>
                <Input
                  className='font-mono text-xs'
                  list='playground-optimizer-models'
                  placeholder='gpt-4o-mini, gemini-2.5-flash...'
                  value={promptOptimizerModel}
                  onChange={(e) => setPromptOptimizerModel(e.target.value)}
                  aria-required='true'
                />
                <datalist id='playground-optimizer-models'>
                  {modelOptions.map((model) => (
                    <option key={model} value={model} />
                  ))}
                </datalist>
                <p className='text-muted-foreground text-[11px]'>
                  {t('Required. A fast, vision-capable chat model that plans and refines every generation in the Studio, for images and multi-part videos.')}
                </p>
              </div>

              <div className='space-y-1.5'>
                <Label className='text-xs font-medium'>{t('Assistant System Prompt')}</Label>
                <Textarea
                  rows={4}
                  className='text-xs font-mono'
                  value={promptOptimizerSystemPrompt}
                  onChange={(e) => setPromptOptimizerSystemPrompt(e.target.value)}
                  placeholder={t('System instructions for prompt rewriting...')}
                />
                <p className='text-muted-foreground text-[11px]'>
                  {t('Instructs the assistant. Dialogue and quotes are preserved verbatim, in their original language.')}
                </p>
              </div>
            </div>
          </div>

          <SettingsSwitchItem>
            <SettingsSwitchContent>
              <Label>{t('Studio page')}</Label>
              <SettingsDescription>
                {t('Shows the studio page in the dashboard menu.')}
              </SettingsDescription>
            </SettingsSwitchContent>
            <Switch checked={studioEnabled} onCheckedChange={setStudioEnabled} />
          </SettingsSwitchItem>

          {studioEnabled && (
            <div className='border border-border rounded-xl p-4 bg-card/50 space-y-2 shadow-xs'>
              <Label className='text-xs font-medium'>{t('Studio Default Model')}</Label>
              <Input
                className='font-mono text-xs'
                list='playground-all-studio-models'
                placeholder='seedance-2.5, dall-e-3...'
                value={studioDefaultModel}
                onChange={(e) => setStudioDefaultModel(e.target.value)}
              />
              <datalist id='playground-all-studio-models'>
                {[
                  ...videoModels.map((m) => m.model),
                  ...imageModels.map((m) => m.model),
                ]
                  .filter(Boolean)
                  .map((m) => (
                    <option key={m} value={m} />
                  ))}
              </datalist>
              <p className='text-muted-foreground text-[11px]'>
                {t('When configured, opening Studio automatically preselects this model and its corresponding mode (video or image).')}
              </p>
            </div>
          )}

        </div>
        )}
      </SettingsForm>
    </SettingsSection>
  )
}
