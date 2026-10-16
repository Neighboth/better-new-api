import { useMemo, useState } from 'react'
import { useFormContext } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

import type { ChannelFormValues } from '../../../lib/channel-form'

function parseOverrideMap(raw: string | undefined): Record<string, number> {
  if (!raw || !raw.trim()) return {}
  try {
    const parsed: unknown = JSON.parse(raw)
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      const out: Record<string, number> = {}
      for (const [k, v] of Object.entries(parsed as Record<string, unknown>)) {
        if (typeof v === 'number' && v > 0) out[k] = v
      }
      return out
    }
  } catch {
    // Invalid JSON: start from an empty map rather than destroying input.
  }
  return {}
}

function parseModelList(models: unknown): string[] {
  if (Array.isArray(models)) {
    return models.map((m) => String(m).trim()).filter(Boolean)
  }
  if (typeof models === 'string') {
    return models
      .split(',')
      .map((m) => m.trim())
      .filter(Boolean)
  }
  return []
}

function percentToMultiplier(percent: number): number {
  const p = Math.min(100, Math.max(0, percent))
  return Math.round((1 - p / 100) * 10000) / 10000
}

export function DiscountQuickSet() {
  const { t } = useTranslation()
  const form = useFormContext<ChannelFormValues>()
  const overrideRaw = form.watch('model_ratio_override') ?? ''
  const modelsRaw = form.watch('models')
  const [percent, setPercent] = useState('20')
  const [checked, setChecked] = useState<string[]>([])

  const channelModels = useMemo(
    () => parseModelList(modelsRaw),
    [modelsRaw]
  )
  const entries = useMemo(
    () => parseOverrideMap(overrideRaw),
    [overrideRaw]
  )

  const toggle = (model: string, on: boolean) => {
    setChecked((prev) =>
      on ? [...new Set([...prev, model])] : prev.filter((m) => m !== model)
    )
  }

  const apply = () => {
    const p = Number(percent)
    if (!Number.isFinite(p) || p < 0 || p > 100 || checked.length === 0) return
    const next = { ...entries }
    if (p === 0) {
      for (const m of checked) delete next[m]
    } else {
      const mult = percentToMultiplier(p)
      for (const m of checked) next[m] = mult
    }
    form.setValue('model_ratio_override', JSON.stringify(next, null, 2), {
      shouldDirty: true,
      shouldValidate: true,
    })
    setChecked([])
  }

  const removeEntry = (model: string) => {
    const next = { ...entries }
    delete next[model]
    form.setValue('model_ratio_override', JSON.stringify(next, null, 2), {
      shouldDirty: true,
      shouldValidate: true,
    })
  }

  const entryModels = Object.keys(entries)

  return (
    <div className='space-y-3 rounded-lg border p-3'>
      <div className='space-y-1'>
        <div className='text-sm font-medium'>
          {t('Quick discount setter')}
        </div>
        <p className='text-muted-foreground text-xs'>
          {t(
            'Pick models, enter a percent, and apply. The percent is taken off the model base price, then the group ratio applies. Final = base x discount x group ratio.'
          )}
        </p>
      </div>

      {channelModels.length > 0 ? (
        <div className='grid max-h-40 gap-1 overflow-y-auto'>
          {channelModels.map((model) => (
            <label key={model} className='flex items-center gap-2 text-sm'>
              <Checkbox
                checked={checked.includes(model)}
                onCheckedChange={(v) => toggle(model, v === true)}
              />
              <span className='truncate font-mono text-xs'>{model}</span>
              {entries[model] !== undefined && (
                <span className='text-muted-foreground text-xs'>
                  -{Math.round((1 - entries[model]) * 100)}%
                </span>
              )}
            </label>
          ))}
        </div>
      ) : (
        <p className='text-muted-foreground text-xs'>
          {t('Add models to this channel first to pick them here.')}
        </p>
      )}

      <div className='flex flex-wrap items-end gap-2'>
        <div className='space-y-1'>
          <Label className='text-xs'>{t('Discount %')}</Label>
          <Input
            type='number'
            min={0}
            max={100}
            step={1}
            className='w-24'
            value={percent}
            onChange={(e) => setPercent(e.target.value)}
          />
        </div>
        <Button
          type='button'
          variant='outline'
          size='sm'
          disabled={checked.length === 0}
          onClick={() => setChecked(channelModels)}
        >
          {t('Select all')}
        </Button>
        <Button
          type='button'
          size='sm'
          disabled={checked.length === 0}
          onClick={apply}
        >
          {t('Apply to selected')}
        </Button>
      </div>

      {entryModels.length > 0 && (
        <div className='space-y-1'>
          <div className='text-xs font-medium'>{t('Active discounts')}</div>
          {entryModels.map((model) => (
            <div
              key={model}
              className='flex items-center justify-between gap-2 text-xs'
            >
              <span className='truncate font-mono'>
                {model} (-{Math.round((1 - entries[model]) * 100)}%)
              </span>
              <Button
                type='button'
                variant='ghost'
                size='sm'
                onClick={() => removeEntry(model)}
              >
                {t('Remove')}
              </Button>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
