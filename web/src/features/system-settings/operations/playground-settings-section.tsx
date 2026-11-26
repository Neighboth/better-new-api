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
export type MediaModelConstraintConfig = {
  /** Client-side row identity only; stripped before persisting. */
  _rowId?: string
  /** Match conditions (empty array = any value matches). */
  durations?: number[]
  qualities?: string[]
  ratios?: string[]
  /** Effects (empty array / 0 = no effect). */
  durations_only?: number[]
  qualities_only?: string[]
  ratios_only?: string[]
  /** 0 = no override, -1 = reference images disabled, >=1 = cap override. */
  max_reference_images?: number
}

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
  /** Max reference images for this model (uploads + web search combined). */
  max_reference_images?: number
  /** Conditional option rules, applied in order, last match wins per field. */
  constraints?: MediaModelConstraintConfig[]
  /** '' = no video edit, 'supported' = may edit an input video, 'only' = edit-only. */
  edit_mode?: 'none' | 'supported' | 'only'
  /** The model generates an audio track with the video. */
  audio?: boolean
  /** The model accepts audio reference clips/tracks. */
  audio_reference?: boolean
  max_audio_references?: number
  /** The model accepts video reference clips. */
  video_reference?: boolean
  max_video_references?: number
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
  playgroundDefaultModel?: string
  playgroundHiddenModels?: string[]
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
  return models.map((model) => ({
    ...model,
    _rowId: model._rowId ?? nanoid(),
    constraints: model.constraints?.map((rule) =>
      rule._rowId ? rule : { ...rule, _rowId: nanoid() }
    ),
  }))
}

/** Strips the client-only row identities right before persisting. */
function stripRowIds(models: MediaModelConfig[]): MediaModelConfig[] {
  return models.map(({ _rowId, constraints, ...rest }) => ({
    ...rest,
    constraints: constraints?.map(({ _rowId: _ruleRowId, ...rule }) => rule),
  }))
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
  /** Optional — omit when the parent already renders a label. */
  label?: string
  description?: string
  values: string[]
  options?: string[]
  placeholder?: string
  allowCreate?: boolean
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
      {props.label ? <Label className='text-xs'>{props.label}</Label> : null}
      <MultiSelect
        allowCreate={props.allowCreate ?? true}
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

/** Clamps a raw <input type=number> value into [min, max]. */
function clampInt(raw: string, min: number, max: number, fallback: number): number {
  const n = Number(raw)
  if (!Number.isFinite(n)) return fallback
  return Math.min(max, Math.max(min, Math.floor(n)))
}

type MediaModelBlockProps = {
  value: MediaModelConfig
  onChange: (patch: Partial<MediaModelConfig>) => void
}

/** Shared "Reference images" block: master switch + per-model cap. */
function ReferenceImageBlock(props: MediaModelBlockProps) {
  const { t } = useTranslation()
  const { value, onChange } = props
  return (
    <div className='space-y-3 pt-3 border-t border-border/40'>
      <SettingsSwitchRow>
        <SettingsSwitchContent>
          <Label className='text-sm'>{t('Accepts reference images')}</Label>
          <SettingsDescription>
            {t('Shows the reference image picker for this model. Applies to uploads and to web-search-discovered references combined.')}
          </SettingsDescription>
        </SettingsSwitchContent>
        <Switch
          checked={Boolean(value.reference_image)}
          onCheckedChange={(checked) => onChange({ reference_image: checked })}
        />
      </SettingsSwitchRow>
      {value.reference_image && (
        <div className='flex items-center gap-2 ps-1'>
          <Label className='text-xs shrink-0'>{t('Max reference images')}</Label>
          <Input
            className='h-8 w-20 text-xs'
            max={20}
            min={1}
            onChange={(event) =>
              onChange({ max_reference_images: clampInt(event.target.value, 1, 20, 5) })
            }
            type='number'
            value={value.max_reference_images ?? 5}
          />
          <SettingsDescription>
            {t('Caps uploads and web-search-discovered references combined for this model.')}
          </SettingsDescription>
        </div>
      )}
    </div>
  )
}

type RuleRowProps = {
  rule: MediaModelConstraintConfig
  index: number
  model: MediaModelConfig
  onChange: (patch: Partial<MediaModelConstraintConfig>) => void
  onRemove: () => void
}

function RuleEditorRow(props: RuleRowProps) {
  const { t } = useTranslation()
  const { rule, index, model, onChange, onRemove } = props
  const modelDurations = (model.durations ?? []).map(String)
  const modelQualities = model.qualities ?? []
  const modelRatios = model.ratios ?? []
  const toNumbers = (raw: string[]) =>
    raw.map((item) => Number(item)).filter((item) => Number.isFinite(item) && item > 0)

  return (
    <div className='space-y-2 rounded-md border border-border/60 bg-muted/30 p-2'>
      <div className='flex items-center justify-between'>
        <span className='text-[11px] font-medium text-muted-foreground'>
          {t('Rule {{index}}', { index: index + 1 })}
        </span>
        <Button
          aria-label={t('Remove rule')}
          className='size-6'
          onClick={onRemove}
          size='icon'
          type='button'
          variant='ghost'
        >
          <XIcon className='size-3.5' />
        </Button>
      </div>
      <div className='grid grid-cols-1 sm:grid-cols-3 gap-2'>
        <TagField
          description={t('Empty = any')}
          label={t('When duration is')}
          onChange={(raw) => onChange({ durations: toNumbers(raw) })}
          options={modelDurations}
          placeholder='15, 30'
          values={(rule.durations ?? []).map(String)}
        />
        <TagField
          description={t('Empty = any')}
          label={t('When quality is')}
          onChange={(qualities) => onChange({ qualities })}
          options={modelQualities}
          placeholder='1080p'
          values={rule.qualities ?? []}
        />
        <TagField
          description={t('Empty = any')}
          label={t('When ratio is')}
          onChange={(ratios) => onChange({ ratios })}
          options={modelRatios}
          placeholder='16:9'
          values={rule.ratios ?? []}
        />
      </div>
      <div className='grid grid-cols-1 sm:grid-cols-4 gap-2'>
        <TagField
          allowCreate={false}
          description={t('Empty = keep all')}
          label={t('Offer only durations')}
          onChange={(raw) => onChange({ durations_only: toNumbers(raw) })}
          options={modelDurations}
          placeholder='10'
          values={(rule.durations_only ?? []).map(String)}
        />
        <TagField
          allowCreate={false}
          description={t('Empty = keep all')}
          label={t('Offer only qualities')}
          onChange={(qualities_only) => onChange({ qualities_only })}
          options={modelQualities}
          placeholder='720p'
          values={rule.qualities_only ?? []}
        />
        <TagField
          allowCreate={false}
          description={t('Empty = keep all')}
          label={t('Offer only ratios')}
          onChange={(ratios_only) => onChange({ ratios_only })}
          options={modelRatios}
          placeholder='16:9'
          values={rule.ratios_only ?? []}
        />
        <div className='space-y-1'>
          <Label className='text-xs'>{t('Reference images')}</Label>
          <Select
            onValueChange={(next) => onChange({ max_reference_images: Number(next) })}
            value={String(rule.max_reference_images ?? 0)}
          >
            <SelectTrigger className='h-8 text-xs'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem className='text-xs' value='0'>
                {t('Unchanged')}
              </SelectItem>
              <SelectItem className='text-xs' value='-1'>
                {t('Disabled')}
              </SelectItem>
              {Array.from({ length: 20 }, (_, position) => position + 1).map((n) => (
                <SelectItem className='text-xs' key={n} value={String(n)}>
                  {n}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>
    </div>
  )
}

/** Conditional option rules editor ("15s: no 1080p", "at 1080p: max 2 references"). */
function OptionRulesBlock(props: MediaModelBlockProps) {
  const { t } = useTranslation()
  const { value, onChange } = props
  const rules = value.constraints ?? []
  const setRules = (next: MediaModelConstraintConfig[]) => onChange({ constraints: next })

  return (
    <div className='space-y-2 pt-3 border-t border-border/40'>
      <div className='flex items-start justify-between gap-2'>
        <div>
          <Label className='text-sm font-medium'>{t('Option rules')}</Label>
          <SettingsDescription>
            {t('Narrow this model’s options for specific selections — e.g. “at 15s there is no 1080p”, “at 10s references are disabled”, “at 1080p at most 2 references”. Rules apply in order; the last matching rule wins per field.')}
          </SettingsDescription>
        </div>
        <Button
          className='shrink-0'
          onClick={() => setRules([...rules, { _rowId: nanoid() }])}
          size='sm'
          type='button'
          variant='outline'
        >
          <PlusIcon className='size-3.5' />
          {t('Add rule')}
        </Button>
      </div>
      {rules.length > 0 && (
        <div className='space-y-2'>
          {rules.map((rule, index) => (
            <RuleEditorRow
              index={index}
              key={rule._rowId ?? JSON.stringify(rule)}
              model={value}
              onChange={(patch) =>
                setRules(rules.map((item, i) => (i === index ? { ...item, ...patch } : item)))
              }
              onRemove={() => setRules(rules.filter((_, i) => i !== index))}
              rule={rule}
            />
          ))}
        </div>
      )}
    </div>
  )
}

/** Video-model capabilities: edit mode, audio generation, media references. */
function VideoCapabilitiesBlock(props: MediaModelBlockProps) {
  const { t } = useTranslation()
  const { value, onChange } = props
  return (
    <div className='space-y-3 pt-3 border-t border-border/40'>
      <div className='space-y-1 pt-3 border-t border-border/40'>
        <Label className='text-sm font-medium'>{t('Model capabilities')}</Label>
        <div className='grid grid-cols-1 sm:grid-cols-3 gap-3 pt-2'>
          <div className='space-y-1'>
            <Label className='text-xs'>{t('Video edit')}</Label>
            <Select
              onValueChange={(next) => onChange({ edit_mode: next as MediaModelConfig['edit_mode'] })}
              value={value.edit_mode ?? 'none'}
            >
              <SelectTrigger className='h-9 text-xs'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem className='text-xs' value='none'>
                  {t('No')}
                </SelectItem>
                <SelectItem className='text-xs' value='supported'>
                  {t('Supported')}
                </SelectItem>
                <SelectItem className='text-xs' value='only'>
                  {t('Edit-only')}
                </SelectItem>
              </SelectContent>
            </Select>
            <SettingsDescription>
              {t('“Supported”: the model may edit an input video. “Edit-only”: the model requires an input video and does not generate from text alone.')}
            </SettingsDescription>
          </div>
          <SettingsSwitchRow className='items-center'>
            <SettingsSwitchContent>
              <Label className='text-sm'>{t('Generates audio')}</Label>
              <SettingsDescription>
                {t('The model renders an audio track with the video.')}
              </SettingsDescription>
            </SettingsSwitchContent>
            <Switch
              checked={Boolean(value.audio)}
              onCheckedChange={(checked) => onChange({ audio: checked })}
            />
          </SettingsSwitchRow>
        </div>
        <div className='grid grid-cols-1 sm:grid-cols-2 gap-x-5 gap-y-2 pt-1'>
          <SettingsSwitchRow>
            <SettingsSwitchContent>
              <Label className='text-sm'>{t('Accepts audio references')}</Label>
              <SettingsDescription>
                {t('Audio clips the model conditions the video on.')}
              </SettingsDescription>
            </SettingsSwitchContent>
            <Switch
              checked={Boolean(value.audio_reference)}
              onCheckedChange={(checked) => onChange({ audio_reference: checked })}
            />
          </SettingsSwitchRow>
          <SettingsSwitchRow>
            <SettingsSwitchContent>
              <Label className='text-sm'>{t('Accepts video references')}</Label>
              <SettingsDescription>
                {t('Video clips the model conditions the video on.')}
              </SettingsDescription>
            </SettingsSwitchContent>
            <Switch
              checked={Boolean(value.video_reference)}
              onCheckedChange={(checked) => onChange({ video_reference: checked })}
            />
          </SettingsSwitchRow>
        </div>
        {(value.audio_reference || value.video_reference) && (
          <div className='flex flex-wrap items-center gap-4 ps-1'>
            {value.audio_reference && (
              <div className='flex items-center gap-2'>
                <Label className='text-xs shrink-0'>{t('Max audio references')}</Label>
                <Input
                  className='h-8 w-16 text-xs'
                  max={10}
                  min={1}
                  onChange={(event) =>
                    onChange({ max_audio_references: clampInt(event.target.value, 1, 10, 3) })
                  }
                  type='number'
                  value={value.max_audio_references ?? 3}
                />
              </div>
            )}
            {value.video_reference && (
              <div className='flex items-center gap-2'>
                <Label className='text-xs shrink-0'>{t('Max video references')}</Label>
                <Input
                  className='h-8 w-16 text-xs'
                  max={10}
                  min={1}
                  onChange={(event) =>
                    onChange({ max_video_references: clampInt(event.target.value, 1, 10, 2) })
                  }
                  type='number'
                  value={value.max_video_references ?? 2}
                />
              </div>
            )}
          </div>
        )}
      </div>
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
        </>
      )}
      <ReferenceImageBlock onChange={update} value={value} />
      <OptionRulesBlock onChange={update} value={value} />
      {mode === 'video' && <VideoCapabilitiesBlock onChange={update} value={value} />}
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
  const [playgroundDefaultModel, setPlaygroundDefaultModel] = useState<string>(
    defaultValues.playgroundDefaultModel ?? ''
  )
  const [playgroundHiddenModelsText, setPlaygroundHiddenModelsText] = useState<string>(
    (defaultValues.playgroundHiddenModels ?? []).join(', ')
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
    playground_default_model: playgroundDefaultModel,
    playground_hidden_models: playgroundHiddenModelsText
      .split(/[\n,]+/)
      .map((s) => s.trim())
      .filter(Boolean),
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
        'playground_setting.playground_default_model': playgroundDefaultModel.trim(),
        'playground_setting.playground_hidden_models': JSON.stringify(
          playgroundHiddenModelsText
            .split(/[\n,]+/)
            .map((s) => s.trim())
            .filter(Boolean)
        ),
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
            reference_image: false,
            max_reference_images: 5,
            default_size: '1024x1024',
            default_quality: 'standard',
          }
        : {
            _rowId: rowId,
            model: '',
            durations: [5, 10],
            ratios: ['16:9', '9:16', '1:1'],
            qualities: ['standard', 'high'],
            max_reference_images: 5,
            edit_mode: 'none',
            audio: false,
            audio_reference: false,
            max_audio_references: 3,
            video_reference: false,
            max_video_references: 2,
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
                  {t('Required — always on. Assists every generation in the Studio — it rewrites each prompt into an exhaustive, physics-consistent scene description before the job is submitted. Dialogue and quotes are preserved verbatim, in their original language. The model must support image (vision) input.')}
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
                <TagField
                  onChange={(values) => {
                    // Persist as a comma-separated list; duplicates are harmless (trimmed on read).
                    setPromptOptimizerModel(
                      [...new Set(values.map((v) => v.trim()).filter(Boolean))].join(', ')
                    )
                  }}
                  options={modelOptions}
                  placeholder='gpt-4o-mini, gemini-2.5-flash'
                  values={
                    promptOptimizerModel
                      .split(',')
                      .map((v) => v.trim())
                      .filter(Boolean)
                  }
                />
                <p className='text-muted-foreground text-[11px]'>
                  {t('Required. One or more vision-capable chat models. With several configured, the Studio shows a selector in the composer — the first model is the default.')}
                </p>
                <div className='rounded-lg border border-amber-500/30 bg-amber-500/10 px-3 py-2 text-[11px] leading-relaxed text-amber-800 dark:text-amber-300'>
                  {t('Note: Requests to this prompt enhancement model (and its internal search steps) are exempt from user rate limits and do not deduct from user balance.')}
                </div>
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

          <div className='border border-border rounded-xl p-4 bg-card/50 space-y-3 shadow-xs'>
            <div className='space-y-1.5'>
              <Label className='text-xs font-medium'>{t('Playground Default Model')}</Label>
              <Input
                className='font-mono text-xs'
                placeholder='gpt-4o, claude-3-5-sonnet...'
                value={playgroundDefaultModel}
                onChange={(e) => setPlaygroundDefaultModel(e.target.value)}
              />
              <p className='text-muted-foreground text-[11px]'>
                {t('Default model preselected when opening the Playground chat.')}
              </p>
            </div>
            <div className='space-y-1.5'>
              <Label className='text-xs font-medium'>{t('Hidden Models in Playground')}</Label>
              <Input
                className='font-mono text-xs'
                placeholder='whisper-1, tts-1, text-embedding-3-small'
                value={playgroundHiddenModelsText}
                onChange={(e) => setPlaygroundHiddenModelsText(e.target.value)}
              />
              <p className='text-muted-foreground text-[11px]'>
                {t('Comma-separated model IDs to hide from the Playground model selector.')}
              </p>
            </div>
          </div>

        </div>
        )}
      </SettingsForm>
    </SettingsSection>
  )
}
