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
import { Link } from '@tanstack/react-router'
import { ArrowRight, BookOpen, KeyRound, Layers, Wallet } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { IconBadge } from '@/components/ui/icon-badge'
import { Skeleton } from '@/components/ui/skeleton'
import { QUOTA_TYPE_VALUES } from '@/features/pricing/constants'
import { usePricingData } from '@/features/pricing/hooks/use-pricing-data'
import { formatPrice, formatRequestPrice } from '@/features/pricing/lib'
import { useStatus } from '@/hooks/use-status'
import { useAuthStore } from '@/stores/auth-store'

const FEATURED_MODEL_LIMIT = 6

/**
 * Introduction shown on /about when the administrator has not written any
 * content. It describes the gateway and lists the models that are actually on
 * sale, read from the public pricing endpoint, so it never drifts from the
 * catalogue.
 */
export function AboutIntro() {
  const { t } = useTranslation()
  const { status } = useStatus()
  const { models, isLoading, error } = usePricingData()
  const user = useAuthStore((state) => state.auth.user)

  const docsUrl = (status?.docs_link as string | undefined) || '/docs'
  const docsIsExternal = docsUrl.startsWith('http')
  const modelCount = models.length
  const vendorCount = new Set(
    models.map((model) => model.vendor_name).filter(Boolean)
  ).size
  const featured = models.slice(0, FEATURED_MODEL_LIMIT)

  const steps = [
    {
      icon: <KeyRound className='size-4' />,
      title: t('Create an account and an API key'),
      description: t(
        'Sign up, then mint a key in the console. Every key carries its own quota and model access.'
      ),
    },
    {
      icon: <Wallet className='size-4' />,
      title: t('Top up your balance'),
      description: t(
        'Pay by bank transfer or card. Credit lands in your balance immediately and is consumed per token.'
      ),
    },
    {
      icon: <Layers className='size-4' />,
      title: t('Point your client at one endpoint'),
      description: t(
        'Use the OpenAI-compatible base URL with any SDK, CLI, or IDE. Switching models is a single string change.'
      ),
    },
  ]

  const renderCatalogue = () => {
    if (isLoading) {
      return (
        <div className='grid gap-3 sm:grid-cols-2 lg:grid-cols-3'>
          {Array.from({ length: FEATURED_MODEL_LIMIT }, (_, index) => (
            <Skeleton key={index} className='h-24 rounded-xl' />
          ))}
        </div>
      )
    }

    if (featured.length === 0) {
      return (
        <p className='text-muted-foreground text-sm'>
          {t('No models are on sale yet.')}
        </p>
      )
    }

    return (
      <div className='grid gap-3 sm:grid-cols-2 lg:grid-cols-3'>
        {featured.map((model) => {
          const perRequest = model.quota_type === QUOTA_TYPE_VALUES.REQUEST
          const input = perRequest ? null : formatPrice(model, 'input', 'M')
          const output = perRequest ? null : formatPrice(model, 'output', 'M')

          return (
            <Card
              key={model.model_name}
              data-card-hover='false'
              className='py-0'
            >
              <CardContent className='space-y-2 p-4'>
                <div className='truncate font-mono text-sm font-medium'>
                  {model.model_name}
                </div>
                {model.vendor_name ? (
                  <div className='text-muted-foreground text-xs'>
                    {model.vendor_name}
                  </div>
                ) : null}
                <div className='text-xs tabular-nums'>
                  {perRequest ? (
                    <span>
                      {t('{{price}} per request', {
                        price: formatRequestPrice(model),
                      })}
                    </span>
                  ) : (
                    <span className='text-muted-foreground'>
                      {t('In')} {input} · {t('Out')} {output}
                    </span>
                  )}
                </div>
              </CardContent>
            </Card>
          )
        })}
      </div>
    )
  }

  return (
    <div className='mx-auto max-w-6xl px-4 py-10 sm:py-14'>
      {/* Hero */}
      <section className='space-y-5'>
        <Badge variant='outline' className='gap-1.5'>
          <span className='bg-success size-1.5 rounded-full' />
          {t('Unified AI API gateway')}
        </Badge>
        <h1 className='text-3xl leading-tight font-bold tracking-tight sm:text-4xl'>
          {modelCount > 0
            ? t('One key, {{count}} models, one endpoint', {
                count: modelCount,
              })
            : t('One key, one endpoint, every model')}
        </h1>
        <p className='text-muted-foreground max-w-2xl text-base leading-relaxed'>
          {t(
            'Route requests to every supported provider through a single OpenAI-compatible API, with per-token billing, usage logs and quota control.'
          )}
        </p>
        <div className='flex flex-wrap items-center gap-3'>
          {user ? (
            <Button render={<Link to='/dashboard' />}>
              {t('Open the console')}
              <ArrowRight className='ms-1.5 size-4' />
            </Button>
          ) : (
            <Button render={<Link to='/sign-up' />}>
              {t('Get started')}
              <ArrowRight className='ms-1.5 size-4' />
            </Button>
          )}
          <Button
            variant='outline'
            render={
              docsIsExternal ? (
                <a href={docsUrl} target='_blank' rel='noopener noreferrer' />
              ) : (
                <Link to={docsUrl} />
              )
            }
          >
            <BookOpen className='me-1.5 size-4' />
            {t('Docs')}
          </Button>
        </div>
      </section>

      {/* Counters */}
      {modelCount > 0 ? (
        <section className='mt-10 grid gap-3 sm:grid-cols-3'>
          {[
            [t('Models available'), String(modelCount)],
            [t('Providers'), String(vendorCount)],
            [t('Billing'), t('Pay per token')],
          ].map(([label, value]) => (
            <Card key={label} data-card-hover='false' className='py-0'>
              <CardContent className='p-4'>
                <div className='text-muted-foreground text-[10px] font-medium tracking-wider uppercase'>
                  {label}
                </div>
                <div className='mt-1 text-xl font-semibold tabular-nums'>
                  {value}
                </div>
              </CardContent>
            </Card>
          ))}
        </section>
      ) : null}

      {/* Live catalogue — hidden when the pricing endpoint is off, because an
          empty list there means "unavailable", not "nothing on sale". */}
      {error ? null : (
        <section className='mt-10 space-y-3'>
          <div className='flex items-end justify-between gap-3'>
            <div>
              <h2 className='text-lg font-semibold'>{t('Available models')}</h2>
              <p className='text-muted-foreground text-sm'>
                {t(
                  'Prices shown per 1M tokens, unless the model bills per request.'
                )}
              </p>
            </div>
            <Button variant='ghost' size='sm' render={<Link to='/pricing' />}>
              {t('See all pricing')}
              <ArrowRight className='ms-1.5 size-3.5' />
            </Button>
          </div>

          {renderCatalogue()}
        </section>
      )}

      {/* How it works */}
      <section className='mt-10'>
        <h2 className='text-lg font-semibold'>{t('How it works')}</h2>
        <ol className='mt-3 grid gap-3 sm:grid-cols-3'>
          {steps.map((step, index) => (
            <li key={step.title}>
              <Card data-card-hover='false' className='h-full py-0'>
                <CardContent className='space-y-2 p-4'>
                  <div className='flex items-center gap-2'>
                    <IconBadge tone='chart-2'>{step.icon}</IconBadge>
                    <span className='text-muted-foreground text-xs font-semibold tabular-nums'>
                      {index + 1}
                    </span>
                  </div>
                  <div className='text-sm font-medium'>{step.title}</div>
                  <p className='text-muted-foreground text-xs leading-relaxed'>
                    {step.description}
                  </p>
                </CardContent>
              </Card>
            </li>
          ))}
        </ol>
      </section>
    </div>
  )
}
