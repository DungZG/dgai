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
import { describe, expect, test } from 'vitest'

import { PAYMENT_TYPES } from '../constants'
import type { TopupInfo } from '../types'
import {
  checkCryptomusAmount,
  dispatchSelectedPayment,
  getCryptomusRange,
  getDefaultPaymentType,
  isStripePayment,
  isWaffoPayment,
  isWaffoPancakePayment,
} from './payment'

describe('payment type classification', () => {
  test('keeps Waffo and Waffo Pancake on their dedicated flows', () => {
    expect(isWaffoPayment(PAYMENT_TYPES.WAFFO)).toBe(true)
    expect(isWaffoPayment(PAYMENT_TYPES.WAFFO_PANCAKE)).toBe(false)
    expect(isWaffoPancakePayment(PAYMENT_TYPES.WAFFO_PANCAKE)).toBe(true)
    expect(isWaffoPancakePayment(PAYMENT_TYPES.WAFFO)).toBe(false)
    expect(isStripePayment(PAYMENT_TYPES.STRIPE)).toBe(true)
  })
})

describe('payment dispatch', () => {
  test('keeps the selected Waffo method index through confirmation', async () => {
    const calls: string[] = []
    const success = await dispatchSelectedPayment(
      { name: 'Waffo Card', type: PAYMENT_TYPES.WAFFO },
      120,
      3,
      {
        regular: async () => {
          calls.push('regular')
          return false
        },
        waffo: async (amount, index) => {
          calls.push(`waffo:${amount}:${index}`)
          return true
        },
        waffoPancake: async () => {
          calls.push('pancake')
          return false
        },
      }
    )

    expect(success).toBe(true)
    expect(calls).toEqual(['waffo:120:3'])
  })

  test('does not create a Waffo order without a selected method index', async () => {
    let called = false
    const success = await dispatchSelectedPayment(
      { name: 'Waffo Card', type: PAYMENT_TYPES.WAFFO },
      120,
      null,
      {
        regular: async () => false,
        waffo: async () => {
          called = true
          return true
        },
        waffoPancake: async () => false,
      }
    )

    expect(success).toBe(false)
    expect(called).toBe(false)
  })

  test('never posts a cryptomus method to the Epay form', async () => {
    const calls: string[] = []
    const success = await dispatchSelectedPayment(
      { name: 'USDT', type: PAYMENT_TYPES.CRYPTOMUS },
      120,
      null,
      {
        regular: async () => {
          calls.push('regular')
          return true
        },
        waffo: async () => {
          calls.push('waffo')
          return true
        },
        waffoPancake: async () => {
          calls.push('pancake')
          return true
        },
      }
    )

    expect(success).toBe(false)
    expect(calls).toEqual([])
  })
})

describe('cryptomus payment rules', () => {
  const cryptoInfo = (overrides: Partial<TopupInfo> = {}): TopupInfo => ({
    enable_online_topup: true,
    enable_stripe_topup: false,
    pay_methods: [],
    min_topup: 1,
    stripe_min_topup: 1,
    amount_options: [],
    discount: {},
    enable_crypto_topup: true,
    crypto_min_topup: 10,
    crypto_max_topup: 100,
    ...overrides,
  })

  test('stays unavailable without the enable flag or usable USD bounds', () => {
    expect(getCryptomusRange(cryptoInfo({ enable_crypto_topup: false }))).toBe(
      null
    )
    expect(getCryptomusRange(cryptoInfo({ crypto_max_topup: 5 }))).toBe(null)
    expect(getCryptomusRange(cryptoInfo({ crypto_max_topup: 0 }))).toBe(null)
    expect(getCryptomusRange(cryptoInfo())).toEqual({ min: 10, max: 100 })
  })

  test('rejects amounts outside the USD bounds or not whole', () => {
    expect(checkCryptomusAmount(9, cryptoInfo()).error).toBe('min')
    expect(checkCryptomusAmount(10, cryptoInfo()).error).toBe(null)
    expect(checkCryptomusAmount(100, cryptoInfo()).error).toBe(null)
    expect(checkCryptomusAmount(101, cryptoInfo()).error).toBe('max')
    expect(checkCryptomusAmount(10.5, cryptoInfo()).error).toBe('integer')
    expect(checkCryptomusAmount(0, cryptoInfo()).error).toBe('integer')
  })

  test('uses the backend conversion rate for USD, CNY and custom currency invoices', () => {
    expect(
      checkCryptomusAmount(20, cryptoInfo({ crypto_usd_rate: 1 }))
    ).toEqual({ invoiceUsd: 20, error: null })
    expect(
      checkCryptomusAmount(140, cryptoInfo({ crypto_usd_rate: 7 }))
    ).toEqual({ invoiceUsd: 20, error: null })
    expect(
      checkCryptomusAmount(600, cryptoInfo({ crypto_usd_rate: 30 }))
    ).toEqual({ invoiceUsd: 20, error: null })

    const tokens = cryptoInfo({
      quota_display_type: 'TOKENS',
      quota_per_unit: 500_000,
      crypto_usd_rate: 999,
    })
    expect(checkCryptomusAmount(10_000_000, tokens)).toEqual({
      invoiceUsd: 20,
      error: null,
    })
  })

  test('never preselects the cryptomus method', () => {
    expect(
      getDefaultPaymentType(
        cryptoInfo({
          pay_methods: [
            { name: 'USDT', type: 'cryptomus' },
            { name: 'Stripe', type: 'stripe' },
          ],
        })
      )
    ).toBe('stripe')
  })
})
