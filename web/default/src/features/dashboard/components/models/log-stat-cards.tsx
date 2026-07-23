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
import {
  Activity,
  BarChart3,
  Coins,
  Gauge,
  Hash,
  Layers,
  WalletCards,
  Zap,
  type LucideIcon,
} from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { IconBadge } from '@/components/ui/icon-badge'
import { Skeleton } from '@/components/ui/skeleton'
import { getUserQuotaDates } from '@/features/dashboard/api'
import {
  buildQueryParams,
  calculateDashboardStats,
  getDefaultDays,
  safeDivide,
} from '@/features/dashboard/lib'
import type {
  QuotaDataItem,
  DashboardFilters,
} from '@/features/dashboard/types'
import { toIntlLocale } from '@/i18n/languages'
import { formatCompactNumber, formatNumber, formatQuota } from '@/lib/format'
import { computeTimeRange } from '@/lib/time'
import { useAuthStore } from '@/stores/auth-store'

interface LogStatCardsProps {
  filters?: DashboardFilters
  onDataUpdate?: (data: QuotaDataItem[], loading: boolean) => void
}

const MAX_INLINE_STAT_CHARS = 9

function formatStatNumber(value: number, locale: Intl.LocalesArgument) {
  const fullValue = formatNumber(value, locale)
  const displayValue =
    fullValue.length > MAX_INLINE_STAT_CHARS
      ? formatCompactNumber(value, locale)
      : fullValue

  return {
    displayValue,
    fullValue,
  }
}

export function LogStatCards(props: LogStatCardsProps) {
  const { t, i18n } = useTranslation()
  const user = useAuthStore((state) => state.auth.user)
  const isAdmin = !!(user?.role && user.role >= 10)
  const [stats, setStats] = useState<{
    totalQuota: number
    totalCount: number
    totalTokens: number
  } | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(false)

  const [timeRangeMinutes, setTimeRangeMinutes] = useState(0)

  const { filters, onDataUpdate } = props

  useEffect(() => {
    const abortController = new AbortController()
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setLoading(true)

    setError(false)
    onDataUpdate?.([], true)

    const timeRange = computeTimeRange(
      getDefaultDays(filters?.time_granularity),
      filters?.start_timestamp,
      filters?.end_timestamp
    )
    const timeDiff = (timeRange.end_timestamp - timeRange.start_timestamp) / 60
    setTimeRangeMinutes(timeDiff)

    void getUserQuotaDates(buildQueryParams(timeRange, filters), isAdmin)
      .then((res) => {
        if (abortController.signal.aborted) return
        const data = res?.data || []
        setStats(calculateDashboardStats(data))
        onDataUpdate?.(data, false)
      })
      .catch(() => {
        if (abortController.signal.aborted) return
        setStats(null)
        setError(true)
        onDataUpdate?.([], false)
      })
      .finally(() => {
        if (!abortController.signal.aborted) {
          setLoading(false)
        }
      })

    return () => {
      abortController.abort()
    }
  }, [filters, isAdmin, onDataUpdate])

  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const statCount = formatStatNumber(stats?.totalCount ?? 0, locale)
  const statTokens = formatStatNumber(stats?.totalTokens ?? 0, locale)
  const averageRpm = formatStatNumber(
    safeDivide(stats?.totalCount ?? 0, timeRangeMinutes),
    locale
  )
  const averageTpm = formatStatNumber(
    safeDivide(stats?.totalTokens ?? 0, timeRangeMinutes),
    locale
  )
  const groups: Array<{
    title: string
    tone: 'info' | 'success' | 'warning' | 'chart-4'
    icon: LucideIcon
    items: Array<{
      label: string
      value: string
      fullValue?: string
      icon: LucideIcon
      dependsOnStats?: boolean
    }>
  }> = [
    {
      title: t('Account Data'),
      tone: 'info',
      icon: WalletCards,
      items: [
        {
          label: t('Remaining Balance'),
          value: formatQuota(user?.quota ?? 0),
          icon: WalletCards,
        },
        {
          label: t('Historical Usage'),
          value: formatQuota(user?.used_quota ?? 0),
          icon: BarChart3,
        },
      ],
    },
    {
      title: t('Usage Statistics'),
      tone: 'success',
      icon: Activity,
      items: [
        {
          label: t('Request Count'),
          value: formatNumber(user?.request_count ?? 0, locale),
          icon: Activity,
        },
        {
          label: t('Statistical Count'),
          value: statCount.displayValue,
          fullValue: statCount.fullValue,
          icon: Hash,
          dependsOnStats: true,
        },
      ],
    },
    {
      title: t('Resource Consumption'),
      tone: 'warning',
      icon: Zap,
      items: [
        {
          label: t('Statistical Quota'),
          value: formatQuota(stats?.totalQuota ?? 0),
          icon: Coins,
          dependsOnStats: true,
        },
        {
          label: t('Statistical Tokens'),
          value: statTokens.displayValue,
          fullValue: statTokens.fullValue,
          icon: Layers,
          dependsOnStats: true,
        },
      ],
    },
    {
      title: t('Performance Metrics'),
      tone: 'chart-4',
      icon: Gauge,
      items: [
        {
          label: t('Average RPM'),
          value: averageRpm.displayValue,
          fullValue: averageRpm.fullValue,
          icon: Gauge,
          dependsOnStats: true,
        },
        {
          label: t('Average TPM'),
          value: averageTpm.displayValue,
          fullValue: averageTpm.fullValue,
          icon: Zap,
          dependsOnStats: true,
        },
      ],
    },
  ]

  return (
    <div className='grid min-w-0 gap-3 sm:grid-cols-2 xl:grid-cols-4'>
      {groups.map((group) => {
        const GroupIcon = group.icon
        return (
          <section
            key={group.title}
            className='overflow-hidden rounded-lg border'
          >
            <div className='flex items-center gap-2 border-b px-3 py-2.5 text-sm font-semibold'>
              <IconBadge tone={group.tone} size='xs'>
                <GroupIcon />
              </IconBadge>
              {group.title}
            </div>
            <div className='divide-y px-3'>
              {group.items.map((item) => {
                const Icon = item.icon
                const showError = error && item.dependsOnStats
                return (
                  <div
                    key={item.label}
                    className='flex min-w-0 items-center gap-3 py-3'
                  >
                    <IconBadge tone={group.tone} size='stat'>
                      <Icon />
                    </IconBadge>
                    <div className='min-w-0 flex-1'>
                      <div className='text-muted-foreground truncate text-xs'>
                        {item.label}
                      </div>
                      {loading && item.dependsOnStats ? (
                        <Skeleton className='mt-1 h-6 w-24' />
                      ) : (
                        <div
                          className='mt-0.5 truncate font-mono text-base font-bold tabular-nums sm:text-lg'
                          title={item.fullValue ?? item.value}
                        >
                          {showError ? '--' : item.value}
                        </div>
                      )}
                    </div>
                  </div>
                )
              })}
            </div>
          </section>
        )
      })}
    </div>
  )
}
