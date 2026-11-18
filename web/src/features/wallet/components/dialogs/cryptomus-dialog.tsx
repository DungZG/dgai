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
import { ExternalLink, Loader2, RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { StatusBadge } from '@/components/status-badge'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { formatTimestamp } from '@/lib/format'

import { getStatusConfig } from '../../lib/billing'
import { formatUsdInvoice } from '../../lib/format'
import type { CryptomusOrder } from '../../types'

interface CryptomusDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** Invoice the gateway will charge, in USD */
  invoiceUsd: number
  order: CryptomusOrder | null
  creating: boolean
  checking: boolean
  onConfirm: () => void
  onCheckStatus: () => void
  recovering?: boolean
  error: string | null
}

/** Checkout links must be https and credential-free before they reach the DOM. */
function safeCheckoutUrl(raw: string | undefined): string | null {
  if (!raw) return null
  try {
    const url = new URL(raw)
    if (url.protocol !== 'https:' || url.username || url.password) return null
    return url.toString()
  } catch {
    return null
  }
}

export function CryptomusDialog(props: CryptomusDialogProps) {
  const { t } = useTranslation()
  const order = props.order
  const checkoutUrl = safeCheckoutUrl(order?.checkout_url)
  const invoiceUsd =
    order && Number.isFinite(Number(order.amount_usd))
      ? Number(order.amount_usd)
      : props.invoiceUsd
  const status = order ? getStatusConfig(order.status) : null
  const pending = order?.status === 'pending'

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='sm:max-w-md'>
        <DialogHeader>
          <DialogTitle>{t('Pay with USDT')}</DialogTitle>
          <DialogDescription>
            {t('Pay with USDT through the Cryptomus checkout page.')}
          </DialogDescription>

        </DialogHeader>
        <Alert>
          <AlertDescription>
            {t(
              'The invoice includes the merchant fee. Your wallet or network withdrawal fee is charged separately.'
            )}
          </AlertDescription>
        </Alert>

        <div className='space-y-4'>
          {props.recovering && !order ? (
            <>
              <Alert>
                <AlertDescription>{t('Loading')}</AlertDescription>
              </Alert>
              {props.error ? (
                <Alert variant='destructive'>
                  <AlertDescription>{props.error}</AlertDescription>
                </Alert>
              ) : null}
            </>
          ) : null}
          {!props.recovering || order ? (
            <div className='space-y-4'>
              {props.error ? (
                <Alert variant='destructive'>
                  <AlertDescription>{props.error}</AlertDescription>
                </Alert>
              ) : null}

              <div className='flex items-center justify-between gap-3'>
                <span className='text-muted-foreground text-sm'>
                  {t('Amount to pay:')}
                </span>
                <span className='flex items-baseline gap-1'>
                  <span className='text-lg font-semibold'>
                    {formatUsdInvoice(invoiceUsd)}
                  </span>
                  <span className='text-muted-foreground text-sm font-normal'>
                    USDT
                  </span>
                </span>
              </div>

              {order ? (
                <>
                  <div className='flex items-center justify-between gap-3'>
                    <span className='text-muted-foreground text-sm'>
                      {t('Order number')}
                    </span>
                    <span className='min-w-0 font-mono text-xs break-all'>
                      {order.trade_no}
                    </span>
                  </div>
                  {order.network ? (
                    <div className='flex items-center justify-between gap-3'>
                      <span className='text-muted-foreground text-sm'>
                        {t('Network')}
                      </span>
                      <span className='text-sm font-medium'>
                        {order.network}
                      </span>
                    </div>
                  ) : null}
                  <div className='flex items-center justify-between gap-3'>
                    <span className='text-muted-foreground text-sm'>
                      {t('Expires at')}
                    </span>
                    <span className='text-sm'>
                      {formatTimestamp(order.expires_at)}
                    </span>
                  </div>
                  {status ? (
                    <div className='border-t pt-4'>
                      <StatusBadge
                        label={t(status.label)}
                        variant={status.variant}
                        copyable={false}
                      />
                    </div>
                  ) : null}

                  {order.status === 'success' ? (
                    <Alert>
                      <AlertDescription>
                        {t('Payment confirmed. Your balance has been credited.')}
                      </AlertDescription>
                    </Alert>
                  ) : null}
                  {order.status === 'expired' ? (
                    <Alert>
                      <AlertDescription>
                        {t('This payment order has expired.')}
                      </AlertDescription>
                    </Alert>
                  ) : null}
                  {order.status === 'failed' ? (
                    <Alert variant='destructive'>
                      <AlertDescription>{t('Failed')}</AlertDescription>
                    </Alert>
                  ) : null}
                  {order.needs_review ? (
                    <Alert>
                      <AlertDescription>
                        {t('This payment is waiting for manual review.')}
                      </AlertDescription>
                    </Alert>
                  ) : null}
                  {pending && !checkoutUrl ? (
                    <Alert>
                      <AlertDescription>
                        {t(
                          'The payment page is not ready yet. Check the status again in a moment instead of creating another order.'
                        )}
                      </AlertDescription>
                    </Alert>
                  ) : null}
                </>
              ) : null}
            </div>
          ) : null}
        </div>

        <DialogFooter>
          {props.recovering && !order ? (
            <>
              <Button
                variant='outline'
                onClick={props.onCheckStatus}
                disabled={props.checking}
              >
                {props.checking ? (
                  <Loader2 className='h-4 w-4 animate-spin' />
                ) : (
                  <RefreshCw className='h-4 w-4' />
                )}
                {t('Check status')}
              </Button>
              <Button variant='ghost' onClick={() => props.onOpenChange(false)}>
                {t('Close')}
              </Button>
            </>
          ) : null}
          {order && pending ? (
            <>
              <Button
                variant='outline'
                onClick={props.onCheckStatus}
                disabled={props.checking}
              >
                {props.checking ? (
                  <Loader2 className='h-4 w-4 animate-spin' />
                ) : (
                  <RefreshCw className='h-4 w-4' />
                )}
                {t('Check status')}
              </Button>
              {checkoutUrl ? (
                <Button
                  nativeButton={false}
                  role='link'
                  render={
                    <a
                      href={checkoutUrl}
                      target='_blank'
                      rel='noopener noreferrer'
                    />
                  }
                >
                  <ExternalLink className='h-4 w-4' />
                  {t('Open checkout')}
                </Button>
              ) : null}
              <Button variant='ghost' onClick={() => props.onOpenChange(false)}>
                {t('Close')}
              </Button>
            </>
          ) : null}
          {order && !pending ? (
            <Button onClick={() => props.onOpenChange(false)}>
              {t('Close')}
            </Button>
          ) : null}
          {!order && !props.recovering ? (
            <>
              <Button
                variant='outline'
                onClick={() => props.onOpenChange(false)}
                disabled={props.creating}
              >
                {t('Cancel')}
              </Button>
              <Button onClick={props.onConfirm} disabled={props.creating}>
                {props.creating ? (
                  <Loader2 className='h-4 w-4 animate-spin' />
                ) : null}
                {t('Pay with USDT')}
              </Button>
            </>
          ) : null}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
