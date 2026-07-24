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
  ArrowUpRight,
  BarChart3,
  Gift,
  Receipt,
  TrendingUp,
  Users,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { IconBadge } from '@/components/ui/icon-badge'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { formatQuota } from '@/lib/format'

import type { UserWalletData } from '../types'

interface AffiliateRewardsCardProps {
  user: UserWalletData | null
  affiliateLink: string
  onTransfer: () => void
  onOpenBilling: () => void
  complianceConfirmed?: boolean
  loading?: boolean
}

export function AffiliateRewardsCard({
  user,
  affiliateLink,
  onTransfer,
  onOpenBilling,
  complianceConfirmed = true,
  loading,
}: AffiliateRewardsCardProps) {
  const { t } = useTranslation()
  if (loading) {
    return (
      <Card
        data-card-hover='false'
        className='h-full gap-0 overflow-hidden py-0'
      >
        <CardHeader className='border-b p-3 sm:p-5'>
          <Skeleton className='h-6 w-32' />
          <Skeleton className='h-4 w-48' />
        </CardHeader>
        <CardContent className='space-y-4 p-3 sm:p-5'>
          <Skeleton className='h-40 rounded-md' />
          <Skeleton className='h-16 rounded-md' />
          <Skeleton className='h-28 rounded-md' />
        </CardContent>
      </Card>
    )
  }

  const hasRewards = (user?.aff_quota ?? 0) > 0

  return (
    <Card data-card-hover='false' className='h-full gap-0 overflow-hidden py-0'>
      <CardHeader className='border-b p-3 !pb-3 sm:p-5 sm:!pb-5'>
        <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
          <div className='flex min-w-0 items-center gap-2.5'>
            <IconBadge tone='success'>
              <Gift />
            </IconBadge>
            <div className='min-w-0'>
              <h3 className='text-sm font-semibold sm:text-base'>
                {t('Referral Rewards')}
              </h3>
              <p className='text-muted-foreground text-xs sm:text-sm'>
                {t('Invite friends and earn additional rewards')}
              </p>
            </div>
          </div>
          <Button
            variant='outline'
            size='sm'
            onClick={onOpenBilling}
            className='w-full shrink-0 gap-2 sm:w-auto'
          >
            <Receipt className='h-4 w-4' />
            {t('Order History')}
          </Button>
        </div>
      </CardHeader>

      <CardContent className='space-y-4 p-3 sm:space-y-5 sm:p-5'>
        <section className='overflow-hidden rounded-md border'>
          <div className='flex items-center justify-between gap-3 border-b px-3 py-3'>
            <h4 className='text-sm font-semibold'>
              {t('Earnings Statistics')}
            </h4>
            <Button
              onClick={onTransfer}
              disabled={!hasRewards || !complianceConfirmed}
              variant='secondary'
              size='sm'
              className='h-8 gap-1.5'
            >
              <ArrowUpRight className='size-3.5' />
              {t('Transfer to Balance')}
            </Button>
          </div>

          <div className='bg-success/[0.06] grid grid-cols-3 divide-x px-1 py-5'>
            {[
              [
                t('Pending Rewards'),
                formatQuota(user?.aff_quota ?? 0),
                TrendingUp,
              ],
              [
                t('Total Rewards'),
                formatQuota(user?.aff_history_quota ?? 0),
                BarChart3,
              ],
              [t('Invited Users'), String(user?.aff_count ?? 0), Users],
            ].map(([label, value, Icon]) => (
              <div key={String(label)} className='min-w-0 px-2 text-center'>
                <div className='text-primary font-mono text-lg font-bold tabular-nums sm:text-2xl'>
                  {String(value)}
                </div>
                <div className='text-muted-foreground mt-2 flex items-center justify-center gap-1 truncate text-xs'>
                  <Icon className='size-3.5 shrink-0' />
                  <span className='truncate'>{String(label)}</span>
                </div>
              </div>
            ))}
          </div>

          <div className='grid grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-2 border-t p-3'>
            <span className='text-muted-foreground text-xs font-medium'>
              {t('Referral Link')}
            </span>
            <Input
              value={affiliateLink}
              readOnly
              className='bg-muted/30 h-9 min-w-0 font-mono text-xs'
            />
            <CopyButton
              value={affiliateLink}
              variant='outline'
              className='size-9 shrink-0'
              iconClassName='size-4'
              tooltip={t('Copy referral link')}
              aria-label={t('Copy referral link')}
            />
          </div>
        </section>

        <section className='overflow-hidden rounded-md border'>
          <h4 className='border-b px-3 py-3 text-sm font-semibold'>
            {t('My Referrals')}
          </h4>
          <div className='text-muted-foreground px-3 py-4 text-sm'>
            {(user?.aff_count ?? 0) > 0
              ? t('{{count}} referred user(s)', { count: user?.aff_count ?? 0 })
              : t('No referred users yet')}
          </div>
        </section>

        <section className='overflow-hidden rounded-md border'>
          <h4 className='border-b px-3 py-3 text-sm font-semibold'>
            {t('Reward Details')}
          </h4>
          <ul className='text-muted-foreground space-y-3 px-4 py-4 text-sm'>
            {[
              t(
                'Invite friends to register and earn rewards after they top up'
              ),
              t('Transfer available rewards into your account balance'),
              t('Invite more friends to earn more rewards'),
            ].map((item) => (
              <li key={item} className='flex gap-2'>
                <span className='bg-success mt-1.5 size-2 shrink-0 rounded-full' />
                <span>{item}</span>
              </li>
            ))}
          </ul>
        </section>

        {!complianceConfirmed ? (
          <p className='text-muted-foreground text-xs lg:col-span-3'>
            {t(
              'Referral reward transfer is disabled until the administrator confirms compliance terms.'
            )}
          </p>
        ) : null}
      </CardContent>
    </Card>
  )
}
