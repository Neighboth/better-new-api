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
import { Braces, ListChecks, Percent, Plus, Table2, Trash2 } from 'lucide-react'
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
  /** Models of the channel(s); used for the visual table and the quick set. */
  models?: string[]
  disabled?: boolean
  /** Shows the quick "apply a percentage to the selected models" helper. */
  showQuickSet?: boolean
  className?: string
}

type Mode = 'visual' | 'json'

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
 * The channel form and the batch edit popup both use it, so a price
 * multiplier can be entered in a table (visual mode) or as JSON, and the quick
 * percentage helper is available wherever the editor is used.
 */
export function PriceMultiplierEditor(props: PriceMultiplierEditorProps) {
  const { t } = useTranslation()
  const [mode, setMode] = useState<Mode>('visual')
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

  const updateEntry = (
    index: number,
    patch: { model?: string; multiplier?: number }
  ) => {
    const next = entries.map((entry, i) => {
      if (i !== index) return entry
      return [
        patch.model !== undefined ? patch.model : entry[0],
        patch.multiplier !== undefined ? patch.multiplier : entry[1],
      ] as [string, number]
    })
    props.onChange(serialize(next))
  }

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

  return (
    <div className={cn('space-y-3', props.className)}>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <div className='bg-muted inline-flex rounded-md p-0.5'>
          <button
            className={cn(
              'inline-flex items-center gap-1 rounded px-2 py-1 text-xs transition',
              mode === 'visual'
                ? 'bg-background text-foreground shadow-sm'
                : 'text-muted-foreground'
            )}
            onClick={() => setMode('visual')}
            type='button'
          >
            <Table2 className='size-3.5' />
            {t('Visual')}
          </button>
          <button
            className={cn(
              'inline-flex items-center gap-1 rounded px-2 py-1 text-xs transition',
              mode === 'json'
                ? 'bg-background text-foreground shadow-sm'
                : 'text-muted-foreground'
            )}
            onClick={() => setMode('json')}
            type='button'
          >
            <Braces className='size-3.5' />
            {t('JSON')}
          </button>
        </div>
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

      {mode === 'json' ? (
        <JsonEditor
          emptyMessage={t('No model ratios configured.')}
          keyLabel={t('Model')}
          keyPlaceholder='gpt-4o'
          onChange={props.onChange}
          template={{ 'gpt-4o': 0.8 }}
          value={props.value}
          valueLabel={t('Multiplier')}
          valueType='number'
        />
      ) : (
        <div className='space-y-2'>
          {entries.length === 0 && (
            <p className='text-muted-foreground rounded-md border border-dashed px-3 py-2 text-xs'>
              {t('No price multiplier configured yet.')}
            </p>
          )}
          {entries.map(([model, multiplier], index) => (
            <div className='flex items-center gap-2' key={`${model}-${index}`}>
              <Input
                className='font-mono text-xs'
                disabled={props.disabled}
                list='price-multiplier-models'
                onChange={(event) => updateEntry(index, { model: event.target.value })}
                placeholder='gpt-4o'
                value={model}
              />
              <Input
                className='w-24 font-mono text-xs'
                disabled={props.disabled}
                min={0}
                onChange={(event) =>
                  updateEntry(index, { multiplier: Number(event.target.value) })
                }
                placeholder='1'
                step={0.05}
                type='number'
                value={Number.isFinite(multiplier) ? multiplier : ''}
              />
              <Button
                aria-label={t('Remove')}
                disabled={props.disabled}
                onClick={() =>
                  props.onChange(
                    serialize(entries.filter((_, i) => i !== index))
                  )
                }
                size='icon-sm'
                type='button'
                variant='ghost'
              >
                <Trash2 className='size-3.5' />
              </Button>
            </div>
          ))}
          <datalist id='price-multiplier-models'>
            {(props.models ?? []).map((model) => (
              <option key={model} value={model} />
            ))}
          </datalist>
          <Button
            disabled={props.disabled}
            onClick={() => props.onChange(serialize([...entries, ['', 1]]))}
            size='sm'
            type='button'
            variant='outline'
          >
            <Plus className='size-3.5' />
            {t('Add model')}
          </Button>
        </div>
      )}

      {props.showQuickSet && (
        <div className='bg-muted/30 space-y-2 rounded-lg border p-3'>
          <div className='flex items-center gap-2'>
            <Percent className='size-3.5' />
            <Label className='text-xs font-medium'>
              {t('Quick price multiplier')}
            </Label>
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
              'The discount is applied to the models above: 20% makes the model cost 80% of its price.'
            )}
          </p>
        </div>
      )}
    </div>
  )
}
