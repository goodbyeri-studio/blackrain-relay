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
import { CircleCheck, Clock3, Loader2, RefreshCw } from 'lucide-react'
import { QRCodeSVG } from 'qrcode.react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

import type { AlipayPrecreateOrder } from '../../types'

interface AlipayPrecreateDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  order: AlipayPrecreateOrder | null
  refreshing: boolean
  refreshError: boolean
  onRefresh: () => void
}

export function AlipayPrecreateDialog(props: AlipayPrecreateDialogProps) {
  const { t } = useTranslation()
  const paid = props.order?.status === 'credited'
  const failed =
    props.order?.status === 'failed' || props.order?.status === 'closed'
  const expired =
    !!props.order &&
    props.order.status === 'pending' &&
    props.order.expires_at * 1000 <= Date.now()

  let paymentContent: ReactNode
  if (paid) {
    paymentContent = (
      <div className='flex flex-col items-center gap-3 text-center'>
        <CircleCheck className='h-16 w-16 text-green-600' />
        <p className='text-lg font-semibold'>{t('Payment completed')}</p>
      </div>
    )
  } else if (failed || expired) {
    paymentContent = (
      <div className='flex flex-col items-center gap-3 text-center'>
        <Clock3 className='text-muted-foreground h-14 w-14' />
        <p className='font-medium'>{t('This QR code has expired')}</p>
      </div>
    )
  } else if (props.order?.qr_code) {
    paymentContent = (
      <div className='flex flex-col items-center gap-4'>
        <div className='rounded-md border bg-white p-3'>
          <QRCodeSVG
            value={props.order.qr_code}
            size={224}
            level='M'
            marginSize={0}
            title={t('Alipay QR code')}
          />
        </div>
        <div className='text-center'>
          <p className='text-2xl font-semibold'>
            ¥{(props.order.amount_fen / 100).toFixed(2)}
          </p>
          <p className='text-muted-foreground mt-1 flex items-center justify-center gap-1.5 text-sm'>
            {props.refreshing && (
              <Loader2 className='h-3.5 w-3.5 animate-spin' />
            )}
            {t('Waiting for payment')}
          </p>
        </div>
      </div>
    )
  } else {
    paymentContent = (
      <Loader2 className='text-muted-foreground h-8 w-8 animate-spin' />
    )
  }

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='max-sm:w-[calc(100vw-1.5rem)] sm:max-w-sm'>
        <DialogHeader>
          <DialogTitle>{t('Scan with Alipay')}</DialogTitle>
          <DialogDescription>
            {t('Use Alipay to scan the QR code and complete payment.')}
          </DialogDescription>
        </DialogHeader>

        <div className='flex min-h-72 flex-col items-center justify-center gap-4 py-2'>
          {paymentContent}

          {props.refreshError && !paid && (
            <Button variant='outline' size='sm' onClick={props.onRefresh}>
              <RefreshCw className='h-4 w-4' />
              {t('Refresh payment status')}
            </Button>
          )}
        </div>

        <DialogFooter>
          <Button
            variant={paid ? 'default' : 'outline'}
            className='w-full'
            onClick={() => props.onOpenChange(false)}
          >
            {t('Close')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
