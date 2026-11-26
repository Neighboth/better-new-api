import { type ReactNode } from 'react'

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

export const SITE_STATS_KPI_GRID_CLASS =
  'grid grid-cols-2 gap-3 md:grid-cols-3 lg:grid-cols-5'

export interface SiteStatCardProps {
  icon: ReactNode
  label: string
  value: string
  description?: string
}

/**
 * Shared KPI stat card used across both the core Site Statistics sections and
 * all bottom integration cards (Fine-Tuning, Proxy Pool, Clarity, GA4, GSC) so
 * every metric box on the page has identical spacing, borders, typography, and
 * column rhythm.
 */
export function SiteStatCard({
  icon,
  label,
  value,
  description,
}: SiteStatCardProps) {
  return (
    <Card>
      <CardHeader className='pb-2'>
        <CardTitle className='text-muted-foreground flex items-center gap-2 text-sm font-medium'>
          <span className='flex size-4 shrink-0 items-center justify-center'>
            {icon}
          </span>
          <span className='truncate'>{label}</span>
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div className='truncate text-2xl font-bold tabular-nums'>{value}</div>
        {description ? (
          <p className='text-muted-foreground mt-1 truncate text-xs'>
            {description}
          </p>
        ) : null}
      </CardContent>
    </Card>
  )
}
