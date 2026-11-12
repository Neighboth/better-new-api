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
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Check, ChevronDown, Loader2, Trash2 } from 'lucide-react'
import { useEffect, useState, useCallback, useMemo, useRef } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

import {
  SideDrawerSection,
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
  sideDrawerSwitchItemClassName,
} from '@/components/drawer-layout'
import { JsonEditor } from '@/components/json-editor'
import { TagInput } from '@/components/tag-input'
import { Button } from '@/components/ui/button'
import { ADMIN_PERMISSION_RESOURCES, hasPermission } from '@/lib/admin-permissions'
import { PIN_COLOR_OPTIONS } from '@/lib/pin-color'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import {
  useSystemOptions,
  getOptionValue,
} from '@/features/system-settings/hooks/use-system-options'
import { useUpdateOption } from '@/features/system-settings/hooks/use-update-option'
import { normalizeJsonString } from '@/features/system-settings/models/utils'
import type { ModelSettings } from '@/features/system-settings/types'
import { safeJsonParse } from '@/features/system-settings/utils/json-parser'

import { createModel, updateModel, getModel, getVendors } from '../../api'
import { getNameRuleOptions, ENDPOINT_TEMPLATES } from '../../constants'
import { modelsQueryKeys, vendorsQueryKeys, parseModelTags } from '../../lib'
import type { Model } from '../../types'

// Extended schema for ratio configuration (internal form state only)
const extendedModelFormSchema = z.object({
  id: z.number().optional(),
  model_name: z.string().min(1, 'Model name is required'),
  display_name: z.string().optional(),
  tagline: z.string().optional(),
  property_boxes: z.string().optional(),
  localizations: z.string().optional(),
  description: z.string(),
  icon: z.string(),
  tags: z.array(z.string()),
  vendor_id: z.number().optional(),
  endpoints: z.string(),
  name_rule: z.number(),
  status: z.boolean(),
  sync_official: z.boolean(),
  is_pinned: z.boolean(),
  pin_order: z.number().optional(),
  pin_color: z.string().optional(),
  announcement_bar: z.string().optional(),
  price: z.string().optional(),
  ratio: z.string().optional(),
  cacheRatio: z.string().optional(),
  completionRatio: z.string().optional(),
  imageRatio: z.string().optional(),
  audioRatio: z.string().optional(),
  audioCompletionRatio: z.string().optional(),
})

type ExtendedModelFormValues = z.infer<typeof extendedModelFormSchema>

const MODEL_LANGUAGES = [
  { code: 'tr', flag: '🇹🇷', label: 'Türkçe' },
  { code: 'en', flag: '🇬🇧', label: 'English' },
  { code: 'zh_CN', flag: '🇨🇳', label: '简体中文' },
  { code: 'zh_TW', flag: '🇹🇼', label: '繁體中文' },
  { code: 'fr', flag: '🇫🇷', label: 'Français' },
  { code: 'ru', flag: '🇷🇺', label: 'Русский' },
  { code: 'ja', flag: '🇯🇵', label: '日本語' },
  { code: 'vi', flag: '🇻🇳', label: 'Tiếng Việt' },
] as const

type LocalizedModelData = {
  display_names: Record<string, string>
  descriptions: Record<string, string>
  taglines: Record<string, string>
  property_boxes: Record<string, Array<{ label: string; value: string }>>
}

type PricingMode = 'per-token' | 'per-request' | 'duration_second' | 'characters'
type PricingSubMode = 'ratio' | 'price'
type ChargeScope = 'all' | 'input_only' | 'output_only'

type PricingFields = Pick<
  ExtendedModelFormValues,
  | 'price'
  | 'ratio'
  | 'cacheRatio'
  | 'completionRatio'
  | 'imageRatio'
  | 'audioRatio'
  | 'audioCompletionRatio'
>

// Form state describing the pricing currently configured for one model name.
type PricingConfig = {
  mode: PricingMode
  chargeScope: ChargeScope
  fields: PricingFields
  promptPrice: string
  completionPrice: string
  advancedOpen: boolean
}

const EMPTY_PRICING_FIELDS: PricingFields = {
  price: '',
  ratio: '',
  cacheRatio: '',
  completionRatio: '',
  imageRatio: '',
  audioRatio: '',
  audioCompletionRatio: '',
}

const EMPTY_PRICING_CONFIG: PricingConfig = {
  mode: 'per-token',
  chargeScope: 'all',
  fields: EMPTY_PRICING_FIELDS,
  promptPrice: '',
  completionPrice: '',
  advancedOpen: false,
}

function lookupModelRatio(
  rawMap: string,
  modelName: string
): number | undefined {
  return safeJsonParse<Record<string, number>>(rawMap, {
    fallback: {},
    silent: true,
  })[modelName]
}

// Pricing is not stored on the model row: it lives in system options as
// model-name keyed JSON maps, so it has to be read back out of those maps to
// populate the form. Both create and edit rely on this, because submit rebuilds
// the maps from the form and would otherwise drop pricing it never loaded.
function readPricingConfig(
  settings: ModelSettings | null,
  modelName: string
): PricingConfig {
  if (!settings || !modelName) return EMPTY_PRICING_CONFIG

  const billingModeMap = safeJsonParse<Record<string, string>>(
    settings['billing_setting.billing_mode'],
    { fallback: {}, silent: true }
  )
  const savedMode = billingModeMap[modelName]

  const price = lookupModelRatio(settings.ModelPrice, modelName)
  const ratio = lookupModelRatio(settings.ModelRatio, modelName)
  const cacheRatio = lookupModelRatio(settings.CacheRatio, modelName)
  const completionRatio = lookupModelRatio(settings.CompletionRatio, modelName)
  const imageRatio = lookupModelRatio(settings.ImageRatio, modelName)
  const audioRatio = lookupModelRatio(settings.AudioRatio, modelName)
  const audioCompletionRatio = lookupModelRatio(
    settings.AudioCompletionRatio,
    modelName
  )

  // A fixed per-request price wins outright at billing time (see
  // GetModelRatioOrPrice), so a name that has one is shown, and saved back, as
  // price-only: the ratios alongside it are dead weight.
  if (price !== undefined && price !== null) {
    return {
      ...EMPTY_PRICING_CONFIG,
      mode: 'per-request',
      fields: { ...EMPTY_PRICING_FIELDS, price: price.toString() },
    }
  }

  let promptPrice = ''
  let completionPrice = ''
  if (ratio !== undefined && ratio !== null) {
    const tokenPrice = ratio * 2
    promptPrice = tokenPrice.toString()
    if (completionRatio !== undefined && completionRatio !== null) {
      completionPrice = (tokenPrice * completionRatio).toString()
    }
  }

  let mode: PricingMode = 'per-token'
  let chargeScope: ChargeScope = 'all'

  if (savedMode === 'duration_second') {
    mode = 'duration_second'
  } else if (savedMode === 'characters') {
    mode = 'characters'
  } else if (savedMode === 'input_only') {
    mode = 'per-token'
    chargeScope = 'input_only'
  } else if (savedMode === 'output_only') {
    mode = 'per-token'
    chargeScope = 'output_only'
  }

  return {
    mode,
    chargeScope,
    fields: {
      price: '',
      ratio: ratio?.toString() || '',
      cacheRatio: cacheRatio?.toString() || '',
      completionRatio: completionRatio?.toString() || '',
      imageRatio: imageRatio?.toString() || '',
      audioRatio: audioRatio?.toString() || '',
      audioCompletionRatio: audioCompletionRatio?.toString() || '',
    },
    promptPrice,
    completionPrice,
    // Configured is not the same as non-zero: a 0 ratio (free cache reads, for
    // instance) still has to be visible rather than hidden behind the collapse.
    advancedOpen: [
      cacheRatio,
      imageRatio,
      audioRatio,
      audioCompletionRatio,
    ].some((value) => value !== undefined && value !== null),
  }
}

type ModelMutateDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow?: Model | null
}

export function ModelMutateDrawer({
  open,
  onOpenChange,
  currentRow,
}: ModelMutateDrawerProps) {
  const { t } = useTranslation()
  const user = useAuthStore((s) => s.auth.user)
  const canWrite = hasPermission(user, ADMIN_PERMISSION_RESOURCES.MODEL, 'write')
  const queryClient = useQueryClient()
  const currentModelId = currentRow?.id
  const isEditing = Boolean(currentModelId)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [pricingMode, setPricingMode] = useState<PricingMode>('per-token')
  const [chargeScope, setChargeScope] = useState<ChargeScope>('all')
  const [pricingSubMode, setPricingSubMode] = useState<PricingSubMode>('ratio')
  const [advancedOpen, setAdvancedOpen] = useState(false)
  const [propertyBoxes, setPropertyBoxes] = useState<Array<{ label: string; value: string }>>([])
  const [activeModelLang, setActiveModelLang] = useState<string>('tr')
  const [localizedData, setLocalizedData] = useState<LocalizedModelData>({
    display_names: {},
    descriptions: {},
    taglines: {},
    property_boxes: {},
  })
  const [promptPrice, setPromptPrice] = useState('')
  const [completionPrice, setCompletionPrice] = useState('')
  const [oldModelName, setOldModelName] = useState<string>('')
  // Model name whose pricing was read into the form when the drawer opened.
  // Submit may only rewrite pricing for this name, or for a name the user
  // explicitly priced; anything else it never saw and must leave alone.
  const [loadedPricingName, setLoadedPricingName] = useState<string>('')
  // Keep a ref so the load effect can read the latest modelSettings without
  // depending on it: modelSettings is a fresh object on every system-options
  // refetch, and including it in the deps would reset the form under the user.
  const modelSettingsRef = useRef<ModelSettings | null>(null)

  // Fetch vendors for dropdown
  const { data: vendorsData } = useQuery({
    queryKey: vendorsQueryKeys.list(),
    queryFn: () => getVendors({ page_size: 1000 }),
    enabled: open,
  })

  const vendors = vendorsData?.data?.items || []

  // Fetch model detail if editing
  const { data: modelData } = useQuery({
    queryKey: modelsQueryKeys.detail(currentModelId || 0),
    queryFn: () => {
      if (!currentModelId) {
        throw new Error('Model ID is required')
      }
      return getModel(currentModelId)
    },
    enabled: open && isEditing,
  })

  // Fetch system options for ratio configuration
  const { data: systemOptionsData } = useSystemOptions()

  const updateOption = useUpdateOption()

  // Get model settings from system options
  const modelSettings = useMemo(() => {
    if (!systemOptionsData?.data) return null
    const defaultModelSettings: ModelSettings = {
      'global.pass_through_request_enabled': false,
      'global.thinking_model_blacklist': '[]',
      'global.chat_completions_to_responses_policy': '{}',
      'general_setting.ping_interval_enabled': false,
      'general_setting.ping_interval_seconds': 60,
      'gemini.safety_settings': '',
      'gemini.version_settings': '',
      'gemini.supported_imagine_models': '',
      'gemini.thinking_adapter_enabled': false,
      'gemini.thinking_adapter_budget_tokens_percentage': 0.6,
      'gemini.function_call_thought_signature_enabled': false,
      'gemini.remove_function_response_id_enabled': true,
      'claude.model_headers_settings': '',
      'claude.default_max_tokens': '',
      'claude.thinking_adapter_enabled': true,
      'claude.thinking_adapter_budget_tokens_percentage': 0.8,
      ModelPrice: '',
      ModelRatio: '',
      CacheRatio: '',
      CompletionRatio: '',
      ImageRatio: '',
      AudioRatio: '',
      AudioCompletionRatio: '',
      ExposeRatioEnabled: false,
      'billing_setting.billing_mode': '{}',
      'billing_setting.billing_expr': '{}',
      'tool_price_setting.prices': '{}',
      TopupGroupRatio: '',
      GroupRatio: '',
      UserUsableGroups: '',
      GroupGroupRatio: '',
      AutoGroups: '',
      MaxTokenAutoGroups: 5,
      DefaultUseAutoGroup: false,
      CreateCacheRatio: '',
      'group_ratio_setting.group_special_usable_group': '{}',
      'grok.violation_deduction_enabled': false,
      'grok.violation_deduction_amount': 0,
      RetryTimes: 0,
      ChannelDisableThreshold: '',
      AutomaticDisableChannelEnabled: false,
      AutomaticEnableChannelEnabled: false,
      AutomaticDisableKeywords: '',
      AutomaticDisableStatusCodes: '401',
      AutomaticRetryStatusCodes:
        '100-199,300-399,401-407,409-499,500-503,505-523,525-599',
      'monitor_setting.auto_test_channel_enabled': false,
      'monitor_setting.auto_test_channel_minutes': 10,
      'monitor_setting.channel_test_concurrency': 1,
      'monitor_setting.channel_test_mode': 'scheduled_all',
      'channel_affinity_setting.enabled': false,
      'channel_affinity_setting.switch_on_success': true,
      'channel_affinity_setting.keep_on_channel_disabled': false,
      'channel_affinity_setting.max_entries': 100000,
      'channel_affinity_setting.default_ttl_seconds': 3600,
      'channel_affinity_setting.rules': '[]',
      'model_deployment.ionet.api_key': '',
      'model_deployment.ionet.enabled': false,
    }
    return getOptionValue(systemOptionsData.data, defaultModelSettings)
  }, [systemOptionsData])

  // The load effect keys off this boolean, not the object: it re-runs once
  // when the settings first arrive (so a drawer opened before that still gets
  // its pricing prefilled), while later refetches only produce a new object
  // reference and must not reset a form the user may be editing.
  const hasModelSettings = modelSettings !== null
  useEffect(() => {
    modelSettingsRef.current = modelSettings
  })

  const form = useForm<ExtendedModelFormValues>({
    resolver: zodResolver(extendedModelFormSchema),
    defaultValues: {
      model_name: '',
      description: '',
      icon: '',
      tags: [],
      vendor_id: undefined,
      endpoints: '',
      name_rule: 0,
      status: true,
      sync_official: true,
      is_pinned: false,
      pin_order: 0,
      pin_color: 'amber',
      announcement_bar: '',
      price: '',
      ratio: '',
      cacheRatio: '',
      completionRatio: '',
      imageRatio: '',
      audioRatio: '',
      audioCompletionRatio: '',
    },
  })

  // The virtual "auto" model has no price of its own: the request is billed
  // as the model it picks. The pricing section is meaningless for it.
  const watchedModelName = form.watch('model_name')
  const isAutoModel =
    (watchedModelName ?? '').trim().toLowerCase() === 'auto'

  const validateNumber = (value: string) => {
    if (value === '') return true
    return !Number.isNaN(Number.parseFloat(value))
  }

  const handlePromptPriceChange = (value: string) => {
    setPromptPrice(value)
    if (value && !Number.isNaN(Number.parseFloat(value))) {
      const ratio = Number.parseFloat(value) / 2
      form.setValue('ratio', ratio.toString())
    } else {
      form.setValue('ratio', '')
    }
  }

  const handleCompletionPriceChange = (value: string) => {
    setCompletionPrice(value)
    if (
      value &&
      !Number.isNaN(Number.parseFloat(value)) &&
      promptPrice &&
      !Number.isNaN(Number.parseFloat(promptPrice)) &&
      Number.parseFloat(promptPrice) > 0
    ) {
      const completionRatio =
        Number.parseFloat(value) / Number.parseFloat(promptPrice)
      form.setValue('completionRatio', completionRatio.toString())
    } else {
      form.setValue('completionRatio', '')
    }
  }

  // Load model data for editing and ratio configuration
  useEffect(() => {
    if (open && isEditing && modelData?.data) {
      const model = modelData.data
      setOldModelName(model.model_name)

      const pricing = readPricingConfig(
        modelSettingsRef.current,
        model.model_name
      )
      setLoadedPricingName(model.model_name)
      setPricingMode(pricing.mode)
      setChargeScope(pricing.chargeScope)
      setPromptPrice(pricing.promptPrice)
      setCompletionPrice(pricing.completionPrice)
      setAdvancedOpen(pricing.advancedOpen)
      let locData: LocalizedModelData = {
        display_names: {},
        descriptions: {},
        taglines: {},
        property_boxes: {},
      }
      if (model.localizations) {
        try {
          const parsed = JSON.parse(model.localizations)
          locData = {
            display_names: parsed.display_names || {},
            descriptions: parsed.descriptions || {},
            taglines: parsed.taglines || {},
            property_boxes: parsed.property_boxes || {},
          }
        } catch {}
      }
      if (!locData.display_names['tr'] && model.display_name) locData.display_names['tr'] = model.display_name
      if (!locData.descriptions['tr'] && model.description) locData.descriptions['tr'] = model.description
      if (!locData.taglines['tr'] && model.tagline) locData.taglines['tr'] = model.tagline

      try {
        const pb = model.property_boxes ? JSON.parse(model.property_boxes) : []
        const parsedPb = Array.isArray(pb) ? pb.slice(0, 8) : []
        setPropertyBoxes(parsedPb)
        if (!locData.property_boxes['tr'] && parsedPb.length > 0) locData.property_boxes['tr'] = parsedPb
      } catch {
        setPropertyBoxes([])
      }
      setLocalizedData(locData)
      form.reset({
        id: model.id,
        model_name: model.model_name,
        display_name: model.display_name || '',
        tagline: model.tagline || '',
        property_boxes: model.property_boxes || '',
        description: model.description || '',
        icon: model.icon || '',
        tags: parseModelTags(model.tags),
        vendor_id: model.vendor_id,
        endpoints: model.endpoints || '',
        name_rule: model.name_rule || 0,
        status: model.status === 1,
        sync_official: model.sync_official === 1,
        is_pinned: Boolean(model.is_pinned),
        pin_order: model.pin_order ?? 0,
        pin_color: model.pin_color || 'amber',
        announcement_bar: model.announcement_bar || '',
        ...pricing.fields,
      })
    } else if (open && !isEditing) {
      // Pre-fill model name if passed from missing models, along with any
      // pricing that name already has, so the user edits it instead of being
      // shown an empty form that hides existing configuration.
      const modelName = currentRow?.model_name || ''
      const pricing = readPricingConfig(modelSettingsRef.current, modelName)
      setOldModelName('')
      setLoadedPricingName(modelName)
      setPricingSubMode('ratio')
      setPropertyBoxes([])
      setLocalizedData({
        display_names: {},
        descriptions: {},
        taglines: {},
        property_boxes: {},
      })
      setPricingMode(pricing.mode)
      setChargeScope(pricing.chargeScope)
      setPromptPrice(pricing.promptPrice)
      setCompletionPrice(pricing.completionPrice)
      setAdvancedOpen(pricing.advancedOpen)
      setPropertyBoxes([])
      form.reset({
        model_name: modelName,
        display_name: '',
        tagline: '',
        property_boxes: '',
        description: '',
        icon: '',
        tags: [],
        vendor_id: undefined,
        endpoints: '',
        name_rule: 0,
        status: true,
        sync_official: true,
        is_pinned: false,
        pin_order: 0,
        pin_color: 'amber',
        announcement_bar: '',
        ...pricing.fields,
      })
    }
  }, [open, isEditing, modelData, currentRow, form, hasModelSettings])

  const onSubmit = useCallback(
    async (values: ExtendedModelFormValues): Promise<void> => {
      setIsSubmitting(true)
      try {
        const submitData = {
          ...values,
          localizations: JSON.stringify(localizedData),
          id: isEditing ? currentModelId : undefined,
          tags: Array.isArray(values.tags) ? values.tags.join(',') : '',
          status: values.status ? 1 : 0,
          sync_official: values.sync_official ? 1 : 0,
        }

        // Remove ratio fields from model data (they're stored in system settings)
        const {
          price,
          ratio,
          cacheRatio,
          completionRatio,
          imageRatio,
          audioRatio,
          audioCompletionRatio,
          ...modelData
        } = submitData

        const response =
          isEditing && currentModelId
            ? await updateModel({ ...modelData, id: currentModelId })
            : await createModel(modelData)

        if (response.success) {
          // Handle ratio configuration updates in system settings
          const finalModelName = values.model_name
          const isSet = (v: any) => v !== undefined && v !== null && v !== ''
          const hasRatioConfig =
            (pricingMode === 'per-request' && isSet(values.price)) ||
            ((pricingMode === 'per-token' ||
              pricingMode === 'duration_second' ||
              pricingMode === 'characters') &&
              (isSet(values.ratio) ||
                isSet(values.cacheRatio) ||
                isSet(values.completionRatio) ||
                isSet(values.imageRatio) ||
                isSet(values.audioRatio) ||
                isSet(values.audioCompletionRatio)))

          // The auto model has no pricing to write: its price is the price of
          // the model it picks, so the maps stay untouched.
          const skipPricing = (finalModelName ?? '').trim().toLowerCase() === 'auto'

          // Always process system settings updates if we have modelSettings
          // This ensures we can remove stale entries even when clearing all pricing fields
          if (modelSettings && !skipPricing) {
            // Read existing configurations
            const priceMap = safeJsonParse<Record<string, number>>(
              modelSettings.ModelPrice,
              { fallback: {}, silent: true }
            )
            const ratioMap = safeJsonParse<Record<string, number>>(
              modelSettings.ModelRatio,
              { fallback: {}, silent: true }
            )
            const cacheMap = safeJsonParse<Record<string, number>>(
              modelSettings.CacheRatio,
              { fallback: {}, silent: true }
            )
            const completionMap = safeJsonParse<Record<string, number>>(
              modelSettings.CompletionRatio,
              { fallback: {}, silent: true }
            )
            const imageMap = safeJsonParse<Record<string, number>>(
              modelSettings.ImageRatio,
              { fallback: {}, silent: true }
            )
            const audioMap = safeJsonParse<Record<string, number>>(
              modelSettings.AudioRatio,
              { fallback: {}, silent: true }
            )
            const audioCompletionMap = safeJsonParse<Record<string, number>>(
              modelSettings.AudioCompletionRatio,
              { fallback: {}, silent: true }
            )
            const billingModeMap = safeJsonParse<Record<string, string>>(
              modelSettings['billing_setting.billing_mode'],
              { fallback: {}, silent: true }
            )

            // Remove old model name entries if model name changed (always, even if no new config)
            if (isEditing && oldModelName && oldModelName !== finalModelName) {
              delete priceMap[oldModelName]
              delete ratioMap[oldModelName]
              delete cacheMap[oldModelName]
              delete completionMap[oldModelName]
              delete imageMap[oldModelName]
              delete audioMap[oldModelName]
              delete audioCompletionMap[oldModelName]
              delete billingModeMap[oldModelName]
            }

            // Rebuild this model name's entries from the form
            if (hasRatioConfig || finalModelName === loadedPricingName) {
              delete priceMap[finalModelName]
              delete ratioMap[finalModelName]
              delete cacheMap[finalModelName]
              delete completionMap[finalModelName]
              delete imageMap[finalModelName]
              delete audioMap[finalModelName]
              delete audioCompletionMap[finalModelName]
              delete billingModeMap[finalModelName]
            }

            // Only add new entries if user provided new configuration
            if (hasRatioConfig) {
              if (pricingMode === 'per-request' && isSet(values.price)) {
                priceMap[finalModelName] = Number.parseFloat(String(values.price))
              } else if (pricingMode === 'duration_second') {
                billingModeMap[finalModelName] = 'duration_second'
                if (isSet(values.ratio)) {
                  ratioMap[finalModelName] = Number.parseFloat(String(values.ratio))
                }
              } else if (pricingMode === 'characters') {
                billingModeMap[finalModelName] = 'characters'
                if (isSet(values.ratio)) {
                  ratioMap[finalModelName] = Number.parseFloat(String(values.ratio))
                }
              } else if (pricingMode === 'per-token') {
                if (chargeScope === 'input_only') {
                  billingModeMap[finalModelName] = 'input_only'
                } else if (chargeScope === 'output_only') {
                  billingModeMap[finalModelName] = 'output_only'
                }
                if (isSet(values.ratio)) {
                  ratioMap[finalModelName] = Number.parseFloat(String(values.ratio))
                }
                if (isSet(values.cacheRatio)) {
                  cacheMap[finalModelName] = Number.parseFloat(
                    String(values.cacheRatio)
                  )
                }
                if (isSet(values.completionRatio)) {
                  completionMap[finalModelName] = Number.parseFloat(
                    String(values.completionRatio)
                  )
                }
                if (isSet(values.imageRatio)) {
                  imageMap[finalModelName] = Number.parseFloat(
                    String(values.imageRatio)
                  )
                }
                if (isSet(values.audioRatio)) {
                  audioMap[finalModelName] = Number.parseFloat(
                    String(values.audioRatio)
                  )
                }
                if (isSet(values.audioCompletionRatio)) {
                  audioCompletionMap[finalModelName] = Number.parseFloat(
                    String(values.audioCompletionRatio)
                  )
                }
              }
            }

            // Update system options if there are changes
            const updates: Array<{ key: string; value: string }> = []

            const newBillingMode = normalizeJsonString(
              JSON.stringify(billingModeMap)
            )
            if (
              newBillingMode !==
              normalizeJsonString(modelSettings['billing_setting.billing_mode'])
            ) {
              updates.push({
                key: 'billing_setting.billing_mode',
                value: newBillingMode,
              })
            }

            const newModelPrice = normalizeJsonString(JSON.stringify(priceMap))
            if (
              newModelPrice !== normalizeJsonString(modelSettings.ModelPrice)
            ) {
              updates.push({ key: 'ModelPrice', value: newModelPrice })
            }

            const newModelRatio = normalizeJsonString(JSON.stringify(ratioMap))
            if (
              newModelRatio !== normalizeJsonString(modelSettings.ModelRatio)
            ) {
              updates.push({ key: 'ModelRatio', value: newModelRatio })
            }

            const newCacheRatio = normalizeJsonString(JSON.stringify(cacheMap))
            if (
              newCacheRatio !== normalizeJsonString(modelSettings.CacheRatio)
            ) {
              updates.push({ key: 'CacheRatio', value: newCacheRatio })
            }

            const newCompletionRatio = normalizeJsonString(
              JSON.stringify(completionMap)
            )
            if (
              newCompletionRatio !==
              normalizeJsonString(modelSettings.CompletionRatio)
            ) {
              updates.push({
                key: 'CompletionRatio',
                value: newCompletionRatio,
              })
            }

            const newImageRatio = normalizeJsonString(JSON.stringify(imageMap))
            if (
              newImageRatio !== normalizeJsonString(modelSettings.ImageRatio)
            ) {
              updates.push({ key: 'ImageRatio', value: newImageRatio })
            }

            const newAudioRatio = normalizeJsonString(JSON.stringify(audioMap))
            if (
              newAudioRatio !== normalizeJsonString(modelSettings.AudioRatio)
            ) {
              updates.push({ key: 'AudioRatio', value: newAudioRatio })
            }

            const newAudioCompletionRatio = normalizeJsonString(
              JSON.stringify(audioCompletionMap)
            )
            if (
              newAudioCompletionRatio !==
              normalizeJsonString(modelSettings.AudioCompletionRatio)
            ) {
              updates.push({
                key: 'AudioCompletionRatio',
                value: newAudioCompletionRatio,
              })
            }

            // Apply all updates (including deletions when clearing fields)
            for (const update of updates) {
              await updateOption.mutateAsync(update)
            }
          }

          toast.success(
            isEditing
              ? 'Model updated successfully'
              : 'Model created successfully'
          )
          queryClient.invalidateQueries({ queryKey: modelsQueryKeys.lists() })
          queryClient.invalidateQueries({ queryKey: ['system-options'] })
          onOpenChange(false)
        } else {
          toast.error(response.message || 'Operation failed')
        }
      } catch (error: unknown) {
        toast.error((error as Error)?.message || 'Operation failed')
      } finally {
        setIsSubmitting(false)
      }
    },
    [
      isEditing,
      currentModelId,
      queryClient,
      onOpenChange,
      pricingMode,
      oldModelName,
      loadedPricingName,
      modelSettings,
      updateOption,
    ]
  )

  const handleFillEndpointTemplate = (templateKey: string) => {
    const template = ENDPOINT_TEMPLATES[templateKey]
    if (template) {
      const templateJson = JSON.stringify({ [templateKey]: template }, null, 2)
      form.setValue('endpoints', templateJson)
    }
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className={sideDrawerContentClassName('sm:max-w-2xl')}>
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>
            {isEditing ? t('Edit Model') : t('Create Model')}
          </SheetTitle>
          <SheetDescription>
            {isEditing
              ? t("Update model configuration and click save when you're done.")
              : t(
                  'Add a new model to the system by providing the necessary information.'
                )}
          </SheetDescription>
        </SheetHeader>

        <Form {...form}>
          <form
            id='model-form'
            onSubmit={form.handleSubmit(
              onSubmit as Parameters<typeof form.handleSubmit>[0]
            )}
            className={sideDrawerFormClassName()}
          >
            {/* Basic Information */}
            <SideDrawerSection>
              <h3 className='text-sm font-semibold'>
                {t('Basic Information')}
              </h3>

              {/* Multi-language Selector */}
              <div className='space-y-2 rounded-xl border border-primary/20 bg-muted/10 p-3'>
                <div className='flex items-center justify-between'>
                  <Label className='text-xs font-semibold text-foreground flex items-center gap-1.5'>
                    <span>🌐</span>
                    <span>{t('Language-Specific Model Information (8 Languages)')}</span>
                  </Label>
                  <span className='text-[11px] font-medium text-primary'>
                    {MODEL_LANGUAGES.find(l => l.code === activeModelLang)?.flag} {MODEL_LANGUAGES.find(l => l.code === activeModelLang)?.label}
                  </span>
                </div>
                <div className='flex flex-wrap gap-1'>
                  {MODEL_LANGUAGES.map((l) => (
                    <Button
                      key={l.code}
                      type='button'
                      variant={activeModelLang === l.code ? 'default' : 'outline'}
                      size='sm'
                      className='h-7 px-2 text-xs gap-1'
                      onClick={() => setActiveModelLang(l.code)}
                    >
                      <span>{l.flag}</span>
                      <span>{l.label}</span>
                    </Button>
                  ))}
                </div>
              </div>

              {/* Display Name for active language */}
              <div className='space-y-2'>
                <Label className='text-sm font-medium'>
                  {t('Display Name')} ({MODEL_LANGUAGES.find(l => l.code === activeModelLang)?.label})
                </Label>
                <Input
                  placeholder={t('Claude Opus 5, GPT-4o, etc.')}
                  value={localizedData.display_names[activeModelLang] ?? (activeModelLang === 'tr' ? form.watch('display_name') : '')}
                  onChange={(e) => {
                    const val = e.target.value
                    setLocalizedData(prev => ({
                      ...prev,
                      display_names: { ...prev.display_names, [activeModelLang]: val }
                    }))
                    if (activeModelLang === 'tr') {
                      form.setValue('display_name', val)
                    }
                  }}
                />
                <p className='text-xs text-muted-foreground'>
                  {t('Optional friendly name shown as title on model cards')}
                </p>
              </div>

              <FormField
                control={form.control}
                name='model_name'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Model ID *')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t('gpt-4, claude-3-opus, etc.')}
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('The unique API identifier for this model')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              {/* Description for active language */}
              <div className='space-y-2'>
                <Label className='text-sm font-medium'>
                  {t('Description')} ({MODEL_LANGUAGES.find(l => l.code === activeModelLang)?.label})
                </Label>
                <Textarea
                  placeholder={t('Describe this model...')}
                  rows={3}
                  value={localizedData.descriptions[activeModelLang] ?? (activeModelLang === 'tr' ? form.watch('description') : '')}
                  onChange={(e) => {
                    const val = e.target.value
                    setLocalizedData(prev => ({
                      ...prev,
                      descriptions: { ...prev.descriptions, [activeModelLang]: val }
                    }))
                    if (activeModelLang === 'tr') {
                      form.setValue('description', val)
                    }
                  }}
                />
              </div>

              {/* Tagline for active language */}
              <div className='space-y-2'>
                <Label className='text-sm font-medium'>
                  {t('Tagline / Extra Text')} ({MODEL_LANGUAGES.find(l => l.code === activeModelLang)?.label})
                </Label>
                <Input
                  placeholder={t('Flagship model • Reasoning, Under maintenance, etc.')}
                  value={localizedData.taglines[activeModelLang] ?? (activeModelLang === 'tr' ? form.watch('tagline') : '')}
                  onChange={(e) => {
                    const val = e.target.value
                    setLocalizedData(prev => ({
                      ...prev,
                      taglines: { ...prev.taglines, [activeModelLang]: val }
                    }))
                    if (activeModelLang === 'tr') {
                      form.setValue('tagline', val)
                    }
                  }}
                />
                <p className='text-xs text-muted-foreground'>
                  {t('Optional text shown on the model card below the description')}
                </p>
              </div>

              {/* Property Boxes for active language */}
              {(() => {
                const curBoxes = localizedData.property_boxes[activeModelLang] ?? (activeModelLang === 'tr' ? propertyBoxes : [])
                return (
                  <div className='space-y-3 rounded-lg border p-3 bg-muted/20'>
                    <div className='flex items-center justify-between'>
                      <div>
                        <h4 className='text-xs font-semibold'>
                          {t('Property Boxes (Max 8)')} ({MODEL_LANGUAGES.find(l => l.code === activeModelLang)?.label})
                        </h4>
                        <p className='text-[11px] text-muted-foreground'>{t('Custom rounded boxes (e.g. Context Window: 200K)')}</p>
                      </div>
                      {curBoxes.length < 8 && (
                        <Button
                          type='button'
                          variant='outline'
                          size='sm'
                          className='h-7 text-xs'
                          onClick={() => {
                            const updated = [...curBoxes, { label: '', value: '' }]
                            setLocalizedData(prev => ({
                              ...prev,
                              property_boxes: { ...prev.property_boxes, [activeModelLang]: updated }
                            }))
                            if (activeModelLang === 'tr') {
                              setPropertyBoxes(updated)
                              form.setValue('property_boxes', JSON.stringify(updated))
                            }
                          }}
                        >
                          + {t('Add Box')}
                        </Button>
                      )}
                    </div>
                    {curBoxes.map((box, idx) => (
                      <div key={idx} className='flex items-center gap-2'>
                        <Input
                          placeholder={t('Label (e.g. Context Window)')}
                          value={box.label}
                          className='h-8 text-xs flex-1'
                          onChange={(e) => {
                            const updated = [...curBoxes]
                            updated[idx].label = e.target.value
                            setLocalizedData(prev => ({
                              ...prev,
                              property_boxes: { ...prev.property_boxes, [activeModelLang]: updated }
                            }))
                            if (activeModelLang === 'tr') {
                              setPropertyBoxes(updated)
                              form.setValue('property_boxes', JSON.stringify(updated.filter(b => b.label.trim() || b.value.trim())))
                            }
                          }}
                        />
                        <Input
                          placeholder={t('Value (e.g. 200K)')}
                          value={box.value}
                          className='h-8 text-xs flex-1'
                          onChange={(e) => {
                            const updated = [...curBoxes]
                            updated[idx].value = e.target.value
                            setLocalizedData(prev => ({
                              ...prev,
                              property_boxes: { ...prev.property_boxes, [activeModelLang]: updated }
                            }))
                            if (activeModelLang === 'tr') {
                              setPropertyBoxes(updated)
                              form.setValue('property_boxes', JSON.stringify(updated.filter(b => b.label.trim() || b.value.trim())))
                            }
                          }}
                        />
                        <Button
                          type='button'
                          variant='ghost'
                          size='icon'
                          className='size-8 text-destructive hover:bg-destructive/10'
                          onClick={() => {
                            const updated = curBoxes.filter((_, i) => i !== idx)
                            setLocalizedData(prev => ({
                              ...prev,
                              property_boxes: { ...prev.property_boxes, [activeModelLang]: updated }
                            }))
                            if (activeModelLang === 'tr') {
                              setPropertyBoxes(updated)
                              form.setValue('property_boxes', JSON.stringify(updated))
                            }
                          }}
                        >
                          <Trash2 className='size-3.5' />
                        </Button>
                      </div>
                    ))}
                  </div>
                )
              })()}

              <FormField
                control={form.control}
                name='icon'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Icon')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t('OpenAI, Anthropic, etc.')}
                        {...field}
                      />
                    </FormControl>
                    <FormDescription className='text-xs'>
                      {t('@lobehub/icons key')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='vendor_id'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Vendor')}</FormLabel>
                    <Select
                      items={vendors.map((vendor) => ({
                        value: String(vendor.id),
                        label: vendor.name,
                      }))}
                      onValueChange={(value) =>
                        field.onChange(
                          value ? Number.parseInt(value) : undefined
                        )
                      }
                      value={field.value ? String(field.value) : undefined}
                    >
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue placeholder={t('Select vendor')} />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent alignItemWithTrigger={false}>
                        <SelectGroup>
                          {vendors.map((vendor) => (
                            <SelectItem
                              key={vendor.id}
                              value={String(vendor.id)}
                            >
                              {vendor.name}
                            </SelectItem>
                          ))}
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='tags'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Tags')}</FormLabel>
                    <FormControl>
                      <TagInput
                        value={field.value || []}
                        onChange={field.onChange}
                        placeholder={t('Add tags...')}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Press Enter or comma to add tags')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SideDrawerSection>

            {/* Matching Configuration */}
            <SideDrawerSection>
              <h3 className='text-sm font-semibold'>{t('Matching Rules')}</h3>

              <FormField
                control={form.control}
                name='name_rule'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Name Rule')}</FormLabel>
                    <FormControl>
                      <RadioGroup
                        onValueChange={(value) =>
                          field.onChange(Number.parseInt(value))
                        }
                        value={String(field.value)}
                        className='grid grid-cols-2 gap-4'
                      >
                        {getNameRuleOptions(t).map((option) => (
                          <div
                            key={option.value}
                            className='flex items-center space-x-2'
                          >
                            <RadioGroupItem
                              value={String(option.value)}
                              id={`rule-${option.value}`}
                            />
                            <Label
                              htmlFor={`rule-${option.value}`}
                              className='cursor-pointer font-normal'
                            >
                              {option.label}
                            </Label>
                          </div>
                        ))}
                      </RadioGroup>
                    </FormControl>
                    <FormDescription>
                      {t('How this model name should match requests')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SideDrawerSection>

            {/* Endpoints Configuration */}
            <SideDrawerSection>
              <div className='flex items-center justify-between'>
                <h3 className='text-sm font-semibold'>{t('Endpoints')}</h3>
                <Select<string>
                  items={Object.keys(ENDPOINT_TEMPLATES).map((key) => ({
                    value: key,
                    label: key,
                  }))}
                  onValueChange={(v) =>
                    v !== null && handleFillEndpointTemplate(v)
                  }
                >
                  <SelectTrigger size='sm' className='w-[200px]'>
                    <SelectValue placeholder={t('Load template...')} />
                  </SelectTrigger>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      {Object.keys(ENDPOINT_TEMPLATES).map((key) => (
                        <SelectItem key={key} value={key}>
                          {key}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </div>

              <FormField
                control={form.control}
                name='endpoints'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Endpoint Configuration')}</FormLabel>
                    <FormControl>
                      <JsonEditor
                        value={field.value || ''}
                        onChange={field.onChange}
                        keyPlaceholder='endpoint_type'
                        valuePlaceholder='{"path": "/v1/...", "method": "POST"}'
                        keyLabel='Endpoint Type'
                        valueLabel='Configuration'
                        valueType='any'
                        emptyMessage={t(
                          'No endpoints configured. Switch to JSON mode or add rows to define endpoints.'
                        )}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Define API endpoints for this model (JSON format)')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SideDrawerSection>

            {/* Pricing Configuration */}
            {isAutoModel && (
              <SideDrawerSection>
                <h3 className='text-sm font-semibold'>
                  {t('Pricing Configuration')}
                </h3>
                <div className='rounded-md border border-amber-500/40 bg-amber-500/5 px-4 py-3 text-sm text-muted-foreground'>
                  {t(
                    'The auto model has a dynamic price: every request is billed as the model the auto model picks for it, so it carries no price of its own. Configure pinning, the description and the icon here; configure routing in the auto model settings.'
                  )}
                </div>
              </SideDrawerSection>
            )}
            {!isAutoModel && (
            <SideDrawerSection>
              <h3 className='text-sm font-semibold'>
                {t('Pricing Configuration')}
              </h3>

              <div className='space-y-4'>
                <Label>{t('Pricing mode')}</Label>
                <RadioGroup
                  value={pricingMode}
                  onValueChange={(value) =>
                    setPricingMode(value as PricingMode)
                  }
                  className='grid grid-cols-1 sm:grid-cols-2 gap-2'
                >
                  <div className='flex items-center space-x-2'>
                    <RadioGroupItem value='per-token' id='per-token' />
                    <Label htmlFor='per-token' className='font-normal cursor-pointer'>
                      {t('Per-token')}
                    </Label>
                  </div>
                  <div className='flex items-center space-x-2'>
                    <RadioGroupItem value='per-request' id='per-request' />
                    <Label htmlFor='per-request' className='font-normal cursor-pointer'>
                      {t('Per-request')}
                    </Label>
                  </div>
                  <div className='flex items-center space-x-2'>
                    <RadioGroupItem value='duration_second' id='duration_second' />
                    <Label htmlFor='duration_second' className='font-normal cursor-pointer'>
                      {t('Duration-based')}
                    </Label>
                  </div>
                  <div className='flex items-center space-x-2'>
                    <RadioGroupItem value='characters' id='characters' />
                    <Label htmlFor='characters' className='font-normal cursor-pointer'>
                      {t('Character-based')}
                    </Label>
                  </div>
                </RadioGroup>
              </div>

              {pricingMode === 'duration_second' && (
                <div className='space-y-2 rounded-md border p-4 bg-muted/20'>
                  <Label>{t('Price per Second (USD/s)')}</Label>
                  <Input
                    type='text'
                    placeholder='0.005'
                    value={promptPrice}
                    onChange={(e) => {
                      const val = e.target.value
                      setPromptPrice(val)
                      if (val && !Number.isNaN(Number.parseFloat(val))) {
                        const ratio = Number.parseFloat(val) / 2
                        form.setValue('ratio', ratio.toString())
                      } else {
                        form.setValue('ratio', '')
                      }
                    }}
                  />
                  <p className='text-muted-foreground text-xs'>
                    {promptPrice && !Number.isNaN(Number.parseFloat(promptPrice))
                      ? `Calculated ratio: ${(Number.parseFloat(promptPrice) / 2).toFixed(6)}`
                      : t('Cost in USD per second for audio/video duration. Tokens are not charged.')}
                  </p>
                </div>
              )}

              {pricingMode === 'characters' && (
                <div className='space-y-2 rounded-md border p-4 bg-muted/20'>
                  <Label>{t('Price per 1M Characters (USD/1M chars)')}</Label>
                  <Input
                    type='text'
                    placeholder='15.0'
                    value={promptPrice}
                    onChange={(e) => {
                      const val = e.target.value
                      setPromptPrice(val)
                      if (val && !Number.isNaN(Number.parseFloat(val))) {
                        const ratio = Number.parseFloat(val) / 2
                        form.setValue('ratio', ratio.toString())
                      } else {
                        form.setValue('ratio', '')
                      }
                    }}
                  />
                  <p className='text-muted-foreground text-xs'>
                    {promptPrice && !Number.isNaN(Number.parseFloat(promptPrice))
                      ? `Calculated ratio: ${(Number.parseFloat(promptPrice) / 2).toFixed(6)}`
                      : t('Cost in USD per 1M text characters for TTS synthesis.')}
                  </p>
                </div>
              )}

              {pricingMode === 'per-request' && (
                <FormField
                  control={form.control}
                  name='price'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Fixed price (USD)')}</FormLabel>
                      <FormControl>
                        <Input
                          type='text'
                          placeholder='0.01'
                          {...field}
                          onChange={(e) => {
                            const value = e.target.value
                            if (validateNumber(value)) {
                              field.onChange(value)
                            }
                          }}
                        />
                      </FormControl>
                      <FormDescription>
                        {t(
                          'Cost in USD per request, regardless of tokens used.'
                        )}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              )}

              {pricingMode === 'per-token' && (
                <>
                  <div className='space-y-2 rounded-md border p-3 bg-muted/30'>
                    <Label className='text-xs font-medium'>{t('Billing Scope')}</Label>
                    <RadioGroup
                      value={chargeScope}
                      onValueChange={(value) => setChargeScope(value as ChargeScope)}
                      className='grid grid-cols-3 gap-2'
                    >
                      <div className='flex items-center space-x-2'>
                        <RadioGroupItem value='all' id='scope-all' />
                        <Label htmlFor='scope-all' className='font-normal text-xs cursor-pointer'>
                          {t('Input & Output')}
                        </Label>
                      </div>
                      <div className='flex items-center space-x-2'>
                        <RadioGroupItem value='input_only' id='scope-input-only' />
                        <Label htmlFor='scope-input-only' className='font-normal text-xs cursor-pointer'>
                          {t('Input Only')}
                        </Label>
                      </div>
                      <div className='flex items-center space-x-2'>
                        <RadioGroupItem value='output_only' id='scope-output-only' />
                        <Label htmlFor='scope-output-only' className='font-normal text-xs cursor-pointer'>
                          {t('Output Only')}
                        </Label>
                      </div>
                    </RadioGroup>
                    <p className='text-muted-foreground text-[11px]'>
                      {chargeScope === 'input_only'
                        ? t('Only input tokens are billed. Output tokens are completely free.')
                        : chargeScope === 'output_only'
                        ? t('Only output tokens are billed. Input tokens are completely free.')
                        : t('Standard token billing: both prompt and completion tokens are billed.')}
                    </p>
                  </div>

                  <div className='space-y-4'>
                    <Label>{t('Input mode')}</Label>
                    <RadioGroup
                      value={pricingSubMode}
                      onValueChange={(value) =>
                        setPricingSubMode(value as PricingSubMode)
                      }
                    >
                      <div className='flex items-center space-x-2'>
                        <RadioGroupItem value='ratio' id='ratio' />
                        <Label htmlFor='ratio' className='font-normal'>
                          {t('Ratio mode')}
                        </Label>
                      </div>
                      <div className='flex items-center space-x-2'>
                        <RadioGroupItem value='price' id='price' />
                        <Label htmlFor='price' className='font-normal'>
                          {t('Price mode (USD per 1M tokens)')}
                        </Label>
                      </div>
                    </RadioGroup>
                  </div>

                  {pricingSubMode === 'ratio' ? (
                    <>
                      {chargeScope !== 'output_only' && (
                        <FormField
                          control={form.control}
                          name='ratio'
                          render={({ field }) => (
                            <FormItem>
                              <FormLabel>{t('Model ratio (Prompt / Input)')}</FormLabel>
                              <FormControl>
                                <Input
                                  type='text'
                                  placeholder='1.0'
                                  {...field}
                                  onChange={(e) => {
                                    const value = e.target.value
                                    if (validateNumber(value)) {
                                      field.onChange(value)
                                      if (value) {
                                        setPromptPrice(
                                          (
                                            Number.parseFloat(value) * 2
                                          ).toString()
                                        )
                                      } else {
                                        setPromptPrice('')
                                      }
                                    }
                                  }}
                                />
                              </FormControl>
                              <FormDescription>
                                {field.value &&
                                !Number.isNaN(Number.parseFloat(field.value))
                                  ? `Calculated price: $${(Number.parseFloat(field.value) * 2).toFixed(4)} per 1M tokens`
                                  : t('Multiplier for prompt tokens.')}
                              </FormDescription>
                              <FormMessage />
                            </FormItem>
                          )}
                        />
                      )}

                      {chargeScope !== 'input_only' && (
                        <FormField
                          control={form.control}
                          name='completionRatio'
                          render={({ field }) => (
                            <FormItem>
                              <FormLabel>{t('Completion ratio (Output)')}</FormLabel>
                              <FormControl>
                                <Input
                                  type='text'
                                  placeholder='1.0'
                                  {...field}
                                  onChange={(e) => {
                                    const value = e.target.value
                                    if (validateNumber(value)) {
                                      field.onChange(value)
                                      const ratio = form.getValues('ratio')
                                      if (value && ratio) {
                                        const compPrice =
                                          Number.parseFloat(ratio) *
                                          2 *
                                          Number.parseFloat(value)
                                        setCompletionPrice(compPrice.toString())
                                      } else {
                                        setCompletionPrice('')
                                      }
                                    }
                                  }}
                                />
                              </FormControl>
                              <FormDescription>
                                {field.value &&
                                !Number.isNaN(Number.parseFloat(field.value)) &&
                                promptPrice &&
                                !Number.isNaN(Number.parseFloat(promptPrice))
                                  ? `Calculated price: $${(Number.parseFloat(promptPrice) * Number.parseFloat(field.value)).toFixed(4)} per 1M tokens`
                                  : t('Multiplier for completion tokens.')}
                              </FormDescription>
                              <FormMessage />
                            </FormItem>
                          )}
                        />
                      )}
                    </>
                  ) : (
                    <div className='space-y-4'>
                      {chargeScope !== 'output_only' && (
                        <div className='space-y-2'>
                          <Label>{t('Prompt price ($/1M tokens)')}</Label>
                          <Input
                            type='text'
                            placeholder='2.0'
                            value={promptPrice}
                            onChange={(e) =>
                              handlePromptPriceChange(e.target.value)
                            }
                          />
                          <p className='text-muted-foreground text-sm'>
                            {promptPrice &&
                            !Number.isNaN(Number.parseFloat(promptPrice))
                              ? `Calculated ratio: ${(Number.parseFloat(promptPrice) / 2).toFixed(4)}`
                              : t('Enter Input price to calculate ratio')}
                          </p>
                        </div>
                      )}

                      {chargeScope !== 'input_only' && (
                        <div className='space-y-2'>
                          <Label>{t('Completion price ($/1M tokens)')}</Label>
                          <Input
                            type='text'
                            placeholder='4.0'
                            value={completionPrice}
                            onChange={(e) =>
                              handleCompletionPriceChange(e.target.value)
                            }
                          />
                          <p className='text-muted-foreground text-sm'>
                            {completionPrice &&
                            !Number.isNaN(Number.parseFloat(completionPrice)) &&
                            promptPrice &&
                            !Number.isNaN(Number.parseFloat(promptPrice)) &&
                            Number.parseFloat(promptPrice) > 0
                              ? `Calculated ratio: ${(Number.parseFloat(completionPrice) / Number.parseFloat(promptPrice)).toFixed(4)}`
                              : t('Enter Completion price to calculate ratio')}
                          </p>
                        </div>
                      )}
                    </div>
                  )}

                  <Collapsible
                    open={advancedOpen}
                    onOpenChange={setAdvancedOpen}
                  >
                    <CollapsibleTrigger
                      render={
                        <Button
                          type='button'
                          variant='outline'
                          className='flex w-full items-center justify-between'
                        />
                      }
                    >
                      {t('Advanced options')}
                      <ChevronDown
                        className={`h-4 w-4 transition-transform duration-200 ${
                          advancedOpen ? 'rotate-180' : ''
                        }`}
                      />
                    </CollapsibleTrigger>
                    <CollapsibleContent className='flex flex-col gap-4 pt-4'>
                      <FormField
                        control={form.control}
                        name='cacheRatio'
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('Cache ratio')}</FormLabel>
                            <FormControl>
                              <Input
                                type='text'
                                placeholder='0.1'
                                {...field}
                                onChange={(e) => {
                                  const value = e.target.value
                                  if (validateNumber(value)) {
                                    field.onChange(value)
                                  }
                                }}
                              />
                            </FormControl>
                            <FormDescription>
                              {t('Discount ratio for cache hits.')}
                            </FormDescription>
                            <FormMessage />
                          </FormItem>
                        )}
                      />

                      <FormField
                        control={form.control}
                        name='imageRatio'
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('Image ratio')}</FormLabel>
                            <FormControl>
                              <Input
                                type='text'
                                placeholder='1.0'
                                {...field}
                                onChange={(e) => {
                                  const value = e.target.value
                                  if (validateNumber(value)) {
                                    field.onChange(value)
                                  }
                                }}
                              />
                            </FormControl>
                            <FormDescription>
                              {t('Multiplier for image processing.')}
                            </FormDescription>
                            <FormMessage />
                          </FormItem>
                        )}
                      />

                      <FormField
                        control={form.control}
                        name='audioRatio'
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('Audio ratio')}</FormLabel>
                            <FormControl>
                              <Input
                                type='text'
                                placeholder='1.0'
                                {...field}
                                onChange={(e) => {
                                  const value = e.target.value
                                  if (validateNumber(value)) {
                                    field.onChange(value)
                                  }
                                }}
                              />
                            </FormControl>
                            <FormDescription>
                              {t('Multiplier for audio inputs.')}
                            </FormDescription>
                            <FormMessage />
                          </FormItem>
                        )}
                      />

                      <FormField
                        control={form.control}
                        name='audioCompletionRatio'
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('Audio completion ratio')}</FormLabel>
                            <FormControl>
                              <Input
                                type='text'
                                placeholder='1.0'
                                {...field}
                                onChange={(e) => {
                                  const value = e.target.value
                                  if (validateNumber(value)) {
                                    field.onChange(value)
                                  }
                                }}
                              />
                            </FormControl>
                            <FormDescription>
                              {t('Multiplier for audio outputs.')}
                            </FormDescription>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                    </CollapsibleContent>
                  </Collapsible>
                </>
              )}
            </SideDrawerSection>
            )}

            {/* Status & Sync */}
            <SideDrawerSection>
              <h3 className='text-sm font-semibold'>{t('Status & Sync')}</h3>

              <FormField
                control={form.control}
                name='status'
                render={({ field }) => (
                  <FormItem className={sideDrawerSwitchItemClassName()}>
                    <div className='flex flex-col gap-0.5'>
                      <FormLabel className='text-base'>
                        {t('Enabled')}
                      </FormLabel>
                      <FormDescription>
                        {t('Enable or disable this model')}
                      </FormDescription>
                    </div>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='sync_official'
                render={({ field }) => (
                  <FormItem className={sideDrawerSwitchItemClassName()}>
                    <div className='flex flex-col gap-0.5'>
                      <FormLabel className='text-base'>
                        {t('Official Sync')}
                      </FormLabel>
                      <FormDescription>
                        {t('Sync this model with official upstream')}
                      </FormDescription>
                    </div>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                  </FormItem>
                )}
              />

              <div className='pt-2 border-t mt-4'>
                <FormField
                  control={form.control}
                  name='is_pinned'
                  render={({ field }) => (
                    <FormItem className={sideDrawerSwitchItemClassName()}>
                      <div className='flex flex-col gap-0.5'>
                        <FormLabel className='text-base'>
                          {t('Pin in Model Square')}
                        </FormLabel>
                        <FormDescription>
                          {t(
                            'Pinned models are always displayed at the very top of Model Square regardless of sorting'
                          )}
                        </FormDescription>
                      </div>
                      <FormControl>
                        <Switch
                          checked={field.value}
                          onCheckedChange={field.onChange}
                        />
                      </FormControl>
                    </FormItem>
                  )}
                />

                {form.watch('is_pinned') && (
                  <div className='grid grid-cols-1 gap-3 sm:grid-cols-2 mt-3'>
                    <FormField
                      control={form.control}
                      name='pin_order'
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>{t('Pin Order (1, 2, 3...)')}</FormLabel>
                          <FormControl>
                            <Input
                              type='number'
                              min={0}
                              step={1}
                              placeholder='0'
                              value={field.value ?? 0}
                              onChange={(e) => field.onChange(Number(e.target.value) || 0)}
                            />
                          </FormControl>
                          <FormDescription>
                            {t('Lower numbers appear first among pinned models')}
                          </FormDescription>
                        </FormItem>
                      )}
                    />

                    <FormField
                      control={form.control}
                      name='pin_color'
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>{t('Pinned Card Highlight Color')}</FormLabel>
                          {/* A grid of swatches instead of a select: the long
                              option labels used to overflow the narrow trigger
                              of the drawer, a grid always fits. */}
                          <div className='grid grid-cols-2 gap-1.5'>
                            {PIN_COLOR_OPTIONS.map((option) => {
                              const selected =
                                (field.value || 'amber') === option.value
                              return (
                                <button
                                  key={option.value}
                                  type='button'
                                  onClick={() => field.onChange(option.value)}
                                  aria-pressed={selected}
                                  className={cn(
                                    'flex min-w-0 items-center gap-1.5 rounded-md border px-2 py-1.5 text-left text-xs transition-colors',
                                    selected
                                      ? 'border-primary/60 bg-primary/5 font-medium'
                                      : 'border-border hover:bg-muted/60',
                                    option.value === 'none' && 'col-span-2'
                                  )}
                                >
                                  <span
                                    className={cn(
                                      'size-3 shrink-0 rounded-full border border-black/10 dark:border-white/20',
                                      option.style.dot
                                    )}
                                  />
                                  <span className='min-w-0 truncate'>
                                    {t(option.style.label)}
                                  </span>
                                  {selected && (
                                    <Check className='ms-auto size-3.5 shrink-0 text-primary' />
                                  )}
                                </button>
                              )
                            })}
                          </div>
                          <FormDescription>
                            {t('Choose how the card background changes when pinned')}
                          </FormDescription>
                        </FormItem>
                      )}
                    />
                  </div>
                )}

                <FormField
                  control={form.control}
                  name='announcement_bar'
                  render={({ field }) => {
                    let parsedBar: {
                      text: string
                      translations: Record<string, string>
                      bg_color: string
                      text_color: string
                    } = {
                      text: '',
                      translations: {},
                      bg_color: '#8b5cf6',
                      text_color: '#ffffff',
                    }

                    if (field.value) {
                      try {
                        const p = JSON.parse(field.value)
                        if (typeof p === 'object' && p !== null) {
                          parsedBar = {
                            text: p.text || '',
                            translations: p.translations || {},
                            bg_color: p.bg_color || '#8b5cf6',
                            text_color: p.text_color || '#ffffff',
                          }
                        }
                      } catch {
                        parsedBar.text = field.value
                      }
                    }

                    const currentLangText =
                      parsedBar.translations?.[activeModelLang] ??
                      (activeModelLang === 'tr' ? parsedBar.text : '') ??
                      ''

                    const updateBarText = (newVal: string) => {
                      const nextTrans = { ...parsedBar.translations, [activeModelLang]: newVal }
                      const next = {
                        ...parsedBar,
                        translations: nextTrans,
                        text: activeModelLang === 'tr' || !parsedBar.text ? newVal : parsedBar.text,
                      }
                      const hasAnyText =
                        Boolean(next.text.trim()) ||
                        Object.values(next.translations).some((v) => Boolean(v?.trim()))
                      if (!hasAnyText) {
                        field.onChange('')
                      } else {
                        field.onChange(JSON.stringify(next))
                      }
                    }

                    const updateColor = (k: 'bg_color' | 'text_color', v: string) => {
                      const next = { ...parsedBar, [k]: v }
                      field.onChange(JSON.stringify(next))
                    }

                    return (
                      <div className='space-y-3 pt-3 border-t mt-4'>
                        <div className='flex items-center justify-between'>
                          <FormLabel>{t('Model Announcement Bar (Top Strip)')}</FormLabel>
                          <span className='text-[11px] text-muted-foreground'>
                            {t('Supports all 8 languages')}
                          </span>
                        </div>

                        {/* Language Tabs for Model Announcement */}
                        <div className='flex flex-wrap gap-1'>
                          {MODEL_LANGUAGES.map((lang) => {
                            const hasVal = Boolean(
                              parsedBar.translations?.[lang.code]?.trim() ||
                                (lang.code === 'tr' && parsedBar.text?.trim())
                            )
                            return (
                              <Button
                                key={lang.code}
                                type='button'
                                size='sm'
                                variant={activeModelLang === lang.code ? 'default' : 'outline'}
                                className='h-6 text-[11px] px-2 gap-1'
                                onClick={() => setActiveModelLang(lang.code)}
                              >
                                <span>{lang.flag}</span>
                                <span>{lang.label}</span>
                                {hasVal && (
                                  <span className='size-1.5 rounded-full bg-emerald-500 ms-0.5' />
                                )}
                              </Button>
                            )
                          })}
                        </div>

                        <Input
                          placeholder={t(
                            'Announcement message for {{lang}} (e.g. ⚡ 1M Context Window)...',
                            {
                              lang:
                                MODEL_LANGUAGES.find((l) => l.code === activeModelLang)
                                  ?.label || activeModelLang,
                            }
                          )}
                          value={currentLangText}
                          onChange={(e) => updateBarText(e.target.value)}
                        />

                        {(currentLangText.trim() || parsedBar.text.trim()) && (
                          <div className='grid grid-cols-2 gap-3 pt-1'>
                            <div className='space-y-1'>
                              <Label className='text-xs'>{t('Background Color')}</Label>
                              <div className='flex items-center gap-2'>
                                <input
                                  type='color'
                                  value={parsedBar.bg_color}
                                  onChange={(e) => updateColor('bg_color', e.target.value)}
                                  className='h-8 w-10 cursor-pointer rounded border p-0.5 bg-background'
                                />
                                <Input
                                  className='text-xs font-mono'
                                  value={parsedBar.bg_color}
                                  onChange={(e) => updateColor('bg_color', e.target.value)}
                                />
                              </div>
                            </div>
                            <div className='space-y-1'>
                              <Label className='text-xs'>{t('Text Color')}</Label>
                              <div className='flex items-center gap-2'>
                                <input
                                  type='color'
                                  value={parsedBar.text_color}
                                  onChange={(e) => updateColor('text_color', e.target.value)}
                                  className='h-8 w-10 cursor-pointer rounded border p-0.5 bg-background'
                                />
                                <Input
                                  className='text-xs font-mono'
                                  value={parsedBar.text_color}
                                  onChange={(e) => updateColor('text_color', e.target.value)}
                                />
                              </div>
                            </div>
                          </div>
                        )}
                        <FormDescription>
                          {t('Displays an announcement banner with auto marquee scrolling when overflowing on this model card in Model Square')}
                        </FormDescription>
                      </div>
                    )
                  }}
                />
              </div>
            </SideDrawerSection>
          </form>
        </Form>

        <SheetFooter className={sideDrawerFooterClassName()}>
          <SheetClose
            render={<Button variant='outline' disabled={isSubmitting} />}
          >
            {t('Cancel')}
          </SheetClose>
          {canWrite && (
            <Button form='model-form' type='submit' disabled={isSubmitting}>
              {isSubmitting && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
              {isEditing ? t('Update Model') : t('Save changes')}
            </Button>
          )}
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
