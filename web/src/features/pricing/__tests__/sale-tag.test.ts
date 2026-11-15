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
import { describe, expect, it } from 'vitest'

import { salePercentFromTags } from '../lib/filters'

describe('salePercentFromTags', () => {
  it('reads the discount from a tag', () => {
    expect(salePercentFromTags('fast,sale:50')).toBe(50)
    expect(salePercentFromTags('sale-30')).toBe(30)
    expect(salePercentFromTags('SALE:15')).toBe(15)
  })

  it('ignores models without a sale marker', () => {
    expect(salePercentFromTags(undefined)).toBeNull()
    expect(salePercentFromTags('')).toBeNull()
    expect(salePercentFromTags('fast,reasoning')).toBeNull()
    // A bare word must not be read as a discount.
    expect(salePercentFromTags('sale')).toBeNull()
  })

  it('rejects percentages outside the usable range', () => {
    expect(salePercentFromTags('sale:0')).toBeNull()
    expect(salePercentFromTags('sale:100')).toBeNull()
  })
})
