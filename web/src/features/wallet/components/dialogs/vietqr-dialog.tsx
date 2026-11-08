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
import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

import type { VietQRPaymentDetails } from '../../types'

interface VietQRDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  details: VietQRPaymentDetails | null
  loading?: boolean
}

export function VietQRDialog(props: VietQRDialogProps) {
  const { t } = useTranslation()

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
            <div className='flex justify-center'>
              <img
                src={props.details.qr_image_url}
                alt={t('VietQR payment code')}
                className='border-border/50 w-full max-w-[280px] rounded-xl border bg-white p-2'
              />
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
              {t(
                'Keep the transfer content exactly as shown so the payment can be matched to your account. The balance is credited after the bank confirms the transaction.'
              )}
            </p>
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}

function Row(props: { label: string; children: React.ReactNode }) {
  return (
    <div className='border-border/40 flex items-center justify-between border-b pb-2 last:border-b-0'>
      <dt className='text-muted-foreground'>{props.label}</dt>
      <dd>{props.children}</dd>
    </div>
  )
}
