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
import { act, render, screen } from '@testing-library/react'
import { createInstance } from 'i18next'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

import { VietQRDialog } from '@/features/wallet/components/dialogs/vietqr-dialog'
import en from '@/i18n/locales/en.json'
import viLocale from '@/i18n/locales/vi.json'

import type { VietQRPaymentDetails } from '../types'

const i18n = createInstance()

const details: VietQRPaymentDetails = {
  trade_no: 'VQR11XF4LRX',
  amount: 5,
  pay_money: 5,
  memo: 'DGAIVQR11XF4LRX',
  vnd_amount: 132500,
  vnd_rate: 26500,
  qr_image_url: 'https://img.vietqr.io/image/TPB-10006460824-compact2.png',
  bank_code: 'TPB',
  bank_account: '10006460824',
  account_name: 'NGUYEN TRI DUNG',
  expires_at: 0,
}

beforeEach(async () => {
  await i18n.use(initReactI18next).init({
    lng: 'vi',
    fallbackLng: false,
    nsSeparator: false,
    resources: { en, vi: viLocale },
    interpolation: { escapeValue: false },
  })
  vi.useFakeTimers({ shouldAdvanceTime: false })
  vi.setSystemTime(new Date('2026-11-10T12:00:00Z'))
})

afterEach(() => {
  vi.useRealTimers()
})

function renderDialog(overrides: Partial<VietQRPaymentDetails> = {}) {
  return render(
    <I18nextProvider i18n={i18n}>
      <VietQRDialog
        open
        onOpenChange={vi.fn()}
        details={{ ...details, ...overrides }}
      />
    </I18nextProvider>
  )
}

it('counts the 30-minute transfer window down every second', () => {
  const now = Math.floor(Date.now() / 1000)
  renderDialog({ expires_at: now + 30 * 60 })

  expect(screen.getByRole('timer')).toHaveTextContent('Thời gian còn lại')
  expect(screen.getByRole('timer')).toHaveTextContent('30:00')

  act(() => {
    vi.advanceTimersByTime(1000)
  })
  expect(screen.getByRole('timer')).toHaveTextContent('29:59')

  act(() => {
    vi.advanceTimersByTime(59 * 1000)
  })
  expect(screen.getByRole('timer')).toHaveTextContent('29:00')
})

it('warns when the code has expired and stops the countdown', () => {
  const now = Math.floor(Date.now() / 1000)
  renderDialog({ expires_at: now + 2 })

  act(() => {
    vi.advanceTimersByTime(3000)
  })

  expect(screen.getByRole('timer')).toHaveTextContent('Hết hạn')
  expect(screen.getByRole('timer')).toHaveTextContent('00:00')
  expect(
    screen.getByText(
      'Mã thanh toán này đã hết hạn. Hãy tạo lần nạp mới để nhận mã mới.'
    )
  ).toBeVisible()

  act(() => {
    vi.advanceTimersByTime(60 * 1000)
  })
  expect(screen.getByRole('timer')).toHaveTextContent('00:00')
})

it('hides the countdown when the order carries no deadline', () => {
  renderDialog({ expires_at: 0 })
  expect(screen.queryByRole('timer')).not.toBeInTheDocument()
})
