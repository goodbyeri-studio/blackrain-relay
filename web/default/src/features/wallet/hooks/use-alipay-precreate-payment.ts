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
import i18next from 'i18next'
import { useCallback, useState } from 'react'
import { toast } from 'sonner'

import {
  getAlipayPrecreatePaymentStatus,
  isApiSuccess,
  requestAlipayPrecreatePayment,
} from '../api'
import { mergeAlipayPrecreateOrder } from '../lib'
import type { AlipayPrecreateOrder } from '../types'

export function useAlipayPrecreatePayment() {
  const [open, setOpen] = useState(false)
  const [creating, setCreating] = useState(false)
  const [createdOrder, setCreatedOrder] = useState<AlipayPrecreateOrder | null>(
    null
  )

  const statusQuery = useQuery({
    queryKey: ['wallet', 'alipay-precreate-order', createdOrder?.trade_no],
    queryFn: async () => {
      const response = await getAlipayPrecreatePaymentStatus(
        createdOrder?.trade_no || ''
      )
      if (!isApiSuccess(response) || !response.data) {
        throw new Error(response.message || 'Payment status request failed')
      }
      return response.data
    },
    enabled:
      open && !!createdOrder?.trade_no && createdOrder.status === 'pending',
    refetchInterval: (query) => {
      return query.state.data && query.state.data.status !== 'pending'
        ? false
        : 2000
    },
    retry: 2,
  })

  const order = mergeAlipayPrecreateOrder(createdOrder, statusQuery.data)

  const processAlipayPrecreatePayment = useCallback(async (amount: number) => {
    setOpen(false)
    setCreatedOrder(null)
    setCreating(true)
    try {
      const response = await requestAlipayPrecreatePayment({
        amount: Math.floor(amount),
        client_request_id: crypto.randomUUID(),
      })
      if (!isApiSuccess(response) || !response.data?.qr_code) {
        toast.error(response.message || i18next.t('Payment request failed'))
        return false
      }
      setCreatedOrder(response.data)
      setOpen(true)
      return true
    } catch {
      toast.error(i18next.t('Payment request failed'))
      return false
    } finally {
      setCreating(false)
    }
  }, [])

  const handleOpenChange = useCallback((nextOpen: boolean) => {
    setOpen(nextOpen)
    if (!nextOpen) setCreatedOrder(null)
  }, [])

  return {
    open,
    setOpen: handleOpenChange,
    order,
    creating,
    refreshing: statusQuery.isFetching,
    refreshError: statusQuery.isError,
    refresh: statusQuery.refetch,
    processAlipayPrecreatePayment,
  }
}
