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
import { ChevronRight, Copy } from 'lucide-react'
import { memo, useMemo, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Markdown } from '@/components/ui/markdown'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { getLobeIcon } from '@/lib/lobe-icon'
import { cn } from '@/lib/utils'

import { DEFAULT_TOKEN_UNIT } from '../constants'
import {
  getDynamicDisplayGroupRatio,
  getDynamicPricingSummary,
} from '../lib/dynamic-price'
import { parseTags } from '../lib/filters'
import { isTokenBasedModel } from '../lib/model-helpers'
import { formatPrice, formatRequestPrice } from '../lib/price'
import type { PricingModel, TokenUnit } from '../types'
import { ModelBillingModeBadge } from './model-billing-mode-badge'
import {
  ModelPerfStatsRow,
  ModelPerfSparkline,
  type ModelPerfBadgeData,
} from './model-perf-badge'

export interface ModelCardProps {
  model: PricingModel
  onClick: () => void
  priceRate?: number
  usdExchangeRate?: number
  tokenUnit?: TokenUnit
  showRechargePrice?: boolean
  selectedGroup?: string
  perf?: ModelPerfBadgeData
  alwaysCacheHit?: boolean
}

export const ModelCard = memo(function ModelCard(props: ModelCardProps) {
  const { t } = useTranslation()
  const { copyToClipboard } = useCopyToClipboard()
  const tokenUnit = props.tokenUnit ?? DEFAULT_TOKEN_UNIT
  const priceRate = props.priceRate ?? 1
  const usdExchangeRate = props.usdExchangeRate ?? 1
  const showRechargePrice = props.showRechargePrice ?? false
  const isTokenBased = isTokenBasedModel(props.model)
  const propertyBoxes = useMemo(() => {
    if (!props.model.property_boxes) return []
    try {
      const parsed = JSON.parse(props.model.property_boxes)
      return Array.isArray(parsed) ? parsed.slice(0, 8) : []
    } catch {
      return []
    }
  }, [props.model.property_boxes])
  const tokenUnitLabel = tokenUnit === 'K' ? '1K' : '1M'
  const groups = props.model.enable_groups || []
  const endpoints = props.model.supported_endpoint_types || []
  const modelIconKey = props.model.icon || props.model.vendor_icon
  const modelIcon = modelIconKey ? getLobeIcon(modelIconKey, 28) : null
  const initial = props.model.model_name?.charAt(0).toUpperCase() || '?'
  const isDynamicPricing =
    props.model.billing_mode === 'tiered_expr' &&
    Boolean(props.model.billing_expr)

  // Cache price is shown if alwaysCacheHit is enabled or when the model explicitly supports caching
  const hasCachedPrice =
    isTokenBased &&
    (Boolean(props.alwaysCacheHit) ||
      (props.model.cache_ratio != null &&
        Number(props.model.cache_ratio) > 0 &&
        Number(props.model.cache_ratio) < 1) ||
      (props.model.cache_price != null &&
        Number(props.model.cache_price) > 0) ||
      props.model.supported_endpoint_types?.includes('caching' as any))

  const dynamicSummary = isDynamicPricing
    ? getDynamicPricingSummary(props.model, {
        tokenUnit,
        showRechargePrice,
        priceRate,
        usdExchangeRate,
        groupRatioMultiplier: getDynamicDisplayGroupRatio(
          props.model,
          props.selectedGroup
        ),
      })
    : null

  const primaryGroup = groups[0]

  const handleCopy = (e: React.MouseEvent) => {
    e.stopPropagation()
    copyToClipboard(props.model.model_name || '')
  }

  let priceSummary: ReactNode
  if (dynamicSummary) {
    if (dynamicSummary.isSpecialExpression) {
      priceSummary = (
        <span className='min-w-0'>
          <span className='text-amber-700 dark:text-amber-300'>
            {t('Special billing expression')}
          </span>
          <code className='text-muted-foreground/70 mt-0.5 line-clamp-1 block font-mono text-[11px] break-all'>
            {dynamicSummary.rawExpression}
          </code>
        </span>
      )
    } else if (dynamicSummary.primaryEntries.length > 0) {
      priceSummary = (
        <>
          {dynamicSummary.primaryEntries.map((entry) => (
            <span
              key={entry.key}
              className='text-muted-foreground whitespace-nowrap'
            >
              {t(entry.shortLabel)}{' '}
              <span className='text-foreground font-mono font-semibold'>
                {entry.formatted}
              </span>
            </span>
          ))}
        </>
      )
    } else {
      priceSummary = (
        <span className='text-muted-foreground text-sm'>
          {t('Dynamic Pricing')}
        </span>
      )
    }
  } else if (props.model.billing_mode === 'input_only') {
    priceSummary = (
      <span className='text-muted-foreground whitespace-nowrap'>
        {t('Input')}{' '}
        <span className='text-foreground font-mono font-semibold'>
          {formatPrice(
            props.model,
            'input',
            tokenUnit,
            showRechargePrice,
            priceRate,
            usdExchangeRate,
            props.selectedGroup
          )}
        </span>
      </span>
    )
  } else if (props.model.billing_mode === 'output_only') {
    priceSummary = (
      <span className='text-muted-foreground whitespace-nowrap'>
        {t('Output')}{' '}
        <span className='text-foreground font-mono font-semibold'>
          {formatPrice(
            props.model,
            'output',
            tokenUnit,
            showRechargePrice,
            priceRate,
            usdExchangeRate,
            props.selectedGroup
          )}
        </span>
      </span>
    )
  } else if (props.model.billing_mode === 'duration_second') {
    priceSummary = (
      <span className='text-muted-foreground whitespace-nowrap'>
        {t('Per Second')}{' '}
        <span className='text-foreground font-mono font-semibold'>
          {formatPrice(
            props.model,
            'input',
            tokenUnit,
            showRechargePrice,
            priceRate,
            usdExchangeRate,
            props.selectedGroup
          )}
        </span>
      </span>
    )
  } else if (props.model.billing_mode === 'characters') {
    priceSummary = (
      <span className='text-muted-foreground whitespace-nowrap'>
        {t('1M Characters')}{' '}
        <span className='text-foreground font-mono font-semibold'>
          {formatPrice(
            props.model,
            'input',
            tokenUnit,
            showRechargePrice,
            priceRate,
            usdExchangeRate,
            props.selectedGroup
          )}
        </span>
      </span>
    )
  } else if (isTokenBased) {
    priceSummary = (
      <>
        <span className='text-muted-foreground whitespace-nowrap'>
          {t('Input')}{' '}
          <span className='text-foreground font-mono font-semibold'>
            {formatPrice(
              props.model,
              'input',
              tokenUnit,
              showRechargePrice,
              priceRate,
              usdExchangeRate,
              props.selectedGroup
            )}
          </span>
        </span>
        <span className='text-muted-foreground whitespace-nowrap'>
          {t('Output')}{' '}
          <span className='text-foreground font-mono font-semibold'>
            {formatPrice(
              props.model,
              'output',
              tokenUnit,
              showRechargePrice,
              priceRate,
              usdExchangeRate,
              props.selectedGroup
            )}
          </span>
        </span>
        {hasCachedPrice && (
          <span className='text-muted-foreground whitespace-nowrap'>
            {t('Cache')}{' '}
            <span className='text-foreground font-mono font-semibold'>
              {formatPrice(
                props.model,
                'cache',
                tokenUnit,
                showRechargePrice,
                priceRate,
                usdExchangeRate,
                props.selectedGroup
              )}
            </span>
          </span>
        )}
      </>
    )
  } else {
    priceSummary = (
      <span className='text-muted-foreground whitespace-nowrap'>
        <span className='text-foreground font-mono font-semibold'>
          {formatRequestPrice(
            props.model,
            showRechargePrice,
            priceRate,
            usdExchangeRate,
            props.selectedGroup
          )}
        </span>{' '}
        / {t('request')}
      </span>
    )
  }

  return (
    <div
      onClick={props.onClick}
      className={cn(
        'group relative flex flex-col rounded-xl border p-3.5 transition-colors sm:p-5 cursor-pointer',
        'hover:bg-muted/20'
      )}
    >
      {/* Header: icon + title + billing mode + id copy + price */}
      <div className='flex items-start justify-between gap-2.5 sm:gap-3'>
        <div className='flex min-w-0 flex-1 items-start gap-2.5 sm:gap-3'>
          <div className='bg-muted/40 flex size-9 shrink-0 items-center justify-center rounded-lg sm:size-10 sm:rounded-xl'>
            {modelIcon || (
              <span className='text-muted-foreground text-sm font-bold'>
                {initial}
              </span>
            )}
          </div>
          <div className='min-w-0 flex-1'>
            <div className='flex items-center gap-2 flex-wrap'>
              <h3 className='text-foreground truncate font-mono text-[15px] leading-tight font-bold'>
                {props.model.display_name || props.model.model_name}
              </h3>
              <ModelBillingModeBadge
                model={props.model}
                className='shrink-0 text-[10px] py-0 px-1.5 h-4.5'
              />
            </div>
            <div className='mt-1 flex items-center gap-1.5 text-xs text-muted-foreground/75 font-mono'>
              <span className='truncate'>{props.model.model_name}</span>
              <button
                type='button'
                onClick={handleCopy}
                className='text-muted-foreground/50 hover:text-foreground p-0.5 rounded transition-colors inline-flex items-center cursor-pointer shrink-0'
                title={t('Copy ID')}
              >
                <Copy className='size-2.5' />
              </button>
            </div>
            <div className='mt-1.5 flex flex-wrap items-baseline gap-x-2 gap-y-0.5 text-sm sm:gap-x-3'>
              {priceSummary}
            </div>
          </div>
        </div>
      </div>

      {/* Description */}
      <div className='text-muted-foreground mt-2.5 line-clamp-1 flex-1 text-[13px] leading-relaxed sm:mt-3.5 sm:line-clamp-2 sm:min-h-[2.5rem] prose prose-xs dark:prose-invert max-w-none'>
        {props.model.description ? (
          <Markdown>{props.model.description}</Markdown>
        ) : (
          t('No description available.')
        )}
      </div>

      {/* Tagline / Extra Text */}
      {props.model.tagline && (
        <div className='text-xs font-semibold text-foreground/80 mt-2'>
          {props.model.tagline}
        </div>
      )}

      {/* Property Boxes (Up to 8) */}
      {propertyBoxes.length > 0 && (
        <div className='mt-2.5 grid grid-cols-2 sm:grid-cols-4 gap-2'>
          {propertyBoxes.map((box, idx) => (
            <div
              key={idx}
              className='rounded-xl border border-border/60 bg-muted/20 px-3 py-2 flex flex-col justify-between'
            >
              <span className='text-[10px] text-muted-foreground font-medium truncate leading-tight'>
                {box.label}
              </span>
              <span className='text-xs font-bold text-foreground truncate mt-1 leading-tight'>
                {box.value}
              </span>
            </div>
          ))}
        </div>
      )}

      {/* Groups & Endpoints row */}
      <div className='mt-3 flex items-center justify-between text-xs text-muted-foreground font-mono gap-2'>
        <div className='flex items-center gap-1.5 shrink-0'>
          <span className='text-muted-foreground/80'>{t('Groups')}</span>
          <span className='text-foreground font-medium'>
            {primaryGroup || 'default'}
          </span>
          {groups.length > 1 && (
            <span className='text-muted-foreground/60 text-[10px]'>
              +{groups.length - 1}
            </span>
          )}
        </div>
        {endpoints.length > 0 ? (
          <div className='flex items-center gap-1.5 min-w-0 truncate text-right'>
            <span className='text-muted-foreground/80 shrink-0'>
              {t('Endpoints')}
            </span>
            <span className='text-foreground font-medium truncate'>
              {endpoints.join(', ')}
            </span>
          </div>
        ) : (
          <span className='text-muted-foreground/60 text-[11px] shrink-0'>
            {tokenUnitLabel}
          </span>
        )}
      </div>

      {/* Divider */}
      <div className='border-t border-border/50 my-2.5' />

      {/* Footer: OpenRouter style metrics row + full-width 24 sparkline bars */}
      <div className='flex flex-col gap-2 pt-0.5'>
        <div className='flex items-center justify-between gap-2 min-w-0'>
          <ModelPerfStatsRow
            perf={props.perf}
            channels={props.model.channels}
          />

          <button
            type='button'
            onClick={(e) => {
              e.stopPropagation()
              props.onClick()
            }}
            className='text-xs font-medium text-muted-foreground hover:text-foreground inline-flex items-center gap-1 transition-colors cursor-pointer shrink-0 ml-auto'
          >
            {t('Details')}
            <ChevronRight className='size-3.5' />
          </button>
        </div>

        <ModelPerfSparkline
          perf={props.perf}
          channels={props.model.channels}
        />
      </div>
    </div>
  )
})
