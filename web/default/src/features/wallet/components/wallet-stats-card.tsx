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
import { Activity, BarChart3, WalletCards } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { IconBadge, type IconBadgeTone } from '@/components/ui/icon-badge'
import { Skeleton } from '@/components/ui/skeleton'
import { formatQuotaCNY } from '@/lib/format'

import type { UserWalletData } from '../types'

interface WalletStatsCardProps {
  user: UserWalletData | null
  loading?: boolean
}

export function WalletStatsCard(props: WalletStatsCardProps) {
  const { t } = useTranslation()
  if (props.loading) {
    return (
      <div className='border-y'>
        <div className='px-3 py-3 sm:px-5'>
          <Skeleton className='h-5 w-28' />
        </div>
        <div className='bg-primary/[0.06] grid grid-cols-3 divide-x px-1 py-4 sm:px-3 sm:py-6'>
          {['balance', 'usage', 'requests'].map((key) => (
            <div key={key} className='min-w-0 px-2 text-center sm:px-5'>
              <Skeleton className='mx-auto h-7 w-20' />
              <Skeleton className='mx-auto mt-2 h-4 w-24' />
            </div>
          ))}
        </div>
      </div>
    )
  }

  const stats: {
    label: string
    value: string
    description: string
    icon: typeof WalletCards
    tone: IconBadgeTone
  }[] = [
    {
      label: t('Current Balance'),
      value: formatQuotaCNY(props.user?.quota ?? 0),
      description: t('Remaining quota'),
      icon: WalletCards,
      tone: 'success',
    },
    {
      label: t('Total Usage'),
      value: formatQuotaCNY(props.user?.used_quota ?? 0),
      description: t('Total consumed quota'),
      icon: BarChart3,
      tone: 'info',
    },
    {
      label: t('API Requests'),
      value: (props.user?.request_count ?? 0).toLocaleString(),
      description: t('Total requests made'),
      icon: Activity,
      tone: 'chart-4',
    },
  ]

  return (
    <div className='border-y'>
      <div className='px-3 py-3 text-sm font-semibold sm:px-5'>
        {t('Account Statistics')}
      </div>
      <div className='bg-primary/[0.06] grid grid-cols-3 divide-x px-1 py-4 sm:px-3 sm:py-6'>
        {stats.map((item) => (
          <div key={item.label} className='min-w-0 px-2 text-center sm:px-5'>
            <div className='flex items-center justify-center gap-1.5 sm:gap-2'>
              <IconBadge tone={item.tone} size='sm'>
                <item.icon />
              </IconBadge>
              <div className='text-primary font-mono text-base font-bold break-all tabular-nums sm:text-2xl'>
                {item.value}
              </div>
            </div>
            <div className='text-muted-foreground mt-2 truncate text-xs sm:text-sm'>
              {item.label}
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
