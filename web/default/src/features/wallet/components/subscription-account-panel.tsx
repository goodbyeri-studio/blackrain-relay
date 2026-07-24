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
import { RefreshCw } from 'lucide-react'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { Progress } from '@/components/ui/progress'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import type {
  PlanRecord,
  UserSubscriptionRecord,
} from '@/features/subscriptions/types'
import { formatQuotaCNY } from '@/lib/format'

const BILLING_PREFERENCES = [
  'subscription_first',
  'wallet_first',
  'subscription_only',
  'wallet_only',
] as const

type BillingPreference = (typeof BILLING_PREFERENCES)[number]

interface SubscriptionAccountPanelProps {
  plans: PlanRecord[]
  activeSubscriptions: UserSubscriptionRecord[]
  allSubscriptions: UserSubscriptionRecord[]
  billingPreference: string
  loadError: boolean
  updatingPreference: boolean
  refreshing: boolean
  onPreferenceChange: (preference: string) => void
  onRefresh: () => void
}

function getBillingPreferenceLabel(
  preference: BillingPreference,
  t: (key: string) => string
): string {
  switch (preference) {
    case 'subscription_first':
      return t('Subscription First')
    case 'wallet_first':
      return t('Wallet First')
    case 'subscription_only':
      return t('Subscription Only')
    case 'wallet_only':
      return t('Wallet Only')
  }
}

export function SubscriptionAccountPanel(props: SubscriptionAccountPanelProps) {
  const { t } = useTranslation()
  const hasActive = props.activeSubscriptions.length > 0
  const hasAny = props.allSubscriptions.length > 0
  const currentPreference = BILLING_PREFERENCES.includes(
    props.billingPreference as BillingPreference
  )
    ? (props.billingPreference as BillingPreference)
    : 'subscription_first'
  const hasUnavailableSubscriptionPreference =
    !props.loadError &&
    !hasActive &&
    (currentPreference === 'subscription_first' ||
      currentPreference === 'subscription_only')
  let accountStatusLabel = t('No Active')
  let accountStatusVariant: 'danger' | 'neutral' | 'success' = 'neutral'
  if (props.loadError) {
    accountStatusLabel = t('Failed to load')
    accountStatusVariant = 'danger'
  } else if (hasActive) {
    accountStatusLabel = `${props.activeSubscriptions.length} ${t('active')}`
    accountStatusVariant = 'success'
  }

  const planTitleMap = useMemo(() => {
    const map = new Map<number, string>()
    for (const record of props.plans) {
      if (record.plan?.id) {
        map.set(record.plan.id, record.plan.title || '')
      }
    }
    return map
  }, [props.plans])

  return (
    <section className='overflow-hidden rounded-md border'>
      <div className='flex flex-col gap-3 p-3 sm:flex-row sm:items-center sm:justify-between sm:p-4'>
        <div className='min-w-0'>
          <div className='flex flex-wrap items-center gap-2'>
            <h4 className='text-sm font-semibold'>{t('My Subscriptions')}</h4>
            <StatusBadge
              label={accountStatusLabel}
              variant={accountStatusVariant}
              size='sm'
              copyable={false}
            />
          </div>
          <p className='text-muted-foreground mt-1 text-xs'>
            {t('Billing Priority')}
          </p>
        </div>

        <div className='flex w-full items-center gap-2 sm:w-auto'>
          <Select
            items={BILLING_PREFERENCES.map((preference) => ({
              value: preference,
              label: getBillingPreferenceLabel(preference, t),
            }))}
            value={currentPreference}
            disabled={props.loadError || props.updatingPreference}
            onValueChange={(value) => {
              if (value !== null) props.onPreferenceChange(value)
            }}
          >
            <SelectTrigger
              className='h-8 flex-1 text-xs sm:w-40 sm:flex-none'
              aria-label={t('Billing Priority')}
            >
              <SelectValue>
                {getBillingPreferenceLabel(currentPreference, t)}
              </SelectValue>
            </SelectTrigger>
            <SelectContent alignItemWithTrigger={false}>
              <SelectGroup>
                <SelectItem value='subscription_first' disabled={!hasActive}>
                  {t('Subscription First')}
                </SelectItem>
                <SelectItem value='wallet_first'>
                  {t('Wallet First')}
                </SelectItem>
                <SelectItem value='subscription_only' disabled={!hasActive}>
                  {t('Subscription Only')}
                </SelectItem>
                <SelectItem value='wallet_only'>{t('Wallet Only')}</SelectItem>
              </SelectGroup>
            </SelectContent>
          </Select>

          <Tooltip>
            <TooltipTrigger
              render={
                <Button
                  variant='ghost'
                  size='icon'
                  className='size-8 shrink-0'
                  onClick={props.onRefresh}
                  disabled={props.refreshing}
                  aria-label={t('Refresh')}
                />
              }
            >
              <RefreshCw
                className={props.refreshing ? 'animate-spin' : undefined}
                aria-hidden='true'
              />
            </TooltipTrigger>
            <TooltipContent>{t('Refresh')}</TooltipContent>
          </Tooltip>
        </div>
      </div>

      {hasUnavailableSubscriptionPreference ? (
        <p className='text-muted-foreground border-t px-3 py-2 text-xs sm:px-4'>
          {currentPreference === 'subscription_only'
            ? t(
                'No active subscription is available. Subscription-only billing can cause API requests to fail.'
              )
            : t(
                'Preference saved as {{pref}}, but no active subscription. Wallet will be used automatically.',
                {
                  pref: getBillingPreferenceLabel(currentPreference, t),
                }
              )}
        </p>
      ) : null}

      {hasAny ? (
        <>
          <Separator />
          <div className='max-h-72 space-y-2 overflow-y-auto p-3 sm:p-4'>
            {props.allSubscriptions.map((record) => {
              const subscription = record.subscription
              const totalAmount = Number(subscription.amount_total || 0)
              const usedAmount = Number(subscription.amount_used || 0)
              const remainingAmount =
                totalAmount > 0 ? Math.max(0, totalAmount - usedAmount) : 0
              const usagePercent =
                totalAmount > 0
                  ? Math.min(100, Math.round((usedAmount / totalAmount) * 100))
                  : 0
              const isExpired = subscription.end_time < Date.now() / 1000
              const isCancelled = subscription.status === 'cancelled'
              const isActive = subscription.status === 'active' && !isExpired
              const planTitle = planTitleMap.get(subscription.plan_id)

              let statusLabel = t('Expired')
              let statusVariant: 'success' | 'neutral' = 'neutral'
              let endTimeLabel = t('Expired at')
              if (isActive) {
                statusLabel = t('Active')
                statusVariant = 'success'
                endTimeLabel = t('Until')
              } else if (isCancelled) {
                statusLabel = t('Cancelled')
                endTimeLabel = t('Cancelled at')
              }

              return (
                <div
                  key={subscription.id}
                  className='bg-background rounded-md border p-3'
                >
                  <div className='flex flex-wrap items-center justify-between gap-2'>
                    <span className='min-w-0 truncate text-sm font-medium'>
                      {planTitle || t('Subscription')} #{subscription.id}
                    </span>
                    <StatusBadge
                      label={statusLabel}
                      variant={statusVariant}
                      size='sm'
                      copyable={false}
                    />
                  </div>

                  <div className='text-muted-foreground mt-2 grid gap-1 text-xs sm:grid-cols-2'>
                    <span>
                      {endTimeLabel}:{' '}
                      {new Date(subscription.end_time * 1000).toLocaleString()}
                    </span>
                    {isActive && subscription.next_reset_time ? (
                      <span>
                        {t('Next reset')}:{' '}
                        {new Date(
                          subscription.next_reset_time * 1000
                        ).toLocaleString()}
                      </span>
                    ) : null}
                  </div>

                  <div className='text-muted-foreground mt-2 text-xs'>
                    {totalAmount > 0 ? (
                      <span>
                        {t('Used')} {formatQuotaCNY(usedAmount)} ·{' '}
                        {t('Remaining')} {formatQuotaCNY(remainingAmount)}
                      </span>
                    ) : (
                      <span>
                        {t('Total Quota')}: {t('Unlimited')}
                      </span>
                    )}
                  </div>
                  {totalAmount > 0 && isActive ? (
                    <Progress value={usagePercent} className='mt-2 h-1.5' />
                  ) : null}
                </div>
              )
            })}
          </div>
        </>
      ) : null}

      {!hasAny && props.loadError ? (
        <p
          className='text-destructive border-t px-3 py-3 text-xs sm:px-4'
          role='alert'
        >
          {t('Failed to load')}
        </p>
      ) : null}

      {!hasAny && !props.loadError ? (
        <p className='text-muted-foreground border-t px-3 py-3 text-xs sm:px-4'>
          {t('Subscribe to a plan for model access')}
        </p>
      ) : null}
    </section>
  )
}
