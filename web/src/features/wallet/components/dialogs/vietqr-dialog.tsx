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
import { CheckCircle2, Clock, Loader2 } from 'lucide-react'
import { QRCodeSVG } from 'qrcode.react'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { CopyButton } from '@/components/copy-button'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { handleServerError } from '@/lib/handle-server-error'
import { cn } from '@/lib/utils'

import { confirmVietQRPayment } from '../../api'
import type { VietQRPaymentDetails } from '../../types'

interface VietQRDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  details: VietQRPaymentDetails | null
  loading?: boolean
  onPaid?: () => void
}

export function VietQRDialog(props: VietQRDialogProps) {
  const { t } = useTranslation()
  const [confirming, setConfirming] = useState(false)
  const [paid, setPaid] = useState(false)
  const expiresAt = props.details?.expires_at ?? 0
  const secondsLeft = useSecondsLeft(expiresAt, props.open)
  const expired = expiresAt > 0 && secondsLeft === 0
  const urgent = !expired && secondsLeft > 0 && secondsLeft <= 5 * 60
  let timerTone = 'border-border/60 bg-muted/30'
  if (expired) {
    timerTone = 'border-destructive/40 bg-destructive/5 text-destructive'
  } else if (urgent) {
    timerTone =
      'border-amber-500/40 bg-amber-500/5 text-amber-600 dark:text-amber-500'
  }

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='sm:max-w-md'>
        <DialogHeader>
          <DialogTitle>{t('Transfer to complete the top-up')}</DialogTitle>
          <DialogDescription>
            {t(
              'Scan the QR code with your banking app, or transfer manually using the account details below.'
            )}
          </DialogDescription>
        </DialogHeader>

        {props.loading || !props.details ? (
          <div className='flex h-64 items-center justify-center'>
            <Loader2 className='text-muted-foreground size-6 animate-spin' />
          </div>
        ) : (
          <div className='space-y-4'>
            {expiresAt > 0 && (
              <div
                className={cn(
                  'flex items-center justify-between gap-3 rounded-lg border px-3 py-2 text-sm',
                  timerTone
                )}
                role='timer'
                aria-live='polite'
              >
                <span className='flex items-center gap-1.5'>
                  <Clock className='size-4' aria-hidden='true' />
                  {expired ? t('Expired') : t('Time remaining')}
                </span>
                <span className='font-mono font-semibold tabular-nums'>
                  {expired ? '00:00' : formatCountdown(secondsLeft)}
                </span>
              </div>
            )}

            <div className='flex justify-center'>
              {props.details.qr_content ? (
                <div
                  className={cn(
                    'border-border/50 rounded-xl border bg-white p-3',
                    expired && 'opacity-40'
                  )}
                >
                  <QRCodeSVG
                    value={props.details.qr_content}
                    size={240}
                    level='M'
                    aria-label={t('VietQR payment code')}
                  />
                </div>
              ) : (
                <img
                  src={props.details.qr_image_url}
                  alt={t('VietQR payment code')}
                  className={cn(
                    'border-border/50 w-full max-w-[280px] rounded-xl border bg-white p-2',
                    expired && 'opacity-40'
                  )}
                />
              )}
            </div>

            <dl className='space-y-2 text-sm'>
              <Row label={t('Amount')}>
                <span className='font-medium'>
                  {new Intl.NumberFormat('vi-VN').format(
                    props.details.vnd_amount
                  )}{' '}
                  VND
                </span>
              </Row>
              <Row label={t('Bank')}>
                <span className='font-medium'>{props.details.bank_code}</span>
              </Row>
              <Row label={t('Account number')}>
                <span className='flex items-center gap-1.5 font-mono font-medium'>
                  {props.details.bank_account}
                  <CopyButton
                    value={props.details.bank_account}
                    variant='ghost'
                    size='icon'
                    className='size-6'
                  />
                </span>
              </Row>
              {props.details.account_name && (
                <Row label={t('Account holder')}>
                  <span className='font-medium'>
                    {props.details.account_name}
                  </span>
                </Row>
              )}
              <Row label={t('Transfer content')}>
                <span className='flex items-center gap-1.5 font-mono font-medium'>
                  {props.details.memo}
                  <CopyButton
                    value={props.details.memo}
                    variant='ghost'
                    size='icon'
                    className='size-6'
                  />
                </span>
              </Row>
            </dl>

            <p className='text-muted-foreground text-xs leading-relaxed'>
              {expired
                ? t(
                    'This payment code has expired. Create a new top-up to get a fresh code.'
                  )
                : t(
                    'Keep the transfer content exactly as shown so the payment can be matched to your account. The balance is credited after the bank confirms the transaction.'
                  )}
            </p>

            {paid ? (
              <div className='flex items-center justify-center gap-2 rounded-lg border border-green-600/40 bg-green-600/5 px-3 py-2 text-sm text-green-700 dark:text-green-500'>
                <CheckCircle2 className='size-4' aria-hidden='true' />
                {t('Payment confirmed. Your balance has been credited.')}
              </div>
            ) : (
              <Button
                className='w-full'
                disabled={confirming}
                onClick={async () => {
                  if (!props.details) return
                  setConfirming(true)
                  try {
                    const result = await confirmVietQRPayment(
                      props.details.trade_no
                    )
                    if (result.status === 'success') {
                      setPaid(true)
                      toast.success(t('Top-up credited'))
                      props.onPaid?.()
                    } else if (result.reason === 'sandbox') {
                      toast.warning(
                        t(
                          'This gateway is running VietQR in test mode, so a real transfer cannot be verified automatically. An administrator will credit it manually.'
                        )
                      )
                    } else {
                      toast.info(
                        t(
                          'We have not received your transfer yet. Wait a moment and check again.'
                        )
                      )
                    }
                  } catch (error) {
                    handleServerError(error)
                  } finally {
                    setConfirming(false)
                  }
                }}
              >
                {confirming && <Loader2 className='me-2 size-4 animate-spin' />}
                {t('I have paid — check my payment')}
              </Button>
            )}
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}

/**
 * Counts down to a unix deadline while the dialog is open. Recomputes from the
 * clock each tick so a sleeping tab or a resumed device does not drift.
 */
function useSecondsLeft(expiresAt: number, open: boolean): number {
  const [now, setNow] = useState(() => Math.floor(Date.now() / 1000))

  useEffect(() => {
    if (!open || !expiresAt) return
    setNow(Math.floor(Date.now() / 1000))
    const timer = window.setInterval(() => {
      setNow(Math.floor(Date.now() / 1000))
    }, 1000)
    return () => window.clearInterval(timer)
  }, [open, expiresAt])

  return useMemo(
    () => (expiresAt ? Math.max(0, expiresAt - now) : 0),
    [expiresAt, now]
  )
}

function formatCountdown(totalSeconds: number): string {
  const minutes = Math.floor(totalSeconds / 60)
  const seconds = totalSeconds % 60
  return `${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`
}

function Row(props: { label: string; children: React.ReactNode }) {
  return (
    <div className='border-border/40 flex items-center justify-between border-b pb-2 last:border-b-0'>
      <dt className='text-muted-foreground'>{props.label}</dt>
      <dd>{props.children}</dd>
    </div>
  )
}
