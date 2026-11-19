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
import { afterAll, beforeAll, expect, it, vi } from 'vitest'

import en from '@/i18n/locales/en.json'
import viLocale from '@/i18n/locales/vi.json'

import { CONTACT_PAGE_DESCRIPTION, CONTACT_PAGE_TITLE } from '../constants'
import { Contact } from '../index'

vi.mock('@/components/layout', () => ({
  PublicLayout: ({ children }: { children: React.ReactNode }) => children,
}))

vi.mock('@/components/layout/components/footer', () => ({
  Footer: () => null,
}))

const i18n = createInstance()

beforeAll(async () => {
  const meta = document.createElement('meta')
  meta.setAttribute('name', 'description')
  meta.content = 'Original site description'
  document.head.appendChild(meta)

  await i18n.use(initReactI18next).init({
    lng: 'en',
    fallbackLng: 'en',
    resources: {
      en: { translation: en.translation },
      vi: { translation: viLocale.translation },
    },
  })
})

afterAll(() => {
  document.querySelector('meta[name="description"]')?.remove()
})

function renderContact() {
  return render(
    <I18nextProvider i18n={i18n}>
      <Contact />
    </I18nextProvider>
  )
}

it('shows DGAI support contacts and public mailto and website links', () => {
  renderContact()

  expect(
    screen.getByRole('heading', { level: 1, name: 'Contact Us' })
  ).toBeVisible()
  expect(document.title).toBe(CONTACT_PAGE_TITLE)
  expect(document.querySelector('meta[name="description"]')).toHaveAttribute(
    'content',
    CONTACT_PAGE_DESCRIPTION
  )
  expect(screen.getByText('DGAI')).toBeVisible()

  const emailLinks = screen.getAllByRole('link', {
    name: 'tridung4323@gmail.com',
  })
  expect(emailLinks).toHaveLength(2)
  for (const link of emailLinks) {
    expect(link).toHaveAttribute('href', 'mailto:tridung4323@gmail.com')
  }

  expect(
    screen.getByRole('link', { name: 'https://dgai.io.vn' })
  ).toHaveAttribute('href', 'https://dgai.io.vn')
})

it('lists all five customer support topics without signing in', () => {
  renderContact()

  expect(screen.getByText('Payment issues')).toBeVisible()
  expect(screen.getByText('AI Credits')).toBeVisible()
  expect(screen.getByText('API usage')).toBeVisible()
  expect(screen.getByText('Account-related issues')).toBeVisible()
  expect(screen.getByText('Technical support')).toBeVisible()
})

it('translates customer contact information into Vietnamese', async () => {
  await i18n.changeLanguage('vi')
  renderContact()

  expect(
    screen.getByRole('heading', { level: 1, name: 'Liên hệ Chúng tôi' })
  ).toBeVisible()
  const emailLinks = screen.getAllByRole('link', {
    name: 'tridung4323@gmail.com',
  })
  expect(emailLinks).toHaveLength(2)
  for (const link of emailLinks) {
    expect(link).toHaveAttribute('href', 'mailto:tridung4323@gmail.com')
  }

  await i18n.changeLanguage('en')
})
