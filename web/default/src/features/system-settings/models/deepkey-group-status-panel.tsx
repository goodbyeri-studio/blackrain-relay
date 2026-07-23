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
import { AlertTriangle, CheckCircle2, RefreshCw } from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { cn } from '@/lib/utils'

import { getDeepKeyGroupAdminStatuses } from '../api'
import type { DeepKeyGroupIssue } from '../types'

function formatIssue(
  issue: DeepKeyGroupIssue,
  t: (key: string) => string
): string {
  switch (issue) {
    case 'not_in_catalog':
      return t('Not in DeepKey catalog')
    case 'missing_configuration':
      return t('Missing local ratio')
    case 'missing_channel':
      return t('Missing channel')
    case 'no_enabled_channel':
      return t('No enabled channel')
    case 'invalid_key_configuration':
      return t('Invalid upstream key configuration')
    case 'ratio_drift':
      return t('Ratio differs from catalog')
  }
}

export function DeepKeyGroupStatusPanel() {
  const { t } = useTranslation()
  const statusQuery = useQuery({
    queryKey: ['deepkey-group-admin-status'],
    queryFn: getDeepKeyGroupAdminStatuses,
    staleTime: 30_000,
  })
  const rows = statusQuery.data?.data ?? []
  const hasError = statusQuery.isError || statusQuery.data?.success === false
  const catalogUnavailable = statusQuery.data?.catalog_available === false
  let statusContent: ReactNode
  if (hasError) {
    statusContent = (
      <div className='text-destructive flex min-h-20 items-center gap-2 text-sm'>
        <AlertTriangle className='h-4 w-4' />
        {statusQuery.data?.message || t('Failed to load DeepKey group status')}
      </div>
    )
  } else if (statusQuery.isLoading) {
    statusContent = (
      <div className='text-muted-foreground flex min-h-20 items-center text-sm'>
        {t('Loading...')}
      </div>
    )
  } else if (rows.length === 0) {
    statusContent = (
      <div className='text-muted-foreground flex min-h-20 items-center text-sm'>
        {t('No DeepKey groups found')}
      </div>
    )
  } else {
    statusContent = (
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('Group')}</TableHead>
            <TableHead>{t('Health')}</TableHead>
            <TableHead>{t('Ratios')}</TableHead>
            <TableHead>{t('Channels')}</TableHead>
            <TableHead>{t('Models / Tokens')}</TableHead>
            <TableHead>{t('Upstream key fingerprint')}</TableHead>
            <TableHead>{t('Last test')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((row) => (
            <TableRow key={row.group}>
              <TableCell className='font-medium'>{row.group}</TableCell>
              <TableCell>
                {row.issues.length === 0 ? (
                  <StatusBadge
                    variant='success'
                    icon={CheckCircle2}
                    label={t('Healthy')}
                    copyable={false}
                  />
                ) : (
                  <div className='flex max-w-72 flex-wrap gap-1'>
                    {row.issues.map((issue) => (
                      <StatusBadge
                        key={issue}
                        variant='warning'
                        label={formatIssue(issue, t)}
                        copyable={false}
                      />
                    ))}
                  </div>
                )}
              </TableCell>
              <TableCell>
                <div className='space-y-1'>
                  <div>
                    {t('Local')}: {row.configured_ratio ?? '-'}
                  </div>
                  <div className='text-muted-foreground'>
                    {t('Catalog')}: {row.catalog_ratio ?? '-'}
                  </div>
                </div>
              </TableCell>
              <TableCell>
                <div>
                  {row.enabled_channel_count} / {row.channel_count}
                </div>
                {row.disabled_channel_count > 0 && (
                  <div className='text-muted-foreground'>
                    {t('{{count}} disabled', {
                      count: row.disabled_channel_count,
                    })}
                  </div>
                )}
              </TableCell>
              <TableCell>
                <div>
                  {row.model_count} / {row.token_count}
                </div>
                <div className='text-muted-foreground'>
                  {t('models / tokens')}
                </div>
              </TableCell>
              <TableCell>
                {row.key_fingerprint ? (
                  <StatusBadge
                    variant='neutral'
                    label={row.key_fingerprint}
                    copyText={row.key_fingerprint}
                  />
                ) : (
                  <span className='text-muted-foreground'>
                    {t('Not configured')}
                  </span>
                )}
              </TableCell>
              <TableCell>
                {row.last_test_time > 0 ? (
                  <div className='space-y-1'>
                    <div>
                      {new Date(row.last_test_time * 1000).toLocaleString()}
                    </div>
                    <div className='text-muted-foreground'>
                      {t('{{milliseconds}} ms', {
                        milliseconds: row.response_time,
                      })}
                    </div>
                  </div>
                ) : (
                  <span className='text-muted-foreground'>
                    {t('Never tested')}
                  </span>
                )}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    )
  }

  return (
    <Card className='before:border-border/90 relative shadow-sm ring-0 before:pointer-events-none before:absolute before:inset-0 before:rounded-xl before:border'>
      <CardHeader className='bg-muted/20 border-b'>
        <div className='flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between'>
          <div>
            <CardTitle>{t('DeepKey group health')}</CardTitle>
            <CardDescription>
              {t(
                'Check catalog ratios, local channels, customer tokens, and upstream key mappings before changing a group.'
              )}
            </CardDescription>
          </div>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={() => statusQuery.refetch()}
            disabled={statusQuery.isFetching}
          >
            <RefreshCw
              className={cn(
                'mr-2 h-4 w-4',
                statusQuery.isFetching && 'animate-spin'
              )}
            />
            {t('Refresh status')}
          </Button>
        </div>
      </CardHeader>
      <CardContent className='space-y-4 pt-4'>
        {catalogUnavailable && (
          <div className='text-muted-foreground flex items-center gap-2 text-sm'>
            <AlertTriangle className='h-4 w-4' aria-hidden='true' />
            {t('DeepKey catalog unavailable; showing local status only')}
          </div>
        )}
        {statusContent}
      </CardContent>
    </Card>
  )
}
