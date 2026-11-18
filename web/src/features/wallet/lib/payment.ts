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
import { getServerMessage } from '@/lib/server-error-message'

import {
  PAYMENT_TYPES,
  DEFAULT_PRESET_MULTIPLIERS,
  DEFAULT_PAYMENT_TYPE,
  DEFAULT_MIN_TOPUP,
} from '../constants'
import type { PaymentMethod, PresetAmount, TopupInfo } from '../types'

// ============================================================================
// Payment Processing Functions
// ============================================================================

/**
 * Payment endpoints answer errors as {"message": "error", "data": text}. When
 * the response also carries a message_key, this is its translation; callers
 * fall back to the text they already read.
 */
export function getPaymentErrorTranslation(
  response: unknown
): string | undefined {
  if (!response || typeof response !== 'object') return undefined
  const body = response as Record<string, unknown>
  if (typeof body.message_key !== 'string' || !body.message_key) {
    return undefined
  }
  return getServerMessage(body)
}

/**
 * Check if browser is Safari
 */
function isSafariBrowser(): boolean {
  return (
    navigator.userAgent.includes('Safari') &&
    !navigator.userAgent.includes('Chrome')
  )
}

/**
 * Submit payment form (for non-Stripe payments)
 */
export function submitPaymentForm(
  url: string,
  params: Record<string, unknown>
): void {
  const form = document.createElement('form')
  form.action = url
  form.method = 'POST'

  // Don't open in new tab for Safari
  if (!isSafariBrowser()) {
    form.target = '_blank'
  }

  // Add form parameters
  Object.entries(params).forEach(([key, value]) => {
    const input = document.createElement('input')
    input.type = 'hidden'
    input.name = key
    input.value = String(value)
    form.appendChild(input)
  })

  document.body.appendChild(form)
  form.submit()
  document.body.removeChild(form)
}

/**
 * Check if payment method is Stripe
 */
export function isStripePayment(paymentType: string): boolean {
  return paymentType === PAYMENT_TYPES.STRIPE
}

/**
 * Check if payment method is Waffo
 */
export function isWaffoPayment(paymentType: string): boolean {
  return paymentType === PAYMENT_TYPES.WAFFO
}

/**
 * Check if payment method is Waffo Pancake
 *
 * Pancake is a metered-style payment that goes through a dedicated checkout
 * URL flow rather than the generic epay form submission, so it must be
 * special-cased in payment dispatch logic.
 */
export function isWaffoPancakePayment(paymentType: string): boolean {
  return paymentType === PAYMENT_TYPES.WAFFO_PANCAKE
}

/**
 * Cryptomus pays through its own checkout dialog and has no quote endpoint.
 */
export function isCryptomusPayment(paymentType: string): boolean {
  return paymentType === PAYMENT_TYPES.CRYPTOMUS
}

export interface CryptomusRange {
  min: number
  max: number
}

const CRYPTOMUS_BOUND_LIMIT = 1_000_000

/** Null unless crypto top-up is enabled and both USD bounds are usable. */
export function getCryptomusRange(
  info: TopupInfo | null
): CryptomusRange | null {
  if (!info?.enable_crypto_topup) return null
  const min = Math.floor(Number(info.crypto_min_topup))
  const max = Math.floor(Number(info.crypto_max_topup))
  const usable = (value: number) =>
    Number.isFinite(value) && value >= 1 && value <= CRYPTOMUS_BOUND_LIMIT
  if (!usable(min) || !usable(max) || max < min) return null
  return { min, max }
}

/**
 * The gateway converts the display amount back to USD before invoicing; mirror
 * that conversion so the preview equals the invoice.
 */
export function cryptomusInvoiceUSD(
  amount: number,
  info: TopupInfo | null
): number {
  if (info?.quota_display_type === 'TOKENS') {
    const perUnit = info.quota_per_unit || 0
    return perUnit > 0 ? amount / perUnit : Number.NaN
  }
  const usdRate = Number(info?.crypto_usd_rate)
  return amount / (usdRate > 0 ? usdRate : 1)
}

export type CryptomusAmountError = 'disabled' | 'integer' | 'min' | 'max'

/**
 * The request amount keeps the display unit; the floor and ceiling are USD.
 */
export function checkCryptomusAmount(
  amount: number,
  info: TopupInfo | null
): { invoiceUsd: number; error: CryptomusAmountError | null } {
  const invoiceUsd = cryptomusInvoiceUSD(amount, info)
  const range = getCryptomusRange(info)
  if (!range) return { invoiceUsd, error: 'disabled' }
  if (!Number.isInteger(amount) || amount <= 0) {
    return { invoiceUsd, error: 'integer' }
  }
  if (invoiceUsd < range.min) return { invoiceUsd, error: 'min' }
  if (invoiceUsd > range.max) return { invoiceUsd, error: 'max' }
  return { invoiceUsd, error: null }
}

export interface PaymentProcessors {
  regular: (topupAmount: number, paymentType: string) => Promise<boolean>
  waffo: (topupAmount: number, payMethodIndex: number) => Promise<boolean>
  waffoPancake: (topupAmount: number) => Promise<boolean>
}

export async function dispatchSelectedPayment(
  paymentMethod: PaymentMethod,
  topupAmount: number,
  waffoMethodIndex: number | null,
  processors: PaymentProcessors
): Promise<boolean> {
  if (isCryptomusPayment(paymentMethod.type)) {
    // Falling through would post a cryptomus method to the Epay form endpoint.
    return false
  }

  if (isWaffoPayment(paymentMethod.type)) {
    if (waffoMethodIndex === null) {
      return false
    }
    return processors.waffo(topupAmount, waffoMethodIndex)
  }

  if (isWaffoPancakePayment(paymentMethod.type)) {
    return processors.waffoPancake(topupAmount)
  }

  return processors.regular(topupAmount, paymentMethod.type)
}

/**
 * Get default payment type from topup info
 */
export function getDefaultPaymentType(topupInfo: TopupInfo | null): string {
  if (!topupInfo) {
    return DEFAULT_PAYMENT_TYPE
  }

  // Return first available payment method or default. Cryptomus is chosen from
  // its own USDT button, so it never becomes the preselected method.
  const firstStandardMethod = topupInfo.pay_methods?.find(
    (method) => !isCryptomusPayment(method.type)
  )
  if (firstStandardMethod) {
    return firstStandardMethod.type
  }

  if (topupInfo.enable_stripe_topup) {
    return PAYMENT_TYPES.STRIPE
  }

  if (topupInfo.enable_waffo_topup) {
    return PAYMENT_TYPES.WAFFO
  }

  if (topupInfo.enable_waffo_pancake_topup) {
    return PAYMENT_TYPES.WAFFO_PANCAKE
  }

  return DEFAULT_PAYMENT_TYPE
}

/**
 * Get minimum topup amount from topup info
 */
export function getMinTopupAmount(topupInfo: TopupInfo | null): number {
  if (!topupInfo) {
    return DEFAULT_MIN_TOPUP
  }

  if (topupInfo.enable_online_topup) {
    return topupInfo.min_topup
  }

  if (topupInfo.enable_stripe_topup) {
    return topupInfo.stripe_min_topup
  }

  if (topupInfo.enable_waffo_topup) {
    return topupInfo.waffo_min_topup || DEFAULT_MIN_TOPUP
  }

  if (topupInfo.enable_waffo_pancake_topup) {
    return topupInfo.waffo_pancake_min_topup || DEFAULT_MIN_TOPUP
  }

  return DEFAULT_MIN_TOPUP
}

/**
 * Generate preset amounts based on minimum topup
 */
export function generatePresetAmounts(minAmount: number): PresetAmount[] {
  return DEFAULT_PRESET_MULTIPLIERS.map((multiplier) => ({
    value: minAmount * multiplier,
  }))
}

/**
 * Merge custom preset amounts with discounts
 */
export function mergePresetAmounts(
  amountOptions: number[],
  discounts: Record<number, number>
): PresetAmount[] {
  if (!amountOptions || amountOptions.length === 0) {
    return []
  }

  return amountOptions.map((amount) => ({
    value: amount,
    discount: discounts[amount] || 1.0,
  }))
}

export function topUpAmountUSD(amount: number, info: TopupInfo | null): number {
  if (info?.quota_display_type === 'TOKENS') {
    return info.quota_per_unit ? Math.floor(amount / info.quota_per_unit) : 0
  }
  return amount
}

export function getTopUpBonusPercent(
  baseAmountUsd: number,
  tiers: { min_amount_usd: number; percent: number }[] = []
): number {
  let percent = 0
  let minimum = 0
  for (const tier of tiers) {
    if (tier.min_amount_usd > minimum && tier.min_amount_usd <= baseAmountUsd) {
      minimum = tier.min_amount_usd
      percent = tier.percent
    }
  }
  return percent
}
