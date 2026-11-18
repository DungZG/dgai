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

import en from '@/i18n/locales/en.json'

import { RechargeFormCard } from '../components/recharge-form-card'
import type { TopupInfo } from '../types'

const i18n = createInstance()

const cryptoInfo: TopupInfo = {
  enable_online_topup: true,
  enable_stripe_topup: false,
  pay_methods: [],
  min_topup: 1,
  stripe_min_topup: 1,
  amount_options: [],
  discount: {},
  enable_crypto_topup: true,
  crypto_min_topup: 10,
  crypto_max_topup: 100,
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

function renderCard(
  topupInfo: TopupInfo,
  topupAmount: number,
  onPaymentMethodSelect = vi.fn()
) {
  render(
    <I18nextProvider i18n={i18n}>
      <RechargeFormCard
        topupInfo={topupInfo}
        presetAmounts={[]}
        selectedPreset={null}
        topupAmount={topupAmount}
        paymentAmount={topupAmount}
        calculating={false}
        paymentLoading={null}
        redemptionCode=''
        redeeming={false}
        onSelectPreset={vi.fn()}
        onTopupAmountChange={vi.fn()}
        onPaymentMethodSelect={onPaymentMethodSelect}
        onRedemptionCodeChange={vi.fn()}
        onRedeem={vi.fn()}
      />
    </I18nextProvider>
  )
  return { onPaymentMethodSelect }
}

it('disables the USDT method below the crypto USD floor', () => {
  renderCard(cryptoInfo, 9)

  const button = screen.getByRole('button', { name: /^USDT\./ })
  expect(button).toBeDisabled()
  expect(button).toHaveAccessibleName('USDT. Minimum topup amount: $10')
})

it('disables the USDT method above the crypto USD ceiling', () => {
  renderCard(cryptoInfo, 101)

  const button = screen.getByRole('button', { name: /^USDT\./ })
  expect(button).toBeDisabled()
  expect(button).toHaveAccessibleName('USDT. Maximum topup amount: $100')
})

it('opens the crypto flow with the invoice when the amount is inside the bounds', () => {
  const { onPaymentMethodSelect } = renderCard(cryptoInfo, 20)

  const button = screen.getByRole('button', { name: 'USDT $20' })
  expect(button).toBeEnabled()
  fireEvent.click(button)
  expect(onPaymentMethodSelect).toHaveBeenCalledWith({
    name: 'USDT',
    type: 'cryptomus',
  })
})

it('never renders a stale cryptomus entry from pay_methods', () => {
  renderCard(
    {
      ...cryptoInfo,
      enable_crypto_topup: false,
      pay_methods: [{ name: 'Crypto', type: 'cryptomus' }],
    },
    20
  )

  expect(screen.queryByRole('button', { name: /Crypto|USDT/ })).toBe(null)
  expect(
    screen.getByText('No payment methods available. Please contact administrator.')
  ).toBeVisible()
})
