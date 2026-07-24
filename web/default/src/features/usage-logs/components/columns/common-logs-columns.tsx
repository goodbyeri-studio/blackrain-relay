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
import type { ColumnDef } from '@tanstack/react-table'
import { GitBranch, Sparkles, KeyRound } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { GroupBadge } from '@/components/group-badge'
import { StatusBadge, type StatusBadgeProps } from '@/components/status-badge'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { getUserAvatarFallback, getUserAvatarStyle } from '@/lib/avatar'
import { formatCurrencyFromUSDAsCNY } from '@/lib/currency'
import {
  formatLogQuotaCNY,
  formatTimestampToDate,
  formatUseTime,
} from '@/lib/format'
import { cn } from '@/lib/utils'

import { LOG_TYPE_ALL_VALUE } from '../../constants'
import type { UsageLog } from '../../data/schema'
import {
  formatModelName,
  getTieredBillingSummary,
  parseLogOther,
  isViolationFeeLog,
  renderAuditContent,
} from '../../lib/format'
import {
  isDisplayableLogType,
  isTimingLogType,
  getLogTypeConfig,
  isPerCallBilling,
} from '../../lib/utils'
import type { LogOtherData } from '../../types'
import { DetailsDialog } from '../dialogs/details-dialog'
import { ModelBadge } from '../model-badge'
import { getCommonLogColumnOrder } from '../usage-log-column-order'
import { UsageLogCostCell } from '../usage-log-cost'
import { useUsageLogsContext } from '../usage-logs-provider'

interface DetailSegment {
  text: string
  muted?: boolean
  danger?: boolean
}

function formatRatioCompact(ratio: number | undefined): string {
  if (ratio == null || !Number.isFinite(ratio)) return '-'
  return ratio % 1 === 0
    ? String(ratio)
    : ratio.toFixed(4).replace(/\.?0+$/, '')
}

function getGroupRatio(other: LogOtherData | null): number | null {
  const userGroupRatio = other?.user_group_ratio
  if (
    userGroupRatio != null &&
    userGroupRatio !== -1 &&
    Number.isFinite(userGroupRatio)
  ) {
    return userGroupRatio
  }

  const groupRatio = other?.group_ratio
  if (groupRatio != null && Number.isFinite(groupRatio)) {
    return groupRatio
  }

  return null
}

function buildDetailSegments(
  log: UsageLog,
  other: LogOtherData | null,
  t: (key: string, opts?: Record<string, unknown>) => string,
  isAdmin: boolean
): DetailSegment[] {
  const segments = buildTypeDetailSegments(log, other, t)
  // Quota saturation is a rare, admin-only anomaly marker; surface it first
  // and in danger styling so it stands out on the related billing log. The
  // backend already strips admin_info for non-admins; gate on isAdmin too as
  // defense in depth so the marker never leaks if that changes.
  if (isAdmin && other?.admin_info?.quota_saturation) {
    return [{ text: t('Quota clamped'), danger: true }, ...segments]
  }
  return segments
}

function buildTypeDetailSegments(
  log: UsageLog,
  other: LogOtherData | null,
  t: (key: string, opts?: Record<string, unknown>) => string
): DetailSegment[] {
  // Audit (type=3) and login (type=7) logs: render localized content from the
  // structured op descriptor instead of the raw (English-fallback) content.
  if (log.type === 3 || log.type === 7) {
    const text = renderAuditContent(other, t)
    return text ? [{ text }] : []
  }

  if (log.type === 6) {
    return [{ text: t('Async task refund') }]
  }

  if (log.type !== 2) return []

  const isViolation = isViolationFeeLog(other)
  if (isViolation) {
    const segments: DetailSegment[] = []
    segments.push({ text: t('Violation Fee'), danger: true })
    if (other?.violation_fee_code) {
      segments.push({
        text: other.violation_fee_code,
        muted: true,
      })
    }
    segments.push({
      text: `${t('Fee')}: ${formatLogQuotaCNY(other?.fee_quota ?? log.quota)}`,
      muted: true,
    })
    return segments
  }

  if (!other) return []

  const segments: DetailSegment[] = []

  const priceOpts = { digitsLarge: 4, digitsSmall: 6, abbreviate: false }
  const formatPrice = (price: number) =>
    `${formatCurrencyFromUSDAsCNY(price, priceOpts)}/M`
  const formatPriceCompact = (price: number) =>
    formatCurrencyFromUSDAsCNY(price, priceOpts)
  const formatPriceList = (prices: string[], showUnit: boolean) => {
    const text = prices.join(' / ')
    return showUnit ? `${text}/M` : text
  }
  const isTieredExpr = other.billing_mode === 'tiered_expr'
  const tieredSummary = getTieredBillingSummary(other)
  if (isTieredExpr) {
    if (tieredSummary) {
      const baseEntries = tieredSummary.priceEntries
        .filter((entry) => ['inputPrice', 'outputPrice'].includes(entry.field))
        .map((entry) => formatPriceCompact(entry.price))
      if (baseEntries.length > 0) {
        const tierLabel = tieredSummary.tier.label || t('Default')
        segments.push({
          text: `${tierLabel} · ${formatPriceList(baseEntries, true)}`,
        })
      }

      const cacheEntries = tieredSummary.priceEntries
        .filter((entry) =>
          ['cacheReadPrice', 'cacheCreatePrice', 'cacheCreate1hPrice'].includes(
            entry.field
          )
        )
        .map((entry) => {
          return formatPriceCompact(entry.price)
        })
      if (cacheEntries.length > 0) {
        segments.push({
          text: `${t('Cache')} ${formatPriceList(cacheEntries, false)}`,
          muted: true,
        })
      }

      const otherEntries = tieredSummary.priceEntries
        .filter(
          (entry) =>
            ![
              'inputPrice',
              'outputPrice',
              'cacheReadPrice',
              'cacheCreatePrice',
              'cacheCreate1hPrice',
            ].includes(entry.field)
        )
        .map((entry) => `${t(entry.shortLabel)} ${formatPrice(entry.price)}`)
      if (otherEntries.length > 0) {
        segments.push({
          text: otherEntries.join(' · '),
          muted: true,
        })
      }
    } else {
      segments.push({
        text: `${t('Dynamic Pricing')} · ${t('No matching results')}`,
        muted: true,
      })
    }
  } else {
    const modelPrice = other.model_price
    const isPerCall = isPerCallBilling(modelPrice)
    if (isPerCall && modelPrice != null) {
      segments.push({
        text: `${t('Per-call')} · ${formatCurrencyFromUSDAsCNY(modelPrice, priceOpts)}`,
      })
    } else if (other.model_ratio != null) {
      const ratioParts = [
        `${t('Model')}: ${formatRatioCompact(other.model_ratio)}`,
      ]
      if (other.cache_ratio != null) {
        ratioParts.push(
          `${t('Cache')}: ${formatRatioCompact(other.cache_ratio)}`
        )
      }
      const groupRatio = getGroupRatio(other)
      if (groupRatio != null) {
        ratioParts.push(
          `${t('Group Ratio')}: ${formatRatioCompact(groupRatio)}`
        )
      }
      segments.push({ text: ratioParts.join(' * ') })
    } else {
      const userGroupRatio = other.user_group_ratio
      const groupRatio = other.group_ratio
      const isUserGroup =
        userGroupRatio != null &&
        Number.isFinite(userGroupRatio) &&
        userGroupRatio !== -1
      const effectiveRatio = isUserGroup ? userGroupRatio : groupRatio
      const ratioLabel = isUserGroup
        ? t('User Exclusive Ratio')
        : t('Group Ratio')

      if (effectiveRatio != null && Number.isFinite(effectiveRatio)) {
        segments.push({
          text: `${ratioLabel} ${formatRatioCompact(effectiveRatio)}x`,
        })
      }
    }
  }

  if (other.is_system_prompt_overwritten) {
    segments.push({
      text: t('System Prompt Override'),
      danger: true,
    })
  }

  return segments
}

export function useCommonLogsColumns(isAdmin: boolean): ColumnDef<UsageLog>[] {
  const { t } = useTranslation()
  const columns: ColumnDef<UsageLog>[] = [
    {
      id: 'created_at',
      accessorKey: 'created_at',
      header: t('Time'),
      cell: ({ row }) => {
        const timestamp = row.getValue('created_at') as number

        return (
          <span className='truncate font-mono text-xs tabular-nums'>
            {formatTimestampToDate(timestamp)}
          </span>
        )
      },
      filterFn: (row, _id, value) => {
        if (!Array.isArray(value) || value.length === 0) return true
        if (value.includes(LOG_TYPE_ALL_VALUE)) return true
        return value.includes(String(row.original.type))
      },
      enableHiding: false,
      size: 180,
    },
  ]

  if (isAdmin) {
    columns.push(
      {
        id: 'channel',
        header: t('Channel'),
        accessorFn: (row) => row.channel,
        cell: function ChannelCell({ row }) {
          const { sensitiveVisible, setAffinityTarget, setAffinityDialogOpen } =
            useUsageLogsContext()
          const log = row.original

          if (!isDisplayableLogType(log.type)) return null

          const other = parseLogOther(log.other)
          const affinity = other?.admin_info?.channel_affinity
          const rawUseChannel = other?.admin_info?.use_channel ?? []
          const useChannel = Array.isArray(rawUseChannel)
            ? rawUseChannel.map(String).filter(Boolean)
            : []
          const hasRetryChain = useChannel.length > 1
          const channelChain = hasRetryChain
            ? useChannel.join(' → ')
            : undefined
          const channelDisplay = log.channel_name
            ? `${log.channel_name} #${log.channel}`
            : `#${log.channel}`
          const channelIdDisplay = `#${log.channel}`
          const channelName = sensitiveVisible ? log.channel_name : '••••'
          const multiKeyIndex = other?.admin_info?.multi_key_index
          const showMultiKeyIndex =
            other?.admin_info?.is_multi_key === true &&
            typeof multiKeyIndex === 'number' &&
            Number.isFinite(multiKeyIndex)

          return (
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger
                  render={
                    <div className='flex max-w-[160px] flex-col gap-0.5' />
                  }
                >
                  <div className='relative inline-flex w-fit items-center gap-1'>
                    <StatusBadge
                      label={channelIdDisplay}
                      autoColor={String(log.channel)}
                      copyText={String(log.channel)}
                      size='sm'
                      showDot={false}
                      className='font-mono'
                    />
                    {showMultiKeyIndex && (
                      <StatusBadge
                        label={String(multiKeyIndex)}
                        size='sm'
                        showDot={false}
                        copyable={false}
                        variant='neutral'
                        className='h-5 min-w-5 justify-center rounded-full px-1 font-mono text-xs'
                        aria-label={`${t('Key')} ${multiKeyIndex}`}
                      />
                    )}
                    {hasRetryChain && (
                      <Popover>
                        <PopoverTrigger
                          render={
                            <button
                              type='button'
                              className='text-muted-foreground hover:text-foreground focus-visible:ring-ring inline-flex size-5 shrink-0 items-center justify-center rounded-full transition-colors focus-visible:ring-2 focus-visible:outline-none'
                              aria-label={t('Retry Chain')}
                              onClick={(e) => e.stopPropagation()}
                            />
                          }
                        >
                          <GitBranch
                            className='size-3.5 text-amber-500'
                            aria-hidden='true'
                          />
                        </PopoverTrigger>
                        <PopoverContent
                          side='top'
                          align='start'
                          className='w-64 text-xs'
                        >
                          <div className='flex flex-col gap-1'>
                            <p className='font-medium'>{t('Retry Chain')}</p>
                            <p className='text-muted-foreground font-mono break-all'>
                              {channelChain}
                            </p>
                          </div>
                        </PopoverContent>
                      </Popover>
                    )}
                    {affinity && (
                      <button
                        type='button'
                        className='absolute -top-1 -right-1 leading-none text-amber-500'
                        onClick={(e) => {
                          e.stopPropagation()
                          setAffinityTarget({
                            rule_name: affinity.rule_name || '',
                            using_group:
                              affinity.using_group ||
                              affinity.selected_group ||
                              '',
                            key_hint: affinity.key_hint || '',
                            key_fp: affinity.key_fp || '',
                          })
                          setAffinityDialogOpen(true)
                        }}
                      >
                        <Sparkles className='size-3 fill-current' />
                      </button>
                    )}
                  </div>
                  {log.channel_name && (
                    <span className='text-muted-foreground/70 truncate [font-family:var(--font-body)] !text-xs'>
                      {channelName}
                    </span>
                  )}
                </TooltipTrigger>
                <TooltipContent>
                  <div className='space-y-1'>
                    <p>
                      {sensitiveVisible ? channelDisplay : channelIdDisplay}
                    </p>
                    {channelChain && (
                      <p className='text-muted-foreground text-xs'>
                        {t('Chain')}: {channelChain}
                      </p>
                    )}
                    {showMultiKeyIndex && (
                      <p className='text-muted-foreground text-xs'>
                        {t('Key')}: {multiKeyIndex}
                      </p>
                    )}
                    {affinity && (
                      <div className='border-t pt-1 text-xs'>
                        <p className='font-medium'>{t('Channel Affinity')}</p>
                        <p>
                          {t('Rule')}: {affinity.rule_name || '-'}
                        </p>
                        <p>
                          {t('Group')}:{' '}
                          {sensitiveVisible
                            ? affinity.using_group ||
                              affinity.selected_group ||
                              '-'
                            : '••••'}
                        </p>
                      </div>
                    )}
                  </div>
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>
          )
        },
      },
      {
        id: 'user',
        header: t('User'),
        accessorFn: (row) => row.username,
        cell: function UserCell({ row }) {
          const { sensitiveVisible, setSelectedUserId, setUserInfoDialogOpen } =
            useUsageLogsContext()
          const log = row.original

          if (!log.username) return null

          return (
            <button
              type='button'
              className='flex items-center gap-1.5 text-left'
              onClick={(e) => {
                e.stopPropagation()
                setSelectedUserId(log.user_id)
                setUserInfoDialogOpen(true)
              }}
            >
              <Avatar className='ring-border/60 size-6 ring-1 max-sm:hidden'>
                <AvatarFallback
                  className={cn(
                    'text-[11px] font-semibold',
                    !sensitiveVisible && 'bg-muted text-muted-foreground'
                  )}
                  style={
                    sensitiveVisible
                      ? getUserAvatarStyle(log.username)
                      : undefined
                  }
                >
                  {sensitiveVisible ? getUserAvatarFallback(log.username) : '•'}
                </AvatarFallback>
              </Avatar>
              <TooltipProvider delay={300}>
                <Tooltip>
                  <TooltipTrigger
                    render={
                      <span className='text-muted-foreground max-w-[100px] truncate text-sm hover:underline' />
                    }
                  >
                    {sensitiveVisible ? log.username : '••••'}
                  </TooltipTrigger>
                  {sensitiveVisible && log.username.length > 12 && (
                    <TooltipContent side='top'>{log.username}</TooltipContent>
                  )}
                </Tooltip>
              </TooltipProvider>
            </button>
          )
        },
      }
    )
  }

  columns.push({
    id: 'token_name',
    accessorKey: 'token_name',
    header: t('Token'),
    cell: function TokenNameCell({ row }) {
      const { sensitiveVisible } = useUsageLogsContext()
      const log = row.original
      if (!isDisplayableLogType(log.type)) return null

      const tokenName = log.token_name
      if (!tokenName) return null

      const displayName = sensitiveVisible ? tokenName : '••••'

      return (
        <div className='flex max-w-[160px]'>
          <TooltipProvider delay={300}>
            <Tooltip>
              <TooltipTrigger render={<div className='max-w-full' />}>
                <StatusBadge
                  label={displayName}
                  icon={KeyRound}
                  copyText={sensitiveVisible ? tokenName : undefined}
                  size='sm'
                  showDot={false}
                  className='border-border/60 bg-muted/30 text-foreground h-6 max-w-full gap-1.5 overflow-hidden rounded-md border px-2 py-0.5 [font-family:var(--font-body)]'
                />
              </TooltipTrigger>
              {sensitiveVisible && tokenName.length > 16 && (
                <TooltipContent side='top' className='max-w-xs break-all'>
                  {tokenName}
                </TooltipContent>
              )}
            </Tooltip>
          </TooltipProvider>
        </div>
      )
    },
    size: 130,
  })
  columns.push({
    id: 'group',
    accessorKey: 'group',
    header: t('Group'),
    cell: function GroupCell({ row }) {
      const { sensitiveVisible } = useUsageLogsContext()
      const log = row.original
      if (!isDisplayableLogType(log.type)) return null

      const other = parseLogOther(log.other)
      const group = log.group || other?.group || ''
      if (!group) return <span className='text-muted-foreground'>-</span>

      return (
        <GroupBadge
          group={group}
          label={sensitiveVisible ? undefined : '••••'}
          size='sm'
        />
      )
    },
    size: 130,
  })
  columns.push(
    {
      id: 'model_name',
      accessorKey: 'model_name',
      header: t('Model'),
      cell: function ModelCell({ row }) {
        const log = row.original
        if (!isDisplayableLogType(log.type)) return null

        const modelInfo = formatModelName(log)

        return (
          <div className='flex w-fit flex-col gap-0.5'>
            <ModelBadge
              modelName={modelInfo.name}
              actualModel={modelInfo.actualModel}
            />
          </div>
        )
      },
      meta: { mobileTitle: true },
    },
    {
      id: 'type',
      accessorKey: 'type',
      header: t('Type'),
      cell: ({ row }) => {
        const log = row.original
        const config = getLogTypeConfig(log.type)

        return (
          <StatusBadge
            label={t(config.label)}
            variant={config.color as StatusBadgeProps['variant']}
            size='sm'
            copyable={false}
          />
        )
      },
      size: 90,
    },
    {
      id: 'prompt_tokens',
      accessorKey: 'prompt_tokens',
      header: t('Input'),
      cell: ({ row }) => {
        const log = row.original
        if (!isDisplayableLogType(log.type)) return null

        const promptTokens = log.prompt_tokens || 0
        return (
          <span className='font-mono text-xs font-medium tabular-nums'>
            {promptTokens}
          </span>
        )
      },
      size: 90,
    },
    {
      id: 'completion_tokens',
      accessorKey: 'completion_tokens',
      header: t('Output'),
      cell: ({ row }) => {
        const log = row.original
        if (!isDisplayableLogType(log.type)) return null

        return (
          <span className='font-mono text-xs font-medium tabular-nums'>
            {log.completion_tokens || 0}
          </span>
        )
      },
      size: 90,
    },

    {
      id: 'use_time',
      accessorKey: 'use_time',
      header: t('Duration / First Token'),
      cell: ({ row }) => {
        const log = row.original
        if (!isTimingLogType(log.type)) return null

        const useTime = row.getValue('use_time') as number
        const other = parseLogOther(log.other)

        const firstTokenSeconds =
          other?.frt != null && other.frt > 0 ? other.frt / 1000 : null

        return (
          <div className='flex min-w-max items-center gap-1.5'>
            <StatusBadge
              label={formatUseTime(useTime)}
              variant='success'
              size='sm'
              copyable={false}
            />
            {log.is_stream && firstTokenSeconds != null && (
              <StatusBadge
                label={formatUseTime(firstTokenSeconds)}
                variant='success'
                size='sm'
                copyable={false}
              />
            )}
            {log.is_stream && (
              <StatusBadge
                label={t('Stream')}
                variant='info'
                size='sm'
                copyable={false}
              />
            )}
          </div>
        )
      },
      size: 190,
    },

    {
      id: 'cache_tokens',
      header: t('Cache'),
      accessorFn: (row) => row.other,
      cell: ({ row }) => {
        const log = row.original
        if (!isDisplayableLogType(log.type)) return null

        const other = parseLogOther(log.other)
        const cacheReadTokens = other?.cache_tokens || 0
        const cacheWrite5m = other?.cache_creation_tokens_5m || 0
        const cacheWrite1h = other?.cache_creation_tokens_1h || 0
        const cacheWriteTokens =
          cacheWrite5m > 0 || cacheWrite1h > 0
            ? cacheWrite5m + cacheWrite1h
            : other?.cache_creation_tokens || 0

        return (
          <span className='font-mono text-xs font-medium tabular-nums'>
            {cacheReadTokens}/{cacheWriteTokens}
          </span>
        )
      },
      size: 110,
    },

    {
      id: 'quota',
      accessorKey: 'quota',
      header: t('Cost'),
      cell: ({ row }) => (
        <UsageLogCostCell
          log={row.original}
          subscriptionLabel={t('Subscription')}
          deductedBySubscriptionLabel={t('Deducted by subscription')}
        />
      ),
      size: 130,
    },

    {
      id: 'content',
      accessorKey: 'content',
      header: t('Details'),
      cell: function DetailsCell({ row }) {
        const [dialogOpen, setDialogOpen] = useState(false)
        const log = row.original
        const other = parseLogOther(log.other)

        const segments = buildDetailSegments(log, other, t, isAdmin)
        const primary = segments[0]
        const hasMore = segments.length > 1
        let primaryTextClass = 'text-foreground'
        if (primary?.muted) {
          primaryTextClass = 'text-muted-foreground/60'
        } else if (primary?.danger) {
          primaryTextClass = 'text-red-600 dark:text-red-400'
        }
        let detailPreview = <span className='text-muted-foreground/40'>—</span>
        if (primary) {
          detailPreview = (
            <span
              className={cn(
                'truncate leading-snug group-hover:underline',
                primaryTextClass
              )}
            >
              {primary.text}
              {hasMore && (
                <span className='text-muted-foreground/40 ml-0.5'>
                  +{segments.length - 1}
                </span>
              )}
            </span>
          )
        } else if (log.content) {
          detailPreview = (
            <span className='text-muted-foreground truncate group-hover:underline'>
              {log.content}
            </span>
          )
        }

        return (
          <>
            <button
              type='button'
              className='group flex max-w-[200px] items-center gap-1 text-left text-xs'
              onClick={() => setDialogOpen(true)}
              title={t('Click to view full details')}
            >
              {detailPreview}
            </button>
            <DetailsDialog
              log={log}
              isAdmin={isAdmin}
              open={dialogOpen}
              onOpenChange={setDialogOpen}
            />
          </>
        )
      },
      size: 260,
      maxSize: 320,
    }
  )

  return getCommonLogColumnOrder(isAdmin).flatMap((columnId) => {
    const column = columns.find((item) => item.id === columnId)
    return column ? [column] : []
  })
}
