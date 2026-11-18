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
import * as z from 'zod'

const CRYPTO_TOPUP_LIMIT = 1_000_000

export interface CryptomusSettingsValues {
  CryptoEnabled: boolean
  CryptomusMerchantID: string
  CryptomusPaymentAPIKey: string
  CryptomusPaymentAPIKeyConfigured: boolean
  CryptoNetwork: string
  CryptoMinTopUp: number
  CryptoMaxTopUp: number
}

const amount = z.coerce
  .number()
  .int('Must be a whole number')
  .min(1, 'Must be between 1 and 1000000')
  .max(CRYPTO_TOPUP_LIMIT, 'Must be between 1 and 1000000')

export const cryptomusSettingsShape = {
  CryptoEnabled: z.boolean(),
  CryptomusMerchantID: z.string(),
  CryptomusPaymentAPIKey: z.string(),
  CryptomusPaymentAPIKeyConfigured: z.boolean(),
  CryptoNetwork: z.string(),
  CryptoMinTopUp: amount,
  CryptoMaxTopUp: amount,
}

export function refineCryptomusSettings(
  values: CryptomusSettingsValues,
  context: z.RefinementCtx
) {
  if (values.CryptoMaxTopUp < values.CryptoMinTopUp) {
    context.addIssue({
      code: 'custom',
      path: ['CryptoMaxTopUp'],
      message: 'Maximum must be greater than or equal to the minimum',
    })
  }

  if (!values.CryptoEnabled) return

  if (!values.CryptoNetwork.trim()) {
    context.addIssue({
      code: 'custom',
      path: ['CryptoNetwork'],
      message: 'Enter the network code before enabling Cryptomus',
    })
  }

  if (
    !values.CryptomusPaymentAPIKeyConfigured &&
    !values.CryptomusPaymentAPIKey.trim()
  ) {
    context.addIssue({
      code: 'custom',
      path: ['CryptomusPaymentAPIKey'],
      message: 'Configure a payment API key before enabling Cryptomus',
    })
  }
}

export type CryptomusOptionUpdate = {
  key: string
  value: string | number | boolean
}

// Credentials and limits are written before the enable flag so a partial save
// never activates an incomplete Cryptomus configuration.
export function buildCryptomusUpdates(
  values: CryptomusSettingsValues,
  initial: CryptomusSettingsValues
): CryptomusOptionUpdate[] {
  const updates: CryptomusOptionUpdate[] = []
  const merchantID = values.CryptomusMerchantID.trim()
  const paymentAPIKey = values.CryptomusPaymentAPIKey.trim()
  const network = values.CryptoNetwork.trim()

  if (merchantID !== initial.CryptomusMerchantID.trim()) {
    updates.push({
      key: 'CryptomusMerchantID',
      value: merchantID,
    })
  }

  if (
    paymentAPIKey &&
    paymentAPIKey !== initial.CryptomusPaymentAPIKey.trim()
  ) {
    updates.push({
      key: 'CryptomusPaymentAPIKey',
      value: paymentAPIKey,
    })
  }

  if (network !== initial.CryptoNetwork.trim()) {
    updates.push({ key: 'CryptoNetwork', value: network })
  }

  if (values.CryptoMinTopUp !== initial.CryptoMinTopUp) {
    updates.push({ key: 'CryptoMinTopUp', value: values.CryptoMinTopUp })
  }

  if (values.CryptoMaxTopUp !== initial.CryptoMaxTopUp) {
    updates.push({ key: 'CryptoMaxTopUp', value: values.CryptoMaxTopUp })
  }

  if (values.CryptoEnabled !== initial.CryptoEnabled) {
    updates.push({ key: 'CryptoEnabled', value: values.CryptoEnabled })
  }

  return updates
}
