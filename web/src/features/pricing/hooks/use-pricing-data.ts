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

    const vendorMap = new Map(data.vendors.map((v) => [v.id, v]))

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
      return {
        ...model,
        key: model.model_name,
        rank: getRank(model.model_name),
        vendor_name: vendor?.name,
        vendor_icon: vendor?.icon,
        vendor_description: vendor?.description,
        group_ratio: data.group_ratio,
      }
    })
  }, [data, rankMap])

  return {
    models,
    vendors: data?.vendors ?? [],
    groupRatio: data?.group_ratio ?? {},
    usableGroup: data?.usable_group ?? {},
    endpointMap: data?.supported_endpoint ?? {},
    autoGroups: data?.auto_groups ?? [],
    isLoading,
    error,
    refetch,
    priceRate,
    usdExchangeRate,
  }
}
