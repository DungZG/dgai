import { describe, expect, test } from 'vitest'

import type { TopupInfo } from '../../types'
import { getTopUpBonusPercent, topUpAmountUSD } from '../payment'

describe('top-up bonus tiers', () => {
  const tiers = [
    { min_amount_usd: 50, percent: 12 },
    { min_amount_usd: 10, percent: 2 },
    { min_amount_usd: 100, percent: 25 },
    { min_amount_usd: 20, percent: 5 },
  ]

  test.each([
    [9.99, 0],
    [10, 2],
    [19.99, 2],
    [20, 5],
    [50, 12],
    [100, 25],
  ])('selects the highest applicable tier for $%s', (amount, expected) => {
    expect(getTopUpBonusPercent(amount, tiers)).toBe(expected)
  })
})

test('token-mode top-ups qualify on whole USD credit rather than raw quota', () => {
  const info = {
    quota_display_type: 'TOKENS',
    quota_per_unit: 500000,
  } as TopupInfo
  expect(topUpAmountUSD(9999999, info)).toBe(19)
  expect(topUpAmountUSD(10000000, info)).toBe(20)
  expect(topUpAmountUSD(50, null)).toBe(50)
})
