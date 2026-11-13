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
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import { beforeAll, expect, it, vi } from 'vitest'

import {
  ReferralEvents,
  ReferralHowItWorks,
} from '@/features/referral/components/referral-events'
import {
  ReferralHero,
  ReferralStats,
} from '@/features/referral/components/referral-hero'
import en from '@/i18n/locales/en.json'

import type { ReferralInfo } from '@/features/wallet/types'

const transferQuota = vi.fn(async () => true)

vi.mock('@/features/wallet/hooks', () => ({
  useAffiliate: () => ({
    affiliateLink: 'https://dgai.io.vn/register?aff=ab12',
    referral: {
      aff_code: 'ab12',
      aff_count: 7,
      aff_quota: 500_000,
      aff_history_quota: 2_000_000,
      commission_percent: 30,
      events: [],
    },
    loading: false,
    transferring: false,
    transferQuota,
  }),
  useTopupInfo: () => ({
    topupInfo: { payment_compliance_confirmed: true },
    loading: false,
  }),
}))

const i18n = createInstance()

beforeAll(async () => {
  await i18n.use(initReactI18next).init({
    lng: 'en',
    fallbackLng: 'en',
    resources: { en: { translation: en.translation } },
  })
})

const referral: ReferralInfo = {
  aff_code: 'ab12',
  aff_count: 7,
  aff_quota: 500_000,
  aff_history_quota: 2_000_000,
  commission_percent: 30,
  events: [
    {
      id: 11,
      created_at: 1_760_000_000,
      content: 'Referral top-up commission 0.50 from dung',
      other: {
        content_parts: [
          {
            key: 'Referral top-up commission {{quota}} from {{username}}',
            params: { quota: '0.50', username: 'dung' },
          },
        ],
      },
    },
  ],
}

function wrap(ui: React.ReactNode) {
  return render(<I18nextProvider i18n={i18n}>{ui}</I18nextProvider>)
}

it('states the commission rate and shows the invite link', () => {
  wrap(
    <ReferralHero
      referral={referral}
      affiliateLink='https://dgai.io.vn/register?aff=ab12'
      onTransfer={() => undefined}
    />
  )

  expect(screen.getByText(/earn 30% of every top-up/i)).toBeVisible()
  expect(screen.getByLabelText('Your referral link')).toHaveValue(
    'https://dgai.io.vn/register?aff=ab12'
  )
})

it('offers the transfer only when rewards are pending', () => {
  const onTransfer = vi.fn()
  const { rerender } = wrap(
    <ReferralHero
      referral={{ ...referral, aff_quota: 0 }}
      affiliateLink='link'
      onTransfer={onTransfer}
    />
  )
  expect(
    screen.queryByRole('button', { name: 'Transfer to Balance' })
  ).not.toBeInTheDocument()

  rerender(
    <I18nextProvider i18n={i18n}>
      <ReferralHero
        referral={referral}
        affiliateLink='link'
        onTransfer={onTransfer}
      />
    </I18nextProvider>
  )
  expect(
    screen.getByRole('button', { name: 'Transfer to Balance' })
  ).toBeVisible()
})

it('disables the transfer while compliance is unconfirmed', () => {
  wrap(
    <ReferralHero
      referral={referral}
      affiliateLink='link'
      complianceConfirmed={false}
      onTransfer={() => undefined}
    />
  )

  expect(
    screen.getByRole('button', { name: 'Transfer to Balance' })
  ).toBeDisabled()
})

it('reports invites, lifetime earnings and the commission rate', () => {
  wrap(<ReferralStats referral={referral} />)

  expect(screen.getByText('Invites')).toBeVisible()
  expect(screen.getByText('7')).toBeVisible()
  expect(screen.getByText('Total Earned')).toBeVisible()
  expect(screen.getByText('Commission')).toBeVisible()
  expect(screen.getByText('30%')).toBeVisible()
  // Pending lives in the hero next to the transfer button, not here.
  expect(screen.queryByText('Pending')).not.toBeInTheDocument()
})

it('renders reward events in the viewer language', () => {
  wrap(<ReferralEvents events={referral.events} />)

  expect(screen.getByText('Reward history')).toBeVisible()
  expect(
    screen.getByText('Referral top-up commission 0.50 from dung')
  ).toBeVisible()
})

it('shows an empty state before the first reward', () => {
  wrap(<ReferralEvents events={[]} />)

  expect(screen.getByText('No reward events yet')).toBeVisible()
})

it('explains the commission in the third step', () => {
  wrap(<ReferralHowItWorks percent={30} />)

  expect(screen.getByText('How it works')).toBeVisible()
  expect(
    screen.getByText('You earn 30% of every top-up')
  ).toBeVisible()
})

it('does not claim a commission rate when none is configured', () => {
  wrap(<ReferralHowItWorks percent={0} />)

  expect(
    screen.getByText('You earn a reward for every signup')
  ).toBeVisible()
})

it('copies the invite link from the hero', async () => {
  const user = userEvent.setup()
  wrap(
    <ReferralHero
      referral={referral}
      affiliateLink='https://dgai.io.vn/register?aff=ab12'
      onTransfer={() => undefined}
    />
  )

  await user.click(screen.getByRole('button', { name: 'Copy referral link' }))
  await expect(navigator.clipboard.readText()).resolves.toBe(
    'https://dgai.io.vn/register?aff=ab12'
  )
})

it('renders the referral page with its sections in place', async () => {
  const { Referral } = await import('@/features/referral')
  wrap(<Referral />)

  expect(
    screen.getByRole('heading', { name: 'Referral Program' })
  ).toBeVisible()
  expect(screen.getByLabelText('Your referral link')).toBeVisible()
  expect(screen.getByText('How it works')).toBeVisible()
  expect(screen.getByText('Reward history')).toBeVisible()
})
