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
import * as z from 'zod'

import {
  buildCryptomusUpdates,
  type CryptomusSettingsValues,
  cryptomusSettingsShape,
  refineCryptomusSettings,
} from '../cryptomus-settings'

const schema = z
  .object(cryptomusSettingsShape)
  .superRefine(refineCryptomusSettings)

const initial: CryptomusSettingsValues = {
  CryptoEnabled: false,
  CryptomusMerchantID: '',
  CryptomusPaymentAPIKey: '',
  CryptomusPaymentAPIKeyConfigured: false,
  CryptoNetwork: '',
  CryptoMinTopUp: 10,
  CryptoMaxTopUp: 100,
}

describe('Cryptomus settings schema', () => {
  it('rejects enabling without a network code', () => {
    const result = schema.safeParse({
      ...initial,
      CryptoEnabled: true,
      CryptomusPaymentAPIKey: 'new-key',
    })

    expect(result.success).toBe(false)
    expect(result.error?.issues).toContainEqual(
      expect.objectContaining({
        path: ['CryptoNetwork'],
        message: 'Enter the network code before enabling Cryptomus',
      })
    )
  })

  it('rejects enabling without a key when none is configured', () => {
    const result = schema.safeParse({
      ...initial,
      CryptoEnabled: true,
      CryptoNetwork: 'MERCHANT-NETWORK',
    })

    expect(result.success).toBe(false)
    expect(result.error?.issues).toContainEqual(
      expect.objectContaining({
        path: ['CryptomusPaymentAPIKey'],
        message: 'Configure a payment API key before enabling Cryptomus',
      })
    )
  })

  it('accepts enabling with a configured key and blank key field', () => {
    const result = schema.safeParse({
      ...initial,
      CryptoEnabled: true,
      CryptomusPaymentAPIKeyConfigured: true,
      CryptoNetwork: 'MERCHANT-NETWORK',
    })

    expect(result.success).toBe(true)
  })

  it('rejects top-up bounds outside 1..1000000', () => {
    for (const amount of [0, 1_000_001]) {
      const result = schema.safeParse({
        ...initial,
        CryptoMinTopUp: amount,
        CryptoMaxTopUp: Math.max(amount, 100),
      })

      expect(result.success).toBe(false)
      expect(result.error?.issues).toContainEqual(
        expect.objectContaining({ message: 'Must be between 1 and 1000000' })
      )
    }
  })

  it('rejects a maximum below the minimum', () => {
    const result = schema.safeParse({
      ...initial,
      CryptoMinTopUp: 50,
      CryptoMaxTopUp: 20,
    })

    expect(result.success).toBe(false)
    expect(result.error?.issues).toContainEqual(
      expect.objectContaining({
        path: ['CryptoMaxTopUp'],
        message: 'Maximum must be greater than or equal to the minimum',
      })
    )
  })
})

describe('buildCryptomusUpdates', () => {
  it('writes the enable flag only after the other Cryptomus fields', () => {
    const updates = buildCryptomusUpdates(
      {
        ...initial,
        CryptoEnabled: true,
        CryptomusMerchantID: ' merchant ',
        CryptomusPaymentAPIKey: ' new-key ',
        CryptomusPaymentAPIKeyConfigured: true,
        CryptoNetwork: ' MERCHANT-NETWORK ',
        CryptoMinTopUp: 20,
        CryptoMaxTopUp: 200,
      },
      initial
    )

    expect(updates).toEqual([
      { key: 'CryptomusMerchantID', value: 'merchant' },
      { key: 'CryptomusPaymentAPIKey', value: 'new-key' },
      { key: 'CryptoNetwork', value: 'MERCHANT-NETWORK' },
      { key: 'CryptoMinTopUp', value: 20 },
      { key: 'CryptoMaxTopUp', value: 200 },
      { key: 'CryptoEnabled', value: true },
    ])
  })

  it('keeps the saved key when the field is blank', () => {
    const updates = buildCryptomusUpdates(
      { ...initial, CryptomusPaymentAPIKeyConfigured: true },
      { ...initial, CryptomusPaymentAPIKeyConfigured: false }
    )

    expect(updates).toEqual([])
  })
})
