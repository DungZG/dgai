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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, within } from '@testing-library/react'
import { createInstance } from 'i18next'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import { beforeEach, expect, it, vi } from 'vitest'

import type { SubscriptionPlan } from '@/features/subscriptions/types'
import en from '@/i18n/locales/en.json'
import { api } from '@/lib/api'
import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

import { SubscriptionPlansCard } from '../components/subscription-plans-card'

const i18n = createInstance()

beforeEach(async () => {
  await i18n.use(initReactI18next).init({
    lng: 'en',
    fallbackLng: 'en',
    resources: { en },
    interpolation: { escapeValue: false },
  })
  useSystemConfigStore.getState().setConfig({
    currency: { ...DEFAULT_CURRENCY_CONFIG },
  })
})

it.each([
  ['Starter', 10, 750000, '$1.5'],
  ['Pro', 20, 1500000, '$3'],
  ['Business', 50, 2500000, '$5'],
  ['Enterprise', 100, 5000000, '$10'],
] as const)(
  'shows %s quota as daily usage in both the offer and purchase confirmation',
  async (title, price, quota, allowance) => {
    const plan: SubscriptionPlan = {
      id: 1,
      title,
      price_amount: price,
      currency: 'USD',
      duration_unit: 'day',
      duration_value: 30,
      enabled: true,
      sort_order: 10,
      allow_balance_pay: true,
      allow_wallet_overflow: true,
      max_purchase_per_user: 0,
      total_amount: quota,
      quota_reset_period: 'custom',
      quota_reset_custom_seconds: 86400,
    }
    vi.spyOn(api, 'get').mockImplementation(async (url) => {
      if (url === '/api/subscription/plans') {
        return { data: { success: true, data: [{ plan }] } }
      }
      if (url === '/api/subscription/self') {
        return {
          data: {
            success: true,
            data: {
              billing_preference: 'subscription_first',
              subscriptions: [],
              all_subscriptions: [],
            },
          },
        }
      }
      throw new Error(`Unexpected API request: ${url}`)
    })

    render(
      <QueryClientProvider client={new QueryClient()}>
        <I18nextProvider i18n={i18n}>
          <SubscriptionPlansCard topupInfo={null} userQuota={50000000} />
        </I18nextProvider>
      </QueryClientProvider>
    )
    expect(await screen.findByText(`Daily quota: ${allowance}`)).toBeVisible()
    expect(screen.getByText('Validity Period: 30 days')).toBeVisible()
    expect(screen.getByText('Quota Reset: Every 24 hours')).toBeVisible()
    expect(screen.queryByText(/^Total Quota:/)).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Subscribe Now' }))
    const confirmation = within(await screen.findByRole('dialog'))
    expect(confirmation.getByText('Daily quota')).toBeVisible()
    expect(confirmation.getByText(allowance)).toBeVisible()
    expect(confirmation.getByText('Every 24 hours')).toBeVisible()
    expect(
      confirmation.getByText('Unused quota does not roll over')
    ).toBeVisible()
    expect(
      confirmation.getByText(
        'Wallet may be charged after the plan quota is exhausted'
      )
    ).toBeVisible()
  }
)
