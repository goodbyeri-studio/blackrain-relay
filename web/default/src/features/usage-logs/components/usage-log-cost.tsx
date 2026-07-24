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
import { StatusBadge } from '@/components/status-badge'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { formatLogQuotaCNY } from '@/lib/format'

import type { UsageLog } from '../data/schema'
import { parseLogOther } from '../lib/format'
import { isDisplayableLogType } from '../lib/utils'

export interface UsageLogCostCellProps {
  log: UsageLog
  subscriptionLabel: string
  deductedBySubscriptionLabel: string
}

function splitQuotaDisplay(value: string): { prefix: string; amount: string } {
  const match = value.match(/^([^0-9+\-.,\s]+)(.+)$/)
  if (!match) return { prefix: '', amount: value }
  return { prefix: match[1], amount: match[2] }
}

export function UsageLogCostCell(props: UsageLogCostCellProps) {
  if (!isDisplayableLogType(props.log.type)) return null

  const quotaText = formatLogQuotaCNY(props.log.quota)
  const other = parseLogOther(props.log.other)
  const isSubscription = other?.billing_source === 'subscription'

  if (isSubscription) {
    const tooltipText = `${props.deductedBySubscriptionLabel}: ${quotaText}`

    return (
      <TooltipProvider>
        <Tooltip>
          <TooltipTrigger
            render={
              <StatusBadge
                label={props.subscriptionLabel}
                variant='success'
                size='sm'
                copyable={false}
                className='cursor-help'
                aria-label={tooltipText}
              />
            }
          />
          <TooltipContent>
            <span>{tooltipText}</span>
          </TooltipContent>
        </Tooltip>
      </TooltipProvider>
    )
  }

  const quotaDisplay = splitQuotaDisplay(quotaText)

  return (
    <div className='flex flex-col gap-0.5'>
      <span className='border-border/80 bg-muted/60 inline-flex h-6 w-fit items-center rounded-md border px-2 [font-family:var(--font-body)] text-sm leading-none font-semibold tabular-nums'>
        {quotaDisplay.prefix && (
          <span className='mr-1'>{quotaDisplay.prefix}</span>
        )}
        <span>{quotaDisplay.amount}</span>
      </span>
    </div>
  )
}
