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

import { Footer } from '../footer'

vi.mock('@tanstack/react-router', () => ({
  Link: ({
    to,
    children,
    ...props
  }: React.AnchorHTMLAttributes<HTMLAnchorElement> & {
    to: string
    children?: React.ReactNode
  }) => (
    <a href={to} {...props}>
      {children}
    </a>
  ),
}))

vi.mock('@/hooks/use-status', () => ({
  useStatus: () => ({ status: {} }),
}))

vi.mock('@/hooks/use-system-config', () => ({
  useSystemConfig: () => ({
    systemName: 'DGAI',
    footerHtml: '',
    demoSiteEnabled: true,
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

it('shows public customer support and a clickable email in the footer', () => {
  render(
    <I18nextProvider i18n={i18n}>
      <Footer />
    </I18nextProvider>
  )

  expect(screen.getByRole('contentinfo')).toBeVisible()
  expect(
    screen.getByRole('heading', { name: 'Customer Support' })
  ).toBeVisible()
  expect(
    screen.getByRole('link', { name: 'tridung4323@gmail.com' })
  ).toHaveAttribute('href', 'mailto:tridung4323@gmail.com')
  expect(
    screen.getByRole('link', { name: 'View contact details' })
  ).toHaveAttribute('href', '/contact')
})
