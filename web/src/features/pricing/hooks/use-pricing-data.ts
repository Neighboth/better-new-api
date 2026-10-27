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
import { useMemo } from 'react'

import { useStatus } from '@/hooks/use-status'

import { getPricing } from '../api'
import { getRankings } from '../../rankings/api'

export function usePricingData() {
  const { status } = useStatus()

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['pricing'],
    queryFn: getPricing,
    staleTime: 5 * 60 * 1000,
  })

  const { data: rankingsData } = useQuery({
    queryKey: ['pricing-rankings'],
    queryFn: () => getRankings('month'),
    staleTime: 5 * 60 * 1000,
  })

  // Ensure rates never reach zero to prevent division errors
  const priceRate = useMemo(
    () => Math.max((status?.price as number) ?? 1, 0.001),
    [status?.price]
  )
  const usdExchangeRate = useMemo(
    () => Math.max((status?.usd_exchange_rate as number) ?? priceRate, 0.001),
    [status?.usd_exchange_rate, priceRate]
  )

  const rankMap = useMemo(() => {
    const map = new Map<string, number>()
    const normalize = (name: string) => {
      let n = name.trim().toLowerCase()
      const slashIdx = n.indexOf('/')
      if (slashIdx !== -1) {
        n = n.substring(slashIdx + 1)
      }
      return n
    }

    if (rankingsData?.data?.models) {
      rankingsData.data.models.forEach((m) => {
        if (m.model_name) {
          const raw = m.model_name.trim().toLowerCase()
          map.set(raw, m.rank)
          const norm = normalize(m.model_name)
          if (!map.has(norm)) {
            map.set(norm, m.rank)
          }
        }
      })
    }
    return map
  }, [rankingsData])

  const models = useMemo(() => {
    if (!data?.data || !data?.vendors) return []

    const VENDOR_NAME_NORM: Record<string, string> = {
      '讯飞': 'iFlytek',
      '科大讯飞': 'iFlytek',
      '快手': 'Kuaishou',
      '字节跳动': 'ByteDance',
      '火山引擎': 'ByteDance',
      '百度': 'Baidu',
      '腾讯': 'Tencent',
      '零一万物': '01.AI',
      '即梦': 'Jimeng',
      '智谱': 'Z.AI',
      '智谱AI': 'Z.AI',
      '阿里巴巴': 'Qwen',
      '阿里': 'Qwen',
    }

    const normalizedVendors = (data.vendors || []).map((v) => ({
      ...v,
      name: VENDOR_NAME_NORM[v.name] || v.name,
      icon:
        v.name === 'Meta'
          ? 'Meta.Color'
          : v.name === 'Z.AI' || v.name === '智谱' || v.name === '智谱AI'
            ? 'ZAI'
            : v.icon,
    }))

    const vendorMap = new Map(normalizedVendors.map((v) => [v.id, v]))

    const getRank = (name: string): number => {
      if (!name) return 999999
      const raw = name.trim().toLowerCase()
      if (rankMap.has(raw)) return rankMap.get(raw)!
      let norm = raw
      const slashIdx = norm.indexOf('/')
      if (slashIdx !== -1) {
        norm = norm.substring(slashIdx + 1)
      }
      return rankMap.get(norm) ?? 999999
    }

    return data.data.map((model) => {
      const vendor = model.vendor_id
        ? vendorMap.get(model.vendor_id)
        : undefined

      let modelIcon = model.icon
      const mNameLower = model.model_name.toLowerCase()
      const isMetaModel =
        vendor?.name === 'Meta' ||
        mNameLower.startsWith('meta/') ||
        mNameLower.startsWith('meta-') ||
        mNameLower.includes('llama')

      if (isMetaModel && (!modelIcon || modelIcon === 'Ollama' || modelIcon === 'Ollama.Color')) {
        modelIcon = 'Meta.Color'
      }

      const isZaiModel =
        vendor?.name === 'Z.AI' ||
        mNameLower.startsWith('glm-') ||
        mNameLower.includes('chatglm')

      if (isZaiModel && (!modelIcon || modelIcon.includes('Zhipu'))) {
        modelIcon = 'ZAI'
      }

      return {
        ...model,
        icon: modelIcon,
        key: model.model_name,
        rank: getRank(model.model_name),
        vendor_name: vendor ? (VENDOR_NAME_NORM[vendor.name] || vendor.name) : undefined,
        vendor_icon: isMetaModel ? 'Meta.Color' : (isZaiModel ? 'ZAI' : vendor?.icon),
        vendor_description: vendor?.description,
        group_ratio: data.group_ratio,
      }
    })
  }, [data, rankMap])

  const VENDOR_NAME_NORM: Record<string, string> = {
    '讯飞': 'iFlytek',
    '科大讯飞': 'iFlytek',
    '快手': 'Kuaishou',
    '字节跳动': 'ByteDance',
    '火山引擎': 'ByteDance',
    '百度': 'Baidu',
    '腾讯': 'Tencent',
    '零一万物': '01.AI',
    '即梦': 'Jimeng',
    '智谱': 'Z.AI',
    '智谱AI': 'Z.AI',
    '阿里巴巴': 'Qwen',
    '阿里': 'Qwen',
  }

  const vendors = useMemo(() => {
    return (data?.vendors ?? []).map((v) => ({
      ...v,
      name: VENDOR_NAME_NORM[v.name] || v.name,
      icon:
        v.name === 'Meta'
          ? 'Meta.Color'
          : v.name === 'Z.AI' || v.name === '智谱' || v.name === '智谱AI'
            ? 'ZAI'
            : v.icon,
    }))
  }, [data?.vendors])

  return {
    models,
    vendors,
    groupRatio: data?.group_ratio ?? {},
    usableGroup: data?.usable_group ?? {},
    endpointMap: data?.supported_endpoint ?? {},
    autoGroups: data?.auto_groups ?? [],
    alwaysCacheHit: Boolean(data?.always_cache_hit),
    isLoading,
    error,
    refetch,
    priceRate,
    usdExchangeRate,
  }
}
