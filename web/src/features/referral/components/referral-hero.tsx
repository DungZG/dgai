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
import { Gift, Share2, Users } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { IconBadge } from '@/components/ui/icon-badge'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { formatQuota } from '@/lib/format'

import type { ReferralInfo } from '@/features/wallet/types'

interface ReferralHeroProps {
  referral: ReferralInfo | null
  affiliateLink: string
  loading?: boolean
  complianceConfirmed?: boolean
  onTransfer: () => void
}

export function ReferralHero(props: ReferralHeroProps) {
  const { t } = useTranslation()

  if (props.loading) {
    return (
      <Card data-card-hover='false' className='overflow-hidden py-0'>
        <CardContent className='space-y-4 p-4 sm:p-6'>
          <Skeleton className='h-6 w-48' />
          <Skeleton className='h-12 w-full' />
          <Skeleton className='h-10 w-64' />
        </CardContent>
      </Card>
    )
  }

  const percent = props.referral?.commission_percent ?? 0
  const pending = props.referral?.aff_quota ?? 0
  const hasRewards = pending > 0

  return (
    <Card data-card-hover='false' className='overflow-hidden py-0'>
      <CardContent className='space-y-5 p-4 sm:p-6'>
        <div className='flex min-w-0 items-start gap-3'>
          <IconBadge tone='chart-3'>
            <Share2 />
          </IconBadge>
          <div className='min-w-0 flex-1'>
            <p className='text-muted-foreground text-sm'>
              {percent > 0
                ? t(
                    'Share your link and earn {{percent}}% of every top-up the people you invite make.',
                    { percent }
                  )
                : t(
                    'Share your link and earn rewards when people you invite join and top up.'
                  )}
            </p>
          </div>
          {percent > 0 ? (
            <div className='hidden shrink-0 text-right sm:block'>
              <div className='text-3xl font-semibold tabular-nums'>
                {percent}%
              </div>
              <div className='text-muted-foreground text-[10px] font-medium tracking-wider uppercase'>
                {t('Commission')}
              </div>
            </div>
          ) : null}
        </div>

        <div className='space-y-2'>
          <div className='text-muted-foreground text-xs font-medium'>
            {t('Your referral link')}
          </div>
          <div className='flex items-center gap-2'>
            <Input
              value={props.affiliateLink}
              readOnly
              aria-label={t('Your referral link')}
              className='border-muted bg-background/70 h-10 min-w-0 flex-1 font-mono text-xs'
            />
            <CopyButton
              value={props.affiliateLink}
              variant='outline'
              className='bg-background size-10 shrink-0'
              iconClassName='size-4'
              tooltip={t('Copy referral link')}
              aria-label={t('Copy referral link')}
            />
          </div>
        </div>

        <div className='flex flex-wrap items-center justify-between gap-3'>
          <div>
            <div className='text-muted-foreground text-[10px] font-medium tracking-wider uppercase'>
              {t('Pending rewards')}
            </div>
            <div className='text-xl font-semibold tabular-nums'>
              {formatQuota(pending)}
            </div>
          </div>
          {hasRewards ? (
            <Button
              onClick={props.onTransfer}
              disabled={props.complianceConfirmed === false}
            >
              {t('Transfer to Balance')}
            </Button>
          ) : null}
        </div>

        {props.complianceConfirmed === false ? (
          <p className='text-muted-foreground text-xs'>
            {t(
              'Referral reward transfer is disabled until the administrator confirms compliance terms.'
            )}
          </p>
        ) : null}
      </CardContent>
    </Card>
  )
}

/** Three headline numbers: invites, pending rewards and lifetime earnings. */
export function ReferralStats(props: { referral: ReferralInfo | null }) {
  const { t } = useTranslation()

  // Pending is deliberately absent: the hero already shows it next to the
  // transfer button, and repeating the same number twice is noise.
  const percent = props.referral?.commission_percent ?? 0
  const items = [
    {
      icon: <Users className='size-4' />,
      label: t('Invites'),
      value: String(props.referral?.aff_count ?? 0),
    },
    {
      icon: <Gift className='size-4' />,
      label: t('Total Earned'),
      value: formatQuota(props.referral?.aff_history_quota ?? 0),
    },
    {
      icon: <Gift className='size-4' />,
      label: t('Commission'),
      value: percent > 0 ? `${percent}%` : t('Not configured'),
    },
  ]

  return (
    <div className='grid gap-3 sm:grid-cols-3'>
      {items.map((item) => (
        <Card key={item.label} data-card-hover='false' className='py-0'>
          <CardContent className='flex items-center gap-3 p-4'>
            <IconBadge tone='chart-2'>{item.icon}</IconBadge>
            <div className='min-w-0'>
              <div className='text-muted-foreground text-[10px] font-medium tracking-wider uppercase'>
                {item.label}
              </div>
              <div className='truncate text-lg font-semibold tabular-nums'>
                {item.value}
              </div>
            </div>
          </CardContent>
        </Card>
      ))}
    </div>
  )
}
