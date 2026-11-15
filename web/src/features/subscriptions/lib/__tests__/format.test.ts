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
import { createInstance } from 'i18next'
import { beforeAll, expect, it } from 'vitest'

import en from '@/i18n/locales/en.json'

import { formatPlanQuotaLabel } from '../format'

const i18n = createInstance()

beforeAll(async () => {
  await i18n.init({ lng: 'en', resources: { en } })
})

it.each([
  ['never', 0, 'Total Quota'],
  ['daily', 0, 'Daily quota'],
  ['weekly', 0, 'Quota per period'],
  ['monthly', 0, 'Quota per period'],
  ['custom', 86400, 'Daily quota'],
  ['custom', 3600, 'Quota per period'],
  ['custom', 172800, 'Quota per period'],
] as const)(
  'labels %s with interval %i without promising a different allowance period',
  (period, seconds, label) => {
    expect(
      formatPlanQuotaLabel(
        { quota_reset_period: period, quota_reset_custom_seconds: seconds },
        i18n.t
      )
    ).toBe(label)
  }
)
