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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { TransferDialog } from '@/features/wallet/components/dialogs/transfer-dialog'
import { useAffiliate, useTopupInfo } from '@/features/wallet/hooks'

import { ReferralEvents, ReferralHowItWorks } from './components/referral-events'
import { ReferralHero, ReferralStats } from './components/referral-hero'

export function Referral() {
  const { t } = useTranslation()
  const [transferOpen, setTransferOpen] = useState(false)
  const { topupInfo } = useTopupInfo()
  const {
    affiliateLink,
    referral,
    loading,
    transferring,
    transferQuota,
  } = useAffiliate()

  const complianceConfirmed = topupInfo?.payment_compliance_confirmed !== false
  const percent = referral?.commission_percent ?? 0

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        {t('Referral Program')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='space-y-4'>
          <ReferralHero
            referral={referral}
            affiliateLink={affiliateLink}
            loading={loading}
            complianceConfirmed={complianceConfirmed}
            onTransfer={() => setTransferOpen(true)}
          />

          <ReferralStats referral={referral} />

          <div className='grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.1fr)]'>
            <ReferralHowItWorks percent={percent} />
            <ReferralEvents
              events={referral?.events ?? []}
              loading={loading}
            />
          </div>
        </div>
      </SectionPageLayout.Content>

      <TransferDialog
        open={transferOpen}
        onOpenChange={setTransferOpen}
        onConfirm={transferQuota}
        availableQuota={referral?.aff_quota ?? 0}
        transferring={transferring}
      />
    </SectionPageLayout>
  )
}
