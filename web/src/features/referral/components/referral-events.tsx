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
import type { TFunction } from 'i18next'
import { Gift } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { EmptyState } from '@/components/empty-state'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import { formatDateTimeObject } from '@/lib/time'

import type { ReferralEvent } from '@/features/wallet/types'

/**
 * Reward events are stored as English source keys plus params so the text
 * follows the viewer's language; older rows fall back to the plain content.
 */
function eventText(event: ReferralEvent, t: TFunction): string {
  const parts = event.other?.content_parts
  if (!Array.isArray(parts) || parts.length === 0) {
    return event.content
  }
  return parts.map((part) => t(part.key, part.params)).join(', ')
}

interface ReferralEventsProps {
  events: ReferralEvent[]
  loading?: boolean
}

export function ReferralEvents(props: ReferralEventsProps) {
  const { t } = useTranslation()

  const renderBody = () => {
    if (props.loading) {
      return (
        <div className='space-y-3'>
          <Skeleton className='h-10 w-full' />
          <Skeleton className='h-10 w-full' />
          <Skeleton className='h-10 w-full' />
        </div>
      )
    }

    if (props.events.length === 0) {
      return (
        <EmptyState
          icon={Gift}
          title={t('No reward events yet')}
          description={t('Invite someone or top up to start earning')}
          bordered={false}
          className='min-h-[220px]'
        />
      )
    }

    return (
      <div className='flex flex-col'>
        {props.events.map((event, index) => (
          <div key={event.id}>
            <div className='flex items-start justify-between gap-3 py-3'>
              <span className='min-w-0 flex-1 text-sm'>
                {eventText(event, t)}
              </span>
              <span className='text-muted-foreground shrink-0 text-xs tabular-nums'>
                {formatDateTimeObject(new Date(event.created_at * 1000))}
              </span>
            </div>
            {index < props.events.length - 1 ? <Separator /> : null}
          </div>
        ))}
      </div>
    )
  }

  return (
    <Card data-card-hover='false' className='py-0'>
      <CardHeader className='border-b p-4 !pb-3'>
        <CardTitle className='text-sm font-semibold'>
          {t('Reward history')}
        </CardTitle>
      </CardHeader>
      <CardContent className='p-4'>{renderBody()}</CardContent>
    </Card>
  )
}

/** Explains the three steps so the page is not just numbers. */
export function ReferralHowItWorks(props: { percent: number }) {
  const { t } = useTranslation()

  const steps = [
    {
      title: t('Share your link'),
      description: t(
        'Send your referral link to friends or post it wherever your audience is.'
      ),
    },
    {
      title: t('They sign up and top up'),
      description: t(
        'The link registers them under your account automatically.'
      ),
    },
    {
      title:
        props.percent > 0
          ? t('You earn {{percent}}% of every top-up', {
              percent: props.percent,
            })
          : t('You earn a reward for every signup'),
      description: t(
        'Rewards accumulate here and can be moved to your balance at any time.'
      ),
    },
  ]

  return (
    <Card data-card-hover='false' className='py-0'>
      <CardHeader className='border-b p-4 !pb-3'>
        <CardTitle className='text-sm font-semibold'>
          {t('How it works')}
        </CardTitle>
      </CardHeader>
      <CardContent className='p-4'>
        <ol className='space-y-4'>
          {steps.map((step, index) => (
            <li key={step.title} className='flex gap-3'>
              <span className='bg-muted text-muted-foreground flex size-6 shrink-0 items-center justify-center rounded-full text-xs font-semibold'>
                {index + 1}
              </span>
              <div className='min-w-0'>
                <div className='text-sm font-medium'>{step.title}</div>
                <p className='text-muted-foreground text-xs'>
                  {step.description}
                </p>
              </div>
            </li>
          ))}
        </ol>
      </CardContent>
    </Card>
  )
}
