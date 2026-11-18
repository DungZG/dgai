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
import { fireEvent, render, screen } from '@testing-library/react'
import { createInstance } from 'i18next'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import { beforeEach, expect, it, vi } from 'vitest'

import { CryptomusDialog } from '@/features/wallet/components/dialogs/cryptomus-dialog'
import en from '@/i18n/locales/en.json'

import type { CryptomusOrder } from '../types'

const i18n = createInstance()

const pendingOrder: CryptomusOrder = {
  trade_no: 'CRY9F2K4Q7L1M8N3P5R6S7T8U9V0W1X2Y',
  status: 'pending',
  provider_status: 'check',
  amount_usd: '10.43',
  currency: 'USDT',
  network: 'tron',
  base_quota: 5000000,
  bonus_quota: 0,
  expires_at: 1790000000,
  needs_review: false,
}

beforeEach(async () => {
  await i18n.use(initReactI18next).init({
    lng: 'en',
    fallbackLng: false,
    nsSeparator: false,
    resources: { en },
    interpolation: { escapeValue: false },
  })
})

function renderDialog(overrides: Partial<CryptomusOrder> = {}) {
  const onCheckStatus = vi.fn()
  render(
    <I18nextProvider i18n={i18n}>
      <CryptomusDialog
        open
        onOpenChange={vi.fn()}
        invoiceUsd={10}
        order={{ ...pendingOrder, ...overrides }}
        creating={false}
        checking={false}
        error={null}
        onConfirm={vi.fn()}
        onCheckStatus={onCheckStatus}
      />
    </I18nextProvider>
  )
  return { onCheckStatus }
}

it('shows the USD invoice from the order instead of the wallet display amount', () => {
  renderDialog()

  expect(screen.getByText('$10.43')).toBeVisible()
  expect(screen.getByText('CRY9F2K4Q7L1M8N3P5R6S7T8U9V0W1X2Y')).toBeVisible()
  expect(screen.getByText('tron')).toBeVisible()
})

it('asks for a status check when the provider has no checkout link yet', () => {
  const { onCheckStatus } = renderDialog()

  expect(
    screen.getByText(
      'The payment page is not ready yet. Check the status again in a moment instead of creating another order.'
    )
  ).toBeVisible()
  expect(screen.queryByRole('link')).toBe(null)

  fireEvent.click(screen.getByRole('button', { name: 'Check status' }))
  expect(onCheckStatus).toHaveBeenCalledTimes(1)
})

it('links to the hosted checkout page and retries status only', () => {
  renderDialog({ checkout_url: 'https://pay.cryptomus.com/pay/abc123' })

  const link = screen.getByRole('link', { name: 'Open checkout' })
  expect(link).toHaveAttribute('href', 'https://pay.cryptomus.com/pay/abc123')
  expect(link).toHaveAttribute('target', '_blank')
  expect(link).toHaveAttribute('rel', 'noopener noreferrer')
  expect(screen.getByRole('button', { name: 'Check status' })).toBeEnabled()
})

it('refuses a checkout link that is not a plain https URL', () => {
  renderDialog({ checkout_url: 'javascript:alert(1)' })

  expect(screen.queryByRole('link')).toBe(null)
  expect(
    screen.getByText(
      'The payment page is not ready yet. Check the status again in a moment instead of creating another order.'
    )
  ).toBeVisible()
})

it('shows a create failure inline without creating another order', () => {
  const onConfirm = vi.fn()
  render(
    <I18nextProvider i18n={i18n}>
      <CryptomusDialog
        open
        onOpenChange={vi.fn()}
        invoiceUsd={10}
        order={null}
        creating={false}
        checking={false}
        error='Payment request failed'
        onConfirm={onConfirm}
        onCheckStatus={vi.fn()}
      />
    </I18nextProvider>
  )

  expect(screen.getByText('Payment request failed')).toBeVisible()
  expect(onConfirm).not.toHaveBeenCalled()
  expect(screen.getByRole('button', { name: 'Pay with USDT' })).toBeEnabled()
})

it('stops asking for status once the provider confirmed the payment', () => {
  renderDialog({ status: 'success' })

  expect(
    screen.getByText('Payment confirmed. Your balance has been credited.')
  ).toBeVisible()
  expect(screen.queryByRole('button', { name: 'Check status' })).toBe(null)
})
