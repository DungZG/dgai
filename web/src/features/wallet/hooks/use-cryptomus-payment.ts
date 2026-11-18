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
import i18next from 'i18next'
import { useCallback, useEffect, useRef, useState } from 'react'

import {
  getServerErrorMessage,
  getServerMessage,
} from '@/lib/server-error-message'

import {
  getCryptomusOrder,
  isApiSuccess,
  requestCryptomusPayment,
} from '../api'
import {
  CRYPTOMUS_PENDING_STORAGE_KEY,
  CRYPTOMUS_POLL_INTERVAL_MS,
} from '../constants'
import type { CryptomusOrder } from '../types'

// ============================================================================
// Cryptomus Payment Hook
// ============================================================================

function readPendingTradeNo(): string | null {
  try {
    return sessionStorage.getItem(CRYPTOMUS_PENDING_STORAGE_KEY)
  } catch {
    return null
  }
}

function rememberPendingTradeNo(tradeNo: string | null): void {
  try {
    if (tradeNo) {
      sessionStorage.setItem(CRYPTOMUS_PENDING_STORAGE_KEY, tradeNo)
    } else {
      sessionStorage.removeItem(CRYPTOMUS_PENDING_STORAGE_KEY)
    }
  } catch {
    // Storage can be blocked (private browsing); the order still shows this visit.
  }
}

/**
 * Owns the Cryptomus dialog state. A pending order is only ever resolved by
 * reading its status, so polling and the return trip can never create a
 * second order.
 */
export function useCryptomusPayment(onPaid: () => void) {
  const [open, setOpen] = useState(false)
  const [order, setOrder] = useState<CryptomusOrder | null>(null)
  const [creating, setCreating] = useState(false)
  const [checking, setChecking] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [pendingTradeNo, setPendingTradeNo] = useState(readPendingTradeNo)
  const paidRef = useRef(false)
  const onPaidRef = useRef(onPaid)
  onPaidRef.current = onPaid

  const applyOrder = useCallback((next: CryptomusOrder) => {
    setOrder(next)
    if (next.status === 'pending') {
      rememberPendingTradeNo(next.trade_no)
      setPendingTradeNo(next.trade_no)
    }
    if (next.status !== 'pending') {
      rememberPendingTradeNo(null)
      setPendingTradeNo(null)
    }
    if (next.status === 'success' && !paidRef.current) {
      paidRef.current = true
      onPaidRef.current()
    }
  }, [])
  const refresh = useCallback(
    async (
      tradeNo: string,
      options: { silent?: boolean; resume?: boolean } = {}
    ): Promise<boolean> => {
      if (!options.silent) setChecking(true)
      try {
        const response = await getCryptomusOrder(tradeNo)
        if (isApiSuccess(response) && response.data) {
          setError(null)
          applyOrder(response.data)
          if (options.resume) setOpen(true)
          return true
        }
        const message =
          getServerMessage(response) ??
          getServerErrorMessage(response, i18next.t('Payment request failed'))
        if (message === i18next.t('Order not found')) {
          rememberPendingTradeNo(null)
          setPendingTradeNo(null)
        }
        setError(message)
        return false
      } catch (caught) {
        setError(
          getServerErrorMessage(caught, i18next.t('Payment request failed'))
        )
        return false
      } finally {
        if (!options.silent) setChecking(false)
      }
    },
    [applyOrder]
  )

  const begin = useCallback(() => {
    setError(null)
    setOpen(true)
    const savedTradeNo = readPendingTradeNo()
    if (savedTradeNo) {
      setPendingTradeNo(savedTradeNo)
      void refresh(savedTradeNo, { resume: true })
      return
    }
    setOrder(null)
    paidRef.current = false
  }, [refresh])

  const createPayment = useCallback(
    async (amount: number) => {
      const savedTradeNo = readPendingTradeNo()
      if (savedTradeNo) {
        setPendingTradeNo(savedTradeNo)
        await refresh(savedTradeNo, { resume: true })
        return false
      }
      setCreating(true)
      setError(null)
      try {
        const response = await requestCryptomusPayment({ amount })
        if (!isApiSuccess(response) || !response.data) {
          setError(
            getServerMessage(response) ??
              getServerErrorMessage(
                response,
                i18next.t('Payment request failed')
              )
          )
          return false
        }
        rememberPendingTradeNo(response.data.trade_no)
        applyOrder(response.data)
        return true
      } catch (caught) {
        setError(
          getServerErrorMessage(caught, i18next.t('Payment request failed'))
        )
        return false
      } finally {
        setCreating(false)
      }
    },
    [applyOrder, refresh]
  )

  const checkPendingOrder = useCallback(() => {
    const tradeNo = order?.trade_no || pendingTradeNo || readPendingTradeNo()
    if (!tradeNo) return
    setPendingTradeNo(tradeNo)
    void refresh(tradeNo, { resume: true })
  }, [order?.trade_no, pendingTradeNo, refresh])

  useEffect(() => {
    const tradeNo = readPendingTradeNo()
    if (!tradeNo) return
    setPendingTradeNo(tradeNo)
    setOpen(true)
    void refresh(tradeNo, { silent: true, resume: true })
  }, [refresh])

  useEffect(() => {
    if (!pendingTradeNo) return
    const timer = setInterval(() => {
      void refresh(pendingTradeNo, { silent: true })
    }, CRYPTOMUS_POLL_INTERVAL_MS)
    return () => clearInterval(timer)
  }, [pendingTradeNo, order, refresh])


  return {
    open,
    setOpen,
    order,
    creating,
    checking,
    error,
    recovering: Boolean(pendingTradeNo) && !order,
    begin,
    createPayment,
    refresh,
    checkPendingOrder,
  }
}

