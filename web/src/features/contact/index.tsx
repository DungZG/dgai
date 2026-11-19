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
import { useEffect } from 'react'
import { useTranslation } from 'react-i18next'

import { PublicLayout } from '@/components/layout'
import { Footer } from '@/components/layout/components/footer'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { SITE_CONTACT } from '@/lib/site-contact'

import { CONTACT_PAGE_DESCRIPTION } from './constants'

const SUPPORT_TOPICS = [
  'Payment issues',
  'AI Credits',
  'API usage',
  'Account-related issues',
  'Technical support',
] as const

export function Contact() {
  const { t } = useTranslation()

  useEffect(() => {
    const description = document.head.querySelector<HTMLMetaElement>(
      'meta[name="description"]'
    )
    if (!description) return

    const previousDescription = description.content
    description.content = CONTACT_PAGE_DESCRIPTION
    return () => {
      description.content = previousDescription
    }
  }, [])

  return (
    <PublicLayout showMainContainer={false}>
      <main>
        <section
          aria-labelledby='contact-page-title'
          className='border-border/40 bg-muted/10 border-b'
        >
          <div className='mx-auto max-w-6xl px-4 py-14 sm:px-6 sm:py-20'>
            <div className='max-w-3xl space-y-4'>
              <p className='text-primary text-sm font-semibold tracking-wide uppercase'>
                {t('Customer Support')}
              </p>
              <h1
                id='contact-page-title'
                className='text-foreground text-3xl font-semibold tracking-tight sm:text-4xl'
              >
                {t('footer.columns.about.links.contact')}
              </h1>
              <p className='text-muted-foreground max-w-2xl text-base leading-7 sm:text-lg'>
                {t(
                  'Get help with payments, AI Credits, API usage, account issues, or technical support.'
                )}
              </p>
            </div>
          </div>
        </section>

        <section
          aria-labelledby='contact-details-title'
          className='mx-auto grid max-w-6xl gap-5 px-4 py-8 sm:px-6 sm:py-12 md:grid-cols-2'
        >
          <Card>
            <CardHeader>
              <CardTitle>
                <h2 id='contact-details-title' className='font-medium'>
                  {t('Contact details')}
                </h2>
              </CardTitle>
              <CardDescription>
                {t('Official customer support information for DGAI.')}
              </CardDescription>
            </CardHeader>
            <CardContent>
              <dl className='divide-border/60 divide-y'>
                <div className='grid gap-1 py-3 first:pt-0 sm:grid-cols-[10rem_1fr] sm:gap-4'>
                  <dt className='text-muted-foreground text-sm'>
                    {t('Business name')}
                  </dt>
                  <dd className='text-foreground text-sm font-medium'>
                    {SITE_CONTACT.businessName}
                  </dd>
                </div>
                <div className='grid gap-1 py-3 sm:grid-cols-[10rem_1fr] sm:gap-4'>
                  <dt className='text-muted-foreground text-sm'>
                    {t('Support email')}
                  </dt>
                  <dd>
                    <a
                      href={`mailto:${SITE_CONTACT.supportEmail}`}
                      className='text-foreground focus-visible:ring-ring rounded-sm text-sm font-medium underline underline-offset-4 outline-none hover:opacity-80 focus-visible:ring-2'
                    >
                      {SITE_CONTACT.supportEmail}
                    </a>
                  </dd>
                </div>
                <div className='grid gap-1 py-3 last:pb-0 sm:grid-cols-[10rem_1fr] sm:gap-4'>
                  <dt className='text-muted-foreground text-sm'>
                    {t('Website')}
                  </dt>
                  <dd>
                    <a
                      href={SITE_CONTACT.website}
                      target='_blank'
                      rel='noopener noreferrer'
                      className='text-foreground focus-visible:ring-ring rounded-sm text-sm font-medium underline underline-offset-4 outline-none hover:opacity-80 focus-visible:ring-2'
                    >
                      {SITE_CONTACT.website}
                    </a>
                  </dd>
                </div>
              </dl>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>
                <h2 className='font-medium'>{t('How DGAI can help')}</h2>
              </CardTitle>
              <CardDescription>
                {t(
                  'Include your account email and a short description so we can assist you faster.'
                )}
              </CardDescription>
            </CardHeader>
            <CardContent>
              <ul className='text-muted-foreground list-disc space-y-3 pl-5 text-sm leading-6'>
                {SUPPORT_TOPICS.map((topic) => (
                  <li key={topic}>{t(topic)}</li>
                ))}
              </ul>
            </CardContent>
          </Card>
        </section>

        <div className='mx-auto max-w-6xl px-4 pb-12 sm:px-6'>
          <p className='text-muted-foreground text-sm'>
            {t('Email DGAI customer support at')}{' '}
            <a
              href={`mailto:${SITE_CONTACT.supportEmail}`}
              className='text-foreground focus-visible:ring-ring rounded-sm font-medium underline underline-offset-4 outline-none hover:opacity-80 focus-visible:ring-2'
            >
              {SITE_CONTACT.supportEmail}
            </a>
            .
          </p>
        </div>
      </main>
      <Footer />
    </PublicLayout>
  )
}
