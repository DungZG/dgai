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
import { render, screen } from '@testing-library/react'
import { createInstance } from 'i18next'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import { beforeAll, expect, it } from 'vitest'

import { AffiliateRewardsCard } from '@/features/wallet/components/affiliate-rewards-card'
import en from '@/i18n/locales/en.json'

import type { ReferralInfo, UserWalletData } from '../types'

const i18n = createInstance()

beforeAll(async () => {
  await i18n.use(initReactI18next).init({
    lng: 'en',
    fallbackLng: 'en',
    resources: { en: { translation: en.translation } },
  })
})

const walletUser = {
  aff_quota: 0,
  aff_history_quota: 0,
  aff_count: 2,
} as UserWalletData

function renderCard(referral: ReferralInfo | null) {
  return render(
    <I18nextProvider i18n={i18n}>
      <AffiliateRewardsCard
        user={walletUser}
        affiliateLink='https://dgai.io.vn/register?aff=ab12'
        referral={referral}
        onTransfer={() => undefined}
      />
    </I18nextProvider>
  )
}

it('shows the commission rate the operator configured', () => {
  renderCard({
    aff_code: 'ab12',
    aff_count: 2,
    aff_quota: 0,
    aff_history_quota: 0,
    commission_percent: 10,
    events: [],
  })

  expect(screen.getByText(/10% of every top-up/i)).toBeVisible()
})

it('lists the newest reward events with their localised text', () => {
  renderCard({
    aff_code: 'ab12',
    aff_count: 2,
    aff_quota: 0,
    aff_history_quota: 0,
    commission_percent: 10,
    events: [
      {
        id: 7,
        created_at: 1_760_000_000,
        content: 'Referral top-up commission 1.00 from dung',
        other: {
          content_parts: [
            {
              key: 'Referral top-up commission {{quota}} from {{username}}',
              params: { quota: '1.00', username: 'dung' },
            },
          ],
        },
      },
    ],
  })

  expect(screen.getByText('Recent reward events')).toBeVisible()
  expect(
    screen.getByText('Referral top-up commission 1.00 from dung')
  ).toBeVisible()
})

it('falls back to the plain notice when the rate is disabled', () => {
  renderCard({
    aff_code: 'ab12',
    aff_count: 0,
    aff_quota: 0,
    aff_history_quota: 0,
    commission_percent: 0,
    events: [],
  })

  expect(
    screen.getByText(/Earn rewards when users join through your referral link/i)
  ).toBeVisible()
  expect(screen.queryByText(/Recent reward events/i)).not.toBeInTheDocument()
})
