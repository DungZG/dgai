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
import { beforeAll, expect, it, vi } from 'vitest'

import en from '@/i18n/locales/en.json'

import { AboutIntro } from '../components/about-intro'

vi.mock('@tanstack/react-router', () => ({
  Link: (props: { to: string; children?: React.ReactNode }) => (
    <a href={props.to}>{props.children}</a>
  ),
}))

vi.mock('@/hooks/use-status', () => ({
  useStatus: () => ({ status: { docs_link: '/docs' }, loading: false }),
}))

vi.mock('@/stores/auth-store', () => ({
  useAuthStore: (selector: (state: unknown) => unknown) =>
    selector({ auth: { user: null } }),
}))

vi.mock('@/features/pricing/hooks/use-pricing-data', () => ({
  usePricingData: () => ({
    models: [
      {
        id: 1,
        model_name: 'gpt-6.1-sol',
        quota_type: 0,
        model_ratio: 0.026954,
        completion_ratio: 6,
        vendor_name: 'OpenAI',
        enable_groups: ['default'],
      },
      {
        id: 2,
        model_name: 'deepseek-pro-v4',
        quota_type: 1,
        model_ratio: 0,
        completion_ratio: 0,
        model_price: 0.001078,
        vendor_name: 'DeepSeek',
        enable_groups: ['default'],
      },
    ],
    isLoading: false,
  }),
}))

vi.mock('@/features/pricing/lib', async (importOriginal) => {
  const actual = await importOriginal<Record<string, unknown>>()
  return {
    ...actual,
    formatPrice: () => '$0.0539',
    formatRequestPrice: () => '$0.0011',
  }
})

const i18n = createInstance()

beforeAll(async () => {
  await i18n.use(initReactI18next).init({
    lng: 'en',
    fallbackLng: 'en',
    resources: { en: { translation: en.translation } },
  })
})

function renderIntro() {
  return render(
    <I18nextProvider i18n={i18n}>
      <AboutIntro />
    </I18nextProvider>
  )
}

it('counts the models that are actually on sale', () => {
  renderIntro()

  expect(screen.getByText('One key, 2 models, one endpoint')).toBeVisible()
  expect(screen.getByText('Models available')).toBeVisible()
  expect(screen.getByText('Providers')).toBeVisible()
})

it('lists the models with their prices', () => {
  renderIntro()

  expect(screen.getByText('gpt-6.1-sol')).toBeVisible()
  expect(screen.getByText('deepseek-pro-v4')).toBeVisible()
  expect(screen.getByText(/\$0\.0539/)).toBeVisible()
  expect(screen.getByText(/\$0\.0011 per request/)).toBeVisible()
})

it('offers signup to visitors and the console to signed-in users', () => {
  renderIntro()
  expect(screen.getByRole('link', { name: /get started/i })).toHaveAttribute(
    'href',
    '/sign-up'
  )
})

it('explains the three onboarding steps', () => {
  renderIntro()

  expect(screen.getByText('How it works')).toBeVisible()
  expect(screen.getByText('Create an account and an API key')).toBeVisible()
  expect(screen.getByText('Top up your balance')).toBeVisible()
  expect(screen.getByText('Point your client at one endpoint')).toBeVisible()
})
