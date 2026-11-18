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
import { act, renderHook, waitFor } from '@testing-library/react'
import { beforeEach, expect, it, vi } from 'vitest'

import { CRYPTOMUS_PENDING_STORAGE_KEY } from '../constants'
import { useCryptomusPayment } from '../hooks/use-cryptomus-payment'
import type { CryptomusOrder } from '../types'

const { getOrder, createOrder } = vi.hoisted(() => ({
  getOrder: vi.fn(),
  createOrder: vi.fn(),
}))

vi.mock('../api', () => ({
  getCryptomusOrder: getOrder,
  isApiSuccess: (response: { success?: boolean; message?: string }) =>
    response.success === true || response.message === 'success',
  requestCryptomusPayment: createOrder,
}))


const pendingOrder: CryptomusOrder = {
  trade_no: 'crypto-order-1',
  status: 'pending',
  provider_status: 'check',
  amount_usd: '10',
  currency: 'USDT',
  network: 'tron',
  base_quota: 1000,
  bonus_quota: 0,
  expires_at: 1_900_000_000,
  needs_review: false,
}

beforeEach(() => {
  sessionStorage.clear()
  getOrder.mockReset()
  createOrder.mockReset()
})

it('keeps a pending trade number and only retries its status after a temporary GET failure', async () => {
  sessionStorage.setItem(CRYPTOMUS_PENDING_STORAGE_KEY, pendingOrder.trade_no)
  getOrder.mockResolvedValue({
    success: false,
    message_key: 'Failed to get order',
    message: 'Failed to get order',
  })

  const { result } = renderHook(() => useCryptomusPayment(vi.fn()))

  await waitFor(() => expect(result.current.recovering).toBe(true))
  expect(result.current.open).toBe(true)
  expect(sessionStorage.getItem(CRYPTOMUS_PENDING_STORAGE_KEY)).toBe(
    pendingOrder.trade_no
  )

  await act(() => result.current.createPayment(10))

  expect(getOrder).toHaveBeenCalledWith(pendingOrder.trade_no)
  expect(createOrder).not.toHaveBeenCalled()
  expect(sessionStorage.getItem(CRYPTOMUS_PENDING_STORAGE_KEY)).toBe(
    pendingOrder.trade_no
  )
})

it('clears the saved trade number only after the provider reports a terminal order', async () => {
  sessionStorage.setItem(CRYPTOMUS_PENDING_STORAGE_KEY, pendingOrder.trade_no)
  getOrder.mockResolvedValue({ success: true, data: { ...pendingOrder, status: 'success' } })
  const onPaid = vi.fn()

  renderHook(() => useCryptomusPayment(onPaid))

  await waitFor(() => {
    expect(sessionStorage.getItem(CRYPTOMUS_PENDING_STORAGE_KEY)).toBeNull()
  })
  expect(onPaid).toHaveBeenCalledTimes(1)
})
