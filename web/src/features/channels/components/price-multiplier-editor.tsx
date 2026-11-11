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
import { ListChecks, Percent } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { JsonEditor } from '@/components/json-editor'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { MultiSelect } from '@/components/multi-select'
import { cn } from '@/lib/utils'

export type PriceMultiplierEditorProps = {
  /** JSON string, e.g. `{"gpt-4o":0.5}`. */
  value: string
  onChange: (value: string) => void
  /** Models of the channel(s); used for the quick set. */
  models?: string[]
  disabled?: boolean
  /** Shows the quick "apply a percentage to the selected models" helper. */
  showQuickSet?: boolean
  /**
   * Shows the "Clear" action of the editor. Callers that already render their
   * own Clear button next to the field title (the channel drawer) turn it off,
   * so the form never shows two Clear buttons for the same field.
   */
  showClear?: boolean
  className?: string
}

function parseMultipliers(raw: string): Array<[string, number]> {
  if (!raw || !raw.trim()) return []
  try {
    const parsed: unknown = JSON.parse(raw)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return []
    return Object.entries(parsed as Record<string, unknown>)
      .map(([key, value]) => [key, Number(value)] as [string, number])
      .filter(([key]) => key.trim() !== '')
  } catch {
    return []
  }
}

function serialize(entries: Array<[string, number]>): string {
  const result: Record<string, number> = {}
  for (const [key, value] of entries) {
    const model = key.trim()
    if (!model) continue
    if (!Number.isFinite(value) || value <= 0) continue
    result[model] = value
  }
  return Object.keys(result).length === 0 ? '' : JSON.stringify(result, null, 2)
}

function percentToMultiplier(percent: number): number {
  const clamped = Math.min(100, Math.max(0, percent))
  return Math.round((1 - clamped / 100) * 10000) / 10000
}

/**
 * Editor for the per-model price multiplier of a channel.
 *
 * The table/JSON switch lives inside {@link JsonEditor} itself: the previous
 * version wrapped it in a second Visual/JSON toggle, so the field showed two
 * toggles and the outer one rendered a broken table. Only the inner editor is
 * rendered now, and the quick percentage helper (with an "all models" shortcut)
 * sits underneath it.
 */
export function PriceMultiplierEditor(props: PriceMultiplierEditorProps) {
  const { t } = useTranslation()
  const [quickPercent, setQuickPercent] = useState('20')
  const [quickModels, setQuickModels] = useState<string[]>([])

  const entries = useMemo(() => parseMultipliers(props.value), [props.value])
  const modelOptions = useMemo(
    () =>
      (props.models ?? [])
        .filter(Boolean)
        .map((model) => ({ value: model, label: model })),
    [props.models]
  )

  const applyQuickSet = () => {
    const percent = Number(quickPercent)
    if (!Number.isFinite(percent) || percent < 0 || percent > 100) return
    if (quickModels.length === 0) return
    const multiplier = percentToMultiplier(percent)
    const map = new Map(entries)
    for (const model of quickModels) {
      if (percent === 0) {
        map.delete(model)
      } else {
        map.set(model, multiplier)
      }
    }
    props.onChange(serialize([...map.entries()]))
  }

  const allModelsSelected =
    modelOptions.length > 0 && quickModels.length === modelOptions.length

  return (
    <div className={cn('space-y-3', props.className)}>
      <JsonEditor
        emptyMessage={t('No price multiplier configured yet.')}
        disabled={props.disabled}
        keyLabel={t('Model')}
        keyPlaceholder='gpt-4o'
        onChange={props.onChange}
        template={{ 'gpt-4o': 0.8 }}
        value={props.value}
        valueLabel={t('Multiplier (1 = no discount)')}
        valueType='number'
      />

      {props.showClear !== false && (
        <div className='flex justify-end'>
          <Button
            disabled={props.disabled}
            onClick={() => props.onChange('')}
            size='sm'
            type='button'
            variant='ghost'
          >
            {t('Clear')}
          </Button>
        </div>
      )}

      {props.showQuickSet && (
        <div className='bg-muted/30 space-y-2 rounded-lg border p-3'>
          <div className='flex flex-wrap items-center justify-between gap-2'>
            <div className='flex items-center gap-2'>
              <Percent className='size-3.5' />
              <Label className='text-xs font-medium'>
                {t('Quick price multiplier')}
              </Label>
            </div>
            <div className='flex items-center gap-2'>
              <Button
                disabled={props.disabled || modelOptions.length === 0}
                onClick={() =>
                  setQuickModels(
                    allModelsSelected
                      ? []
                      : modelOptions.map((option) => option.value)
                  )
                }
                size='sm'
                type='button'
                variant='outline'
              >
                {t('All models')}
              </Button>
              <Button
                disabled={props.disabled || quickModels.length === 0}
                onClick={() => setQuickModels([])}
                size='sm'
                type='button'
                variant='ghost'
              >
                {t('Clear selection')}
              </Button>
            </div>
          </div>
          <MultiSelect
            allowCreate
            onChange={setQuickModels}
            options={modelOptions}
            placeholder={t('Select the models this discount applies to')}
            selected={quickModels}
          />
          <div className='flex items-end gap-2'>
            <div className='flex-1 space-y-1'>
              <Label className='text-[11px]'>{t('Discount (%)')}</Label>
              <Input
                className='text-xs'
                max={100}
                min={0}
                onChange={(event) => setQuickPercent(event.target.value)}
                type='number'
                value={quickPercent}
              />
            </div>
            <Button
              disabled={props.disabled || quickModels.length === 0}
              onClick={applyQuickSet}
              size='sm'
              type='button'
            >
              <ListChecks className='size-3.5' />
              {t('Apply')}
            </Button>
          </div>
          <p className='text-muted-foreground text-[11px]'>
            {t(
              'The discount is applied to the models above: 20% makes the model cost 80% of its price. 0% removes the multiplier.'
            )}
          </p>
        </div>
      )}
    </div>
  )
}
